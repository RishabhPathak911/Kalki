// Package collect contains local-only raw telemetry sources. RawSample must never
// be used as an outbound network payload.
package collect

import (
	"context"
	"fmt"
	"math"
	"time"
)

const (
	Live       = "live"
	Simulated  = "simulated"
	Replayed   = "replayed_real_trace"
	TimeLayout = "2006-01-02T15:04:05.000Z"
)

type RawSample struct {
	CollectedAt  string            `json:"collected_at"`
	OriginalTime string            `json:"orig_ts,omitempty"`
	Source       string            `json:"source"`
	Metric       string            `json:"metric,omitempty"`
	Value        float64           `json:"value"`
	Log          string            `json:"log,omitempty"`
	Hostname     string            `json:"hostname,omitempty"`
	Username     string            `json:"username,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
	// StreamID distinguishes traces/targets inside Tier 1; never export it.
	StreamID string `json:"stream_id,omitempty"`
}

type Source interface {
	Name() string
	Stream(context.Context) <-chan RawSample
}

type Reporter func(error)

func report(fn Reporter, err error) {
	if fn != nil && err != nil {
		fn(err)
	}
}
func Timestamp(t time.Time) string { return t.UTC().Format(TimeLayout) }
func finite(v float64) bool        { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func emit(ctx context.Context, out chan<- RawSample, s RawSample) bool {
	select {
	case out <- s:
		return true
	case <-ctx.Done():
		return false
	}
}
func pause(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func validateInterval(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	return nil
}
