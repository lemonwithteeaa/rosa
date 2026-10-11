//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// A separate full-size chart view; the same rendering and source values are
// reused, rather than generating a second set of synthetic chart data.
const (
	nnoExpandedCanvasID   = 9981
	nnoExpandedMinusID    = 9982
	nnoExpandedResetID    = 9983
	nnoExpandedPlusID     = 9984
	nnoExpandedCloseID    = 9985
	nnoExpandedApplyID    = 9986
	nnoExpandedAxisAutoID = 9987
	nnoExpandedYEditID    = 9989
)

var (
	nnoExpandedDone                       bool
	nnoExpandedOpen                       bool
	nnoExpandedPainting                   bool
	nnoExpandedKind                       int
	nnoExpandedWindow                     syscall.Handle
	nnoExpandedCanvas                     syscall.Handle
	nnoExpandedTitle                      syscall.Handle
	nnoExpandedScale                      syscall.Handle
	nnoExpandedMinus                      syscall.Handle
	nnoExpandedReset                      syscall.Handle
	nnoExpandedPlus                       syscall.Handle
	nnoExpandedClose                      syscall.Handle
	nnoExpandedCallback                   uintptr
	nnoExpandedYEdit                      syscall.Handle
	nnoExpandedYAxisLabel                 syscall.Handle
	nnoExpandedApply, nnoExpandedAxisAuto syscall.Handle
	// 0 = automatic grid interval; values persist during the application session.
	nnoExpandedYStep          = [2]int{0, 0}
	nnoExpandedCanvasOldProc  uintptr
	nnoExpandedCanvasCallback uintptr
	nnoExpandedPicking        bool
)

// The enlarged window has two independent control bands. The close action is
// right-aligned and never competes with the chart title or axis settings.
func nnoExpandedLayout(width, height int32) {
	if width < 820 || height < 420 {
		return
	}
	move := func(h syscall.Handle, x, y, w, hgt int32) {
		if h != 0 && w > 0 && hgt > 0 {
			pSetWindowPos.Call(uintptr(h), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(hgt), SWP_NOZORDER|0x0010)
		}
	}
	// Row 1: page title on the left; zoom and Close form a stable right group.
	move(nnoExpandedTitle, 24, 18, width-542, 36)
	move(nnoExpandedMinus, width-510, 16, 48, 38)
	move(nnoExpandedReset, width-454, 16, 82, 38)
	move(nnoExpandedPlus, width-364, 16, 48, 38)
	move(nnoExpandedScale, width-306, 20, 142, 30)
	move(nnoExpandedClose, width-150, 16, 126, 38)
	// Row 2: only the Y-axis interval is editable. The X-axis remains automatic.
	move(nnoExpandedYAxisLabel, 24, 74, 156, 28)
	move(nnoExpandedYEdit, 188, 72, 108, 34)
	move(nnoExpandedApply, 311, 70, 136, 38)
	move(nnoExpandedAxisAuto, 459, 70, 159, 38)
	move(nnoExpandedCanvas, 24, 125, width-48, height-157)
}

func nnoExpandedRefresh() {
	if nnoExpandedScale != 0 {
		setText(nnoExpandedScale, fmt.Sprintf("Масштаб %.0f%%", nnoZoomValue(nnoExpandedKind)*100))
	}
	if nnoExpandedCanvas != 0 {
		pInvalidateRect.Call(uintptr(nnoExpandedCanvas), 0, 0)
	}
	nnoZoomRepaint()
}

func nnoExpandedAxisText(step int) string {
	if step == 0 {
		return "Авто"
	}
	return strconv.Itoa(step)
}

func nnoExpandedSetAxisControls() {
	if nnoExpandedYEdit != 0 {
		setText(nnoExpandedYEdit, nnoExpandedAxisText(nnoExpandedYStep[nnoExpandedKind]))
	}
}

func nnoExpandedParseAxisStep(raw string, maxStep int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "Авто") {
		return 0, nil
	}
	step, err := strconv.Atoi(value)
	if err != nil || step < 1 || step > maxStep {
		return 0, fmt.Errorf("шаг должен быть целым числом от 1 до %d либо «Авто»", maxStep)
	}
	return step, nil
}

func nnoExpandedApplyAxes() bool {
	y, err := nnoExpandedParseAxisStep(getText(nnoExpandedYEdit), 10000)
	if err != nil {
		msg("Шаг вертикальной оси", err.Error(), MB_OK|MB_ICONWARNING)
		return false
	}
	minY := 10
	if nnoExpandedKind == 0 && nnoFleetShowEventTrend {
		minY = 1
	}
	if y > 0 && y < minY {
		unit := "суток"
		if minY == 1 {
			unit = "событий"
		}
		msg("Шаг вертикальной оси", fmt.Sprintf("Минимальный ручной шаг — %d %s.", minY, unit), MB_OK|MB_ICONWARNING)
		return false
	}
	nnoExpandedYStep[nnoExpandedKind] = y
	nnoExpandedRefresh()
	return true
}

// A real child mouse handler is required: WM_DRAWITEM alone cannot select values.
func nnoExpandedCanvasProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == WM_LBUTTONDOWN || message == 0x0200 {
		nnoExpandedPicking = true
		defer func() { nnoExpandedPicking = false }()
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		x := int32(int16(loWord(lParam)))
		y := int32(int16(hiWord(lParam)))
		if nnoExpandedKind == 1 {
			groups := nnoDistributionGroupIndices(nnoSelectedGroup())
			picked, ok := nnoPickDistributionMarkerExtended(rc, groups, nnoDistributionDisplayScale(), x, y, nnoFleetShowMedian, nnoFleetCycleSummaries, true)
			if message == WM_LBUTTONDOWN {
				if ok {
					nnoDistributionSelected = picked
				} else {
					nnoDistributionSelected.Valid = false
				}
				pInvalidateRect.Call(hwnd, 0, 0)
				return 0
			}
			if !nnoDistributionSelected.Valid && (nnoDistributionHover.Valid != ok || (ok && (nnoDistributionHover.Group != picked.Group || nnoDistributionHover.Kind != picked.Kind))) {
				nnoDistributionHover = picked
				pInvalidateRect.Call(hwnd, 0, 0)
			}
		} else if !nnoFleetShowEventTrend {
			picked := nnoPickRolling(rc, x, y)
			if message == WM_LBUTTONDOWN {
				nnoRollingSelected = picked
				pInvalidateRect.Call(hwnd, 0, 0)
				return 0
			}
			if !nnoRollingSelected.Valid && (nnoRollingHover.Valid != picked.Valid || (picked.Valid && (nnoRollingHover.Group != picked.Group || nnoRollingHover.Month != picked.Month || nnoRollingHover.Median != picked.Median))) {
				nnoRollingHover = picked
				pInvalidateRect.Call(hwnd, 0, 0)
			}
		}
	}
	if nnoExpandedCanvasOldProc != 0 {
		r, _, _ := nnoDistributionCallWndProc.Call(nnoExpandedCanvasOldProc, hwnd, uintptr(message), wParam, lParam)
		return r
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func nnoExpandedWindowProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := hwnd
		title := "Динамика наработки · 12 месяцев"
		if nnoExpandedKind == 1 {
			title = "Распределение наработки по способам эксплуатации"
		}
		nnoExpandedTitle = modalCreate("STATIC", title, WS_CHILD|WS_VISIBLE, 24, 18, 600, 34, parent, 0)
		nnoExpandedMinus = modalCreate("BUTTON", "−", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 600, 16, 52, 38, parent, nnoExpandedMinusID)
		nnoExpandedReset = modalCreate("BUTTON", "100%", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 655, 16, 85, 38, parent, nnoExpandedResetID)
		nnoExpandedPlus = modalCreate("BUTTON", "+", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 745, 16, 52, 38, parent, nnoExpandedPlusID)
		nnoExpandedScale = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 810, 20, 120, 30, parent, 0)
		nnoExpandedClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 945, 16, 125, 38, parent, nnoExpandedCloseID)
		yAxisLabel := "Шаг Y, суток:"
		if nnoExpandedKind == 0 && nnoFleetShowEventTrend {
			yAxisLabel = "Шаг Y, событий:"
		}
		nnoExpandedYAxisLabel = modalCreate("STATIC", yAxisLabel, WS_CHILD|WS_VISIBLE, 24, 73, 172, 30, parent, 0)
		nnoExpandedYEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 206, 72, 100, 32, parent, nnoExpandedYEditID)
		nnoExpandedApply = modalCreate("BUTTON", "Применить", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 320, 69, 130, 38, parent, nnoExpandedApplyID)
		nnoExpandedAxisAuto = modalCreate("BUTTON", "Шаг Y: авто", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 462, 69, 154, 38, parent, nnoExpandedAxisAutoID)
		nnoExpandedCanvas = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER|SS_OWNERDRAW|SS_NOTIFY, 24, 126, 1050, 505, parent, nnoExpandedCanvasID)
		if nnoExpandedCanvas != 0 {
			if nnoExpandedCanvasCallback == 0 {
				nnoExpandedCanvasCallback = syscall.NewCallback(nnoExpandedCanvasProc)
			}
			old, _, _ := nnoDistributionSetWndProc.Call(uintptr(nnoExpandedCanvas), ^uintptr(3), nnoExpandedCanvasCallback)
			nnoExpandedCanvasOldProc = old
		}
		nnoExpandedSetAxisControls()
		if nnoExpandedTitle != 0 && hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(nnoExpandedTitle), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}
		// Both control rows must be laid out before the first paint, not only
		// after the user manually resizes the separate chart window.
		var initial RECT
		pGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&initial)))
		nnoExpandedLayout(initial.Right-initial.Left, initial.Bottom-initial.Top)
		nnoExpandedRefresh()
		return 0
	case WM_SIZE:
		nnoExpandedLayout(int32(uint16(lParam)), int32(uint16(lParam>>16)))
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X, mmi.PtMinTrackSize.Y = 900, 560
		}
		return 0
	case WM_DRAWITEM:
		if di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam)); di != nil && int(di.CtlID) == nnoExpandedCanvasID {
			nnoExpandedPainting = true
			defer func() { nnoExpandedPainting = false }()
			if nnoExpandedKind == 0 {
				if nnoFleetShowEventTrend {
					nnoDrawEventCountChart(di)
				} else {
					nnoDrawRollingCycleChart(di)
				}
			} else {
				nnoDrawCycleDistribution(di)
			}
			return 1
		}
	case WM_COMMAND:
		if hiWord(wParam) == 0 {
			switch int(loWord(wParam)) {
			case nnoExpandedMinusID:
				nnoSetChartZoom(nnoExpandedKind, nnoZoomValue(nnoExpandedKind)/1.25)
			case nnoExpandedPlusID:
				nnoSetChartZoom(nnoExpandedKind, nnoZoomValue(nnoExpandedKind)*1.25)
			case nnoExpandedResetID:
				nnoSetChartZoom(nnoExpandedKind, 1)
			case nnoExpandedApplyID:
				nnoExpandedApplyAxes()
			case nnoExpandedAxisAutoID:
				nnoExpandedYStep[nnoExpandedKind] = 0
				nnoExpandedSetAxisControls()
			case nnoExpandedCloseID:
				nnoExpandedDone = true
				nnoDestroyTargetWindow.Call(uintptr(hwnd))
				return 0
			default:
				break
			}
			nnoExpandedRefresh()
			return 0
		}
	case nnoWMClose:
		nnoExpandedDone = true
		nnoDestroyTargetWindow.Call(uintptr(hwnd))
		return 0
	case WM_DESTROY:
		nnoExpandedDone = true
		return 0
	}
	result, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return result
}

func nnoOpenExpandedChart(kind int) {
	if nnoExpandedOpen || (kind != 0 && kind != 1) {
		return
	}
	nnoExpandedOpen = true
	defer func() { nnoExpandedOpen = false }()
	nnoExpandedDone = false
	nnoExpandedKind = kind
	nnoExpandedCanvas, nnoExpandedTitle, nnoExpandedScale = 0, 0, 0
	nnoExpandedCanvasOldProc = 0
	nnoExpandedYEdit = 0
	cls := p("ROSANNOLargeChart820")
	inst, _, _ := pGetModuleHandleW.Call(0)
	if nnoExpandedCallback == 0 {
		nnoExpandedCallback = syscall.NewCallback(nnoExpandedWindowProc)
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: nnoExpandedCallback,
		HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls,
		HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	title := "РОСА — динамика наработки"
	if kind == 1 {
		title = "РОСА — распределение по способам"
	}
	win, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(title))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN_AUTH, 0x80000000, 0x80000000, 1100, 680,
		uintptr(hMain), 0, inst, 0)
	if win == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Наработка", "Не удалось открыть увеличенный график.", MB_OK|MB_ICONWARNING)
		return
	}
	nnoExpandedWindow = syscall.Handle(win)
	var m MSG
	for !nnoExpandedDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(win, &m) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	nnoExpandedWindow, nnoExpandedCanvas = 0, 0
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	nnoZoomRepaint()
}
