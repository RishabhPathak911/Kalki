// Package ingest contains the in-memory Setu receiver. No database, raw logs,
// or filesystem storage is used. Production listeners must wrap this internal
// protocol handler in workspace.Gateway, which enforces authentication.
package ingest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
)

type StoredRecord struct {
	Seq        uint64           `json:"seq"`
	BatchID    string           `json:"batch_id"`
	ReceivedAt string           `json:"received_at"`
	Record     telemetry.Record `json:"record"`
}
type node struct {
	tenant, region                string
	seen                          map[uint64]string
	contiguous, max               uint64
	duplicates, accepted, evicted uint64
	watermark                     time.Time
	lastSeen                      string
	ring                          []StoredRecord
	next                          int
}
type Snapshot struct {
	Records            []StoredRecord `json:"records"`
	HighestContiguous  uint64         `json:"highest_contiguous_seq"`
	Gaps               []uint64       `json:"gaps"`
	DuplicatesRejected uint64         `json:"duplicates_rejected"`
	RecordsAccepted    uint64         `json:"records_accepted"`
	RecordsEvicted     uint64         `json:"records_evicted"`
	LastSeen           string         `json:"last_seen"`
}
type Server struct {
	mu            sync.Mutex
	gate          *policy.Gate
	epoch, region string
	capacity      int
	nodes         map[string]*node
}

func New(gate *policy.Gate, region string, capacity int) (*Server, error) {
	if gate == nil || capacity < 1 || (region != "ap-south-1" && region != "eu-central-1") {
		return nil, fmt.Errorf("invalid receiver configuration")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	return &Server{gate: gate, epoch: hex.EncodeToString(id[:]), region: region, capacity: capacity, nodes: map[string]*node{}}, nil
}
func (s *Server) ID() string { return s.epoch }
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if r.TLS == nil {
		http.Error(w, `{"error":"HTTPS_REQUIRED"}`, http.StatusUpgradeRequired)
		return
	}
	switch r.URL.Path {
	case "/api/session":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		writeJSON(w, map[string]string{"server_id": s.epoch, "region": s.region})
	case "/api/ingest":
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		// A restart between handshake and POST cannot accept a new batch before
		// older retained WAL history is replayed into the new receiver session.
		if r.Header.Get("X-Kalki-Server-ID") != s.epoch {
			http.Error(w, `{"error":"SESSION_CHANGED"}`, 409)
			return
		}
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
		if err != nil {
			http.Error(w, `{"error":"PAYLOAD_SIZE"}`, 413)
			return
		}
		approved, err := s.gate.ApproveBatch(data)
		if err != nil || approved.StrippedCount() != 0 || string(approved.Bytes()) != string(data) {
			http.Error(w, `{"error":"POLICY_REJECTED"}`, 400)
			return
		}
		var batch telemetry.Batch
		if json.Unmarshal(approved.Bytes(), &batch) != nil {
			http.Error(w, `{"error":"BATCH_INVALID"}`, 400)
			return
		}
		ack, code := s.accept(batch, approved.SHA256(), time.Now())
		if code != 200 {
			http.Error(w, `{"error":"INGEST_REJECTED"}`, code)
			return
		}
		writeJSON(w, ack)
	case "/api/records":
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		// Internal protocol test hook. The production workspace gateway handles
		// this route itself and never forwards user-selected tenant parameters.
		writeJSON(w, s.Snapshot(r.URL.Query().Get("tenant"), r.URL.Query().Get("collector_id")))
	default:
		w.WriteHeader(404)
	}
}

func (s *Server) accept(b telemetry.Batch, hash string, now time.Time) (telemetry.Ack, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.Region != s.region {
		return telemetry.Ack{}, 421
	}
	n := s.nodes[b.CollectorID]
	if n == nil {
		if len(s.nodes) >= 10000 {
			return telemetry.Ack{}, 503
		}
		n = &node{tenant: b.Tenant, region: b.Region, seen: map[uint64]string{}}
	} else if n.tenant != b.Tenant || n.region != b.Region {
		return telemetry.Ack{}, 403
	}
	duplicate := false
	// Old committed sequences may have outlived the retained hash window after
	// PostgreSQL hydration. ACK without mutating telemetry or accepting new data.
	if _, known := n.seen[b.Seq]; !known && b.Seq <= n.contiguous {
		n.duplicates++
		n.lastSeen = collect.Timestamp(now)
		return telemetry.Ack{Seq: b.Seq, HighestContiguous: n.contiguous, Gaps: gaps(n), PayloadSHA256: hash, ServerID: s.epoch, ReceivedAt: collect.Timestamp(now), Duplicate: true, DuplicatesRejected: n.duplicates}, 200
	}
	if prev, ok := n.seen[b.Seq]; ok {
		if prev != hash {
			return telemetry.Ack{}, 409
		}
		duplicate = true
		n.duplicates++
	} else {
		if b.Seq > n.contiguous+4096 {
			return telemetry.Ack{}, 429
		}
		watermark := n.watermark
		for _, r := range b.Records {
			at, err := time.Parse(collect.TimeLayout, r.CollectedAt)
			if err != nil {
				return telemetry.Ack{}, 400
			}
			if at.After(watermark) {
				watermark = at
			}
		}
		for _, r := range b.Records {
			at, err := time.Parse(collect.TimeLayout, r.CollectedAt)
			if err != nil {
				return telemetry.Ack{}, 400
			}
			if at.Before(watermark.Add(-10 * time.Minute)) {
				return telemetry.Ack{}, 422
			}
		}
		// All checks precede mutation: a rejected batch changes no watermark/data.
		n.watermark = watermark
		n.seen[b.Seq] = hash
		if b.Seq > n.max {
			n.max = b.Seq
		}
		for {
			if _, ok := n.seen[n.contiguous+1]; !ok {
				break
			}
			n.contiguous++
			// Bound hash history without stopping long-running collectors.
			if n.contiguous > 4096 {
				delete(n.seen, n.contiguous-4096)
			}
		}
		for _, record := range b.Records {
			entry := StoredRecord{Seq: b.Seq, BatchID: b.BatchID, ReceivedAt: collect.Timestamp(now), Record: record}
			if len(n.ring) < s.capacity {
				n.ring = append(n.ring, entry)
			} else {
				n.ring[n.next] = entry
				n.next = (n.next + 1) % s.capacity
				n.evicted++
			}
			n.accepted++
		}
	}
	n.lastSeen = collect.Timestamp(now)
	s.nodes[b.CollectorID] = n
	return telemetry.Ack{Seq: b.Seq, HighestContiguous: n.contiguous, Gaps: gaps(n), PayloadSHA256: hash, ServerID: s.epoch, ReceivedAt: n.lastSeen, Duplicate: duplicate, DuplicatesRejected: n.duplicates}, 200
}
func gaps(n *node) []uint64 {
	out := []uint64{}
	for seq := n.contiguous + 1; seq < n.max; seq++ {
		if _, ok := n.seen[seq]; !ok {
			out = append(out, seq)
		}
	}
	return out
}
func (s *Server) Snapshot(tenant, collector string) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := Snapshot{Records: []StoredRecord{}, Gaps: []uint64{}}
	n := s.nodes[collector]
	if n == nil || n.tenant != tenant {
		return result
	}
	// Deep-copy maps so callers cannot mutate stored data or race the receiver.
	data, err := json.Marshal(n.ring)
	if err != nil {
		return result
	}
	if json.Unmarshal(data, &result.Records) != nil {
		return result
	}
	sort.SliceStable(result.Records, func(i, j int) bool {
		a, b := result.Records[i], result.Records[j]
		if a.Record.CollectedAt == b.Record.CollectedAt {
			return a.Seq < b.Seq
		}
		return a.Record.CollectedAt < b.Record.CollectedAt
	})
	result.HighestContiguous = n.contiguous
	result.Gaps = gaps(n)
	result.DuplicatesRejected = n.duplicates
	result.RecordsAccepted = n.accepted
	result.RecordsEvicted = n.evicted
	result.LastSeen = n.lastSeen
	return result
}
func writeJSON(w http.ResponseWriter, value any) {
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return /* disconnected client; never log payload contents */
	}
}
