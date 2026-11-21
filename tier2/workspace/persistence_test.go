package workspace

import (
	"context"
	"github.com/jackc/pgx/v5"
	"kalki/internal/testdb"
	"kalki/tier2/store"
	"testing"
)

func TestPostgresWorkspaceRestart(t *testing.T) {
	dsn := testdb.Schema(t)
	open := func() *store.Store {
		s, e := store.Open(context.Background(), dsn)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	configure := func(f *fixture, s *store.Store) {
		t.Helper()
		if e := f.a.ConfigureMedha("../../config/thresholds.yaml", "../../config/playbooks.yaml"); e != nil {
			t.Fatal(e)
		}
		if e := f.a.AttachStore(s, 20); e != nil {
			t.Fatal(e)
		}
	}
	db := open()
	f := setup(t)
	configure(f, db)
	alpha := login(t, f.aps, "tenant_alpha")
	beta := login(t, f.eus, "tenant_beta")
	old := mint(t, f.aps, alpha, "host_12345678", false)["token"].(string)
	token := mint(t, f.aps, alpha, "host_12345678", true)["token"].(string)
	payload := batch(t, f, "tenant_alpha", "ap-south-1", "host_12345678")
	if code, _ := request(t, f.aps, "POST", "/api/ingest", token, payload, f.ap.receiver.ID()); code != 200 {
		t.Fatal("ingest", code)
	}
	item := Remediation{ID: "remediation-test", Collector: "host_12345678", Status: "APPROVED", Reversible: true, TargetValid: true, Authorized: true, Action: "Simulate", Result: "Dry run", At: "2026-01-01T00:00:00Z"}
	if e := f.a.saveRemediation("tenant_alpha", item); e != nil {
		t.Fatal(e)
	}
	if e := db.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	db = open()
	defer db.Close(context.Background())
	restored := setup(t)
	configure(restored, db)
	if code, out := request(t, restored.aps, "GET", "/api/audit/verify", alpha, nil, ""); code != 200 || object(t, out)["valid"] != true {
		t.Fatal("audit chain/session not restored")
	}
	for _, tc := range []struct {
		token string
		count int
	}{{alpha, 1}, {beta, 0}} {
		code, out := request(t, restored.aps, "GET", "/api/records?collector_id=host_12345678", tc.token, nil, "")
		if code != 200 || len(object(t, out)["records"].([]any)) != tc.count {
			t.Fatal("restored records or isolation failed")
		}
	}
	if code, _ := request(t, restored.aps, "GET", "/api/session", old, nil, ""); code != 401 {
		t.Fatal("revoked token revived")
	}
	if code, out := request(t, restored.aps, "POST", "/api/ingest", token, payload, restored.ap.receiver.ID()); code != 200 || object(t, out)["duplicate"] != true {
		t.Fatal("dedup progress not restored")
	}
	if len(restored.a.remediations["tenant_alpha"]) != 1 {
		t.Fatal("remediation not restored")
	}
	if code, out := request(t, restored.aps, "GET", "/api/fleet", alpha, nil, ""); code != 200 || len(object(t, out)["nodes"].([]any)) != 1 {
		t.Fatal("fleet not restored")
	}
	if e := db.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	if code, _ := request(t, restored.aps, "GET", "/api/audit/verify", alpha, nil, ""); code != 503 {
		t.Fatal("DB failure did not fail closed")
	}
	conn, e := pgx.Connect(context.Background(), dsn)
	if e != nil {
		t.Fatal("test connection failed")
	}
	defer conn.Close(context.Background())
	if _, e = conn.Exec(context.Background(), "UPDATE audit_entries SET event='tampered' WHERE tenant='tenant_alpha' AND entry_index=1"); e != nil {
		t.Fatal("tamper setup failed")
	}
	db2 := open()
	defer db2.Close(context.Background())
	broken := setup(t)
	if e := broken.a.AttachStore(db2, 20); e == nil {
		t.Fatal("tampered persisted audit accepted")
	}
}
