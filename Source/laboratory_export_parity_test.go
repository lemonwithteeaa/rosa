//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestLAB90PNGAndTextExportShareCriticalSemantics(t *testing.T) {
	pass := LabPassRecord{
		ObjectKey: "OBJ-77", Date: "18.09.2026", Source: "LAB-PROTOCOL-77", ScaleModel: "davies",
		Groups: map[string]map[string]string{"corrosion": {"ph": "7.1", "chlorides": "1200"}},
		Units:  map[string]map[string]string{"corrosion": {}},
	}
	metric := laboratoryTrendMetricByIndex(0)
	pngSummary := laboratoryInfographicSemanticSummary(pass, laboratoryScopeObject, metric, []LabPassRecord{pass}, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	for _, want := range []string{"OBJ-77", "LAB-PROTOCOL-77", "QC:", "данные по", "Ограничение:", "Отсутствие оценки не означает низкий риск"} {
		if !strings.Contains(pngSummary, want) {
			t.Fatalf("PNG semantic summary lost %q: %s", want, pngSummary)
		}
	}
	textSummary := laboratoryReportVisualizationForPass(pass) + "\n" + laboratoryPassQCText(pass, time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local))
	for _, want := range []string{"Корроз", "Базовые данные всего", "ПАСПОРТ КАЧЕСТВА"} {
		if !strings.Contains(textSummary, want) {
			t.Fatalf("text export semantic block lost %q", want)
		}
	}
}
