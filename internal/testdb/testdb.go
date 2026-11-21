// Package testdb creates an isolated, disposable schema for opt-in DB tests.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"testing"
	"time"
)

func Schema(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("KALKI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KALKI_TEST_DATABASE_URL for isolated PostgreSQL integration tests")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid test database URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("test database connection failed")
	}
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	name := "kalki_test_" + hex.EncodeToString(random[:])
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = conn.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		conn.Close(ctx)
		t.Fatal("test schema creation failed")
	}
	conn.Close(ctx)
	t.Cleanup(func() {
		ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		conn, e := pgx.Connect(ctx, dsn)
		if e != nil {
			t.Error("test schema cleanup connection failed")
			return
		}
		defer conn.Close(ctx)
		if _, e = conn.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); e != nil {
			t.Error("test schema cleanup failed")
		}
	})
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	return u.String()
}
