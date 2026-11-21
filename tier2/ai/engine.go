// Package ai is a read-only, privacy-projected explanation adapter.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kalki/tier2/medha"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Input struct {
	Tenant     string
	Question   string
	IncidentID string
	Fleet      []medha.Node
	Incidents  []medha.Explanation
	AuditValid bool
}
type Result struct {
	Mode      string          `json:"mode"`
	Model     string          `json:"model"`
	Summary   string          `json:"summary"`
	Root      string          `json:"root_cause_brief"`
	Commands  []string        `json:"local_diagnostic_commands"`
	Rationale string          `json:"remediation_rationale"`
	Answer    string          `json:"answer"`
	Prompt    json.RawMessage `json:"sanitized_prompt_json"`
}
type Engine struct{ URL, Key, Model string }

func FromEnv() Engine {
	return Engine{os.Getenv("KALKI_AI_URL"), os.Getenv("KALKI_AI_KEY"), os.Getenv("KALKI_AI_MODEL")}
}

var host = regexp.MustCompile(`^host_[a-f0-9]{8,64}$`)
var metrics = map[string]bool{"cpu_pct": true, "collector_cpu_pct": true, "mem_pct": true, "disk_pct": true, "latency_ms": true, "packet_loss_pct": true, "bandwidth_mbps": true, "service_up": true, "wal_depth": true}

func enum(s string, allowed ...string) string {
	for _, v := range allowed {
		if s == v {
			return s
		}
	}
	return "unknown"
}
func intent(q string) string {
	q = strings.ToLower(q)
	if strings.Contains(q, "security") || strings.Contains(q, "audit") {
		return "security_posture"
	}
	return "least_healthy_node"
}

// Project permits no arbitrary text: not even operator questions or configurable
// playbook strings cross the boundary. Question text selects a fixed local intent.
func Project(in Input) (json.RawMessage, error) {
	nodes := []any{}
	incidents := []any{}
	found := in.IncidentID == ""
	for _, n := range in.Fleet {
		if n.Tenant != in.Tenant || !host.MatchString(n.Collector) {
			continue
		}
		m := map[string]float64{}
		for k, v := range n.Metrics {
			if metrics[k] {
				m[k] = v.Avg
			}
		}
		nodes = append(nodes, map[string]any{"collector_id": n.Collector, "health": enum(n.Health, "Excellent", "Good", "Fair", "Poor"), "connection_state": enum(n.Connection, "ONLINE", "STALE", "OFFLINE"), "metrics": m})
	}
	for _, x := range in.Incidents {
		if x.Tenant != in.Tenant || !host.MatchString(x.Collector) || (in.IncidentID != "" && x.ID != in.IncidentID) {
			continue
		}
		found = true
		signals := []any{}
		for _, s := range x.Signals {
			if metrics[s.Metric] {
				signals = append(signals, map[string]any{"metric": s.Metric, "observed": s.Observed, "baseline": s.Baseline, "deviation": s.Deviation, "weight": s.Weight, "direction": enum(s.Direction, "up", "down")})
			}
		}
		incidents = append(incidents, map[string]any{"collector_id": x.Collector, "severity": enum(x.Severity, "critical", "warning", "info"), "status": enum(x.Status, "ACTIVE", "RESOLVED"), "confidence": x.Confidence, "signals": signals, "time_to_detect_ms": x.DetectionMS})
	}
	if !found {
		return nil, fmt.Errorf("incident not found")
	}
	return json.Marshal(map[string]any{"intent": intent(in.Question), "fleet": nodes, "incidents": incidents, "audit_chain_valid": in.AuditValid})
}
func (e Engine) Run(ctx context.Context, in Input) (Result, error) {
	prompt, err := Project(in)
	if err != nil {
		return Result{}, err
	}
	summary := "No collector observations available."
	worst := -1
	states := map[string]int{"Excellent": 0, "Good": 1, "Fair": 2, "Poor": 3}
	for _, n := range in.Fleet {
		if n.Tenant == in.Tenant && host.MatchString(n.Collector) && states[n.Health] > worst {
			worst = states[n.Health]
			summary = fmt.Sprintf("%s is %s (%s). Review its current metric thresholds and incident evidence.", n.Collector, enum(n.Health, "Excellent", "Good", "Fair", "Poor"), enum(n.Connection, "ONLINE", "STALE", "OFFLINE"))
		}
	}
	count := 0
	for _, x := range in.Incidents {
		if x.Tenant == in.Tenant && x.Status == "ACTIVE" {
			count++
		}
	}
	if intent(in.Question) == "security_posture" {
		summary = fmt.Sprintf("Audit chain verified: %t. Active incidents: %d. This snapshot alone does not establish a security breach.", in.AuditValid, count)
	}
	out := Result{Mode: "deterministic_fallback", Model: "fixed-rules", Summary: summary, Root: "Correlated observations are evidence, not proof of a root cause. Inspect contributing metrics locally.", Commands: []string{"Get-Process | Sort-Object CPU -Descending | Select-Object -First 5", "Get-NetTCPConnection | Group-Object State | Select-Object Name,Count"}, Rationale: "Use the existing tenant-authorized, reversible remediation simulator; human approval is required.", Answer: summary, Prompt: prompt}
	u, err := url.Parse(e.URL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return out, nil
	}
	ip := net.ParseIP(u.Hostname())
	local := ip != nil && ip.IsLoopback()
	if !local && (u.Scheme != "https" || e.Key == "") {
		return out, nil
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return out, nil
	}
	if e.Model == "" {
		return out, nil
	}
	system := "Use only the supplied metadata. Briefly explain observations and uncertainty. Do not invent facts, commands, identities, or actions. No tools. Never claim remediation ran."
	messages := []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": string(prompt)}}
	body := map[string]any{"model": e.Model, "messages": messages, "stream": false}
	gemini := strings.Contains(u.Path, ":generateContent")
	ollama := strings.HasSuffix(u.Path, "/api/chat")
	if gemini {
		body = map[string]any{"systemInstruction": map[string]any{"parts": []map[string]string{{"text": system}}}, "contents": []any{map[string]any{"role": "user", "parts": []map[string]string{{"text": string(prompt)}}}}}
	}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(raw))
	if err != nil {
		return out, nil
	}
	req.Header.Set("Content-Type", "application/json")
	if e.Key != "" {
		if gemini {
			req.Header.Set("x-goog-api-key", e.Key)
		} else {
			req.Header.Set("Authorization", "Bearer "+e.Key)
		}
	}
	client := &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return out, nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return out, nil
	}
	raw, err = io.ReadAll(io.LimitReader(res.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return out, nil
	}
	var v struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return out, nil
	}
	answer := ""
	if gemini && len(v.Candidates) > 0 {
		for _, p := range v.Candidates[0].Content.Parts {
			answer += p.Text
		}
	} else if ollama {
		answer = v.Message.Content
	} else if len(v.Choices) > 0 {
		answer = v.Choices[0].Message.Content
	}
	answer = strings.TrimSpace(answer)
	if answer == "" || len(answer) > 8000 {
		return out, nil
	}
	out.Mode = "cloud"
	if local {
		out.Mode = "local_offline"
	}
	out.Model = e.Model
	out.Summary = answer
	out.Answer = answer
	out.Root = answer
	return out, nil
}
