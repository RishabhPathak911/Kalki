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
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kalki/tier1/policy"
	"kalki/tier2/ingest"
	"kalki/tier2/store"
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
	publicURL := os.Getenv("RENDER_EXTERNAL_URL")
	hosted := publicURL != ""
	if hosted {
		u, e := url.Parse(publicURL)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.User != nil || u.RawQuery != "" {
			return fmt.Errorf("invalid public HTTPS origin")
		}
		*origin = publicURL
	}
	gate, err := policy.Load(*allowlist, nil)
	if err != nil {
		return err
	}
	a := workspace.NewAuthority(password("KALKI_ALPHA_PASSWORD", "alpha-demo-local"), password("KALKI_BETA_PASSWORD", "beta-demo-local"))
	if err := a.ConfigureMedha(*thresholds, *playbooks); err != nil {
		return err
	}
	servers := []*http.Server{}
	configs := []struct{ addr, origin, region string }{{*addr, *origin, "ap-south-1"}, {*euAddr, *euOrigin, "eu-central-1"}}
	if hosted {
		region := password("KALKI_REGION", "ap-south-1")
		configs = configs[:1]
		configs[0].region = region
		configs[0].addr = "0.0.0.0:" + password("PORT", "10000")
	}
	db, err := store.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	if db != nil {
		defer func() {
			ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
			defer c()
			if db.Close(ctx) != nil {
				log.Print("telemetry drain incomplete")
			}
		}()
	}
	for _, cfg := range configs {
		receiver, err := ingest.New(gate, cfg.region, *capacity)
		if err != nil {
			return err
		}
		handler, err := workspace.New(a, receiver, gate, cfg.region, cfg.origin, *releases)
		if err != nil {
			return err
		}
		var serve http.Handler = handler
		if hosted {
			serve = renderHandler(handler, db)
		}
		servers = append(servers, &http.Server{Addr: cfg.addr, Handler: serve, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384, ErrorLog: log.New(io.Discard, "", 0)})
	}
	if err := a.AttachStore(db, *capacity); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go a.RunHealth(ctx)
	done := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) {
			if hosted {
				done <- s.ListenAndServe()
			} else {
				done <- s.ListenAndServeTLS(*cert, *key)
			}
		}(s)
	}
	if db != nil {
		fmt.Println("Kalki: persistent PostgreSQL")
	} else {
		fmt.Println("Kalki: volatile in-memory")
	}
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

// Only installed in explicit Render mode; the internal listener is reached via
// Render's TLS-terminating ingress. Core gateways still require r.TLS != nil.
func renderHandler(next http.Handler, db *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != "GET" {
				w.WriteHeader(405)
				return
			}
			if db != nil && db.Ping(r.Context()) != nil {
				w.WriteHeader(503)
				io.WriteString(w, `{"status":"unavailable"}`)
				return
			}
			io.WriteString(w, `{"status":"ok"}`)
			return
		}
		if r.TLS == nil {
			if strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")) != "https" {
				http.Error(w, "HTTPS required", 426)
				return
			}
			r = r.Clone(r.Context())
			r.TLS = &tls.ConnectionState{Version: tls.VersionTLS12, HandshakeComplete: true}
		}
		next.ServeHTTP(w, r)
	})
}
