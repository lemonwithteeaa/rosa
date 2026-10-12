//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// Section 6: presentation only. Do not change the economics calculations,
// export implementation or scenario persistence in this file.
var (
	econSchemesTitle      syscall.Handle
	econSchemesList       syscall.Handle
	econInputsTitle       syscall.Handle
	econCompareTitle      syscall.Handle
	econCompareList       syscall.Handle
	econFundFilterCombo   syscall.Handle
	econConclusionTitle   syscall.Handle
	econTechnologistBrief syscall.Handle
	econHeaderDivider     syscall.Handle
	econAtomicLayoutDepth int
	econ812TableHeader    syscall.Handle
	econ812Breakdown      syscall.Handle
	econ812Units          [5]syscall.Handle
	econ812States         [5]syscall.Handle
	econ812Rules          [5]syscall.Handle
	econ812Columns        [4]int32
	econ812Cards          [3]syscall.Handle
	econ812Details        syscall.Handle
	econ813BreakTitle     syscall.Handle
	econ813ScenarioHint   syscall.Handle
)

func buildEconomicsVisualReference() {
	econSchemesTitle = label(ID_ECON, "СРАВНЕНИЕ ПАРАЛЛЕЛЬНЫХ СХЕМ", 242, 198, 900, 24)
	econTemplateSaveButton = button(ID_ECON, "Скачать бланк", 0, 0, 145, 38, ID_ECON_TEMPLATE_SAVE)
	econTemplateLoadButton = button(ID_ECON, "Загрузить бланк", 0, 0, 165, 38, ID_ECON_TEMPLATE_LOAD)
	econSchemesList = newList(ID_ECON, 242, 222, 900, 110, []struct {
		n string
		w int32
	}{
		{"Схема", 76}, {"Методы / мероприятия", 280}, {"Доступность", 130},
		{"МРП, сут.", 105}, {"OPEX, руб./год", 143}, {"CAPEX, руб.", 135},
		{"Чистый эффект, тыс. руб.", 155}, {"Статус", 136},
	})
	econInputsTitle = label(ID_ECON, "ИСХОДНЫЕ ЭКОНОМИЧЕСКИЕ ПАРАМЕТРЫ", 242, 343, 900, 24)
	econCompareTitle = label(ID_ECON, "НАКОПИТЕЛЬНАЯ ЭКОНОМИКА ФОНДА · ДВОЙНОЙ КЛИК — ПОДРОБНО", 242, 492, 900, 24)
	// The dropdown is intentionally native: no extra hover handlers or modal loops.
	econFundFilterCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST,
		0, 0, 254, 180, hMain, ID_ECON_FUND_FILTER)
	addPageControl(ID_ECON, econFundFilterCombo)
	for _, option := range []string{"Все объекты", "Требуют расчёта", "Есть расчёты", "Положительный эффект"} {
		pSendMessageW.Call(uintptr(econFundFilterCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(option))))
	}
	pSendMessageW.Call(uintptr(econFundFilterCombo), CB_SETCURSEL, 0, 0)
	if hFontButtonCompact != 0 {
		pSendMessageW.Call(uintptr(econFundFilterCombo), WM_SETFONT, uintptr(hFontButtonCompact), 1)
	}
	econCompareList = newList(ID_ECON, 242, 516, 900, 110, []struct {
		n string
		w int32
	}{
		{"Объект", 280}, {"Схем, шт.", 100}, {"Рассчитано", 110},
		{"Макс. МРП, сут.", 155}, {"Лучший эффект, тыс. руб.", 190},
		{"Обновлено", 160}, {"Состояние", 230},
	})
	econConclusionTitle = label(ID_ECON, "РАСЧЁТ И ВЫВОД ДЛЯ ТЕХНОЛОГА", 242, 620, 900, 24)
	// Always-visible concise assessment; the original full result stays
	// available through the 'Formulas and assumptions' action.
	econTechnologistBrief = edit(ID_ECON, "ВЫВОД ТЕХНОЛОГУ: выполните расчёт выбранной схемы.", 242, 620, 900, 54, true, true, 0)
	// A subtle native separator closes the identity/status block. No custom painting.
	econHeaderDivider = create("STATIC", "", WS_CHILD|0x00000010 /* SS_ETCHEDHORZ */, 242, py(246), 900, 2, hMain, 0)
	addPageControl(ID_ECON, econHeaderDivider)
	for _, h := range []syscall.Handle{econSchemesTitle, econInputsTitle, econCompareTitle, econConclusionTitle} {
		if h != 0 && hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}
	}
	buildEconomicsMetricCards()
	buildEconomics812Controls()
	rosaUIApplyEconomicsStyles()
}

func econReferenceMoney(v float64, known bool) string {
	if !known {
		return "—"
	}
	return fmt.Sprintf("%.0f", v)
}
func econReferenceMRP(v float64) string {
	if v <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", v)
}
func econReferenceStatus(s AlternativeMethodScenario) string {
	if strings.EqualFold(strings.TrimSpace(s.Status), "Исходные данные изменились") {
		return "Исходные данные изменились"
	}
	if strings.TrimSpace(s.EconomicsStatus) != "" {
		return s.EconomicsStatus
	}
	if strings.TrimSpace(s.Status) != "" {
		return s.Status
	}
	return "Не рассчитано"
}

func refreshEconomicsVisualReference() {
	if econSchemesList == 0 || econCompareList == 0 {
		return
	}
	schemes := []AlternativeMethodScenario(nil)
	if strings.TrimSpace(selectedObjectKey) != "" {
		schemes = alternativeScenariosForObject(selectedObjectKey)
	}
	listBeginUpdate(econSchemesList)
	defer listEndUpdate(econSchemesList)
	listClear(econSchemesList)
	if len(schemes) == 0 {
		listAdd(econSchemesList, 0, []string{"—", "Сначала сформируйте схемы в разделе 4", "—", "—", "—", "—", "—", "Нет данных"})
		refreshEconomicsFundArchive()
		refreshEconomicsMetricCards()
		refreshEconomics812Display()
		refreshEconomicsTechnologistBrief()
		refreshEconomics812ScenarioCards()
		return
	}
	for i, s := range schemes {
		methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
		if methods == "" {
			methods = "Методы не выбраны"
		}
		status := econReferenceStatus(s)
		costKnown := strings.TrimSpace(s.EconomicsCalculated) != "" &&
			strings.EqualFold(strings.TrimSpace(s.EconomicsStatus), "Расчёт актуален")
		kind := strings.TrimSpace(s.SelectionKind)
		schemeName := strings.TrimSpace(s.Name)
		if kind != "" {
			schemeName += " · " + kind
		}
		listAdd(econSchemesList, i, []string{
			schemeName, methods, dash(s.Availability), econReferenceMRP(s.ForecastBaseDays),
			econReferenceMoney(s.AnnualOPEXRub, costKnown || s.AnnualOPEXRub != 0),
			econReferenceMoney(s.OneTimeCAPEXRub, costKnown || s.OneTimeCAPEXRub != 0),
			econReferenceMoney(s.NetBenefitK, costKnown), status,
		})
		if strings.EqualFold(s.ID, activeAlternativeScenarioID(selectedObjectKey)) {
			selectListRow(econSchemesList, i)
		}
	}
	// Cross-fund archive is never limited to the globally selected object.
	refreshEconomicsFundArchive()
	refreshEconomics812Display()
	refreshEconomicsTechnologistBrief()
	refreshEconomics812ScenarioCards()
}

func selectEconomicsSchemeFromTable(row int) {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return
	}
	schemes := alternativeScenariosForObject(selectedObjectKey)
	if row < 0 || row >= len(schemes) {
		return
	}
	selected := schemes[row]
	if strings.EqualFold(selected.ID, activeAlternativeScenarioID(selectedObjectKey)) {
		return
	}
	if _, err := setAlternativeScenarioActiveByID(selectedObjectKey, selected.ID); err != nil {
		msg("Экономическая оценка", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	useSelectedObjectForEconomics()
	refreshEconomicsDecisionIndicators()
}

func layoutEconomicsReferenceColumns(list syscall.Handle, total int32, weights, minimums []int32) {
	if list == 0 || len(weights) != len(minimums) {
		return
	}
	// Wider windows have no blank trailing column; narrow windows get
	// native horizontal scrolling instead of truncated headings.
	available := total - 18
	if available < 0 {
		available = 0
	}
	widths := make([]int32, len(weights))
	sum := int32(0)
	for i := range weights {
		widths[i] = max32(minimums[i], available*weights[i]/100)
		sum += widths[i]
	}
	if sum < available {
		widths[len(widths)-1] += available - sum
	}
	for i, width := range widths {
		setListColumnWidth(list, i, width)
	}
}

func setEconomicsReferenceVisible(h syscall.Handle, wanted bool) {
	if h != 0 && pageControlVisible[h] != wanted {
		show(h, wanted)
	}
}
func beginEconomicsAtomicLayout() {
	econAtomicLayoutDepth++
	if econAtomicLayoutDepth == 1 {
		beginPageVisualUpdate(ID_ECON)
	}
}
func endEconomicsAtomicLayout() {
	if econAtomicLayoutDepth == 0 {
		return
	}
	econAtomicLayoutDepth--
	if econAtomicLayoutDepth == 0 {
		endPageVisualUpdate(ID_ECON)
	}
}

// economicsHeaderLayout describes stable bands rather than independent positions.
// The former scheme buttons were laid out at 35px despite the shared button
// control needing >=40px; the caption was clipped and the row appeared broken.
// Every interactive item here now has a measured, identical 44px frame.
type economicsHeaderLayout struct {
	titleY, subtitleY, statusY, objectY, dividerY, sectionY, listY int32
	objectButtonY, schemeButtonY                                   int32
}

func economicsHeaderGeometry() economicsHeaderLayout {
	return economicsHeaderLayout{
		titleY: 94, subtitleY: 136, statusY: 164, objectY: 197,
		dividerY: 244, sectionY: 258, listY: 307,
		objectButtonY: 195, schemeButtonY: 255,
	}
}

// Avoid forcing new paint operations when Win32 geometry is unchanged.
func setEconomicsCaption(h syscall.Handle, caption string) {
	if h != 0 && getText(h) != caption {
		setText(h, caption)
	}
}

func setEconomicsFieldCaption(h syscall.Handle, index int, condensed bool, tight bool) {
	long := map[int]string{
		0: "Текущий МРП, сут.", 5: "Горизонт, полных месяцев",
		6: "Длительность ремонта, сут.", 7: "Стоимость одного ремонта, руб.",
		9: "Инфляция затрат, %/год",
	}
	short := map[int]string{
		0: "Текущий МРП, сут.", 5: "Горизонт, мес.",
		6: "Ремонт, сут.", 7: "Стоимость ремонта, руб.",
		9: "Инфляция, %/год",
	}
	// The last unit is always a dedicated column: do not repeat it in a
	// narrow caption. This retains meaning without reducing the font.
	if tight {
		minimal := map[int]string{0: "Текущий МРП", 5: "Горизонт", 6: "Время ремонта", 7: "Цена ремонта", 9: "Инфляция"}
		setEconomicsCaption(h, minimal[index])
	} else if condensed {
		setEconomicsCaption(h, short[index])
	} else {
		setEconomicsCaption(h, long[index])
	}
}

// Native two-column economics dashboard. The content hierarchy follows the
// accepted visual reference: KPI cards -> inputs/scenario comparison -> fund.
// The fund table always gets its own height budget before optional commentary.
// 812-RC: reference-driven layout. Absolutely no child background panes.
// Every control is individually positioned; no owner-draw surface overlaps an
// EDIT or LISTVIEW. All coordinates are client-relative via py() exactly once.
// 813-RC. The approved reference's visual hierarchy without child-overlay HWNDs.
// Parent WM_ERASEBKGND paints panel backgrounds behind WS_CLIPCHILDREN controls.
func layoutEconomicsVisualReference(w, h int32) {
	beginEconomicsAtomicLayout()
	defer endEconomicsAtomicLayout()
	const x int32 = 242
	cw := max32(760, w-270)
	compact := h <= 1020
	ultra := h <= 805

	setPos(econPageTitle, x, py(92), max32(310, cw-329), 39)
	setPos(econTemplateSaveButton, x+cw-314, py(92), 148, 41)
	setPos(econTemplateLoadButton, x+cw-157, py(92), 157, 41)
	setPos(econModeInfo, x, py(130), cw, 21)
	setPos(econDecisionStatus, x, py(151), cw-290, 20)
	setEconomicsReferenceVisible(econDeferralInfo, false) // Deferral is in the KPI row.
	setPos(econObjectLabel, x, py(175), cw-262, 22)
	// Keep the control ABOVE the KPI strip (812 overlapped this by 13px).
	setPos(econUseObjectButton, x+cw-256, py(150), 256, 47)
	setEconomicsReferenceVisible(econHeaderDivider, false)

	cardY, cardH := int32(209), int32(78)
	if compact {
		cardY, cardH = 203, 71
	}
	if ultra {
		cardY, cardH = 198, 60
	}
	layoutEconomicsMetricCards(x, cardY, cw, cardH)
	areaY := cardY + cardH + 11
	gap := int32(12)
	leftW := max32(420, cw*54/100)
	if leftW > cw-350 {
		leftW = cw - 350
	}
	rightX := x + leftW + gap
	rightW := cw - leftW - gap
	innerLeft := leftW - 24

	// Preserve enough content height for the native five-row parameter editor.
	rowHeight := int32(35)
	headerH := int32(26)
	headOffset := int32(44)
	buttonsGap := int32(8)
	panelFooter := int32(50)
	breakGap := int32(8)
	breakH := int32(112)
	fundGap := int32(8)
	fundHeader := int32(34)
	if compact {
		rowHeight, headerH, headOffset, buttonsGap, panelFooter = 28, 24, 44, 5, 44
		breakGap, breakH, fundGap, fundHeader = 7, 78, 6, 32
	}
	if ultra {
		rowHeight, headerH, headOffset, buttonsGap, panelFooter = 24, 22, 44, 5, 43
		breakGap, breakH, fundGap, fundHeader = 6, 54, 6, 30
	}
	headY := areaY + headOffset
	rowY := headY + headerH + 1
	buttonsY := rowY + 5*rowHeight + buttonsGap
	bottomPanels := buttonsY + panelFooter
	breakY := bottomPanels + breakGap
	fundTitleY := breakY + breakH + fundGap
	fundY := fundTitleY + fundHeader
	fundH := h - py(fundY) - 10
	// At 768px there is only room for the heading and two data rows.
	// Respect the client bottom rather than clipping the grid border.
	if fundH < 66 {
		fundH = 66
	}
	if fundH > 325 {
		fundH = 325
	}
	// Store the parent-painted panel bounds. All child controls stay above them.
	econ813Panels = [4]RECT{
		{Left: x, Top: py(areaY), Right: x + leftW, Bottom: py(bottomPanels)},
		{Left: rightX, Top: py(areaY), Right: rightX + rightW, Bottom: py(bottomPanels)},
		{Left: x, Top: py(breakY), Right: x + cw, Bottom: py(breakY + breakH)},
		{Left: x, Top: py(fundTitleY), Right: x + cw, Bottom: py(fundY + fundH + 5)},
	}

	setPos(econInputsTitle, x+14, py(areaY+9), max32(190, leftW-216), 25)
	setEconomicsCaption(econInputsTitle, "Исходные экономические параметры")
	setPos(econCalculateAllButton, x+leftW-169, py(areaY+2), 159, 40)
	setEconomicsCaption(econCalculateAllButton, "Пересчитать")
	nameW := innerLeft * 42 / 100
	valueW := max32(96, innerLeft*22/100)
	unitW := max32(64, innerLeft*13/100)
	statusW := innerLeft - nameW - valueW - unitW
	if statusW < 73 {
		statusW = 73
		nameW = innerLeft - valueW - unitW - statusW
	}
	econ812Columns = [4]int32{nameW, valueW, unitW, statusW}
	setPos(econ812TableHeader, x+12, py(headY), innerLeft, headerH)
	unitCaptions := []string{"сут.", "мес.", "сут.", "руб.", "%/год"}
	indices := []int{0, 5, 6, 7, 9}
	inputs := economicsInputHandles()
	for row, index := range indices {
		y := rowY + int32(row)*rowHeight
		setEconomicsFieldCaption(econFieldLabels[index], index, compact, nameW < 195)
		setPos(econFieldLabels[index], x+16, py(y+3), nameW-10, rowHeight-5)
		setPos(inputs[index], x+12+nameW+3, py(y+1), valueW-10, max32(23, rowHeight-3))
		setEconomicsCaption(econ812Units[row], unitCaptions[row])
		setPos(econ812Units[row], x+12+nameW+valueW+5, py(y+3), unitW-8, rowHeight-5)
		setPos(econ812States[row], x+12+nameW+valueW+unitW+2, py(y+3), statusW-7, max32(20, rowHeight-5))
		setPos(econ812Rules[row], x+12, py(y+rowHeight-2), innerLeft, 1)
	}
	leftButtonWidth := (leftW - 34) / 2
	setPos(econFullResultButton, x+13, py(buttonsY), leftButtonWidth, rosaUIButtonHeight)
	setEconomicsCaption(econFullResultButton, "Формулы и допущения ›")
	setPos(econChartButton, x+21+leftButtonWidth, py(buttonsY), leftButtonWidth, rosaUIButtonHeight)
	setEconomicsCaption(econChartButton, "Диаграмма сравнения ›")

	setPos(econSchemesTitle, rightX+14, py(areaY+9), max32(76, rightW-257), 25)
	if rightW < 485 {
		setEconomicsCaption(econSchemesTitle, "Схемы")
	} else {
		setEconomicsCaption(econSchemesTitle, "Альтернативные схемы")
	}
	setPos(econMethodBaseButton, rightX+rightW-233, py(areaY+2), 108, 40)
	setPos(econMethodNextButton, rightX+rightW-119, py(areaY+2), 109, 40)
	setEconomicsCaption(econMethodBaseButton, "Первая")
	setEconomicsCaption(econMethodNextButton, "Следующая ›")
	setEconomicsReferenceVisible(econSchemesList, false)
	cardsY := areaY + 53
	cardGap := int32(7)
	cardHeight := int32(57)
	if compact {
		cardHeight, cardGap, cardsY = 49, 5, areaY+46
	}
	if ultra {
		cardHeight, cardGap, cardsY = 43, 4, areaY+44
	}
	for i, control := range econ812Cards {
		setPos(control, rightX+13, py(cardsY+int32(i)*(cardHeight+cardGap)), rightW-26, cardHeight)
	}
	// The old editable scenario name is redundant and caused a visual regression.
	// Its state stays intact for calculations and persistence, but the card is
	// the visible selection control, as in the reference.
	setEconomicsReferenceVisible(econScenarioCard, false)
	// Reserve enough native button width for the full 17px caption on DPI
	// scaled desktops. Never reduce the text size to make it fit.
	const detailsWidth int32 = 238
	setPos(econ812Details, rightX+rightW-detailsWidth-14, py(buttonsY), detailsWidth, rosaUIButtonHeight)
	setEconomicsCaption(econ812Details, "Подробнее о схеме")
	// The hint and action must never compete for the same pixels.
	hintWidth := rightW - detailsWidth - 45
	setEconomicsReferenceVisible(econ813ScenarioHint, hintWidth >= 165)
	if hintWidth >= 165 {
		setPos(econ813ScenarioHint, rightX+15, py(buttonsY+8), hintWidth, 27)
	}
	setEconomicsReferenceVisible(econCalculateButton, false)

	setEconomicsReferenceVisible(econ813BreakTitle, false)
	// Preserve the quantitative breakdown on normal desktops. On compact
	// screens prioritize a readable engineering conclusion in the same band.
	if compact {
		// On laptop-height work areas there is no room to squeeze two readable
		// text panels into the same 54–78 px band. Keep the conclusion readable;
		// numeric breakdown remains available in the formula details.
		setEconomicsReferenceVisible(econ812Breakdown, false)
		setPos(econTechnologistBrief, x, py(breakY), cw, breakH)
	} else {
		breakdownH := int32(64)
		setEconomicsReferenceVisible(econ812Breakdown, true)
		setPos(econ812Breakdown, x, py(breakY), cw, breakdownH)
		setPos(econTechnologistBrief, x, py(breakY+breakdownH+2), cw, breakH-breakdownH-2)
	}
	setEconomicsReferenceVisible(econTechnologistBrief, true)
	setEconomicsReferenceVisible(econConclusionTitle, false)
	setEconomicsReferenceVisible(econResult, false)

	filterW := min32(240, max32(185, cw/5))
	setPos(econCompareTitle, x+14, py(fundTitleY+7), cw-filterW-33, 24)
	setEconomicsCaption(econCompareTitle, "Накопительная экономика фонда · двойной щелчок — детали")
	setPos(econFundFilterCombo, x+cw-filterW-11, py(fundTitleY+2), filterW, 180)
	setPos(econCompareList, x+12, py(fundY), cw-24, fundH)
	layoutEconomicsReferenceColumns(econCompareList, cw-24,
		[]int32{28, 9, 10, 13, 16, 12, 12},
		[]int32{250, 94, 102, 133, 166, 124, 153})

	refreshEconomics812Display()
	// Repaint only the section's parent surface. WS_CLIPCHILDREN preserves fields.
	if currentPage == ID_ECON {
		// Invalidate only section 6 work surfaces; the protected corporate
		// header and sidebar need no full-window erase on a page-local layout.
		dirty := RECT{Left: x - 2, Top: py(cardY) - 5, Right: x + cw + 2, Bottom: h}
		pInvalidateRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&dirty)), 1)
	}
}
