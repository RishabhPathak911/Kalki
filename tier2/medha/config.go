package medha

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

type Threshold struct {
	Warn             float64 `yaml:"warn" json:"warn"`
	Critical         float64 `yaml:"critical" json:"critical"`
	MultipleWarn     float64 `yaml:"baseline_multiple_warn" json:"baseline_multiple_warn,omitempty"`
	MultipleCritical float64 `yaml:"baseline_multiple_critical" json:"baseline_multiple_critical,omitempty"`
}
type Heartbeat struct {
	Stale   float64 `yaml:"stale_after_s" json:"stale_after_s"`
	Offline float64 `yaml:"offline_after_s" json:"offline_after_s"`
}
type Config struct {
	CPU          Threshold `yaml:"cpu_pct" json:"cpu_pct"`
	CollectorCPU Threshold `yaml:"collector_cpu_pct" json:"collector_cpu_pct"`
	Memory       Threshold `yaml:"mem_pct" json:"mem_pct"`
	Disk         Threshold `yaml:"disk_pct" json:"disk_pct"`
	Latency      Threshold `yaml:"latency_ms" json:"latency_ms"`
	Loss         Threshold `yaml:"packet_loss_pct" json:"packet_loss_pct"`
	Heartbeat    Heartbeat `yaml:"heartbeat" json:"heartbeat"`
	Persistence  int       `yaml:"persistence_windows" json:"persistence_windows"`
	Warmup       int       `yaml:"warmup_samples" json:"warmup_samples"`
	Alpha        float64   `yaml:"ewma_alpha" json:"ewma_alpha"`
	RaiseZ       float64   `yaml:"raise_z" json:"raise_z"`
	ClearZ       float64   `yaml:"clear_z" json:"clear_z"`
	ClearRatio   float64   `yaml:"clear_ratio" json:"clear_ratio"`
	Cooldown     float64   `yaml:"cooldown_s" json:"cooldown_s"`
}
type Playbook struct {
	Pattern string   `yaml:"pattern" json:"pattern"`
	Action  string   `yaml:"recommended_action" json:"recommended_action"`
	Why     string   `yaml:"why" json:"why"`
	Steps   []string `yaml:"steps" json:"steps"`
}
type Books struct {
	Playbooks []Playbook `yaml:"playbooks" json:"playbooks"`
}

func DefaultConfig() Config {
	return Config{CPU: Threshold{Warn: 80, Critical: 95}, CollectorCPU: Threshold{Warn: 15, Critical: 30}, Memory: Threshold{Warn: 85, Critical: 95}, Disk: Threshold{Warn: 85, Critical: 95}, Latency: Threshold{Warn: 150, Critical: 400}, Loss: Threshold{Warn: 2, Critical: 10}, Heartbeat: Heartbeat{15, 45}, Persistence: 3, Warmup: 30, Alpha: .1, RaiseZ: 4, ClearZ: 2, ClearRatio: .8, Cooldown: 60}
}
func readYAML(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	d := yaml.NewDecoder(f)
	d.KnownFields(true)
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one configuration document")
	}
	return nil
}
func Load(thresholds, playbooks string) (Config, Books, error) {
	c := DefaultConfig()
	b := Books{}
	if err := readYAML(thresholds, &c); err != nil {
		return c, b, err
	}
	if err := readYAML(playbooks, &b); err != nil {
		return c, b, err
	}
	if _, err := json.Marshal(c); err != nil {
		return c, b, fmt.Errorf("non-finite Medha threshold")
	}
	if c.Warmup < 30 || c.Warmup > 120 || c.Persistence < 1 || c.Persistence > 60 || c.Alpha <= 0 || c.Alpha > 1 || c.ClearZ <= 0 || c.ClearZ >= c.RaiseZ || c.ClearRatio <= 0 || c.ClearRatio >= 1 || c.Cooldown < 0 || c.Heartbeat.Stale <= 0 || c.Heartbeat.Offline <= c.Heartbeat.Stale || c.Heartbeat.Offline >= 60 {
		return c, b, fmt.Errorf("invalid Medha thresholds")
	}
	for _, t := range []Threshold{c.CPU, c.CollectorCPU, c.Memory, c.Disk, c.Latency, c.Loss} {
		if t.Warn <= 0 || t.Critical <= t.Warn {
			return c, b, fmt.Errorf("invalid metric threshold")
		}
	}
	found := map[string]bool{}
	for _, p := range b.Playbooks {
		if found[p.Pattern] || p.Action == "" || p.Why == "" || len(p.Steps) == 0 {
			return c, b, fmt.Errorf("invalid playbook")
		}
		found[p.Pattern] = true
		for _, step := range p.Steps {
			if step == "" {
				return c, b, fmt.Errorf("empty playbook step")
			}
		}
	}
	for _, p := range []string{"network_path_issue", "local_workload", "possible_attack_exfiltration", "memory_leak", "capacity_risk", "shared_dependency", "collector_down", "buffer_full"} {
		if !found[p] {
			return c, b, fmt.Errorf("missing playbook %s", p)
		}
	}
	return c, b, nil
}
func (c Config) threshold(metric string) Threshold {
	switch metric {
	case "cpu_pct":
		return c.CPU
	case "collector_cpu_pct":
		return c.CollectorCPU
	case "mem_pct":
		return c.Memory
	case "disk_pct":
		return c.Disk
	case "latency_ms":
		return c.Latency
	case "packet_loss_pct":
		return c.Loss
	}
	return Threshold{}
}
