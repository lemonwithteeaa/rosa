//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

type CorrosionInfluenceBlock struct {
	Name       string
	Level      string
	Confidence string
	Basis      []string
	Mechanism  string
	Missing    []string
	Conclusion string
	rank       int
	detected   bool
}

type CorrosionInfluenceAssessment struct {
	Oxygen           CorrosionInfluenceBlock
	CarbonDioxide    CorrosionInfluenceBlock
	HydrogenSulfide  CorrosionInfluenceBlock
	Combined         CorrosionInfluenceBlock
	CommonContext    []string
	ObservedEvidence []string
	Values           map[string]string
}

func corrosionLevel(rank int) string {
	switch rank {
	case 1:
		return "низкий по имеющимся данным"
	case 2:
		return "умеренный"
	case 3:
		return "повышенный"
	case 4:
		return "высокий"
	default:
		return "не определён: недостаточно данных"
	}
}

func corrosionValue(values map[string]string, key, label, unit string) (float64, string, bool) {
	n, ok := parseLabNumber(values[key])
	if !ok {
		return 0, "", false
	}
	return n, fmt.Sprintf("%s = %g %s", label, n, unit), true
}

func corrosionConfidence(source string, contextCount int) string {
	if source == "" {
		return "недостаточная"
	}
	if source == "газовая доля" {
		return "средняя: нет прямого результата по жидкости или парциальному давлению"
	}
	if contextCount >= 4 {
		return "повышенная для качественного скрининга"
	}
	return "средняя: часть условий эксплуатации не задана"
}

func assessGasBlock(values map[string]string, gas string, contextCount int) CorrosionInfluenceBlock {
	var block CorrosionInfluenceBlock
	var source string
	var value float64
	var present bool
	switch gas {
	case "o2":
		block.Name = "Кислород O₂"
		block.Mechanism = "Кислород поддерживает катодную реакцию и может усиливать общую и локальную коррозию, особенно при наличии водной фазы."
		if n, text, ok := corrosionValue(values, "o2", "растворённый O₂", "мг/л"); ok {
			value, present, source = n, true, "растворённая концентрация"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		}
		if !present {
			block.Missing = append(block.Missing, "растворённый O₂")
		}
	case "co2":
		block.Name = "Диоксид углерода CO₂"
		block.Mechanism = "В водной фазе CO₂ снижает pH и поддерживает углекислотную коррозию; характер защитных карбонатных плёнок зависит от температуры, состава воды и потока."
		if n, text, ok := corrosionValue(values, "pco2", "pCO₂", "МПа"); ok {
			value, present, source = n, true, "парциальное давление"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		} else if n, text, ok := corrosionValue(values, "co2", "растворённый CO₂", "мг/л"); ok {
			value, present, source = n, true, "растворённая концентрация"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		} else if n, text, ok := corrosionValue(values, "co2_gas", "CO₂ в газовой фазе", "% об."); ok {
			value, present, source = n, true, "газовая доля"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		}
		for _, item := range []struct{ key, label, unit string }{{"pco2", "pCO₂", "МПа"}, {"co2", "растворённый CO₂", "мг/л"}, {"co2_gas", "CO₂ в газовой фазе", "% об."}} {
			if _, text, ok := corrosionValue(values, item.key, item.label, item.unit); ok && !strings.Contains(strings.Join(block.Basis, " "), text) {
				block.Basis = append(block.Basis, text+" — справочно, повторно в уровень не включён")
			}
		}
		if !present {
			block.Missing = append(block.Missing, "pCO₂ либо растворённый CO₂")
		}
	case "h2s":
		block.Name = "Сероводород H₂S"
		block.Mechanism = "H₂S участвует в сульфидной коррозии и образовании FeS; для чувствительных материалов отдельно требуется проверка риска сульфидного растрескивания."
		if n, text, ok := corrosionValue(values, "ph2s", "pH₂S", "МПа"); ok {
			value, present, source = n, true, "парциальное давление"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		} else if n, text, ok := corrosionValue(values, "h2s", "растворённый H₂S", "мг/л"); ok {
			value, present, source = n, true, "растворённая концентрация"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		} else if n, text, ok := corrosionValue(values, "h2s_gas", "H₂S в газовой фазе", "% об."); ok {
			value, present, source = n, true, "газовая доля"
			block.Basis = append(block.Basis, text+" — основной учтённый показатель")
		}
		for _, item := range []struct{ key, label, unit string }{{"ph2s", "pH₂S", "МПа"}, {"h2s", "растворённый H₂S", "мг/л"}, {"h2s_gas", "H₂S в газовой фазе", "% об."}} {
			if _, text, ok := corrosionValue(values, item.key, item.label, item.unit); ok && !strings.Contains(strings.Join(block.Basis, " "), text) {
				block.Basis = append(block.Basis, text+" — справочно, повторно в уровень не включён")
			}
		}
		if !present {
			block.Missing = append(block.Missing, "pH₂S либо растворённый H₂S")
		}
	}

	if !present {
		block.Level = corrosionLevel(0)
		block.Confidence = "недостаточная"
		block.Conclusion = "Влияние не может быть исключено: профильный показатель не задан."
		return block
	}
	block.detected = value > 0
	block.rank = 1
	if value > 0 {
		block.rank = 2
	}
	if gas == "o2" && value > 0.1 || gas == "co2" && ((source == "парциальное давление" && value > 0.2) || (source == "растворённая концентрация" && value > 50) || (source == "газовая доля" && value > 1)) || gas == "h2s" && value > 0 {
		block.rank = 3
	}
	if gas == "co2" && ((source == "парциальное давление" && value > 1) || (source == "растворённая концентрация" && value > 500) || (source == "газовая доля" && value > 10)) {
		block.rank = 4
	}
	block.Level = corrosionLevel(block.rank)
	block.Confidence = corrosionConfidence(source, contextCount)
	if value == 0 {
		block.Conclusion = "По введённому результату газ не обнаружен; вывод относится только к данной пробе и пределу обнаружения метода."
	} else {
		block.Conclusion = "Газовый фактор присутствует; итоговую значимость следует подтверждать по скорости коррозии, пластинкам и производственной динамике."
	}
	return block
}

func corrosionCommonContext(values map[string]string) []string {
	items := []struct{ key, label, unit string }{
		{"ph", "pH", "ед."}, {"sample_temp", "температура", "°C"}, {"bottomhole_pressure", "забойное давление (Р)", "МПа"},
		{"watercut", "обводнённость", "%"}, {"chlorides", "хлориды", "мг/л"}, {"mineralization", "минерализация", "г/л"},
		{"orp", "ОВП", "мВ"}, {"iron_total", "железо общее", "мг/л"}, {"fe2", "Fe²⁺", "мг/л"}, {"fe3", "Fe³⁺", "мг/л"},
		{"flow_velocity", "скорость потока", "м/с"}, {"inhibitor_residual", "остаточная концентрация ингибитора", "мг/л"},
	}
	var context []string
	for _, item := range items {
		if _, text, ok := corrosionValue(values, item.key, item.label, item.unit); ok {
			context = append(context, text)
		}
	}
	if material := strings.TrimSpace(values["equipment_material"]); material != "" {
		context = append(context, "материал оборудования = "+material)
	}
	return context
}

func corrosionMissingContext(values map[string]string) []string {
	checks := []struct {
		keys  []string
		label string
	}{
		{[]string{"ph"}, "pH"}, {[]string{"sample_temp"}, "температура"},
		{[]string{"bottomhole_pressure"}, "забойное давление (Р)"}, {[]string{"watercut"}, "обводнённость"},
		{[]string{"chlorides", "mineralization"}, "хлориды или минерализация"},
		{[]string{"orp"}, "ОВП"}, {[]string{"iron_total", "fe2", "fe3"}, "железо"},
		{[]string{"flow_velocity"}, "скорость потока"}, {[]string{"equipment_material"}, "материал оборудования"},
		{[]string{"inhibitor_residual", "corrosion_rate_without", "corrosion_rate_with"}, "данные об ингибиторной защите"},
	}
	var missing []string
	for _, check := range checks {
		found := false
		for _, key := range check.keys {
			if strings.TrimSpace(values[key]) != "" {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, check.label)
		}
	}
	return missing
}

func corrosionObservedEvidence(values map[string]string) []string {
	var evidence []string
	if _, text, ok := corrosionValue(values, "corrosion_rate", "измеренная скорость коррозии", "мм/год"); ok {
		evidence = append(evidence, text)
	}
	loss, lossOK := parseLabNumber(values["plate_mass_loss"])
	initial, initialOK := parseLabNumber(values["plate_initial_mass"])
	final, finalOK := parseLabNumber(values["plate_final_mass"])
	if !lossOK && initialOK && finalOK && initial >= final {
		loss, lossOK = initial-final, true
	}
	if lossOK {
		text := fmt.Sprintf("потеря массы пластинки = %g г", loss)
		days, daysOK := parseLabNumber(values["plate_exposure_days"])
		area, areaOK := parseLabNumber(values["plate_area"])
		density, densityOK := parseLabNumber(values["plate_material_density"])
		if daysOK {
			text += fmt.Sprintf(" за %g сут", days)
		}
		if areaOK {
			text += fmt.Sprintf(" при площади %g см²", area)
		}
		if daysOK && areaOK && densityOK && days > 0 && area > 0 && density > 0 {
			rate := 87.6 * loss * 1000 / (density * area * days * 24)
			text += fmt.Sprintf("; средняя скорость по потере массы = %g мм/год", rate)
		} else {
			text += "; для скорости нужны положительные площадь, экспозиция и плотность материала"
		}
		evidence = append(evidence, text)
	}
	without, withoutOK := parseLabNumber(values["corrosion_rate_without"])
	with, withOK := parseLabNumber(values["corrosion_rate_with"])
	if withoutOK && withOK {
		if without > 0 {
			effect := (without - with) / without * 100
			evidence = append(evidence, fmt.Sprintf("эффект ингибиторной защиты по парному испытанию = %g %% (без ингибитора %g; с ингибитором %g мм/год)", effect, without, with))
		} else {
			evidence = append(evidence, "парное испытание ингибитора задано, но эффективность не вычисляется при нулевой скорости без ингибитора")
		}
	}
	if allowance, ok := parseLabNumber(values["corrosion_allowance"]); ok {
		evidence = append(evidence, fmt.Sprintf("припуск на коррозию = %g мм; ресурс без толщинометрии и минимально допустимой толщины не рассчитывается", allowance))
	}
	if actual, actualOK := parseLabNumber(values["wall_thickness"]); actualOK {
		if minimum, minimumOK := parseLabNumber(values["minimum_wall_thickness"]); minimumOK {
			evidence = append(evidence, fmt.Sprintf("запас толщины = %g мм (фактическая %g; минимально допустимая %g мм)", actual-minimum, actual, minimum))
		}
	}
	if depth, ok := parseLabNumber(values["pit_depth"]); ok {
		if years, yearsOK := parseLabNumber(values["pit_period_years"]); yearsOK && years > 0 {
			evidence = append(evidence, fmt.Sprintf("ориентировочная локальная скорость по язве = %g мм/год", depth/years))
		}
	}
	return evidence
}

func formatH2SCrackingAssessment(values map[string]string) string {
	inputs := []struct{ key, label string }{
		{"equipment_material", "марка и состояние материала"}, {"material_yield_strength", "предел текучести"},
		{"material_hardness_base", "твёрдость основного металла"}, {"material_hardness_weld", "твёрдость сварного шва / ЗТВ"},
		{"working_stresses", "рабочие и остаточные напряжения"}, {"ssc_test", "испытание на сульфидное коррозионное растрескивание (SSC)"},
		{"hic_test", "испытание на водородоиндуцированное растрескивание (HIC)"}, {"hydrogen_permeation", "водородопроницаемость"},
		{"material_zone_conclusion", "заключение материаловеда"},
	}
	var present, missing []string
	for _, item := range inputs {
		if value := strings.TrimSpace(values[item.key]); value != "" {
			if (item.key == "material_hardness_base" || item.key == "material_hardness_weld") && ambiguousHardnessValue(value) {
				missing = append(missing, item.label+" (число введено без указания шкалы твёрдости (HV/HRC/HB))")
				continue
			}
			present = append(present, item.label+" = "+value)
		} else {
			missing = append(missing, item.label)
		}
	}
	h2sKnown, h2sPositive := false, false
	for _, key := range []string{"ph2s", "h2s", "h2s_gas"} {
		if value, ok := parseLabNumber(values[key]); ok {
			h2sKnown = true
			if value > 0 {
				h2sPositive = true
			}
		}
	}
	level := "не определена"
	conclusion := "Нужны данные H₂S и материала."
	if h2sKnown && !h2sPositive {
		level = "по введённой пробе H₂S не обнаружен"
		conclusion = "Результат не исключает иной режим или локальную H₂S-среду; учитывайте предел обнаружения и историю объекта."
	} else if h2sPositive {
		level = "требуется материаловедческая проверка"
		conclusion = "Положительный H₂S является основанием проверить сульфидное коррозионное растрескивание (SSC), водородоиндуцированное растрескивание (HIC) и ступенчатое водородное растрескивание (SOHIC), но приложение не присваивает автоматически зону пригодности материала."
	}
	if len(present) == 0 {
		present = []string{"дополнительные материаловедческие данные не внесены"}
	}
	if len(missing) == 0 {
		missing = []string{"критичные пробелы в перечне полей не выявлены; проверить сопоставимость методов и условий"}
	}
	return "СКРИНИНГ ВЕРОЯТНОСТИ РАСТРЕСКИВАНИЯ В H₂S-СРЕДЕ\n" +
		"Предварительный уровень: " + level + ".\n" +
		"Материаловедческие данные: " + strings.Join(present, "; ") + ".\n" +
		"Недостаёт: " + strings.Join(missing, "; ") + ".\n" +
		"Вывод: " + conclusion
}

func ambiguousHardnessValue(value string) bool {
	value = strings.TrimSpace(value)
	upper := strings.ToUpper(value)
	_, numericOnly := parseLabNumber(value)
	return numericOnly && !strings.Contains(upper, "HV") && !strings.Contains(upper, "HRC") && !strings.Contains(upper, "HB")
}

func assessCorrosionInfluence(values map[string]string) CorrosionInfluenceAssessment {
	context := corrosionCommonContext(values)
	missingContext := corrosionMissingContext(values)
	a := CorrosionInfluenceAssessment{CommonContext: context, ObservedEvidence: corrosionObservedEvidence(values), Values: values}
	a.Oxygen = assessGasBlock(values, "o2", len(context))
	a.CarbonDioxide = assessGasBlock(values, "co2", len(context))
	a.HydrogenSulfide = assessGasBlock(values, "h2s", len(context))
	blocks := []*CorrosionInfluenceBlock{&a.Oxygen, &a.CarbonDioxide, &a.HydrogenSulfide}
	active, maxRank := 0, 0
	var names []string
	for _, block := range blocks {
		if block.detected {
			active++
			names = append(names, block.Name)
		}
		if block.rank > maxRank {
			maxRank = block.rank
		}
	}
	a.Combined = CorrosionInfluenceBlock{Name: "Совместное влияние O₂ + CO₂ + H₂S", Mechanism: "Совместное действие определяется не суммой трёх независимых баллов, а наиболее выраженным газовым фактором и наличием межфакторного взаимодействия."}
	if active == 0 {
		a.Combined.Level = corrosionLevel(0)
		a.Combined.Confidence = "недостаточная"
		a.Combined.Missing = append([]string{"положительное измерение хотя бы одного из O₂, CO₂ или H₂S"}, missingContext...)
		a.Combined.Conclusion = "Совместное влияние не оценено: газы не заданы либо введены как нулевые."
	} else {
		a.Combined.rank = maxRank
		if active >= 2 && a.Combined.rank < 4 {
			a.Combined.rank++
		}
		a.Combined.Level = corrosionLevel(a.Combined.rank)
		a.Combined.Basis = []string{"обнаружены: " + strings.Join(names, ", ")}
		if active >= 2 {
			a.Combined.Basis = append(a.Combined.Basis, "межфакторное взаимодействие учтено один раз повышением уровня на одну ступень")
			a.Combined.Conclusion = "Возможен синергетический либо конкурирующий характер плёнкообразования; требуется подтверждение пластинками, анализом продуктов коррозии и динамикой промысловых данных."
		} else {
			a.Combined.Conclusion = "Совместный эффект не повышался: положительный результат имеется только по одному газовому фактору."
		}
		a.Combined.Confidence = "средняя для качественного скрининга"
		a.Combined.Missing = missingContext
		if len(missingContext) == 0 && len(a.ObservedEvidence) > 0 {
			a.Combined.Confidence = "повышенная для качественного скрининга"
		}
	}
	return a
}

func formatCorrosionBlock(block CorrosionInfluenceBlock) string {
	basis := "нет"
	if len(block.Basis) > 0 {
		basis = strings.Join(block.Basis, "; ")
	}
	missing := "нет критичных пробелов для качественного скрининга"
	if len(block.Missing) > 0 {
		missing = strings.Join(block.Missing, "; ")
	}
	return fmt.Sprintf("%s\nУровень влияния: %s. Достоверность: %s.\nУчтённые данные: %s.\nМеханизм: %s\nНедостаёт: %s.\nВывод: %s", strings.ToUpper(block.Name), block.Level, block.Confidence, basis, block.Mechanism, missing, block.Conclusion)
}

func formatCorrosionInfluenceAssessment(a CorrosionInfluenceAssessment) string {
	context := append([]string(nil), a.CommonContext...)
	sort.Strings(context)
	contextText := "не заданы"
	if len(context) > 0 {
		contextText = strings.Join(context, "; ")
	}
	evidenceText := "не заданы; нужны скорость коррозии и/или результаты пластинок"
	if len(a.ObservedEvidence) > 0 {
		evidenceText = strings.Join(a.ObservedEvidence, "; ")
	}
	return "РАСШИРЕННАЯ ОЦЕНКА ВЛИЯНИЯ НА КОРРОЗИЮ\n" +
		"Общие условия (pH, температура, давление, водная фаза, соли, ОВП, железо, поток, материал и ингибитор) учтены один раз в совместной интерпретации и не суммируются повторно в газовых уровнях.\n" +
		"Общие условия: " + contextText + ".\n\n" +
		formatCorrosionBlock(a.Oxygen) + "\n\n" +
		formatCorrosionBlock(a.CarbonDioxide) + "\n\n" +
		formatCorrosionBlock(a.HydrogenSulfide) + "\n\n" +
		formatH2SCrackingAssessment(a.Values) + "\n\n" +
		formatCorrosionBlock(a.Combined) + "\n\n" +
		"ОЦЕНКА КОРРОЗИОННЫХ ПЛАСТИНОК И ИНГИБИТОРНОЙ ЗАЩИТЫ\n" + evidenceText + ".\n" +
		"Ограничение: это объяснимый качественный скрининг, а не расчёт нормативной скорости коррозии или пригодности материала к H₂S-среде."
}

func corrosionEnvironmentCompactText(values map[string]string) string {
	a := assessCorrosionInfluence(values)
	active := []string{}
	for _, b := range []CorrosionInfluenceBlock{a.Oxygen, a.CarbonDioxide, a.HydrogenSulfide} {
		if b.detected {
			active = append(active, b.Name+": "+b.Level)
		}
	}
	activeText := "положительные O₂/CO₂/H₂S по введённым данным не подтверждены"
	if len(active) > 0 {
		activeText = strings.Join(active, "; ")
	}
	missing := corrosionMissingContext(values)
	missingText := "ключевой контекст заполнен"
	if len(missing) > 0 {
		shown := missing
		if len(shown) > 5 {
			shown = shown[:5]
		}
		missingText = "нужно дополнить: " + strings.Join(shown, ", ")
		if len(missing) > len(shown) {
			missingText += fmt.Sprintf(" (+%d)", len(missing)-len(shown))
		}
	}
	return "КОРРОЗИОННАЯ СРЕДА\n" +
		"Газовые механизмы: " + activeText + ".\n" +
		"Совместная оценка: " + a.Combined.Level + "; " + missingText + ".\n" +
		"Общие условия учитываются один раз; газовые факторы не суммируются как независимые баллы."
}
