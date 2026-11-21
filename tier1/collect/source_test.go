package collect

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	psnet "github.com/shirou/gopsutil/v3/net"
)

func TestBandwidth(t *testing.T) {
	var b Bandwidth
	now := time.Now()
	stat := func(sent, recv uint64) []psnet.IOCountersStat {
		return []psnet.IOCountersStat{{Name: "eth0", BytesSent: sent, BytesRecv: recv}}
	}
	if v, ok := b.Observe(now, stat(100, 100)); ok || v != 0 {
		t.Fatal("first tick must be unknown")
	}
	if v, ok := b.Observe(now.Add(time.Second), stat(625100, 625100)); !ok || v != 10 {
		t.Fatalf("rate=%v, valid=%v", v, ok)
	}
	if v, ok := b.Observe(now.Add(2*time.Second), stat(5, 5)); ok || v != 0 {
		t.Fatal("counter reset must not spike")
	}
	if v, ok := b.Observe(now.Add(3*time.Second), stat(625005, 625005)); !ok || v != 10 {
		t.Fatal("did not recover")
	}
	if _, ok := b.Observe(now.Add(2*time.Second), stat(625005, 625005)); ok {
		t.Fatal("clock reversal accepted")
	}
}

func TestProbeWindow(t *testing.T) {
	var p ProbeWindow
	if p.Observe(false) != 100 {
		t.Fatal("wrong first probe loss")
	}
	for i := 0; i < 9; i++ {
		p.Observe(true)
	}
	if p.Observe(true) != 0 {
		t.Fatal("window did not roll after ten probes")
	}
}

func TestReplayRestampsAndValidates(t *testing.T) {
	now := time.Date(2026, 11, 20, 1, 2, 3, 456000000, time.UTC)
	m := DatasetMapping{Metric: "cpu_pct", Scale: 1, Min: 0, Max: 100}
	s, err := replaySample([]string{"2014-01-01 00:00:00", "42"}, 0, 1, m, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.CollectedAt != "2026-11-20T01:02:03.456Z" || s.OriginalTime != "2014-01-01 00:00:00" || s.Source != Replayed || s.Value != 42 {
		t.Fatalf("bad sample: %+v", s)
	}
	for _, value := range []string{"NaN", "Inf", "-1", "101", "bad", ""} {
		if _, err := replaySample([]string{"2014-01-01 00:00:00", value}, 0, 1, m, now); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	if _, err := replaySample([]string{"bad", "2"}, 0, 1, m, now); err == nil {
		t.Fatal("accepted invalid timestamp")
	}
	if _, err := replaySample([]string{"2014-01-01 00:00:00"}, 0, 1, m, now); err == nil {
		t.Fatal("accepted missing value")
	}
}

func TestReplayLoopOrderAndCancellation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.csv")
	write(t, path, "timestamp,value\n2014-01-02 00:00:00,1\n2014-01-01 00:00:00,2\n2014-01-01 00:00:00,2\ninvalid,NaN\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &DatasetReplaySource{Root: dir, Mappings: []DatasetMapping{{Pattern: "*.csv", Metric: "cpu_pct", Scale: 1, Max: 100}}, Interval: time.Millisecond}
	ch := s.Stream(ctx)
	for _, want := range []float64{1, 2, 2, 1} {
		select {
		case sample := <-ch:
			if sample.Value != want {
				t.Fatalf("value %v, want %v", sample.Value, want)
			}
			stamp, err := time.Parse(TimeLayout, sample.CollectedAt)
			if err != nil || time.Since(stamp) > time.Second {
				t.Fatal("not stamped now")
			}
		case <-time.After(time.Second):
			t.Fatal("replay hung")
		}
	}
	cancel()
	assertClosed(t, ch)
}

func TestLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.log")
	tail := &LogTail{}
	if _, err := tail.Read(path); !os.IsNotExist(err) {
		t.Fatal("missing file error expected")
	}
	write(t, path, "first\npar")
	lines, err := tail.Read(path)
	if err != nil || len(lines) != 1 || lines[0] != "first" {
		t.Fatalf("%v %v", lines, err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("tial\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	lines, err = tail.Read(path)
	if err != nil || len(lines) != 1 || lines[0] != "partial" {
		t.Fatalf("%v %v", lines, err)
	}
	write(t, path, "new\n")
	lines, err = tail.Read(path)
	if err != nil || len(lines) != 1 || lines[0] != "new" {
		t.Fatalf("truncation: %v %v", lines, err)
	}
	write(t, path, "replacement longer than prior read\n")
	lines, err = tail.Read(path)
	if err != nil || len(lines) != 1 || lines[0] != "replacement longer than prior read" {
		t.Fatalf("rewrite: %v %v", lines, err)
	}
}

func TestLogReplay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.log")
	write(t, path, "line one\nline two\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &LogSource{Files: []string{path}, LivePath: filepath.Join(dir, "live.log"), Interval: time.Millisecond, PollInterval: time.Millisecond}
	ch := s.Stream(ctx)
	for _, want := range []string{"line one", "line two", "line one"} {
		select {
		case sample := <-ch:
			if sample.Log != want || sample.Source != Replayed {
				t.Fatalf("%+v", sample)
			}
		case <-time.After(time.Second):
			t.Fatal("log replay hung")
		}
	}
	cancel()
	assertClosed(t, ch)
}

func TestMissingDatasetAndInvalidInterval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assertClosed(t, (&DatasetReplaySource{Root: t.TempDir(), Mappings: []DatasetMapping{{Pattern: "*.csv"}}, Interval: time.Millisecond}).Stream(ctx))
	assertClosed(t, (&LocalSource{Interval: 0}).Stream(ctx))
	assertClosed(t, (&LogSource{Interval: 0}).Stream(ctx))
}

func TestDatasetConfigurationAndNetworkUnits(t *testing.T) {
	cfg, err := LoadDatasets(filepath.Join("..", "..", "config", "datasets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, mapping := range cfg.Metrics {
		if mapping.Metric != "bandwidth_mbps" {
			continue
		}
		found = true
		sample, err := replaySample([]string{"2014-01-01 00:00:00", "37500000"}, 0, 1, mapping, time.Now())
		if err != nil || sample.Value < 0.999999 || sample.Value > 1.000001 {
			t.Fatalf("network unit conversion: %+v %v", sample, err)
		}
	}
	if !found || len(cfg.Logs) != 2 {
		t.Fatal("missing required mappings")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func assertClosed(t *testing.T, ch <-chan RawSample) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("source did not shut down")
		}
	}
}
