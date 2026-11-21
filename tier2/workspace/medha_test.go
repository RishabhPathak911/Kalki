package workspace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"kalki/internal/telemetry"
	"net/http"
	"testing"
	"time"
)

func phase5(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	if err := f.a.ConfigureMedha("../../config/thresholds.yaml", "../../config/playbooks.yaml"); err != nil {
		t.Fatal(err)
	}
	return f
}
func activeIncident(t *testing.T, f *fixture, session string) string {
	t.Helper()
	token := mint(t, f.aps, session, "host_11111111", false)["token"].(string)
	data := batch(t, f, "tenant_alpha", "ap-south-1", "host_11111111")
	var b telemetry.Batch
	_ = json.Unmarshal(data, &b)
	b.Records[0].Metrics["cpu_pct"] = telemetry.Stats{Min: 99, Max: 99, Avg: 99, P95: 99, Count: 1}
	raw, _ := json.Marshal(b)
	p, err := f.gate.ApproveBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := request(t, f.aps, "POST", "/api/ingest", token, p.Bytes(), f.ap.receiver.ID()); code != 200 {
		t.Fatal(code)
	}
	items := f.a.engine.Incidents("tenant_alpha")
	if len(items) != 1 {
		t.Fatalf("expected active incident: %+v", items)
	}
	return items[0].ID
}
func TestRemediationPolicyLifecycleAuditAndIsolation(t *testing.T) {
	f := phase5(t)
	alpha := login(t, f.aps, "tenant_alpha")
	beta := login(t, f.eus, "tenant_beta")
	incident := activeIncident(t, f, alpha)
	payload, _ := json.Marshal(map[string]string{"incident_id": incident})
	if code, _ := request(t, f.eus, "POST", "/api/remediations/propose", beta, payload, ""); code != 404 {
		t.Fatal("cross-tenant proposal", code)
	}
	code, out := request(t, f.aps, "POST", "/api/remediations/propose", alpha, payload, "")
	if code != 200 {
		t.Fatal(code)
	}
	var item Remediation
	if json.Unmarshal(out, &item) != nil {
		t.Fatal("JSON")
	}
	if !item.Reversible || !item.TargetValid || !item.Authorized || item.Status != "PROPOSED" {
		t.Fatal("policy flags", item)
	}
	base := "/api/remediations/" + item.ID
	if code, _ := request(t, f.eus, "POST", base+"/approve", beta, []byte(`{"approved":true}`), ""); code != 404 {
		t.Fatal("cross-tenant approval", code)
	}
	if code, _ := request(t, f.aps, "POST", base+"/approve", alpha, []byte(`{"approved":false}`), ""); code != 400 {
		t.Fatal("implicit approval", code)
	}
	code, out = request(t, f.aps, "POST", base+"/approve", alpha, []byte(`{"approved":true}`), "")
	if code != 200 || object(t, out)["status"] != "EXECUTED_SIMULATED" {
		t.Fatal("approval failed", code)
	}
	if code, _ := request(t, f.aps, "POST", base+"/approve", alpha, []byte(`{"approved":true}`), ""); code != 409 {
		t.Fatal("repeated execution", code)
	}
	if code, _ := request(t, f.eus, "POST", base+"/rollback", beta, nil, ""); code != 404 {
		t.Fatal("cross-tenant rollback", code)
	}
	code, out = request(t, f.aps, "POST", base+"/rollback", alpha, nil, "")
	if code != 200 || object(t, out)["status"] != "ROLLED_BACK" {
		t.Fatal("rollback failed", code)
	}
	_, out = request(t, f.aps, "GET", "/api/audit", alpha, nil, "")
	for _, event := range []string{"remediation_propose", "remediation_approve", "remediation_rollback"} {
		if !bytes.Contains(out, []byte(event)) {
			t.Fatal("missing audit", event)
		}
	}
	_, out = request(t, f.aps, "GET", "/api/audit/verify", alpha, nil, "")
	if object(t, out)["valid"] != true {
		t.Fatal("audit chain")
	}
	for _, path := range []string{"/api/fleet", "/api/incidents", "/api/remediations", "/api/thresholds"} {
		code, out := request(t, f.eus, "GET", path, beta, nil, "")
		if code != 200 {
			t.Fatal(path, code)
		}
		if bytes.Contains(out, []byte("host_11111111")) || bytes.Contains(out, []byte(incident)) {
			t.Fatal("cross-tenant read", path)
		}
		if code, _ := request(t, f.aps, "GET", path+"?tenant=tenant_alpha", beta, nil, ""); code != 400 {
			t.Fatal("selector accepted")
		}
		if code, _ := request(t, f.aps, "GET", path, "", nil, ""); code != 401 {
			t.Fatal("anonymous read")
		}
	}
	// Reject a policy-forged request body rather than trusting caller flags.
	if code, _ := request(t, f.aps, "POST", "/api/remediations/propose", alpha, []byte(`{"incident_id":"x","tenant_authorized":true}`), ""); code != 400 {
		t.Fatal("forged policy accepted")
	}
}
func TestSSETenantIsolationNoLockStarvationAndLogout(t *testing.T) {
	f := phase5(t)
	alpha := login(t, f.aps, "tenant_alpha")
	beta := login(t, f.eus, "tenant_beta")
	_ = activeIncident(t, f, alpha)
	if code, _ := request(t, f.aps, "GET", "/api/stream?token="+beta, "", nil, ""); code != 400 {
		t.Fatal("URL token accepted")
	}
	if code, _ := request(t, f.aps, "GET", "/api/stream", "", nil, ""); code != 401 {
		t.Fatal("anonymous SSE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, _ := http.NewRequestWithContext(ctx, "GET", f.aps.URL+"/api/stream", nil)
	r.Header.Set("Authorization", "Bearer "+beta)
	resp, err := f.aps.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("not SSE")
	}
	reader := bufio.NewReader(resp.Body)
	var initial bytes.Buffer
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		initial.WriteString(line)
		if line == "\n" {
			break
		}
	}
	if bytes.Contains(initial.Bytes(), []byte("host_11111111")) || bytes.Contains(initial.Bytes(), []byte("tenant_alpha")) {
		t.Fatal("SSE tenant leak")
	}
	// A stream must not retain the shared auth lock and block logout/ingest.
	if code, _ := request(t, f.eus, "POST", "/api/auth/logout", beta, nil, ""); code != 200 {
		t.Fatal(code)
	}
	_, err = io.Copy(io.Discard, reader)
	if err != nil && ctx.Err() != nil {
		t.Fatal("stream did not close on logout")
	}
}
func TestSSEPublishesTelemetryHealthIncidentAndRemediation(t *testing.T) {
	f := phase5(t)
	alpha := login(t, f.aps, "tenant_alpha")
	id := activeIncident(t, f, alpha)
	payload, _ := json.Marshal(map[string]string{"incident_id": id})
	if code, _ := request(t, f.aps, "POST", "/api/remediations/propose", alpha, payload, ""); code != 200 {
		t.Fatal(code)
	}
	f.a.engine.Tick(time.Now().Add(46 * time.Second))
	events, _ := f.a.engine.Events("tenant_alpha", 0)
	kinds := map[string]bool{}
	for _, e := range events {
		kinds[e.Kind] = true
	}
	for _, kind := range []string{"telemetry", "health", "incident", "remediation"} {
		if !kinds[kind] {
			t.Fatal("missing SSE event", kind)
		}
	}
	// Expiring a token is enforced even without an explicit logout.
	f.a.mu.Lock()
	key := bearerForTest(alpha)
	s := f.a.sessions[key]
	s.Expires = time.Now().Add(-time.Second)
	f.a.sessions[key] = s
	f.a.mu.Unlock()
	if code, _ := request(t, f.aps, "GET", "/api/stream", alpha, nil, ""); code != 401 {
		t.Fatal("expired SSE")
	}
}
func bearerForTest(token string) [32]byte {
	r := &http.Request{Header: http.Header{}}
	r.Header.Set("Authorization", "Bearer "+token)
	return bearer(r)
}
