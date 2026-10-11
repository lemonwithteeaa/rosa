//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// TrainingEconomicsPoint816 is a read-only, reproducible economics snapshot
// for a real event ID from the 20-well synthetic corpus. The figures are never
// treated as measured production history or as a current production decision.
type TrainingEconomicsPoint816 struct {
	ObjectKey, ScenarioID, FailureID, LabPassID, Date string
	BaselineMRP, ForecastMRP, CAPEXRub, OPEXRub       float64
	Result                                            BaseEconomicsResult
}

const trainingEconomicsSeed816Name = "training20_economics_seed816.json"

// Seeding is a single startup transaction: reuse source signatures across the
// three alternatives belonging to one well, rather than repeating the whole
// Methods assessment pipeline for each row.
var trainingEconomicsSignatureCache816 map[string]string

func teachingEconomicsSourceSignature816(objectKey string) string {
	if trainingEconomicsSignatureCache816 == nil {
		return alternativeSourceSignature(objectKey)
	}
	key := alternativeObjectIdentity(objectKey)
	if signature, ok := trainingEconomicsSignatureCache816[key]; ok {
		return signature
	}
	signature := alternativeSourceSignature(objectKey)
	trainingEconomicsSignatureCache816[key] = signature
	return signature
}

// 2023-11 through 2026-10: the common corpus contains exactly three failures
// per well, in 2024, 2025 and 2026. One 12-month calculation per event and
// per alternative, always using CalculateBaseEconomics and its validation.
func teachingEconomicsHistory816(objectKey, scenarioID string) []TrainingEconomicsPoint816 {
	w, ok := trainingWellForObject(objectKey)
	if !ok {
		return nil
	}
	expectedPrefix := fmt.Sprintf("TRN-%02d-", w.Ordinal)
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(scenarioID)), expectedPrefix) {
		return nil
	}
	suffix := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(scenarioID)), expectedPrefix)
	index := -1
	switch suffix {
	case "A":
		index = 0
	case "B":
		index = 1
	case "C":
		index = 2
	}
	if index < 0 {
		return nil
	}
	template, ok := teachingAlternativeTemplates(objectKey)
	if !ok || len(template) != 3 {
		return nil
	}
	repairDays, repairRub, _, _, defaults := teachingEconomicsDefaultsForObject(objectKey)
	if !defaults || repairRub <= 0 {
		return nil
	}
	out := make([]TrainingEconomicsPoint816, 0, len(w.Failures))
	for _, event := range w.Failures {
		if event.MRP <= 0 || event.ResultingMRP <= 0 {
			continue
		}
		// Historical synthetic cost variation makes each year's result distinct.
		// No prices or production values are inferred from real wells.
		multiplier := 0.90 + 0.05*float64(event.EventNo)
		capex := template[index].OneTimeCAPEXRub * multiplier
		opex := template[index].AnnualOPEXRub * multiplier
		forecast := float64(event.ResultingMRP + index*30)
		low := math.Max(1, forecast-20)
		input := BaseEconomicsInput{
			CurrentMRPDays: float64(event.MRP), MRPLowDays: low,
			MRPBaseDays: forecast, MRPHighDays: forecast + 25,
			TargetMRPDays: 1000, TargetSource: "Учебный ориентир",
			SelectedScenario: "Центральный", HorizonMonths: 12,
			RepairDurationDays: repairDays, RepairCostK: repairRub / 1000,
			AnnualOPEXK: opex / 1000, OPEXInflationPct: 0,
			OneTimeMTRK: capex / 1000,
		}
		result := CalculateBaseEconomics(input)
		out = append(out, TrainingEconomicsPoint816{
			ObjectKey: w.ObjectKey, ScenarioID: scenarioID, FailureID: event.ID,
			LabPassID: event.LabPassID, Date: event.Date,
			BaselineMRP: float64(event.MRP), ForecastMRP: forecast,
			CAPEXRub: capex, OPEXRub: opex, Result: result,
		})
	}
	return out
}

// The current displayed scheme is the same 36-month formula, evaluated using
// the last failure from the three-year teaching timeline.
func teachingLatestEconomics816(objectKey, scenarioID string) (BaseEconomicsResult, bool) {
	history := teachingEconomicsHistory816(objectKey, scenarioID)
	if len(history) == 0 {
		return BaseEconomicsResult{}, false
	}
	last := history[len(history)-1]
	w, ok := trainingWellForObject(objectKey)
	if !ok || len(w.Failures) == 0 {
		return BaseEconomicsResult{}, false
	}
	repairDays, repairRub, _, _, defaults := teachingEconomicsDefaultsForObject(objectKey)
	if !defaults {
		return BaseEconomicsResult{}, false
	}
	input := BaseEconomicsInput{
		CurrentMRPDays: float64(w.Failures[len(w.Failures)-1].ResultingMRP),
		MRPLowDays:     math.Max(1, last.ForecastMRP-20),
		MRPBaseDays:    last.ForecastMRP, MRPHighDays: last.ForecastMRP + 25,
		TargetMRPDays: 1000, TargetSource: "Учебный ориентир",
		SelectedScenario: "Центральный", HorizonMonths: 36,
		RepairDurationDays: repairDays, RepairCostK: repairRub / 1000,
		AnnualOPEXK:      last.OPEXRub / 1000,
		OPEXInflationPct: 0, OneTimeMTRK: last.CAPEXRub / 1000,
	}
	result := CalculateBaseEconomics(input)
	return result, result.Status == "PASS"
}

// First launch of 816: seed all twenty existing wells in one persistence
// transaction. The receipt prevents deleted examples from being recreated on
// later launches. Existing user-created scenarios are never replaced.
func seedTeachingEconomicsFund816() {
	receipt := sharedPath(trainingEconomicsSeed816Name)
	if _, err := os.Stat(receipt); err == nil {
		return
	}
	corpus, err := loadTrainingCorpus()
	if err != nil {
		diagLog("ECON816: %v", err)
		return
	}
	if len(corpus.Wells) != 20 {
		diagLog("ECON816: invalid training count")
		return
	}
	trainingEconomicsSignatureCache816 = make(map[string]string, len(corpus.Wells))
	defer func() { trainingEconomicsSignatureCache816 = nil }()
	existing := make(map[string]int, len(alternativeScenarios))
	for i, s := range alternativeScenarios {
		existing[strings.ToUpper(strings.TrimSpace(s.ID))] = i
	}
	for _, well := range corpus.Wells {
		templates, ok := teachingAlternativeTemplates(well.ObjectKey)
		if !ok || len(templates) != 3 {
			diagLog("ECON816: incomplete %s", well.ID)
			return
		}
		for _, t := range templates {
			replacement := teachingAlternativeRecord(well.ObjectKey, well.Ordinal, t)
			id := strings.ToUpper(strings.TrimSpace(replacement.ID))
			if i, exists := existing[id]; exists {
				old := alternativeScenarios[i]
				// Only the known legacy fixture from the previous releases is
				// migrated; manually recalculated/edited scenarios stay as-is.
				if old.EconomicsCalculated == "2026-10-06T07:30:00Z" &&
					old.EconomicsStatus == "Расчёт актуален · учебный пример" &&
					strings.EqualFold(strings.TrimSpace(old.ObjectKey), well.ObjectKey) {
					alternativeScenarios[i] = replacement
				}
				continue
			}
			existing[id] = len(alternativeScenarios)
			alternativeScenarios = append(alternativeScenarios, replacement)
		}
		identity := alternativeObjectIdentity(well.ObjectKey)
		if strings.TrimSpace(alternativeScenarioActive[identity]) == "" {
			for _, t := range templates {
				if t.Selected {
					alternativeScenarioActive[identity] = t.ID
					break
				}
			}
		}
	}
	if err := saveAlternativeScenarios(); err != nil {
		diagLog("ECON816: save failed %v", err)
		return
	}
	// The receipt is written only after the complete snapshot was saved.
	if err := saveJSON(receipt, map[string]interface{}{
		"schema_version":          1,
		"source":                  "training_corpus.json",
		"synthetic":               true,
		"wells":                   20,
		"scenarios_per_well":      3,
		"history_events_per_well": 3,
		"formula":                 baseEconomicsFormulaVersion,
	}); err != nil {
		diagLog("ECON816: receipt failed %v", err)
	}
}
