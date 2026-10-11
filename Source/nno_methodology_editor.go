//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	nnoEditorPasswordID = 9840
	nnoEditorUnlockID   = 9841
	nnoEditorIncludeID  = 9842
	nnoEditorExcludeID  = 9843
	nnoEditorUnclearID  = 9844
	nnoEditorSaveID     = 9845
	nnoEditorCloseID    = 9846
	nnoEditorNoteID     = 9847
)

type NNORuleDraft struct {
	Key               string `json:"key"`
	ClassifierVersion string `json:"classifier_version"`
	Branch            string `json:"branch"`
	Code              string `json:"code"`
	Treatment         string `json:"treatment"` // include, exclude, unresolved
	Reason            string `json:"reason"`
}
type NNOEditorDraftFile struct {
	SchemaVersion int            `json:"schema_version"`
	DraftID       string         `json:"draft_id"`
	UpdatedAt     string         `json:"updated_at"`
	UpdatedBy     string         `json:"updated_by"`
	Approved      bool           `json:"approved"`
	State         string         `json:"state"`
	PeriodMonths  int            `json:"period_months"`
	Formula       string         `json:"formula"`
	Rules         []NNORuleDraft `json:"rules"`
}

var (
	nnoEditorDone, nnoEditorUnlocked                                                                     bool
	nnoEditorPassword, nnoEditorGrid, nnoEditorNote, nnoEditorHeader, nnoEditorHint                      syscall.Handle
	nnoEditorUnlock, nnoEditorClose, nnoEditorSave, nnoEditorInclude, nnoEditorExclude, nnoEditorUnclear syscall.Handle
	nnoEditorRows                                                                                        []OfficialFailureCause2024
	nnoEditorDraft                                                                                       NNOEditorDraftFile
)

func nnoRuleKey(c OfficialFailureCause2024) string {
	return strings.Join([]string{c.Version, c.Branch, c.Code}, "|")
}
func nnoDraftPath() string { return sharedPath("nno_methodology_draft.json") }
func nnoEditorRule(index int) *NNORuleDraft {
	if index < 0 || index >= len(nnoEditorRows) {
		return nil
	}
	k := nnoRuleKey(nnoEditorRows[index])
	for i := range nnoEditorDraft.Rules {
		if nnoEditorDraft.Rules[i].Key == k {
			return &nnoEditorDraft.Rules[i]
		}
	}
	c := nnoEditorRows[index]
	nnoEditorDraft.Rules = append(nnoEditorDraft.Rules, NNORuleDraft{Key: k, ClassifierVersion: c.Version, Branch: c.Branch, Code: c.Code, Treatment: "unresolved"})
	return &nnoEditorDraft.Rules[len(nnoEditorDraft.Rules)-1]
}
func nnoLoadDraft() {
	nnoEditorDraft = NNOEditorDraftFile{SchemaVersion: 1, PeriodMonths: 12, Formula: "H12 = ΣTi / ΣOi", State: "ЧЕРНОВИК — НЕ ДЛЯ ОФИЦИАЛЬНОЙ ННО"}
	data, err := os.ReadFile(nnoDraftPath())
	if err == nil {
		var saved NNOEditorDraftFile
		if json.Unmarshal(data, &saved) == nil && saved.SchemaVersion == 1 && !saved.Approved && saved.PeriodMonths == 12 {
			nnoEditorDraft = saved
		}
	}
	nnoEditorDraft.Approved = false
	nnoEditorDraft.State = "ЧЕРНОВИК — НЕ ДЛЯ ОФИЦИАЛЬНОЙ ННО"
	nnoEditorDraft.PeriodMonths = 12
	nnoEditorDraft.Formula = "H12 = ΣTi / ΣOi"
}
func nnoEditorTableRefresh() {
	if nnoEditorGrid == 0 {
		return
	}
	listBeginUpdate(nnoEditorGrid)
	listClear(nnoEditorGrid)
	for i, c := range nnoEditorRows {
		rule := nnoEditorRule(i)
		caption := "На согласовании"
		if rule.Treatment == "include" {
			caption = "Учитывать"
		} else if rule.Treatment == "exclude" {
			caption = "Не учитывать"
		}
		listAdd(nnoEditorGrid, i, []string{c.Version, c.Branch, c.Code, c.Title, caption, rule.Reason})
	}
	listEndUpdate(nnoEditorGrid)
}
func nnoEditSelectedTreatment(treatment string) {
	i := selectedRow(nnoEditorGrid)
	if i < 0 || i >= len(nnoEditorRows) {
		msg("Методика ННО", "Выберите строку классификатора.", MB_OK|MB_ICONWARNING)
		return
	}
	reason := strings.TrimSpace(getText(nnoEditorNote))
	if treatment != "unresolved" && len([]rune(reason)) < 8 {
		msg("Методика ННО", "Укажите основание решения (не менее 8 символов).", MB_OK|MB_ICONWARNING)
		return
	}
	r := nnoEditorRule(i)
	r.Treatment = treatment
	r.Reason = reason
	nnoEditorTableRefresh()
	// Row selection is restored by the operator before the next action.
	setText(nnoEditorHint, fmt.Sprintf("Изменено правило: %s. Это черновик; производственный показатель не пересчитан.", r.Key))
}
func nnoEditorSaveDraft() {
	if !nnoEditorUnlocked || currentSession == nil || !currentSession.IsAdmin {
		return
	}
	nnoEditorDraft.UpdatedAt = time.Now().Format(time.RFC3339)
	nnoEditorDraft.UpdatedBy = currentSession.Login
	nnoEditorDraft.DraftID = "NNO-DRAFT-" + time.Now().Format("20060102-150405.000000000")
	nnoEditorDraft.Approved = false
	historyDir := sharedPath("nno_methodology_history")
	if err := os.MkdirAll(historyDir, 0700); err != nil {
		msg("Методика ННО", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	snapshot := filepath.Join(historyDir, nnoEditorDraft.DraftID+".json")
	if _, err := os.Stat(snapshot); err == nil {
		msg("Методика ННО", "Версия черновика уже существует.", MB_OK|MB_ICONWARNING)
		return
	}
	if err := saveJSON(snapshot, nnoEditorDraft); err != nil {
		msg("Методика ННО", "Не удалось сохранить: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if err := saveJSON(nnoDraftPath(), nnoEditorDraft); err != nil {
		msg("Методика ННО", "Снимок сохранён, но указатель на текущий черновик не обновлён: "+err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	msg("Методика ННО", "Черновик сохранён отдельно от утверждённой методики. Для его активации нужны проверка Ti, согласование и версия с датой действия.", MB_OK|MB_ICONINFORMATION)
}
func nnoBuildEditorGrid(parent syscall.Handle) {
	nnoEditorRows = append([]OfficialFailureCause2024(nil), officialFailureCauses2024...)
	nnoEditorGrid = modalCreate("SysListView32", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS, 24, 124, 995, 400, parent, 0)
	pSendMessageW.Call(uintptr(nnoEditorGrid), LVM_SETEXTENDEDLISTVIEWSTYLE, LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER, LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER)
	pSendMessageW.Call(uintptr(nnoEditorGrid), WM_SETFONT, uintptr(hFont), 1)
	cols := []struct {
		name string
		w    int32
	}{{"Версия", 90}, {"Раздел", 155}, {"Код", 70}, {"Причина отказа", 300}, {"Правило ННО", 125}, {"Основание", 260}}
	for i, c := range cols {
		col := LVCOLUMN{Mask: LVCF_TEXT | LVCF_WIDTH | LVCF_FMT, Fmt: LVCFMT_LEFT, Cx: c.w, PszText: p(c.name)}
		pSendMessageW.Call(uintptr(nnoEditorGrid), LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
	}
	modalCreate("STATIC", "Основание изменения (обязательно при выборе «Учитывать» / «Не учитывать»):", WS_CHILD|WS_VISIBLE, 24, 532, 900, 21, parent, 0)
	nnoEditorNote = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 24, 558, 995, 33, parent, nnoEditorNoteID)
	nnoEditorInclude = modalCreate("BUTTON", "Учитывать", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 24, 603, 175, 42, parent, nnoEditorIncludeID)
	nnoEditorExclude = modalCreate("BUTTON", "Не учитывать", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 209, 603, 175, 42, parent, nnoEditorExcludeID)
	nnoEditorUnclear = modalCreate("BUTTON", "На согласовании", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 394, 603, 185, 42, parent, nnoEditorUnclearID)
	nnoEditorSave = modalCreate("BUTTON", "Сохранить черновик", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 589, 603, 210, 42, parent, nnoEditorSaveID)
	nnoEditorClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 809, 603, 210, 42, parent, nnoEditorCloseID)
	nnoEditorTableRefresh()
}
func nnoEditorProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		nnoEditorHeader = modalCreate("STATIC", "РЕДАКТОР ПРАВИЛ ННО · ТАБЛИЧНЫЙ РЕЖИМ", WS_CHILD|WS_VISIBLE, 24, 19, 990, 34, parent, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(nnoEditorHeader), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		nnoEditorHint = modalCreate("STATIC", "Доступ: администратор. После подтверждения пароля доступны правила классификатора.", WS_CHILD|WS_VISIBLE, 24, 63, 990, 45, parent, 0)
		nnoEditorPassword = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_PASSWORD|ES_AUTOHSCROLL, 24, 124, 530, 38, parent, nnoEditorPasswordID)
		nnoEditorUnlock = modalCreate("BUTTON", "Подтвердить пароль", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 570, 124, 245, 38, parent, nnoEditorUnlockID)
		nnoEditorClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 827, 124, 192, 38, parent, nnoEditorCloseID)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case nnoEditorUnlockID:
			if currentSession == nil || !currentSession.IsAdmin || currentSession.IsGuest {
				return 0
			}
			attempt := getText(nnoEditorPassword)
			verified, err := authenticateLocal(currentSession.Login, attempt)
			setText(nnoEditorPassword, "")
			if err != nil || verified == nil || !verified.IsAdmin || verified.UserID != currentSession.UserID {
				setText(nnoEditorHint, "Пароль не подтверждён. Доступ к правилам закрыт.")
				return 0
			}
			for i := range verified.masterKey {
				verified.masterKey[i] = 0
			}
			nnoEditorUnlocked = true
			show(nnoEditorPassword, false)
			show(nnoEditorUnlock, false)
			show(nnoEditorClose, false)
			setText(nnoEditorHint, "ЧЕРНОВИК · 12 месяцев · формула ΣTi/ΣOi. Меняется включение кодов причин, но не исходные факты. Активация отдельно.")
			nnoBuildEditorGrid(syscall.Handle(hwnd))
			return 0
		case nnoEditorIncludeID:
			if nnoEditorUnlocked {
				nnoEditSelectedTreatment("include")
			}
			return 0
		case nnoEditorExcludeID:
			if nnoEditorUnlocked {
				nnoEditSelectedTreatment("exclude")
			}
			return 0
		case nnoEditorUnclearID:
			if nnoEditorUnlocked {
				nnoEditSelectedTreatment("unresolved")
			}
			return 0
		case nnoEditorSaveID:
			if nnoEditorUnlocked {
				nnoEditorSaveDraft()
			}
			return 0
		case nnoEditorCloseID:
			nnoEditorDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case WM_NOTIFY:
		if lParam != 0 && nnoEditorUnlocked {
			hdr := (*NMHDR)(unsafe.Pointer(lParam))
			if hdr.HwndFrom == nnoEditorGrid && (hdr.Code == NM_CLICK || hdr.Code == NM_DBLCLK) {
				i := selectedRow(nnoEditorGrid)
				if rule := nnoEditorRule(i); rule != nil {
					setText(nnoEditorNote, rule.Reason)
				}
			}
		}
	case labMappingClose:
		nnoEditorDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}
func openNNOMethodologyEditor() {
	if currentSession == nil || !currentSession.IsAdmin || currentSession.IsGuest {
		msg("Методика ННО", "Редактирование доступно только администратору, после повторного ввода пароля. Просмотр состава — без изменения методики.", MB_OK|MB_ICONWARNING)
		return
	}
	nnoEditorDone, nnoEditorUnlocked = false, false
	nnoEditorGrid, nnoEditorNote = 0, 0
	nnoLoadDraft()
	cls := p("ROSANNOEditorV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(nnoEditorProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0x00000080, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — правила расчёта ННО"))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 1050, 710, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return
	}
	var m MSG
	for !nnoEditorDone {
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
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
