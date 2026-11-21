package collect

import (
	"testing"
	"time"
)

func TestCollectorCPUIsNormalizedAndSeparate(t *testing.T) {
	var c ProcessCPU
	now := time.Now()
	if _, ok := c.Observe(now, 10, 4); ok {
		t.Fatal("first sample must not invent a rate")
	}
	value, ok := c.Observe(now.Add(time.Second), 11, 4)
	if !ok || value != 25 {
		t.Fatalf("want 25%% of 4-core host, got %v", value)
	}
	if _, ok := c.Observe(now.Add(2*time.Second), 1, 4); ok {
		t.Fatal("counter reset")
	}
}
