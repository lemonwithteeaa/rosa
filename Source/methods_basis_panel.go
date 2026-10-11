//go:build windows

package main

import (
	"strings"
	"syscall"
	"time"
)

var methodsBasisNumbers [5]syscall.Handle
var methodsBasisItems [5]syscall.Handle
var methodsCandidateDraftSelected [6]bool

func buildMethodsBasisPanel() {
	for i := 0; i < 5; i++ {
		methodsBasisNumbers[i] = create("STATIC", []string{"①", "②", "③", "④", "⑤"}[i], WS_CHILD|SS_CENTER|SS_NOPREFIX, 776, py(502), 30, 30, hMain, 0)
		methodsBasisItems[i] = create("STATIC", "", WS_CHILD|SS_LEFT|SS_NOPREFIX, 810, py(502), 200, 38, hMain, 0)
		addPageControl(ID_ACTIVITIES, methodsBasisNumbers[i])
		addPageControl(ID_ACTIVITIES, methodsBasisItems[i])
		pSendMessageW.Call(uintptr(methodsBasisNumbers[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeSection, true)), 0)
		show(methodsBasisNumbers[i], false)
		show(methodsBasisItems[i], false)
	}
}
func hideMethodsBasisPanel() {
	for i := 0; i < 5; i++ {
		show(methodsBasisNumbers[i], false)
		show(methodsBasisItems[i], false)
	}
}
func layoutMethodsBasisPanel(x, y, w, h int32) {
	s, ok := methodsSelectedSuggestion()
	if !ok || methodsBackgroundBusy {
		hideMethodsBasisPanel()
		show(methodsSuggestionInfo, true)
		return
	}
	finish := beginMethodsEvaluation()
	defer finish()
	grade := methodApplicabilityForSuggestion(s, selectedObjectKey, time.Now())
	base := methodForecastSnapshotForSelectedObject().Base
	parts := strings.Split(methodsBasisSummaryText(s, grade, ReplicationAssessment{}, base), "\r\n\r\n")
	if len(parts) != 5 {
		hideMethodsBasisPanel()
		show(methodsSuggestionInfo, true)
		return
	}
	rowH := (h - 16) / 5
	size := methodsTypeBody
	fits := false
	for ; size >= methodsTypeCompact; size-- {
		fits = true
		for _, part := range parts {
			text := methodsBasisItemText(part)
			measured := methodsMeasuredTextHeight(text, w-34, size, false)
			if measured <= 0 || measured > rowH-6 {
				fits = false
				break
			}
		}
		if fits {
			break
		}
	}
	if !fits {
		hideMethodsBasisPanel()
		show(methodsSuggestionInfo, true)
		return
	}
	show(methodsSuggestionInfo, false)
	for i, part := range parts {
		rowY := y + int32(i)*(rowH+4)
		setPos(methodsBasisNumbers[i], x, rowY, 22, min32(26, rowH))
		setPos(methodsBasisItems[i], x+28, rowY+3, w-28, rowH-3)
		setText(methodsBasisItems[i], methodsBasisItemText(part))
		pSendMessageW.Call(uintptr(methodsBasisItems[i]), WM_SETFONT, uintptr(methodsFont(size, false)), 0)
		show(methodsBasisNumbers[i], true)
		show(methodsBasisItems[i], true)
	}
}
func methodsBasisItemText(text string) string {
	if _, rest, ok := strings.Cut(text, ". "); ok {
		return rest
	}
	return text
}
