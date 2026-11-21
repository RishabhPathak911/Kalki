package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
)

func gateForTest(t *testing.T) *policy.Gate {
	t.Helper()
	g, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func batchForTest(t *testing.T, g *policy.Gate, seq uint64, at time.Time) policy.Approved {
	t.Helper()
	r := telemetry.Record{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1", Source: "live", CollectedAt: collect.Timestamp(at), Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 1, Max: 1, Avg: 1, P95: 1, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	data, err := json.Marshal(telemetry.Batch{Tenant: r.Tenant, CollectorID: r.CollectorID, Region: r.Region, Seq: seq, BatchID: fmt.Sprintf("%032x", seq), SentAt: collect.Timestamp(time.Now()), Records: []telemetry.Record{r}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.ApproveBatch(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestIdempotentOutOfOrderAndLateness(t *testing.T) {
	g := gateForTest(t)
	s, err := New(g, "ap-south-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	call := func(p policy.Approved) (telemetry.Ack, int) {
		var b telemetry.Batch
		if err := json.Unmarshal(p.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		return s.accept(b, p.SHA256(), now)
	}
	p3 := batchForTest(t, g, 3, now)
	a, code := call(p3)
	if code != 200 || a.HighestContiguous != 0 || len(a.Gaps) != 2 {
		t.Fatal("seq 3 gaps incorrect")
	}
	p1 := batchForTest(t, g, 1, now.Add(-time.Minute))
	a, code = call(p1)
	if code != 200 || a.HighestContiguous != 1 || len(a.Gaps) != 1 || a.Gaps[0] != 2 {
		t.Fatal("seq 1 ACK incorrect")
	}
	p2 := batchForTest(t, g, 2, now.Add(-30*time.Second))
	a, code = call(p2)
	if code != 200 || a.HighestContiguous != 3 || len(a.Gaps) != 0 {
		t.Fatal("gap did not close")
	}
	a, code = call(p2)
	if code != 200 || !a.Duplicate || a.DuplicatesRejected != 1 || a.PayloadSHA256 != p2.SHA256() {
		t.Fatal("duplicate not identified")
	}
	view := s.Snapshot("tenant_alpha", "host_12345678")
	if len(view.Records) != 3 || view.Records[0].Seq != 1 || view.Records[2].Seq != 3 {
		t.Fatal("duplicate stored or records not ordered")
	}
	if _, code = call(batchForTest(t, g, 4, now.Add(-10*time.Minute-time.Second))); code != 422 {
		t.Fatal("too-late record accepted")
	}
	if _, code = call(batchForTest(t, g, 4, now.Add(-10*time.Minute))); code != 200 {
		t.Fatal("10-minute boundary rejected")
	}
	if _, code = call(batchForTest(t, g, 2, now.Add(time.Second))); code != 409 {
		t.Fatal("conflicting duplicate accepted")
	}
	if len(s.Snapshot("tenant_beta", "host_12345678").Records) != 0 {
		t.Fatal("tenant-scoped read leaked data")
	}
}
func TestRingEvictionAndSessionGuard(t *testing.T) {
	g := gateForTest(t)
	s, err := New(g, "ap-south-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	for seq := uint64(1); seq <= 3; seq++ {
		p := batchForTest(t, g, seq, time.Now())
		var b telemetry.Batch
		if err := json.Unmarshal(p.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if _, code := s.accept(b, p.SHA256(), time.Now()); code != 200 {
			t.Fatal(code)
		}
	}
	v := s.Snapshot("tenant_alpha", "host_12345678")
	if len(v.Records) != 2 || v.RecordsAccepted != 3 || v.RecordsEvicted != 1 {
		t.Fatal("bounded retention counters wrong")
	}
	r := httptest.NewRequest("POST", "https://localhost/api/ingest", bytes.NewReader(batchForTest(t, g, 4, time.Now()).Bytes()))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 409 || s.Snapshot("tenant_alpha", "host_12345678").HighestContiguous != 3 {
		t.Fatal("stale session mutated receiver")
	}
}
