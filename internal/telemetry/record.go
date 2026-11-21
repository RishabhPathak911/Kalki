// Package telemetry defines the aggregate-only outbound vocabulary. Raw samples
// and free text intentionally have no representation here.
package telemetry

type Stats struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	P95   float64 `json:"p95"`
	Count float64 `json:"count"`
}

type Risk struct {
	Type            string `json:"type"`
	Severity        string `json:"severity"`
	Count           int    `json:"count"`
	WindowSec       int    `json:"window_sec"`
	SourcePseudonym string `json:"source_pseudonym,omitempty"`
}

type Record struct {
	Tenant          string           `json:"tenant"`
	CollectorID     string           `json:"collector_id"`
	Region          string           `json:"region"`
	CollectedAt     string           `json:"collected_at"`
	Source          string           `json:"source"`
	Metrics         map[string]Stats `json:"metrics"`
	RiskEvents      []Risk           `json:"risk_events"`
	RedactionCounts map[string]int   `json:"redaction_counts"`
}
