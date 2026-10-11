//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC544TeachingExamplesFormPhysicalObjectSeries(t *testing.T) {
	passes := []LabPassRecord{
		{ObjectKey: "Учебный пример 001 · Авиловское / 114 / Бобриковский горизонт", Date: "01.01.2024"},
		{ObjectKey: "Учебный пример 002 · Авиловское / 114 / Бобриковский горизонт", Date: "01.02.2024"},
		{ObjectKey: "Учебный пример 003 · Бешкульское / 17 / Батский III пласт", Date: "01.03.2024"},
	}
	groups := complexPassGroups(passes)
	if len(groups) != 2 || len(groups[0]) != 2 {
		t.Fatalf("teaching dates were not grouped by physical object: %#v", groups)
	}
}

func TestRC544IndividualInfographicMatchesApprovedLayout(t *testing.T) {
	dialog, err := os.ReadFile("laboratory_infographic_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	visual, err := os.ReadFile("laboratory_visualization.go")
	if err != nil {
		t.Fatal(err)
	}
	history, err := os.ReadFile("laboratory_history_chart.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`chartH := int32(310)`, `trendY := int32(440)`,
		`if di.HwndItem == labInfographicChart`, `columns = 1`, `rowH = 44`,
		`Линейные зависимости по выбранным датам доступны в «Комплексной диагностике»`,
	} {
		if !strings.Contains(string(dialog)+string(visual)+string(history), required) {
			t.Fatalf("approved individual infographic contract missing %q", required)
		}
	}
}

func TestRC544BuildVersion(t *testing.T) {
	if appBuildVersion == "7.3.11.544-RC" || !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected or rolled-back build version %q", appBuildVersion)
	}
}
