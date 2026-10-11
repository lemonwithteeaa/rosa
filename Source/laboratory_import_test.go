//go:build windows

package main

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func writeLabTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func utf16LELabTest(text string) []byte {
	data := []byte{0xff, 0xfe}
	for _, r := range text {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(r))
		data = append(data, buf...)
	}
	return data
}

func writeLabTestXLSX(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wide.xlsx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
		`<row r="1"><c r="A1" t="inlineStr"><is><t>ключ объекта</t></is></c><c r="B1" t="inlineStr"><is><t>дата</t></is></c><c r="C1" t="inlineStr"><is><t>pH</t></is></c></row>` +
		`<row r="2"><c r="A2" t="inlineStr"><is><t>OBJ-1</t></is></c><c r="B2" t="inlineStr"><is><t>03.09.2026</t></is></c><c r="C2"><v>6.2</v></c></row>` +
		`</sheetData></worksheet>`
	if _, err = w.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLabTestDOCX(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "laboratory.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		`<w:p><w:r><w:t>Объект: OBJ-1</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Дата исследования: 05.09.2026</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Направление анализа: Коррозия</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Протокол: DOCX-55</w:t></w:r></w:p><w:tbl>` +
		`<w:tr><w:tc><w:p><w:r><w:t>Показатель</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Значение</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Единица</w:t></w:r></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:r><w:t>pH</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>6,7</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>ед.</w:t></w:r></w:p></w:tc></w:tr>` +
		`</w:tbl></w:body></w:document>`
	if _, err = w.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeLabTestDOC(t *testing.T) string {
	t.Helper()
	text := "Ключ объекта\tДата\tПоказатель\tЗначение\tИсточник\r\nOBJ-1\t06.09.2026\tpH\t6,8\tDOC-56\r\n"
	data := []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1, 0, 1, 2, 3}
	for _, unit := range utf16.Encode([]rune(text)) {
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, unit)
		data = append(data, buf...)
	}
	return writeLabTestFile(t, "laboratory.doc", data)
}

func resetLabImportTest(t *testing.T) {
	t.Helper()
	dataDir = t.TempDir()
	currentSession = nil
	labRecords = nil
	labRecordsLoaded = true
	selectedObjectKey = "OBJ-1"
	techCurrent = []TechObjectState{{ObjectKey: "OBJ-1", Well: "1"}}
}

func TestLaboratoryImportFormatsAndLayouts(t *testing.T) {
	resetLabImportTest(t)
	csvPath := writeLabTestFile(t, "wide.csv", []byte("ключ объекта;дата;источник;pH;Хлориды Cl⁻\nOBJ-1;01.09.2026;CSV;6,3;12000\n"))
	added, skipped, err := importLaboratoryFile(csvPath)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("wide CSV: added=%d skipped=%d err=%v", added, skipped, err)
	}

	txt := "показатель\tзначение\tединица\tдата\tисточник\r\npH\t7,1\tед.\t02.09.2026\tTXT UTF-16\r\n"
	txtPath := writeLabTestFile(t, "vertical.txt", utf16LELabTest(txt))
	added, skipped, err = importLaboratoryFile(txtPath)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("vertical UTF-16 TXT: added=%d skipped=%d err=%v", added, skipped, err)
	}

	xlsxPath := writeLabTestXLSX(t)
	added, skipped, err = importLaboratoryFile(xlsxPath)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("wide XLSX: added=%d skipped=%d err=%v", added, skipped, err)
	}

	docxPath := writeLabTestDOCX(t)
	added, skipped, err = importLaboratoryFile(docxPath)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("DOCX table: added=%d skipped=%d err=%v", added, skipped, err)
	}
	foundDOCXMetadata := false
	for _, record := range labRecords {
		if record.Source == "DOCX-55" {
			foundDOCXMetadata = record.ObjectKey == "OBJ-1" && record.Date == "05.09.2026" && record.GroupCode == "corrosion" && record.Values["ph"] == "6,7"
		}
	}
	if !foundDOCXMetadata {
		t.Fatalf("metadata above DOCX table was not propagated: %#v", labRecords)
	}

	docPath := writeLabTestDOC(t)
	added, skipped, err = importLaboratoryFile(docPath)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("legacy DOC text: added=%d skipped=%d err=%v", added, skipped, err)
	}

	if len(labRecords) != 5 {
		t.Fatalf("want 5 records, got %d", len(labRecords))
	}
	if _, _, err = importLaboratoryFile(writeLabTestFile(t, "wrong.json", []byte("{}"))); err == nil || !strings.Contains(err.Error(), "DOCX или DOC") {
		t.Fatalf("unsupported extension must be rejected, err=%v", err)
	}
}

func TestLaboratoryAttentionPhraseUsesCorrectRussianGrammar(t *testing.T) {
	tests := map[int]string{
		1:  "требует внимания 1 показатель",
		2:  "требуют внимания 2 показателя",
		5:  "требуют внимания 5 показателей",
		11: "требуют внимания 11 показателей",
		21: "требуют внимания 21 показатель",
	}
	for n, want := range tests {
		if got := laboratoryAttentionPhrase(n, "показатель", "показателя", "показателей"); got != want {
			t.Fatalf("n=%d: got %q, want %q", n, got, want)
		}
	}
}

func TestLaboratoryImportRecognizesAllFields(t *testing.T) {
	resetLabImportTest(t)
	data := "№ скв.;Дата исследования;Направление анализа;Наименование параметра;Результат анализа;Размерность;Номер протокола;Комментарий лаборатории\n" +
		"1;04.09.2026;Коррозия;pH;6,4;ед.;ПР-77;\n" +
		"1;04.09.2026;Коррозия;Хлориды Cl⁻;14500;мг/л;;повторить контроль через месяц\n"
	path := writeLabTestFile(t, "all-fields.csv", []byte(data))
	added, skipped, err := importLaboratoryFile(path)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("all fields: added=%d skipped=%d err=%v", added, skipped, err)
	}
	if len(labRecords) != 1 {
		t.Fatalf("want 1 aggregate record, got %d", len(labRecords))
	}
	r := labRecords[0]
	if r.ObjectKey != "OBJ-1" || r.Date != "04.09.2026" || r.GroupCode != "corrosion" {
		t.Fatalf("identity fields not recognized: %#v", r)
	}
	if r.Values["ph"] != "6,4" || r.Values["chlorides"] != "14500" || r.Units["chlorides"] != "мг/л" {
		t.Fatalf("measurement fields not recognized: values=%v units=%v", r.Values, r.Units)
	}
	if r.Source != "ПР-77" || r.Comment != "повторить контроль через месяц" {
		t.Fatalf("source/comment not preserved: source=%q comment=%q", r.Source, r.Comment)
	}
}

func TestLaboratoryImportAllowsMissingOptionalColumns(t *testing.T) {
	resetLabImportTest(t)
	data := "Код объекта;Раздел;Показатель исследования;Полученное значение\n" +
		"OBJ-1;АСПО;Парафины;4,2\n"
	path := writeLabTestFile(t, "optional-columns-absent.csv", []byte(data))
	added, skipped, err := importLaboratoryFile(path)
	if err != nil || added != 1 || skipped != 0 {
		t.Fatalf("optional columns absent: added=%d skipped=%d err=%v", added, skipped, err)
	}
	r := labRecords[0]
	if r.Date == "" || r.Source != "" || r.Comment != "" {
		t.Fatalf("optional fields handled incorrectly: %#v", r)
	}
	if r.Values["paraffins"] != "4,2" || r.Units["paraffins"] != "% масс." {
		t.Fatalf("default unit or value lost: values=%v units=%v", r.Values, r.Units)
	}
}

func TestLaboratoryImportSkipsIssuesAndRejectsDuplicates(t *testing.T) {
	resetLabImportTest(t)
	data := "Ключ объекта;Раздел;Показатель;Значение;Источник\n" +
		"OBJ-1;Коррозия;pH;6,4;ПР-88\n" +
		"OBJ-1;Коррозия;Неизвестный показатель;1;ПР-88\n" +
		"UNKNOWN;Коррозия;pH;6,5;ПР-88\n"
	path := writeLabTestFile(t, "partial-with-errors.csv", []byte(data))
	added, skipped, duplicates, issues, err := importLaboratoryFileDetailed(path)
	if err != nil || added != 1 || skipped != 2 || duplicates != 0 || len(issues) != 2 {
		t.Fatalf("first import: added=%d skipped=%d duplicates=%d issues=%v err=%v", added, skipped, duplicates, issues, err)
	}
	if len(labRecords) != 1 || labRecords[0].Values["ph"] != "6,4" {
		t.Fatalf("valid row was not preserved: %#v", labRecords)
	}
	if issues[0].Row != 3 || issues[1].Row != 4 {
		t.Fatalf("wrong source row numbers: %#v", issues)
	}
	added, skipped, duplicates, issues, err = importLaboratoryFileDetailed(path)
	if err != nil || added != 0 || skipped != 2 || duplicates != 1 || len(labRecords) != 1 {
		t.Fatalf("repeat import: added=%d skipped=%d duplicates=%d issues=%v records=%d err=%v", added, skipped, duplicates, issues, len(labRecords), err)
	}
	report := laboratoryImportReport(added, skipped, duplicates, issues)
	for _, want := range []string{"Повторных анализов без создания дубля: 1", "Строка 3:", "Строка 4:"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report does not contain %q: %s", want, report)
		}
	}
}

func TestLaboratoryCustomMappingPreviewDoesNotWrite(t *testing.T) {
	resetLabImportTest(t)
	data := "Код заказчика;Когда отобрано;Название теста;Результат теста;Протокол заказчика\n" +
		"OBJ-1;15.09.2026;pH;6,6;ПР-99\n"
	path := writeLabTestFile(t, "custom-columns.csv", []byte(data))
	rows, err := readLaboratoryRows(path)
	if err != nil {
		t.Fatal(err)
	}
	mapping := LabColumnMapping{HeaderRow: 0, Object: 0, Date: 1, Group: -1, Parameter: 2, Value: 3, Unit: -1, Source: 4, Comment: -1}
	invalidMapping := mapping
	invalidMapping.Value = invalidMapping.Parameter
	if err := validateLaboratoryMapping(rows, invalidMapping); err == nil {
		t.Fatal("duplicate column mapping was accepted")
	}
	preview := laboratoryMappingPreview(path, rows, mapping)
	if len(labRecords) != 0 {
		t.Fatalf("preview wrote laboratory records: %#v", labRecords)
	}
	for _, want := range []string{"СОПОСТАВЛЕНИЕ", "Название теста", "Результат теста", "Запись ещё не выполнена"} {
		if !strings.Contains(preview, want) {
			t.Fatalf("preview does not contain %q: %s", want, preview)
		}
	}
	added, skipped, duplicates, issues, err := importLaboratoryFileWithMapping(path, mapping)
	if err != nil || added != 1 || skipped != 0 || duplicates != 0 || len(issues) != 0 {
		t.Fatalf("mapped import: added=%d skipped=%d duplicates=%d issues=%v err=%v", added, skipped, duplicates, issues, err)
	}
	if len(labRecords) != 1 || labRecords[0].Values["ph"] != "6,6" || labRecords[0].Source != "ПР-99" {
		t.Fatalf("mapped fields were not imported: %#v", labRecords)
	}
}

func TestLaboratoryBaselineAndOptionalCoverage(t *testing.T) {
	for _, group := range labGroups {
		baseline := laboratoryBaselineKeys(group.Code)
		if len(baseline) == 0 {
			t.Fatalf("group %s has no baseline fields", group.Code)
		}
		known := map[string]bool{}
		for _, field := range group.Fields {
			known[field.Key] = true
		}
		for key := range baseline {
			if !known[key] {
				t.Fatalf("group %s has unknown baseline field %s", group.Code, key)
			}
		}
	}

	group := labGroups[0]
	values := map[string]string{"ph": "6,2"}
	baseCount := len(laboratoryBaselineKeys(group.Code))
	techContextCount := 0
	for _, field := range group.Fields {
		if laboratoryTechRegimeKey(group.Code, field.Key) {
			techContextCount++
		}
	}
	baseFilled, baseTotal, optionalFilled, optionalTotal := laboratoryDataCoverage(group, values)
	if baseFilled != 1 || baseTotal != baseCount || optionalFilled != 0 || optionalTotal != len(group.Fields)-baseCount-techContextCount {
		t.Fatalf("unexpected coverage: base=%d/%d optional=%d/%d", baseFilled, baseTotal, optionalFilled, optionalTotal)
	}
	conclusion := buildLaboratoryConclusion(group, values)
	for _, want := range []string{"Базовый минимум данных: сформирован частично", "Предварительный вывод доступен по заполненной части базового минимума", "обводнённость из техрежима не найдена", fmt.Sprintf("Дополнительные параметры: заполнено 0 из %d", len(group.Fields)-baseCount-techContextCount), "их отсутствие не блокирует предварительную оценку", "pH — фактическое значение: 6.2 ед.; рабочий порог: ниже 6,5 ед.; результат: требует внимания", "кислая среда может ускорять электрохимическую коррозию"} {
		if !strings.Contains(conclusion, want) {
			t.Fatalf("conclusion does not contain %q: %s", want, conclusion)
		}
	}

	emptyConclusion := buildLaboratoryConclusion(group, map[string]string{})
	for _, want := range []string{"Базовый минимум данных: не сформирован", "Анализ не блокируется"} {
		if !strings.Contains(emptyConclusion, want) {
			t.Fatalf("empty conclusion does not contain %q: %s", want, emptyConclusion)
		}
	}
}

func TestLaboratoryThresholdOutputForAllGroups(t *testing.T) {
	cases := []struct {
		groupCode string
		key       string
		values    map[string]string
		actual    string
		threshold string
	}{
		{"corrosion", "ph", map[string]string{"ph": "6,2"}, "6.2 ед.", "ниже 6,5 ед."},
		{"scale", "ca", map[string]string{"ca": "2501"}, "2501 мг/л", "выше 2 000 мг/л"},
		{"aspo", "paraffins", map[string]string{"paraffins": "4"}, "4 % масс.", "выше 3 % масс."},
		{"solids", "suspended", map[string]string{"suspended": "101"}, "101 мг/л", "выше 100 мг/л"},
		{"microbiology", "gmb", map[string]string{"gmb": "1001"}, "1001 КОЕ/мл", "выше 10³ КОЕ/мл"},
		{"gases", "co2", map[string]string{"co2": "51"}, "51 мг/л", "выше 50 мг/л"},
	}
	for _, tc := range cases {
		var group LabGroupDef
		for _, candidate := range labGroups {
			if candidate.Code == tc.groupCode {
				group = candidate
				break
			}
		}
		conclusion := buildLaboratoryConclusion(group, tc.values)
		for _, want := range []string{"фактическое значение: " + tc.actual, "рабочий порог: " + tc.threshold, "результат: требует внимания", laboratoryAttentionExplanation(tc.groupCode, tc.key)} {
			if !strings.Contains(conclusion, want) {
				t.Fatalf("group %s does not contain %q: %s", tc.groupCode, want, conclusion)
			}
		}
	}

	safe := buildLaboratoryConclusion(labGroups[0], map[string]string{"ph": "7"})
	if !strings.Contains(safe, "результат: выход за рабочую границу не выявлен") {
		t.Fatalf("safe verbal result is absent: %s", safe)
	}
}

func TestLaboratoryValuesWithoutNumericThreshold(t *testing.T) {
	micro := buildLaboratoryConclusion(labGroups[4], map[string]string{"srb": "обнаружены"})
	for _, want := range []string{"СВБ — фактический результат: обнаружены", "единица / тип: качественный результат", "числовой рабочий порог: не применяется", "результат: требуется экспертная интерпретация"} {
		if !strings.Contains(micro, want) {
			t.Fatalf("qualitative output does not contain %q: %s", want, micro)
		}
	}

	hydrate := buildLaboratoryConclusion(labGroups[5], map[string]string{"hydrate_temp": "12,5"})
	for _, want := range []string{"Равновесная температура гидратов — фактическое значение: 12.5 °C", "числовой рабочий порог: не применяется", "нужны температура и давление процесса", "требуется сопоставление с фактической температурой и давлением процесса"} {
		if !strings.Contains(hydrate, want) {
			t.Fatalf("context output does not contain %q: %s", want, hydrate)
		}
	}
}

func TestOfficialTemplateMissingBaselineIsWarningStateNotRowError(t *testing.T) {
	rows := [][]string{
		{"Статус показателя", "Дата", "Раздел", "Показатель", "Значение", "Единица", "Источник", "Комментарий"},
		{"Базовый, общий", "", "Коррозия", "Водородный показатель (pH)", "6,8", "ед.", "", ""},
		{"Базовый, общий", "", "Коррозия", "Хлориды Cl⁻", "", "мг/л", "", ""},
		{"Дополнительный", "", "Коррозия", "FeS в водной фазе", "", "мг/л", "", ""},
	}
	m := detectLaboratoryColumnMapping(rows)
	filled, total := officialLaboratoryBaselineCoverage(rows, m)
	if filled != 1 || total != 2 {
		t.Fatalf("baseline coverage = %d/%d, want 1/2", filled, total)
	}
	normalized := normalizeOfficialLaboratoryTemplateRows(rows, m)
	if len(normalized) != 2 {
		t.Fatalf("empty template rows must be ignored, got %d rows", len(normalized))
	}
	if normalized[1][m.Date] == "" {
		t.Fatal("official blank must get a usable default date instead of producing a missing-date error")
	}
}
