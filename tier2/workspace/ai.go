package workspace

import (
	"kalki/tier2/ai"
	"net/http"
	"time"
	"unicode/utf8"
)

func (g *Gateway) ai(w http.ResponseWriter, r *http.Request) {
	a := g.authority
	a.mu.Lock()
	a.persistErr = nil
	key := bearer(r)
	s, ok := a.sessions[key]
	if !ok || !time.Now().Before(s.Expires) {
		a.audit("", "auth_fail", 401)
		a.mu.Unlock()
		failure(w, 401)
		return
	}
	if r.Method != "POST" {
		a.mu.Unlock()
		failure(w, 405)
		return
	}
	if a.engine == nil {
		a.mu.Unlock()
		failure(w, 503)
		return
	}
	var body struct {
		IncidentID string `json:"incident_id"`
		Question   string `json:"question"`
	}
	if !decode(w, r, &body) || utf8.RuneCountInString(body.Question) > 500 {
		a.mu.Unlock()
		failure(w, 400)
		return
	}
	event := "ai_query"
	if r.URL.Path == "/api/ai/analyze-incident" {
		event = "ai_analyze"
		if body.IncidentID == "" || body.Question != "" {
			a.mu.Unlock()
			failure(w, 400)
			return
		}
	} else if body.Question == "" || body.IncidentID != "" {
		a.mu.Unlock()
		failure(w, 400)
		return
	}
	fleet, _ := a.engine.Fleet(s.Tenant, time.Now())
	input := ai.Input{Tenant: s.Tenant, Question: body.Question, IncidentID: body.IncidentID, Fleet: fleet, Incidents: a.engine.Incidents(s.Tenant), AuditValid: a.audits[s.Tenant] != nil && a.audits[s.Tenant].verify()}
	if _, err := ai.Project(input); err != nil {
		a.audit(s.Tenant, event, 404)
		a.mu.Unlock()
		failure(w, 404)
		return
	}
	a.audit(s.Tenant, event, 200)
	if a.persistErr != nil {
		a.mu.Unlock()
		failure(w, 503)
		return
	}
	a.mu.Unlock()
	// External I/O must never hold the authority lock or stall ingest/SSE.
	result, err := ai.FromEnv().Run(r.Context(), input)
	if err != nil {
		failure(w, 500)
		return
	}
	a.mu.Lock()
	current, ok := a.sessions[key]
	valid := ok && time.Now().Before(current.Expires)
	a.mu.Unlock()
	if !valid {
		failure(w, 401)
		return
	}
	respond(w, 200, result)
}
