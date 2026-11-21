package collect

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type DatasetMapping struct {
	Pattern string  `yaml:"pattern"`
	Metric  string  `yaml:"metric"`
	Scale   float64 `yaml:"scale"`
	Min     float64 `yaml:"min"`
	Max     float64 `yaml:"max"`
}
type DatasetConfig struct {
	Metrics []DatasetMapping `yaml:"metrics"`
	Logs    []string         `yaml:"logs"`
}

func LoadDatasets(path string) (c DatasetConfig, resultErr error) {
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	for _, m := range c.Metrics {
		if m.Pattern == "" || m.Metric == "" || !finite(m.Scale) || m.Scale <= 0 || !finite(m.Min) || !finite(m.Max) || m.Max <= m.Min {
			return c, fmt.Errorf("invalid dataset mapping")
		}
	}
	return c, nil
}

type DatasetReplaySource struct {
	Root     string
	Mappings []DatasetMapping
	Interval time.Duration
	Report   Reporter
}

func (*DatasetReplaySource) Name() string { return "dataset-replay" }
func (s *DatasetReplaySource) Stream(ctx context.Context) <-chan RawSample {
	out := make(chan RawSample)
	go func() {
		defer close(out)
		if err := validateInterval(s.Interval); err != nil {
			report(s.Report, err)
			return
		}
		var wg sync.WaitGroup
		seen := map[string]bool{}
		for _, mapping := range s.Mappings {
			paths, err := filepath.Glob(filepath.Join(s.Root, filepath.FromSlash(mapping.Pattern)))
			if err != nil {
				report(s.Report, err)
				continue
			}
			if len(paths) == 0 {
				report(s.Report, fmt.Errorf("dataset unavailable: %s", mapping.Pattern))
			}
			for _, path := range paths {
				if seen[path] {
					continue
				}
				seen[path] = true
				wg.Add(1)
				go func(path string, mapping DatasetMapping) { defer wg.Done(); s.replay(ctx, out, path, mapping) }(path, mapping)
			}
		}
		wg.Wait()
	}()
	return out
}

func replaySample(row []string, ti, vi int, m DatasetMapping, now time.Time) (RawSample, error) {
	if ti < 0 || vi < 0 || ti >= len(row) || vi >= len(row) {
		return RawSample{}, fmt.Errorf("missing CSV fields")
	}
	original := strings.TrimSpace(row[ti])
	parsed := false
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02 15:04:05.999999999"} {
		if _, err := time.Parse(layout, original); err == nil {
			parsed = true
			break
		}
	}
	if !parsed {
		return RawSample{}, fmt.Errorf("invalid trace timestamp")
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(row[vi]), 64)
	v *= m.Scale
	if err != nil || !finite(v) || v < m.Min || v > m.Max {
		return RawSample{}, fmt.Errorf("invalid trace metric")
	}
	return RawSample{CollectedAt: Timestamp(now), OriginalTime: original, Source: Replayed, Metric: m.Metric, Value: v}, nil
}

func (s *DatasetReplaySource) replay(ctx context.Context, out chan<- RawSample, path string, m DatasetMapping) {
	for ctx.Err() == nil {
		f, err := os.Open(path)
		if err != nil {
			report(s.Report, err)
			if !pause(ctx, s.Interval) {
				return
			}
			continue
		}
		err = s.readPass(ctx, out, f, path, m)
		report(s.Report, f.Close())
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			report(s.Report, err)
		}
		if !pause(ctx, s.Interval) {
			return
		}
	}
}

func (s *DatasetReplaySource) readPass(ctx context.Context, out chan<- RawSample, input io.Reader, path string, m DatasetMapping) error {
	r := csv.NewReader(input)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return err
	}
	ti, vi := -1, -1
	for i, name := range header {
		switch strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")) {
		case "timestamp":
			ti = i
		case "value":
			vi = i
		}
	}
	if ti < 0 || vi < 0 {
		return fmt.Errorf("trace needs timestamp,value columns: %s", path)
	}
	first := true
	for ctx.Err() == nil {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			report(s.Report, fmt.Errorf("invalid CSV row in %s", path))
			continue
		}
		// Validate before waiting, then stamp at emission rather than file load.
		sample, err := replaySample(row, ti, vi, m, time.Now())
		if err != nil {
			report(s.Report, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if !first && !pause(ctx, s.Interval) {
			return ctx.Err()
		}
		first = false
		sample.CollectedAt = Timestamp(time.Now())
		sample.StreamID = path
		if !emit(ctx, out, sample) {
			return ctx.Err()
		}
	}
	return ctx.Err()
}
