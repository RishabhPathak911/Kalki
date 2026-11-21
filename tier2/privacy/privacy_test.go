// These are boundary contract tests, not a deployed Tier 2 implementation.
package privacy_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kalki/tier1/collect"
	"kalki/tier1/filter"
	"kalki/tier1/pipeline"
	"kalki/tier1/policy"
)

type fixtureSource struct{ samples []collect.RawSample }

func (fixtureSource) Name() string { return "test-fixture" }
func (s fixtureSource) Stream(ctx context.Context) <-chan collect.RawSample {
	out := make(chan collect.RawSample)
	go func() {
		defer close(out)
		for _, sample := range s.samples {
			select {
			case out <- sample:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func TestNoSecretsAtTier2Boundary(t *testing.T) {
	var mu sync.Mutex
	var receiverMemory [][]byte
	var receiverLog bytes.Buffer
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			http.Error(w, "read failed", 400)
			return
		}
		digest := sha256.Sum256(data)
		hash := hex.EncodeToString(digest[:])
		mu.Lock()
		receiverMemory = append(receiverMemory, append([]byte{}, data...))
		fmt.Fprintf(&receiverLog, "received sha256=%s\n", hash)
		mu.Unlock()
		w.Header().Set("X-Payload-SHA256", hash)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	identity, err := filter.LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	var policyLogs bytes.Buffer
	gate, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), func(e policy.Event) {
		fmt.Fprintf(&policyLogs, "%s %s %d %s", e.Action, e.Reason, e.Stripped, e.SHA256)
	})
	if err != nil {
		t.Fatal(err)
	}
	now := collect.Timestamp(time.Now())
	secrets := []string{"AKIAABCDEFGHIJKLMNOP", "bearer-secret-123", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature", "private-key-material", "password-value", "aB3dE5gH7jK9mN2pQ4sT6vW8xY0z", "unrecognized-low-entropy-secret"}
	lines := []string{secrets[0], "Bearer " + secrets[1], secrets[2], "-----BEGIN PRIVATE KEY-----\n" + secrets[3] + "\n-----END PRIVATE KEY-----", "password=" + secrets[4], secrets[5], secrets[6]}
	samples := []collect.RawSample{}
	for _, line := range lines {
		samples = append(samples, collect.RawSample{CollectedAt: now, Source: collect.Replayed, Metric: "cpu_pct", Value: 42, Log: "Failed password for privateuser from 192.0.2.17 port 22 ssh2 " + line, Hostname: "private-host.internal", Username: "privateuser", StreamID: "/private/raw/path", Tags: map[string]string{"tag": "api_key=tag-secret"}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = pipeline.Run(ctx, []collect.Source{fixtureSource{samples}}, pipeline.Options{Tenant: "tenant_alpha", NodeID: "private-host.internal", Region: "ap-south-1", Identity: identity, Gate: gate}, func(p policy.Approved) error {
		body := p.Bytes()
		for _, secret := range secrets {
			if bytes.Contains(body, []byte(secret)) {
				return fmt.Errorf("secret in outbound payload")
			}
		}
		resp, err := server.Client().Post(server.URL, "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		closeErr := resp.Body.Close()
		if closeErr != nil {
			return closeErr
		}
		if resp.StatusCode != http.StatusNoContent || resp.Header.Get("X-Payload-SHA256") != p.SHA256() {
			return fmt.Errorf("received payload did not match approved bytes")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(receiverMemory) != 1 {
		t.Fatalf("expected one summary, got %d", len(receiverMemory))
	}
	for _, secret := range append(secrets, "private-host.internal", "privateuser", "192.0.2.17", "/private/raw/path", "tag-secret") {
		if bytes.Contains(receiverMemory[0], []byte(secret)) || strings.Contains(receiverLog.String(), secret) || strings.Contains(policyLogs.String(), secret) {
			t.Fatal("raw data in boundary state or logs")
		}
	}
	var record struct {
		Redactions map[string]int   `json:"redaction_counts"`
		Risks      []map[string]any `json:"risk_events"`
	}
	if err := json.Unmarshal(receiverMemory[0], &record); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"aws_key", "bearer", "jwt", "private_key", "credential", "high_entropy"} {
		if record.Redactions[kind] < 1 {
			t.Fatalf("missing %s counter", kind)
		}
	}
	if len(record.Risks) != 1 || record.Risks[0]["type"] != "bruteforce_suspected" {
		t.Fatal("missing brute-force risk")
	}
}

func TestViolationNeverInvokesSender(t *testing.T) {
	identity, err := filter.LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	gate, err := policy.Load(filepath.Join("..", "..", "config", "allowlist.yaml"), nil)
	if err != nil {
		t.Fatal(err)
	}
	sample := collect.RawSample{CollectedAt: collect.Timestamp(time.Now()), Source: collect.Live, Metric: "cpu_pct", Value: 101}
	sent := false
	err = pipeline.Run(context.Background(), []collect.Source{fixtureSource{[]collect.RawSample{sample}}}, pipeline.Options{Tenant: "tenant_alpha", NodeID: "node-1", Region: "ap-south-1", Identity: identity, Gate: gate}, func(policy.Approved) error { sent = true; return nil })
	if err == nil || sent || gate.Counters().Blocked != 1 {
		t.Fatal("invalid payload reached sender")
	}
}
