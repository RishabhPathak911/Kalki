package privacy

import (
	"encoding/json"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/filter"
	"kalki/tier1/policy"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorCPUPassesKavachIndependently(t *testing.T) {
	id, err := filter.LoadIdentity(filepath.Join(t.TempDir(), "key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	f := filter.Filter{Identity: id}
	sample, err := f.Process(collect.RawSample{CollectedAt: collect.Timestamp(time.Now()), Source: collect.Live, Metric: "collector_cpu_pct", Value: 7}, time.Now())
	if err != nil || sample.Metric != "collector_cpu_pct" {
		t.Fatal("filter dropped process CPU", err)
	}
	gate, err := policy.Load("../../config/allowlist.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := telemetry.Record{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1", Source: "live", CollectedAt: collect.Timestamp(time.Now()), Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 80, Max: 80, Avg: 80, P95: 80, Count: 1}, "collector_cpu_pct": {Min: 7, Max: 7, Avg: 7, P95: 7, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	raw, _ := json.Marshal(r)
	p, err := gate.Approve(raw)
	if err != nil {
		t.Fatal(err)
	}
	var safe telemetry.Record
	_ = json.Unmarshal(p.Bytes(), &safe)
	if safe.Metrics["cpu_pct"].Avg != 80 || safe.Metrics["collector_cpu_pct"].Avg != 7 || p.StrippedCount() != 0 {
		t.Fatal("CPU measurements conflated")
	}
	r.Metrics["collector_cpu_pct"] = telemetry.Stats{Min: 101, Max: 101, Avg: 101, P95: 101, Count: 1}
	raw, _ = json.Marshal(r)
	if _, err := gate.Approve(raw); err == nil {
		t.Fatal("out-of-range process CPU accepted")
	}
}
