// Package store owns optional PostgreSQL persistence. Errors never include DSNs.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kalki/internal/telemetry"
	"kalki/tier2/ingest"
	"kalki/tier2/medha"
	"sync"
	"time"
)

var ErrUnavailable = errors.New("PostgreSQL persistence unavailable")

type Session struct {
	Hash, Tenant, Region string
	Expires              time.Time
}
type Token struct{ Hash, Tenant, Region, Collector string }
type Audit struct {
	Tenant         string
	Index          int
	At, Event      string
	Status         int
	Previous, Hash string
}
type Remediation struct {
	ID, Tenant, Incident, Collector, Status string
	Reversible, TargetValid, Authorized     bool
	Action, Result, At                      string
}
type Job struct {
	Batch          telemetry.Batch
	Hash, Received string
	Progress       ingest.Checkpoint
	Incidents      []medha.Explanation
}
type Loaded struct {
	Sessions     []Session
	Tokens       []Token
	Audit        []Audit
	Remediations []Remediation
	Progress     []ingest.Checkpoint
	Records      []ingest.StoredRecord
	Incidents    []medha.Explanation
}
type Store struct {
	pool          *pgxpool.Pool
	queue         chan Job
	slots         chan struct{}
	stop          chan struct{}
	done          chan struct{}
	once          sync.Once
	mu            sync.Mutex
	closed        bool
	failed        bool
	writerContext context.Context
	cancelWriter  context.CancelFunc
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY,tenant TEXT NOT NULL,region TEXT NOT NULL,expires_at TIMESTAMPTZ NOT NULL);
CREATE TABLE IF NOT EXISTS collector_tokens(token_hash TEXT PRIMARY KEY,tenant TEXT NOT NULL,region TEXT NOT NULL,collector_id TEXT UNIQUE NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS audit_entries(tenant TEXT NOT NULL,entry_index INT NOT NULL,at TEXT NOT NULL,event TEXT NOT NULL,status INT NOT NULL,previous_sha256 TEXT NOT NULL,sha256 TEXT NOT NULL,PRIMARY KEY(tenant,entry_index));
CREATE TABLE IF NOT EXISTS telemetry_records(tenant TEXT NOT NULL,region TEXT NOT NULL,collector_id TEXT NOT NULL,seq BIGINT NOT NULL,batch_id TEXT NOT NULL,payload_sha256 TEXT NOT NULL,received_at TEXT NOT NULL,collected_at TEXT NOT NULL,record_json JSONB NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS telemetry_records_identity ON telemetry_records(tenant,region,collector_id,seq,collected_at);
CREATE INDEX IF NOT EXISTS telemetry_records_recent ON telemetry_records(tenant,region,collector_id,collected_at DESC);
CREATE TABLE IF NOT EXISTS remediations(id TEXT PRIMARY KEY,tenant TEXT NOT NULL,incident_id TEXT NOT NULL,collector_id TEXT NOT NULL,status TEXT NOT NULL,reversible BOOL NOT NULL,target_valid BOOL NOT NULL,tenant_authorized BOOL NOT NULL,action TEXT NOT NULL,result TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS collector_progress(tenant TEXT NOT NULL,region TEXT NOT NULL,collector_id TEXT PRIMARY KEY,state JSONB NOT NULL);
CREATE TABLE IF NOT EXISTS analysis_incidents(tenant TEXT PRIMARY KEY,items JSONB NOT NULL);
`

func Open(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, nil
	}
	cfg, e := pgxpool.ParseConfig(dsn)
	if e != nil {
		return nil, ErrUnavailable
	}
	cfg.MaxConns = 5
	cfg.MinConns = 1
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	p, e := pgxpool.NewWithConfig(c, cfg)
	if e != nil {
		return nil, ErrUnavailable
	}
	if e = p.Ping(c); e == nil {
		_, e = p.Exec(c, schema)
	}
	if e != nil {
		p.Close()
		return nil, ErrUnavailable
	}
	s := &Store{pool: p, queue: make(chan Job, 256), slots: make(chan struct{}, 256), stop: make(chan struct{}), done: make(chan struct{})}
	s.writerContext, s.cancelWriter = context.WithCancel(context.Background())
	go s.writer()
	return s, nil
}
func (s *Store) exec(query string, args ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tag, e := s.pool.Exec(ctx, query, args...)
	if e != nil || tag.RowsAffected() != 1 {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) SaveSession(v Session) error {
	return s.exec(`INSERT INTO sessions VALUES($1,$2,$3,$4) ON CONFLICT(token_hash) DO UPDATE SET expires_at=excluded.expires_at`, v.Hash, v.Tenant, v.Region, v.Expires)
}
func (s *Store) DeleteSession(hash string) error {
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_, e := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hash)
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) SaveToken(v Token) error {
	return s.exec(`INSERT INTO collector_tokens(token_hash,tenant,region,collector_id) VALUES($1,$2,$3,$4) ON CONFLICT(collector_id) DO UPDATE SET token_hash=excluded.token_hash,region=excluded.region,updated_at=now() WHERE collector_tokens.tenant=excluded.tenant`, v.Hash, v.Tenant, v.Region, v.Collector)
}
func (s *Store) AppendAudit(v Audit) error {
	return s.exec(`INSERT INTO audit_entries VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant,entry_index) DO UPDATE SET sha256=audit_entries.sha256 WHERE audit_entries.sha256=excluded.sha256`, v.Tenant, v.Index, v.At, v.Event, v.Status, v.Previous, v.Hash)
}
func (s *Store) SaveRemediation(v Remediation) error {
	return s.exec(`INSERT INTO remediations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO UPDATE SET status=excluded.status,result=excluded.result,updated_at=excluded.updated_at WHERE remediations.tenant=excluded.tenant`, v.ID, v.Tenant, v.Incident, v.Collector, v.Status, v.Reversible, v.TargetValid, v.Authorized, v.Action, v.Result, v.At)
}

// Reserve before accepting ingest. Accepted jobs can always enter the queue.
func (s *Store) Reserve() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return false
	}
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}
func (s *Store) Release()      { <-s.slots }
func (s *Store) Enqueue(j Job) { s.queue <- j }
func (s *Store) Healthy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.failed && !s.closed && len(s.slots) < cap(s.slots)
}
func (s *Store) Ping(ctx context.Context) error {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if !s.Healthy() || s.pool.Ping(c) != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) writer() {
	defer close(s.done)
	timer := time.NewTicker(250 * time.Millisecond)
	defer timer.Stop()
	prune := time.NewTicker(time.Minute)
	defer prune.Stop()
	pending := []Job{}
	draining := false
	stop := s.stop
	flush := func() bool {
		if len(pending) == 0 {
			return true
		}
		err := s.flush(pending)
		s.mu.Lock()
		s.failed = err != nil
		s.mu.Unlock()
		if err != nil {
			return false
		}
		for range pending {
			s.Release()
		}
		pending = nil
		return true
	}
	for {
		if draining && len(s.queue) == 0 && flush() {
			return
		}
		select {
		case <-s.writerContext.Done():
			return
		case j := <-s.queue:
			pending = append(pending, j)
			if len(pending) >= 25 {
				flush()
			}
		case <-timer.C:
			flush()
		case <-prune.C:
			s.prune()
		case <-stop:
			draining = true
			stop = nil
		}
	}
}
func (s *Store) flush(jobs []Job) error {
	ctx, c := context.WithTimeout(s.writerContext, 5*time.Second)
	defer c()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return ErrUnavailable
	}
	defer tx.Rollback(ctx)
	b := &pgx.Batch{}
	for _, j := range jobs {
		for _, r := range j.Batch.Records {
			raw, _ := json.Marshal(r)
			b.Queue(`INSERT INTO telemetry_records VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT DO NOTHING`, r.Tenant, r.Region, r.CollectorID, int64(j.Batch.Seq), j.Batch.BatchID, j.Hash, j.Received, r.CollectedAt, string(raw))
		}
		progress, _ := json.Marshal(j.Progress)
		b.Queue(`INSERT INTO collector_progress VALUES($1,$2,$3,$4) ON CONFLICT(collector_id) DO UPDATE SET state=excluded.state WHERE collector_progress.tenant=excluded.tenant`, j.Batch.Tenant, j.Batch.Region, j.Batch.CollectorID, string(progress))
		inc, _ := json.Marshal(j.Incidents)
		b.Queue(`INSERT INTO analysis_incidents VALUES($1,$2) ON CONFLICT(tenant) DO UPDATE SET items=excluded.items`, j.Batch.Tenant, string(inc))
	}
	result := tx.SendBatch(ctx, b)
	if result.Close() != nil {
		return ErrUnavailable
	}
	if tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Store) prune() {
	ctx, c := context.WithTimeout(s.writerContext, 5*time.Second)
	defer c()
	s.pool.Exec(ctx, `DELETE FROM telemetry_records WHERE ctid IN (SELECT row_id FROM (SELECT ctid row_id,row_number() OVER(PARTITION BY tenant,region,collector_id ORDER BY collected_at DESC,seq DESC) rn FROM telemetry_records) x WHERE rn>4096)`)
	s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at<now()`)
}
func (s *Store) Close(ctx context.Context) error {
	s.once.Do(func() { s.mu.Lock(); s.closed = true; s.mu.Unlock(); close(s.stop) })
	select {
	case <-s.done:
		s.cancelWriter()
		s.pool.Close()
		return nil
	case <-ctx.Done():
		s.cancelWriter()
		<-s.done
		s.pool.Close()
		return errors.New("telemetry drain incomplete; unflushed acknowledgements may be lost")
	}
}

func (s *Store) Load(capacity int) (Loaded, error) {
	var out Loaded
	queries := []struct {
		sql  string
		read func(pgx.Rows) error
	}{
		{`SELECT token_hash,tenant,region,expires_at FROM sessions WHERE expires_at>now()`, func(r pgx.Rows) error {
			var v Session
			e := r.Scan(&v.Hash, &v.Tenant, &v.Region, &v.Expires)
			out.Sessions = append(out.Sessions, v)
			return e
		}},
		{`SELECT token_hash,tenant,region,collector_id FROM collector_tokens`, func(r pgx.Rows) error {
			var v Token
			e := r.Scan(&v.Hash, &v.Tenant, &v.Region, &v.Collector)
			out.Tokens = append(out.Tokens, v)
			return e
		}},
		{`SELECT tenant,entry_index,at,event,status,previous_sha256,sha256 FROM audit_entries ORDER BY tenant,entry_index`, func(r pgx.Rows) error {
			var v Audit
			e := r.Scan(&v.Tenant, &v.Index, &v.At, &v.Event, &v.Status, &v.Previous, &v.Hash)
			out.Audit = append(out.Audit, v)
			return e
		}},
		{`SELECT id,tenant,incident_id,collector_id,status,reversible,target_valid,tenant_authorized,action,result,updated_at FROM remediations`, func(r pgx.Rows) error {
			var v Remediation
			e := r.Scan(&v.ID, &v.Tenant, &v.Incident, &v.Collector, &v.Status, &v.Reversible, &v.TargetValid, &v.Authorized, &v.Action, &v.Result, &v.At)
			out.Remediations = append(out.Remediations, v)
			return e
		}},
		{`SELECT state FROM collector_progress`, func(r pgx.Rows) error {
			var raw []byte
			var v ingest.Checkpoint
			if e := r.Scan(&raw); e != nil {
				return e
			}
			e := json.Unmarshal(raw, &v)
			out.Progress = append(out.Progress, v)
			return e
		}},
		{`SELECT items FROM analysis_incidents`, func(r pgx.Rows) error {
			var raw []byte
			var v []medha.Explanation
			if e := r.Scan(&raw); e != nil {
				return e
			}
			e := json.Unmarshal(raw, &v)
			out.Incidents = append(out.Incidents, v...)
			return e
		}},
	}
	for _, q := range queries {
		ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
		rows, e := s.pool.Query(ctx, q.sql)
		if e == nil {
			for rows.Next() {
				if e = q.read(rows); e != nil {
					break
				}
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
		}
		c()
		if e != nil {
			return out, ErrUnavailable
		}
	}
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	rows, e := s.pool.Query(ctx, `SELECT seq,batch_id,received_at,record_json FROM (SELECT *,row_number() OVER(PARTITION BY tenant,region,collector_id ORDER BY collected_at DESC,seq DESC) rn FROM telemetry_records) x WHERE rn<=$1 ORDER BY collected_at,seq`, capacity)
	if e != nil {
		return out, ErrUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var v ingest.StoredRecord
		var raw []byte
		if rows.Scan(&v.Seq, &v.BatchID, &v.ReceivedAt, &raw) != nil || json.Unmarshal(raw, &v.Record) != nil {
			return out, ErrUnavailable
		}
		out.Records = append(out.Records, v)
	}
	if rows.Err() != nil {
		return out, ErrUnavailable
	}
	return out, nil
}
