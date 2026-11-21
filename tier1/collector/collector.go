package collector

import (
	"context"
	"sync"

	"kalki/tier1/collect"
)

// Run fans in local raw sources with backpressure. It joins every source on
// cancellation or sink failure. Phase 1 deliberately has no network sink.
func Run(ctx context.Context, sources []collect.Source, sink func(collect.RawSample) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := make(chan collect.RawSample)
	var wg sync.WaitGroup
	for _, source := range sources {
		stream := source.Stream(ctx)
		wg.Add(1)
		go func(ch <-chan collect.RawSample) {
			defer wg.Done()
			for sample := range ch {
				select {
				case out <- sample:
				case <-ctx.Done():
					// Drain until source cleanup closes its channel.
					for range ch {
					}
					return
				}
			}
		}(stream)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(out); close(done) }()
	for sample := range out {
		if err := sink(sample); err != nil {
			cancel()
			<-done
			return err
		}
	}
	return nil
}
