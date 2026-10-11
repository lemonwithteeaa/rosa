package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC245TargetIsConfigurableNotHardcodedAsForecast(t *testing.T) {
	b, err := os.ReadFile("target_runlife.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"defaultTargetRunlifeDays = 1000.0", "ObjectDays", "индивидуальная настройка объекта", "не является жёстким нормативом"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestRC245MRPUIHasTargetControls(t *testing.T) {
	b, _ := os.ReadFile("mrp_forecast_ui.go")
	s := string(b)
	for _, want := range []string{"Целевой ориентир наработки", "Сохранить цель", "Сбросить", "ID_MRP_TARGET_SAVE", "ID_MRP_TARGET_RESET"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestRC245UserFacingMRPTextAvoidsEnglishSurvivalBaseline(t *testing.T) {
	for _, name := range []string{"mrp_forecast_ui.go", "workflow_pages.go"} {
		b, _ := os.ReadFile(name)
		s := strings.ToLower(string(b))
		if strings.Contains(s, "survival-набор") || strings.Contains(s, "median baseline") || strings.Contains(s, "gate достаточности/backtest") {
			t.Fatalf("legacy english UI text remains in %s", name)
		}
	}
}
