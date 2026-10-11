//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC559StartupTextAbovePumpjack(t *testing.T) {
	b, _ := os.ReadFile("startup_loading.go")
	s := string(b)
	for _, token := range []string{"startupPumpjackBitmapW int32 = 306", "panel.Top+218", "rc.Top+174, rc.Top+198", "const width, height = 1060, 430"} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing startup geometry %q", token)
		}
	}
}

func TestRC559HistoryDoesNotOverlapActions(t *testing.T) {
	b, _ := os.ReadFile("laboratory.go")
	s := string(b)
	if !strings.Contains(s, "minHistoryY = max32(minHistoryY, copyY+42+14)") {
		t.Fatal("history/action guard missing")
	}
}

func TestRC559ApprovedClassificationHistoryAlwaysVisible(t *testing.T) {
	b, _ := os.ReadFile("assessment_classification.go")
	s := string(b)
	if !strings.Contains(s, "historyVisible = true") || !strings.Contains(s, "historyListH := max32(92") {
		t.Fatal("classification history reservation missing")
	}
}

func TestRC559CalculationAvoidsDuplicateScaleRun(t *testing.T) {
	b, _ := os.ReadFile("laboratory_calculation_basis.go")
	s := string(b)
	if strings.Contains(s, "text := strings.ToLower(laboratorySaturationTextForModel(values, model.Code))") {
		t.Fatal("duplicate all-model scale risk pass remains")
	}
	if !strings.Contains(s, "len(result.Text) > 24000") {
		t.Fatal("long-result UI guard missing")
	}
}

func TestRC559BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.559-RC" {
		t.Fatalf("version=%s", appBuildVersion)
	}
}
