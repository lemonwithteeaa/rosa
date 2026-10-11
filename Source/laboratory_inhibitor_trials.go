//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type laboratoryInhibitorTrial struct {
	PassID, ObjectKey, Date, GroupCode string
	Brand, Target, Protocol, Result    string
	Dose, Temperature, DurationH       float64
	Efficiency                         float64
	HasDose, HasTemperature            bool
	HasDuration, HasEfficiency         bool
	EfficiencyDerived                  bool
}

type laboratoryInhibitorDoseSeries struct {
	GroupCode string
	Brand     string
	Target    string
	Points    []laboratoryInhibitorTrial
}

func laboratoryBestInhibitorDoseSeries(trials []laboratoryInhibitorTrial) (laboratoryInhibitorDoseSeries, bool) {
	type seriesKey struct{ Group, Brand, Target string }
	series := map[seriesKey][]laboratoryInhibitorTrial{}
	for _, trial := range trials {
		if !trial.HasDose || !trial.HasEfficiency {
			continue
		}
		key := seriesKey{trial.GroupCode, strings.ToLower(strings.TrimSpace(trial.Brand)), strings.ToLower(strings.TrimSpace(trial.Target))}
		series[key] = append(series[key], trial)
	}
	var best laboratoryInhibitorDoseSeries
	bestLatest := time.Time{}
	for key, pts := range series {
		if len(pts) < 2 {
			continue
		}
		sort.SliceStable(pts, func(i, j int) bool {
			if pts[i].Dose == pts[j].Dose {
				return parseDate(pts[i].Date).Before(parseDate(pts[j].Date))
			}
			return pts[i].Dose < pts[j].Dose
		})
		latest := time.Time{}
		for _, pt := range pts {
			if d := parseDate(pt.Date); d.After(latest) {
				latest = d
			}
		}
		if len(pts) > len(best.Points) || len(pts) == len(best.Points) && latest.After(bestLatest) {
			best = laboratoryInhibitorDoseSeries{GroupCode: key.Group, Brand: pts[0].Brand, Target: pts[0].Target, Points: append([]laboratoryInhibitorTrial(nil), pts...)}
			bestLatest = latest
		}
	}
	return best, len(best.Points) >= 2
}

func laboratoryInhibitorDoseSeriesForObject(objectKey string, passes []LabPassRecord, current *LabPassRecord) (laboratoryInhibitorDoseSeries, bool) {
	return laboratoryBestInhibitorDoseSeries(laboratoryInhibitorTrialsForObject(objectKey, passes, current))
}

func laboratoryInhibitorDoseSeriesCaption(series laboratoryInhibitorDoseSeries) string {
	group := series.GroupCode
	if group == "corrosion" {
		group = "Коррозия"
	} else if group == "scale" {
		group = "Солеотложения"
	}
	return fmt.Sprintf("Dose-response · %s · %s · %s", group, series.Brand, series.Target)
}

func laboratoryInhibitorTrialFromPass(pass LabPassRecord, groupCode string) (laboratoryInhibitorTrial, bool) {
	group, ok := labGroupByCode(groupCode)
	if !ok || (groupCode != "corrosion" && groupCode != "scale") {
		return laboratoryInhibitorTrial{}, false
	}
	values := canonicalizeLaboratoryValues(group, pass.Groups[groupCode], pass.Units[groupCode])
	trial := laboratoryInhibitorTrial{
		PassID: pass.PassID, ObjectKey: pass.ObjectKey, Date: pass.Date, GroupCode: groupCode,
		Brand:    strings.TrimSpace(values["inhibitor_trial_brand"]),
		Target:   strings.TrimSpace(values["inhibitor_trial_target"]),
		Protocol: strings.TrimSpace(values["inhibitor_trial_protocol"]),
		Result:   strings.TrimSpace(values["inhibitor_trial_result"]),
	}
	if trial.Brand == "" && groupCode == "corrosion" {
		trial.Brand = strings.TrimSpace(values["inhibitor_brand_water"])
	}
	if trial.Protocol == "" {
		trial.Protocol = strings.TrimSpace(pass.Source)
	}
	if n, ok := parseLabNumber(values["inhibitor_trial_dose"]); ok {
		trial.Dose, trial.HasDose = n, true
	}
	if n, ok := parseLabNumber(values["inhibitor_trial_temp"]); ok {
		trial.Temperature, trial.HasTemperature = n, true
	}
	if n, ok := parseLabNumber(values["inhibitor_trial_duration_h"]); ok {
		trial.DurationH, trial.HasDuration = n, true
	}
	if n, ok := parseLabNumber(values["inhibitor_trial_efficiency"]); ok {
		trial.Efficiency, trial.HasEfficiency = n, true
	} else if groupCode == "corrosion" {
		without, okWithout := parseLabNumber(values["corrosion_rate_without"])
		with, okWith := parseLabNumber(values["corrosion_rate_with"])
		if okWithout && okWith && without > 0 && with >= 0 && with <= without {
			trial.Efficiency = (without - with) / without * 100
			trial.HasEfficiency, trial.EfficiencyDerived = true, true
		}
	}
	endpoint := trial.HasEfficiency || trial.Result != ""
	if trial.Brand == "" || trial.Target == "" || !trial.HasDose || !endpoint {
		return laboratoryInhibitorTrial{}, false
	}
	return trial, true
}

func laboratoryInhibitorTrialsForObject(objectKey string, passes []LabPassRecord, current *LabPassRecord) []laboratoryInhibitorTrial {
	objectKey = normalizeLaboratoryObjectKey(objectKey)
	byID := map[string]laboratoryInhibitorTrial{}
	add := func(pass LabPassRecord) {
		if pass.Status != labPassSavedStatus && (current == nil || pass.PassID != current.PassID) {
			return
		}
		if normalizeLaboratoryObjectKey(pass.ObjectKey) != objectKey {
			return
		}
		for _, groupCode := range []string{"corrosion", "scale"} {
			if trial, ok := laboratoryInhibitorTrialFromPass(pass, groupCode); ok {
				byID[pass.PassID+"|"+groupCode] = trial
			}
		}
	}
	for _, pass := range passes {
		add(pass)
	}
	if current != nil {
		add(*current)
	}
	out := make([]laboratoryInhibitorTrial, 0, len(byID))
	for _, trial := range byID {
		out = append(out, trial)
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := parseDate(out[i].Date), parseDate(out[j].Date)
		if di.Equal(dj) {
			return out[i].Dose < out[j].Dose
		}
		if di.IsZero() {
			return false
		}
		if dj.IsZero() {
			return true
		}
		return di.Before(dj)
	})
	return out
}

func laboratoryInhibitorTrialRowText(t laboratoryInhibitorTrial) string {
	parts := []string{emptyLabText(t.Date), t.Brand, t.Target, fmt.Sprintf("доза %g мг/л", t.Dose)}
	if t.HasTemperature {
		parts = append(parts, fmt.Sprintf("T %g °C", t.Temperature))
	}
	if t.HasDuration {
		parts = append(parts, fmt.Sprintf("%g ч", t.DurationH))
	}
	if t.HasEfficiency {
		suffix := ""
		if t.EfficiencyDerived {
			suffix = " (расчёт по парному испытанию)"
		}
		parts = append(parts, fmt.Sprintf("эффективность %g %%", t.Efficiency)+suffix)
	}
	if t.Result != "" {
		parts = append(parts, "результат: "+t.Result)
	}
	if t.Protocol != "" {
		parts = append(parts, "протокол: "+t.Protocol)
	}
	return strings.Join(parts, "; ")
}

func laboratoryInhibitorDoseResponseText(trials []laboratoryInhibitorTrial) string {
	type seriesKey struct{ Group, Brand, Target string }
	series := map[seriesKey][]laboratoryInhibitorTrial{}
	for _, trial := range trials {
		if !trial.HasDose || !trial.HasEfficiency {
			continue
		}
		key := seriesKey{trial.GroupCode, strings.ToLower(trial.Brand), strings.ToLower(trial.Target)}
		series[key] = append(series[key], trial)
	}
	var lines []string
	for key, pts := range series {
		if len(pts) < 2 {
			continue
		}
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].Dose < pts[j].Dose })
		pointText := make([]string, 0, len(pts))
		for _, p := range pts {
			pointText = append(pointText, fmt.Sprintf("%g мг/л → %g %%", p.Dose, p.Efficiency))
		}
		lines = append(lines, fmt.Sprintf("%s / %s / %s: %s", key.Group, pts[0].Brand, pts[0].Target, strings.Join(pointText, "; ")))
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		return "ЗАВИСИМОСТЬ «ДОЗА–ЭФФЕКТ» (dose–response): недостаточно минимум двух фактических точек с одинаковыми маркой и целью испытания. РОСА не рассчитывает оптимальную дозу."
	}
	return "ЗАВИСИМОСТЬ «ДОЗА–ЭФФЕКТ» (dose–response) ПО ФАКТИЧЕСКИМ ИСПЫТАНИЯМ\n" + strings.Join(lines, "\n") + "\nИнтерполяция между точками, минимальная эффективная и оптимальная дозы не рассчитываются: для решения нужен утверждённый протокол испытаний и инженерная проверка."
}

func laboratoryInhibitorDatabaseText(pass LabPassRecord, passes []LabPassRecord) string {
	trials := laboratoryInhibitorTrialsForObject(pass.ObjectKey, passes, &pass)
	if len(trials) == 0 {
		return "БАЗА ФАКТИЧЕСКИХ ИСПЫТАНИЙ ИНГИБИТОРОВ\nДля этого объекта подтверждённых испытательных точек пока нет. Производственное наличие реагента не считается лабораторным испытанием."
	}
	shown := trials
	if len(shown) > 8 {
		shown = shown[len(shown)-8:]
	}
	lines := []string{"БАЗА ФАКТИЧЕСКИХ ИСПЫТАНИЙ ИНГИБИТОРОВ"}
	for _, trial := range shown {
		lines = append(lines, "• "+laboratoryInhibitorTrialRowText(trial))
	}
	lines = append(lines, laboratoryInhibitorDoseResponseText(trials))
	lines = append(lines, "Ограничение: база хранит фактические точки и не выдаёт рекомендованную/минимальную эффективную дозу без утверждённой методики и достаточной серии испытаний.")
	return strings.Join(lines, "\n")
}

func laboratoryInhibitorHistoryShort(objectKey string, passes []LabPassRecord) string {
	trials := laboratoryInhibitorTrialsForObject(objectKey, passes, nil)
	if len(trials) == 0 {
		return "ИСПЫТАНИЯ ИНГИБИТОРОВ: фактических точек нет"
	}
	series := strings.Contains(laboratoryInhibitorDoseResponseText(trials), "ПО ФАКТИЧЕСКИМ ИСПЫТАНИЯМ")
	if series {
		return fmt.Sprintf("ИСПЫТАНИЯ ИНГИБИТОРОВ: %d точек; есть серия dose-response", len(trials))
	}
	return fmt.Sprintf("ИСПЫТАНИЯ ИНГИБИТОРОВ: %d точек; для dose-response нужна ещё сопоставимая точка", len(trials))
}
