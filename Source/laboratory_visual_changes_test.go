//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestGenericTechnologicalOutcomeIsBold(t *testing.T) {
	text := "ПРЕДВАРИТЕЛЬНОЕ ЗАКЛЮЧЕНИЕ\nТЕХНОЛОГИЧЕСКИЙ ИТОГ\nВероятность механизма: выявлены признаки"
	found := false
	for _, r := range laboratoryBoldRanges(text) {
		selected := string([]rune(text)[r.Start:r.End])
		if selected == "ТЕХНОЛОГИЧЕСКИЙ ИТОГ" {
			found = true
		}
	}
	if !found {
		t.Fatal("generic technological outcome heading is not bold")
	}
}

func TestReportStylePreservesTechnologyAndStatusAccents(t *testing.T) {
	if got := laboratoryReportLineStyle("ТЕХНОЛОГИЧЕСКИЙ ИТОГ", 7); got != reportStyleTechnology {
		t.Fatalf("technology heading style=%d", got)
	}
	if got := laboratoryReportLineStyle("Коррозия | ПОВЫШЕННЫЙ | База 5/7 · 71%", 8); got != reportStyleElevated {
		t.Fatalf("elevated status style=%d", got)
	}
	if got := laboratoryReportLineStyle("Соли | НИЗКИЙ | База 6/7 · 86%", 9); got != reportStyleLow {
		t.Fatalf("low status style=%d", got)
	}
}

func TestReportVisualizationContainsScreenSemantics(t *testing.T) {
	group, ok := labGroupByCode("corrosion")
	if !ok {
		t.Fatal("corrosion group missing")
	}
	values := map[string]string{"ph": "6.5", "co2": "100", "h2s": "1", "corrosion_rate": "0.2"}
	visual := laboratoryReportVisualizationForGroup(group, values, "davies")
	for _, want := range []string{"ВИЗУАЛЬНАЯ СВОДКА ПО НАПРАВЛЕНИЯМ", "Коррозия", "База"} {
		if !strings.Contains(visual, want) {
			t.Fatalf("visual summary lost %q: %s", want, visual)
		}
	}
}

func TestLaboratoryButtonsKeepOneFontSize(t *testing.T) {
	ids := []int{ID_LAB_IMPORT, ID_LAB_DOWNLOAD_TEMPLATE, ID_LAB_USE_SELECTED, ID_LAB_ANALYZE, ID_LAB_EXPORT, ID_LAB_SAVE, ID_LAB_CONTINUE, ID_LAB_COPY_RESULT, ID_LAB_DELETE_ROW, ID_LAB_CLEAR_ALL, ID_LAB_FIELDS_PREV, ID_LAB_FIELDS_NEXT}
	for _, g := range labGroups {
		ids = append(ids, g.ID)
	}
	for _, id := range ids {
		if !buttonUsesUniformStandardFont(id) {
			t.Fatalf("laboratory button %d may shrink to a different font size", id)
		}
	}
}

func TestPageHeadingsUseLargerVisualRole(t *testing.T) {
	if uiHeadingHeight < 28 {
		t.Fatalf("section heading is too small: %d", uiHeadingHeight)
	}
}

func TestOverallReportVisualizationListsAllLaboratoryDirections(t *testing.T) {
	visual := laboratoryReportVisualizationForPass(LabPassRecord{Groups: map[string]map[string]string{}})
	for _, g := range labGroups {
		if g.Code == "overall" {
			continue
		}
		if !strings.Contains(visual, g.Name) {
			t.Fatalf("overall report lost laboratory direction %q: %s", g.Name, visual)
		}
	}
}
