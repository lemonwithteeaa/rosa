//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

const baseEconomicsFormulaVersion = "ECON-BASE-EXCEL-2026-09"

type BaseEconomicsInput struct {
	TechnicalScenario  *EconomicScenarioContract `json:"technical_scenario,omitempty"`
	CurrentMRPDays     float64                   `json:"current_mrp_days"`
	MRPLowDays         float64                   `json:"mrp_low_days,omitempty"`
	MRPBaseDays        float64                   `json:"mrp_base_days"`
	MRPHighDays        float64                   `json:"mrp_high_days,omitempty"`
	TargetMRPDays      float64                   `json:"target_mrp_days,omitempty"`
	TargetSource       string                    `json:"target_source,omitempty"`
	TargetNearPct      float64                   `json:"target_near_pct,omitempty"`
	SelectedScenario   string                    `json:"selected_scenario"`
	HorizonMonths      int                       `json:"horizon_months"`
	RepairDurationDays float64                   `json:"repair_duration_days"`
	RepairCostK        float64                   `json:"repair_cost_thousand_rub"`
	AnnualOPEXK        float64                   `json:"annual_opex_thousand_rub"`
	OPEXInflationPct   float64                   `json:"opex_inflation_pct"`
	OneTimeMTRK        float64                   `json:"one_time_mtr_thousand_rub"`
}

type BaseEconomicsScenario struct {
	Name                       string  `json:"name"`
	MRPDays                    float64 `json:"mrp_days"`
	RepairsPerYearBase         float64 `json:"repairs_per_year_base"`
	RepairsPerYearScenario     float64 `json:"repairs_per_year_scenario"`
	PreventedRepairsPerYear    float64 `json:"prevented_repairs_per_year"`
	GrossBenefitPerYearK       float64 `json:"gross_benefit_per_year_thousand_rub"`
	OPEXForHorizonK            float64 `json:"opex_for_horizon_thousand_rub"`
	OneTimeMTRK                float64 `json:"one_time_mtr_thousand_rub"`
	NetBenefitForHorizonK      float64 `json:"net_benefit_for_horizon_thousand_rub"`
	BenefitCostRatio           float64 `json:"benefit_cost_ratio,omitempty"`
	PreventedDowntimeDaysPerYr float64 `json:"prevented_downtime_days_per_year"`
}

type BaseEconomicsResult struct {
	FormulaVersion   string                  `json:"formula_version"`
	Status           string                  `json:"status"`
	Errors           []string                `json:"errors,omitempty"`
	Scenarios        []BaseEconomicsScenario `json:"scenarios"`
	Selected         BaseEconomicsScenario   `json:"selected"`
	PaybackMonths    *float64                `json:"payback_months,omitempty"`
	BreakEvenMRPDays *float64                `json:"break_even_mrp_days,omitempty"`
	TargetMRPDays    float64                 `json:"target_mrp_days,omitempty"`
	TargetGapDays    float64                 `json:"target_gap_days,omitempty"`
	TargetRatioPct   float64                 `json:"target_ratio_pct,omitempty"`
	TargetStatus     string                  `json:"target_status,omitempty"`
	TargetSource     string                  `json:"target_source,omitempty"`
	TargetNearPct    float64                 `json:"target_near_pct,omitempty"`
}

func normalizeScenarioName(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "низкий", "low", "осторожный":
		return "Осторожный"
	case "высокий", "high", "благоприятный":
		return "Благоприятный"
	case "базовый", "base", "центральный", "":
		return "Центральный"
	default:
		return strings.TrimSpace(s)
	}
}

func validateBaseEconomicsInput(in BaseEconomicsInput) []string {
	var errs []string
	errs = append(errs, validateEconomicScenarioContract(in.TechnicalScenario)...)
	for _, value := range []float64{in.CurrentMRPDays, in.MRPLowDays, in.MRPBaseDays,
		in.MRPHighDays, in.RepairDurationDays, in.RepairCostK, in.AnnualOPEXK,
		in.OPEXInflationPct, in.OneTimeMTRK} {
		if !finiteNumber(value) {
			return []string{"исходные показатели должны содержать только конечные числа"}
		}
	}
	switch strings.ToLower(strings.TrimSpace(in.SelectedScenario)) {
	case "низкий", "low", "осторожный", "базовый", "base", "центральный", "высокий", "high", "благоприятный", "":
	default:
		errs = append(errs, "сценарий должен быть указан как «Осторожный», «Центральный» или «Благоприятный»")
	}
	if in.CurrentMRPDays <= 0 {
		errs = append(errs, "не заполнен текущий МРП")
	}
	if in.MRPBaseDays <= 0 {
		errs = append(errs, "не заполнен базовый прогноз МРП")
	}
	if in.RepairCostK <= 0 {
		errs = append(errs, "не заполнена стоимость ремонта")
	}
	if in.HorizonMonths < 1 || in.HorizonMonths > 60 {
		errs = append(errs, "горизонт должен быть от 1 до 60 месяцев")
	}
	selected := normalizeScenarioName(in.SelectedScenario)
	if selected == "Осторожный" && in.MRPLowDays <= 0 {
		errs = append(errs, "для выбранного осторожного сценария не заполнен МРП")
	}
	if selected == "Благоприятный" && in.MRPHighDays <= 0 {
		errs = append(errs, "для выбранного благоприятного сценария не заполнен МРП")
	}
	if in.AnnualOPEXK < 0 || in.OneTimeMTRK < 0 || in.RepairDurationDays < 0 {
		errs = append(errs, "затраты и длительность ремонта не могут быть отрицательными")
	}
	if in.OPEXInflationPct <= -100 {
		errs = append(errs, "инфляция OPEX должна быть больше -100%")
	}
	return errs
}

// opexForHorizon reproduces the monthly Excel logic: annual OPEX is divided by
// 12, and each completed 12-month block is multiplied by (1+inflation)^year.
func opexForHorizon(annualK, inflationPct float64, months int) float64 {
	if annualK <= 0 || months <= 0 {
		return 0
	}
	r := inflationPct / 100.0
	total := 0.0
	for month := 1; month <= months; month++ {
		yearIndex := (month - 1) / 12
		total += annualK / 12.0 * math.Pow(1+r, float64(yearIndex))
	}
	return total
}

func calculateBaseScenario(in BaseEconomicsInput, name string, mrp float64, horizonOPEX float64) BaseEconomicsScenario {
	s := BaseEconomicsScenario{Name: name, MRPDays: mrp, OPEXForHorizonK: horizonOPEX, OneTimeMTRK: in.OneTimeMTRK}
	if in.CurrentMRPDays <= 0 || mrp <= 0 {
		return s
	}
	s.RepairsPerYearBase = 365.0 / in.CurrentMRPDays
	s.RepairsPerYearScenario = 365.0 / mrp
	s.PreventedRepairsPerYear = math.Max(0, s.RepairsPerYearBase-s.RepairsPerYearScenario)
	s.GrossBenefitPerYearK = s.PreventedRepairsPerYear * in.RepairCostK
	s.NetBenefitForHorizonK = s.GrossBenefitPerYearK*float64(in.HorizonMonths)/12.0 - horizonOPEX - in.OneTimeMTRK
	denom := horizonOPEX + in.OneTimeMTRK
	if denom > 0 {
		s.BenefitCostRatio = (s.GrossBenefitPerYearK * float64(in.HorizonMonths) / 12.0) / denom
	}
	s.PreventedDowntimeDaysPerYr = s.PreventedRepairsPerYear * in.RepairDurationDays
	return s
}

func CalculateBaseEconomics(in BaseEconomicsInput) BaseEconomicsResult {
	out := BaseEconomicsResult{FormulaVersion: baseEconomicsFormulaVersion, TargetMRPDays: normalizedTargetRunlifeDays(in.TargetMRPDays), TargetSource: strings.TrimSpace(in.TargetSource), TargetNearPct: normalizedTargetNearPct(in.TargetNearPct)}
	out.Errors = validateBaseEconomicsInput(in)
	if len(out.Errors) > 0 {
		out.Status = "FAIL — ПРОВЕРИТЬ ИСХОДНЫЕ ДАННЫЕ"
		return out
	}
	opex := opexForHorizon(in.AnnualOPEXK, in.OPEXInflationPct, in.HorizonMonths)
	scenarios := []struct {
		name string
		mrp  float64
	}{
		{"Осторожный", in.MRPLowDays},
		{"Центральный", in.MRPBaseDays},
		{"Благоприятный", in.MRPHighDays},
	}
	for _, sc := range scenarios {
		out.Scenarios = append(out.Scenarios, calculateBaseScenario(in, sc.name, sc.mrp, opex))
	}
	selectedName := normalizeScenarioName(in.SelectedScenario)
	for _, sc := range out.Scenarios {
		if sc.Name == selectedName {
			out.Selected = sc
			break
		}
	}
	out.TargetGapDays, out.TargetRatioPct, out.TargetStatus = targetRunlifeComparisonWithTolerance(out.Selected.MRPDays, out.TargetMRPDays, out.TargetNearPct)

	// Excel B16: one-time MTR divided by the annual gross benefit less annual OPEX.
	annualNetBeforeMTR := out.Selected.GrossBenefitPerYearK - in.AnnualOPEXK
	if annualNetBeforeMTR > 0 {
		p := 0.0
		if in.OneTimeMTRK > 0 {
			p = in.OneTimeMTRK / annualNetBeforeMTR * 12.0
		}
		out.PaybackMonths = &p
	}

	// Excel B17: MRP where prevented-repair benefit equals OPEX + annualized MTR.
	denominator := 365.0/in.CurrentMRPDays - (in.AnnualOPEXK+in.OneTimeMTRK*12.0/float64(in.HorizonMonths))/in.RepairCostK
	if denominator > 0 {
		b := 365.0 / denominator
		if !math.IsInf(b, 0) && !math.IsNaN(b) && b > 0 {
			out.BreakEvenMRPDays = &b
		}
	}
	out.Status = "PASS"
	return out
}

func baseEconomicsSummary(r BaseEconomicsResult) string {
	if r.Status != "PASS" {
		return economicsStatusRu(r.Status) + ": " + strings.Join(r.Errors, "; ")
	}
	payback := "не окупается"
	if r.PaybackMonths != nil {
		payback = fmt.Sprintf("%.1f мес.", *r.PaybackMonths)
	}
	breakEven := "не определяется"
	if r.BreakEvenMRPDays != nil {
		breakEven = fmt.Sprintf("%.0f сут.", *r.BreakEvenMRPDays)
	}
	return fmt.Sprintf("%s: МРП %.0f сут.; цель %.0f сут. — %s (%.0f%%, %+.0f сут.; источник: %s); предотвращено %.2f ремонта/год; чистая выгода за горизонт %.1f тыс. руб.; выгода/затраты %.2f; срок возврата %s; граничный МРП %s", r.Selected.Name, r.Selected.MRPDays, r.TargetMRPDays, r.TargetStatus, r.TargetRatioPct, r.TargetGapDays, dash(r.TargetSource), r.Selected.PreventedRepairsPerYear, r.Selected.NetBenefitForHorizonK, r.Selected.BenefitCostRatio, payback, breakEven)
}

// baseEconomicsScenarioSensitivityText makes all three already-calculated MRP
// outcomes visible in the UI. It only formats results; calculation and storage
// continue to use the selected forecast as before.
func baseEconomicsScenarioSensitivityText(scenarios []BaseEconomicsScenario) string {
	if len(scenarios) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Эффект за горизонт по прогнозам МРП:\r\n")
	for _, scenario := range scenarios {
		name := strings.ToLower(strings.TrimSpace(scenario.Name))
		if name == "" {
			name = "сценарий"
		}
		if scenario.MRPDays <= 0 || !finiteNumber(scenario.MRPDays) {
			fmt.Fprintf(&b, "• %s: прогноз МРП не задан; эффект не рассчитывается.\r\n", name)
			continue
		}
		fmt.Fprintf(&b, "• %s: МРП %.0f сут.; чистая выгода %+.1f тыс. руб.\r\n", name, scenario.MRPDays, scenario.NetBenefitForHorizonK)
	}
	return strings.TrimRight(b.String(), "\r\n")
}

func selfCheckBaseEconomics() error {
	in := BaseEconomicsInput{
		CurrentMRPDays: 1000, MRPBaseDays: 1500, MRPLowDays: 1200, MRPHighDays: 1800,
		SelectedScenario: "Базовый", HorizonMonths: 24, RepairDurationDays: 5,
		RepairCostK: 1000, AnnualOPEXK: 100, OPEXInflationPct: 5, OneTimeMTRK: 50,
	}
	r := CalculateBaseEconomics(in)
	if r.Status != "PASS" || math.Abs(r.Selected.PreventedRepairsPerYear-0.1216666667) > 1e-6 || math.Abs(r.Selected.OPEXForHorizonK-205.0) > 1e-6 {
		return fmt.Errorf("несовпадение контрольного расчёта: %+v", r)
	}
	return nil
}
