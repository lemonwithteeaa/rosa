package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultTargetRunlifeDays = 1000.0
	minTargetRunlifeDays     = 1000.0
	maxTargetRunlifeDays     = 5000.0
	defaultTargetNearPct     = 5.0
	minTargetNearPct         = 1.0
	maxTargetNearPct         = 20.0
)

const (
	targetScopeGlobal = "global"
	targetScopeLift   = "lift"
	targetScopeObject = "object"
)

type TargetRunlifeSettings struct {
	SchemaVersion int                   `json:"schema_version"`
	DefaultDays   float64               `json:"default_days"`
	NearPct       float64               `json:"near_pct,omitempty"`
	LiftDays      map[string]float64    `json:"lift_days,omitempty"`
	ObjectDays    map[string]float64    `json:"object_days,omitempty"`
	UpdatedAt     string                `json:"updated_at,omitempty"`
	History       []TargetRunlifeChange `json:"history,omitempty"`
}

type TargetRunlifeResolved struct {
	Days       float64
	Scope      string
	ScopeLabel string
	Key        string
	Source     string
	LiftType   string
	NearPct    float64
}

type TargetRunlifeChange struct {
	ChangedAt string  `json:"changed_at"`
	Scope     string  `json:"scope"`
	Key       string  `json:"key,omitempty"`
	OldDays   float64 `json:"old_days,omitempty"`
	NewDays   float64 `json:"new_days,omitempty"`
	Action    string  `json:"action"`
}

var targetRunlifeSettings = TargetRunlifeSettings{
	SchemaVersion: 2,
	DefaultDays:   defaultTargetRunlifeDays,
	NearPct:       defaultTargetNearPct,
	LiftDays:      map[string]float64{},
	ObjectDays:    map[string]float64{},
}

func targetRunlifePath() string { return filepath.Join(dataDir, "target_runlife.json") }

func normalizedTargetRunlifeDays(v float64) float64 {
	if !finiteNumber(v) || v < minTargetRunlifeDays || v > maxTargetRunlifeDays {
		return defaultTargetRunlifeDays
	}
	return v
}

func normalizedTargetNearPct(v float64) float64 {
	if !finiteNumber(v) || v < minTargetNearPct || v > maxTargetNearPct {
		return defaultTargetNearPct
	}
	return v
}

func normalizeTargetLiftType(equipment string) string {
	s := strings.ToUpper(strings.TrimSpace(equipment))
	s = strings.NewReplacer("Ё", "Е", "–", "-", "—", "-", "_", " ").Replace(s)
	switch {
	case strings.Contains(s, "УЭЦН") || strings.Contains(s, "ЭЦН") || strings.Contains(s, "ESP"):
		return "УЭЦН"
	case strings.Contains(s, "УШГН") || strings.Contains(s, "ШГН") || strings.Contains(s, "ШСН") || strings.Contains(s, "SRP"):
		return "УШГН"
	case strings.Contains(s, "ВШН") || strings.Contains(s, "ШВН") || strings.Contains(s, "УЭВН") || strings.Contains(s, "УЭОВН") || strings.Contains(s, "PCP") || strings.Contains(s, "ВИНТО"):
		return "ВШН"
	case strings.Contains(s, "ФОН"):
		return "Фонтанный"
	case strings.Contains(s, "ОРД"):
		return "ОРД"
	default:
		return strings.TrimSpace(equipment)
	}
}

func targetScopeLabel(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case targetScopeGlobal:
		return "общий ориентир"
	case targetScopeLift:
		return "ориентир по типу ГНО"
	case targetScopeObject:
		return "индивидуальный ориентир объекта"
	default:
		return "целевой ориентир"
	}
}

func loadTargetRunlifeSettings() {
	cfg := TargetRunlifeSettings{}
	loadJSON(targetRunlifePath(), &cfg)
	if cfg.SchemaVersion > 2 {
		diagLog("TARGET RUNLIFE newer schema=%d; using defaults", cfg.SchemaVersion)
		return
	}
	// Совместимость с RC245: схема 1 содержала только общий и объектовые ориентиры.
	cfg.SchemaVersion = 2
	cfg.DefaultDays = normalizedTargetRunlifeDays(cfg.DefaultDays)
	cfg.NearPct = normalizedTargetNearPct(cfg.NearPct)
	if cfg.LiftDays == nil {
		cfg.LiftDays = map[string]float64{}
	}
	if cfg.ObjectDays == nil {
		cfg.ObjectDays = map[string]float64{}
	}
	for k, v := range cfg.LiftDays {
		key := normalizeTargetLiftType(k)
		if key == "" || !finiteNumber(v) || v < minTargetRunlifeDays || v > maxTargetRunlifeDays {
			delete(cfg.LiftDays, k)
			continue
		}
		if key != k {
			delete(cfg.LiftDays, k)
			cfg.LiftDays[key] = v
		}
	}
	for k, v := range cfg.ObjectDays {
		key := normalizeLaboratoryObjectKey(k)
		if key == "" || !finiteNumber(v) || v < minTargetRunlifeDays || v > maxTargetRunlifeDays {
			delete(cfg.ObjectDays, k)
			continue
		}
		if key != k {
			delete(cfg.ObjectDays, k)
			cfg.ObjectDays[key] = v
		}
	}
	targetRunlifeSettings = cfg
}

func targetRunlifeResolved(objectKey, equipment string) TargetRunlifeResolved {
	key := normalizeLaboratoryObjectKey(objectKey)
	lift := normalizeTargetLiftType(equipment)
	near := normalizedTargetNearPct(targetRunlifeSettings.NearPct)
	if key != "" {
		if v, ok := targetRunlifeSettings.ObjectDays[key]; ok && v >= minTargetRunlifeDays && v <= maxTargetRunlifeDays {
			return TargetRunlifeResolved{Days: v, Scope: targetScopeObject, ScopeLabel: targetScopeLabel(targetScopeObject), Key: key, Source: "индивидуальная настройка объекта", LiftType: lift, NearPct: near}
		}
	}
	if lift != "" {
		if v, ok := targetRunlifeSettings.LiftDays[lift]; ok && v >= minTargetRunlifeDays && v <= maxTargetRunlifeDays {
			return TargetRunlifeResolved{Days: v, Scope: targetScopeLift, ScopeLabel: targetScopeLabel(targetScopeLift), Key: lift, Source: fmt.Sprintf("настройка для типа ГНО %s", lift), LiftType: lift, NearPct: near}
		}
	}
	v := normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	return TargetRunlifeResolved{Days: v, Scope: targetScopeGlobal, ScopeLabel: targetScopeLabel(targetScopeGlobal), Source: fmt.Sprintf("общий ориентир %.0f сут.", v), LiftType: lift, NearPct: near}
}

func targetRunlifeResolvedForSelectedObject() TargetRunlifeResolved {
	if st, ok := currentSelectedState(); ok {
		return targetRunlifeResolved(st.ObjectKey, st.Equipment)
	}
	if v, ok := latestVersionForSelectedObject(); ok {
		return targetRunlifeResolved(v.ObjectKey, v.Equipment)
	}
	return targetRunlifeResolved(selectedObjectKey, "")
}

func targetRunlifeDaysForObject(objectKey string) float64 {
	return targetRunlifeResolved(objectKey, "").Days
}

func targetRunlifeDaysForState(objectKey, equipment string) float64 {
	return targetRunlifeResolved(objectKey, equipment).Days
}

func targetRunlifeHasObjectOverride(objectKey string) bool {
	key := normalizeLaboratoryObjectKey(objectKey)
	_, ok := targetRunlifeSettings.ObjectDays[key]
	return key != "" && ok
}

func targetRunlifeHasLiftOverride(equipment string) bool {
	key := normalizeTargetLiftType(equipment)
	_, ok := targetRunlifeSettings.LiftDays[key]
	return key != "" && ok
}

func appendTargetRunlifeChange(scope, key, action string, oldDays, newDays float64) {
	change := TargetRunlifeChange{
		ChangedAt: time.Now().Format(time.RFC3339),
		Scope:     scope,
		Key:       strings.TrimSpace(key),
		OldDays:   oldDays,
		NewDays:   newDays,
		Action:    strings.TrimSpace(action),
	}
	targetRunlifeSettings.History = append(targetRunlifeSettings.History, change)
	const maxHistory = 200
	if len(targetRunlifeSettings.History) > maxHistory {
		targetRunlifeSettings.History = append([]TargetRunlifeChange(nil), targetRunlifeSettings.History[len(targetRunlifeSettings.History)-maxHistory:]...)
	}
}

func targetRunlifeLastChange() (TargetRunlifeChange, bool) {
	if len(targetRunlifeSettings.History) == 0 {
		return TargetRunlifeChange{}, false
	}
	return targetRunlifeSettings.History[len(targetRunlifeSettings.History)-1], true
}

func targetRunlifeLastChangeFor(scope, key string) (TargetRunlifeChange, bool) {
	scope = strings.TrimSpace(scope)
	key = strings.TrimSpace(key)
	for i := len(targetRunlifeSettings.History) - 1; i >= 0; i-- {
		c := targetRunlifeSettings.History[i]
		if c.Scope == scope && (key == "" || c.Key == key) {
			return c, true
		}
	}
	return TargetRunlifeChange{}, false
}

func targetRunlifeChangeText(c TargetRunlifeChange) string {
	when := strings.TrimSpace(c.ChangedAt)
	if t, err := time.Parse(time.RFC3339, when); err == nil {
		when = t.Local().Format("02.01.2006 15:04")
	}
	if when == "" {
		when = "дата не указана"
	}
	return fmt.Sprintf("последнее изменение %s: %.0f → %.0f сут. (%s)", when, c.OldDays, c.NewDays, c.Action)
}

func targetRunlifeDeviationNote(days float64) string {
	base := normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	if base <= 0 || days <= 0 {
		return ""
	}
	delta := (days/base - 1) * 100
	if delta > 25 || delta < -25 {
		return fmt.Sprintf("Ориентир отличается от общего более чем на 25%% (%+.0f%%); проверьте обоснование настройки.", delta)
	}
	return ""
}

func prepareTargetRunlifeSettingsForSave() {
	targetRunlifeSettings.SchemaVersion = 2
	targetRunlifeSettings.DefaultDays = normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	targetRunlifeSettings.NearPct = normalizedTargetNearPct(targetRunlifeSettings.NearPct)
	if targetRunlifeSettings.LiftDays == nil {
		targetRunlifeSettings.LiftDays = map[string]float64{}
	}
	if targetRunlifeSettings.ObjectDays == nil {
		targetRunlifeSettings.ObjectDays = map[string]float64{}
	}
	targetRunlifeSettings.UpdatedAt = time.Now().Format(time.RFC3339)
}

func saveTargetRunlifeGlobal(days float64) error {
	if !finiteNumber(days) || days < minTargetRunlifeDays || days > maxTargetRunlifeDays {
		return fmt.Errorf("целевой ориентир наработки должен быть от %.0f до %.0f суток", minTargetRunlifeDays, maxTargetRunlifeDays)
	}
	old := normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	targetRunlifeSettings.DefaultDays = days
	appendTargetRunlifeChange(targetScopeGlobal, "", "сохранено", old, days)
	prepareTargetRunlifeSettingsForSave()
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func resetTargetRunlifeGlobal() error {
	old := normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	targetRunlifeSettings.DefaultDays = defaultTargetRunlifeDays
	appendTargetRunlifeChange(targetScopeGlobal, "", "сброшено", old, defaultTargetRunlifeDays)
	prepareTargetRunlifeSettingsForSave()
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func saveTargetRunlifeForLift(equipment string, days float64) error {
	lift := normalizeTargetLiftType(equipment)
	if lift == "" {
		return fmt.Errorf("не определён тип ГНО для сохранения целевого ориентира")
	}
	if !finiteNumber(days) || days < minTargetRunlifeDays || days > maxTargetRunlifeDays {
		return fmt.Errorf("целевой ориентир наработки должен быть от %.0f до %.0f суток", minTargetRunlifeDays, maxTargetRunlifeDays)
	}
	prepareTargetRunlifeSettingsForSave()
	old := targetRunlifeDaysForScope(targetScopeLift, "", lift)
	targetRunlifeSettings.LiftDays[lift] = days
	appendTargetRunlifeChange(targetScopeLift, lift, "сохранено", old, days)
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func saveTargetRunlifeForObject(objectKey string, days float64) error {
	return saveTargetRunlifeForObjectWithEquipment(objectKey, "", days)
}

func saveTargetRunlifeForObjectWithEquipment(objectKey, equipment string, days float64) error {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return fmt.Errorf("не выбран объект для сохранения целевого ориентира")
	}
	if !finiteNumber(days) || days < minTargetRunlifeDays || days > maxTargetRunlifeDays {
		return fmt.Errorf("целевой ориентир наработки должен быть от %.0f до %.0f суток", minTargetRunlifeDays, maxTargetRunlifeDays)
	}
	old := targetRunlifeResolved(objectKey, equipment).Days
	prepareTargetRunlifeSettingsForSave()
	targetRunlifeSettings.ObjectDays[key] = days
	appendTargetRunlifeChange(targetScopeObject, key, "сохранено", old, days)
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func resetTargetRunlifeForLift(equipment string) error {
	lift := normalizeTargetLiftType(equipment)
	if lift == "" {
		return fmt.Errorf("не определён тип ГНО")
	}
	prepareTargetRunlifeSettingsForSave()
	old, existed := targetRunlifeSettings.LiftDays[lift]
	delete(targetRunlifeSettings.LiftDays, lift)
	newDays := normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	if existed {
		appendTargetRunlifeChange(targetScopeLift, lift, "сброшено", old, newDays)
	}
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func resetTargetRunlifeForObject(objectKey string) error {
	return resetTargetRunlifeForObjectWithEquipment(objectKey, "")
}

func resetTargetRunlifeForObjectWithEquipment(objectKey, equipment string) error {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return fmt.Errorf("не выбран объект")
	}
	prepareTargetRunlifeSettingsForSave()
	old, existed := targetRunlifeSettings.ObjectDays[key]
	delete(targetRunlifeSettings.ObjectDays, key)
	if existed {
		newDays := targetRunlifeResolved(objectKey, equipment).Days
		appendTargetRunlifeChange(targetScopeObject, key, "сброшено", old, newDays)
	}
	return saveJSON(targetRunlifePath(), targetRunlifeSettings)
}

func targetRunlifeSourceText(objectKey string) string {
	return targetRunlifeResolved(objectKey, "").Source
}

func targetRunlifeSourceTextForState(objectKey, equipment string) string {
	return targetRunlifeResolved(objectKey, equipment).Source
}

func targetRunlifeDaysForScope(scope, objectKey, equipment string) float64 {
	switch scope {
	case targetScopeObject:
		key := normalizeLaboratoryObjectKey(objectKey)
		if v, ok := targetRunlifeSettings.ObjectDays[key]; ok {
			return normalizedTargetRunlifeDays(v)
		}
		return targetRunlifeResolved(objectKey, equipment).Days
	case targetScopeLift:
		lift := normalizeTargetLiftType(equipment)
		if v, ok := targetRunlifeSettings.LiftDays[lift]; ok {
			return normalizedTargetRunlifeDays(v)
		}
		return normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	default:
		return normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)
	}
}

func targetRunlifeComparisonWithTolerance(forecastDays, targetDays, nearPct float64) (gapDays, ratioPct float64, status string) {
	targetDays = normalizedTargetRunlifeDays(targetDays)
	nearPct = normalizedTargetNearPct(nearPct)
	if forecastDays <= 0 || !finiteNumber(forecastDays) {
		return 0, 0, "прогноз для сравнения не рассчитан"
	}
	gapDays = forecastDays - targetDays
	ratioPct = forecastDays / targetDays * 100
	deltaPct := (forecastDays/targetDays - 1) * 100
	switch {
	case deltaPct < -nearPct:
		status = "ниже целевого ориентира"
	case deltaPct > nearPct:
		status = "выше целевого ориентира"
	default:
		status = "вблизи целевого ориентира"
	}
	return
}

func targetRunlifeComparison(forecastDays, targetDays float64) (gapDays, ratioPct float64, status string) {
	return targetRunlifeComparisonWithTolerance(forecastDays, targetDays, targetRunlifeSettings.NearPct)
}

func targetRunlifeSummary(forecastDays, targetDays float64) string {
	gap, ratio, status := targetRunlifeComparison(forecastDays, targetDays)
	if forecastDays <= 0 {
		return fmt.Sprintf("Целевой ориентир: %.0f сут.; %s.", normalizedTargetRunlifeDays(targetDays), status)
	}
	return fmt.Sprintf("Целевой ориентир: %.0f сут.; прогноз %.0f сут. — %s (%+.0f сут.; %.0f%% от ориентира).", normalizedTargetRunlifeDays(targetDays), forecastDays, strings.TrimSpace(status), gap, ratio)
}
