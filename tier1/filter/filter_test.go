package filter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"kalki/tier1/collect"
)

func TestSecretRules(t *testing.T) {
	cases := []struct{ kind, positive, negative, secret string }{
		{"aws_key", "key AKIAABCDEFGHIJKLMNOP here", "key AKIA123456789012345 here", "AKIAABCDEFGHIJKLMNOP"},
		{"bearer", "Authorization: Bearer abc.def-123_foo", "Authorization: Bearer ", "abc.def-123_foo"},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature", "eyJabc.notjwt.signature", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature"},
		{"private_key", "-----BEGIN RSA PRIVATE KEY-----", "-----BEGIN PUBLIC KEY-----", "-----BEGIN RSA PRIVATE KEY-----"},
		{"credential", "password = \"correct horse battery\"", "passwordless=true", "correct horse battery"},
		{"high_entropy", "aB3dE5gH7jK9mN2pQ4sT6vW8xY0z", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "aB3dE5gH7jK9mN2pQ4sT6vW8xY0z"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			clean, counts := Scrub(tc.positive)
			if counts[tc.kind] != 1 || !strings.Contains(clean, "[REDACTED:"+tc.kind+"]") || strings.Contains(clean, tc.secret) {
				t.Fatalf("rule %s did not redact", tc.kind)
			}
			clean, counts = Scrub(tc.negative)
			if len(counts) != 0 || clean != tc.negative {
				t.Fatalf("rule %s redacted benign text", tc.kind)
			}
		})
	}
	for _, name := range []string{"api_key", "token", "secret", "password", "passwd"} {
		clean, counts := Scrub(name + "=sensitive-value")
		if counts["credential"] != 1 || strings.Contains(clean, "sensitive-value") {
			t.Fatalf("missed %s", name)
		}
	}
	clean, counts := Scrub("-----BEGIN PRIVATE KEY-----\nprivate material\n-----END PRIVATE KEY-----")
	if clean != "[REDACTED:private_key]" || counts["private_key"] != 1 {
		t.Fatal("PEM body not removed")
	}
	clean, counts = Scrub("prefixAKIAABCDEFGHIJKLMNOPsuffix")
	if counts["aws_key"] != 1 || strings.Contains(clean, "AKIAABCDEFGHIJKLMNOP") {
		t.Fatal("embedded AWS key was not scrubbed")
	}
}

func TestIdentityPersistenceAndTenantSeparation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenant.key")
	a, err := LoadIdentity(path, "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadIdentity(path, "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	c, err := LoadIdentity(filepath.Join(dir, "beta.key"), "tenant_beta")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"host", "user", "ip"} {
		p := a.Pseudonym(kind, "private-identity")
		if !regexp.MustCompile("^" + kind + "_[0-9a-f]{8}$").MatchString(p) {
			t.Fatal("invalid pseudonym format")
		}
		if p != b.Pseudonym(kind, "private-identity") || p == c.Pseudonym(kind, "private-identity") {
			t.Fatal("identity persistence or tenant isolation failed")
		}
	}
	if a.Pseudonym("host", "same")[5:] == a.Pseudonym("user", "same")[5:] {
		t.Fatal("missing kind separation")
	}
	if _, err := LoadIdentity(path, "tenant_beta"); err == nil {
		t.Fatal("shared key across tenants")
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIdentity(path, "tenant_alpha"); err == nil {
		t.Fatal("regenerated corrupt key")
	}
}

func TestKeyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows permissions use inherited ACLs")
	}
	path := filepath.Join(t.TempDir(), "tenant.key")
	if _, err := LoadIdentity(path, "tenant_alpha"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIdentity(path, "tenant_alpha"); err == nil {
		t.Fatal("accepted world-readable key")
	}
}

func TestFilterDropsAllTextAndPseudonymizesIdentities(t *testing.T) {
	id, err := LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	f := Filter{Identity: id}
	now := time.Now()
	raw := collect.RawSample{CollectedAt: collect.Timestamp(now), Source: collect.Replayed, Hostname: "private-host.internal", Username: "privateuser", StreamID: "/private/trace", Tags: map[string]string{"token=tag-key-secret": "secret=tag-value-secret"}, Log: "Failed password for invalid user privateuser from 192.0.2.17 port 22 ssh2 api_key=log-secret"}
	s, err := f.Process(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Failure == nil || s.Failure.SourcePseudonym != id.Pseudonym("ip", "192.0.2.17") || s.HostPseudonym != id.Pseudonym("host", raw.Hostname) || s.UserPseudonym != id.Pseudonym("user", "privateuser") {
		t.Fatal("identities were not pseudonymized")
	}
	if s.RedactionCounts["credential"] != 3 {
		t.Fatalf("redaction count=%d", s.RedactionCounts["credential"])
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-host", "privateuser", "192.0.2.17", "/private/trace", "log-secret", "tag-key-secret", "tag-value-secret"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("raw data escaped filter")
		}
	}
	raw.Log = "Failed password for privateuser from 2001:db8::1 port 22 ssh2"
	s, err = f.Process(raw, now)
	if err != nil || s.Failure == nil || s.Failure.SourcePseudonym != id.Pseudonym("ip", "2001:db8::1") {
		t.Fatal("IPv6 login not parsed")
	}
	raw.Log = "Accepted password for privateuser from 192.0.2.17 port 22 ssh2"
	s, err = f.Process(raw, now)
	if err != nil || s.Failure != nil {
		t.Fatal("successful login became a failure")
	}
	raw.Log = "Failed password for privateuser from invalid-address port 22 ssh2"
	s, err = f.Process(raw, now)
	if err != nil || s.Failure != nil {
		t.Fatal("invalid address accepted")
	}
}

func TestSecretLookingIdentitiesDoNotCollapse(t *testing.T) {
	id, err := LoadIdentity(filepath.Join(t.TempDir(), "tenant.key"), "tenant_alpha")
	if err != nil {
		t.Fatal(err)
	}
	f := Filter{Identity: id}
	now := time.Now()
	raw := collect.RawSample{CollectedAt: collect.Timestamp(now), Source: collect.Live, Hostname: "token=first-secret"}
	a, err := f.Process(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	raw.Hostname = "token=second-secret"
	b, err := f.Process(raw, now)
	if err != nil {
		t.Fatal(err)
	}
	if a.HostPseudonym == b.HostPseudonym || a.RedactionCounts["credential"] != 1 || b.RedactionCounts["credential"] != 1 {
		t.Fatal("identity collision after redaction")
	}
}
