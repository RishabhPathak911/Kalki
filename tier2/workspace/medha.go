package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"kalki/tier1/collect"
	"kalki/tier2/medha"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ConfigureMedha is called once before serving either regional listener.
func (a *Authority) ConfigureMedha(thresholds, playbooks string) error {
	c, b, err := medha.Load(thresholds, playbooks)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.engine = medha.New(c, b)
	a.audit("", "medha_config_loaded", 200)
	return nil
}
func (a *Authority) RunHealth(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.mu.Lock()
			engine := a.engine
			a.mu.Unlock()
			if engine != nil {
				engine.Tick(now)
			}
		}
	}
}

type Remediation struct {
	ID          string `json:"id"`
	Incident    string `json:"incident_id"`
	Collector   string `json:"collector_id"`
	Status      string `json:"status"`
	Reversible  bool   `json:"reversible"`
	TargetValid bool   `json:"target_valid"`
	Authorized  bool   `json:"tenant_authorized"`
	Action      string `json:"action"`
	Result      string `json:"result"`
	At          string `json:"updated_at"`
}

func (g *Gateway) remediate(w http.ResponseWriter, r *http.Request, id identity) {
	a := g.authority
	if a.engine == nil {
		failure(w, 404)
		return
	}
	if r.URL.Path == "/api/remediations" && r.Method == "GET" {
		a.audit(id.Tenant, "dashboard_remediations", 200)
		respond(w, 200, map[string]any{"remediations": append([]Remediation{}, a.remediations[id.Tenant]...)})
		return
	}
	if r.Method != "POST" {
		failure(w, 405)
		return
	}
	if r.URL.Path == "/api/remediations/propose" {
		var req struct {
			Incident string `json:"incident_id"`
		}
		if !decode(w, r, &req) {
			failure(w, 400)
			return
		}
		var found *medha.Explanation
		for _, incident := range a.engine.Incidents(id.Tenant) {
			if incident.ID == req.Incident && incident.Status == "ACTIVE" {
				copy := incident
				found = &copy
				break
			}
		}
		if found == nil {
			a.audit(id.Tenant, "remediation_policy_reject", 404)
			failure(w, 404)
			return
		}
		home := a.regions[id.Region]
		target := home != nil && found.Tenant == id.Tenant && found.Region == id.Region && home.receiver.Snapshot(id.Tenant, found.Collector).LastSeen != ""
		if !target {
			a.audit(id.Tenant, "remediation_policy_reject", 403)
			failure(w, 403)
			return
		}
		for _, old := range a.remediations[id.Tenant] {
			if old.Incident == req.Incident && old.Status != "ROLLED_BACK" {
				failure(w, 409)
				return
			}
		}
		if len(a.remediations[id.Tenant]) >= 256 {
			failure(w, 429)
			return
		}
		random, err := opaque()
		if err != nil {
			failure(w, 500)
			return
		}
		// The only permitted action is an in-memory simulated diagnostic plan.
		// No OS-command interface or real execution path exists.
		item := Remediation{ID: "remediation-" + random[:16], Incident: req.Incident, Collector: found.Collector, Status: "PROPOSED", Reversible: true, TargetValid: target, Authorized: true, Action: "Simulate diagnostic plan: " + found.Action, Result: "Awaiting explicit human approval", At: collect.Timestamp(time.Now())}
		if a.saveRemediation(id.Tenant, item) != nil {
			failure(w, 503)
			return
		}
		a.remediations[id.Tenant] = append(a.remediations[id.Tenant], item)
		a.audit(id.Tenant, "remediation_propose", 200)
		a.engine.Publish(id.Tenant, "remediation", item)
		respond(w, 200, item)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/remediations/"), "/")
	if len(parts) != 2 {
		failure(w, 404)
		return
	}
	list := a.remediations[id.Tenant]
	index := -1
	for i := range list {
		if list[i].ID == parts[0] {
			index = i
		}
	}
	if index < 0 {
		a.audit(id.Tenant, "remediation_policy_reject", 404)
		failure(w, 404)
		return
	}
	item := list[index]
	switch parts[1] {
	case "approve":
		var req struct {
			Approved bool `json:"approved"`
		}
		if !decode(w, r, &req) || !req.Approved {
			failure(w, 400)
			return
		}
		if item.Status != "PROPOSED" || !item.Reversible || !item.TargetValid || !item.Authorized {
			failure(w, 409)
			return
		}
		active := false
		for _, incident := range a.engine.Incidents(id.Tenant) {
			if incident.ID == item.Incident && incident.Status == "ACTIVE" {
				active = true
			}
		}
		if !active {
			a.audit(id.Tenant, "remediation_policy_reject", 409)
			failure(w, 409)
			return
		}
		item.Status = "EXECUTED_SIMULATED"
		item.Result = "Dry-run completed; no host state or commands changed"
	case "rollback":
		if item.Status != "EXECUTED_SIMULATED" {
			failure(w, 409)
			return
		}
		item.Status = "ROLLED_BACK"
		item.Result = "Simulated rollback completed; no host state changed"
	default:
		failure(w, 404)
		return
	}
	item.At = collect.Timestamp(time.Now())
	if a.saveRemediation(id.Tenant, item) != nil {
		failure(w, 503)
		return
	}
	list[index] = item
	a.remediations[id.Tenant] = list
	a.audit(id.Tenant, "remediation_"+parts[1], 200)
	a.engine.Publish(id.Tenant, "remediation", item)
	respond(w, 200, item)
}
func (g *Gateway) stream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		failure(w, 405)
		return
	}
	a := g.authority
	key := bearer(r)
	a.mu.Lock()
	a.persistErr = nil
	s, ok := a.sessions[key]
	engine := a.engine
	if !ok || !time.Now().Before(s.Expires) {
		a.audit("", "auth_fail", 401)
		a.mu.Unlock()
		failure(w, 401)
		return
	}
	a.audit(s.Tenant, "dashboard_stream", 200)
	if a.persistErr != nil {
		a.mu.Unlock()
		failure(w, 503)
		return
	}
	a.mu.Unlock()
	if engine == nil {
		failure(w, 503)
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		failure(w, 500)
		return
	}
	after := uint64(0)
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		var err error
		after, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			failure(w, 400)
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	write := func(kind string, id uint64, data any) bool {
		if err := controller.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil && err != http.ErrNotSupported {
			return false
		}
		b, err := json.Marshal(data)
		if err != nil {
			return false
		}
		if _, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", id, kind, b); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	snapshot := func() bool {
		fleet, counters := engine.Fleet(s.Tenant, time.Now())
		a.mu.Lock()
		remediations := append([]Remediation{}, a.remediations[s.Tenant]...)
		a.mu.Unlock()
		return write("snapshot", after, map[string]any{"nodes": fleet, "counters": counters, "incidents": engine.Incidents(s.Tenant), "remediations": remediations})
	}
	wake, unsubscribe := engine.Subscribe(s.Tenant)
	defer unsubscribe()
	if !snapshot() {
		return
	}
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	expiry := time.NewTimer(time.Until(s.Expires))
	defer expiry.Stop()
	for {
		// Revalidate on every tick; logout and expiry terminate existing streams.
		a.mu.Lock()
		current, valid := a.sessions[key]
		valid = valid && current.Tenant == s.Tenant && time.Now().Before(current.Expires)
		a.mu.Unlock()
		if !valid {
			return
		}
		events, gap := engine.Events(s.Tenant, after)
		if gap {
			after = 0
			if !snapshot() {
				return
			}
		}
		for _, event := range events {
			if !write(event.Kind, event.ID, event.Data) {
				return
			}
			after = event.ID
		}
		select {
		case <-r.Context().Done():
			return
		case <-expiry.C:
			return
		case <-wake:
		case <-ticker.C:
			if !write("keepalive", after, map[string]string{"at": collect.Timestamp(time.Now())}) {
				return
			}
		}
	}
}
