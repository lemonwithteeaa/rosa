package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	ID_MRP              = 119
	ID_MRP_CALCULATE    = 9490
	ID_MRP_TARGET_SAVE  = 9491
	ID_MRP_TARGET_RESET = 9492
	ID_MRP_TARGET_SCOPE = 9493
	ID_MRP_GUIDE        = 9494
	ID_MRP_DRILL        = 9495
	ID_MRP_DETAIL       = 9496
)

var (
	mrpPageTitle         syscall.Handle
	mrpIntroLabel        syscall.Handle
	mrpObjectLabel       syscall.Handle
	mrpEvidenceLabel     syscall.Handle
	mrpCurrentHeader     syscall.Handle
	mrpBaseHeader        syscall.Handle
	mrpTargetHeader      syscall.Handle
	mrpRangeHeader       syscall.Handle
	mrpScenarioBandTitle syscall.Handle
	mrpCurrentLabel      syscall.Handle
	mrpBaseLabel         syscall.Handle
	mrpTargetStatusLabel syscall.Handle
	mrpRangeLabel        syscall.Handle
	mrpHistoryList       syscall.Handle
	mrpMethodsList       syscall.Handle
	mrpHistoryHeader     syscall.Handle
	mrpMethodsHeader     syscall.Handle
	mrpDetailHeader      syscall.Handle
	mrpDetail            syscall.Handle
	mrpDetailButton      syscall.Handle
	mrpCalculateButton   syscall.Handle
	mrpGuideButton       syscall.Handle
	mrpDrillButton       syscall.Handle
	mrpRollingHeader     syscall.Handle
	mrpRollingLabel      syscall.Handle
	mrpRollingBackground syscall.Handle
	mrpMetricBackgrounds [4]syscall.Handle
	mrpProbabilityHeader syscall.Handle
	mrpSurvival90Header  syscall.Handle
	mrpSurvival180Header syscall.Handle
	mrpSurvival365Header syscall.Handle
	mrpFailure365Header  syscall.Handle
	mrpSurvival90Label   syscall.Handle
	mrpSurvival180Label  syscall.Handle
	mrpSurvival365Label  syscall.Handle
	mrpFailure365Label   syscall.Handle
	mrpTargetLabel       syscall.Handle
	mrpTargetScopeLabel  syscall.Handle
	mrpTargetEdit        syscall.Handle
	mrpTargetScopeCombo  syscall.Handle
	mrpTargetSourceLabel syscall.Handle
	mrpCauseSummaryLabel syscall.Handle
	mrpTargetSaveButton  syscall.Handle
	mrpTargetResetButton syscall.Handle
	mrpAtomicLayoutDepth int
	mrpCompactLayout     bool
	mrpMethodsRowCount   int
	mrpLastDetailText    string
)

var mrpTargetScopeOptions = []struct {
	Label string
	Code  string
}{
	{"Для объекта", targetScopeObject},
	{"Для типа ГНО", targetScopeLift},
	{"Общий ориентир", targetScopeGlobal},
}

func mrpCenteredLabel(text string, x, y, w, h int32, bold bool) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|SS_CENTER|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	addPageControl(ID_MRP, z)
	if bold && z != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFontBold), 1)
	}
	return z
}

func buildMRPForecastUI() {
	firstObjectControl := len(pageControls[ID_MRP])
	mrpPageTitle = pageTitle(ID_MRP, "Наработка и отказы", 242, 94, 970, 34)
	mrpIntroLabel = label(ID_MRP, "Прогноз межремонтного периода по фактической истории объекта. Режимные и лабораторные данные учитываются только там, где их влияние подтверждено проверкой на истории.", 242, 132, 1010, 42)
	mrpObjectLabel = label(ID_MRP, "Объект не выбран.", 242, 181, 1010, 30)
	if mrpObjectLabel != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(mrpObjectLabel), WM_SETFONT, uintptr(hFontBold), 1)
	}
	mrpGuideButton = button(ID_MRP, "Методика ННО", 242, 220, 198, 42, ID_MRP_GUIDE)
	mrpCalculateButton = button(ID_MRP, "Рассчитать прогноз", 450, 220, 230, 42, ID_MRP_CALCULATE)
	mrpDrillButton = button(ID_MRP, "Проверить расчёт", 690, 220, 230, 42, ID_MRP_DRILL)

	mrpTargetLabel = label(ID_MRP, "Целевой ориентир, сут.", 242, 282, 185, 26)
	mrpTargetEdit = edit(ID_MRP, "1000", 432, 275, 86, 38, false, false, 0)
	mrpTargetScopeLabel = label(ID_MRP, "Уровень настройки", 530, 282, 145, 26)
	mrpTargetScopeCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 680, py(275), 170, 160, hMain, ID_MRP_TARGET_SCOPE)
	addPageControl(ID_MRP, mrpTargetScopeCombo)
	for _, item := range mrpTargetScopeOptions {
		pSendMessageW.Call(uintptr(mrpTargetScopeCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(item.Label))))
	}
	pSendMessageW.Call(uintptr(mrpTargetScopeCombo), CB_SETCURSEL, 0, 0)
	mrpTargetSaveButton = button(ID_MRP, "Сохранить цель", 860, 274, 140, 42, ID_MRP_TARGET_SAVE)
	mrpTargetResetButton = button(ID_MRP, "Сбросить", 1010, 274, 140, 42, ID_MRP_TARGET_RESET)
	mrpTargetSourceLabel = label(ID_MRP, "общий ориентир 1000 сут.", 242, 318, 1010, 20)
	mrpCauseSummaryLabel = label(ID_MRP, "Статистика завершений: фактов пока нет.", 242, 341, 1010, 20)
	mrpScenarioBandTitle = label(ID_MRP, "ПРОГНОЗ НАРАБОТКИ · БАЗА БЕЗ НОВОГО МЕРОПРИЯТИЯ → СЦЕНАРИИ ВЫБРАННОГО КОМПЛЕКСА", 242, 363, 1010, 20)
	if mrpScenarioBandTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(mrpScenarioBandTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	for i := range mrpMetricBackgrounds {
		mrpMetricBackgrounds[i] = create("STATIC", "", WS_CHILD|0x00000012, 242+int32(i)*252, py(386), 244, 79, hMain, 0) // SS_ETCHEDFRAME
		addPageControl(ID_MRP, mrpMetricBackgrounds[i])
	}
	mrpRollingBackground = create("STATIC", "", WS_CHILD|0x00000012, 242, py(468), 1010, 19, hMain, 0)
	addPageControl(ID_MRP, mrpRollingBackground)

	mrpCurrentHeader = mrpCenteredLabel("Базовая оценка", 250, 388, 230, 20, true)
	mrpBaseHeader = mrpCenteredLabel("Сдержанный", 502, 388, 230, 20, true)
	mrpTargetHeader = mrpCenteredLabel("Расчётный", 754, 388, 230, 20, true)
	mrpRangeHeader = mrpCenteredLabel("Благоприятный", 1006, 388, 230, 20, true)
	mrpCurrentLabel = mrpCenteredLabel("—", 250, 409, 230, 53, false)
	mrpBaseLabel = mrpCenteredLabel("—", 502, 409, 230, 53, false)
	mrpTargetStatusLabel = mrpCenteredLabel("—", 754, 409, 230, 53, false)
	mrpRangeLabel = mrpCenteredLabel("—", 1006, 409, 230, 53, false)
	mrpEvidenceLabel = labelEllipsis(ID_MRP, "Факт / цель / история: —", 250, 469, 994, 17)

	mrpRollingHeader = label(ID_MRP, "Скользящая наработка по способам · последние 3 / 4 подтверждённых отказа", 242, 437, 1010, 24)
	mrpRollingLabel = labelEllipsis(ID_MRP, "Выберите объект.", 242, 464, 1010, 23)
	if mrpRollingHeader != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(mrpRollingHeader), WM_SETFONT, uintptr(hFontBold), 1)
	}
	show(mrpRollingHeader, false)
	show(mrpRollingLabel, false)
	for _, h := range []syscall.Handle{mrpSurvival90Label, mrpSurvival180Label, mrpSurvival365Label, mrpFailure365Label} {
		if h != 0 && hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
	}

	mrpHistoryHeader = label(ID_MRP, "Фактическая история МРП по выбранному объекту", 242, 490, 800, 26)
	mrpHistoryList = newList(ID_MRP, 242, 518, 1010, 116, []struct {
		n string
		w int32
	}{{"Дата", 90}, {"МРП, сут.", 100}, {"Тип события", 125}, {"Механизм", 140}, {"Осложнение", 105}, {"ГНО", 120}, {"Сопоставимость", 135}, {"Источник", 195}})
	mrpMethodsHeader = label(ID_MRP, "Наработка по способам · все объекты архива", 242, 640, 900, 26)
	mrpMethodsList = newList(ID_MRP, 242, 668, 1010, 86, []struct {
		n string
		w int32
	}{{"Способ / схема", 320}, {"Объекты", 100}, {"Отказы", 90}, {"Σ, сут.", 110}, {"Средняя средних", 130}, {"Скольз. 3", 130}, {"Скольз. 4", 130}})
	centerListHeaders(mrpMethodsList, 7)
	centerListHeaders(mrpHistoryList, 8)
	mrpDetailHeader = label(ID_MRP, "Обоснование прогноза и ограничения", 242, 760, 600, 26)
	mrpDetail = edit(ID_MRP, "Выберите объект и выполните расчёт.", 242, 788, 1010, 122, true, true, 0)
	for _, h := range []syscall.Handle{mrpHistoryHeader, mrpMethodsHeader, mrpDetailHeader} {
		if h != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(hFontBold), 1)
		}
	}
	mrpDetailButton = button(ID_MRP, "ОТКРЫТЬ ОБОСНОВАНИЕ ПРОГНОЗА", 242, 788, 1010, 40, ID_MRP_DETAIL)
	show(mrpDetailButton, false)
	firstFleetControl := len(pageControls[ID_MRP])
	buildNNOFleetUI()
	registerPageSubviewControl(ID_MRP, pageSubviewMRPObject, pageControls[ID_MRP][firstObjectControl:firstFleetControl]...)
	for _, control := range pageControls[ID_MRP][firstFleetControl:] {
		if control != nnoFleetBack {
			registerPageSubviewControl(ID_MRP, pageSubviewMRPFleet, control)
		}
	}
	registerPageSubviewControl(ID_MRP, pageSubviewMRPObject, nnoFleetBack)
	setPageSubview(ID_MRP, pageSubviewMRPFleet)
}

func setMRPDetailText(text string) {
	mrpLastDetailText = strings.TrimSpace(text)
	setText(mrpDetail, text)
}

func openMRPForecastDetail() {
	body := strings.TrimSpace(mrpLastDetailText)
	if body == "" {
		body = "Выберите объект и выполните расчёт. После этого здесь появится полное обоснование сценариев, историческая база и ограничения прогноза."
	}
	openMRPTextDialog("ОБОСНОВАНИЕ ПРОГНОЗА И ОГРАНИЧЕНИЯ", body)
}

func targetScopeIndex(code string) int {
	for i, item := range mrpTargetScopeOptions {
		if item.Code == code {
			return i
		}
	}
	return 0
}

func targetScopeFromUI() string {
	if mrpTargetScopeCombo == 0 {
		return targetScopeObject
	}
	idx, _, _ := pSendMessageW.Call(uintptr(mrpTargetScopeCombo), CB_GETCURSEL, 0, 0)
	if int(idx) >= 0 && int(idx) < len(mrpTargetScopeOptions) {
		return mrpTargetScopeOptions[int(idx)].Code
	}
	return targetScopeObject
}

func selectTargetScope(scope string) {
	if mrpTargetScopeCombo != 0 {
		pSendMessageW.Call(uintptr(mrpTargetScopeCombo), CB_SETCURSEL, uintptr(targetScopeIndex(scope)), 0)
	}
}

func refreshTargetEditorForScope() {
	st, ok := currentSelectedState()
	if !ok {
		setText(mrpTargetEdit, fmt.Sprintf("%.0f", normalizedTargetRunlifeDays(targetRunlifeSettings.DefaultDays)))
		setText(mrpTargetSourceLabel, "общий ориентир; выберите объект для индивидуальной настройки")
		return
	}
	scope := targetScopeFromUI()
	days := targetRunlifeDaysForScope(scope, st.ObjectKey, st.Equipment)
	setText(mrpTargetEdit, fmt.Sprintf("%.0f", days))
	effective := targetRunlifeResolved(st.ObjectKey, st.Equipment)
	lift := normalizeTargetLiftType(st.Equipment)
	text := ""
	changeKey := ""
	switch scope {
	case targetScopeGlobal:
		text = fmt.Sprintf("Редактируется общий ориентир. Сейчас для объекта действует: %s.", effective.Source)
	case targetScopeLift:
		changeKey = lift
		if lift == "" {
			text = "Тип ГНО не определён: ориентир по типу ГНО сохранить нельзя."
		} else if targetRunlifeHasLiftOverride(st.Equipment) {
			text = fmt.Sprintf("Редактируется ориентир для типа ГНО %s. Сейчас для объекта действует: %s.", lift, effective.Source)
		} else {
			text = fmt.Sprintf("Для типа ГНО %s отдельная цель не задана; показан общий ориентир. Сейчас для объекта действует: %s.", lift, effective.Source)
		}
	default:
		changeKey = normalizeLaboratoryObjectKey(st.ObjectKey)
		if targetRunlifeHasObjectOverride(st.ObjectKey) {
			text = "Редактируется индивидуальный ориентир выбранного объекта."
		} else {
			text = fmt.Sprintf("Индивидуальная цель не задана; показано наследуемое значение. Сейчас действует: %s.", effective.Source)
		}
	}
	if change, found := targetRunlifeLastChangeFor(scope, changeKey); found {
		text += " " + targetRunlifeChangeText(change) + "."
	}
	if note := targetRunlifeDeviationNote(days); note != "" {
		text += " " + note
	}
	setText(mrpTargetSourceLabel, text)
}

func refreshMRPForecastPage() {
	if nnoFleetEnabled {
		refreshNNOFleetUI()
		return
	}
	in, ok := mrpForecastInputForSelectedObject()
	if !ok {
		setText(mrpObjectLabel, "Объект не выбран. Выберите объект на первом шаге.")
		setText(mrpCurrentLabel, "—")
		setText(mrpBaseLabel, "—")
		setText(mrpTargetStatusLabel, "—")
		setText(mrpRangeLabel, "—")
		setText(mrpEvidenceLabel, "Факт / цель / история: —")
		setText(mrpSurvival90Label, "—")
		setText(mrpSurvival180Label, "—")
		setText(mrpSurvival365Label, "—")
		setText(mrpFailure365Label, "—")
		setText(mrpCauseSummaryLabel, "Статистика завершений: —")
		setText(mrpRollingLabel, "")
		selectTargetScope(targetScopeGlobal)
		refreshTargetEditorForScope()
		listClear(mrpHistoryList)
		listClear(mrpMethodsList)
		mrpMethodsRowCount = 0
		refreshMRPMethodsHeader()
		setMRPDetailText("Прогноз МРП не формируется без выбранного объекта. История другого объекта автоматически не подмешивается.")
		return
	}
	st, _ := currentSelectedState()
	setText(mrpObjectLabel, fmt.Sprintf("%s / скв. %s • %s • %s", dash(st.Field), dash(st.Well), dash(st.ObjectPlast), dash(st.Equipment)))
	effective := targetRunlifeResolved(st.ObjectKey, st.Equipment)
	selectTargetScope(effective.Scope)
	refreshTargetEditorForScope()
	populateMRPHistory(in)
	populateMRPMethods(in)
	setText(mrpCauseSummaryLabel, outcomeCycleStatisticsCompact(outcomeCycleStatisticsForObject(in.ObjectKey, versions, outcomes)))
	setText(mrpRollingLabel, mrpRollingHeadline(in))
	applyMRPForecastToPage(calculateMRPForecast(in))
	if scenario, full := teachingFullScenarioForObject(in.ObjectKey); full {
		setMRPDetailText(mrpLastDetailText + fmt.Sprintf("\r\n\r\nУЧЕБНЫЙ КОМПЛЕКС %03d · %s\r\nМетоды: %s. Учебный диапазон после комплекса: %.0f / %.0f / %.0f сут. Статистический прогноз выше рассчитывается отдельно по истории МРП.", scenario.Ordinal, scenario.Name, strings.Join(scenario.Methods, " + "), scenario.ForecastLow, scenario.ForecastBase, scenario.ForecastHigh))
	}
}

func populateMRPHistory(in MRPForecastInput) {
	obs := in.Observations
	listBeginUpdate(mrpHistoryList)
	defer listEndUpdate(mrpHistoryList)
	listClear(mrpHistoryList)
	for i := len(obs) - 1; i >= 0; i-- {
		o := obs[i]
		date := "—"
		if !o.Date.IsZero() {
			date = o.Date.Format("02.01.2006")
		}
		listAdd(mrpHistoryList, len(obs)-1-i, []string{date, fmt.Sprintf("%.0f", o.MRPDays), dash(normalizeMRPEventType(o.EventType, o.Planned)), dash(o.FailureMode), dash(o.Complication), dash(o.Equipment), mrpObservationComparability(in, o), dash(o.Source)})
	}
}

func refreshMRPMethodsHeader() {
	if mrpCompactLayout {
		setText(mrpMethodsHeader, fmt.Sprintf("Способы · %d схем · весь архив", mrpMethodsRowCount))
	} else {
		setText(mrpMethodsHeader, fmt.Sprintf("Наработка по способам · %d схем · все объекты архива", mrpMethodsRowCount))
	}
}

func populateMRPMethods(in MRPForecastInput) {
	rows, _ := mrpRollingRows(in)
	listBeginUpdate(mrpMethodsList)
	defer listEndUpdate(mrpMethodsList)
	listClear(mrpMethodsList)
	if len(rows) == 0 {
		listAdd(mrpMethodsList, 0, []string{"Нет подтверждённых циклов с известным способом", "—", "0", "—", "—", "—", "—"})
	}
	for i, row := range rows {
		three, four := "нет 3 циклов", "нет 4 циклов"
		if row.Mean3 > 0 {
			three = fmt.Sprintf("%.0f (%d об.)", row.Mean3, row.Ready3)
		}
		if row.Mean4 > 0 {
			four = fmt.Sprintf("%.0f (%d об.)", row.Mean4, row.Ready4)
		}
		listAdd(mrpMethodsList, i, []string{row.Method, fmt.Sprintf("%d", len(row.Objects)), fmt.Sprintf("%d", len(row.Cycles)), fmt.Sprintf("%.0f", row.TotalDays), fmt.Sprintf("%.0f сут.", row.MeanAll), three, four})
	}
	mrpMethodsRowCount = len(rows)
	refreshMRPMethodsHeader()
}

func applyMRPForecastToPage(r MRPForecastResult) {
	scenarios := mrpScenarioBand(r)
	cards := []syscall.Handle{mrpCurrentLabel, mrpBaseLabel, mrpTargetStatusLabel, mrpRangeLabel}
	for i, h := range cards {
		if i < len(scenarios) {
			setText(h, mrpScenarioCardText(scenarios[i]))
		} else {
			setText(h, "—")
		}
	}
	current := "нет данных"
	if r.CurrentMRPDays > 0 {
		current = fmt.Sprintf("%.0f сут.", r.CurrentMRPDays)
		if r.Advanced.CurrentAgeDays > 0 {
			current += fmt.Sprintf(" · текущий цикл %.0f сут.", r.Advanced.CurrentAgeDays)
		}
	} else if r.Advanced.CurrentAgeDays > 0 {
		current = fmt.Sprintf("текущий цикл %.0f сут.", r.Advanced.CurrentAgeDays)
	}
	ev := r.Advanced.ObjectAny.EventCount
	cens := r.Advanced.ObjectAny.CensoredCount
	setText(mrpEvidenceLabel, fmt.Sprintf("Текущий МРП: %s  ·  цель: %.0f сут. (%s)  ·  история: %d завершённых циклов  ·  событий: %d отказа / %d в работе", current, r.TargetMRPDays, r.TargetStatus, r.ObservationCount, ev, cens))

	s := r.Advanced.ObjectAny
	probabilityReady := s.EventCount >= 5 && s.Backtest.Eligible
	setText(mrpSurvival90Label, formatProbability(s.Survival90More, probabilityReady && s.HasSurvival90More))
	setText(mrpSurvival180Label, formatProbability(s.Survival180More, probabilityReady && s.HasSurvival180More))
	setText(mrpSurvival365Label, formatProbability(s.Survival365More, probabilityReady && s.HasSurvival365More))
	if probabilityReady && r.Advanced.Competing.Has365 {
		setText(mrpFailure365Label, fmt.Sprintf("%.0f%%", r.Advanced.Competing.FailureCIF365*100))
	} else {
		setText(mrpFailure365Label, "не рассчитано")
	}
	detail := formatMRPScenarioDetail(scenarios) + "\r\n\r\n" + mrpAccumulationStatus(r) + "\r\n\r\n" + formatMRPForecast(r)
	stats := outcomeCycleStatisticsTextForObject(r.ObjectKey)
	if strings.TrimSpace(stats) != "" {
		detail += "\r\n\r\nФАКТИЧЕСКАЯ СТРУКТУРА ЗАВЕРШЕНИЙ ЦИКЛОВ\r\n" + stats
	}
	setMRPDetailText(detail)
}

func saveMRPTargetFromUI() {
	st, ok := currentSelectedState()
	if !ok {
		msg("Целевой ориентир", "Сначала выберите объект.", MB_OK|MB_ICONWARNING)
		return
	}
	v, err := parseEconomicNumber(getText(mrpTargetEdit), "Целевой ориентир наработки", true)
	if err != nil {
		msg("Целевой ориентир", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	scope := targetScopeFromUI()
	switch scope {
	case targetScopeGlobal:
		err = saveTargetRunlifeGlobal(v)
	case targetScopeLift:
		err = saveTargetRunlifeForLift(st.Equipment, v)
	default:
		err = saveTargetRunlifeForObjectWithEquipment(st.ObjectKey, st.Equipment, v)
	}
	if err != nil {
		msg("Целевой ориентир", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	refreshMRPForecastPage()
	refreshResultPage()
	msg("Целевой ориентир", fmt.Sprintf("Сохранён %s: %.0f суток. Это настраиваемая цель, а не жёсткий норматив.", targetScopeLabel(scope), v), MB_OK|MB_ICONINFORMATION)
}

func resetMRPTargetFromUI() {
	st, ok := currentSelectedState()
	if !ok {
		msg("Целевой ориентир", "Сначала выберите объект.", MB_OK|MB_ICONWARNING)
		return
	}
	scope := targetScopeFromUI()
	var err error
	switch scope {
	case targetScopeGlobal:
		err = resetTargetRunlifeGlobal()
	case targetScopeLift:
		err = resetTargetRunlifeForLift(st.Equipment)
	default:
		err = resetTargetRunlifeForObjectWithEquipment(st.ObjectKey, st.Equipment)
	}
	if err != nil {
		msg("Целевой ориентир", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	refreshMRPForecastPage()
	refreshResultPage()
	effective := targetRunlifeResolved(st.ObjectKey, st.Equipment)
	msg("Целевой ориентир", fmt.Sprintf("Настройка уровня «%s» сброшена. Для объекта теперь действует %.0f суток (%s).", targetScopeLabel(scope), effective.Days, effective.Source), MB_OK|MB_ICONINFORMATION)
}

func handleMRPTargetScopeChange() {
	refreshTargetEditorForScope()
}

func calculateMRPForecastFromUI() {
	in, ok := mrpForecastInputForSelectedObject()
	if !ok {
		msg("Прогноз МРП", "Сначала выберите объект.", MB_OK|MB_ICONWARNING)
		return
	}
	r := calculateMRPForecast(in)
	populateMRPHistory(in)
	populateMRPMethods(in)
	applyMRPForecastToPage(r)
	if r.BaseDays <= 0 {
		msg("Прогноз МРП", strings.Join(r.Warnings, "\n"), MB_OK|MB_ICONWARNING)
	}
}

func setMRPLayoutVisibility(h syscall.Handle, yes bool) {
	if h == 0 {
		return
	}
	if current, ok := pageControlVisible[h]; ok && current == yes {
		return
	}
	show(h, yes)
}

func beginMRPAtomicLayout() {
	mrpAtomicLayoutDepth++
	if mrpAtomicLayoutDepth == 1 {
		beginPageVisualUpdate(ID_MRP)
	}
}

func endMRPAtomicLayout() {
	if mrpAtomicLayoutDepth == 0 {
		return
	}
	mrpAtomicLayoutDepth--
	if mrpAtomicLayoutDepth == 0 {
		endPageVisualUpdate(ID_MRP)
	}
}

func layoutMRPForecast(w, h int32) {
	beginMRPAtomicLayout()
	defer endMRPAtomicLayout()
	if nnoFleetEnabled {
		layoutNNOFleetUI(w, h)
		return
	}
	setNNOFleetVisibility(false)
	for _, old := range nnoOldHandles() {
		setMRPLayoutVisibility(old, true)
	}
	setMRPLayoutVisibility(nnoFleetBack, true)
	xBack := int32(242) + max32(720, w-270) - 190
	setPos(nnoFleetBack, xBack, py(94), 188, 42)
	x := int32(242)
	contentW := max32(720, w-270)
	compact := h < 960
	veryCompact := h < 760
	mrpCompactLayout = compact
	refreshMRPMethodsHeader()

	setPos(mrpPageTitle, x, py(94), contentW, 34)
	if compact {
		setText(mrpHistoryHeader, "История МРП выбранного объекта")
		setMRPLayoutVisibility(mrpIntroLabel, false)
		setPos(mrpObjectLabel, x, py(122), contentW, 24)

		// One dense action row keeps the scenario strip and at least four history
		// rows on a 1366x768-class screen. The target remains editable here; no
		// functionality is hidden behind a smaller window.
		rowY := py(152)
		rowH := int32(38)
		gap := int32(8)
		actionW := contentW * 48 / 100
		guideW := max32(120, actionW*27/100)
		calcW := max32(150, actionW*35/100)
		drillW := actionW - guideW - calcW - 2*gap
		setPos(mrpGuideButton, x, rowY, guideW, rowH)
		setPos(mrpCalculateButton, x+guideW+gap, rowY, calcW, rowH)
		setPos(mrpDrillButton, x+guideW+gap+calcW+gap, rowY, drillW, rowH)

		targetX := x + actionW + 12
		targetW := contentW - actionW - 12
		labelW := max32(62, targetW*14/100)
		editW := max32(60, targetW*13/100)
		saveW := max32(106, targetW*23/100)
		resetW := max32(78, targetW*17/100)
		comboW := targetW - labelW - editW - saveW - resetW - 4*5
		if comboW < 108 {
			deficit := 108 - comboW
			saveW = max32(98, saveW-deficit)
			comboW = targetW - labelW - editW - saveW - resetW - 4*5
		}
		cx := targetX
		setText(mrpTargetLabel, "Цель, сут.")
		setText(mrpTargetSaveButton, "Сохранить")
		setPos(mrpTargetLabel, cx, rowY+8, labelW, 22)
		cx += labelW + 5
		setPos(mrpTargetEdit, cx, rowY+2, editW, 34)
		cx += editW + 5
		setPos(mrpTargetScopeCombo, cx, rowY+2, comboW, 160)
		cx += comboW + 5
		setPos(mrpTargetSaveButton, cx, rowY, saveW, rowH)
		cx += saveW + 5
		setPos(mrpTargetResetButton, cx, rowY, resetW, rowH)
		setMRPLayoutVisibility(mrpTargetLabel, true)
		setMRPLayoutVisibility(mrpTargetScopeLabel, false)

		setPos(mrpTargetSourceLabel, x, py(194), contentW, 30)
		setPos(mrpCauseSummaryLabel, x, py(226), contentW, 18)
		setPos(mrpScenarioBandTitle, x, py(248), contentW, 20)

		cardGap := int32(8)
		cardW := (contentW - 3*cardGap) / 4
		headers := []syscall.Handle{mrpCurrentHeader, mrpBaseHeader, mrpTargetHeader, mrpRangeHeader}
		cards := []syscall.Handle{mrpCurrentLabel, mrpBaseLabel, mrpTargetStatusLabel, mrpRangeLabel}
		for i, hCard := range cards {
			cardX := x + int32(i)*(cardW+cardGap)
			setPos(mrpMetricBackgrounds[i], cardX, py(270), cardW, 74)
			setPos(headers[i], cardX+8, py(272), cardW-16, 18)
			setPos(hCard, cardX+8, py(292), cardW-16, 49)
		}
		setPos(mrpRollingBackground, x, py(346), contentW, 20)
		setPos(mrpEvidenceLabel, x+8, py(348), contentW-16, 18)

		lowerY := py(370)
		if veryCompact {
			// Below 760 px there is not enough honest vertical space for two tables
			// plus a text editor. Preserve the primary object history with a useful
			// viewport; the complete scheme calculation remains one click away via
			// "Проверить расчёт" and the full rationale via the button below.
			setMRPLayoutVisibility(mrpHistoryHeader, true)
			setMRPLayoutVisibility(mrpHistoryList, true)
			setMRPLayoutVisibility(mrpMethodsHeader, false)
			setMRPLayoutVisibility(mrpMethodsList, false)
			setMRPLayoutVisibility(mrpRollingHeader, false)
			setMRPLayoutVisibility(mrpRollingLabel, true)
			setMRPLayoutVisibility(mrpDetailHeader, false)
			setMRPLayoutVisibility(mrpDetail, false)
			setMRPLayoutVisibility(mrpDetailButton, true)
			setPos(mrpHistoryHeader, x, lowerY, contentW, 20)
			listY := lowerY + 22
			buttonH := int32(36)
			buttonY := max32(listY+98, h-10-buttonH)
			rollingY := buttonY - 24
			listH := max32(66, rollingY-listY-6)
			setPos(mrpHistoryList, x, listY, contentW, listH)
			setPos(mrpRollingLabel, x, rollingY, contentW, 20)
			setPos(mrpDetailButton, x, buttonY, contentW, buttonH)
			layoutMRPTableColumns(contentW, contentW)
			return
		}

		// 760–899 px: show both data sets simultaneously as two stable columns.
		// Native ListView horizontal scrolling preserves readable column widths;
		// text is never silently cut just to force eight columns into half a page.
		setMRPLayoutVisibility(mrpHistoryHeader, true)
		setMRPLayoutVisibility(mrpHistoryList, true)
		setMRPLayoutVisibility(mrpMethodsHeader, true)
		setMRPLayoutVisibility(mrpMethodsList, true)
		setMRPLayoutVisibility(mrpRollingHeader, false)
		setMRPLayoutVisibility(mrpRollingLabel, false)
		setMRPLayoutVisibility(mrpDetailHeader, false)
		setMRPLayoutVisibility(mrpDetail, false)
		setMRPLayoutVisibility(mrpDetailButton, true)
		colGap := int32(10)
		leftW := (contentW - colGap) * 56 / 100
		rightW := contentW - colGap - leftW
		setPos(mrpHistoryHeader, x, lowerY, leftW, 20)
		setPos(mrpMethodsHeader, x+leftW+colGap, lowerY, rightW, 20)
		listY := lowerY + 22
		buttonH := int32(36)
		buttonY := max32(listY+92, h-10-buttonH)
		listH := max32(82, buttonY-listY-6)
		setPos(mrpHistoryList, x, listY, leftW, listH)
		setPos(mrpMethodsList, x+leftW+colGap, listY, rightW, listH)
		setPos(mrpDetailButton, x, buttonY, contentW, buttonH)
		layoutMRPTableColumns(leftW, rightW)
		return
	}

	setText(mrpHistoryHeader, "Фактическая история МРП по выбранному объекту")
	setMRPLayoutVisibility(mrpIntroLabel, true)
	setMRPLayoutVisibility(mrpHistoryHeader, true)
	setMRPLayoutVisibility(mrpHistoryList, true)
	setMRPLayoutVisibility(mrpMethodsHeader, true)
	setMRPLayoutVisibility(mrpMethodsList, true)
	setMRPLayoutVisibility(mrpRollingHeader, false)
	setMRPLayoutVisibility(mrpRollingLabel, false)
	setMRPLayoutVisibility(mrpDetailHeader, true)
	setMRPLayoutVisibility(mrpDetail, true)
	setMRPLayoutVisibility(mrpDetailButton, false)
	setText(mrpTargetLabel, "Целевой ориентир, сут.")
	setText(mrpTargetScopeLabel, "Уровень настройки")
	setText(mrpTargetSaveButton, "Сохранить цель")
	setMRPLayoutVisibility(mrpTargetLabel, true)
	setMRPLayoutVisibility(mrpTargetScopeLabel, true)
	setPos(mrpIntroLabel, x, py(132), contentW, 42)
	setPos(mrpObjectLabel, x, py(181), contentW, 30)
	setPos(mrpGuideButton, x, py(220), 198, 42)
	setPos(mrpCalculateButton, x+208, py(220), 230, 42)
	setPos(mrpDrillButton, x+448, py(220), 230, 42)
	setPos(mrpTargetLabel, x, py(282), 185, 26)
	setPos(mrpTargetEdit, x+190, py(275), 86, 38)
	setPos(mrpTargetScopeLabel, x+288, py(282), 145, 26)
	setPos(mrpTargetScopeCombo, x+438, py(275), 170, 160)
	setPos(mrpTargetSaveButton, x+618, py(274), 140, 42)
	setPos(mrpTargetResetButton, x+768, py(274), 140, 42)
	setPos(mrpTargetSourceLabel, x, py(318), contentW, 20)
	setPos(mrpCauseSummaryLabel, x, py(341), contentW, 20)
	setPos(mrpScenarioBandTitle, x, py(363), contentW, 20)
	cardGap := int32(10)
	cardW := (contentW - 3*cardGap) / 4
	headers := []syscall.Handle{mrpCurrentHeader, mrpBaseHeader, mrpTargetHeader, mrpRangeHeader}
	cards := []syscall.Handle{mrpCurrentLabel, mrpBaseLabel, mrpTargetStatusLabel, mrpRangeLabel}
	for i, hCard := range cards {
		cardX := x + int32(i)*(cardW+cardGap)
		setPos(mrpMetricBackgrounds[i], cardX, py(386), cardW, 79)
		setPos(headers[i], cardX+8, py(388), cardW-16, 20)
		setPos(hCard, cardX+8, py(409), cardW-16, 53)
	}
	setPos(mrpRollingBackground, x, py(468), contentW, 19)
	setPos(mrpEvidenceLabel, x+8, py(469), contentW-16, 17)
	sectionY := py(490)
	payload := max32(210, h-sectionY-16-3*26-3*8)
	historyH := max32(88, payload*30/100)
	methodsH := max32(92, payload*32/100)
	setPos(mrpHistoryHeader, x, sectionY, contentW, 26)
	setPos(mrpHistoryList, x, sectionY+26, contentW, historyH)
	methodsY := sectionY + 26 + historyH + 8
	setPos(mrpMethodsHeader, x, methodsY, contentW, 26)
	setPos(mrpMethodsList, x, methodsY+26, contentW, methodsH)
	detailY := methodsY + 26 + methodsH + 8
	setPos(mrpDetailHeader, x, detailY, contentW, 26)
	setPos(mrpDetail, x, detailY+26, contentW, max32(82, h-(detailY+26)-14))
	layoutMRPTableColumns(contentW, contentW)
}

// Fill the actual ListView width instead of leaving an unlabelled strip after
// the last fixed-width column when the window grows.
func layoutMRPTableColumns(historyW, methodsW int32) {
	setWidths := func(list syscall.Handle, clientW int32, weights []int32, minimumTotal int32) {
		available := max32(minimumTotal, clientW-19) // preserve text; ListView scrolls horizontally when compact
		used := int32(0)
		for i, weight := range weights {
			width := available * weight / 100
			if i == len(weights)-1 {
				width = available - used
			}
			setListColumnWidth(list, i, width)
			used += width
		}
	}
	setWidths(mrpHistoryList, historyW, []int32{8, 9, 11, 20, 12, 8, 12, 20}, 860)
	setWidths(mrpMethodsList, methodsW, []int32{29, 11, 8, 10, 15, 14, 13}, 720)
}
