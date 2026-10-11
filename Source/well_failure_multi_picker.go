//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	idFailurePickerList = 9840 + iota
	idFailurePickerEntry
	idFailurePickerAdd
	idFailurePickerApply
	idFailurePickerCancel
)

var failurePickerDone bool
var failurePickerBrowse bool
var failurePickerCategory, failurePickerHeading syscall.Handle
var failurePickerCategories []failureCatalogCategory
var failurePickerField syscall.Handle
var failurePickerKey, failurePickerTitle string
var failurePickerOptions []string
var failurePickerList, failurePickerEntry syscall.Handle
var failureCustomOptions map[string][]string

func failurePickerFieldInfo() (syscall.Handle, string, string) {
	h := wellActiveFailureField
	for i, field := range wellFormFields {
		if field == h && wellCoreSection(i) == wellFormSection {
			catalogs := map[int]string{3: "pump", 4: "pump", 5: "node_all", 6: "sign_all", 9: "comp", 12: "status", 15: "comp", 16: "brand_all", 17: "size_all"}
			return h, "core_" + strings.TrimSpace(wellFieldCaptions[i]), catalogs[i]
		}
	}
	for i, field := range wellExtraFields {
		if field == h && wellExtraSpecs[i].section == wellFormSection {
			spec := wellExtraSpecs[i]
			return h, "extra_" + spec.column, spec.catalog
		}
	}
	return 0, "", ""
}
func failurePickerStoredOptions(key string) []string {
	if failureCustomOptions == nil {
		failureCustomOptions = map[string][]string{}
		data, err := os.ReadFile(sharedPath("failure_custom_catalog.json"))
		if err == nil {
			_ = json.Unmarshal(data, &failureCustomOptions)
		}
	}
	return failureCustomOptions[key]
}
func failurePickerSuggestions(key, catalog string) []string {
	values := append([]string{}, wellFailureCatalog[wellCatalogName(catalog)]...)
	values = append(values, failurePickerStoredOptions(key)...)
	if key == "core_Код причины по классификатору" {
		for _, cause := range wellCauseChoices {
			values = append(values, cause.Code)
		}
	}
	if key == "core_Причина по классификатору" {
		for _, cause := range wellCauseChoices {
			values = append(values, cause.Title)
		}
	}

	if strings.HasPrefix(key, "core_") {
		for _, e := range failureEvents {
			switch key {
			case "core_Осмотр / дефектовка":
				values = append(values, e.Inspection)
			case "core_Заключение комиссии / акт":
				values = append(values, e.CommissionConclusion)
			case "core_Утверждённое заключение":
				values = append(values, e.ApprovedConclusion)
			case "core_Предварительное заключение":
				values = append(values, e.PreliminaryConclusion)
			case "core_Код причины по классификатору":
				values = append(values, e.SourceCauseCode)
			case "core_Причина по классификатору":
				values = append(values, e.SourceCause)
			case "core_Источник / документ":
				values = append(values, e.Source)
			case "core_Комментарий":
				values = append(values, e.Comment)
			case "core_Дата отказа / окончания *":
				values = append(values, e.Date)
			case "core_Дата пуска *":
				values = append(values, e.CycleStartDate)
			case "core_Дата окончания ремонта":
				values = append(values, e.RepairEndDate)
			case "core_Признак / предпосылка *":
				values = append(values, e.StopCause)
			case "core_Способ эксплуатации *":
				values = append(values, e.OperationMode)
			}
		}
	} else if strings.HasPrefix(key, "extra_") {
		column := strings.TrimPrefix(key, "extra_")
		for _, e := range failureEvents {
			values = append(values, e.Extra[column])
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || v == wellChoicePrompt || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}
func failurePickerAddValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, existing := range failurePickerOptions {
		if strings.EqualFold(strings.Join(strings.Fields(existing), " "), strings.Join(strings.Fields(value), " ")) {
			msg("Справочник", "Такое значение уже есть в этом разделе.", MB_OK|MB_ICONINFORMATION)
			return false
		}
	}
	current := failurePickerStoredOptions(failurePickerKey)
	updated := make(map[string][]string, len(failureCustomOptions))
	for key, items := range failureCustomOptions {
		updated[key] = append([]string(nil), items...)
	}
	updated[failurePickerKey] = append(append([]string(nil), current...), value)
	data, err := json.MarshalIndent(updated, "", "  ")
	if err == nil {
		err = os.WriteFile(sharedPath("failure_custom_catalog.json"), data, 0600)
	}
	if err != nil {
		msg("Справочник", "Не удалось сохранить значение: "+err.Error(), MB_OK|MB_ICONERROR)
		return false
	}
	failureCustomOptions = updated
	failurePickerOptions = append(failurePickerOptions, value)
	pSendMessageW.Call(uintptr(failurePickerList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(value))))
	return true
}

func failurePickerWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		title := modalCreate("STATIC", failurePickerTitle, WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 20, 720, 32, parent, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		failurePickerHeading = modalCreate("STATIC", "Отметьте несколько вариантов; собственный вариант добавьте ниже.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 58, 720, 24, parent, 0)
		listTop := int32(86)
		listHeight := int32(384)
		if failurePickerBrowse {
			setText(failurePickerHeading, "Выберите раздел. Исходные значения доступны только для чтения.")
			listTop, listHeight = 128, 342
			failurePickerCategory = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 24, 85, 720, 260, parent, 9845)
			for _, category := range failurePickerCategories {
				pSendMessageW.Call(uintptr(failurePickerCategory), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(category.title))))
			}
			pSendMessageW.Call(uintptr(failurePickerCategory), CB_SETCURSEL, 0, 0)
		}
		failurePickerList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|WS_TABSTOP|0x0008, 24, listTop, 720, listHeight, parent, idFailurePickerList) // LBS_MULTIPLESEL
		for _, v := range failurePickerOptions {
			pSendMessageW.Call(uintptr(failurePickerList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(v))))
		}
		if len(failurePickerOptions) == 0 {
			modalCreate("STATIC", "В справочнике пока нет значений. Добавьте своё ниже.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 474, 720, 22, parent, 0)
		}
		failurePickerEntry = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 24, 506, 400, 36, parent, idFailurePickerEntry)
		add := modalCreate("BUTTON", "ДОБАВИТЬ В СПРАВОЧНИК", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 436, 506, 308, 36, parent, idFailurePickerAdd)
		if hFontBold != 0 {
			pSendMessageW.Call(uintptr(add), WM_SETFONT, uintptr(hFontBold), 1)
		}
		if !failurePickerBrowse {
			modalCreate("BUTTON", "Применить выбранное", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 363, 565, 220, 42, parent, idFailurePickerApply)
		}
		modalCreate("BUTTON", "Отмена", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 596, 565, 148, 42, parent, idFailurePickerCancel)
		return 0
	case WM_CTLCOLORSTATIC:
		if syscall.Handle(lParam) == failurePickerHeading {
			pSetTextColor.Call(wParam, rgb(180, 20, 38))
			pSetBkMode.Call(wParam, TRANSPARENT)
			return uintptr(brushWhite)
		}
	case WM_COMMAND:
		if int(loWord(wParam)) == 9845 && hiWord(wParam) == 1 && failurePickerBrowse {
			index, _, _ := pSendMessageW.Call(uintptr(failurePickerCategory), CB_GETCURSEL, 0, 0)
			if int(index) >= 0 && int(index) < len(failurePickerCategories) {
				cat := failurePickerCategories[index]
				failurePickerKey = cat.key
				failurePickerOptions = failurePickerSuggestions(cat.key, cat.catalog)
				pSendMessageW.Call(uintptr(failurePickerList), LB_RESETCONTENT, 0, 0)
				for _, v := range failurePickerOptions {
					pSendMessageW.Call(uintptr(failurePickerList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(v))))
				}
			}
			return 0
		}
		switch int(loWord(wParam)) {
		case idFailurePickerAdd:
			v := strings.TrimSpace(getText(failurePickerEntry))
			if v != "" {
				if failurePickerAddValue(v) {
					setText(failurePickerEntry, "")
					pSendMessageW.Call(uintptr(failurePickerList), 0x0185, 1, uintptr(len(failurePickerOptions)-1))
				}
			}
			return 0
		case idFailurePickerApply:
			var selected []string
			for i, v := range failurePickerOptions {
				r, _, _ := pSendMessageW.Call(uintptr(failurePickerList), 0x0187, uintptr(i), 0)
				if r != 0 {
					selected = append(selected, v)
				}
			}
			if custom := strings.TrimSpace(getText(failurePickerEntry)); custom != "" {
				if failurePickerAddValue(custom) {
					selected = append(selected, custom)
				}
			}
			if len(selected) > 0 {
				setText(failurePickerField, strings.Join(selected, "; "))
			}
			failurePickerDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case idFailurePickerCancel:
			failurePickerDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		failurePickerDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
func openFailureMultiPicker() bool {
	field, key, catalog := failurePickerFieldInfo()
	if field == 0 {
		msg("Выбор из справочника", "Сначала щёлкните нужное поле, затем нажмите «Из справочника».", MB_OK|MB_ICONINFORMATION)
		return false
	}
	failurePickerBrowse = false
	failurePickerField, failurePickerKey = field, key
	failurePickerTitle = "Варианты · " + strings.TrimPrefix(key, "core_")
	if strings.HasPrefix(key, "extra_") {
		for _, spec := range wellExtraSpecs {
			if "extra_"+spec.column == key {
				failurePickerTitle = "Варианты · " + spec.title
				break
			}
		}
	}
	failurePickerOptions = failurePickerSuggestions(key, catalog)
	return openFailurePickerWindow()
}
func openFailurePickerWindow() bool {
	failurePickerDone = false
	cls := p("ROSAFailureMultiPickerV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(failurePickerWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(failurePickerTitle))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 790, 665, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return false
	}
	var msg MSG
	for !failurePickerDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	return true
}

type failureCatalogCategory struct{ title, key, catalog string }

func openFailureCatalogBrowser() {
	failurePickerBrowse = true
	failurePickerCategories = nil
	for i, title := range wellFieldCaptions {
		if i < 3 {
			continue
		}
		catalogs := map[int]string{3: "pump", 4: "pump", 5: "node_all", 6: "sign_all", 9: "comp", 12: "status", 15: "comp", 16: "brand_all", 17: "size_all"}
		failurePickerCategories = append(failurePickerCategories, failureCatalogCategory{"Основные · " + strings.TrimSuffix(title, " *"), "core_" + title, catalogs[i]})
	}
	for _, spec := range wellExtraSpecs {
		failurePickerCategories = append(failurePickerCategories, failureCatalogCategory{"Дополнительные · " + strings.TrimSuffix(spec.title, " *"), "extra_" + spec.column, spec.catalog})
	}
	failurePickerKey = failurePickerCategories[0].key
	failurePickerOptions = failurePickerSuggestions(failurePickerKey, failurePickerCategories[0].catalog)
	failurePickerTitle = "СПРАВОЧНИК ОТКАЗОВ · исходные записи защищены"
	failurePickerField = 0
	openFailurePickerWindow()
}

type failureFormCatalogCandidate struct {
	Key, Catalog, Value string
}

func failureCatalogContainsValue(key, catalog, value string) bool {
	valueNorm := strings.ToLower(failureNormalizeEntry(value))
	if valueNorm == "" || valueNorm == strings.ToLower(failureNoSelection) {
		return true
	}
	for _, existing := range failurePickerSuggestions(key, catalog) {
		if strings.ToLower(failureNormalizeEntry(existing)) == valueNorm {
			return true
		}
	}
	return false
}

func failureFormNewCatalogValues() []failureFormCatalogCandidate {
	var out []failureFormCatalogCandidate
	seen := map[string]bool{}
	add := func(key, catalog, value string) {
		if catalog == "" {
			return
		}
		for _, part := range strings.Split(value, ";") {
			part = failureNormalizeEntry(part)
			if !failureHasValue(part) || failureCatalogContainsValue(key, catalog, part) {
				continue
			}
			dedupe := strings.ToLower(key + "\x00" + part)
			if seen[dedupe] {
				continue
			}
			seen[dedupe] = true
			out = append(out, failureFormCatalogCandidate{Key: key, Catalog: catalog, Value: part})
		}
	}
	coreCatalogs := map[int]string{3: "pump", 4: "pump", 5: "node_all", 6: "sign_all", 9: "comp", 12: "status", 15: "comp", 16: "brand_all", 17: "size_all"}
	for index, catalog := range coreCatalogs {
		if wellFormFields[index] == 0 {
			continue
		}
		add("core_"+strings.TrimSpace(wellFieldCaptions[index]), catalog, getText(wellFormFields[index]))
	}
	for i, spec := range wellExtraSpecs {
		if spec.catalog == "" || wellExtraFields[i] == 0 || !wellExtraApplicable(spec.column) {
			continue
		}
		add("extra_"+spec.column, spec.catalog, getText(wellExtraFields[i]))
	}
	return out
}

func storeFailureFormCatalogValues(values []failureFormCatalogCandidate) error {
	_ = failurePickerStoredOptions("") // ensure the custom catalog is loaded
	updated := make(map[string][]string, len(failureCustomOptions))
	for key, items := range failureCustomOptions {
		updated[key] = append([]string(nil), items...)
	}
	for _, candidate := range values {
		value := failureNormalizeEntry(candidate.Value)
		duplicate := false
		for _, existing := range updated[candidate.Key] {
			if strings.EqualFold(failureNormalizeEntry(existing), value) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			updated[candidate.Key] = append(updated[candidate.Key], value)
		}
	}
	data, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(sharedPath("failure_custom_catalog.json"), data, 0600); err != nil {
		return err
	}
	failureCustomOptions = updated
	return nil
}

func confirmAndStoreFailureFormCustomValues() {
	values := failureFormNewCatalogValues()
	if len(values) == 0 {
		return
	}
	lines := make([]string, 0, minInt(len(values), 8))
	for i, value := range values {
		if i >= 8 {
			break
		}
		lines = append(lines, "• "+value.Value)
	}
	if len(values) > len(lines) {
		lines = append(lines, "… и ещё "+fmt.Sprint(len(values)-len(lines)))
	}
	question := "В форме введены новые справочные значения:\r\n\r\n" + strings.Join(lines, "\r\n") + "\r\n\r\nДобавить их в справочник для следующих записей?"
	r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(p(question))), uintptr(unsafe.Pointer(p("РОСА — пополнение справочника"))), 0x4|MB_ICONINFORMATION)
	if r != 6 {
		return
	}
	if err := storeFailureFormCatalogValues(values); err != nil {
		msg("Справочник отказов", "Не удалось сохранить новые значения: "+err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	refreshWellEditableSuggestions()
}
