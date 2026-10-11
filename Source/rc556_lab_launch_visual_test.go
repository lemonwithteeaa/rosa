//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC556CalculationDialogLaunchPathIsBounded(t *testing.T) {
	b, err := os.ReadFile("laboratory_calculation_basis.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func openLaboratoryCalculationBasisDialog()")
	if start < 0 {
		t.Fatal("calculation dialog entry point missing")
	}
	end := strings.Index(s[start:], "\n}")
	if end < 0 {
		t.Fatal("calculation dialog body missing")
	}
	body := s[start : start+end]
	if strings.Contains(body, "saveLabDraft()") || strings.Contains(body, "loadLabPasses()") || strings.Contains(body, "sort.Slice") {
		t.Fatal("dialog launch path must not save the draft or scan/sort the complete pass archive")
	}
	for _, token := range []string{"calculationBasisCurrentSnapshot", "showCalculationBasisPreview", "case idBasisRun:", "calculationBasisRefresh()", "go func(pass LabPassRecord", "recover()"} {
		if !strings.Contains(s, token) {
			t.Fatalf("missing RC556 calculation guard %q", token)
		}
	}
}

func TestRC556LegendAndLoadingSurfaceContracts(t *testing.T) {
	legend, err := os.ReadFile("laboratory_visualization.go")
	if err != nil {
		t.Fatal(err)
	}
	loading, err := os.ReadFile("startup_loading.go")
	if err != nil {
		t.Fatal(err)
	}
	ls := string(legend)
	for _, token := range []string{"laboratoryLegendMeasuredCaptionWidth", "DT_CALCRECT", "items[:3], items[3:]", "резкое изменение > 30 %"} {
		if !strings.Contains(ls, token) {
			t.Fatalf("legend regression guard missing %q", token)
		}
	}
	ss := string(loading)
	for _, token := range []string{"rgb(247, 249, 252)", "ROSAStartupLoadingV3", `stages := []string{"Профиль", "Данные", "Модели", "Готово"}`} {
		if !strings.Contains(ss, token) {
			t.Fatalf("loading-surface regression guard missing %q", token)
		}
	}
}

func TestRC556BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.556-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
