//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

const technicalScenarioContractVersion = "TECH-SCENARIO-1.1"

type MethodAlternative struct {
	Method string `json:"method"`
	Status string `json:"status"`
	Basis  string `json:"basis,omitempty"`
}

type SpecificCostRequirement struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Basis    string `json:"basis"`
}

type TechnicalScenarioContract struct {
	ContractVersion                string                    `json:"contract_version"`
	ObjectKey                      string                    `json:"object_key"`
	ScenarioID                     string                    `json:"scenario_id"`
	GNO                            string                    `json:"gno,omitempty"`
	Complication                   string                    `json:"complication,omitempty"`
	BaseMethod                     string                    `json:"base_method,omitempty"`
	CanonicalMethod                string                    `json:"canonical_method,omitempty"`
	Alternatives                   []MethodAlternative       `json:"alternatives,omitempty"`
	MRPSource                      string                    `json:"mrp_source,omitempty"`
	MRPModelVersion                string                    `json:"mrp_model_version,omitempty"`
	CurrentMRPDays                 float64                   `json:"current_mrp_days,omitempty"`
	MRPLowDays                     float64                   `json:"mrp_low_days,omitempty"`
	MRPBaseDays                    float64                   `json:"mrp_base_days,omitempty"`
	MRPHighDays                    float64                   `json:"mrp_high_days,omitempty"`
	HasLowHigh                     bool                      `json:"has_low_high"`
	SurvivalLowDays                float64                   `json:"survival_low_days,omitempty"`
	SurvivalBaseDays               float64                   `json:"survival_base_days,omitempty"`
	SurvivalHighDays               float64                   `json:"survival_high_days,omitempty"`
	HasSurvivalTriplet             bool                      `json:"has_survival_triplet"`
	SurvivalSource                 string                    `json:"survival_source,omitempty"`
	SurvivalModelVersion           string                    `json:"survival_model_version,omitempty"`
	SurvivalEventCount             int                       `json:"survival_event_count,omitempty"`
	SurvivalCensoredCount          int                       `json:"survival_censored_count,omitempty"`
	CurrentCycleAgeDays            float64                   `json:"current_cycle_age_days,omitempty"`
	SurvivalBacktestStatus         string                    `json:"survival_backtest_status,omitempty"`
	SurvivalTarget                 string                    `json:"survival_target,omitempty"`
	SurvivalDistinctObjects        int                       `json:"survival_distinct_objects,omitempty"`
	SurvivalClassificationCoverage float64                   `json:"survival_classification_coverage,omitempty"`
	SurvivalConfirmedFailures      int                       `json:"survival_confirmed_failures,omitempty"`
	SurvivalBrier90                float64                   `json:"survival_brier_90,omitempty"`
	SurvivalBrier180               float64                   `json:"survival_brier_180,omitempty"`
	SurvivalBrier365               float64                   `json:"survival_brier_365,omitempty"`
	SurvivalCompetingUnknown       int                       `json:"survival_competing_unknown,omitempty"`
	Readiness                      string                    `json:"readiness"`
	ReadinessIssues                []string                  `json:"readiness_issues,omitempty"`
	SpecificCosts                  []SpecificCostRequirement `json:"specific_cost_requirements,omitempty"`
}

type EconomicMethodCostLine struct {
	Method            string   `json:"method"`
	AnnualOPEXRub     float64  `json:"annual_opex_rub,omitempty"`
	OneTimeMTRRub     float64  `json:"one_time_mtr_rub,omitempty"`
	SpecificCostCodes []string `json:"specific_cost_codes,omitempty"`
}

type EconomicScenarioContract struct {
	TechnicalContractVersion       string                   `json:"technical_contract_version"`
	ObjectKey                      string                   `json:"object_key"`
	ScenarioID                     string                   `json:"scenario_id"`
	GNO                            string                   `json:"gno,omitempty"`
	Complication                   string                   `json:"complication,omitempty"`
	Method                         string                   `json:"method,omitempty"`
	Methods                        []string                 `json:"methods,omitempty"`
	MethodCosts                    []EconomicMethodCostLine `json:"method_costs,omitempty"`
	MRPSource                      string                   `json:"mrp_source,omitempty"`
	MRPModelVersion                string                   `json:"mrp_model_version,omitempty"`
	SpecificCostCodes              []string                 `json:"specific_cost_codes,omitempty"`
	SpecificAnnualOPEXRub          float64                  `json:"specific_annual_opex_rub,omitempty"`
	SpecificOneTimeMTRRub          float64                  `json:"specific_one_time_mtr_rub,omitempty"`
	SurvivalLowDays                float64                  `json:"survival_low_days,omitempty"`
	SurvivalBaseDays               float64                  `json:"survival_base_days,omitempty"`
	SurvivalHighDays               float64                  `json:"survival_high_days,omitempty"`
	HasSurvivalTriplet             bool                     `json:"has_survival_triplet"`
	SurvivalSource                 string                   `json:"survival_source,omitempty"`
	SurvivalModelVersion           string                   `json:"survival_model_version,omitempty"`
	SurvivalEventCount             int                      `json:"survival_event_count,omitempty"`
	SurvivalCensoredCount          int                      `json:"survival_censored_count,omitempty"`
	CurrentCycleAgeDays            float64                  `json:"current_cycle_age_days,omitempty"`
	SurvivalBacktestStatus         string                   `json:"survival_backtest_status,omitempty"`
	SurvivalTarget                 string                   `json:"survival_target,omitempty"`
	SurvivalDistinctObjects        int                      `json:"survival_distinct_objects,omitempty"`
	SurvivalClassificationCoverage float64                  `json:"survival_classification_coverage,omitempty"`
	SurvivalConfirmedFailures      int                      `json:"survival_confirmed_failures,omitempty"`
	SurvivalBrier90                float64                  `json:"survival_brier_90,omitempty"`
	SurvivalBrier180               float64                  `json:"survival_brier_180,omitempty"`
	SurvivalBrier365               float64                  `json:"survival_brier_365,omitempty"`
	SurvivalCompetingUnknown       int                      `json:"survival_competing_unknown,omitempty"`
}

// Scenario-specific manual inputs are kept separately for every object+method.
// Switching a technical alternative must not silently reuse costs from another method.
type ScenarioCostDraft struct {
	AnnualOPEXRub float64 `json:"annual_opex_rub,omitempty"`
	OneTimeMTRRub float64 `json:"one_time_mtr_rub,omitempty"`
}

var technicalMethodSelection = map[string]string{}
var technicalScenarioCostDrafts = map[string]ScenarioCostDraft{}

func scenarioDraftKey(objectKey, method string) string {
	return strings.TrimSpace(objectKey) + "|" + activeAlternativeScenarioID(objectKey) + "|" + normalizeMethodText(method)
}

func scenarioDraftKeyForID(objectKey, scenarioID, method string) string {
	return strings.TrimSpace(objectKey) + "|" + strings.TrimSpace(scenarioID) + "|" + normalizeMethodText(method)
}

func legacyScenarioDraftKey(objectKey, method string) string {
	return strings.TrimSpace(objectKey) + "|" + normalizeMethodText(method)
}

func selectedTechnicalMethod(objectKey, baseMethod string, alternatives []MethodAlternative) string {
	key := strings.TrimSpace(objectKey)
	selected := strings.TrimSpace(technicalMethodSelection[key])
	if selected == "" || strings.EqualFold(selected, strings.TrimSpace(baseMethod)) {
		return strings.TrimSpace(baseMethod)
	}
	for _, a := range alternatives {
		if strings.EqualFold(strings.TrimSpace(a.Method), selected) {
			return strings.TrimSpace(a.Method)
		}
	}
	return strings.TrimSpace(baseMethod)
}

func setTechnicalMethodSelection(objectKey, method string) {
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return
	}
	technicalMethodSelection[key] = strings.TrimSpace(method)
}

func saveScenarioCostDraft(objectKey, method string, annualOPEXRub, oneTimeMTRRub float64) {
	if strings.TrimSpace(objectKey) == "" || strings.TrimSpace(method) == "" {
		return
	}
	technicalScenarioCostDrafts[scenarioDraftKey(objectKey, method)] = ScenarioCostDraft{AnnualOPEXRub: annualOPEXRub, OneTimeMTRRub: oneTimeMTRRub}
}

func loadScenarioCostDraft(objectKey, method string) ScenarioCostDraft {
	if d, ok := technicalScenarioCostDrafts[scenarioDraftKey(objectKey, method)]; ok {
		return d
	}
	return technicalScenarioCostDrafts[legacyScenarioDraftKey(objectKey, method)]
}

func loadScenarioCostDraftForID(objectKey, scenarioID, method string) ScenarioCostDraft {
	if d, ok := technicalScenarioCostDrafts[scenarioDraftKeyForID(objectKey, scenarioID, method)]; ok {
		return d
	}
	return technicalScenarioCostDrafts[legacyScenarioDraftKey(objectKey, method)]
}

func cycleTechnicalMethod(c TechnicalScenarioContract, direction int) string {
	methods := []string{strings.TrimSpace(c.BaseMethod)}
	for _, a := range c.Alternatives {
		if strings.TrimSpace(a.Method) != "" {
			methods = append(methods, strings.TrimSpace(a.Method))
		}
	}
	if len(methods) == 0 {
		return ""
	}
	current := strings.TrimSpace(selectedTechnicalMethod(c.ObjectKey, c.BaseMethod, c.Alternatives))
	idx := 0
	for i, m := range methods {
		if strings.EqualFold(m, current) {
			idx = i
			break
		}
	}
	idx = (idx + direction) % len(methods)
	if idx < 0 {
		idx += len(methods)
	}
	setTechnicalMethodSelection(c.ObjectKey, methods[idx])
	return methods[idx]
}

func normalizeComplicationFamily(s string) string {
	n := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(n, "корроз") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "К"):
		return "коррозия"
	case strings.Contains(n, "сол") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "С"):
		return "соли"
	case strings.Contains(n, "аспо") || strings.Contains(n, "парафин") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "А"):
		return "аспо"
	case strings.Contains(n, "мех") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "М"):
		return "мехпримеси"
	case strings.Contains(n, "вяз") || strings.Contains(n, "эмуль") || strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "В"):
		return "вязкость/эмульсия"
	default:
		return n
	}
}

func technicalAlternatives(complication, baseMethod string) []MethodAlternative {
	family := normalizeComplicationFamily(complication)
	seen := map[string]bool{}
	var out []MethodAlternative
	for _, r := range ruleCatalog.Rules {
		if family != "" && !strings.Contains(normalizeComplicationFamily(r.Complication), family) && !strings.Contains(family, normalizeComplicationFamily(r.Complication)) {
			continue
		}
		name := strings.TrimSpace(r.Method)
		if name == "" || strings.EqualFold(name, strings.TrimSpace(baseMethod)) || seen[normalizeMethodText(name)] {
			continue
		}
		seen[normalizeMethodText(name)] = true
		status := "применимость требует проверки"
		if strings.TrimSpace(r.PreliminaryStatus) != "" {
			status = strings.TrimSpace(r.PreliminaryStatus)
		} else if strings.TrimSpace(r.CriterionStatus) != "" {
			status = strings.TrimSpace(r.CriterionStatus)
		}
		basis := strings.TrimSpace(r.Purpose)
		if basis == "" && len(r.Criteria) > 0 {
			basis = strings.Join(r.Criteria, "; ")
		}
		out = append(out, MethodAlternative{Method: name, Status: status, Basis: basis})
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Method) < strings.ToLower(out[j].Method) })
	return out
}

func methodSpecificCostRequirements(method string) []SpecificCostRequirement {
	n := normalizeMethodText(method)
	req := []SpecificCostRequirement{}
	add := func(code, name, basis string) {
		req = append(req, SpecificCostRequirement{Code: code, Name: name, Required: true, Basis: basis})
	}
	switch {
	case strings.Contains(n, "ингибитор") || strings.Contains(n, "деэмульг") || strings.Contains(n, "реагент"):
		add("reagent", "Реагент: дозировка, цена, периодичность и логистика", "Заполняется только для реагентного метода; сумму приложение не подставляет автоматически.")
	}
	if strings.Contains(n, "промыв") || strings.Contains(n, "кислот") || strings.Contains(n, "растворител") {
		add("wash", "Разовая обработка/промывка: объём состава, цена, техника и работа", "Заполняется только для разовой обработки.")
	}
	if strings.Contains(n, "покрыт") || strings.Contains(n, "нкт") || strings.Contains(n, "гно") || strings.Contains(n, "кабель") || strings.Contains(n, "фильтр") || strings.Contains(n, "контейнер") {
		add("equipment", "Оборудование/МТР: капитальные затраты (CAPEX), монтаж/СПО, ресурс и замена", "Заполняется только если метод требует оборудования или специального МТР.")
	}
	return req
}

func technicalScenarioReadiness(c TechnicalScenarioContract) (string, []string) {
	var issues []string
	if strings.TrimSpace(c.ObjectKey) == "" {
		issues = append(issues, "не выбран объект")
	}
	if c.CurrentMRPDays <= 0 {
		issues = append(issues, "нет текущего МРП")
	}
	if c.MRPBaseDays <= 0 {
		issues = append(issues, "нет базового прогноза МРП")
	}
	if strings.TrimSpace(c.Complication) == "" {
		issues = append(issues, "не зафиксирован класс/вид осложнения")
	}
	if strings.TrimSpace(c.BaseMethod) == "" {
		issues = append(issues, "не выбран базовый метод")
	}
	if len(issues) == 0 {
		return "ГОТОВ К ЭКОНОМИЧЕСКОЙ ОЦЕНКЕ", nil
	}
	if c.MRPBaseDays > 0 && strings.TrimSpace(c.ObjectKey) != "" {
		return "ЧАСТИЧНО ГОТОВ — НУЖНА ПРОВЕРКА СЦЕНАРИЯ МЕРОПРИЯТИЯ", issues
	}
	return "НЕ ГОТОВ — НЕДОСТАТОЧНО ИСХОДНЫХ ДАННЫХ", issues
}

func technicalScenarioForSelectedObject() (TechnicalScenarioContract, bool) {
	var c TechnicalScenarioContract
	st, hasState := currentSelectedState()
	v, hasV := latestVersionForSelectedObject()
	if !hasState && !hasV {
		return c, false
	}
	c.ContractVersion = technicalScenarioContractVersion
	c.ObjectKey = strings.TrimSpace(selectedObjectKey)
	c.GNO = strings.TrimSpace(st.Equipment)
	if c.GNO == "" {
		c.GNO = strings.TrimSpace(v.Equipment)
	}
	c.Complication = strings.TrimSpace(v.Complication)
	if approved := approvedPrimaryComplication(c.ObjectKey); approved != "" {
		c.Complication = approved
	}
	originalBaseMethod := strings.TrimSpace(v.Method)
	if approvedMethod, ok := latestApprovedMethodForObject(c.ObjectKey); ok {
		originalBaseMethod = approvedMethod
	}
	c.CanonicalMethod = originalBaseMethod
	c.BaseMethod = originalBaseMethod
	if x, err := parseEconomicNumber(economicsCurrentMRP(st.CurrentMRP, v.CurrentMRP), "Текущий МРП", false); err == nil {
		c.CurrentMRPDays = x
	}
	if in, ok := mrpForecastInputForSelectedObject(); ok {
		r := calculateMRPForecast(in)
		c.MRPSource = "текущий расчёт"
		c.MRPModelVersion = r.ModelVersion
		c.MRPBaseDays, c.MRPLowDays, c.MRPHighDays, c.HasLowHigh = r.BaseDays, r.LowDays, r.HighDays, r.HasLowHigh
		c.SurvivalLowDays, c.SurvivalBaseDays, c.SurvivalHighDays, c.HasSurvivalTriplet = r.SurvivalLowDays, r.SurvivalBaseDays, r.SurvivalHighDays, r.HasSurvivalTriplet
		c.SurvivalSource = r.SurvivalEconSource
		c.SurvivalModelVersion = r.Advanced.ObjectAny.ModelVersion
		c.SurvivalEventCount = r.Advanced.ObjectAny.EventCount
		c.SurvivalCensoredCount = r.Advanced.ObjectAny.CensoredCount
		c.CurrentCycleAgeDays = r.Advanced.CurrentAgeDays
		c.SurvivalTarget = "до любого следующего вмешательства"
		provSummary := r.Advanced.ObjectAny
		provClass := r.Advanced.Classification
		provCompeting := r.Advanced.Competing
		if strings.Contains(strings.ToLower(r.SurvivalEconSource), "сопоставимой группе") {
			provSummary = r.Advanced.CohortAny
			provClass = r.Advanced.CohortClassification
			provCompeting = r.Advanced.CohortCompeting
		}
		c.SurvivalDistinctObjects = provSummary.DistinctObjectCount
		c.SurvivalClassificationCoverage = provClass.Coverage
		c.SurvivalConfirmedFailures = provClass.ConfirmedFailures
		c.SurvivalBrier90 = provSummary.Backtest.Brier90
		c.SurvivalBrier180 = provSummary.Backtest.Brier180
		c.SurvivalBrier365 = provSummary.Backtest.Brier365
		c.SurvivalCompetingUnknown = provCompeting.UnknownCount
		if r.Advanced.ObjectAny.Backtest.Eligible {
			c.SurvivalBacktestStatus = "последовательная проверка подтверждена"
		} else {
			c.SurvivalBacktestStatus = "последовательная проверка не подтверждена"
		}
	}
	if c.MRPBaseDays <= 0 {
		c.MRPSource = "сохранённая версия"
		c.MRPModelVersion = "сохранённый расчёт"
		c.MRPBaseDays, _ = parseEconomicNumber(v.MRPBase, "МРП базовый", false)
		c.MRPLowDays, _ = parseEconomicNumber(v.MRPLow, "МРП низкий", false)
		c.MRPHighDays, _ = parseEconomicNumber(v.MRPHigh, "МРП высокий", false)
		c.HasLowHigh = c.MRPLowDays > 0 && c.MRPHighDays > 0
	}
	c.Alternatives = technicalAlternatives(c.Complication, originalBaseMethod)
	c.BaseMethod = selectedTechnicalMethod(c.ObjectKey, originalBaseMethod, c.Alternatives)
	c.ScenarioID = fmt.Sprintf("%s|%s|%s", c.ObjectKey, normalizeMethodText(c.BaseMethod), c.MRPModelVersion)
	c.SpecificCosts = methodSpecificCostRequirements(c.BaseMethod)
	c.Readiness, c.ReadinessIssues = technicalScenarioReadiness(c)
	return c, true
}

func economicContractFromTechnical(c TechnicalScenarioContract) EconomicScenarioContract {
	draft := loadScenarioCostDraft(c.ObjectKey, c.BaseMethod)
	codes := make([]string, 0, len(c.SpecificCosts))
	for _, x := range c.SpecificCosts {
		codes = append(codes, x.Code)
	}
	return EconomicScenarioContract{
		TechnicalContractVersion: c.ContractVersion, ObjectKey: c.ObjectKey, ScenarioID: c.ScenarioID,
		GNO: c.GNO, Complication: c.Complication, Method: c.BaseMethod, MRPSource: c.MRPSource, MRPModelVersion: c.MRPModelVersion,
		SpecificCostCodes: codes, SpecificAnnualOPEXRub: draft.AnnualOPEXRub, SpecificOneTimeMTRRub: draft.OneTimeMTRRub,
		SurvivalLowDays: c.SurvivalLowDays, SurvivalBaseDays: c.SurvivalBaseDays, SurvivalHighDays: c.SurvivalHighDays, HasSurvivalTriplet: c.HasSurvivalTriplet,
		SurvivalSource: c.SurvivalSource, SurvivalModelVersion: c.SurvivalModelVersion, SurvivalEventCount: c.SurvivalEventCount, SurvivalCensoredCount: c.SurvivalCensoredCount, CurrentCycleAgeDays: c.CurrentCycleAgeDays, SurvivalBacktestStatus: c.SurvivalBacktestStatus,
		SurvivalTarget: c.SurvivalTarget, SurvivalDistinctObjects: c.SurvivalDistinctObjects, SurvivalClassificationCoverage: c.SurvivalClassificationCoverage, SurvivalConfirmedFailures: c.SurvivalConfirmedFailures, SurvivalBrier90: c.SurvivalBrier90, SurvivalBrier180: c.SurvivalBrier180, SurvivalBrier365: c.SurvivalBrier365, SurvivalCompetingUnknown: c.SurvivalCompetingUnknown,
	}
}

func validateEconomicScenarioContract(c *EconomicScenarioContract) []string {
	if c == nil {
		return nil
	}
	var errs []string
	if strings.TrimSpace(c.ObjectKey) == "" {
		errs = append(errs, "в экономическом сценарии отсутствует ключ объекта")
	}
	if strings.TrimSpace(c.ScenarioID) == "" {
		errs = append(errs, "в экономическом сценарии отсутствует идентификатор технического сценария")
	}
	if strings.TrimSpace(c.TechnicalContractVersion) == "" {
		errs = append(errs, "не указана версия технического контракта")
	}
	return errs
}

type TechnicalScenarioComparisonRow struct {
	Method        string                    `json:"method"`
	Status        string                    `json:"status"`
	Selected      bool                      `json:"selected"`
	SpecificCosts []SpecificCostRequirement `json:"specific_cost_requirements,omitempty"`
	Draft         ScenarioCostDraft         `json:"draft"`
}

func technicalScenarioComparison(c TechnicalScenarioContract) []TechnicalScenarioComparisonRow {
	rows := []TechnicalScenarioComparisonRow{{Method: c.BaseMethod, Status: "выбранный сценарий", Selected: true, SpecificCosts: methodSpecificCostRequirements(c.BaseMethod), Draft: loadScenarioCostDraft(c.ObjectKey, c.BaseMethod)}}
	seen := map[string]bool{normalizeMethodText(c.BaseMethod): true}
	add := func(method, status string) {
		method = strings.TrimSpace(method)
		key := normalizeMethodText(method)
		if method == "" || seen[key] {
			return
		}
		seen[key] = true
		rows = append(rows, TechnicalScenarioComparisonRow{Method: method, Status: status, SpecificCosts: methodSpecificCostRequirements(method), Draft: loadScenarioCostDraft(c.ObjectKey, method)})
	}
	add(c.CanonicalMethod, "исходный метод последней сохранённой оценки")
	for _, a := range c.Alternatives {
		add(a.Method, a.Status)
	}
	return rows
}

func technicalScenarioComparisonText(c TechnicalScenarioContract) string {
	rows := technicalScenarioComparison(c)
	if len(rows) == 0 {
		return "Сценарии для сравнения не сформированы."
	}
	var b strings.Builder
	b.WriteString("Сравнение сценариев мероприятий — без рейтинга и автоматического выбора победителя:\r\n")
	for _, r := range rows {
		mark := "  "
		if r.Selected {
			mark = "→ "
		}
		cost := "специфические затраты не требуются"
		if len(r.SpecificCosts) > 0 {
			names := make([]string, 0, len(r.SpecificCosts))
			for _, x := range r.SpecificCosts {
				names = append(names, x.Code)
			}
			cost = strings.Join(names, ", ")
		}
		fmt.Fprintf(&b, "%s%s | %s | %s | эксплуатационные затраты (OPEX) %.0f руб./год | разово %.0f руб.\r\n", mark, r.Method, r.Status, cost, r.Draft.AnnualOPEXRub, r.Draft.OneTimeMTRRub)
	}
	return strings.TrimSpace(b.String())
}

func technicalScenarioSummary(c TechnicalScenarioContract) string {
	alt := "нет"
	if len(c.Alternatives) > 0 {
		names := make([]string, 0, len(c.Alternatives))
		for _, a := range c.Alternatives {
			names = append(names, a.Method+" ["+a.Status+"]")
		}
		alt = strings.Join(names, "; ")
	}
	costs := "специфические затраты не требуются"
	if len(c.SpecificCosts) > 0 {
		names := make([]string, 0, len(c.SpecificCosts))
		for _, x := range c.SpecificCosts {
			names = append(names, x.Name)
		}
		costs = strings.Join(names, "; ")
	}
	issues := "нет"
	if len(c.ReadinessIssues) > 0 {
		issues = strings.Join(c.ReadinessIssues, "; ")
	}
	surv := "набор вероятностных оценок МРП для экономики не подтверждён"
	if c.HasSurvivalTriplet {
		surv = fmt.Sprintf("распределение МРП P25/P50/P75 %.0f/%.0f/%.0f сут.; %s; %s; цель: %s; подтверждение причин %.0f%%; оценка Брайера на 180 сут. %.3f", c.SurvivalLowDays, c.SurvivalBaseDays, c.SurvivalHighDays, dash(c.SurvivalSource), dash(c.SurvivalBacktestStatus), dash(c.SurvivalTarget), c.SurvivalClassificationCoverage*100, c.SurvivalBrier180)
	}
	return fmt.Sprintf("Готовность перехода МРП → мероприятие → экономика: %s.\r\nСценарий мероприятия: %s. Фактическое ГНО: %s. Осложнение: %s. Базовый метод: %s.\r\nИсточник МРП: %s / %s. %s.\r\nАльтернативы без ранжирования: %s.\r\nСпецифические затраты: %s.\r\nЧто требуется уточнить: %s.", c.Readiness, c.ScenarioID, dash(c.GNO), dash(c.Complication), dash(c.BaseMethod), dash(c.MRPSource), mrpUserModelName(c.MRPModelVersion), surv, alt, costs, issues)
}
