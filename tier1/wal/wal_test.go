package wal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
)

var testBinding = Binding{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1"}

func builder(t *testing.T) func(uint64) (policy.Approved, error) {
	t.Helper()
	g, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return func(seq uint64) (policy.Approved, error) {
		data, err := json.Marshal(telemetry.Batch{Tenant: testBinding.Tenant, CollectorID: testBinding.CollectorID, Region: testBinding.Region, Seq: seq, BatchID: fmt.Sprintf("%032x", seq), SentAt: collect.Timestamp(time.Now()), Records: []telemetry.Record{}})
		if err != nil {
			return policy.Approved{}, err
		}
		return g.ApproveBatch(data)
	}
}
func openJournal(t *testing.T, path string, cap int64) *Journal {
	t.Helper()
	j, err := Open(path, testBinding, cap)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := j.Close(); err != nil {
			t.Error(err)
		}
	})
	return j
}

func TestPersistedSequenceAckAndEpochReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.wal")
	j := openJournal(t, path, 1<<20)
	makeBatch := builder(t)
	a, err := j.Enqueue(makeBatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.SetEpoch("server-one"); err != nil {
		t.Fatal(err)
	}
	if err := j.Ack(a.Seq, "server-one", "wrong-hash"); err == nil {
		t.Fatal("bad ACK removed batch")
	}
	if err := j.Ack(a.Seq, "server-one", a.Hash); err != nil {
		t.Fatal(err)
	}
	b, err := j.Enqueue(makeBatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j = openJournal(t, path, 1<<20)
	next, ok := j.Next()
	if !ok || next.Seq != 2 || string(next.Bytes) != string(b.Bytes) || j.Depth() != 1 {
		t.Fatal("restart lost pending bytes or sequence")
	}
	c, err := j.Enqueue(makeBatch)
	if err != nil || c.Seq != 3 {
		t.Fatal("sequence not monotonic after restart")
	}
	if err := j.SetEpoch("server-two"); err != nil {
		t.Fatal(err)
	}
	next, ok = j.Next()
	if !ok || next.Seq != 1 || string(next.Bytes) != string(a.Bytes) || j.Depth() != 3 {
		t.Fatal("receiver restart cannot replay ACKed history")
	}
}
func TestTailRepairAndCorruptionFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.wal")
	j := openJournal(t, path, 1<<20)
	if _, err := j.Enqueue(builder(t)); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte{0, 0, 0, 100, 1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	j = openJournal(t, path, 1<<20)
	if !j.RepairedTail || j.LastSeq() != 1 {
		t.Fatal("incomplete tail not repaired")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if bad, err := Open(path, testBinding, 1<<20); err == nil {
		bad.Close()
		t.Fatal("corrupt committed frame accepted")
	}
}
func TestCapacityBindingAndExclusiveOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.wal")
	j := openJournal(t, path, 4096)
	if second, err := Open(path, testBinding, 4096); err == nil {
		second.Close()
		t.Fatal("two writers accepted")
	}
	build := builder(t)
	for i := 0; i < 100; i++ {
		last := j.LastSeq()
		_, err := j.Enqueue(build)
		if errors.Is(err, ErrFull) {
			if j.LastSeq() != last {
				t.Fatal("full WAL consumed sequence")
			}
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if i == 99 {
			t.Fatal("capacity not enforced")
		}
	}
	if err := j.SetEpoch("server"); err != nil {
		t.Fatal("ACK headroom absent", err)
	}
	e, _ := j.Next()
	if err := j.Ack(e.Seq, "server", e.Hash); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	other := testBinding
	other.Tenant = "tenant_beta"
	if wrong, err := Open(path, other, 4096); err == nil {
		wrong.Close()
		t.Fatal("rebound WAL to another tenant")
	}
}
func TestWALCrashHelper(t *testing.T) {
	path := os.Getenv("KALKI_TEST_CRASH_WAL")
	if path == "" {
		t.Skip("subprocess helper")
	}
	j, err := Open(path, testBinding, 1<<20)
	if err != nil {
		os.Exit(21)
	}
	if _, err := j.Enqueue(builder(t)); err != nil {
		os.Exit(22)
	}
	os.Exit(23) // Deliberately bypass Close and every defer after durable append.
}
func TestDurableAfterAbruptProcessExit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collector.wal")
	cmd := exec.Command(os.Args[0], "-test.run=^TestWALCrashHelper$")
	cmd.Env = append(os.Environ(), "KALKI_TEST_CRASH_WAL="+path)
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("helper did not reach crash point: %v", err)
	}
	j := openJournal(t, path, 1<<20)
	if j.LastSeq() != 1 || j.Depth() != 1 {
		t.Fatal("fsynced batch lost across process exit")
	}
}
