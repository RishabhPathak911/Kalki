package collect

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// Bandwidth tracks each interface independently, so one counter reset does not
// turn into a giant unsigned delta. First observations have no measurable rate.
type Bandwidth struct {
	previous map[string]psnet.IOCountersStat
	at       time.Time
}

func (b *Bandwidth) Observe(now time.Time, counters []psnet.IOCountersStat) (float64, bool) {
	next := make(map[string]psnet.IOCountersStat, len(counters))
	var bytes float64
	valid := false
	for _, c := range counters {
		next[c.Name] = c
		p, ok := b.previous[c.Name]
		if ok && c.BytesSent >= p.BytesSent && c.BytesRecv >= p.BytesRecv {
			bytes += float64(c.BytesSent-p.BytesSent) + float64(c.BytesRecv-p.BytesRecv)
			valid = true
		}
	}
	elapsed := now.Sub(b.at).Seconds()
	b.previous, b.at = next, now
	if !valid || elapsed <= 0 {
		return 0, false
	}
	return bytes * 8 / elapsed / 1e6, true
}

type ProbeWindow struct{ results []bool }

func (p *ProbeWindow) Observe(success bool) float64 {
	p.results = append(p.results, success)
	if len(p.results) > 10 {
		p.results = p.results[1:]
	}
	failed := 0
	for _, ok := range p.results {
		if !ok {
			failed++
		}
	}
	return float64(failed) * 100 / float64(len(p.results))
}

type LocalSource struct {
	Demo     *DemoFault
	Interval time.Duration
	Targets  []string
	Report   Reporter
}

func (*LocalSource) Name() string { return "local" }
func (s *LocalSource) Stream(ctx context.Context) <-chan RawSample {
	out := make(chan RawSample)
	go func() {
		defer close(out)
		if err := validateInterval(s.Interval); err != nil {
			report(s.Report, err)
			return
		}
		root := "/"
		if runtime.GOOS == "windows" {
			root = os.Getenv("SystemDrive") + "\\"
			if root == "\\" {
				root = "C:\\"
			}
		}
		var bandwidth Bandwidth
		var prevCPU *cpu.TimesStat
		self, selfErr := process.NewProcess(int32(os.Getpid()))
		report(s.Report, selfErr)
		var processCPU ProcessCPU
		windows := make(map[string]*ProbeWindow)
		for _, target := range s.Targets {
			windows[target] = &ProbeWindow{}
		}
		ticker := time.NewTicker(s.Interval)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			now := time.Now()
			send := func(metric string, value float64, stream string) bool {
				if !finite(value) || value < 0 {
					return true
				}
				if s.Demo != nil && (metric == "latency_ms" || metric == "packet_loss_pct" || metric == "service_up") {
					until := s.Demo.Until.Load()
					if until > 0 && now.UnixNano() >= until && now.UnixNano() < until+int64(30*time.Second) {
						// Explicitly close the simulated fault series through normal
						// hysteresis, while retaining its SIMULATED provenance.
						if !emit(ctx, out, RawSample{CollectedAt: Timestamp(now), Source: Simulated, Metric: metric, Value: value, StreamID: stream}) {
							return false
						}
					}
				}
				return emit(ctx, out, RawSample{CollectedAt: Timestamp(now), Source: Live, Metric: metric, Value: value, StreamID: stream})
			}
			if stats, err := cpu.TimesWithContext(ctx, false); err != nil {
				report(s.Report, fmt.Errorf("CPU: %w", err))
			} else if len(stats) > 0 {
				c := stats[0]
				if prevCPU != nil {
					total := cpuTotal(c) - cpuTotal(*prevCPU)
					idle := (c.Idle + c.Iowait) - (prevCPU.Idle + prevCPU.Iowait)
					if total > 0 && idle >= 0 && idle <= total {
						if !send("cpu_pct", (total-idle)*100/total, "") {
							return
						}
					}
				}
				prevCPU = &c
			}
			if self != nil {
				if times, err := self.TimesWithContext(ctx); err != nil {
					report(s.Report, fmt.Errorf("collector CPU: %w", err))
				} else if value, ok := processCPU.Observe(now, times.User+times.System, runtime.NumCPU()); ok {
					if !send("collector_cpu_pct", value, "") {
						return
					}
				}
			}
			if v, err := mem.VirtualMemoryWithContext(ctx); err != nil {
				report(s.Report, fmt.Errorf("memory: %w", err))
			} else if !send("mem_pct", v.UsedPercent, "") {
				return
			}
			if v, err := disk.UsageWithContext(ctx, root); err != nil {
				report(s.Report, fmt.Errorf("disk: %w", err))
			} else if !send("disk_pct", v.UsedPercent, "") {
				return
			}
			if v, err := psnet.IOCountersWithContext(ctx, true); err != nil {
				report(s.Report, fmt.Errorf("network: %w", err))
			} else if rate, ok := bandwidth.Observe(now, v); ok {
				if !send("bandwidth_mbps", rate, "") {
					return
				}
			}
			// Share a bounded budget across targets, keeping a failed probe from
			// holding up the next sampling tick.
			budget := s.Interval / time.Duration(len(s.Targets)+1)
			if budget <= 0 {
				budget = time.Nanosecond
			}
			if budget > time.Second {
				budget = time.Second
			}
			for _, target := range s.Targets {
				if s.Demo.Degraded() {
					for metric, value := range map[string]float64{"latency_ms": 600, "packet_loss_pct": 60, "service_up": 0} {
						if !emit(ctx, out, RawSample{CollectedAt: Timestamp(now), Source: Simulated, Metric: metric, Value: value, StreamID: target}) {
							return
						}
					}
					continue
				}
				start := time.Now()
				conn, err := (&net.Dialer{Timeout: budget}).DialContext(ctx, "tcp", target)
				latency := float64(time.Since(start).Microseconds()) / 1000
				success := err == nil
				if success {
					report(s.Report, conn.Close())
				}
				if ctx.Err() != nil {
					return
				}
				up := 0.0
				if success {
					up = 1
					if !send("latency_ms", latency, target) {
						return
					}
				}
				if !send("service_up", up, target) || !send("packet_loss_pct", windows[target].Observe(success), target) {
					return
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return out
}

// ProcessCPU expresses collector CPU as a percentage of total logical CPU
// capacity, comparable with host cpu_pct. The first sample has no rate.
type ProcessCPU struct {
	at      time.Time
	seconds float64
}

func (p *ProcessCPU) Observe(now time.Time, seconds float64, cores int) (float64, bool) {
	previous, at := p.seconds, p.at
	p.seconds, p.at = seconds, now
	if at.IsZero() || !now.After(at) || seconds < previous || cores < 1 {
		return 0, false
	}
	v := (seconds - previous) / now.Sub(at).Seconds() / float64(cores) * 100
	if v > 100 {
		v = 100
	}
	return v, finite(v)
}

func cpuTotal(c cpu.TimesStat) float64 {
	// Guest time is already included in User/Nice on Linux.
	return c.User + c.System + c.Idle + c.Nice + c.Iowait + c.Irq + c.Softirq + c.Steal
}
