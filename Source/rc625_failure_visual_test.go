//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC625FountainAndRodsHaveOwnVisibleSections(t *testing.T) {
	if len(wellSectionButtons) != 7 {
		t.Fatalf("failure form section count: got %d, want 7", len(wellSectionButtons))
	}
	wantSection := map[string]int{"AG": 2, "AH": 2, "AI": 2, "AJ": 2, "AK": 2, "AL": 3, "AM": 3, "AN": 3, "AO": 3, "BH": 5, "BI": 5, "BJ": 5, "BK": 5, "BL": 5, "BM": 5, "BN": 5}
	for _, spec := range wellExtraSpecs {
		if section, ok := wantSection[spec.column]; ok {
			if spec.section != section {
				t.Fatalf("field %s is in section %d, want %d", spec.column, spec.section, section)
			}
			delete(wantSection, spec.column)
		}
	}
	if len(wantSection) != 0 {
		t.Fatalf("missing Fountain/Rods fields: %v", wantSection)
	}
}

func TestRC625RodFreeEntryAndCatalogsMatchPrompt(t *testing.T) {
	for _, column := range []string{"BJ", "BK", "BL", "BN"} {
		if !wellExtraUsesFreeText(column) {
			t.Fatalf("rod field %s must stay free text", column)
		}
	}
	loadWellFailureCatalog()
	got := strings.Join(wellFailureCatalog["equip_rod_maker"], "|")
	for _, want := range []string{`ОАО "МЗС"`, `АО "МЗ"`, `АО "ИОМЗС"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("rod maker missing: %s in %s", want, got)
		}
	}
}

func TestRC625RegisterObjectFilterIsLocal(t *testing.T) {
	oldSelected, oldPreview := selectedObjectKey, failurePreviewObjectKey
	defer func() { selectedObjectKey, failurePreviewObjectKey = oldSelected, oldPreview }()
	selectedObjectKey = "WORKING"
	failurePreviewObjectKey = "REGISTER"
	if got := selectedFailureObjectKey(); got != "REGISTER" {
		t.Fatalf("local register filter ignored: %q", got)
	}
	if selectedObjectKey != "WORKING" {
		t.Fatal("register filter changed the global working object")
	}
}
