// Package scale measures simulated ingest; it never impersonates live probes.
package scale

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier1/policy"
	"kalki/tier2/ingest"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"
)

type Credential struct {
	Tenant    string `json:"tenant"`
	Collector string `json:"collector_id"`
	Region    string `json:"region"`
	Token     string `json:"token"`
}
type Options struct {
	Collectors, Tenants          int
	Duration                     time.Duration
	URL, CA, Credentials, Policy string
}
type Report struct {
	Source     string  `json:"source"`
	Mode       string  `json:"mode"`
	Collectors int     `json:"collectors"`
	Tenants    int     `json:"tenants"`
	Sent       int     `json:"batches_sent"`
	Failed     int     `json:"batches_failed"`
	Throughput float64 `json:"throughput_batches_per_sec"`
	P95        float64 `json:"ingest_p95_ms"`
	Peak       float64 `json:"peak_alloc_mb"`
	Seconds    float64 `json:"duration_s"`
}

func Run(o Options) (Report, error) {
	out := Report{Source: "SIMULATED", Mode: "in_process_protocol", Collectors: o.Collectors, Tenants: o.Tenants}
	if o.Collectors < 1 || o.Collectors > 10000 || o.Tenants < 1 || o.Tenants > o.Collectors || o.Duration <= 0 {
		return out, fmt.Errorf("invalid scale options")
	}
	gate, err := policy.Load(o.Policy, nil)
	if err != nil {
		return out, err
	}
	receiver, err := ingest.New(gate, "ap-south-1", 128)
	if err != nil {
		return out, err
	}
	creds := make([]Credential, o.Collectors)
	epoch := receiver.ID()
	var client *http.Client
	if o.URL != "" {
		if len(o.URL) < 8 || o.URL[:8] != "https://" {
			return out, fmt.Errorf("HTTPS URL required")
		}
		b, e := os.ReadFile(o.CA)
		if e != nil {
			return out, e
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(b) {
			return out, fmt.Errorf("invalid CA")
		}
		client = &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		defer client.CloseIdleConnections()
		b, e = os.ReadFile(o.Credentials)
		if e != nil {
			return out, e
		}
		if json.Unmarshal(b, &creds) != nil || len(creds) != o.Collectors {
			return out, fmt.Errorf("credentials must contain one pre-minted identity per collector")
		}
		tenants := map[string]bool{}
		ids := map[string]bool{}
		for _, c := range creds {
			if c.Token == "" || ids[c.Collector] {
				return out, fmt.Errorf("invalid credentials")
			}
			ids[c.Collector] = true
			tenants[c.Tenant] = true
		}
		if len(tenants) != o.Tenants {
			return out, fmt.Errorf("credential tenant count mismatch")
		}
		req, _ := http.NewRequest("GET", o.URL+"/api/session", nil)
		req.Header.Set("Authorization", "Bearer "+creds[0].Token)
		res, e := client.Do(req)
		if e != nil {
			return out, fmt.Errorf("session handshake failed")
		}
		var session struct {
			ID string `json:"server_id"`
		}
		e = json.NewDecoder(res.Body).Decode(&session)
		res.Body.Close()
		if e != nil || res.StatusCode != 200 || session.ID == "" {
			return out, fmt.Errorf("session handshake rejected")
		}
		epoch = session.ID
		out.Mode = "https_authenticated"
	} else {
		for i := range creds {
			creds[i] = Credential{Tenant: fmt.Sprintf("tenant_%d", i%o.Tenants), Collector: fmt.Sprintf("host_%08x", i+1), Region: "ap-south-1"}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), o.Duration)
	defer cancel()
	start := time.Now()
	var mu sync.Mutex
	var latencies []float64
	var wg sync.WaitGroup
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		mu.Lock()
		if mb := float64(m.Alloc) / (1 << 20); mb > out.Peak {
			out.Peak = mb
		}
		mu.Unlock()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				sample()
				return
			case <-t.C:
				sample()
			}
		}
	}()
	for _, c := range creds {
		wg.Add(1)
		go func(c Credential) {
			defer wg.Done()
			seq := uint64(0)
			for ctx.Err() == nil {
				seq++
				at := collect.Timestamp(time.Now())
				r := telemetry.Record{Tenant: c.Tenant, CollectorID: c.Collector, Region: c.Region, CollectedAt: at, Source: "simulated", Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: 20, Max: 20, Avg: 20, P95: 20, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
				raw, _ := json.Marshal(telemetry.Batch{Tenant: c.Tenant, CollectorID: c.Collector, Region: c.Region, Seq: seq, BatchID: fmt.Sprintf("%032x", seq), SentAt: at, Records: []telemetry.Record{r}})
				approved, e := gate.ApproveBatch(raw)
				if e != nil {
					mu.Lock()
					out.Failed++
					mu.Unlock()
					return
				}
				began := time.Now()
				code := 0
				if client == nil {
					req := httptest.NewRequest("POST", "https://local/api/ingest", bytes.NewReader(approved.Bytes()))
					req.Header.Set("X-Kalki-Server-ID", epoch)
					w := httptest.NewRecorder()
					receiver.ServeHTTP(w, req)
					code = w.Code
				} else {
					req, e := http.NewRequestWithContext(ctx, "POST", o.URL+"/api/ingest", bytes.NewReader(approved.Bytes()))
					if e == nil {
						req.Header.Set("Authorization", "Bearer "+c.Token)
						req.Header.Set("Content-Type", "application/json")
						req.Header.Set("X-Kalki-Server-ID", epoch)
						res, e := client.Do(req)
						if e == nil {
							code = res.StatusCode
							io.Copy(io.Discard, res.Body)
							res.Body.Close()
						}
					}
				}
				ms := float64(time.Since(began)) / float64(time.Millisecond)
				mu.Lock()
				if code == 200 {
					out.Sent++
					latencies = append(latencies, ms)
				} else {
					out.Failed++
				}
				mu.Unlock()
				select {
				case <-ctx.Done():
					return
				case <-time.After(100 * time.Millisecond):
				}
			}
		}(c)
	}
	wg.Wait()
	cancel()
	<-done
	out.Seconds = time.Since(start).Seconds()
	out.Throughput = float64(out.Sent) / out.Seconds
	sort.Float64s(latencies)
	if len(latencies) > 0 {
		out.P95 = latencies[(len(latencies)-1)*95/100]
	}
	return out, nil
}
