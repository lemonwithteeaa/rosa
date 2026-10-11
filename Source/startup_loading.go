//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	idStartupLoadingCanvas       = 7591
	startupPumpjackBitmapW int32 = 306
	startupPumpjackBitmapH int32 = 202
)

var (
	startupLoadingCanvas   syscall.Handle
	startupLoadingStage    = 1
	startupLoadingTitle    syscall.Handle
	startupLoadingText     syscall.Handle
	startupLoadingSmall    syscall.Handle
	startupLoadingMenu     syscall.Handle
	startupPumpjackBitmap  syscall.Handle
	startupLoadingProgress string
	startupLoadingHeadline = "ВХОД ВЫПОЛНЕН"
	startupCreatePen       = gdi32.NewProc("CreatePen")
	startupPolyline        = gdi32.NewProc("Polyline")
	startupPolygon         = gdi32.NewProc("Polygon")
	startupEllipse         = gdi32.NewProc("Ellipse")
	startupPeekMessage     = user32.NewProc("PeekMessageW")
)

func startupDrawText(hdc syscall.Handle, text string, rect RECT, font syscall.Handle, color uintptr, flags uint32) {
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(font))
	pSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	pSetTextColor.Call(uintptr(hdc), color)
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), uintptr(flags))
	if old != 0 {
		pSelectObject.Call(uintptr(hdc), old)
	}
}

func startupMeasuredTextWidth(hdc syscall.Handle, text string, font syscall.Handle) int32 {
	if text == "" {
		return 0
	}
	old, _, _ := pSelectObject.Call(uintptr(hdc), uintptr(font))
	r := RECT{Left: 0, Top: 0, Right: 1200, Bottom: 80}
	pDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(DT_SINGLELINE|DT_CALCRECT|DT_NOPREFIX))
	if old != 0 {
		pSelectObject.Call(uintptr(hdc), old)
	}
	return max32(0, r.Right-r.Left)
}

func startupDrawCenteredInitialWord(hdc syscall.Handle, word string, panel RECT, yTop, yBottom int32) {
	runes := []rune(word)
	if len(runes) == 0 {
		return
	}
	first, rest := string(runes[:1]), string(runes[1:])
	firstW := startupMeasuredTextWidth(hdc, first, startupLoadingMenu)
	restW := startupMeasuredTextWidth(hdc, rest, startupLoadingMenu)
	totalW := firstW + restW
	x := panel.Left + (panel.Right-panel.Left-totalW)/2
	startupDrawText(hdc, first, RECT{Left: x, Top: yTop, Right: x + firstW + 2, Bottom: yBottom}, startupLoadingMenu, rgb(199, 28, 46), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
	startupDrawText(hdc, rest, RECT{Left: x + firstW, Top: yTop, Right: x + totalW + 3, Bottom: yBottom}, startupLoadingMenu, rgb(255, 255, 255), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
}

func startupDrawApprovedPumpjack(hdc syscall.Handle, panel RECT) bool {
	if startupPumpjackBitmap == 0 {
		return false
	}
	left := panel.Left + 12
	right := panel.Right - 12
	bottom := panel.Bottom - 10
	// The approved artwork lives only in the lower image zone. Keep a hard
	// gap below the last word so text can never overlap the pumpjack.
	top := max32(panel.Top+226, bottom-startupPumpjackBitmapH)
	zoneW, zoneH := right-left, bottom-top
	if zoneW <= 0 || zoneH <= 0 {
		return false
	}
	drawW, drawH := zoneW, zoneH
	if startupPumpjackBitmapW > 0 && startupPumpjackBitmapH > 0 {
		if zoneW*startupPumpjackBitmapH <= zoneH*startupPumpjackBitmapW {
			drawH = max32(1, zoneW*startupPumpjackBitmapH/startupPumpjackBitmapW)
			drawW = zoneW
		} else {
			drawW = max32(1, zoneH*startupPumpjackBitmapW/startupPumpjackBitmapH)
			drawH = zoneH
		}
	}
	drawLeft := left + (zoneW-drawW)/2
	drawTop := top + (zoneH-drawH)/2
	memDC, _, _ := pCreateCompatibleDC.Call(uintptr(hdc))
	if memDC == 0 {
		return false
	}
	old, _, _ := pSelectObject.Call(memDC, uintptr(startupPumpjackBitmap))
	pStretchBlt.Call(uintptr(hdc), uintptr(drawLeft), uintptr(drawTop), uintptr(drawW), uintptr(drawH), memDC, 0, 0, uintptr(startupPumpjackBitmapW), uintptr(startupPumpjackBitmapH), 0x00CC0020)
	if old != 0 {
		pSelectObject.Call(memDC, old)
	}
	pDeleteDC.Call(memDC)
	return true
}

func startupDrawPumpjack(hdc syscall.Handle, ox, oy, scale int32, color uintptr) {
	pen, _, _ := startupCreatePen.Call(0, uintptr(max32(3, 4*scale)), color)
	brush, _, _ := pCreateSolidBrush.Call(color)
	old, _, _ := pSelectObject.Call(uintptr(hdc), pen)
	oldBrush, _, _ := pSelectObject.Call(uintptr(hdc), brush)
	drawLine := func(points ...POINT) {
		startupPolyline.Call(uintptr(hdc), uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
	}
	// Classical, proportionally balanced pumpjack silhouette. The walking beam,
	// horse head, polished rod, A-frame and crank are deliberately separated so
	// the icon remains recognisable at 125–200% Windows scaling.
	base := RECT{Left: ox, Top: oy - 7*scale, Right: ox + 224*scale, Bottom: oy}
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&base)), brush)
	// A-frame with cross brace.
	drawLine(POINT{ox + 42*scale, oy - 7*scale}, POINT{ox + 92*scale, oy - 112*scale}, POINT{ox + 140*scale, oy - 7*scale})
	drawLine(POINT{ox + 61*scale, oy - 48*scale}, POINT{ox + 121*scale, oy - 48*scale})
	drawLine(POINT{ox + 57*scale, oy - 7*scale}, POINT{ox + 116*scale, oy - 7*scale})
	// Walking beam, slightly descending towards the well.
	beam := []POINT{{ox + 81*scale, oy - 121*scale}, {ox + 172*scale, oy - 138*scale}, {ox + 211*scale, oy - 124*scale}, {ox + 207*scale, oy - 111*scale}, {ox + 170*scale, oy - 123*scale}, {ox + 84*scale, oy - 106*scale}}
	startupPolygon.Call(uintptr(hdc), uintptr(unsafe.Pointer(&beam[0])), uintptr(len(beam)))
	// Curved-looking horse head, approximated by a clean tapered polygon.
	horse := []POINT{{ox + 205*scale, oy - 129*scale}, {ox + 226*scale, oy - 122*scale}, {ox + 222*scale, oy - 91*scale}, {ox + 214*scale, oy - 76*scale}, {ox + 203*scale, oy - 82*scale}, {ox + 207*scale, oy - 111*scale}}
	startupPolygon.Call(uintptr(hdc), uintptr(unsafe.Pointer(&horse[0])), uintptr(len(horse)))
	// Polished rod and wellhead.
	drawLine(POINT{ox + 217*scale, oy - 82*scale}, POINT{ox + 217*scale, oy - 18*scale})
	wellhead := RECT{Left: ox + 207*scale, Top: oy - 18*scale, Right: ox + 227*scale, Bottom: oy - 7*scale}
	pFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&wellhead)), brush)
	// Pivot, counterweight and crank linkage.
	startupEllipse.Call(uintptr(hdc), uintptr(ox+82*scale), uintptr(oy-121*scale), uintptr(ox+102*scale), uintptr(oy-101*scale))
	startupEllipse.Call(uintptr(hdc), uintptr(ox+17*scale), uintptr(oy-58*scale), uintptr(ox+63*scale), uintptr(oy-12*scale))
	drawLine(POINT{ox + 40*scale, oy - 35*scale}, POINT{ox + 72*scale, oy - 91*scale}, POINT{ox + 88*scale, oy - 112*scale})
	pSelectObject.Call(uintptr(hdc), old)
	pSelectObject.Call(uintptr(hdc), oldBrush)
	pDeleteObject.Call(pen)
	pDeleteObject.Call(brush)
}

func drawStartupLoading(di *DRAWITEMSTRUCT) {
	rc := di.RcItem
	// RC556: restrained loading surface — light workspace, one navy identity
	// panel and a thin red brand accent. No large progress "bricks".
	bg, _, _ := pCreateSolidBrush.Call(rgb(247, 249, 252))
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&rc)), bg)
	pDeleteObject.Call(bg)
	red, _, _ := pCreateSolidBrush.Call(rgb(199, 28, 46))
	topAccent := RECT{Left: rc.Left, Top: rc.Top, Right: rc.Right, Bottom: rc.Top + 6}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&topAccent)), red)

	width := rc.Right - rc.Left
	leftWidth := int32(330)
	if width < 980 {
		leftWidth = max32(296, width*32/100)
	}
	navy, _, _ := pCreateSolidBrush.Call(rgb(18, 53, 84))
	leftPanel := RECT{Left: rc.Left, Top: rc.Top + 6, Right: rc.Left + leftWidth, Bottom: rc.Bottom}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&leftPanel)), navy)
	pDeleteObject.Call(navy)
	panelRect := RECT{Left: rc.Left, Top: rc.Top + 6, Right: rc.Left + leftWidth, Bottom: rc.Bottom}
	identityAccent := RECT{Left: rc.Left + leftWidth/2 - 30, Top: rc.Top + 82, Right: rc.Left + leftWidth/2 + 30, Bottom: rc.Top + 86}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&identityAccent)), red)
	pDeleteObject.Call(red)

	// RC557: approved loading-panel hierarchy. The identity and four working
	// directions are centered; only the initials Р-О-С-А are red.
	startupDrawText(di.HDC, "РОСА", RECT{Left: rc.Left + 12, Top: rc.Top + 20, Right: rc.Left + leftWidth - 12, Bottom: rc.Top + 72}, startupLoadingTitle, rgb(255, 255, 255), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
	startupDrawCenteredInitialWord(di.HDC, "РАСЧЁТ", panelRect, rc.Top+96, rc.Top+120)
	startupDrawCenteredInitialWord(di.HDC, "ОЦЕНКА", panelRect, rc.Top+122, rc.Top+146)
	startupDrawCenteredInitialWord(di.HDC, "СОПРОВОЖДЕНИЕ", panelRect, rc.Top+148, rc.Top+172)
	startupDrawCenteredInitialWord(di.HDC, "АВТОМАТИЗАЦИЯ", panelRect, rc.Top+174, rc.Top+198)
	// Use only the approved realistic pumpjack. Never replace it with the old
	// vector silhouette: a missing bitmap is preferable to showing the wrong art.
	startupDrawApprovedPumpjack(di.HDC, panelRect)

	contentLeft := rc.Left + leftWidth + 38
	contentRight := rc.Right - 32
	startupDrawText(di.HDC, "ПОДГОТОВКА РАБОЧЕГО ПРОСТРАНСТВА", RECT{Left: contentLeft, Top: rc.Top + 28, Right: contentRight, Bottom: rc.Top + 66}, startupLoadingTitle, rgb(18, 53, 84), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
	startupDrawText(di.HDC, "Вход выполнен. РОСА проверяет профиль и рабочие данные.", RECT{Left: contentLeft, Top: rc.Top + 68, Right: contentRight, Bottom: rc.Top + 100}, startupLoadingText, rgb(77, 91, 105), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)

	stages := []string{"Профиль", "Данные", "Модели", "Готово"}
	stageTop := rc.Top + 130
	lineLeft := contentLeft + 12
	lineRight := contentRight - 12
	segment := (lineRight - lineLeft) / 3
	linePen, _, _ := startupCreatePen.Call(0, 2, rgb(196, 204, 213))
	oldPen, _, _ := pSelectObject.Call(uintptr(di.HDC), linePen)
	linePoints := []POINT{{lineLeft, stageTop + 9}, {lineRight, stageTop + 9}}
	startupPolyline.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&linePoints[0])), uintptr(len(linePoints)))
	pSelectObject.Call(uintptr(di.HDC), oldPen)
	pDeleteObject.Call(linePen)
	for i, label := range stages {
		cx := lineLeft + int32(i)*segment
		fillColor := rgb(221, 226, 232)
		textColor := rgb(112, 123, 134)
		if i+1 < startupLoadingStage {
			fillColor = rgb(18, 53, 84)
			textColor = rgb(18, 53, 84)
		} else if i+1 == startupLoadingStage {
			fillColor = rgb(199, 28, 46)
			textColor = rgb(18, 53, 84)
		}
		brush, _, _ := pCreateSolidBrush.Call(fillColor)
		oldBrush, _, _ := pSelectObject.Call(uintptr(di.HDC), brush)
		pen, _, _ := startupCreatePen.Call(0, 1, fillColor)
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), pen)
		startupEllipse.Call(uintptr(di.HDC), uintptr(cx-8), uintptr(stageTop+1), uintptr(cx+8), uintptr(stageTop+17))
		pSelectObject.Call(uintptr(di.HDC), old)
		pSelectObject.Call(uintptr(di.HDC), oldBrush)
		pDeleteObject.Call(pen)
		pDeleteObject.Call(brush)
		labelRect := RECT{Left: cx - 58, Top: stageTop + 24, Right: cx + 58, Bottom: stageTop + 52}
		startupDrawText(di.HDC, label, labelRect, startupLoadingSmall, textColor, DT_SINGLELINE|DT_CENTER|DT_NOPREFIX)
	}

	stageText := []string{"Проверяем профиль пользователя…", "Загружаем рабочие данные и примеры…", "Подготавливаем расчётные модели и графики…", "Рабочее пространство готово"}
	index := startupLoadingStage - 1
	if index < 0 {
		index = 0
	}
	if index >= len(stageText) {
		index = len(stageText) - 1
	}
	status := stageText[index]
	if startupLoadingProgress != "" {
		status = startupLoadingProgress
	}
	card := RECT{Left: contentLeft, Top: rc.Top + 205, Right: contentRight, Bottom: rc.Top + 270}
	white, _, _ := pCreateSolidBrush.Call(rgb(255, 255, 255))
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&card)), white)
	pDeleteObject.Call(white)
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&card)), uintptr(brushBorder))
	accent, _, _ := pCreateSolidBrush.Call(rgb(199, 28, 46))
	accentBar := RECT{Left: card.Left, Top: card.Top, Right: card.Left + 5, Bottom: card.Bottom}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accentBar)), accent)
	pDeleteObject.Call(accent)
	startupDrawText(di.HDC, status, RECT{Left: card.Left + 18, Top: card.Top + 8, Right: card.Right - 14, Bottom: card.Bottom - 7}, startupLoadingText, rgb(31, 49, 66), DT_CENTER|DT_VCENTER|DT_WORDBREAK|DT_NOPREFIX)
	startupDrawText(di.HDC, "Окно приложения откроется автоматически", RECT{Left: contentLeft, Top: rc.Top + 282, Right: contentRight, Bottom: rc.Top + 314}, startupLoadingSmall, rgb(116, 128, 140), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
}

func startupLoadingWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		var client RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		canvas, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(p("STATIC"))), 0, WS_CHILD|WS_VISIBLE|SS_OWNERDRAW, 0, 0, uintptr(client.Right-client.Left), uintptr(client.Bottom-client.Top), hwnd, idStartupLoadingCanvas, 0, 0)
		startupLoadingCanvas = syscall.Handle(canvas)
		return 0
	case 0x0005: // WM_SIZE
		if startupLoadingCanvas != 0 {
			pSetWindowPos.Call(uintptr(startupLoadingCanvas), 0, 0, 0, uintptr(loWord(lParam)), uintptr(hiWord(lParam)), 0x0014)
		}
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil && int(di.CtlID) == idStartupLoadingCanvas {
			drawStartupLoading(di)
			return 1
		}
	case 0x0010: // WM_CLOSE: preparation cannot be cancelled halfway.
		return 0
	case WM_DESTROY:
		if startupLoadingTitle != 0 {
			pDeleteObject.Call(uintptr(startupLoadingTitle))
			startupLoadingTitle = 0
		}
		if startupLoadingText != 0 {
			pDeleteObject.Call(uintptr(startupLoadingText))
			startupLoadingText = 0
		}
		if startupLoadingSmall != 0 {
			pDeleteObject.Call(uintptr(startupLoadingSmall))
			startupLoadingSmall = 0
		}
		if startupLoadingMenu != 0 {
			pDeleteObject.Call(uintptr(startupLoadingMenu))
			startupLoadingMenu = 0
		}
		if startupPumpjackBitmap != 0 {
			pDeleteObject.Call(uintptr(startupPumpjackBitmap))
			startupPumpjackBitmap = 0
		}
		startupLoadingCanvas = 0
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

// showStartupLoadingWindow keeps a bright branded oilfield surface visible
// between successful authentication and creation of the main workspace.
func showStartupLoadingWindow() syscall.Handle {
	startupLoadingStage = 1
	startupLoadingProgress = ""
	startupLoadingTitle = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(27), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	startupLoadingText = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(17), 0, 0, 0, 500, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	startupLoadingSmall = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(14), 0, 0, 0, 500, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	startupLoadingMenu = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(15), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	if hb, err := buildStartupPumpjackBitmap(int(startupPumpjackBitmapW), int(startupPumpjackBitmapH)); err == nil {
		startupPumpjackBitmap = hb
	}
	cls := p("ROSAStartupLoadingV4")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(startupLoadingWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	metrics := user32.NewProc("GetSystemMetrics")
	screenW, _, _ := metrics.Call(0)
	screenH, _, _ := metrics.Call(1)
	const width, height = 1100, 450
	x := int32(screenW)/2 - width/2
	y := int32(screenH)/2 - height/2
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — подготовка рабочего пространства"))), 0x00C00000|WS_VISIBLE|0x02000000, uintptr(x), uintptr(y), width, height, 0, 0, inst, 0)
	if hwnd != 0 {
		pShowWindow.Call(hwnd, 5)
		pUpdateWindow.Call(hwnd)
	}
	return syscall.Handle(hwnd)
}

func setStartupLoadingStage(stage int) {
	startupLoadingStage = stage
	startupLoadingProgress = ""
	if startupLoadingCanvas != 0 {
		pInvalidateRect.Call(uintptr(startupLoadingCanvas), 0, 1)
		pRedrawWindow.Call(uintptr(startupLoadingCanvas), 0, 0, 0x0001|0x0100)
	}
}

// The initial teaching set is prepared on the Win32 UI thread because the
// application state and SQLite handle are initialised there.  Pumping only the
// splash-window queue between bounded work units prevents Windows from marking
// the application as hung during the one-time first launch.
func pumpStartupLoadingMessages() {
	var message MSG
	for {
		r, _, _ := startupPeekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, 0x0001) // PM_REMOVE
		if r == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
}

func setStartupLoadingProgress(text string) {
	startupLoadingProgress = text
	if startupLoadingCanvas != 0 {
		pInvalidateRect.Call(uintptr(startupLoadingCanvas), 0, 1)
		pUpdateWindow.Call(uintptr(startupLoadingCanvas))
	}
	pumpStartupLoadingMessages()
}
