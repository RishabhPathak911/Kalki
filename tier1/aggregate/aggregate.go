// Package aggregate builds five-second summaries exclusively from filtered data.
package aggregate

import (
	"fmt"
	"math"
	"sort"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/filter"
)

type Window struct {
	Record   telemetry.Record
	Failures []filter.Failure
}
type group struct {
	samples  int
	earliest time.Time
	values   map[string][]float64
	counts   map[string]int
	failures []filter.Failure
}
type Aggregator struct{ groups map[string]*group }

func (a *Aggregator) Add(s filter.Sample) error {
	if a.groups == nil {
		a.groups = map[string]*group{}
	}
	g := a.groups[s.Source]
	if g == nil {
		g = &group{earliest: s.CollectedAt, values: map[string][]float64{}, counts: map[string]int{}}
		a.groups[s.Source] = g
	}
	if g.samples >= 100000 {
		return fmt.Errorf("aggregate capacity exceeded; window blocked")
	}
	g.samples++
	if s.CollectedAt.Before(g.earliest) {
		g.earliest = s.CollectedAt
	}
	if s.Metric != "" {
		if len(g.values[s.Metric]) >= 100000 {
			return fmt.Errorf("aggregate capacity exceeded; window blocked")
		}
		g.values[s.Metric] = append(g.values[s.Metric], s.Value)
	}
	for kind, n := range s.RedactionCounts {
		g.counts[kind] += n
	}
	if s.Failure != nil {
		g.failures = append(g.failures, *s.Failure)
	}
	return nil
}

func (a *Aggregator) Flush(tenant, collector, region string) []Window {
	var out []Window
	sources := make([]string, 0, len(a.groups))
	for source := range a.groups {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	for _, source := range sources {
		g := a.groups[source]
		r := telemetry.Record{Tenant: tenant, CollectorID: collector, Region: region, CollectedAt: collect.Timestamp(g.earliest), Source: source, Metrics: map[string]telemetry.Stats{}, RiskEvents: []telemetry.Risk{}, RedactionCounts: g.counts}
		for metric, values := range g.values {
			sort.Float64s(values)
			var avg float64
			for i, v := range values {
				avg += (v - avg) / float64(i+1)
			}
			r.Metrics[metric] = telemetry.Stats{Min: values[0], Max: values[len(values)-1], Avg: avg, P95: values[int(math.Ceil(float64(len(values))*.95))-1], Count: float64(len(values))}
		}
		out = append(out, Window{Record: r, Failures: g.failures})
	}
	a.groups = nil
	return out
}
