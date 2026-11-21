package main

import (
	"kalki/internal/scale"
	"testing"
	"time"
)

func TestScale(t *testing.T) {
	r, e := scale.Run(scale.Options{Collectors: 4, Tenants: 2, Duration: 150 * time.Millisecond, Policy: "../config/allowlist.yaml"})
	if e != nil || r.Sent < 4 || r.Failed != 0 || r.Source != "SIMULATED" || r.Peak <= 0 {
		t.Fatalf("%+v %v", r, e)
	}
}
