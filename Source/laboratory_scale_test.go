//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestLaboratorySaturationIdealCalcite(t *testing.T) {
	values := map[string]string{"ca": "40.078", "co3": "60.01", "na_k": "22.99", "cl": "35.45"}
	results, _, warnings := laboratorySaturationResults(values, "ideal")
	if len(results) == 0 || results[0].Mineral != "Кальцит" {
		t.Fatalf("calcite result missing: %#v", results)
	}
	if results[0].SI < 2.4 || results[0].SI > 2.6 {
		t.Fatalf("unexpected calcite SI: %.4f", results[0].SI)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
}

func TestLaboratorySaturationDaviesWarnsForStrongBrine(t *testing.T) {
	values := map[string]string{"ca": "40000", "mg": "24000", "na_k": "23000", "cl": "71000", "so4": "9600"}
	_, ionicStrength, warnings := laboratorySaturationResults(values, "davies")
	if ionicStrength <= 0.5 {
		t.Fatalf("expected high ionic strength, got %.4f", ionicStrength)
	}
	joined := strings.Join(warnings, " ")
	if !strings.Contains(joined, "Питцер") && !strings.Contains(joined, "Pitzer") {
		t.Fatalf("high salinity warning must recommend Pitzer: %q", joined)
	}
}

func TestUnavailableScaleModelNeverFallsBackSilently(t *testing.T) {
	results, _, warnings := laboratorySaturationResults(map[string]string{"ca": "1000", "so4": "1000"}, "pitzer")
	if len(results) == 0 || !results[0].Partial || len(warnings) == 0 {
		t.Fatalf("unavailable model must expose a labelled partial result: results=%#v warnings=%#v", results, warnings)
	}
	if !strings.Contains(strings.ToLower(strings.Join(warnings, " ")), "частич") {
		t.Fatalf("partial status must be explicit: %#v", warnings)
	}
}

func TestModelSpecificPartialCoreIsScoped(t *testing.T) {
	values := map[string]string{"ca": "1000", "co3": "100", "so4": "1000", "ba": "1"}
	results, _, _ := laboratorySaturationResults(values, "stiff_davis")
	if len(results) != 1 || results[0].Mineral != "Кальцит" || !results[0].Partial {
		t.Fatalf("Stiff-Davis partial core must be carbonate-only: %#v", results)
	}
}

func TestScaleCardShowsEveryLimitation(t *testing.T) {
	values := map[string]string{"ca": "40000", "mg": "24000", "na_k": "23000", "cl": "71000", "so4": "9600", "sample_temp": "80"}
	text := laboratoryScalePanelTextForModel(values, "davies")
	for _, expected := range []string{"ОГРАНИЧЕННЫЙ РАСЧЁТ", "0,5 моль/л", "баланса зарядов", "температура отличается"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing limitation %q: %s", expected, text)
		}
	}
}

func TestScaleCardDoesNotClaimCalculationWithoutIons(t *testing.T) {
	text := laboratoryScalePanelTextForModel(map[string]string{}, "davies")
	if !strings.Contains(text, "НЕ РАССЧИТАНО") {
		t.Fatal(text)
	}
}

func TestScaleTextUsesExplicitModelWithoutChangingSelection(t *testing.T) {
	before := labScaleModelCode
	text := laboratorySaturationTextForModel(map[string]string{"ca": "40.078", "co3": "60.01"}, "ideal")
	if !strings.Contains(text, "Модель: Идеальный раствор") {
		t.Fatal(text)
	}
	if labScaleModelCode != before {
		t.Fatal("report changed form selection")
	}
	unknown := laboratoryScalePanelTextForModel(map[string]string{"ca": "40.078", "co3": "60.01"}, "future_model")
	if !strings.Contains(unknown, "Неизвестная модель: future_model") || !strings.Contains(unknown, "ЧАСТИЧНЫЙ РАСЧЁТ") {
		t.Fatal(unknown)
	}
}

func TestOddoTomsonExplicitlyStatesTemperatureAndPressureAreNotCalculated(t *testing.T) {
	text := laboratoryScaleDecisionText(map[string]string{"sample_temp": "80", "sample_pressure": "12"}, "oddo")
	for _, want := range []string{"Оддо—Томсон", "Учёт T/P в текущей версии: НЕТ", "температура и давление не участвуют", "Вероятность выпадения: НЕ ОПРЕДЕЛЕНА"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
}

func TestScaleDecisionAnswersWhetherPrecipitationTendencyExists(t *testing.T) {
	text := laboratoryScaleDecisionText(map[string]string{"ca": "40.078", "co3": "60.01", "na_k": "22.99", "cl": "35.45"}, "ideal")
	for _, want := range []string{"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ", "ТЕРМОДИНАМИЧЕСКАЯ ПРЕДПОСЫЛКА ВЫЯВЛЕНА", "Кальцит", "не доказывает фактическое выпадение"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %s", want, text)
		}
	}
}

func TestScaleSixComponentProducesCautiousFinalForecast(t *testing.T) {
	values := map[string]string{
		"mineralization": "44", "ph": "7.5", "sample_temp": "35",
		"ca": "100", "mg": "50", "na_k": "1000", "hco3": "300", "so4": "200", "cl": "900",
	}
	if !laboratoryScaleSixComponentReady(values) {
		t.Fatal("полный минимальный набор не распознан")
	}
	text := laboratoryScaleDecisionText(values, "davies")
	for _, want := range []string{"ИТОГОВЫЙ ОСТОРОЖНЫЙ ПРОГНОЗ", "требует уточнения", "Вероятность выпадения:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("в выводе нет %q: %s", want, text)
		}
	}
	delete(values, "mg")
	if laboratoryScaleSixComponentReady(values) {
		t.Fatal("неполный шестикомпонентный состав ошибочно принят как полный")
	}
}
