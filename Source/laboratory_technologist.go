//go:build windows

package main

import (
	"fmt"
	"strings"
)

func laboratoryTechnologistHeading(line string) bool {
	line = strings.TrimSpace(line)
	return strings.EqualFold(line, "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА") || strings.EqualFold(line, "ЧТО ЭТО ОЗНАЧАЕТ ДЛЯ ТЕХНОЛОГА")
}

func laboratoryTechnologistRanges(display string) []laboratoryTextRange {
	display = strings.ReplaceAll(strings.ReplaceAll(display, "\r\n", "\n"), "\r", "\n")
	var ranges []laboratoryTextRange
	offset := 0
	for _, part := range strings.SplitAfter(display, "\n") {
		line := strings.TrimSuffix(part, "\n")
		if laboratoryTechnologistHeading(line) {
			leading := len(line) - len(strings.TrimLeft(line, " \t"))
			start := offset + laboratoryUTF16Len(line[:leading])
			ranges = append(ranges, laboratoryTextRange{start, start + laboratoryUTF16Len(strings.TrimSpace(line))})
		}
		offset += laboratoryUTF16Len(part)
	}
	return ranges
}

func laboratoryPlainSection(text, label string, nextLabels ...string) string {
	start := strings.Index(text, label)
	if start < 0 {
		return ""
	}
	value := strings.TrimSpace(text[start+len(label):])
	for _, next := range nextLabels {
		if pos := strings.Index(value, "\n\n"+next); pos >= 0 {
			value = strings.TrimSpace(value[:pos])
		}
	}
	return value
}

func laboratoryTechnologistCannotAssert(groupCode string) string {
	items := map[string]string{
		"corrosion":    "По одной пробе нельзя назвать единственную причину повреждения, перенести скорость коррозии на всю скважину, рассчитать срок до отказа или назначить дозировку ингибитора.",
		"scale":        "Расчётная пересыщенность не доказывает наличие фактического осадка. По этому выводу нельзя определить скорость и массу отложений, срок до отказа, МРП или дозировку ингибитора.",
		"aspo":         "Содержание парафинов, смол и асфальтенов само по себе не доказывает образование АСПО в конкретной точке. По этому выводу нельзя выбрать растворитель, температуру прогрева или периодичность обработки без испытаний.",
		"solids":       "Количество и размер частиц не определяют их происхождение. Нельзя считать мехпримеси песком, продуктами коррозии или солями без анализа состава и сопоставления с оборудованием.",
		"microbiology": "Обнаружение микроорганизмов или их ДНК не доказывает, что клетки жизнеспособны, активны и являются причиной повреждения. По одной ПЦР нельзя назначать биоцид и его дозу.",
		"gases":        "Наличие газа, воды или расчётной температуры гидратов не доказывает образование гидратной пробки или газовое влияние на насос. Для вывода нужны совместные температура, давление, состав и фактическое поведение оборудования.",
	}
	if text := strings.TrimSpace(items[groupCode]); text != "" {
		return text
	}
	return "Предварительная оценка не подтверждает факт осложнения, его скорость развития, срок до отказа или эффективность конкретного мероприятия без дополнительных данных."
}

func laboratoryTechnologistNextAction(group LabGroupDef) string {
	plain := laboratoryPlainLanguageSummary(group, "")
	next := laboratoryPlainSection(plain, "Что проверить первым:", "Как принять решение:", "Как понять, помогли ли меры:")
	if next == "" {
		return "Сверить исходные данные, место и время отбора, режим работы объекта и подтвердить ключевой признак профильным анализом."
	}
	return next
}

func laboratoryPassTechnologistSummary(pass LabPassRecord) string {
	var lines, missing []string
	lines = append(lines,
		"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА",
		"Ниже — свод по текущей лабораторной диагностике. Он показывает, где есть признаки, где данных недостаточно и какое действие логично выполнить следующим. Это не сумма процентов риска и не подтверждение факта осложнения.",
		"СВОД ПО НАПРАВЛЕНИЯМ",
	)
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := canonicalizeLaboratoryValues(group, pass.Groups[group.Code], pass.Units[group.Code])
		validated, _, errors := validateLaboratoryGroup(group, values)
		filled := laboratoryLabValueCount(group, validated) > 0
		if !filled && len(errors) == 0 {
			missing = append(missing, group.Name)
			continue
		}
		base, total, _, _ := laboratoryDataCoverageForModel(group, validated, pass.ScaleModel)
		assessment := laboratoryGroupSummary(group, values, pass.ScaleModel)
		status := assessment.Message
		if len(errors) > 0 {
			status += " Есть ошибки ввода; сначала исправьте их."
		}
		lines = append(lines, fmt.Sprintf("• %s — %s Базовый уровень: %d/%d. Следующее действие: %s", group.Name, status, base, total, laboratoryTechnologistNextAction(group)))
		if group.Code == "scale" && pass.ScaleModel == "water_mixing" {
			// Keep the full canonical mixing block once because it contains the
			// mineral-by-mineral compatibility result and explicit missing analyses.
			lines = append(lines, "СОВМЕСТИМОСТЬ ДВУХ ВОД В ОБЩЕМ ВЫВОДЕ\n"+laboratoryWaterMixingText(values))
		}
	}
	if len(missing) > 0 {
		lines = append(lines, "Не заполнены направления: "+strings.Join(missing, ", ")+". Их отсутствие не означает низкий риск; по ним вывод не сформирован.")
	}
	lines = append(lines,
		"КОМПЛЕКСНАЯ ИНЖЕНЕРНАЯ ИНТЕРПРЕТАЦИЯ",
		"Если признаки появляются сразу в нескольких направлениях, сначала сопоставьте их с одним режимом и временем отбора. Твёрдые частицы могут быть продуктами коррозии или солевым осадком; сульфиды могут сопровождать поступление H₂S и микробиологический фактор. Совпадение признаков показывает, что версии нужно проверять совместно, но само по себе не доказывает причинную связь.",
		"Первый общий шаг: проверить объект, дату, точку отбора и ошибки ввода, затем сопоставить ключевые признаки с техрежимом, состоянием оборудования и историей ремонтов. После этого назначать профильное подтверждение — состав отложений, контроль металла, повторный анализ воды/газа или микробиологическое исследование по соответствующему механизму.",
		"В итоговой записи фиксируйте предполагаемую причину, чем она подтверждается, какие данные ещё нужны, выбранное действие и показатель контроля до/после. Этот свод не рассчитывает дозировки реагентов, срок до отказа или дату следующего ремонта.",
	)
	return strings.Join(lines, "\n\n")
}

func laboratoryRecordPracticalExplanation(group LabGroupDef, record LabRecord) string {
	group = laboratoryScaleInputGroup(group, record.ScaleModel)
	values := canonicalizeLaboratoryValues(group, record.Values, record.Units)
	assessment := laboratoryGroupSummary(group, values, record.ScaleModel)
	text := laboratoryPracticalExplanation(group, assessment)
	if group.Code == "scale" && record.ScaleModel == "water_mixing" {
		text += "\n\nСОВМЕСТИМОСТЬ ДВУХ ВОД ИЗ СОХРАНЁННОЙ ЗАПИСИ\n" + laboratoryWaterMixingText(values)
	}
	return text
}

// laboratoryHistoryConclusion deliberately rebuilds the visible conclusion from
// the saved source values instead of reusing a text blob created by an older
// application version. This keeps history review in the same four-part
// technologist language as a fresh calculation without mutating the source row.
func laboratoryHistoryConclusion(pass LabPassRecord, preferredGroup string) string {
	if preferredGroup == "overall" {
		return buildLaboratoryPassConclusion(pass)
	}
	group, ok := labGroupByCode(preferredGroup)
	if !ok || group.Code == "overall" || !laboratoryPassHasGroupData(pass, preferredGroup) {
		return buildLaboratoryPassConclusion(pass)
	}
	group = laboratoryScaleInputGroup(group, pass.ScaleModel)
	values := canonicalizeLaboratoryValues(group, pass.Groups[group.Code], pass.Units[group.Code])
	return buildLaboratoryConclusionWithScaleModel(group, values, pass.ScaleModel)
}
