package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"kalki/internal/telemetry"
	"kalki/tier1/collect"
	"kalki/tier2/medha"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type quality struct {
	Status    string  `json:"status"`
	Files     int     `json:"files_evaluated"`
	TP        int     `json:"true_positives"`
	FP        int     `json:"false_positives"`
	FN        int     `json:"false_negatives"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	FA        float64 `json:"false_alarms_per_hour"`
	Method    string  `json:"method"`
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if t, e := time.Parse(layout, s); e == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("bad timestamp")
}
func nab(root string) quality {
	q := quality{Status: "datasets_not_downloaded", Method: "Stable event-time sort; duplicate timestamps keep the last source row. Chronological 50% holdout per trace; first half warms the fixed detector. Training-only min/max maps values to cpu_pct [10,60], clipped [0,100]. Exact labeled timestamp scoring, one true positive per timestamp; not the official NAB window score. Labels are read only after detection."}
	paths := []string{}
	for _, dir := range []string{"realAWSCloudwatch", "realKnownCause"} {
		p, _ := filepath.Glob(filepath.Join(root, dir, "*.csv"))
		paths = append(paths, p...)
	}
	sort.Strings(paths)
	if _, err := os.Stat(filepath.Join(root, "labels", "combined_labels.json")); err != nil || len(paths) == 0 {
		return q
	}
	type result struct {
		path       string
		begin, end time.Time
		detections []time.Time
	}
	results := []result{}
	hours := 0.0
	cfg, books, err := medha.Load("config/thresholds.yaml", "config/playbooks.yaml")
	if err != nil {
		q.Status = "configuration_error"
		return q
	}
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			q.Status = "invalid_dataset"
			return q
		}
		rows, e := csv.NewReader(f).ReadAll()
		f.Close()
		if e != nil || len(rows) < 61 {
			q.Status = "invalid_dataset"
			return q
		}
		rows = rows[1:]
		// NAB contains duplicate and out-of-order timestamps. Normalize without
		// consulting labels, before choosing the chronological train/test split.
		for _, row := range rows {
			if len(row) < 2 {
				q.Status = "invalid_dataset"
				return q
			}
			if _, err := parseTime(row[0]); err != nil {
				q.Status = "invalid_dataset"
				return q
			}
		}
		sort.SliceStable(rows, func(i, j int) bool { a, _ := parseTime(rows[i][0]); b, _ := parseTime(rows[j][0]); return a.Before(b) })
		ordered := rows[:0]
		for _, row := range rows {
			at, _ := parseTime(row[0])
			if len(ordered) > 0 {
				previous, _ := parseTime(ordered[len(ordered)-1][0])
				if at.Equal(previous) {
					ordered[len(ordered)-1] = row
					continue
				}
			}
			ordered = append(ordered, row)
		}
		rows = ordered
		if len(rows) < 60 {
			q.Status = "invalid_dataset"
			return q
		}
		split := len(rows) / 2
		vals := make([]float64, len(rows))
		times := make([]time.Time, len(rows))
		lo, hi := math.Inf(1), math.Inf(-1)
		for i, row := range rows {
			if len(row) < 2 {
				q.Status = "invalid_dataset"
				return q
			}
			times[i], e = parseTime(row[0])
			if e != nil {
				q.Status = "invalid_dataset"
				return q
			}
			vals[i], e = strconv.ParseFloat(row[1], 64)
			if e != nil || math.IsNaN(vals[i]) || math.IsInf(vals[i], 0) {
				q.Status = "invalid_dataset"
				return q
			}
			if i > 0 && !times[i].After(times[i-1]) {
				q.Status = "invalid_dataset"
				return q
			}
			if i < split {
				lo = math.Min(lo, vals[i])
				hi = math.Max(hi, vals[i])
			}
		}
		engine := medha.New(cfg, books)
		seen := map[string]bool{}
		r := result{begin: times[split], end: times[len(times)-1]}
		r.path, _ = filepath.Rel(root, path)
		r.path = filepath.ToSlash(r.path)
		for i, v := range vals {
			scaled := 20.0
			if hi > lo {
				scaled = math.Max(0, math.Min(100, 10+50*(v-lo)/(hi-lo)))
			}
			engine.Observe(observation(times[i], scaled), times[i], false)
			for _, incident := range engine.Incidents("tenant_alpha") {
				if !seen[incident.ID] {
					seen[incident.ID] = true
					if i >= split {
						r.detections = append(r.detections, times[i])
					}
				}
			}
		}
		results = append(results, r)
		hours += r.end.Sub(r.begin).Hours()
		q.Files++
	}
	// Labels cannot influence normalization, detector configuration or warm-up.
	raw, e := os.ReadFile(filepath.Join(root, "labels", "combined_labels.json"))
	labels := map[string][]string{}
	if e != nil || json.Unmarshal(raw, &labels) != nil {
		q.Status = "invalid_labels"
		return q
	}
	for _, r := range results {
		truth := map[int64]bool{}
		ls, ok := labels[r.path]
		if !ok {
			q.Status = "missing_trace_labels"
			return q
		}
		for _, s := range ls {
			t, e := parseTime(s)
			if e != nil {
				q.Status = "invalid_labels"
				return q
			}
			if !t.Before(r.begin) && !t.After(r.end) {
				truth[t.UnixNano()] = false
			}
		}
		for _, t := range r.detections {
			hit, exists := truth[t.UnixNano()]
			if exists && !hit {
				q.TP++
				truth[t.UnixNano()] = true
			} else {
				q.FP++
			}
		}
		for _, hit := range truth {
			if !hit {
				q.FN++
			}
		}
	}
	if q.TP+q.FP > 0 {
		q.Precision = float64(q.TP) / float64(q.TP+q.FP)
	}
	if q.TP+q.FN > 0 {
		q.Recall = float64(q.TP) / float64(q.TP+q.FN)
	}
	if hours > 0 {
		q.FA = float64(q.FP) / hours
	}
	q.Status = "evaluated"
	return q
}
func observation(t time.Time, value float64) telemetry.Batch {
	r := telemetry.Record{Tenant: "tenant_alpha", CollectorID: "host_12345678", Region: "ap-south-1", CollectedAt: collect.Timestamp(t), Source: "replayed_real_trace", Metrics: map[string]telemetry.Stats{"cpu_pct": {Min: value, Max: value, Avg: value, P95: value, Count: 1}}, RiskEvents: []telemetry.Risk{}, RedactionCounts: map[string]int{}}
	return telemetry.Batch{Tenant: r.Tenant, CollectorID: r.CollectorID, Region: r.Region, Records: []telemetry.Record{r}}
}
