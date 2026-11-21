package send

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
	"kalki/tier1/wal"
	"kalki/tier2/ingest"
)

var binding = wal.Binding{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1"}

func testPolicy(t *testing.T) *policy.Gate {
	t.Helper()
	g, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func record(t *testing.T, g *policy.Gate) policy.Approved {
	t.Helper()
	r := telemetry.Record{Tenant: binding.Tenant, CollectorID: binding.CollectorID, Region: binding.Region, CollectedAt: collect.Timestamp(time.Now()), Source: "live", Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 42, Max: 42, Avg: 42, P95: 42, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	p, err := g.Approve(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func testDelivery(t *testing.T, g *policy.Gate, base string, client *http.Client) *Delivery {
	t.Helper()
	j, err := wal.Open(filepath.Join(t.TempDir(), "collector.wal"), binding, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := j.Close(); err != nil {
			t.Error(err)
		}
	})
	d, err := NewDelivery(j, g, binding, base, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func startDelivery(t *testing.T, d *Delivery) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(stop)
	return stop
}
func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition did not become true before deadline")
}

func TestLostAckRetriesIdenticalBytesWithoutDuplicates(t *testing.T) {
	g := testPolicy(t)
	receiver, err := ingest.New(g, binding.Region, 100)
	if err != nil {
		t.Fatal(err)
	}
	var posts atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ingest" && posts.Add(1) == 1 {
			saved := httptest.NewRecorder()
			receiver.ServeHTTP(saved, r)
			w.WriteHeader(503)
			return
		}
		receiver.ServeHTTP(w, r)
	}))
	defer server.Close()
	d := testDelivery(t, g, server.URL, server.Client())
	if err := d.Enqueue(record(t, g)); err != nil {
		t.Fatal(err)
	}
	stop := startDelivery(t, d)
	waitFor(t, 5*time.Second, func() bool { return d.journal.Depth() == 0 })
	stop()
	v := receiver.Snapshot(binding.Tenant, binding.CollectorID)
	if v.RecordsAccepted != 1 || len(v.Records) != 1 || v.DuplicatesRejected != 1 {
		t.Fatalf("lost ACK duplicated data: %+v", v)
	}
	proof := d.proofs.Snapshot()
	if len(proof) != 1 || proof[0].Status != "ACKED" || !proof[0].Match || d.Counters().Retried < 1 {
		t.Fatal("missing matched proof/retry counters")
	}
}
func TestMismatchedEchoRetainsWAL(t *testing.T) {
	g := testPolicy(t)
	receiver, err := ingest.New(g, binding.Region, 100)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saved := httptest.NewRecorder()
		receiver.ServeHTTP(saved, r)
		if r.URL.Path == "/api/ingest" && saved.Code == 200 {
			var ack telemetry.Ack
			if err := json.Unmarshal(saved.Body.Bytes(), &ack); err != nil {
				t.Error(err)
			}
			ack.PayloadSHA256 = strings.Repeat("0", 64)
			if err := json.NewEncoder(w).Encode(ack); err != nil {
				t.Error(err)
			}
			return
		}
		w.WriteHeader(saved.Code)
		if _, err := w.Write(saved.Body.Bytes()); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	d := testDelivery(t, g, server.URL, server.Client())
	if err := d.Enqueue(record(t, g)); err != nil {
		t.Fatal(err)
	}
	if err := d.journal.SetEpoch(receiver.ID()); err != nil {
		t.Fatal(err)
	}
	e, _ := d.journal.Next()
	if err := d.deliver(context.Background(), e, receiver.ID()); err == nil || d.journal.Depth() != 1 {
		t.Fatal("mismatched echo accepted")
	}
	proof := d.proofs.Snapshot()
	if proof[0].Match || proof[0].Status == "ACKED" {
		t.Fatal("false green proof")
	}
}

func TestProofPublicationCannotRegressAcknowledgement(t *testing.T) {
	g := testPolicy(t)
	d := testDelivery(t, g, "https://127.0.0.1:8443", nil)
	if err := d.Enqueue(record(t, g)); err != nil {
		t.Fatal(err)
	}
	e, _ := d.journal.Next()
	d.proofs.Update(e, "ACKED", e.Hash)
	d.proofs.Update(e, "BUFFERED", "")
	if p := d.proofs.Snapshot()[0]; p.Status != "ACKED" || !p.Match {
		t.Fatal("late enqueue publication regressed ACK")
	}
	d.proofs.Reset()
	d.proofs.Update(e, "BUFFERED", "")
	if p := d.proofs.Snapshot()[0]; p.Status != "BUFFERED" || p.Match {
		t.Fatal("new receiver session retained stale match")
	}
}
func TestIdleHeartbeatAndReadOnlyViolation(t *testing.T) {
	g := testPolicy(t)
	receiver, err := ingest.New(g, binding.Region, 100)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(receiver)
	defer server.Close()
	d := testDelivery(t, g, server.URL, server.Client())
	stop := startDelivery(t, d)
	waitFor(t, 7*time.Second, func() bool { return d.Counters().Acked >= 1 })
	stop()
	v := receiver.Snapshot(binding.Tenant, binding.CollectorID)
	if v.LastSeen == "" || v.RecordsAccepted != 0 {
		t.Fatal("heartbeat fabricated records")
	}
	seq, depth := d.journal.LastSeq(), d.journal.Depth()
	r := httptest.NewRequest("GET", "http://127.0.0.1:9090/api/attempt-violation", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	d.ProofHandler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"blocked":true`) || d.journal.LastSeq() != seq || d.journal.Depth() != depth {
		t.Fatal("violation probe changed delivery state")
	}
	p := d.proofs.Snapshot()
	if p[len(p)-1].Status != "BLOCKED" || p[len(p)-1].Hash == "" {
		t.Fatal("blocked proof missing")
	}
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	d.ProofHandler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("cross-origin proof access accepted")
	}
	r.Header.Del("Origin")
	r.Host = "evil.example"
	w = httptest.NewRecorder()
	d.ProofHandler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("DNS rebinding host accepted")
	}
	if listener, _, err := ListenProof("0.0.0.0:0", d); err == nil {
		listener.Close()
		t.Fatal("public proof binding accepted")
	}
}

func TestReceiverProcessHelper(t *testing.T) {
	addr := os.Getenv("KALKI_TEST_RECEIVER_ADDR")
	if addr == "" {
		t.Skip("subprocess helper")
	}
	g := testPolicy(t)
	s, err := ingest.New(g, binding.Region, 100)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 3 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	if err := server.ListenAndServeTLS(os.Getenv("KALKI_TEST_RECEIVER_CERT"), os.Getenv("KALKI_TEST_RECEIVER_KEY")); err != nil {
		t.Fatal(err)
	}
}

func TestThirtySecondOutageAndReceiverProcessRestart(t *testing.T) {
	if testing.Short() {
		t.Skip("real 30-second outage test")
	}
	g := testPolicy(t)
	seed := httptest.NewTLSServer(http.NotFoundHandler())
	client := seed.Client()
	base := seed.URL
	addr := seed.Listener.Addr().String()
	cert := seed.TLS.Certificates[0]
	private, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "server.pem"), filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600); err != nil {
		t.Fatal(err)
	}
	seed.Close()
	var child *exec.Cmd
	stopReceiver := func() {
		if child == nil {
			return
		}
		killErr := child.Process.Kill()
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			t.Error(killErr)
		}
		err := child.Wait()
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Error(err)
		}
		child = nil
	}
	t.Cleanup(stopReceiver)
	startReceiver := func() {
		child = exec.Command(os.Args[0], "-test.run=^TestReceiverProcessHelper$")
		child.Env = append(os.Environ(), "KALKI_TEST_RECEIVER_ADDR="+addr, "KALKI_TEST_RECEIVER_CERT="+certPath, "KALKI_TEST_RECEIVER_KEY="+keyPath)
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		waitFor(t, 6*time.Second, func() bool {
			resp, err := client.Get(base + "/api/session")
			if err != nil {
				return false
			}
			closeErr := resp.Body.Close()
			return closeErr == nil && resp.StatusCode == 200
		})
	}
	startReceiver()
	d := testDelivery(t, g, base, client)
	for i := 0; i < 3; i++ {
		if err := d.Enqueue(record(t, g)); err != nil {
			t.Fatal(err)
		}
	}
	stop := startDelivery(t, d)
	waitFor(t, 5*time.Second, func() bool { return d.journal.Depth() == 0 })
	stopReceiver()
	t.Log("receiver process killed; buffering across a real 30-second outage")
	for i := 0; i < 4; i++ {
		if err := d.Enqueue(record(t, g)); err != nil {
			t.Fatal(err)
		}
	}
	timer := time.NewTimer(30 * time.Second)
	<-timer.C
	if d.journal.Depth() < 4 {
		t.Fatal("outage data was lost from WAL")
	}
	startReceiver()
	t.Log("new receiver process started; replaying acknowledged and pending batches")
	var snapshot ingest.Snapshot
	waitFor(t, 20*time.Second, func() bool {
		resp, err := client.Get(base + "/api/records?tenant=" + binding.Tenant + "&collector_id=" + binding.CollectorID)
		if err != nil {
			return false
		}
		err = json.NewDecoder(resp.Body).Decode(&snapshot)
		closeErr := resp.Body.Close()
		return err == nil && closeErr == nil && snapshot.RecordsAccepted == 7 && d.journal.Depth() == 0
	})
	stop()
	seen := map[uint64]bool{}
	for _, r := range snapshot.Records {
		if seen[r.Seq] {
			t.Fatal("duplicate record after restart")
		}
		seen[r.Seq] = true
	}
	if len(seen) != 7 || snapshot.RecordsAccepted != 7 || snapshot.RecordsEvicted != 0 {
		t.Fatal("lost records after restart")
	}
	t.Log(fmt.Sprintf("PASS: %d unique records, zero lost/duplicated, retries=%d", len(seen), d.Counters().Retried))
}
