package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"kalki/tier1/collect"
	"kalki/tier1/pipeline"
	"kalki/tier1/policy"
	"kalki/tier1/send"
	"kalki/tier1/wal"
)

type bridgeConfig struct {
	Demo                 func(context.Context, string) (map[string]any, error)
	Token                string
	URL, CA, Path, Proof string
	Capacity             int64
	Drain                time.Duration
	Print                bool
}

func runBridge(ctx context.Context, sources []collect.Source, options pipeline.Options, out io.Writer, cfg bridgeConfig) error {
	ca, err := os.ReadFile(cfg.CA)
	if err != nil {
		return fmt.Errorf("cannot read receiver CA certificate")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return fmt.Errorf("invalid receiver CA certificate")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	binding := wal.Binding{Tenant: options.Tenant, CollectorID: options.Identity.Pseudonym("host", options.NodeID), Region: options.Region}
	if options.CollectorID != "" {
		binding.CollectorID = options.CollectorID
	}
	journal, err := wal.Open(cfg.Path, binding, cfg.Capacity)
	if err != nil {
		return err
	}
	delivery, err := send.NewDelivery(journal, options.Gate, binding, cfg.URL, &http.Client{Transport: transport, Timeout: 3 * time.Second}, func(message string) { localLog("%s", message) })
	if err != nil {
		return errors.Join(err, journal.Close())
	}
	if journal.RepairedTail {
		localLog("WAL_RECOVERY: repaired incomplete trailing frame")
	}
	delivery.Token = cfg.Token
	delivery.Demo = cfg.Demo
	listener, handler, err := send.ListenProof(cfg.Proof, delivery)
	if err != nil {
		return errors.Join(err, journal.Close())
	}
	proofServer := &http.Server{Handler: handler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	proofDone := make(chan error, 1)
	go func() {
		err := proofServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		proofDone <- err
	}()
	localLog("Phase 3: HTTPS delivery enabled; localhost proof inspection at http://%s/api/egress-proof", listener.Addr().String())
	collectCtx, stopCollect := context.WithCancel(ctx)
	defer stopCollect()
	deliveryCtx, stopDelivery := context.WithCancel(context.Background())
	deliveryDone := make(chan error, 1)
	go func() {
		err := delivery.Run(deliveryCtx)
		if err != nil {
			localLog("delivery stopped: %v", err)
			stopCollect()
		}
		deliveryDone <- err
	}()
	pipeErr := pipeline.Run(collectCtx, sources, options, func(p policy.Approved) error {
		if err := delivery.Enqueue(p); err != nil {
			return err
		}
		if cfg.Print {
			return send.Preview(out, p)
		}
		return nil
	})
	deadline := time.Now().Add(cfg.Drain)
	for journal.Depth() > 0 && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	stopDelivery()
	deliveryErr := <-deliveryDone
	shutdownCtx, stopShutdown := context.WithTimeout(context.Background(), 3*time.Second)
	shutdownErr := proofServer.Shutdown(shutdownCtx)
	stopShutdown()
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, proofServer.Close())
	}
	proofErr := <-proofDone
	c := delivery.Counters()
	localLog("Setu stopped: sent=%d acked=%d retried=%d wal_depth=%d last_seq=%d", c.Sent, c.Acked, c.Retried, c.WALDepth, c.LastSeq)
	return errors.Join(pipeErr, deliveryErr, shutdownErr, proofErr, journal.Close())
}
