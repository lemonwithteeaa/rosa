//go:build windows

package main

import (
	"strings"
	"testing"
	"time"
)

func TestLaboratoryDataQualityCardShowsFourDimensions(t *testing.T) {
	group, ok := labGroupByCode("corrosion")
	if !ok {
		t.Fatal("corrosion group missing")
	}
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local)
	text := laboratoryDataQualityCardText(group, map[string]string{"ph": "7.0"}, "", "18.09.2026", "Протокол №12", now, 0, 0)
	for _, token := range []string{"Полнота:", "Согласованность:", "Свежесть:", "Источник:"} {
		if !strings.Contains(text, token) {
			t.Fatalf("missing %q in %s", token, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "балл: 100") {
		t.Fatal("must not create opaque quality score")
	}
}

func TestLaboratoryDataQualityFreshnessWarnsForOldAnalysis(t *testing.T) {
	group, _ := labGroupByCode("corrosion")
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.Local)
	q := laboratoryDataQualitySummaryFor(group, map[string]string{}, "", "01.01.2026", "", now, 0, 0)
	if !strings.Contains(q.FreshnessLabel, "подтвердить актуальность") {
		t.Fatalf("unexpected freshness: %s", q.FreshnessLabel)
	}
	if q.SourcePresent {
		t.Fatal("empty source must reduce traceability")
	}
}

func TestLaboratoryDataQualityScaleUsesWaterQC(t *testing.T) {
	group, _ := labGroupByCode("scale")
	values := map[string]string{"ca": "400", "mg": "200", "na": "200", "k": "0", "cl": "10", "so4": "10", "hco3": "10", "mineralization": "0.83"}
	q := laboratoryDataQualitySummaryFor(group, values, "davies", "18.09.2026", "Лаборатория", time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local), 0, 0)
	if !strings.Contains(strings.ToLower(q.ConsistencyLabel), "ионный баланс") {
		t.Fatalf("scale QC not reflected: %s", q.ConsistencyLabel)
	}
}
