package store

import (
	"context"
	"kalki/internal/testdb"
	"testing"
	"time"
)

func TestOptionalAndSanitized(t *testing.T) {
	s, e := Open(context.Background(), "")
	if e != nil || s != nil {
		t.Fatal("offline mode must not open DB")
	}
	_, e = Open(context.Background(), "postgres://bad:%zz@invalid/")
	if e != ErrUnavailable {
		t.Fatal("connection error must be sanitized")
	}
	s = &Store{slots: make(chan struct{}, 256)}
	for i := 0; i < 256; i++ {
		if !s.Reserve() {
			t.Fatal("premature queue exhaustion")
		}
	}
	if s.Reserve() {
		t.Fatal("unbounded queue")
	}
	s.Release()
	if !s.Reserve() {
		t.Fatal("capacity not returned")
	}
}

func TestPostgresPoolRotationAndDurability(t *testing.T) {
	dsn := testdb.Schema(t)
	s, e := Open(context.Background(), dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close(context.Background())
	cfg := s.pool.Config()
	if cfg.MaxConns != 5 || cfg.MinConns != 1 || cfg.MaxConnIdleTime != 5*time.Minute || cfg.MaxConnLifetime != 30*time.Minute {
		t.Fatal("wrong pool limits")
	}
	if e = s.SaveToken(Token{"old-hash", "tenant_alpha", "ap-south-1", "host_12345678"}); e != nil {
		t.Fatal(e)
	}
	if e = s.SaveToken(Token{"new-hash", "tenant_alpha", "ap-south-1", "host_12345678"}); e != nil {
		t.Fatal(e)
	}
	if e = s.SaveToken(Token{"foreign-hash", "tenant_beta", "ap-south-1", "host_12345678"}); e == nil {
		t.Fatal("cross-tenant rotation accepted")
	}
	if e = s.SaveSession(Session{"session-hash", "tenant_alpha", "ap-south-1", time.Now().Add(time.Hour)}); e != nil {
		t.Fatal(e)
	}
	data, e := s.Load(20)
	if e != nil {
		t.Fatal(e)
	}
	if len(data.Tokens) != 1 || data.Tokens[0].Hash != "new-hash" || len(data.Sessions) != 1 {
		t.Fatal("durable auth mismatch")
	}
	if e = s.DeleteSession("session-hash"); e != nil {
		t.Fatal(e)
	}
	data, e = s.Load(20)
	if e != nil || len(data.Sessions) != 0 {
		t.Fatal("logout not durable")
	}
	// Exercise the real retention query against more than the configured ceiling.
	_, e = s.pool.Exec(context.Background(), `INSERT INTO telemetry_records SELECT 'tenant_alpha','ap-south-1','host_12345678',n,'batch','hash','2026-01-01T00:00:00.000Z',to_char(timestamp '2026-01-01' + n * interval '1 second','YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),'{}'::jsonb FROM generate_series(1,4100) n`)
	if e != nil {
		t.Fatal("retention fixture failed")
	}
	s.prune()
	var count int
	var minSeq int64
	if e = s.pool.QueryRow(context.Background(), `SELECT count(*),min(seq) FROM telemetry_records`).Scan(&count, &minSeq); e != nil || count != 4096 || minSeq != 5 {
		t.Fatal("telemetry retention did not preserve newest 4096")
	}
}
