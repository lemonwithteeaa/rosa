//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const extendedEconomicsFormulaVersion = "ECON-EXT-MONTHLY-2026-09"

const economicsWarningBanner = "Экономическая оценка является предварительной. Результат зависит от прогнозных предпосылок, включая цену реализации нефти, профиль добычи, налоговый эффект, затраты и продолжительность эффекта мероприятия. Перед использованием результата для окончательного решения проверьте источник и дату критических исходных данных и выполните анализ чувствительности."

// EconomicAssumptionMeta keeps provenance separate from the numeric value.
// MaxAgeDays is profile-controlled: zero means that no automatic age gate is
// applied, but source/date completeness is still checked for critical inputs.
type EconomicAssumptionMeta struct {
	Name            string `json:"name"`
	Source          string `json:"source,omitempty"`
	AsOf            string `json:"as_of,omitempty"`
	ScenarioVersion string `json:"scenario_version,omitempty"`
	Critical        bool   `json:"critical"`
	MaxAgeDays      int    `json:"max_age_days,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

// ExtendedEconomicsMonth is an incremental monthly cash-flow record for a
// concrete intervention. It is deliberately not a full well-life model.
// Taxes are accepted as approved monthly effects rather than reconstructed
// from public tax parameters at single-well level.
type ExtendedEconomicsMonth struct {
	Month                     string  `json:"month"`
	IncrementalOilTons        float64 `json:"incremental_oil_tons"`
	RealizationPriceRubPerTon float64 `json:"realization_price_rub_per_ton"`
	ReferenceOilPriceUSDBbl   float64 `json:"reference_oil_price_usd_bbl,omitempty"`
	FXRubPerUSD               float64 `json:"fx_rub_per_usd,omitempty"`
	AvoidedRepairCostK        float64 `json:"avoided_repair_cost_thousand_rub,omitempty"`
	AvoidedDowntimeBenefitK   float64 `json:"avoided_downtime_benefit_thousand_rub,omitempty"`
	OPEXK                     float64 `json:"opex_thousand_rub,omitempty"`
	CAPEXK                    float64 `json:"capex_thousand_rub,omitempty"`
	ApprovedExtractionTaxK    float64 `json:"approved_extraction_tax_effect_thousand_rub,omitempty"`
	ProfitTaxK                float64 `json:"profit_tax_thousand_rub,omitempty"`
	PropertyTaxK              float64 `json:"property_tax_thousand_rub,omitempty"`
	DepreciationK             float64 `json:"depreciation_thousand_rub,omitempty"`
	OtherCashFlowK            float64 `json:"other_cash_flow_thousand_rub,omitempty"`
}

type ExtendedEconomicsInput struct {
	TechnicalScenario             *EconomicScenarioContract `json:"technical_scenario,omitempty"`
	HorizonMonths                 int                       `json:"horizon_months"`
	DiscountRatePct               float64                   `json:"discount_rate_pct"`
	Months                        []ExtendedEconomicsMonth  `json:"months"`
	Assumptions                   []EconomicAssumptionMeta  `json:"assumptions,omitempty"`
	OrganizationProfile           string                    `json:"organization_profile,omitempty"`
	IncludeTechnicalScenarioCosts bool                      `json:"include_technical_scenario_costs,omitempty"`
}

type ExtendedEconomicsMonthResult struct {
	Month              string  `json:"month"`
	RevenueEffectK     float64 `json:"revenue_effect_thousand_rub"`
	AvoidedCostEffectK float64 `json:"avoided_cost_effect_thousand_rub"`
	CashOutflowK       float64 `json:"cash_outflow_thousand_rub"`
	UndiscountedFCFK   float64 `json:"undiscounted_fcf_thousand_rub"`
	DiscountFactor     float64 `json:"discount_factor"`
	DiscountedFCFK     float64 `json:"discounted_fcf_thousand_rub"`
}

type EconomicSensitivity struct {
	Parameter   string  `json:"parameter"`
	Delta       string  `json:"delta"`
	LowNPVK     float64 `json:"low_npv_thousand_rub"`
	BaseNPVK    float64 `json:"base_npv_thousand_rub"`
	HighNPVK    float64 `json:"high_npv_thousand_rub"`
	ImpactK     float64 `json:"impact_thousand_rub"`
	Criticality string  `json:"criticality"`
}

type ExtendedEconomicsResult struct {
	FormulaVersion   string                         `json:"formula_version"`
	Status           string                         `json:"status"`
	WarningBanner    string                         `json:"warning_banner"`
	Errors           []string                       `json:"errors,omitempty"`
	Warnings         []string                       `json:"warnings,omitempty"`
	Monthly          []ExtendedEconomicsMonthResult `json:"monthly"`
	UndiscountedFCFK float64                        `json:"undiscounted_fcf_thousand_rub"`
	NPVK             float64                        `json:"npv_thousand_rub"`
	Sensitivity      []EconomicSensitivity          `json:"sensitivity,omitempty"`
}

func validateExtendedEconomicsInput(in ExtendedEconomicsInput) ([]string, []string) {
	var errs, warns []string
	errs = append(errs, validateEconomicScenarioContract(in.TechnicalScenario)...)
	if in.HorizonMonths < 1 || in.HorizonMonths > 60 {
		errs = append(errs, "горизонт расширенной оценки должен быть от 1 до 60 месяцев")
	}
	if in.DiscountRatePct <= -100 {
		errs = append(errs, "ставка дисконтирования должна быть больше -100%")
	}
	if len(in.Months) < in.HorizonMonths {
		errs = append(errs, fmt.Sprintf("заполнено %d мес. из требуемых %d", len(in.Months), in.HorizonMonths))
	}
	n := in.HorizonMonths
	if n > len(in.Months) {
		n = len(in.Months)
	}
	for i := 0; i < n; i++ {
		m := in.Months[i]
		if strings.TrimSpace(m.Month) == "" {
			errs = append(errs, fmt.Sprintf("месяц %d: отсутствует обозначение периода", i+1))
		}
		if m.IncrementalOilTons != 0 && m.RealizationPriceRubPerTon <= 0 {
			errs = append(errs, fmt.Sprintf("%s: есть изменение добычи, но не задана подтверждённая цена реализации, руб./т", monthLabel(m, i)))
		}
		if m.RealizationPriceRubPerTon < 0 || m.OPEXK < 0 || m.CAPEXK < 0 || m.AvoidedRepairCostK < 0 || m.AvoidedDowntimeBenefitK < 0 || m.ProfitTaxK < 0 || m.PropertyTaxK < 0 || m.DepreciationK < 0 {
			errs = append(errs, fmt.Sprintf("%s: затраты, налоги, цена и предотвращённые расходы не могут быть отрицательными", monthLabel(m, i)))
		}
		if m.ReferenceOilPriceUSDBbl > 0 && m.FXRubPerUSD <= 0 {
			warns = append(warns, fmt.Sprintf("%s: указана справочная цена нефти в долл. США/барр. (USD/bbl) без курса; справочный ряд не может быть сопоставлен в рублях", monthLabel(m, i)))
		}
	}

	for _, a := range in.Assumptions {
		if !a.Critical {
			continue
		}
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = "критический показатель"
		}
		if strings.TrimSpace(a.Source) == "" {
			warns = append(warns, name+": не указан источник")
		}
		if strings.TrimSpace(a.AsOf) == "" {
			warns = append(warns, name+": не указана дата актуальности")
			continue
		}
		if a.MaxAgeDays > 0 {
			if t, ok := parseEconomicsDate(a.AsOf); ok {
				age := int(time.Since(t).Hours() / 24)
				if age > a.MaxAgeDays {
					warns = append(warns, fmt.Sprintf("%s: исходные данные старше допустимого срока профиля (%d > %d сут.)", name, age, a.MaxAgeDays))
				}
			} else {
				warns = append(warns, name+": дата актуальности не распознана")
			}
		}
	}
	return uniqueStrings(errs), uniqueStrings(warns)
}

func monthLabel(m ExtendedEconomicsMonth, i int) string {
	if s := strings.TrimSpace(m.Month); s != "" {
		return s
	}
	return fmt.Sprintf("месяц %d", i+1)
}

func parseEconomicsDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	layouts := []string{"2006-01-02", "02.01.2006", time.RFC3339, "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func calculateExtendedNoSensitivity(in ExtendedEconomicsInput) ExtendedEconomicsResult {
	out := ExtendedEconomicsResult{FormulaVersion: extendedEconomicsFormulaVersion, WarningBanner: economicsWarningBanner}
	out.Errors, out.Warnings = validateExtendedEconomicsInput(in)
	if len(out.Errors) > 0 {
		out.Status = "FAIL — ПРОВЕРИТЬ ИСХОДНЫЕ ДАННЫЕ"
		return out
	}
	r := in.DiscountRatePct / 100.0
	for i := 0; i < in.HorizonMonths; i++ {
		m := in.Months[i]
		revenueK := m.IncrementalOilTons * m.RealizationPriceRubPerTon / 1000.0
		avoidedK := m.AvoidedRepairCostK + m.AvoidedDowntimeBenefitK
		// Depreciation is a non-cash charge and is therefore not subtracted from FCF here.
		// Any tax effect of depreciation must be reflected in the approved tax rows.
		cashOutK := m.OPEXK + m.CAPEXK + m.ApprovedExtractionTaxK + m.ProfitTaxK + m.PropertyTaxK
		fcf := revenueK + avoidedK + m.OtherCashFlowK - cashOutK
		yearsFromStart := float64(i) / 12.0
		factor := math.Pow(1.0+r, yearsFromStart)
		discounted := fcf
		if factor != 0 {
			discounted = fcf / factor
		}
		out.Monthly = append(out.Monthly, ExtendedEconomicsMonthResult{
			Month: monthLabel(m, i), RevenueEffectK: revenueK, AvoidedCostEffectK: avoidedK,
			CashOutflowK: cashOutK, UndiscountedFCFK: fcf, DiscountFactor: factor, DiscountedFCFK: discounted,
		})
		out.UndiscountedFCFK += fcf
		out.NPVK += discounted
	}
	if len(out.Warnings) > 0 {
		out.Status = "ПРЕДВАРИТЕЛЬНЫЙ — АКТУАЛИЗИРОВАТЬ КРИТИЧЕСКИЕ ПРЕДПОСЫЛКИ"
	} else {
		out.Status = "PASS"
	}
	return out
}

func scaleExtended(in ExtendedEconomicsInput, parameter string, factor float64) ExtendedEconomicsInput {
	out := in
	out.Months = append([]ExtendedEconomicsMonth(nil), in.Months...)
	switch parameter {
	case "Цена реализации нефти":
		for i := range out.Months {
			out.Months[i].RealizationPriceRubPerTon *= factor
		}
	case "Дополнительная / сохранённая добыча":
		for i := range out.Months {
			out.Months[i].IncrementalOilTons *= factor
		}
	case "OPEX":
		for i := range out.Months {
			out.Months[i].OPEXK *= factor
		}
	case "CAPEX":
		for i := range out.Months {
			out.Months[i].CAPEXK *= factor
		}
	case "Предотвращённые ремонты и простой":
		for i := range out.Months {
			out.Months[i].AvoidedRepairCostK *= factor
			out.Months[i].AvoidedDowntimeBenefitK *= factor
		}
	}
	return out
}

func calculateExtendedSensitivity(in ExtendedEconomicsInput, baseNPV float64) []EconomicSensitivity {
	params := []string{"Цена реализации нефти", "Дополнительная / сохранённая добыча", "OPEX", "CAPEX", "Предотвращённые ремонты и простой"}
	out := make([]EconomicSensitivity, 0, len(params)+1)
	for _, p := range params {
		low := calculateExtendedNoSensitivity(scaleExtended(in, p, 0.90))
		high := calculateExtendedNoSensitivity(scaleExtended(in, p, 1.10))
		if len(low.Errors) > 0 || len(high.Errors) > 0 {
			continue
		}
		impact := math.Max(math.Abs(low.NPVK-baseNPV), math.Abs(high.NPVK-baseNPV))
		out = append(out, EconomicSensitivity{Parameter: p, Delta: "±10%", LowNPVK: low.NPVK, BaseNPVK: baseNPV, HighNPVK: high.NPVK, ImpactK: impact})
	}
	// Discount-rate sensitivity is expressed as ±2 percentage points, not ±10%.
	lowIn, highIn := in, in
	lowIn.DiscountRatePct = math.Max(-99.0, in.DiscountRatePct-2.0)
	highIn.DiscountRatePct = in.DiscountRatePct + 2.0
	lowR, highR := calculateExtendedNoSensitivity(lowIn), calculateExtendedNoSensitivity(highIn)
	if len(lowR.Errors) == 0 && len(highR.Errors) == 0 {
		impact := math.Max(math.Abs(lowR.NPVK-baseNPV), math.Abs(highR.NPVK-baseNPV))
		out = append(out, EconomicSensitivity{Parameter: "Ставка дисконтирования", Delta: "±2 п.п.", LowNPVK: lowR.NPVK, BaseNPVK: baseNPV, HighNPVK: highR.NPVK, ImpactK: impact})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ImpactK > out[j].ImpactK })
	if len(out) > 0 {
		maxImpact := out[0].ImpactK
		for i := range out {
			switch {
			case maxImpact <= 0:
				out[i].Criticality = "низкая"
			case out[i].ImpactK >= maxImpact*0.67:
				out[i].Criticality = "критическая"
			case out[i].ImpactK >= maxImpact*0.33:
				out[i].Criticality = "существенная"
			default:
				out[i].Criticality = "умеренная"
			}
		}
	}
	return out
}

func applyTechnicalScenarioCostsToExtended(in ExtendedEconomicsInput) ExtendedEconomicsInput {
	if !in.IncludeTechnicalScenarioCosts || in.TechnicalScenario == nil || in.HorizonMonths <= 0 || len(in.Months) == 0 {
		return in
	}
	annualK := in.TechnicalScenario.SpecificAnnualOPEXRub / 1000.0
	oneTimeK := in.TechnicalScenario.SpecificOneTimeMTRRub / 1000.0
	n := in.HorizonMonths
	if n > len(in.Months) {
		n = len(in.Months)
	}
	for i := 0; i < n; i++ {
		if annualK > 0 {
			in.Months[i].OPEXK += annualK / 12.0
		}
	}
	if n > 0 && oneTimeK > 0 {
		in.Months[0].CAPEXK += oneTimeK
	}
	return in
}

func CalculateExtendedEconomics(in ExtendedEconomicsInput) ExtendedEconomicsResult {
	in = applyTechnicalScenarioCostsToExtended(in)
	out := calculateExtendedNoSensitivity(in)
	if len(out.Errors) == 0 {
		out.Sensitivity = calculateExtendedSensitivity(in, out.NPVK)
	}
	return out
}

func selfCheckExtendedEconomics() error {
	in := ExtendedEconomicsInput{
		HorizonMonths:   2,
		DiscountRatePct: 15,
		Months: []ExtendedEconomicsMonth{
			{Month: "2026-10", IncrementalOilTons: 10, RealizationPriceRubPerTon: 30000, AvoidedRepairCostK: 100, OPEXK: 20, CAPEXK: 50},
			{Month: "2026-11", IncrementalOilTons: 8, RealizationPriceRubPerTon: 31000, OPEXK: 20},
		},
		Assumptions: []EconomicAssumptionMeta{{Name: "Цена реализации нефти", Source: "контрольный сценарий", AsOf: "2026-09-15", Critical: true}},
	}
	r := CalculateExtendedEconomics(in)
	if len(r.Errors) != 0 || r.NPVK == 0 || len(r.Sensitivity) == 0 {
		return fmt.Errorf("ошибка контрольного расширенного расчёта: %+v", r)
	}
	return nil
}
