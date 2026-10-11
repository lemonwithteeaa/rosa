//go:build windows

package main

import (
	"archive/zip"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestRC743ApprovedWorkbookUsesArchivedMeasurementsAndFormulas(t *testing.T) {
	r := FullReportRecord{
		ObjectLabel: "Учебный объект", ForecastDays: 1080, NetBenefitK: 12000,
		Sections: map[string]string{"section_2": "РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\nМодель солеотложений: davies\r\nНАПРАВЛЕНИЕ: СОЛЕОТЛОЖЕНИЯ\r\n" +
			reportLaboratoryIndicatorLine("Кальций Ca²⁺", "520", "мг/л") +
			"РАСЧЁТЫ, ФОРМУЛЫ, ПОДСТАНОВКА И РЕЗУЛЬТАТЫ\r\nПодстановка: I = ½·(очень длинная строка)."},
	}
	// Use the exact catalog label rather than assuming a particular translation.
	group, _ := labGroupByCode("scale")
	for _, field := range group.Fields {
		if field.Key == "ca" {
			r.Sections["section_2"] = strings.Replace(r.Sections["section_2"], "Кальций Ca²⁺", field.Label, 1)
		}
	}
	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := writeApprovedRegistryExportPayload(path, "xlsx", r, ""); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	parts := map[string]string{}
	for _, f := range z.File {
		if f.Name == "xl/worksheets/sheet3.xml" || f.Name == "xl/worksheets/sheet9.xml" || f.Name == "xl/workbook.xml" {
			reader, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			data, e := io.ReadAll(reader)
			_ = reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			parts[f.Name] = string(data)
		}
	}
	if strings.Contains(parts["xl/worksheets/sheet3.xml"], "очень длинная строка") {
		t.Fatal("verbose prose calculation leaked into the compact laboratory sheet")
	}
	calc := parts["xl/worksheets/sheet9.xml"]
	if !strings.Contains(calc, "<f>B6/1000/C6</f>") || !strings.Contains(calc, "<f>SUM(F6:F6)</f>") || !strings.Contains(calc, "<v>520</v>") {
		t.Fatalf("archived input or editable ionic strength formula missing: %s", calc)
	}
	if !strings.Contains(parts["xl/workbook.xml"], `forceFullCalc="1"`) {
		t.Fatal("Excel recalculation was not requested")
	}
}

func TestRC743WordKeepsIndicatorWithUnit(t *testing.T) {
	line := reportLaboratoryIndicatorLine("Кальций Ca²⁺", "520", "мг/л")
	path := filepath.Join(t.TempDir(), "report.docx")
	if err := writeLaboratoryDOCXReport(path, "ОТЧЁТ\r\nНАПРАВЛЕНИЕ: СОЛЕОТЛОЖЕНИЯ\r\n"+line); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name != "word/document.xml" {
			continue
		}
		reader, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		data, e := io.ReadAll(reader)
		_ = reader.Close()
		if e != nil {
			t.Fatal(e)
		}
		body := string(data)
		for _, want := range []string{`w:line="240"`, `<w:keepLines/>`, `<w:b/>`, "520\u00a0мг/л", `w:ascii="Times New Roman"`, `w:sz w:val="24"`} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in DOCX", want)
			}
		}
		return
	}
	t.Fatal("Word document missing")
}
