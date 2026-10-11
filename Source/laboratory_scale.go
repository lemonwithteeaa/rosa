//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

type laboratoryScaleModel struct {
	Code, Name, Summary, Applicability, Limitation string
	Implemented                                    bool
}

var laboratoryScaleModels = []laboratoryScaleModel{
	{Code: "oddo", Name: "Оддо—Томсон / Oddo–Tomson", Summary: "Промысловая условная модель карбонатного и сульфатного солеобразования с учётом условий системы.", Applicability: "Нужна верифицированная реализация зависимостей температуры, давления, газовой фазы и состава.", Limitation: "В этой контрольной точке доступна как метод-план; численный результат не подменяется упрощённой формулой."},
	{Code: "stiff_davis", Name: "Стифф—Дэвис / Stiff–Davis", Summary: "Промысловый показатель стабильности карбоната кальция: сопоставляет рабочий pH и pH насыщения.", Applicability: "Карбонатное солеобразование при наличии полного шестикомпонентного анализа, pH и условий измерения.", Limitation: "Не заменяет сульфатные модели; реализация должна быть сверена с выбранной редакцией РД и тестовыми примерами."},
	{Code: "rd_caso4", Name: "РД: пересыщение CaSO₄", Summary: "Расчёт коэффициента пересыщения сульфата кальция по произведению активностей и произведению растворимости.", Applicability: "Пластовые и поверхностные воды и их смеси в области применимости выбранного РД.", Limitation: "Нужно явно фиксировать номер/редакцию РД, коэффициент активности, температуру и модификацию CaSO₄."},
	{Code: "pitzer", Name: "Питцер / PHREEQC", Summary: "Многокомпонентная модель Питцера / Pitzer для специфических ионных взаимодействий с термодинамической базой.", Applicability: "Предпочтительна для высокоминерализованных рассолов при полной химии и проверенной базе параметров.", Limitation: "Требует встроенного проверяемого ядра и версии базы; в этой контрольной точке численно не исполняется."},
	{Code: "davies", Name: "Дэвис: активности / IAP-Ksp", Summary: "Активности ионов оцениваются уравнением Дэвиса / Davies; Kнас — отношение ионного произведения активности к произведению растворимости (IAP/Ksp), ИН / SI = log10(Kнас).", Applicability: "Разбавленные и умеренно минерализованные воды; контрольный расчёт при 25 °C.", Limitation: "При ионной силе выше 0,5 моль/л результат считать ненадёжным; кинетику, смешение и количество осадка не определяет.", Implemented: true},
	{Code: "ideal", Name: "Идеальный раствор: IAP/Ksp", Summary: "Экспресс-оценка по концентрациям без поправок активности. Kнас — коэффициент насыщения; ИН / SI — индекс насыщения.", Applicability: "Только первичный ориентир для слабоминерализованной воды.", Limitation: "Для промысловых рассолов обычно завышает уверенность; нельзя применять как основной прогноз.", Implemented: true},
	{Code: "temperature_ideal", Name: "Температурный скрининг: IAP/Ksp", Summary: "Концентрационное ионное произведение сравнивается с температурно-зависимой константой растворимости из базы USGS PHREEQC 3.8.6-17100.", Applicability: "Отдельный предварительный расчёт при температуре пробы 0–100 °C и опорном давлении 1 атм.", Limitation: "Активности ионов, активность воды, давление, комплексообразование и изменение состава не рассчитываются; это не полноценный PHREEQC, Питцер или Оддо—Томсон.", Implemented: true},
	{Code: "carbonate_ph", Name: "Кальцит: равновесие по pH и HCO₃⁻", Summary: "Карбонат рассчитывается по равновесию HCO₃⁻ ⇌ H⁺ + CO₃²⁻ при температуре пробы; затем определяется насыщение кальцитом.", Applicability: "Разбавленная вода, pH и гидрокарбонат одной пробы при 0–100 °C, опорное давление 1 атм.", Limitation: "Концентрации приняты за активности свободных ионов; комплексообразование и давление не учитываются. Не является моделью Стифф—Дэвис или прогнозом для рассола.", Implemented: true},
	{Code: "carbonate_davies", Name: "Кальцит: равновесие Дэвиса, 25 °C", Summary: "Самосогласованная ионная сила и активности карбонатных форм с отдельными Na⁺/K⁺; насыщение кальцитом по той же базе PHREEQC.", Applicability: "25 °C; ионная сила не выше 0,1 моль/л; основные ионы заданы отдельно.", Limitation: "Свободные концентрации без комплексов, активность воды 1; не является полным PHREEQC, Стифф—Дэвис или Питцером.", Implemented: true},
	{Code: "carbonate_complex", Name: "Кальцит: общий Ca/C и комплексы", Summary: "По общему кальцию, неорганическому углероду и pH рассчитываются свободные формы и два карбонатных комплекса кальция.", Applicability: "25 °C; отдельные основные ионы; I ≤ 0,1 моль/л.", Limitation: "Только CaCO₃(aq)/CaHCO₃⁺; остальные комплексы и давление не учтены. Сокращённая модель, не полный PHREEQC.", Implemented: true},
	{Code: "multicomplex", Name: "Ca/Mg/Ba/Sr/C/S: 17 комплексов", Summary: "По общим Ca/Mg/C/S и опциональным Ba/Sr рассчитываются свободные ионы, 17 комплексов и насыщение кальцитом/гипсом/баритом/целестином.", Applicability: "25 °C; отдельные общие Ca/Mg/C/S, Na/K/Cl; Ba/Sr при наличии; I ≤ 0,1 моль/л.", Limitation: "Сокращённая химия и Дэвис; без давления, плотности и газовой фазы. Не полный PHREEQC/Питцер.", Implemented: true},
	{Code: "multicomplex_molal", Name: "Ca/Mg/Ba/Sr/C/S: моляльности", Summary: "17 комплексов на моляльной основе с расчётом массы и активности воды по формуле стандартного Davies-ядра PHREEQC.", Applicability: "25 °C; плотность той же водной пробы, масса растворённых веществ и отдельные общие анализы.", Limitation: "Дэвис, I ≤ 0,1 моль/кг воды; a(H₂O)=1−0,017·Σm не является Pitzer/SIT; без давления и газовой фазы. Не полный PHREEQC/Питцер.", Implemented: true},
	{Code: "water_mixing", Name: "Смешение двух вод: соли, 25 °C", Summary: "Общие анализы каждой воды в мг/л пересчитываются по массе воды, смешиваются и передаются в проверенное сокращённое ядро 17 комплексов. pH смеси задаётся по измерению, а не усредняется.", Applicability: "Обе воды и измерение pH смеси при 25 °C; плотность и полная масса растворённых веществ каждой воды; Дэвис, I ≤ 0,1 моль/кг воды.", Limitation: "Доля второй воды — доля объёма до смешения. Плотность и конечный объём смеси, тепловой баланс, газообмен, осаждение и его скорость не прогнозируются. Это не полный PHREEQC/Питцер.", Implemented: true},
	{Code: "co2_gas_water", Name: "CO₂ ↔ вода: проверенный учебный режим", Summary: "Связанное равновесие чистого CO₂ с электронейтральным фоном NaCl: внешний конечный резервуар либо замкнутый газовый объём.", Applicability: "Только 25 °C, P≤1 атм, чистый CO₂, Na=Cl≤0,1 моль/кг воды; без Ca/Mg/Ba/Sr, минералов, осаждения и кинетики.", Limitation: "Это проверенное сокращённое ядро для учебной постановки, а не промысловая газовая модель, не T/P-траектория и не прогноз солеотложения.", Implemented: true},
}

type laboratorySaturationResult struct {
	Mineral, Formula   string
	SI, Ratio          float64
	TemperatureC, LogK float64
	Partial            bool
}

func initializeLaboratoryScaleModels() {
	for index, model := range laboratoryScaleModels {
		pSendMessageW.Call(uintptr(labScaleModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(model.Name))))
		if model.Code == labScaleModelCode {
			pSendMessageW.Call(uintptr(labScaleModelCombo), CB_SETCURSEL, uintptr(index), 0)
		}
	}
	updateLaboratoryScaleModelInfo()
}

func setLaboratoryScaleModel(code string) {
	if code == "" {
		code = "davies"
	}
	labScaleModelCode = code
	if labScaleModelCombo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(labScaleModelCombo), CB_RESETCONTENT, 0, 0)
	selected := -1
	for index, model := range laboratoryScaleModels {
		pSendMessageW.Call(uintptr(labScaleModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(model.Name))))
		if model.Code == code {
			selected = index
		}
	}
	if selected < 0 {
		selected = len(laboratoryScaleModels)
		pSendMessageW.Call(uintptr(labScaleModelCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(laboratoryScaleModelByCode(code).Name))))
	}
	pSendMessageW.Call(uintptr(labScaleModelCombo), CB_SETCURSEL, uintptr(selected), 0)
}

func currentLaboratoryScaleModel() laboratoryScaleModel {
	return laboratoryScaleModelByCode(labScaleModelCode)
}

func laboratoryScaleModelByCode(code string) laboratoryScaleModel {
	if code == "" {
		code = "davies"
	} // Legacy records used Davies.
	for _, model := range laboratoryScaleModels {
		if model.Code == code {
			return model
		}
	}
	return laboratoryScaleModel{Code: code, Name: "Неизвестная модель: " + code, Limitation: "Код модели не поддерживается; доступен только явно обозначенный частичный концентрационный расчёт."}
}

func handleLaboratoryScaleModelChange(id int) bool {
	if id != ID_LAB_SCALE_MODEL {
		return false
	}
	saveLabDraft()
	index, _, _ := pSendMessageW.Call(uintptr(labScaleModelCombo), CB_GETCURSEL, 0, 0)
	if int(index) >= 0 && int(index) < len(laboratoryScaleModels) {
		labScaleModelCode = laboratoryScaleModels[index].Code
	}
	labFieldsPage = 0
	if labActiveGroup == "scale" {
		labPageSwitching = true
		switchLabGroup("scale", false)
		labPageSwitching = false
	}
	saveLabDraft()
	updateLaboratoryScaleModelInfo()
	if labActiveGroup == "scale" {
		setLaboratoryConclusion(buildLaboratoryConclusion(currentLabGroup(), canonicalizeLaboratoryValues(currentLabGroup(), currentLabValues(), currentLaboratoryUnits())))
	}
	return true
}

func scaleIonMolar(values map[string]string, key string, molarMass float64) (float64, bool) {
	mgL, ok := parseLabNumber(values[key])
	if !ok || mgL < 0 {
		return 0, false
	}
	return mgL / 1000 / molarMass, true
}

func laboratoryScaleIonicStrength(values map[string]string) (float64, int) {
	type ion struct {
		key          string
		mass, charge float64
	}
	ions := []ion{{"ca", 40.078, 2}, {"mg", 24.305, 2}, {"na_k", 22.990, 1}, {"so4", 96.06, -2}, {"co3", 60.01, -2}, {"hco3", 61.016, -1}, {"cl", 35.45, -1}, {"ba", 137.327, 2}, {"sr", 87.62, 2}}
	strength, used := 0.0, 0
	for _, ion := range ions {
		molar, ok := scaleIonMolar(values, ion.key, ion.mass)
		if !ok {
			continue
		}
		strength += 0.5 * molar * ion.charge * ion.charge
		used++
	}
	return strength, used
}

func laboratoryIonBalance(values map[string]string) (float64, bool) {
	type ion struct {
		key          string
		mass, charge float64
		cation       bool
	}
	ions := []ion{{"ca", 40.078, 2, true}, {"mg", 24.305, 2, true}, {"na_k", 22.990, 1, true}, {"ba", 137.327, 2, true}, {"sr", 87.62, 2, true}, {"so4", 96.06, 2, false}, {"co3", 60.01, 2, false}, {"hco3", 61.016, 1, false}, {"cl", 35.45, 1, false}}
	cations, anions, cCount, aCount := 0.0, 0.0, 0, 0
	for _, ion := range ions {
		molar, ok := scaleIonMolar(values, ion.key, ion.mass)
		if !ok {
			continue
		}
		equivalents := molar * math.Abs(ion.charge)
		if ion.cation {
			cations += equivalents
			cCount++
		} else {
			anions += equivalents
			aCount++
		}
	}
	if cCount < 2 || aCount < 2 || cations+anions == 0 {
		return 0, false
	}
	return 100 * (cations - anions) / (cations + anions), true
}

func daviesActivityCoefficient(charge, ionicStrength float64) float64 {
	if ionicStrength <= 0 {
		return 1
	}
	sqrtI := math.Sqrt(ionicStrength)
	logGamma := -0.509 * charge * charge * (sqrtI/(1+sqrtI) - 0.3*ionicStrength)
	return math.Pow(10, logGamma)
}

func laboratorySaturationResults(values map[string]string, modelCode string) ([]laboratorySaturationResult, float64, []string) {
	if modelCode == "water_mixing" {
		return laboratoryWaterMixingResults(values)
	}
	if modelCode == "multicomplex_molal" {
		return laboratoryMolalityResults(values)
	}
	if modelCode == "multicomplex" {
		return laboratoryMultiComplexResults(values)
	}
	if modelCode == "carbonate_complex" {
		return laboratoryCarbonComplexResults(values)
	}
	if modelCode == "carbonate_davies" {
		return laboratoryCarbonDaviesResults(values)
	}
	if modelCode == "carbonate_ph" {
		return laboratoryCarbonatePHResults(values)
	}
	temperatureModel := modelCode == "temperature_ideal"
	partial := modelCode != "davies" && modelCode != "ideal" && !temperatureModel
	activityModel := modelCode
	if partial {
		// This is an explicitly labelled concentration IAP/Ksp core, not a silent
		// substitution for the selected field or thermodynamic model.
		activityModel = "ideal"
	}
	ionicStrength, ionsUsed := laboratoryScaleIonicStrength(values)
	var warnings []string
	temperatureC := 25.0
	if temperatureModel {
		var ok bool
		temperatureC, ok = parseLabNumber(values["sample_temp"])
		if !ok || math.IsNaN(temperatureC) || math.IsInf(temperatureC, 0) {
			return nil, ionicStrength, []string{"Температура пробы не указана или некорректна; заполните её для температурного скрининга."}
		}
		if _, err := laboratoryTemperatureLogK("Кальцит", temperatureC); err != nil {
			return nil, ionicStrength, []string{err.Error()}
		}
		warnings = append(warnings, "Температурный скрининг использует концентрации вместо активностей, активность воды принята равной 1; давление и комплексообразование не учтены.")
	}
	if partial {
		warnings = append(warnings, "Выполнен частичный концентрационный расчёт IAP/Ksp. Поправки и зависимости выбранной модели ещё не применены; результат не является полным расчётом по этой модели.")
	}
	if ionsUsed < 4 {
		warnings = append(warnings, "Ионная сила оценена по неполному составу; заполните основные катионы и анионы.")
	}
	if activityModel == "davies" && ionicStrength > 0.5 {
		warnings = append(warnings, "Ионная сила выше 0,5 моль/л: Дэвис / Davies вне надёжной области; нужен Питцер / Pitzer или SIT с проверенной базой.")
	}
	type mineral struct {
		name, formula, cation, anion         string
		cMass, aMass, cCharge, aCharge, logK float64
	}
	minerals := []mineral{
		{"Кальцит", "CaCO₃", "ca", "co3", 40.078, 60.01, 2, -2, -8.48},
		{"Гипс", "CaSO₄·2H₂O", "ca", "so4", 40.078, 96.06, 2, -2, -4.58},
		{"Барит", "BaSO₄", "ba", "so4", 137.327, 96.06, 2, -2, -9.97},
		{"Целестин", "SrSO₄", "sr", "so4", 87.62, 96.06, 2, -2, -6.63},
	}
	var results []laboratorySaturationResult
	for _, mineral := range minerals {
		if modelCode == "stiff_davis" && mineral.name != "Кальцит" {
			continue
		}
		if modelCode == "rd_caso4" && mineral.name != "Гипс" {
			continue
		}
		cation, cOK := scaleIonMolar(values, mineral.cation, mineral.cMass)
		anion, aOK := scaleIonMolar(values, mineral.anion, mineral.aMass)
		if !cOK || !aOK || cation <= 0 || anion <= 0 {
			continue
		}
		if activityModel == "davies" {
			cation *= daviesActivityCoefficient(mineral.cCharge, ionicStrength)
			anion *= daviesActivityCoefficient(mineral.aCharge, ionicStrength)
		}
		logK := mineral.logK
		if temperatureModel {
			logK, _ = laboratoryTemperatureLogK(mineral.name, temperatureC)
		}
		si := math.Log10(cation*anion) - logK
		results = append(results, laboratorySaturationResult{Mineral: mineral.name, Formula: mineral.formula, SI: si, Ratio: math.Pow(10, si), Partial: partial, TemperatureC: temperatureC, LogK: logK})
	}
	if balance, ok := laboratoryIonBalance(values); ok && math.Abs(balance) > 5 {
		warnings = append(warnings, fmt.Sprintf("Невязка баланса зарядов %+.1f %% превышает 5 %%: сверить анализ до применения результата.", balance))
	}
	if temperature, ok := parseLabNumber(values["sample_temp"]); !temperatureModel && ok && math.Abs(temperature-25) > 2 {
		warnings = append(warnings, "Константы текущего скрининга приведены к 25 °C; фактическая температура отличается, поэтому нужен температурно-зависимый метод.")
	}
	return results, ionicStrength, warnings
}

func laboratorySaturationText(values map[string]string) string {
	return laboratorySaturationTextForModel(values, labScaleModelCode)
}

func laboratorySaturationTextForModel(values map[string]string, modelCode string) string {
	if modelCode == "co2_gas_water" {
		return "СВЯЗАННОЕ CO₂ ↔ ВОДА\r\n" + laboratoryCO2EquilibriumResultText(values) + "\r\nОграничение: этот режим не рассчитывает насыщение минералов и не является прогнозом солеотложения."
	}
	model := laboratoryScaleModelByCode(modelCode)
	results, ionicStrength, warnings := laboratorySaturationResults(values, model.Code)
	lines := []string{"МЕТОД КОЭФФИЦИЕНТА НАСЫЩЕНИЯ", "Модель: " + model.Name, "Что это за метод: " + laboratoryScaleMethodPlainExplanation(model.Code), model.Summary, "Область применения: " + model.Applicability, "Ограничение: " + model.Limitation}
	lines = append(lines, laboratoryScaleTemperaturePressureText(model.Code, values))
	if model.Code == "water_mixing" {
		lines = append(lines, laboratoryWaterMixingText(values))
	}
	lines = append(lines, "Статус расчёта: "+laboratoryScaleCalculationStatus(model, len(results), warnings)+".")
	if model.Code == "multicomplex_molal" && len(results) > 0 {
		lines = append(lines, laboratoryMolalityText(values), "База констант: "+laboratoryTemperatureDatabase)
	}
	if model.Code == "multicomplex" && len(results) > 0 {
		lines = append(lines, laboratoryMultiComplexText(values), "База констант: "+laboratoryTemperatureDatabase)
	}
	if model.Code == "temperature_ideal" || (model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex")) {
		lines = append(lines, "База констант: "+laboratoryTemperatureDatabase+".", "Условия: температура лабораторной пробы; опорное давление 1 атм; исходные концентрации не пересчитываются по траектории скважины.")
		if model.Code == "temperature_ideal" {
			if profile := laboratoryTemperatureProfileText(values); profile != "" {
				lines = append(lines, profile)
			}
		}
	}
	if (model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex")) && len(results) > 0 {
		if model.Code == "carbonate_davies" || model.Code == "carbonate_complex" {
			if model.Code == "carbonate_complex" {
				lines = append(lines, laboratoryCarbonComplexText(values))
			} else {
				lines = append(lines, laboratoryCarbonDaviesText(values))
			}
			if sol, err := laboratoryCarbonDaviesFromValues(values); model.Code != "carbonate_complex" && err == nil {
				lines = append(lines, laboratoryCarbonIndependentBalanceText(values, sol.Forms))
			}
		} else {
			lines = append(lines, laboratoryCarbonatePHBasis(values), laboratoryCarbonFormsText(values))
		}
	}
	strengthUnit := "моль/л"
	if model.Code == "multicomplex_molal" || model.Code == "water_mixing" {
		strengthUnit = "моль/кг воды"
	}
	lines = append(lines, fmt.Sprintf("Оценённая ионная сила: %.3g %s.", ionicStrength, strengthUnit))
	if balance, ok := laboratoryScaleModelIonBalance(values, model.Code); ok {
		lines = append(lines, fmt.Sprintf("Невязка баланса зарядов: %+.1f %%.", balance))
		if math.Abs(balance) > 5 {
			lines = append(lines, "Проверить: невязка превышает рабочий контрольный уровень 5 %; результат модели считать ограниченным до сверки анализа.")
		}
	} else {
		lines = append(lines, "Баланс зарядов не рассчитан: недостаточно основных катионов и анионов.")
	}
	for _, result := range results {
		state := "ненасыщено"
		if result.SI > 0 {
			state = "пересыщено термодинамически"
		} else if math.Abs(result.SI) <= 0.1 {
			state = "близко к насыщению"
		}
		basis := ""
		if model.Code == "temperature_ideal" || (model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex")) {
			basis = fmt.Sprintf("; T=%.2f °C; log K(T)=%.8f", result.TemperatureC, result.LogK)
		}
		if result.Partial {
			basis = "; частичное ядро без поправок выбранной модели"
		}
		lines = append(lines, fmt.Sprintf("• %s (%s): Кнас=%.3g; ИН / SI=%.3f — %s%s.", result.Mineral, result.Formula, result.Ratio, result.SI, state, basis))
	}
	if len(results) == 0 {
		lines = append(lines, "Кнас не рассчитан: недостаточно требуемой пары ионов для выбранной модели.")
	}
	for _, warning := range warnings {
		lines = append(lines, "Проверить: "+warning)
	}
	lines = append(lines, "Кнас > 1 (ИН / SI > 0) означает термодинамическое пересыщение, но не доказывает фактическое выпадение: нужны кинетика, время, смешение вод, температура/давление и проверка отложений.")
	return strings.Join(lines, "\r\n")
}

func laboratoryScaleTemperaturePressureText(modelCode string, values map[string]string) string {
	if modelCode == "co2_gas_water" {
		return "Учёт T/P: только фиксированные 25 °C и абсолютное давление чистого CO₂ до 1 атм. T/P-траектория скважины не рассчитывается."
	}
	if modelCode == "water_mixing" {
		return "Учёт T/P: обе исходные воды и смесь только при 25 °C; давление и газовая фаза в результат не включены. pH смеси — измеренный вход."
	}
	switch modelCode {
	case "oddo":
		return "Учёт T/P в текущей версии: НЕТ. Метод Оддо—Томсона выбран как метод-план, но численно не реализован; введённые температура и давление не участвуют в расчёте. Для вывода по Оддо—Томсону нужна проверенная реализация зависимостей T, P, газовой фазы и состава."
	case "temperature_ideal":
		return "Учёт T/P: температура пробы учитывается в Ksp. Если заполнены Tmin/P(Tmin) и Tmax/P(Tmax), дополнительно строятся три температурные контрольные точки профиля; давление сохраняется рядом, но численно НЕ применяется до внешней PHREEQC-сверки."
	case "carbonate_ph":
		return "Учёт T/P: температура пробы учитывается в Ksp; давление не рассчитывается, принято опорное давление 1 атм."
	case "davies", "ideal", "carbonate_davies", "carbonate_complex", "multicomplex", "multicomplex_molal":
		return "Учёт T/P: расчёт выполняется при 25 °C; фактическая температура и давление не учитываются."
	default:
		return "Учёт T/P: численная реализация выбранного метода отсутствует; температура и давление в результат не включены."
	}
}

func laboratoryScaleThreeLayerAssessment(values map[string]string, modelCode string) string {
	if modelCode == "co2_gas_water" {
		return "ТЕРМОДИНАМИКА / КОЛИЧЕСТВО / КИНЕТИКА\n• Термодинамика: этот режим оценивает газо-водное равновесие CO₂, а не насыщение минеральной фазой.\n• Возможное количество осадка: НЕ РАССЧИТЫВАЕТСЯ.\n• Кинетика / время индукции: НЕ РАССЧИТЫВАЕТСЯ."
	}
	model := laboratoryScaleModelByCode(modelCode)
	results, _, _ := laboratorySaturationResults(values, model.Code)
	thermo := "НЕ ОПРЕДЕЛЕНА — нет достаточного расчётного результата по минеральным фазам."
	if len(results) > 0 {
		var high, near, low []string
		for _, r := range results {
			switch {
			case r.SI > 0.1:
				high = append(high, fmt.Sprintf("%s SI=%+.2f", r.Mineral, r.SI))
			case r.SI >= -0.1:
				near = append(near, fmt.Sprintf("%s SI=%+.2f", r.Mineral, r.SI))
			default:
				low = append(low, fmt.Sprintf("%s SI=%+.2f", r.Mineral, r.SI))
			}
		}
		parts := []string{}
		if len(high) > 0 {
			parts = append(parts, "пересыщение: "+strings.Join(high, ", "))
		}
		if len(near) > 0 {
			parts = append(parts, "погранично: "+strings.Join(near, ", "))
		}
		if len(low) > 0 {
			parts = append(parts, "ненасыщенно: "+strings.Join(low, ", "))
		}
		thermo = strings.Join(parts, "; ")
	}
	return strings.Join([]string{
		"ТЕРМОДИНАМИКА / КОЛИЧЕСТВО / КИНЕТИКА",
		"• Термодинамика: " + thermo + ". SI показывает предпосылку к пересыщению при принятых условиях, но не факт образования осадка.",
		"• Возможное количество / масса осадка: НЕ РАССЧИТЫВАЕТСЯ этой моделью. Для оценки количества нужен отдельный валидированный равновесный/материальный расчёт с полным составом и условиями.",
		"• Кинетика / время индукции: НЕ РАССЧИТЫВАЕТСЯ. Без лабораторной кинетики, времени контакта, поверхности зародышеобразования и данных по ингибитору нельзя переводить SI в скорость или время выпадения.",
	}, "\n")
}

func laboratoryScaleDecisionText(values map[string]string, modelCode string) string {
	if modelCode == "co2_gas_water" {
		return laboratoryCO2EquilibriumDecisionText(values)
	}
	model := laboratoryScaleModelByCode(modelCode)
	results, _, warnings := laboratorySaturationResults(values, model.Code)
	completeSix := laboratoryScaleSixComponentReady(values) && modelCode != "water_mixing"
	lines := []string{"ТЕХНОЛОГИЧЕСКИЙ ИТОГ ПО СОЛЕОТЛОЖЕНИЯМ", "Выбранная модель: " + model.Name + ".", "Что это за метод: " + laboratoryScaleMethodPlainExplanation(model.Code), laboratoryScaleTemperaturePressureText(model.Code, values)}
	if completeSix {
		lines = append(lines, "Статус исходных данных: минерализация, pH, температура и шестикомпонентный ионный состав заполнены. Формируется ИТОГОВЫЙ ОСТОРОЖНЫЙ ПРОГНОЗ; результат требует уточнения по T/P, смешению вод, кинетике и фактическим отложениям.")
	}
	if !model.Implemented {
		if completeSix && len(results) > 0 {
			lines = append(lines, "Выбранная методика численно не реализована полностью. Ниже дан осторожный концентрационный прогноз IAP/Ksp; применение именно выбранной методики требует уточнения.")
		} else {
			lines = append(lines,
				"Вероятность выпадения: НЕ ОПРЕДЕЛЕНА выбранным методом — численный расчёт этой модели пока не реализован.",
				"Что это означает: нельзя делать вывод «соли выпадут» или «соли не выпадут» только по концентрации одного иона.",
				"Что требуется: полный согласованный анализ воды, фактические температура и давление по траектории, газовая фаза/CO₂, условия смешения вод и проверенная реализация выбранной методики.")
			return strings.Join(lines, "\n")
		}
	}
	if len(results) == 0 {
		lines = append(lines,
			"Вероятность выпадения: НЕ ОПРЕДЕЛЕНА — нет требуемой пары ионов для расчёта хотя бы одного минерала.",
			"Что требуется: заполнить катион и соответствующий анион, а для карбонатов также pH и условия температуры/давления согласно области выбранной модели.")
		return strings.Join(lines, "\n")
	}
	var supersaturated, near, undersaturated []string
	for _, result := range results {
		name := fmt.Sprintf("%s (%s, ИН %.2f)", result.Mineral, result.Formula, result.SI)
		switch {
		case result.SI > 0.1:
			supersaturated = append(supersaturated, name)
		case result.SI >= -0.1:
			near = append(near, name)
		default:
			undersaturated = append(undersaturated, name)
		}
	}
	if len(supersaturated) > 0 {
		lines = append(lines, "Вероятность выпадения: ТЕРМОДИНАМИЧЕСКАЯ ПРЕДПОСЫЛКА ВЫЯВЛЕНА для "+strings.Join(supersaturated, ", ")+
			". Положительный индекс насыщения означает пересыщение, но не доказывает фактическое выпадение без учёта кинетики, времени, поверхности зародышеобразования и изменения условий по траектории.")
	} else if len(near) > 0 {
		lines = append(lines, "Вероятность выпадения: ПОГРАНИЧНАЯ для "+strings.Join(near, ", ")+". Небольшое изменение состава, T/P, pH или смешение вод может изменить знак индекса.")
	} else {
		lines = append(lines, "Вероятность выпадения: по рассчитанным минералам ТЕРМОДИНАМИЧЕСКАЯ ПРЕДПОСЫЛКА НЕ ВЫЯВЛЕНА при принятых условиях. Это не исключает другие соли и другие точки T/P по траектории.")
	}
	if len(undersaturated) > 0 {
		lines = append(lines, "Ненасыщенные по расчёту минералы: "+strings.Join(undersaturated, ", ")+".")
	}
	lines = append(lines, laboratoryScaleThreeLayerAssessment(values, modelCode))
	confidence := "рабочая"
	if len(warnings) > 0 {
		confidence = "ограниченная"
	}
	lines = append(lines, "Надёжность оценки: "+confidence+fmt.Sprintf("; предупреждений модели: %d.", len(warnings)), laboratoryWaterQualityQCText(values), "Следующий шаг: проверить баланс анализа, фактические T/P и смешение вод; при наличии отложений сопоставить расчёт с их минералогическим составом.")
	return strings.Join(lines, "\n")
}

// Six-component water chemistry: Ca, Mg, Na+K, HCO3, SO4 and Cl. Separate
// Na/K values are accepted instead of the combined result.
func laboratoryScaleSixComponentReady(values map[string]string) bool {
	for _, key := range []string{"mineralization", "ph", "sample_temp", "ca", "mg", "hco3", "so4", "cl"} {
		value, ok := parseLabNumber(values[key])
		if !ok || value < 0 {
			return false
		}
	}
	if value, ok := parseLabNumber(values["na_k"]); ok && value >= 0 {
		return true
	}
	na, naOK := parseLabNumber(values["na"])
	k, kOK := parseLabNumber(values["k"])
	return naOK && kOK && na >= 0 && k >= 0
}

func laboratoryScaleCalculationStatus(model laboratoryScaleModel, count int, warnings []string) string {
	if count == 0 {
		if (model.Code == "multicomplex" || model.Code == "multicomplex_molal" || model.Code == "water_mixing") && len(warnings) > 0 {
			return "НЕ РАССЧИТАНО: " + warnings[0]
		}
		if (model.Code == "temperature_ideal" || (model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex"))) && len(warnings) > 0 && (strings.HasPrefix(warnings[0], "Температура") || strings.HasPrefix(warnings[0], "Для карбонатного")) {
			return "НЕ РАССЧИТАНО: " + warnings[0]
		}
		return "НЕ РАССЧИТАНО: нет требуемой пары ионов"
	}
	if !model.Implemented {
		return "ЧАСТИЧНЫЙ РАСЧЁТ: концентрационный IAP/Ksp"
	}
	if len(warnings) > 0 {
		return "ОГРАНИЧЕННЫЙ РАСЧЁТ: проверить условия применимости"
	}
	return "Скрининговый расчёт при 25 °C"
}

func laboratoryScaleMethodPlainExplanation(modelCode string) string {
	switch modelCode {
	case "oddo":
		return "Метод Оддо—Томсона — промысловый способ оценить склонность воды к солеобразованию с учётом условий системы. В этой версии он показан как метод-план: полноценный численный расчёт по Оддо—Томсону пока не выполняется, поэтому приложение не подменяет его упрощённой формулой."
	case "stiff_davis":
		return "Стифф—Дэвис — промысловый показатель устойчивости воды по карбонату кальция. Он сопоставляет фактический pH воды с расчётным pH насыщения и помогает понять, есть ли термодинамическая предпосылка к карбонатным отложениям. Это не метод для всех типов солей."
	case "rd_caso4":
		return "Метод по РД для CaSO₄ сравнивает фактическое ионное произведение кальция и сульфата с растворимостью соответствующей формы сульфата кальция. По сути он отвечает, есть ли термодинамическая предпосылка к пересыщению CaSO₄ в заданных условиях."
	case "pitzer":
		return "Модель Питцера учитывает специфические взаимодействия ионов и предназначена прежде всего для высокоминерализованных рассолов, где простые поправки активности уже недостаточны. Для надёжного расчёта нужна проверенная термодинамическая база; в этой версии численно не исполняется."
	case "davies":
		return "Метод Дэвиса корректирует введённые концентрации с учётом электростатического взаимодействия ионов, затем сравнивает ионное произведение активности с произведением растворимости. Он удобен как предварительный расчёт для разбавленных и умеренно минерализованных вод, но не предназначен для крепких рассолов."
	case "ideal":
		return "Идеальный раствор — самый простой скрининг: концентрации принимаются как активности без поправок на взаимодействие ионов. Он годится только для грубой первичной оценки и не должен использоваться как основной прогноз для промыслового рассола."
	case "temperature_ideal":
		return "Температурный IAP/Ksp-скрининг пересчитывает константу растворимости с температурой и сравнивает её с концентрационным ионным произведением. Он показывает влияние температуры на термодинамическую предпосылку, но не учитывает давление, комплексообразование и изменение состава по стволу."
	case "carbonate_ph":
		return "Карбонатный расчёт по pH и HCO₃⁻ оценивает, сколько карбонат-иона CO₃²⁻ соответствует измеренным pH и гидрокарбонату, после чего рассчитывает насыщение кальцитом. Это упрощённая равновесная модель одной пробы, а не полный расчёт рассола."
	case "carbonate_davies":
		return "Карбонатная модель Дэвиса совместно уточняет ионную силу и активности карбонатных форм, а затем оценивает насыщение кальцитом. Она точнее простого концентрационного расчёта, но применима только при невысокой ионной силе."
	case "carbonate_complex":
		return "Модель общего Ca/C с комплексами разделяет общий кальций и неорганический углерод на свободные формы и несколько растворённых комплексов. Это позволяет реалистичнее оценить свободные ионы для кальцита, но химическая схема остаётся сокращённой."
	case "multicomplex":
		return "Многокомпонентное сокращённое ядро распределяет Ca, Mg, Ba, Sr, углерод и серу между свободными ионами и 17 растворёнными комплексами, после чего оценивает насыщение кальцитом, гипсом, баритом и целестином. Это расширенный скрининг, но не полный PHREEQC/Питцер."
	case "multicomplex_molal":
		return "Моляльная многокомпонентная модель выполняет тот же расширенный расчёт 17 комплексов, но переводит состав к моляльностям и отдельно оценивает активность воды. Такой подход физически лучше согласован с термодинамическими расчётами, однако остаётся сокращённым и ограничен невысокой ионной силой."
	case "water_mixing":
		return "Модель смешения двух вод сначала переводит анализы обеих вод к количеству вещества, затем смешивает их в заданной доле и пересчитывает свободные и комплексные формы. pH смеси должен быть измерен отдельно: простое усреднение pH химически некорректно."
	case "co2_gas_water":
		return "CO₂ ↔ вода — отдельный учебный равновесный расчёт газ–вода. Он показывает, как чистый CO₂ распределяется между газовой и водной фазой в строго ограниченной постановке; минералы и риск солеотложения этим режимом не рассчитываются."
	default:
		return "Описание выбранного метода пока не подготовлено. Используйте область применимости и ограничения как обязательную часть интерпретации результата."
	}
}

func laboratoryScalePanelText(values map[string]string) string {
	return laboratoryScalePanelTextForModel(values, labScaleModelCode)
}

func laboratoryScalePanelTextForModel(values map[string]string, modelCode string) string {
	if modelCode == "co2_gas_water" {
		return laboratoryCO2EquilibriumPanelText(values)
	}
	model := laboratoryScaleModelByCode(modelCode)
	results, _, warnings := laboratorySaturationResults(values, model.Code)
	status := laboratoryScaleCalculationStatus(model, len(results), warnings)
	lines := []string{"МЕТОД", model.Name, "ЧТО ЭТО ЗА МЕТОД", laboratoryScaleMethodPlainExplanation(model.Code), "СТАТУС", status, "QC ВОДЫ", laboratoryWaterQualityQCShort(values)}
	if model.Code == "temperature_ideal" || (model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex")) {
		if temp, ok := parseLabNumber(values["sample_temp"]); ok {
			lines = append(lines, "УСЛОВИЯ РАСЧЁТА", fmt.Sprintf("Температура пробы: %.2f °C; опорное давление 1 атм.", temp))
		}
	}
	if len(results) == 0 {
		if !((model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex")) && len(warnings) > 0) && !strings.HasPrefix(status, "НЕ РАССЧИТАНО: Температура") {
			lines = append(lines, "Кнас: нет данных для требуемой пары ионов")
		}
	} else {
		lines = append(lines, "ДОСТУПНЫЙ РЕЗУЛЬТАТ")
		if model.Code == "multicomplex_molal" {
			lines = append(lines, laboratoryMolalityText(values))
		}
		if model.Code == "multicomplex" {
			lines = append(lines, laboratoryMultiComplexText(values))
		}
		if model.Code == "carbonate_ph" || (model.Code == "carbonate_davies" || model.Code == "carbonate_complex") {
			if model.Code == "carbonate_davies" || model.Code == "carbonate_complex" {
				if model.Code == "carbonate_complex" {
					lines = append(lines, laboratoryCarbonComplexText(values))
				} else {
					lines = append(lines, laboratoryCarbonDaviesText(values))
				}
				if sol, err := laboratoryCarbonDaviesFromValues(values); model.Code != "carbonate_complex" && err == nil {
					lines = append(lines, laboratoryCarbonIndependentBalanceText(values, sol.Forms))
				}
			} else {
				lines = append(lines, laboratoryCarbonatePHBasis(values), laboratoryCarbonFormsText(values))
			}
		}
		for _, result := range results {
			lines = append(lines, fmt.Sprintf("%s: Кнас %.3g; ИН / SI %.3f", result.Mineral, result.Ratio, result.SI))
		}
	}
	lines = append(lines, "РАЗДЕЛЕНИЕ РЕЗУЛЬТАТА", laboratoryScaleThreeLayerAssessment(values, modelCode))
	if model.Code == "water_mixing" {
		lines = append(lines, "ДИАПАЗОНЫ ПОВЫШЕННОГО ТЕРМОДИНАМИЧЕСКОГО РИСКА", laboratoryMixingRiskText(values))
	}
	if len(warnings) > 0 {
		lines = append(lines, "ОГРАНИЧЕНИЕ")
		for _, warning := range warnings {
			lines = append(lines, "• "+warning)
		}
	}
	return strings.Join(lines, "\r\n")
}

func updateLaboratoryScaleModelInfo() {
	if labScaleModelInfo == 0 {
		return
	}
	values := map[string]string{}
	if labActiveGroup == "scale" {
		values = canonicalizeLaboratoryValues(currentLabGroup(), currentLabValues(), currentLaboratoryUnits())
	}
	text := laboratoryScalePanelText(values)
	setText(labScaleModelInfo, text)
	formatLaboratoryInfoPanel(labScaleModelInfo, text)
}
