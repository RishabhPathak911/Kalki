// Package pipeline enforces collect -> filter -> aggregate -> analyse -> policy
// -> approved sink. Delivery is deliberately local-only until Phase 3.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kalki/tier1/aggregate"
	"kalki/tier1/analyse"
	"kalki/tier1/collect"
	"kalki/tier1/collector"
	"kalki/tier1/filter"
	"kalki/tier1/policy"
)

type Options struct {
	CollectorID string // Optional opaque onboarding ID; raw host identities remain local.
	Tenant      string
	NodeID      string
	Region      string
	Identity    *filter.Identity
	Gate        *policy.Gate
	// RawSink is an explicit LOCAL diagnostic tap, never a network sender.
	RawSink func(collect.RawSample) error
	Report  func(error)
}

func Run(ctx context.Context, sources []collect.Source, options Options, sink func(policy.Approved) error) error {
	if options.Identity == nil || options.Gate == nil || sink == nil {
		return fmt.Errorf("privacy pipeline: missing configuration")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	f := &filter.Filter{Identity: options.Identity}
	collectorID := options.Identity.Pseudonym("host", options.NodeID)
	if options.CollectorID != "" {
		collectorID = options.CollectorID
	}
	clean := make(chan filter.Sample)
	done := make(chan error, 1)
	go func() {
		err := collector.Run(ctx, sources, func(raw collect.RawSample) error {
			if options.RawSink != nil {
				if err := options.RawSink(raw); err != nil {
					return err
				}
			}
			if raw.Hostname == "" {
				raw.Hostname = options.NodeID
			}
			sample, err := f.Process(raw, time.Now())
			if err != nil {
				if options.Report != nil {
					options.Report(err)
					return nil
				}
				return err
			}
			select {
			case clean <- sample:
				return nil
			case <-ctx.Done():
				return nil
			}
		})
		done <- err
		close(clean)
	}()
	var aggregator aggregate.Aggregator
	var detector analyse.Security
	flush := func() error {
		now := time.Now()
		for _, window := range aggregator.Flush(options.Tenant, collectorID, options.Region) {
			window.Record.RiskEvents = append(window.Record.RiskEvents, detector.Observe(window.Record.Source, window.Failures, now)...)
			data, err := json.Marshal(window.Record)
			if err != nil {
				return fmt.Errorf("privacy pipeline: aggregate encoding failed")
			}
			approved, err := options.Gate.Approve(data)
			if err != nil {
				return err
			}
			if err := sink(approved); err != nil {
				return err
			}
		}
		return nil
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	// Every return joins the producer. This also stops source goroutines when an
	// approved sink fails, without leaving them blocked on the fan-in channel.
	stop := func() {
		cancel()
		for range clean {
		}
		<-done
	}
	for {
		select {
		case sample, ok := <-clean:
			if !ok {
				if err := <-done; err != nil {
					return err
				}
				return flush()
			}
			if err := aggregator.Add(sample); err != nil {
				stop()
				return err
			}
		case <-ticker.C:
			if err := flush(); err != nil {
				stop()
				return err
			}
		}
	}
}
