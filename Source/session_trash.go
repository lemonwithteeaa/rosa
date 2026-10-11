//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_SESSION_TRASH = 5275
	idTrashList      = 7471
	idTrashRestore   = 7472
	idTrashClear     = 7473
	idTrashClose     = 7474
)

type sessionTrashItem struct {
	Title     string
	Details   string
	DeletedAt time.Time
	Restore   func() error
	Purge     func()
}

var (
	sessionTrashItems  []sessionTrashItem
	sessionTrashButton syscall.Handle
	trashDialogDone    bool
	trashList          syscall.Handle
	trashDetails       syscall.Handle
)

func updateSessionTrashCaption() {
	if sessionTrashButton != 0 {
		setText(sessionTrashButton, fmt.Sprintf("Корзина · %d", len(sessionTrashItems)))
	}
}

func addSessionTrashItem(title, details string, restore func() error, purge func()) {
	sessionTrashItems = append(sessionTrashItems, sessionTrashItem{
		Title: title, Details: details, DeletedAt: time.Now(), Restore: restore, Purge: purge,
	})
	updateSessionTrashCaption()
}

func finishSessionTrash() {
	for _, item := range sessionTrashItems {
		if item.Purge != nil {
			item.Purge()
		}
	}
	sessionTrashItems = nil
	updateSessionTrashCaption()
}

func refreshSessionTrashDialog() {
	if trashList == 0 {
		return
	}
	pSendMessageW.Call(uintptr(trashList), LB_RESETCONTENT, 0, 0)
	for i := len(sessionTrashItems) - 1; i >= 0; i-- {
		item := sessionTrashItems[i]
		line := fmt.Sprintf("%s · %s", item.DeletedAt.Format("15:04:05"), item.Title)
		pSendMessageW.Call(uintptr(trashList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(line))))
	}
	if len(sessionTrashItems) == 0 {
		setText(trashDetails, "Корзина текущей сессии пуста. После выхода из профиля восстановление удалённых данных недоступно.")
		return
	}
	pSendMessageW.Call(uintptr(trashList), 0x0186, 0, 0) // LB_SETCURSEL
	showSelectedTrashDetails()
}

func selectedTrashIndex() int {
	selection, _, _ := pSendMessageW.Call(uintptr(trashList), 0x0188, 0, 0) // LB_GETCURSEL
	if int32(selection) < 0 {
		return -1
	}
	return len(sessionTrashItems) - 1 - int(selection)
}

func showSelectedTrashDetails() {
	index := selectedTrashIndex()
	if index < 0 || index >= len(sessionTrashItems) {
		return
	}
	item := sessionTrashItems[index]
	setText(trashDetails, item.Title+"\r\n\r\n"+item.Details+"\r\n\r\nУдалено в "+item.DeletedAt.Format("15:04:05")+". Доступно для восстановления только до завершения текущей сессии.")
}

func restoreSelectedTrashItem() {
	index := selectedTrashIndex()
	if index < 0 || index >= len(sessionTrashItems) {
		msg("Корзина", "Выберите запись для восстановления.", MB_OK|MB_ICONWARNING)
		return
	}
	item := sessionTrashItems[index]
	if item.Restore == nil {
		msg("Корзина", "Эта запись не поддерживает автоматическое восстановление.", MB_OK|MB_ICONWARNING)
		return
	}
	if err := item.Restore(); err != nil {
		msg("Корзина", "Не удалось восстановить запись: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	sessionTrashItems = append(sessionTrashItems[:index], sessionTrashItems[index+1:]...)
	updateSessionTrashCaption()
	refreshSessionTrashDialog()
}

func clearSessionTrashFromDialog(owner syscall.Handle) {
	if len(sessionTrashItems) == 0 {
		return
	}
	r, _, _ := pMessageBoxW.Call(uintptr(owner), uintptr(unsafe.Pointer(p("Очистить корзину текущей сессии без возможности восстановления?"))), uintptr(unsafe.Pointer(p("Очистка корзины"))), 0x00000004|MB_ICONWARNING)
	if r != 6 {
		return
	}
	finishSessionTrash()
	refreshSessionTrashDialog()
}

func sessionTrashWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		h := syscall.Handle(hwnd)
		title := modalCreate("STATIC", "КОРЗИНА ТЕКУЩЕЙ СЕССИИ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 18, 700, 34, h, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		trashList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 24, 64, 700, 180, h, idTrashList)
		trashDetails = modalCreate("RichEdit20W", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL|ES_READONLY, 24, 256, 700, 142, h, 0)
		modalCreate("BUTTON", "Восстановить", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 24, 414, 210, 42, h, idTrashRestore)
		modalCreate("BUTTON", "Очистить корзину", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 246, 414, 220, 42, h, idTrashClear)
		modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 514, 414, 210, 42, h, idTrashClose)
		refreshSessionTrashDialog()
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idTrashList:
			if hiWord(wParam) == 1 {
				showSelectedTrashDetails()
			}
		case idTrashRestore:
			restoreSelectedTrashItem()
		case idTrashClear:
			clearSessionTrashFromDialog(syscall.Handle(hwnd))
		case idTrashClose:
			trashDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
		}
		return 0
	case labMappingClose:
		trashDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openSessionTrashDialog() {
	trashDialogDone = false
	cls := p("ROSASessionTrashDialogV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(sessionTrashWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — корзина текущей сессии"))), 0x00C00000|WS_VISIBLE, 0x80000000, 0x80000000, 770, 520, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return
	}
	var message MSG
	for !trashDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
