//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	nnoZoomChooseRolling      = 9970
	nnoZoomChooseDistribution = 9971
	nnoZoomDecrease           = 9972
	nnoZoomIncrease           = 9973
	nnoZoomDefault            = 9974
	nnoZoomClose              = 9975
)

var (
	nnoZoomDialogDone         bool
	nnoZoomDialogCaption      syscall.Handle
	nnoZoomSelectedCaption    syscall.Handle
	nnoZoomDialogProcCallback uintptr
)

func nnoZoomUpdateDialog() {
	name := "Динамика наработки"
	if nnoZoomActiveChart == 1 {
		name = "Распределение по способам"
	}
	if nnoZoomSelectedCaption != 0 {
		setText(nnoZoomSelectedCaption, fmt.Sprintf("Выбран: %s · масштаб %.0f%%", name, nnoCurrentZoom()*100))
	}
	if nnoZoomDialogCaption != 0 {
		setText(nnoZoomDialogCaption, fmt.Sprintf("Динамика: %.0f%%     Распределение: %.0f%%", nnoFleetRollingZoom*100, nnoFleetDistributionZoom*100))
	}
	nnoZoomRepaint()
}

func nnoZoomDialogProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		modalCreate("STATIC", "УПРАВЛЕНИЕ МАСШТАБОМ", WS_CHILD|WS_VISIBLE, 20, 18, 460, 28, hwnd, 0)
		modalCreate("STATIC", "Выберите инфографику. Изменения не влияют на расчёты.", WS_CHILD|WS_VISIBLE, 20, 52, 470, 24, hwnd, 0)
		modalCreate("BUTTON", "Динамика наработки", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 20, 85, 225, 40, hwnd, nnoZoomChooseRolling)
		modalCreate("BUTTON", "Распределение по способам", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 252, 85, 235, 40, hwnd, nnoZoomChooseDistribution)
		nnoZoomSelectedCaption = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 20, 139, 470, 27, hwnd, 0)
		modalCreate("BUTTON", "−", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 20, 174, 110, 40, hwnd, nnoZoomDecrease)
		modalCreate("BUTTON", "+", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 141, 174, 110, 40, hwnd, nnoZoomIncrease)
		modalCreate("BUTTON", "Сбросить 100%", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 264, 174, 223, 40, hwnd, nnoZoomDefault)
		nnoZoomDialogCaption = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 20, 232, 470, 27, hwnd, 0)
		modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 337, 271, 150, 39, hwnd, nnoZoomClose)
		nnoZoomUpdateDialog()
		return 0
	case WM_COMMAND:
		if hiWord(wParam) != 0 {
			break
		}
		switch int(loWord(wParam)) {
		case nnoZoomChooseRolling:
			nnoZoomActiveChart = 0
		case nnoZoomChooseDistribution:
			nnoZoomActiveChart = 1
		case nnoZoomDecrease:
			nnoSetChartZoom(nnoZoomActiveChart, nnoCurrentZoom()/1.25)
		case nnoZoomIncrease:
			nnoSetChartZoom(nnoZoomActiveChart, nnoCurrentZoom()*1.25)
		case nnoZoomDefault:
			nnoSetChartZoom(nnoZoomActiveChart, 1)
		case nnoZoomClose:
			nnoZoomDialogDone = true
			nnoDestroyTargetWindow.Call(uintptr(hwnd))
			return 0
		default:
			break
		}
		nnoZoomUpdateDialog()
		return 0
	case nnoWMClose:
		nnoZoomDialogDone = true
		nnoDestroyTargetWindow.Call(uintptr(hwnd))
		return 0
	}
	result, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return result
}

func nnoOpenZoomDialog() {
	nnoZoomDialogDone = false
	nnoZoomDialogCaption = 0
	nnoZoomSelectedCaption = 0
	name := p("ROSANNOZoomDialog819")
	inst, _, _ := pGetModuleHandleW.Call(0)
	if nnoZoomDialogProcCallback == 0 {
		nnoZoomDialogProcCallback = syscall.NewCallback(nnoZoomDialogProc)
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: nnoZoomDialogProcCallback, HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: name}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0x00000080, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p("РОСА — масштаб графиков"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 525, 370, uintptr(hMain), 0, inst, 0)
	if hwnd != 0 {
		var m MSG
		for !nnoZoomDialogDone {
			result, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(result) <= 0 {
				break
			}
			if handleModalEscape(hwnd, &m) {
				continue
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	nnoZoomRepaint()
}
