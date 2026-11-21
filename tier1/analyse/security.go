// Package analyse holds detectors that consume only filtered/aggregated data.
package analyse

import (
	"sort"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/filter"
)

type state struct {
	failures    []filter.Failure
	active      map[string]bool
	totalActive bool
}
type Security struct{ sources map[string]*state }

// Observe uses local arrival times, so skewed/out-of-order source timestamps
// cannot extend an attack window. Separate LIVE/REPLAYED streams never mix.
func (d *Security) Observe(source string, failures []filter.Failure, now time.Time) []telemetry.Risk {
	if d.sources == nil {
		d.sources = map[string]*state{}
	}
	s := d.sources[source]
	if s == nil {
		s = &state{active: map[string]bool{}}
		d.sources[source] = s
	}
	cutoff := now.Add(-60 * time.Second)
	kept := s.failures[:0]
	counts := map[string]int{}
	for _, f := range s.failures {
		if f.At.After(cutoff) && !f.At.After(now) {
			kept = append(kept, f)
			counts[f.SourcePseudonym]++
		}
	}
	for peer := range s.active {
		if counts[peer] < 5 {
			delete(s.active, peer)
		}
	}
	// Rearm from the old window before adding new failures. An idle source may
	// not have been observed for minutes; its previous alert must still expire.
	if len(kept) < 10 {
		s.totalActive = false
	}
	for _, f := range failures {
		if f.At.After(cutoff) && !f.At.After(now) {
			kept = append(kept, f)
			counts[f.SourcePseudonym]++
		}
	}
	s.failures = kept
	var out []telemetry.Risk
	peers := make([]string, 0, len(counts))
	for peer := range counts {
		peers = append(peers, peer)
	}
	sort.Strings(peers)
	for _, peer := range peers {
		n := counts[peer]
		if n >= 5 && !s.active[peer] {
			out = append(out, telemetry.Risk{Type: "bruteforce_suspected", Severity: "warning", Count: n, WindowSec: 60, SourcePseudonym: peer})
			s.active[peer] = true
		}
	}
	if len(kept) >= 10 && !s.totalActive && len(out) == 0 {
		out = append(out, telemetry.Risk{Type: "bruteforce_suspected", Severity: "warning", Count: len(kept), WindowSec: 60})
	}
	s.totalActive = len(kept) >= 10
	return out
}
