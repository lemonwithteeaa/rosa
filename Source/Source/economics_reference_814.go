//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

// Section-6 presentation only: no data writes or child background windows.
var econ814GreenBrush syscall.Handle

func econ814PositiveBrush() syscall.Handle {
	if econ814GreenBrush == 0 {
		h, _, _ := pCreateSolidBrush.Call(rgb(46, 139, 100))
		econ814GreenBrush = syscall.Handle(h)
	}
	if econ814GreenBrush == 0 {
		return brushNav
	}
	return econ814GreenBrush
}
func econ814RoundedRect(dc syscall.Handle, b RECT, bg, line uintptr, radius int32) {
	if b.Right <= b.Left || b.Bottom <= b.Top {
		return
	}
	brush, _, _ := pCreateSolidBrush.Call(bg)
	pen, _, _ := pCreatePen.Call(0, 1, line)
	if brush == 0 || pen == 0 {
		if brush != 0 {
			pDeleteObject.Call(brush)
		}
		if pen != 0 {
			pDeleteObject.Call(pen)
		}
		pFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&b)), uintptr(brushWhite))
		return
	}
	oldBrush, _, _ := pSelectObject.Call(uintptr(dc), brush)
	oldPen, _, _ := pSelectObject.Call(uintptr(dc), pen)
	econ813RoundRect.Call(uintptr(dc), uintptr(b.Left), uintptr(b.Top), uintptr(b.Right), uintptr(b.Bottom), uintptr(radius), uintptr(radius))
	pSelectObject.Call(uintptr(dc), oldPen)
	pSelectObject.Call(uintptr(dc), oldBrush)
	pDeleteObject.Call(pen)
	pDeleteObject.Call(brush)
}
func econ814Badge(dc syscall.Handle, b RECT, status string) {
	if b.Right-b.Left < 35 || b.Bottom-b.Top < 13 {
		return
	}
	bg, fg := rgb(242, 245, 249), rgb(94, 111, 127)
	if status == "Расчёт актуален" {
		bg, fg = rgb(233, 246, 239), rgb(39, 117, 82)
	}
	if status == "Не рассчитано" || status == "На пересчёт" {
		bg, fg = rgb(255, 244, 225), rgb(147, 101, 35)
	}
	econ814RoundedRect(dc, b, bg, bg, 10)
	r := RECT{Left: b.Left + 6, Top: b.Top, Right: b.Right - 4, Bottom: b.Bottom}
	economicsMetricText(dc, status, r, rosaUIFont(rosaUICaption), fg)
}
func economics814HasCurrentResult() bool {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return false
	}
	s, ok := activeAlternativeScenario(selectedObjectKey)
	return ok && economicsScenarioCalculated(s)
}
