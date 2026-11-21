package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
	"kalki/tier1/send"
	"kalki/tier1/wal"
	"kalki/tier2/ingest"
)

type fixture struct {
	a        *Authority
	gate     *policy.Gate
	ap, eu   *Gateway
	aps, eus *httptest.Server
	dir      string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	gate, err := policy.Load("../../config/allowlist.yaml", nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{a: NewAuthority("alpha-pass", "beta-pass"), gate: gate, dir: t.TempDir()}
	for _, cfg := range []struct {
		region string
		g      **Gateway
		s      **httptest.Server
	}{{"ap-south-1", &f.ap, &f.aps}, {"eu-central-1", &f.eu, &f.eus}} {
		recv, err := ingest.New(gate, cfg.region, 20)
		if err != nil {
			t.Fatal(err)
		}
		g, err := New(f.a, recv, gate, cfg.region, "https://localhost:8443", f.dir)
		if err != nil {
			t.Fatal(err)
		}
		*cfg.g = g
		s := httptest.NewTLSServer(g)
		*cfg.s = s
		g.origin = s.URL
		t.Cleanup(s.Close)
	}
	return f
}
func request(t *testing.T, s *httptest.Server, method, path, token string, body []byte, epoch string) (int, []byte) {
	t.Helper()
	r, err := http.NewRequest(method, s.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if epoch != "" {
		r.Header.Set("X-Kalki-Server-ID", epoch)
	}
	resp, err := s.Client().Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, b
}
func object(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if json.Unmarshal(b, &v) != nil {
		t.Fatal("invalid response JSON")
	}
	return v
}
func login(t *testing.T, s *httptest.Server, tenant string) string {
	t.Helper()
	pass := "alpha-pass"
	if tenant == "tenant_beta" {
		pass = "beta-pass"
	}
	b, _ := json.Marshal(map[string]string{"username": tenant, "password": pass})
	code, out := request(t, s, "POST", "/api/auth/login", "", b, "")
	if code != 200 {
		t.Fatalf("login: %d", code)
	}
	return object(t, out)["token"].(string)
}
func mint(t *testing.T, s *httptest.Server, session, id string, rotate bool) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"collector_id": id, "rotate": rotate})
	code, out := request(t, s, "POST", "/api/collectors/tokens", session, b, "")
	if code != 200 {
		t.Fatalf("mint: %d", code)
	}
	return object(t, out)
}
func batch(t *testing.T, f *fixture, tenant, region, id string) []byte {
	t.Helper()
	r := telemetry.Record{Tenant: tenant, Region: region, CollectorID: id, Source: "live", CollectedAt: collect.Timestamp(time.Now()), Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 42, Max: 42, Avg: 42, P95: 42, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	b, _ := json.Marshal(telemetry.Batch{Tenant: tenant, Region: region, CollectorID: id, Seq: 1, BatchID: "00000000000000000000000000000001", SentAt: collect.Timestamp(time.Now()), Records: []telemetry.Record{r}})
	p, err := f.gate.ApproveBatch(b)
	if err != nil {
		t.Fatal(err)
	}
	return p.Bytes()
}
func TestTenantBoundaryEveryRoute(t *testing.T) {
	f := setup(t)
	alpha := login(t, f.aps, "tenant_alpha")
	beta := login(t, f.eus, "tenant_beta")
	at := mint(t, f.aps, alpha, "host_11111111", false)["token"].(string)
	bt := mint(t, f.eus, beta, "host_22222222", false)["token"].(string)
	b := batch(t, f, "tenant_beta", "eu-central-1", "host_22222222")
	if code, _ := request(t, f.eus, "POST", "/api/ingest", bt, b, f.eu.receiver.ID()); code != 200 {
		t.Fatal(code)
	}
	// Positive control: B really has data, and can read through either listener.
	for _, s := range []*httptest.Server{f.aps, f.eus} {
		code, out := request(t, s, "GET", "/api/records?collector_id=host_22222222", beta, nil, "")
		if code != 200 || !bytes.Contains(out, []byte("tenant_beta")) {
			t.Fatal("missing beta data")
		}
		code, out = request(t, s, "GET", "/api/records?collector_id=host_22222222", alpha, nil, "")
		if code != 200 || len(object(t, out)["records"].([]any)) != 0 {
			t.Fatal("cross-tenant read")
		}
		for _, path := range []string{"/api/global", "/api/audit", "/api/audit/verify", "/api/events", "/api/remediations", "/api/collectors/tokens"} {
			_, out := request(t, s, "GET", path, alpha, nil, "")
			if bytes.Contains(out, []byte("host_22222222")) || bytes.Contains(out, []byte("tenant_beta")) || bytes.Contains(out, []byte(bt)) {
				t.Fatalf("leak at %s", path)
			}
			code, _ := request(t, s, "GET", path+"?tenant=tenant_beta", alpha, nil, "")
			if code < 400 {
				t.Fatalf("tenant selector accepted: %s", path)
			}
			code, _ = request(t, s, "GET", path, at, nil, "")
			if code != 401 {
				t.Fatalf("collector used as session: %s", path)
			}
		}
		for _, path := range []string{"/api/records?tenant=tenant_beta&collector_id=host_22222222", "/api/records?region=eu-central-1"} {
			code, _ := request(t, s, "GET", path, alpha, nil, "")
			if code != 403 {
				t.Fatal(code)
			}
		}
	}
	if code, _ := request(t, f.aps, "POST", "/api/collectors/tokens", alpha, []byte(`{"tenant":"tenant_beta","collector_id":"host_22222222"}`), ""); code != 400 {
		t.Fatal(code)
	}
	if code, _ := request(t, f.aps, "POST", "/api/collectors/tokens", alpha, []byte(`{"rotate":true,"collector_id":"host_22222222"}`), ""); code != 409 {
		t.Fatal(code)
	}
	if code, _ := request(t, f.aps, "POST", "/api/ingest", alpha, b, f.ap.receiver.ID()); code != 401 {
		t.Fatal("session used as collector")
	}
}
func TestClaimsRegionRotationAuditAndLogout(t *testing.T) {
	f := setup(t)
	session := login(t, f.aps, "tenant_alpha")
	old := mint(t, f.aps, session, "host_11111111", false)["token"].(string)
	cases := []struct {
		s      *httptest.Server
		b      []byte
		status int
	}{{f.aps, batch(t, f, "tenant_beta", "ap-south-1", "host_11111111"), 403}, {f.aps, batch(t, f, "tenant_alpha", "eu-central-1", "host_11111111"), 421}, {f.eus, batch(t, f, "tenant_alpha", "ap-south-1", "host_11111111"), 421}, {f.aps, batch(t, f, "tenant_alpha", "ap-south-1", "host_33333333"), 403}, {f.aps, []byte(`{"raw_log":"secret"}`), 400}}
	for _, c := range cases {
		code, _ := request(t, c.s, "POST", "/api/ingest", old, c.b, f.ap.receiver.ID())
		if code != c.status {
			t.Fatalf("want %d got %d", c.status, code)
		}
	}
	fresh := mint(t, f.aps, session, "host_11111111", true)["token"].(string)
	for _, path := range []string{"/api/session", "/api/ingest"} {
		method := "GET"
		if path == "/api/ingest" {
			method = "POST"
		}
		code, _ := request(t, f.aps, method, path, old, nil, "")
		if code != 401 {
			t.Fatal("revoked token accepted")
		}
	}
	b := batch(t, f, "tenant_alpha", "ap-south-1", "host_11111111")
	if code, _ := request(t, f.aps, "POST", "/api/ingest", fresh, b, f.ap.receiver.ID()); code != 200 {
		t.Fatal(code)
	}
	_, audit := request(t, f.aps, "GET", "/api/audit", session, nil, "")
	for _, event := range []string{"wrong_region", "token_rotate", "allowlist_violation", "ingest_accept", "ingest_reject"} {
		if !bytes.Contains(audit, []byte(event)) {
			t.Fatal("missing audit", event)
		}
	}
	for _, secret := range []string{old, fresh, session, "secret", "alpha-pass"} {
		if bytes.Contains(audit, []byte(secret)) {
			t.Fatal("audit leaked credential or raw data")
		}
	}
	_, out := request(t, f.aps, "GET", "/api/audit/verify", session, nil, "")
	if object(t, out)["valid"] != true {
		t.Fatal("clean chain failed")
	}
	f.a.mu.Lock()
	f.a.audits["tenant_alpha"].Entries[0].Event = "tampered"
	f.a.mu.Unlock()
	_, out = request(t, f.aps, "GET", "/api/audit/verify", session, nil, "")
	if object(t, out)["valid"] != false {
		t.Fatal("tamper undetected")
	}
	if code, _ := request(t, f.aps, "POST", "/api/auth/logout", session, nil, ""); code != 200 {
		t.Fatal(code)
	}
	if code, _ := request(t, f.aps, "GET", "/api/global", session, nil, ""); code != 401 {
		t.Fatal("logout failed")
	}
}
func TestAuditDeletionAndCredentialStorage(t *testing.T) {
	c := &chain{}
	c.append("login_success", 200)
	c.append("token_mint", 200)
	c.Entries = c.Entries[:1]
	if c.verify() {
		t.Fatal("tail deletion undetected")
	}
	f := setup(t)
	s := login(t, f.aps, "tenant_alpha")
	token := mint(t, f.aps, s, "host_12345678", false)["token"].(string)
	f.a.mu.Lock()
	defer f.a.mu.Unlock()
	if _, ok := f.a.tokens[sha256.Sum256([]byte(token))]; !ok {
		t.Fatal("token not hashed")
	}
	if _, ok := f.a.sessions[sha256.Sum256([]byte(s))]; !ok {
		t.Fatal("session not hashed")
	}
}

func TestExpiredSessionAndFailedLogin(t *testing.T) {
	f := setup(t)
	code, out := request(t, f.aps, "POST", "/api/auth/login", "", []byte(`{"username":"tenant_beta","password":"wrong-secret"}`), "")
	if code != 401 || bytes.Contains(out, []byte("wrong-secret")) {
		t.Fatal("invalid login response")
	}
	s := login(t, f.aps, "tenant_alpha")
	key := sha256.Sum256([]byte(s))
	f.a.mu.Lock()
	expired := f.a.sessions[key]
	expired.Expires = time.Now().Add(-time.Second)
	f.a.sessions[key] = expired
	if !f.a.audits[""].verify() {
		t.Error("system audit invalid")
	}
	f.a.mu.Unlock()
	for _, path := range []string{"/api/records", "/api/global", "/api/audit", "/api/audit/verify", "/api/events", "/api/remediations/propose", "/api/collectors/tokens"} {
		code, _ := request(t, f.aps, "GET", path, s, nil, "")
		if code != 401 {
			t.Fatalf("expired session accepted at %s", path)
		}
	}
}
func TestInstallerDownloadsAndHTTPSOnly(t *testing.T) {
	f := setup(t)
	for _, name := range []string{"collector-linux-amd64", "collector-darwin-arm64", "collector-windows-amd64.exe", "allowlist.yaml"} {
		data := []byte("fixture-" + name)
		if err := os.WriteFile(filepath.Join(f.dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
		code, out := request(t, f.aps, "GET", "/downloads/"+name, "", nil, "")
		if code != 200 || !bytes.Equal(data, out) {
			t.Fatal("download mismatch")
		}
	}
	code, script := request(t, f.aps, "GET", "/install.sh", "", nil, "")
	if code != 200 || !bytes.Contains(script, []byte("--cacert")) || !bytes.Contains(script, []byte("chmod 700")) {
		t.Fatal("installer invalid")
	}
	for _, path := range []string{"/downloads/../ca.key", "/downloads/unknown", "/downloads/", "/install.sh?token=secret"} {
		code, _ := request(t, f.aps, "GET", path, "", nil, "")
		if code < 400 {
			t.Fatal("unsafe download accepted")
		}
	}
	s := login(t, f.aps, "tenant_alpha")
	result := mint(t, f.aps, s, "host_12345678", false)
	token := result["token"].(string)
	command := result["install_command"].(string)
	if !strings.Contains(command, "sh -s --") || !strings.Contains(command, "--token="+token) {
		t.Fatal("flags missing")
	}
	for _, word := range strings.Fields(command) {
		if strings.Contains(word, "https://") && strings.Contains(word, token) {
			t.Fatal("URL credential")
		}
	}
	if bytes.Contains(script, []byte(token)) {
		t.Fatal("installer contains token")
	}
	for _, path := range []string{"/api/ingest", "/api/auth/login", "/install.sh"} {
		r := httptest.NewRequest("GET", "http://localhost"+path, nil)
		w := httptest.NewRecorder()
		f.ap.ServeHTTP(w, r)
		if w.Code != 426 {
			t.Fatal("plaintext path")
		}
	}
}
func TestAuthenticatedWALDelivery(t *testing.T) {
	f := setup(t)
	s := login(t, f.aps, "tenant_alpha")
	token := mint(t, f.aps, s, "host_12345678", false)["token"].(string)
	binding := wal.Binding{Tenant: "tenant_alpha", Region: "ap-south-1", CollectorID: "host_12345678"}
	j, err := wal.Open(filepath.Join(t.TempDir(), "collector.wal"), binding, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	d, err := send.NewDelivery(j, f.gate, binding, f.aps.URL, f.aps.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	d.Token = token
	var b telemetry.Batch
	_ = json.Unmarshal(batch(t, f, binding.Tenant, binding.Region, binding.CollectorID), &b)
	raw, _ := json.Marshal(b.Records[0])
	p, err := f.gate.Approve(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.Enqueue(p); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for j.Depth() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if j.Depth() != 0 || len(f.ap.receiver.Snapshot(binding.Tenant, binding.CollectorID).Records) != 1 {
		t.Fatal("authenticated WAL delivery failed")
	}
}
