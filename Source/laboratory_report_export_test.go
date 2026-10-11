//go:build windows

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaboratoryReportExportsSameContentToWordAndExcel(t *testing.T) {
	report := "РОСА — ОТЧЁТ\r\n\r\nОбъект: TEST-42\r\nВЫВОД\r\nВероятность выпадения солей требует уточнения."
	cases := []struct {
		ext, part string
		write     func(string, string) error
	}{
		{"docx", "word/document.xml", writeLaboratoryDOCXReport},
		{"xlsx", "xl/worksheets/sheet1.xml", writeLaboratoryXLSXReport},
	}
	for _, item := range cases {
		path := filepath.Join(t.TempDir(), "report."+item.ext)
		if err := item.write(path, report); err != nil {
			t.Fatal(err)
		}
		z, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, file := range z.File {
			if file.Name != item.part {
				continue
			}
			r, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.Contains(text, "TEST-42") || !strings.Contains(text, "требует уточнения") {
				t.Fatalf("%s lost report content", item.ext)
			}
			found = true
		}
		_ = z.Close()
		if !found {
			t.Fatalf("%s missing %s", item.ext, item.part)
		}
	}
}

func TestTechnologistHeadingEmphasisInExport(t *testing.T) {
	report := "РОСА\nЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА\nОбычный текст"
	for _, item := range []struct {
		ext, part, want string
		write           func(string, string) error
	}{
		{"docx", "word/document.xml", `<w:b/><w:rFonts w:ascii="Times New Roman"`, writeLaboratoryDOCXReport},
		{"xlsx", "xl/worksheets/sheet1.xml", `<c r="A2" s="2"`, writeLaboratoryXLSXReport},
		{"xlsx", "xl/styles.xml", `<font><b/><name val="Times New Roman"/><sz val="12"/>`, writeLaboratoryXLSXReport},
	} {
		path := filepath.Join(t.TempDir(), "report."+item.ext)
		if err := item.write(path, report); err != nil {
			t.Fatal(err)
		}
		z, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range z.File {
			if f.Name == item.part {
				reader, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				found = strings.Contains(string(data), item.want)
			}
		}
		z.Close()
		if !found {
			t.Fatalf("%s missing requested emphasis in %s", item.ext, item.part)
		}
	}
}

func TestLongExcelExplanationPreservesUnicodeAndFitsRows(t *testing.T) {
	long := strings.Repeat("Технолог: проба №1 😀 <Ca> & проверка. ", 140)
	parts := laboratoryExcelReportLines(long)
	if len(parts) < 2 || strings.Join(parts, "") != long {
		t.Fatal("long explanation lost characters")
	}
	for _, part := range parts {
		if len([]rune(part)) > 900 {
			t.Fatal("oversized report row")
		}
	}
	report := "РОСА\r\nЧто это значит для технолога\r\n" + long
	path := filepath.Join(t.TempDir(), "long.xlsx")
	if err := writeLaboratoryXLSXReport(path, report); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, f := range z.File {
		if f.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `c r="A2" s="2"`) || strings.Contains(string(data), `ht="409"`) {
			t.Fatal("heading style or safe row height lost")
		}
		if !strings.Contains(string(data), "&lt;Ca&gt; &amp;") {
			t.Fatal("XML escaping lost")
		}
	}
}

func TestScaleTechnologistHeadingBoldWithoutFollowingValue(t *testing.T) {
	heading := "ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ"
	for _, prefix := range []string{"", "ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ\n"} {
		text := prefix + heading + "\nВыбранная модель: Дэвис"
		found := false
		for _, r := range laboratoryBoldRanges(text) {
			// All characters in this test are in the Unicode BMP.
			selected := string([]rune(text)[r.Start:r.End])
			if selected == heading {
				found = true
			}
			if strings.Contains(selected, "Выбранная модель") {
				t.Fatal("heading emphasis leaked into following line")
			}
		}
		if !found {
			t.Fatal("scale heading not bold")
		}
	}
}

func TestGeneratedDOCXValidationRejectsMissingAndEmpty(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.docx")
	if err := validateLaboratoryGeneratedDOCX(missing); err == nil {
		t.Fatal("missing DOCX accepted")
	}
	empty := filepath.Join(t.TempDir(), "empty.docx")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateLaboratoryGeneratedDOCX(empty); err == nil {
		t.Fatal("empty DOCX accepted")
	}
	valid := filepath.Join(t.TempDir(), "valid.docx")
	if err := writeLaboratoryDOCXReport(valid, "РОСА\nТест"); err != nil {
		t.Fatal(err)
	}
	if err := validateLaboratoryGeneratedDOCX(valid); err != nil {
		t.Fatal(err)
	}
}

func TestWordPDFScriptGuardsCOMObjectsAndPaths(t *testing.T) {
	for _, want := range []string{"Test-Path -LiteralPath $Docx", "$word.Documents", "$documents.Open", "ExportAsFixedFormat", "FinalReleaseComObject", "$word.Quit()"} {
		if !strings.Contains(laboratoryWordPDFPowerShell, want) {
			t.Fatalf("PowerShell export script misses %q", want)
		}
	}
}

func TestPowerShellErrorTextHidesStackNoise(t *testing.T) {
	input := []byte("Object reference not set to an instance of an object.\r\n+ $doc = ...\r\nCategoryInfo : OperationStopped\r\nFullyQualifiedErrorId : NullReferenceException")
	got := laboratoryPowerShellErrorText(input)
	if got != "Microsoft Word не смог открыть сформированный DOCX через системный интерфейс" {
		t.Fatalf("unexpected compact error: %q", got)
	}
}

func TestPDFFallbackPreservesDOCXAndExcel(t *testing.T) {
	dir := t.TempDir()
	tmpDocx := filepath.Join(dir, "temp.docx")
	report := "РОСА — отчёт\nОбъект: ТЕСТ"
	if err := writeLaboratoryDOCXReport(tmpDocx, report); err != nil {
		t.Fatal(err)
	}
	pdf := filepath.Join(dir, "Отчёт с пробелом.pdf")
	docx, xlsx, docxErr, xlsxErr := preserveLaboratoryReportFallbacks(pdf, tmpDocx, report)
	if docxErr != nil || xlsxErr != nil {
		t.Fatalf("fallback failed: docx=%v xlsx=%v", docxErr, xlsxErr)
	}
	for _, path := range []string{docx, xlsx} {
		info, err := os.Stat(path)
		if err != nil || info.Size() == 0 {
			t.Fatalf("fallback missing or empty: %s, %v", path, err)
		}
	}
	if !strings.Contains(filepath.Base(docx), "резервный") || !strings.Contains(filepath.Base(xlsx), "резервный") {
		t.Fatal("fallback names are not explicit")
	}
}

func TestFallbackPathDoesNotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "report.pdf")
	first := strings.TrimSuffix(pdf, ".pdf") + "_резервный.docx"
	if err := os.WriteFile(first, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := laboratoryAvailableFallbackPath(pdf, ".docx")
	if got == first || !strings.HasSuffix(got, "_резервный_2.docx") {
		t.Fatalf("existing fallback would be overwritten: %q", got)
	}
}

func TestPDFFallbackMessageExplainsThatDataAreSaved(t *testing.T) {
	err := (&laboratoryPDFFallbackError{Cause: fmt.Errorf("Word unavailable"), DOCXPath: `C:\\Отчёты\\a.docx`, XLSXPath: `C:\\Отчёты\\a.xlsx`}).Error()
	for _, want := range []string{"PDF не сформирован", "Данные отчёта не потеряны", "DOCX:", "Excel:", "Microsoft Word"} {
		if !strings.Contains(err, want) {
			t.Fatalf("fallback message misses %q: %s", want, err)
		}
	}
}
