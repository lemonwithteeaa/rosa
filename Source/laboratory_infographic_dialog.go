//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	idLabInfographicScope   = 7381
	idLabInfographicMetric  = 7382
	idLabInfographicChart   = 7383
	idLabInfographicTrend   = 7384
	idLabInfographicExport  = 7386
	idLabInfographicExcel   = 7387
	infographicSWPNoZOrder  = 0x0004
	infographicWSThickFrame = 0x00040000
	infographicWSMaximize   = 0x00010000
	infographicWSMinimize   = 0x00020000
)

var (
	labInfographicDialogDone      bool
	labInfographicScopeCombo      syscall.Handle
	labInfographicMetricCombo     syscall.Handle
	labInfographicMetricLabel     syscall.Handle
	labInfographicChart           syscall.Handle
	labInfographicTrend           syscall.Handle
	labInfographicExcelButton     syscall.Handle
	labInfographicExportButton    syscall.Handle
	labInfographicTitle           syscall.Handle
	labInfographicObjectInfo      syscall.Handle
	labInfographicExportRequested bool
	labInfographicExcelRequested  bool
)

func refreshLaboratoryInfographicMetricLabel() {
	if labInfographicMetricLabel == 0 || labInfographicMetricCombo == 0 {
		return
	}
	metric := laboratoryTrendMetricByIndex(laboratoryComboSelection(labInfographicMetricCombo))
	caption := "Показатель: " + metric.Label
	object := strings.TrimSpace(getText(labObjectEdit))
	active := ensureActiveLabPass()
	active.ObjectKey = object
	active.Groups = cloneLabGroups(labDrafts)
	active.Units = cloneLabGroups(labUnitDrafts)
	if pass, ok := laboratoryPersonalInfluencePass(object, labPasses, active); ok {
		if value, found := laboratoryTrendValue(pass, metric); found {
			caption = fmt.Sprintf("%s: %.4g %s", metric.Label, value, metric.Unit)
		}
	}
	setText(labInfographicMetricLabel, caption)
}

func laboratoryComboSelection(h syscall.Handle) int {
	if h == 0 {
		return 0
	}
	idx, _, _ := pSendMessageW.Call(uintptr(h), CB_GETCURSEL, 0, 0)
	if int32(idx) < 0 {
		return 0
	}
	return int(idx)
}

// Keep the summary panel tall enough for its own rows and make use of two
// columns on ordinary laptop/desktop widths. The old 1-column override below
// 1450 px squeezed six rows into a 302 px panel, then pushed the detail chart
// into the same vertical space on common 1366 px displays.
func laboratoryInfographicLiveLayout(width, height int32) (chartHeight, trendTop, trendHeight int32) {
	if width < 760 {
		width = 760
	}
	columns := laboratoryInfluenceChartColumns(width - 48)
	rows := (6 + columns - 1) / columns
	chartHeight = 8 + rows*laboratoryInfluenceRowHeight(columns, 6) + 27 + 48 + 8
	if chartHeight < 220 {
		chartHeight = 220
	}
	trendTop = 130 + chartHeight + 10
	trendHeight = height - trendTop - 32
	if trendHeight < 180 {
		trendHeight = 180
	}
	return
}

func laboratoryInfographicDialogWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		labInfographicTitle = modalCreate("STATIC", "Инфографика осложнений", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 14, 470, 38, parent, 0)
		if labInfographicTitle != 0 && hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(labInfographicTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		objectText := strings.TrimSpace(getText(labObjectEdit))
		if objectText == "" {
			objectText = strings.TrimSpace(selectedObjectKey)
		}
		if objectText == "" {
			objectText = "объект не выбран"
		}
		// Object keys may arrive from the technical regime as a compact
		// delimiter-free string. The infographic must show the readable form,
		// otherwise the object name visually runs into the next field.
		objectText = readableObjectKey(objectText)
		labInfographicObjectInfo = modalCreate("STATIC", fmt.Sprintf("Объект: %s", objectText), WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 52, 1070, 28, parent, 0)
		if labInfographicObjectInfo != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(labInfographicObjectInfo), WM_SETFONT, uintptr(hFontBold), 1)
		}

		labInfographicMetricLabel = modalCreate("STATIC", "Показатель", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 86, 250, 26, parent, 0)
		labInfographicMetricCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 282, 82, 340, 240, parent, idLabInfographicMetric)
		for _, metric := range laboratoryTrendMetrics {
			pSendMessageW.Call(uintptr(labInfographicMetricCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(metric.Label))))
		}
		pSendMessageW.Call(uintptr(labInfographicMetricCombo), CB_SETCURSEL, uintptr(laboratoryComboSelection(labTrendMetricCombo)), 0)
		refreshLaboratoryInfographicMetricLabel()

		labInfographicChart = createLaboratoryInfographicCanvas(parent, idLabInfographicChart, 24, 120, 1110, 310, true)
		labInfographicTrend = createLaboratoryInfographicCanvas(parent, idLabInfographicTrend, 24, 440, 1110, 230, true)
		labInfographicExportButton = modalCreate("BUTTON", "Выгрузить DOCX / Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 624, 16, 224, 36, parent, idLabInfographicExport)
		labInfographicExcelButton = modalCreate("BUTTON", "Графики и данные в Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 856, 16, 238, 36, parent, idLabInfographicExcel)
		return 0

	case WM_COMMAND:
		id := int(loWord(wParam))
		code := hiWord(wParam)
		if id == idLabInfographicScope && code == 1 {
			idx := laboratoryComboSelection(labInfographicScopeCombo)
			labOverviewScopeCode = laboratoryScopeCodeByIndex(idx)
			if labScopeCombo != 0 {
				pSendMessageW.Call(uintptr(labScopeCombo), CB_SETCURSEL, uintptr(idx), 0)
			}
			laboratoryRefreshDetailObjects()
			pInvalidateRect.Call(uintptr(labInfographicChart), 0, 0)
			pInvalidateRect.Call(uintptr(labInfographicTrend), 0, 0)
			return 0
		}
		if id == idLabInfographicMetric && code == 1 {
			idx := laboratoryComboSelection(labInfographicMetricCombo)
			if labTrendMetricCombo != 0 {
				pSendMessageW.Call(uintptr(labTrendMetricCombo), CB_SETCURSEL, uintptr(idx), 0)
			}
			pInvalidateRect.Call(uintptr(labInfographicTrend), 0, 0)
			pInvalidateRect.Call(uintptr(labInfographicChart), 0, 0)
			refreshLaboratoryInfographicMetricLabel()
			return 0
		}
		if id == idLabInfographicExport {
			// Export opens a Windows Save dialog. The main window is disabled while
			// this modal is active, so opening it here can strand the dialog behind
			// the infographic and leave ROSA looking unresponsive.
			labInfographicExportRequested = true
			labInfographicDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
		if id == idLabInfographicExcel {
			// Close the modal first, then create the workbook in the application's
			// export cache and launch the user's Excel handler immediately.
			labInfographicExcelRequested = true
			labInfographicDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case WM_SIZE:
		// Use the actual client rect: packed WM_SIZE dimensions truncate to
		// 16-bit fields and can be stale during initial creation/maximize.
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		w, h := rc.Right-rc.Left, rc.Bottom-rc.Top
		if w > 0 && h > 0 {
			chartW := max32(320, w-48)
			chartH, trendY, trendH := laboratoryInfographicLiveLayout(w, h)
			pSetWindowPos.Call(uintptr(labInfographicChart), 0, 24, 120, uintptr(chartW), uintptr(chartH), infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicTrend), 0, 24, uintptr(trendY), uintptr(chartW), uintptr(trendH), infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicMetricCombo), 0, 282, 82, 340, 240, infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicMetricLabel), 0, 24, 86, 250, 26, infographicSWPNoZOrder)
			// Fit actions into a reserved header column so they never cover the title.
			gap := int32(6)
			usable := max32(1, w-48)
			exportW, excelW := int32(224), int32(238)
			buttonsW := exportW + excelW + gap
			if usable < buttonsW+250 {
				exportW, excelW = 160, 170
				buttonsW = exportW + excelW + gap
			}
			buttonsX := max32(24+250+gap, w-24-buttonsW)
			titleW := max32(220, buttonsX-24-gap)
			pSetWindowPos.Call(uintptr(labInfographicTitle), 0, 24, 14, uintptr(titleW), 34, infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicObjectInfo), 0, 24, 52, uintptr(usable), 28, infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicExportButton), 0, uintptr(buttonsX), 16, uintptr(exportW), 36, infographicSWPNoZOrder)
			pSetWindowPos.Call(uintptr(labInfographicExcelButton), 0, uintptr(buttonsX+exportW+gap), 16, uintptr(excelW), 36, infographicSWPNoZOrder)
			pInvalidateRect.Call(uintptr(labInfographicChart), 0, 0)
			pInvalidateRect.Call(uintptr(labInfographicTrend), 0, 0)
		}

	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil {
			switch int(di.CtlID) {
			case idLabInfographicChart:
				drawLaboratoryInfluenceChart(di)
				return 1
			case idLabInfographicTrend:
				drawLaboratoryTrendChart(di)
				return 1
			}
		}

	case 0x0010: // WM_CLOSE from the standard title-bar close control
		labInfographicDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openLaboratoryInfographicDialog() {
	// This dialog belongs to one pinned object. Cross-object selection is
	// handled by the separate complex-analysis screen.
	labOverviewScopeCode = laboratoryScopeObject
	laboratoryRefreshDetailObjects()
	labInfographicDialogDone = false
	labInfographicExportRequested = false
	labInfographicExcelRequested = false
	labInfographicScopeCombo = 0
	labInfographicMetricCombo = 0
	labInfographicMetricLabel = 0
	labInfographicChart = 0
	labInfographicTrend = 0
	labInfographicExcelButton = 0
	labInfographicExportButton = 0

	cls := p("ROSALaboratoryInfographicDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(laboratoryInfographicDialogWndProc),
		HInstance:     syscall.Handle(hInstRaw),
		HbrBackground: syscall.Handle(5 + 1),
		LpszClassName: cls,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(p("РОСА — инфографика осложнений"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN_FRAME,
		0x80000000, 0x80000000,
		1180, 720,
		uintptr(hMain), 0, hInstRaw, 0,
	)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Инфографика осложнений", "Не удалось открыть окно инфографики.", MB_OK|MB_ICONWARNING)
		return
	}

	var message MSG
	for !labInfographicDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	if labInfographicExportRequested {
		labInfographicExportRequested = false
		exportLaboratoryReport()
	}
	if labInfographicExcelRequested {
		labInfographicExcelRequested = false
		exportLaboratoryInfographicExcel()
	}
}
