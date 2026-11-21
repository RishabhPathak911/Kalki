package main

import (
	"context"
	"fmt"
	"kalki/tier1/collect"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func demoActions(ctx context.Context, state *collect.DemoFault, path string) func(context.Context, string) (map[string]any, error) {
	var mu sync.Mutex
	return func(_ context.Context, action string) (map[string]any, error) {
		mu.Lock()
		defer mu.Unlock()
		result := map[string]any{"action": action, "status": "finished"}
		line := ""
		switch action {
		case "inject-secret":
			line = "KALKI_DEMO fake AWS access_key=AKIAIOSFODNN7EXAMPLE password=FakeDemoOnly987!\n"
			result["detail"] = "One synthetic secret line appended locally; inspect redaction counts after the next rollup"
		case "bruteforce":
			line = strings.Repeat("KALKI_DEMO Failed password for demo_user from 198.51.100.42 port 2222 ssh2\n", 6)
			result["detail"] = "Six synthetic failures appended; inspect SIMULATED risk events after the next rollup"
		case "cpu-stress":
			if !state.Stress(ctx) {
				return nil, fmt.Errorf("CPU demo already running")
			}
			result["status"] = "running"
			result["duration_s"] = 15
			result["detail"] = "One bounded CPU worker active for 15 seconds; real utilization depends on host core count"
		case "net-degrade":
			state.Until.Store(time.Now().Add(20 * time.Second).UnixNano())
			result["status"] = "running"
			result["duration_s"] = 20
			result["detail"] = "SIMULATED probe latency/loss active for 20 seconds; normal probes resume automatically"
		default:
			return nil, fmt.Errorf("unknown action")
		}
		if line != "" {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				return nil, err
			}
			_, err = f.WriteString(line)
			syncErr := f.Sync()
			closeErr := f.Close()
			if err != nil {
				return nil, err
			}
			if syncErr != nil {
				return nil, syncErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
		return result, nil
	}
}
