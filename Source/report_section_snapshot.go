//go:build windows

package main

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Historical RC733 QA marker: RC733-SECTIONS-V2
const fullReportSectionSnapshotVersion = "RC750-SECTIONS-V4"

func reportLatestSavedLabPassForObject(objectKey string) (LabPassRecord, bool) {
	rows := repositoryLabPassesForObject(objectKey)
	var candidates []LabPassRecord
	for _, pass := range rows {
		if pass.Status == labPassSavedStatus {
			candidates = append(candidates, pass)
		}
	}
	if len(candidates) == 0 {
		// Built-in cross-cutting examples used by section 8 existed before the
		// section snapshot was introduced. Rebuild their synthetic laboratory
		// passport so old approved teaching rows export the same level of detail
		// as a newly approved production row.
		if _, ok := teachingFullScenarioForObject(objectKey); ok {
			pass := LabPassRecord{
				PassID:    "TEACHING-REPORT-" + safeFilePart(objectKey),
				ObjectKey: objectKey,
				Date:      "06.10.2026",
				Author:    "Технолог · учебный контур",
				Status:    labPassSavedStatus,
				Groups:    map[string]map[string]string{},
				Units:     map[string]map[string]string{},
				CreatedAt: "2026-10-06T10:00:00Z",
				UpdatedAt: "2026-10-06T10:30:00Z",
				Comment:   "Синтетические данные для проверки полного маршрута и подробной выгрузки РОСА.",
			}
			teachingCompleteLabDraft(&pass)
			for _, group := range labGroups {
				if group.Code != "overall" && len(pass.Groups[group.Code]) > 0 {
					pass.Completed = append(pass.Completed, group.Code)
				}
			}
			pass.Conclusion = buildLaboratoryPassConclusion(pass)
			return pass, true
		}
		return LabPassRecord{}, false
	}
	rank := func(pass LabPassRecord) time.Time {
		for _, raw := range []string{pass.UpdatedAt, pass.CreatedAt, pass.ImportedAt} {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw)); err == nil {
				return t
			}
		}
		if t := parseDate(pass.Date); !t.IsZero() {
			return t
		}
		return time.Time{}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return rank(candidates[i]).Before(rank(candidates[j])) })
	latest := candidates[len(candidates)-1]
	// RC734: old bundled teaching passes may exist in user storage but contain
	// only the historical profile group. Enrich that synthetic pass in-memory
	// before snapshotting so an old teaching row exports the same full laboratory
	// passport as a fresh lesson. Production passes are never modified here.
	if _, teaching := teachingFullScenarioForObject(objectKey); teaching {
		teachingCompleteLabDraft(&latest)
		latest.Conclusion = buildLaboratoryPassConclusion(latest)
	}
	return latest, true
}

var reportLongDecimalPattern = regexp.MustCompile(`[-+]?\d+\.\d{4,}`)

func reportDisplayNumber(value float64) string {
	if value == 0 {
		return "0"
	}
	text := strconv.FormatFloat(value, 'f', 3, 64)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "-0" {
		return "0"
	}
	return text
}

func reportLimitNumericPrecision(text string) string {
	matches := reportLongDecimalPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	isDigit := func(b byte) bool { return b >= '0' && b <= '9' }
	var out strings.Builder
	last := 0
	for _, loc := range matches {
		start, end := loc[0], loc[1]
		// Do not reinterpret fragments of dates, versions or other dotted IDs
		// (for example 07.10.2026 or 7.3.11.747) as decimal values.
		if (start > 0 && (text[start-1] == '.' || isDigit(text[start-1]))) ||
			(end < len(text) && (text[end] == '.' || isDigit(text[end]))) {
			continue
		}
		raw := text[start:end]
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		out.WriteString(text[last:start])
		out.WriteString(reportDisplayNumber(value))
		last = end
	}
	if last == 0 {
		return text
	}
	out.WriteString(text[last:])
	return out.String()
}

func reportCleanUserText(text string) string {
	text = reportLimitNumericPrecision(text)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	lastHeading := ""
	blank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		isHeading := strings.HasPrefix(trimmed, "РАЗДЕЛ ") || strings.HasPrefix(trimmed, "ПРИЛОЖЕНИЕ ") || (trimmed == strings.ToUpper(trimmed) && len([]rune(trimmed)) > 4)
		if isHeading && strings.EqualFold(trimmed, lastHeading) {
			continue
		}
		if isHeading {
			lastHeading = trimmed
		} else {
			lastHeading = ""
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\r\n")) + "\r\n"
}

func reportLaboratoryRawData(pass LabPassRecord) string {
	var b strings.Builder
	b.WriteString("ИСХОДНЫЕ ЛАБОРАТОРНЫЕ ПОКАЗАТЕЛИ\r\n")
	if strings.TrimSpace(pass.Comment) != "" {
		b.WriteString("Комментарий лаборатории: " + strings.TrimSpace(pass.Comment) + "\r\n")
	}
	if strings.TrimSpace(pass.ImportFileName) != "" {
		b.WriteString("Файл-источник импорта: " + strings.TrimSpace(pass.ImportFileName) + "\r\n")
	}
	if strings.TrimSpace(pass.ImportedAt) != "" {
		b.WriteString("Дата импорта: " + strings.TrimSpace(pass.ImportedAt) + "\r\n")
	}
	if strings.TrimSpace(pass.UpdatedAt) != "" {
		b.WriteString("Последнее изменение: " + strings.TrimSpace(pass.UpdatedAt) + "\r\n")
	}
	for _, baseGroup := range labGroups {
		if baseGroup.Code == "overall" {
			continue
		}
		group := laboratoryScaleInputGroup(baseGroup, pass.ScaleModel)
		values := pass.Groups[group.Code]
		units := pass.Units[group.Code]
		var groupText strings.Builder
		filled := 0
		known := map[string]bool{}
		allFields := map[string]LabFieldDef{}
		for _, field := range baseGroup.Fields {
			allFields[field.Key] = field
		}
		for _, field := range group.Fields {
			known[field.Key] = true
			value := strings.TrimSpace(values[field.Key])
			if value == "" {
				continue
			}
			filled++
			unit := strings.TrimSpace(units[field.Key])
			if unit == "" {
				unit = field.Unit
			}
			groupText.WriteString(reportLaboratoryIndicatorLine(field.Label, value, unit))
		}
		var extraKeys []string
		for key, value := range values {
			if !known[key] && strings.TrimSpace(value) != "" && reportLaboratoryFieldBelongsToModel(key, pass.ScaleModel) {
				extraKeys = append(extraKeys, key)
			}
		}
		sort.Strings(extraKeys)
		for _, key := range extraKeys {
			field, catalogued := allFields[key]
			label := field.Label
			if !catalogued {
				label = "Дополнительный показатель (" + key + ")"
			}
			unit := strings.TrimSpace(units[key])
			if unit == "" && catalogued {
				unit = field.Unit
			}
			groupText.WriteString(reportLaboratoryIndicatorLine(label, strings.TrimSpace(values[key]), unit))
			filled++
		}
		if filled == 0 {
			// Unfilled fields belong in Appendix 1 at the end of the report.
			continue
		}
		b.WriteString("\r\nНАПРАВЛЕНИЕ: " + strings.ToUpper(group.Name) + "\r\n")
		b.WriteString(groupText.String())
		assessment := laboratoryGroupSummary(group, values, pass.ScaleModel)
		fmt.Fprintf(&b, "Итог направления: %s; базовый набор %d/%d.\r\n", firstNonEmpty(strings.TrimSpace(assessment.State), "нет оценки"), assessment.BaseFilled, assessment.BaseTotal)
	}
	if len(pass.SharedValues) > 0 {
		keys := make([]string, 0, len(pass.SharedValues))
		for key := range pass.SharedValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var shared strings.Builder
		for _, key := range keys {
			value := strings.TrimSpace(pass.SharedValues[key])
			if value == "" || reportSharedValueRepeatedInGroup(pass, key, value) {
				continue
			}
			if label, unit, ok := reportSharedFieldDefinition(key); ok {
				shared.WriteString(reportLaboratoryIndicatorLine(label, value, unit))
			}
		}
		if shared.Len() > 0 {
			b.WriteString("\r\nОБЩИЕ ПАРАМЕТРЫ ОБЪЕКТА И РЕЖИМА\r\nПоказатели из общего паспорта анализа, не повторяющие значения профильных разделов.\r\n")
			b.WriteString(shared.String())
		}
	}
	if len(pass.ExtraValues) > 0 {
		keys := make([]string, 0, len(pass.ExtraValues))
		for key := range pass.ExtraValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var extra strings.Builder
		for _, key := range keys {
			if strings.TrimSpace(pass.ExtraValues[key]) == "" || reportLaboratoryKeyAlreadyPresent(pass, key) {
				continue
			}
			switch key {
			case "qc_note":
				extra.WriteString(reportLaboratoryIndicatorLine("Пояснение к качеству данных", pass.ExtraValues[key], ""))
			case "sample_chain":
				extra.WriteString(reportLaboratoryIndicatorLine("Цепочка отбора и утверждения пробы", pass.ExtraValues[key], ""))
			default:
				if label, unit, ok := reportSharedFieldDefinition(key); ok {
					extra.WriteString(reportLaboratoryIndicatorLine(label, pass.ExtraValues[key], unit))
				}
			}
		}
		if extra.Len() > 0 {
			b.WriteString("\r\nДОПОЛНИТЕЛЬНЫЕ ИМПОРТИРОВАННЫЕ ПОКАЗАТЕЛИ\r\n")
			b.WriteString(extra.String())
		}
	}
	return b.String()
}

func reportSharedValueRepeatedInGroup(pass LabPassRecord, key, value string) bool {
	for _, group := range pass.Groups {
		if strings.TrimSpace(group[key]) == value {
			return true
		}
	}
	return false
}

func reportSharedFieldDefinition(key string) (label, unit string, ok bool) {
	known := map[string]struct{ label, unit string }{
		"watercut":           {"Обводнённость продукции", "%"},
		"liquid_rate_m3_day": {"Дебит жидкости", "м³/сут"},
		"oil_rate_t_day":     {"Дебит нефти", "т/сут"},
		"sampling_point":     {"Точка отбора проб", "текст"},
	}
	if item, found := known[key]; found {
		return item.label, item.unit, true
	}
	for _, group := range labGroups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field.Label, field.Unit, true
			}
		}
	}
	return "", "", false
}

func reportLaboratoryFieldBelongsToModel(key, model string) bool {
	if strings.HasPrefix(key, "co2_eq_") {
		return model == "co2_gas_water"
	}
	if strings.HasPrefix(key, "mix1_") || strings.HasPrefix(key, "mix2_") || strings.HasPrefix(key, "mix_") {
		return model == "water_mixing"
	}
	return true
}

func reportLaboratoryKeyAlreadyPresent(pass LabPassRecord, key string) bool {
	if strings.TrimSpace(pass.SharedValues[key]) != "" {
		return true
	}
	for _, values := range pass.Groups {
		if strings.TrimSpace(values[key]) != "" {
			return true
		}
	}
	return false
}

// Keep the measured value and its unit together even when Word wraps the line.
// An unknown imported field receives no invented unit.
func reportLaboratoryIndicatorLine(label, value, unit string) string {
	if number, ok := parseLabNumber(value); ok {
		value = reportDisplayNumber(number)
	}
	if unit = strings.TrimSpace(unit); unit != "" && !strings.EqualFold(unit, "текст") && !strings.EqualFold(unit, "расчёт") {
		return fmt.Sprintf("• %s: %s\u00a0%s\r\n", label, value, strings.ReplaceAll(unit, " ", "\u00a0"))
	}
	return fmt.Sprintf("• %s: %s\r\n", label, value)
}

func reportMethodAssessmentDetail(a MethodEffectAssessment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Метод: %s\r\nНаправление: %s\r\nКлассификация: %s\r\nПрименимость: %d%%\r\nДостоверность объектовой оценки: %d%% (%s)\r\n", dash(a.Method), dash(a.Direction), dash(a.Classification), a.Applicability, a.Reliability, firstNonEmpty(strings.TrimSpace(a.ReliabilityLevel), dash(a.Confidence)))
	fmt.Fprintf(&b, "Совпадений с историей: %d; подтверждённых совпадений: %d; доля релевантных причин отказов: %.0f%%.\r\n", a.HistoryMatches, a.HistoryApprovedMatch, a.CauseRiskShare*100)
	if a.ProbabilityAvailable {
		fmt.Fprintf(&b, "Вероятность подтверждения эффекта по внутренней статистике: %d%%; интервал %d–%d%%; выборка %d применений / %d успехов / %d объектов.\r\n", a.EffectProbability, a.ProbabilityLow, a.ProbabilityHigh, a.ProbabilitySample, a.ProbabilitySuccesses, a.ProbabilityObjects)
	} else {
		b.WriteString("Вероятность успеха: не рассчитывается — недостаточно сопоставимой подтверждённой внутренней статистики.\r\n")
	}
	if a.EffectQuantified {
		fmt.Fprintf(&b, "Оценочный вклад в наработку: +%.0f%% / +%.0f%% / +%.0f%% (негативный / оптимальный / позитивный).\r\n", a.LowPct, a.BasePct, a.HighPct)
	} else {
		b.WriteString("Численный вклад в МРП автоматически не начисляется; эффект учитывается качественно.\r\n")
	}
	if strings.TrimSpace(a.EffectBasisType) != "" {
		b.WriteString("Тип доказательной базы эффекта: " + strings.TrimSpace(a.EffectBasisType) + ".\r\n")
	}
	if strings.TrimSpace(a.Evidence) != "" {
		b.WriteString("Основание: " + strings.TrimSpace(a.Evidence) + "\r\n")
	}
	if len(a.Reasons) > 0 {
		b.WriteString("Факторы в пользу метода:\r\n")
		for _, item := range a.Reasons {
			b.WriteString("• " + item + "\r\n")
		}
	}
	if len(a.Warnings) > 0 {
		b.WriteString("Ограничения / предупреждения:\r\n")
		for _, item := range a.Warnings {
			b.WriteString("• " + item + "\r\n")
		}
	}
	return b.String()
}

func reportSection1FundAndFailures(objectKey string) string {
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 1. ФОНД И КАРТОЧКА СКВАЖИНЫ\r\n")
	if st, ok := currentSelectedState(); ok && methodsObjectKeysEquivalent(selectedObjectKey, objectKey) {
		fmt.Fprintf(&b, "Организация: %s\r\n%s: %s\r\nМесторождение: %s\r\nСкважина: %s\r\nЗалежь / объект / пласт: %s\r\nГНО: %s\r\nТекущий МРП: %s сут.\r\n",
			dash(st.Organization), currentUnitLabel(), dash(st.ProductionUnit), dash(st.Field), dash(st.Well), dash(st.ObjectPlast), dash(st.Equipment), formatWholeDaysText(st.CurrentMRP))
	} else if v, ok := teachingCompletedVersionForObject(objectKey); ok {
		fmt.Fprintf(&b, "Месторождение: %s\r\nСкважина: %s\r\nЗалежь: %s\r\nОбъект / пласт: %s\r\nГНО: %s\r\nТекущий МРП: %s сут.\r\n", dash(v.Field), dash(v.Well), dash(v.Deposit), dash(v.ObjectPlast), dash(v.Equipment), dash(v.CurrentMRP))
	} else {
		fmt.Fprintf(&b, "Объект: %s\r\n", readableObjectKey(objectKey))
	}

	failures := failuresForObject(objectKey, time.Now())
	if len(failures) == 0 {
		if teaching, ok := teachingWorkflowOrdinal(objectKey); ok && teaching > 0 {
			failures = teachingFailureHistoryForObject(objectKey, time.Now())
		}
	}
	b.WriteString("\r\nПРЕДЫСТОРИЯ ОТКАЗОВ · 36 МЕС.\r\n")
	if len(failures) == 0 {
		b.WriteString("Подтверждённые/внесённые события для объекта не найдены.\r\n")
		return b.String()
	}
	approved := 0
	causeCounts := map[string]int{}
	for _, e := range failures {
		if e.ConclusionApproved || strings.Contains(strings.ToLower(e.InvestigationStatus), "подтверждено комиссией") {
			approved++
		}
		cause := firstNonEmpty(strings.TrimSpace(e.FinalCause), strings.TrimSpace(e.SourceCause), strings.TrimSpace(e.StopCause), strings.TrimSpace(e.RefinedComplication), strings.TrimSpace(e.SourceClass))
		if cause != "" {
			causeCounts[cause]++
		}
	}
	fmt.Fprintf(&b, "Всего событий: %d; подтверждено: %d; предварительно / без утверждения: %d.\r\n", len(failures), approved, len(failures)-approved)
	if len(causeCounts) > 0 {
		causes := make([]string, 0, len(causeCounts))
		for cause := range causeCounts {
			causes = append(causes, cause)
		}
		sort.SliceStable(causes, func(i, j int) bool {
			if causeCounts[causes[i]] == causeCounts[causes[j]] {
				return causes[i] < causes[j]
			}
			return causeCounts[causes[i]] > causeCounts[causes[j]]
		})
		parts := make([]string, 0, len(causes))
		for _, cause := range causes {
			parts = append(parts, fmt.Sprintf("%s — %d", cause, causeCounts[cause]))
		}
		b.WriteString("Повторяемость причин / осложнений: " + strings.Join(parts, "; ") + ".\r\n")
	}
	b.WriteString("\r\nСОБЫТИЯ\r\n")
	for i, e := range failures {
		status := "предварительно"
		if e.ConclusionApproved || strings.Contains(strings.ToLower(e.InvestigationStatus), "подтверждено комиссией") {
			status = "подтверждено"
		}
		cause := firstNonEmpty(strings.TrimSpace(e.FinalCause), strings.TrimSpace(e.SourceCause), strings.TrimSpace(e.StopCause), "—")
		class := firstNonEmpty(strings.TrimSpace(e.RefinedComplication), strings.TrimSpace(e.NormalizedClass), strings.TrimSpace(e.SourceClass), "—")
		cycle := ""
		if start, err1 := failureDate(e.CycleStartDate); err1 == nil {
			if end, err2 := failureDate(e.Date); err2 == nil && !end.Before(start) {
				cycle = fmt.Sprintf("; наработка цикла %d сут.", int(end.Sub(start).Hours()/24))
			}
		}
		fmt.Fprintf(&b, "%d. %s · %s · причина: %s · класс: %s · ГНО: %s · %s%s\r\n", i+1, dash(e.Date), dash(e.EventType), dash(cause), dash(class), dash(e.Equipment), status, cycle)
	}
	return b.String()
}

func technologistReportSection(pass LabPassRecord) string {
	brief := strings.TrimSpace(reportLaboratoryActionConclusion(pass))
	if brief == "" {
		brief = strings.TrimSpace(buildLaboratoryPassConclusion(pass))
	}
	if brief == "" {
		return "Вывод для технолога не сформирован: недостаточно подтверждённых лабораторных данных."
	}
	return reportCleanUserText(brief)
}

func reportLaboratoryCalculatedSummary(pass LabPassRecord) string {
	var b strings.Builder
	modelCode := strings.TrimSpace(pass.ScaleModel)
	if modelCode == "" {
		modelCode = "davies"
	}
	model := laboratoryScaleModelByCode(modelCode)
	if scale := pass.Groups["scale"]; len(scale) > 0 {
		results, ionicStrength, warnings := laboratorySaturationResults(scale, model.Code)
		strengthUnit := "моль/л"
		if model.Code == "multicomplex_molal" || model.Code == "water_mixing" {
			strengthUnit = "моль/кг воды"
		}
		if ionicStrength > 0 || len(results) > 0 {
			fmt.Fprintf(&b, "Солеотложения · модель: %s. Ионная сила I = %.3g %s.\r\n", model.Name, ionicStrength, strengthUnit)
		}
		if balance, ok := laboratoryScaleModelIonBalance(scale, model.Code); ok {
			fmt.Fprintf(&b, "Баланс зарядов: %+.1f %%", balance)
			if math.Abs(balance) > 5 {
				b.WriteString(" — превышен контрольный уровень ±5 %; анализ требует сверки")
			}
			b.WriteString(".\r\n")
		}
		if model.Code == "davies" && ionicStrength > 0 {
			fmt.Fprintf(&b, "Коэффициенты активности Дэвиса: γ₁ = %.3f; γ₂ = %.3f.\r\n", daviesActivityCoefficient(1, ionicStrength), daviesActivityCoefficient(2, ionicStrength))
		}
		for _, result := range results {
			state := "ненасыщено"
			if result.SI > 0.1 {
				state = "пересыщено"
			} else if result.SI >= -0.1 {
				state = "близко к насыщению"
			}
			fmt.Fprintf(&b, "%s (%s): Кнас = %.3g; SI = %.3f — %s.\r\n", result.Mineral, result.Formula, result.Ratio, result.SI, state)
		}
		if len(warnings) > 0 {
			// One compact limitation line is enough in the general report; the
			// editable workbook retains the detailed numerical trace.
			fmt.Fprintf(&b, "Ограничение расчёта солей: %s\r\n", reportCleanUserText(warnings[0]))
		}
	}

	if scale := pass.Groups["scale"]; len(scale) > 0 {
		qc := laboratoryWaterQualityQC(scale)
		if qc.TDSComparisonOK {
			fmt.Fprintf(&b, "Контроль TDS: %.0f мг/л; сумма основных ионов %.0f мг/л; расхождение %.1f %%.\r\n", qc.TDSMgL, qc.IonSumMgL, qc.TDSDifferencePct)
		}
		if qc.HardnessOK {
			fmt.Fprintf(&b, "Расчётная жёсткость Ca²⁺+Mg²⁺: %.3f мг-экв/л (%.0f мг/л как CaCO₃).\r\n", qc.HardnessMeqL, qc.HardnessCaCO3MgL)
		}
		if sulin := strings.TrimSpace(laboratorySulinWaterType(scale)); sulin != "" && !strings.HasPrefix(sulin, "Недостаточно данных") {
			fmt.Fprintf(&b, "Тип воды по В. А. Сулину: %s.\r\n", sulin)
		}
	}

	if values := pass.Groups["corrosion"]; len(values) > 0 {
		loss, lossOK := parseLabNumber(values["plate_mass_loss"])
		initial, initialOK := parseLabNumber(values["plate_initial_mass"])
		final, finalOK := parseLabNumber(values["plate_final_mass"])
		if !lossOK && initialOK && finalOK && initial >= final {
			loss, lossOK = initial-final, true
		}
		if lossOK {
			if days, okDays := parseLabNumber(values["plate_exposure_days"]); okDays && days > 0 {
				if area, okArea := parseLabNumber(values["plate_area"]); okArea && area > 0 {
					if density, okDensity := parseLabNumber(values["plate_material_density"]); okDensity && density > 0 {
						rate := 87.6 * loss * 1000 / (density * area * days * 24)
						fmt.Fprintf(&b, "Коррозия по контрольной пластинке: %.3f мм/год.\r\n", rate)
					}
				}
			}
		}
		if without, okWithout := parseLabNumber(values["corrosion_rate_without"]); okWithout && without > 0 {
			if with, okWith := parseLabNumber(values["corrosion_rate_with"]); okWith {
				fmt.Fprintf(&b, "Эффективность ингибиторной защиты: %.1f %%.\r\n", (without-with)/without*100)
			}
		}
		if actual, okActual := parseLabNumber(values["wall_thickness"]); okActual {
			if minimum, okMinimum := parseLabNumber(values["minimum_wall_thickness"]); okMinimum {
				fmt.Fprintf(&b, "Запас толщины стенки: %.3f мм.\r\n", actual-minimum)
			}
		}
		if depth, okDepth := parseLabNumber(values["pit_depth"]); okDepth {
			if years, okYears := parseLabNumber(values["pit_period_years"]); okYears && years > 0 {
				fmt.Fprintf(&b, "Ориентировочная локальная скорость по язве: %.3f мм/год.\r\n", depth/years)
			}
		}
	}

	if values := pass.Groups["gases"]; len(values) > 0 {
		if actual, okActual := parseLabNumber(values["sample_temp"]); okActual {
			if hydrate, okHydrate := parseLabNumber(values["hydrate_temp"]); okHydrate {
				fmt.Fprintf(&b, "Температурный запас до гидратообразования ΔT = %.3f °C.\r\n", actual-hydrate)
			}
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return "Расчётные показатели по заполненным данным не сформированы.\r\n"
	}
	return b.String()
}

func reportSection2Laboratory(objectKey string) string {
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\n")
	if pass, ok := reportLatestSavedLabPassForObject(objectKey); ok {
		fmt.Fprintf(&b, "Идентификатор анализа: %s\r\nДата анализа: %s\r\nИсточник / протокол: %s\r\nАвтор: %s\r\nСтатус: %s\r\nМодель солеотложений: %s\r\n",
			emptyLabText(pass.PassID), emptyLabText(pass.Date), emptyLabText(pass.Source), emptyLabText(pass.Author), emptyLabText(pass.Status), emptyLabText(pass.ScaleModel))

		b.WriteString("\r\n" + reportLaboratoryRawData(pass))
		b.WriteString("\r\nРАСЧЁТНЫЕ ПОКАЗАТЕЛИ\r\n")
		b.WriteString(reportLaboratoryCalculatedSummary(pass))
		b.WriteString("\r\nВЫВОД ДЛЯ ТЕХНОЛОГА\r\n")
		b.WriteString(technologistReportSection(pass))
		return reportCleanUserText(b.String())
	}

	latest := latestLaboratoryRecordsForObject(objectKey)
	if len(latest) == 0 {
		b.WriteString("Подтверждённый сохранённый лабораторный анализ по объекту отсутствует.\r\n")
		return b.String()
	}
	codes := make([]string, 0, len(latest))
	for code := range latest {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		rec := latest[code]
		group, _ := labGroupByCode(code)
		name := code
		if strings.TrimSpace(group.Name) != "" {
			name = group.Name
		}
		fmt.Fprintf(&b, "\r\n%s\r\nДата: %s\r\nИсточник: %s\r\nКомментарий: %s\r\n", strings.ToUpper(name), emptyLabText(rec.Date), emptyLabText(rec.Source), emptyLabText(rec.Comment))
		for _, field := range group.Fields {
			if value := strings.TrimSpace(rec.Values[field.Key]); value != "" {
				unit := strings.TrimSpace(rec.Units[field.Key])
				if unit == "" {
					unit = field.Unit
				}
				fmt.Fprintf(&b, "%s: %s %s\r\n", field.Label, value, unit)
			}
		}
		if strings.TrimSpace(rec.Conclusion) != "" {
			b.WriteString("Заключение: " + strings.TrimSpace(rec.Conclusion) + "\r\n")
		}
	}
	b.WriteString("\r\nПримечание: запись создана в старом формате; расчётный паспорт нового формата отсутствовал на момент её сохранения.\r\n")
	return b.String()
}

func reportSection3Assessment(objectKey string) string {
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 3. ОЦЕНКА ОСЛОЖНЕНИЙ\r\n")
	detail := strings.TrimSpace(assessmentFinalReportText(objectKey))
	if detail == "" {
		detail = "Итоговая классификация по объекту не зафиксирована."
	}
	b.WriteString(detail + "\r\n")
	_, ready, _ := approvedAssessmentForMethods(objectKey)
	if ready {
		b.WriteString("Полнота утверждения: полная классификация подтверждена технологом.\r\n")
	} else {
		b.WriteString("Полнота утверждения: требуется подтверждение технологом.\r\n")
	}
	return b.String()
}

func reportSection4Methods(objectKey string, scenario AlternativeMethodScenario) string {
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 4. МЕТОДЫ И МЕРОПРИЯТИЯ\r\n")
	methods := alternativeScenarioMethodsForCalculation(scenario)
	fmt.Fprintf(&b, "Вариант: %s\r\nСтатус: %s\r\nДоступность: %s\r\nКомментарий по доступности: %s\r\nМетоды / мероприятия: %s\r\nУтверждённая классификация: %s\r\n",
		dash(scenario.Name), dash(scenario.Status), dash(scenario.Availability), dash(scenario.AvailabilityNote), firstNonEmpty(strings.Join(methods, " + "), "не выбраны"), dash(scenario.Classification))

	decision, decisionOK := latestApprovedMethodDecision(objectKey)
	// RC734: the bundled teaching reports must stay deterministic and fully
	// populated even when an older sparse synthetic decision is present in the
	// user's data directory. Production decisions still use the persisted record.
	if _, teaching := teachingFullScenarioForObject(objectKey); teaching {
		summary, _, _ := approvedAssessmentForMethods(objectKey)
		decision, decisionOK = teachingActiveMethodDecision(objectKey, summary)
	}
	if decisionOK {
		b.WriteString("\r\nУТВЕРЖДЁННОЕ РЕШЕНИЕ ПО МЕТОДАМ\r\n")
		fmt.Fprintf(&b, "ID решения: %s\r\nУтверждено: %s\r\nУтвердил: %s\r\nИсточник метода: %s\r\nСтатус метода: %s\r\n",
			dash(decision.ID), dash(decision.ApprovedAt), dash(decision.ApprovedBy), dash(decision.MethodSource), dash(decision.MethodStatus))
		if len(decision.MethodAssessments) > 0 {
			b.WriteString("\r\nПОДРОБНАЯ ОЦЕНКА КАЖДОГО МЕТОДА\r\n")
			for i, a := range decision.MethodAssessments {
				fmt.Fprintf(&b, "\r\nМЕТОД %d\r\n%s", i+1, reportMethodAssessmentDetail(a))
			}
		} else {
			for _, method := range methods {
				if a, ok := teachingMethodEffect(method); ok {
					b.WriteString("\r\n" + reportMethodAssessmentDetail(a))
				}
			}
		}
		if len(decision.Implementation) > 0 {
			b.WriteString("\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\n")
			for _, item := range decision.Implementation {
				b.WriteString(methodImplementationReportText(item))
			}
		} else {
			for _, method := range methods {
				item := methodImplementationSnapshotForName(objectKey, method)
				if len(item.Fields) > 0 {
					b.WriteString(methodImplementationReportText(item))
				}
			}
		}
		if len(decision.Plan) > 0 {
			b.WriteString("\r\nПЛАН ИСПОЛНЕНИЯ И КОНТРОЛЯ\r\n")
			commonTeachingNote := false
			for i, item := range decision.Plan {
				fmt.Fprintf(&b, "\r\nМЕРОПРИЯТИЕ %d. %s\r\n", i+1, dash(item.MethodName))
				fmt.Fprintf(&b, "Срок: %s; статус: %s.\r\n", dash(item.DueDate), dash(item.Status))
				fmt.Fprintf(&b, "Ответственный: %s.\r\n", dash(item.Responsible))
				note := strings.TrimSpace(item.ControlComment)
				if strings.Contains(strings.ToLower(note), "учебный план контроля") {
					commonTeachingNote = true
				} else if note != "" && note != "—" {
					b.WriteString("Контроль выполнения: " + strings.TrimSuffix(note, ".") + ".\r\n")
				}
			}
			if commonTeachingNote {
				b.WriteString("Условие ОПИ: подтвердить фактические параметры, совместимость реагентов и критерий контроля до начала работ.\r\n")
			}
		}
		if strings.TrimSpace(decision.Interaction) != "" {
			b.WriteString("\r\nВЗАИМОДЕЙСТВИЕ МЕТОДОВ\r\n" + strings.TrimSpace(decision.Interaction) + "\r\n")
		}
	}

	if strings.TrimSpace(scenario.SelectionBasis) != "" {
		basis := strings.TrimSpace(scenario.SelectionBasis)
		if strings.Contains(basis, "полностью обеспеченная оборудованием и материалами") {
			basis = "Наличие материалов и оборудования необходимо подтвердить до выдачи задания на реализацию."
		}
		b.WriteString("\r\nИТОГОВОЕ ОСНОВАНИЕ ВЫБОРА ВАРИАНТА\r\n" + basis + "\r\n")
	}
	b.WriteString("\r\nОГРАНИЧЕНИЕ\r\nПрименимость — скрининговый индекс, а не вероятность успеха. Конкретные дозировки, режимы и конструктивные решения требуют лабораторной/промысловой проверки и утверждения технологом; подбор ГНО выполняется в «Автотехнологе».\r\n")
	return b.String()
}

func reportSection5Runlife(objectKey string, scenario AlternativeMethodScenario) string {
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 5. НАРАБОТКА И ОТКАЗЫ\r\n")
	targetDays := scenario.TargetMRPDays
	targetSource := "целевой ориентир варианта"
	if targetDays <= 0 && methodsObjectKeysEquivalent(selectedObjectKey, objectKey) {
		target := targetRunlifeResolvedForSelectedObject()
		targetDays, targetSource = target.Days, target.Source
	}
	if targetDays <= 0 {
		targetDays = 1000
		targetSource = "рабочий целевой ориентир РОСА"
	}
	fmt.Fprintf(&b, "Базовая оценка без нового мероприятия: %.0f сут.\r\nЦелевой ориентир: %.0f сут.\r\nИсточник целевого ориентира: %s\r\nСдержанный сценарий: %.0f сут.\r\nРасчётный сценарий: %.0f сут.\r\nБлагоприятный сценарий: %.0f сут.\r\nОтклонение расчётного сценария от цели: %+.0f сут.\r\n",
		scenario.BaselineMRPDays, targetDays, dash(targetSource), scenario.ForecastLowDays, scenario.ForecastBaseDays, scenario.ForecastHighDays, scenario.ForecastBaseDays-targetDays)
	if scenario.BaselineMRPDays > 0 {
		fmt.Fprintf(&b, "Изменение к базовой оценке: сдержанный %+.0f сут. (%+.1f%%); расчётный %+.0f сут. (%+.1f%%); благоприятный %+.0f сут. (%+.1f%%).\r\n",
			scenario.ForecastLowDays-scenario.BaselineMRPDays, (scenario.ForecastLowDays/scenario.BaselineMRPDays-1)*100,
			scenario.ForecastBaseDays-scenario.BaselineMRPDays, (scenario.ForecastBaseDays/scenario.BaselineMRPDays-1)*100,
			scenario.ForecastHighDays-scenario.BaselineMRPDays, (scenario.ForecastHighDays/scenario.BaselineMRPDays-1)*100)
	}
	h := methodHistoryBasisForObject(objectKey, time.Now())
	fmt.Fprintf(&b, "\r\nПредыстория 36 мес.: отказов %d; завершённых циклов %d; уровень уверенности сценарной оценки: %s.\r\n", h.FailureCount, h.CycleCount, dash(h.Precision))
	if strings.TrimSpace(h.Warning) != "" {
		b.WriteString("Предупреждение: " + h.Warning + "\r\n")
	}
	if in, ok := mrpForecastInputForSelectedObject(); ok && methodsObjectKeysEquivalent(selectedObjectKey, objectKey) {
		r := calculateMRPForecast(in)
		s := r.Advanced.ObjectAny
		b.WriteString("\r\nРАСЧЁТ БЕЗОТКАЗНОЙ РАБОТЫ\r\n")
		if s.HasConditionalMedianMore {
			fmt.Fprintf(&b, "Остаточный ресурс P50: ещё %.0f сут.\r\n", s.ConditionalMedianMoreDays)
		}
		fmt.Fprintf(&b, "Вероятность продолжения работы ещё 90 / 180 / 365 сут.: %s / %s / %s.\r\n", formatProbability(s.Survival90More, s.HasSurvival90More), formatProbability(s.Survival180More, s.HasSurvival180More), formatProbability(s.Survival365More, s.HasSurvival365More))
		if r.Advanced.Competing.Has365 {
			fmt.Fprintf(&b, "Вероятность подтверждённого отказа к 365 сут.: %.0f%%.\r\n", r.Advanced.Competing.FailureCIF365*100)
		}
		fmt.Fprintf(&b, "Достаточность истории: %s; событий %d; цензурированных циклов %d.\r\n", r.EvidenceLevel, s.EventCount, s.CensoredCount)
		if len(r.Warnings) > 0 || len(r.Advanced.Warnings) > 0 {
			b.WriteString("\r\nОГРАНИЧЕНИЯ И ПРЕДУПРЕЖДЕНИЯ\r\n")
			warnings := append([]string{}, r.Warnings...)
			warnings = append(warnings, r.Advanced.Warnings...)
			for _, warning := range warnings {
				b.WriteString("• " + warning + "\r\n")
			}
		}
	}
	return b.String()
}

func reportSection6Economics(objectKey string, scenario AlternativeMethodScenario) string {
	return reportSection6EconomicsWithUI(objectKey, scenario, true)
}

func reportSection6EconomicsDetached(objectKey string, scenario AlternativeMethodScenario) string {
	return reportSection6EconomicsWithUI(objectKey, scenario, false)
}

func reportSection6EconomicsWithUI(objectKey string, scenario AlternativeMethodScenario, includeUI bool) string {
	_ = objectKey
	_ = includeUI
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 6. ЭКОНОМИЧЕСКАЯ ОЦЕНКА / BENEFIT MODELING\r\n")
	annualCost := scenario.OneTimeCAPEXRub + scenario.AnnualOPEXRub
	netBenefitRub := scenario.NetBenefitK * 1000
	grossBenefitRub := netBenefitRub + annualCost
	fmt.Fprintf(&b, "Вариант: %s\r\nСтатус: %s; дата расчёта: %s.\r\n", dash(scenario.Name), dash(scenario.EconomicsStatus), dash(scenario.EconomicsCalculated))
	fmt.Fprintf(&b, "CAPEX: %.2f млн руб.; OPEX: %.2f млн руб./год; затраты первого года: %.2f млн руб.\r\n", scenario.OneTimeCAPEXRub/1e6, scenario.AnnualOPEXRub/1e6, annualCost/1e6)
	fmt.Fprintf(&b, "Чистая выгода: %+.2f млн руб.; Benefit/Cost: %.2f; окупаемость: %s.\r\n", netBenefitRub/1e6, scenario.BenefitCostRatio, formatOptionalMonths(scenario.PaybackMonths))
	fmt.Fprintf(&b, "Расчётная трассировка: затраты = CAPEX + OPEX = %.2f млн руб.; валовая выгода = чистая выгода + затраты = %.2f млн руб.\r\n", annualCost/1e6, grossBenefitRub/1e6)
	b.WriteString("Примечание: экономика служит для сравнения вариантов и рассматривается вместе с технической применимостью и прогнозом наработки.\r\n")
	return b.String()
}

func reportSection7Final(objectKey string, scenario AlternativeMethodScenario, resultSnapshot string) string {
	_ = objectKey
	_ = resultSnapshot
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 7. ИТОГОВОЕ РЕШЕНИЕ\r\n")
	fmt.Fprintf(&b, "Выбранный вариант: %s\r\nСтатус решения: %s\r\n", dash(scenario.Name), dash(scenario.Status))
	return b.String()
}

func buildFullReportSectionSnapshot(objectKey string, scenario AlternativeMethodScenario, resultSnapshot string) map[string]string {
	sections := map[string]string{}
	sections["section_1"] = reportSection1FundAndFailures(objectKey)
	sections["section_2"] = reportSection2Laboratory(objectKey)
	sections["section_3"] = reportSection3Assessment(objectKey)
	sections["section_4"] = reportSection4Methods(objectKey, scenario)
	sections["section_5"] = reportSection5Runlife(objectKey, scenario)
	sections["section_6"] = reportSection6Economics(objectKey, scenario)
	sections["section_7"] = reportSection7Final(objectKey, scenario, resultSnapshot)
	return sections
}

func fullReportSection(r FullReportRecord, key string) string {
	if r.Sections == nil {
		return ""
	}
	return strings.TrimSpace(r.Sections[key])
}

func reportLaboratoryKeyInputs(pass LabPassRecord) string {
	keyOrder := map[string][]string{
		"scale":        {"mineralization", "ph", "sample_temp", "ca", "mg", "na_k", "na", "k", "so4", "co3", "hco3", "cl", "ba", "sr"},
		"corrosion":    {"ph", "chlorides", "co2", "h2s", "o2", "corrosion_rate", "corrosion_rate_without", "corrosion_rate_with", "inhibitor_residual"},
		"aspo":         {"paraffins", "asphaltenes", "resins", "viscosity", "pour_point", "watercut"},
		"solids":       {"suspended", "sand", "turbidity"},
		"microbiology": {"srb", "apb", "iron_bacteria", "qpcr_srb"},
		"gases":        {"co2", "h2s", "o2", "gas_factor", "sample_temp", "hydrate_temp"},
	}
	var b strings.Builder
	for _, code := range []string{"scale", "corrosion", "aspo", "solids", "microbiology", "gases"} {
		values := pass.Groups[code]
		if len(values) == 0 {
			continue
		}
		group, ok := labGroupByCode(code)
		if !ok {
			continue
		}
		defs := map[string]LabFieldDef{}
		for _, field := range group.Fields {
			defs[field.Key] = field
		}
		var lines []string
		for _, key := range keyOrder[code] {
			value := strings.TrimSpace(values[key])
			if value == "" {
				continue
			}
			field, exists := defs[key]
			if !exists {
				continue
			}
			unit := strings.TrimSpace(pass.Units[code][key])
			if unit == "" {
				unit = field.Unit
			}
			lines = append(lines, reportLaboratoryIndicatorLine(field.Label, value, unit))
		}
		if len(lines) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(group.Name) + "\r\n")
		for _, line := range lines {
			b.WriteString(line)
		}
	}
	return b.String()
}

func compactLaboratorySectionForExport(text string) string {
	pass := reportLabSnapshotInputs(text)
	var b strings.Builder
	b.WriteString("РАЗДЕЛ 2. ЛАБОРАТОРНЫЕ ДАННЫЕ\r\n")
	prefixes := []string{"Идентификатор анализа:", "Дата анализа:", "Источник / протокол:", "Автор:", "Статус:", "Модель солеотложений:"}
	seen := map[string]bool{}
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		for _, prefix := range prefixes {
			if strings.HasPrefix(line, prefix) && !seen[prefix] {
				b.WriteString(line + "\r\n")
				seen[prefix] = true
			}
		}
	}
	if key := strings.TrimSpace(reportLaboratoryKeyInputs(pass)); key != "" {
		b.WriteString("\r\nКЛЮЧЕВЫЕ ИСХОДНЫЕ ДАННЫЕ\r\n" + key + "\r\n")
	}
	b.WriteString("\r\nРАСЧЁТНЫЕ ПОКАЗАТЕЛИ\r\n" + reportLaboratoryCalculatedSummary(pass))

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	conclusion := ""
	for _, heading := range []string{"ВЫВОД ДЛЯ ТЕХНОЛОГА", "ГЛАВНОЕ ДЛЯ ТЕХНОЛОГА И ХИМИКА"} {
		if pos := strings.Index(normalized, heading); pos >= 0 {
			conclusion = strings.TrimSpace(normalized[pos+len(heading):])
			break
		}
	}
	if conclusion != "" {
		for _, stop := range []string{"\nИсточник: ", "\nРАСЧЁТЫ, ФОРМУЛЫ", "\nРАСЧЁТНЫЕ ПОКАЗАТЕЛИ"} {
			if pos := strings.Index(conclusion, stop); pos >= 0 {
				conclusion = strings.TrimSpace(conclusion[:pos])
			}
		}
	}
	// Reconstruct the comment from the same frozen measurements. The old
	// stored conclusion remains unchanged in the archive, but repeated generic
	// six-direction phrases do not belong in the reader-facing report.
	if len(pass.Groups) > 0 {
		conclusion = strings.TrimSpace(technologistReportSection(pass))
	} else if conclusion == "" {
		conclusion = "Нужны исходные числовые показатели и протокол анализа для предметной рекомендации."
	}
	if conclusion != "" {
		b.WriteString("\r\nВЫВОД ДЛЯ ТЕХНОЛОГА\r\n" + conclusion + "\r\n")
	}
	return reportCleanUserText(b.String())
}

func compactStoredSectionForExport(index int, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	// Historical section snapshots are immutable. Compact them only in the
	// export presentation so old approvals remain auditable while current PDF/
	// DOCX output does not repeat large calculation narratives.
	if index == 2 {
		return compactLaboratorySectionForExport(text)
	}

	if index == 4 {
		// Forecast/history belong to section 5. Old approved snapshots can contain
		// the same forecast inside Methods, so suppress only those presentation
		// lines during export without touching the immutable archived record.
		var kept []string
		for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "Источник прогноза:") || strings.HasPrefix(line, "Предыстория 36 мес.:") ||
				strings.HasPrefix(line, "Прогноз наработки ПОСЛЕ мероприятия:") || strings.HasPrefix(line, "Предварительный вклад комплекса") ||
				strings.HasPrefix(line, "Ограничение прогноза:") {
				continue
			}
			kept = append(kept, raw)
		}
		text = strings.Join(kept, "\r\n")

		// These historical encyclopaedic blocks duplicate the selected-method
		// assessment and the final basis already printed in the same section.
		for _, heading := range []string{"ИНЖЕНЕРНЫЕ КАРТОЧКИ ВЫБРАННЫХ МЕТОДОВ", "ПАРАЛЛЕЛЬНЫЕ ВАРИАНТЫ ДЛЯ СРАВНЕНИЯ"} {
			if start := strings.Index(text, heading); start >= 0 {
				end := len(text)
				for _, next := range []string{"ИТОГОВОЕ ОСНОВАНИЕ ВЫБОРА ВАРИАНТА", "ОГРАНИЧЕНИЕ"} {
					if pos := strings.Index(text[start+len(heading):], next); pos >= 0 {
						candidate := start + len(heading) + pos
						if candidate < end {
							end = candidate
						}
					}
				}
				text = strings.TrimSpace(text[:start]) + "\r\n\r\n" + strings.TrimSpace(text[end:])
			}
		}
		text = reportNormalizeHistoricalPlan(text)
		text = reportAddChemicalOverview(text)
	}
	if index == 6 {
		if start := strings.Index(text, "РАСЧЁТНАЯ ЛОГИКА"); start >= 0 {
			if pos := strings.Index(text[start:], "ПОДСТАНОВКА ЧИСЕЛ ДЛЯ ВЫБРАННОГО ВАРИАНТА"); pos >= 0 {
				text = strings.TrimSpace(text[:start]) + "\r\n\r\n" + strings.TrimSpace(text[start+pos:])
			}
		}
		if start := strings.Index(text, "СОПОСТАВЛЕНИЕ С ТЕХНИЧЕСКИМ РЕЗУЛЬТАТОМ"); start >= 0 {
			end := len(text)
			if pos := strings.Index(text[start:], "Примечание:"); pos >= 0 {
				end = start + pos
			}
			text = strings.TrimSpace(text[:start]) + "\r\n\r\n" + strings.TrimSpace(text[end:])
		}
	}
	if index == 7 {
		if before, _, found := strings.Cut(text, "ПОЛНАЯ СВОДКА РАЗДЕЛА 7"); found {
			text = strings.TrimSpace(before)
		}
		// The final section confirms the chosen decision; methods, forecast,
		// benefit and selection basis are already explained in sections 4-6.
		var concise []string
		for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "РАЗДЕЛ 7.") || strings.HasPrefix(line, "Выбранный вариант:") || strings.HasPrefix(line, "Статус:") || strings.HasPrefix(line, "Статус решения:") {
				concise = append(concise, raw)
			}
		}
		if len(concise) > 0 {
			text = strings.Join(concise, "\r\n")
		}
	}
	return reportCleanUserText(text)
}

func storedSectionDetailedReport(r FullReportRecord, approved bool) string {
	if len(r.Sections) == 0 {
		return ""
	}
	var b strings.Builder
	title := "РОСА — СОХРАНЁННЫЙ ПОЛНЫЙ ОТЧЁТ"
	if approved || strings.TrimSpace(r.ApprovedAt) != "" {
		title = "РОСА — УТВЕРЖДЁННЫЙ ПОЛНЫЙ ОТЧЁТ"
	}
	b.WriteString(title + "\r\n")
	fmt.Fprintf(&b, "Отчёт: %s\r\nОбъект: %s\r\nВариант: %s\r\nДата отчёта: %s\r\n", r.ID, r.ObjectLabel, r.ScenarioName, r.CreatedAt)
	if strings.TrimSpace(r.ApprovedAt) != "" {
		fmt.Fprintf(&b, "Утверждено: %s\r\nУтвердил: %s\r\n", firstNonEmpty(strings.TrimSpace(r.ApprovedAt), r.CreatedAt), firstNonEmpty(strings.TrimSpace(r.Author), "технолог"))
	} else {
		fmt.Fprintf(&b, "Автор: %s\r\n", firstNonEmpty(strings.TrimSpace(r.Author), "технолог"))
	}
	fmt.Fprintf(&b, "Статус: %s\r\n", firstNonEmpty(strings.TrimSpace(r.DecisionStatus), "Рассчитано"))
	b.WriteString("\r\nРАЗДЕЛЫ 1–7\r\n")
	for i := 1; i <= 7; i++ {
		key := fmt.Sprintf("section_%d", i)
		text := compactStoredSectionForExport(i, fullReportSection(r, key))
		if text == "" {
			fmt.Fprintf(&b, "\r\nРАЗДЕЛ %d\r\nДанные раздела в этом историческом снимке отсутствуют.\r\n", i)
			continue
		}
		b.WriteString("\r\n" + text + "\r\n")
	}
	b.WriteString("\r\n" + reportMissingLaboratoryFields(fullReportSection(r, "section_2")))
	return reportCleanUserText(b.String())
}

func reportMissingLaboratoryFields(section string) string {
	var b strings.Builder
	b.WriteString("ПРИЛОЖЕНИЕ 1. НЕЗАПОЛНЕННЫЕ ПОКАЗАТЕЛИ ЛАБОРАТОРНОГО ПАСПОРТА\r\n")
	if section == "" {
		b.WriteString("Лабораторный паспорт не был включён в утверждённый снимок.\r\n")
		return b.String()
	}
	model := "davies"
	current := ""
	seen := map[string]bool{}
	hasRawGroup := false
	for _, line := range strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Модель солеотложений: ") {
			model = strings.TrimSpace(strings.TrimPrefix(line, "Модель солеотложений: "))
		}
		if strings.HasPrefix(line, "ПАСПОРТ КАЧЕСТВА И ПРОСЛЕЖИВАЕМОСТИ") || strings.HasPrefix(line, "РАСЧЁТЫ, ФОРМУЛЫ") {
			break
		}
		if strings.HasPrefix(line, "НАПРАВЛЕНИЕ: ") {
			current = ""
			for _, group := range labGroups {
				if strings.EqualFold(group.Name, strings.TrimPrefix(line, "НАПРАВЛЕНИЕ: ")) {
					current = group.Code
					hasRawGroup = true
					break
				}
			}
			continue
		}
		if current == "" || !strings.HasPrefix(line, "• ") {
			continue
		}
		group, ok := labGroupByCode(current)
		if !ok {
			continue
		}
		for _, field := range laboratoryScaleInputGroup(group, model).Fields {
			if strings.HasPrefix(line, "• "+field.Label+": ") {
				seen[current+"/"+field.Key] = true
				break
			}
		}
	}
	if !hasRawGroup {
		b.WriteString("Перечень незаполненных полей для этой исторической записи не зафиксирован.\r\n")
		return b.String()
	}
	count := 0
	for _, base := range labGroups {
		if base.Code == "overall" {
			continue
		}
		var missing strings.Builder
		for _, field := range laboratoryScaleInputGroup(base, model).Fields {
			if seen[base.Code+"/"+field.Key] {
				continue
			}
			unit := ""
			if field.Unit != "" && field.Unit != "текст" && field.Unit != "расчёт" {
				unit = " (" + field.Unit + ")"
			}
			missing.WriteString("• " + field.Label + unit + "\r\n")
			count++
		}
		if missing.Len() > 0 {
			b.WriteString("\r\n" + strings.ToUpper(base.Name) + "\r\n" + missing.String())
		}
	}
	if count == 0 {
		b.WriteString("Незаполненных полей в выбранной лабораторной модели нет.\r\n")
	}
	return b.String()
}

func fullReportSectionsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func fullReportWorkbookSummary(r FullReportRecord, approved bool) string {
	var b strings.Builder
	title := "РОСА — СОХРАНЁННЫЙ ПОЛНЫЙ ОТЧЁТ"
	if approved || strings.TrimSpace(r.ApprovedAt) != "" {
		title = "РОСА — УТВЕРЖДЁННЫЙ ПОЛНЫЙ ОТЧЁТ"
	}
	b.WriteString(title + "\r\n")
	fmt.Fprintf(&b, "Отчёт: %s\r\nОбъект: %s\r\nВариант: %s\r\nДата отчёта: %s\r\n", r.ID, r.ObjectLabel, r.ScenarioName, r.CreatedAt)
	if strings.TrimSpace(r.ApprovedAt) != "" {
		fmt.Fprintf(&b, "Утверждено: %s\r\nУтвердил: %s\r\n", firstNonEmpty(strings.TrimSpace(r.ApprovedAt), r.CreatedAt), firstNonEmpty(strings.TrimSpace(r.Author), "технолог"))
	} else {
		fmt.Fprintf(&b, "Автор: %s\r\n", firstNonEmpty(strings.TrimSpace(r.Author), "технолог"))
	}
	fmt.Fprintf(&b, "Статус: %s\r\nМетоды / мероприятия: %s\r\nДоступность: %s\r\nПрогноз: %s сут.\r\nЧистая выгода: %s млн руб.\r\nОснование выбора: %s\r\n",
		firstNonEmpty(strings.TrimSpace(r.DecisionStatus), "Рассчитано"), firstNonEmpty(strings.TrimSpace(r.Methods), "не указаны"), firstNonEmpty(strings.TrimSpace(r.Availability), "не указана"), reportDisplayNumber(r.ForecastDays), reportDisplayNumber(r.NetBenefitK/1000), firstNonEmpty(strings.TrimSpace(r.DecisionBasis), "не указано"))
	b.WriteString("\r\nПодробные числовые расчёты, коэффициенты, единицы и Excel-формулы находятся на листе «Расчёты».\r\n")
	return reportCleanUserText(b.String())
}

func fullReportXLSXLaboratorySummary(section string) string {
	lines := strings.Split(strings.ReplaceAll(section, "\r\n", "\n"), "\n")
	var out []string
	keepBody := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "РАЗДЕЛ 2.") || strings.HasPrefix(line, "Дата анализа:") || strings.HasPrefix(line, "Источник / протокол:") || strings.HasPrefix(line, "Модель солеотложений:") {
			out = append(out, line)
			continue
		}
		if line == "ВЫВОД ДЛЯ ТЕХНОЛОГА" || line == "ГЛАВНОЕ ДЛЯ ТЕХНОЛОГА И ХИМИКА" {
			keepBody = true
			out = append(out, line)
			continue
		}
		if strings.HasPrefix(line, "РАСЧЁТЫ, ФОРМУЛЫ") {
			break
		}
		if keepBody {
			out = append(out, line)
		}
	}
	out = append(out, "Расчётные показатели, коэффициенты, исходные числа и формулы вынесены на лист «Расчёты».")
	return strings.Join(out, "\r\n")
}

func fullReportXLSXSummarySheetXML(r FullReportRecord, approved bool) string {
	return fullReportXLSXSheetXML(fullReportWorkbookSummary(r, approved))
}

func fullReportXLSXSheetXML(report string) string {
	report = reportCleanUserText(report)
	lines := strings.Split(strings.ReplaceAll(report, "\r\n", "\n"), "\n")
	var rows strings.Builder
	for i, line := range lines {
		row := i + 1
		style := 0
		reportStyle := laboratoryReportLineStyle(line, i)
		switch reportStyle {
		case reportStyleTitle:
			style = 1
		case reportStyleSection, reportStyleTechnology, reportStyleTechnologist:
			style = 2
		case reportStyleLabel:
			style = 3
		}
		label, value, unit := line, "", ""
		if strings.HasPrefix(line, "• ") {
			if colon := strings.LastIndex(line, ": "); colon > 0 {
				label, value = line[:colon+1], strings.TrimSpace(line[colon+1:])
				if split := strings.Index(value, "\u00a0"); split >= 0 {
					value, unit = value[:split], strings.ReplaceAll(value[split+len("\u00a0"):], "\u00a0", " ")
				}
			}
		} else if colon := strings.Index(line, ": "); colon > 0 && colon < 80 {
			label, value = line[:colon+1], strings.TrimSpace(line[colon+1:])
		}
		height := 18 * maxInt(1, maxInt((len([]rune(value))+65)/66, (len([]rune(label))+51)/52))
		if style == 1 {
			height = maxInt(height, 24)
		}
		if height > 409 {
			height = 409
		}
		rows.WriteString(fmt.Sprintf(`<row r="%d" ht="%d" customHeight="1">%s%s%s</row>`, row, height,
			xlsxInlineCell(fmt.Sprintf("A%d", row), label, style),
			xlsxInlineCell(fmt.Sprintf("B%d", row), value, 0),
			xlsxInlineCell(fmt.Sprintf("C%d", row), unit, 0)))
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="54" customWidth="1"/><col min="2" max="2" width="68" customWidth="1"/><col min="3" max="3" width="20" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData></worksheet>`
}

func fullReportXLSXStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><numFmts count="3"><numFmt numFmtId="164" formatCode="0.###"/><numFmt numFmtId="165" formatCode="0.000E+00"/><numFmt numFmtId="166" formatCode="0.0%"/></numFmts><fonts count="6">` +
		`<font><name val="Times New Roman"/><sz val="12"/></font>` +
		`<font><b/><name val="Times New Roman"/><sz val="12"/></font>` +
		`<font><b/><name val="Times New Roman"/><sz val="12"/><color rgb="FF153C6A"/></font>` +
		`<font><b/><i/><name val="Times New Roman"/><sz val="12"/></font>` +
		`<font><b/><name val="Times New Roman"/><sz val="14"/><color rgb="FFFFFFFF"/></font>` +
		`<font><b/><name val="Times New Roman"/><sz val="12"/><color rgb="FFFFFFFF"/></font></fonts>` +
		`<fills count="5"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFFFF2D4"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FF153C6A"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEDF2F8"/></patternFill></fill></fills>` +
		`<borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="11">` +
		`<xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="2" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="3" fillId="0" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="164" fontId="0" fillId="2" borderId="0" xfId="0" applyNumberFormat="1"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="4" fillId="3" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="5" fillId="3" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="0" fontId="2" fillId="4" borderId="0" xfId="0"><alignment wrapText="1" vertical="center"/></xf>` +
		`<xf numFmtId="164" fontId="1" fillId="2" borderId="0" xfId="0" applyNumberFormat="1"><alignment horizontal="right" vertical="center"/></xf>` +
		`<xf numFmtId="165" fontId="1" fillId="2" borderId="0" xfId="0" applyNumberFormat="1"><alignment horizontal="right" vertical="center"/></xf>` +
		`<xf numFmtId="166" fontId="1" fillId="2" borderId="0" xfId="0" applyNumberFormat="1"><alignment horizontal="right" vertical="center"/></xf>` +
		`</cellXfs></styleSheet>`
}
func writeFullReportSectionXLSX(path string, r FullReportRecord, approved bool) error {
	if len(r.Sections) == 0 {
		return writeLaboratoryXLSXReport(path, firstNonEmpty(storedSectionDetailedReport(r, approved), r.Snapshot))
	}
	names := []string{"Сводка", "1 Фонд", "2 Лаборатория", "3 Осложнения", "4 Методы", "5 Наработка", "6 Экономика", "7 Итог", "Расчёты", "Приложение 1"}
	reports := []string{fullReportWorkbookSummary(r, approved)}
	for i := 1; i <= 7; i++ {
		section := fullReportSection(r, fmt.Sprintf("section_%d", i))
		if i == 2 {
			if before, _, found := strings.Cut(section, "РАСЧЁТЫ, ФОРМУЛЫ, ПОДСТАНОВКА И РЕЗУЛЬТАТЫ"); found {
				section = strings.TrimSpace(before) + "\r\n\r\nЧисловые формулы и исходные величины — на листе «Расчёты»."
			}
		}
		reports = append(reports, section)
	}
	reports = append(reports, "", reportMissingLaboratoryFields(fullReportSection(r, "section_2")))

	var contentTypes strings.Builder
	contentTypes.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	var sheets, rels strings.Builder
	rels.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	files := map[string]string{}
	for i, name := range names {
		id := i + 1
		sheetPath := fmt.Sprintf("xl/worksheets/sheet%d.xml", id)
		contentTypes.WriteString(fmt.Sprintf(`<Override PartName="/%s" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, sheetPath))
		sheets.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, reportXMLText(name), id, id))
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, id, id))
		if i == 0 {
			files[sheetPath] = fullReportXLSXSummarySheetXML(r, approved)
		} else if i == 2 {
			files[sheetPath] = fullReportLaboratoryDashboardXML(r)
		} else if i == 5 {
			files[sheetPath] = reportScenarioTabXML(r)
		} else if i == 6 {
			files[sheetPath] = reportEconomyTabXML(r)
		} else if i == len(names)-2 {
			files[sheetPath] = fullReportCalculationSheetXML(r)
		} else {
			files[sheetPath] = fullReportXLSXSheetXML(reports[i])
		}
	}
	styleRelID := len(names) + 1
	rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, styleRelID))
	rels.WriteString(`</Relationships>`)
	contentTypes.WriteString(`</Types>`)
	files["[Content_Types].xml"] = contentTypes.String()
	files["_rels/.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	files["xl/workbook.xml"] = `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` + sheets.String() + `</sheets><calcPr calcId="191029" fullCalcOnLoad="1" forceFullCalc="1"/></workbook>`
	files["xl/_rels/workbook.xml.rels"] = rels.String()
	files["xl/styles.xml"] = fullReportXLSXStylesXML()
	return writeReportZip(path, files)
}
