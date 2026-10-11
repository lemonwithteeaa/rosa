//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

const multiMethodScenarioVersion = "MULTI-METHOD-1.0"
const maxMethodsPerScenario = 6

type ScenarioMethodLine struct {
	Method            string   `json:"method"`
	Status            string   `json:"status"`
	AnnualOPEXRub     float64  `json:"annual_opex_rub,omitempty"`
	OneTimeMTRRub     float64  `json:"one_time_mtr_rub,omitempty"`
	SpecificCostCodes []string `json:"specific_cost_codes,omitempty"`
}

type CombinedTechnicalScenario struct {
	Version            string               `json:"version"`
	ObjectKey          string               `json:"object_key"`
	ScenarioID         string               `json:"scenario_id"`
	GNO                string               `json:"gno,omitempty"`
	Complication       string               `json:"complication,omitempty"`
	MRPSource          string               `json:"mrp_source,omitempty"`
	MRPModelVersion    string               `json:"mrp_model_version,omitempty"`
	MRPBaseDays        float64              `json:"mrp_base_days,omitempty"`
	Methods            []ScenarioMethodLine `json:"methods"`
	TotalAnnualOPEXRub float64              `json:"total_annual_opex_rub,omitempty"`
	TotalOneTimeMTRRub float64              `json:"total_one_time_mtr_rub,omitempty"`
	Readiness          string               `json:"readiness"`
	Issues             []string             `json:"issues,omitempty"`
}

var combinedScenarioMethods = map[string][]string{}

// Methods can be reached through several human-readable representations of the
// same object. Keep one in-memory draft across those harmless caption changes.
func methodsObjectIdentityKey(value string) string {
	value = strings.TrimSpace(value)
	if n, ok := teachingWorkflowOrdinal(value); ok {
		return fmt.Sprintf("teaching:%03d", n)
	}
	if fp := laboratoryObjectKeyFingerprint(value); fp != "" {
		return fp
	}
	return strings.ToLower(value)
}

func methodsObjectKeysEquivalent(left, right string) bool {
	if strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right)) {
		return true
	}
	if laboratoryObjectKeysEquivalent(left, right) {
		return true
	}
	ln, lok := teachingWorkflowOrdinal(left)
	rn, rok := teachingWorkflowOrdinal(right)
	return lok && rok && ln == rn
}

func combinedMethodsForObject(objectKey string) ([]string, bool) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey != "" {
		if s, ok := activeAlternativeScenario(objectKey); ok && s.MethodsInitialized {
			return append([]string(nil), s.Methods...), true
		}
	}
	if methods, ok := combinedScenarioMethods[objectKey]; ok {
		return methods, true
	}
	identity := methodsObjectIdentityKey(objectKey)
	for key, methods := range combinedScenarioMethods {
		if methodsObjectIdentityKey(key) == identity || methodsObjectKeysEquivalent(key, objectKey) {
			return methods, true
		}
	}
	return nil, false
}

func clearCombinedMethods(objectKey string) {
	identity := methodsObjectIdentityKey(objectKey)
	for key := range combinedScenarioMethods {
		if methodsObjectIdentityKey(key) == identity || methodsObjectKeysEquivalent(key, objectKey) {
			delete(combinedScenarioMethods, key)
		}
	}
	for _, idx := range alternativeScenarioIndices(objectKey) {
		alternativeScenarios[idx].Status = "Исходные данные изменились"
	}
	_ = saveAlternativeScenarios()
}

func normalizeMethodList(methods []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		method = strings.TrimSpace(method)
		key := normalizeMethodText(method)
		if method == "" || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, method)
	}
	return out
}

func selectedCombinedMethods(c TechnicalScenarioContract) []string {
	key := strings.TrimSpace(c.ObjectKey)
	if explicit, ok := combinedMethodsForObject(key); ok {
		methods := normalizeMethodList(explicit)
		if len(methods) > maxMethodsPerScenario {
			methods = methods[:maxMethodsPerScenario]
		}
		if len(methods) > 0 {
			return methods
		}
		return nil
	}
	// Until the technologist explicitly creates a complex, the approved/current
	// single method remains the anchor. Once a complex is explicitly stored, we
	// do not silently inject another method into it.
	anchor := strings.TrimSpace(c.BaseMethod)
	if anchor == "" {
		return nil
	}
	return []string{anchor}
}

func setCombinedMethods(objectKey string, methods []string) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return
	}
	methods = normalizeMethodList(methods)
	if len(methods) > maxMethodsPerScenario {
		methods = methods[:maxMethodsPerScenario]
	}
	// Reuse an existing equivalent key so changing only the visible caption does
	// not create a second draft for the same object.
	target := objectKey
	identity := methodsObjectIdentityKey(objectKey)
	for key := range combinedScenarioMethods {
		if methodsObjectIdentityKey(key) == identity || methodsObjectKeysEquivalent(key, objectKey) {
			target = key
			break
		}
	}
	combinedScenarioMethods[target] = methods
	setAlternativeScenarioMethods(objectKey, methods)
}

func addCombinedMethod(c TechnicalScenarioContract) (string, bool) {
	methods := selectedCombinedMethods(c)
	if len(methods) >= maxMethodsPerScenario {
		return "", false
	}
	present := map[string]bool{}
	for _, m := range methods {
		present[normalizeMethodText(m)] = true
	}
	candidates := append([]MethodAlternative(nil), c.Alternatives...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return strings.ToLower(candidates[i].Method) < strings.ToLower(candidates[j].Method)
	})
	for _, a := range candidates {
		k := normalizeMethodText(a.Method)
		if k != "" && !present[k] {
			methods = append(methods, a.Method)
			setCombinedMethods(c.ObjectKey, methods)
			return a.Method, true
		}
	}
	return "", false
}

func removeCombinedMethod(c TechnicalScenarioContract) (string, bool) {
	methods := selectedCombinedMethods(c)
	if len(methods) <= 1 {
		return "", false
	}
	removed := methods[len(methods)-1]
	setCombinedMethods(c.ObjectKey, methods[:len(methods)-1])
	return removed, true
}

func resetCombinedMethods(c TechnicalScenarioContract) {
	setCombinedMethods(c.ObjectKey, []string{c.BaseMethod})
}

func methodStatusForCombined(c TechnicalScenarioContract, method string) string {
	if strings.EqualFold(normalizeMethodText(method), normalizeMethodText(c.BaseMethod)) {
		return "выбранный основной метод"
	}
	for _, a := range c.Alternatives {
		if strings.EqualFold(normalizeMethodText(method), normalizeMethodText(a.Method)) {
			return a.Status
		}
	}
	return "применимость требует проверки"
}

func combinedTechnicalScenario(c TechnicalScenarioContract) CombinedTechnicalScenario {
	out := CombinedTechnicalScenario{Version: multiMethodScenarioVersion, ObjectKey: c.ObjectKey, GNO: c.GNO, Complication: c.Complication, MRPSource: c.MRPSource, MRPModelVersion: c.MRPModelVersion, MRPBaseDays: c.MRPBaseDays}
	methods := selectedCombinedMethods(c)
	idParts := make([]string, 0, len(methods))
	for _, method := range methods {
		d := loadScenarioCostDraft(c.ObjectKey, method)
		reqs := methodSpecificCostRequirements(method)
		codes := make([]string, 0, len(reqs))
		for _, req := range reqs {
			if req.Required {
				codes = append(codes, req.Code)
			}
		}
		out.Methods = append(out.Methods, ScenarioMethodLine{Method: method, Status: methodStatusForCombined(c, method), AnnualOPEXRub: d.AnnualOPEXRub, OneTimeMTRRub: d.OneTimeMTRRub, SpecificCostCodes: uniqueStrings(codes)})
		out.TotalAnnualOPEXRub += d.AnnualOPEXRub
		out.TotalOneTimeMTRRub += d.OneTimeMTRRub
		idParts = append(idParts, normalizeMethodText(method))
	}
	sort.Strings(idParts)
	out.ScenarioID = fmt.Sprintf("%s|combo:%s|%s", c.ObjectKey, strings.Join(idParts, "+"), c.MRPModelVersion)
	if len(out.Methods) == 0 {
		out.Issues = append(out.Issues, "не выбран ни один метод")
	}
	if len(out.Methods) > 1 {
		out.Readiness = "КОМБИНИРОВАННЫЙ СЦЕНАРИЙ — ПРОВЕРИТЬ СОВМЕСТИМОСТЬ МЕТОДОВ"
	} else {
		out.Readiness = c.Readiness
	}
	if c.MRPBaseDays <= 0 {
		out.Issues = append(out.Issues, "нет базового прогноза МРП")
	}
	return out
}

func economicContractFromCombined(c TechnicalScenarioContract) EconomicScenarioContract {
	combo := combinedTechnicalScenario(c)
	methods := make([]string, 0, len(combo.Methods))
	details := make([]EconomicMethodCostLine, 0, len(combo.Methods))
	allCodes := []string{}
	for _, line := range combo.Methods {
		methods = append(methods, line.Method)
		allCodes = append(allCodes, line.SpecificCostCodes...)
		details = append(details, EconomicMethodCostLine{Method: line.Method, AnnualOPEXRub: line.AnnualOPEXRub, OneTimeMTRRub: line.OneTimeMTRRub, SpecificCostCodes: line.SpecificCostCodes})
	}
	return EconomicScenarioContract{TechnicalContractVersion: c.ContractVersion, ObjectKey: c.ObjectKey, ScenarioID: combo.ScenarioID, GNO: c.GNO, Complication: c.Complication, Method: strings.Join(methods, " + "), Methods: methods, MethodCosts: details, MRPSource: c.MRPSource, MRPModelVersion: c.MRPModelVersion, SpecificCostCodes: uniqueStrings(allCodes), SpecificAnnualOPEXRub: combo.TotalAnnualOPEXRub, SpecificOneTimeMTRRub: combo.TotalOneTimeMTRRub, SurvivalLowDays: c.SurvivalLowDays, SurvivalBaseDays: c.SurvivalBaseDays, SurvivalHighDays: c.SurvivalHighDays, HasSurvivalTriplet: c.HasSurvivalTriplet, SurvivalSource: c.SurvivalSource, SurvivalModelVersion: c.SurvivalModelVersion, SurvivalEventCount: c.SurvivalEventCount, SurvivalCensoredCount: c.SurvivalCensoredCount, CurrentCycleAgeDays: c.CurrentCycleAgeDays, SurvivalBacktestStatus: c.SurvivalBacktestStatus, SurvivalTarget: c.SurvivalTarget, SurvivalDistinctObjects: c.SurvivalDistinctObjects, SurvivalClassificationCoverage: c.SurvivalClassificationCoverage, SurvivalConfirmedFailures: c.SurvivalConfirmedFailures, SurvivalBrier90: c.SurvivalBrier90, SurvivalBrier180: c.SurvivalBrier180, SurvivalBrier365: c.SurvivalBrier365, SurvivalCompetingUnknown: c.SurvivalCompetingUnknown}
}

func combinedScenarioText(c TechnicalScenarioContract) string {
	combo := combinedTechnicalScenario(c)
	var b strings.Builder
	fmt.Fprintf(&b, "КОМБИНИРОВАННЫЙ СЦЕНАРИЙ МЕРОПРИЯТИЙ\r\nID: %s\r\n%s\r\n", combo.ScenarioID, combo.Readiness)
	fmt.Fprintf(&b, "Фактическое ГНО: %s; осложнение: %s; базовый МРП: %.0f сут. (%s / %s)\r\n", dash(combo.GNO), dash(combo.Complication), combo.MRPBaseDays, dash(combo.MRPSource), mrpUserModelName(combo.MRPModelVersion))
	if len(combo.Methods) == 0 {
		b.WriteString("Методы не выбраны.\r\n")
		return b.String()
	}
	b.WriteString("\r\nМетоды и собственные затраты:\r\n")
	for i, line := range combo.Methods {
		fmt.Fprintf(&b, "%d. %s — %s; эксплуатационные затраты (OPEX) %.0f руб./год; разово %.0f руб.; статьи: %s\r\n", i+1, line.Method, line.Status, line.AnnualOPEXRub, line.OneTimeMTRRub, dash(strings.Join(line.SpecificCostCodes, ", ")))
	}
	fmt.Fprintf(&b, "\r\nИтого для экономики: эксплуатационные затраты (OPEX) %.0f руб./год; разово %.0f руб.\r\n", combo.TotalAnnualOPEXRub, combo.TotalOneTimeMTRRub)
	methods := make([]string, 0, len(combo.Methods))
	for _, line := range combo.Methods {
		methods = append(methods, line.Method)
	}
	b.WriteString("Совместный эффект: " + methodInteractionSummary(methods) + "\r\n")
	b.WriteString("Эффект сочетания методов на МРП автоматически не начисляется простым сложением. Основной ориентир — фактические отказы/циклы за последние 36 месяцев; синергия требует подтверждения на факте объекта или сопоставимых аналогах.\r\n")
	if len(combo.Issues) > 0 {
		b.WriteString("Проверить: " + strings.Join(combo.Issues, "; ") + ".\r\n")
	}
	return b.String()
}
