package main

import (
	"io"
	"testing"
)

func TestInvalidOptions(t *testing.T) {
	for _, args := range [][]string{{"--mode=wrong"}, {"--interval=0s"}, {"--interval=bad"}, {"--tier2-url=http://localhost:8443"}, {"--probe-target=bad"}, {"--duration=-1s"}} {
		if err := run(args, io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
