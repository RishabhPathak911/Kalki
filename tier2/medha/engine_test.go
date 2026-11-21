package medha

import (
	"encoding/json"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"math"
	"testing"
	"time"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	c, b, err := Load("../../config/thresholds.yaml", "../../config/playbooks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return New(c, b)
}
func observation(tenant, node string, at time.Time, values map[string]float64, risk bool) telemetry.Batch {
	r := telemetry.Record{Tenant: tenant, CollectorID: node, Region: "ap-south-1", CollectedAt: stamp(at), Source: "live", Metrics: map[string]telemetry.Stats{}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	for k, v := range values {
		r.Metrics[k] = telemetry.Stats{Min: v, Max: v, Avg: v, P95: v, Count: 5}
	}
	if risk {
		r.RiskEvents = append(r.RiskEvents, telemetry.Risk{Type: "bruteforce_suspected", Count: 12, Severity: "warning", WindowSec: 60})
	}
	return telemetry.Batch{Tenant: tenant, CollectorID: node, Region: r.Region, Records: []telemetry.Record{r}}
}

var quiet = map[string]float64{"cpu_pct": 20, "collector_cpu_pct": 1, "mem_pct": 30, "disk_pct": 40, "bandwidth_mbps": 1, "latency_ms": 10, "packet_loss_pct": 0, "service_up": 1}

func warm(t *testing.T, e *Engine, node string, start time.Time) time.Time {
	t.Helper()
	for i := 0; i < 35; i++ {
		at := start.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, at, quiet, false), at, false)
	}
	return start.Add(175 * time.Second)
}
func assertExplanations(t *testing.T, e *Engine) {
	t.Helper()
	for _, item := range e.Incidents("tenant_alpha") {
		if err := ValidateExplanation(item); err != nil {
			t.Fatalf("%s: %v", item.Pattern, err)
		}
		b, _ := json.Marshal(item)
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(b, &fields)
		if len(fields) != 18 {
			t.Fatalf("schema keys: %d", len(fields))
		}
		for _, key := range []string{"signals", "why_flagged", "why_not_just_threshold", "recommended_action", "why_this_action", "playbook_steps", "time_to_detect_ms"} {
			if _, ok := fields[key]; !ok {
				t.Fatal("missing", key)
			}
		}
	}
}
func TestTenMinuteQuietBaseline(t *testing.T) {
	e := testEngine(t)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 120; i++ {
		at := start.Add(time.Duration(i) * 5 * time.Second)
		values := map[string]float64{}
		for name, value := range quiet {
			values[name] = value
		}
		for _, name := range []string{"cpu_pct", "mem_pct", "disk_pct", "latency_ms"} {
			values[name] += .1 * math.Sin(float64(i))
		}
		e.Observe(observation("tenant_alpha", "host_11111111", at, values, false), at, false)
		e.Tick(at.Add(4 * time.Second))
	}
	if len(e.Incidents("tenant_alpha")) != 0 {
		t.Fatal("quiet baseline generated alerts")
	}
	fleet, _ := e.Fleet("tenant_alpha", start.Add(600*time.Second))
	if fleet[0].Health != "Excellent" {
		t.Fatal(fleet[0].Health)
	}
}
func TestAllCorrelationPatterns(t *testing.T) {
	for _, pattern := range []string{"network_path_issue", "local_workload", "possible_attack_exfiltration", "memory_leak", "capacity_risk", "shared_dependency", "collector_down", "buffer_full"} {
		t.Run(pattern, func(t *testing.T) {
			e := testEngine(t)
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			at := warm(t, e, "host_11111111", start)
			if pattern == "shared_dependency" {
				warm(t, e, "host_22222222", start)
			}
			if pattern == "collector_down" {
				e.Tick(at.Add(45 * time.Second))
			} else {
				for i := 0; i < 5; i++ {
					now := at.Add(time.Duration(i) * 5 * time.Second)
					v := map[string]float64{}
					switch pattern {
					case "network_path_issue":
						v = map[string]float64{"latency_ms": 250, "packet_loss_pct": 5}
					case "local_workload", "shared_dependency":
						v = map[string]float64{"cpu_pct": 85}
					case "possible_attack_exfiltration":
						v = map[string]float64{"cpu_pct": 85, "bandwidth_mbps": 30}
					case "memory_leak":
						v = map[string]float64{"mem_pct": 60 + float64(i)*3}
					case "capacity_risk":
						v = map[string]float64{"disk_pct": 70 + float64(i)*3}
					case "buffer_full":
						v = map[string]float64{"wal_buffer_full": 1}
					}
					e.Observe(observation("tenant_alpha", "host_11111111", now, v, pattern == "possible_attack_exfiltration"), now, false)
					if pattern == "shared_dependency" {
						e.Observe(observation("tenant_alpha", "host_22222222", now, v, false), now, false)
					}
				}
			}
			found := false
			for _, item := range e.Incidents("tenant_alpha") {
				if item.Pattern == pattern {
					found = true
					if pattern == "shared_dependency" && len(item.Correlated) == 0 {
						t.Fatal("missing peers")
					}
				}
			}
			if !found {
				t.Fatalf("missing %s: %+v", pattern, e.Incidents("tenant_alpha"))
			}
			assertExplanations(t, e)
		})
	}
}
func TestWarmupPersistenceHysteresisCooldownAndCPUSeparation(t *testing.T) {
	e := testEngine(t)
	start := time.Now()
	node := "host_11111111"
	for i := 0; i < 29; i++ {
		at := start.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, at, map[string]float64{"cpu_pct": 10 + float64(i%2)*30}, false), at, false)
	}
	if len(e.Incidents("tenant_alpha")) != 0 {
		t.Fatal("baseline alert before warmup")
	}
	e = testEngine(t)
	at := warm(t, e, node, start)
	for i := 0; i < 3; i++ {
		now := at.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, now, map[string]float64{"cpu_pct": 60}, false), now, false)
		fleet, _ := e.Fleet("tenant_alpha", now)
		want := "Good"
		if i == 2 {
			want = "Fair"
		}
		if fleet[0].Health != want {
			t.Fatalf("window %d want %s got %s", i, want, fleet[0].Health)
		}
	}
	if len(e.Incidents("tenant_alpha")) != 1 {
		t.Fatal("missing persistent baseline incident")
	}
	// Repeated observations update the active incident instead of duplicating it.
	for i := 3; i < 5; i++ {
		now := at.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, now, map[string]float64{"cpu_pct": 60}, false), now, false)
	}
	if len(e.Incidents("tenant_alpha")) != 1 {
		t.Fatal("duplicate incident")
	}
	// Recovery must survive multiple clear windows; a short flap cannot clear it.
	now := at.Add(25 * time.Second)
	e.Observe(observation("tenant_alpha", node, now, quiet, false), now, false)
	if e.Incidents("tenant_alpha")[0].Status != "ACTIVE" {
		t.Fatal("cleared on one window")
	}
	for i := 6; i < 12; i++ {
		now = at.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, now, quiet, false), now, false)
	}
	if e.Incidents("tenant_alpha")[0].Status != "RESOLVED" {
		t.Fatal("did not recover")
	}
	for i := 12; i < 15; i++ {
		now = at.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, now, map[string]float64{"cpu_pct": 60}, false), now, false)
	}
	if len(e.Incidents("tenant_alpha")) != 1 {
		t.Fatal("cooldown failed")
	}
	_, c := e.Fleet("tenant_alpha", now)
	if c.Suppressed == 0 || c.FalseAlarms == 0 {
		t.Fatal("missing suppression counters")
	}
	e = testEngine(t)
	at = warm(t, e, node, start)
	for i := 0; i < 3; i++ {
		now := at.Add(time.Duration(i) * 5 * time.Second)
		e.Observe(observation("tenant_alpha", node, now, map[string]float64{"collector_cpu_pct": 20, "cpu_pct": 20}, false), now, false)
	}
	if len(e.Incidents("tenant_alpha")) != 0 {
		t.Fatal("collector CPU mislabeled as host workload")
	}
	fleet, _ := e.Fleet("tenant_alpha", at.Add(10*time.Second))
	if fleet[0].Health != "Fair" {
		t.Fatal("collector overhead not reflected in health")
	}
}
func TestHeartbeatTransitionsBackfillAndTenantPeers(t *testing.T) {
	e := testEngine(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	b := observation("tenant_alpha", "host_11111111", at, quiet, false)
	e.Observe(b, at, false)
	fleet, _ := e.Fleet("tenant_alpha", at.Add(15*time.Second))
	if fleet[0].Health != "Excellent" {
		t.Fatal("stale too early")
	}
	fleet, _ = e.Fleet("tenant_alpha", at.Add(16*time.Second))
	if fleet[0].Health != "Poor" || fleet[0].Connection != "STALE" {
		t.Fatal("stale transition missing")
	}
	fleet, _ = e.Fleet("tenant_alpha", at.Add(45*time.Second))
	if fleet[0].Connection != "OFFLINE" {
		t.Fatal("offline late")
	}
	if len(e.Incidents("tenant_alpha")) != 1 {
		t.Fatal("offline alert missing")
	}
	for _, tr := range fleet[0].History {
		if _, err := time.Parse(collect.TimeLayout, tr.At); err != nil {
			t.Fatal(err)
		}
	}
	e.Observe(b, at.Add(46*time.Second), true)
	fleet, _ = e.Fleet("tenant_alpha", at.Add(46*time.Second))
	if fleet[0].Connection != "ONLINE" || fleet[0].Duplicates != 1 {
		t.Fatal("reconnect missing")
	}
	// Late accepted WAL history must not overwrite live metrics or fire stale alarms.
	old := observation("tenant_alpha", "host_11111111", at.Add(-time.Minute), map[string]float64{"cpu_pct": 99}, false)
	e.Observe(old, at.Add(47*time.Second), false)
	fleet, _ = e.Fleet("tenant_alpha", at.Add(47*time.Second))
	if fleet[0].Metrics["cpu_pct"].Avg != 20 {
		t.Fatal("late sample overwrote latest")
	}
	e = testEngine(t)
	warm(t, e, "host_11111111", at)
	for i := 0; i < 5; i++ {
		now := at.Add(time.Duration(35+i) * 5 * time.Second)
		e.Observe(observation("tenant_beta", "host_22222222", now, map[string]float64{"cpu_pct": 99}, false), now, false)
		e.Observe(observation("tenant_alpha", "host_11111111", now, map[string]float64{"cpu_pct": 85}, false), now, false)
	}
	for _, incident := range e.Incidents("tenant_alpha") {
		if incident.Pattern == "shared_dependency" || len(incident.Correlated) > 0 {
			t.Fatal("cross-tenant peer correlation")
		}
	}
}
