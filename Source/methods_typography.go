//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var methodsFonts = map[int]syscall.Handle{}
var methodsBandBrush syscall.Handle
var methodsSectionBrushes [5]syscall.Handle
var methodsPanelBrushes [3]syscall.Handle
var pMethodsGetDC = user32.NewProc("GetDC")
var pMethodsReleaseDC = user32.NewProc("ReleaseDC")

// Methods uses one shared four-level type scale. Keep these tokens suitable
// for the other workflow tabs as they are brought to the same visual system.
// Local one-off sizes are deliberately avoided: emphasis is expressed by
// weight, colour and spacing rather than by introducing another font size.
const (
	methodsTypeDisplay = 26
	methodsTypeSection = 19
	methodsTypeBody    = 16
	methodsTypeCompact = 14
)

func methodsFont(height int, bold bool) syscall.Handle {
	key := height
	weight := uintptr(400)
	if bold {
		key += 100
		weight = 600
	}
	if font := methodsFonts[key]; font != 0 {
		return font
	}
	h, _, _ := pCreateFontW.Call(^uintptr(height-1), 0, 0, 0, weight, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
	methodsFonts[key] = syscall.Handle(h)
	return syscall.Handle(h)
}
func deleteMethodsFonts() {
	for _, font := range methodsFonts {
		if font != 0 {
			pDeleteObject.Call(uintptr(font))
		}
	}
	methodsFonts = map[int]syscall.Handle{}
	if methodsBandBrush != 0 {
		pDeleteObject.Call(uintptr(methodsBandBrush))
		methodsBandBrush = 0
	}
	for i, brush := range methodsSectionBrushes {
		if brush != 0 {
			pDeleteObject.Call(uintptr(brush))
			methodsSectionBrushes[i] = 0
		}
	}
	for i, brush := range methodsPanelBrushes {
		if brush != 0 {
			pDeleteObject.Call(uintptr(brush))
			methodsPanelBrushes[i] = 0
		}
	}
}

func methodsSectionAccentBrush(index int) syscall.Handle {
	// RC674 former neutral token retained for static QA: rgb(248, 250, 252)
	// RC674: section backgrounds are deliberately neutral. The previous pale
	// blue bands made the screen visually noisy and competed with the actual
	// status accents. Borders and navy headings keep the hierarchy instead.
	colors := [5]uint32{
		uint32(rgb(255, 255, 255)),
		uint32(rgb(255, 255, 255)),
		uint32(rgb(255, 255, 255)),
		uint32(rgb(255, 255, 255)),
		uint32(rgb(255, 255, 255)),
	}
	if index < 0 || index >= len(methodsSectionBrushes) {
		return brushWhite
	}
	if methodsSectionBrushes[index] == 0 {
		h, _, _ := pCreateSolidBrush.Call(uintptr(colors[index]))
		methodsSectionBrushes[index] = syscall.Handle(h)
	}
	return methodsSectionBrushes[index]
}

func methodsWorkspacePanelBrush(index int) syscall.Handle {
	if index < 0 || index >= len(methodsPanelBrushes) {
		return brushWhite
	}
	if methodsPanelBrushes[index] == 0 {
		h, _, _ := pCreateSolidBrush.Call(uintptr(rgb(255, 255, 255)))
		methodsPanelBrushes[index] = syscall.Handle(h)
	}
	return methodsPanelBrushes[index]
}
func methodsSectionBrush() syscall.Handle {
	if methodsBandBrush == 0 {
		h, _, _ := pCreateSolidBrush.Call(uintptr(rgb(255, 255, 255)))
		methodsBandBrush = syscall.Handle(h)
	}
	return methodsBandBrush
}

// Measure using the actual Windows font and client rectangle, including the
// checkbox glyph. No estimates based on character count, and no owner-draw.
func fitMethodsControlText(handle syscall.Handle, maxHeight int, bold bool, inset int32) {
	if handle == 0 {
		return
	}
	text := getText(handle)
	var rc RECT
	pGetClientRect.Call(uintptr(handle), uintptr(unsafe.Pointer(&rc)))
	if rc.Right <= inset+8 || rc.Bottom < 12 {
		return
	}
	dc, _, _ := pMethodsGetDC.Call(uintptr(handle))
	if dc == 0 {
		return
	}
	defer pMethodsReleaseDC.Call(uintptr(handle), dc)
	chosen := methodsFont(methodsTypeCompact, bold)
	for _, size := range []int{methodsTypeDisplay, methodsTypeSection, methodsTypeBody, methodsTypeCompact} {
		if size > maxHeight {
			continue
		}
		font := methodsFont(size, bold)
		old, _, _ := pSelectObject.Call(dc, uintptr(font))
		measure := RECT{Right: rc.Right - inset - 8}
		pDrawTextW.Call(dc, uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&measure)), DT_CALCRECT|DT_WORDBREAK|DT_NOPREFIX)
		pSelectObject.Call(dc, old)
		chosen = font
		if measure.Bottom <= rc.Bottom-4 {
			break
		}
	}
	const wmGetFont = 0x0031
	current, _, _ := pSendMessageW.Call(uintptr(handle), wmGetFont, 0, 0)
	if current != uintptr(chosen) {
		pSendMessageW.Call(uintptr(handle), WM_SETFONT, uintptr(chosen), 0)
	}
}
func fitMethodsVisibleText() {
	fitMethodsControlText(methodsPageTitle, methodsTypeDisplay, true, 0)
	for _, handle := range []syscall.Handle{methodsObjectTitle, methodsKeyDataTitle, methodsClassTitle, methodsSuggestionTitle, methodsAvailableTitle, methodsBasisTitle, methodsImplementationTitle, methodsForecastTitle, methodsPlanTitle, methodsArchiveTitle} {
		fitMethodsControlText(handle, methodsTypeBody, true, 0)
	}
	for _, handle := range []syscall.Handle{methodsObjectCode, methodsObjectContext, methodsClassSummary, methodsReadyBanner, methodsForecastNote} {
		fitMethodsControlText(handle, methodsTypeBody, false, 0)
	}
	for _, handle := range []syscall.Handle{methodsTechEquipment, methodsTechMRP, methodsTechLiquid, methodsTechWatercut} {
		fitMethodsControlText(handle, methodsTypeSection, true, 0)
	}
	for _, handle := range []syscall.Handle{methodsTechEquipmentValue, methodsTechMRPValue, methodsTechLiquidValue, methodsTechWatercutValue} {
		fitMethodsControlText(handle, methodsTypeSection, true, 0)
	}
	// The result remains the focal point through weight and colour, without a
	// fifth one-off size that makes the page look typographically fragmented.
	for _, handle := range []syscall.Handle{methodsForecastCurrent, methodsForecastGain, methodsForecastResult, methodsForecastBasis} {
		fitMethodsControlText(handle, methodsTypeBody, true, 0)
	}
	for i, card := range methodsCandidateCards {
		fitMethodsControlText(card, methodsTypeBody, true, 24)
		fitMethodsControlText(methodsCandidateDetails[i], methodsTypeCompact, false, 0)
	}
	for _, handle := range []syscall.Handle{methodsSuggestionInfo, methodsImplementation, methodsEmptyState} {
		font := methodsFont(methodsTypeBody, false)
		const wmGetFont = 0x0031
		current, _, _ := pSendMessageW.Call(uintptr(handle), wmGetFont, 0, 0)
		if current != uintptr(font) {
			pSendMessageW.Call(uintptr(handle), WM_SETFONT, uintptr(font), 0)
		}
	}
}

// Shared by multiline parameter and basis panels. Returning zero asks the
// caller to keep the scrollable full-text view if GDI measurement is unavailable.
func methodsMeasuredTextHeight(text string, width int32, size int, bold bool) int32 {
	if width < 20 {
		return 0
	}
	dc, _, _ := pMethodsGetDC.Call(uintptr(hMain))
	if dc == 0 {
		return 0
	}
	defer pMethodsReleaseDC.Call(uintptr(hMain), dc)
	old, _, _ := pSelectObject.Call(dc, uintptr(methodsFont(size, bold)))
	defer pSelectObject.Call(dc, old)
	rc := RECT{Right: width}
	pDrawTextW.Call(dc, uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), DT_CALCRECT|DT_WORDBREAK|DT_NOPREFIX)
	if rc.Right > width {
		return 0
	}
	return rc.Bottom
}
