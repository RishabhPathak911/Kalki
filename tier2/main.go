package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kalki/tier1/policy"
	"kalki/tier2/ingest"
	"kalki/tier2/workspace"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func password(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:8443", "ap-south-1 HTTPS listener")
	euAddr := flag.String("eu-addr", "127.0.0.1:8444", "eu-central-1 HTTPS listener")
	origin := flag.String("public-url", "https://localhost:8443", "LAN-accessible ap-south-1 HTTPS origin for onboarding")
	euOrigin := flag.String("eu-public-url", "https://localhost:8444", "LAN-accessible eu-central-1 HTTPS origin")
	cert := flag.String("cert", "data/certs/server.pem", "server certificate")
	key := flag.String("key", "data/certs/server.key", "server key")
	allowlist := flag.String("allowlist", "config/allowlist.yaml", "privacy schema")
	releases := flag.String("releases", "data/releases", "read-only collector release directory")
	capacity := flag.Int("ring-capacity", 4096, "retained records per node")
	thresholds := flag.String("thresholds", "config/thresholds.yaml", "Medha thresholds")
	playbooks := flag.String("playbooks", "config/playbooks.yaml", "Medha explanations")
	flag.Parse()

	gate, err := policy.Load(*allowlist, nil)
	if err != nil {
		return err
	}
	a := workspace.NewAuthority(
		password("KALKI_ALPHA_PASSWORD", "alpha-demo-local"),
		password("KALKI_BETA_PASSWORD", "beta-demo-local"),
	)
	if err := a.ConfigureMedha(*thresholds, *playbooks); err != nil {
		return err
	}

	// Use Render's external HTTPS URL as the onboarding origin when running on Render.
	apPublicURL := *origin
	euPublicURL := *euOrigin
	if ext := strings.TrimRight(os.Getenv("RENDER_EXTERNAL_URL"), "/"); ext != "" {
		if os.Getenv("KALKI_REGION") == "eu-central-1" {
			euPublicURL = ext
		} else {
			apPublicURL = ext
		}
	}

	handlers := map[string]http.Handler{}
	servers := []*http.Server{}
	for _, cfg := range []struct{ addr, origin, region string }{
		{*addr, apPublicURL, "ap-south-1"},
		{*euAddr, euPublicURL, "eu-central-1"},
	} {
		receiver, err := ingest.New(gate, cfg.region, *capacity)
		if err != nil {
			return err
		}
		handler, err := workspace.New(a, receiver, gate, cfg.region, cfg.origin, *releases)
		if err != nil {
			return err
		}
		handlers[cfg.region] = handler
		servers = append(servers, &http.Server{
			Addr:              cfg.addr,
			Handler:           handler,
			TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    16384,
			ErrorLog:          log.New(io.Discard, "", 0),
		})
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go a.RunHealth(ctx)

	// Render / cloud reverse-proxy mode: single plain-HTTP listener on 0.0.0.0:$PORT.
	if port := os.Getenv("PORT"); port != "" {
		region := os.Getenv("KALKI_REGION")
		if region == "" {
			region = "ap-south-1"
		}
		handler, ok := handlers[region]
		if !ok {
			return fmt.Errorf("invalid KALKI_REGION %q: expected ap-south-1 or eu-central-1", region)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Render terminates TLS at its edge and forwards plain HTTP with X-Forwarded-Proto: https.
			// workspace.Gateway and ingest.Server require r.TLS != nil; populate it for TLS-terminated
			// (or local PORT test) requests while still rejecting explicit X-Forwarded-Proto: http.
			if r.TLS == nil {
				if proto := r.Header.Get("X-Forwarded-Proto"); proto == "" || proto == "https" {
					r2 := r.Clone(r.Context())
					r2.TLS = &tls.ConnectionState{Version: tls.VersionTLS12}
					r = r2
				}
			}
			handler.ServeHTTP(w, r)
		}))

		httpAddr := "0.0.0.0:" + port
		srv := &http.Server{
			Addr:              httpAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    16384,
			ErrorLog:          log.New(io.Discard, "", 0),
		}

		done := make(chan error, 1)
		go func() { done <- srv.ListenAndServe() }()
		fmt.Printf("Kalki: listening on HTTP %s (region=%s); volatile in-memory state\n", httpAddr, region)

		select {
		case err = <-done:
		case <-ctx.Done():
			shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			if e := srv.Shutdown(shutdown); e != nil {
				err = errors.Join(err, e, srv.Close())
			}
			if e := <-done; !errors.Is(e, http.ErrServerClosed) {
				err = errors.Join(err, e)
			}
		}
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}

	// Default local/LAN dual-HTTPS mode.
	done := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) { done <- s.ListenAndServeTLS(*cert, *key) }(s)
	}
	fmt.Printf("Kalki: authenticated HTTPS regions on %s and %s; volatile in-memory state\n", *addr, *euAddr)
	received := 0
	select {
	case err = <-done:
		received++
	case <-ctx.Done():
	}
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	for _, s := range servers {
		if e := s.Shutdown(shutdown); e != nil {
			err = errors.Join(err, e, s.Close())
		}
	}
	for received < len(servers) {
		e := <-done
		received++
		if !errors.Is(e, http.ErrServerClosed) {
			err = errors.Join(err, e)
		}
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}