//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

type laboratoryTemperatureProfilePoint struct {
	Label       string
	Temperature float64
	PressureMPa float64
	Results     []laboratorySaturationResult
}

// laboratoryTemperatureProfile evaluates only the already-verified temperature
// dependence of Ksp at three profile points. Pressure is retained as provenance
// but is deliberately not applied until a numerical PHREEQC pressure benchmark
// is stored with the project.
func laboratoryTemperatureProfile(values map[string]string) ([]laboratoryTemperatureProfilePoint, error) {
	keys := []string{"profile_temp_min", "profile_pressure_min", "profile_temp_max", "profile_pressure_max"}
	present := 0
	for _, k := range keys {
		if strings.TrimSpace(values[k]) != "" {
			present++
		}
	}
	if present == 0 {
		return nil, nil
	}
	if present != len(keys) {
		return nil, fmt.Errorf("Для профильного температурного скрининга заполните Tmin/P(Tmin) и Tmax/P(Tmax) полностью")
	}
	tmin, ok1 := parseLabNumber(values["profile_temp_min"])
	pmin, ok2 := parseLabNumber(values["profile_pressure_min"])
	tmax, ok3 := parseLabNumber(values["profile_temp_max"])
	pmax, ok4 := parseLabNumber(values["profile_pressure_max"])
	if !ok1 || !ok2 || !ok3 || !ok4 || math.IsNaN(tmin) || math.IsNaN(tmax) || math.IsNaN(pmin) || math.IsNaN(pmax) || math.IsInf(tmin, 0) || math.IsInf(tmax, 0) || math.IsInf(pmin, 0) || math.IsInf(pmax, 0) {
		return nil, fmt.Errorf("Профиль T/P содержит некорректное число")
	}
	if tmin > tmax {
		return nil, fmt.Errorf("Минимальная температура профиля выше максимальной")
	}
	if pmin < 0 || pmax < 0 || pmin > 250 || pmax > 250 {
		return nil, fmt.Errorf("Давление профиля должно быть в диапазоне 0–250 МПа")
	}
	if _, err := laboratoryTemperatureLogK("Кальцит", tmin); err != nil {
		return nil, err
	}
	if _, err := laboratoryTemperatureLogK("Кальцит", tmax); err != nil {
		return nil, err
	}
	ts := []float64{tmin, .5 * (tmin + tmax), tmax}
	ps := []float64{pmin, .5 * (pmin + pmax), pmax}
	labels := []string{"Tmin", "середина", "Tmax"}
	out := make([]laboratoryTemperatureProfilePoint, 0, 3)
	for i := range ts {
		v := make(map[string]string, len(values)+1)
		for k, x := range values {
			v[k] = x
		}
		v["sample_temp"] = fmt.Sprintf("%.12g", ts[i])
		r, _, w := laboratorySaturationResults(v, "temperature_ideal")
		if len(r) == 0 {
			if len(w) > 0 {
				return nil, fmt.Errorf("Точка %s: %s", labels[i], w[0])
			}
			return nil, fmt.Errorf("Точка %s: нет требуемой пары ионов", labels[i])
		}
		out = append(out, laboratoryTemperatureProfilePoint{Label: labels[i], Temperature: ts[i], PressureMPa: ps[i], Results: r})
	}
	return out, nil
}

func laboratoryTemperatureProfileText(values map[string]string) string {
	pts, err := laboratoryTemperatureProfile(values)
	if err != nil {
		return "ПРОФИЛЬ T/P: НЕ РАССЧИТАНО — " + err.Error()
	}
	if len(pts) == 0 {
		return ""
	}
	lines := []string{
		"ПРОФИЛЬ T/P — ПРОВЕРЕНА ТОЛЬКО ТЕМПЕРАТУРНАЯ ЧАСТЬ",
		"Давление сохранено для каждой точки, но пока НЕ участвует в SI: численная pressure-поправка будет включена только после отдельного внешнего PHREEQC-эталона.",
	}
	for _, pt := range pts {
		vals := make([]string, 0, len(pt.Results))
		for _, r := range pt.Results {
			vals = append(vals, fmt.Sprintf("%s SI=%+.3f", r.Mineral, r.SI))
		}
		lines = append(lines, fmt.Sprintf("• %s: T=%.2f °C; P=%.3g МПа (сохранено, не применено); %s.", pt.Label, pt.Temperature, pt.PressureMPa, strings.Join(vals, "; ")))
	}
	return strings.Join(lines, "\r\n")
}
