package policy

import (
	"encoding/json"
	"testing"

	"kalki/internal/telemetry"
)

func TestBatchHeartbeatAndNestedPolicy(t *testing.T) {
	g := testGate(t, nil)
	b := telemetry.Batch{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1", Seq: 1, BatchID: "0123456789abcdef0123456789abcdef", SentAt: "2026-11-21T00:00:00.000Z", Records: []telemetry.Record{}}
	p, err := g.ApproveBatch(encode(t, b))
	if err != nil || len(p.Bytes()) == 0 {
		t.Fatal("valid high-entropy hex batch ID rejected", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(encode(t, b), &obj); err != nil {
		t.Fatal(err)
	}
	obj["extra"] = "secret"
	obj["records"] = []any{validPayload()}
	p, err = g.ApproveBatch(encode(t, obj))
	if err != nil || p.StrippedCount() != 1 {
		t.Fatal("batch stripping failed", err)
	}
	obj["records"].([]any)[0].(map[string]any)["tenant"] = "tenant_beta"
	if _, err := g.ApproveBatch(encode(t, obj)); err == nil {
		t.Fatal("cross-tenant record accepted in envelope")
	}
}
