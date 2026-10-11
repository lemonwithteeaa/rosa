//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

type laboratoryInfluence struct {
	Name                  string
	Level                 int
	BaseFilled, BaseTotal int
	State                 string
}

type laboratoryOverviewStats struct {
	Directions                      int
	BaseFilled, BaseTotal           int
	NoData, Low, Moderate, Elevated int
}

func summarizeLaboratoryInfluence(items []laboratoryInfluence) laboratoryOverviewStats {
	var result laboratoryOverviewStats
	result.Directions = len(items)
	for _, item := range items {
		result.BaseFilled += item.BaseFilled
		result.BaseTotal += item.BaseTotal
		switch item.Level {
		case 1:
			result.Low++
		case 2:
			result.Moderate++
		case 3:
			result.Elevated++
		default:
			result.NoData++
		}
	}
	return result
}

func laboratoryCoveragePercent(filled, total int) int {
	if total <= 0 || filled <= 0 {
		return 0
	}
	if filled >= total {
		return 100
	}
	return (filled*100 + total/2) / total
}

func laboratoryInfluenceChartColumns(width int32) int32 {
	if width >= 760 {
		return 2
	}
	return 1
}

func laboratoryInfluenceChartHeight(width int32, itemCount int) int32 {
	columns := laboratoryInfluenceChartColumns(width)
	rows := (int32(itemCount) + columns - 1) / columns
	rowH := laboratoryInfluenceRowHeight(columns, itemCount)
	return 8 + rows*rowH + 22
}

func laboratoryInfluenceStatusWidth(columnW int32) int32 {
	return min32(198, max32(170, columnW*30/100))
}

func laboratoryInfluenceLabelWidth(columnW, columns int32) int32 {
	// RC168: in the two-column overview the wrapped captions
	// «Механические / примеси» and «Газы и / гидраты» still need a wider real
	// text column. Reserve more width for the caption and give a little less to
	// the status/coverage area, rather than clipping the user-visible direction name.
	if columns <= 1 {
		return min32(230, max32(205, columnW*30/100))
	}
	return min32(240, max32(210, columnW*46/100))
}

func laboratoryInfluenceStatusWidthForLayout(columnW, columns int32) int32 {
	if columns <= 1 {
		return min32(190, max32(150, columnW*24/100))
	}
	return min32(138, max32(118, columnW*24/100))
}

func laboratoryFieldMarkerColor(code string) uintptr {
	switch code {
	case "Б":
		return rgb(15, 48, 82)
	case "Д":
		return rgb(96, 104, 112)
	case "Б/О":
		return rgb(31, 93, 145)
	case "Д/О":
		return rgb(176, 117, 28)
	case "Т", "Т/О":
		return rgb(62, 107, 104)
	case "Р":
		return rgb(107, 76, 142)
	default:
		return rgb(96, 104, 112)
	}
}

func laboratoryFieldMarkerCaption(code string) string {
	switch code {
	case "Б":
		return "базовый показатель"
	case "Д":
		return "дополнительный показатель"
	case "Б/О":
		return "базовый показатель; повторяется в других разделах"
	case "Д/О":
		return "дополнительный показатель; повторяется в других разделах"
	case "Т":
		return "показатель из техрежима"
	case "Т/О":
		return "показатель из техрежима; повторяется в других разделах"
	case "Р":
		return "расчётный показатель; заполняется приложением"
	default:
		return "тип показателя не определён"
	}
}

func laboratoryLegendCaptionWidth(caption string) int32 {
	// Conservative fallbacks for tests and non-GDI callers. The actual legend
	// measures the active Windows font at draw time so DPI scaling cannot clip it.
	width := map[string]int32{
		"базовый": 96, "дополнительный": 142,
		"базовый, повторяется": 190, "доп., повторяется": 176,
		"из техрежима": 126, "расчётный": 108,
		"резкое изменение > 30 %": 196,
	}[caption]
	if width == 0 {
		return 126
	}
	return width
}

func laboratoryLegendMeasuredCaptionWidth(hdc syscall.Handle, font syscall.Handle, caption string) int32 {
	if hdc == 0 || font == 0 {
		return laboratoryLegendCaptionWidth(caption)
	}
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(font))
	measure := RECT{Left: 0, Top: 0, Right: 1200, Bottom: 40}
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&measure)), DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(uintptr(hdc), old)
	}
	return max32(48, measure.Right-measure.Left+8)
}

func laboratoryLegendRowWidth(hdc syscall.Handle, font syscall.Handle, items []struct{ code, caption string }) int32 {
	var width int32
	for _, item := range items {
		badgeW := int32(34)
		if strings.Contains(item.code, "/") {
			badgeW = 44
		}
		width += badgeW + 5 + laboratoryLegendMeasuredCaptionWidth(hdc, font, item.caption) + 10
	}
	return width
}

func drawLaboratoryFieldBadge(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	// Per-field marker is deliberately color-only. The meaning of each color
	// is explained once in the legend above, so the input matrix stays compact
	// and does not repeat Б/Д/Б/О/Д/О next to every parameter.
	code := getText(di.HwndItem)
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	marker := di.RcItem
	size := marker.Bottom - marker.Top - 6
	if size > 16 {
		size = 16
	}
	if size < 8 {
		size = 8
	}
	marker.Left += 2
	marker.Top += (marker.Bottom - marker.Top - size) / 2
	marker.Right = marker.Left + size
	marker.Bottom = marker.Top + size
	brush, _, _ := pCreateSolidBrush.Call(laboratoryFieldMarkerColor(code))
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&marker)), brush)
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&marker)), uintptr(brush))
	pDeleteObject.Call(brush)
}

func drawLaboratoryLegend(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushLight))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	items := []struct{ code, caption string }{
		{"Б", "базовый"}, {"Д", "дополнительный"}, {"Б/О", "базовый, повторяется"}, {"Д/О", "доп., повторяется"}, {"Т", "из техрежима"}, {"Р", "расчётный"},
		{"О", "резкое изменение > 30 %"},
	}
	available := di.RcItem.Right - di.RcItem.Left
	font := hFont
	if font == 0 {
		font = hFontHeaderSmall
	}
	rows := [][]struct{ code, caption string }{items}
	if laboratoryLegendRowWidth(di.HDC, font, items) > available {
		// A stable 3/4 split keeps both rows visually balanced and, unlike the old
		// one-line strip, preserves the complete captions at 125–200% DPI.
		rows = [][]struct{ code, caption string }{items[:3], items[3:]}
		if hFontHeaderSmall != 0 {
			font = hFontHeaderSmall
		}
		if laboratoryLegendRowWidth(di.HDC, font, rows[1]) > available && hFontHeaderTiny != 0 {
			font = hFontHeaderTiny
		}
	}
	rowH := max32(20, (di.RcItem.Bottom-di.RcItem.Top)/int32(len(rows)))
	for rowIndex, row := range rows {
		x := di.RcItem.Left
		top := di.RcItem.Top + int32(rowIndex)*rowH
		bottom := top + rowH
		if rowIndex == len(rows)-1 {
			bottom = di.RcItem.Bottom
		}
		for _, item := range row {
			badgeW := int32(34)
			if strings.Contains(item.code, "/") {
				badgeW = 44
			}
			badge := RECT{Left: x, Top: top + 2, Right: x + badgeW, Bottom: bottom - 2}
			brush, _, _ := pCreateSolidBrush.Call(laboratoryFieldMarkerColor(item.code))
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&badge)), brush)
			pSetTextColor.Call(uintptr(di.HDC), rgb(255, 255, 255))
			old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(item.code))), ^uintptr(0), uintptr(unsafe.Pointer(&badge)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			pDeleteObject.Call(brush)
			captionW := laboratoryLegendMeasuredCaptionWidth(di.HDC, font, item.caption)
			caption := RECT{Left: badge.Right + 5, Top: top, Right: badge.Right + 5 + captionW, Bottom: bottom}
			pSetTextColor.Call(uintptr(di.HDC), rgb(43, 46, 50))
			old, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(font))
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(item.caption))), ^uintptr(0), uintptr(unsafe.Pointer(&caption)), DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			x = caption.Right + 10
		}
	}
}

func laboratoryInfluenceDisplayName(name string) string {
	switch strings.TrimSpace(name) {
	case "Механические примеси":
		return "Механические\nпримеси"
	case "Газы и гидраты":
		return "Газы и\nгидраты"
	default:
		return name
	}
}

func laboratoryInfluenceRowHeight(columns int32, itemCount int) int32 {
	// RC167: the user explicitly requires two visible lines for
	// «Механические примеси» and «Газы и гидраты». In the two-column summary
	// layout with six directions we therefore reserve a real two-line envelope.
	// The footer is already optional, so the row itself should not be squeezed
	// below a readable two-line height.
	if columns <= 2 && itemCount >= 5 {
		return 42
	}
	return 34
}

func laboratoryInfluenceData(pass LabPassRecord) []laboratoryInfluence {
	result := make([]laboratoryInfluence, 0, 6)
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		values := canonicalizeLaboratoryValues(group, pass.Groups[group.Code], pass.Units[group.Code])
		assessment := laboratoryGroupSummary(group, values, pass.ScaleModel)
		item := laboratoryInfluence{Name: group.Name, BaseFilled: assessment.BaseFilled, BaseTotal: assessment.BaseTotal, Level: assessment.Level, State: assessment.State}
		result = append(result, item)
	}
	return result
}

func laboratoryInfluenceDrawFlags(extra uintptr) uintptr {
	// Multi-line captions must never carry DT_SINGLELINE: Windows ignores explicit
	// line breaks in that mode and renders «Механическиепримеси» as one clipped line.
	if extra&DT_WORDBREAK != 0 {
		return DT_WORDBREAK | DT_NOPREFIX
	}
	return DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX | extra
}

func drawLaboratoryInfluenceChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	selectedObject := strings.TrimSpace(getText(labObjectEdit))
	// A single-object infographic must not hydrate the complete laboratory
	// history during paint. On large work databases that read blocked WM_PAINT,
	// leaving the dialog blank and making the application appear frozen.
	passes := repositoryLatestSavedLabPasses()
	if labOverviewScopeCode == laboratoryScopeObject {
		passes = repositoryLabPassesForObject(selectedObject)
	}
	var items []laboratoryInfluence
	objectCount := 0
	if labOverviewScopeCode == laboratoryScopeObject {
		active := ensureActiveLabPass()
		active.ObjectKey = selectedObject
		active.Groups = cloneLabGroups(labDrafts)
		active.Units = cloneLabGroups(labUnitDrafts)
		if pass, ok := laboratoryPersonalInfluencePass(selectedObject, passes, active); ok {
			items = laboratoryInfluenceData(pass)
			objectCount = 1
		} else {
			items = laboratoryInfluenceData(*active)
		}
	} else {
		items, objectCount = laboratoryAggregateInfluence(labOverviewScopeCode, selectedObject, passes)
	}
	stats := summarizeLaboratoryInfluence(items)
	draw := func(text string, rect RECT, color uintptr, bold bool, flags uintptr) {
		pSetTextColor.Call(uintptr(di.HDC), color)
		font := hFont
		if bold {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		value := p(text)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(value)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), laboratoryInfluenceDrawFlags(flags))
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	left := di.RcItem.Left + 12
	right := di.RcItem.Right - 12
	chartW := right - left
	columns := laboratoryInfluenceChartColumns(chartW)
	// The approved single-object layout (RC502-LAB-LAYOUT) uses one readable
	// column. Keep both long direction names on two complete lines.
	if di.HwndItem == labInfographicChart {
		columns = 1
	}
	columnGap := int32(12)
	columnW := (chartW - (columns-1)*columnGap) / columns
	rowTop, rowH := di.RcItem.Top+8, laboratoryInfluenceRowHeight(columns, len(items))
	if di.HwndItem == labInfographicChart && rowH < 44 {
		rowH = 44
	}
	for index, item := range items {
		column := int32(index) % columns
		row := int32(index) / columns
		tileLeft := left + column*(columnW+columnGap)
		tileRight := tileLeft + columnW
		top := rowTop + row*rowH
		if top+rowH > di.RcItem.Bottom-2 {
			break
		}
		if row%2 == 1 {
			rowRect := RECT{Left: tileLeft - 3, Top: top, Right: tileRight + 3, Bottom: top + rowH}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rowRect)), uintptr(brushLight))
		}
		labelW := laboratoryInfluenceLabelWidth(columnW, columns)
		statusW := laboratoryInfluenceStatusWidthForLayout(columnW, columns)
		draw(laboratoryInfluenceDisplayName(item.Name), RECT{Left: tileLeft, Top: top, Right: tileLeft + labelW - 6, Bottom: top + rowH}, rgb(15, 48, 82), true, DT_WORDBREAK)
		color := rgb(170, 177, 185)
		if item.Level == 1 {
			color = rgb(69, 130, 92)
		}
		if item.Level == 2 {
			color = rgb(225, 151, 39)
		}
		if item.Level == 3 {
			color = rgb(203, 27, 43)
		}
		state := item.State
		if state == "" || state == "нет данных" {
			state = "НЕТ ДАННЫХ"
		}
		brush, _, _ := pCreateSolidBrush.Call(color)
		badge := RECT{Left: tileLeft + labelW, Top: top + 6, Right: tileLeft + labelW + statusW - 4, Bottom: top + 25}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&badge)), brush)
		draw(state, badge, rgb(255, 255, 255), true, DT_CENTER)

		coverageLeft := tileLeft + labelW + statusW
		coverageText := fmt.Sprintf("База %d/%d · %d%%", item.BaseFilled, item.BaseTotal, laboratoryCoveragePercent(item.BaseFilled, item.BaseTotal))
		coverageLabelRight := min32(coverageLeft+122, tileRight)
		draw(coverageText, RECT{Left: coverageLeft, Top: top + 1, Right: coverageLabelRight, Bottom: top + rowH - 1}, rgb(43, 46, 50), true, 0)
		if item.BaseTotal > 0 && tileRight-coverageLabelRight >= 52 {
			coverageTrack := RECT{Left: coverageLabelRight + 6, Top: top + 11, Right: tileRight, Bottom: top + 20}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&coverageTrack)), uintptr(brushBorder))
			coverageFill := coverageTrack
			coverageFill.Right = coverageFill.Left + (coverageTrack.Right-coverageTrack.Left)*int32(item.BaseFilled)/int32(item.BaseTotal)
			coverageBrush, _, _ := pCreateSolidBrush.Call(rgb(31, 93, 145))
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&coverageFill)), coverageBrush)
			pDeleteObject.Call(coverageBrush)
		}
		pDeleteObject.Call(brush)
	}
	rows := (int32(len(items)) + columns - 1) / columns
	footerTop := rowTop + rows*rowH
	if di.HwndItem == labInfographicChart && labInfographicMetricCombo != 0 {
		metric := laboratoryTrendMetricByIndex(laboratoryComboSelection(labInfographicMetricCombo))
		valueText := "значение не заполнено"
		active := ensureActiveLabPass()
		active.ObjectKey = selectedObject
		active.Groups = cloneLabGroups(labDrafts)
		active.Units = cloneLabGroups(labUnitDrafts)
		if pass, ok := laboratoryPersonalInfluencePass(selectedObject, passes, active); ok {
			if value, found := laboratoryTrendValue(pass, metric); found {
				valueText = fmt.Sprintf("%.4g %s", value, metric.Unit)
			}
		}
		if footerTop+27 <= di.RcItem.Bottom {
			draw("ВЫБРАННЫЙ ПОКАЗАТЕЛЬ: "+metric.Label+" — "+valueText, RECT{Left: left, Top: footerTop, Right: right, Bottom: footerTop + 27}, rgb(15, 48, 82), true, DT_SINGLELINE)
			footerTop += 27
		}
	}
	if footerTop+48 <= di.RcItem.Bottom {
		overallCoverage := fmt.Sprintf("%s · анализов: %d · базовые данные: %d/%d (%d%%). Нет оценки ≠ низкое.", laboratoryScopeCaption(labOverviewScopeCode, selectedObject, passes), objectCount, stats.BaseFilled, stats.BaseTotal, laboratoryCoveragePercent(stats.BaseFilled, stats.BaseTotal))
		draw(overallCoverage, RECT{Left: left, Top: footerTop, Right: right, Bottom: footerTop + 48}, rgb(96, 104, 112), false, DT_WORDBREAK)
	}
}

func drawLaboratoryPagerTitle(di *DRAWITEMSTRUCT) {
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushLight))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), rgb(15, 48, 82))
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
	text := p(getText(labFieldsPageLabel))
	rect := di.RcItem
	rect.Left += 8
	rect.Right -= 8
	measure := RECT{Left: rect.Left, Right: rect.Right}
	flags := uintptr(DT_CENTER | DT_WORDBREAK | DT_NOPREFIX)
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&measure)), flags|DT_CALCRECT)
	height := measure.Bottom - measure.Top
	rect.Top += max32(0, (rect.Bottom-rect.Top-height)/2)
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), flags)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
}
