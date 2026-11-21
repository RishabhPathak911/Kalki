package ingest

import (
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"testing"
	"time"
)

func TestLongRunningCheckpointAndPrunedDuplicates(t *testing.T) {
	s, err := New(gateForTest(t), "ap-south-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	b := telemetry.Batch{Tenant: "tenant_alpha", Region: "ap-south-1", CollectorID: "host_12345678", Records: []telemetry.Record{{Tenant: "tenant_alpha", Region: "ap-south-1", CollectorID: "host_12345678", CollectedAt: collect.Timestamp(now)}}}
	for seq := uint64(1); seq <= 100005; seq++ {
		b.Seq = seq
		if _, code := s.accept(b, "test-hash", now); code != 200 {
			t.Fatal("long running collector blocked", seq, code)
		}
	}
	cp := s.Checkpoint(b.Tenant, b.CollectorID)
	if len(cp.Seen) != 4096 || cp.Contiguous != 100005 {
		t.Fatal("unbounded or lost sequence history")
	}
	recovered, _ := New(gateForTest(t), "ap-south-1", 2)
	recovered.Restore(cp, s.Snapshot(b.Tenant, b.CollectorID).Records)
	b.Seq = 1
	if ack, code := recovered.accept(b, "test-hash", now); code != 200 || !ack.Duplicate {
		t.Fatal("pruned duplicate not recognized")
	}
	b.Seq = 100006
	if ack, code := recovered.accept(b, "test-hash", now); code != 200 || ack.HighestContiguous != 100006 {
		t.Fatal("post-restart sequence did not advance")
	}
	if len(recovered.Snapshot("tenant_beta", b.CollectorID).Records) != 0 {
		t.Fatal("restored tenant leak")
	}
}
