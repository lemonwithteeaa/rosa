//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPressureComparisonBundlePreparedWithoutFakeResult(t *testing.T) {
	base := filepath.Join("reference", "tp_profile")
	for _, name := range []string{"compare_pressure_golden_to_rosa.py", "COMPARE_PRESSURE_GOLDEN.cmd", "README_COMPARE_PRESSURE_GOLDEN.txt"} {
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			t.Fatalf("missing pressure comparison bundle file %s: %v", name, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(base, "compare_pressure_golden_to_rosa.py"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"ACCEPTED_EXTERNAL_GOLDEN", "ROSA_pressure_delta_current=+0", "PRESSURE_EFFECT_DETECTED_EXTERNALLY__ROSA_IMPLEMENTATION_REQUIRED_BEFORE_ENABLE"} {
		if !strings.Contains(s, want) {
			t.Fatalf("comparison gate lost marker %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "pressure_comparison_report.txt")); err == nil {
		t.Fatal("comparison report must not be bundled before an accepted external golden exists")
	}
}
