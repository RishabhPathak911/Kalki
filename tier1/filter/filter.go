package filter

import (
	"fmt"
	"math"
	"net"
	"regexp"
	"strings"
	"time"

	"kalki/tier1/collect"
)

// Failure holds only a pseudonym and local observation time, never the address.
type Failure struct {
	SourcePseudonym string
	At              time.Time
}
type Sample struct {
	CollectedAt     time.Time
	Source          string
	Metric          string
	Value           float64
	HostPseudonym   string
	UserPseudonym   string
	Failure         *Failure
	RedactionCounts map[string]int
}

type Filter struct{ Identity *Identity }

var failedPassword = regexp.MustCompile(`(?i)\bFailed password for (?:invalid user )?(\S+) from (\S+)`)
var metricNames = map[string]bool{"collector_cpu_pct": true, "cpu_pct": true, "mem_pct": true, "disk_pct": true, "bandwidth_mbps": true, "latency_ms": true, "packet_loss_pct": true, "service_up": true}

func (f *Filter) Process(raw collect.RawSample, observedAt time.Time) (Sample, error) {
	if f.Identity == nil {
		return Sample{}, fmt.Errorf("privacy filter: missing identity")
	}
	at, err := time.Parse(collect.TimeLayout, raw.CollectedAt)
	if err != nil {
		return Sample{}, fmt.Errorf("privacy filter: invalid collected_at")
	}
	if raw.Source != collect.Live && raw.Source != collect.Replayed && raw.Source != collect.Simulated {
		return Sample{}, fmt.Errorf("privacy filter: invalid source")
	}
	if raw.Metric != "" && (!metricNames[raw.Metric] || math.IsNaN(raw.Value) || math.IsInf(raw.Value, 0) || raw.Value < 0) {
		return Sample{}, fmt.Errorf("privacy filter: invalid metric")
	}
	s := Sample{CollectedAt: at, Source: raw.Source, Metric: raw.Metric, Value: raw.Value, RedactionCounts: map[string]int{}}
	scrub := func(text string) string {
		clean, counts := Scrub(text)
		for kind, n := range counts {
			s.RedactionCounts[kind] += n
		}
		return clean
	}
	// Extract known identity fields locally, then scrub ALL text before producing
	// a Sample. Only HMAC results and counters can reach the next stage.
	match := failedPassword.FindStringSubmatch(raw.Log)
	scrub(raw.Log)
	scrub(raw.StreamID)
	scrub(raw.OriginalTime)
	if raw.Hostname != "" {
		scrub(raw.Hostname)
		s.HostPseudonym = f.Identity.Pseudonym("host", raw.Hostname)
	}
	if raw.Username != "" {
		scrub(raw.Username)
		s.UserPseudonym = f.Identity.Pseudonym("user", raw.Username)
	}
	for key, value := range raw.Tags {
		scrub(key)
		scrub(value)
	}
	if len(match) == 3 {
		// The full log was already scrubbed/counts recorded. HMAC the original
		// identity so distinct secret-looking names do not collapse to one marker.
		s.UserPseudonym = f.Identity.Pseudonym("user", match[1])
		ip := net.ParseIP(strings.Trim(match[2], "[]"))
		if ip != nil {
			s.Failure = &Failure{SourcePseudonym: f.Identity.Pseudonym("ip", ip.String()), At: observedAt}
		}
	}
	return s, nil
}
