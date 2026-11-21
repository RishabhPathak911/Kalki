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
	a := workspace.NewAuthority(password("KALKI_ALPHA_PASSWORD", "alpha-demo-local"), password("KALKI_BETA_PASSWORD", "beta-demo-local"))
	if err := a.ConfigureMedha(*thresholds, *playbooks); err != nil {
		return err
	}
	servers := []*http.Server{}
	for _, cfg := range []struct{ addr, origin, region string }{{*addr, *origin, "ap-south-1"}, {*euAddr, *euOrigin, "eu-central-1"}} {
		receiver, err := ingest.New(gate, cfg.region, *capacity)
		if err != nil {
			return err
		}
		handler, err := workspace.New(a, receiver, gate, cfg.region, cfg.origin, *releases)
		if err != nil {
			return err
		}
		servers = append(servers, &http.Server{Addr: cfg.addr, Handler: handler, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384, ErrorLog: log.New(io.Discard, "", 0)})
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go a.RunHealth(ctx)
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
