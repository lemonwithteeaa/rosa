//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	idBasisList            = 7481
	idBasisClose           = 7482
	idBasisMode            = 7483
	idBasisPass            = 7484
	idBasisRun             = 7485
	idBasisExportDOCX      = 7486
	idBasisExportXLSX      = 7487
	idBasisModel           = 7488
	wmBasisCalculationDone = 0x8009
)

type calculationBasisSection struct {
	Code, Name, Method, Formula, Variables, Limits string
}

type calculationBasisAsyncResult struct {
	Text string
}

var calculationBasisSections = []calculationBasisSection{
	{Code: "scale", Name: "Соли", Method: "Термодинамический скрининг насыщения воды выбранной моделью.", Formula: "Kнас = IAP / Ksp\r\nSI = log₁₀(Kнас)\r\nIAP = произведение активностей ионов с учётом стехиометрии.\r\nДля модели Дэвиса: log₁₀γᵢ = −A·zᵢ²·(√I/(1+√I) − 0,3I).", Variables: "IAP — ионное произведение активностей; Ksp — произведение растворимости; γᵢ — коэффициент активности; zᵢ — заряд; I — ионная сила.", Limits: "Это оценка термодинамической возможности, а не скорости и массы осадка. Применимость зависит от выбранной модели, полноты состава, температуры и давления."},
	{Code: "corrosion", Name: "Коррозия", Method: "Пороговый скрининг лабораторных признаков и фактического коррозионного мониторинга.", Formula: "Контрольные условия: pH < 6,5; Cl⁻ > 10 000 мг/л; H₂S > 0; измеренная скорость коррозии > 0,1 мм/год.\r\nИтог не рассчитывается сложением несопоставимых показателей: каждый фактор показывается отдельно.", Variables: "pH, хлориды, O₂, CO₂, H₂S, минерализация, обводнённость и измеренная скорость коррозии.", Limits: "По одной пробе нельзя определить механизм повреждения, срок до отказа или дозировку ингибитора."},
	{Code: "aspo", Name: "АСПО", Method: "Скрининг состава нефти и температурно-вязкостных предпосылок.", Formula: "Контрольные условия: парафины > 3 % масс.; асфальтены > 1 % масс.; температура застывания > 15 °C; вязкость > 50 мПа·с.\r\nКаждый признак оценивается отдельно, без непрозрачного суммарного балла.", Variables: "Парафины, смолы, асфальтены, температура застывания, вязкость, вода в пробе и температурный режим.", Limits: "Состав пробы не доказывает место образования АСПО и не задаёт растворитель, температуру прогрева или периодичность обработки."},
	{Code: "solids", Name: "Механические примеси", Method: "Скрининг количества и гранулометрии твёрдой фазы.", Formula: "Расчётный блок формируется как сопоставление концентрации взвешенных веществ, песка, мутности и d₅₀ с рабочими порогами и паспортом метода анализа.", Variables: "Взвешенные вещества, песок, мутность, d₅₀, состав и твёрдость частиц.", Limits: "Без анализа состава нельзя считать частицы песком, продуктами коррозии или солевым осадком."},
	{Code: "microbiology", Name: "Микробиология", Method: "Скрининг результатов посевов/тестов с учётом отбора и хранения пробы.", Formula: "Отдельно показываются СВБ, ГМБ и сульфиды; автоматическое доказательство микробиологической коррозии не формируется.", Variables: "СВБ, ГМБ, сульфиды, дата, метод отбора, консервация и время до анализа.", Limits: "Для подтверждения нужны профильное исследование, повторяемость и сопоставление с коррозионными продуктами."},
	{Code: "gases", Name: "Газы и гидраты", Method: "Скрининг газовой фазы и сопоставление равновесной температуры гидратов с фактическими T/P.", Formula: "Запас по температуре: ΔT = Tфакт − Tгидр.\r\nПоложительный запас сам по себе не исключает локальные зоны; отрицательный требует проверки давления, состава газа и фактического режима.", Variables: "CO₂, H₂S, газосодержание, вода, Tфакт, Pфакт и расчётная Tгидр.", Limits: "Один газовый показатель не подтверждает гидратную пробку, коррозионный механизм или дефект работы оборудования."},
}

var (
	basisDialogDone      bool
	basisList            syscall.Handle
	basisText            syscall.Handle
	basisTitle           syscall.Handle
	basisHeader          syscall.Handle
	basisAccent          syscall.Handle
	basisClose           syscall.Handle
	basisModeCombo       syscall.Handle
	basisPassCombo       syscall.Handle
	basisModelCombo      syscall.Handle
	basisRun             syscall.Handle
	basisExportDOCX      syscall.Handle
	basisExportXLSX      syscall.Handle
	basisPasses          []LabPassRecord
	basisInitial         int
	basisWindowHandle    syscall.Handle
	basisCalculationBusy bool
	basisCalculationDone chan calculationBasisAsyncResult
)

func calculationBasisCurrentInputsWithUnits(code string, values map[string]string, units map[string]string) string {
	if len(values) == 0 {
		return "Текущие исходные данные: раздел пока не заполнен."
	}
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "Текущие исходные данные: раздел пока не заполнен."
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		label := key
		defaultUnit := ""
		if group, ok := labGroupByCode(code); ok {
			for _, field := range group.Fields {
				if field.Key == key {
					label = laboratoryFieldDisplayLabel(field)
					defaultUnit = field.Unit
					break
				}
			}
		}
		unit := strings.TrimSpace(units[key])
		if unit == "" {
			unit = strings.TrimSpace(defaultUnit)
		}
		value := strings.TrimSpace(values[key])
		if numeric, ok := parseLabNumber(value); ok {
			value = reportDisplayNumber(numeric)
		}
		if unit != "" && !strings.EqualFold(unit, "текст") && !strings.EqualFold(unit, "расчёт") {
			lines = append(lines, label+" = "+value+" "+unit)
		} else {
			lines = append(lines, label+" = "+value)
		}
	}
	return "ТЕКУЩИЕ ИСХОДНЫЕ ДАННЫЕ\r\n" + strings.Join(lines, "\r\n")
}

func calculationBasisCurrentInputsForValues(code string, values map[string]string) string {
	return calculationBasisCurrentInputsWithUnits(code, values, nil)
}

func calculationBasisCurrentInputs(code string) string {
	return calculationBasisCurrentInputsWithUnits(code, labDrafts[code], labUnitDrafts[code])
}

func calculationNumeric(values map[string]string, key string) (float64, bool) {
	if values == nil {
		return 0, false
	}
	return parseLabNumber(values[key])
}

type calculationThresholdRule struct {
	Key, Label, Op string
	Limit          float64
}

func calculationThresholdExample(values map[string]string, rules []calculationThresholdRule) string {
	var lines []string
	step := 1
	for _, rule := range rules {
		value, ok := calculationNumeric(values, rule.Key)
		if !ok {
			continue
		}
		triggered := (rule.Op == ">" && value > rule.Limit) || (rule.Op == "<" && value < rule.Limit)
		result := "условие не сработало"
		if triggered {
			result = "условие сработало"
		}
		lines = append(lines, fmt.Sprintf("ШАГ %d. %s\r\nФормула проверки: x %s %.4g.\r\nПодстановка: %.4g %s %.4g.\r\nРезультат: %s.", step, rule.Label, rule.Op, rule.Limit, value, rule.Op, rule.Limit, result))
		step++
	}
	if len(lines) == 0 {
		return "Для числового примера заполните показатели этой вкладки."
	}
	return strings.Join(lines, "\r\n\r\n")
}

func calculationScaleTextbookForModel(values map[string]string, modelCode string) string {
	model := laboratoryScaleModelByCode(modelCode)
	if !model.Implemented {
		return "Численная подстановка не выполняется: для этой модели нет проверенной реализации. " + model.Limitation
	}
	results, ionicStrength, warnings := laboratorySaturationResults(values, model.Code)
	var b strings.Builder
	b.WriteString("ШАГ 1. Нормализация исходных данных\r\n")
	b.WriteString("Формула перевода концентрации: c, моль/л = C, мг/л ÷ 1000 ÷ M, г/моль.\r\n")
	var strengthTerms []string
	for _, ion := range []struct {
		key, label   string
		mass, charge float64
	}{{"ca", "Ca²⁺", 40.078, 2}, {"mg", "Mg²⁺", 24.305, 2}, {"ba", "Ba²⁺", 137.327, 2}, {"sr", "Sr²⁺", 87.62, 2}, {"co3", "CO₃²⁻", 60.01, -2}, {"hco3", "HCO₃⁻", 61.016, -1}, {"so4", "SO₄²⁻", 96.06, -2}, {"na_k", "Na⁺+K⁺", 22.990, 1}, {"cl", "Cl⁻", 35.45, -1}} {
		if raw, ok := calculationNumeric(values, ion.key); ok {
			molar := raw / 1000 / ion.mass
			fmt.Fprintf(&b, "Подстановка %s: %.6g ÷ 1000 ÷ %.6g = %.8g моль/л.\r\n", ion.label, raw, ion.mass, molar)
			strengthTerms = append(strengthTerms, fmt.Sprintf("%.8g·(%.0f)²", molar, ion.charge))
		}
	}
	fmt.Fprintf(&b, "\r\nШАГ 2. Ионная сила\r\nФормула: I = ½·Σ(cᵢ·zᵢ²).\r\nПодстановка: I = ½·(%s) = %.8g моль/л.\r\n", strings.Join(strengthTerms, " + "), ionicStrength)
	if model.Code == "davies" {
		gamma2 := daviesActivityCoefficient(2, ionicStrength)
		fmt.Fprintf(&b, "\r\nШАГ 3. Коэффициенты активности по Дэвису\r\nФормула: log₁₀γᵢ = −0,509·zᵢ²·(√I/(1+√I) − 0,3I).\r\nПодстановка для |z|=2: log₁₀γ = −0,509·4·(√%.8g/(1+√%.8g) − 0,3·%.8g).\r\nРезультат: γ₂ = %.8g.\r\n", ionicStrength, ionicStrength, ionicStrength, gamma2)
	} else {
		b.WriteString("\r\nШАГ 3. Активности\r\nДля выбранной концентрационной модели принято γᵢ = 1, поэтому aᵢ = cᵢ.\r\n")
	}
	if len(results) == 0 {
		b.WriteString("\r\nШАГ 4. IAP/Ksp\r\nРасчёт не завершён: не хватает пары ионов для проверяемого минерала.\r\n")
	} else {
		for i, result := range results {
			ksp := math.Pow(10, result.LogK)
			iap := result.Ratio * ksp
			fmt.Fprintf(&b, "\r\nШАГ %d. %s (%s)\r\nФормулы: IAP = Π(aᵢ^νᵢ); Ksp = 10^(logK); Kнас = IAP/Ksp; SI = log₁₀(Kнас).\r\nКонстанта: logK = %.6g; Ksp = 10^(%.6g) = %.8g.\r\nПодстановка: IAP = %.8g; Kнас = %.8g / %.8g = %.8g; SI = log₁₀(%.8g) = %.6f.\r\nРезультат: ", i+4, result.Mineral, result.Formula, result.LogK, result.LogK, ksp, iap, iap, ksp, result.Ratio, result.Ratio, result.SI)
			if result.SI > 0 {
				b.WriteString("раствор термодинамически пересыщен.")
			} else if result.SI >= -0.1 {
				b.WriteString("состояние близко к насыщению.")
			} else {
				b.WriteString("раствор ненасыщен.")
			}
		}
	}
	if len(warnings) > 0 {
		b.WriteString("\r\n\r\nОГРАНИЧЕНИЯ РАСЧЁТА\r\n• " + strings.Join(warnings, "\r\n• "))
	}
	return b.String()
}

func calculationBasisExample(code string, values map[string]string) string {
	switch code {
	case "corrosion":
		return calculationThresholdExample(values, []calculationThresholdRule{{"ph", "pH", "<", 6.5}, {"chlorides", "Cl⁻", ">", 10000}, {"h2s", "H₂S", ">", 0}, {"corrosion_rate", "Скорость коррозии", ">", 0.1}})
	case "aspo":
		return calculationThresholdExample(values, []calculationThresholdRule{{"paraffins", "Парафины", ">", 3}, {"asphaltenes", "Асфальтены", ">", 1}, {"pour_point", "Температура застывания", ">", 15}, {"viscosity", "Вязкость", ">", 50}})
	case "solids":
		return calculationThresholdExample(values, []calculationThresholdRule{{"suspended", "Взвешенные вещества", ">", 50}, {"sand", "Песок", ">", 0}, {"turbidity", "Мутность", ">", 0}})
	case "microbiology":
		var parts []string
		for _, item := range []struct{ key, label string }{{"srb", "СВБ"}, {"gmb", "Общее микробное число"}, {"sulfide", "Сульфиды"}, {"qpcr_srb", "СВБ qPCR"}} {
			if value := strings.TrimSpace(values[item.key]); value != "" {
				parts = append(parts, item.label+" = "+value+" → отдельный признак, без суммирования с другими методами.")
			}
		}
		if len(parts) == 0 {
			return "Для примера заполните результаты СВБ/ГМБ, сульфиды либо qPCR."
		}
		return strings.Join(parts, "\r\n")
	case "gases":
		actual, okActual := calculationNumeric(values, "sample_temp")
		hydrate, okHydrate := calculationNumeric(values, "hydrate_temp")
		if okActual && okHydrate {
			delta := actual - hydrate
			state := "температура выше равновесной границы"
			if delta < 0 {
				state = "температура ниже равновесной границы — требуется проверка гидратообразования"
			}
			return fmt.Sprintf("ШАГ 1. Температурный запас\r\nФормула: ΔT = Tфакт − Tгидр.\r\nПодстановка: ΔT = %.3g − %.3g = %.3g °C.\r\nРезультат: %s.", actual, hydrate, delta, state)
		}
		return "Для примера заполните фактическую и равновесную температуру гидратов."
	}
	return "Для числового примера заполните показатели этой вкладки."
}

func calculationScaleAllModels(values map[string]string) string {
	var b strings.Builder
	b.WriteString("РАСЧЁТ СРАЗУ ПО ВСЕМ МОДЕЛЯМ\r\n")
	for _, model := range laboratoryScaleModels {
		fmt.Fprintf(&b, "\r\nМОДЕЛЬ: %s\r\n", model.Name)
		if !model.Implemented {
			b.WriteString("Численный расчёт не выполняется: модель пока доступна как метод-план, без подмены упрощённой формулой.\r\n")
			b.WriteString("Область применения: " + model.Applicability + "\r\nОграничение: " + model.Limitation + "\r\n")
			continue
		}
		b.WriteString(calculationScaleTextbookForModel(values, model.Code))
		b.WriteString("\r\n")
	}
	return strings.TrimSpace(b.String())
}

func calculationBasisTextForValues(index int, values map[string]string, units map[string]string) string {
	if index < 0 || index >= len(calculationBasisSections) {
		return "Выберите осложнение."
	}
	section := calculationBasisSections[index]
	if section.Code == "scale" {
		if group, ok := labGroupByCode("scale"); ok {
			values = canonicalizeLaboratoryValues(group, values, units)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\r\n\r\nМЕТОДИКА\r\n%s\r\n\r\nФОРМУЛЫ И ПРАВИЛА\r\n%s\r\n\r\nПРИМЕР РАСЧЁТА ПО ТЕКУЩИМ ДАННЫМ\r\n", strings.ToUpper(section.Name), section.Method, section.Formula)
	if section.Code == "scale" {
		b.WriteString(calculationScaleAllModels(values))
	} else {
		b.WriteString(calculationBasisExample(section.Code, values))
	}
	fmt.Fprintf(&b, "\r\n\r\nПЕРЕМЕННЫЕ\r\n%s\r\n\r\nГРАНИЦЫ ПРИМЕНИМОСТИ\r\n%s\r\n\r\n%s", section.Variables, section.Limits, calculationBasisCurrentInputsWithUnits(section.Code, values, units))
	return b.String()
}

func calculationBasisTextForModel(index int, values map[string]string, units map[string]string, modelCode string) string {
	if index < 0 || index >= len(calculationBasisSections) || calculationBasisSections[index].Code != "scale" || modelCode == "" {
		return calculationBasisTextForValues(index, values, units)
	}
	section := calculationBasisSections[index]
	if group, ok := labGroupByCode("scale"); ok {
		values = canonicalizeLaboratoryValues(group, values, units)
	}
	model := laboratoryScaleModelByCode(modelCode)
	result := "Расчёт не выполнен. Проверенная численная реализация этой модели отсутствует; упрощённая подмена не используется."
	if model.Implemented {
		result = calculationScaleTextbookForModel(values, model.Code)
	}
	return fmt.Sprintf("%s\r\n\r\nМЕТОДИКА\r\n%s\r\n\r\nВЫБРАННАЯ МОДЕЛЬ\r\n%s\r\nОбласть применимости: %s\r\nОграничение: %s\r\n\r\nФОРМУЛЫ И ПРАВИЛА\r\n%s\r\n\r\nПОДСТАНОВКА И РЕЗУЛЬТАТ\r\n%s\r\n\r\nПЕРЕМЕННЫЕ\r\n%s\r\n\r\n%s", strings.ToUpper(section.Name), section.Method, model.Name, model.Applicability, model.Limitation, section.Formula, result, section.Variables, calculationBasisCurrentInputsWithUnits(section.Code, values, units))
}

func calculationBasisText(index int) string {
	if index < 0 || index >= len(calculationBasisSections) {
		return "Выберите осложнение."
	}
	code := calculationBasisSections[index].Code
	return calculationBasisTextForValues(index, labDrafts[code], labUnitDrafts[code])
}

func laboratoryCalculationAppendix(pass LabPassRecord) string {
	var b strings.Builder
	b.WriteString("РАСЧЁТЫ И УСЛОВИЯ ОЦЕНКИ\r\n")
	added := 0
	for index, section := range calculationBasisSections {
		values := pass.Groups[section.Code]
		if len(values) == 0 {
			continue
		}
		if added > 0 {
			b.WriteString("\r\n\r\n")
		}
		b.WriteString(calculationBasisTextForValues(index, values, pass.Units[section.Code]))
		added++
	}
	if added == 0 {
		b.WriteString("В текущем анализе нет показателей для расчётного приложения.\r\n")
	}
	return b.String()
}

func calculationRequiredKeys(code string) []string {
	switch code {
	case "scale":
		return []string{"sample_temp", "ph", "ca", "hco3", "so4"}
	case "corrosion":
		return []string{"ph", "chlorides", "co2", "h2s", "o2", "corrosion_rate"}
	case "aspo":
		return []string{"paraffins", "asphaltenes", "pour_point", "viscosity"}
	case "solids":
		return []string{"suspended", "sand", "turbidity", "d50"}
	case "microbiology":
		return []string{"srb", "gmb", "sulfide"}
	case "gases":
		return []string{"sample_temp", "hydrate_temp", "pressure"}
	}
	return nil
}

func calculationFieldLabel(code, key string) string {
	if group, ok := labGroupByCode(code); ok {
		for _, field := range group.Fields {
			if field.Key == key {
				return laboratoryFieldDisplayLabel(field)
			}
		}
	}
	return key
}

func calculationMissingData(code string, values map[string]string) []string {
	var missing []string
	for _, key := range calculationRequiredKeys(code) {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, calculationFieldLabel(code, key))
		}
	}
	return missing
}

func calculationRisk(code string, values map[string]string) string {
	if len(values) == 0 {
		return "недостаточно данных"
	}
	if code == "scale" {
		// RC559: do not execute the iterative chemistry a second time merely to
		// build the one-line risk caption. The full SI/Kнас results are already
		// calculated and printed by the selected model block above.
		if len(calculationMissingData(code, values)) > 0 {
			return "недостаточно данных для полной оценки — см. недостающие параметры ниже"
		}
		return "оценка определяется рассчитанными SI/Кнас выбранной модели выше; положительный SI означает термодинамическое пересыщение"
	}
	rules := map[string][]calculationThresholdRule{
		"corrosion": {{"ph", "pH", "<", 6.5}, {"chlorides", "Cl⁻", ">", 10000}, {"h2s", "H₂S", ">", 0}, {"corrosion_rate", "Скорость коррозии", ">", 0.1}},
		"aspo":      {{"paraffins", "Парафины", ">", 3}, {"asphaltenes", "Асфальтены", ">", 1}, {"pour_point", "Температура застывания", ">", 15}, {"viscosity", "Вязкость", ">", 50}},
		"solids":    {{"suspended", "Взвешенные вещества", ">", 50}, {"sand", "Песок", ">", 0}, {"turbidity", "Мутность", ">", 0}},
	}
	triggered, available := 0, 0
	for _, rule := range rules[code] {
		value, ok := calculationNumeric(values, rule.Key)
		if !ok {
			continue
		}
		available++
		if (rule.Op == ">" && value > rule.Limit) || (rule.Op == "<" && value < rule.Limit) {
			triggered++
		}
	}
	if code == "gases" {
		actual, a := calculationNumeric(values, "sample_temp")
		hydrate, hydrateOK := calculationNumeric(values, "hydrate_temp")
		if !a || !hydrateOK {
			return "недостаточно данных"
		}
		if actual-hydrate < 0 {
			return "высокий — фактическая температура ниже расчётной температуры гидратообразования"
		}
		return "низкий по температурному запасу; проверьте локальные зоны, давление и состав газа"
	}
	if code == "microbiology" {
		for _, key := range []string{"srb", "gmb", "sulfide", "qpcr_srb"} {
			if strings.TrimSpace(values[key]) != "" {
				return "средний — выявлен отдельный микробиологический признак; требуется подтверждение повторным профильным исследованием"
			}
		}
		return "недостаточно данных"
	}
	if available == 0 {
		return "недостаточно данных"
	}
	if triggered >= 2 {
		return "высокий — одновременно сработало несколько проверяемых условий"
	}
	if triggered == 1 {
		return "средний — сработало одно проверяемое условие"
	}
	return "низкий по заполненным показателям; отсутствие срабатывания не исключает осложнение"
}

func calculationPassSectionWithModel(pass LabPassRecord, index int, modelCode string) string {
	if index < 0 || index >= len(calculationBasisSections) {
		return "Выберите осложнение."
	}
	section := calculationBasisSections[index]
	values := pass.Groups[section.Code]
	units := pass.Units[section.Code]
	var b strings.Builder
	fmt.Fprintf(&b, "%s\r\nОбъект: %s\r\nДата исходных данных: %s\r\nИсточник / протокол: %s\r\nСтатус расчёта: предварительная диагностика\r\n\r\n", strings.ToUpper(section.Name), emptyLabText(readableObjectKey(pass.ObjectKey)), emptyLabText(pass.Date), emptyLabText(pass.Source))
	b.WriteString(calculationBasisTextForModel(index, values, units, modelCode))
	missing := calculationMissingData(section.Code, values)
	b.WriteString("\r\n\r\nОЦЕНКА РИСКА\r\n" + calculationRisk(section.Code, values) + ".")
	if len(missing) > 0 {
		b.WriteString("\r\n\r\nРАСЧЁТ НЕ ВЫПОЛНЕН ПОЛНОСТЬЮ\r\nНедостающие параметры: " + strings.Join(missing, "; ") + ".\r\nГде получить: лабораторный протокол, техрежим либо подтверждённый расчёт профильной модели. Уже реализован показ доступных исходных данных, проверяемых условий и исполняемых моделей без демонстрационных констант.")
	} else {
		b.WriteString("\r\n\r\nПОЛНОТА ИСХОДНЫХ ДАННЫХ\r\nМинимальный перечень для этого диагностического блока заполнен. Проверьте совпадение дат, единиц и область применимости каждой модели.")
	}
	b.WriteString("\r\n\r\nПРАКТИЧЕСКАЯ РЕКОМЕНДАЦИЯ\r\nСопоставьте расчёт с историей отказов, режимом, повторным анализом и фактическим состоянием оборудования. Не назначайте реагент или ремонт только по одному расчётному признаку.")
	return b.String()
}

func calculationPassSection(pass LabPassRecord, index int) string {
	return calculationPassSectionWithModel(pass, index, "")
}

func calculationPassReportWithModel(pass LabPassRecord, sectionIndex int, full bool, modelCode string) string {
	var b strings.Builder
	b.WriteString("РОСА — РАСЧЁТЫ И ФОРМУЛЫ ПО ОБЪЕКТУ\r\n")
	fmt.Fprintf(&b, "Объект: %s\r\nДата формирования отчёта: %s\r\nИспользованный анализ: %s · %s\r\n", emptyLabText(readableObjectKey(pass.ObjectKey)), time.Now().Format("02.01.2006 15:04"), emptyLabText(pass.Date), emptyLabText(pass.Source))
	if full {
		for index := range calculationBasisSections {
			b.WriteString("\r\n\r\n" + calculationPassSectionWithModel(pass, index, ""))
		}
	} else {
		b.WriteString("\r\n\r\n" + calculationPassSectionWithModel(pass, sectionIndex, modelCode))
	}
	return b.String()
}

func calculationPassReport(pass LabPassRecord, sectionIndex int, full bool) string {
	return calculationPassReportWithModel(pass, sectionIndex, full, "")
}

func calculationBasisCurrentSnapshot() LabPassRecord {
	// Calculation preview/run must never persist a draft or scan the whole history.
	// Capture the visible values directly so the dialog opens immediately and the
	// user can calculate unsaved edits without creating a database revision.
	groups := cloneLabGroups(labDrafts)
	units := cloneLabGroups(labUnitDrafts)
	g := currentLabGroup()
	if g.Code != "overall" {
		values := groups[g.Code]
		if values == nil {
			values = map[string]string{}
		}
		start := labFieldsPage * 10
		for i := 0; i < 10 && start+i < len(g.Fields); i++ {
			values[g.Fields[start+i].Key] = getText(labFieldEdits[i])
		}
		groups[g.Code] = values
		units[g.Code] = currentLaboratoryUnits()
	}
	return LabPassRecord{
		ObjectKey:  strings.TrimSpace(getText(labObjectEdit)),
		Date:       strings.TrimSpace(getText(labDateEdit)),
		Source:     strings.TrimSpace(getText(labSourceEdit)),
		Status:     labPassDraftStatus,
		Groups:     groups,
		Units:      units,
		ScaleModel: labScaleModelCode,
	}
}

func calculationBasisSelectedPass() LabPassRecord {
	if basisPassCombo != 0 {
		index, _, _ := pSendMessageW.Call(uintptr(basisPassCombo), CB_GETCURSEL, 0, 0)
		if int(index) >= 0 && int(index) < len(basisPasses) {
			return basisPasses[index]
		}
	}
	return calculationBasisCurrentSnapshot()
}

func calculationBasisFullMode() bool {
	if basisModeCombo == 0 {
		return false
	}
	index, _, _ := pSendMessageW.Call(uintptr(basisModeCombo), CB_GETCURSEL, 0, 0)
	return int(index) == 1
}

func calculationBasisSectionIndex() int {
	index := basisInitial
	if basisList != 0 {
		selection, _, _ := pSendMessageW.Call(uintptr(basisList), LB_GETCURSEL, 0, 0)
		if int(selection) >= 0 && int(selection) < len(calculationBasisSections) {
			index = int(selection)
		}
	}
	return index
}

func calculationBasisPopulateModels(sectionIndex int) {
	if basisModelCombo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(basisModelCombo), CB_RESETCONTENT, 0, 0)
	if sectionIndex >= 0 && sectionIndex < len(calculationBasisSections) && calculationBasisSections[sectionIndex].Code == "scale" {
		pSendMessageW.Call(uintptr(basisModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p("Все доступные модели (сравнение)"))))
		for _, model := range laboratoryScaleModels {
			pSendMessageW.Call(uintptr(basisModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(model.Name))))
		}
	} else {
		pSendMessageW.Call(uintptr(basisModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p("Основная проверяемая методика"))))
	}
	pSendMessageW.Call(uintptr(basisModelCombo), CB_SETCURSEL, 0, 0)
}

func calculationBasisSelectedModel(sectionIndex int) string {
	if calculationBasisFullMode() || sectionIndex < 0 || sectionIndex >= len(calculationBasisSections) || calculationBasisSections[sectionIndex].Code != "scale" || basisModelCombo == 0 {
		return ""
	}
	selection, _, _ := pSendMessageW.Call(uintptr(basisModelCombo), CB_GETCURSEL, 0, 0)
	modelIndex := int(selection) - 1
	if modelIndex >= 0 && modelIndex < len(laboratoryScaleModels) {
		return laboratoryScaleModels[modelIndex].Code
	}
	return ""
}

func calculationBasisPreview() string {
	index := calculationBasisSectionIndex()
	if index < 0 || index >= len(calculationBasisSections) {
		index = 0
	}
	section := calculationBasisSections[index]
	pass := calculationBasisSelectedPass()
	values := pass.Groups[section.Code]
	modelLine := ""
	if section.Code == "scale" {
		modelCode := calculationBasisSelectedModel(index)
		if modelCode == "" {
			modelLine = "\r\nМодель: все доступные модели (сравнение)."
		} else {
			modelLine = "\r\nМодель: " + laboratoryScaleModelByCode(modelCode).Name + "."
		}
	}
	return fmt.Sprintf("%s\r\n\r\nМЕТОДИКА\r\n%s%s\r\n\r\nФОРМУЛЫ И ПРАВИЛА\r\n%s\r\n\r\nПЕРЕМЕННЫЕ\r\n%s\r\n\r\nГРАНИЦЫ ПРИМЕНИМОСТИ\r\n%s\r\n\r\n%s\r\n\r\nНажмите «Выполнить расчёт». Расчёт запускается отдельно и не сохраняет текущий черновик.", strings.ToUpper(section.Name), section.Method, modelLine, section.Formula, section.Variables, section.Limits, calculationBasisCurrentInputsWithUnits(section.Code, values, pass.Units[section.Code]))
}

func showCalculationBasisPreview() {
	if basisText != 0 && !basisCalculationBusy {
		formatComplexText(basisText, calculationBasisPreview())
	}
}

func setCalculationBasisBusy(busy bool) {
	basisCalculationBusy = busy
	if basisRun != 0 {
		pEnableWindow.Call(uintptr(basisRun), uintptr(boolToInt(!busy)))
		if busy {
			setText(basisRun, "Расчёт…")
		} else {
			setText(basisRun, "Выполнить расчёт")
		}
	}
	for _, control := range []syscall.Handle{basisModeCombo, basisPassCombo, basisExportDOCX, basisExportXLSX} {
		if control != 0 {
			pEnableWindow.Call(uintptr(control), uintptr(boolToInt(!busy)))
		}
	}
	if busy {
		if basisList != 0 {
			pEnableWindow.Call(uintptr(basisList), 0)
		}
		if basisModelCombo != 0 {
			pEnableWindow.Call(uintptr(basisModelCombo), 0)
		}
		return
	}
	index := calculationBasisSectionIndex()
	full := calculationBasisFullMode()
	if basisList != 0 {
		pEnableWindow.Call(uintptr(basisList), uintptr(boolToInt(!full)))
	}
	if basisModelCombo != 0 && index >= 0 && index < len(calculationBasisSections) {
		pEnableWindow.Call(uintptr(basisModelCombo), uintptr(boolToInt(!full && calculationBasisSections[index].Code == "scale")))
	}
}

func calculationBasisRefresh() {
	if basisCalculationBusy || basisWindowHandle == 0 {
		return
	}
	index := calculationBasisSectionIndex()
	if index < 0 || index >= len(calculationBasisSections) {
		index = 0
	}
	full := calculationBasisFullMode()
	pass := calculationBasisSelectedPass()
	modelCode := calculationBasisSelectedModel(index)
	setCalculationBasisBusy(true)
	setText(basisText, "Выполняется расчёт… Окно не блокируется. Текущие данные и формулы не изменяются.")
	done := basisCalculationDone
	hwnd := basisWindowHandle
	go func(pass LabPassRecord, sectionIndex int, fullMode bool, model string) {
		result := calculationBasisAsyncResult{}
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					result.Text = fmt.Sprintf("Расчёт остановлен внутренней защитой: %v.\r\n\r\nИсходные данные не изменены. Попробуйте отдельную модель или проверьте введённые значения.", recovered)
				}
			}()
			result.Text = calculationPassReportWithModel(pass, sectionIndex, fullMode, model)
		}()
		select {
		case done <- result:
		default:
		}
		pPostMessageW.Call(uintptr(hwnd), wmBasisCalculationDone, 0, 0)
	}(pass, index, full, modelCode)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func calculationExportIndexes(full bool, selected int) []int {
	if !full {
		if selected < 0 || selected >= len(calculationBasisSections) {
			selected = 0
		}
		return []int{selected}
	}
	out := make([]int, len(calculationBasisSections))
	for i := range out {
		out[i] = i
	}
	return out
}

func calculationChoosePath(owner syscall.Handle, extension string) string {
	_ = owner
	dir := rosaDesktopDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	return filepath.Join(dir, "РОСА_расчёты_ДЛЯ_ПРОВЕРКИ_И_ИЗМЕНЕНИЙ_"+time.Now().Format("20060102_150405")+"."+extension)
}

func calculationOrderedKeys(code string, values map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	if group, ok := labGroupByCode(code); ok {
		for _, field := range group.Fields {
			if strings.TrimSpace(values[field.Key]) != "" {
				keys = append(keys, field.Key)
				seen[field.Key] = true
			}
		}
	}
	var extra []string
	for key, value := range values {
		if !seen[key] && strings.TrimSpace(value) != "" {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	return append(keys, extra...)
}

func calculationFormulaCell(ref, formula string, cached float64, style int) string {
	return fmt.Sprintf(`<c r="%s" s="%d"><f>%s</f><v>%s</v></c>`, ref, style, reportXMLText(formula), strconv.FormatFloat(cached, 'g', -1, 64))
}

func calculationWorkbookFormulaRows(code string, values map[string]string, modelCode string, startRow int) (string, int) {
	var rows strings.Builder
	row := startRow
	header := func() {
		rows.WriteString(fmt.Sprintf(`<row r="%d" ht="34" customHeight="1">%s%s%s%s%s%s</row>`, row,
			xlsxInlineCell("A"+strconv.Itoa(row), "Шаг", 3), xlsxInlineCell("B"+strconv.Itoa(row), "Исходное число", 3),
			xlsxInlineCell("C"+strconv.Itoa(row), "Порог / константа", 3), xlsxInlineCell("D"+strconv.Itoa(row), "Формула и пошаговая подстановка", 3),
			xlsxInlineCell("E"+strconv.Itoa(row), "Excel-формула", 3), xlsxInlineCell("F"+strconv.Itoa(row), "Результат Excel", 3)))
		row++
	}
	addRule := func(rule calculationThresholdRule) {
		value, ok := calculationNumeric(values, rule.Key)
		if !ok {
			return
		}
		triggered := (rule.Op == ">" && value > rule.Limit) || (rule.Op == "<" && value < rule.Limit)
		cached := 0.0
		if triggered {
			cached = 1
		}
		ref := strconv.Itoa(row)
		formula := "--(B" + ref + rule.Op + "C" + ref + ")"
		rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
			xlsxInlineCell("A"+ref, rule.Label, 2), xlsxNumberCell("B"+ref, value, 0), xlsxNumberCell("C"+ref, rule.Limit, 0),
			xlsxInlineCell("D"+ref, fmt.Sprintf("1) критерий: x %s %.6g; 2) подстановка: %.6g %s %.6g; 3) 1 = условие сработало, 0 = не сработало", rule.Op, rule.Limit, value, rule.Op, rule.Limit), 0),
			xlsxInlineCell("E"+ref, "="+formula, 2), calculationFormulaCell("F"+ref, formula, cached, 0)))
		row++
	}
	header()
	switch code {
	case "corrosion":
		for _, rule := range []calculationThresholdRule{{"ph", "pH", "<", 6.5}, {"chlorides", "Cl⁻", ">", 10000}, {"h2s", "H₂S", ">", 0}, {"corrosion_rate", "Скорость коррозии", ">", 0.1}} {
			addRule(rule)
		}
	case "aspo":
		for _, rule := range []calculationThresholdRule{{"paraffins", "Парафины", ">", 3}, {"asphaltenes", "Асфальтены", ">", 1}, {"pour_point", "Температура застывания", ">", 15}, {"viscosity", "Вязкость", ">", 50}} {
			addRule(rule)
		}
	case "solids":
		for _, rule := range []calculationThresholdRule{{"suspended", "Взвешенные вещества", ">", 50}, {"sand", "Песок", ">", 0}, {"turbidity", "Мутность", ">", 0}} {
			addRule(rule)
		}
	case "gases":
		actual, aOK := calculationNumeric(values, "sample_temp")
		hydrate, hOK := calculationNumeric(values, "hydrate_temp")
		if aOK && hOK {
			ref := strconv.Itoa(row)
			formula := "B" + ref + "-C" + ref
			rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
				xlsxInlineCell("A"+ref, "Температурный запас ΔT", 2), xlsxNumberCell("B"+ref, actual, 0), xlsxNumberCell("C"+ref, hydrate, 0),
				xlsxInlineCell("D"+ref, fmt.Sprintf("1) ΔT = Tфакт − Tгидр; 2) ΔT = %.6g − %.6g; 3) отрицательное ΔT означает отсутствие температурного запаса", actual, hydrate), 0),
				xlsxInlineCell("E"+ref, "="+formula, 2), calculationFormulaCell("F"+ref, formula, actual-hydrate, 0)))
			row++
		}
	case "scale":
		if strings.TrimSpace(modelCode) == "" {
			modelCode = "davies"
		}
		model := laboratoryScaleModelByCode(modelCode)
		results, ionicStrength, _ := laboratorySaturationResults(values, model.Code)
		rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s</row>`, row, xlsxInlineCell("A"+strconv.Itoa(row), "Модель", 2), xlsxInlineCell("B"+strconv.Itoa(row), model.Name, 0)))
		row++
		for _, result := range results {
			ref := strconv.Itoa(row)
			formula := "B" + ref
			rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
				xlsxInlineCell("A"+ref, result.Mineral+" — ионная сила", 2), xlsxNumberCell("B"+ref, ionicStrength, 0), xlsxInlineCell("C"+ref, "моль/л", 0),
				xlsxInlineCell("D"+ref, fmt.Sprintf("1) мг/л → моль/л; 2) для каждого иона cᵢ·zᵢ²; 3) I = ½·Σ(cᵢ·zᵢ²) = %.8g", ionicStrength), 0),
				xlsxInlineCell("E"+ref, "="+formula, 2), calculationFormulaCell("F"+ref, formula, ionicStrength, 0)))
			row++
			gamma := 1.0
			gammaFormula := "1"
			if model.Code == "davies" {
				gamma = daviesActivityCoefficient(2, ionicStrength)
				gammaFormula = fmt.Sprintf("POWER(10,-0.509*4*(SQRT(F%d)/(1+SQRT(F%d))-0.3*F%d))", row-1, row-1, row-1)
			}
			ref = strconv.Itoa(row)
			rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
				xlsxInlineCell("A"+ref, result.Mineral+" — γ(|z|=2)", 2), xlsxNumberCell("B"+ref, ionicStrength, 0), xlsxNumberCell("C"+ref, 2, 0),
				xlsxInlineCell("D"+ref, fmt.Sprintf("1) z² = 2² = 4; 2) I = %.8g; 3) γ = 10^[−0,509·4·(√I/(1+√I)−0,3I)]", ionicStrength), 0),
				xlsxInlineCell("E"+ref, "="+gammaFormula, 2), calculationFormulaCell("F"+ref, gammaFormula, gamma, 0)))
			row++
			ref = strconv.Itoa(row)
			ksp := math.Pow(10, result.LogK)
			iap := result.Ratio * ksp
			ratioFormula := "B" + ref + "/C" + ref
			rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
				xlsxInlineCell("A"+ref, result.Mineral+" — Kнас", 2), xlsxNumberCell("B"+ref, iap, 0), xlsxNumberCell("C"+ref, ksp, 0),
				xlsxInlineCell("D"+ref, fmt.Sprintf("1) IAP = %.8g; 2) Ksp = 10^(%.6g) = %.8g; 3) Kнас = IAP/Ksp = %.8g", iap, result.LogK, ksp, result.Ratio), 0),
				xlsxInlineCell("E"+ref, "="+ratioFormula, 2), calculationFormulaCell("F"+ref, ratioFormula, result.Ratio, 0)))
			row++
			ref = strconv.Itoa(row)
			siFormula := "LOG10(B" + ref + ")"
			rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s%s%s</row>`, row,
				xlsxInlineCell("A"+ref, result.Mineral+" — SI", 2), xlsxNumberCell("B"+ref, result.Ratio, 0), xlsxInlineCell("C"+ref, "безразмерно", 0),
				xlsxInlineCell("D"+ref, fmt.Sprintf("1) SI = log₁₀(Kнас); 2) SI = log₁₀(%.8g); 3) SI > 0 — пересыщение", result.Ratio), 0),
				xlsxInlineCell("E"+ref, "="+siFormula, 2), calculationFormulaCell("F"+ref, siFormula, result.SI, 0)))
			row++
		}
	default:
		rows.WriteString(fmt.Sprintf(`<row r="%d">%s%s</row>`, row, xlsxInlineCell("A"+strconv.Itoa(row), "Метод", 2), xlsxInlineCell("B"+strconv.Itoa(row), "Качественная проверка; числовая формула не применяется.", 0)))
		row++
	}
	return rows.String(), row
}

func calculationWorkbookSheet(pass LabPassRecord, index int, modelCode string) string {
	section := calculationBasisSections[index]
	values := pass.Groups[section.Code]
	units := pass.Units[section.Code]
	var rows strings.Builder
	row := 1
	addText := func(label, value string, labelStyle, valueStyle int) {
		rows.WriteString(fmt.Sprintf(`<row r="%d" ht="24" customHeight="1">%s%s</row>`, row, xlsxInlineCell("A"+strconv.Itoa(row), label, labelStyle), xlsxInlineCell("B"+strconv.Itoa(row), value, valueStyle)))
		row++
	}
	addText(strings.ToUpper(section.Name), "Расчёт по данным выбранного объекта", 1, 1)
	addText("Объект", readableObjectKey(pass.ObjectKey), 2, 0)
	addText("Дата данных", emptyLabText(pass.Date), 2, 0)
	addText("Источник", emptyLabText(pass.Source), 2, 0)
	addText("Модель / методика", section.Method, 2, 0)
	addText("Область применимости", section.Limits, 2, 0)
	addText("Оценка риска", calculationRisk(section.Code, values), 2, 0)
	row++
	addText("ИСХОДНЫЕ ДАННЫЕ", "Значение · единица · дата/источник", 3, 3)
	for _, key := range calculationOrderedKeys(section.Code, values) {
		unit := strings.TrimSpace(units[key])
		value := strings.TrimSpace(values[key])
		if unit != "" {
			value += " " + unit
		}
		value += " · " + emptyLabText(pass.Date) + " · " + emptyLabText(pass.Source)
		addText(calculationFieldLabel(section.Code, key), value, 2, 0)
	}
	if len(calculationOrderedKeys(section.Code, values)) == 0 {
		addText("Исходные данные", "не заполнены", 2, 0)
	}
	row++
	addText("ФОРМУЛА / ПРАВИЛА", section.Formula, 3, 0)
	if section.Code == "scale" {
		for _, model := range laboratoryScaleModels {
			if modelCode != "" && model.Code != modelCode {
				continue
			}
			status := "Расчёт не выполнен: проверенная численная реализация отсутствует. " + model.Limitation
			if model.Implemented {
				status = laboratorySaturationTextForModel(values, model.Code)
			}
			addText("Модель: "+model.Name, status, 2, 0)
		}
	} else {
		addText("Подстановка и результат", calculationBasisExample(section.Code, values), 2, 0)
	}
	row++
	formulaModel := modelCode
	if section.Code == "scale" && formulaModel == "" {
		formulaModel = strings.TrimSpace(pass.ScaleModel)
	}
	formulaRows, nextRow := calculationWorkbookFormulaRows(section.Code, values, formulaModel, row)
	rows.WriteString(formulaRows)
	row = nextRow
	missing := calculationMissingData(section.Code, values)
	missingText := "нет по минимальному перечню"
	if len(missing) > 0 {
		missingText = strings.Join(missing, "; ")
	}
	addText("Недостающие данные", missingText, 2, 0)
	addText("Рекомендация", "Сопоставить расчёт с историей отказов, режимом, повторным анализом и фактическим состоянием оборудования.", 2, 0)
	return `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="34" customWidth="1"/><col min="2" max="3" width="22" customWidth="1"/><col min="4" max="4" width="68" customWidth="1"/><col min="5" max="5" width="42" customWidth="1"/><col min="6" max="6" width="22" customWidth="1"/></cols><sheetData>` + rows.String() + `</sheetData><mergeCells count="1"><mergeCell ref="A1:F1"/></mergeCells><conditionalFormatting sqref="F1:F500"><cfRule type="expression" dxfId="0" priority="1"><formula>AND($E1&lt;&gt;&quot;&quot;,IFERROR(FORMULATEXT($F1)&lt;&gt;$E1,TRUE))</formula></cfRule></conditionalFormatting></worksheet>`
}

func writeCalculationWorkbookXLSX(path string, pass LabPassRecord, indexes []int, modelCode string) error {
	if len(indexes) == 0 {
		return fmt.Errorf("не выбрано осложнение")
	}
	files := map[string]string{}
	var contentTypes, sheets, rels, summary strings.Builder
	contentTypes.WriteString(`<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	sheets.WriteString(`<sheet name="Сводка" sheetId="1" r:id="rId1"/>`)
	rels.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>`)
	summary.WriteString(`<row r="1">` + xlsxInlineCell("A1", "РОСА — сводка расчётов", 1) + `</row>`)
	summary.WriteString(`<row r="2">` + xlsxInlineCell("A2", "Объект", 2) + xlsxInlineCell("B2", readableObjectKey(pass.ObjectKey), 0) + `</row>`)
	summary.WriteString(`<row r="3">` + xlsxInlineCell("A3", "Дата анализа", 2) + xlsxInlineCell("B3", emptyLabText(pass.Date), 0) + `</row>`)
	summary.WriteString(`<row r="4" ht="34" customHeight="1">` + xlsxInlineCell("A4", "РАБОЧАЯ КОПИЯ", 3) + xlsxInlineCell("B4", "Изменения в этом файле не загружаются обратно в РОСА.", 2) + `</row>`)
	summary.WriteString(`<row r="5">` + xlsxInlineCell("A5", "Осложнение", 3) + xlsxInlineCell("B5", "Риск", 3) + xlsxInlineCell("C5", "Недостающие данные", 3) + `</row>`)
	for position, index := range indexes {
		if index < 0 || index >= len(calculationBasisSections) {
			continue
		}
		sheetID := position + 2
		section := calculationBasisSections[index]
		name := infographicSheetName(position+1, section.Name)
		sheets.WriteString(fmt.Sprintf(`<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, reportXMLText(name), sheetID, sheetID))
		rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, sheetID, sheetID))
		contentTypes.WriteString(fmt.Sprintf(`<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, sheetID))
		sheetModel := ""
		if len(indexes) == 1 {
			sheetModel = modelCode
		}
		files[fmt.Sprintf("xl/worksheets/sheet%d.xml", sheetID)] = calculationWorkbookSheet(pass, index, sheetModel)
		missing := calculationMissingData(section.Code, pass.Groups[section.Code])
		missingText := "нет по минимальному перечню"
		if len(missing) > 0 {
			missingText = strings.Join(missing, "; ")
		}
		row := position + 6
		summary.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s</row>`, row, xlsxInlineCell("A"+strconv.Itoa(row), section.Name, 0), xlsxInlineCell("B"+strconv.Itoa(row), calculationRisk(section.Code, pass.Groups[section.Code]), 0), xlsxInlineCell("C"+strconv.Itoa(row), missingText, 0)))
	}
	contentTypes.WriteString(`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`)
	styleRel := len(indexes) + 2
	rels.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, styleRel))
	files["[Content_Types].xml"] = contentTypes.String()
	files["_rels/.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
	files["xl/workbook.xml"] = `<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` + sheets.String() + `</sheets><calcPr calcId="191029" fullCalcOnLoad="1" forceFullCalc="1"/></workbook>`
	files["xl/_rels/workbook.xml.rels"] = `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + rels.String() + `</Relationships>`
	files["xl/styles.xml"] = `<?xml version="1.0" encoding="UTF-8"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="4"><font><name val="Segoe UI"/><sz val="11"/></font><font><b/><color rgb="FFFFFFFF"/><name val="Segoe UI"/><sz val="14"/></font><font><b/><color rgb="FF0F3052"/><name val="Segoe UI"/><sz val="11"/></font><font><b/><color rgb="FFFFFFFF"/><name val="Segoe UI"/><sz val="11"/></font></fonts><fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF12385F"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFC4353D"/></patternFill></fill></fills><borders count="2"><border/><border><left style="thin"/><right style="thin"/><top style="thin"/><bottom style="thin"/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0"><alignment wrapText="1" vertical="top"/></xf><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0"><alignment wrapText="1" vertical="center"/></xf><xf numFmtId="0" fontId="2" fillId="0" borderId="1" xfId="0"><alignment wrapText="1" vertical="top"/></xf><xf numFmtId="0" fontId="3" fillId="3" borderId="1" xfId="0"><alignment wrapText="1" vertical="center"/></xf></cellXfs><cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles><dxfs count="1"><dxf><font><b/><color rgb="FF9C0006"/></font><fill><patternFill patternType="solid"><fgColor rgb="FFFFC7CE"/><bgColor indexed="64"/></patternFill></fill></dxf></dxfs></styleSheet>`
	files["xl/worksheets/sheet1.xml"] = `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="5" topLeftCell="A6" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="28" customWidth="1"/><col min="2" max="2" width="55" customWidth="1"/><col min="3" max="3" width="70" customWidth="1"/></cols><sheetData>` + summary.String() + `</sheetData><mergeCells count="1"><mergeCell ref="A1:C1"/></mergeCells></worksheet>`
	return writeReportZip(path, files)
}

func calculationExport(owner syscall.Handle, extension string) {
	pass := calculationBasisSelectedPass()
	selected := basisInitial
	if basisList != 0 {
		value, _, _ := pSendMessageW.Call(uintptr(basisList), LB_GETCURSEL, 0, 0)
		if int(value) >= 0 {
			selected = int(value)
		}
	}
	full := calculationBasisFullMode()
	modelCode := calculationBasisSelectedModel(selected)
	path := calculationChoosePath(owner, extension)
	if path == "" {
		return
	}
	var err error
	if extension == "docx" {
		err = writeLaboratoryDOCXReport(path, calculationPassReportWithModel(pass, selected, full, modelCode))
	} else {
		err = writeCalculationWorkbookXLSX(path, pass, calculationExportIndexes(full, selected), modelCode)
	}
	if err != nil {
		msg("Экспорт расчётов", "Не удалось сформировать файл: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if extension == "xlsx" {
		notifyDetachedExcelCopy(path)
	}
	openExternal(path)
}

func showCalculationBasis(index int) {
	basisInitial = index
	calculationBasisRefresh()
}

func layoutCalculationBasis(w, h int32) {
	if basisText == 0 || w < 900 || h < 620 {
		return
	}
	setPos(basisHeader, 0, 0, w, 62)
	setPos(basisAccent, 0, 62, w, 5)
	setPos(basisTitle, 24, 10, w-48, 44)
	setPos(basisModeCombo, 24, 88, 250, 180)
	setPos(basisModelCombo, 288, 88, 300, 420)
	setPos(basisPassCombo, 602, 88, w-626, 360)
	contentTop := int32(136)
	buttonsY := h - 64
	setPos(basisList, 24, contentTop, 250, buttonsY-contentTop-12)
	setPos(basisText, 288, contentTop, w-312, buttonsY-contentTop-12)
	buttonGap := int32(10)
	buttonW := (w - 48 - buttonGap*3) / 4
	setPos(basisRun, 24, buttonsY, buttonW, 42)
	setPos(basisExportDOCX, 24+buttonW+buttonGap, buttonsY, buttonW, 42)
	setPos(basisExportXLSX, 24+2*(buttonW+buttonGap), buttonsY, buttonW, 42)
	setPos(basisClose, 24+3*(buttonW+buttonGap), buttonsY, buttonW, 42)
}

func calculationBasisWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		h := syscall.Handle(hwnd)
		basisWindowHandle = h
		basisHeader = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1010, 62, h, 0)
		basisAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 62, 1010, 5, h, 0)
		basisTitle = modalCreate("STATIC", "РАСЧЁТЫ И ФОРМУЛЫ", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_CENTERIMAGE, 24, 10, 920, 44, h, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(basisTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		basisModeCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 24, 88, 250, 180, h, idBasisMode)
		for _, label := range []string{"Индивидуальная диагностика", "Полный расчёт по объекту"} {
			pSendMessageW.Call(uintptr(basisModeCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(label))))
		}
		pSendMessageW.Call(uintptr(basisModeCombo), CB_SETCURSEL, 0, 0)
		basisModelCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 288, 88, 300, 420, h, idBasisModel)
		calculationBasisPopulateModels(basisInitial)
		if basisInitial >= 0 && basisInitial < len(calculationBasisSections) && calculationBasisSections[basisInitial].Code == "scale" {
			for i, model := range laboratoryScaleModels {
				if model.Code == labScaleModelCode {
					pSendMessageW.Call(uintptr(basisModelCombo), CB_SETCURSEL, uintptr(i+1), 0)
					break
				}
			}
		}
		basisPassCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 602, 88, 362, 360, h, idBasisPass)
		selectedPass := 0
		objectCaption := strings.TrimSpace(getText(labObjectCaption))
		if objectCaption == "" || objectCaption == "Выберите объект на Главной" {
			objectCaption = strings.TrimSpace(calculationBasisSelectedPass().ObjectKey)
		}
		for i, pass := range basisPasses {
			prefix := "Сохранённый анализ"
			if i == 0 && pass.Status == labPassDraftStatus {
				prefix = "Текущие данные"
			}
			label := prefix + " · " + objectCaption + " · " + emptyLabText(pass.Date) + " · " + emptyLabText(pass.Source)
			pSendMessageW.Call(uintptr(basisPassCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(label))))
		}
		if len(basisPasses) > 0 {
			pSendMessageW.Call(uintptr(basisPassCombo), CB_SETCURSEL, uintptr(selectedPass), 0)
		}
		basisList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 24, 136, 250, 500, h, idBasisList)
		for _, section := range calculationBasisSections {
			pSendMessageW.Call(uintptr(basisList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(section.Name))))
		}
		basisText = modalCreate("RichEdit20W", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL|ES_READONLY, 288, 136, 676, 500, h, 0)
		if hFont != 0 {
			pSendMessageW.Call(uintptr(basisText), WM_SETFONT, uintptr(hFont), 1)
		}
		basisRun = modalCreate("BUTTON", "Выполнить расчёт", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 650, 220, 42, h, idBasisRun)
		basisExportDOCX = modalCreate("BUTTON", "Выгрузить в DOCX", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 252, 650, 220, 42, h, idBasisExportDOCX)
		basisExportXLSX = modalCreate("BUTTON", "Выгрузить в Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 480, 650, 220, 42, h, idBasisExportXLSX)
		basisClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 744, 650, 220, 42, h, idBasisClose)
		pSendMessageW.Call(uintptr(basisList), 0x0186, uintptr(basisInitial), 0)
		var client RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		layoutCalculationBasis(client.Right-client.Left, client.Bottom-client.Top)
		showCalculationBasisPreview()
		return 0
	case WM_SIZE:
		layoutCalculationBasis(int32(loWord(lParam)), int32(hiWord(lParam)))
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil && (int(di.CtlID) == idBasisClose || int(di.CtlID) == idBasisRun || int(di.CtlID) == idBasisExportDOCX || int(di.CtlID) == idBasisExportXLSX) {
			br := brushHeader
			if di.ItemState&ODS_SELECTED != 0 {
				br = brushRedPressed
			}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(br))
			pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
			pSetTextColor.Call(uintptr(di.HDC), rgb(255, 255, 255))
			old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(getText(di.HwndItem)))), ^uintptr(0), uintptr(unsafe.Pointer(&di.RcItem)), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			return 1
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		if child == basisTitle {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		}
		if child == basisHeader {
			return uintptr(brushNav)
		}
		if child == basisAccent {
			return uintptr(brushHeader)
		}
	case wmBasisCalculationDone:
		if basisCalculationDone != nil {
			select {
			case result := <-basisCalculationDone:
				if basisText != 0 {
					if len(result.Text) > 24000 {
						setText(basisText, result.Text)
					} else {
						formatComplexText(basisText, result.Text)
					}
				}
				setCalculationBasisBusy(false)
			default:
			}
		}
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idBasisMode, idBasisPass, idBasisModel:
			if hiWord(wParam) == 1 {
				setCalculationBasisBusy(false)
				showCalculationBasisPreview()
			}
		case idBasisList:
			if hiWord(wParam) == 1 {
				index, _, _ := pSendMessageW.Call(uintptr(basisList), 0x0188, 0, 0)
				basisInitial = int(index)
				calculationBasisPopulateModels(int(index))
				if int(index) >= 0 && int(index) < len(calculationBasisSections) && calculationBasisSections[int(index)].Code == "scale" {
					for i, model := range laboratoryScaleModels {
						if model.Code == labScaleModelCode {
							pSendMessageW.Call(uintptr(basisModelCombo), CB_SETCURSEL, uintptr(i+1), 0)
							break
						}
					}
				}
				showCalculationBasisPreview()
			}
		case idBasisRun:
			calculationBasisRefresh()
		case idBasisExportDOCX:
			calculationExport(syscall.Handle(hwnd), "docx")
		case idBasisExportXLSX:
			calculationExport(syscall.Handle(hwnd), "xlsx")
		case idBasisClose:
			basisDialogDone = true
			basisCalculationBusy = false
			basisWindowHandle = 0
			pDestroyWindowAuth.Call(hwnd)
		}
		return 0
	case labMappingClose:
		basisDialogDone = true
		basisCalculationBusy = false
		basisWindowHandle = 0
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openLaboratoryCalculationBasisDialog() {
	// RC556: never persist a draft and never scan/sort the whole laboratory
	// archive before the window exists. The first item is a zero-write snapshot
	// of what the technologist currently sees; recent saved analyses already in
	// the visible history are added from memory only.
	current := calculationBasisCurrentSnapshot()
	basisPasses = []LabPassRecord{current}
	seen := map[string]bool{}
	for _, entry := range labHistoryEntries {
		if len(basisPasses) >= 41 {
			break
		}
		if entry.Kind != "pass" || entry.Pass.Status != labPassSavedStatus {
			continue
		}
		pass := entry.Pass
		if !laboratoryObjectKeysEquivalent(pass.ObjectKey, current.ObjectKey) || strings.TrimSpace(pass.PassID) == "" || seen[pass.PassID] {
			continue
		}
		seen[pass.PassID] = true
		basisPasses = append(basisPasses, pass)
	}
	basisInitial = 0
	for i, section := range calculationBasisSections {
		if section.Code == labActiveGroup {
			basisInitial = i
			break
		}
	}
	basisDialogDone = false
	basisWindowHandle = 0
	basisCalculationBusy = false
	basisCalculationDone = make(chan calculationBasisAsyncResult, 1)
	cls := p("ROSALaboratoryCalculationBasisV3")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(calculationBasisWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — индивидуальная и комплексная диагностика"))), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1180, 800, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return
	}
	var message MSG
	for !basisDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	basisWindowHandle = 0
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
