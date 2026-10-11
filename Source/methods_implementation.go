//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_METHODS_IMPLEMENTATION_EDIT = 9636

	methodsImplementationEditorSave         = 9670
	methodsImplementationEditorReset        = 9671
	methodsImplementationEditorCancel       = 9672
	methodsImplementationEditorSaveScheme   = 9673
	methodsImplementationEditorChooseScheme = 9674
	methodsImplementationSchemeList         = 9675
	methodsImplementationSchemeApply        = 9676
	methodsImplementationSchemeCancel       = 9677
	methodsImplementationReferenceToggle    = 9678
	methodsImplementationEditorBase         = 9680
	methodsImplementationFieldCount         = 7
)

type methodImplementationField struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Suggested string `json:"suggested"`
	Value     string `json:"value"`
	Changed   bool   `json:"changed"`
}

type MethodImplementationRecord struct {
	ObjectKey string                      `json:"object_key"`
	Method    string                      `json:"method"`
	Kind      string                      `json:"kind"`
	UpdatedAt string                      `json:"updated_at"`
	UpdatedBy string                      `json:"updated_by,omitempty"`
	Fields    []methodImplementationField `json:"fields"`
}

type MethodChemicalScheme struct {
	Method    string                      `json:"method"`
	Kind      string                      `json:"kind"`
	UpdatedAt string                      `json:"updated_at"`
	UpdatedBy string                      `json:"updated_by,omitempty"`
	Fields    []methodImplementationField `json:"fields"`
}

type methodImplementationAuditChange struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Suggested string `json:"suggested"`
	Before    string `json:"before"`
	After     string `json:"after"`
	Changed   bool   `json:"changed"`
}

type methodImplementationAuditEntry struct {
	ObjectKey string                            `json:"object_key"`
	Method    string                            `json:"method"`
	Kind      string                            `json:"kind"`
	ChangedAt string                            `json:"changed_at"`
	ChangedBy string                            `json:"changed_by,omitempty"`
	Changes   []methodImplementationAuditChange `json:"changes"`
}

var (
	methodImplementationRecordsLoaded bool
	methodImplementationRecords       []MethodImplementationRecord

	methodImplementationEditorDone               bool
	methodImplementationEditorObject             string
	methodImplementationEditorMethod             string
	methodImplementationEditorKind               string
	methodImplementationEditorFields             []methodImplementationField
	methodImplementationEditorEdits              [methodsImplementationFieldCount]syscall.Handle
	methodImplementationEditorLabels             [methodsImplementationFieldCount]syscall.Handle
	methodImplementationHeaderBand               syscall.Handle
	methodImplementationAccent                   syscall.Handle
	methodImplementationTitle                    syscall.Handle
	methodImplementationSubtitle                 syscall.Handle
	methodImplementationFieldsPanel              syscall.Handle
	methodImplementationFieldsAccent             syscall.Handle
	methodImplementationSectionTitle             syscall.Handle
	methodImplementationSectionRule              syscall.Handle
	methodImplementationGroupPanels              [3]syscall.Handle
	methodImplementationGroupAccents             [3]syscall.Handle
	methodImplementationGroupTitles              [3]syscall.Handle
	methodImplementationNote                     syscall.Handle
	methodImplementationNoteBand                 syscall.Handle
	methodImplementationActionsTitle             syscall.Handle
	methodImplementationReagentSummary           syscall.Handle
	methodImplementationReferenceToggleButton    syscall.Handle
	methodImplementationReferenceExpanded        bool
	methodImplementationReferencePanel           syscall.Handle
	methodImplementationReferenceAccent          syscall.Handle
	methodImplementationObjectBadge              syscall.Handle
	methodImplementationBottomRule               syscall.Handle
	methodImplementationResetButton              syscall.Handle
	methodImplementationCancelButton             syscall.Handle
	methodImplementationSaveButton               syscall.Handle
	methodImplementationSchemeSaveButton         syscall.Handle
	methodImplementationSchemeChooseButton       syscall.Handle
	methodImplementationReagentIndex             int
	methodImplementationSchemeChooserDone        bool
	methodImplementationSchemeChooserList        syscall.Handle
	methodImplementationSchemeChooserDetails     syscall.Handle
	methodImplementationSchemeChooserItems       []MethodChemicalScheme
	methodImplementationSchemeChooserHeader      syscall.Handle
	methodImplementationSchemeChooserAccent      syscall.Handle
	methodImplementationSchemeChooserTitle       syscall.Handle
	methodImplementationSchemeChooserSub         syscall.Handle
	methodImplementationSchemeChooserListTitle   syscall.Handle
	methodImplementationSchemeChooserDetailTitle syscall.Handle
	methodImplementationSchemeChooserNote        syscall.Handle
	methodImplementationEditorScrollPos          int32
	methodImplementationEditorScrollMax          int32
	methodImplementationScrollWindowEx           = user32.NewProc("ScrollWindowEx")
)

func methodImplementationPath() string { return sharedPath("method_implementation_parameters.json") }
func methodImplementationAuditPath() string {
	return sharedPath("method_implementation_parameters_journal.jsonl")
}

func methodReagentDictionaryPath() string { return sharedPath("method_reagents.json") }
func methodChemicalSchemesPath() string   { return sharedPath("method_chemical_schemes.json") }

func methodChemicalSchemes() []MethodChemicalScheme {
	values := []MethodChemicalScheme{}
	loadJSON(methodChemicalSchemesPath(), &values)
	for i := range values {
		if !strings.EqualFold(strings.TrimSpace(values[i].Kind), "Дозирование") {
			continue
		}
		hasUnit := false
		for _, field := range values[i].Fields {
			if field.Key == "unit" {
				hasUnit = true
				break
			}
		}
		if hasUnit {
			continue
		}
		upgraded := make([]methodImplementationField, 0, len(values[i].Fields)+1)
		for _, field := range values[i].Fields {
			upgraded = append(upgraded, field)
			if field.Key == "basis" {
				upgraded = append(upgraded, methodImplementationField{Key: "unit", Label: "Единица измерения", Suggested: "гр/м³", Value: "Не выбрано", Changed: true})
			}
		}
		values[i].Fields = upgraded
	}
	return values
}

func methodReagentDictionary() []string {
	values := []string{}
	loadJSON(methodReagentDictionaryPath(), &values)
	values = append(values, "Не выбрано")
	for _, scheme := range methodChemicalSchemes() {
		for _, field := range scheme.Fields {
			if field.Key == "reagent" {
				values = append(values, field.Value)
			}
		}
	}
	ensureMethodImplementationRecordsLoaded()
	for _, record := range methodImplementationRecords {
		for _, field := range record.Fields {
			if field.Key == "reagent" {
				value := strings.TrimSpace(field.Value)
				if value != "" && !strings.Contains(strings.ToLower(value), "уточнить марку") && !strings.Contains(strings.ToLower(value), "по карточке метода") {
					values = append(values, value)
				}
			}
		}
	}
	return uniqueSortedCaseInsensitive(values)
}

func uniqueSortedCaseInsensitive(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	for i, value := range result {
		if strings.EqualFold(value, "Не выбрано") {
			result[0], result[i] = result[i], result[0]
			break
		}
	}
	return result
}

func saveMethodReagent(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "Не выбрано") || strings.Contains(strings.ToLower(value), "уточнить марку") || strings.Contains(strings.ToLower(value), "по карточке метода") {
		return fmt.Errorf("введите конкретное наименование или марку реагента")
	}
	values := methodReagentDictionary()
	for _, existing := range values {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return fmt.Errorf("реагент «%s» уже существует в справочнике. Дублирование не выполнено", value)
		}
	}
	values = uniqueSortedCaseInsensitive(append(values, value))
	return saveJSON(methodReagentDictionaryPath(), values)
}

func methodChemicalSchemeSignature(fields []methodImplementationField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, strings.ToLower(strings.TrimSpace(field.Key))+"="+strings.ToLower(strings.TrimSpace(field.Value)))
	}
	return strings.Join(parts, "\x1f")
}

func methodChemicalSchemeFieldKeys(fields []methodImplementationField) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, strings.ToLower(strings.TrimSpace(field.Key)))
	}
	return strings.Join(parts, "\x1f")
}

func methodChemicalSchemeFromEditor() (MethodChemicalScheme, error) {
	fields := make([]methodImplementationField, len(methodImplementationEditorFields))
	copy(fields, methodImplementationEditorFields)
	for i := range fields {
		fields[i].Value = strings.TrimSpace(getText(methodImplementationEditorEdits[i]))
		fields[i].Changed = fields[i].Value != fields[i].Suggested
		if fields[i].Value == "" || strings.EqualFold(fields[i].Value, "Не выбрано") {
			return MethodChemicalScheme{}, fmt.Errorf("заполните поле «%s»: схема сохраняется только как полная совокупность параметров", fields[i].Label)
		}
	}
	return MethodChemicalScheme{Method: methodImplementationEditorMethod, Kind: methodImplementationEditorKind, Fields: fields}, nil
}

func saveMethodChemicalScheme(scheme MethodChemicalScheme) error {
	if len(scheme.Fields) == 0 {
		return fmt.Errorf("схема химизации не содержит параметров")
	}
	signature := methodChemicalSchemeSignature(scheme.Fields)
	values := methodChemicalSchemes()
	for _, existing := range values {
		if methodChemicalSchemeSignature(existing.Fields) == signature {
			return fmt.Errorf("схема химизации с такой совокупностью параметров уже существует. Дублирование не выполнено")
		}
	}
	scheme.UpdatedAt = time.Now().Format("02.01.2006 15:04")
	scheme.UpdatedBy = methodImplementationAuditUser()
	values = append(values, scheme)
	return saveJSON(methodChemicalSchemesPath(), values)
}

func compatibleMethodChemicalSchemes(kind string, fields []methodImplementationField) []MethodChemicalScheme {
	key := methodChemicalSchemeFieldKeys(fields)
	result := []MethodChemicalScheme{}
	for _, scheme := range methodChemicalSchemes() {
		if strings.EqualFold(strings.TrimSpace(scheme.Kind), strings.TrimSpace(kind)) && methodChemicalSchemeFieldKeys(scheme.Fields) == key {
			result = append(result, scheme)
		}
	}
	return result
}

func methodChemicalSchemeCaption(scheme MethodChemicalScheme) string {
	byKey := map[string]string{}
	for _, field := range scheme.Fields {
		byKey[field.Key] = strings.TrimSpace(field.Value)
	}
	values := make([]string, 0, 4)
	values = append(values, byKey["reagent"])
	if byKey["rate"] != "" || byKey["unit"] != "" {
		values = append(values, strings.TrimSpace(byKey["rate"]+" "+byKey["unit"]))
	}
	values = append(values, byKey["basis"], byKey["point"])
	compact := values[:0]
	for _, value := range values {
		if value != "" {
			compact = append(compact, value)
		}
	}
	values = compact
	if len(values) == 0 {
		for i := 0; i < len(scheme.Fields) && i < 3; i++ {
			values = append(values, strings.TrimSpace(scheme.Fields[i].Value))
		}
	}
	return strings.Join(values, " · ")
}

func methodChemicalSchemeDetails(scheme MethodChemicalScheme) string {
	var b strings.Builder
	b.WriteString("СХЕМА ХИМИЗАЦИИ\r\n\r\n")
	fmt.Fprintf(&b, "Метод: %s\r\n", scheme.Method)
	fmt.Fprintf(&b, "Тип реализации: %s\r\n\r\n", scheme.Kind)
	b.WriteString("ПАРАМЕТРЫ\r\n\r\n")
	for _, field := range scheme.Fields {
		fmt.Fprintf(&b, "• %s: %s\r\n", field.Label, field.Value)
	}
	if strings.TrimSpace(scheme.UpdatedAt) != "" {
		fmt.Fprintf(&b, "\r\nСохранено: %s", scheme.UpdatedAt)
	}
	return b.String()
}

func methodChemicalSchemeFieldValue(scheme MethodChemicalScheme, key string) string {
	for _, field := range scheme.Fields {
		if field.Key == key {
			return strings.TrimSpace(field.Value)
		}
	}
	return ""
}

func drawMethodChemicalSchemeListItem(di *DRAWITEMSTRUCT, schemes []MethodChemicalScheme) uintptr {
	if di == nil || int(di.ItemID) < 0 || int(di.ItemID) >= len(schemes) {
		return 0
	}
	scheme := schemes[int(di.ItemID)]
	selected := di.ItemState&ODS_SELECTED != 0
	fill := brushWhite
	titleColor := rgb(15, 48, 82)
	metaColor := rgb(75, 84, 94)
	border := brushBorder
	if selected {
		fill = brushNav
		titleColor = rgb(255, 255, 255)
		metaColor = rgb(220, 229, 238)
		border = brushNav
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(fill))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(border))

	accent := RECT{Left: di.RcItem.Left + 1, Top: di.RcItem.Top + 4, Right: di.RcItem.Left + 5, Bottom: di.RcItem.Bottom - 4}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(brushHeader))

	title := methodChemicalSchemeFieldValue(scheme, "reagent")
	if title == "" || strings.EqualFold(title, "Не выбрано") {
		title = scheme.Method
	}
	rate := methodChemicalSchemeFieldValue(scheme, "rate")
	unit := methodChemicalSchemeFieldValue(scheme, "unit")
	basis := methodChemicalSchemeFieldValue(scheme, "basis")
	metaParts := []string{}
	if rate != "" {
		if unit != "" && !strings.EqualFold(unit, "Не выбрано") {
			rate += " · " + unit
		}
		metaParts = append(metaParts, rate)
	}
	if basis != "" {
		metaParts = append(metaParts, basis)
	}
	if len(metaParts) == 0 {
		metaParts = append(metaParts, scheme.Kind)
	}
	meta := strings.Join(metaParts, " · ")

	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	titleRect := RECT{Left: di.RcItem.Left + 14, Top: di.RcItem.Top + 4, Right: di.RcItem.Right - 10, Bottom: di.RcItem.Top + 29}
	metaRect := RECT{Left: di.RcItem.Left + 14, Top: di.RcItem.Top + 29, Right: di.RcItem.Right - 10, Bottom: di.RcItem.Bottom - 3}
	titleFont := methodsFont(methodsTypeBody, true)
	metaFont := methodsFont(methodsTypeCompact, false)
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(titleFont))
	pSetTextColor.Call(uintptr(di.HDC), titleColor)
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(title))), ^uintptr(0), uintptr(unsafe.Pointer(&titleRect)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000)
	pSelectObject.Call(uintptr(di.HDC), uintptr(metaFont))
	pSetTextColor.Call(uintptr(di.HDC), metaColor)
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(meta))), ^uintptr(0), uintptr(unsafe.Pointer(&metaRect)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return 1
}

func selectedMethodChemicalScheme() (MethodChemicalScheme, bool) {
	if methodImplementationSchemeChooserList == 0 || len(methodImplementationSchemeChooserItems) == 0 {
		return MethodChemicalScheme{}, false
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodImplementationSchemeChooserList), LB_GETCURSEL, 0, 0)
	i := int(idx)
	if i < 0 || i >= len(methodImplementationSchemeChooserItems) {
		return MethodChemicalScheme{}, false
	}
	return methodImplementationSchemeChooserItems[i], true
}

func showSelectedMethodChemicalSchemeDetails() {
	if scheme, ok := selectedMethodChemicalScheme(); ok {
		setText(methodImplementationSchemeChooserDetails, methodChemicalSchemeDetails(scheme))
		formatMethodsReferenceText(methodImplementationSchemeChooserDetails, methodChemicalSchemeDetails(scheme))
	}
}

func applyMethodChemicalScheme(scheme MethodChemicalScheme) {
	byKey := map[string]methodImplementationField{}
	for _, field := range scheme.Fields {
		byKey[field.Key] = field
	}
	for i, field := range methodImplementationEditorFields {
		if selected, ok := byKey[field.Key]; ok {
			if field.Key == "unit" {
				field.Value = selected.Value
				fillMethodImplementationCombo(methodImplementationEditorEdits[i], field)
			} else {
				setText(methodImplementationEditorEdits[i], selected.Value)
			}
		}
	}
}

func methodChemicalSchemeChooserWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		methodImplementationSchemeChooserHeader = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1024, 86, parent, 0)
		methodImplementationSchemeChooserAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 82, 1024, 4, parent, 0)
		methodImplementationSchemeChooserTitle = modalCreate("STATIC", "ВЫБОР СХЕМЫ ХИМИЗАЦИИ", WS_CHILD|WS_VISIBLE|SS_LEFT, 28, 14, 960, 30, parent, 0)
		subtitle := fmt.Sprintf("%s · доступно схем: %d", methodImplementationEditorKind, len(methodImplementationSchemeChooserItems))
		methodImplementationSchemeChooserSub = modalCreate("STATIC", subtitle, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_ENDELLIPSIS, 28, 47, 960, 24, parent, 0)
		methodImplementationSchemeChooserListTitle = modalCreate("STATIC", "СОХРАНЁННЫЕ СХЕМЫ", WS_CHILD|WS_VISIBLE|SS_LEFT, 28, 108, 426, 28, parent, 0)
		methodImplementationSchemeChooserDetailTitle = modalCreate("STATIC", "СОСТАВ ВЫБРАННОЙ СХЕМЫ", WS_CHILD|WS_VISIBLE|SS_LEFT, 472, 108, 540, 28, parent, 0)
		for _, control := range []syscall.Handle{methodImplementationSchemeChooserTitle, methodImplementationSchemeChooserListTitle, methodImplementationSchemeChooserDetailTitle} {
			pSendMessageW.Call(uintptr(control), WM_SETFONT, uintptr(methodsFont(methodsTypeSection, true)), 1)
		}
		pSendMessageW.Call(uintptr(methodImplementationSchemeChooserSub), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)

		methodImplementationSchemeChooserList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY|LBS_OWNERDRAWFIXED|LBS_HASSTRINGS, 28, 140, 426, 424, parent, methodsImplementationSchemeList)
		_ = riched20.Load()
		methodImplementationSchemeChooserDetails = modalCreate("RichEdit20W", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 472, 140, 540, 424, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationSchemeChooserDetails), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		methodImplementationSchemeChooserNote = modalCreate("STATIC", "Будут заменены все параметры текущей схемы. Перед применением проверьте состав справа.", WS_CHILD|WS_VISIBLE|SS_LEFT, 28, 588, 610, 42, parent, 0)
		apply := modalCreate("BUTTON", "ПРИМЕНИТЬ СХЕМУ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 658, 588, 220, 44, parent, methodsImplementationSchemeApply)
		cancel := modalCreate("BUTTON", "ОТМЕНА", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 892, 588, 120, 44, parent, methodsImplementationSchemeCancel)
		for _, control := range []syscall.Handle{methodImplementationSchemeChooserList, methodImplementationSchemeChooserDetails} {
			if hFontPanel != 0 {
				pSendMessageW.Call(uintptr(control), WM_SETFONT, uintptr(hFontPanel), 1)
			}
		}
		pSendMessageW.Call(uintptr(methodImplementationSchemeChooserList), 0x01A0, 0, 58) // LB_SETITEMHEIGHT
		pSendMessageW.Call(uintptr(methodImplementationSchemeChooserNote), WM_SETFONT, uintptr(methodsFont(methodsTypeCompact, false)), 1)
		for _, control := range []syscall.Handle{apply, cancel} {
			if hFontButton != 0 {
				pSendMessageW.Call(uintptr(control), WM_SETFONT, uintptr(hFontButton), 1)
			}
		}
		for _, scheme := range methodImplementationSchemeChooserItems {
			caption := methodChemicalSchemeCaption(scheme)
			pSendMessageW.Call(uintptr(methodImplementationSchemeChooserList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
		}
		if len(methodImplementationSchemeChooserItems) > 0 {
			pSendMessageW.Call(uintptr(methodImplementationSchemeChooserList), LB_SETCURSEL, 0, 0)
			showSelectedMethodChemicalSchemeDetails()
		}
		methodImplementationEditorUpdateScroll(hwnd)
		return 0
	case WM_SIZE:
		methodImplementationEditorUpdateScroll(hwnd)
		return 0
	case WM_VSCROLL:
		// lParam == 0 identifies the editor window's own vertical scrollbar.
		// Child combobox/list scroll messages are left to the default procedure.
		if lParam == 0 {
			code := uint16(wParam & 0xffff)
			target := methodImplementationEditorScrollPos
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			page := max32(120, (rc.Bottom-rc.Top)*3/4)
			switch code {
			case 0: // SB_LINEUP
				target -= 36
			case 1: // SB_LINEDOWN
				target += 36
			case 2: // SB_PAGEUP
				target -= page
			case 3: // SB_PAGEDOWN
				target += page
			case 4, 5: // SB_THUMBPOSITION / SB_THUMBTRACK
				target = int32((wParam >> 16) & 0xffff)
			case 6: // SB_TOP
				target = 0
			case 7: // SB_BOTTOM
				target = methodImplementationEditorScrollMax
			}
			methodImplementationEditorSetScroll(hwnd, target)
			return 0
		}
	case 0x020A: // WM_MOUSEWHEEL
		delta := int16((wParam >> 16) & 0xffff)
		if delta != 0 {
			methodImplementationEditorSetScroll(hwnd, methodImplementationEditorScrollPos-int32(delta)/120*54)
			return 0
		}
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil {
			switch int(di.CtlID) {
			case methodsImplementationSchemeList:
				return drawMethodChemicalSchemeListItem(di, methodImplementationSchemeChooserItems)
			case methodsImplementationSchemeApply, methodsImplementationSchemeCancel:
				return drawMethodImplementationButton(di)
			}
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case methodImplementationSchemeChooserHeader:
			return uintptr(brushNav)
		case methodImplementationSchemeChooserAccent:
			return uintptr(brushHeader)
		case methodImplementationSchemeChooserTitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodImplementationSchemeChooserSub:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		case methodImplementationSchemeChooserListTitle, methodImplementationSchemeChooserDetailTitle, methodImplementationSchemeChooserNote:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
	case WM_COMMAND:
		id := int(loWord(wParam))
		notify := int(hiWord(wParam))
		if id == methodsImplementationSchemeList && notify == LBN_SELCHANGE {
			showSelectedMethodChemicalSchemeDetails()
			return 0
		}
		switch id {
		case methodsImplementationSchemeApply:
			if scheme, ok := selectedMethodChemicalScheme(); ok {
				applyMethodChemicalScheme(scheme)
			}
			methodImplementationSchemeChooserDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case methodsImplementationSchemeCancel:
			methodImplementationSchemeChooserDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		methodImplementationSchemeChooserDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openMethodChemicalSchemeChooser(parent uintptr) {
	methodImplementationSchemeChooserItems = compatibleMethodChemicalSchemes(methodImplementationEditorKind, methodImplementationEditorFields)
	if len(methodImplementationSchemeChooserItems) == 0 {
		msg("Справочник схем химизации", "Подходящей схемы в справочнике пока нет. Заполните параметры вручную и сохраните всю схему в справочник.", MB_OK|MB_ICONINFORMATION)
		return
	}
	methodImplementationSchemeChooserDone = false
	methodImplementationSchemeChooserList = 0
	methodImplementationSchemeChooserDetails = 0
	cls := p("ROSAMethodChemicalSchemeChooserV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodChemicalSchemeChooserWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(parent, 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — Схемы химизации"))), 0x00C80000|WS_VISIBLE, 0x80000000, 0x80000000, 1040, 700, parent, 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(parent, 1)
		msg("Справочник схем химизации", "Не удалось открыть выбор схемы.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !methodImplementationSchemeChooserDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(parent, 1)
	pSetFocus.Call(parent)
}

func methodImplementationFieldOptions(field methodImplementationField) []string {
	options := []string{"Не выбрано"}
	if field.Key != "unit" {
		options = append(options, field.Suggested)
	}
	switch field.Key {
	case "reagent":
		options = append(options, methodReagentDictionary()...)
	case "basis":
		options = append(options, "По воде", "По нефти", "По всей жидкости", "По объёму закачки", "По утверждённой методике")
	case "unit":
		basis := ""
		for i, candidate := range methodImplementationEditorFields {
			if candidate.Key == "basis" {
				basis = candidate.Value
				if methodImplementationEditorEdits[i] != 0 {
					basis = getText(methodImplementationEditorEdits[i])
				}
				break
			}
		}
		options = append(options, methodImplementationUnitOptionsForBasis(basis)...)
	case "rate":
		options = append(options, "По лабораторному подбору", "По промысловому подбору", "По лабораторному и промысловому подбору", "По утверждённому регламенту")
	case "point":
		options = append(options, "До зоны начала осложнения", "На приём насоса", "В затрубное пространство", "В выкидную линию", "На установке подготовки", "По утверждённой схеме дозирования")
	case "periodicity":
		options = append(options, "Непрерывно", "Периодически по утверждённому регламенту", "Разовая обработка", "По результатам контроля")
	case "control":
		options = append(options, "Остаточная концентрация", "Динамика осложнения", "МРП", "Остаточная концентрация / динамика осложнения / МРП", "Лабораторный и промысловый контроль")
	case "volume":
		options = append(options, "По фактическому объёму обрабатываемой зоны", "По объёму НКТ", "По объёму затрубного пространства", "По технологическому расчёту")
	case "strength":
		options = append(options, "По лабораторному подбору", "По подтверждённой совместимости", "По лабораторному подбору и подтверждённой совместимости")
	case "sequence":
		options = append(options, "Подготовка → закачка → выдержка → освоение", "Подготовка → промывка → контроль", "По утверждённому плану работ")
	case "contact":
		options = append(options, "По лабораторному подбору", "По регламенту", "По результатам ОЛИ / ОПИ", "По регламенту и результатам ОЛИ / ОПИ")
	case "mode":
		options = append(options, "По действующему технологическому режиму", "По утверждённому технологическому режиму объекта", "По отдельному плану работ")
	case "duration":
		options = append(options, "До достижения контрольной точки", "До достижения критерия прекращения", "По утверждённому регламенту")
	case "equipment":
		options = append(options, "Совместимо с действующим ГНО", "Требуется проверка совместимости с ГНО", "Требуется подбор конструкции в «Автотехнологе»")
	case "stop":
		options = append(options, "Достижение целевого эффекта", "Появление технологического ограничения", "Достижение целевого эффекта либо появление технологического ограничения")
	case "note":
		options = append(options, "Дополнительных условий нет", "По решению технолога", "По утверждённому плану работ")
	}
	return uniqueSortedCaseInsensitive(options)
}

func methodImplementationUnitOptionsForBasis(basis string) []string {
	basis = strings.ToLower(strings.TrimSpace(basis))
	switch {
	case strings.Contains(basis, "нефт"):
		return []string{"гр/тн", "тн"}
	case strings.Contains(basis, "вод"), strings.Contains(basis, "жидк"), strings.Contains(basis, "закач"):
		return []string{"гр/м³", "м³"}
	default:
		return []string{"гр/м³", "гр/тн", "тн", "м³"}
	}
}

func refreshMethodImplementationUnitCombo() {
	unitIndex := -1
	for i, field := range methodImplementationEditorFields {
		if field.Key == "unit" {
			unitIndex = i
			break
		}
	}
	if unitIndex < 0 || methodImplementationEditorEdits[unitIndex] == 0 {
		return
	}
	field := methodImplementationEditorFields[unitIndex]
	field.Value = strings.TrimSpace(getText(methodImplementationEditorEdits[unitIndex]))
	allowed := methodImplementationFieldOptions(field)
	valid := false
	for _, value := range allowed {
		if strings.EqualFold(value, field.Value) {
			valid = true
			break
		}
	}
	if !valid {
		field.Value = "Не выбрано"
	}
	fillMethodImplementationCombo(methodImplementationEditorEdits[unitIndex], field)
}

func fillMethodImplementationCombo(combo syscall.Handle, field methodImplementationField) {
	if combo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(combo), CB_RESETCONTENT, 0, 0)
	selected := -1
	for i, option := range methodImplementationFieldOptions(field) {
		pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(option))))
		if strings.EqualFold(strings.TrimSpace(option), strings.TrimSpace(field.Value)) {
			selected = i
		}
	}
	if field.Key == "unit" && selected >= 0 {
		pSendMessageW.Call(uintptr(combo), CB_SETCURSEL, uintptr(selected), 0)
		return
	}
	setText(combo, field.Value)
}

func ensureMethodImplementationRecordsLoaded() {
	if methodImplementationRecordsLoaded {
		return
	}
	methodImplementationRecordsLoaded = true
	methodImplementationRecords = nil
	loadJSON(methodImplementationPath(), &methodImplementationRecords)
}

func methodImplementationRecordIndex(objectKey, method string) int {
	ensureMethodImplementationRecordsLoaded()
	for i := len(methodImplementationRecords) - 1; i >= 0; i-- {
		if methodsObjectKeysEquivalent(methodImplementationRecords[i].ObjectKey, objectKey) &&
			normalizeMethodText(methodImplementationRecords[i].Method) == normalizeMethodText(method) {
			return i
		}
	}
	return -1
}

func methodImplementationSuggestedFields(s methodWorkflowSuggestion) (string, []methodImplementationField) {
	name := strings.TrimSpace(s.Method.Name)
	n := normalizeMethodText(name)
	kind := methodVisualType(name)
	field := func(key, label, suggested string) methodImplementationField {
		return methodImplementationField{Key: key, Label: label, Suggested: suggested, Value: suggested}
	}

	isDosage := kind == "Дозирование" || strings.Contains(n, "биоцид") || strings.Contains(n, "деэмульгатор")
	isWash := kind == "Промывка" || strings.Contains(n, "растворител") || strings.Contains(n, "кислот") || strings.Contains(n, "комплексон")
	if isDosage {
		basis := "По всей жидкости / уточнить методику"
		d := strings.ToLower(strings.TrimSpace(s.Direction))
		switch {
		case strings.Contains(d, "солеотлож") || strings.Contains(d, "корроз"):
			basis = "По воде"
		case strings.Contains(d, "аспо") || strings.Contains(d, "эмуль"):
			basis = "По нефти"
		}
		return "Дозирование", []methodImplementationField{
			field("reagent", "Реагент", "Уточнить марку по лабораторному подбору"),
			field("basis", "База дозирования", basis),
			field("unit", "Единица измерения", "гр/м³"),
			field("rate", "Концентрация / расход", "По лабораторному и промысловому подбору"),
			field("point", "Точка ввода", "До зоны начала осложнения / на приём насоса"),
			field("periodicity", "Периодичность", "Непрерывно либо по утверждённому регламенту"),
			field("control", "Контроль", "Остаточная концентрация / динамика осложнения / МРП"),
		}
	}
	if isWash {
		return "Промывка / обработка", []methodImplementationField{
			field("volume", "Объём", "Рассчитать по фактическому объёму обрабатываемой зоны"),
			field("strength", "Тоннаж / концентрация", "По лабораторному подбору и подтверждённой совместимости"),
			field("reagent", "Растворитель / реагент", "По карточке метода и результатам лабораторного подбора"),
			field("sequence", "Последовательность", "Подготовка → закачка → выдержка → освоение / промывка"),
			field("contact", "Время контакта", "По регламенту и результатам ОЛИ / ОПИ"),
			field("control", "Контроль результата", "Дебит / давление / состав отложений / динамика МРП"),
		}
	}
	return "Механическое / технологическое", []methodImplementationField{
		field("mode", "Режим", "По утверждённому технологическому режиму объекта"),
		field("duration", "Продолжительность", "До достижения критерия прекращения / контрольной точки"),
		field("equipment", "Ограничения оборудования", "Проверить совместимость с действующим ГНО; подбор конструкции — «Автотехнолог»"),
		field("stop", "Критерии прекращения", "Достижение целевого эффекта либо появление технологического ограничения"),
		field("control", "Контроль", "Режим, дебит, давление, проявление осложнения и отказность"),
		field("note", "Дополнительное условие", "Зафиксировать ограничения и основание решения технолога"),
	}
}

func teachingMethodImplementationFields(objectKey string, s methodWorkflowSuggestion, fields []methodImplementationField) []methodImplementationField {
	if _, ok := teachingWorkflowOrdinal(objectKey); !ok {
		return fields
	}
	n := normalizeMethodText(s.Method.Name)
	values := map[string]string{}
	switch {
	case strings.Contains(n, "ингиб") && strings.Contains(n, "корроз"):
		values = map[string]string{
			"reagent": "Ингибитор коррозии · учебный образец К-01",
			"basis":   "По воде", "unit": "гр/м³", "rate": "35",
			"point": "На приём насоса / до зоны риска коррозии", "periodicity": "Непрерывно",
			"control": "Остаточная концентрация + коррозионный контроль + МРП ≥ 1000 сут.",
		}
	case strings.Contains(n, "ингиб") && strings.Contains(n, "сол"):
		values = map[string]string{
			"reagent": "Ингибитор солеотложений · учебный образец С-02",
			"basis":   "По воде", "unit": "гр/м³", "rate": "30",
			"point": "На приём насоса / до зоны начала солеотложения", "periodicity": "Непрерывно",
			"control": "Остаточная концентрация + ионный состав + отложения + МРП ≥ 1000 сут.",
		}
	case strings.Contains(n, "промыв") && strings.Contains(n, "сол"):
		values = map[string]string{
			"strength": "Учебный раствор по лабораторному подбору и совместимости",
			"reagent":  "Состав для удаления карбонатных отложений · учебный образец",
			"sequence": "Изоляция → промывка → выдержка → контрольный запуск",
			"contact":  "4 ч; уточнить по результатам пробы растворимости",
			"control":  "Ионный состав, перепад давления, дебит и динамика МРП",
		}
	case strings.Contains(n, "растворител") && strings.Contains(n, "аспо"):
		values = map[string]string{
			"strength": "По учебной пробе растворимости АСПО",
			"reagent":  "Растворитель АСПО · учебный образец А-02",
			"sequence": "Подготовка → закачка → выдержка → освоение",
			"contact":  "6 ч по учебному протоколу",
			"control":  "Состав отложений, дебит, вязкость и МРП",
		}
	case strings.Contains(n, "деэмуль"):
		values = map[string]string{
			"reagent": "Деэмульгатор · учебный образец В-02",
			"basis":   "По жидкости", "unit": "гр/м³", "rate": "45",
			"point": "До зоны формирования устойчивой эмульсии", "periodicity": "Непрерывно",
			"control": "Вязкость, обводнённость, разделение фаз и МРП",
		}
	case strings.Contains(n, "фильтр") || strings.Contains(n, "сепарац") || strings.Contains(n, "мехприм"):
		values = map[string]string{
			"mode":      "Фильтрация / сепарация мехпримесей по учебному режиму",
			"duration":  "На весь межремонтный период с контрольными осмотрами",
			"equipment": "Совместимость с УЭЦН подтверждается в учебном сценарии",
			"stop":      "Рост перепада / снижение дебита / достижение контрольного срока",
			"control":   "КВЧ + перепад давления + состояние фильтрующего узла + МРП",
			"note":      "Синтетические параметры сквозного примера; не производственный регламент",
		}
	}
	for i := range fields {
		if value := strings.TrimSpace(values[fields[i].Key]); value != "" {
			fields[i].Suggested = value
			fields[i].Value = value
			fields[i].Changed = false
		}
	}
	return fields
}

func methodImplementationEffectiveRecord(objectKey string, s methodWorkflowSuggestion) MethodImplementationRecord {
	kind, fields := methodImplementationSuggestedFields(s)
	fields = teachingMethodImplementationFields(objectKey, s, fields)
	r := MethodImplementationRecord{ObjectKey: strings.TrimSpace(objectKey), Method: strings.TrimSpace(s.Method.Name), Kind: kind, Fields: fields}
	idx := methodImplementationRecordIndex(objectKey, s.Method.Name)
	if idx < 0 {
		return r
	}
	stored := methodImplementationRecords[idx]
	byKey := map[string]methodImplementationField{}
	for _, f := range stored.Fields {
		byKey[f.Key] = f
	}
	for i := range r.Fields {
		if old, ok := byKey[r.Fields[i].Key]; ok && strings.TrimSpace(old.Value) != "" {
			r.Fields[i].Value = strings.TrimSpace(old.Value)
			r.Fields[i].Changed = r.Fields[i].Value != r.Fields[i].Suggested
		}
	}
	r.UpdatedAt = stored.UpdatedAt
	r.UpdatedBy = stored.UpdatedBy
	return r
}

func methodImplementationSuggestionForName(objectKey, method string) (methodWorkflowSuggestion, bool) {
	for _, s := range methodWorkflowSuggestions {
		if normalizeMethodText(s.Method.Name) == normalizeMethodText(method) {
			return s, true
		}
	}
	for _, s := range methodSuggestionsForObject(objectKey) {
		if normalizeMethodText(s.Method.Name) == normalizeMethodText(method) {
			return s, true
		}
	}
	return methodWorkflowSuggestion{}, false
}

func methodImplementationSnapshotForName(objectKey, method string) MethodImplementationRecord {
	if s, ok := methodImplementationSuggestionForName(objectKey, method); ok {
		return methodImplementationEffectiveRecord(objectKey, s)
	}
	fallback := methodWorkflowSuggestion{Method: assessmentMethodCandidate{Name: method}}
	return methodImplementationEffectiveRecord(objectKey, fallback)
}

func methodImplementationChangedCount(r MethodImplementationRecord) int {
	n := 0
	for _, f := range r.Fields {
		if f.Changed {
			n++
		}
	}
	return n
}

func methodImplementationSummaryText(objectKey string, s methodWorkflowSuggestion) string {
	r := methodImplementationEffectiveRecord(objectKey, s)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\r\n", strings.ToUpper(r.Kind))
	limit := len(r.Fields)
	for i := 0; i < limit; i++ {
		f := r.Fields[i]
		mark := ""
		if f.Changed {
			mark = " [изменено]"
		}
		fmt.Fprintf(&b, "%s: %s%s\r\n", f.Label, dash(f.Value), mark)
	}
	if len(r.Fields) > limit {
		fmt.Fprintf(&b, "+ ещё %d параметр(а/ов) — «ИЗМЕНИТЬ»\r\n", len(r.Fields)-limit)
	}
	if n := methodImplementationChangedCount(r); n > 0 {
		fmt.Fprintf(&b, "Ручных изменений: %d; они фиксируются в журнале.", n)
	} else {
		b.WriteString("Пока используются предложения РОСА; технолог может скорректировать их вручную.")
	}
	return b.String()
}

func methodImplementationAuditUser() string {
	if currentSession != nil && strings.TrimSpace(currentSession.DisplayName) != "" {
		return strings.TrimSpace(currentSession.DisplayName)
	}
	return "Неавторизованный профиль"
}

func appendMethodImplementationAudit(entry methodImplementationAuditEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := methodImplementationAuditPath()
	if err := os.MkdirAll(filepathDir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// filepathDir is intentionally tiny to keep the audit writer independent of UI code.
func filepathDir(path string) string {
	i := strings.LastIndexAny(path, `\\/`)
	if i < 0 {
		return "."
	}
	return path[:i]
}

func saveMethodImplementationValues(objectKey string, s methodWorkflowSuggestion, values []string) error {
	kind, suggested := methodImplementationSuggestedFields(s)
	if len(values) != len(suggested) {
		return fmt.Errorf("неверное число параметров реализации")
	}
	before := methodImplementationEffectiveRecord(objectKey, s)
	fields := make([]methodImplementationField, len(suggested))
	for i := range suggested {
		v := strings.TrimSpace(values[i])
		if v == "" {
			return fmt.Errorf("заполните поле «%s»", suggested[i].Label)
		}
		fields[i] = suggested[i]
		fields[i].Value = v
		fields[i].Changed = v != fields[i].Suggested
	}
	now := time.Now().Format("02.01.2006 15:04")
	user := methodImplementationAuditUser()
	record := MethodImplementationRecord{ObjectKey: strings.TrimSpace(objectKey), Method: strings.TrimSpace(s.Method.Name), Kind: kind, UpdatedAt: now, UpdatedBy: user, Fields: fields}

	ensureMethodImplementationRecordsLoaded()
	backup := append([]MethodImplementationRecord(nil), methodImplementationRecords...)
	idx := methodImplementationRecordIndex(objectKey, s.Method.Name)
	if idx >= 0 {
		methodImplementationRecords[idx] = record
	} else {
		methodImplementationRecords = append(methodImplementationRecords, record)
	}
	if err := saveJSON(methodImplementationPath(), methodImplementationRecords); err != nil {
		methodImplementationRecords = backup
		return err
	}

	beforeByKey := map[string]string{}
	for _, f := range before.Fields {
		beforeByKey[f.Key] = f.Value
	}
	changes := make([]methodImplementationAuditChange, 0, len(fields))
	for _, f := range fields {
		if beforeByKey[f.Key] == f.Value {
			continue
		}
		changes = append(changes, methodImplementationAuditChange{Key: f.Key, Label: f.Label, Suggested: f.Suggested, Before: beforeByKey[f.Key], After: f.Value, Changed: f.Changed})
	}
	if len(changes) == 0 {
		return nil
	}
	entry := methodImplementationAuditEntry{ObjectKey: strings.TrimSpace(objectKey), Method: strings.TrimSpace(s.Method.Name), Kind: kind, ChangedAt: now, ChangedBy: user, Changes: changes}
	if err := appendMethodImplementationAudit(entry); err != nil {
		methodImplementationRecords = backup
		if rollbackErr := saveJSON(methodImplementationPath(), methodImplementationRecords); rollbackErr != nil {
			return fmt.Errorf("журнал параметров не сохранён: %v; откат параметров также не выполнен: %v", err, rollbackErr)
		}
		return fmt.Errorf("журнал параметров не сохранён, изменения отменены: %v", err)
	}
	if decision, ok := latestApprovedMethodDecision(objectKey); ok && approvedDecisionContainsMethod(decision, s.Method.Name) {
		if err := markEconomicsNeedsRecalcForDecision(decision, "изменены параметры реализации утверждённого метода"); err != nil {
			return fmt.Errorf("параметры сохранены, но статус экономики не обновлён: %v", err)
		}
	}
	return nil
}

func drawMethodImplementationButton(di *DRAWITEMSTRUCT) uintptr {
	if di == nil {
		return 0
	}
	id := int(di.CtlID)
	fill := brushWhite
	border := brushBorder
	fg := rgb(15, 48, 82)
	font := methodsFont(methodsTypeBody, true)

	switch id {
	case methodsImplementationEditorChooseScheme:
		fill = brushNav
		border = brushNav
		fg = rgb(255, 255, 255)
	case methodsImplementationEditorSave, methodsImplementationSchemeApply:
		fill = brushHeader
		border = brushHeader
		fg = rgb(255, 255, 255)
	case methodsImplementationEditorSaveScheme:
		border = brushNav
	case methodsImplementationEditorCancel, methodsImplementationSchemeCancel:
		fg = rgb(43, 46, 50)
	}
	if di.ItemState&ODS_SELECTED != 0 {
		fill = brushGraphitePressed
		border = brushGraphitePressed
		fg = rgb(255, 255, 255)
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(fill))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(border))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), fg)
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	caption := getText(di.HwndItem)
	r := di.RcItem
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return 1
}

func methodImplementationEditorGroupTitles() [3]string {
	switch methodImplementationEditorKind {
	case "Дозирование":
		return [3]string{"РЕАГЕНТ И ДОЗИРОВАНИЕ", "ВВОД И ПЕРИОДИЧНОСТЬ", "КОНТРОЛЬ РЕЗУЛЬТАТА"}
	case "Промывка / обработка":
		return [3]string{"СОСТАВ И ОБЪЁМ", "ПОСЛЕДОВАТЕЛЬНОСТЬ И КОНТАКТ", "КОНТРОЛЬ РЕЗУЛЬТАТА"}
	default:
		return [3]string{"РЕЖИМ И ДЛИТЕЛЬНОСТЬ", "ОГРАНИЧЕНИЯ И ПРЕКРАЩЕНИЕ", "КОНТРОЛЬ И ДОПОЛНИТЕЛЬНЫЕ УСЛОВИЯ"}
	}
}

func methodImplementationEditorSlot(index int, key string) int {
	if methodImplementationEditorKind == "Дозирование" {
		switch key {
		case "reagent":
			return 0
		case "basis":
			return 1
		case "unit":
			return 2
		case "rate":
			return 3
		case "point":
			return 4
		case "periodicity":
			return 5
		case "control":
			return 6
		}
	}
	return index
}

func methodImplementationReagentSummaryText() string {
	if methodImplementationReagentIndex < 0 || methodImplementationEditorEdits[methodImplementationReagentIndex] == 0 {
		return "Для этого типа реализации отдельная схема реагента не требуется."
	}
	values := map[string]string{}
	changed := 0
	for i, field := range methodImplementationEditorFields {
		if methodImplementationEditorEdits[i] == 0 {
			continue
		}
		value := strings.TrimSpace(getText(methodImplementationEditorEdits[i]))
		values[field.Key] = value
		if value != strings.TrimSpace(field.Suggested) {
			changed++
		}
	}
	name := values["reagent"]
	if name == "" || strings.EqualFold(name, "Не выбрано") {
		return "Текущая схема: не выбрана\r\nЗаполните параметры вручную или выберите схему химизации из справочника."
	}
	status := "без ручных изменений"
	if changed > 0 {
		status = fmt.Sprintf("ручных изменений: %d", changed)
	}
	rate := values["rate"]
	if unit := values["unit"]; unit != "" && !strings.EqualFold(unit, "Не выбрано") {
		rate = strings.TrimSpace(rate + " " + unit)
	}
	return fmt.Sprintf("Текущая схема: %s\r\nСтатус: %s\r\nБаза: %s\r\nДозирование: %s\r\nТочка: %s\r\nПериодичность: %s\r\nКонтроль: %s",
		name, status, dash(values["basis"]), dash(rate), dash(values["point"]), dash(values["periodicity"]), dash(values["control"]))
}

func refreshMethodImplementationReagentSummary() {
	if methodImplementationReagentSummary != 0 {
		text := methodImplementationReagentSummaryText()
		setText(methodImplementationReagentSummary, text)
		// RC646: use exactly the same body typography as the editable fields above.
		// Do not call formatMethodsReferenceText here: it promoted uppercase text
		// to a large heading and made section 04 visually inconsistent.
		pSendMessageW.Call(uintptr(methodImplementationReagentSummary), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
		pSendMessageW.Call(uintptr(methodImplementationReagentSummary), EM_SETSEL, 0, 0)
	}
}

func relayoutMethodImplementationReferenceSection(hwnd uintptr) {
	if methodImplementationReferencePanel == 0 {
		return
	}
	panelY := int32(530)
	panelH := int32(210)
	summaryH := int32(120)
	noteY := int32(748)
	bottomY := int32(804)
	buttonY := int32(820)
	toggleText := "РАЗВЕРНУТЬ БЛОК"
	if methodImplementationReferenceExpanded {
		panelH = 306
		summaryH = 216
		noteY = 844
		bottomY = 900
		buttonY = 916
		toggleText = "СВЕРНУТЬ БЛОК"
	}
	setPos(methodImplementationReferencePanel, 20, panelY, 1064, panelH)
	setPos(methodImplementationReferenceAccent, 20, panelY, 4, panelH)
	setPos(methodImplementationActionsTitle, 36, 538, 748, 26)
	if methodImplementationReferenceToggleButton != 0 {
		setPos(methodImplementationReferenceToggleButton, 804, 534, 264, 32)
		setText(methodImplementationReferenceToggleButton, toggleText)
	}
	setPos(methodImplementationReagentSummary, 36, 570, 500, summaryH)
	if methodImplementationSchemeChooseButton != 0 {
		setPos(methodImplementationSchemeChooseButton, 558, 574, 510, 50)
	}
	if methodImplementationSchemeSaveButton != 0 {
		setPos(methodImplementationSchemeSaveButton, 558, 636, 510, 50)
	}
	setPos(methodImplementationNoteBand, 20, noteY, 1064, 42)
	setPos(methodImplementationNote, 36, noteY+4, 1032, 32)
	setPos(methodImplementationBottomRule, 20, bottomY, 1064, 1)
	if methodImplementationResetButton != 0 {
		setPos(methodImplementationResetButton, 20, buttonY, 320, 48)
	}
	if methodImplementationCancelButton != 0 {
		setPos(methodImplementationCancelButton, 690, buttonY, 166, 48)
	}
	if methodImplementationSaveButton != 0 {
		setPos(methodImplementationSaveButton, 866, buttonY, 218, 48)
	}
	// Compact mode fits the action row without clipping. Expanded mode gets
	// enough virtual height to scroll below the buttons instead of cutting them.
	methodImplementationEditorContentHeightCurrent = buttonY + 128
	methodImplementationEditorUpdateScroll(hwnd)
}

var methodImplementationEditorContentHeightCurrent int32 = 948

const methodImplementationEditorContentHeight int32 = 980

func methodImplementationEditorSetScroll(hwnd uintptr, target int32) {
	if target < 0 {
		target = 0
	}
	if target > methodImplementationEditorScrollMax {
		target = methodImplementationEditorScrollMax
	}
	if target == methodImplementationEditorScrollPos {
		return
	}
	delta := methodImplementationEditorScrollPos - target
	methodImplementationEditorScrollPos = target
	methodImplementationScrollWindowEx.Call(hwnd, 0, uintptr(int64(delta)), 0, 0, 0, 0, 0x0001|0x0002|0x0004) // SW_SCROLLCHILDREN | SW_INVALIDATE | SW_ERASE
	pSetScrollPos.Call(hwnd, 1, uintptr(target), 1)                                                           // SB_VERT
	pInvalidateRect.Call(hwnd, 0, 1)
}

func methodImplementationEditorUpdateScroll(hwnd uintptr) {
	var rc RECT
	if r, _, _ := pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	clientH := rc.Bottom - rc.Top
	maxScroll := methodImplementationEditorContentHeightCurrent - clientH
	if maxScroll < 0 {
		maxScroll = 0
	}
	methodImplementationEditorScrollMax = maxScroll
	pSetScrollRange.Call(hwnd, 1, 0, uintptr(maxScroll), 1) // SB_VERT
	if methodImplementationEditorScrollPos > maxScroll {
		methodImplementationEditorSetScroll(hwnd, maxScroll)
	} else {
		pSetScrollPos.Call(hwnd, 1, uintptr(methodImplementationEditorScrollPos), 1)
	}
}

func methodImplementationEditorWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		methodImplementationHeaderBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1104, 94, parent, 0)
		methodImplementationAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 8, 94, parent, 0)
		methodImplementationTitle = modalCreate("STATIC", "ПАРАМЕТРЫ РЕАЛИЗАЦИИ", WS_CHILD|WS_VISIBLE|SS_LEFT, 32, 14, 570, 32, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationTitle), WM_SETFONT, uintptr(methodsFont(methodsTypeSection, true)), 1)
		methodImplementationSubtitle = modalCreate("STATIC", methodImplementationEditorMethod+" · "+methodImplementationEditorKind, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_ENDELLIPSIS, 32, 51, 570, 26, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationSubtitle), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
		objectCaption := "ОБЪЕКТ ·\r\n" + readableObjectKey(methodImplementationEditorObject)
		methodImplementationObjectBadge = modalCreate("STATIC", objectCaption, WS_CHILD|WS_VISIBLE|SS_CENTER|SS_NOPREFIX, 620, 10, 454, 70, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationObjectBadge), WM_SETFONT, uintptr(methodsFont(methodsTypeCompact, true)), 1)

		groupTitles := methodImplementationEditorGroupTitles()
		groupRects := [3][4]int32{{20, 108, 1064, 188}, {20, 306, 1064, 116}, {20, 432, 1064, 88}}
		if methodImplementationEditorKind == "Дозирование" {
			// RC640: the promised visual redesign is explicit on screen. Reagent /
			// dosage and injection / periodicity are independent cards side-by-side;
			// result control is a separate full-width card below them.
			groupRects = [3][4]int32{{20, 108, 514, 238}, {546, 108, 538, 238}, {20, 358, 1064, 162}}
		}
		for i, r := range groupRects {
			methodImplementationGroupPanels[i] = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER, r[0], r[1], r[2], r[3], parent, 0)
			methodImplementationGroupAccents[i] = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, r[0], r[1], 4, r[3], parent, 0)
			methodImplementationGroupTitles[i] = modalCreate("STATIC", groupTitles[i], WS_CHILD|WS_VISIBLE|SS_LEFT, r[0]+16, r[1]+10, r[2]-32, 24, parent, 0)
			pSendMessageW.Call(uintptr(methodImplementationGroupTitles[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeCompact, true)), 1)
		}
		methodImplementationFieldsPanel = methodImplementationGroupPanels[0]
		methodImplementationFieldsAccent = methodImplementationGroupAccents[0]
		methodImplementationSectionTitle = methodImplementationGroupTitles[0]
		sectionRuleW := int32(1032)
		if methodImplementationEditorKind == "Дозирование" {
			sectionRuleW = 482
		}
		methodImplementationSectionRule = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 36, 140, sectionRuleW, 1, parent, 0)
		methodImplementationReagentIndex = -1
		for i := 0; i < methodsImplementationFieldCount && i < len(methodImplementationEditorFields); i++ {
			f := methodImplementationEditorFields[i]
			if f.Key == "reagent" {
				methodImplementationReagentIndex = i
			}
			caption := f.Label
			if f.Changed {
				caption += " · изменено"
			}
			slot := methodImplementationEditorSlot(i, f.Key)
			var x, labelY, width int32
			if methodImplementationEditorKind == "Дозирование" {
				switch {
				case slot < 4:
					// RC641: four reagent/dosage fields are stacked full-width.
					// Long values such as «По лабораторному и промысловому подбору»
					// are now readable in the closed combobox instead of being clipped.
					x = 40
					labelY = 147 + int32(slot)*47
					width = 474
				case slot < 6:
					x = 566
					labelY = 151 + int32(slot-4)*84
					width = 498
				default:
					x = 40
					labelY = 398
					width = 1024
				}
			} else {
				switch {
				case slot < 4:
					x = 40 + int32(slot%2)*518
					labelY = 151 + int32(slot/2)*68
					width = 492
				case slot < 6:
					x = 40 + int32(slot-4)*518
					labelY = 340
					width = 492
				default:
					x = 40
					labelY = 458
					width = 1018
				}
			}
			methodImplementationEditorLabels[i] = modalCreate("STATIC", caption, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_CENTERIMAGE, x, labelY, width, 20, parent, 0)
			comboStyle := uint32(WS_CHILD | WS_VISIBLE | WS_BORDER | WS_TABSTOP | WS_VSCROLL | CBS_DROPDOWN)
			if f.Key == "unit" {
				comboStyle = WS_CHILD | WS_VISIBLE | WS_BORDER | WS_TABSTOP | WS_VSCROLL | CBS_DROPDOWNLIST
			}
			methodImplementationEditorEdits[i] = modalCreate("COMBOBOX", "", comboStyle, x, labelY+23, width, 280, parent, methodsImplementationEditorBase+i)
			if hFont != 0 {
				pSendMessageW.Call(uintptr(methodImplementationEditorLabels[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
				pSendMessageW.Call(uintptr(methodImplementationEditorEdits[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
			}
			fillMethodImplementationCombo(methodImplementationEditorEdits[i], f)
		}
		methodImplementationReferencePanel = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE|WS_BORDER, 20, 530, 1064, 210, parent, 0)
		methodImplementationReferenceAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 20, 530, 4, 210, parent, 0)
		methodImplementationActionsTitle = modalCreate("STATIC", "04 · РЕАГЕНТЫ · СПРАВОЧНИК СХЕМ ХИМИЗАЦИИ", WS_CHILD|WS_VISIBLE|SS_LEFT, 36, 538, 1018, 22, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationActionsTitle), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, true)), 1)
		methodImplementationReagentSummary = modalCreate("RichEdit20W", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_READONLY|WS_VSCROLL|ES_AUTOVSCROLL, 36, 565, 500, 120, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationReagentSummary), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
		pSendMessageW.Call(uintptr(methodImplementationReagentSummary), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		if methodImplementationReagentIndex >= 0 {
			methodImplementationSchemeChooseButton = modalCreate("BUTTON", "ВЫБРАТЬ СХЕМУ ХИМИЗАЦИИ ИЗ СПРАВОЧНИКА", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 558, 568, 510, 48, parent, methodsImplementationEditorChooseScheme)
			methodImplementationSchemeSaveButton = modalCreate("BUTTON", "СОХРАНИТЬ ТЕКУЩУЮ СХЕМУ В СПРАВОЧНИК", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 558, 626, 510, 48, parent, methodsImplementationEditorSaveScheme)
		} else {
			methodImplementationSchemeSaveButton = 0
			methodImplementationSchemeChooseButton = 0
		}
		refreshMethodImplementationReagentSummary()
		methodImplementationReferenceToggleButton = modalCreate("BUTTON", "РАЗВЕРНУТЬ БЛОК", WS_CHILD|WS_VISIBLE|WS_TABSTOP, 804, 534, 264, 32, parent, methodsImplementationReferenceToggle)
		pSendMessageW.Call(uintptr(methodImplementationReferenceToggleButton), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, true)), 1)
		methodImplementationReferenceExpanded = false
		relayoutMethodImplementationReferenceSection(hwnd)

		methodImplementationNoteBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 20, 844, 1064, 42, parent, 0)
		methodImplementationNote = modalCreate("STATIC", "Справочник хранит всю совокупность параметров схемы. Точный дубль не сохраняется — программа покажет предупреждение.", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_CENTERIMAGE, 36, 850, 1032, 30, parent, 0)
		pSendMessageW.Call(uintptr(methodImplementationNote), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)

		methodImplementationBottomRule = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 20, 900, 1064, 1, parent, 0)
		methodImplementationResetButton = modalCreate("BUTTON", "ВОССТАНОВИТЬ ПРЕДЛОЖЕННЫЕ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 20, 820, 320, 48, parent, methodsImplementationEditorReset)
		methodImplementationCancelButton = modalCreate("BUTTON", "ОТМЕНА", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 690, 820, 166, 48, parent, methodsImplementationEditorCancel)
		methodImplementationSaveButton = modalCreate("BUTTON", "СОХРАНИТЬ ПАРАМЕТРЫ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 866, 820, 218, 48, parent, methodsImplementationEditorSave)
		for _, h := range []syscall.Handle{methodImplementationSchemeSaveButton, methodImplementationSchemeChooseButton, methodImplementationResetButton, methodImplementationCancelButton, methodImplementationSaveButton} {
			if h == 0 {
				continue
			}
			pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, true)), 1)
		}
		// Run layout once more after the footer controls exist; otherwise a
		// compact 04 section could leave its note/rule at expanded coordinates.
		relayoutMethodImplementationReferenceSection(hwnd)
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil {
			switch int(di.CtlID) {
			case methodsImplementationEditorSaveScheme, methodsImplementationEditorChooseScheme, methodsImplementationEditorReset, methodsImplementationEditorCancel, methodsImplementationEditorSave:
				return drawMethodImplementationButton(di)
			}
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case methodImplementationHeaderBand:
			return uintptr(brushNav)
		case methodImplementationAccent:
			return uintptr(brushHeader)
		case methodImplementationFieldsPanel:
			return uintptr(brushWhite)
		case methodImplementationFieldsAccent:
			return uintptr(brushHeader)
		case methodImplementationReferencePanel:
			return uintptr(brushWhite)
		case methodImplementationReferenceAccent:
			return uintptr(brushHeader)
		case methodImplementationSectionRule:
			return uintptr(brushHeader)
		case methodImplementationBottomRule:
			return uintptr(brushBorder)
		case methodImplementationNoteBand:
			return uintptr(brushWhite)
		case methodImplementationTitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodImplementationSubtitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		case methodImplementationObjectBadge:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodImplementationNote:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		case methodImplementationSectionTitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		case methodImplementationActionsTitle, methodImplementationReagentSummary:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushWhite)
		}
		for i := range methodImplementationGroupPanels {
			if child == methodImplementationGroupPanels[i] && child != 0 {
				return uintptr(brushWhite)
			}
			if child == methodImplementationGroupAccents[i] && child != 0 {
				return uintptr(brushHeader)
			}
			if child == methodImplementationGroupTitles[i] && child != 0 {
				pSetBkMode.Call(wParam, TRANSPARENT)
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
		for _, label := range methodImplementationEditorLabels {
			if child == label && label != 0 {
				pSetBkMode.Call(wParam, TRANSPARENT)
				pSetTextColor.Call(wParam, rgb(43, 46, 50))
				return uintptr(brushWhite)
			}
		}
	case WM_COMMAND:
		commandID := int(loWord(wParam))
		notify := int(hiWord(wParam))
		if commandID >= methodsImplementationEditorBase && commandID < methodsImplementationEditorBase+methodsImplementationFieldCount && (notify == 1 || notify == 5) {
			fieldIndex := commandID - methodsImplementationEditorBase
			if fieldIndex >= 0 && fieldIndex < len(methodImplementationEditorFields) && methodImplementationEditorFields[fieldIndex].Key == "basis" {
				refreshMethodImplementationUnitCombo()
			}
			refreshMethodImplementationReagentSummary()
		}
		switch commandID {
		case methodsImplementationEditorReset:
			for i := 0; i < len(methodImplementationEditorFields) && i < methodsImplementationFieldCount; i++ {
				field := methodImplementationEditorFields[i]
				field.Value = field.Suggested
				fillMethodImplementationCombo(methodImplementationEditorEdits[i], field)
			}
			refreshMethodImplementationReagentSummary()
			return 0
		case methodsImplementationEditorSaveScheme:
			scheme, err := methodChemicalSchemeFromEditor()
			if err != nil {
				msg("Справочник схем химизации", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			if err := saveMethodChemicalScheme(scheme); err != nil {
				msg("Справочник схем химизации", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			for i, field := range scheme.Fields {
				fillMethodImplementationCombo(methodImplementationEditorEdits[i], field)
			}
			msg("Справочник схем химизации", "Полная схема химизации сохранена и доступна для выбора.", MB_OK|MB_ICONINFORMATION)
			return 0
		case methodsImplementationEditorChooseScheme:
			openMethodChemicalSchemeChooser(hwnd)
			refreshMethodImplementationReagentSummary()
			return 0
		case methodsImplementationReferenceToggle:
			methodImplementationReferenceExpanded = !methodImplementationReferenceExpanded
			relayoutMethodImplementationReferenceSection(hwnd)
			return 0
		case methodsImplementationEditorCancel:
			methodImplementationEditorDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case methodsImplementationEditorSave:
			values := make([]string, len(methodImplementationEditorFields))
			for i := range methodImplementationEditorFields {
				values[i] = getText(methodImplementationEditorEdits[i])
			}
			s, ok := methodImplementationSuggestionForName(methodImplementationEditorObject, methodImplementationEditorMethod)
			if !ok {
				msg("Параметры реализации", "Выбранный метод больше не найден в текущем наборе кандидатов. Обновите раздел и повторите.", MB_OK|MB_ICONWARNING)
				return 0
			}
			if err := saveMethodImplementationValues(methodImplementationEditorObject, s, values); err != nil {
				msg("Параметры реализации", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			methodImplementationEditorDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		methodImplementationEditorDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openMethodsImplementationEditor() {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		msg("Параметры реализации", "Сначала закрепите объект на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	s, ok := methodsSelectedSuggestion()
	if !ok || strings.TrimSpace(s.Method.Name) == "" {
		msg("Параметры реализации", "Сначала выберите метод-кандидат.", MB_OK|MB_ICONWARNING)
		return
	}
	r := methodImplementationEffectiveRecord(key, s)
	methodImplementationEditorObject = key
	methodImplementationEditorMethod = r.Method
	methodImplementationEditorKind = r.Kind
	methodImplementationEditorFields = append(methodImplementationEditorFields[:0], r.Fields...)
	methodImplementationEditorDone = false
	methodImplementationEditorScrollPos = 0
	methodImplementationEditorScrollMax = 0
	methodImplementationReferenceExpanded = false
	methodImplementationResetButton = 0
	methodImplementationCancelButton = 0
	methodImplementationSaveButton = 0
	for i := range methodImplementationEditorEdits {
		methodImplementationEditorEdits[i] = 0
		methodImplementationEditorLabels[i] = 0
	}

	cls := p("ROSAMethodImplementationEditorV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodImplementationEditorWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — Параметры реализации"))), WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_VSCROLL, 0x80000000, 0x80000000, 1120, 980, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Параметры реализации", "Не удалось открыть редактор параметров.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !methodImplementationEditorDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	refreshMethodsSuggestionInfo()
	refreshMethodsVisualTables()
}

func methodImplementationReportText(r MethodImplementationRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\r\n", r.Method, r.Kind)
	for _, f := range r.Fields {
		mark := ""
		if f.Changed {
			mark = " [изменено технологом]"
		}
		fmt.Fprintf(&b, "• %s: %s%s\r\n", f.Label, dash(f.Value), mark)
	}
	if strings.TrimSpace(r.UpdatedAt) != "" {
		fmt.Fprintf(&b, "Последняя ручная корректировка: %s; %s.\r\n", r.UpdatedAt, dash(r.UpdatedBy))
	}
	return b.String()
}
