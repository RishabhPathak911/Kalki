// Package policy is the sole constructor of approved outbound payloads. It
// strips unknown fields and blocks known fields with invalid types or values.
package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"kalki/tier1/collect"
	"kalki/tier1/filter"
)

type Rule struct {
	Type   string   `yaml:"type"`
	Format string   `yaml:"format"`
	Enum   []string `yaml:"enum"`
	Min    *float64 `yaml:"min"`
	Max    *float64 `yaml:"max"`
}
type Bounds struct {
	Min float64 `yaml:"min"`
	Max float64 `yaml:"max"`
}
type Config struct {
	Fields     map[string]Rule   `yaml:"fields"`
	Statistics map[string]Rule   `yaml:"metric_statistics"`
	Metrics    map[string]Bounds `yaml:"metrics"`
	Risk       map[string]Rule   `yaml:"risk_events"`
	Redactions struct {
		Type   string   `yaml:"type"`
		Values string   `yaml:"values"`
		Keys   []string `yaml:"keys"`
	} `yaml:"redaction_counts"`
}

type Approved struct {
	data     []byte
	hash     string
	stripped int
}

func (p Approved) Bytes() []byte      { return append([]byte(nil), p.data...) }
func (p Approved) SHA256() string     { return p.hash }
func (p Approved) StrippedCount() int { return p.stripped }

type Counters struct {
	Approved uint64
	Blocked  uint64
	Stripped uint64
}

// Event never contains payload values, field names supplied by callers, or text.
type Event struct {
	Action   string
	Reason   string
	Stripped int
	SHA256   string
}
type Gate struct {
	cfg      Config
	mu       sync.Mutex
	reportMu sync.Mutex
	counts   Counters
	report   func(Event)
}

var identifiers = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var hostID = regexp.MustCompile(`^host_[0-9a-f]{8}$`)
var peerID = regexp.MustCompile(`^ip_[0-9a-f]{8}$`)
var batchID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func Load(path string, report func(Event)) (*Gate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("policy: cannot read allowlist")
	}
	return Parse(data, report)
}
func Parse(data []byte, report func(Event)) (*Gate, error) {
	var cfg Config
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if d.Decode(&cfg) != nil {
		return nil, fmt.Errorf("policy: invalid allowlist")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("policy: multiple allowlist documents")
	}
	if !validConfig(cfg) {
		return nil, fmt.Errorf("policy: incomplete or unsafe allowlist")
	}
	return &Gate{cfg: cfg, report: report}, nil
}

func validConfig(c Config) bool {
	fields := map[string]string{"tenant": "string", "collector_id": "string", "region": "string", "seq": "integer", "batch_id": "string", "collected_at": "string", "sent_at": "string", "source": "string"}
	stats := map[string]string{"min": "float", "max": "float", "avg": "float", "p95": "float", "count": "float"}
	risks := map[string]string{"type": "string", "severity": "string", "count": "integer", "window_sec": "integer", "source_pseudonym": "string"}
	for _, pair := range []struct {
		rules map[string]Rule
		types map[string]string
	}{{c.Fields, fields}, {c.Statistics, stats}, {c.Risk, risks}} {
		if len(pair.rules) != len(pair.types) {
			return false
		}
		for k, typ := range pair.types {
			r, ok := pair.rules[k]
			if !ok || r.Type != typ {
				return false
			}
			if r.Min != nil && !finite(*r.Min) || r.Max != nil && !finite(*r.Max) || r.Min != nil && r.Max != nil && *r.Min > *r.Max {
				return false
			}
			switch r.Format {
			case "", "identifier", "pseudonym", "hex32", "utc_rfc3339_milliseconds":
			default:
				return false
			}
		}
	}
	names := []string{"cpu_pct", "collector_cpu_pct", "wal_depth", "wal_buffer_full", "mem_pct", "disk_pct", "bandwidth_mbps", "latency_ms", "packet_loss_pct", "service_up"}
	if len(c.Metrics) != len(names) {
		return false
	}
	for _, name := range names {
		b, ok := c.Metrics[name]
		if !ok || !finite(b.Min) || !finite(b.Max) || b.Min < 0 || b.Max <= b.Min {
			return false
		}
	}
	if len(c.Fields["source"].Enum) == 0 || len(c.Risk["type"].Enum) == 0 || len(c.Risk["severity"].Enum) == 0 {
		return false
	}
	if c.Redactions.Type != "map" || c.Redactions.Values != "nonnegative_integer" || len(c.Redactions.Keys) == 0 {
		return false
	}
	for _, key := range c.Redactions.Keys {
		if !contains([]string{"aws_key", "bearer", "jwt", "private_key", "credential", "high_entropy"}, key) {
			return false
		}
	}
	return true
}

func (g *Gate) Counters() Counters { g.mu.Lock(); defer g.mu.Unlock(); return g.counts }

func (g *Gate) Approve(input []byte) (Approved, error) {
	stripped := 0
	obj, reason := decode(input)
	var clean map[string]any
	if reason == "" {
		clean, reason = g.sanitize(obj, &stripped)
	}
	var p Approved
	if reason == "" {
		data, err := json.Marshal(clean)
		if err != nil {
			reason = "encoding"
		} else {
			digest := sha256.Sum256(data)
			p = Approved{data: data, hash: hex.EncodeToString(digest[:]), stripped: stripped}
		}
	}
	g.mu.Lock()
	g.counts.Stripped += uint64(stripped)
	if reason != "" {
		g.counts.Blocked++
	} else {
		g.counts.Approved++
	}
	g.mu.Unlock()
	if g.report != nil {
		g.reportMu.Lock()
		action := "approved"
		if reason != "" {
			action = "blocked"
		}
		g.report(Event{Action: action, Reason: reason, Stripped: stripped, SHA256: p.hash})
		g.reportMu.Unlock()
	}
	if reason != "" {
		return Approved{}, fmt.Errorf("policy violation: %s", reason)
	}
	return p, nil
}

func (g *Gate) sanitize(obj map[string]any, stripped *int) (map[string]any, string) {
	clean := map[string]any{}
	for key, value := range obj {
		if r, ok := g.cfg.Fields[key]; ok {
			if !validValue(value, r) || !validIdentityField(key, value) {
				return nil, "field_value"
			}
			clean[key] = value
			continue
		}
		switch key {
		case "metrics":
			metrics, ok := value.(map[string]any)
			if !ok {
				return nil, "metrics_type"
			}
			kept := map[string]any{}
			for metric, v := range metrics {
				bounds, ok := g.cfg.Metrics[metric]
				if !ok {
					*stripped++
					continue
				}
				stats, ok := v.(map[string]any)
				if !ok {
					return nil, "statistics_type"
				}
				result, valid := fields(stats, g.cfg.Statistics, stripped)
				if !valid || len(result) != 5 {
					return nil, "statistics_value"
				}
				lo, _ := number(result["min"])
				hi, _ := number(result["max"])
				avg, _ := number(result["avg"])
				p95, _ := number(result["p95"])
				count, _ := number(result["count"])
				if lo < bounds.Min || hi > bounds.Max || lo > hi || avg < lo || avg > hi || p95 < lo || p95 > hi || count < 1 || count > 100000 || math.Trunc(count) != count {
					return nil, "statistics_range"
				}
				kept[metric] = result
			}
			clean[key] = kept
		case "risk_events":
			events, ok := value.([]any)
			if !ok || len(events) > 10000 {
				return nil, "risk_type"
			}
			kept := make([]any, 0, len(events))
			for _, event := range events {
				obj, ok := event.(map[string]any)
				if !ok {
					return nil, "risk_type"
				}
				result, valid := fields(obj, g.cfg.Risk, stripped)
				if !valid {
					return nil, "risk_value"
				}
				for _, field := range []string{"type", "severity", "count", "window_sec"} {
					if _, ok := result[field]; !ok {
						return nil, "risk_missing"
					}
				}
				if result["type"] != "bruteforce_suspected" || result["severity"] != "warning" {
					return nil, "risk_value"
				}
				count, _ := number(result["count"])
				window, _ := number(result["window_sec"])
				if count < 5 || window != 60 {
					return nil, "risk_range"
				}
				if peer, ok := result["source_pseudonym"]; ok {
					if s, ok := peer.(string); !ok || !peerID.MatchString(s) {
						return nil, "risk_identity"
					}
				} else if count < 10 {
					return nil, "risk_range"
				}
				kept = append(kept, result)
			}
			clean[key] = kept
		case "redaction_counts":
			counts, ok := value.(map[string]any)
			if !ok {
				return nil, "redactions_type"
			}
			kept := map[string]any{}
			for kind, v := range counts {
				if !contains(g.cfg.Redactions.Keys, kind) {
					*stripped++
					continue
				}
				n, ok := number(v)
				if !ok || n < 0 || n > 1000000000 || math.Trunc(n) != n {
					return nil, "redactions_value"
				}
				kept[kind] = v
			}
			clean[key] = kept
		default:
			*stripped++
		}
	}
	for _, key := range []string{"tenant", "collector_id", "region", "collected_at", "source"} {
		if _, ok := clean[key]; !ok {
			return nil, "required_field"
		}
	}
	return clean, ""
}

func fields(obj map[string]any, rules map[string]Rule, stripped *int) (map[string]any, bool) {
	out := map[string]any{}
	for key, value := range obj {
		rule, ok := rules[key]
		if !ok {
			*stripped++
			continue
		}
		if !validValue(value, rule) {
			return nil, false
		}
		out[key] = value
	}
	return out, true
}
func validValue(value any, r Rule) bool {
	switch r.Type {
	case "string":
		s, ok := value.(string)
		if !ok || len(s) == 0 || len(s) > 128 {
			return false
		}
		if len(r.Enum) > 0 && !contains(r.Enum, s) {
			return false
		}
		if r.Format != "hex32" {
			if clean, counts := filter.Scrub(s); clean != s || len(counts) > 0 {
				return false
			}
		}
		switch r.Format {
		case "identifier":
			return identifiers.MatchString(s)
		case "hex32":
			return batchID.MatchString(s)
		case "pseudonym":
			return hostID.MatchString(s) || peerID.MatchString(s)
		case "utc_rfc3339_milliseconds":
			t, err := time.Parse(collect.TimeLayout, s)
			return err == nil && collect.Timestamp(t) == s
		}
		return true
	case "float", "integer":
		n, ok := number(value)
		return ok && (r.Type != "integer" || math.Trunc(n) == n) && (r.Min == nil || n >= *r.Min) && (r.Max == nil || n <= *r.Max)
	}
	return false
}
func validIdentityField(key string, value any) bool {
	s, _ := value.(string)
	switch key {
	case "tenant", "region":
		return identifiers.MatchString(s)
	case "collector_id":
		return hostID.MatchString(s)
	case "source":
		return contains([]string{"live", "replayed_real_trace", "simulated"}, s)
	case "batch_id":
		return batchID.MatchString(s)
	case "collected_at", "sent_at":
		t, err := time.Parse(collect.TimeLayout, s)
		return err == nil && collect.Timestamp(t) == s
	case "seq":
		n, ok := number(value)
		return ok && n >= 1 && n <= 9007199254740991
	}
	return false
}
func number(v any) (float64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := n.Float64()
	return f, err == nil && finite(f)
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
func contains(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

// Token-based decoding rejects duplicate keys, trailing documents, excessive
// nesting and oversized input rather than relying on last-key-wins semantics.
func decode(data []byte) (map[string]any, string) {
	if len(data) > 1024*1024 {
		return nil, "payload_size"
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := readValue(d, 0)
	if err != nil {
		return nil, "invalid_json"
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, "invalid_json"
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, "payload_type"
	}
	return obj, ""
}
func readValue(d *json.Decoder, depth int) (any, error) {
	if depth > 16 {
		return nil, fmt.Errorf("nesting")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := t.(json.Delim); ok {
		switch delim {
		case '{':
			obj := map[string]any{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("key")
				}
				if _, exists := obj[name]; exists {
					return nil, fmt.Errorf("duplicate")
				}
				value, err := readValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				obj[name] = value
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("object")
			}
			return obj, nil
		case '[':
			items := []any{}
			for d.More() {
				v, err := readValue(d, depth+1)
				if err != nil {
					return nil, err
				}
				items = append(items, v)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("array")
			}
			return items, nil
		}
		return nil, fmt.Errorf("delimiter")
	}
	return t, nil
}
