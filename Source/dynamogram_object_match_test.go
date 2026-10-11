//go:build windows

package main

import (
	"strings"
	"testing"
)

func preserveDynamogramMatchGlobals(t *testing.T) {
	t.Helper()
	oldTech := techCurrent
	oldTelemetry := telemetryImports
	oldHistory := dynHistory
	oldProfiles := installationProfiles
	t.Cleanup(func() {
		techCurrent = oldTech
		telemetryImports = oldTelemetry
		dynHistory = oldHistory
		installationProfiles = oldProfiles
	})
	installationProfiles = map[string]InstallationProfile{}
}

func TestDynamogramObjectContextUsesSavedInstallationProfile(t *testing.T) {
	preserveDynamogramMatchGlobals(t)
	techCurrent = []TechObjectState{{ObjectKey: "OBJ-1", Equipment: "ШГН", PumpModel: "НВ-44"}}
	installationProfiles = map[string]InstallationProfile{
		"obj1": {ObjectKey: "OBJ-1", EquipmentType: "ШГН", PumpDepth: 1500, StrokeLength: 3, UpdatedAt: "2026-09-16T10:00:00Z"},
	}
	text := dynamogramObjectContext(&DynamogramAnalysis{ID: "DYN-1", ObjectKey: "OBJ-1"})
	for _, want := range []string{"ПАРАМЕТРЫ УСТАНОВКИ", "сохранённый профиль установки", "глубина спуска насоса = 1500 м", "не окончательный диагноз"} {
		if !strings.Contains(text, want) {
			t.Fatalf("context misses %q: %s", want, text)
		}
	}
}

func TestDynamogramObjectMatchFindsAllSources(t *testing.T) {
	preserveDynamogramMatchGlobals(t)
	techCurrent = []TechObjectState{{
		ObjectKey: "OBJ-1", Equipment: "УШГН", PumpModel: "НВ-44",
		LiquidRate: "18,5", OilRate: "7,2", WaterCut: "61",
		DynamicLevel: "820", CurrentMRP: "210", BufferPressure: "1,4",
		LinePressure: "2,1", TMSTemperature: "42", PlannedActivities: "контроль режима",
	}}
	telemetryImports = []TelemetryImport{
		{ID: "TEL-OLD", ObjectKey: "OBJ-1", SourceSystem: "Телескоп", ImportedAt: "14.09.2026 10:00", StartedAt: "01.09.2026 00:00", EndedAt: "14.09.2026 00:00", Quality: "приемлемое"},
		{ID: "TEL-NEW", ObjectKey: "OBJ-1", SourceSystem: "WebSCADA", ImportedAt: "15.09.2026 10:00", StartedAt: "14.09.2026 00:00", EndedAt: "15.09.2026 00:00", Quality: "хорошее", Channels: []TelemetryChannelSummary{{Canonical: "motor_current"}}},
		{ID: "TEL-OTHER", ObjectKey: "OBJ-2", SourceSystem: "MegaWeb"},
	}
	dynHistory = []DynamogramAnalysis{
		{ID: "DYN-CURRENT", ObjectKey: "OBJ-1", Created: "2026-09-15T12:00:00Z"},
		{ID: "DYN-PREV", ObjectKey: "OBJ-1", Created: "2026-09-14T12:00:00Z", Quality: "хорошее", ReviewStatus: "confirmed", DetectionMode: "цифровой массив"},
		{ID: "DYN-OTHER", ObjectKey: "OBJ-2", Created: "2026-09-13T12:00:00Z"},
	}
	current := &DynamogramAnalysis{ID: "DYN-CURRENT", ObjectKey: "OBJ-1"}
	match := matchDynamogramObjectData(current)
	if !match.TechFound || match.TelemetryCount != 2 || match.LatestTelemetry.ID != "TEL-NEW" || match.PreviousDynamograms != 1 || match.LatestPrevious.ID != "DYN-PREV" || len(match.Missing) != 0 {
		t.Fatalf("unexpected full match: %#v", match)
	}
	text := dynamogramObjectContext(current)
	for _, want := range []string{
		"Ключ объекта: найден — OBJ-1", "Техрежим: найден",
		"Оборудование: найдено — УШГН", "Насос: найдено — НВ-44",
		"Дебит жидкости: найдено — 18,5", "Дебит нефти: найдено — 7,2",
		"Обводнённость: найдено — 61", "Динамический уровень: найдено — 820",
		"Режим: найдено — МРП 210 сут.", "Телеметрия: найдено импортов 2; последний — WebSCADA",
		"Предыдущие динамограммы: найдено 1", "Отсутствующие обязательные сопоставления: не выявлены",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("full context does not contain %q: %s", want, text)
		}
	}
}

func TestDynamogramObjectMatchListsMissingData(t *testing.T) {
	preserveDynamogramMatchGlobals(t)
	techCurrent = []TechObjectState{{ObjectKey: "OBJ-1", Equipment: "УШГН"}}
	telemetryImports = []TelemetryImport{}
	dynHistory = []DynamogramAnalysis{}
	current := &DynamogramAnalysis{ID: "DYN-1", ObjectKey: "OBJ-1"}
	match := matchDynamogramObjectData(current)
	for _, want := range []string{"насос в Техрежиме", "дебит жидкости в Техрежиме", "обводнённость в Техрежиме", "динамический уровень в Техрежиме", "режимные параметры в Техрежиме", "телеметрия объекта", "предыдущие динамограммы объекта"} {
		if !strings.Contains(strings.Join(match.Missing, " | "), want) {
			t.Fatalf("missing list does not contain %q: %#v", want, match.Missing)
		}
	}
	text := dynamogramObjectContext(current)
	for _, want := range []string{"Насос: отсутствует", "Телеметрия: для объекта не найдена", "Предыдущие динамограммы: для объекта не найдены", "Отсутствующие данные:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("partial context does not contain %q: %s", want, text)
		}
	}
}

func TestDynamogramObjectMatchRequiresObjectKey(t *testing.T) {
	preserveDynamogramMatchGlobals(t)
	techCurrent = nil
	telemetryImports = nil
	dynHistory = []DynamogramAnalysis{}
	text := dynamogramObjectContext(&DynamogramAnalysis{})
	for _, want := range []string{"Ключ объекта: отсутствует", "строка Техрежима", "телеметрия объекта", "предыдущие динамограммы объекта"} {
		if !strings.Contains(text, want) {
			t.Fatalf("unbound context does not contain %q: %s", want, text)
		}
	}
}

func TestDynamogramAxisUnitsAndSteps(t *testing.T) {
	if got := axisUnitASCII("кН"); got != "kN" {
		t.Fatalf("unexpected force unit: %q", got)
	}
	if got := axisUnitASCII("м"); got != "m" {
		t.Fatalf("unexpected stroke unit: %q", got)
	}
	if got := compactAxisNumber(0.2); got != "0.2" {
		t.Fatalf("unexpected step: %q", got)
	}
}
