//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Presentation-only background geometry. Drawn onto the parent HDC, never
// as child HWNDs, so no panel can hide an editable field or a list view.
const econ813IDStatusFirst = 15120

var econ813Panels [4]RECT
var econ813RoundRect = gdi32.NewProc("RoundRect")

func paintEconomics813Parent(dc syscall.Handle) bool {
	if dc == 0 {
		return false
	}
	var rc RECT
	if ret, _, _ := pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&rc))); ret == 0 {
		return false
	}
	pFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&rc)), uintptr(brushLight))
	for i, b := range econ813Panels {
		if b.Right <= b.Left || b.Bottom <= b.Top {
			continue
		}
		econ814RoundedRect(dc, b, rgb(255, 255, 255), rgb(215, 223, 231), 14)
		// Subtle separator under each panel's title/toolbar.
		head := RECT{Left: b.Left + 1, Top: b.Top + 42, Right: b.Right - 1, Bottom: b.Top + 43}
		if i == 3 {
			head.Top, head.Bottom = b.Top+29, b.Top+30
		} else if i == 2 {
			head.Top, head.Bottom = b.Top+27, b.Top+28
		}
		pFillRect.Call(uintptr(dc), uintptr(unsafe.Pointer(&head)), uintptr(brushBorder))
	}
	return true
}

func paintEconomics813Status(di *DRAWITEMSTRUCT) bool {
	if di == nil || di.CtlID < econ813IDStatusFirst || di.CtlID >= econ813IDStatusFirst+5 {
		return false
	}
	b := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&b)), uintptr(brushWhite))
	value := getText(di.HwndItem)
	bg := rgb(232, 245, 239)
	fg := rgb(34, 113, 84)
	if value == "Допущение" {
		bg = rgb(255, 248, 226)
		fg = rgb(140, 99, 34)
	}
	if value == "Нет данных" {
		bg = rgb(241, 244, 247)
		fg = rgb(100, 116, 130)
	}
	brush, _, _ := pCreateSolidBrush.Call(bg)
	if brush != 0 {
		badge := RECT{Left: b.Left, Top: b.Top + 2, Right: b.Right - 4, Bottom: b.Bottom - 2}
		if badge.Bottom > badge.Top {
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&badge)), brush)
		}
		pDeleteObject.Call(brush)
	}
	r := RECT{Left: b.Left + 5, Top: b.Top, Right: b.Right - 5, Bottom: b.Bottom}
	economicsMetricText(di.HDC, value, r, rosaUIFont(rosaUICaption), fg)
	return true
}
