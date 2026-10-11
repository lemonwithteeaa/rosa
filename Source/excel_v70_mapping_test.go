//go:build windows

package main

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
)

func TestExcelV70RC152MappedLaboratoryFieldsExistInROSA(t *testing.T) {
	f, err := os.Open("reference/excel_v70_inventory/СОПОСТАВЛЕНИЕ_ЛАБОРАТОРИЯ_И_ОПР_RC152.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 40 {
		t.Fatalf("mapping unexpectedly small: %d rows", len(rows))
	}
	groups := map[string]map[string]bool{}
	for _, g := range labGroups {
		if g.Code == "overall" {
			continue
		}
		if groups[g.Code] == nil {
			groups[g.Code] = map[string]bool{}
		}
		for _, f := range g.Fields {
			groups[g.Code][f.Key] = true
		}
	}
	for i, row := range rows[1:] {
		if len(row) < 7 {
			t.Fatalf("row %d malformed: %#v", i+2, row)
		}
		group := strings.TrimSpace(row[3])
		key := strings.TrimSpace(row[4])
		status := strings.TrimSpace(row[6])
		if group == "" || key == "" || strings.HasPrefix(status, "not_equivalent") {
			continue
		}
		if !groups[group][key] {
			t.Errorf("Excel mapping row %s points to missing ROSA field %s/%s (%s)", row[0], group, key, row[1])
		}
	}
}

func TestExcelV70RC152HiddenReferenceReportUsesActualSheets(t *testing.T) {
	b, err := os.ReadFile("reference/excel_v70_inventory/СКРЫТЫЕ_СПРАВОЧНИКИ_RC152.txt")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"Фактические замеры", "Данные модели", "Тесты границ", "состояние=hidden"} {
		if !strings.Contains(s, want) {
			t.Fatalf("hidden reference report missing %q", want)
		}
	}
}
