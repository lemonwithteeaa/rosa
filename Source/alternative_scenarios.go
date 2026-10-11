//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const alternativeScenarioSchemaVersion = 1

const (
	scenarioAvailabilityUnknown = "Не проверено"
	scenarioAvailabilityReady   = "Доступно"
	scenarioAvailabilityPartial = "Частично доступно"
	scenarioAvailabilityMissing = "Недоступно"
)

// AlternativeMethodScenario is a versioned draft of one complete technical
// scheme. Object, classification and historical inputs are shared snapshots;
// method-specific choices, availability, forecast and economics are isolated.
type AlternativeMethodScenario struct {
	SchemaVersion       int      `json:"schema_version"`
	ID                  string   `json:"id"`
	ObjectKey           string   `json:"object_key"`
	Name                string   `json:"name"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	SourceSignature     string   `json:"source_signature"`
	Classification      string   `json:"classification,omitempty"`
	BaselineMRPDays     float64  `json:"baseline_mrp_days,omitempty"`
	TargetMRPDays       float64  `json:"target_mrp_days,omitempty"`
	Methods             []string `json:"methods,omitempty"`
	MethodsInitialized  bool     `json:"methods_initialized"`
	Availability        string   `json:"availability"`
	AvailabilityNote    string   `json:"availability_note,omitempty"`
	Status              string   `json:"status"`
	ApprovedAt          string   `json:"approved_at,omitempty"`
	ForecastLowDays     float64  `json:"forecast_low_days,omitempty"`
	ForecastBaseDays    float64  `json:"forecast_base_days,omitempty"`
	ForecastHighDays    float64  `json:"forecast_high_days,omitempty"`
	EconomicsStatus     string   `json:"economics_status,omitempty"`
	EconomicsCalculated string   `json:"economics_calculated_at,omitempty"`
	AnnualOPEXRub       float64  `json:"annual_opex_rub,omitempty"`
	OneTimeCAPEXRub     float64  `json:"one_time_capex_rub,omitempty"`
	NetBenefitK         float64  `json:"net_benefit_thousand_rub,omitempty"`
	BenefitCostRatio    float64  `json:"benefit_cost_ratio,omitempty"`
	PaybackMonths       *float64 `json:"payback_months,omitempty"`
	SelectionKind       string   `json:"selection_kind,omitempty"`
	SelectionBasis      string   `json:"selection_basis,omitempty"`
}

type alternativeScenarioStore struct {
	SchemaVersion int                         `json:"schema_version"`
	Scenarios     []AlternativeMethodScenario `json:"scenarios"`
	Active        map[string]string           `json:"active_by_object"`
}

var (
	alternativeScenariosLoaded bool
	alternativeScenarios       []AlternativeMethodScenario
	alternativeScenarioActive  = map[string]string{}
)

func alternativeScenariosPath() string { return sharedPath("alternative_method_scenarios.json") }

func ensureAlternativeScenariosLoaded() {
	if alternativeScenariosLoaded {
		return
	}
	alternativeScenariosLoaded = true
	var store alternativeScenarioStore
	loadJSON(alternativeScenariosPath(), &store)
	alternativeScenarios = store.Scenarios
	if store.Active != nil {
		alternativeScenarioActive = store.Active
	}
	if alternativeScenarioActive == nil {
		alternativeScenarioActive = map[string]string{}
	}
	// Remove only legacy synthetic objects from the retired training corpus.
	// Current 20-well scenarios and user-created scenarios remain intact.
	kept := alternativeScenarios[:0]
	removed := false
	for _, scenario := range alternativeScenarios {
		if isRetiredLegacyTeachingObjectKey(scenario.ObjectKey) {
			removed = true
			continue
		}
		kept = append(kept, scenario)
	}
	alternativeScenarios = kept
	for objectKey := range alternativeScenarioActive {
		if isRetiredLegacyTeachingObjectKey(objectKey) {
			delete(alternativeScenarioActive, objectKey)
			removed = true
		}
	}
	if removed {
		_ = saveAlternativeScenarios()
	}
	// A one-time, versioned and idempotent seed makes the 20 existing
	// teaching wells visible in the fund archive before opening each well.
	// Users' own scenarios are left untouched; subsequent deletions are respected.
	seedTeachingEconomicsFund816()
}

func saveAlternativeScenarios() error {
	store := alternativeScenarioStore{SchemaVersion: alternativeScenarioSchemaVersion, Scenarios: alternativeScenarios, Active: alternativeScenarioActive}
	return saveJSON(alternativeScenariosPath(), store)
}

func alternativeObjectIdentity(objectKey string) string { return methodsObjectIdentityKey(objectKey) }

func alternativeSourceSignature(objectKey string) string {
	parts := []string{alternativeObjectIdentity(objectKey)}
	if d, ok := latestApprovedMethodDecision(objectKey); ok {
		parts = append(parts, d.ID, d.ApprovedAt, d.Classification)
	}
	if v, ok := latestVersionForSelectedObject(); ok && methodsObjectKeysEquivalent(v.ObjectKey, objectKey) {
		parts = append(parts, v.VersionID, v.Created)
	}
	return strings.Join(parts, "|")
}

func scenarioLetter(index int) string {
	if index >= 0 && index < 26 {
		return string(rune('А' + index))
	}
	return fmt.Sprintf("%d", index+1)
}

func alternativeScenarioIndices(objectKey string) []int {
	ensureAlternativeScenariosLoaded()
	ensureTeachingAlternativeScenarios(objectKey)
	id := alternativeObjectIdentity(objectKey)
	indices := []int{}
	for i := range alternativeScenarios {
		if alternativeObjectIdentity(alternativeScenarios[i].ObjectKey) == id {
			indices = append(indices, i)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return alternativeScenarios[indices[i]].CreatedAt < alternativeScenarios[indices[j]].CreatedAt
	})
	return indices
}

func createBaseAlternativeScenario(objectKey string) int {
	now := time.Now().Format(time.RFC3339)
	classification, _, _ := approvedAssessmentForMethods(objectKey)
	target := targetRunlifeResolvedForSelectedObject()
	baseline := methodForecastSnapshotForSelectedObject().Base
	record := AlternativeMethodScenario{
		SchemaVersion: alternativeScenarioSchemaVersion, ID: "SCHEME-A", ObjectKey: strings.TrimSpace(objectKey), Name: "Схема А",
		CreatedAt: now, UpdatedAt: now, SourceSignature: alternativeSourceSignature(objectKey), Classification: classification,
		BaselineMRPDays: baseline, TargetMRPDays: target.Days, Availability: scenarioAvailabilityUnknown, Status: "Черновик",
	}
	alternativeScenarios = append(alternativeScenarios, record)
	alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = record.ID
	_ = saveAlternativeScenarios()
	return len(alternativeScenarios) - 1
}

func ensureBaseAlternativeScenario(objectKey string) int {
	indices := alternativeScenarioIndices(objectKey)
	if len(indices) == 0 {
		return createBaseAlternativeScenario(objectKey)
	}
	return indices[0]
}

func activeAlternativeScenarioIndex(objectKey string) int {
	base := ensureBaseAlternativeScenario(objectKey)
	activeID := alternativeScenarioActive[alternativeObjectIdentity(objectKey)]
	for _, i := range alternativeScenarioIndices(objectKey) {
		if strings.EqualFold(alternativeScenarios[i].ID, activeID) {
			return i
		}
	}
	alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = alternativeScenarios[base].ID
	return base
}

func activeAlternativeScenario(objectKey string) (AlternativeMethodScenario, bool) {
	if strings.TrimSpace(objectKey) == "" {
		return AlternativeMethodScenario{}, false
	}
	i := activeAlternativeScenarioIndex(objectKey)
	return alternativeScenarios[i], true
}

func activeAlternativeScenarioID(objectKey string) string {
	if s, ok := activeAlternativeScenario(objectKey); ok {
		return s.ID
	}
	return "SCHEME-A"
}

func updateActiveAlternativeScenario(objectKey string, mutate func(*AlternativeMethodScenario)) error {
	if strings.TrimSpace(objectKey) == "" {
		return fmt.Errorf("сначала закрепите объект на Главной")
	}
	i := activeAlternativeScenarioIndex(objectKey)
	mutate(&alternativeScenarios[i])
	alternativeScenarios[i].UpdatedAt = time.Now().Format(time.RFC3339)
	return saveAlternativeScenarios()
}

func updateAlternativeScenarioByID(objectKey, scenarioID string, mutate func(*AlternativeMethodScenario)) error {
	for _, i := range alternativeScenarioIndices(objectKey) {
		if strings.EqualFold(alternativeScenarios[i].ID, scenarioID) {
			mutate(&alternativeScenarios[i])
			alternativeScenarios[i].UpdatedAt = time.Now().Format(time.RFC3339)
			return saveAlternativeScenarios()
		}
	}
	return fmt.Errorf("схема %s не найдена", scenarioID)
}

func createAlternativeScenario(objectKey string, copyCurrent bool) (AlternativeMethodScenario, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return AlternativeMethodScenario{}, fmt.Errorf("сначала закрепите объект на Главной")
	}
	indices := alternativeScenarioIndices(objectKey)
	if len(indices) >= 8 {
		return AlternativeMethodScenario{}, fmt.Errorf("для одного объекта можно создать не более 8 параллельных схем")
	}
	base := alternativeScenarios[ensureBaseAlternativeScenario(objectKey)]
	now := time.Now().Format(time.RFC3339)
	letter := scenarioLetter(len(indices))
	record := AlternativeMethodScenario{
		SchemaVersion: alternativeScenarioSchemaVersion, ID: fmt.Sprintf("SCHEME-%d-%d", len(indices)+1, time.Now().UnixNano()),
		ObjectKey: objectKey, Name: "Схема " + letter, CreatedAt: now, UpdatedAt: now,
		SourceSignature: alternativeSourceSignature(objectKey), Classification: base.Classification, BaselineMRPDays: base.BaselineMRPDays,
		TargetMRPDays: base.TargetMRPDays, Availability: scenarioAvailabilityUnknown, Status: "Черновик",
		MethodsInitialized: true,
	}
	if copyCurrent {
		current := alternativeScenarios[activeAlternativeScenarioIndex(objectKey)]
		record.Methods = append([]string(nil), current.Methods...)
		record.Availability = current.Availability
		record.AvailabilityNote = current.AvailabilityNote
	}
	alternativeScenarios = append(alternativeScenarios, record)
	alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = record.ID
	if err := saveAlternativeScenarios(); err != nil {
		return AlternativeMethodScenario{}, err
	}
	return record, nil
}

func cycleAlternativeScenario(objectKey string, direction int) (AlternativeMethodScenario, bool) {
	indices := alternativeScenarioIndices(objectKey)
	if len(indices) == 0 {
		return AlternativeMethodScenario{}, false
	}
	current := activeAlternativeScenarioIndex(objectKey)
	pos := 0
	for i, idx := range indices {
		if idx == current {
			pos = i
			break
		}
	}
	pos = (pos + direction) % len(indices)
	if pos < 0 {
		pos += len(indices)
	}
	next := alternativeScenarios[indices[pos]]
	alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = next.ID
	_ = saveAlternativeScenarios()
	return next, true
}

func setBaseAlternativeScenario(objectKey string) (AlternativeMethodScenario, bool) {
	indices := alternativeScenarioIndices(objectKey)
	if len(indices) == 0 {
		return AlternativeMethodScenario{}, false
	}
	s := alternativeScenarios[indices[0]]
	alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = s.ID
	_ = saveAlternativeScenarios()
	return s, true
}

// setAlternativeScenarioActiveByID makes an existing parallel scheme the active
// working scenario without modifying its methods, forecast or economics.  The
// final-report page uses this for an explicit row selection / double-click flow.
func setAlternativeScenarioActiveByID(objectKey, scenarioID string) (AlternativeMethodScenario, error) {
	objectKey = strings.TrimSpace(objectKey)
	scenarioID = strings.TrimSpace(scenarioID)
	if objectKey == "" {
		return AlternativeMethodScenario{}, fmt.Errorf("сначала закрепите объект на Главной")
	}
	if scenarioID == "" {
		return AlternativeMethodScenario{}, fmt.Errorf("схема не выбрана")
	}
	for _, i := range alternativeScenarioIndices(objectKey) {
		if strings.EqualFold(strings.TrimSpace(alternativeScenarios[i].ID), scenarioID) {
			alternativeScenarioActive[alternativeObjectIdentity(objectKey)] = alternativeScenarios[i].ID
			if err := saveAlternativeScenarios(); err != nil {
				return AlternativeMethodScenario{}, err
			}
			return alternativeScenarios[i], nil
		}
	}
	return AlternativeMethodScenario{}, fmt.Errorf("схема %s не найдена", scenarioID)
}

func cycleActiveScenarioAvailability(objectKey string) (string, error) {
	states := []string{scenarioAvailabilityUnknown, scenarioAvailabilityReady, scenarioAvailabilityPartial, scenarioAvailabilityMissing}
	value := ""
	err := updateActiveAlternativeScenario(objectKey, func(s *AlternativeMethodScenario) {
		idx := 0
		for i, state := range states {
			if strings.EqualFold(s.Availability, state) {
				idx = i
				break
			}
		}
		s.Availability = states[(idx+1)%len(states)]
		s.Status = "Требует пересчёта"
		value = s.Availability
	})
	return value, err
}

func alternativeScenarioPosition(objectKey, scenarioID string) (int, int) {
	indices := alternativeScenarioIndices(objectKey)
	for i, idx := range indices {
		if strings.EqualFold(alternativeScenarios[idx].ID, scenarioID) {
			return i + 1, len(indices)
		}
	}
	return 1, len(indices)
}

func setAlternativeScenarioMethods(objectKey string, methods []string) {
	_ = updateActiveAlternativeScenario(objectKey, func(s *AlternativeMethodScenario) {
		s.Methods = append([]string(nil), normalizeMethodList(methods)...)
		s.MethodsInitialized = true
		s.Status = "Черновик"
		s.ApprovedAt = ""
		s.EconomicsStatus = ""
	})
}

func markAlternativeScenarioForecast(objectKey string, f CombinedMethodForecast) {
	_ = updateActiveAlternativeScenario(objectKey, func(s *AlternativeMethodScenario) {
		s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays = f.ForecastLow, f.ForecastBase, f.ForecastHigh
	})
}

func approveActiveAlternativeScenario(objectKey string) error {
	current, ok := activeAlternativeScenario(objectKey)
	if !ok {
		return fmt.Errorf("схема не найдена")
	}
	if len(alternativeScenarioMethodsForCalculation(current)) == 0 {
		return fmt.Errorf("в %s не выбраны методы и мероприятия", current.Name)
	}
	if current.Availability == scenarioAvailabilityMissing {
		return fmt.Errorf("%s недоступна по ресурсам: сначала выберите доступную схему либо измените статус после подтверждения оборудования и подрядчика", current.Name)
	}
	return updateActiveAlternativeScenario(objectKey, func(s *AlternativeMethodScenario) {
		s.SourceSignature = alternativeSourceSignature(objectKey)
		if s.Availability != scenarioAvailabilityReady {
			s.Status = "Условно утверждено"
		} else {
			s.Status = "Утверждено"
		}
		s.ApprovedAt = time.Now().Format(time.RFC3339)
	})
}

func alternativeScenariosForObject(objectKey string) []AlternativeMethodScenario {
	indices := alternativeScenarioIndices(objectKey)
	out := make([]AlternativeMethodScenario, 0, len(indices))
	currentSignature := alternativeSourceSignature(objectKey)
	for _, i := range indices {
		s := alternativeScenarios[i]
		if s.SourceSignature != "" && currentSignature != "" && s.SourceSignature != currentSignature && s.Status != "Черновик" {
			s.Status = "Исходные данные изменились"
		}
		out = append(out, s)
	}
	return out
}
