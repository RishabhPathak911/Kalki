package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingEvidenceFailsHonestly(t *testing.T) {
	d := t.TempDir()
	if nab(d).Status != "datasets_not_downloaded" {
		t.Fatal("missing NAB mislabeled")
	}
	if latencyCheck(filepath.Join(d, "absent")).Status != "FAIL" {
		t.Fatal("missing browser evidence passed")
	}
	p := filepath.Join(d, "bad.json")
	os.WriteFile(p, []byte(`{"samples_ms":[]}`), 0600)
	if latencyCheck(p).Status != "FAIL" {
		t.Fatal("empty evidence passed")
	}
}
