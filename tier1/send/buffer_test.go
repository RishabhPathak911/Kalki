package send

import (
	"encoding/json"
	"errors"
	"kalki/internal/telemetry"
	"kalki/tier1/wal"
	"kalki/tier2/ingest"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestBufferFullStatusIsPolicyApprovedAndDurable(t *testing.T) {
	gate := testPolicy(t)
	receiver, err := ingest.New(gate, binding.Region, 20)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(receiver)
	defer server.Close()
	path := filepath.Join(t.TempDir(), "full.wal")
	j, err := wal.Open(path, binding, 8192)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDelivery(j, gate, binding, server.URL, server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	full := false
	for i := 0; i < 100; i++ {
		if err := d.Enqueue(record(t, gate)); err != nil {
			if !errors.Is(err, wal.ErrFull) {
				t.Fatal(err)
			}
			full = true
			break
		}
	}
	if !full {
		t.Fatal("WAL never filled")
	}
	entries := j.Entries()
	last := entries[len(entries)-1]
	var b telemetry.Batch
	if json.Unmarshal(last.Bytes, &b) != nil || b.Records[0].Metrics["wal_buffer_full"].Avg != 1 {
		t.Fatal("emergency status missing")
	}
	if p, err := gate.ApproveBatch(last.Bytes); err != nil || p.SHA256() != last.Hash {
		t.Fatal("unapproved status")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	recovered, err := wal.Open(path, binding, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	if recovered.LastSeq() != last.Seq {
		t.Fatal("status not persisted")
	}
}
