package medha

import (
	"encoding/json"
	"fmt"
	"kalki/tier1/collect"
	"math"
	"time"
)

// ValidateExplanation enforces the complete outgoing explanation contract.
func ValidateExplanation(x Explanation) error {
	for _, s := range []string{x.ID, x.Tenant, x.Collector, x.Region, x.Pattern, x.Severity, x.Summary, x.WhyFlagged, x.WhyNotThreshold, x.Action, x.WhyAction, x.Status} {
		if s == "" {
			return fmt.Errorf("empty explanation field")
		}
	}
	if _, err := time.Parse(collect.TimeLayout, x.Detected); err != nil {
		return err
	}
	if len(x.Signals) == 0 || len(x.Steps) == 0 || x.Correlated == nil || x.Confidence < 0 || x.Confidence > 1 || math.IsNaN(x.Confidence) || x.DetectionMS < 0 {
		return fmt.Errorf("invalid explanation evidence")
	}
	for _, s := range x.Signals {
		if s.Metric == "" || (s.Direction != "up" && s.Direction != "down") || s.Weight <= 0 || s.Weight > 1 {
			return fmt.Errorf("invalid signal")
		}
	}
	for _, s := range x.Steps {
		if s == "" {
			return fmt.Errorf("empty playbook step")
		}
	}
	_, err := json.Marshal(x)
	return err
}
