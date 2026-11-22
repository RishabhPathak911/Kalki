package send

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
	"kalki/tier1/wal"
)

type DeliveryCounters struct {
	Sent               uint64   `json:"batches_sent"`
	Acked              uint64   `json:"acked"`
	Retried            uint64   `json:"retried"`
	DuplicatesRejected uint64   `json:"duplicates_rejected"`
	Gaps               []uint64 `json:"gaps"`
	WALDepth           int      `json:"wal_depth"`
	LastSeq            uint64   `json:"last_seq"`
}
type Delivery struct {
	Demo             func(context.Context, string) (map[string]any, error)
	paused           bool
	duplicatePending bool
	Token            string // Set before Run; sent only in the HTTPS Authorization header.
	journal          *wal.Journal
	gate             *policy.Gate
	binding          wal.Binding
	base             string
	client           *http.Client
	proofs           ProofRing
	mu               sync.Mutex
	counts           DeliveryCounters
	lastQueued       time.Time
	wake             chan struct{}
	report           func(string)
	fullOnce         sync.Once
}

func NewDelivery(j *wal.Journal, g *policy.Gate, b wal.Binding, base string, client *http.Client, report func(string)) (*Delivery, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("delivery URL must be an HTTPS origin")
	}
	if j == nil || g == nil {
		return nil, fmt.Errorf("delivery configuration missing")
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	copyClient := *client
	if copyClient.Timeout == 0 {
		copyClient.Timeout = 3 * time.Second
	}
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	d := &Delivery{journal: j, gate: g, binding: b, base: strings.TrimRight(base, "/"), client: &copyClient, wake: make(chan struct{}, 1), report: report, lastQueued: time.Now()}
	for _, e := range j.Entries() {
		p, err := g.ApproveBatch(e.Bytes)
		if err != nil || p.SHA256() != e.Hash {
			return nil, fmt.Errorf("WAL replay blocked by policy")
		}
		var batch telemetry.Batch
		if json.Unmarshal(e.Bytes, &batch) != nil || batch.Tenant != b.Tenant || batch.CollectorID != b.CollectorID || batch.Region != b.Region || batch.Seq != e.Seq {
			return nil, fmt.Errorf("WAL replay identity mismatch")
		}
		if j.IsAcked(e.Seq) {
			d.proofs.Update(e, "ACKED", e.Hash)
		} else {
			d.proofs.Update(e, "BUFFERED", "")
		}
	}
	return d, nil
}
func (d *Delivery) note(message string) {
	if d.report != nil {
		d.report(message)
	}
}
func (d *Delivery) Counters() DeliveryCounters {
	d.mu.Lock()
	c := d.counts
	c.Gaps = append([]uint64{}, c.Gaps...)
	d.mu.Unlock()
	c.WALDepth = d.journal.Depth()
	c.LastSeq = d.journal.LastSeq()
	return c
}

func (d *Delivery) Enqueue(record policy.Approved) error {
	var records []telemetry.Record = make([]telemetry.Record, 0, 1)
	if len(record.Bytes()) > 0 {
		var r telemetry.Record
		if json.Unmarshal(record.Bytes(), &r) != nil {
			return fmt.Errorf("delivery requires approved record")
		}
		records = append(records, r)
		if records[0].Metrics == nil {
			records[0].Metrics = map[string]telemetry.Stats{}
		}
		depth := float64(d.journal.Depth())
		records[0].Metrics["wal_depth"] = telemetry.Stats{Min: depth, Max: depth, Avg: depth, P95: depth, Count: 1}
		records[0].Metrics["wal_buffer_full"] = telemetry.Stats{Count: 1}
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return fmt.Errorf("batch ID generation failed")
	}
	e, err := d.journal.Enqueue(func(seq uint64) (policy.Approved, error) {
		b := telemetry.Batch{Tenant: d.binding.Tenant, CollectorID: d.binding.CollectorID, Region: d.binding.Region, Seq: seq, BatchID: hex.EncodeToString(id[:]), SentAt: collect.Timestamp(time.Now()), Records: records}
		data, err := json.Marshal(b)
		if err != nil {
			return policy.Approved{}, err
		}
		return d.gate.ApproveBatch(data)
	}, record.StrippedCount())
	if err != nil {
		d.note(err.Error())
		if errors.Is(err, wal.ErrFull) {
			d.fullOnce.Do(d.bufferFull)
		}
		return err
	}
	d.proofs.Update(e, "BUFFERED", "")
	d.mu.Lock()
	d.lastQueued = time.Now()
	d.mu.Unlock()
	select {
	case d.wake <- struct{}{}:
	default:
	}
	return nil
}

func (d *Delivery) bufferFull() {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return
	}
	r := telemetry.Record{Tenant: d.binding.Tenant, CollectorID: d.binding.CollectorID, Region: d.binding.Region, CollectedAt: collect.Timestamp(time.Now()), Source: "live", Metrics: map[string]telemetry.Stats{"wal_buffer_full": {Min: 1, Max: 1, Avg: 1, P95: 1, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	e, err := d.journal.EnqueueEmergency(func(seq uint64) (policy.Approved, error) {
		b := telemetry.Batch{Tenant: r.Tenant, CollectorID: r.CollectorID, Region: r.Region, Seq: seq, BatchID: hex.EncodeToString(id[:]), SentAt: r.CollectedAt, Records: []telemetry.Record{r}}
		data, err := json.Marshal(b)
		if err != nil {
			return policy.Approved{}, err
		}
		return d.gate.ApproveBatch(data)
	})
	if err != nil {
		d.note("BUFFER_FULL_STATUS_UNAVAILABLE: local WAL could not persist status")
		return
	}
	d.proofs.Update(e, "BUFFERED", "")
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Run owns both retry delivery and idle heartbeats. Cancelling never deletes WAL.
func (d *Delivery) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	heartbeatDone := make(chan error, 1)
	go func() {
		for {
			d.mu.Lock()
			delay := time.Until(d.lastQueued.Add(5 * time.Second))
			d.mu.Unlock()
			if delay < 0 {
				delay = 0
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				heartbeatDone <- nil
				return
			case <-timer.C:
				d.mu.Lock()
				idle := time.Since(d.lastQueued) >= 5*time.Second
				d.mu.Unlock()
				if idle {
					if err := d.Enqueue(policy.Approved{}); err != nil {
						heartbeatDone <- err
						cancel()
						return
					}
				}
			}
		}
	}()
	err := d.worker(ctx)
	cancel()
	heartbeatErr := <-heartbeatDone
	if heartbeatErr != nil {
		return heartbeatErr
	}
	return err
}

func (d *Delivery) worker(ctx context.Context) error {
	backoff := 250 * time.Millisecond
	for ctx.Err() == nil {
		d.mu.Lock()
		paused := d.paused
		duplicate := d.duplicatePending
		d.duplicatePending = false
		d.mu.Unlock()
		if paused {
			d.mu.Lock()
			d.duplicatePending = d.duplicatePending || duplicate
			d.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}
		if duplicate {
			entries := d.journal.Entries()
			for i := len(entries) - 1; i >= 0; i-- {
				if d.journal.IsAcked(entries[i].Seq) {
					_ = d.deliver(ctx, entries[i], d.journal.Epoch())
					break
				}
			}
		}
		// Handshake even when the WAL is empty: it detects receiver data loss.
		epoch, err := d.session(ctx)
		if err == nil && epoch != d.journal.Epoch() {
			if err := d.journal.SetEpoch(epoch); err != nil {
				d.note(err.Error())
				return err
			}
			d.note("RECEIVER_SESSION_CHANGED: replaying retained WAL history")
			d.proofs.Reset()
			for _, entry := range d.journal.Entries() {
				d.proofs.Update(entry, "BUFFERED", "")
			}
		}
		if err == nil {
			e, ok := d.journal.Next()
			if !ok {
				select {
				case <-ctx.Done():
					return nil
				case <-d.wake:
				case <-time.After(time.Second):
				}
				backoff = 250 * time.Millisecond
				continue
			}
			err = d.deliver(ctx, e, epoch)
			if err == nil {
				backoff = 250 * time.Millisecond
				continue
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		d.mu.Lock()
		d.counts.Retried++
		d.mu.Unlock()
		d.note("DELIVERY_RETRY: batch retained in WAL")
		jitter, randomErr := rand.Int(rand.Reader, big.NewInt(int64(backoff/2)+1))
		if randomErr != nil {
			return fmt.Errorf("retry jitter unavailable")
		}
		timer := time.NewTimer(backoff + time.Duration(jitter.Int64()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
	return nil
}
func (d *Delivery) request(ctx context.Context, method, path string, body []byte, epoch string, out any) error {
	r, err := http.NewRequestWithContext(ctx, method, d.base+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request construction failed")
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if d.Token != "" {
		r.Header.Set("Authorization", "Bearer "+d.Token)
	}
	if epoch != "" {
		r.Header.Set("X-Kalki-Server-ID", epoch)
	}
	resp, err := d.client.Do(r)
	if err != nil {
		return fmt.Errorf("receiver unavailable")
	}
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	closeErr := resp.Body.Close()
	if readErr != nil || closeErr != nil || len(data) > 1024*1024 {
		return fmt.Errorf("invalid receiver response")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("receiver rejected request")
	}
	if json.Unmarshal(data, out) != nil {
		return fmt.Errorf("invalid receiver JSON")
	}
	return nil
}
func (d *Delivery) session(ctx context.Context) (string, error) {
	var response struct {
		ServerID string `json:"server_id"`
		Region   string `json:"region"`
	}
	if err := d.request(ctx, "GET", "/api/session", nil, "", &response); err != nil {
		return "", err
	}
	decoded, err := hex.DecodeString(response.ServerID)
	if err != nil || len(decoded) != 16 || response.Region != d.binding.Region {
		return "", fmt.Errorf("receiver session/region invalid")
	}
	return response.ServerID, nil
}
func (d *Delivery) Disconnect(ctx context.Context) error {
	return d.request(ctx, "POST", "/api/disconnect", nil, "", nil)
}

func (d *Delivery) deliver(ctx context.Context, e wal.Entry, epoch string) error {
	d.proofs.Update(e, "SENT", "")
	d.mu.Lock()
	d.counts.Sent++
	d.mu.Unlock()
	var ack telemetry.Ack
	if err := d.request(ctx, "POST", "/api/ingest", e.Bytes, epoch, &ack); err != nil {
		return err
	}
	// Echoes are untrusted data: retain only syntactically valid hashes locally.
	echoBytes, decodeErr := hex.DecodeString(ack.PayloadSHA256)
	if decodeErr != nil || len(echoBytes) != 32 {
		return fmt.Errorf("invalid proof echo")
	}
	d.proofs.Update(e, "SENT", ack.PayloadSHA256)
	if ack.ServerID != epoch || ack.Seq != e.Seq || ack.PayloadSHA256 != e.Hash || ack.HighestContiguous > d.journal.LastSeq() {
		d.note("EGRESS_PROOF_MISMATCH: ACK rejected; WAL retained")
		return fmt.Errorf("ACK mismatch")
	}
	for i, gap := range ack.Gaps {
		if gap == e.Seq || gap <= ack.HighestContiguous || gap > d.journal.LastSeq() || (i > 0 && gap <= ack.Gaps[i-1]) {
			return fmt.Errorf("invalid ACK gaps")
		}
	}
	if err := d.journal.Ack(e.Seq, epoch, e.Hash); err != nil {
		d.note(err.Error())
		return err
	}
	d.proofs.Update(e, "ACKED", ack.PayloadSHA256)
	d.mu.Lock()
	d.counts.Acked++
	d.counts.DuplicatesRejected = ack.DuplicatesRejected
	d.counts.Gaps = append([]uint64{}, ack.Gaps...)
	d.mu.Unlock()
	return nil
}

func (d *Delivery) attemptViolation() map[string]any {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return map[string]any{"blocked": true, "reason": "random_unavailable"}
	}
	b := telemetry.Batch{Tenant: d.binding.Tenant, CollectorID: d.binding.CollectorID, Region: d.binding.Region, Seq: d.journal.LastSeq() + 1, BatchID: hex.EncodeToString(id[:]), SentAt: collect.Timestamp(time.Now()), Records: []telemetry.Record{{Tenant: d.binding.Tenant, CollectorID: d.binding.CollectorID, Region: d.binding.Region, CollectedAt: collect.Timestamp(time.Now()), Source: "live", Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 101, Max: 101, Avg: 101, P95: 101, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}}}
	data, err := json.Marshal(b)
	if err != nil {
		return map[string]any{"blocked": true, "reason": "encoding"}
	}
	_, err = d.gate.ApproveBatch(data)
	if err != nil {
		sum := sha256.Sum256(data)
		d.proofs.Update(wal.Entry{Seq: b.Seq, Bytes: data, Hash: hex.EncodeToString(sum[:])}, "BLOCKED", "")
	}
	return map[string]any{"blocked": err != nil, "sent": false, "reason": "statistics_range", "wal_depth": d.journal.Depth()}
}
