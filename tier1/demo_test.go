package main

import (
	"context"
	"kalki/tier1/collect"
	"kalki/tier1/filter"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoActionsUseOnlySyntheticLocalLogsAndBoundedCPU(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	state := &collect.DemoFault{}
	path := filepath.Join(t.TempDir(), "live_auth.log")
	actions := demoActions(ctx, state, path)
	for _, name := range []string{"inject-secret", "bruteforce"} {
		if _, err := actions(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "Failed password") != 6 || !strings.Contains(string(raw), "198.51.100.42") {
		t.Fatal("synthetic burst missing")
	}
	_, counts := filter.Scrub(string(raw))
	if counts["aws_key"] == 0 || counts["credential"] == 0 {
		t.Fatal("fake secrets do not exercise scrubbing")
	}
	if _, err := actions(ctx, "net-degrade"); err != nil || !state.Degraded() {
		t.Fatal("fault not enabled")
	}
	if _, err := actions(ctx, "cpu-stress"); err != nil {
		t.Fatal(err)
	}
	if _, err := actions(ctx, "cpu-stress"); err == nil {
		t.Fatal("unbounded concurrent stress allowed")
	}
	cancel()
}
