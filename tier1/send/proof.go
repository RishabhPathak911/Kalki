package send

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"kalki/internal/telemetry"
	"kalki/tier1/wal"
)

type Proof struct {
	Seq        uint64         `json:"seq"`
	BatchID    string         `json:"batch_id"`
	Bytes      string         `json:"pre_send_bytes"`
	Hash       string         `json:"pre_send_sha256"`
	Status     string         `json:"status"`
	Echo       string         `json:"tier2_echoed_sha256"`
	Match      bool           `json:"match_bool"`
	Redactions map[string]int `json:"redaction_counts"`
	Stripped   int            `json:"stripped_count"`
}
type ProofRing struct {
	mu      sync.Mutex
	entries []Proof
}

func (p *ProofRing) Update(e wal.Entry, status, echo string) {
	var b telemetry.Batch
	if json.Unmarshal(e.Bytes, &b) != nil {
		return
	}
	counts := map[string]int{}
	for _, r := range b.Records {
		for kind, n := range r.RedactionCounts {
			counts[kind] += n
		}
	}
	entry := Proof{Seq: e.Seq, BatchID: b.BatchID, Bytes: string(e.Bytes), Hash: e.Hash, Status: status, Echo: echo, Match: echo != "" && echo == e.Hash, Redactions: counts, Stripped: e.Stripped}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, old := range p.entries {
		if old.BatchID == entry.BatchID {
			// The worker can ACK a batch before Enqueue publishes its initial
			// BUFFERED proof. Do not let that late publication regress its state.
			if status == "BUFFERED" && (old.Status == "SENT" || old.Status == "ACKED") {
				return
			}
			p.entries[i] = entry
			return
		}
	}
	if len(p.entries) == 50 {
		copy(p.entries, p.entries[1:])
		p.entries[49] = entry
	} else {
		p.entries = append(p.entries, entry)
	}
}

func (p *ProofRing) Reset() { p.mu.Lock(); defer p.mu.Unlock(); p.entries = nil }
func (p *ProofRing) Snapshot() []Proof {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Proof, len(p.entries))
	for i, e := range p.entries {
		out[i] = e
		out[i].Redactions = map[string]int{}
		for k, n := range e.Redactions {
			out[i].Redactions[k] = n
		}
	}
	return out
}

func ListenProof(addr string, d *Delivery) (net.Listener, http.Handler, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, nil, fmt.Errorf("proof address must be a literal loopback IP")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	return listener, d.ProofHandler(), nil
}
func (d *Delivery) ProofHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		ip := net.ParseIP(host)
		remote, _, remoteErr := net.SplitHostPort(r.RemoteAddr)
		if ip == nil || !ip.IsLoopback() || remoteErr != nil || net.ParseIP(remote) == nil || !net.ParseIP(remote).IsLoopback() {
			w.WriteHeader(403)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || u.Host != r.Host {
				w.WriteHeader(403)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/demo/") {
			d.demoRequest(w, r)
			return
		}
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		var result any
		switch r.URL.Path {
		case "/api/identity":
			result = map[string]string{"tenant": d.binding.Tenant, "collector_id": d.binding.CollectorID, "region": d.binding.Region}
		case "/api/egress-proof":
			d.mu.Lock()
			paused := d.paused
			d.mu.Unlock()
			result = map[string]any{"batches": d.proofs.Snapshot(), "counters": d.Counters(), "demo": map[string]bool{"enabled": d.Demo != nil, "delivery_paused": paused}}
		case "/api/attempt-violation":
			// A fixed, non-secret invalid value tests the real gate. It never enters
			// the WAL or network; only diagnostic counters/proof history change.
			result = d.attemptViolation()
		default:
			w.WriteHeader(404)
			return
		}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			return
		}
	})
}
