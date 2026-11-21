package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNABHoldoutAndNoLabelLeak(t *testing.T) {
	cwd, _ := os.Getwd()
	if err := os.Chdir("../.."); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "realKnownCause"), 0700)
	os.MkdirAll(filepath.Join(root, "labels"), 0700)
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var data strings.Builder
	data.WriteString("timestamp,value\n")
	for i := 0; i < 100; i++ {
		value := 20 + i%2
		if i == 80 {
			value = 100
		}
		fmt.Fprintf(&data, "%s,%d\n", start.Add(time.Duration(i)*time.Minute).Format("2006-01-02 15:04:05"), value)
	}
	// A late duplicate must be normalized before the holdout boundary is chosen.
	fmt.Fprintf(&data, "%s,21\n", start.Add(time.Minute).Format("2006-01-02 15:04:05"))
	os.WriteFile(filepath.Join(root, "realKnownCause", "trace.csv"), []byte(data.String()), 0600)
	labels := filepath.Join(root, "labels", "combined_labels.json")
	os.WriteFile(labels, []byte(`{"realKnownCause/trace.csv":[]}`), 0600)
	a := nab(root)
	if a.Status != "evaluated" || a.Files != 1 || a.FP < 1 {
		t.Fatalf("%+v", a)
	}
	os.WriteFile(labels, []byte(fmt.Sprintf(`{"realKnownCause/trace.csv":["%s"]}`, start.Add(80*time.Minute).Format("2006-01-02 15:04:05"))), 0600)
	b := nab(root)
	if a.TP+a.FP != b.TP+b.FP || b.TP != 1 {
		t.Fatalf("labels changed detection or holdout missed spike: %+v %+v", a, b)
	}
}
