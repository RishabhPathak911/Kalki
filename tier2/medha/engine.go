package medha

import (
	"encoding/json"
	"fmt"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"math"
	"sort"
	"sync"
	"time"
)

type Signal struct {
	Metric    string  `json:"metric"`
	Observed  float64 `json:"observed"`
	Baseline  float64 `json:"baseline"`
	Deviation float64 `json:"deviation"`
	Weight    float64 `json:"weight"`
	Direction string  `json:"direction"`
}
type Explanation struct {
	ID              string   `json:"id"`
	Tenant          string   `json:"tenant"`
	Collector       string   `json:"collector_id"`
	Region          string   `json:"region"`
	Pattern         string   `json:"pattern"`
	Severity        string   `json:"severity"`
	Confidence      float64  `json:"confidence"`
	Summary         string   `json:"summary"`
	Signals         []Signal `json:"signals"`
	WhyFlagged      string   `json:"why_flagged"`
	WhyNotThreshold string   `json:"why_not_just_threshold"`
	Correlated      []string `json:"correlated_with"`
	Action          string   `json:"recommended_action"`
	WhyAction       string   `json:"why_this_action"`
	Steps           []string `json:"playbook_steps"`
	Detected        string   `json:"detected_at"`
	DetectionMS     int64    `json:"time_to_detect_ms"`
	Status          string   `json:"status"`
}
type Transition struct {
	ConnectionFrom string `json:"connection_from"`
	ConnectionTo   string `json:"connection_to"`
	From           string `json:"from"`
	To             string `json:"to"`
	Reason         string `json:"reason"`
	At             string `json:"at"`
}
type Counters struct {
	Suppressed  uint64 `json:"suppressed_alerts"`
	FalseAlarms uint64 `json:"false_alarms_suppressed"`
}
type Node struct {
	BatchesAccepted uint64                     `json:"batches_accepted"`
	RecordsAccepted uint64                     `json:"records_accepted"`
	LatestCollected string                     `json:"latest_collected_at"`
	DataAge         float64                    `json:"data_age_s"`
	Tenant          string                     `json:"tenant"`
	Collector       string                     `json:"collector_id"`
	Region          string                     `json:"region"`
	Health          string                     `json:"health"`
	Connection      string                     `json:"connection_state"`
	LastSeen        string                     `json:"last_seen_at"`
	Age             float64                    `json:"last_seen_age_s"`
	Metrics         map[string]telemetry.Stats `json:"metrics"`
	Sources         []string                   `json:"source_badges"`
	History         []Transition               `json:"state_transitions"`
	Duplicates      uint64                     `json:"duplicates_rejected"`
	WALDepth        float64                    `json:"wal_depth"`
	Counters        Counters                   `json:"suppressed_counters"`
}
type baseline struct {
	bad, critical          bool
	history                []float64
	ewma, previous         float64
	last                   time.Time
	consecutive, clear, up int
	active                 bool
	first                  time.Time
}
type node struct {
	Node
	seen                        time.Time
	latest                      time.Time
	sourceAt                    map[string]time.Time
	baselines                   map[string]*baseline
	signals                     map[string]Signal
	signalAt                    map[string]time.Time
	minor, persistent, critical bool
	riskAt                      time.Time
	anomalyAt                   time.Time
	quiet                       int
	lastResolved                time.Time
	incident                    string
}
type Event struct {
	ID     uint64          `json:"id"`
	Kind   string          `json:"kind"`
	Data   json.RawMessage `json:"data"`
	tenant string
}
type Engine struct {
	mu          sync.Mutex
	Config      Config
	Books       Books
	nodes       map[string]*node
	incidents   map[string][]Explanation
	counters    map[string]Counters
	events      map[string][]Event
	next        uint64
	subscribers map[string]map[chan struct{}]struct{}
}

func New(c Config, b Books) *Engine {
	return &Engine{Config: c, Books: b, nodes: map[string]*node{}, incidents: map[string][]Explanation{}, counters: map[string]Counters{}, events: map[string][]Event{}}
}
func clone[T any](v T) T       { b, _ := json.Marshal(v); var out T; _ = json.Unmarshal(b, &out); return out }
func stamp(t time.Time) string { return collect.Timestamp(t) }
func (e *Engine) event(tenant, kind string, v any) {
	e.next++
	data, _ := json.Marshal(v)
	list := append(e.events[tenant], Event{e.next, kind, data, tenant})
	if len(list) > 256 {
		list = list[len(list)-256:]
	}
	e.events[tenant] = list
	e.notify(tenant)
}
func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	a := append([]float64{}, v...)
	sort.Float64s(a)
	n := len(a)
	if n%2 == 0 {
		return (a[n/2-1] + a[n/2]) / 2
	}
	return a[n/2]
}
func (e *Engine) suppress(n *node, falseAlarm bool) {
	c := e.counters[n.Tenant]
	c.Suppressed++
	n.Counters.Suppressed++
	if falseAlarm {
		c.FalseAlarms++
		n.Counters.FalseAlarms++
	}
	e.counters[n.Tenant] = c
}
func (e *Engine) metric(n *node, source, name string, value float64, at time.Time, historical bool) {
	if name == "wal_depth" {
		return
	}
	key := source + ":" + name
	b := n.baselines[key]
	if b == nil {
		b = &baseline{ewma: value, previous: value}
		n.baselines[key] = b
	}
	med := median(b.history)
	if len(b.history) == 0 {
		med = value
	}
	devs := make([]float64, len(b.history))
	for i, v := range b.history {
		devs[i] = math.Abs(v - med)
	}
	scale := math.Max(1.4826*median(devs), math.Max(math.Abs(med)*.05, .1))
	z := (value - med) / scale
	rate := 0.0
	if !b.last.IsZero() && at.After(b.last) {
		rate = (value - b.previous) / at.Sub(b.last).Seconds()
	}
	warmed := len(b.history) >= e.Config.Warmup
	adaptive := warmed && z >= e.Config.RaiseZ && value > b.ewma+scale*2
	threshold := e.Config.threshold(name)
	if warmed && threshold.MultipleWarn > 1 && b.ewma > 0 && value >= b.ewma*threshold.MultipleWarn {
		adaptive = true
	}
	warn := threshold.Warn > 0 && value >= threshold.Warn
	critical := threshold.Critical > 0 && value >= threshold.Critical
	if warmed && threshold.MultipleCritical > 1 && b.ewma > 0 && value >= b.ewma*threshold.MultipleCritical {
		critical = true
	}
	if name == "service_up" {
		warn = value < 1
		critical = value == 0
	}
	if name == "wal_buffer_full" {
		warn = value == 1
		critical = warn
	}
	trend := warmed && (name == "mem_pct" || name == "disk_pct") && rate >= .02 && b.up >= e.Config.Persistence-1 && value-b.history[0] >= 2
	bad := warn || adaptive || trend
	clear := (!warn && (!warmed || z < e.Config.ClearZ))
	if threshold.Warn > 0 {
		clear = clear && value < threshold.Warn*e.Config.ClearRatio
	}
	if rate > 0 {
		b.up++
	} else {
		b.up = 0
	}
	if bad && !historical {
		if b.consecutive == 0 {
			b.first = at
		}
		b.consecutive++
		b.clear = 0
		if b.consecutive >= e.Config.Persistence || critical {
			b.active = true
		} else {
			e.suppress(n, false)
		}
	} else if clear || historical {
		if b.consecutive > 0 && !b.active {
			e.suppress(n, true)
		}
		b.consecutive = 0
		b.clear++
		if b.clear >= e.Config.Persistence {
			b.active = false
		}
	}
	if !historical && (bad || b.active) {
		direction := "up"
		if name == "service_up" {
			direction = "down"
		}
		weight := .45
		if adaptive {
			weight = .55
		}
		if critical {
			weight = .85
		}
		n.signals[name] = Signal{name, value, b.ewma, z, weight, direction}
		n.signalAt[name] = at
		n.minor = true
		if b.active {
			n.persistent = true
		}
		if critical {
			n.critical = true
		}
		n.anomalyAt = at
		// Rate and fleet context supplement, rather than replace, the local baseline.
		if b.up >= e.Config.Persistence && rate > 0 && (name == "mem_pct" || name == "disk_pct") {
			n.signals[name+"_rate"] = Signal{name + "_rate", rate, 0, rate, .3, "up"}
			n.signalAt[name+"_rate"] = at
		}
		peerValues := []float64{}
		for _, peer := range e.nodes {
			if peer != n && peer.Tenant == n.Tenant && math.Abs(at.Sub(peer.latest).Seconds()) <= 30 {
				if v, ok := peer.Metrics[name]; ok {
					peerValues = append(peerValues, v.Avg)
				}
			}
		}
		if len(peerValues) > 0 {
			peerMedian := median(peerValues)
			gap := (value - peerMedian) / math.Max(math.Abs(peerMedian)*.1, 1)
			if math.Abs(gap) >= 3 {
				n.signals[name+"_peer"] = Signal{name + "_peer", value, peerMedian, gap, .2, direction}
				n.signalAt[name+"_peer"] = at
			}
		}
	}
	if !historical {
		b.bad, b.critical = bad, critical
	}
	// Freeze anomaly baselines so sustained faults do not train themselves away.
	if !bad || !warmed || historical {
		b.ewma = e.Config.Alpha*value + (1-e.Config.Alpha)*b.ewma
		b.history = append(b.history, value)
		if len(b.history) > 120 {
			b.history = b.history[1:]
		}
	}
	b.previous = value
	b.last = at
}
func (e *Engine) Observe(batch telemetry.Batch, now time.Time, duplicate bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := batch.Tenant + ":" + batch.CollectorID
	n := e.nodes[key]
	created := n == nil
	if n == nil {
		n = &node{Node: Node{Tenant: batch.Tenant, Collector: batch.CollectorID, Region: batch.Region, Health: "Excellent", Connection: "ONLINE", Metrics: map[string]telemetry.Stats{}, Sources: []string{}, History: []Transition{}}, baselines: map[string]*baseline{}, signals: map[string]Signal{}, signalAt: map[string]time.Time{}, sourceAt: map[string]time.Time{}}
		e.nodes[key] = n
	}
	n.seen = now
	n.LastSeen = stamp(now)
	if duplicate {
		n.Duplicates++
		e.health(n, now)
		return
	}
	n.BatchesAccepted++
	n.RecordsAccepted += uint64(len(batch.Records))
	for _, r := range batch.Records {
		at, err := time.Parse(collect.TimeLayout, r.CollectedAt)
		if err != nil {
			continue
		}
		if !at.After(n.sourceAt[r.Source]) {
			continue
		}
		n.sourceAt[r.Source] = at
		historical := now.Sub(at) > time.Duration(e.Config.Heartbeat.Stale*float64(time.Second))
		// A persisted full-buffer status remains actionable after an outage.
		// It is cleared by a subsequent normal batch, not discarded as old metrics.
		if r.Metrics["wal_buffer_full"].Avg == 1 {
			historical = false
		}
		n.minor = false
		n.persistent = false
		n.critical = false
		names := make([]string, 0, len(r.Metrics))
		for name := range r.Metrics {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			v := r.Metrics[name]
			e.metric(n, r.Source, name, v.Avg, at, historical)
			if !at.Before(n.latest) {
				n.Metrics[name] = v
			}
			if name == "wal_depth" {
				n.WALDepth = v.Avg
			}
		}
		if !at.Before(n.latest) {
			n.latest = at
			n.LatestCollected = stamp(at)
		}
		found := false
		for _, source := range n.Sources {
			if source == r.Source {
				found = true
			}
		}
		if !found {
			n.Sources = append(n.Sources, r.Source)
		}
		for _, risk := range r.RiskEvents {
			if risk.Type == "bruteforce_suspected" && !historical {
				n.riskAt = at
				n.signals["bruteforce_suspected"] = Signal{"bruteforce_suspected", float64(risk.Count), 0, float64(risk.Count), .4, "up"}
				n.signalAt["bruteforce_suspected"] = at
			}
		}
		if !historical {
			for _, series := range n.baselines {
				n.minor = n.minor || series.bad || series.active
				n.persistent = n.persistent || series.active
				n.critical = n.critical || series.critical
			}
			e.correlate(n, now, at)
		}
	}
	e.health(n, now)
	if created {
		e.event(n.Tenant, "health", n.Node)
	}
	e.event(n.Tenant, "telemetry", batch)
}
func (e *Engine) correlate(n *node, now, at time.Time) {
	for name, t := range n.signalAt {
		if at.Sub(t) > 30*time.Second {
			delete(n.signals, name)
			delete(n.signalAt, name)
		}
	}
	has := func(k string) bool { _, ok := n.signals[k]; return ok }
	pattern := ""
	if has("wal_buffer_full") && n.signals["wal_buffer_full"].Observed == 1 {
		pattern = "buffer_full"
	} else if has("cpu_pct") && has("bandwidth_mbps") && at.Sub(n.riskAt) <= 30*time.Second {
		pattern = "possible_attack_exfiltration"
	} else if has("latency_ms") && has("packet_loss_pct") {
		pattern = "network_path_issue"
	} else if has("mem_pct_rate") {
		pattern = "memory_leak"
	} else if has("disk_pct_rate") {
		pattern = "capacity_risk"
	} else if has("cpu_pct") && !has("packet_loss_pct") && !has("latency_ms") {
		pattern = "local_workload"
	}
	peers := []string{}
	if n.minor {
		for _, p := range e.nodes {
			if p != n && p.Tenant == n.Tenant && !p.anomalyAt.IsZero() && math.Abs(at.Sub(p.anomalyAt).Seconds()) <= 30 {
				peers = append(peers, p.Collector)
			}
		}
	}
	sort.Strings(peers)
	if len(peers) > 0 && (n.persistent || n.critical) {
		pattern = "shared_dependency"
	}
	if pattern != "" && (n.persistent || n.critical) {
		e.raise(n, pattern, now, at, peers)
		n.quiet = 0
	} else if !n.minor {
		n.quiet++
		if n.quiet >= e.Config.Persistence {
			e.resolve(n, now)
		}
	}
}
func (e *Engine) raise(n *node, pattern string, now, at time.Time, peers []string) {
	list := e.incidents[n.Tenant]
	existing := -1
	for i := range list {
		if list[i].ID == n.incident && list[i].Status == "ACTIVE" {
			existing = i
		}
	}
	if existing < 0 && !n.lastResolved.IsZero() && now.Sub(n.lastResolved) < time.Duration(e.Config.Cooldown*float64(time.Second)) && pattern != "collector_down" && pattern != "buffer_full" {
		e.suppress(n, true)
		return
	}
	signals := []Signal{}
	score := 0.0
	names := []string{}
	for k := range n.signals {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		s := n.signals[k]
		signals = append(signals, s)
		score += s.Weight
	}
	if len(signals) == 0 {
		return
	}
	confidence := math.Min(.99, score)
	book := Playbook{}
	for _, b := range e.Books.Playbooks {
		if b.Pattern == pattern {
			book = b
			break
		}
	}
	if book.Action == "" {
		return
	}
	severity := "warning"
	if n.critical || confidence >= .8 {
		severity = "critical"
	}
	if peers == nil {
		peers = []string{}
	}
	ms := now.Sub(at).Milliseconds()
	if pattern != "collector_down" {
		for _, series := range n.baselines {
			if series.active && !series.first.IsZero() && series.first.Before(at) {
				at = series.first
			}
		}
		ms = now.Sub(at).Milliseconds()
	}
	if ms < 0 {
		ms = 0
	}
	item := Explanation{Tenant: n.Tenant, Collector: n.Collector, Region: n.Region, Pattern: pattern, Severity: severity, Confidence: confidence, Summary: book.Why, Signals: signals, WhyFlagged: fmt.Sprintf("%s Evidence persisted for %d windows or crossed a critical boundary.", book.Why, e.Config.Persistence), WhyNotThreshold: "Uses pre-observation EWMA, median/MAD deviation, rate of change, tenant peers and a 30-second evidence window; baseline alerts require warm-up.", Correlated: peers, Action: book.Action, WhyAction: book.Why, Steps: append([]string{}, book.Steps...), Detected: stamp(now), DetectionMS: ms, Status: "ACTIVE"}
	if existing >= 0 {
		item.ID = list[existing].ID
		item.Detected = list[existing].Detected
		item.DetectionMS = list[existing].DetectionMS
		list[existing] = item
		e.suppress(n, false)
	} else {
		e.next++
		item.ID = fmt.Sprintf("incident-%d", e.next)
		list = append(list, item)
		n.incident = item.ID
	}
	if ValidateExplanation(item) != nil {
		return
	}
	if len(list) > 256 {
		list = list[len(list)-256:]
	}
	e.incidents[n.Tenant] = list
	e.event(n.Tenant, "incident", item)
}
func (e *Engine) resolve(n *node, now time.Time) {
	list := e.incidents[n.Tenant]
	for i := range list {
		if list[i].ID == n.incident && list[i].Status == "ACTIVE" {
			list[i].Status = "RESOLVED"
			n.lastResolved = now
			e.event(n.Tenant, "incident", list[i])
		}
	}
	e.incidents[n.Tenant] = list
	n.incident = ""
}
func (e *Engine) health(n *node, now time.Time) {
	age := math.Max(0, now.Sub(n.seen).Seconds())
	connection := "ONLINE"
	state := "Excellent"
	reason := "fresh heartbeat and normal metrics"
	if n.minor {
		state = "Good"
		reason = "minor deviation below persistence"
	}
	if n.persistent {
		state = "Fair"
		reason = "persistent deviation"
	}
	if n.critical {
		state = "Poor"
		reason = "critical metric"
	}
	for _, i := range e.incidents[n.Tenant] {
		if i.ID == n.incident && i.Status == "ACTIVE" {
			if state == "Excellent" || state == "Good" {
				state = "Fair"
			}
			if i.Confidence >= .8 || i.Pattern == "network_path_issue" || i.Pattern == "possible_attack_exfiltration" || i.Pattern == "shared_dependency" {
				state = "Poor"
				reason = "correlated or high-confidence incident"
			}
		}
	}
	if age > e.Config.Heartbeat.Stale {
		connection = "STALE"
		state = "Poor"
		reason = "heartbeat stale"
	}
	if age >= e.Config.Heartbeat.Offline {
		connection = "OFFLINE"
		state = "Poor"
		reason = "heartbeat offline"
		if n.Connection != "OFFLINE" {
			n.signals = map[string]Signal{"heartbeat_age_s": {"heartbeat_age_s", age, 5, age - 5, .95, "up"}}
			e.raise(n, "collector_down", now, n.seen.Add(time.Duration(e.Config.Heartbeat.Offline*float64(time.Second))), nil)
		}
	}
	if connection == "ONLINE" && n.Connection == "OFFLINE" {
		for _, i := range e.incidents[n.Tenant] {
			if i.ID == n.incident && i.Pattern == "collector_down" {
				e.resolve(n, now)
				state = "Excellent"
				if n.minor {
					state = "Good"
				}
				if n.persistent {
					state = "Fair"
				}
				if n.critical {
					state = "Poor"
				}
			}
		}
	}
	if state != n.Health || connection != n.Connection {
		n.History = append(n.History, Transition{From: n.Health, To: state, ConnectionFrom: n.Connection, ConnectionTo: connection, Reason: reason, At: stamp(now)})
		if len(n.History) > 128 {
			n.History = n.History[len(n.History)-128:]
		}
		e.event(n.Tenant, "health", map[string]any{"collector_id": n.Collector, "health": state, "connection_state": connection, "transition": n.History[len(n.History)-1]})
	}
	n.Health = state
	n.Connection = connection
	n.Age = age
	if !n.latest.IsZero() {
		n.DataAge = math.Max(0, now.Sub(n.latest).Seconds())
	}
}
func (e *Engine) Tick(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, n := range e.nodes {
		e.health(n, now)
	}
}
func (e *Engine) Fleet(tenant string, now time.Time) ([]Node, Counters) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := []Node{}
	for _, n := range e.nodes {
		if n.Tenant == tenant {
			e.health(n, now)
			out = append(out, clone(n.Node))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Collector < out[j].Collector })
	return out, e.counters[tenant]
}
func (e *Engine) Incidents(tenant string) []Explanation {
	e.mu.Lock()
	defer e.mu.Unlock()
	return clone(append([]Explanation{}, e.incidents[tenant]...))
}
func (e *Engine) Events(tenant string, after uint64) ([]Event, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	list := e.events[tenant]
	out := []Event{}
	gap := len(list) > 0 && after != 0 && after < list[0].ID
	// Event IDs are process-local; a reconnect may carry the previous boot's ID.
	if after > e.next {
		after = 0
		gap = true
	}
	for _, v := range list {
		if v.ID > after {
			out = append(out, clone(v))
		}
	}
	return out, gap
}
func (e *Engine) Publish(tenant, kind string, v any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.event(tenant, kind, v)
}
func (e *Engine) RemoveNode(tenant, collector string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := tenant + ":" + collector
	if n, ok := e.nodes[key]; ok {
		if n.incident != "" {
			e.resolve(n, time.Now())
		}
		delete(e.nodes, key)
		e.event(tenant, "node_removed", map[string]string{
			"tenant":       tenant,
			"collector_id": collector,
		})
	}
}
