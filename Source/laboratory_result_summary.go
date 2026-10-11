//go:build windows

package main

import (
	"fmt"
	"strings"
)

const laboratorySummaryPrefix = "Итог для технолога: "

type laboratoryScreeningCheck struct {
	key, label string
	risky      func(float64) bool
}

func laboratoryScreeningChecks() map[string][]laboratoryScreeningCheck {
	return map[string][]laboratoryScreeningCheck{
		"corrosion":    {{"ph", "низкий pH", func(v float64) bool { return v < 6.5 }}, {"chlorides", "высокие хлориды", func(v float64) bool { return v > 10000 }}, {"h2s", "H₂S", func(v float64) bool { return v > 0 }}, {"corrosion_rate", "повышенная скорость коррозии", func(v float64) bool { return v > .1 }}},
		"aspo":         {{"paraffins", "парафины", func(v float64) bool { return v > 3 }}, {"asphaltenes", "асфальтены", func(v float64) bool { return v > 1 }}, {"pour_point", "температура застывания", func(v float64) bool { return v > 15 }}, {"viscosity", "вязкость", func(v float64) bool { return v > 50 }}},
		"solids":       {{"suspended", "взвешенные вещества", func(v float64) bool { return v > 100 }}, {"sand", "песок", func(v float64) bool { return v > .1 }}, {"size_d50", "крупная фракция d50", func(v float64) bool { return v > 50 }}, {"turbidity", "мутность", func(v float64) bool { return v > 50 }}},
		"microbiology": {{"gmb", "общее микробное число", func(v float64) bool { return v > 1000 }}, {"sulfide", "сульфиды", func(v float64) bool { return v > 0 }}},
		"gases":        {{"h2s", "H₂S", func(v float64) bool { return v > 0 }}, {"co2", "CO₂", func(v float64) bool { return v > 50 }}, {"water_content", "влагосодержание", func(v float64) bool { return v > .1 }}},
	}
}

type laboratoryResultSummary struct {
	Level                 int
	HasData               bool
	BaseFilled, BaseTotal int
	Message, State        string
}

func (s laboratoryResultSummary) Text() string { return laboratorySummaryPrefix + s.Message }

// Uses existing screening limits and the selected calculation's results.
// It never assigns a technological complication class or a failure probability.
func laboratoryGroupSummary(group LabGroupDef, values map[string]string, modelCode string) laboratoryResultSummary {
	group = laboratoryScaleInputGroup(group, modelCode)
	clean, _, errors := validateLaboratoryGroup(group, values)
	base, total, _, _ := laboratoryDataCoverageForModel(group, clean, modelCode)
	result := laboratoryResultSummary{BaseFilled: base, BaseTotal: total, State: "нет данных"}
	result.HasData = laboratoryLabValueCount(group, values) > 0
	if !result.HasData {
		result.Message = "Оценить пока нельзя: лабораторные данные не внесены."
		return result
	}
	evaluated, signals := 0, 0
	ready := total > 0 && base == total && len(errors) == 0
	var reasons []string
	borderline := false
	calculationReason := ""
	if group.Code == "scale" {
		if modelCode == "co2_gas_water" {
			co2, err := laboratoryCO2EquilibriumFromValues(clean)
			result.State = "НЕТ ОЦЕНКИ"
			if err != nil {
				result.Message = "CO₂-равновесие не рассчитано: " + err.Error() + ". Это не означает, что солеотложения нет."
			} else {
				result.Message = fmt.Sprintf("CO₂-равновесие рассчитано: %s, pH %.4f. Этот специальный режим не рассчитывает насыщение минералов, поэтому уровень солеотложения по нему не назначается.", laboratoryCO2EquilibriumStatusLabel(co2.Status), co2.PH)
			}
			return result
		}
		model := laboratoryScaleModelByCode(modelCode)
		minerals, _, calculationWarnings := laboratorySaturationResults(clean, model.Code)
		if len(minerals) == 0 && len(calculationWarnings) > 0 {
			calculationReason = calculationWarnings[0]
		}
		if !model.Implemented {
			calculationReason = "Выбранная модель «" + model.Name + "» численно не реализована полностью."
		}
		if !model.Implemented && !laboratoryScaleSixComponentReady(clean) {
			minerals = nil
		}
		evaluated = len(minerals)
		ready = len(errors) == 0 && model.Implemented && len(minerals) > 0
		for _, warning := range calculationWarnings {
			if strings.HasPrefix(warning, "Невязка баланса зарядов") || strings.HasPrefix(warning, "Ионная сила выше") || strings.HasPrefix(warning, "Константы текущего") || model.Code == "water_mixing" && strings.HasPrefix(warning, "Не хватает анализа") {
				ready = false
				calculationReason = warning
			}
		}
		if model.Code != "multicomplex" && model.Code != "multicomplex_molal" && model.Code != "carbonate_complex" && model.Code != "water_mixing" {
			ready = ready && laboratoryScaleSixComponentReady(clean)
		}
		for _, mineral := range minerals {
			if mineral.SI > 0.1 {
				signals++
				reasons = append(reasons, fmt.Sprintf("%s: индекс насыщения %.2f — есть условие для выпадения этой соли", mineral.Mineral, mineral.SI))
			} else if mineral.SI >= -0.1 {
				borderline = true
				reasons = append(reasons, fmt.Sprintf("%s: индекс насыщения %.2f — около границы насыщения", mineral.Mineral, mineral.SI))
			}
		}
	} else {
		for _, check := range laboratoryScreeningChecks()[group.Code] {
			if value, ok := parseLabNumber(clean[check.key]); ok {
				evaluated++
				if check.risky(value) {
					signals++
					unit := ""
					for _, field := range group.Fields {
						if field.Key == check.key {
							unit = field.Unit
							break
						}
					}
					reasons = append(reasons, fmt.Sprintf("%s: %g %s", check.label, value, unit))
				}
			}
		}
		if group.Code == "corrosion" {
			influence := assessCorrosionInfluence(clean)
			if influence.Combined.rank >= 2 && influence.Combined.rank > result.Level {
				result.Level = influence.Combined.rank
				if result.Level > 3 {
					result.Level = 3
				}
			}
			for _, gas := range []CorrosionInfluenceBlock{influence.Oxygen, influence.CarbonDioxide, influence.HydrogenSulfide} {
				if gas.rank > 0 {
					evaluated++
				}
				if gas.rank >= 2 {
					reasons = append(reasons, gas.Name+": "+gas.Level)
					if result.Level < gas.rank {
						result.Level = gas.rank
						if result.Level > 3 {
							result.Level = 3
						}
					}
				}
			}
		}
		if group.Code == "gases" && strings.TrimSpace(clean["hydrate_temp"]) != "" {
			ready = false
		}
		if group.Code == "microbiology" {
			// Qualitative results need expert interpretation; DNA is not proof of activity.
			for _, key := range []string{"srb", "apb", "biofilm", "iron_bacteria", "qpcr_srb", "qpcr_apb"} {
				text := strings.ToLower(strings.TrimSpace(clean[key]))
				if text != "" && text != "не обнаружены" && text != "не обнаружено" && text != "отрицательно" && text != "отрицательный" && text != "нет" && text != "0" {
					ready = false
				}
			}
		}
	}
	if signals >= 2 && result.Level < 3 {
		result.Level = 3
	} else if (signals > 0 || borderline) && result.Level < 2 {
		result.Level = 2
	}
	switch {
	case result.Level >= 2:
		result.State = "УМЕРЕННОЕ"
		if result.Level >= 3 {
			result.State = "ПОВЫШЕННОЕ"
		}
		result.Message = "Есть признаки возможного осложнения. "
		if signals == 0 && borderline && result.Level == 2 {
			result.Message = "Пограничный результат. "
		}
		result.Message += strings.Join(reasons, "; ") + ". Это основание для проверки, а не подтверждённый факт повреждения."
		if !ready {
			result.Message += " Данные или условия оценки требуют уточнения; найденные признаки при этом не отменяются."
		}
	case evaluated > 0 && ready:
		result.Level, result.State = 1, "НИЗКОЕ*"
		result.Message = "По внесённым данным признаки не выявлены. Это не исключает осложнение при другом составе, режиме или по неучтённым причинам."
	default:
		result.State = "НЕТ ОЦЕНКИ"
		result.Message = "Оценить пока нельзя: данных недостаточно, результат требует интерпретации или выбранный расчёт не выполнен. Это не означает, что осложнения нет."
	}
	if calculationReason != "" {
		result.Message += " Причина уточнения: " + calculationReason
	}
	if len(errors) > 0 {
		result.Message += " Исправьте ошибки ввода: " + strings.Join(errors, "; ") + ". Ошибочные значения исключены из расчёта."
	}
	return result
}

func laboratorySummaryShortMessage(message string) string {
	for _, prefix := range []string{"Есть признаки возможного осложнения.", "Пограничный результат.", "По внесённым данным признаки не выявлены."} {
		if strings.HasPrefix(message, prefix) {
			return prefix
		}
	}
	return "Оценить пока нельзя; требуется уточнение данных."
}

func laboratoryOverallShortText(text string) string {
	summaries := map[string]string{}
	known := map[string]bool{}
	for _, group := range labGroups {
		if group.Code != "overall" {
			known[strings.ToUpper(group.Name)] = true
		}
	}
	active := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "РАЗДЕЛ: ") {
			active = strings.TrimSpace(strings.TrimPrefix(line, "РАЗДЕЛ: "))
			if !known[active] {
				active = ""
			} else {
				if _, exists := summaries[active]; !exists {
					summaries[active] = ""
				}
			}
		} else if line == "ОБЩИЙ ВЫВОД" || line == "СВОД ОБЩИХ ПОЛЕЙ" || strings.HasPrefix(line, "НАПРАВЛЕНИЕ:") {
			active = ""
		} else if active != "" && strings.HasPrefix(line, laboratorySummaryPrefix) {
			summaries[active] = strings.TrimPrefix(line, laboratorySummaryPrefix)
		}
	}
	if len(summaries) == 0 {
		return "Краткий вывод: общая оценка пока не сформирована. Нажмите для пояснения."
	}
	risk, unknown := 0, 0
	for _, message := range summaries {
		short := laboratorySummaryShortMessage(message)
		if strings.HasPrefix(short, "Есть признаки") || strings.HasPrefix(short, "Пограничный") {
			risk++
		} else if strings.HasPrefix(short, "Оценить") {
			unknown++
		}
	}
	return fmt.Sprintf("Краткий вывод: учтено %d из 6 направлений; признаки или пограничный результат — %d; оценить пока нельзя — %d. Нажмите для полного текста.", len(summaries), risk, unknown)
}

func laboratoryTechnologistMechanism(groupCode string) string {
	items := map[string]string{
		"corrosion":    "Коррозионно-активная среда поддерживает электрохимическое разрушение металла. pH, хлориды, кислород, CO₂ и H₂S влияют на механизм по-разному, поэтому их рассматривают вместе с условиями эксплуатации и состоянием металла.",
		"scale":        "При изменении состава воды, температуры, давления или при смешении вод растворённые компоненты могут достигать насыщения и образовывать твёрдую минеральную фазу. Расчёт показывает термодинамическую предпосылку для конкретных минералов.",
		"aspo":         "При охлаждении и изменении условий нефти парафины могут кристаллизоваться, а смолы и асфальтены — участвовать в формировании органических отложений. Существенны не только концентрации, но и температурный профиль и свойства нефти.",
		"solids":       "Твёрдые частицы в потоке могут поступать из пласта, образовываться при коррозии или солеотложении либо оставаться после технологических операций. Их влияние определяется количеством, размером, твёрдостью и происхождением.",
		"microbiology": "Микроорганизмы и биоплёнки могут изменять локальную химическую среду, образовывать сульфиды и органические продукты и тем самым участвовать в загрязнении и локальном повреждении металла.",
		"gases":        "Растворённый и свободный газ влияет на свойства потока и работу оборудования; при определённом сочетании состава, температуры и давления возможны гидраты. H₂S и CO₂ одновременно важны для отдельной оценки коррозионного воздействия.",
	}
	if text := strings.TrimSpace(items[groupCode]); text != "" {
		return text
	}
	return "Лабораторные показатели используются для предварительной проверки возможного механизма осложнения и должны сопоставляться с режимом работы объекта."
}

func laboratoryTechnologistDataMeaning(message string) string {
	text := strings.TrimSpace(message)
	// The four-part technologist block has a dedicated limitations paragraph.
	// Remove stock caveats from the evidence paragraph so the same warning is not
	// repeated twice while preserving the actual result, reasons and data-quality notes.
	for _, redundant := range []string{
		" Это основание для проверки, а не подтверждённый факт повреждения.",
		" Это не исключает осложнение при другом составе, режиме или по неучтённым причинам.",
		" Это не означает, что осложнения нет.",
		" Это не означает, что солеотложения нет.",
	} {
		text = strings.ReplaceAll(text, redundant, "")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "По текущему набору данных содержательный результат пока не сформирован."
	}
	return text
}

func laboratoryPracticalExplanation(group LabGroupDef, summary laboratoryResultSummary) string {
	return strings.Join([]string{
		"ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА",
		"Что происходит: " + laboratoryTechnologistMechanism(group.Code),
		"Что означают именно эти данные: " + laboratoryTechnologistDataMeaning(summary.Message),
		"Чего пока нельзя утверждать: " + laboratoryTechnologistCannotAssert(group.Code),
		"Что проверить следующим действием: " + laboratoryTechnologistNextAction(group),
	}, "\n\n")
}

// laboratoryInputFieldEligible is the single gate used by readiness, history,
// short/general conclusions and exported coverage counters. Calculated ROSA
// derivatives and Tech Regime context are useful evidence, but are not manual
// laboratory inputs and therefore must never inflate X/Y coverage.
func laboratoryInputFieldEligible(groupCode, key string) bool {
	return !laboratoryCalculatedField(groupCode, key) && !laboratoryTechRegimeKey(groupCode, key)
}

func laboratoryLabValueCount(group LabGroupDef, values map[string]string) int {
	count := 0
	for _, field := range group.Fields {
		if !laboratoryInputFieldEligible(group.Code, field.Key) {
			continue
		}
		if strings.TrimSpace(values[field.Key]) != "" {
			count++
		}
	}
	return count
}
