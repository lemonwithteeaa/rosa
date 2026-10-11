//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratoryScaleThreeLayerSeparatesThermodynamicsMassAndKinetics(t *testing.T) {
	values := map[string]string{"ca": "40.078", "co3": "60.008", "na_k": "22.99", "cl": "35.45"}
	text := laboratoryScaleThreeLayerAssessment(values, "ideal")
	for _, want := range []string{"Термодинамика", "Возможное количество / масса осадка", "Кинетика / время индукции", "НЕ РАССЧИТЫВАЕТСЯ"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "масса осадка составит") || strings.Contains(strings.ToLower(text), "выпадет через") {
		t.Fatalf("three-layer text must not invent mass or induction time: %s", text)
	}
}

func TestLaboratoryScalePanelCarriesThreeLayerAssessment(t *testing.T) {
	text := laboratoryScalePanelTextForModel(map[string]string{"ca": "40.078", "co3": "60.008", "na_k": "22.99", "cl": "35.45"}, "ideal")
	if !strings.Contains(text, "РАЗДЕЛЕНИЕ РЕЗУЛЬТАТА") || !strings.Contains(text, "Кинетика / время индукции") {
		t.Fatalf("panel missing three-layer assessment: %s", text)
	}
}
