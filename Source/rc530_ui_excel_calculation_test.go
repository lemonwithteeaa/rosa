//go:build windows

package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRC530RequestedUIContracts(t *testing.T) {
	auth, _ := os.ReadFile("auth.go")
	loading, _ := os.ReadFile("startup_loading.go")
	mainSource, _ := os.ReadFile("main.go")
	lab, _ := os.ReadFile("laboratory.go")
	chart, _ := os.ReadFile("laboratory_complex_multi.go")
	checks := map[string]bool{
		"password vertical centring":   strings.Contains(string(auth), "ES_PASSWORD|ES_CENTER|ES_MULTILINE") && strings.Contains(string(auth), "textRect := RECT{Left: 8, Top: 8"),
		"responsive loading canvas":    strings.Contains(string(loading), "case 0x0005: // WM_SIZE") && strings.Contains(string(loading), "const width, height = 940, 372"),
		"trash in global header":       strings.Contains(string(mainSource), `sessionTrashButton = create("BUTTON", "Корзина · 0"`) && !strings.Contains(string(mainSource), `sessionTrashButton = button(ID_HISTORY`),
		"orange point explanation":     strings.Contains(string(chart), "Рыжая точка — изменение более 30%"),
		"calculation on every lab tab": strings.Contains(string(lab), "show(labCalculationBasisButton, true)"),
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("missing contract: %s", name)
		}
	}
}

func TestRC530CalculationAndReportOptions(t *testing.T) {
	if len(labInfoOptionLabels) != 6 || labInfoOptionLabels[5] != "Расчёты, формулы и примеры" {
		t.Fatalf("unexpected report options: %#v", labInfoOptionLabels)
	}
	text := calculationBasisTextForValues(5, map[string]string{"sample_temp": "8", "hydrate_temp": "12"}, nil)
	if !strings.Contains(text, "ΔT = Tфакт − Tгидр = 8 − 12 = -4 °C") {
		t.Fatalf("calculation example missing: %s", text)
	}
	salt := calculationScaleAllModels(map[string]string{"ca": "100", "co3": "20"})
	for _, model := range laboratoryScaleModels {
		if !strings.Contains(salt, model.Name) {
			t.Fatalf("model missing: %s", model.Name)
		}
	}
}

func TestRC530ExcelUsesIndependentXYTimeSeries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dependencies.xlsx")
	passes := []LabPassRecord{
		{ObjectKey: "A", Date: "01.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "10"}}},
		{ObjectKey: "A", Date: "03.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "12"}}},
		{ObjectKey: "B", Date: "02.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "20"}}},
		{ObjectKey: "B", Date: "04.01.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "22"}}},
	}
	if err := writeInfographicDependenciesXLSX(path, passes, false, 0); err != nil {
		t.Fatal(err)
	}
	book, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for _, entry := range book.File {
		if entry.Name != "xl/charts/chart2.xml" {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(body)
		if !strings.Contains(xml, "<c:scatterChart>") || strings.Contains(xml, "<c:lineChart>") || strings.Count(xml, "<c:xVal>") < 2 {
			t.Fatalf("invalid independent XY chart: %s", xml)
		}
		return
	}
	t.Fatal("chart2.xml missing")
}
