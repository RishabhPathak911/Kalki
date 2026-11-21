package ingest

import (
	"encoding/json"
	"time"
)

// Checkpoint retains sequence progress even after chart retention prunes rows.
type Checkpoint struct {
	Tenant, Region, Collector, LastSeen   string
	Contiguous, Max, Accepted, Duplicates uint64
	Watermark                             time.Time
	Seen                                  map[uint64]string
}

func (s *Server) Checkpoint(tenant, collector string) Checkpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.nodes[collector]
	if n == nil || n.tenant != tenant {
		return Checkpoint{}
	}
	v := Checkpoint{Tenant: tenant, Region: n.region, Collector: collector, LastSeen: n.lastSeen, Contiguous: n.contiguous, Max: n.max, Accepted: n.accepted, Duplicates: n.duplicates, Watermark: n.watermark, Seen: map[uint64]string{}}
	for seq, h := range n.seen {
		if seq > n.contiguous || n.contiguous-seq < 4096 {
			v.Seen[seq] = h
		}
	}
	return v
}
func (s *Server) Restore(v Checkpoint, records []StoredRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.Region != s.region {
		return
	}
	n := &node{tenant: v.Tenant, region: v.Region, lastSeen: v.LastSeen, contiguous: v.Contiguous, max: v.Max, accepted: v.Accepted, duplicates: v.Duplicates, watermark: v.Watermark, seen: v.Seen}
	if n.seen == nil {
		n.seen = map[uint64]string{}
	}
	for _, r := range records {
		if r.Record.Tenant == v.Tenant && r.Record.CollectorID == v.Collector && r.Record.Region == v.Region {
			n.ring = append(n.ring, r)
		}
	}
	if len(n.ring) > s.capacity {
		n.ring = n.ring[len(n.ring)-s.capacity:]
	}
	raw, _ := json.Marshal(n.ring)
	json.Unmarshal(raw, &n.ring)
	s.nodes[v.Collector] = n
}
