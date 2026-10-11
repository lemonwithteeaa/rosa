//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC220MRPCommandIDDoesNotConflictWithActivities(t *testing.T) {
	if ID_MRP_CALCULATE == ID_ACTIVITY_ADD_METHOD || ID_MRP_CALCULATE == ID_ACTIVITY_REMOVE_METHOD || ID_MRP_CALCULATE == ID_ACTIVITY_RESET_METHODS {
		t.Fatalf("идентификатор команды расчёта МРП конфликтует с командами методов: %d", ID_MRP_CALCULATE)
	}
}

func TestRC220RussianUserFacingMRPText(t *testing.T) {
	r := MRPForecastResult{ModelVersion: mrpForecastModelVersion, Status: "расчёт выполнен", EvidenceLevel: "ограниченная история"}
	text := formatMRPForecast(r)
	for _, forbidden := range []string{"Time-split", "survival-анализ", "baseline ", "Competing risks", "walk-forward PASS"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("в пользовательском тексте остался нерусифицированный термин %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "базовая модель МРП") || !strings.Contains(text, "анализ безотказной работы") {
		t.Fatalf("не найден ожидаемый русифицированный текст: %s", text)
	}
}

func TestRC220RussianBoolean(t *testing.T) {
	if ruYesNo(true) != "да" || ruYesNo(false) != "нет" {
		t.Fatal("логическое значение должно отображаться по-русски")
	}
}

func TestRC220EconomicsScenarioNamesAreRussianAndBackwardCompatible(t *testing.T) {
	cases := map[string]string{
		"Низкий":        "Осторожный",
		"Базовый":       "Центральный",
		"Высокий":       "Благоприятный",
		"Осторожный":    "Осторожный",
		"Центральный":   "Центральный",
		"Благоприятный": "Благоприятный",
	}
	for input, want := range cases {
		if got := normalizeScenarioName(input); got != want {
			t.Fatalf("%q: получено %q, ожидалось %q", input, got, want)
		}
	}
}

func TestRC220EconomicsStatusIsRussianForUser(t *testing.T) {
	if economicsStatusRu("PASS") != "расчёт выполнен" {
		t.Fatal("в пользовательском интерфейсе не должен отображаться PASS")
	}
}
