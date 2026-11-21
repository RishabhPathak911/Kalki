package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func testGate(t *testing.T, report func(Event)) *Gate {
	t.Helper()
	g, err := Load(filepath.Join("..", "..", "config", "allowlist.yaml"), report)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func validPayload() map[string]any {
	return map[string]any{"tenant": "tenant_alpha", "collector_id": "host_12345678", "region": "ap-south-1", "collected_at": "2026-11-20T00:00:00.000Z", "source": "live", "metrics": map[string]any{"cpu_pct": map[string]any{"min": 10, "max": 20, "avg": 15, "p95": 20, "count": 5}}, "risk_events": []any{map[string]any{"type": "bruteforce_suspected", "severity": "warning", "count": 5, "window_sec": 60, "source_pseudonym": "ip_12345678"}}, "redaction_counts": map[string]any{"aws_key": 2}}
}
func encode(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestUnknownFieldsStrippedAtEveryLevel(t *testing.T) {
	var logs bytes.Buffer
	g := testGate(t, func(e Event) {
		data, err := json.Marshal(e)
		if err != nil {
			t.Error(err)
		}
		logs.Write(data)
	})
	p := validPayload()
	secret := "AKIAABCDEFGHIJKLMNOP"
	p[secret] = secret
	p["metrics"].(map[string]any)["cpu_pct"].(map[string]any)["raw"] = secret
	p["metrics"].(map[string]any)[secret] = secret
	p["risk_events"].([]any)[0].(map[string]any)["username"] = secret
	p["redaction_counts"].(map[string]any)[secret] = secret
	approved, err := g.Approve(encode(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(approved.Bytes(), []byte(secret)) || strings.Contains(logs.String(), secret) {
		t.Fatal("secret escaped policy gate")
	}
	if c := g.Counters(); c.Stripped != 5 || c.Approved != 1 || c.Blocked != 0 {
		t.Fatalf("bad counters: %+v", c)
	}
	data := approved.Bytes()
	digest := sha256.Sum256(data)
	if approved.SHA256() != hex.EncodeToString(digest[:]) {
		t.Fatal("hash mismatch")
	}
	data[0] = '!'
	if approved.Bytes()[0] != '{' {
		t.Fatal("approved bytes can be mutated")
	}
}

func TestViolationsBlockAndDoNotEchoInput(t *testing.T) {
	cases := map[string]func(map[string]any){
		"secret tenant":      func(p map[string]any) { p["tenant"] = "AKIAABCDEFGHIJKLMNOP" },
		"raw hostname":       func(p map[string]any) { p["collector_id"] = "private-host.internal" },
		"unknown region":     func(p map[string]any) { p["region"] = "some-region" },
		"unknown source":     func(p map[string]any) { p["source"] = "pretend-live" },
		"wrong type":         func(p map[string]any) { p["metrics"].(map[string]any)["cpu_pct"].(map[string]any)["avg"] = "15" },
		"range":              func(p map[string]any) { p["metrics"].(map[string]any)["cpu_pct"].(map[string]any)["max"] = 101 },
		"inconsistent stats": func(p map[string]any) { p["metrics"].(map[string]any)["cpu_pct"].(map[string]any)["p95"] = 0 },
		"fractional count":   func(p map[string]any) { p["metrics"].(map[string]any)["cpu_pct"].(map[string]any)["count"] = 1.5 },
		"negative counter":   func(p map[string]any) { p["redaction_counts"].(map[string]any)["aws_key"] = -1 },
		"raw IP": func(p map[string]any) {
			p["risk_events"].([]any)[0].(map[string]any)["source_pseudonym"] = "192.0.2.17"
		},
		"missing required":  func(p map[string]any) { delete(p, "tenant") },
		"noncanonical time": func(p map[string]any) { p["collected_at"] = "2026-11-20T00:00:00Z" },
		"wrong risk type":   func(p map[string]any) { p["risk_events"] = "Failed password" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var events []Event
			g := testGate(t, func(e Event) { events = append(events, e) })
			p := validPayload()
			mutate(p)
			approved, err := g.Approve(encode(t, p))
			if err == nil || len(approved.Bytes()) != 0 || len(events) != 1 || events[0].Action != "blocked" || g.Counters().Blocked != 1 {
				t.Fatal("violation not blocked")
			}
			if strings.Contains(err.Error(), "AKIA") || strings.Contains(err.Error(), "192.0.2.17") || strings.Contains(err.Error(), "private-host") {
				t.Fatal("error reflected raw input")
			}
		})
	}
}

func TestRejectMalformedJSON(t *testing.T) {
	g := testGate(t, nil)
	for _, input := range []string{`{"tenant":"tenant_alpha","tenant":"tenant_beta"}`, `{"metrics":{"cpu_pct":{"avg":1,"avg":2}}}`, `{} {}`, `null`, `[]`, `{"value":NaN}`, `{"value":1e999}`, strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18), strings.Repeat(" ", 1024*1024+1)} {
		if _, err := g.Approve([]byte(input)); err == nil {
			t.Fatalf("accepted malformed payload")
		}
	}
}

func TestConfigFailsClosed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "allowlist.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{[]byte("fields: {}"), append(append([]byte{}, data...), []byte("\nunknown: true\n")...), bytes.Replace(data, []byte("min: 0, max: 100}"), []byte("min: 200, max: 100}"), 1)} {
		if _, err := Parse(bad, nil); err == nil {
			t.Fatal("accepted invalid policy config")
		}
	}
}

func TestConcurrentApprovals(t *testing.T) {
	var logs bytes.Buffer
	g := testGate(t, func(e Event) { logs.WriteString(e.Action) })
	data := encode(t, validPayload())
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.Approve(data); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if g.Counters().Approved != 20 || logs.Len() == 0 {
		t.Fatal("lost approvals")
	}
}
