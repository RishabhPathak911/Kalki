package aggregate

import (
	"testing"
	"time"

	"kalki/tier1/filter"
)

func TestWindowStatisticsAndSourceSeparation(t *testing.T) {
	var a Aggregator
	now := time.Now()
	for i := 1; i <= 20; i++ {
		if err := a.Add(filter.Sample{CollectedAt: now, Source: "live", Metric: "cpu_pct", Value: float64(i), RedactionCounts: map[string]int{"aws_key": 1}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Add(filter.Sample{CollectedAt: now, Source: "replayed_real_trace", Metric: "cpu_pct", Value: 90}); err != nil {
		t.Fatal(err)
	}
	windows := a.Flush("tenant_alpha", "host_12345678", "ap-south-1")
	if len(windows) != 2 {
		t.Fatal("mixed live and replayed data")
	}
	stats := windows[0].Record.Metrics["cpu_pct"]
	if stats.Min != 1 || stats.Max != 20 || stats.Avg != 10.5 || stats.P95 != 19 || stats.Count != 20 {
		t.Fatalf("wrong statistics: %+v", stats)
	}
	if windows[0].Record.RedactionCounts["aws_key"] != 20 || windows[1].Record.Metrics["cpu_pct"].Avg != 90 {
		t.Fatal("incorrect grouping")
	}
	if len(a.Flush("tenant_alpha", "host_12345678", "ap-south-1")) != 0 {
		t.Fatal("window emitted twice")
	}
}
