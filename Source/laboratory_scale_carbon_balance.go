//go:build windows

package main

import (
	"fmt"
	"math"
)

func laboratoryCarbonMeasuredChecks(v map[string]string, f laboratoryCarbonForms) []string {
	var w []string
	for _, c := range []struct {
		key, label, unit string
		calculated       float64
	}{{"tic", "Общий неорганический углерод", "мг C/л", f.TotalCarbon * 12011.1}, {"alkalinity", "Общая щёлочность", "мг/л как CaCO₃", f.Alkalinity * 50043.45}} {
		measured, ok := parseLabNumber(v[c.key])
		if !ok || measured < 0 {
			w = append(w, c.label+": независимый анализ отсутствует или некорректен; сверка не выполнена.")
			continue
		}
		denominator := math.Max(math.Abs(measured), math.Abs(c.calculated))
		difference := 0.0
		if denominator > 0 {
			difference = 100 * (c.calculated - measured) / denominator
		}
		if math.Abs(difference) > 10 {
			w = append(w, fmt.Sprintf("%s: расчёт %.6g, анализ %.6g %s; расхождение %+.2f %% выше рабочего уровня 10 %%. Сверить формы углерода, условия пробы и комплексообразование; уровень не нормативный.", c.label, c.calculated, measured, c.unit, difference))
		}
	}
	return w
}
func laboratoryCarbonIndependentBalanceText(v map[string]string, f laboratoryCarbonForms) string {
	measured, ok := parseLabNumber(v["tic"])
	if !ok || measured < 0 {
		return "Баланс углерода: независимый анализ общего неорганического углерода не задан; баланс не подтверждён."
	}
	calculated := f.TotalCarbon * 12011.1
	denominator := math.Max(math.Abs(measured), math.Abs(calculated))
	d := 0.0
	if denominator > 0 {
		d = 100 * (calculated - measured) / denominator
	}
	return fmt.Sprintf("Независимая сверка углерода: сумма свободных форм %.6g, лабораторный общий неорганический углерод %.6g мг C/л; расхождение %+.2f %% (от большего модуля). Комплексные формы не включены; исходный анализ не заменён.", calculated, measured, d)
}
