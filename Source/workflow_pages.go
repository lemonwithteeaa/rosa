package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Primary workflow now includes the dedicated MRP forecast step before economics; secondary engineering tools remain independent pages.
// Secondary engineering tools (telemetry import, OPI/OPR, technology bank, data/settings,
// registers) remain independent pages and are opened from the relevant workflow step.
const (
	ID_TECHPARAM = 116
	ID_ECON      = 117
	ID_RESULT    = 118

	ID_NAV_TELEMETRY      = 270
	ID_NAV_DATA           = 271
	ID_NAV_TECHBANK       = 272
	ID_NAV_OPI            = 273
	ID_NAV_REGISTRIES     = 274
	ID_ECON_USE_SELECTED  = 275
	ID_ECON_CALCULATE     = 276
	ID_RESULT_REFRESH     = 277
	ID_TECHPROFILE_SAVE   = 278
	ID_NAV_DYNAMOMETRY    = 279
	ID_ECON_METHOD_NEXT   = 280
	ID_ECON_METHOD_BASE   = 281
	ID_ECON_BASIC         = 282
	ID_ECON_EXTENDED      = 283
	ID_ECON_CALCULATE_ALL = 9750
	ID_ECON_FULL_RESULT   = 9759
	ID_ECON_CHART         = 9762
	ID_ECON_TEMPLATE_SAVE = 9780
	ID_ECON_TEMPLATE_LOAD = 9781
	ID_ECON_FUND_FILTER   = 9782
)

var (
	techParamTitle                            syscall.Handle
	techParamDescription                      syscall.Handle
	techParamObjectInfo                       syscall.Handle
	techParamDetail, techParamDetailSecondary syscall.Handle
	techProfileLabels                         [10]syscall.Handle
	techProfileEdits                          [10]syscall.Handle
	techProfileUnits                          [10]syscall.Handle
	techProfileSave                           syscall.Handle
	techProfileSource                         syscall.Handle
	techProfileBoundKey                       string
	techActionButtons                         [4]syscall.Handle

	econPageTitle          syscall.Handle
	econObjectLabel        syscall.Handle
	econCurrentMRP         syscall.Handle
	econMRPLow             syscall.Handle
	econMRPBase            syscall.Handle
	econMRPHigh            syscall.Handle
	econScenario           syscall.Handle
	econHorizon            syscall.Handle
	econRepairDays         syscall.Handle
	econRepairCost         syscall.Handle
	econAnnualOPEX         syscall.Handle
	econInflation          syscall.Handle
	econOneTimeMTR         syscall.Handle
	econResult             syscall.Handle
	econBoundObjectKey     string
	econBoundDecisionID    string
	econFieldLabels        []syscall.Handle
	econUseObjectButton    syscall.Handle
	econCalculateButton    syscall.Handle
	econCalculateAllButton syscall.Handle
	econFullResultButton   syscall.Handle
	econChartButton        syscall.Handle
	econScenarioCard       syscall.Handle
	econMethodNextButton   syscall.Handle
	econMethodBaseButton   syscall.Handle
	econBasicButton        syscall.Handle
	econExtendedButton     syscall.Handle
	econModeInfo           syscall.Handle
	econDecisionStatus     syscall.Handle
	econDeferralInfo       syscall.Handle
	econTemplateSaveButton syscall.Handle
	econTemplateLoadButton syscall.Handle

	resultSummary syscall.Handle
)

func buildWorkflowPagesUI() {
	buildTechParametersUI()
	buildMRPForecastUI()
	buildEconomicsUI()
	buildResultUI()
}

func buildTechParametersUI() {
	// Restored to the last stable "Состояние ГНО" geometry.  The split lower
	// panels introduced in the previous pass made the screen visually drift from
	// the established object/passport pages and reduced useful vertical space.
	techParamTitle = pageTitle(ID_TECHPARAM, "Состояние оборудования", 242, 94, 970, 34)
	techParamDescription = label(ID_TECHPARAM, "Фактическая компоновка, Техрежим и диагностика оборудования выбранной скважины. Подбор ГНО выполняется в «Автотехнологе».", 242, 132, 1010, 36)
	techParamObjectInfo = label(ID_TECHPARAM, "Объект не выбран.", 242, 172, 1010, 28)
	if techParamObjectInfo != 0 && hFontHeaderBold != 0 {
		pSendMessageW.Call(uintptr(techParamObjectInfo), WM_SETFONT, uintptr(hFontHeaderBold), 1)
	}
	techActionButtons[0] = button(ID_TECHPARAM, "Мониторинг состояния ГНО", 242, 210, 250, 48, ID_NAV_TELEMETRY)
	techActionButtons[1] = button(ID_TECHPARAM, "Динамометрический контроль", 502, 210, 250, 48, ID_NAV_DYNAMOMETRY)
	techActionButtons[2] = button(ID_TECHPARAM, "Импорт Техрежима", 762, 210, 230, 48, ID_NAV_DATA)
	techActionButtons[3] = button(ID_TECHPARAM, "Методы и технологии", 1002, 210, 250, 48, ID_NAV_TECHBANK)
	profileFields := installationProfileFieldDefinitions()
	for i, field := range profileFields {
		col, row := i/5, i%5
		x := int32(242 + col*510)
		y := int32(270 + row*36)
		techProfileLabels[i] = label(ID_TECHPARAM, field.label, x, y, 205, 26)
		techProfileEdits[i] = edit(ID_TECHPARAM, "", x+210, y-3, 170, 29, false, false, 0)
		techProfileUnits[i] = label(ID_TECHPARAM, field.unit, x+386, y, 110, 26)
	}
	techProfileSave = button(ID_TECHPARAM, "Сохранить фактическую компоновку", 242, 454, 300, 36, ID_TECHPROFILE_SAVE)
	techProfileSource = label(ID_TECHPARAM, "Фактическая компоновка не сохранена.", 556, 460, 696, 26)
	techParamDetail = edit(ID_TECHPARAM, "Выберите объект на шаге 1.", 242, 504, 1010, 210, true, true, 0)
	techParamDetailSecondary = 0
}

type installationProfileFieldDefinition struct{ label, unit string }

func installationProfileFieldDefinitions() []installationProfileFieldDefinition {
	return []installationProfileFieldDefinition{
		{"Тип оборудования", "текст"}, {"Модель насоса", "текст"}, {"Модель привода", "текст"},
		{"Диаметр НКТ", "мм"}, {"Глубина спуска насоса", "м"}, {"Длина хода", "м"},
		{"Число качаний", "мин⁻¹"}, {"Номинальный ток", "А"},
		{"Номинальная подача", "м³/сут"}, {"Диаметр штанг", "мм"},
	}
}

func buildEconomicsUI() {
	econPageTitle = pageTitle(ID_ECON, "Экономическая оценка", 242, 94, 970, 34)
	econBasicButton = button(ID_ECON, "Базовая", 242, 130, 170, 34, ID_ECON_BASIC)
	econExtendedButton = button(ID_ECON, "Расширенная", 422, 130, 190, 34, ID_ECON_EXTENDED)
	econModeInfo = label(ID_ECON, "BENEFIT MODELING · УПРОЩЁННОЕ СРАВНЕНИЕ ПАРАЛЛЕЛЬНЫХ СХЕМ", 242, 132, 1010, 24)
	econDecisionStatus = create("STATIC", "Связь с решением: утверждённое решение отсутствует", WS_CHILD|SS_LEFT|SS_CENTERIMAGE|SS_ENDELLIPSIS|SS_NOPREFIX, 242, py(158), 620, 26, hMain, 0)
	addPageControl(ID_ECON, econDecisionStatus)
	econDeferralInfo = create("STATIC", "Отсрочка ремонта, мес.: —", WS_CHILD|SS_RIGHT|SS_CENTERIMAGE|SS_NOPREFIX, 870, py(158), 382, 26, hMain, 0)
	addPageControl(ID_ECON, econDeferralInfo)
	if econDecisionStatus != 0 && hFontHeaderSmall != 0 {
		pSendMessageW.Call(uintptr(econDecisionStatus), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	}
	if econDeferralInfo != 0 && hFontHeaderSmall != 0 {
		pSendMessageW.Call(uintptr(econDeferralInfo), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	}
	econObjectLabel = create("STATIC", "Объект не выбран.", WS_CHILD|SS_LEFT|SS_CENTERIMAGE|SS_ENDELLIPSIS|SS_NOPREFIX, 242, py(176), 760, 38, hMain, 0)
	addPageControl(ID_ECON, econObjectLabel)
	if econObjectLabel != 0 && hFontHeaderSmall != 0 {
		pSendMessageW.Call(uintptr(econObjectLabel), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	}
	econUseObjectButton = button(ID_ECON, "Подставить выбранный объект", 1010, 170, 242, 34, ID_ECON_USE_SELECTED)
	econScenarioCard = edit(ID_ECON, "Сценарий мероприятия ещё не сформирован.", 242, 208, 760, 80, true, true, 0)
	econMethodNextButton = button(ID_ECON, "Следующая ›", 1010, 208, 242, 36, ID_ECON_METHOD_NEXT)
	econMethodBaseButton = button(ID_ECON, "Схема А", 1010, 252, 242, 36, ID_ECON_METHOD_BASE)
	fields := []struct {
		title, initial string
		handle         *syscall.Handle
	}{
		{"Текущий МРП, сут.", "", &econCurrentMRP},
		{"Прогноз МРП — осторожный, сут.", "", &econMRPLow},
		{"Прогноз МРП — центральный, сут.", "", &econMRPBase},
		{"Прогноз МРП — благоприятный, сут.", "", &econMRPHigh},
		{"Сценарий: осторожный / центральный / благоприятный", "Центральный", &econScenario},
		{"Горизонт, полных месяцев", "36", &econHorizon},
		{"Длительность ремонта, сут.", "", &econRepairDays},
		{"Стоимость одного ремонта, руб.", "", &econRepairCost},
		{"Эксплуатационные затраты (OPEX), руб./год", "", &econAnnualOPEX},
		{"Инфляция затрат, %/год", "0", &econInflation},
		{"Капитальные / разовые затраты (CAPEX), руб.", "", &econOneTimeMTR},
	}
	econFieldLabels = nil
	for i, f := range fields {
		x, y, width := economicsFieldPosition(i, 1010)
		econFieldLabels = append(econFieldLabels, label(ID_ECON, f.title, x, y, width, 36))
		*f.handle = edit(ID_ECON, f.initial, x, y+38, width, 30, false, false, 0)
	}
	x, y, width := economicsFieldPosition(11, 1010)
	econCalculateButton = button(ID_ECON, "Текущая схема", x, y+34, (width-8)/2, 36, ID_ECON_CALCULATE)
	econCalculateAllButton = button(ID_ECON, "РАССЧИТАТЬ СХЕМЫ", x+(width-8)/2+8, y+34, width-(width-8)/2-8, 36, ID_ECON_CALCULATE_ALL)
	econFullResultButton = button(ID_ECON, "Полный вывод ›", x, y+80, 230, 40, ID_ECON_FULL_RESULT)
	econChartButton = button(ID_ECON, "Диаграмма ›", x+240, y+80, 220, 40, ID_ECON_CHART)
	econResult = edit(ID_ECON, "Заполните исходные данные. Неизвестные значения не заменяйте нулём.", 242, 512, 1010, 200, true, true, 0)
	buildEconomicsVisualReference()
}

func buildResultUI() {
	pageTitle(ID_RESULT, "07  Итоговый отчёт", 242, 94, 970, 34)
	resultRefreshButton = button(ID_RESULT, "Обновить сводку", 1062, 94, 190, 34, ID_RESULT_REFRESH)
	buildResultWorkspaceUI()
}

func parseFloatUI(h syscall.Handle, field string, required bool) (float64, error) {
	return parseEconomicNumber(getText(h), field, required)
}

func currentSelectedState() (TechObjectState, bool) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return TechObjectState{}, false
	}
	for _, s := range techCurrent {
		if strings.EqualFold(strings.TrimSpace(s.ObjectKey), key) {
			return s, true
		}
	}
	if teaching, ok := teachingWorkflowStateForObject(key); ok {
		return teaching, true
	}
	return TechObjectState{}, false
}

func latestVersionForSelectedObject() (VersionRecord, bool) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return VersionRecord{}, false
	}
	var best VersionRecord
	ok := false
	for _, v := range versions {
		if !strings.EqualFold(strings.TrimSpace(v.ObjectKey), key) {
			continue
		}
		if !ok || parseDate(v.Created).After(parseDate(best.Created)) {
			best, ok = v, true
		}
	}
	if !ok {
		return teachingCompletedVersionForObject(key)
	}
	return best, ok
}

func refreshTechParametersPage() {
	st, ok := currentSelectedState()
	if !ok {
		setText(techParamObjectInfo, "Объект не выбран. Перейдите к шагу 1 «Объект и исходные данные».")
		setText(techParamDetail, "После выбора объекта здесь отображаются технологические параметры из текущего Техрежима. Телеметрия хранится накопительно и не заменяет снимок Техрежима.")
		if techProfileBoundKey != "" {
			for _, h := range techProfileEdits {
				setText(h, "")
			}
			techProfileBoundKey = ""
		}
		setText(techProfileSource, "Фактическая компоновка не сохранена.")
		return
	}
	if normalizeLaboratoryObjectKey(techProfileBoundKey) != normalizeLaboratoryObjectKey(st.ObjectKey) {
		profile := installationProfileForObject(st.ObjectKey)
		if strings.TrimSpace(profile.ObjectKey) == "" {
			profile.ObjectKey = st.ObjectKey
			profile.EquipmentType = st.Equipment
			profile.PumpModel = st.PumpModel
			setText(techProfileSource, "Источник начального заполнения: текущий Техрежим; после проверки сохраните фактическую компоновку.")
		} else {
			setText(techProfileSource, "Фактическая компоновка сохранена: "+profile.UpdatedAt+".")
		}
		setInstallationProfileFields(profile)
		techProfileBoundKey = st.ObjectKey
	}
	setText(techParamObjectInfo, fmt.Sprintf("%s / скв. %s   •   %s: %s", st.Field, st.Well, currentUnitLabel(), st.ProductionUnit))
	lines := []string{
		"Ключ объекта: " + st.ObjectKey,
		"Организация: " + st.Organization,
		currentUnitLabel() + ": " + st.ProductionUnit,
		"Месторождение: " + st.Field,
		"Скважина: " + st.Well,
		"Залежь: " + st.Deposit,
		"Объект / пласт: " + st.ObjectPlast,
		"Фактическое ГНО: " + st.Equipment,
		"Модель насоса: " + st.PumpModel,
		"Текущий МРП, сут.: " + displayMRP(st.CurrentMRP),
		"Буферное давление (Рбуф): " + displayTechNumber(st.BufferPressure, 2),
		"Линейное давление (Рлин): " + displayTechNumber(st.LinePressure, 2),
		"Забойное давление (Рзаб): " + displayTechNumber(st.BottomholePressure, 2),
		"Пластовое давление (Рпл): " + displayTechNumber(st.ReservoirPressure, 2),
		"Динамический уровень: " + displayTechNumber(st.DynamicLevel, 1),
		"Температура: " + displayTechNumber(st.TMSTemperature, 1),
		"Дебит жидкости: " + displayTechNumber(st.LiquidRate, 2),
		"Дебит нефти: " + displayTechNumber(st.OilRate, 2),
		"Обводнённость: " + displayTechNumber(st.WaterCut, 1),
		"Плановые мероприятия: " + st.PlannedActivities,
	}
	setText(techParamDetail, strings.Join(lines, "\r\n"))
}

func displayTechNumber(value string, precision int) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "—" || value == "-" {
		return "—"
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil {
		return value
	}
	formatted := strconv.FormatFloat(n, 'f', precision, 64)
	if strings.Contains(formatted, ".") {
		formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
	}
	return formatted
}

func profileNumber(v float64) string {
	if v <= 0 {
		return ""
	}
	return fmt.Sprintf("%g", v)
}

func setInstallationProfileFields(p InstallationProfile) {
	values := []string{p.EquipmentType, p.PumpModel, p.DriveModel, profileNumber(p.TubingDiameter), profileNumber(p.PumpDepth), profileNumber(p.StrokeLength), profileNumber(p.StrokesPerMin), profileNumber(p.NominalCurrent), profileNumber(p.NominalRate), profileNumber(p.RodDiameter)}
	for i, value := range values {
		setText(techProfileEdits[i], value)
	}
}

func saveInstallationProfileFromUI() {
	st, ok := currentSelectedState()
	if !ok {
		msg("Параметры установки", "Сначала выберите объект.", MB_OK|MB_ICONWARNING)
		return
	}
	numbers := make([]float64, 7)
	for i := range numbers {
		value, err := parseEconomicNumber(getText(techProfileEdits[i+3]), installationProfileFieldDefinitions()[i+3].label, false)
		if err != nil {
			msg("Параметры установки", err.Error(), MB_OK|MB_ICONWARNING)
			return
		}
		numbers[i] = value
	}
	p := InstallationProfile{ObjectKey: st.ObjectKey, EquipmentType: strings.TrimSpace(getText(techProfileEdits[0])), PumpModel: strings.TrimSpace(getText(techProfileEdits[1])), DriveModel: strings.TrimSpace(getText(techProfileEdits[2])), TubingDiameter: numbers[0], PumpDepth: numbers[1], StrokeLength: numbers[2], StrokesPerMin: numbers[3], NominalCurrent: numbers[4], NominalRate: numbers[5], RodDiameter: numbers[6]}
	if err := saveInstallationProfile(p); err != nil {
		msg("Параметры установки", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	techProfileBoundKey = ""
	refreshTechParametersPage()
	msg("Параметры установки", "Профиль сохранён. Он будет снимком использоваться в новых импортах телеметрии и в контексте динамограмм.", MB_OK|MB_ICONINFORMATION)
}

func refreshEconomicsDecisionIndicators() {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		setText(econDecisionStatus, "Связь с решением: утверждённое решение отсутствует")
		setText(econDeferralInfo, "Отсрочка ремонта, мес.: —")
		return
	}
	setText(econDecisionStatus, economicsDecisionStatusText(key))
	current, _ := parseEconomicNumber(getText(econCurrentMRP), "Текущий МРП", false)
	scenario, _ := parseEconomicNumber(getText(econMRPBase), "Прогноз МРП", false)
	if d := economicsDeferralMonths(current, scenario); d > 0 {
		setText(econDeferralInfo, fmt.Sprintf("Отсрочка ремонта, мес.: %.1f", d))
	} else {
		setText(econDeferralInfo, "Отсрочка ремонта, мес.: —")
	}
}

func useSelectedObjectForEconomics() {
	defer func() {
		refreshEconomicsVisualReference()
		refreshEconomicsMetricCards()
	}()
	st, hasState := currentSelectedState()
	v, hasV := latestVersionForSelectedObject()
	if !hasState && !hasV {
		resetEconomicsForm()
		msg("Экономическая оценка", "Сначала выберите объект.", MB_OK|MB_ICONWARNING)
		return
	}
	if economicsBindingChanged(econBoundObjectKey, selectedObjectKey) {
		resetEconomicsForm()
	}
	econBoundObjectKey = strings.TrimSpace(selectedObjectKey)
	econBoundDecisionID = ""
	if decision, ok := latestApprovedMethodDecision(selectedObjectKey); ok {
		econBoundDecisionID = strings.TrimSpace(decision.ID)
	}
	teachingEconomics := false
	if repairDays, repairCost, annualOpex, oneTime, ok := teachingEconomicsDefaultsForObject(selectedObjectKey); ok {
		setText(econRepairDays, fmt.Sprintf("%.0f", repairDays))
		setText(econRepairCost, fmt.Sprintf("%.0f", repairCost))
		setText(econAnnualOPEX, fmt.Sprintf("%.0f", annualOpex))
		setText(econOneTimeMTR, fmt.Sprintf("%.0f", oneTime))
		teachingEconomics = true
	}
	// Empty fields in the new source must clear earlier values as well.
	// RC195: prefer a freshly calculated transparent MRP baseline for the selected
	// object. Saved scenario values remain the fallback when the live MRP context
	// cannot be reconstructed. This avoids manual re-entry without hiding provenance.
	setText(econCurrentMRP, economicsCurrentMRP(st.CurrentMRP, v.CurrentMRP))
	usedLiveMRP := false
	// RC587: an approved Methods decision is the canonical scenario for economics.
	// The economic page receives the exact approved forecast and implementation
	// snapshot; no re-entry and no silent substitution with a newer live forecast.
	if decision, ok := latestApprovedMethodDecision(selectedObjectKey); ok && decision.ForecastBaseDays > 0 {
		if decision.ForecastLowDays > 0 {
			setText(econMRPLow, fmt.Sprintf("%.0f", decision.ForecastLowDays))
		} else {
			setText(econMRPLow, "")
		}
		setText(econMRPBase, fmt.Sprintf("%.0f", decision.ForecastBaseDays))
		if decision.ForecastHighDays > 0 {
			setText(econMRPHigh, fmt.Sprintf("%.0f", decision.ForecastHighDays))
		} else {
			setText(econMRPHigh, "")
		}
		usedLiveMRP = true
		setText(econResult, fmt.Sprintf("В экономику подставлена утверждённая версия решения v%d от %s. Прогноз и параметры реализации взяты из архива раздела 4. Выполните экономический расчёт заново.", methodDecisionStoredVersion(decision), dash(decision.ApprovedAt)))
	}
	if !usedLiveMRP {
		if mrpIn, ok := mrpForecastInputForSelectedObject(); ok {
			mrp := calculateMRPForecast(mrpIn)
			if mrp.HasSurvivalTriplet {
				setText(econMRPLow, fmt.Sprintf("%.0f", mrp.SurvivalLowDays))
				setText(econMRPBase, fmt.Sprintf("%.0f", mrp.SurvivalBaseDays))
				setText(econMRPHigh, fmt.Sprintf("%.0f", mrp.SurvivalHighDays))
				usedLiveMRP = true
				setText(econResult, fmt.Sprintf("В экономику подставлен подтверждённый набор анализа безотказной работы P25/P50/P75 из %s (%s). Событий %d, цензурировано %d; текущий возраст цикла %.0f сут. Старый медианный базовый ориентир %.0f сут. сохранён как контрольная линия и не удалён. Выполните экономический расчёт заново.", mrp.Advanced.ObjectAny.ModelVersion, mrp.SurvivalEconSource, mrp.Advanced.ObjectAny.EventCount, mrp.Advanced.ObjectAny.CensoredCount, mrp.Advanced.CurrentAgeDays, mrp.BaseDays))
			} else if mrp.BaseDays > 0 {
				setText(econMRPBase, fmt.Sprintf("%.0f", mrp.BaseDays))
				if mrp.HasLowHigh {
					setText(econMRPLow, fmt.Sprintf("%.0f", mrp.LowDays))
					setText(econMRPHigh, fmt.Sprintf("%.0f", mrp.HighDays))
				} else {
					setText(econMRPLow, "")
					setText(econMRPHigh, "")
				}
				usedLiveMRP = true
				setText(econResult, fmt.Sprintf("Набор анализа безотказной работы пока не прошёл проверку достаточности и последовательную проверку по времени, поэтому экономика оставлена на прозрачном %s. Достаточность: %s. Сопоставимая история: %d; отличается: %d; снимок недостаточен: %d. Выполните экономический расчёт заново.", mrp.ModelVersion, mrp.EvidenceLevel, mrp.ComparableCount, mrp.NonComparableCount, mrp.UnknownComparable))
			}
		}
	}
	if !usedLiveMRP {
		setText(econMRPLow, strings.TrimSpace(v.MRPLow))
		setText(econMRPBase, strings.TrimSpace(v.MRPBase))
		setText(econMRPHigh, strings.TrimSpace(v.MRPHigh))
		setText(econResult, "МРП подставлен из последней сохранённой версии: актуальный базовый расчёт по выбранному объекту сейчас недоступен. Проверьте происхождение сценария и выполните расчёт заново.")
	}
	setText(econScenario, normalizeScenarioName(v.Scenario))
	if hasState {
		setText(econObjectLabel, fmt.Sprintf("%s / скв. %s", st.Field, st.Well))
	}
	if hasV {
		if !hasState {
			setText(econObjectLabel, objectLabel(v))
		}
	}
	if contract, ok := technicalScenarioForSelectedObject(); ok {
		scenarioText := technicalScenarioSummary(contract) + "\r\n\r\n" + technicalScenarioComparisonText(contract)
		if decision, ok := latestApprovedMethodDecision(selectedObjectKey); ok {
			scenarioText = economicsApprovedDecisionText(decision) + "\r\n\r\n" + scenarioText
		}
		setText(econScenarioCard, scenarioText)
		combo := combinedTechnicalScenario(contract)
		if combo.TotalAnnualOPEXRub > 0 {
			setText(econAnnualOPEX, fmt.Sprintf("%.0f", combo.TotalAnnualOPEXRub))
		} else {
			setText(econAnnualOPEX, "")
		}
		if combo.TotalOneTimeMTRRub > 0 {
			setText(econOneTimeMTR, fmt.Sprintf("%.0f", combo.TotalOneTimeMTRRub))
		} else {
			setText(econOneTimeMTR, "")
		}
		prefix := technicalScenarioSummary(contract) + "\r\n" + combinedScenarioText(contract)
		current := strings.TrimSpace(getText(econResult))
		if current != "" {
			setText(econResult, prefix+"\r\n\r\n"+current)
		} else {
			setText(econResult, prefix)
		}
	} else {
		setText(econScenarioCard, "Сценарий мероприятия не сформирован.")
	}
	// Parallel schemes are independent economic alternatives. When the user
	// switches a scheme, show its own forecast and costs instead of silently
	// retaining the approved base decision's values.
	if scenario, ok := activeAlternativeScenario(selectedObjectKey); ok && len(alternativeScenarioMethodsForCalculation(scenario)) > 0 {
		if scenario.ForecastLowDays > 0 {
			setText(econMRPLow, fmt.Sprintf("%.0f", scenario.ForecastLowDays))
		} else {
			setText(econMRPLow, "")
		}
		if scenario.ForecastBaseDays > 0 {
			setText(econMRPBase, fmt.Sprintf("%.0f", scenario.ForecastBaseDays))
		} else {
			setText(econMRPBase, "")
		}
		if scenario.ForecastHighDays > 0 {
			setText(econMRPHigh, fmt.Sprintf("%.0f", scenario.ForecastHighDays))
		} else {
			setText(econMRPHigh, "")
		}
		if scenario.AnnualOPEXRub != 0 || economicsScenarioCalculated(scenario) {
			setText(econAnnualOPEX, fmt.Sprintf("%.0f", scenario.AnnualOPEXRub))
		} else {
			setText(econAnnualOPEX, "")
		}
		if scenario.OneTimeCAPEXRub != 0 || economicsScenarioCalculated(scenario) {
			setText(econOneTimeMTR, fmt.Sprintf("%.0f", scenario.OneTimeCAPEXRub))
		} else {
			setText(econOneTimeMTR, "")
		}
		setText(econScenarioCard, fmt.Sprintf("%s · %s · %s\r\n%s\r\nМетоды: %s",
			strings.ToUpper(scenario.Name), strings.ToUpper(dash(scenario.SelectionKind)), strings.ToUpper(scenario.Availability),
			dash(scenario.SelectionBasis), strings.Join(alternativeScenarioMethodsForCalculation(scenario), " + ")))
	}
	refreshEconomicsDecisionIndicators()
	if teachingEconomics {
		current := strings.TrimSpace(getText(econResult))
		banner := "УЧЕБНЫЙ РЕЖИМ: стоимость ремонта, длительность ремонта и затраты заполнены синтетическими значениями только для проверки экономического расчёта. Они не являются промысловыми данными и не должны переноситься в реальный объект."
		if current != "" {
			setText(econResult, banner+"\r\n\r\n"+current)
		} else {
			setText(econResult, banner)
		}
	}
}

func economicsInputHandles() []syscall.Handle {
	return []syscall.Handle{econCurrentMRP, econMRPLow, econMRPBase, econMRPHigh,
		econScenario, econHorizon, econRepairDays, econRepairCost, econAnnualOPEX,
		econInflation, econOneTimeMTR}
}

func invalidateEconomicsForEdit(h syscall.Handle) bool {
	if h == 0 || econResult == 0 {
		return false
	}
	for _, input := range economicsInputHandles() {
		if input == h {
			setText(econResult, "Исходные данные изменены. Выполните расчёт заново.")
			refreshEconomics812Display()
			return true
		}
	}
	return false
}

func resetEconomicsForm() {
	for _, h := range economicsInputHandles() {
		if h != 0 {
			setText(h, "")
		}
	}
	econBoundObjectKey = ""
	econBoundDecisionID = ""
	setText(econScenario, "Центральный")
	setText(econHorizon, "36")
	setText(econInflation, "0")
	setText(econObjectLabel, "Объект не выбран.")
	setText(econScenarioCard, "Сценарий мероприятия ещё не сформирован.")
	setText(econDecisionStatus, "Связь с решением: утверждённое решение отсутствует")
	setText(econDeferralInfo, "Отсрочка ремонта, мес.: —")
	setText(econResult, "Заполните исходные данные. Неизвестные значения не заменяйте нулём.")
}

func refreshEconomicsPage() {
	defer func() {
		refreshEconomicsVisualReference()
		refreshEconomicsMetricCards()
	}()
	applySimplifiedEconomicsUI()
	_, hasState := currentSelectedState()
	_, hasV := latestVersionForSelectedObject()
	if !hasState && !hasV {
		resetEconomicsForm()
		return
	}
	// Switching tabs on the same object must not overwrite manual inputs, but a
	// newly approved/restored technology-decision version must refresh the bound scenario.
	decisionChanged := false
	if decision, ok := latestApprovedMethodDecision(selectedObjectKey); ok {
		decisionChanged = !strings.EqualFold(strings.TrimSpace(econBoundDecisionID), strings.TrimSpace(decision.ID))
	} else if strings.TrimSpace(econBoundDecisionID) != "" {
		decisionChanged = true
	}
	if economicsBindingChanged(econBoundObjectKey, selectedObjectKey) || decisionChanged {
		useSelectedObjectForEconomics()
	} else {
		refreshEconomicsDecisionIndicators()
	}
}

func applySimplifiedEconomicsUI() {
	// RC701: the separate extended/DCF surface is removed from the working page.
	// Benefit Modeling compares approved parallel schemes without duplicating the
	// forecast and method-specific CAPEX/OPEX inputs already stored in Methods.
	show(econBasicButton, false)
	show(econExtendedButton, false)
	show(econCalculateButton, false)
	show(econCalculateAllButton, true)
	setText(econModeInfo, "ФОНД · НАКОПИТЕЛЬНАЯ ЭКОНОМИКА / ВАРИАНТЫ МЕРОПРИЯТИЙ")
	visible := map[int]bool{0: true, 5: true, 6: true, 7: true, 9: true}
	for i, input := range economicsInputHandles() {
		show(input, visible[i])
		if i < len(econFieldLabels) {
			show(econFieldLabels[i], visible[i])
		}
	}
}

func calculateEconomicsFromUI() {
	defer func() {
		refreshEconomicsVisualReference()
		refreshEconomicsMetricCards()
	}()
	if strings.TrimSpace(selectedObjectKey) == "" || economicsBindingChanged(econBoundObjectKey, selectedObjectKey) {
		refreshEconomicsPage()
		msg("Экономическая оценка", "Объект изменён или не выбран. Проверьте исходные данные перед расчётом.", MB_OK|MB_ICONWARNING)
		return
	}
	_, hasState := currentSelectedState()
	_, hasV := latestVersionForSelectedObject()
	if !hasState && !hasV {
		resetEconomicsForm()
		msg("Экономическая оценка", "Объект не найден в текущих данных или истории.", MB_OK|MB_ICONWARNING)
		return
	}
	setText(econResult, "Расчёт не выполнен. Проверяются исходные данные.")
	currentMRP, err := parseFloatUI(econCurrentMRP, "Текущий МРП", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	low, err := parseFloatUI(econMRPLow, "Прогноз МРП — низкий", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	base, err := parseFloatUI(econMRPBase, "Прогноз МРП — базовый", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	high, err := parseFloatUI(econMRPHigh, "Прогноз МРП — высокий", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	horizon, err := parseFloatUI(econHorizon, "Горизонт", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	if err = validateEconomicsHorizon(horizon); err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	repairDays, err := parseFloatUI(econRepairDays, "Длительность ремонта", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	repairCost, err := parseFloatUI(econRepairCost, "Стоимость ремонта", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	annualOpex, err := parseFloatUI(econAnnualOPEX, "Дополнительные затраты", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	inflation, err := parseFloatUI(econInflation, "Инфляция затрат", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	oneTime, err := parseFloatUI(econOneTimeMTR, "Разовые затраты", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}

	var econContract *EconomicScenarioContract
	if techContract, ok := technicalScenarioForSelectedObject(); ok {
		combo := combinedTechnicalScenario(techContract)
		if len(combo.Methods) <= 1 {
			saveScenarioCostDraft(techContract.ObjectKey, techContract.BaseMethod, annualOpex, oneTime)
		} else {
			// In a combined scenario the aggregate must never be written back into one method line.
			// Economics uses the explicit sum of method-specific drafts only.
			annualOpex = combo.TotalAnnualOPEXRub
			oneTime = combo.TotalOneTimeMTRRub
			setText(econAnnualOPEX, fmt.Sprintf("%.0f", annualOpex))
			setText(econOneTimeMTR, fmt.Sprintf("%.0f", oneTime))
		}
		c := economicContractFromCombined(techContract)
		econContract = &c
		scenarioText := technicalScenarioSummary(techContract) + "\r\n\r\n" + combinedScenarioText(techContract) + "\r\n" + technicalScenarioComparisonText(techContract)
		if decision, ok := latestApprovedMethodDecision(selectedObjectKey); ok {
			scenarioText = economicsApprovedDecisionText(decision) + "\r\n\r\n" + scenarioText
		}
		setText(econScenarioCard, scenarioText)
	}
	target := targetRunlifeResolvedForSelectedObject()
	in := BaseEconomicsInput{
		TechnicalScenario:  econContract,
		CurrentMRPDays:     currentMRP,
		MRPLowDays:         low,
		MRPBaseDays:        base,
		MRPHighDays:        high,
		TargetMRPDays:      target.Days,
		TargetSource:       target.Source,
		TargetNearPct:      target.NearPct,
		SelectedScenario:   strings.TrimSpace(getText(econScenario)),
		HorizonMonths:      int(horizon),
		RepairDurationDays: repairDays,
		RepairCostK:        repairCost / 1000.0,
		AnnualOPEXK:        annualOpex / 1000.0,
		OPEXInflationPct:   inflation,
		OneTimeMTRK:        oneTime / 1000.0,
	}
	r := CalculateBaseEconomics(in)
	if r.Status != "PASS" {
		setText(econResult, "Расчёт не выполнен:\r\n"+strings.Join(r.Errors, "; "))
		return
	}
	selected := r.Selected
	totalCostsK := selected.OPEXForHorizonK + selected.OneTimeMTRK
	scenarioHeader := "Сценарий мероприятия: не зафиксирован\r\n"
	if econContract != nil {
		scenarioHeader = fmt.Sprintf("Сценарий мероприятия: %s\r\nМетод: %s; фактическое ГНО: %s; осложнение: %s; источник МРП: %s / %s\r\n", econContract.ScenarioID, dash(econContract.Method), dash(econContract.GNO), dash(econContract.Complication), dash(econContract.MRPSource), dash(econContract.MRPModelVersion))
	}
	out := fmt.Sprintf("BENEFIT MODELING · УПРОЩЁННЫЙ РАСЧЁТ\r\nВерсия формул: %s\r\n%s\r\nВыбранный сценарий: %s\r\nПрогноз МРП: %.1f сут.\r\nЦелевой ориентир: %.0f сут. — %s (%.0f%% от цели; %+.0f сут.)\r\nРемонтов в год — исходно / после мероприятия: %.3f / %.3f\r\nПредотвращено ремонтов: %.3f в год\r\nПредотвращённый простой: %.2f сут./год\r\nПредотвращённые расходы на ремонты: %.2f тыс. руб./год\r\nЗатраты метода за горизонт: %.2f тыс. руб.\r\nЧистая выгода за горизонт: %.2f тыс. руб.\r\n\r\nСтатус: %s",
		r.FormulaVersion, scenarioHeader, selected.Name, selected.MRPDays, r.TargetMRPDays, r.TargetStatus, r.TargetRatioPct, r.TargetGapDays, selected.RepairsPerYearBase, selected.RepairsPerYearScenario, selected.PreventedRepairsPerYear, selected.PreventedDowntimeDaysPerYr, selected.GrossBenefitPerYearK, totalCostsK, selected.NetBenefitForHorizonK, economicsStatusRu(r.Status))
	out += "\r\n\r\n" + baseEconomicsScenarioSensitivityText(r.Scenarios)
	setText(econResult, out)
	scenarioID := ""
	if econContract != nil {
		scenarioID = econContract.ScenarioID
	}
	if err := markEconomicsCalculatedForCurrentDecision(selectedObjectKey, scenarioID); err != nil {
		setText(econResult, out+"\r\n\r\nПРЕДУПРЕЖДЕНИЕ: расчёт выполнен, но статус связи с версией решения не сохранён: "+err.Error())
	}
	_ = updateActiveAlternativeScenario(selectedObjectKey, func(s *AlternativeMethodScenario) {
		s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays = low, base, high
		s.AnnualOPEXRub, s.OneTimeCAPEXRub = annualOpex, oneTime
		s.EconomicsStatus, s.EconomicsCalculated = "Расчёт актуален", time.Now().Format(time.RFC3339)
		s.NetBenefitK, s.BenefitCostRatio, s.PaybackMonths = selected.NetBenefitForHorizonK, selected.BenefitCostRatio, r.PaybackMonths
	})
	setText(econDeferralInfo, fmt.Sprintf("Отсрочка ремонта, мес.: %.1f", economicsDeferralMonths(currentMRP, selected.MRPDays)))
	refreshEconomicsDecisionIndicators()
}

func alternativeScenarioMethodsForCalculation(s AlternativeMethodScenario) []string {
	methods := normalizeMethodList(s.Methods)
	if len(methods) > 0 || s.MethodsInitialized {
		return methods
	}
	if d, ok := latestApprovedMethodDecision(s.ObjectKey); ok {
		methods = normalizeMethodList(d.Methods)
		if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
			methods = []string{d.Method}
		}
	}
	return methods
}

func calculateAllAlternativeEconomics() {
	defer func() {
		refreshEconomicsVisualReference()
		refreshEconomicsMetricCards()
	}()
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Экономическая оценка", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	currentMRP, err := parseFloatUI(econCurrentMRP, "Текущий МРП", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	horizon, err := parseFloatUI(econHorizon, "Горизонт", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	if err = validateEconomicsHorizon(horizon); err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	repairDays, err := parseFloatUI(econRepairDays, "Длительность ремонта", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	repairCost, err := parseFloatUI(econRepairCost, "Стоимость ремонта", true)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	inflation, err := parseFloatUI(econInflation, "Инфляция затрат", false)
	if err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}

	scenarios := alternativeScenariosForObject(selectedObjectKey)
	snap := methodForecastSnapshotForSelectedObject()
	var b strings.Builder
	b.WriteString("ПАРАЛЛЕЛЬНЫЙ РАСЧЁТ СХЕМ\r\nОбщие исходные данные: объект, стоимость ремонта, горизонт и инфляция. Прогноз и затраты — по каждой схеме.\r\n\r\n")
	for _, scenario := range scenarios {
		methods := alternativeScenarioMethodsForCalculation(scenario)
		if len(methods) == 0 {
			fmt.Fprintf(&b, "%s | %s | методы не выбраны | расчёт не выполнен\r\n", scenario.Name, scenario.Availability)
			continue
		}
		forecast := combinedMethodForecastForNames(selectedObjectKey, methods, snap, time.Now())
		if scenario.ForecastBaseDays > 0 {
			forecast.ForecastLow, forecast.ForecastBase, forecast.ForecastHigh = scenario.ForecastLowDays, scenario.ForecastBaseDays, scenario.ForecastHighDays
		}
		opex, capex := scenario.AnnualOPEXRub, scenario.OneTimeCAPEXRub
		if opex <= 0 && capex <= 0 {
			for _, method := range methods {
				draft := loadScenarioCostDraftForID(selectedObjectKey, scenario.ID, method)
				opex += draft.AnnualOPEXRub
				capex += draft.OneTimeMTRRub
			}
		}
		in := BaseEconomicsInput{CurrentMRPDays: currentMRP, MRPLowDays: forecast.ForecastLow, MRPBaseDays: forecast.ForecastBase, MRPHighDays: forecast.ForecastHigh,
			TargetMRPDays: targetRunlifeResolvedForSelectedObject().Days, TargetSource: targetRunlifeResolvedForSelectedObject().Source,
			SelectedScenario: strings.TrimSpace(getText(econScenario)), HorizonMonths: int(horizon), RepairDurationDays: repairDays,
			RepairCostK: repairCost / 1000, AnnualOPEXK: opex / 1000, OPEXInflationPct: inflation, OneTimeMTRK: capex / 1000}
		result := CalculateBaseEconomics(in)
		if result.Status != "PASS" {
			fmt.Fprintf(&b, "%s | %s | МРП %.0f / %.0f / %.0f сут. | %s\r\n", scenario.Name, scenario.Availability, forecast.ForecastLow, forecast.ForecastBase, forecast.ForecastHigh, strings.Join(result.Errors, "; "))
			continue
		}
		selected := result.Selected
		_ = updateAlternativeScenarioByID(selectedObjectKey, scenario.ID, func(s *AlternativeMethodScenario) {
			s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays = forecast.ForecastLow, forecast.ForecastBase, forecast.ForecastHigh
			s.EconomicsStatus, s.EconomicsCalculated = "Расчёт актуален", time.Now().Format(time.RFC3339)
			s.AnnualOPEXRub, s.OneTimeCAPEXRub = opex, capex
			s.NetBenefitK, s.BenefitCostRatio, s.PaybackMonths = selected.NetBenefitForHorizonK, selected.BenefitCostRatio, result.PaybackMonths
		})
		fmt.Fprintf(&b, "%s | %s | %s | %s\r\nМетоды: %s\r\nМРП: %.0f / %.0f / %.0f сут. | до цели: %+.0f сут. | OPEX %.0f руб./год | CAPEX %.0f руб. | предотвращённые расходы %.1f тыс. руб./год | чистая выгода %.1f тыс. руб.\r\n%s\r\nОснование: %s\r\n\r\n",
			scenario.Name, scenario.Availability, scenario.Status, dash(scenario.SelectionKind), strings.Join(methods, " + "), forecast.ForecastLow, forecast.ForecastBase, forecast.ForecastHigh,
			forecast.ForecastBase-targetRunlifeResolvedForSelectedObject().Days, opex, capex, selected.GrossBenefitPerYearK, selected.NetBenefitForHorizonK,
			baseEconomicsScenarioSensitivityText(result.Scenarios), dash(scenario.SelectionBasis))
	}
	b.WriteString("Схема утверждается технологом. Недоступность ресурсов не обнуляет технический прогноз, но требует условного решения и срока обеспечения.")
	setText(econResult, b.String())
	refreshEconomicsDecisionIndicators()
}

func formatOptionalMonths(v *float64) string {
	if v == nil {
		return "не определяется / не требуется"
	}
	return fmt.Sprintf("%.2f мес.", *v)
}
func formatOptionalDays(v *float64) string {
	if v == nil {
		return "не определяется"
	}
	return fmt.Sprintf("%.1f сут.", *v)
}

func persistCurrentScenarioCosts() {
	contract, ok := technicalScenarioForSelectedObject()
	if !ok {
		return
	}
	combo := combinedTechnicalScenario(contract)
	if len(combo.Methods) > 1 {
		return // aggregate fields are derived from method lines and must not overwrite one method
	}
	opex, err1 := parseFloatUI(econAnnualOPEX, "Дополнительные затраты", false)
	one, err2 := parseFloatUI(econOneTimeMTR, "Разовые затраты", false)
	if err1 == nil && err2 == nil {
		saveScenarioCostDraft(contract.ObjectKey, contract.BaseMethod, opex, one)
	}
}

func switchEconomicTechnicalMethod(toBase bool) {
	persistCurrentScenarioCosts()
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Сценарий мероприятия", "Сначала выберите объект и сформируйте сценарий мероприятия.", MB_OK|MB_ICONWARNING)
		return
	}
	if toBase {
		setBaseAlternativeScenario(selectedObjectKey)
	} else {
		cycleAlternativeScenario(selectedObjectKey, 1)
	}
	_ = markEconomicsNeedsRecalc(selectedObjectKey, "выбрана другая параллельная схема")
	useSelectedObjectForEconomics()
	setText(econResult, "Выбрана другая схема. Проверьте её методы, прогноз, доступность и затраты. Для общей таблицы нажмите «Все схемы».")
	refreshEconomicsDecisionIndicators()
}

func refreshResultPage() {
	st, hasState := currentSelectedState()
	v, hasV := latestVersionForSelectedObject()
	if !hasState && !hasV {
		setText(resultSummary, "Объект не выбран.\r\n\r\nРабочая последовательность: 1) выбрать объект; 2) проверить обязательный базовый уровень технологических данных; 3) добавить доступные лабораторные показатели; 4) при наличии — дополнительные исследования, телеметрию и динамограмму; 5) выполнить предварительную оценку осложнений; 6) выбрать мероприятие; 7) выполнить экономическую оценку. Отсутствие дополнительных исследований не блокирует предварительную оценку.")
		refreshResultWorkspaceUI(getText(resultSummary))
		return
	}
	var b strings.Builder
	b.WriteString("РОСА — сводный результат по объекту\r\n")
	b.WriteString("Предварительная технологическая оценка; окончательное решение принимается после подтверждающих исследований и производственной проверки.\r\n\r\n")
	if hasState {
		b.WriteString("СЕЙЧАС\r\n")
		fmt.Fprintf(&b, "Организация: %s\r\n%s: %s\r\nМесторождение: %s\r\nСкважина: %s\r\nОбъект / пласт: %s\r\nГНО: %s\r\nТекущий МРП: %s сут.\r\n", st.Organization, currentUnitLabel(), st.ProductionUnit, st.Field, st.Well, st.ObjectPlast, st.Equipment, st.CurrentMRP)
		if in, ok := mrpForecastInputForSelectedObject(); ok {
			r := calculateMRPForecast(in)
			s := r.Advanced.ObjectAny
			b.WriteString("\r\nПРОГНОЗ БЕЗРЕМОНТНОЙ РАБОТЫ\r\n")
			if s.HasConditionalMedianMore {
				fmt.Fprintf(&b, "Остаточный ресурс, P50: ещё %.0f сут.\r\n", s.ConditionalMedianMoreDays)
			}
			fmt.Fprintf(&b, "Вероятность продолжения работы ещё 90 / 180 / 365 сут.: %s / %s / %s.\r\n", formatProbability(s.Survival90More, s.HasSurvival90More), formatProbability(s.Survival180More, s.HasSurvival180More), formatProbability(s.Survival365More, s.HasSurvival365More))
			if r.Advanced.Competing.Has365 {
				fmt.Fprintf(&b, "Вероятность подтверждённого отказа к 365 сут.: %.0f%%.\r\n", r.Advanced.Competing.FailureCIF365*100)
			}
			fmt.Fprintf(&b, "Достаточность истории: %s; событий %d; незавершённых/цензурированных циклов %d.\r\n", r.EvidenceLevel, s.EventCount, s.CensoredCount)
			fmt.Fprintf(&b, "%s\r\n", targetRunlifeSummary(r.TargetForecastDays, r.TargetMRPDays))
			if strings.TrimSpace(r.TargetSource) != "" {
				fmt.Fprintf(&b, "Источник цели: %s.\r\n", r.TargetSource)
			}
			if len(r.Warnings) > 0 || len(r.Advanced.Warnings) > 0 {
				b.WriteString("\r\nЧТО ТРЕБУЕТ УТОЧНЕНИЯ\r\n")
				warnings := append([]string{}, r.Warnings...)
				warnings = append(warnings, r.Advanced.Warnings...)
				for i, w := range warnings {
					if i >= 4 {
						break
					}
					b.WriteString("• " + w + "\r\n")
				}
			}
		}
		b.WriteString("\r\n")
	}
	objectKey := selectedObjectKey
	if strings.TrimSpace(objectKey) == "" && hasV {
		objectKey = v.ObjectKey
	}
	if classification := assessmentFinalReportText(objectKey); strings.TrimSpace(classification) != "" {
		b.WriteString(classification + "\r\n")
	}
	if hasV {
		fmt.Fprintf(&b, "Последняя расчётная версия: %s\r\nДата: %s\r\nОсложнение: %s\r\nВыбранный метод: %s\r\nПрогноз МРП: %s сут.\r\nЛаборатория: %s\r\nЭкономика: %s — %s\r\nРешение: %s\r\nКомментарий: %s\r\nОтветственный: %s\r\n", v.VersionID, v.Created, v.Complication, v.Method, v.ForecastMRP, v.LabStatus, v.EconStatus, v.EconResult, v.Decision, v.DecisionComment, v.Responsible)
		if tc, ok := technicalScenarioForSelectedObject(); ok {
			ec := economicContractFromCombined(tc)
			fmt.Fprintf(&b, "\r\nТекущий сценарий мероприятия: %s\r\nМетод сценария: %s; фактическое ГНО: %s; осложнение: %s\r\nСпецифические затраты сценария: эксплуатационные затраты (OPEX) %.0f руб./год; разово %.0f руб.\r\nИсточник МРП: %s / %s\r\n", ec.ScenarioID, dash(ec.Method), dash(ec.GNO), dash(ec.Complication), ec.SpecificAnnualOPEXRub, ec.SpecificOneTimeMTRRub, dash(ec.MRPSource), mrpUserModelName(ec.MRPModelVersion))
			b.WriteString("\r\n" + combinedScenarioText(tc))
		}
		if rules := methodRulesForName(v.Method); len(rules) > 0 {
			b.WriteString("\r\nОснование метода:\r\n")
			for _, r := range rules {
				fmt.Fprintf(&b, "• %s: %s\r\n", r.Complication, r.Criteria)
			}
		}
		if o := outcomeFor(v.VersionID); o != nil {
			b.WriteString("\r\nФАКТИЧЕСКИЙ РЕЗУЛЬТАТ И ДЕФЕКТОВКА\r\n")
			fmt.Fprintf(&b, "Тип события: %s; МРП факт: %s сут.; затраты факт: %s тыс. руб.\r\n", dash(o.EventType), dash(o.ActualMRP), dash(o.ActualCost))
			if structured := outcomeStructuredSummary(*o); strings.TrimSpace(structured) != "" {
				b.WriteString(structured + "\r\n")
			}
			b.WriteString("\r\nСОПОСТАВЛЕНИЕ ПРОГНОЗ → ФАКТ\r\n")
			b.WriteString(outcomeForecastFactComparison(v, *o) + "\r\n")
			b.WriteString("Важно: наблюдаемое изменение после мероприятия не интерпретируется РОСА как причинный эффект автоматически.\r\n")
		}
		b.WriteString("\r\nНАКОПИТЕЛЬНАЯ СТАТИСТИКА ЗАВЕРШЕНИЙ ЦИКЛОВ\r\n")
		b.WriteString(outcomeCycleStatisticsTextForObject(v.ObjectKey) + "\r\n")
	} else {
		b.WriteString("Расчётная версия по выбранному объекту ещё не создана.\r\n")
	}
	b.WriteString(alternativeScenarioReportText(objectKey))
	b.WriteString(resultDecisionAndControlText(objectKey))
	setText(resultSummary, b.String())
	refreshResultWorkspaceUI(b.String())
}

func layoutWorkflowPage(w, h int32) {
	contentW := max32(720, w-270)
	switch currentPage {
	case ID_TECHPARAM:
		// Stable single-panel geometry restored; keep the page aligned with the
		// earlier object-selection/passport visual language.
		setPos(techParamTitle, 242, py(94), contentW, 34)
		setPos(techParamDescription, 242, py(132), contentW, 36)
		setPos(techParamObjectInfo, 242, py(172), contentW, 28)
		buttonGap := int32(10)
		buttonW := (contentW - 3*buttonGap) / 4
		for i, hButton := range techActionButtons {
			setPos(hButton, 242+int32(i)*(buttonW+buttonGap), py(210), buttonW, 48)
		}
		colW := (contentW - 20) / 2
		for i := range techProfileEdits {
			col, row := int32(i/5), int32(i%5)
			x, y := int32(242)+col*(colW+20), int32(270)+row*36
			labelW := max32(160, colW-290)
			setPos(techProfileLabels[i], x, py(y), labelW, 26)
			setPos(techProfileEdits[i], x+labelW+5, py(y-3), 170, 29)
			setPos(techProfileUnits[i], x+labelW+180, py(y), max32(60, colW-labelW-180), 26)
		}
		setPos(techProfileSave, 242, py(454), 280, 36)
		setPos(techProfileSource, 536, py(460), max32(300, contentW-294), 26)
		setPos(techParamDetail, 242, py(504), contentW, max32(160, h-py(504)-24))
	case ID_MRP:
		layoutMRPForecast(w, h)
	case ID_ECON:
		layoutEconomicsVisualReference(w, h)
	case ID_RESULT:
		layoutResultWorkspace(w, h)
	}
}

func handleWorkflowCommand(id int) bool {
	if (currentPage == ID_RESULT || currentPage == ID_ARCHIVE) && handleResultWorkspaceCommand(id) {
		return true
	}
	if page, ok := techParameterActionPage(id); ok {
		switchPage(page)
		return true
	}
	switch id {
	case ID_NAV_OPI:
		switchPage(ID_OPI)
	case ID_NAV_REGISTRIES:
		switchPage(ID_REGISTRIES)
	case ID_ECON_USE_SELECTED:
		useSelectedObjectForEconomics()
	case ID_ECON_TEMPLATE_SAVE:
		downloadEconomicsTemplate()
	case ID_ECON_TEMPLATE_LOAD:
		importEconomicsTemplate()
	case ID_MRP_CALCULATE:
		calculateMRPForecastFromUI()
	case ID_MRP_GUIDE:
		openMRPAccumulationGuide()
	case ID_MRP_DRILL:
		openMRPRollingDrill()
	case ID_MRP_DETAIL:
		openMRPForecastDetail()
	case ID_MRP_TARGET_SAVE:
		saveMRPTargetFromUI()
	case ID_MRP_TARGET_RESET:
		resetMRPTargetFromUI()
	case ID_ECON_CALCULATE:
		calculateEconomicsFromUI()
	case ID_ECON_CALCULATE_ALL:
		calculateAllAlternativeEconomics()
	case ID_ECON_FULL_RESULT:
		body := strings.TrimSpace(getText(econResult))
		if body == "" {
			body = "Выполните расчёт, чтобы получить вывод по схемам."
		}
		openMRPTextDialog("ЭКОНОМИЧЕСКИЙ РАСЧЁТ · ВЫВОД ТЕХНОЛОГУ", body)
	case ID_ECON_CHART:
		openEconomicsComparisonChart()
	case ID_ECON_METHOD_NEXT:
		switchEconomicTechnicalMethod(false)
	case ID_ECON_METHOD_BASE:
		switchEconomicTechnicalMethod(true)
	case ID_ECON_BASIC:
		applySimplifiedEconomicsUI()
		setText(econResult, "BENEFIT MODELING\r\nСравнение предотвращённых ремонтов и расходов, затрат метода и чистой выгоды по каждой утверждённой схеме.")
	case ID_ECON_EXTENDED:
		applySimplifiedEconomicsUI()
	case ID_RESULT_REFRESH:
		refreshResultPage()
	case ID_TECHPROFILE_SAVE:
		saveInstallationProfileFromUI()
	default:
		return false
	}
	return true
}

func techParameterActionPage(id int) (int, bool) {
	switch id {
	case ID_NAV_TELEMETRY:
		return ID_TELEMETRY, true
	case ID_NAV_DYNAMOMETRY:
		return ID_DYN, true
	case ID_NAV_DATA:
		return ID_DATA, true
	case ID_NAV_TECHBANK:
		return ID_TECH, true
	default:
		return 0, false
	}
}
