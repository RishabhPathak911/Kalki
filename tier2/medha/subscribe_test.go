package medha

import (
	"testing"
	"time"
)

func TestSubscriptionIsolationAndFirstHealth(t *testing.T) {
	e := testEngine(t)
	alpha, cancel := e.Subscribe("tenant_alpha")
	beta, stop := e.Subscribe("tenant_beta")
	defer stop()
	now := time.Now()
	e.Observe(observation("tenant_alpha", "host_12345678", now, quiet, false), now, false)
	select {
	case <-alpha:
	default:
		t.Fatal("event must wake subscriber before Observe returns")
	}
	select {
	case <-beta:
		t.Fatal("cross-tenant notification")
	default:
	}
	events, _ := e.Events("tenant_alpha", 0)
	health := false
	for _, event := range events {
		if event.Kind == "health" {
			health = true
		}
	}
	if !health {
		t.Fatal("first collector observation must emit health")
	}
	replayed, gap := e.Events("tenant_alpha", 1000000)
	if !gap || len(replayed) != len(events) {
		t.Fatal("previous process SSE cursor suppressed new events")
	}
	for i := 0; i < 1000; i++ {
		e.Notify("tenant_alpha")
	}
	<-alpha
	cancel()
	cancel()
	e.Notify("tenant_alpha")
	select {
	case <-alpha:
		t.Fatal("unsubscribed channel notified")
	default:
	}
}
