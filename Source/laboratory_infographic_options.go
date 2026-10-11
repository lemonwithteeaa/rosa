//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	idLabInfoOptionsOK     = 9961
	idLabInfoOptionsCancel = 9962
)

var (
	labInfoOptionsDone, labInfoOptionsOK bool
	labInfoChecks                        [1]syscall.Handle
	labInfoOptionsInclude                [1]bool
	labReportOptionsHeader               syscall.Handle
	labReportOptionsTitle                syscall.Handle
	labReportOptionsAccent               syscall.Handle
)

var labInfoOptionLabels = []string{"Добавить формулы и условия расчёта отдельным приложением"}

func labInfoOptionsWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		h := syscall.Handle(hwnd)
		labReportOptionsHeader = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 560, 62, h, 0)
		labReportOptionsAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 62, 560, 5, h, 0)
		labReportOptionsTitle = modalCreate("STATIC", "ДОПОЛНИТЕЛЬНОЕ ПРИЛОЖЕНИЕ К ОТЧЁТУ", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_CENTERIMAGE, 24, 10, 510, 44, h, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(labReportOptionsTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		intro := modalCreate("STATIC", "Основное заключение и показатели войдут в отчёт автоматически.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 82, 510, 36, h, 0)
		if hFontBold != 0 {
			pSendMessageW.Call(uintptr(intro), WM_SETFONT, uintptr(hFontBold), 1)
		}
		for i, label := range labInfoOptionLabels {
			labInfoChecks[i] = modalCreate("BUTTON", label, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 30, 132+int32(i)*38, 500, 30, h, 100+i)
			pSendMessageW.Call(uintptr(labInfoChecks[i]), BM_SETCHECK, 0, 0)
			if hFont != 0 {
				pSendMessageW.Call(uintptr(labInfoChecks[i]), WM_SETFONT, uintptr(hFont), 1)
			}
		}
		modalCreate("BUTTON", "Продолжить", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 188, 220, 42, h, idLabInfoOptionsOK)
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 258, 188, 160, 42, h, idLabInfoOptionsCancel)
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil && (int(di.CtlID) == idLabInfoOptionsOK || int(di.CtlID) == idLabInfoOptionsCancel) {
			br, fg := brushLight, rgb(15, 48, 82)
			if int(di.CtlID) == idLabInfoOptionsOK {
				br, fg = brushHeader, rgb(255, 255, 255)
			}
			if di.ItemState&ODS_SELECTED != 0 {
				br, fg = brushRedPressed, rgb(255, 255, 255)
			}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(br))
			pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
			pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
			pSetTextColor.Call(uintptr(di.HDC), fg)
			old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(getText(di.HwndItem)))), ^uintptr(0), uintptr(unsafe.Pointer(&di.RcItem)), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			return 1
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		if child == labReportOptionsTitle {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		}
		if child == labReportOptionsHeader {
			return uintptr(brushNav)
		}
		if child == labReportOptionsAccent {
			return uintptr(brushHeader)
		}
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idLabInfoOptionsOK:
			for i := range labInfoChecks {
				value, _, _ := pSendMessageW.Call(uintptr(labInfoChecks[i]), BM_GETCHECK, 0, 0)
				labInfoOptionsInclude[i] = value != 0
			}
			labInfoOptionsOK, labInfoOptionsDone = true, true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case idLabInfoOptionsCancel:
			labInfoOptionsOK, labInfoOptionsDone = false, true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010:
		labInfoOptionsOK, labInfoOptionsDone = false, true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func chooseLaboratoryInfographicOptions() bool {
	labInfoOptionsInclude[0] = false
	labInfoOptionsDone, labInfoOptionsOK = false, false
	cls := p("ROSA_LabReportOptionsV2")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(labInfoOptionsWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — дополнительное приложение"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 560, 285, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return false
	}
	var message MSG
	for !labInfoOptionsDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return labInfoOptionsOK
}
