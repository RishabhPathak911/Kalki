// Package workspace is the authenticated boundary around regional telemetry.
// Tokens and sessions are stored only as hashes. All state is local and volatile.
package workspace

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
	"kalki/tier2/ingest"
	"kalki/tier2/medha"
)

type identity struct{ Tenant, Region, Collector string }
type session struct {
	identity
	Expires time.Time
}
type user struct {
	identity
	Password [32]byte
}
type Entry struct {
	Index    int    `json:"index"`
	At       string `json:"at"`
	Event    string `json:"event"`
	Status   int    `json:"status"`
	Previous string `json:"previous_sha256"`
	Hash     string `json:"sha256"`
}
type chain struct {
	Entries []Entry
	Head    string
	Count   int
}

func digest(e Entry) string {
	e.Hash = ""
	b, _ := json.Marshal(e)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (c *chain) append(event string, status int) {
	e := Entry{Index: c.Count + 1, At: collect.Timestamp(time.Now()), Event: event, Status: status, Previous: c.Head}
	e.Hash = digest(e)
	c.Entries = append(c.Entries, e)
	c.Head = e.Hash
	c.Count++
}
func (c *chain) verify() bool {
	prev := ""
	for i, e := range c.Entries {
		if e.Index != i+1 || e.Previous != prev || digest(e) != e.Hash {
			return false
		}
		prev = e.Hash
	}
	return len(c.Entries) == c.Count && prev == c.Head
}

// Authority is shared by the two regional listeners in the same process.
// Keep it outside the HTTP mux: no request can select its own tenant scope.
type Authority struct {
	engine        *medha.Engine
	remediations  map[string][]Remediation
	mu            sync.Mutex
	users         map[string]user
	sessions      map[[32]byte]session
	tokens        map[[32]byte]identity
	collectors    map[string][32]byte
	audits        map[string]*chain
	regions       map[string]*Gateway
	loginWindow   time.Time
	loginAttempts int
}

func NewAuthority(alphaPassword, betaPassword string) *Authority {
	a := &Authority{users: map[string]user{}, sessions: map[[32]byte]session{}, tokens: map[[32]byte]identity{}, collectors: map[string][32]byte{}, audits: map[string]*chain{}, regions: map[string]*Gateway{}}
	a.users["tenant_alpha"] = user{identity: identity{Tenant: "tenant_alpha", Region: "ap-south-1"}, Password: sha256.Sum256([]byte(alphaPassword))}
	a.users["tenant_beta"] = user{identity: identity{Tenant: "tenant_beta", Region: "eu-central-1"}, Password: sha256.Sum256([]byte(betaPassword))}
	a.remediations = map[string][]Remediation{}
	return a
}
func opaque() (string, error) {
	var b [32]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}
func (a *Authority) audit(tenant, event string, status int) {
	c := a.audits[tenant]
	if c == nil {
		c = &chain{}
		a.audits[tenant] = c
	}
	c.append(event, status)
}
func bearer(r *http.Request) [32]byte {
	v := r.Header.Get("Authorization")
	if !strings.HasPrefix(v, "Bearer ") || len(v) != 71 {
		return [32]byte{}
	}
	return sha256.Sum256([]byte(v[7:]))
}
func respond(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int) {
	respond(w, status, map[string]string{"error": http.StatusText(status)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}

type Gateway struct {
	authority                *Authority
	receiver                 *ingest.Server
	gate                     *policy.Gate
	region, origin, releases string
}

func New(a *Authority, receiver *ingest.Server, gate *policy.Gate, region, origin, releases string) (*Gateway, error) {
	u, err := url.Parse(origin)
	if a == nil || receiver == nil || gate == nil || err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || (region != "ap-south-1" && region != "eu-central-1") {
		return nil, fmt.Errorf("invalid workspace configuration")
	}
	g := &Gateway{a, receiver, gate, region, origin, releases}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.regions[region] != nil {
		return nil, fmt.Errorf("duplicate regional gateway")
	}
	a.regions[region] = g
	a.audit("", "config_start", 200)
	return g, nil
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.TLS == nil {
		failure(w, 426)
		return
	}
	// Credential URLs are never supported, including on public installer paths.
	if r.URL.RawQuery != "" && r.URL.Path != "/api/records" {
		failure(w, 400)
		return
	}
	if r.URL.Path == "/install.sh" || strings.HasPrefix(r.URL.Path, "/downloads/") {
		g.download(w, r)
		return
	}
	a := g.authority
	if r.URL.Path == "/api/ai/analyze-incident" || r.URL.Path == "/api/ai/ask" {
		g.ai(w, r)
		return
	}
	if r.URL.Path == "/api/stream" {
		g.stream(w, r)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.URL.Path == "/api/auth/login" {
		g.login(w, r)
		return
	}
	if r.URL.Path == "/api/session" || r.URL.Path == "/api/ingest" {
		g.collect(w, r)
		return
	}
	key := bearer(r)
	s, ok := a.sessions[key]
	if !ok || !time.Now().Before(s.Expires) {
		delete(a.sessions, key)
		a.audit("", "auth_fail", 401)
		failure(w, 401)
		return
	}
	a.audit(s.Tenant, "auth_success", 200)
	if r.URL.Path == "/api/auth/logout" {
		if r.Method != "POST" {
			failure(w, 405)
			return
		}
		delete(a.sessions, key)
		a.audit(s.Tenant, "logout", 200)
		respond(w, 200, map[string]bool{"logged_out": true})
		return
	}
	if r.URL.Path == "/api/collectors/tokens" {
		g.mint(w, r, s.identity)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/remediations") {
		g.remediate(w, r, s.identity)
		return
	}
	if r.Method != "GET" {
		a.audit(s.Tenant, "dashboard_reject", 405)
		failure(w, 405)
		return
	}
	switch r.URL.Path {
	case "/api/validation":
		a.audit(s.Tenant, "dashboard_validation", 200)
		validation(w)
	case "/api/fleet", "/api/incidents", "/api/thresholds":
		if a.engine == nil {
			failure(w, 503)
			return
		}
		a.audit(s.Tenant, "dashboard_medha", 200)
		switch r.URL.Path {
		case "/api/fleet":
			nodes, counters := a.engine.Fleet(s.Tenant, time.Now())
			respond(w, 200, map[string]any{"nodes": nodes, "counters": counters})
		case "/api/incidents":
			respond(w, 200, map[string]any{"incidents": a.engine.Incidents(s.Tenant)})
		case "/api/thresholds":
			respond(w, 200, map[string]any{"thresholds": a.engine.Config, "playbooks": a.engine.Books.Playbooks})
		}
	case "/api/records":
		// Reject tenant/region selectors, even if they happen to equal the session.
		q := r.URL.Query()
		for k := range q {
			if k != "collector_id" {
				a.audit(s.Tenant, "dashboard_reject", 403)
				failure(w, 403)
				return
			}
		}
		home := a.regions[s.Region]
		if home == nil {
			failure(w, 503)
			return
		}
		a.audit(s.Tenant, "dashboard_records", 200)
		respond(w, 200, home.receiver.Snapshot(s.Tenant, q.Get("collector_id")))
	case "/api/global":
		// Only metadata is projected: never metric values or raw logs.
		type metadata struct {
			Collector string `json:"collector_id"`
			Region    string `json:"region"`
			LastSeen  string `json:"last_seen"`
			Records   uint64 `json:"records_accepted"`
		}
		result := []metadata{}
		for _, id := range a.tokens {
			if id.Tenant != s.Tenant {
				continue
			}
			home := a.regions[id.Region]
			if home == nil {
				continue
			}
			snap := home.receiver.Snapshot(s.Tenant, id.Collector)
			result = append(result, metadata{id.Collector, id.Region, snap.LastSeen, snap.RecordsAccepted})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Collector < result[j].Collector })
		a.audit(s.Tenant, "dashboard_global", 200)
		respond(w, 200, map[string]any{"tenant": s.Tenant, "collectors": result})
	case "/api/audit", "/api/audit/verify":
		a.audit(s.Tenant, "dashboard_audit", 200)
		c := a.audits[s.Tenant]
		if r.URL.Path == "/api/audit/verify" {
			respond(w, 200, map[string]any{"valid": c.verify(), "entries": c.Count, "head_sha256": c.Head})
		} else {
			respond(w, 200, map[string]any{"entries": c.Entries, "head_sha256": c.Head})
		}
	default:
		// Future SSE/remediation routes must be added inside this session boundary.
		failure(w, 404)
	}
}
func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	a := g.authority
	if r.Method != "POST" {
		failure(w, 405)
		return
	}
	if time.Since(a.loginWindow) > time.Minute {
		a.loginWindow = time.Now()
		a.loginAttempts = 0
	}
	a.loginAttempts++
	if a.loginAttempts > 60 {
		a.audit("", "auth_rate_limit", 429)
		failure(w, 429)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		a.audit("", "auth_fail", 400)
		failure(w, 400)
		return
	}
	u, ok := a.users[req.Username]
	sum := sha256.Sum256([]byte(req.Password))
	if subtle.ConstantTimeCompare(sum[:], u.Password[:]) != 1 || !ok {
		a.audit("", "auth_fail", 401)
		failure(w, 401)
		return
	}
	for key, s := range a.sessions {
		if time.Now().After(s.Expires) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= 10000 {
		failure(w, 503)
		return
	}
	token, err := opaque()
	if err != nil {
		failure(w, 500)
		return
	}
	expires := time.Now().Add(8 * time.Hour)
	a.sessions[sha256.Sum256([]byte(token))] = session{u.identity, expires}
	a.audit(u.Tenant, "login_success", 200)
	respond(w, 200, map[string]any{"token": token, "tenant": u.Tenant, "region": u.Region, "expires_at": collect.Timestamp(expires)})
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

var collectorPattern = regexp.MustCompile(`^host_[0-9a-f]{8}$`)

func (g *Gateway) mint(w http.ResponseWriter, r *http.Request, id identity) {
	a := g.authority
	if r.Method != "POST" {
		failure(w, 405)
		return
	}
	var req struct {
		Collector string `json:"collector_id"`
		Rotate    bool   `json:"rotate"`
	}
	if !decode(w, r, &req) {
		failure(w, 400)
		return
	}
	if req.Collector == "" && !req.Rotate {
		token, err := opaque()
		if err != nil {
			failure(w, 500)
			return
		}
		req.Collector = "host_" + token[:8]
	}
	if !collectorPattern.MatchString(req.Collector) {
		failure(w, 400)
		return
	}
	slot := id.Tenant + ":" + req.Collector
	old, exists := a.collectors[slot]
	if req.Rotate != exists {
		failure(w, 409)
		return
	}
	// Collector IDs are globally unique because the receiver deduplicates by ID.
	for _, v := range a.tokens {
		if v.Collector == req.Collector && v.Tenant != id.Tenant {
			failure(w, 409)
			return
		}
	}
	if len(a.tokens) >= 10000 && !exists {
		failure(w, 503)
		return
	}
	home := a.regions[id.Region]
	if home == nil {
		failure(w, 503)
		return
	}
	token, err := opaque()
	if err != nil {
		failure(w, 500)
		return
	}
	id.Collector = req.Collector
	key := sha256.Sum256([]byte(token))
	delete(a.tokens, old)
	a.tokens[key] = id
	a.collectors[slot] = key
	event := "token_mint"
	if req.Rotate {
		event = "token_rotate"
	}
	a.audit(id.Tenant, event, 200)
	flags := []string{"--tenant=" + id.Tenant, "--region=" + id.Region, "--collector-id=" + id.Collector, "--tier2-url=" + home.origin, "--token=" + token, "--ca-cert=ca.pem", "--probe-target=127.0.0.1:1", "--key-file=data/" + id.Tenant + ".key", "--wal-path=data/" + id.Collector + ".wal"}
	for i := range flags {
		flags[i] = quote(flags[i])
	}
	args := strings.Join(flags, " ")
	respond(w, 200, map[string]string{"token": token, "collector_id": id.Collector, "collector_command": "./collector " + args, "install_command": "curl --fail --silent --show-error --proto '=https' --cacert ca.pem " + quote(home.origin+"/install.sh") + " | sh -s -- --base-url " + quote(home.origin) + " --ca-cert ca.pem " + args})
}
func (g *Gateway) collect(w http.ResponseWriter, r *http.Request) {
	a := g.authority
	id, ok := a.tokens[bearer(r)]
	if !ok {
		a.audit("", "auth_fail", 401)
		failure(w, 401)
		return
	}
	a.audit(id.Tenant, "auth_success", 200)
	if id.Region != g.region {
		a.audit(id.Tenant, "wrong_region", 421)
		failure(w, 421)
		return
	}
	if (r.URL.Path == "/api/session" && r.Method != "GET") || (r.URL.Path == "/api/ingest" && r.Method != "POST") {
		failure(w, 405)
		return
	}
	var batch telemetry.Batch
	if r.URL.Path == "/api/ingest" {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024*1024))
		if err != nil {
			a.audit(id.Tenant, "ingest_reject", 413)
			failure(w, 413)
			return
		}
		p, err := g.gate.ApproveBatch(data)
		if err != nil || p.StrippedCount() != 0 || !bytes.Equal(p.Bytes(), data) {
			a.audit(id.Tenant, "allowlist_violation", 400)
			a.audit(id.Tenant, "ingest_reject", 400)
			failure(w, 400)
			return
		}
		_ = json.Unmarshal(data, &batch)
		if batch.Region != id.Region {
			a.audit(id.Tenant, "wrong_region", 421)
			failure(w, 421)
			return
		}
		if batch.Tenant != id.Tenant || batch.CollectorID != id.Collector {
			a.audit(id.Tenant, "ingest_reject", 403)
			failure(w, 403)
			return
		}
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(bytes.NewReader(data))
	}
	// Audit the status only; never copy request or response bodies into logs.
	rec := &statusWriter{ResponseWriter: w, code: http.StatusOK}
	g.receiver.ServeHTTP(rec, r)
	if r.URL.Path == "/api/ingest" {
		event := "ingest_accept"
		if rec.code != 200 {
			event = "ingest_reject"
		}
		a.audit(id.Tenant, event, rec.code)
		if rec.code == 200 && a.engine != nil {
			var ack telemetry.Ack
			if json.Unmarshal(rec.body.Bytes(), &ack) == nil {
				a.engine.Observe(batch, time.Now(), ack.Duplicate)
			}
		}
	}
}

type statusWriter struct {
	http.ResponseWriter
	code int
	body bytes.Buffer
}

func (w *statusWriter) Write(data []byte) (int, error) {
	_, _ = w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}
