//go:build windows

package main

import "syscall"

var methodsWorkspacePanels [3]syscall.Handle
var methodsClassificationChips [6]syscall.Handle
var methodsContextBand syscall.Handle
var methodsContextHeaderBand syscall.Handle
var methodsClassificationBand syscall.Handle
var methodsActionBand syscall.Handle
var methodsForecastBand syscall.Handle
var methodsArchiveBand syscall.Handle
var methodsTechMetricFrames [4]syscall.Handle

// Native backgrounds precede the page's content in sibling order. They frame
// three complete workspace columns without owner-draw or extra paint hooks.
func buildMethodsWorkspacePanels() {
	// RC650: the reference is built from five clearly separated horizontal
	// bands. Native frames are created first so they remain behind every label,
	// button and list and do not introduce a custom paint path.
	for _, slot := range []*syscall.Handle{
		&methodsContextBand,
		&methodsClassificationBand,
		&methodsActionBand,
		&methodsForecastBand,
		&methodsArchiveBand,
	} {
		// RC676: these are background separators, not cards. Their outer WS_BORDER
		// duplicated the borders of the real controls and produced stray horizontal
		// rectangles at section transitions. Keep them borderless.
		*slot = create("STATIC", "", WS_CHILD, 242, py(132), 1010, 64, hMain, 0)
		addPageControl(ID_ACTIVITIES, *slot)
		show(*slot, false)
	}
	methodsContextHeaderBand = create("STATIC", "", WS_CHILD, 242, py(132), 1010, 22, hMain, 0)
	addPageControl(ID_ACTIVITIES, methodsContextHeaderBand)
	show(methodsContextHeaderBand, false)
	for i := range methodsTechMetricFrames {
		methodsTechMetricFrames[i] = create("STATIC", "", WS_CHILD|WS_BORDER, 980, py(160), 126, 42, hMain, 0)
		addPageControl(ID_ACTIVITIES, methodsTechMetricFrames[i])
		show(methodsTechMetricFrames[i], false)
	}
	for i := 0; i < 3; i++ {
		methodsWorkspacePanels[i] = create("STATIC", "", WS_CHILD|WS_BORDER, 242, py(474), 300, 224, hMain, 0)
		addPageControl(ID_ACTIVITIES, methodsWorkspacePanels[i])
		show(methodsWorkspacePanels[i], false)
	}
	for i := 0; i < 6; i++ {
		methodsClassificationChips[i] = create("STATIC", "", WS_CHILD|WS_BORDER|SS_CENTER|SS_NOPREFIX|0x00000200, 242, py(296), 200, 36, hMain, 0) // SS_CENTERIMAGE
		addPageControl(ID_ACTIVITIES, methodsClassificationChips[i])
		show(methodsClassificationChips[i], false)
	}
}

func layoutMethodsSectionBands(x, contentW, contextY, contextH, classY, classH, actionY, actionH, forecastY, forecastH, archiveY, archiveH int32) {
	items := []struct {
		h  syscall.Handle
		y  int32
		hh int32
	}{
		{methodsContextBand, contextY, contextH},
		{methodsClassificationBand, classY, classH},
		{methodsActionBand, actionY, actionH},
		{methodsForecastBand, forecastY, forecastH},
		{methodsArchiveBand, archiveY, archiveH},
	}
	for _, item := range items {
		setPos(item.h, x, item.y, contentW, max32(1, item.hh))
		show(item.h, true)
	}
}
func layoutMethodsWorkspacePanels(x, midX, rightX, y, leftW, midW, rightW, h int32) {
	xs := []int32{x, midX, rightX}
	widths := []int32{leftW, midW, rightW}
	for i, handle := range methodsWorkspacePanels {
		setPos(handle, xs[i], y, widths[i], h)
		show(handle, true)
	}
}

// layoutMethodsWorkspacePanelsStacked is the RC640 workspace geometry: one
// large selection card at left and two visibly independent decision cards at
// right. The same native panel handles are reused, so no extra paint path is
// introduced and DPI behaviour stays identical to the rest of the page.
func layoutMethodsWorkspacePanelsStacked(x, rightX, y, leftW, rightW, leftH, rightTopH, rightBottomY, rightBottomH int32) {
	setPos(methodsWorkspacePanels[0], x, y, leftW, leftH)
	setPos(methodsWorkspacePanels[1], rightX, y, rightW, rightTopH)
	setPos(methodsWorkspacePanels[2], rightX, rightBottomY, rightW, rightBottomH)
	for _, handle := range methodsWorkspacePanels {
		show(handle, true)
	}
}

func layoutMethodsClassChips(x, y, w, h int32) {
	_, ready, rows := approvedAssessmentForMethods(selectedObjectKey)
	visible := ready && !methodsBackgroundBusy && len(rows) > 0 && len(rows) <= len(methodsClassificationChips)
	show(methodsClassSummary, !visible)
	chipW := int32(0)
	if visible {
		chipW = (w - int32(len(rows)-1)*8) / int32(len(rows))
	}
	for i, handle := range methodsClassificationChips {
		shown := visible && i < len(rows)
		show(handle, shown)
		if !shown {
			continue
		}
		setPos(handle, x+int32(i)*(chipW+8), y, chipW, h)
		setText(handle, rows[i].Direction+"  ·  "+rows[i].FinalClass)
		fitMethodsControlText(handle, methodsTypeSection, true, 0)
	}
}
