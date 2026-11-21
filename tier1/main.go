package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kalki/tier1/collect"
	"kalki/tier1/filter"
	"kalki/tier1/pipeline"
	"kalki/tier1/policy"
	"kalki/tier1/send"
)

func env(key, fallback string) string {
	if value, ok := os.LookupEnv("KALKI_" + key); ok {
		return value
	}
	return fallback
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		localLog("%v", err)
		os.Exit(1)
	}
}

var diagnostics = log.New(os.Stderr, "", 0)

func localLog(format string, args ...any) {
	diagnostics.Printf("%s %s", collect.Timestamp(time.Now()), fmt.Sprintf(format, args...))
}

func run(args []string, output io.Writer) error {
	fs := flag.NewFlagSet("kalki-collector", flag.ContinueOnError)
	node := fs.String("node-id", env("NODE_ID", "node-1"), "local node identifier")
	collectorID := fs.String("collector-id", env("COLLECTOR_ID", ""), "opaque ID from workspace onboarding")
	token := fs.String("token", "", "collector bearer token (or KALKI_TOKEN)")
	tenant := fs.String("tenant", env("TENANT", "tenant_alpha"), "tenant configuration")
	region := fs.String("region", env("REGION", "ap-south-1"), "logical residency region")
	tier2 := fs.String("tier2-url", env("TIER2_URL", ""), "HTTPS receiver origin; enables durable WAL delivery")
	walPath := fs.String("wal-path", env("WAL_PATH", "data/collector.wal"), "append-only collector WAL; one file per collector")
	walCapacity := fs.String("wal-cap-bytes", env("WAL_CAP_BYTES", "67108864"), "WAL byte cap including retained history")
	caCert := fs.String("ca-cert", env("CA_CERT", "data/certs/ca.pem"), "trusted local receiver CA")
	proofAddr := fs.String("proof-addr", env("PROOF_ADDR", "127.0.0.1:9090"), "loopback-only proof inspection address")
	drainTimeout := fs.String("drain-timeout", env("DRAIN_TIMEOUT", "5s"), "shutdown delivery drain budget")
	interval := fs.String("interval", env("INTERVAL", "1s"), "host sampling duration")
	targets := fs.String("probe-target", env("PROBE_TARGET", ""), "comma-separated host:port targets; overrides defaults")
	mode := fs.String("mode", env("MODE", "live"), "live|replay|both")
	printMode := fs.Bool("print", env("PRINT", "false") == "true", "print RAW samples locally; never forward this output")
	demoEnabled := fs.Bool("demo-controls", false, "enable bounded loopback-only evaluation controls")
	printEgress := fs.Bool("print-egress", env("PRINT_EGRESS", "false") == "true", "print policy-approved aggregate JSON locally")
	keyPath := fs.String("key-file", env("KEY_FILE", "data/tenant.key"), "local pseudonym key file, bound to one tenant")
	allowlist := fs.String("allowlist", env("ALLOWLIST", "config/allowlist.yaml"), "outbound policy configuration")
	root := fs.String("datasets", env("DATASETS", "data/datasets"), "dataset root")
	config := fs.String("datasets-config", env("DATASETS_CONFIG", "config/datasets.yaml"), "mapping configuration")
	replayInterval := fs.String("replay-interval", env("REPLAY_INTERVAL", "1s"), "interval per CSV trace")
	logInterval := fs.String("log-interval", env("LOG_INTERVAL", "333.333333ms"), "log replay line interval")
	liveLog := fs.String("log-file", env("LOG_FILE", "data/live_auth.log"), "local replay/tail file")
	duration := fs.String("duration", env("DURATION", "0s"), "stop after duration; 0 runs until interrupted")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *token == "" {
		*token = env("TOKEN", "")
	}
	if *collectorID != "" && !regexp.MustCompile(`^host_[0-9a-f]{8}$`).MatchString(*collectorID) {
		return fmt.Errorf("invalid opaque collector ID")
	}
	if *tier2 != "" && *token == "" {
		return fmt.Errorf("HTTPS delivery requires --token or KALKI_TOKEN")
	}
	if *printMode && *printEgress {
		return fmt.Errorf("choose --print or --print-egress, not both")
	}
	if *node == "" || *tenant == "" || *region == "" {
		return fmt.Errorf("node-id, tenant and region must be nonempty")
	}
	if *mode != "live" && *mode != "replay" && *mode != "both" {
		return fmt.Errorf("invalid mode: use live, replay or both")
	}
	durations := make([]time.Duration, 4)
	for i, raw := range []string{*interval, *replayInterval, *logInterval, *duration} {
		value, err := time.ParseDuration(raw)
		if err != nil || value < 0 || (i < 3 && value == 0) {
			return fmt.Errorf("invalid duration %q", raw)
		}
		durations[i] = value
	}
	probes := []string{"1.1.1.1:443"}
	if *tier2 != "" {
		u, err := url.Parse(*tier2)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return fmt.Errorf("tier2-url must be HTTPS without credentials")
		}
		port := u.Port()
		if port == "" {
			port = "443"
		}
		probes = append(probes, net.JoinHostPort(u.Hostname(), port))
	}
	if *targets != "" {
		probes = strings.Split(*targets, ",")
	}
	unique := make([]string, 0, len(probes))
	seen := map[string]bool{}
	for _, target := range probes {
		target = strings.TrimSpace(target)
		host, port, err := net.SplitHostPort(target)
		if err != nil || host == "" || port == "" {
			return fmt.Errorf("probe-target must be host:port")
		}
		if !seen[target] {
			unique = append(unique, target)
			seen[target] = true
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if durations[3] > 0 {
		var stop context.CancelFunc
		ctx, stop = context.WithTimeout(ctx, durations[3])
		defer stop()
	}
	reporter := func(err error) { localLog("local source: %v", err) }
	sources := []collect.Source{}
	var demo *collect.DemoFault
	if *demoEnabled {
		demo = &collect.DemoFault{}
	}
	if *mode == "live" || *mode == "both" {
		sources = append(sources, &collect.LocalSource{Demo: demo, Interval: durations[0], Targets: unique, Report: reporter})
	}
	if *mode == "replay" || *mode == "both" {
		cfg, err := collect.LoadDatasets(*config)
		if err != nil {
			return fmt.Errorf("dataset config: %w", err)
		}
		paths := make([]string, 0, len(cfg.Logs))
		for _, path := range cfg.Logs {
			paths = append(paths, filepath.Join(*root, filepath.FromSlash(path)))
		}
		sources = append(sources, &collect.DatasetReplaySource{Root: *root, Mappings: cfg.Metrics, Interval: durations[1], Report: reporter}, &collect.LogSource{Files: paths, LivePath: *liveLog, Interval: durations[2], Report: reporter})
	}
	gate, err := policy.Load(*allowlist, func(event policy.Event) {
		localLog("policy action=%s reason=%s stripped=%d sha256=%s", event.Action, event.Reason, event.Stripped, event.SHA256)
	})
	if *demoEnabled && *mode == "live" {
		sources = append(sources, &collect.LogSource{LivePath: *liveLog, Interval: time.Second, Report: reporter})
	}
	if err != nil {
		return err
	}
	identity, err := filter.LoadIdentity(*keyPath, *tenant)
	if err != nil {
		return err
	}
	if *tier2 == "" {
		localLog("Privacy pipeline active; local preview only, no telemetry upload.")
	}
	if *printMode {
		localLog("--print is a RAW local diagnostic; use --print-egress to inspect approved payloads.")
	}
	encoder := json.NewEncoder(output)
	var count uint64
	options := pipeline.Options{Tenant: *tenant, NodeID: *node, Region: *region, Identity: identity, Gate: gate, Report: reporter}
	options.CollectorID = *collectorID
	options.RawSink = func(s collect.RawSample) error {
		count++
		if *printMode {
			return encoder.Encode(s)
		}
		return nil
	}
	if *tier2 != "" {
		capacity, parseErr := strconv.ParseInt(*walCapacity, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid WAL byte cap")
		}
		drain, parseErr := time.ParseDuration(*drainTimeout)
		if parseErr != nil || drain < 0 {
			return fmt.Errorf("invalid drain timeout")
		}
		var demoHook func(context.Context, string) (map[string]any, error)
		if demo != nil {
			demoHook = demoActions(ctx, demo, *liveLog)
		}
		err = runBridge(ctx, sources, options, output, bridgeConfig{Demo: demoHook, Token: *token, URL: *tier2, CA: *caCert, Path: *walPath, Proof: *proofAddr, Capacity: capacity, Drain: drain, Print: *printEgress})
	} else {
		err = pipeline.Run(ctx, sources, options, func(payload policy.Approved) error {
			if *printEgress {
				return send.Preview(output, payload)
			}
			return nil
		})
	}
	counters := gate.Counters()
	localLog("collector stopped; local samples=%d approved=%d blocked=%d stripped=%d", count, counters.Approved, counters.Blocked, counters.Stripped)
	return err
}
