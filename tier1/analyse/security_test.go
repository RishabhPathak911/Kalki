package analyse

import (
	"fmt"
	"testing"
	"time"

	"kalki/tier1/filter"
)

func TestBruteForceThresholdExpiryAndRearm(t *testing.T) {
	var d Security
	now := time.Now()
	f := filter.Failure{SourcePseudonym: "ip_12345678", At: now}
	if len(d.Observe("live", []filter.Failure{f, f, f, f}, now)) != 0 {
		t.Fatal("early alert")
	}
	events := d.Observe("live", []filter.Failure{f}, now)
	if len(events) != 1 || events[0].Count != 5 || events[0].WindowSec != 60 || events[0].SourcePseudonym != f.SourcePseudonym {
		t.Fatal("missing five-failure alert")
	}
	if len(d.Observe("live", []filter.Failure{f}, now)) != 0 {
		t.Fatal("duplicate alert")
	}
	if len(d.Observe("live", nil, now.Add(60*time.Second))) != 0 {
		t.Fatal("expired failures retained")
	}
	f.At = now.Add(61 * time.Second)
	if len(d.Observe("live", []filter.Failure{f, f, f, f, f}, f.At)) != 1 {
		t.Fatal("detector did not rearm")
	}
}

func TestTotalFailuresAndSourceIsolation(t *testing.T) {
	var d Security
	now := time.Now()
	failures := []filter.Failure{}
	for i := 0; i < 9; i++ {
		failures = append(failures, filter.Failure{SourcePseudonym: fmt.Sprintf("ip_%08x", i), At: now})
	}
	if len(d.Observe("live", failures, now)) != 0 {
		t.Fatal("total threshold fired early")
	}
	last := filter.Failure{SourcePseudonym: "ip_ffffffff", At: now}
	if len(d.Observe("replayed_real_trace", []filter.Failure{last}, now)) != 0 {
		t.Fatal("sources mixed")
	}
	events := d.Observe("live", []filter.Failure{last}, now)
	if len(events) != 1 || events[0].Count != 10 || events[0].SourcePseudonym != "" {
		t.Fatal("total threshold missing or misleading peer")
	}
}

func TestOutOfOrderFailureTimes(t *testing.T) {
	var d Security
	now := time.Now()
	failures := []filter.Failure{}
	for _, age := range []time.Duration{time.Second, 59 * time.Second, 3 * time.Second, 20 * time.Second, 2 * time.Second, 61 * time.Second} {
		failures = append(failures, filter.Failure{SourcePseudonym: "ip_12345678", At: now.Add(-age)})
	}
	events := d.Observe("live", failures, now)
	if len(events) != 1 || events[0].Count != 5 {
		t.Fatal("window depends on arrival ordering")
	}
}

func TestRearmAfterUnobservedIdlePeriod(t *testing.T) {
	var d Security
	now := time.Now()
	f := filter.Failure{SourcePseudonym: "ip_12345678", At: now}
	if len(d.Observe("live", []filter.Failure{f, f, f, f, f}, now)) != 1 {
		t.Fatal("initial alert missing")
	}
	f.At = now.Add(2 * time.Minute)
	if len(d.Observe("live", []filter.Failure{f, f, f, f, f}, f.At)) != 1 {
		t.Fatal("stale latch suppressed a new attack after idle")
	}
}
