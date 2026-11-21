package ai

import (
	"context"
	"io"
	"kalki/tier2/medha"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func input() Input {
	return Input{Tenant: "tenant_alpha", Question: "security password=secret 192.168.1.2 alice", AuditValid: true, Fleet: []medha.Node{{Tenant: "tenant_alpha", Collector: "host_12345678", Health: "Poor", Connection: "ONLINE"}, {Tenant: "tenant_beta", Collector: "host_87654321"}}, Incidents: []medha.Explanation{{ID: "a", Tenant: "tenant_alpha", Collector: "host_12345678", Summary: "password=secret alice 192.168.1.2", Action: "secret"}, {ID: "b", Tenant: "tenant_beta", Collector: "host_87654321"}}}
}
func TestPrivacyFallbackAndTenant404(t *testing.T) {
	in := input()
	r, e := (Engine{}).Run(context.Background(), in)
	if e != nil || r.Mode != "deterministic_fallback" {
		t.Fatal(e)
	}
	for _, secret := range []string{"secret", "alice", "192.168", "87654321", "tenant_beta"} {
		if strings.Contains(string(r.Prompt), secret) {
			t.Fatal("leaked", secret)
		}
	}
	in.IncidentID = "b"
	if _, e = Project(in); e == nil {
		t.Fatal("cross-tenant incident found")
	}
}
func TestProvidersAndErrorFallback(t *testing.T) {
	for _, provider := range []string{"openai", "ollama", "gemini", "error"} {
		t.Run(provider, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				if strings.Contains(string(b), "password") || strings.Contains(r.URL.String(), "key") {
					t.Error("privacy boundary")
				}
				switch provider {
				case "openai":
					io.WriteString(w, `{"choices":[{"message":{"content":"Review metrics."}}]}`)
				case "ollama":
					io.WriteString(w, `{"message":{"content":"Review metrics."}}`)
				case "gemini":
					if r.Header.Get("x-goog-api-key") != "test" {
						t.Error("missing key")
					}
					io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"Review metrics."}]}}]}`)
				default:
					w.WriteHeader(503)
				}
			}))
			defer s.Close()
			path := "/chat/completions"
			if provider == "ollama" {
				path = "/api/chat"
			}
			if provider == "gemini" {
				path = "/models/test:generateContent"
			}
			out, e := (Engine{URL: s.URL + path, Key: "test", Model: "test"}).Run(context.Background(), input())
			if e != nil {
				t.Fatal(e)
			}
			if provider == "error" {
				if out.Mode != "deterministic_fallback" {
					t.Fatal(out)
				}
			} else if out.Answer != "Review metrics." {
				t.Fatal(out)
			}
		})
	}
}
