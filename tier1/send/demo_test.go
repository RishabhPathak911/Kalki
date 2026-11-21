package send

import (
	"context"
	"encoding/json"
	"kalki/tier2/ingest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDemoOriginOptInOutageAndDuplicateLateWAL(t *testing.T) {
	gate := testPolicy(t)
	receiver, err := ingest.New(gate, binding.Region, 20)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(receiver)
	defer server.Close()
	d := testDelivery(t, gate, server.URL, server.Client())
	call := func(action, body, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://127.0.0.1:9090/api/demo/"+action, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		d.ProofHandler().ServeHTTP(w, r)
		return w
	}
	if call("outage", `{"enabled":true}`, "http://127.0.0.1:9090").Code != 404 {
		t.Fatal("controls not opt-in")
	}
	d.Demo = func(context.Context, string) (map[string]any, error) {
		return map[string]any{"status": "finished"}, nil
	}
	if call("outage", `{"enabled":true}`, "http://foreign.invalid").Code != 403 {
		t.Fatal("foreign origin accepted")
	}
	if call("outage", `{"enabled":true}`, "http://127.0.0.1:9090").Code != 200 {
		t.Fatal("pause failed")
	}
	if err := d.Enqueue(record(t, gate)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(150 * time.Millisecond)
	if len(receiver.Snapshot(binding.Tenant, binding.CollectorID).Records) != 0 {
		t.Fatal("paused delivery leaked")
	}
	if call("outage", `{"enabled":false}`, "http://127.0.0.1:9090").Code != 200 {
		t.Fatal("restore failed")
	}
	deadline := time.Now().Add(5 * time.Second)
	for d.journal.Depth() > 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if d.journal.Depth() != 0 {
		t.Fatal("WAL did not drain")
	}
	response := call("dup-ooo", `{}`, "http://127.0.0.1:9090")
	if response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	var status map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &status)
	if status["status"] != "running" {
		t.Fatal(status)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap := receiver.Snapshot(binding.Tenant, binding.CollectorID)
		if len(snap.Records) == 2 && snap.DuplicatesRejected > 0 {
			if snap.Records[0].Record.CollectedAt >= snap.Records[1].Record.CollectedAt || snap.Records[0].Record.Source != "simulated" {
				t.Fatal("late record not ordered/labeled")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("duplicate/late delivery missing")
}
