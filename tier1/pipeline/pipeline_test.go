package pipeline

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kalki/tier1/collect"
	"kalki/tier1/filter"
	"kalki/tier1/policy"
)

type idleSource struct{ stopped chan struct{} }

func (idleSource) Name() string { return "idle-test" }
func (s idleSource) Stream(ctx context.Context) <-chan collect.RawSample {
	out := make(chan collect.RawSample)
	go func() {
		defer close(out)
		defer close(s.stopped)
		select {
		case out <- collect.RawSample{CollectedAt: collect.Timestamp(time.Now()), Source: collect.Live, Metric: "cpu_pct", Value: 30}:
		case <-ctx.Done():
			return
		}
		<-ctx.Done()
	}()
	return out
}

func TestIdleWindowFlushAndSinkFailureJoinSources(t *testing.T) {
	id, err := filter.LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	gate, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	source := idleSource{stopped: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	want := errors.New("test sink failure")
	err = Run(ctx, []collect.Source{source}, Options{Tenant: "tenant_alpha", NodeID: "node-1", Region: "ap-south-1", Identity: id, Gate: gate}, func(policy.Approved) error { return want })
	if !errors.Is(err, want) || ctx.Err() != nil {
		t.Fatalf("flush did not run before timeout: %v", err)
	}
	select {
	case <-source.stopped:
	default:
		t.Fatal("source goroutine was not joined")
	}
}

func TestHostedWindowFlushesInitialSampleImmediately(t *testing.T) {
	id, err := filter.LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	gate, err := policy.Load("../../config/allowlist.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	source := idleSource{stopped: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	want := errors.New("initial sample received")
	err = Run(ctx, []collect.Source{source}, Options{Window: 2 * time.Second, Tenant: "tenant_alpha", NodeID: "node", Region: "ap-south-1", Identity: id, Gate: gate}, func(policy.Approved) error { return want })
	if !errors.Is(err, want) || ctx.Err() != nil {
		t.Fatal("initial sample waited for rollup timer")
	}
	select {
	case <-source.stopped:
	default:
		t.Fatal("producer leaked")
	}
}
