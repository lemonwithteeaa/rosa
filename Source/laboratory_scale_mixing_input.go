//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

var laboratoryMixingChemistryKeys = []string{"ca_total", "mg_total", "tic", "so4_total", "na", "k", "cl", "ba", "sr", "solute_mass"}

func laboratoryMixingInputKeys() []string {
	keys := append([]string{}, laboratoryMixingChemistryKeys...)
	for _, key := range laboratoryMixingChemistryKeys {
		keys = append(keys, "mix2_"+key)
	}
	return append(keys, "brine_density", "mix2_brine_density", "mix_percent", "mix_ph", "mix_temp", "mix_ph_temp", "sample_temp", "mix2_sample_temp", "mix1_source", "mix2_source")
}

func laboratoryMixingRequiredKeys() map[string]bool {
	required := map[string]bool{}
	for _, key := range laboratoryMixingInputKeys() {
		if key == "ba" || key == "sr" || key == "mix2_ba" || key == "mix2_sr" || key == "mix1_source" || key == "mix2_source" {
			continue
		}
		required[key] = true
	}
	return required
}

// Return copied field definitions so a model switch never mutates the catalog.
func laboratoryScaleInputGroup(group LabGroupDef, modelCode string) LabGroupDef {
	if group.Code != "scale" {
		return group
	}
	out := group
	out.Fields = nil
	if modelCode == "co2_gas_water" {
		for _, field := range group.Fields {
			if laboratoryIsCO2EquilibriumField(field.Key) {
				out.Fields = append(out.Fields, field)
			}
		}
		return out
	}
	if modelCode != "water_mixing" {
		for _, field := range group.Fields {
			if !laboratoryIsMixingField(field.Key) && !laboratoryIsCO2EquilibriumField(field.Key) {
				out.Fields = append(out.Fields, field)
			}
		}
		return out
	}
	byKey := map[string]LabFieldDef{}
	for _, field := range group.Fields {
		byKey[field.Key] = field
	}
	for _, key := range laboratoryMixingInputKeys() {
		if field, ok := byKey[key]; ok {
			if key == "brine_density" {
				field.Label = "Плотность воды 1"
			}
			if key == "sample_temp" {
				field.Label = "Температура воды 1"
			}
			out.Fields = append(out.Fields, field)
		}
	}
	return out
}

func laboratoryDataCoverageForModel(group LabGroupDef, v map[string]string, modelCode string) (int, int, int, int) {
	group = laboratoryScaleInputGroup(group, modelCode)
	if group.Code == "scale" && modelCode == "co2_gas_water" {
		return laboratoryCO2EquilibriumCoverage(group, v)
	}
	if group.Code != "scale" || modelCode != "water_mixing" {
		return laboratoryDataCoverage(group, v)
	}
	required := laboratoryMixingRequiredKeys()
	filled, optional, requiredTotal, optionalTotal := 0, 0, 0, 0
	for _, field := range group.Fields {
		if !laboratoryInputFieldEligible(group.Code, field.Key) {
			continue
		}
		if required[field.Key] {
			requiredTotal++
			if strings.TrimSpace(v[field.Key]) != "" {
				filled++
			}
		} else {
			optionalTotal++
			if strings.TrimSpace(v[field.Key]) != "" {
				optional++
			}
		}
	}
	return filled, requiredTotal, optional, optionalTotal
}

func laboratoryDataCoverageTextForModel(group LabGroupDef, v map[string]string, modelCode string) string {
	if group.Code == "scale" && modelCode == "co2_gas_water" {
		return laboratoryCO2EquilibriumCoverageText(group, v)
	}
	if group.Code != "scale" || modelCode != "water_mixing" {
		return laboratoryDataCoverageText(laboratoryScaleInputGroup(group, modelCode), v)
	}
	filled, total, optional, optionalTotal := laboratoryDataCoverageForModel(group, v, modelCode)
	var missing []string
	for _, field := range group.Fields {
		if laboratoryMixingRequiredKeys()[field.Key] && strings.TrimSpace(v[field.Key]) == "" {
			missing = append(missing, field.Label)
		}
	}
	text := fmt.Sprintf("Обязательный базовый уровень смешения: %d из %d показателей. Дополнительные: %d из %d. Ba/Sr при отсутствии не считаются измеренным нулём: оценка соответствующих солей остаётся условной.", filled, total, optional, optionalTotal)
	if len(missing) > 0 {
		text += "\nНужно заполнить для расчёта смеси: " + strings.Join(missing, ", ") + ". Другие лабораторные разделы доступны независимо."
	}
	return text
}

func laboratoryFieldPrefixForModel(groupCode, key, modelCode string) string {
	if groupCode == "scale" && modelCode == "water_mixing" {
		if laboratoryMixingRequiredKeys()[key] {
			return "Б · "
		}
		return "Д · "
	}
	if groupCode == "scale" && modelCode == "co2_gas_water" {
		if laboratoryCO2EquilibriumRequiredKey(key) {
			return "Б · "
		}
		return "Д · "
	}
	return laboratoryFieldPrefix(groupCode, key)
}

type laboratoryWaterMixingResult struct {
	Solution                                                                       laboratoryMultiComplexSolution
	FirstWaterMass, SecondWaterMass, SecondVolumeFraction, SecondWaterMassFraction float64
	Totals                                                                         laboratoryMixTotals
	Na, K, Cl, ChargeBalance                                                       float64
	Warnings                                                                       []string
}

func laboratoryWaterMixingFromValues(v map[string]string) (laboratoryWaterMixingResult, error) {
	out := laboratoryWaterMixingResult{}
	for _, key := range []string{"sample_temp", "mix2_sample_temp", "mix_temp", "mix_ph_temp"} {
		value, ok := parseLabNumber(v[key])
		if !ok || math.Abs(value-25) > .001 {
			return out, fmt.Errorf("Для смешения нужны обе воды, смесь и измерение pH при 25 °C; проверьте поле %s", laboratoryMixingFieldLabel(key))
		}
	}
	fraction, ok := parseLabNumber(v["mix_percent"])
	if !ok || fraction <= 0 || fraction >= 100 {
		return out, fmt.Errorf("Доля воды 2 должна быть больше 0 и меньше 100 %% объёма до смешения; для одной воды выберите обычный моляльный расчёт")
	}
	ph, ok := parseLabNumber(v["mix_ph"])
	if !ok || ph < 0 || ph > 14 {
		return out, fmt.Errorf("Введите измеренный pH смеси 0–14 при 25 °C; pH исходных вод не усредняется")
	}
	first := map[string]string{}
	second := map[string]string{}
	for _, key := range append(append([]string{}, laboratoryMixingChemistryKeys...), "brine_density") {
		first[key] = v[key]
		second[key] = v["mix2_"+key]
	}
	firstNormalized, w1, err := laboratoryMolalityValues(first)
	if err != nil {
		return out, laboratoryMixingReadableError("Вода 1", err)
	}
	secondNormalized, w2, err := laboratoryMolalityValues(second)
	if err != nil {
		return out, laboratoryMixingReadableError("Вода 2", err)
	}
	readTotals := func(values map[string]string) (laboratoryMixTotals, float64, float64, float64, error) {
		amounts := map[string]float64{}
		for _, ion := range []struct {
			key  string
			mass float64
		}{{"ca_total", 40.078}, {"mg_total", 24.305}, {"tic", 12.0111}, {"so4_total", 96.06}, {"na", 22.99}, {"k", 39.0983}, {"cl", 35.45}, {"ba", 137.327}, {"sr", 87.62}} {
			if (ion.key == "ba" || ion.key == "sr") && strings.TrimSpace(values[ion.key]) == "" {
				continue
			}
			amount, ok := scaleIonMolar(values, ion.key, ion.mass)
			if !ok {
				return laboratoryMixTotals{}, 0, 0, 0, fmt.Errorf("некорректен общий анализ %s", ion.key)
			}
			amounts[ion.key] = amount
		}
		background := amounts["na"] + amounts["k"] + amounts["cl"]
		return laboratoryMixTotals{Ca: amounts["ca_total"], Mg: amounts["mg_total"], Ba: amounts["ba"], Sr: amounts["sr"], Carbon: amounts["tic"], Sulfate: amounts["so4_total"], Background: .5 * background, BackgroundSpecies: background}, amounts["na"], amounts["k"], amounts["cl"], nil
	}
	a, na1, k1, cl1, err := readTotals(firstNormalized)
	if err != nil {
		return out, laboratoryMixingReadableError("Вода 1", err)
	}
	b, na2, k2, cl2, err := readTotals(secondNormalized)
	if err != nil {
		return out, laboratoryMixingReadableError("Вода 2", err)
	}
	vf := fraction / 100
	// Volume fraction is specified before mixing. The equilibrium uses totals
	// per kg solvent water, so weight by solvent mass rather than solution volume.
	mf := vf * w2 / ((1-vf)*w1 + vf*w2)
	totals, err := laboratoryMixConservativeTotals(a, b, mf)
	if err != nil {
		return out, err
	}
	solution, err := laboratoryMultiComplexSolveMixedMolal(ph, a, b, mf)
	if err != nil {
		return out, err
	}
	out.Solution = solution
	out.FirstWaterMass = w1
	out.SecondWaterMass = w2
	out.SecondVolumeFraction = vf
	out.SecondWaterMassFraction = mf
	out.Totals = totals
	out.Na = (1-mf)*na1 + mf*na2
	out.K = (1-mf)*k1 + mf*k2
	out.Cl = (1-mf)*cl1 + mf*cl2
	s := solution
	c := 2*(s.Ca+s.Mg+s.Ba+s.Sr) + out.Na + out.K + s.Forms.Hydrogen + s.CaHCO3 + s.MgHCO3 + s.BaHCO3 + s.SrHCO3 + s.CaOH + s.MgOH + s.BaOH + s.SrOH
	an := 2*(s.SO4+s.MgSO42) + out.Cl + s.Forms.HCO3 + 2*s.Forms.CO3 + s.Forms.Hydroxide
	if c+an > 0 {
		out.ChargeBalance = 100 * (c - an) / (c + an)
	}
	if math.Abs(out.ChargeBalance) > 5 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("Невязка баланса зарядов смеси %+.2f %% превышает рабочий контроль 5 %%; сверить составы и измеренный pH смеси", out.ChargeBalance))
	}
	for _, key := range []string{"ba", "sr"} {
		if strings.TrimSpace(v[key]) == "" || strings.TrimSpace(v["mix2_"+key]) == "" {
			out.Warnings = append(out.Warnings, "Не хватает анализа "+strings.ToUpper(key)+" одной или обеих вод. Непредставленная доля элемента не учтена; отсутствие риска соответствующей соли не подтверждено.")
		}
	}
	return out, nil
}

func laboratoryMixingFieldLabel(key string) string {
	group, _ := labGroupByCode("scale")
	for _, field := range group.Fields {
		if field.Key == key {
			return field.Label
		}
	}
	return key
}

func laboratoryWaterMixingResults(v map[string]string) ([]laboratorySaturationResult, float64, []string) {
	m, err := laboratoryWaterMixingFromValues(v)
	if err != nil {
		return nil, 0, []string{err.Error()}
	}
	s := m.Solution
	var results []laboratorySaturationResult
	for _, mineral := range []struct {
		name, formula string
		si            float64
		present       bool
	}{
		{"Кальцит", "CaCO₃", s.CalciteSI, m.Totals.Ca > 0 && m.Totals.Carbon > 0},
		{"Гипс", "CaSO₄·2H₂O", s.GypsumSI, m.Totals.Ca > 0 && m.Totals.Sulfate > 0},
		{"Барит", "BaSO₄", s.BariteSI, m.Totals.Ba > 0 && m.Totals.Sulfate > 0},
		{"Целестин", "SrSO₄", s.CelestiteSI, m.Totals.Sr > 0 && m.Totals.Sulfate > 0},
	} {
		if mineral.present {
			k, _ := laboratoryTemperatureLogK(mineral.name, 25)
			results = append(results, laboratorySaturationResult{Mineral: mineral.name, Formula: mineral.formula, SI: mineral.si, Ratio: math.Pow(10, mineral.si), TemperatureC: 25, LogK: k})
		}
	}
	return results, s.I, m.Warnings
}

func laboratoryWaterMixingText(v map[string]string) string {
	m, err := laboratoryWaterMixingFromValues(v)
	if err != nil {
		return "СМЕШЕНИЕ ДВУХ ВОД\nРасчёт не выполнен: " + err.Error() + "\n\nЧто это значит для технолога\nПо этим данным совместимость вод оценить нельзя. Это не означает, что смешение безопасно. Исправьте указанное поле и повторите расчёт; не подбирайте дозировку реагента по незавершённой оценке."
	}
	state := fmt.Sprintf("СМЕШЕНИЕ ДВУХ ВОД\nВода 1: %.2f %% объёма до смешения; вода 2: %.2f %%. Источники анализов: %s / %s.\nИзмеренный pH смеси: %s; обе воды и измерение смеси при 25 °C.\nМасса воды-растворителя: вода 1 — %.9g кг на литр раствора; вода 2 — %.9g. Доля воды 2 по массе растворителя: %.6f %%.\nСмешанные общие Ca/Mg/Ba/Sr/C/S: %.9g / %.9g / %.9g / %.9g / %.9g / %.9g моль/кг воды.\nРасчётная активность воды: %.9g; ионная сила: %.9g моль/кг воды.\nНевязка баланса зарядов смеси: %+.2f %%.", 100*(1-m.SecondVolumeFraction), 100*m.SecondVolumeFraction, emptyLabText(v["mix1_source"]), emptyLabText(v["mix2_source"]), v["mix_ph"], m.FirstWaterMass, m.SecondWaterMass, 100*m.SecondWaterMassFraction, m.Totals.Ca, m.Totals.Mg, m.Totals.Ba, m.Totals.Sr, m.Totals.Carbon, m.Totals.Sulfate, m.Solution.WaterActivity, m.Solution.I, m.ChargeBalance)
	return state + "\n\n" + laboratoryWaterMixingTechnologistText(v, m)
}

func laboratoryMixingReadableError(water string, err error) error {
	text := err.Error()
	for _, key := range append(append([]string{}, laboratoryMixingChemistryKeys...), "brine_density") {
		text = strings.ReplaceAll(text, key, laboratoryMixingFieldLabel(key))
	}
	return fmt.Errorf("%s: %s", water, text)
}

func laboratoryIsMixingField(key string) bool {
	return strings.HasPrefix(key, "mix2_") || strings.HasPrefix(key, "mix_") || key == "mix1_source"
}

// Explain the same evaluated mixture; keep the existing SI decision boundaries.
func laboratoryWaterMixingTechnologistText(v map[string]string, m laboratoryWaterMixingResult) string {
	lines := []string{"Что это значит для технолога", "Вы проверяете, может ли при соединении именно этих двух вод и именно в указанной доле появиться твёрдая соль. Например, кальций одной воды может встретиться с сульфатами другой. Поэтому две воды, которые по отдельности не давали видимого осадка, могут образовать его после смешения.", "Как читать результат. Индекс насыщения (SI) показывает химическую возможность образования соли. При SI выше +0,1 смесь пересыщена: образование этой соли возможно. От −0,1 до +0,1 результат близок к равновесию и чувствителен к погрешности анализа. Ниже −0,1 пересыщение данной солью в рассчитанной смеси не выявлено. Эти границы — рабочее правило интерпретации, а не гарантия появления или отсутствия отложений."}
	minerals := []struct {
		name        string
		si          float64
		present     bool
		consequence string
	}{
		{"Кальцит (карбонат кальция)", m.Solution.CalciteSI, m.Totals.Ca > 0 && m.Totals.Carbon > 0, "проверить карбонатную составляющую отложений; при изменении pH и выделении газа результат может измениться"},
		{"Гипс (сульфат кальция)", m.Solution.GypsumSI, m.Totals.Ca > 0 && m.Totals.Sulfate > 0, "проверить совместное поступление кальция и сульфатов из разных источников воды"},
		{"Барит (сульфат бария)", m.Solution.BariteSI, m.Totals.Ba > 0 && m.Totals.Sulfate > 0, "проверить анализ бария обеих вод и наличие сульфатных отложений"},
		{"Целестин (сульфат стронция)", m.Solution.CelestiteSI, m.Totals.Sr > 0 && m.Totals.Sulfate > 0, "проверить анализ стронция обеих вод и состав сульфатного осадка"},
	}
	for _, mineral := range minerals {
		if !mineral.present {
			reason := "По введённому составу нет одного или обоих необходимых компонентов; численный SI этой соли не выдан. Это относится к представленному анализу, а не ко всем возможным условиям объекта."
			if strings.HasPrefix(mineral.name, "Барит") && (strings.TrimSpace(v["ba"]) == "" || strings.TrimSpace(v["mix2_ba"]) == "") {
				reason = "Не хватает анализа бария одной или обеих вод. Оценить эту соль полностью нельзя; пустое поле не означает нулевое содержание бария."
			}
			if strings.HasPrefix(mineral.name, "Целестин") && (strings.TrimSpace(v["sr"]) == "" || strings.TrimSpace(v["mix2_sr"]) == "") {
				reason = "Не хватает анализа стронция одной или обеих вод. Оценить эту соль полностью нельзя; пустое поле не означает нулевое содержание стронция."
			}
			lines = append(lines, mineral.name+": "+reason)
			continue
		}
		meaning := "пересыщение в рассчитанной смеси не выявлено"
		if mineral.si > .1 {
			meaning = "есть химическая предпосылка образования соли"
		} else if mineral.si >= -.1 {
			meaning = "смесь близка к равновесию; небольшое изменение условий может изменить вывод"
		}
		qualification := ""
		if strings.HasPrefix(mineral.name, "Барит") && (strings.TrimSpace(v["ba"]) == "" || strings.TrimSpace(v["mix2_ba"]) == "") {
			qualification = " Расчёт частичный: барий не измерен в одной или обеих водах; результат учитывает только внесённые количества и не исключает образования барита."
		}
		if strings.HasPrefix(mineral.name, "Целестин") && (strings.TrimSpace(v["sr"]) == "" || strings.TrimSpace(v["mix2_sr"]) == "") {
			qualification = " Расчёт частичный: стронций не измерен в одной или обеих водах; результат учитывает только внесённые количества и не исключает образования целестина."
		}
		lines = append(lines, fmt.Sprintf("%s: SI = %.3f — %s. Для технолога: %s.%s", mineral.name, mineral.si, meaning, mineral.consequence, qualification))
	}
	if len(m.Warnings) > 0 {
		lines = append(lines, "Что мешает уверенному выводу: "+strings.Join(m.Warnings, " "))
	}
	lines = append(lines,
		"Где искать последствия. Сначала найдите фактическую точку соединения потоков. Ниже по потоку осадок может накапливаться в трубах, фильтрах, насосе и другом оборудовании, через которое проходит смесь. Сопоставьте это место с ростом перепада давления, снижением подачи, засорением фильтров и обнаруженным осадком. Сам расчёт не доказывает, что причина ухудшения работы — именно соли.",
		"Что сделать по порядку. 1) Сверьте источники, даты и единицы обоих анализов: состав должен соответствовать реально смешиваемым водам. 2) Проверьте долю воды 2: 30 % означает 30 литров воды 2 и 70 литров воды 1 до смешения. 3) Приготовьте пробную смесь в этой доле при 25 °C и измерьте её pH: pH смеси нужно измерить после смешения, pH исходных вод нельзя усреднять. 4) Проверьте появление осадка и, если он есть, установите его состав. 5) Сопоставьте лабораторный результат с фактической температурой и условиями оборудования. Испытание прозрачной смеси само по себе не исключает более позднее образование осадка.",
		"Как принять решение. При пересыщении сначала подтвердите проблемную соль и место контакта вод. Затем рассмотрите изменение доли или разделение потоков, либо испытание подходящего ингибитора солеотложений. Тип реагента и рабочую дозировку определяют испытанием на реальных водах; этот расчёт дозировку не выдаёт. После внедрения контролируйте отложения и работу оборудования, чтобы проверить фактический эффект.",
		"Если все рассчитанные SI низкие. Это означает только отсутствие пересыщения оценёнными солями в данной смеси при 25 °C. Это не разрешение на смешение при любых условиях. Отсутствующий анализ бария или стронция не равен нулю. Другие доли смешения, другая температура, изменение pH, обмен газом и другие минералы требуют отдельной оценки.",
		"Пределы расчёта. Модель рассчитывает одно заданное соотношение при 25 °C. Она не определяет количество и скорость образования осадка, скорость зарастания, срок до отказа, конечный объём и плотность смеси, нагрев или охлаждение. Количество растворителя учитывается по плотности и массе растворённых веществ каждой воды, поэтому составы нельзя просто усреднить по объёму. Численные подробности выше нужны для проверки расчёта; решение по эксплуатации принимают вместе с промысловыми данными.")
	return strings.Join(lines, "\n\n")
}
