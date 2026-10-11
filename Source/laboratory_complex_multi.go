//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unsafe"
)

var complexSeriesColors = [][3]uint8{{32, 98, 170}, {25, 139, 103}, {150, 83, 174}, {205, 117, 25}, {41, 143, 153}, {115, 88, 58}, {98, 112, 128}, {75, 114, 33}}

var (
	complexChartGroupsCache       [][]LabPassRecord
	complexChartGroupsCacheMetric = -1
	complexChartGroupsCacheValid  bool
)

func invalidateComplexChartGroups() {
	complexChartGroupsCache = nil
	complexChartGroupsCacheMetric = -1
	complexChartGroupsCacheValid = false
}

func complexLegendLayout(objectCount int) (columns, rows int) {
	if objectCount <= 0 {
		return 1, 0
	}
	columns = 1
	if objectCount > 4 {
		columns = 2
	}
	rows = (objectCount + columns - 1) / columns
	return columns, rows
}

func complexLegendViewport(objectCount int, chartHeight int32) (columns, totalRows, visibleRows int) {
	columns, totalRows = complexLegendLayout(objectCount)
	visibleRows = int((chartHeight - 325) / 24)
	if visibleRows < 2 {
		visibleRows = 2
	}
	if visibleRows > totalRows {
		visibleRows = totalRows
	}
	return
}

func pointSegmentDistanceSquared(px, py, x1, y1, x2, y2 float64) float64 {
	dx, dy := x2-x1, y2-y1
	if dx == 0 && dy == 0 {
		return (px-x1)*(px-x1) + (py-y1)*(py-y1)
	}
	t := ((px-x1)*dx + (py-y1)*dy) / (dx*dx + dy*dy)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	x, y := x1+t*dx, y1+t*dy
	return (px-x)*(px-x) + (py-y)*(py-y)
}

func complexSeriesHitTest(groups [][]LabPassRecord, metric complexMetric, rc RECT, x, y int32) int {
	if len(groups) == 0 {
		return -1
	}
	series := make([][]complexValue, len(groups))
	allDates := map[string]bool{}
	minV, maxV, hasValue := 0.0, 0.0, false
	for i, group := range groups {
		series[i] = complexMetricValues(group, metric)
		for _, value := range series[i] {
			allDates[value.date] = true
			if !hasValue {
				minV, maxV, hasValue = value.value, value.value, true
			}
			minV = math.Min(minV, value.value)
			maxV = math.Max(maxV, value.value)
		}
	}
	if !hasValue {
		return -1
	}
	if threshold, ok := laboratoryMetricThreshold(metric.group, metric.key); ok {
		minV = math.Min(minV, threshold.value)
		maxV = math.Max(maxV, threshold.value)
	}
	if maxV-minV < 1e-9 {
		minV, maxV = minV-1, maxV+1
	} else {
		pad := (maxV - minV) * .12
		minV, maxV = minV-pad, maxV+pad
	}
	if complexZoom > 1 {
		center := (minV + maxV) / 2
		half := (maxV - minV) / (2 * complexZoom)
		minV, maxV = center-half, center+half
	}
	dates := make([]string, 0, len(allDates))
	for date := range allDates {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool { return parseDate(dates[i]).Before(parseDate(dates[j])) })
	dateIndex := map[string]int{}
	for i, date := range dates {
		dateIndex[date] = i
	}
	_, totalRows := complexLegendLayout(len(groups))
	_, _, visibleRows := complexLegendViewport(len(groups), rc.Bottom-rc.Top)
	if visibleRows < totalRows {
		totalRows = visibleRows
	}
	plot, ok := complexChartPlotRect(rc, totalRows)
	if !ok {
		return -1
	}
	xFor := func(date string) float64 {
		if len(dates) < 2 {
			return float64(plot.Left + (plot.Right-plot.Left)/2)
		}
		return float64(plot.Left) + float64(dateIndex[date])*float64(plot.Right-plot.Left)/float64(len(dates)-1)
	}
	yFor := func(value float64) float64 {
		ratio := (value - minV) / (maxV - minV)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
		return float64(plot.Bottom) - ratio*float64(plot.Bottom-plot.Top)
	}
	bestIndex, bestDistance := -1, math.MaxFloat64
	for i, values := range series {
		for j, value := range values {
			x2, y2 := xFor(value.date), yFor(value.value)
			distance := pointSegmentDistanceSquared(float64(x), float64(y), x2, y2, x2, y2)
			if j > 0 {
				previous := values[j-1]
				distance = pointSegmentDistanceSquared(float64(x), float64(y), xFor(previous.date), yFor(previous.value), x2, y2)
			}
			if distance < bestDistance {
				bestDistance, bestIndex = distance, i
			}
		}
	}
	if bestDistance > 18*18 {
		return -1
	}
	return bestIndex
}

func complexChartPlotRect(rc RECT, legendRows int) (RECT, bool) {
	width := rc.Right - rc.Left
	height := rc.Bottom - rc.Top
	if width < 300 || height < 170 {
		return RECT{}, false
	}
	leftMargin := int32(210)
	if width < 900 {
		leftMargin = 150
	}
	topOffset := int32(96)
	if height < 300 {
		topOffset = 72
	}
	plot := RECT{
		Left:   rc.Left + leftMargin,
		Top:    rc.Top + topOffset,
		Right:  rc.Right - 48,
		Bottom: rc.Bottom - int32(108+legendRows*24),
	}
	// Never suppress a valid graph solely because the legend reserve is large.
	// Layout normally grows the chart control to fit all rows; this clamp also
	// protects maximized/high-DPI windows while they are between WM_SIZE passes.
	minimumBottom := plot.Top + 82
	maximumBottom := rc.Bottom - 34
	if plot.Bottom < minimumBottom {
		plot.Bottom = minimumBottom
	}
	if plot.Bottom > maximumBottom {
		plot.Bottom = maximumBottom
	}
	return plot, plot.Right-plot.Left >= 180 && plot.Bottom-plot.Top >= 65
}

func complexPassGroups(passes []LabPassRecord) [][]LabPassRecord {
	groups := map[string][]LabPassRecord{}
	representatives := map[string]string{}
	var order []string
	for _, pass := range passes {
		key := complexAnalysisGroupKey(pass.ObjectKey)
		if key == "" {
			continue
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
			representatives[key] = pass.ObjectKey
		}
		groups[key] = append(groups[key], pass)
	}
	sort.SliceStable(order, func(i, j int) bool {
		ai, aj := complexObjectNumber(representatives[order[i]]), complexObjectNumber(representatives[order[j]])
		if ai == 0 || aj == 0 {
			return order[i] < order[j]
		}
		return ai < aj
	})
	out := make([][]LabPassRecord, 0, len(order))
	for _, key := range order {
		out = append(out, groups[key])
	}
	return out
}

func complexLineGroups(groups [][]LabPassRecord, metric complexMetric) [][]LabPassRecord {
	result := make([][]LabPassRecord, 0, len(groups))
	for _, group := range groups {
		if len(complexMetricValues(group, metric)) >= 2 {
			result = append(result, group)
		}
	}
	return result
}

func complexChartGroups() [][]LabPassRecord {
	metricIndex := laboratoryComboSelection(complexMetricCombo)
	if complexChartGroupsCacheValid && complexChartGroupsCacheMetric == metricIndex {
		return complexChartGroupsCache
	}
	complexChartGroupsCache = complexLineGroups(complexPassGroups(complexSelected), complexSelectedMetric())
	complexChartGroupsCacheMetric = metricIndex
	complexChartGroupsCacheValid = true
	return complexChartGroupsCache
}

// Teaching examples keep separate persistent keys so none of the 100 files can
// overwrite another. For trend analysis they must still form one time series
// when they describe the same physical field/well/object.
func complexAnalysisGroupKey(objectKey string) string {
	physicalKey := strings.TrimSpace(objectKey)
	if original := teachingWorkflowOriginalObject(physicalKey); original != "" {
		physicalKey = original
	}
	return normalizeLaboratoryObjectKey(physicalKey)
}

func drawComplexMultiInfographic(di *DRAWITEMSTRUCT, groups [][]LabPassRecord, metric complexMetric) {
	rc := di.RcItem
	brush, _, _ := pCreateSolidBrush.Call(rgb(248, 250, 253))
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rc)), brush)
	pDeleteObject.Call(brush)
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	draw := func(s string, r RECT, bold bool, color uintptr) {
		font := hFont
		if bold {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		pSetTextColor.Call(uintptr(di.HDC), color)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	drawSmall := func(s string, r RECT, color uintptr) {
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontHeaderSmall))
		pSetTextColor.Call(uintptr(di.HDC), color)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_CENTER|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	draw("СРАВНЕНИЕ ПО ДАТАМ · "+metric.label, RECT{Left: rc.Left + 16, Top: rc.Top + 5, Right: rc.Right - 12, Bottom: rc.Top + 32}, true, rgb(22, 51, 82))
	threshold, hasThreshold := laboratoryMetricThreshold(metric.group, metric.key)
	series := make([][]complexValue, len(groups))
	allDates := map[string]bool{}
	minV, maxV := 0.0, 0.0
	hasValue := false
	for i, g := range groups {
		series[i] = complexMetricValues(g, metric)
		for _, v := range series[i] {
			allDates[v.date] = true
			if !hasValue {
				minV, maxV = v.value, v.value
				hasValue = true
			}
			if v.value < minV {
				minV = v.value
			}
			if v.value > maxV {
				maxV = v.value
			}
		}
	}
	if !hasValue {
		draw("По выбранному показателю числовых данных нет.", RECT{Left: rc.Left + 16, Top: rc.Top + 42, Right: rc.Right - 12, Bottom: rc.Top + 70}, false, rgb(22, 51, 82))
		return
	}
	dates := make([]string, 0, len(allDates))
	for date := range allDates {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool { return parseDate(dates[i]).Before(parseDate(dates[j])) })
	dateIndex := map[string]int{}
	for i, date := range dates {
		dateIndex[date] = i
	}
	if hasThreshold {
		if threshold.value < minV {
			minV = threshold.value
		}
		if threshold.value > maxV {
			maxV = threshold.value
		}
	}
	if maxV-minV < 1e-9 {
		minV -= 1
		maxV += 1
	} else {
		pad := (maxV - minV) * .12
		minV -= pad
		maxV += pad
	}
	// Масштаб по вертикали: увеличение сужает диапазон вокруг центра данных,
	// чтобы малые различия между объектами были видны; уменьшение возвращает
	// полный диапазон с запасом.
	if complexZoom > 1 && maxV > minV {
		center := (minV + maxV) / 2
		half := (maxV - minV) / (2 * complexZoom)
		minV, maxV = center-half, center+half
	}
	legendColumns, legendRows, visibleLegendRows := complexLegendViewport(len(groups), rc.Bottom-rc.Top)
	if visibleLegendRows <= 0 {
		visibleLegendRows = legendRows
	}
	plot, plotFits := complexChartPlotRect(rc, visibleLegendRows)
	if !plotFits {
		draw("Увеличьте окно, чтобы разместить график и легенду.", RECT{Left: rc.Left + 16, Top: rc.Top + 42, Right: rc.Right - 12, Bottom: rc.Top + 70}, false, rgb(22, 51, 82))
		return
	}
	xFor := func(date string) int32 {
		if len(dates) < 2 {
			return plot.Left + (plot.Right-plot.Left)/2
		}
		return plot.Left + int32(dateIndex[date])*(plot.Right-plot.Left)/int32(len(dates)-1)
	}
	yFor := func(value float64) int32 {
		ratio := (value - minV) / (maxV - minV)
		if ratio < 0 {
			ratio = 0
		} else if ratio > 1 {
			ratio = 1
		}
		return plot.Bottom - int32(ratio*float64(plot.Bottom-plot.Top))
	}
	// Coordinate axes are permanent navigation elements, not a pale grid line.
	// Keep them dark and thick enough to remain visible at any DPI/zoom.
	axis, _, _ := pCreatePenStiff.Call(0, 2, rgb(15, 48, 82))
	previous, _, _ := pSelectObject.Call(uintptr(di.HDC), axis)
	pts := []POINT{{X: plot.Left, Y: plot.Top}, {X: plot.Left, Y: plot.Bottom}, {X: plot.Right, Y: plot.Bottom}}
	pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&pts[0])), uintptr(len(pts)))
	pSelectObject.Call(uintptr(di.HDC), previous)
	pDeleteObject.Call(axis)
	draw(fmt.Sprintf("%s", metric.unit), RECT{Left: rc.Left + 6, Top: plot.Top - 24, Right: plot.Left - 3, Bottom: plot.Top - 2}, true, rgb(82, 94, 109))
	// The numerical scale is recalculated for every zoom level. Keeping these
	// labels next to the axis makes the effect of +/− explicit and prevents the
	// previous impression that only the lines moved while the axes stayed fixed.
	for tick := 0; tick <= 4; tick++ {
		fraction := float64(tick) / 4
		value := maxV - fraction*(maxV-minV)
		y := plot.Top + int32(fraction*float64(plot.Bottom-plot.Top))
		gridPen, _, _ := pCreatePenStiff.Call(0, 1, rgb(225, 232, 239))
		oldGrid, _, _ := pSelectObject.Call(uintptr(di.HDC), gridPen)
		grid := []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&grid[0])), 2)
		pSelectObject.Call(uintptr(di.HDC), oldGrid)
		pDeleteObject.Call(gridPen)
		drawSmall(fmt.Sprintf("%.4g", value), RECT{Left: rc.Left + 6, Top: y - 10, Right: plot.Left - 8, Bottom: y + 10}, rgb(82, 94, 109))
	}
	xAxisTitle := "Даты измерений →"
	if metric.key == "watercut" {
		xAxisTitle = "Дата записи техрежима →"
	}
	draw(xAxisTitle, RECT{Left: plot.Left, Top: plot.Bottom + 23, Right: plot.Right, Bottom: plot.Bottom + 47}, false, rgb(82, 94, 109))
	labelStep := 1
	if len(dates) > 6 {
		labelStep = (len(dates) + 5) / 6
	}
	for i, date := range dates {
		if i%labelStep != 0 && i != len(dates)-1 {
			continue
		}
		x := xFor(date)
		left := max32(plot.Left, min32(plot.Right-100, x-50))
		drawSmall(date, RECT{Left: left, Top: plot.Bottom + 3, Right: left + 100, Bottom: plot.Bottom + 24}, rgb(82, 94, 109))
	}
	if hasThreshold {
		y := yFor(threshold.value)
		pen, _, _ := pCreatePenStiff.Call(2, 1, rgb(196, 53, 61)) // PS_DOT: threshold only
		prev, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
		line := []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}
		pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), 2)
		pSelectObject.Call(uintptr(di.HDC), prev)
		pDeleteObject.Call(pen)
		draw(fmt.Sprintf("%.3g %s", threshold.value, metric.unit), RECT{Left: rc.Left + 4, Top: y - 13, Right: plot.Left - 3, Bottom: y + 13}, true, rgb(196, 53, 61))
	}
	placed := make([]RECT, 0, len(dates)*len(groups))
	for i, g := range groups {
		c := complexSeriesColors[i%len(complexSeriesColors)]
		color := rgb(c[0], c[1], c[2])
		values := series[i]
		if len(values) > 0 {
			pen, _, _ := pCreatePenStiff.Call(0, 3, color)
			prev, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
			line := make([]POINT, len(values))
			for j, v := range values {
				line[j] = POINT{X: xFor(v.date), Y: yFor(v.value)}
			}
			if len(line) > 1 {
				pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), uintptr(len(line)))
			}
			pSelectObject.Call(uintptr(di.HDC), prev)
			pDeleteObject.Call(pen)
			for j, v := range values {
				x, y := xFor(v.date), yFor(v.value)
				dotColor := color
				if hasThreshold && threshold.exceeded(v.value) {
					dotColor = rgb(196, 53, 61)
				}
				// Резкий скачок относительно предыдущей точки — отдельный сигнал
				// внимания, даже если абсолютный уровень ещё ниже порога.
				if j > 0 {
					prevValue := values[j-1].value
					base := prevValue
					if base < 0 {
						base = -base
					}
					if base < 1e-9 {
						base = 1
					}
					if (v.value-prevValue)/base > 0.30 || (prevValue-v.value)/base > 0.30 {
						dotColor = rgb(220, 120, 24)
					}
				}
				dot := RECT{Left: x - 4, Top: y - 4, Right: x + 5, Bottom: y + 5}
				b, _, _ := pCreateSolidBrush.Call(dotColor)
				pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&dot)), b)
				pDeleteObject.Call(b)
				for _, offset := range []int32{-22, 9, -38, 25, -54, 41} {
					left := max32(plot.Left, min32(plot.Right-48, x-24))
					candidate := RECT{Left: left, Top: y + offset, Right: left + 48, Bottom: y + offset + 16}
					if candidate.Top < plot.Top || candidate.Bottom > plot.Bottom {
						continue
					}
					conflict := false
					for _, used := range placed {
						if candidate.Left < used.Right+3 && candidate.Right+3 > used.Left && candidate.Top < used.Bottom+2 && candidate.Bottom+2 > used.Top {
							conflict = true
							break
						}
					}
					if conflict {
						continue
					}
					drawSmall(fmt.Sprintf("%.3g", v.value), candidate, color)
					placed = append(placed, candidate)
					break
				}
			}
		}
		{
			legendRow := i / legendColumns
			if legendRow < complexLegendScrollRow || legendRow >= complexLegendScrollRow+visibleLegendRows {
				continue
			}
			legendColumn := i % legendColumns
			gap := int32(18)
			available := rc.Right - rc.Left - 32 - int32(legendColumns-1)*gap
			columnWidth := available / int32(legendColumns)
			left := rc.Left + 16 + int32(legendColumn)*(columnWidth+gap)
			top := plot.Bottom + 100 + int32(legendRow-complexLegendScrollRow)*24
			name := strings.TrimSpace(readableObjectKey(g[0].ObjectKey))
			status := ""
			if len(values) == 0 {
				status = " · нет данных по показателю"
			} else if len(values) == 1 {
				status = " · одна дата"
				if metric.key == "watercut" {
					foundInPass := false
					for _, pass := range g {
						if _, ok := parseLabNumber(pass.SharedValues["watercut"]); ok {
							foundInPass = true
						}
						for _, fields := range pass.Groups {
							if _, ok := parseLabNumber(fields["watercut"]); ok {
								foundInPass = true
							}
						}
					}
					if !foundInPass {
						status = " · значение из текущего техрежима"
						if values[0].date == "дата не указана" {
							status += ", дата не указана"
						}
					}
				}
			}
			draw(fmt.Sprintf("Объект №%d — %s%s", complexObjectNumber(g[0].ObjectKey), name, status), RECT{Left: left, Top: top, Right: left + columnWidth, Bottom: top + 22}, false, color)
		}
	}
	if complexHighlightedGroup >= 0 && complexHighlightedGroup < len(groups) {
		group := groups[complexHighlightedGroup]
		if len(group) > 0 {
			label := fmt.Sprintf("Объект №%d — %s", complexObjectNumber(group[0].ObjectKey), readableObjectKey(group[0].ObjectKey))
			left := max32(rc.Left+8, min32(rc.Right-530, complexHighlightX+12))
			top := max32(rc.Top+34, complexHighlightY-42)
			card := RECT{Left: left, Top: top, Right: left + 520, Bottom: top + 32}
			fill, _, _ := pCreateSolidBrush.Call(rgb(235, 244, 252))
			border, _, _ := pCreateSolidBrush.Call(rgb(31, 93, 145))
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&card)), fill)
			pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&card)), border)
			pDeleteObject.Call(fill)
			pDeleteObject.Call(border)
			draw(label, RECT{Left: card.Left + 8, Top: card.Top, Right: card.Right - 8, Bottom: card.Bottom}, true, rgb(15, 48, 82))
		}
	}
	attentionNote := "Рыжая точка — изменение более 30% к предыдущему замеру"
	if complexTrendEnabled {
		trendType := laboratoryComboSelection(complexTrendTypeCombo)
		trendNames := []string{"линейная", "логарифмическая", "экспоненциальная", "полиномиальная (2)", "скользящее среднее"}
		name := "линейная"
		if trendType >= 0 && trendType < len(trendNames) {
			name = trendNames[trendType]
		}
		attentionNote += " · Тренд: " + name + ", исходные точки не изменяются"
		if trendType == 0 {
			for i, values := range series {
				if complexTrendObjectKey != "" && complexAnalysisGroupKey(groups[i][0].ObjectKey) != complexTrendObjectKey {
					continue
				}
				if len(values) < 2 {
					continue
				}
				var sx, sy, sxx, sxy float64
				for _, v := range values {
					x := float64(dateIndex[v.date])
					sx += x
					sy += v.value
					sxx += x * x
					sxy += x * v.value
				}
				den := float64(len(values))*sxx - sx*sx
				if den == 0 {
					continue
				}
				m := (float64(len(values))*sxy - sx*sy) / den
				b := (sy - m*sx) / float64(len(values))
				// A trend is a separate analytical layer: a thick corporate-red line
				// with a white halo remains visible over every source series.
				halo, _, _ := pCreatePenStiff.Call(0, 8, rgb(255, 255, 255))
				prev, _, _ := pSelectObject.Call(uintptr(di.HDC), halo)
				line := []POINT{{X: plot.Left, Y: yFor(b)}, {X: plot.Right, Y: yFor(b + m*float64(len(dates)-1))}}
				pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), 2)
				pSelectObject.Call(uintptr(di.HDC), prev)
				pDeleteObject.Call(halo)
				pen, _, _ := pCreatePenStiff.Call(0, 5, rgb(203, 27, 43))
				prev, _, _ = pSelectObject.Call(uintptr(di.HDC), pen)
				pPolylineStiff.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&line[0])), 2)
				pSelectObject.Call(uintptr(di.HDC), prev)
				pDeleteObject.Call(pen)
				draw("ТРЕНД", RECT{Left: max32(plot.Left, plot.Right-92), Top: line[1].Y - 25, Right: plot.Right, Bottom: line[1].Y - 3}, true, rgb(203, 27, 43))
			}
		}
	}
	thresholdCaption := "Для этого показателя единый числовой порог не задан; оцените сочетание условий."
	thresholdColor := rgb(82, 94, 109)
	thresholdBold := false
	if hasThreshold {
		thresholdCaption = "Порог внимания: " + threshold.caption(metric.unit)
		thresholdColor = rgb(196, 53, 61)
		thresholdBold = true
	} else if metric.key == "watercut" {
		thresholdCaption = "Техрежим: указана дата загрузки записи, если она известна; это не дата лабораторного замера."
	}
	draw(thresholdCaption, RECT{Left: plot.Left + 6, Top: plot.Bottom + 50, Right: plot.Right, Bottom: plot.Bottom + 73}, thresholdBold, thresholdColor)
	draw(attentionNote, RECT{Left: plot.Left + 6, Top: plot.Bottom + 74, Right: plot.Right, Bottom: plot.Bottom + 97}, false, rgb(176, 91, 18))
}
