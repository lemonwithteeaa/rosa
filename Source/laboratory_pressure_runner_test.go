package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPressureReferenceRunnerBundleKeepsPressureDisabledUntilGolden(t *testing.T) {
	base := filepath.Join("reference", "tp_profile")
	for _, name := range []string{"pressure_reference.pqi", "phreeqc.dat", "verify_pressure_reference.py", "run_pressure_reference.ps1", "RUN_PRESSURE_REFERENCE.cmd", "accept_pressure_golden.ps1", "ACCEPT_PRESSURE_GOLDEN.cmd", "pressure_reference_manifest.txt"} {
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			t.Fatalf("missing pressure reference artifact %s: %v", name, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(base, "pressure_reference_manifest.txt"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"engine=USGS PHREEQC 3.8.6-17100", "status=PREPARED_NOT_EXECUTED", "database_sha256=", "pressure_effect_in_ROSA_remains_disabled"} {
		if !strings.Contains(s, want) {
			t.Fatalf("manifest missing %q", want)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "pressure_selected.tsv")); err == nil {
		t.Fatal("pressure_selected.tsv must not be bundled as if it were an executed golden")
	}
	if _, err := os.Stat(filepath.Join(base, "pressure_golden.tsv")); err == nil {
		t.Fatal("pressure_golden.tsv must not be bundled before an external verified run")
	}

	runner, err := os.ReadFile(filepath.Join(base, "run_pressure_reference.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	rs := string(runner)
	for _, want := range []string{"PHREEQC_EXE", "phreeqc.exe", "engine\\phreeqc.exe", "portable\\phreeqc.exe"} {
		if !strings.Contains(rs, want) {
			t.Fatalf("runner missing portable search %q", want)
		}
	}
	accept, err := os.ReadFile(filepath.Join(base, "accept_pressure_golden.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	as := string(accept)
	for _, want := range []string{"PRESSURE_REFERENCE_OK", "database_sha256", "input_sha256", "ACCEPTED_EXTERNAL_GOLDEN"} {
		if !strings.Contains(as, want) {
			t.Fatalf("acceptance gate missing %q", want)
		}
	}
}
