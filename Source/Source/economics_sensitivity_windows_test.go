//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestEconomicsSensitivityShowsThreeForecastOutcomesWithoutInventingMissingValues(t *testing.T) {
	text := baseEconomicsScenarioSensitivityText([]BaseEconomicsScenario{
		{Name: "Осторожный", MRPDays: 120, NetBenefitForHorizonK: -15.5},
		{Name: "Центральный", MRPDays: 180, NetBenefitForHorizonK: 0},
		{Name: "Благоприятный", MRPDays: 240, NetBenefitForHorizonK: 37.2},
	})
	for _, want := range []string{"осторожный: МРП 120 сут.; чистая выгода -15,5", "центральный: МРП 180 сут.; чистая выгода +0,0", "благоприятный: МРП 240 сут.; чистая выгода +37,2"} {
		if !strings.Contains(strings.ReplaceAll(text, ".", ","), want) {
			t.Fatalf("missing sensitivity outcome %q in %q", want, text)
		}
	}
	missing := baseEconomicsScenarioSensitivityText([]BaseEconomicsScenario{{Name: "Осторожный"}})
	if !strings.Contains(missing, "прогноз МРП не задан; эффект не рассчитывается") || strings.Contains(missing, "выгода +0.0") {
		t.Fatalf("missing forecast was presented as a zero effect: %q", missing)
	}
}
