//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

// Controls owned strictly by section 6, with no background overlays.
const (
	econ812IDParamHeader = 15030
	econ812IDBreakdown   = 15031
	econ812IDCardFirst   = 15050
	econ812IDDetails     = 15053
)

func buildEconomics812Controls() {
	econ812TableHeader = create("STATIC", "", WS_CHILD|SS_OWNERDRAW, 0, 0, 700, 25, hMain, econ812IDParamHeader)
	econ812Breakdown = create("STATIC", "", WS_CHILD|SS_OWNERDRAW, 0, 0, 1000, 48, hMain, econ812IDBreakdown)
	addPageControl(ID_ECON, econ812TableHeader)
	addPageControl(ID_ECON, econ812Breakdown)
	for i := 0; i < 3; i++ {
		econ812Cards[i] = button(ID_ECON, "", 0, 0, 300, 52, econ812IDCardFirst+i)
	}
	econ812Details = button(ID_ECON, "Подробнее ›", 0, 0, 137, 41, econ812IDDetails)
	econ813BreakTitle = label(ID_ECON, "ПРОЗРАЧНОСТЬ РАСЧЁТА", 0, 0, 280, 23)
	econ813ScenarioHint = label(ID_ECON, "Выберите схему · подробности по кнопке", 0, 0, 360, 24)
	for i := 0; i < 5; i++ {
		econ812Units[i] = label(ID_ECON, "", 0, 0, 70, 26)
		econ812States[i] = create("STATIC", "", WS_CHILD|SS_OWNERDRAW, 0, 0, 110, 26, hMain, econ813IDStatusFirst+i)
		addPageControl(ID_ECON, econ812States[i])
		econ812Rules[i] = create("STATIC", "", WS_CHILD|0x10 /*SS_ETCHEDHORZ*/, 0, 0, 350, 1, hMain, 0)
		addPageControl(ID_ECON, econ812Rules[i])
		if hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(econ812Units[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
			pSendMessageW.Call(uintptr(econ812States[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}
	}
}

func refreshEconomics812Display() {
	inputs := economicsInputHandles()
	for row, index := range []int{0, 5, 6, 7, 9} {
		if econ812States[row] == 0 || index >= len(inputs) {
			continue
		}
		value := strings.TrimSpace(getText(inputs[index]))
		status := "Нет данных"
		if value != "" {
			status = "Задано"
		}
		if index == 9 && (value == "0" || value == "0,0" || value == "0.0") {
			status = "Допущение"
		}
		setEconomicsCaption(econ812States[row], status)
	}
	if econ812Breakdown != 0 {
		pInvalidateRect.Call(uintptr(econ812Breakdown), 0, 0)
	}
}

// The bars display values only for a recomputed scenario that exactly matches
// the current persisted calculation; stale examples are never shown as a gain.
func economics812BreakdownValues() ([4]string, [4]float64, bool) {
	text := [4]string{"—", "—", "—", "—"}
	amounts := [4]float64{}
	if strings.TrimSpace(selectedObjectKey) == "" {
		return text, amounts, false
	}
	scenario, ok := activeAlternativeScenario(selectedObjectKey)
	if !ok || !economicsScenarioCalculated(scenario) {
		return text, amounts, false
	}
	inputs := economicsInputHandles()
	take := func(i int) (float64, bool) {
		v, err := parseEconomicNumber(getText(inputs[i]), "Параметр", false)
		return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
	}
	current, ok0 := take(0)
	horizon, ok1 := take(5)
	days, ok2 := take(6)
	repairCost, ok3 := take(7)
	inflation, ok4 := take(9)
	if !(ok0 && ok1 && ok2 && ok3 && ok4) || current <= 0 || horizon <= 0 || repairCost <= 0 || scenario.ForecastBaseDays <= 0 {
		return text, amounts, false
	}
	data := BaseEconomicsInput{
		CurrentMRPDays: current, MRPLowDays: scenario.ForecastLowDays,
		MRPBaseDays: scenario.ForecastBaseDays, MRPHighDays: scenario.ForecastHighDays,
		SelectedScenario: "Центральный", HorizonMonths: int(horizon),
		RepairDurationDays: days, RepairCostK: repairCost / 1000.0,
		AnnualOPEXK:      scenario.AnnualOPEXRub / 1000.0,
		OPEXInflationPct: inflation, OneTimeMTRK: scenario.OneTimeCAPEXRub / 1000.0,
	}
	r := CalculateBaseEconomics(data)
	if r.Status != "PASS" || math.Abs(r.Selected.NetBenefitForHorizonK-scenario.NetBenefitK) > 0.05 {
		return text, amounts, false
	}
	amounts = [4]float64{0, r.Selected.OPEXForHorizonK, r.Selected.OneTimeMTRK, r.Selected.NetBenefitForHorizonK}
	// GrossBenefitPerYearK must be converted to the same user-selected horizon.
	amounts[0] = r.Selected.GrossBenefitPerYearK * horizon / 12
	for i, v := range amounts {
		text[i] = fmt.Sprintf("%+.0f тыс. руб.", v)
		if i == 1 || i == 2 {
			text[i] = fmt.Sprintf("%.0f тыс. руб.", v)
		}
	}
	return text, amounts, true
}

func paintEconomics812Bands(di *DRAWITEMSTRUCT) bool {
	if di == nil {
		return false
	}
	if di.CtlID != econ812IDParamHeader && di.CtlID != econ812IDBreakdown {
		return false
	}
	bounds := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(brushWhite))
	if di.CtlID == econ812IDParamHeader {
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(brushLight))
	}
	if di.CtlID == econ812IDParamHeader {
		labels := [4]string{"ПАРАМЕТР", "ЗНАЧЕНИЕ", "ЕД.", "СТАТУС"}
		left := int32(8)
		for i, label := range labels {
			r := RECT{Left: left, Top: 1, Right: left + econ812Columns[i] - 4, Bottom: bounds.Bottom - 1}
			economicsMetricText(di.HDC, label, r, rosaUIFont(rosaUICaption), rgb(32, 64, 95))
			left += econ812Columns[i]
		}
		return true
	}
	// Transparency panel is self-contained and never overlaps another HWND.
	// The old 37px strip had room for values OR bars, not both.
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(brushWhite))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(brushBorder))
	heading := RECT{Left: 14, Top: 3, Right: bounds.Right - 14, Bottom: 26}
	economicsMetricText(di.HDC, "Прозрачность расчёта", heading, rosaUIFont(rosaUICaption), rgb(27, 58, 88))
	// At compact-height desktops keep a legible summary instead of reducing
	// the four breakdown captions to an unreadable microfont. Detailed figures
	// remain available through the existing formulas action.
	if bounds.Bottom-bounds.Top < 60 {
		note := "Нет актуального расчёта · состав эффекта откроется после пересчёта"
		if economics814HasCurrentResult() {
			note = "Состав эффекта — в разделе «Формулы и допущения»"
		}
		r := RECT{Left: 14, Top: 28, Right: bounds.Right - 14, Bottom: bounds.Bottom - 3}
		economicsMetricText(di.HDC, note, r, rosaUIFont(rosaUICaption), rgb(91, 109, 127))
		return true
	}
	if !economics814HasCurrentResult() {
		note := RECT{Left: bounds.Right - 310, Top: 3, Right: bounds.Right - 14, Bottom: 26}
		economicsMetricText(di.HDC, "Нет актуального расчёта", note, rosaUIFont(rosaUICaption), rgb(149, 92, 66))
	}
	separator := RECT{Left: 1, Top: 27, Right: bounds.Right - 1, Bottom: 28}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&separator)), uintptr(brushBorder))
	headers := [4]string{"ПРЕДОТВРАЩЁННЫЕ РЕМОНТЫ", "OPEX ЗА ПЕРИОД", "РАЗОВЫЕ ЗАТРАТЫ", "ЧИСТЫЙ РЕЗУЛЬТАТ"}
	if bounds.Right-bounds.Left < 1130 {
		headers = [4]string{"ЭКОНОМИЯ НА РЕМОНТАХ", "OPEX ЗА ПЕРИОД", "РАЗОВЫЕ ЗАТРАТЫ", "ЧИСТЫЙ РЕЗУЛЬТАТ"}
	}
	if bounds.Right-bounds.Left < 830 {
		headers = [4]string{"ЭКОНОМИЯ", "OPEX ЗА ПЕРИОД", "РАЗОВЫЕ ЗАТРАТЫ", "ЧИСТЫЙ ЭФФЕКТ"}
	}
	values, amounts, known := economics812BreakdownValues()
	gap := int32(8)
	bandW := (bounds.Right - bounds.Left - 30 - gap*3) / 4
	maxAmt := float64(1)
	if known {
		for _, v := range amounts {
			if math.Abs(v) > maxAmt {
				maxAmt = math.Abs(v)
			}
		}
	}
	for i := 0; i < 4; i++ {
		left := int32(15) + int32(i)*(bandW+gap)
		right := left + bandW
		labelTop, labelBottom, valueTop := int32(34), int32(53), int32(54)
		if bounds.Bottom-bounds.Top < 85 {
			labelTop, labelBottom, valueTop = 30, 46, 47
		}
		head := RECT{Left: left, Top: labelTop, Right: right, Bottom: labelBottom}
		economicsMetricText(di.HDC, headers[i], head, rosaUIFont(rosaUICaption), rgb(99, 117, 133))
		val := RECT{Left: left, Top: valueTop, Right: right, Bottom: bounds.Bottom - 10}
		col := rgb(30, 64, 95)
		if values[i] == "—" {
			col = rgb(130, 143, 154)
		}
		if i == 3 && known && amounts[i] < 0 {
			col = rgb(177, 45, 50)
		}
		economicsMetricText(di.HDC, values[i], val, hFontHeaderSmall, col)
		track := RECT{Left: left, Top: bounds.Bottom - 7, Right: right, Bottom: bounds.Bottom - 3}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&track)), uintptr(brushBorder))
		if known {
			track.Right = track.Left + int32(float64(right-left)*math.Abs(amounts[i])/maxAmt)
			bar := brushNav
			if i == 3 {
				bar = brushRedPressed
			}
			if track.Right > track.Left {
				pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&track)), uintptr(bar))
			}
		}
	}
	return true
}

// Full scheme details remain available even when only decision columns fit.
// Double-click does not modify the calculation or reassign an approved scheme.
func showEconomics812SchemeDetails(i int) {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return
	}
	schemes := alternativeScenariosForObject(selectedObjectKey)
	if i < 0 || i >= len(schemes) {
		return
	}
	s := schemes[i]
	methods := strings.Join(alternativeScenarioMethodsForCalculation(s), ", ")
	if methods == "" {
		methods = "Не выбраны"
	}
	benefit := "Не рассчитан"
	if economicsScenarioCalculated(s) {
		benefit = fmt.Sprintf("%.1f тыс. руб.", s.NetBenefitK)
	}
	message := fmt.Sprintf("Схема: %s\nТип: %s\nМетоды и мероприятия: %s\nДоступность: %s\nПрогноз МРП: %.0f сут.\nOPEX: %.0f руб./год\nCAPEX: %.0f руб.\nЧистый эффект: %s\nСтатус: %s\nОснование: %s",
		s.Name, dash(s.SelectionKind), methods, dash(s.Availability), s.ForecastBaseDays,
		s.AnnualOPEXRub, s.OneTimeCAPEXRub, benefit, econReferenceStatus(s), dash(s.SelectionBasis))
	msg("Экономика выбранной схемы", message, MB_OK)
}

// Three visible summary cards; the native 8-column comparison list is retained
// intact for data and selections, but not compressed into unreadable columns.
type econ812CardData struct {
	index                        int
	title, detail, metric, state string
	selected                     bool
}

var econ812CardSnapshot [3]econ812CardData

func refreshEconomics812ScenarioCards() {
	for i := range econ812CardSnapshot {
		econ812CardSnapshot[i] = econ812CardData{index: -1, title: "Нет варианта", detail: "Сформируйте мероприятия в разделе 4", metric: "—"}
	}
	if strings.TrimSpace(selectedObjectKey) != "" {
		scenarios := alternativeScenariosForObject(selectedObjectKey)
		active := activeAlternativeScenarioID(selectedObjectKey)
		activeIndex := 0
		for i, s := range scenarios {
			if strings.EqualFold(s.ID, active) {
				activeIndex = i
				break
			}
		}
		start := max(0, activeIndex-1)
		if len(scenarios) > 3 && start > len(scenarios)-3 {
			start = len(scenarios) - 3
		}
		for i := 0; i < 3 && start+i < len(scenarios); i++ {
			index := start + i
			s := scenarios[index]
			methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
			if methods == "" {
				methods = "Методы не выбраны"
			}
			metric := "МРП " + econReferenceMRP(s.ForecastBaseDays) + " сут."
			if economicsScenarioCalculated(s) {
				metric = fmt.Sprintf("%+.0f тыс. ₽", s.NetBenefitK)
			}
			name := strings.TrimSpace(s.Name)
			if name == "" {
				name = fmt.Sprintf("Схема %d", index+1)
			}
			state := "Не рассчитано"
			if economicsScenarioCalculated(s) {
				state = "Расчёт актуален"
			} else if strings.EqualFold(strings.TrimSpace(s.Status), "Исходные данные изменились") {
				state = "На пересчёт"
			}
			econ812CardSnapshot[i] = econ812CardData{index: index, title: name, detail: methods, metric: metric, state: state, selected: strings.EqualFold(s.ID, active)}
		}
	}
	for i, control := range econ812Cards {
		if control != 0 {
			setEconomicsReferenceVisible(control, econ812CardSnapshot[i].index >= 0)
			pInvalidateRect.Call(uintptr(control), 0, 0)
		}
	}
}
func paintEconomics812ScenarioCard(di *DRAWITEMSTRUCT) bool {
	if di == nil || di.CtlID < econ812IDCardFirst || di.CtlID >= econ812IDCardFirst+3 {
		return false
	}
	card := econ812CardSnapshot[int(di.CtlID-econ812IDCardFirst)]
	b := di.RcItem
	background, outline := rgb(255, 255, 255), rgb(215, 225, 235)
	if card.selected {
		background, outline = rgb(255, 247, 248), rgb(208, 108, 120)
	}
	econ814RoundedRect(di.HDC, b, background, outline, 11)
	if card.selected {
		line := RECT{Left: b.Left + 1, Top: b.Top + 5, Right: b.Left + 5, Bottom: b.Bottom - 5}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line)), uintptr(brushRedPressed))
	}
	label := RECT{Left: b.Left + 13, Top: b.Top + 2, Right: b.Right - 145, Bottom: b.Top + 24}
	economicsMetricText(di.HDC, card.title, label, hFontHeaderSmall, rgb(22, 54, 83))
	val := RECT{Left: b.Right - 141, Top: b.Top + 3, Right: b.Right - 12, Bottom: b.Top + 24}
	valueColor := rgb(78, 101, 124)
	if card.state == "Расчёт актуален" {
		valueColor = rgb(35, 110, 81)
		if strings.HasPrefix(card.metric, "-") {
			valueColor = rgb(172, 48, 53)
		}
	}
	economicsMetricText(di.HDC, card.metric, val, hFontHeaderSmall, valueColor)
	detail := RECT{Left: b.Left + 13, Top: b.Top + 23, Right: b.Right - 154, Bottom: b.Bottom - 3}
	economicsMetricText(di.HDC, card.detail, detail, rosaUIFont(rosaUICaption), rgb(102, 119, 133))
	badge := RECT{Left: b.Right - 151, Top: b.Top + 23, Right: b.Right - 11, Bottom: b.Bottom - 4}
	econ814Badge(di.HDC, badge, card.state)
	return true
}
func handleEconomics812Card(id int) bool {
	if id == econ812IDDetails {
		i := 0
		for _, card := range econ812CardSnapshot {
			if card.selected {
				i = card.index
				break
			}
		}
		showEconomics812SchemeDetails(i)
		return true
	}
	if id >= econ812IDCardFirst && id < econ812IDCardFirst+3 {
		i := econ812CardSnapshot[id-econ812IDCardFirst].index
		if i < 0 {
			return true
		}
		selectEconomicsSchemeFromTable(i)
		refreshEconomics812ScenarioCards()
		return true
	}
	return false
}
