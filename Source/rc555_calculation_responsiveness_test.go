//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC555CalculationBasisRunsHeavyReportOffUIThread(t *testing.T) {
	b, err := os.ReadFile("laboratory_calculation_basis.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "go func(pass LabPassRecord") || !strings.Contains(s, "wmBasisCalculationDone") {
		t.Fatal("calculation basis must execute the heavy report in a background goroutine and post completion to the window")
	}
	oldSync := "text := calculationPassReportWithModel(calculationBasisSelectedPass(), index, full, calculationBasisSelectedModel(index))"
	if strings.Contains(s, oldSync) {
		t.Fatal("synchronous calculation-basis hot path was reintroduced")
	}
	if !strings.Contains(s, "setCalculationBasisBusy(true)") || !strings.Contains(s, "setCalculationBasisBusy(false)") {
		t.Fatal("calculation basis must guard controls while the background calculation is running")
	}
}

func TestRC555BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.555-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
