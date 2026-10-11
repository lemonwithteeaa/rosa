//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

// rho in kg/m3, solute mass in g/L; solvent water mass in kg per L solution.
func laboratoryWaterMassPerLiter(v map[string]string) (float64, error) {
	rho, ok := parseLabNumber(v["brine_density"])
	if !ok || rho < 500 || rho > 2000 {
		return 0, fmt.Errorf("Для моляльного расчёта нужна плотность водного раствора 500–2000 кг/м³ при температуре пробы")
	}
	solutes, ok := parseLabNumber(v["solute_mass"])
	if !ok || solutes < 0 {
		return 0, fmt.Errorf("Для моляльного расчёта нужна полная масса растворённых веществ в г/л; минерализация молча не подставляется")
	}
	water := (rho - solutes) / 1000
	if water <= 0 {
		return 0, fmt.Errorf("Для моляльного расчёта масса растворённых веществ не может быть равна или больше массы раствора в одном литре")
	}
	// Lower bound only: elemental C omits associated O/H; no double counting of free/total analyses.
	lower := 0.0
	for _, key := range []string{"ca_total", "mg_total", "tic", "so4_total", "na", "k", "cl"} {
		x, ok := parseLabNumber(v[key])
		if !ok || x < 0 {
			return 0, fmt.Errorf("Для моляльного расчёта некорректен обязательный анализ %s", key)
		}
		lower += x / 1000
	}
	for _, key := range []string{"ba", "sr"} {
		if strings.TrimSpace(v[key]) == "" {
			continue
		}
		x, ok := parseLabNumber(v[key])
		if !ok || x < 0 {
			return 0, fmt.Errorf("Для моляльного расчёта некорректен анализ %s", key)
		}
		lower += x / 1000
	}
	if solutes+1e-9 < lower {
		return 0, fmt.Errorf("Для моляльного расчёта полная масса растворённых веществ %.6g г/л ниже суммы обязательных анализов %.6g г/л; сверить единицы и данные", solutes, lower)
	}
	return water, nil
}
func laboratoryMolalityValues(v map[string]string) (map[string]string, float64, error) {
	water, e := laboratoryWaterMassPerLiter(v)
	if e != nil {
		return nil, 0, e
	}
	out := map[string]string{}
	for k, x := range v {
		out[k] = x
	}
	for _, key := range []string{"ca_total", "mg_total", "tic", "so4_total", "na", "k", "cl", "ba", "sr", "hco3", "alkalinity"} {
		if strings.TrimSpace(v[key]) != "" {
			x, ok := parseLabNumber(v[key])
			if !ok || x < 0 {
				return nil, 0, fmt.Errorf("Для моляльного расчёта некорректен анализ %s", key)
			}
			x /= water
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, 0, fmt.Errorf("Для моляльного расчёта произошло переполнение")
			}
			out[key] = fmt.Sprintf("%.17g", x)
		}
	}
	return out, water, nil
}
func laboratoryMolalityText(v map[string]string) string {
	normalized, water, e := laboratoryMolalityValues(v)
	if e != nil {
		return e.Error()
	}
	s, e := laboratoryMultiComplexFromValuesMolal(normalized)
	if e != nil {
		return strings.ReplaceAll(e.Error(), "моль/л", "моль/кг воды")
	}
	return fmt.Sprintf("Пересчёт: масса воды %.8g кг в 1 л раствора; I %.8g моль/кг воды; a(H₂O) %.9g; итераций %d. Свободные ионы после обратного пересчёта: Ca²⁺ %.6g; Mg²⁺ %.6g; Ba²⁺ %.6g; Sr²⁺ %.6g; SO₄²⁻ %.6g мг/л. Комплексы, моль/кг воды: CaCO₃ %.6g; CaHCO₃⁺ %.6g; CaSO₄ %.6g; CaOH⁺ %.6g; MgCO₃ %.6g; MgHCO₃⁺ %.6g; MgSO₄ %.6g; Mg(SO₄)₂²⁻ %.6g; MgOH⁺ %.6g; BaSO₄ %.6g; BaCO₃ %.6g; BaHCO₃⁺ %.6g; BaOH⁺ %.6g; SrSO₄ %.6g; SrCO₃ %.6g; SrHCO₃⁺ %.6g; SrOH⁺ %.6g. Щёлочность %.6g мг/л как CaCO₃. Общий C используется как вход баланса; исходные значения сохранены.", water, s.I, s.WaterActivity, s.Iterations, s.Ca*40078*water, s.Mg*24305*water, s.Ba*137327*water, s.Sr*87620*water, s.SO4*96060*water, s.CaCO3, s.CaHCO3, s.CaSO4, s.CaOH, s.MgCO3, s.MgHCO3, s.MgSO4, s.MgSO42, s.MgOH, s.BaSO4, s.BaCO3, s.BaHCO3, s.BaOH, s.SrSO4, s.SrCO3, s.SrHCO3, s.SrOH, s.Forms.Alkalinity*50043.45*water)
}
func laboratoryMolalityResults(v map[string]string) ([]laboratorySaturationResult, float64, []string) {
	normalized, water, e := laboratoryMolalityValues(v)
	if e != nil {
		return nil, 0, []string{e.Error()}
	}
	r, i, w := laboratoryMultiComplexResults(normalized)
	if len(r) == 0 {
		for j := range w {
			w[j] = strings.ReplaceAll(w[j], "моль/л", "моль/кг воды")
		}
		return r, i, w
	}
	// Old route emits scaled comparison numbers; rebuild this comparison in original mg/L.
	out := []string{"Моляльное сокращённое равновесие при 25 °C: 17 комплексов, Дэвис. " + laboratoryMolalWaterActivityBasis + " Полнота/метод определения массы растворённых веществ требуют проверки; это не полный PHREEQC/Питцер."}
	for _, x := range w[1:] {
		if !strings.HasPrefix(x, "Щёлочность:") {
			out = append(out, x)
		}
	}
	s, _ := laboratoryMultiComplexFromValuesMolal(normalized)
	for j := range r {
		switch r[j].Mineral {
		case "Кальцит":
			r[j].SI = s.CalciteSI
		case "Гипс":
			r[j].SI = s.GypsumSI
		case "Барит":
			r[j].SI = s.BariteSI
		case "Целестин":
			r[j].SI = s.CelestiteSI
		}
		r[j].Ratio = math.Pow(10, r[j].SI)
	}
	i = s.I
	if measured, ok := parseLabNumber(v["alkalinity"]); ok && measured >= 0 {
		calc := s.Forms.Alkalinity * 50043.45 * water
		den := math.Max(math.Abs(calc), math.Abs(measured))
		if den > 0 && math.Abs(calc-measured)/den > .1 {
			out = append(out, fmt.Sprintf("Щёлочность: расчёт %.6g, анализ %.6g мг/л как CaCO₃; разность выше рабочего уровня 10 %%, не нормативного.", calc, measured))
		}
	}
	out = append(out, fmt.Sprintf("Расчётная a(H₂O)=%.9g. Предел I 0,1 моль/кг воды; давление и газовая фаза не рассчитаны. Неучтённые вещества могут менять I, a(H₂O) и равновесие. Сухой остаток не всегда равен полной массе растворённых веществ.", s.WaterActivity))
	return r, i, out
}
