package main

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestAssessmentApprovalArchivedReport(t *testing.T) {
	lines := strings.Split(strings.Join([]string{
		"РАЗДЕЛ 3. ОЦЕНКА ОСЛОЖНЕНИЙ",
		"КЛАССИФИКАЦИЯ ОСЛОЖНЕНИЙ — ПОДТВЕРЖДЕНИЕ ТЕХНОЛОГА",
		"✓ Солеотложения: утверждённый итог Не относится; технолог Не относится; программа Не относится; решение — без корректировки.",
		"  Комментарий: Утверждено в сквозном учебном примере; не является производственным заключением.",
		"  Утвердил: Технолог · учебный контур; 04.10.2026 12:00.",
		"⚠ Коррозия: утверждённый итог К2; технолог К2; программа К1; решение — с корректировкой.",
		"  Комментарий: По анализу CO2 и состоянию оборудования.",
		"  Утвердил: Технолог; 05.10.2026 13:30.",
		"Полнота утверждения: полная классификация подтверждена технологом.",
		"",
		"РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ",
	}, "\n"), "\n")
	rows, end, common, ok := assessmentExportParseBlock(lines, 1)
	if !ok || len(rows) != 2 || end != 9 || common != "" {
		t.Fatalf("parse: ok=%v rows=%+v end=%d common=%q", ok, rows, end, common)
	}
	if rows[0].Status != "Утверждено; без корректировки" || rows[0].ApprovedBy != "Технолог · учебный контур" || rows[0].ApprovedAt != "04.10.2026 12:00" {
		t.Fatalf("first row: %+v", rows[0])
	}
	if rows[1].Status != "Утверждено; с корректировкой" || rows[1].Program != "К1" || rows[1].Technologist != "К2" {
		t.Fatalf("second row: %+v", rows[1])
	}
}
func TestAssessmentApprovalPendingAndDeduplicate(t *testing.T) {
	lines := []string{
		"КЛАССИФИКАЦИЯ ОСЛОЖНЕНИЙ — ПОДТВЕРЖДЕНИЕ ТЕХНОЛОГА",
		"… АСПО: выбранный вариант К2; технолог К2; программа К1; решение — с корректировкой; ОЖИДАЕТ ОБЩЕГО УТВЕРЖДЕНИЯ.",
		"  Комментарий: Общий комментарий.",
		"✓ Свободный газ: утверждённый итог НД; технолог НД; программа НД; решение — без корректировки.",
		"  Комментарий: Общий комментарий.",
		"",
		"РАЗДЕЛ 5. НАРАБОТКА И ОТКАЗЫ",
	}
	rows, end, common, ok := assessmentExportParseBlock(lines, 0)
	if !ok || len(rows) != 2 || end != 5 || common != "Общий комментарий." {
		t.Fatalf("parse %v %+v %d %q", ok, rows, end, common)
	}
	if rows[0].Status != "Не утверждено; с корректировкой" || rows[1].Status != "Утверждено; без корректировки" {
		t.Fatalf("statuses: %+v", rows)
	}
	if rows[0].Comment != "—" || rows[1].Comment != "—" {
		t.Fatalf("common note duplicated: %+v", rows)
	}
}
func TestAssessmentApprovalUnrelatedNotConsumed(t *testing.T) {
	lines := []string{"РАЗДЕЛ 3. ОЦЕНКА ОСЛОЖНЕНИЙ", "✓ Солеотложения: ...", "РАЗДЕЛ 4. МЕТОДЫ"}
	_, end, _, ok := assessmentExportParseBlock(lines, 0)
	if ok || end != 0 {
		t.Fatalf("incorrect recognition: %v %d", ok, end)
	}
}

func TestAssessmentApprovalDOCXTableWellFormed(t *testing.T) {
	rows := []assessmentExportRow{{Complication: "Солеотложения", Program: "Не относится", Technologist: "К2", Status: "Утверждено", Comment: "Значение <проверено> & измерено", ApprovedBy: "Технолог", ApprovedAt: "04.10.2026"}}
	xmlString := `<root xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` + assessmentApprovalDOCXTable(rows, "") + `</root>`
	var root struct {
		Tables []struct {
			Rows []struct {
				Cells []struct{} `xml:"tc"`
			} `xml:"tr"`
		} `xml:"tbl"`
	}
	if err := xml.Unmarshal([]byte(xmlString), &root); err != nil {
		t.Fatal(err)
	}
	if len(root.Tables) != 1 || len(root.Tables[0].Rows) != 2 {
		t.Fatalf("rows: %+v", root)
	}
	for _, row := range root.Tables[0].Rows {
		if len(row.Cells) != 7 {
			t.Fatalf("expected 7 columns, got %d", len(row.Cells))
		}
	}
	if !strings.Contains(xmlString, "&lt;проверено&gt; &amp;") {
		t.Fatalf("XML escaping failed")
	}
}
