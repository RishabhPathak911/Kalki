package medha

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationRejectsUnsafeWarmupAndNonFiniteThresholds(t *testing.T) {
	raw, err := os.ReadFile("../../config/thresholds.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []struct{ from, to string }{{"warmup_samples: 30", "warmup_samples: 29"}, {"ewma_alpha: 0.1", "ewma_alpha: .nan"}, {"clear_z: 2", "clear_z: 5"}} {
		path := filepath.Join(t.TempDir(), "thresholds.yaml")
		if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), change.from, change.to)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load(path, "../../config/playbooks.yaml"); err == nil {
			t.Fatal("unsafe config accepted", change.to)
		}
	}
}
