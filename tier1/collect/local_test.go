package collect

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestLocalProbeAndCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			if err := conn.Close(); err != nil {
				return
			}
		}
	}()
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
		<-done
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &LocalSource{Interval: 20 * time.Millisecond, Targets: []string{listener.Addr().String()}}
	ch := s.Stream(ctx)
	seen := map[string]bool{}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for !seen["service_up"] || !seen["latency_ms"] || !seen["packet_loss_pct"] {
		select {
		case sample := <-ch:
			seen[sample.Metric] = true
			if sample.Source != Live {
				t.Fatal("wrong source")
			}
			if sample.Metric == "service_up" && sample.Value != 1 {
				t.Fatal("local listener marked down")
			}
			if sample.Metric == "packet_loss_pct" && sample.Value != 0 {
				t.Fatal("local listener lost probe")
			}
		case <-timer.C:
			t.Fatal("probe timed out")
		}
	}
	cancel()
	assertClosed(t, ch)
}
