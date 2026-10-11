//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const laboratoryInfographicCanvasClass = "ROSALaboratoryInfographicCanvasV1"

var laboratoryInfographicCanvasRegistered bool

type infographicPaintStruct struct {
	Hdc       syscall.Handle
	Erase     int32
	Paint     RECT
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

// Owner-draw STATIC controls are not reliably repainted after hide/show and
// parent resizing on every supported Windows theme. A real child canvas gives
// each chart a WM_PAINT of its own and keeps the chart visible after redraws.
func laboratoryInfographicCanvasWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case 0x000F: // WM_PAINT
		beginPaint := user32.NewProc("BeginPaint")
		endPaint := user32.NewProc("EndPaint")
		var paint infographicPaintStruct
		dc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		if dc != 0 {
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			draw := DRAWITEMSTRUCT{HwndItem: syscall.Handle(hwnd), HDC: syscall.Handle(dc), RcItem: rc}
			switch syscall.Handle(hwnd) {
			case labInfographicChart:
				drawLaboratoryInfluenceChart(&draw)
			case labInfographicTrend:
				drawLaboratoryTrendChart(&draw)
			case complexChart:
				drawComplexInfographic(&draw, complexSelected)
			}
			endPaint.Call(hwnd, uintptr(unsafe.Pointer(&paint)))
		}
		return 0
	case 0x0014: // WM_ERASEBKGND: WM_PAINT fills the full client area.
		return 1
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func registerLaboratoryInfographicCanvas() {
	if laboratoryInfographicCanvasRegistered {
		return
	}
	cls := p(laboratoryInfographicCanvasClass)
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(laboratoryInfographicCanvasWndProc),
		HInstance:     syscall.Handle(inst),
		HbrBackground: syscall.Handle(6), // COLOR_WINDOW + 1
		LpszClassName: cls,
	}
	if atom, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom != 0 {
		laboratoryInfographicCanvasRegistered = true
	} else {
		// ERROR_CLASS_ALREADY_EXISTS is success when another dialog registered it.
		laboratoryInfographicCanvasRegistered = true
	}
}

func createLaboratoryInfographicCanvas(parent syscall.Handle, id int, x, y, w, h int32, visible bool) syscall.Handle {
	registerLaboratoryInfographicCanvas()
	style := uint32(WS_CHILD | WS_BORDER | WS_CLIPCHILDREN_FRAME)
	if visible {
		style |= WS_VISIBLE
	}
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(p(laboratoryInfographicCanvasClass))), 0, uintptr(style), uintptr(x), uintptr(y), uintptr(w), uintptr(h), uintptr(parent), uintptr(id), 0, 0)
	return syscall.Handle(hwnd)
}
