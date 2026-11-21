package workspace

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"kalki/internal/telemetry"
	"kalki/tier2/store"
	"net/http"
	"time"
)

func hashText(key [32]byte) string { return hex.EncodeToString(key[:]) }
func parseHash(s string) ([32]byte, error) {
	var out [32]byte
	b, e := hex.DecodeString(s)
	if e != nil || len(b) != 32 {
		return out, fmt.Errorf("invalid stored credential hash")
	}
	copy(out[:], b)
	return out, nil
}

// AttachStore runs before listeners start; no goroutine may observe partial hydration.
func (a *Authority) AttachStore(db *store.Store, capacity int) error {
	if db == nil {
		return nil
	}
	data, e := db.Load(capacity)
	if e != nil {
		return e
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = map[[32]byte]session{}
	a.tokens = map[[32]byte]identity{}
	a.collectors = map[string][32]byte{}
	a.audits = map[string]*chain{}
	a.remediations = map[string][]Remediation{}
	for _, v := range data.Sessions {
		key, e := parseHash(v.Hash)
		if e != nil {
			return e
		}
		a.sessions[key] = session{identity: identity{Tenant: v.Tenant, Region: v.Region}, Expires: v.Expires}
	}
	for _, v := range data.Tokens {
		key, e := parseHash(v.Hash)
		if e != nil {
			return e
		}
		a.tokens[key] = identity{v.Tenant, v.Region, v.Collector}
		a.collectors[v.Tenant+":"+v.Collector] = key
	}
	for _, v := range data.Audit {
		c := a.audits[v.Tenant]
		if c == nil {
			c = &chain{}
			a.audits[v.Tenant] = c
		}
		c.Entries = append(c.Entries, Entry{v.Index, v.At, v.Event, v.Status, v.Previous, v.Hash})
		c.Count++
		c.Head = v.Hash
	}
	for _, c := range a.audits {
		if !c.verify() {
			return fmt.Errorf("persisted audit chain verification failed")
		}
	}
	for _, v := range data.Remediations {
		a.remediations[v.Tenant] = append(a.remediations[v.Tenant], Remediation{v.ID, v.Incident, v.Collector, v.Status, v.Reversible, v.TargetValid, v.Authorized, v.Action, v.Result, v.At})
	}
	for _, v := range data.Progress {
		if g := a.regions[v.Region]; g != nil {
			g.receiver.Restore(v, data.Records)
		}
	}
	if a.engine != nil {
		for _, v := range data.Records {
			r := v.Record
			at, e := time.Parse(time.RFC3339Nano, r.CollectedAt)
			if e != nil {
				return fmt.Errorf("invalid stored telemetry time")
			}
			a.engine.Observe(telemetry.Batch{Tenant: r.Tenant, Region: r.Region, CollectorID: r.CollectorID, Seq: v.Seq, BatchID: v.BatchID, Records: []telemetry.Record{r}}, at, false)
		}
		a.engine.RestoreIncidents(data.Incidents)
		for _, v := range data.Progress {
			at, e := time.Parse(time.RFC3339Nano, v.LastSeen)
			if e == nil {
				a.engine.RestoreLastSeen(v.Tenant, v.Collector, at)
			}
		}
		a.engine.Tick(time.Now())
	}
	a.db = db
	return nil
}
func (a *Authority) saveRemediation(tenant string, v Remediation) error {
	if a.db == nil {
		return nil
	}
	return a.db.SaveRemediation(store.Remediation{ID: v.ID, Tenant: tenant, Incident: v.Incident, Collector: v.Collector, Status: v.Status, Reversible: v.Reversible, TargetValid: v.TargetValid, Authorized: v.Authorized, Action: v.Action, Result: v.Result, At: v.At})
}

// An audit error must never turn into an apparently successful HTTP response.
type durableResponse struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *durableResponse) Header() http.Header { return w.header }
func (w *durableResponse) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *durableResponse) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	return w.body.Write(p)
}
func (a *Authority) durable(w http.ResponseWriter) (http.ResponseWriter, func()) {
	a.persistErr = nil
	if a.db == nil {
		return w, func() {}
	}
	b := &durableResponse{header: w.Header().Clone()}
	return b, func() {
		if a.persistErr != nil {
			failure(w, 503)
			return
		}
		for k, v := range b.header {
			w.Header()[k] = v
		}
		if b.code == 0 {
			b.code = 200
		}
		w.WriteHeader(b.code)
		w.Write(b.body.Bytes())
	}
}
