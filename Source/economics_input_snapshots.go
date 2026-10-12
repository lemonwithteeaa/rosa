//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

// Exact source fields used for a successful economics calculation. Older
// scenario archives without a snapshot can still restore the forecast and
// scenario costs, but must not fabricate absent repair or horizon inputs.
type EconomicsInputSnapshot struct {
	ObjectKey   string     `json:"object_key"`
	ScenarioID  string     `json:"scenario_id"`
	Fields      [11]string `json:"fields"`
	CalculatedAt string    `json:"calculated_at"`
}

var economicsInputSnapshotsLoaded bool
var economicsInputSnapshots []EconomicsInputSnapshot

func economicsInputSnapshotsPath() string { return sharedPath("economics_input_snapshots.json") }

func ensureEconomicsInputSnapshotsLoaded() {
	if economicsInputSnapshotsLoaded {
		return
	}
	economicsInputSnapshotsLoaded = true
	economicsInputSnapshots = nil
	loadJSON(economicsInputSnapshotsPath(), &economicsInputSnapshots)
}

func economicsSnapshotMatch(record EconomicsInputSnapshot, objectKey, scenarioID string) bool {
	return alternativeObjectIdentity(record.ObjectKey) == alternativeObjectIdentity(objectKey) &&
		strings.EqualFold(strings.TrimSpace(record.ScenarioID), strings.TrimSpace(scenarioID))
}

func economicsSavedInputsForScenario(objectKey, scenarioID string) (EconomicsInputSnapshot, bool) {
	ensureEconomicsInputSnapshotsLoaded()
	for i := len(economicsInputSnapshots)-1; i >= 0; i-- {
		if economicsSnapshotMatch(economicsInputSnapshots[i], objectKey, scenarioID) {
			return economicsInputSnapshots[i], true
		}
	}
	return EconomicsInputSnapshot{}, false
}

func saveEconomicsInputsForScenario(scenario AlternativeMethodScenario) error {
	key := strings.TrimSpace(scenario.ObjectKey)
	if key == "" || strings.TrimSpace(scenario.ID) == "" {
		return fmt.Errorf("economic input snapshot requires an object and scenario")
	}
	ensureEconomicsInputSnapshotsLoaded()
	record := EconomicsInputSnapshot{
		ObjectKey: key, ScenarioID: scenario.ID, CalculatedAt: time.Now().Format(time.RFC3339),
	}
	for i, h := range economicsInputHandles() {
		record.Fields[i] = strings.TrimSpace(getText(h))
	}
	// Per-scheme values differ during the fleet-wide calculation; never save
	// the currently visible alternative as another scheme's forecast/cost.
	for _, n := range []struct{ index int; value float64 }{
		{1, scenario.ForecastLowDays}, {2, scenario.ForecastBaseDays}, {3, scenario.ForecastHighDays},
		{8, scenario.AnnualOPEXRub}, {10, scenario.OneTimeCAPEXRub},
	} {
		if n.value != 0 || n.index == 8 || n.index == 10 {
			record.Fields[n.index] = fmt.Sprintf("%.0f", n.value)
		} else {
			record.Fields[n.index] = ""
		}
	}
	backup := append([]EconomicsInputSnapshot(nil), economicsInputSnapshots...)
	found := false
	for i := range economicsInputSnapshots {
		if economicsSnapshotMatch(economicsInputSnapshots[i], key, scenario.ID) {
			economicsInputSnapshots[i], found = record, true
			break
		}
	}
	if !found {
		economicsInputSnapshots = append(economicsInputSnapshots, record)
	}
	if err := saveJSON(economicsInputSnapshotsPath(), economicsInputSnapshots); err != nil {
		economicsInputSnapshots = backup
		return err
	}
	return nil
}

// Restoring a calculation is a UI action, not a new approval or a new
// economic computation. It never changes the globally pinned well.
func restoreEconomicsFromArchive(s AlternativeMethodScenario) (bool, error) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" || alternativeObjectIdentity(key) != alternativeObjectIdentity(s.ObjectKey) {
		return false, fmt.Errorf("для переноса расчёта сначала закрепите этот же объект на Главной. Данные другого объекта не переносятся автоматически")
	}
	if _, err := setAlternativeScenarioActiveByID(key, s.ID); err != nil {
		return false, err
	}
	useSelectedObjectForEconomics()
	snapshot, saved := economicsSavedInputsForScenario(key, s.ID)
	if saved {
		for i, handle := range economicsInputHandles() {
			setText(handle, snapshot.Fields[i])
		}
	}
	message := "Из предыдущего расчёта восстановлены прогноз МРП и затраты выбранной схемы. Общие исходные параметры проверьте и при необходимости уточните."
	if saved {
		message = "Восстановлены сохранённые исходные параметры расчёта от " + snapshot.CalculatedAt + ". Изменение значений или методов требует нового пересчёта."
	}
	setText(econResult, message)
	refreshEconomicsDecisionIndicators()
	refreshEconomicsVisualReference()
	refreshEconomicsMetricCards()
	return saved, nil
}
