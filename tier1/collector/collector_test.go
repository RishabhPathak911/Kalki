package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"kalki/tier1/collect"
)

func TestSinkFailureStopsSources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	want := errors.New("sink failed")
	err := Run(ctx, []collect.Source{&collect.LocalSource{Interval: time.Millisecond}}, func(collect.RawSample) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("source failed to stop before deadline")
	}
}
