// validate runs named acceptance assertions and writes evidence, never invented scores.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kalki/internal/scale"
	"kalki/tier2/medha"
	"os"
	"os/exec"
	"sort"
	"time"
)

type check struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

func verdict(id string, ok bool, evidence string) check {
	s := "FAIL"
	if ok {
		s = "PASS"
	}
	return check{id, s, evidence}
}
func main() {
	checks := []check{}
	tests := map[string]bool{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "./tier1/filter", "./tier2/privacy", "./tier2/ingest", "./tier1/send", "./tier2/workspace", "-run", "Test(SecretRules|FilterDropsAllTextAndPseudonymizesIdentities|NoSecretsAtTier2Boundary|ViolationNeverInvokesSender|IdempotentOutOfOrderAndLateness|ThirtySecondOutageAndReceiverProcessRestart|TenantBoundaryEveryRoute|ClaimsRegionRotationAuditAndLogout|AuditDeletionAndCredentialStorage)$")
	output, err := cmd.Output()
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var event struct{ Action, Test string }
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Action == "pass" {
			tests[event.Test] = true
		}
	}
	cases := []struct {
		id    string
		names []string
	}{
		{"no_secrets_egress", []string{"TestSecretRules", "TestFilterDropsAllTextAndPseudonymizesIdentities", "TestNoSecretsAtTier2Boundary", "TestViolationNeverInvokesSender"}},
		{"idempotency_dedupe", []string{"TestIdempotentOutOfOrderAndLateness"}},
		{"out_of_order_event_time", []string{"TestIdempotentOutOfOrderAndLateness"}},
		{"zero_loss_after_outage", []string{"TestThirtySecondOutageAndReceiverProcessRestart"}},
		{"tenant_isolation_and_wrong_region_421", []string{"TestTenantBoundaryEveryRoute", "TestClaimsRegionRotationAuditAndLogout"}},
		{"audit_chain_integrity_and_tamper_detect", []string{"TestClaimsRegionRotationAuditAndLogout", "TestAuditDeletionAndCredentialStorage"}},
	}
	for _, c := range cases {
		ok := err == nil
		for _, name := range c.names {
			ok = ok && tests[name]
		}
		checks = append(checks, verdict(c.id, ok, fmt.Sprint(c.names)))
	}
	checks = append(checks, latencyCheck("data/dashboard-latency.json"))
	cfg, books, e := medha.Load("config/thresholds.yaml", "config/playbooks.yaml")
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	engine := medha.New(cfg, books)
	start := time.Now().UTC()
	for i := 0; i < 120; i++ {
		at := start.Add(time.Duration(i) * 5 * time.Second)
		engine.Observe(observation(at, 20), at, false)
	}
	checks = append(checks, verdict("zero_false_alarms_quiet_baseline", len(engine.Incidents("tenant_alpha")) == 0, "120 quiet observations over ten simulated event-time minutes"))
	at := start.Add(600 * time.Second)
	began := time.Now()
	engine.Observe(observation(at, 100), at, false)
	inc := engine.Incidents("tenant_alpha")
	ms := time.Since(began).Milliseconds()
	ok := len(inc) > 0
	for _, v := range inc {
		ok = ok && v.DetectionMS+ms < 15000
	}
	checks = append(checks, verdict("alert_latency_under_15s", ok, fmt.Sprintf("critical CPU fixture; processing %dms; event-time detection checked", ms)))
	engine.Tick(at.Add(46 * time.Second))
	fleet, _ := engine.Fleet("tenant_alpha", at.Add(46*time.Second))
	checks = append(checks, verdict("offline_detected_under_60s", len(fleet) == 1 && fleet[0].Connection == "OFFLINE" && fleet[0].Health == "Poor", "virtual-clock tick at 46s after last heartbeat; not a wall-clock scheduler benchmark"))
	fmt.Println("Measuring 50 SIMULATED collectors / 5 tenants for 3 seconds...")
	load, le := scale.Run(scale.Options{Collectors: 50, Tenants: 5, Duration: 3 * time.Second, Policy: "config/allowlist.yaml"})
	status := "PASS"
	for _, c := range checks {
		fmt.Printf("%s %s: %s\n", c.Status, c.ID, c.Evidence)
		if c.Status != "PASS" {
			status = "FAIL"
		}
	}
	if le != nil || load.Failed > 0 {
		status = "FAIL"
	}
	quality := nab("data/datasets/nab")
	if quality.Status != "evaluated" && quality.Status != "datasets_not_downloaded" {
		status = "FAIL"
	}
	report := map[string]any{"status": status, "generated_at": time.Now().UTC().Format(time.RFC3339), "checks": checks, "nab": quality, "loadgen": load}
	if le != nil {
		report["loadgen_error"] = "scale measurement failed"
	}
	b, e := json.MarshalIndent(report, "", "  ")
	if e == nil {
		e = os.MkdirAll("data", 0700)
	}
	if e == nil {
		e = os.WriteFile("data/validation-report.json.tmp", b, 0600)
	}
	if e == nil {
		e = os.Rename("data/validation-report.json.tmp", "data/validation-report.json")
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("Wrote data/validation-report.json; overall", status)
	if status != "PASS" {
		os.Exit(1)
	}
}
func latencyCheck(path string) check {
	var v struct {
		Generated string    `json:"generated_at"`
		Samples   []float64 `json:"samples_ms"`
	}
	b, e := os.ReadFile(path)
	if e != nil || json.Unmarshal(b, &v) != nil {
		return verdict("dashboard_latency_under_10s", false, "missing measured browser evidence: export from Validation tab to data/dashboard-latency.json")
	}
	at, e := time.Parse(time.RFC3339Nano, v.Generated)
	valid := e == nil && time.Since(at) >= 0 && time.Since(at) < 15*time.Minute && len(v.Samples) >= 30
	for _, s := range v.Samples {
		if s < 0 {
			valid = false
		}
	}
	sort.Float64s(v.Samples)
	p95 := 0.0
	if len(v.Samples) > 0 {
		p95 = v.Samples[(len(v.Samples)-1)*95/100]
	}
	return verdict("dashboard_latency_under_10s", valid && p95 < 10000, fmt.Sprintf("browser export: %d collected_at-to-render samples; p95 %.1fms; must be fresh, >=30 samples", len(v.Samples), p95))
}
