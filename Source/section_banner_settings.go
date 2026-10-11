//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

const (
	// WM_COMMAND carries child-control IDs in a 16-bit word. Keep these below
	// 0x10000 so click notifications reach wndProc without truncation.
	ID_SECTION_BANNER_SETTINGS = 32440

	idSectionBannerSettingsSave   = 32441
	idSectionBannerSettingsCancel = 32442
	idSectionNoticeLabel          = 32443

	sectionBannerSettingsWindowWidth  = 860
	sectionBannerSettingsWindowHeight = 640
)

type SectionBannerSettings struct {
	SchemaVersion int             `json:"schema_version"`
	Enabled       map[string]bool `json:"enabled,omitempty"`
}

var sectionBannerSettings = SectionBannerSettings{
	SchemaVersion: 1,
	Enabled:       map[string]bool{},
}

var sectionBannerDefaultPages = []int{ID_DASH, ID_OBJECTS, ID_LAB, ID_ASSESS, ID_ACTIVITIES, ID_MRP, ID_ECON, ID_RESULT, ID_ARCHIVE, ID_DATA}

func sectionBannerSettingsPath() string { return sharedPath("section_banner_settings.json") }

func sectionBannerKey(pageID int) string {
	switch pageID {
	case ID_DASH:
		return "dashboard"
	case ID_TECHPARAM, ID_TELEMETRY, ID_DYN:
		return "objects"
	case ID_OBJECTS:
		return "objects"
	case ID_LAB:
		return "lab"
	case ID_ASSESS:
		return "assess"
	case ID_ACTIVITIES:
		return "activities"
	case ID_MRP:
		return "mrp"
	case ID_ECON:
		return "economics"
	case ID_RESULT:
		return "result"
	case ID_ARCHIVE:
		return "archive"
	case ID_DATA:
		return "data"
	default:
		return fmt.Sprintf("page_%d", pageID)
	}
}

func sectionBannerLabel(pageID int) string {
	switch pageID {
	case ID_DASH:
		return "Главная"
	case ID_OBJECTS:
		return "1. Фонд и карточка скважины"
	case ID_LAB:
		return "2. Лабораторные данные"
	case ID_ASSESS:
		return "3. Оценка осложнений"
	case ID_ACTIVITIES:
		return "4. Методы и мероприятия"
	case ID_MRP:
		return "5. Наработка и отказы"
	case ID_ECON:
		return "6. Экономическая оценка"
	case ID_RESULT:
		return "7. Итоговый отчёт"
	case ID_ARCHIVE:
		return "8. Утверждённый реестр"
	case ID_DATA:
		return "9. Настройки и журнал"
	default:
		return fmt.Sprintf("Раздел %d", pageID)
	}
}

func normalizeSectionBannerSettings() {
	if sectionBannerSettings.SchemaVersion == 0 {
		sectionBannerSettings.SchemaVersion = 1
	}
	if sectionBannerSettings.Enabled == nil {
		sectionBannerSettings.Enabled = map[string]bool{}
	}
	for _, pageID := range sectionBannerDefaultPages {
		key := sectionBannerKey(pageID)
		if _, ok := sectionBannerSettings.Enabled[key]; !ok {
			sectionBannerSettings.Enabled[key] = true
		}
	}
	// keep keys stable for manual review
	keys := make([]string, 0, len(sectionBannerSettings.Enabled))
	for key := range sectionBannerSettings.Enabled {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]bool, len(keys))
	for _, key := range keys {
		ordered[key] = sectionBannerSettings.Enabled[key]
	}
	sectionBannerSettings.Enabled = ordered
}

func loadSectionBannerSettings() {
	cfg := SectionBannerSettings{}
	loadJSON(sectionBannerSettingsPath(), &cfg)
	if cfg.SchemaVersion != 0 {
		sectionBannerSettings = cfg
	}
	normalizeSectionBannerSettings()
}

func saveSectionBannerSettings() error {
	normalizeSectionBannerSettings()
	return saveJSON(sectionBannerSettingsPath(), sectionBannerSettings)
}

func sectionBannerEnabled(pageID int) bool {
	normalizeSectionBannerSettings()
	v, ok := sectionBannerSettings.Enabled[sectionBannerKey(pageID)]
	if !ok {
		return true
	}
	return v
}

var (
	sectionBannerSettingsCallback = syscall.NewCallback(sectionBannerSettingsWndProc)
	sectionBannerSettingsDone     bool
	sectionBannerSettingsApply    bool
	sectionBannerChecks           [10]syscall.Handle
	sectionBannerModalHeader      syscall.Handle
	sectionBannerModalTitle       syscall.Handle
	sectionBannerModalAccent      syscall.Handle
	sectionNoticeLabel            syscall.Handle
)

// A single control lives in the existing upper notice strip. Its position is
// independent of section layouts, including compact and high-DPI layouts.
func createSectionNoticeControl() {
	sectionNoticeLabel = create("STATIC", "В работе учебные примеры", WS_CHILD|SS_CENTER|SS_CENTERIMAGE|SS_NOPREFIX, 0, WARNING_Y+5, 300, 36, hMain, idSectionNoticeLabel)
	if hFontWarning != 0 {
		pSendMessageW.Call(uintptr(sectionNoticeLabel), WM_SETFONT, uintptr(hFontWarning), 1)
	}
	setPhysicalControlVisibility(sectionNoticeLabel, false)
}

func sectionHasTeachingData() bool {
	for _, pass := range labPasses {
		if pass.Status == labPassDiscardedStatus {
			continue
		}
		if _, ok := teachingWorkflowOrdinal(pass.ObjectKey); ok {
			return true
		}
		if strings.Contains(strings.ToLower(pass.Source), "учеб") {
			return true
		}
	}
	for _, event := range failureEvents {
		source := strings.ToLower(event.Source)
		if strings.Contains(source, "учеб") || strings.Contains(source, "демо") || strings.Contains(source, "demo") {
			return true
		}
	}
	return false
}

func refreshSectionNotice() {
	visible := sectionBannerEnabled(currentPage) && sectionHasTeachingData()
	setPhysicalControlVisibility(sectionNoticeLabel, visible)
}

func sectionBannerSettingsWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		h := syscall.Handle(hwnd)
		sectionBannerModalHeader = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, sectionBannerSettingsWindowWidth, 62, h, 0)
		sectionBannerModalAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 62, sectionBannerSettingsWindowWidth, 5, h, 0)
		sectionBannerModalTitle = modalCreate("STATIC", "ПОДПИСЬ ПО РАЗДЕЛАМ", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_CENTERIMAGE, 24, 10, 810, 44, h, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(sectionBannerModalTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		intro := modalCreate("STATIC", "Отметьте вкладки, где показывать подпись «В работе учебные примеры».", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 82, 810, 38, h, 0)
		pSendMessageW.Call(uintptr(intro), WM_SETFONT, uintptr(hFont), 1)
		for i, pageID := range sectionBannerDefaultPages {
			label := sectionBannerLabel(pageID)
			sectionBannerChecks[i] = modalCreate("BUTTON", label, WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 30, 128+int32(i)*34, 800, 32, h, 200+i)
			if sectionBannerEnabled(pageID) {
				pSendMessageW.Call(uintptr(sectionBannerChecks[i]), BM_SETCHECK, BST_CHECKED, 0)
			}
			if hFont != 0 {
				pSendMessageW.Call(uintptr(sectionBannerChecks[i]), WM_SETFONT, uintptr(hFont), 1)
			}
		}
		footer := modalCreate("STATIC", "Снимите отметку, чтобы скрыть подпись во вкладке.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 482, 810, 30, h, 0)
		pSendMessageW.Call(uintptr(footer), WM_SETFONT, uintptr(hFont), 1)
		modalCreate("BUTTON", "Сохранить настройки", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 532, 260, 42, h, idSectionBannerSettingsSave)
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 300, 532, 160, 42, h, idSectionBannerSettingsCancel)
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil && (int(di.CtlID) == idSectionBannerSettingsSave || int(di.CtlID) == idSectionBannerSettingsCancel) {
			br, fg := brushLight, rgb(15, 48, 82)
			if int(di.CtlID) == idSectionBannerSettingsSave {
				br, fg = brushHeader, rgb(255, 255, 255)
			}
			if di.ItemState&ODS_SELECTED != 0 {
				br, fg = brushRedPressed, rgb(255, 255, 255)
			}
			pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(br))
			pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushBorder))
			pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
			pSetTextColor.Call(uintptr(di.HDC), fg)
			old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
			pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(getText(di.HwndItem)))), ^uintptr(0), uintptr(unsafe.Pointer(&di.RcItem)), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
			if old != 0 {
				pSelectObject.Call(uintptr(di.HDC), old)
			}
			return 1
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		if child == sectionBannerModalTitle {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		}
		if child == sectionBannerModalHeader {
			return uintptr(brushNav)
		}
		if child == sectionBannerModalAccent {
			return uintptr(brushHeader)
		}
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idSectionBannerSettingsSave:
			for i, pageID := range sectionBannerDefaultPages {
				v, _, _ := pSendMessageW.Call(uintptr(sectionBannerChecks[i]), BM_GETCHECK, 0, 0)
				sectionBannerSettings.Enabled[sectionBannerKey(pageID)] = v != 0
			}
			if err := saveSectionBannerSettings(); err != nil {
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p("Не удалось сохранить настройки: "+err.Error()))), uintptr(unsafe.Pointer(p("Настройки разделов"))), MB_OK|MB_ICONERROR)
				return 0
			}
			sectionBannerSettingsApply, sectionBannerSettingsDone = true, true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case idSectionBannerSettingsCancel:
			sectionBannerSettingsApply, sectionBannerSettingsDone = false, true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010:
		sectionBannerSettingsApply, sectionBannerSettingsDone = false, true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r
}

func chooseSectionBannerSettings() bool {
	normalizeSectionBannerSettings()
	sectionBannerSettingsDone, sectionBannerSettingsApply = false, false
	cls := p("ROSA_SectionBannerSettings")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: sectionBannerSettingsCallback, HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — подпись по разделам"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, uintptr(sectionBannerSettingsWindowWidth), uintptr(sectionBannerSettingsWindowHeight), uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return false
	}
	var message MSG
	for !sectionBannerSettingsDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return sectionBannerSettingsApply
}
