//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	nnoWMClose       = 0x0010
	idNNOTargetEdit  = 9940
	idNNOTargetSave  = 9941
	idNNOTargetHide  = 9942
	idNNOTargetClose = 9943
	idNNOTargetScale = 9944
)

type nnoTargetState struct {
	Days        float64 `json:"days"`
	Visible     bool    `json:"visible"`
	ExpandScale bool    `json:"expand_scale,omitempty"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

var (
	nnoDestroyTargetWindow = user32.NewProc("DestroyWindow")
	nnoTarget              = nnoTargetState{Days: 1000, Visible: true}
	nnoTargetDialogDone    bool
	nnoTargetEdit          syscall.Handle
)

func nnoLoadTarget() {
	data, err := os.ReadFile(sharedPath("nno_target.json"))
	if err != nil {
		return
	}
	var saved nnoTargetState
	if json.Unmarshal(data, &saved) == nil && saved.Days > 0 && saved.Days <= 1000000 {
		nnoTarget = saved
	}
}

func nnoSaveTarget() error {
	data, err := json.MarshalIndent(nnoTarget, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sharedPath("nno_target.json"), data, 0600)
}

func nnoTargetProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		modalCreate("STATIC", "Ориентир аналитический: он не меняет выборку и расчёт ННО-12.", WS_CHILD|WS_VISIBLE, 20, 20, 450, 34, hwnd, 0)
		modalCreate("STATIC", "Ориентир наработки, сут. (> 0):", WS_CHILD|WS_VISIBLE, 20, 70, 300, 24, hwnd, 0)
		nnoTargetEdit = modalCreate("EDIT", fmt.Sprintf("%.1f", nnoTarget.Days), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 20, 100, 170, 32, hwnd, idNNOTargetEdit)
		modalCreate("BUTTON", "Сохранить и показать", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 20, 155, 190, 38, hwnd, idNNOTargetSave)
		modalCreate("BUTTON", "Скрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 220, 155, 110, 38, hwnd, idNNOTargetHide)
		modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 340, 155, 110, 38, hwnd, idNNOTargetClose)
		modalCreate("BUTTON", "Показать ориентир на шкале / вернуть масштаб", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 20, 204, 430, 30, hwnd, idNNOTargetScale)
		return 0
	case WM_COMMAND:
		if hiWord(wParam) != 0 {
			break
		}
		switch int(loWord(wParam)) {
		case idNNOTargetSave:
			days, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(getText(nnoTargetEdit)), ",", "."), 64)
			if err != nil || days <= 0 || days > 1000000 {
				msg("Ориентир наработки", "Введите положительное значение в сутках.", MB_OK|MB_ICONWARNING)
				return 0
			}
			nnoTarget.Days, nnoTarget.Visible = days, true
		case idNNOTargetHide:
			nnoTarget.Visible = false
		case idNNOTargetScale:
			nnoTarget.ExpandScale = !nnoTarget.ExpandScale
		case idNNOTargetClose:
			nnoTargetDialogDone = true
			nnoDestroyTargetWindow.Call(uintptr(hwnd))
			return 0
		default:
			break
		}
		nnoTarget.UpdatedAt = time.Now().Format(time.RFC3339)
		if err := nnoSaveTarget(); err != nil {
			msg("Ориентир наработки", err.Error(), MB_OK|MB_ICONERROR)
		}
		page := nnoFleetEventPage
		refreshNNOFleetUI()
		nnoFleetEventPage = page
		nnoRenderEventPage()
		pInvalidateRect.Call(uintptr(nnoFleetChart), 0, 0)
		pInvalidateRect.Call(uintptr(nnoFleetDistribution), 0, 0)
		return 0
	case nnoWMClose:
		nnoTargetDialogDone = true
		nnoDestroyTargetWindow.Call(uintptr(hwnd))
		return 0
	}
	result, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return result
}

func nnoOpenTarget() {
	nnoTargetDialogDone = false
	cls := p("ROSANNOTargetV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(nnoTargetProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0x00000080, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — ориентир наработки"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 485, 300, uintptr(hMain), 0, inst, 0)
	if hwnd != 0 {
		var m MSG
		for !nnoTargetDialogDone {
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
}
