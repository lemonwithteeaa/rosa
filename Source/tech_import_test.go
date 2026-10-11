//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBundledTechRegimeImportsWithoutManualMapping(t *testing.T) {
	path := filepath.Join("ТР_НФ_сентябрь.xlsx")
	rows, preview, err := computeTechPreview(path, nil)
	if err != nil {
		t.Fatalf("read bundled tech regime: %v", err)
	}
	if len(rows) < 1000 {
		t.Fatalf("unexpectedly few accepted objects from September source: got %d", len(rows))
	}

	skipped := 0
	for _, change := range preview {
		if change.Status == "Требует сопоставления" {
			t.Fatalf("bundled tech regime still requests manual mapping: %+v", change)
		}
		if change.Status == "Без идентификатора" {
			skipped += change.ChangedCount
		}
	}
	if skipped < 0 {
		t.Fatalf("invalid skipped-row count: %d", skipped)
	}
}

func TestTechValue145IsNotMissingMarker(t *testing.T) {
	state := TechObjectState{Field: "145", Well: "145", ObjectPlast: "145"}
	if suspiciousTechIdentity(state) {
		t.Fatal("145 must not be treated as a missing-value marker")
	}
	if got := preferTechValue("145", "146"); got != "145" {
		t.Fatalf("145 must remain ordinary data, got %q", got)
	}
}

func TestTechHeaderMappingPreventsShifted145FromBecomingWell(t *testing.T) {
	rows := []techHeaderRow{
		{Row: 5, Values: map[int]string{5: "Месторождение", 7: "Внутренний код", 9: "№ скважины", 11: "Объект разработки/пласт", 54: "Дебит жидкости, м3/сут"}},
	}
	cols, start, ok := resolveTechColumns(rows)
	if !ok {
		t.Fatal("expected recognizable header-based tech schema")
	}
	if cols["field"] != 5 || cols["well"] != 9 {
		t.Fatalf("wrong identity mapping: field=%d well=%d", cols["field"], cols["well"])
	}
	if cols["well"] == 7 {
		t.Fatal("legacy column 7 must not be used as well after explicit header mapping")
	}
	if start != 6 {
		t.Fatalf("unexpected data start row: got %d want 6", start)
	}

	data := map[int]string{5: "Бешкульское", 7: "145", 9: "321"}
	well := data[cols["well"]]
	if well != "321" {
		t.Fatalf("shifted value 145 leaked into well identity: got %q", well)
	}
}

func TestTechHeaderMappingFallsBackOnlyWithoutIdentityHeaders(t *testing.T) {
	rows := []techHeaderRow{{Row: 6, Values: map[int]string{33: "МРП, сут.", 54: "Дебит жидкости, м3/сут"}}}
	cols, start, ok := resolveTechColumns(rows)
	if ok {
		t.Fatal("partial non-identity headers must not activate mixed schema")
	}
	if cols["well"] != 7 || cols["field"] != 5 || start != 7 {
		t.Fatalf("legacy fallback changed: field=%d well=%d start=%d", cols["field"], cols["well"], start)
	}
}

func TestTechKeyMatchesTechnologicalModuleWithBlankDeposit(t *testing.T) {
	state := TechObjectState{Field: "Авиловское", Well: "7Н", Deposit: "", ObjectPlast: "Бобриковский горизонт"}
	got := buildTechKey(state)
	want := "Авиловское7НБобриковскийгоризонт"
	if got != want {
		t.Fatalf("key must follow technological module field+well+deposit+object logic: got %q want %q", got, want)
	}
	if strings.Contains(got, "145") {
		t.Fatalf("unrelated 145 leaked into key: %q", got)
	}
}

func TestTechKeyKeeps145WhenItIsActuallyWellNumber(t *testing.T) {
	state := TechObjectState{Field: "Тестовое", Well: "145", Deposit: "", ObjectPlast: "Бобриковский горизонт"}
	got := buildTechKey(state)
	if !strings.Contains(got, "145") {
		t.Fatalf("real well number 145 must be preserved: %q", got)
	}
}

func TestSeptemberTechRegimeAvilovRowsHaveBlankDeposit(t *testing.T) {
	path := filepath.Join("ТР_НФ_сентябрь.xlsx")
	rows, err := readTechXLSX(path)
	if err != nil {
		t.Fatalf("read September tech regime: %v", err)
	}
	wanted := map[string]bool{"6": false, "7Н": false, "111": false}
	for _, row := range rows {
		if row.Field != "Авиловское" {
			continue
		}
		if _, ok := wanted[row.Well]; !ok {
			continue
		}
		wanted[row.Well] = true
		if strings.TrimSpace(row.Deposit) != "" {
			t.Fatalf("Avilov well %s: deposit must be blank from source, got %q", row.Well, row.Deposit)
		}
		if strings.Contains(row.ObjectKey, "145") {
			t.Fatalf("Avilov well %s: unrelated 145 leaked into key %q", row.Well, row.ObjectKey)
		}
		want := compactKeyPart(row.Field) + compactKeyPart(row.Well) + compactKeyPart(row.ObjectPlast)
		if row.ObjectKey != want {
			t.Fatalf("Avilov well %s: got key %q want %q", row.Well, row.ObjectKey, want)
		}
	}
	for well, found := range wanted {
		if !found {
			t.Fatalf("Avilov well %s not found in September source", well)
		}
	}
}

func TestTechContinuationRowsInheritWellButKeepOwnReservoirIdentity(t *testing.T) {
	anchor := TechObjectState{Field: "Бешкульское", Well: "23", Deposit: "", ObjectPlast: "В целом"}
	child := TechObjectState{Field: "Бешкульское", Well: "", Deposit: "", ObjectPlast: "Батский III пласт"}
	if !techContinuationCanInheritWell(child, anchor) {
		t.Fatal("blank well continuation with another development object must inherit parent well")
	}
	child.Well = anchor.Well
	if got := buildTechKey(child); got != "Бешкульское23БатскийIIIпласт" {
		t.Fatalf("continuation key lost its own reservoir identity: %q", got)
	}
	if child.ObjectPlast != "Батский III пласт" {
		t.Fatalf("continuation object/plast must not be overwritten: %q", child.ObjectPlast)
	}
}

func TestTechContinuationDoesNotCrossFieldBoundary(t *testing.T) {
	anchor := TechObjectState{Field: "Бешкульское", Well: "23", ObjectPlast: "В целом"}
	other := TechObjectState{Field: "Авиловское", Well: "", ObjectPlast: "Бобриковский горизонт"}
	if techContinuationCanInheritWell(other, anchor) {
		t.Fatal("well number must never be inherited across different fields")
	}
}

func TestSeptemberTechRegimeBeshkulWell23CreatesThreeSeparateObjects(t *testing.T) {
	rows, err := readTechXLSX(filepath.Join("ТР_НФ_сентябрь.xlsx"))
	if err != nil {
		t.Fatalf("read September tech regime: %v", err)
	}
	wantObjects := map[string]bool{
		"В целом":            false,
		"Батский III пласт":  false,
		"Байосский II пласт": false,
	}
	keys := map[string]bool{}
	count := 0
	for _, row := range rows {
		if row.Field != "Бешкульское" || row.Well != "23" {
			continue
		}
		if _, ok := wantObjects[row.ObjectPlast]; !ok {
			continue
		}
		wantObjects[row.ObjectPlast] = true
		keys[row.ObjectKey] = true
		count++
	}
	if count != 3 {
		t.Fatalf("Beshkul well 23 must produce three object rows, got %d", count)
	}
	if len(keys) != 3 {
		t.Fatalf("Beshkul well 23 object rows must have three distinct keys, got %d", len(keys))
	}
	for obj, found := range wantObjects {
		if !found {
			t.Fatalf("Beshkul well 23 missing development object %q", obj)
		}
	}
}

func TestXLSXRelationshipTargetIsResolvedRelativeToWorkbook(t *testing.T) {
	cases := map[string]string{
		"worksheets/sheet1.xml":     "xl/worksheets/sheet1.xml",
		"xl/worksheets/sheet2.xml":  "xl/worksheets/sheet2.xml",
		"/xl/worksheets/sheet3.xml": "xl/worksheets/sheet3.xml",
	}
	for in, want := range cases {
		if got := xlsxRelationshipTargetPath(in); got != want {
			t.Fatalf("target %q: got %q want %q", in, got, want)
		}
	}
}
