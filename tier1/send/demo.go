package send

import (
	"encoding/json"
	"fmt"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"net/http"
	"strings"
	"time"
)

func (d *Delivery) demoRequest(w http.ResponseWriter, r *http.Request) {
	if d.Demo == nil {
		http.Error(w, "demo controls disabled", 404)
		return
	}
	if r.Method != "POST" || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		w.WriteHeader(405)
		return
	}
	action := strings.TrimPrefix(r.URL.Path, "/api/demo/")
	result := map[string]any{"status": "finished", "action": action}
	switch action {
	case "outage":
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || body.Enabled == nil {
			w.WriteHeader(400)
			return
		}
		d.mu.Lock()
		d.paused = *body.Enabled
		d.mu.Unlock()
		result["detail"] = "Delivery restored; retained WAL will drain"
		if *body.Enabled {
			result["status"] = "running"
			result["detail"] = "Delivery paused; collection and WAL remain active"
		}
	case "dup-ooo":
		if err := d.queueLateDemo(); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		result["status"] = "running"
		result["detail"] = "Queued one exact duplicate and one policy-approved late SIMULATED record"
	default:
		var err error
		result, err = d.Demo(r.Context(), action)
		if err != nil {
			http.Error(w, "demo action unavailable", 400)
			return
		}
	}
	d.note("DEMO_CONTROL action=" + action + " status=" + fmt.Sprint(result["status"]))
	_ = json.NewEncoder(w).Encode(result)
}
func (d *Delivery) queueLateDemo() error {
	entries := d.journal.Entries()
	var chosen *telemetry.Record
	for i := len(entries) - 1; i >= 0; i-- {
		if !d.journal.IsAcked(entries[i].Seq) {
			continue
		}
		var b telemetry.Batch
		if json.Unmarshal(entries[i].Bytes, &b) != nil {
			continue
		}
		if len(b.Records) > 0 {
			r := b.Records[0]
			chosen = &r
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("wait for an acknowledged metric batch")
	}
	at, err := time.Parse(collect.TimeLayout, chosen.CollectedAt)
	if err != nil {
		return err
	}
	chosen.CollectedAt = collect.Timestamp(at.Add(-90 * time.Second))
	chosen.Source = collect.Simulated
	chosen.RiskEvents = []telemetry.Risk{}
	chosen.RedactionCounts = map[string]int{}
	raw, err := json.Marshal(chosen)
	if err != nil {
		return err
	}
	p, err := d.gate.Approve(raw)
	if err != nil {
		return err
	}
	if err = d.Enqueue(p); err != nil {
		return err
	}
	d.mu.Lock()
	d.duplicatePending = true
	d.mu.Unlock()
	return nil
}
