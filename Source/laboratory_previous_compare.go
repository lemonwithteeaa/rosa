//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type laboratoryComparisonMetric struct {
	GroupCode string
	Key       string
	Label     string
	Unit      string
	Mode      string // percent | absolute
	Threshold float64
}

var laboratoryComparisonMetrics = []laboratoryComparisonMetric{
	{GroupCode: "scale", Key: "mineralization", Label: "Общая минерализация", Unit: "г/л", Mode: "percent", Threshold: 10},
	{GroupCode: "scale", Key: "cl", Label: "Cl⁻", Unit: "мг/л", Mode: "percent", Threshold: 15},
	{GroupCode: "scale", Key: "so4", Label: "SO₄²⁻", Unit: "мг/л", Mode: "percent", Threshold: 15},
	{GroupCode: "scale", Key: "ca", Label: "Ca²⁺", Unit: "мг/л", Mode: "percent", Threshold: 15},
	{GroupCode: "scale", Key: "ba", Label: "Ba²⁺", Unit: "мг/л", Mode: "percent", Threshold: 20},
	{GroupCode: "scale", Key: "sr", Label: "Sr²⁺", Unit: "мг/л", Mode: "percent", Threshold: 20},
	{GroupCode: "scale", Key: "ph", Label: "pH", Unit: "ед.", Mode: "absolute", Threshold: 0.30},
	{GroupCode: "corrosion", Key: "iron_total", Label: "Железо общее", Unit: "мг/л", Mode: "percent", Threshold: 25},
	{GroupCode: "corrosion", Key: "orp", Label: "ОВП", Unit: "мВ", Mode: "absolute", Threshold: 50},
}

func laboratoryPassDate(pass LabPassRecord) time.Time {
	if t, ok := laboratoryAnalysisDate(pass.Date); ok {
		return t
	}
	if t, err := time.Parse(time.RFC3339, strings.TrimSpace(pass.UpdatedAt)); err == nil {
		return t
	}
	return time.Time{}
}

func laboratoryPreviousSavedPass(current LabPassRecord, passes []LabPassRecord) (LabPassRecord, bool) {
	key := normalizeLaboratoryObjectKey(current.ObjectKey)
	if key == "" {
		return LabPassRecord{}, false
	}
	currentDate := laboratoryPassDate(current)
	candidates := make([]LabPassRecord, 0, len(passes))
	for _, p := range passes {
		if p.Status != labPassSavedStatus || p.PassID == current.PassID || normalizeLaboratoryObjectKey(p.ObjectKey) != key {
			continue
		}
		pd := laboratoryPassDate(p)
		if !currentDate.IsZero() && !pd.IsZero() && !pd.Before(currentDate) {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		return LabPassRecord{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		li, lj := laboratoryPassDate(candidates[i]), laboratoryPassDate(candidates[j])
		if !li.Equal(lj) {
			return li.After(lj)
		}
		return candidates[i].UpdatedAt > candidates[j].UpdatedAt
	})
	return candidates[0], true
}

func laboratoryPassCanonicalMetric(pass LabPassRecord, groupCode, key string) (float64, bool) {
	group, ok := labGroupByCode(groupCode)
	if !ok {
		return 0, false
	}
	values := canonicalizeLaboratoryValues(group, pass.Groups[groupCode], pass.Units[groupCode])
	n, ok := parseLabNumber(values[key])
	return n, ok
}

func laboratoryComparisonMetricLine(metric laboratoryComparisonMetric, current, previous float64) (string, bool) {
	delta := current - previous
	direction := "без изменения"
	if delta > 0 {
		direction = "↑"
	} else if delta < 0 {
		direction = "↓"
	}
	if metric.Mode == "absolute" {
		significant := math.Abs(delta) >= metric.Threshold
		return fmt.Sprintf("• %s: %.3g → %.3g %s; Δ %+.3g %s%s", metric.Label, previous, current, metric.Unit, delta, metric.Unit, comparisonFlag(significant)), significant
	}
	if previous == 0 {
		return fmt.Sprintf("• %s: %.3g → %.3g %s; относительная Δ не рассчитана при предыдущем 0%s", metric.Label, previous, current, metric.Unit, comparisonFlag(math.Abs(delta) > 0)), math.Abs(delta) > 0
	}
	pct := 100 * delta / math.Abs(previous)
	significant := math.Abs(pct) >= metric.Threshold
	return fmt.Sprintf("• %s: %.3g → %.3g %s; %s %+.1f%%%s", metric.Label, previous, current, metric.Unit, direction, pct, comparisonFlag(significant)), significant
}

func comparisonFlag(significant bool) string {
	if significant {
		return " — заметное изменение"
	}
	return ""
}

func laboratoryPassScaleWaterType(pass LabPassRecord) string {
	group, ok := labGroupByCode("scale")
	if !ok {
		return ""
	}
	values := canonicalizeLaboratoryValues(group, pass.Groups["scale"], pass.Units["scale"])
	return laboratorySulinWaterType(values)
}

func laboratoryPassComparisonText(current LabPassRecord, passes []LabPassRecord) string {
	previous, ok := laboratoryPreviousSavedPass(current, passes)
	if !ok {
		return "СРАВНЕНИЕ С ПРЕДЫДУЩИМ АНАЛИЗОМ\nПредыдущая сохранённая диагностика по этому объекту не найдена."
	}
	lines := []string{
		"СРАВНЕНИЕ С ПРЕДЫДУЩИМ АНАЛИЗОМ",
		fmt.Sprintf("Предыдущая диагностика: %s; источник: %s.", emptyLabText(previous.Date), emptyLabText(previous.Source)),
	}
	compared := 0
	significant := 0
	for _, metric := range laboratoryComparisonMetrics {
		cv, cok := laboratoryPassCanonicalMetric(current, metric.GroupCode, metric.Key)
		pv, pok := laboratoryPassCanonicalMetric(previous, metric.GroupCode, metric.Key)
		if !cok || !pok {
			continue
		}
		line, flag := laboratoryComparisonMetricLine(metric, cv, pv)
		lines = append(lines, line)
		compared++
		if flag {
			significant++
		}
	}
	currentType, previousType := laboratoryPassScaleWaterType(current), laboratoryPassScaleWaterType(previous)
	if currentType != "" && previousType != "" && !strings.HasPrefix(currentType, "Недостаточно данных") && !strings.HasPrefix(previousType, "Недостаточно данных") {
		if currentType != previousType {
			lines = append(lines, fmt.Sprintf("• Тип воды по Сулину: %s → %s — изменение классификации требует проверки состава и источника воды.", previousType, currentType))
			significant++
		} else {
			lines = append(lines, "• Тип воды по Сулину: классификация не изменилась ("+currentType+").")
		}
	}
	if compared == 0 {
		lines = append(lines, "Недостаточно совпадающих числовых показателей для сравнения двух анализов.")
	} else if significant == 0 {
		lines = append(lines, "По выбранным контрольным показателям выраженной динамики не выявлено. Это не исключает изменений по неохваченным параметрам.")
	} else {
		lines = append(lines, fmt.Sprintf("Заметные изменения отмечены по %d контрольным позициям. Сначала проверить сопоставимость проб, единиц, точки отбора и режима скважины; только затем трактовать изменение как технологический тренд.", significant))
	}
	return strings.Join(lines, "\n")
}

func laboratoryCurrentGroupComparisonShort(group LabGroupDef, currentValues, currentUnits map[string]string, objectKey, date, scaleModel string, passes []LabPassRecord) string {
	current := LabPassRecord{ObjectKey: objectKey, Date: date, Groups: map[string]map[string]string{group.Code: currentValues}, Units: map[string]map[string]string{group.Code: currentUnits}, ScaleModel: scaleModel}
	previous, ok := laboratoryPreviousSavedPass(current, passes)
	if !ok {
		return "ДИНАМИКА: предыдущий анализ не найден"
	}
	var parts []string
	for _, metric := range laboratoryComparisonMetrics {
		if metric.GroupCode != group.Code {
			continue
		}
		cv, cok := laboratoryPassCanonicalMetric(current, group.Code, metric.Key)
		pv, pok := laboratoryPassCanonicalMetric(previous, group.Code, metric.Key)
		if !cok || !pok {
			continue
		}
		if metric.Mode == "absolute" {
			delta := cv - pv
			if math.Abs(delta) >= metric.Threshold {
				parts = append(parts, fmt.Sprintf("%s %+.3g %s", metric.Label, delta, metric.Unit))
			}
		} else if pv != 0 {
			pct := 100 * (cv - pv) / math.Abs(pv)
			if math.Abs(pct) >= metric.Threshold {
				parts = append(parts, fmt.Sprintf("%s %+.0f%%", metric.Label, pct))
			}
		}
	}
	if len(parts) == 0 {
		return "ДИНАМИКА: выраженных изменений по контрольным показателям нет"
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return "ДИНАМИКА: " + strings.Join(parts, "; ")
}
