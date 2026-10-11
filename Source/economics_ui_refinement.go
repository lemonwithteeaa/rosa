//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

// Presentation-only additions to section 6. No calculations, saves, exports,
// navigation, decision links or database access may be introduced here.
const (
	econRefMetricFirstID = 14980
	econRefMetricCount   = 4
)

var econRefMetricControls [econRefMetricCount]syscall.Handle
var econRefMetricSnapshot = [4][2]string{
	{"CAPEX · РАЗОВО", "—"},
	{"OPEX · В ГОД", "—"},
	{"ОТСРОЧКА РЕМОНТА", "—"},
	{"ЧИСТЫЙ ЭФФЕКТ", "—"},
}

func buildEconomicsMetricCards() {
	for i := range econRefMetricControls {
		id := econRefMetricFirstID + i
		handle := create("STATIC", "", WS_CHILD|SS_OWNERDRAW, 0, 0, 100, 56, hMain, id)
		econRefMetricControls[i] = handle
		addPageControl(ID_ECON, handle)
	}
}

// The values are ONLY the selected, existing scenario values. Zero/unknown
// economics never turns into a fabricated benefit or a "successful" status.
func economicsMetricStrings() [4][2]string {
	// Titles must never depend on a previously selected scheme.
	rows := [4][2]string{
		{"CAPEX · РАЗОВО", "—"},
		{"OPEX · В ГОД", "—"},
		{"ОТСРОЧКА РЕМОНТА", "—"},
		{"ЧИСТЫЙ ЭФФЕКТ", "—"},
	}
	if strings.TrimSpace(selectedObjectKey) == "" {
		return rows
	}
	scenario, ok := activeAlternativeScenario(selectedObjectKey)
	if !ok {
		return rows
	}
	for _, candidate := range alternativeScenariosForObject(selectedObjectKey) {
		if strings.EqualFold(candidate.ID, scenario.ID) {
			scenario = candidate
			break
		}
	}
	// A stored cost is shown when explicitly supplied. The estimated benefit is
	// only shown for a current calculation, never for a stale or empty scenario.
	if scenario.OneTimeCAPEXRub != 0 || economicsScenarioCalculated(scenario) {
		rows[0][1] = econMetricRubThousands(scenario.OneTimeCAPEXRub)
	}
	if scenario.AnnualOPEXRub != 0 || economicsScenarioCalculated(scenario) {
		rows[1][1] = econMetricRubThousands(scenario.AnnualOPEXRub)
	}
	current, e := parseEconomicNumber(getText(econCurrentMRP), "Текущий МРП", false)
	if e == nil && current > 0 && scenario.ForecastBaseDays > current {
		rows[2][1] = fmt.Sprintf("%.1f мес.", economicsDeferralMonths(current, scenario.ForecastBaseDays))
		rows[2][1] = strings.Replace(rows[2][1], ".", ",", 1)
	}
	if economicsScenarioCalculated(scenario) {
		rows[3][1] = fmt.Sprintf("%.0f тыс. руб.", scenario.NetBenefitK)
	}
	return rows
}

func econMetricRubThousands(value float64) string {
	if value == 0 {
		return "0 тыс. руб."
	}
	if value > -1000000 && value < 1000000 {
		if value/1000 == float64(int64(value/1000)) {
			return fmt.Sprintf("%.0f тыс. руб.", value/1000)
		}
		return strings.Replace(fmt.Sprintf("%.1f тыс. руб.", value/1000), ".", ",", 1)
	}
	return strings.Replace(fmt.Sprintf("%.2f млн руб.", value/1e6), ".", ",", 1)
}

var econMetricSubtitles = [4]string{
	"Единовременные затраты", "Эксплуатационные расходы", "По прогнозу наработки", "При актуальном расчёте",
}

func economicsMetricText(hdc syscall.Handle, value string, rect RECT, font syscall.Handle, color uintptr) {
	pSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	pSetTextColor.Call(uintptr(hdc), color)
	if font != 0 {
		old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(font))
		defer pSelectObject.Call(uintptr(hdc), old)
	}
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(p(value))), ^uintptr(0),
		uintptr(unsafe.Pointer(&rect)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000 /* DT_END_ELLIPSIS */)
}

// Hook from WM_DRAWITEM. Returning true means the item was fully painted.
func paintEconomicsMetricCard(di *DRAWITEMSTRUCT) bool {
	if di == nil || di.CtlID < econRefMetricFirstID || di.CtlID >= econRefMetricFirstID+econRefMetricCount {
		return false
	}
	index := int(di.CtlID - econRefMetricFirstID)
	bounds := di.RcItem
	color := rgb(216, 225, 234)
	if index == 2 && econRefMetricSnapshot[index][1] != "—" {
		color = rgb(128, 171, 149)
	}
	if index == 3 {
		color = rgb(216, 225, 234)
	}
	econ814RoundedRect(di.HDC, bounds, rgb(255, 255, 255), color, 12)
	accent := bounds
	accent.Right = accent.Left + 4
	accent.Top += 1
	accent.Bottom -= 1
	stroke := brushNav
	if index == 2 && econRefMetricSnapshot[index][1] != "—" {
		stroke = econ814PositiveBrush()
	}
	if index == 3 {
		stroke = brushRedPressed
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(stroke))

	contents := econRefMetricSnapshot
	labelRect := bounds
	labelRect.Left += 12
	labelRect.Right -= 9
	labelRect.Top += 5
	labelRect.Bottom = labelRect.Top + 18
	valueRect := bounds
	valueRect.Left += 12
	valueRect.Right -= 9
	valueRect.Top += 24
	valueRect.Bottom = bounds.Bottom - 19
	if bounds.Bottom-bounds.Top < 65 {
		valueRect.Bottom = bounds.Bottom - 5
	}
	economicsMetricText(di.HDC, contents[index][0], labelRect, rosaUIFont(rosaUICaption), rgb(96, 114, 134))
	color = rgb(15, 48, 82)
	if contents[index][1] == "—" {
		color = rgb(122, 132, 141)
	}
	economicsMetricText(di.HDC, contents[index][1], valueRect, rosaUIFont(rosaUIMetric), color)
	hintRect := bounds
	hintRect.Left += 12
	hintRect.Right -= 9
	hintRect.Top = bounds.Bottom - 19
	hintRect.Bottom -= 3
	if bounds.Bottom-bounds.Top >= 65 && hintRect.Bottom > hintRect.Top+12 {
		hint := econMetricSubtitles[index]
		if contents[index][1] == "—" {
			hint = "Нет актуального расчёта"
			if bounds.Right-bounds.Left < 250 {
				hint = "Не рассчитано"
			}
		}
		economicsMetricText(di.HDC, hint, hintRect, rosaUIFont(rosaUICaption), rgb(102, 119, 135))
	}
	return true
}

// Four cards form the main visual hierarchy directly under object/context.
// No optional placement, no dependence on screen width beyond proportional width.
func layoutEconomicsMetricCards(x, y, width, height int32) (scenarioWidth int32) {
	gap := int32(10)
	cardW := max32(150, (width-3*gap)/4)
	for i, control := range econRefMetricControls {
		setEconomicsReferenceVisible(control, true)
		left := x + int32(i)*(cardW+gap)
		w := cardW
		if i == 3 {
			w = x + width - left
		}
		setPos(control, left, py(y), w, height)
	}
	return width
}

// Hook in WM_NOTIFY, only for the two economics lists. It neither rebuilds
// a list nor changes the selected row, avoiding re-entrant UI refreshes.
func economicsVisualCustomDraw(list syscall.Handle, lParam uintptr) uintptr {
	if list == 0 || lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		row := int(draw.NMCD.ItemSpec)
		// Do not override OS high contrast / selected-row highlighting.
		if draw.NMCD.ItemState&0x0001 != 0 {
			return CDRF_DODEFAULT
		}
		draw.ClrText = uint32(rgb(15, 48, 82))
		draw.ClrTextBk = uint32(rgb(255, 255, 255))
		if row%2 == 1 {
			draw.ClrTextBk = uint32(rgb(246, 248, 251))
		}
		if list == econSchemesList && draw.SubItem == 0 {
			draw.ClrText = uint32(rgb(190, 24, 49))
		}
		if list == econCompareList && row >= 0 && row < len(economicsFundRows) {
			g := economicsFundRows[row]
			// Only confirmed, up-to-date economics may receive an effect accent.
			// An uncalculated/obsolete benefit remains a neutral dash.
			if draw.SubItem == 4 && g.HasBenefit {
				if g.BestBenefit < 0 {
					draw.ClrText = uint32(rgb(176, 53, 33))
				} else if g.BestBenefit > 0 {
					draw.ClrText = uint32(rgb(27, 95, 71))
				}
			}
			if draw.SubItem == 6 && g.Calculated < len(g.Schemes) {
				draw.ClrText = uint32(rgb(176, 53, 33))
			}
		}
		return CDRF_NEWFONT
	}
	return CDRF_DODEFAULT
}

// Run only after existing calculation/selection routines have finished.
// The owner-drawn cards read model values at paint time and never write them.
func refreshEconomicsMetricCards() {
	econRefMetricSnapshot = economicsMetricStrings()
	for _, h := range econRefMetricControls {
		if h != 0 {
			pInvalidateRect.Call(uintptr(h), 0, 0)
		}
	}
}
