package collect

import (
	"context"
	"math"
	"sync/atomic"
	"time"
)

// DemoFault is opt-in, process-local and automatically expires.
type DemoFault struct {
	Until       atomic.Int64
	StressUntil atomic.Int64
}

func (d *DemoFault) Degraded() bool { return d != nil && time.Now().UnixNano() < d.Until.Load() }
func (d *DemoFault) Stress(ctx context.Context) bool {
	now := time.Now()
	end := now.Add(15 * time.Second).UnixNano()
	old := d.StressUntil.Load()
	if old > now.UnixNano() || !d.StressUntil.CompareAndSwap(old, end) {
		return false
	}
	go func() {
		value := 1.1
		for time.Now().UnixNano() < end {
			select {
			case <-ctx.Done():
				d.StressUntil.Store(0)
				return
			default:
			}
			// One worker, yielding between bounded chunks. On large hosts this is a
			// modest real spike, not a promise to saturate every logical CPU.
			for i := 0; i < 100000; i++ {
				value = math.Sqrt(value + 1.01)
			}
		}
		_ = value
		d.StressUntil.Store(0)
	}()
	return true
}
