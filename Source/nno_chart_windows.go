//go:build windows

package main

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

var (
	nnoChartEllipse                = gdi32.NewProc("Ellipse")
	nnoChartSaveDC                 = gdi32.NewProc("SaveDC")
	nnoChartRestoreDC              = gdi32.NewProc("RestoreDC")
	nnoChartIntersectClipRect      = gdi32.NewProc("IntersectClipRect")
	nnoDistributionSetWndProc      = user32.NewProc("SetWindowLongPtrW")
	nnoDistributionCallWndProc     = user32.NewProc("CallWindowProcW")
	nnoDistributionOldWndProc      uintptr
	nnoDistributionWndProcCallback uintptr
	nnoDistributionSelected        nnoDistributionMarkerSelection
	nnoDistributionHover           nnoDistributionMarkerSelection
	nnoRollingOldWndProc           uintptr
	nnoRollingWndProcCallback      uintptr
	nnoRollingSelected             nnoRollingSelection
	nnoRollingHover                nnoRollingSelection
)

type nnoRollingSelection struct {
	Group, Month int
	Median       bool
	Value        float64
	Valid        bool
}

// Independent visual scales; these values never change stored statistics.
func nnoCurrentZoom() float64 { return nnoZoomValue(nnoZoomActiveChart) }
func nnoZoomValue(chart int) float64 {
	if chart == 1 {
		return nnoFleetDistributionZoom
	}
	return nnoFleetRollingZoom
}
func nnoSetChartZoom(chart int, value float64) {
	if value < 0.5 {
		value = 0.5
	}
	if value > 3 {
		value = 3
	}
	if chart == 1 {
		nnoFleetDistributionZoom = value
	} else {
		nnoFleetRollingZoom = value
	}
	nnoRollingSelected.Valid = false
	nnoRollingHover.Valid = false
	nnoDistributionSelected.Valid = false
	nnoDistributionHover.Valid = false
}
func nnoZoomCaption() string {
	return fmt.Sprintf("%.0f%%", math.Round(nnoCurrentZoom()*100))
}
func nnoApplyChartZoom(scale float64, chart int) float64 {
	if scale <= 0 {
		scale = 250
	}
	zoom := nnoZoomValue(chart)
	if zoom <= 0 {
		zoom = 1
	}
	scaled := scale / zoom
	if scaled < 50 {
		return 50
	}
	return scaled
}
func nnoZoomRepaint() {
	if nnoFleetZoomReset != 0 {
		setText(nnoFleetZoomReset, nnoZoomCaption())
	}
	if nnoFleetChart != 0 {
		pInvalidateRect.Call(uintptr(nnoFleetChart), 0, 0)
	}
	if nnoFleetDistribution != 0 {
		pInvalidateRect.Call(uintptr(nnoFleetDistribution), 0, 0)
	}
}

func nnoChartPanelFill(hdc uintptr, rc RECT, fill uintptr) {
	brush, _, _ := pCreateSolidBrush.Call(fill)
	if brush != 0 {
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		pDeleteObject.Call(brush)
	}
}

// Always reserve a right-hand legend. Never move the legend over a plot,
// even when the application is narrowed or a user zooms into the axis.
func nnoRollingPlot(rc RECT) RECT {
	width := rc.Right - rc.Left
	legendW := int32(180)
	if width >= 970 {
		legendW = 225
	}
	if width < 660 {
		legendW = 150
	}
	if width < 460 {
		legendW = 128
	}
	bottomMargin := int32(30)
	if (nnoExpandedPainting || nnoExpandedPicking) && nnoExpandedKind == 0 {
		bottomMargin = 52
	}
	leftMargin := int32(49)
	if (nnoExpandedPainting || nnoExpandedPicking) && nnoExpandedKind == 0 {
		leftMargin = 150 // Dedicated space for the Y-axis title and tick labels.
	}
	return RECT{Left: rc.Left + leftMargin, Top: rc.Top + 30, Right: rc.Right - legendW - 14, Bottom: rc.Bottom - bottomMargin}
}

func nnoRollingScale() float64 {
	maxDays := 0.0
	for g := 0; g < nnoMethodCount; g++ {
		if nnoSelectedGroup() != "Все способы" && nnoSelectedGroup() != nnoMethodName(g) {
			continue
		}
		for _, s := range nnoFleetSeries[g] {
			if s.Defined {
				maxDays = math.Max(maxDays, s.Mean)
				maxDays = math.Max(maxDays, s.Median)
			}
		}
	}
	if nnoTarget.Visible && nnoTarget.ExpandScale {
		maxDays = math.Max(maxDays, nnoTarget.Days)
	}
	return nnoApplyChartZoom(nnoChartScale(maxDays), 0)
}

// A click anywhere along a drawn segment resolves to the nearest real
// monthly observation. Do not interpolate an invented production value.
func nnoPickRolling(rc RECT, x, y int32) nnoRollingSelection {
	plot, count := nnoRollingPlot(rc), len(nnoFleetMonthData)
	if count == 0 || plot.Right-plot.Left < 130 || plot.Bottom-plot.Top < 55 ||
		x < plot.Left-8 || x > plot.Right+8 || y < plot.Top-8 || y > plot.Bottom+8 {
		return nnoRollingSelection{}
	}
	scale := nnoRollingScale()
	best, bestDistance := nnoRollingSelection{}, float64(14*14)
	for g := 0; g < nnoMethodCount; g++ {
		if nnoSelectedGroup() != "Все способы" && nnoSelectedGroup() != nnoMethodName(g) {
			continue
		}
		for mode := 0; mode < 2; mode++ {
			if mode == 1 && !nnoFleetShowBoth {
				continue
			}
			median := (nnoFleetShowMedian && !nnoFleetShowBoth) || mode == 1
			var previous nnoRollingSelection
			var prevX, prevY int32
			for i, stats := range nnoFleetSeries[g] {
				if !stats.Defined {
					previous.Valid = false
					continue
				}
				value := stats.Mean
				if median {
					value = stats.Median
				}
				px := plot.Left + int32(i)*(plot.Right-plot.Left)/int32(maxIntChart(count-1, 1))
				py := plot.Bottom - int32(value/scale*float64(plot.Bottom-plot.Top))
				current := nnoRollingSelection{Group: g, Month: i, Median: median, Value: value, Valid: true}
				dx, dy := float64(x-px), float64(y-py)
				if d := dx*dx + dy*dy; d < bestDistance {
					bestDistance, best = d, current
				}
				if previous.Valid {
					segX, segY := float64(px-prevX), float64(py-prevY)
					length := segX*segX + segY*segY
					if length > 0 {
						t := math.Max(0, math.Min(1, (float64(x-prevX)*segX+float64(y-prevY)*segY)/length))
						deltaX := float64(x-prevX) - t*segX
						deltaY := float64(y-prevY) - t*segY
						if d := deltaX*deltaX + deltaY*deltaY; d < bestDistance {
							bestDistance = d
							best = previous
							if t >= 0.5 {
								best = current
							}
						}
					}
				}
				previous, prevX, prevY = current, px, py
			}
		}
	}
	return best
}

func nnoRollingWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case 0x020A:
		if int16(hiWord(wParam)) > 0 {
			nnoScrollBy(-120)
		} else {
			nnoScrollBy(120)
		}
		return 0
	case 0x0203: // WM_LBUTTONDBLCLK: the expanded view is an explicit action.
		nnoOpenExpandedChart(0)
		return 0
	case WM_LBUTTONDOWN, 0x0200:
		if nnoFleetShowEventTrend {
			break
		}
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		picked := nnoPickRolling(rc, int32(int16(loWord(lParam))), int32(int16(hiWord(lParam))))
		if message == 0x0200 {
			if !nnoRollingSelected.Valid && (nnoRollingHover.Valid != picked.Valid || (picked.Valid && (picked.Group != nnoRollingHover.Group || picked.Month != nnoRollingHover.Month || picked.Median != nnoRollingHover.Median))) {
				nnoRollingHover = picked
				pInvalidateRect.Call(hwnd, 0, 0)
			}
		} else {
			if nnoRollingSelected.Valid && picked.Valid && nnoRollingSelected.Group == picked.Group && nnoRollingSelected.Month == picked.Month && nnoRollingSelected.Median == picked.Median {
				picked.Valid = false
			}
			nnoRollingSelected = picked
			pSetFocus.Call(hwnd)
			pInvalidateRect.Call(hwnd, 0, 0)
			return 0
		}
	case 0x0100:
		if wParam == 0x1b {
			nnoRollingSelected.Valid = false
			pInvalidateRect.Call(hwnd, 0, 0)
			return 0
		}
	}
	if nnoRollingOldWndProc != 0 {
		result, _, _ := nnoDistributionCallWndProc.Call(nnoRollingOldWndProc, hwnd, uintptr(message), wParam, lParam)
		return result
	}
	result, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func initNNORollingInteraction() {
	if nnoFleetChart == 0 || nnoRollingOldWndProc != 0 {
		return
	}
	if nnoRollingWndProcCallback == 0 {
		nnoRollingWndProcCallback = syscall.NewCallback(nnoRollingWndProc)
	}
	old, _, _ := nnoDistributionSetWndProc.Call(uintptr(nnoFleetChart), ^uintptr(3), nnoRollingWndProcCallback)
	nnoRollingOldWndProc = old
}

type nnoDistributionMarkerSelection struct {
	Group int
	Kind  int // 0 = lower, 1 = central, 2 = upper
	Value float64
	Valid bool
}

const (
	nnoDistributionLower = iota
	nnoDistributionCentral
	nnoDistributionUpper
	nnoDistributionQ1
	nnoDistributionQ3
	nnoDistributionMedian
	nnoDistributionMean
)

// Shared by painting and hit-testing. The right legend is never a plotting
// surface and the y-axis and category labels stay inside their own margins.
func nnoDistributionPlot(rc RECT) RECT {
	width := rc.Right - rc.Left
	legendW := int32(260)
	if width >= 970 {
		legendW = 320
	}
	if width < 680 {
		legendW = 214
	}
	if width < 500 {
		legendW = 172
	}
	bottomMargin := int32(31)
	if (nnoExpandedPainting || nnoExpandedPicking) && nnoExpandedKind == 1 {
		bottomMargin = 51
	}
	leftMargin := int32(65)
	if (nnoExpandedPainting || nnoExpandedPicking) && nnoExpandedKind == 1 {
		leftMargin = 150 // Independent Y-axis title, kept outside the plotted data.
	}
	return RECT{Left: rc.Left + leftMargin, Top: rc.Top + 32, Right: rc.Right - legendW - 12, Bottom: rc.Bottom - bottomMargin}
}

func nnoDistributionGroupIndices(selected string) []int {
	groups := make([]int, 0, nnoMethodCount)
	for i := 0; i < nnoMethodCount; i++ {
		if selected == "Все способы" || selected == nnoMethodName(i) {
			groups = append(groups, i)
		}
	}
	return groups
}

func nnoDistributionMarkerValue(stat nnoCycleStats, kind int, median bool) float64 {
	switch kind {
	case nnoDistributionLower:
		return stat.Min
	case nnoDistributionUpper:
		return stat.Max
	case nnoDistributionQ1:
		return stat.Q1
	case nnoDistributionQ3:
		return stat.Q3
	case nnoDistributionMedian:
		return stat.Median
	case nnoDistributionMean:
		return stat.Mean
	default:
		if median {
			return stat.Median
		}
		return stat.Mean
	}
}

func nnoPickDistributionMarker(rc RECT, groups []int, scale float64, x, y int32, median bool, summaries [nnoMethodCount]nnoCycleStats) (nnoDistributionMarkerSelection, bool) {
	return nnoPickDistributionMarkerExtended(rc, groups, scale, x, y, median, summaries, false)
}

func nnoPickDistributionMarkerExtended(rc RECT, groups []int, scale float64, x, y int32, median bool, summaries [nnoMethodCount]nnoCycleStats, expanded bool) (nnoDistributionMarkerSelection, bool) {
	plot := nnoDistributionPlot(rc)
	if scale <= 0 || plot.Right-plot.Left < 100 || plot.Bottom-plot.Top < 40 || len(groups) == 0 {
		return nnoDistributionMarkerSelection{}, false
	}
	if x < plot.Left-4 || x > plot.Right+4 || y < plot.Top-4 || y > plot.Bottom+4 {
		return nnoDistributionMarkerSelection{}, false
	}
	cellW := float64(plot.Right-plot.Left) / float64(len(groups))
	index := int(float64(x-plot.Left) / cellW)
	if index < 0 || index >= len(groups) {
		return nnoDistributionMarkerSelection{}, false
	}
	group := groups[index]
	stat := summaries[group]
	if !stat.Defined {
		return nnoDistributionMarkerSelection{}, false
	}
	centerX := plot.Left + int32(2*index+1)*(plot.Right-plot.Left)/int32(2*len(groups))
	if math.Abs(float64(x-centerX)) > math.Max(20, cellW*.42) {
		return nnoDistributionMarkerSelection{}, false
	}
	best := nnoDistributionMarkerSelection{}
	bestDistance := math.MaxFloat64
	maxKind := nnoDistributionUpper
	if expanded {
		maxKind = nnoDistributionMean
	}
	for kind := nnoDistributionLower; kind <= maxKind; kind++ {
		value := nnoDistributionMarkerValue(stat, kind, median)
		markerY := plot.Bottom - int32(value/scale*float64(plot.Bottom-plot.Top))
		distance := math.Abs(float64(y - markerY))
		if distance < bestDistance {
			bestDistance = distance
			best = nnoDistributionMarkerSelection{Group: group, Kind: kind, Value: value, Valid: true}
		}
	}
	if bestDistance > 11 {
		return nnoDistributionMarkerSelection{}, false
	}
	return best, true
}

// Shared scale for painting and hit-testing; zoom never edits source data.
func nnoDistributionDisplayScale() float64 {
	maxDays := 0.0
	for _, g := range nnoDistributionGroupIndices(nnoSelectedGroup()) {
		if stat := nnoFleetCycleSummaries[g]; stat.Defined && stat.Max > maxDays {
			maxDays = stat.Max
		}
	}
	scale := nnoChartScale(maxDays)
	if nnoTarget.Visible && nnoTarget.ExpandScale {
		scale = nnoChartScale(math.Max(maxDays, nnoTarget.Days))
	}
	return nnoApplyChartZoom(scale, 1)
}

func nnoDistributionWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == WM_LBUTTONDOWN {
		nnoOpenExpandedChart(1)
		return 0
	}
	if message == 0x020A {
		if int16(hiWord(wParam)) > 0 {
			nnoScrollBy(-120)
		} else {
			nnoScrollBy(120)
		}
		return 0
	}
	if message == WM_LBUTTONDOWN || message == 0x0200 { // WM_MOUSEMOVE
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		groups := nnoDistributionGroupIndices(nnoSelectedGroup())
		maxDays := 0.0
		for _, group := range groups {
			if stat := nnoFleetCycleSummaries[group]; stat.Defined && stat.Max > maxDays {
				maxDays = stat.Max
			}
		}
		scale := nnoChartScale(maxDays)
		if nnoTarget.Visible && nnoTarget.ExpandScale {
			scale = nnoChartScale(math.Max(maxDays, nnoTarget.Days))
		}
		scale = nnoApplyChartZoom(scale, 1)
		picked, ok := nnoPickDistributionMarker(rc, groups, scale, int32(int16(loWord(lParam))), int32(int16(hiWord(lParam))), nnoFleetShowMedian, nnoFleetCycleSummaries)
		if message == 0x0200 {
			if !nnoDistributionSelected.Valid && (nnoDistributionHover.Valid != ok || (ok && (picked.Group != nnoDistributionHover.Group || picked.Kind != nnoDistributionHover.Kind))) {
				nnoDistributionHover = picked
				pInvalidateRect.Call(hwnd, 0, 0)
			}
		} else {
			if ok && nnoDistributionSelected.Valid && nnoDistributionSelected.Group == picked.Group && nnoDistributionSelected.Kind == picked.Kind {
				ok = false
			}
			if ok {
				nnoDistributionSelected = picked
				pSetFocus.Call(hwnd)
				pInvalidateRect.Call(hwnd, 0, 0)
				return 0
			}
			nnoDistributionSelected.Valid = false
			pInvalidateRect.Call(hwnd, 0, 0)
		}
	}
	if message == 0x0100 && wParam == 0x1b { // Esc
		nnoDistributionSelected.Valid = false
		pInvalidateRect.Call(hwnd, 0, 0)
		return 0
	}
	if nnoDistributionOldWndProc != 0 {
		result, _, _ := nnoDistributionCallWndProc.Call(nnoDistributionOldWndProc, hwnd, uintptr(message), wParam, lParam)
		return result
	}
	result, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

func initNNODistributionInteraction() {
	if nnoFleetDistribution == 0 || nnoDistributionOldWndProc != 0 {
		return
	}
	if nnoDistributionWndProcCallback == 0 {
		nnoDistributionWndProcCallback = syscall.NewCallback(nnoDistributionWndProc)
	}
	old, _, _ := nnoDistributionSetWndProc.Call(uintptr(nnoFleetDistribution), ^uintptr(3), nnoDistributionWndProcCallback)
	nnoDistributionOldWndProc = old
}

func nnoChartText(hdc uintptr, content string, bounds RECT, color uintptr, bold bool, centered bool) {
	font := hFontButtonCompact
	if bold {
		font = hFontHeaderSmall
	}
	if font == 0 {
		font = hFont
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	pSetTextColor.Call(hdc, color)
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	flags := uintptr(DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX | 0x8000)
	if centered {
		flags |= DT_CENTER
	}
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(content))), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), flags)
	if old != 0 {
		pSelectObject.Call(hdc, old)
	}
}
func nnoChartStroke(hdc uintptr, coords []POINT, color uintptr, width int32, dashed bool) {
	if len(coords) < 2 {
		return
	}
	style := uintptr(0)
	if dashed {
		style = 1
		width = 1
	}
	pen, _, _ := pCreatePenStiff.Call(style, uintptr(width), color)
	if pen == 0 {
		return
	}
	old, _, _ := pSelectObject.Call(hdc, pen)
	pPolylineStiff.Call(hdc, uintptr(unsafe.Pointer(&coords[0])), uintptr(len(coords)))
	pSelectObject.Call(hdc, old)
	pDeleteObject.Call(pen)
}
func nnoChartDashedStroke(hdc uintptr, from, to POINT, color uintptr) {
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	for start := 0.0; start < length; start += 13 {
		end := math.Min(start+8, length)
		a := POINT{X: from.X + int32(math.Round(dx*start/length)), Y: from.Y + int32(math.Round(dy*start/length))}
		b := POINT{X: from.X + int32(math.Round(dx*end/length)), Y: from.Y + int32(math.Round(dy*end/length))}
		nnoChartStroke(hdc, []POINT{a, b}, color, 2, false)
	}
}
func nnoChartDot(hdc uintptr, x, y int32, color uintptr, radius int32) {
	pen, _, _ := pCreatePenStiff.Call(0, 1, color)
	if pen == 0 {
		return
	}
	oldPen, _, _ := pSelectObject.Call(hdc, pen)
	oldBrush, _, _ := pSelectObject.Call(hdc, uintptr(brushWhite))
	nnoChartEllipse.Call(hdc, uintptr(x-radius), uintptr(y-radius), uintptr(x+radius+1), uintptr(y+radius+1))
	if oldBrush != 0 {
		pSelectObject.Call(hdc, oldBrush)
	}
	pSelectObject.Call(hdc, oldPen)
	pDeleteObject.Call(pen)
}
func nnoChartFrame(hdc uintptr, rc RECT) {
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brushWhite))
}
func nnoDrawTarget(hdc uintptr, plot RECT, scale float64) {
	if !nnoTarget.Visible || nnoTarget.Days <= 0 {
		return
	}
	red := rgb(185, 46, 61)
	if nnoTarget.Days > scale {
		nnoChartText(hdc, fmt.Sprintf("↑ Ориентир %.0f сут. выше шкалы", nnoTarget.Days), RECT{Left: plot.Left + 4, Top: plot.Top + 3, Right: plot.Right - 4, Bottom: plot.Top + 23}, red, true, false)
		return
	}
	y := plot.Bottom - int32(nnoTarget.Days/scale*float64(plot.Bottom-plot.Top))
	nnoChartDashedStroke(hdc, POINT{X: plot.Left, Y: y}, POINT{X: plot.Right, Y: y}, red)
	nnoChartText(hdc, fmt.Sprintf("Ориентир %.0f сут.", nnoTarget.Days), RECT{Left: plot.Left + 5, Top: y - 21, Right: plot.Right - 4, Bottom: y - 2}, red, true, false)
}
func nnoChartScale(max float64) float64 {
	if max <= 0 {
		return 250
	}
	step := 100.0
	if max >= 2500 {
		step = 500
	} else if max >= 1200 {
		step = 250
	} else if max < 500 {
		step = 50
	}
	return math.Ceil(max/step) * step
}

// Manual axis intervals are available in the enlarged chart only.
// They change the grid and label cadence, never the source observations.
func nnoDrawYTicks(hdc uintptr, rc, plot RECT, maxValue float64, chart int) {
	step := 0
	if nnoExpandedPainting && nnoExpandedKind == chart {
		step = nnoExpandedYStep[chart]
	}
	if step <= 0 {
		for tick := 0; tick <= 4; tick++ {
			y := plot.Bottom - int32(tick)*(plot.Bottom-plot.Top)/4
			nnoChartStroke(hdc, []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}, rgb(224, 231, 240), 1, false)
			nnoChartTextRight(hdc, fmt.Sprintf("%.0f", maxValue*float64(tick)/4), RECT{Left: rc.Left + 2, Top: y - 10, Right: plot.Left - 7, Bottom: y + 10}, rgb(64, 88, 118), false)
		}
		return
	}
	// Cap the operation count and skip text when tick labels would overlap.
	// Exact grid step remains controlled by the user.
	maxTick := int(math.Floor(maxValue / float64(step)))
	if maxTick > 150 {
		maxTick = 150
	}
	lastLabelY := int32(1 << 30)
	for i := 0; i <= maxTick; i++ {
		value := float64(i * step)
		y := plot.Bottom - int32(value/maxValue*float64(plot.Bottom-plot.Top))
		nnoChartStroke(hdc, []POINT{{X: plot.Left, Y: y}, {X: plot.Right, Y: y}}, rgb(224, 231, 240), 1, false)
		if i == 0 || lastLabelY-y >= 24 {
			nnoChartTextRight(hdc, fmt.Sprintf("%d", i*step), RECT{Left: rc.Left + 2, Top: y - 10, Right: plot.Left - 7, Bottom: y + 10}, rgb(64, 88, 118), false)
			lastLabelY = y
		}
	}
}

func nnoChartAxes(hdc uintptr, rc, plot RECT, maxValue float64, dates []string) {
	nnoDrawYTicks(hdc, rc, plot, maxValue, 0)
	if len(dates) == 0 {
		return
	}
	n := len(dates)
	step := 1
	for step < n && int32(step)*(plot.Right-plot.Left)/int32(maxIntChart(n-1, 1)) < 58 {
		step++
	}
	for i := 0; i < n; i += step {
		x := plot.Left + int32(i)*(plot.Right-plot.Left)/int32(maxIntChart(n-1, 1))
		label := RECT{Left: x - 32, Top: plot.Bottom + 4, Right: x + 32, Bottom: plot.Bottom + 24}
		if i == 0 {
			label.Left, label.Right = x, x+64
		}
		if i == n-1 {
			label.Left, label.Right = x-64, x
		}
		nnoChartText(hdc, dates[i], label, rgb(28, 62, 104), false, true)
	}
}
func maxIntChart(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func nnoChartShortMethod(g int) string {
	switch g {
	case 4:
		return "Фонт."
	case 5:
		return "Проч."
	default:
		return nnoMethodName(g)
	}
}

// Independent legend layer for the rolling and event charts. The reserved
// right-side panel always stays disjoint from plotting, axes and tooltip guides.
func nnoDrawFleetMethodLegend(hdc uintptr, rc, plot RECT, groups []int) {
	panel := RECT{Left: plot.Right + 11, Top: rc.Top + 24, Right: rc.Right - 7, Bottom: rc.Bottom - 9}
	if panel.Right-panel.Left < 109 || panel.Bottom-panel.Top < 78 {
		return
	}
	nnoChartPanelFill(hdc, panel, rgb(246, 249, 253))
	nnoChartStroke(hdc, []POINT{{X: panel.Left, Y: panel.Top}, {X: panel.Right, Y: panel.Top}, {X: panel.Right, Y: panel.Bottom}, {X: panel.Left, Y: panel.Bottom}, {X: panel.Left, Y: panel.Top}}, rgb(214, 225, 236), 1, false)
	nnoChartText(hdc, "СПОСОБЫ", RECT{Left: panel.Left + 5, Top: panel.Top + 5, Right: panel.Right - 5, Bottom: panel.Top + 26}, rgb(21, 54, 96), true, true)
	bodyTop, bodyBottom := panel.Top+27, panel.Bottom-6
	rowH := (bodyBottom - bodyTop) / int32(maxIntChart(len(groups), 1))
	for pos, g := range groups {
		top := bodyTop + int32(pos)*rowH
		mid := top + rowH/2
		clr := nnoMethodAccent(g)
		nnoChartStroke(hdc, []POINT{{X: panel.Left + 6, Y: mid}, {X: panel.Left + 20, Y: mid}}, clr, 2, false)
		nnoChartDot(hdc, panel.Left+13, mid, clr, 2)
		label := nnoMethodName(g)
		if panel.Right-panel.Left < 155 && g == 4 {
			label = "Фонтан"
		}
		nnoChartText(hdc, label, RECT{Left: panel.Left + 24, Top: top, Right: panel.Right - 4, Bottom: top + rowH}, clr, true, true)
	}
}

func nnoDrawRollingCycleChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	hdc := uintptr(di.HDC)
	rc := di.RcItem
	nnoChartFrame(hdc, rc)
	if len(nnoFleetMonthData) == 0 {
		nnoChartText(hdc, "Выберите период анализа", rc, rgb(63, 88, 115), true, true)
		return
	}
	plot := nnoRollingPlot(rc)
	if plot.Right-plot.Left < 130 || plot.Bottom-plot.Top < 55 {
		return
	}
	selected := nnoSelectedGroup()
	groups := make([]int, 0, nnoMethodCount)
	for i := 0; i < nnoMethodCount; i++ {
		if selected == "Все способы" || selected == nnoMethodName(i) {
			groups = append(groups, i)
		}
	}
	maxDays := 0.0
	found := false
	for _, g := range groups {
		for _, stats := range nnoFleetSeries[g] {
			if !stats.Defined {
				continue
			}
			found = true
			value := stats.Mean
			if nnoFleetShowMedian {
				value = stats.Median
			}
			if nnoFleetShowBoth {
				value = math.Max(stats.Mean, stats.Median)
			}
			if value > maxDays {
				maxDays = value
			}
		}
	}
	dates := make([]string, len(nnoFleetMonthData))
	for i, m := range nnoFleetMonthData {
		dates[i] = m.Month.Format("01.06")
	}
	maxScale := nnoRollingScale()
	nnoChartAxes(hdc, rc, plot, maxScale, dates)
	nnoDrawTarget(hdc, plot, maxScale)
	nnoDrawExpandedAxisCaptions(hdc, rc, plot, 0, false)
	metric := "СРЕДНЯЯ · сут."
	if nnoFleetShowMedian {
		metric = "МЕДИАНА · сут."
	}
	if nnoFleetShowBoth {
		metric = "СРЕДНЯЯ — · МЕДИАНА – – · сут."
	}
	nnoChartText(hdc, metric, RECT{Left: plot.Left, Top: rc.Top + 3, Right: plot.Right, Bottom: rc.Top + 23}, rgb(184, 36, 55), true, false)
	n := len(nnoFleetMonthData)
	plotY := func(value float64) int32 { return plot.Bottom - int32(value/maxScale*float64(plot.Bottom-plot.Top)) }
	clipState, _, _ := nnoChartSaveDC.Call(hdc)
	if clipState != 0 {
		nnoChartIntersectClipRect.Call(hdc, uintptr(plot.Left), uintptr(plot.Top), uintptr(plot.Right+1), uintptr(plot.Bottom+1))
	}
	for _, g := range groups {
		clr := nnoMethodAccent(g)
		series := nnoFleetSeries[g]
		for mode := 0; mode < 2; mode++ {
			if mode == 1 && !nnoFleetShowBoth {
				continue
			}
			var prior POINT
			priorExists := false
			for i, stats := range series {
				if !stats.Defined {
					priorExists = false
					continue
				}
				value := stats.Mean
				median := (nnoFleetShowMedian && !nnoFleetShowBoth) || mode == 1
				if median {
					value = stats.Median
				}
				pt := POINT{X: plot.Left + int32(i)*(plot.Right-plot.Left)/int32(maxIntChart(n-1, 1)), Y: plotY(value)}
				if priorExists {
					if median {
						nnoChartDashedStroke(hdc, prior, pt, clr)
					} else {
						nnoChartStroke(hdc, []POINT{prior, pt}, clr, 2, false)
					}
				}
				nnoChartDot(hdc, pt.X, pt.Y, clr, 3)
				prior, priorExists = pt, true
			}
		}
	}
	if clipState != 0 {
		nnoChartRestoreDC.Call(hdc, clipState)
	}
	nnoDrawFleetMethodLegend(hdc, rc, plot, groups)
	if !found {
		middleY := (plot.Top + plot.Bottom) / 2
		nnoChartText(hdc, "Нет подтверждённых циклов с датами начала и окончания", RECT{Left: plot.Left + 6, Top: middleY - 11, Right: plot.Right - 6, Bottom: middleY + 12}, rgb(119, 48, 57), true, true)
	}
	selection := nnoRollingHover
	if nnoRollingSelected.Valid {
		selection = nnoRollingSelected
	}
	if selection.Valid && selection.Month < len(nnoFleetMonthData) {
		px := plot.Left + int32(selection.Month)*(plot.Right-plot.Left)/int32(maxIntChart(n-1, 1))
		py := plotY(selection.Value)
		guide, color := rgb(112, 135, 159), nnoMethodAccent(selection.Group)
		nnoChartDashedStroke(hdc, POINT{X: plot.Left, Y: py}, POINT{X: px, Y: py}, guide)
		nnoChartDashedStroke(hdc, POINT{X: px, Y: py}, POINT{X: px, Y: plot.Bottom}, guide)
		nnoChartDot(hdc, px, py, color, 5)
		nnoChartText(hdc, fmt.Sprintf("%.1f", selection.Value), RECT{Left: rc.Left + 2, Top: py - 12, Right: plot.Left - 4, Bottom: py + 12}, color, true, false)
		nnoChartText(hdc, nnoFleetMonthData[selection.Month].Month.Format("01.2006"), RECT{Left: max32(plot.Left, px-36), Top: plot.Bottom + 5, Right: min32(plot.Right, px+36), Bottom: plot.Bottom + 25}, color, true, true)
		metric := "средняя"
		if selection.Median {
			metric = "медиана"
		}
		info := fmt.Sprintf("%s · %s %.1f сут.", nnoMethodName(selection.Group), metric, selection.Value)
		if nnoTarget.Visible {
			info += fmt.Sprintf(" · Δ %.1f", selection.Value-nnoTarget.Days)
		}
		cardX := max32(plot.Left+3, min32(px+8, plot.Right-215))
		cardY := max32(plot.Top+20, min32(py-28, plot.Bottom-25))
		nnoChartText(hdc, info, RECT{Left: cardX, Top: cardY, Right: min32(plot.Right, cardX+215), Bottom: cardY + 21}, color, true, false)
	}
}
func nnoDrawCycleDistribution(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	hdc := uintptr(di.HDC)
	rc := di.RcItem
	nnoChartFrame(hdc, rc)
	plot := nnoDistributionPlot(rc)
	if plot.Right-plot.Left < 135 || plot.Bottom-plot.Top < 55 {
		nnoChartText(hdc, "Увеличьте окно для просмотра графика", rc, rgb(63, 88, 115), false, true)
		return
	}
	groups := nnoDistributionGroupIndices(nnoSelectedGroup())
	maxDays := 0.0
	for _, i := range groups {
		stat := nnoFleetCycleSummaries[i]
		if stat.Defined && stat.Max > maxDays {
			maxDays = stat.Max
		}
	}
	scale := nnoChartScale(maxDays)
	if nnoTarget.Visible && nnoTarget.ExpandScale {
		scale = nnoChartScale(math.Max(maxDays, nnoTarget.Days))
	}
	scale = nnoApplyChartZoom(scale, 1)
	nnoDrawYTicks(hdc, rc, plot, scale, 1)
	nnoDrawTarget(hdc, plot, scale)
	metric := "РАСПРЕДЕЛЕНИЕ НАРАБОТКИ · СУТКИ"
	nnoChartText(hdc, metric, RECT{Left: plot.Left, Top: rc.Top + 3, Right: plot.Right, Bottom: rc.Top + 22}, rgb(21, 54, 96), true, false)
	nnoDrawDistributionLegend(hdc, rc, plot)
	nnoDrawExpandedAxisCaptions(hdc, rc, plot, 1, false)
	for _, g := range groups {
		if nnoFleetCycleSummaries[g].Count > 160 {
			nnoChartText(hdc, "Точки: до 160; статистика: все", RECT{Left: plot.Left + 3, Top: plot.Top + 21, Right: plot.Right - 2, Bottom: plot.Top + 39}, rgb(84, 103, 124), false, false)
			break
		}
	}
	hasStats := false
	// X labels are outside the clipped plotting surface.
	for j, g := range groups {
		xx := plot.Left + int32(2*j+1)*(plot.Right-plot.Left)/int32(maxIntChart(2*len(groups), 2))
		cellWidth := (plot.Right - plot.Left) / int32(maxIntChart(len(groups), 1))
		label := nnoChartShortMethod(g)
		if cellWidth >= 92 {
			label = nnoMethodName(g)
		}
		half := max32(16, min32(53, cellWidth/2-3))
		nnoChartText(hdc, label, RECT{Left: xx - half, Top: plot.Bottom + 4, Right: xx + half, Bottom: plot.Bottom + 24}, nnoMethodAccent(g), true, true)
	}
	clipState, _, _ := nnoChartSaveDC.Call(hdc)
	if clipState != 0 {
		nnoChartIntersectClipRect.Call(hdc, uintptr(plot.Left), uintptr(plot.Top), uintptr(plot.Right+1), uintptr(plot.Bottom+1))
	}
	y := func(v float64) int32 { return plot.Bottom - int32(v/scale*float64(plot.Bottom-plot.Top)) }
	for j, g := range groups {
		stat := nnoFleetCycleSummaries[g]
		xx := plot.Left + int32(2*j+1)*(plot.Right-plot.Left)/int32(maxIntChart(2*len(groups), 2))
		if !stat.Defined {
			continue
		}
		hasStats = true
		col := nnoMethodAccent(g)
		top := y(stat.Q3)
		bottom := y(stat.Q1)
		if bottom <= top {
			bottom = top + 2
		}
		half := int32(18)
		if len(groups) <= 2 {
			half = 24
		}
		nnoChartStroke(hdc, []POINT{{X: xx, Y: y(stat.WhiskerMax)}, {X: xx, Y: y(stat.WhiskerMin)}}, col, 1, false)
		nnoChartStroke(hdc, []POINT{{X: xx - half/2, Y: y(stat.WhiskerMax)}, {X: xx + half/2, Y: y(stat.WhiskerMax)}}, col, 1, false)
		nnoChartStroke(hdc, []POINT{{X: xx - half/2, Y: y(stat.WhiskerMin)}, {X: xx + half/2, Y: y(stat.WhiskerMin)}}, col, 1, false)
		box := RECT{Left: xx - half, Top: top, Right: xx + half, Bottom: bottom}
		light, _, _ := pCreateSolidBrush.Call(rgb(244, 247, 251))
		if light != 0 {
			pFillRect.Call(hdc, uintptr(unsafe.Pointer(&box)), light)
			pDeleteObject.Call(light)
		}
		nnoChartStroke(hdc, []POINT{{X: xx - half, Y: top}, {X: xx + half, Y: top}, {X: xx + half, Y: bottom}, {X: xx - half, Y: bottom}, {X: xx - half, Y: top}}, col, 2, false)
		nnoChartStroke(hdc, []POINT{{X: xx - half, Y: y(stat.Median)}, {X: xx + half, Y: y(stat.Median)}}, col, 3, false)
		my := y(stat.Mean)
		nnoChartStroke(hdc, []POINT{{X: xx, Y: my - 5}, {X: xx + 5, Y: my}, {X: xx, Y: my + 5}, {X: xx - 5, Y: my}, {X: xx, Y: my - 5}}, rgb(22, 48, 77), 2, false)
		// Individual observations are drawn from the same verified array as the
		// table and quartiles. Bound the number of GDI operations on large funds.
		shown, drawn, step := 0, 0, maxIntChart(stat.Count/120, 1)
		for _, sample := range nnoFleetCycleSamples {
			if sample.Group != nnoMethodName(g) || !nnoSampleInWindow(sample, nnoFleetDateFrom, nnoFleetDateTo) {
				continue
			}
			shown++
			outlier := sample.Days < stat.Q1-1.5*(stat.Q3-stat.Q1) || sample.Days > stat.Q3+1.5*(stat.Q3-stat.Q1)
			if drawn >= 160 || (shown%step != 0 && !outlier) {
				continue
			}
			drawn++
			jitter := int32((shown*17)%15 - 7)
			if outlier {
				nnoChartDot(hdc, xx+jitter, y(sample.Days), col, 2)
			} else {
				nnoChartDot(hdc, xx+jitter, y(sample.Days), rgb(160, 177, 194), 1)
			}
		}
		selection := nnoDistributionHover
		if nnoDistributionSelected.Valid {
			selection = nnoDistributionSelected
		}
		if selection.Valid && selection.Group == g {
			selectedY := y(selection.Value)
			guide := rgb(112, 135, 159)
			nnoChartDashedStroke(hdc, POINT{X: plot.Left, Y: selectedY}, POINT{X: xx, Y: selectedY}, guide)
			nnoChartDashedStroke(hdc, POINT{X: xx, Y: selectedY}, POINT{X: xx, Y: plot.Bottom}, guide)
			nnoChartDot(hdc, xx, selectedY, col, 5)
		}
	}
	if clipState != 0 {
		nnoChartRestoreDC.Call(hdc, clipState)
	}
	// Exact selected values and cross-hair labels belong OUTSIDE the plot clip.
	selection := nnoDistributionHover
	if nnoDistributionSelected.Valid {
		selection = nnoDistributionSelected
	}
	if selection.Valid && selection.Value >= 0 && selection.Value <= scale {
		selectedY := plot.Bottom - int32(selection.Value/scale*float64(plot.Bottom-plot.Top))
		nnoChartTextRight(hdc, fmt.Sprintf("%.1f", selection.Value), RECT{Left: rc.Left + 2, Top: selectedY - 12, Right: plot.Left - 7, Bottom: selectedY + 12}, nnoMethodAccent(selection.Group), true)
		info := fmt.Sprintf("%s · %s: %.1f сут.", nnoMethodName(selection.Group), nnoDistributionSelectionLabel(selection.Kind, nnoFleetShowMedian), selection.Value)
		if nnoTarget.Visible {
			info += fmt.Sprintf(" · Δ %+.1f сут.", selection.Value-nnoTarget.Days)
		}
		cardWidth := min32(310, plot.Right-plot.Left-12)
		if cardWidth >= 145 {
			card := RECT{Left: plot.Left + 9, Top: plot.Top + 10, Right: plot.Left + 9 + cardWidth, Bottom: plot.Top + 38}
			nnoChartPanelFill(hdc, card, rgb(248, 250, 253))
			nnoChartStroke(hdc, []POINT{{X: card.Left, Y: card.Top}, {X: card.Right, Y: card.Top}, {X: card.Right, Y: card.Bottom}, {X: card.Left, Y: card.Bottom}, {X: card.Left, Y: card.Top}}, rgb(214, 225, 236), 1, false)
			nnoChartText(hdc, info, RECT{Left: card.Left + 6, Top: card.Top + 3, Right: card.Right - 4, Bottom: card.Bottom - 3}, rgb(28, 62, 104), true, false)
		}
	}
	if !hasStats {
		middleY := (plot.Top + plot.Bottom) / 2
		nnoChartText(hdc, "Нет подтверждённых циклов для распределения", RECT{Left: plot.Left + 5, Top: middleY - 10, Right: plot.Right - 5, Bottom: middleY + 12}, rgb(119, 48, 57), true, true)
	}
}

func nnoChartTextRight(hdc uintptr, content string, bounds RECT, color uintptr, bold bool) {
	font := hFontButtonCompact
	if bold {
		font = hFontHeaderSmall
	}
	if font == 0 {
		font = hFont
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	pSetTextColor.Call(hdc, color)
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(content))), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000|0x0002)
	if old != 0 {
		pSelectObject.Call(hdc, old)
	}
}

// Keep the same complete words in compact and expanded legends.
// Native DrawText wraps long labels inside the dedicated panel rather than
// truncating them or painting across the chart.
// The axis captions have their own quiet but high-contrast 12px typography.
// They occupy the axis gutters, never the chart title, graph or legend.
func nnoChartAxisCaption(hdc uintptr, caption string, bounds RECT, centered bool) {
	font := hFontReportEvidence
	if font == 0 {
		font = hFontButtonCompact
	}
	if font == 0 {
		font = hFont
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	pSetTextColor.Call(hdc, rgb(28, 71, 116))
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	flags := uintptr(DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX)
	if centered {
		flags |= DT_CENTER
	}
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), flags)
	if old != 0 {
		pSelectObject.Call(hdc, old)
	}
}

func nnoDrawExpandedAxisCaptions(hdc uintptr, rc, plot RECT, chart int, events bool) {
	if !nnoExpandedPainting || nnoExpandedKind != chart {
		return
	}
	yName := "Наработка, сут."
	xName := "Способы эксплуатации"
	if chart == 0 {
		xName = "Период, мес."
		if events {
			yName = "События, шт."
		}
	}
	// Vertical-axis caption is adjacent to the scale at its upper end.
	nnoChartAxisCaption(hdc, yName, RECT{Left: rc.Left + 7, Top: rc.Top + 5, Right: plot.Left - 8, Bottom: plot.Top - 3}, true)
	// X caption is directly beneath the category/month ticks, not beside the title.
	nnoChartAxisCaption(hdc, xName, RECT{Left: plot.Left + 5, Top: plot.Bottom + 28, Right: plot.Right - 5, Bottom: plot.Bottom + 48}, true)
}

// Use natural-height text in a centered row; measured multiline content and
// its marker share one vertical center, including at 125%/150% DPI.
func nnoChartLegendTextCentered(hdc uintptr, content string, bounds RECT, color uintptr) {
	font := hFontButtonCompact
	// The explanatory legend uses the smaller evidence font where available.
	// Its height is more predictable in compact panels at 125/150% DPI.
	if hFontReportEvidence != 0 {
		font = hFontReportEvidence
	}
	if font == 0 {
		font = hFont
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	pSetTextColor.Call(hdc, color)
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	measure := RECT{Left: 0, Top: 0, Right: bounds.Right - bounds.Left, Bottom: 0}
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(content))), ^uintptr(0), uintptr(unsafe.Pointer(&measure)), DT_CALCRECT|DT_WORDBREAK|DT_NOPREFIX)
	height := measure.Bottom - measure.Top
	available := bounds.Bottom - bounds.Top
	if height > available {
		height = available
	}
	top := bounds.Top + (available-height)/2
	draw := RECT{Left: bounds.Left, Top: top, Right: bounds.Right, Bottom: top + height}
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(content))), ^uintptr(0), uintptr(unsafe.Pointer(&draw)), DT_WORDBREAK|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(hdc, old)
	}
}

func nnoChartLegendText(hdc uintptr, content string, bounds RECT, color uintptr) {
	font := hFontButtonCompact
	if font == 0 {
		font = hFont
	}
	pSetBkMode.Call(hdc, TRANSPARENT)
	pSetTextColor.Call(hdc, color)
	old, _, _ := pSelectObject.Call(hdc, uintptr(font))
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(p(content))), ^uintptr(0), uintptr(unsafe.Pointer(&bounds)), DT_WORDBREAK|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(hdc, old)
	}
}

// A four-part explanation in plain engineering Russian, with visual markers.
// All text is drawn in a dedicated header band, never over the chart grid.
// Each explanation gets a full row in its own RIGHT-hand panel. No tooltip,
// label, symbol, or zoomed graph primitive can share this rectangle.
// Legend rows are reserved independently of the plotted surface. Two long
// definitions receive larger rows in compact mode to avoid text collision.
func nnoDistributionLegendRows(panel RECT) [4]RECT {
	bodyTop, bodyBottom := panel.Top+30, panel.Bottom-7
	available := max32(0, bodyBottom-bodyTop)
	// Do not stretch each legend item across a quarter of a tall expanded
	// window. Keep its symbol aligned with the text even if the panel grows.
	heights := [4]int32{52, 52, 42, 42}
	if available < 188 {
		// Reserve enough for two-line explanations before shorter one-line rows.
		heights = [4]int32{max32(34, (available-56)/2), max32(34, (available-56)/2), 28, 28}
	}
	total := heights[0] + heights[1] + heights[2] + heights[3]
	start := bodyTop
	if available > total {
		start += (available - total) / 2
	}
	var rows [4]RECT
	for i, h := range heights {
		rows[i] = RECT{Left: panel.Left + 5, Top: start, Right: panel.Right - 5, Bottom: start + h}
		start += h
	}
	return rows
}

func nnoDistributionLegendLabels(panel RECT) [4]string {
	// Full names are shown when the legend has the space for wrapped text;
	// otherwise use equivalent compact terms without clipping at 125–150% DPI.
	if panel.Bottom-panel.Top < 207 || panel.Right-panel.Left < 235 {
		return [4]string{"Типичный диапазон", "Основные 50 %", "Медиана", "Средняя"}
	}
	return [4]string{"Диапазон типичных значений", "Основные 50 % значений", "Медианная наработка", "Средняя наработка"}
}

func nnoDistributionSelectionLabel(kind int, medianMode bool) string {
	switch kind {
	case nnoDistributionLower:
		return "Минимум"
	case nnoDistributionUpper:
		return "Максимум"
	case nnoDistributionQ1:
		return "Нижняя четверть (Q1)"
	case nnoDistributionQ3:
		return "Верхняя четверть (Q3)"
	case nnoDistributionMedian:
		return "Медиана"
	case nnoDistributionMean:
		return "Средняя"
	default:
		if medianMode {
			return "Медиана"
		}
		return "Средняя"
	}
}

func nnoDrawDistributionLegend(hdc uintptr, rc, plot RECT) {
	panel := RECT{Left: plot.Right + 11, Top: rc.Top + 5, Right: rc.Right - 7, Bottom: rc.Bottom - 5}
	if panel.Right-panel.Left < 116 || panel.Bottom-panel.Top < 94 {
		return
	}
	nnoChartPanelFill(hdc, panel, rgb(246, 249, 253))
	nnoChartStroke(hdc, []POINT{{X: panel.Left, Y: panel.Top}, {X: panel.Right, Y: panel.Top}, {X: panel.Right, Y: panel.Bottom}, {X: panel.Left, Y: panel.Bottom}, {X: panel.Left, Y: panel.Top}}, rgb(214, 225, 236), 1, false)
	nnoChartText(hdc, "ОБОЗНАЧЕНИЯ", RECT{Left: panel.Left + 6, Top: panel.Top + 3, Right: panel.Right - 6, Bottom: panel.Top + 24}, rgb(25, 65, 112), true, true)
	labels := nnoDistributionLegendLabels(panel)
	rows := nnoDistributionLegendRows(panel)
	for i, row := range rows {
		if row.Bottom-row.Top < 18 {
			continue
		}
		if i != 0 {
			nnoChartStroke(hdc, []POINT{{X: row.Left + 4, Y: row.Top}, {X: row.Right - 4, Y: row.Top}}, rgb(232, 238, 245), 1, false)
		}
		midY := row.Top + (row.Bottom-row.Top)/2
		left := panel.Left + 10
		ink := rgb(25, 65, 112)
		switch i {
		case 0:
			nnoChartStroke(hdc, []POINT{{X: left + 9, Y: midY - 6}, {X: left + 9, Y: midY + 6}}, ink, 1, false)
			nnoChartStroke(hdc, []POINT{{X: left + 4, Y: midY - 6}, {X: left + 14, Y: midY - 6}}, ink, 1, false)
			nnoChartStroke(hdc, []POINT{{X: left + 4, Y: midY + 6}, {X: left + 14, Y: midY + 6}}, ink, 1, false)
		case 1:
			nnoChartStroke(hdc, []POINT{{X: left + 3, Y: midY - 6}, {X: left + 16, Y: midY - 6}, {X: left + 16, Y: midY + 6}, {X: left + 3, Y: midY + 6}, {X: left + 3, Y: midY - 6}}, ink, 2, false)
		case 2:
			nnoChartStroke(hdc, []POINT{{X: left + 3, Y: midY}, {X: left + 16, Y: midY}}, ink, 3, false)
		case 3:
			nnoChartStroke(hdc, []POINT{{X: left + 9, Y: midY - 6}, {X: left + 16, Y: midY}, {X: left + 9, Y: midY + 6}, {X: left + 2, Y: midY}, {X: left + 9, Y: midY - 6}}, ink, 2, false)
		}
		// Text is always inside its own row. No vertical-centering flag on wrapped DrawText.
		textArea := RECT{Left: left + 26, Top: row.Top + 2, Right: panel.Right - 9, Bottom: row.Bottom - 2}
		nnoChartLegendTextCentered(hdc, labels[i], textArea, rgb(47, 74, 108))
	}
}

func nnoDrawEventCountChart(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	hdc := uintptr(di.HDC)
	rc := di.RcItem
	nnoChartFrame(hdc, rc)
	n := len(nnoFleetMonthData)
	if n == 0 {
		return
	}
	plot := nnoRollingPlot(rc)
	if plot.Right-plot.Left < 140 || plot.Bottom-plot.Top < 55 {
		return
	}
	selected := nnoSelectedGroup()
	groups := []int{}
	counts := make([][]int, nnoMethodCount)
	for i := 0; i < nnoMethodCount; i++ {
		counts[i] = make([]int, n)
		if selected == "Все способы" || selected == nnoMethodName(i) {
			groups = append(groups, i)
		}
	}
	for _, e := range nnoFleetEventData {
		end, err := failureDate(e.Date)
		if err != nil {
			continue
		}
		index := nnoMonthIndex(nnoMonthStart(end), nnoFleetMonthData[0].Month)
		if index < 0 || index >= n {
			continue
		}
		for _, g := range groups {
			if nnoMethodOfFailure(e) == nnoMethodName(g) {
				counts[g][index]++
				break
			}
		}
	}
	maxi := 0
	for _, g := range groups {
		for _, v := range counts[g] {
			if v > maxi {
				maxi = v
			}
		}
	}
	scale := float64(maxIntChart(((maxi+3)/4)*4, 4)) / nnoZoomValue(0)
	if scale < 1 {
		scale = 1
	}
	dates := make([]string, n)
	for i, m := range nnoFleetMonthData {
		dates[i] = m.Month.Format("01.06")
	}
	nnoChartAxes(hdc, rc, plot, scale, dates)
	nnoChartText(hdc, "СОБЫТИЯ · шт. · НЕ ННО", RECT{Left: plot.Left, Top: rc.Top + 3, Right: plot.Right, Bottom: rc.Top + 23}, rgb(60, 87, 113), true, false)
	nnoDrawExpandedAxisCaptions(hdc, rc, plot, 0, true)
	clipEvents, _, _ := nnoChartSaveDC.Call(hdc)
	if clipEvents != 0 {
		nnoChartIntersectClipRect.Call(hdc, uintptr(plot.Left), uintptr(plot.Top), uintptr(plot.Right+1), uintptr(plot.Bottom+1))
	}
	for _, g := range groups {
		color := nnoMethodAccent(g)
		pts := make([]POINT, n)
		for i, v := range counts[g] {
			pts[i] = POINT{X: plot.Left + int32(i)*(plot.Right-plot.Left)/int32(maxIntChart(n-1, 1)), Y: plot.Bottom - int32(float64(v)/scale*float64(plot.Bottom-plot.Top))}
		}
		nnoChartStroke(hdc, pts, color, 2, false)
		for _, pt := range pts {
			nnoChartDot(hdc, pt.X, pt.Y, color, 2)
		}
	}
	if clipEvents != 0 {
		nnoChartRestoreDC.Call(hdc, clipEvents)
	}
	nnoDrawFleetMethodLegend(hdc, rc, plot, groups)
}
