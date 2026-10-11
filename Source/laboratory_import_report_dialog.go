//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

const (
	laboratoryImportReportTransfer = 9941
)

var (
	laboratoryImportReportDone  bool
	laboratoryImportReportTitle string
	laboratoryImportReportBody  string
)

// laboratoryImportReportWndProc uses a real scrollable report window instead
// of MessageBox. Long imports therefore show every rejected row and its reason.
// The transfer button is intentionally explicit: it brings the imported pass
// into the active laboratory screen and closes the report.
func laboratoryImportReportWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		title := modalCreate("STATIC", laboratoryImportReportTitle, WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 18, 1040, 34, parent, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		_ = riched20.Load()
		// Keep the report and both actions inside the client area even with the
		// larger system font and a 125–150% Windows scale factor.
		edit := modalCreate("RichEdit20W", laboratoryImportReportBody, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 24, 62, 1040, 664, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(edit), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		formatFailureReportText(edit, laboratoryImportReportBody)
		modalCreate("BUTTON", "Перенести в программу", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 24, 730, 300, 46, parent, laboratoryImportReportTransfer)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case laboratoryImportReportTransfer:
			transferred := false
			if strings.TrimSpace(lastLaboratoryImportPassID) != "" {
				for _, pass := range labPasses {
					if pass.PassID != lastLaboratoryImportPassID {
						continue
					}
					if strings.TrimSpace(getText(labObjectEdit)) != "" {
						saveLabDraft()
						if strings.EqualFold(strings.TrimSpace(getText(labObjectEdit)), strings.TrimSpace(selectedObjectKey)) {
							labPinnedDraftID = labActivePassID
						}
					}
					openLaboratoryPass(pass, "")
					transferred = strings.TrimSpace(labActivePassID) != "" &&
						strings.EqualFold(strings.TrimSpace(getText(labObjectEdit)), strings.TrimSpace(pass.ObjectKey))
					break
				}
			}
			if !transferred {
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p("Перенос не выполнен: в отчёте нет сохранённого анализа для открытия. Проверьте список незагруженных строк и исправьте бланк."))), uintptr(unsafe.Pointer(p("Лабораторные данные"))), MB_OK|MB_ICONWARNING)
				return 0
			}
			refreshLaboratory()
			laboratoryImportReportDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		laboratoryImportReportDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openLaboratoryImportReportDialog(title, body string) {
	laboratoryImportReportTitle = title
	laboratoryImportReportBody = body
	laboratoryImportReportDone = false
	cls := p("ROSALaboratoryImportReportDialogV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(laboratoryImportReportWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	// Tool window stays owned by the main application and does not show a
	// second ROSA icon in the Windows taskbar while the import report is open.
	// Height includes the title bar and borders. The previous 780 px window
	// clipped the transfer button on common DPI settings.
	hwnd, _, _ := pCreateWindowExW.Call(0x00000080, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(title))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 1090, 850, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg(title, body, MB_OK|MB_ICONINFORMATION)
		return
	}
	var message MSG
	for !laboratoryImportReportDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &message) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
