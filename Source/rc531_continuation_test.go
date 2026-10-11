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

func zipEntryText(t *testing.T, path, name string) string {
	t.Helper()
	book, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	for _, entry := range book.File {
		if entry.Name != name {
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
		return string(body)
	}
	t.Fatalf("entry %s not found", name)
	return ""
}

func TestRC531PasswordStartsMaskedAndHasToggle(t *testing.T) {
	source, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{"ID_AUTH_SHOW_PASSWORD", "authSetPasswordMask(true)", "authSetPasswordMask(authPasswordVisible)", `"Показать"`, `"Скрыть"`, "ES_PASSWORD|ES_CENTER|ES_MULTILINE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("password contract missing %q", want)
		}
	}
}

func TestRC531ChartInteractionContracts(t *testing.T) {
	source, err := os.ReadFile("laboratory_complex_analysis.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{"CreatePopupMenu", "Удалить из анализа…", "Добавить линию тренда…", "addSessionTrashItem", "complexSelectSeriesAtPoint(point) < 0", `modalCreate("BUTTON", "−"`, `modalCreate("BUTTON", "+"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("chart interaction missing %q", want)
		}
	}
	groups := [][]LabPassRecord{{{ObjectKey: "A", Date: "01.01.2026", Groups: map[string]map[string]string{"corrosion": {"ph": "5"}}}}}
	metric := complexMetric{label: "pH", group: "corrosion", key: "ph"}
	if got := complexSeriesHitTest(groups, metric, RECT{Left: 0, Top: 0, Right: 1000, Bottom: 600}, 900, 50); got != -1 {
		t.Fatalf("empty-area hit=%d", got)
	}
}

func TestRC531ThresholdBlockPrecedesLegend(t *testing.T) {
	source, err := os.ReadFile("laboratory_complex_multi.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	threshold := strings.Index(text, `thresholdCaption = "Порог внимания:`)
	legend := strings.Index(text, `top := plot.Bottom + 100`)
	if threshold < 0 || legend < 0 {
		t.Fatal("threshold/legend layout contract missing")
	}
	if !strings.Contains(text, "plot.Bottom + 50") || !strings.Contains(text, "plot.Bottom + 74") {
		t.Fatal("threshold and orange note do not have reserved rows")
	}
}

func TestRC531ExcelSinglePointAndNumericDates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "single.xlsx")
	passes := []LabPassRecord{{ObjectKey: "A", Date: "27.09.2026", Groups: map[string]map[string]string{"scale": {"mineralization": "120"}, "corrosion": {"corrosion_rate": "0.2"}, "overall": {"watercut": "70"}}, SharedValues: map[string]string{"watercut": "70"}}}
	if err := writeInfographicDependenciesXLSX(path, passes, false, 0); err != nil {
		t.Fatal(err)
	}
	chart := zipEntryText(t, path, "xl/charts/chart2.xml")
	if !strings.Contains(chart, "<c:scatterChart>") || !strings.Contains(chart, "<c:xVal>") {
		t.Fatalf("single-point chart invalid: %s", chart)
	}
	sheet := zipEntryText(t, path, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheet, `s="5"><v>`) {
		t.Fatal("visible Excel dates are not numeric date cells")
	}
}

func TestRC531IndividualAndFullCalculations(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "OBJ-23", Date: "27.09.2026", Source: "Протокол 17", Groups: map[string]map[string]string{
		"corrosion": {"ph": "5.8", "chlorides": "12000", "corrosion_rate": "0.2"},
		"gases":     {"sample_temp": "8", "hydrate_temp": "12"},
		"scale":     {"ca": "100", "co3": "20"},
	}, Units: map[string]map[string]string{}}
	individual := calculationPassReport(pass, 1, false)
	for _, want := range []string{"КОРРОЗИЯ", "ОЦЕНКА РИСКА", "Недостающие параметры", "Объект: OBJ-23", "Дата исходных данных: 27.09.2026"} {
		if !strings.Contains(individual, want) {
			t.Fatalf("individual calculation missing %q", want)
		}
	}
	full := calculationPassReport(pass, 0, true)
	for _, section := range calculationBasisSections {
		if !strings.Contains(full, strings.ToUpper(section.Name)) {
			t.Fatalf("full calculation missing %s", section.Name)
		}
	}
	if !strings.Contains(full, "ΔT = Tфакт − Tгидр = 8 − 12 = -4 °C") {
		t.Fatal("concrete gas calculation missing")
	}
}

func TestRC531CalculationExports(t *testing.T) {
	pass := LabPassRecord{ObjectKey: "OBJ", Date: "27.09.2026", Source: "LAB", Groups: map[string]map[string]string{"corrosion": {"ph": "5.9"}, "scale": {"ca": "100", "co3": "20"}}, Units: map[string]map[string]string{}}
	xlsx := filepath.Join(t.TempDir(), "calc.xlsx")
	if err := writeCalculationWorkbookXLSX(xlsx, pass, calculationExportIndexes(true, 0), ""); err != nil {
		t.Fatal(err)
	}
	workbook := zipEntryText(t, xlsx, "xl/workbook.xml")
	if strings.Count(workbook, "<sheet ") != 1+len(calculationBasisSections) {
		t.Fatalf("unexpected sheet count: %s", workbook)
	}
	docx := filepath.Join(t.TempDir(), "calc.docx")
	if err := writeLaboratoryDOCXReport(docx, calculationPassReport(pass, 0, true)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zipEntryText(t, docx, "word/footer1.xml"), "PAGE") {
		t.Fatal("DOCX page-number footer missing")
	}
}
