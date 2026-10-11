//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

const (
	econComparisonCanvasID = 9763
	econComparisonCloseID  = 9764
)

var (
	econComparisonDone   bool
	econComparisonRows   []AlternativeMethodScenario
	econComparisonActive string
	econComparisonTitle  syscall.Handle
	econComparisonInfo   syscall.Handle
	econComparisonCanvas syscall.Handle
	econComparisonClose  syscall.Handle
)

func economicsComparisonValue(s AlternativeMethodScenario, metric int) (float64, bool) {
	known := economicsScenarioCalculated(s)
	switch metric {
	case 0:
		return s.ForecastBaseDays, s.ForecastBaseDays > 0
	case 1:
		return s.AnnualOPEXRub, s.AnnualOPEXRub != 0 || known
	case 2:
		return s.OneTimeCAPEXRub, s.OneTimeCAPEXRub != 0 || known
	default:
		return s.NetBenefitK, known
	}
}

func economicsComparisonPaint(di *DRAWITEMSTRUCT) {
	if di == nil {
		return
	}
	hdc := uintptr(di.HDC)
	rc := di.RcItem
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), uintptr(brushWhite))
	w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
	if w < 560 || h < 350 {
		economicsMetricText(di.HDC, "Расширьте окно для просмотра диаграмм", rc, hFontHeaderSmall, rgb(15, 48, 82))
		return
	}
	titles := [...]string{"ПРОГНОЗ МРП · сут.", "OPEX · руб./год", "CAPEX · руб.", "ЧИСТЫЙ ЭФФЕКТ · тыс. руб. за горизонт"}
	gap := int32(12)
	panelW, panelH := (w-gap)/2, (h-gap)/2
	for metric := 0; metric < 4; metric++ {
		col, row := int32(metric%2), int32(metric/2)
		panel := RECT{Left: rc.Left + col*(panelW+gap), Top: rc.Top + row*(panelH+gap), Right: rc.Left + col*(panelW+gap) + panelW, Bottom: rc.Top + row*(panelH+gap) + panelH}
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&panel)), uintptr(brushLight))
		pFrameRect.Call(hdc, uintptr(unsafe.Pointer(&panel)), uintptr(brushBorder))
		titleRect := RECT{Left: panel.Left + 12, Top: panel.Top + 5, Right: panel.Right - 10, Bottom: panel.Top + 29}
		economicsMetricText(di.HDC, titles[metric], titleRect, hFontHeaderSmall, rgb(15, 48, 82))
		maxValue := 0.0
		for _, s := range econComparisonRows {
			if v, ok := economicsComparisonValue(s, metric); ok && math.Abs(v) > maxValue {
				maxValue = math.Abs(v)
			}
		}
		labelW := int32(100)
		valueW := int32(82)
		barX := panel.Left + labelW + 17
		barW := panel.Right - valueW - 14 - barX
		rowH := (panelH - 39) / int32(maxIntChart(len(econComparisonRows), 1))
		if rowH > 27 {
			rowH = 27
		}
		for i, s := range econComparisonRows {
			y := panel.Top + 33 + int32(i)*rowH
			v, ok := economicsComparisonValue(s, metric)
			active := strings.EqualFold(s.ID, econComparisonActive)
			nameColor := rgb(15, 48, 82)
			barBrush := brushNav
			if active || v < 0 {
				nameColor = rgb(178, 29, 47)
				barBrush = brushRedPressed
			}
			label := RECT{Left: panel.Left + 11, Top: y, Right: barX - 8, Bottom: y + rowH}
			economicsMetricText(di.HDC, s.Name, label, hFontButtonCompact, nameColor)
			bar := RECT{Left: barX, Top: y + rowH/2 - 5, Right: barX + barW, Bottom: y + rowH/2 + 5}
			pFillRect.Call(hdc, uintptr(unsafe.Pointer(&bar)), uintptr(brushWhite))
			if ok && maxValue > 0 && barW > 0 {
				bar.Right = bar.Left + int32(math.Round(math.Abs(v)/maxValue*float64(barW)))
				if v != 0 && bar.Right <= bar.Left {
					bar.Right = bar.Left + 1
				}
				pFillRect.Call(hdc, uintptr(unsafe.Pointer(&bar)), uintptr(barBrush))
			}
			shown := "—"
			if ok {
				shown = fmt.Sprintf("%.0f", v)
			}
			value := RECT{Left: panel.Right - valueW - 7, Top: y, Right: panel.Right - 7, Bottom: y + rowH}
			economicsMetricText(di.HDC, shown, value, hFontButtonCompact, nameColor)
		}
	}
}

func economicsComparisonLayout(w, h int32) {
	if w < 1 || h < 1 {
		return
	}
	setPos(econComparisonTitle, 22, 17, w-44, 32)
	setPos(econComparisonInfo, 22, 52, w-44, 34)
	setPos(econComparisonCanvas, 22, 94, w-44, max32(350, h-161))
	setPos(econComparisonClose, w-182, h-52, 160, 36)
}

func economicsComparisonWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		econComparisonTitle = modalCreate("STATIC", "СРАВНЕНИЕ ПАРАЛЛЕЛЬНЫХ СХЕМ", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 17, 1000, 32, parent, 0)
		econComparisonInfo = modalCreate("STATIC", "У каждого показателя своя шкала. Красным выделена активная схема или отрицательный эффект; «—» означает отсутствие актуального расчёта.", WS_CHILD|WS_VISIBLE|SS_LEFT, 22, 52, 1000, 34, parent, 0)
		econComparisonCanvas = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER|SS_OWNERDRAW, 22, 94, 1000, 450, parent, econComparisonCanvasID)
		econComparisonClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 850, 600, 160, 36, parent, econComparisonCloseID)
		if hFontSystemName != 0 {
			pSendMessageW.Call(uintptr(econComparisonTitle), WM_SETFONT, uintptr(hFontSystemName), 1)
		}
		if hFontButtonCompact != 0 {
			pSendMessageW.Call(uintptr(econComparisonInfo), WM_SETFONT, uintptr(hFontButtonCompact), 1)
		}
		economicsComparisonLayout(1060, 640)
		return 0
	case WM_SIZE:
		economicsComparisonLayout(int32(uint16(lParam)), int32(uint16(lParam>>16)))
		return 0
	case WM_DRAWITEM:
		if di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam)); di != nil && int(di.CtlID) == econComparisonCanvasID {
			economicsComparisonPaint(di)
			return 1
		}
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X, mmi.PtMinTrackSize.Y = 880, 660
		}
		return 0
	case WM_COMMAND:
		if int(wParam&0xffff) == econComparisonCloseID {
			econComparisonDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010: // WM_CLOSE
		econComparisonDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	case WM_DESTROY:
		econComparisonDone = true
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openEconomicsComparisonChart() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Экономическая оценка", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	econComparisonRows = alternativeScenariosForObject(selectedObjectKey)
	if len(econComparisonRows) == 0 {
		msg("Экономическая оценка", "Для объекта пока нет схем сравнения.", MB_OK|MB_ICONINFORMATION)
		return
	}
	econComparisonActive = activeAlternativeScenarioID(selectedObjectKey)
	econComparisonDone = false
	cls := p("ROSAEconomicsComparisonV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(economicsComparisonWndProc),
		HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — сравнение схем"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1100, 720, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Экономическая оценка", "Не удалось открыть диаграмму.", MB_OK|MB_ICONWARNING)
		return
	}
	var m MSG
	for !econComparisonDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &m) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
