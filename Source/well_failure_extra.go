//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed well_failure_catalog.json
var embeddedFailureCatalog []byte

type wellExtraSpec struct {
	column, title, catalog string
	section                int
}

const (
	ID_WELL_SECTION_BASE = 9760
	ID_WELL_CATALOG      = 9770
	ID_WELL_MANUAL_ENTRY = 9771
	ID_WELL_REFERENCE    = 9772
)

// Columns are the exact column identifiers on the supplied "История отказов" sheet.
// Derived columns and the fields already present in the basic form are excluded.
var wellExtraSpecs = [...]wellExtraSpec{
	{"H", "Типоразмер ГНО 2 (ОРД)", "size_all", 0},
	{"L", "Тип события *", "event", 0},
	{"Q", "Сопутствующее осложнение 1", "comp", 0},
	{"R", "Сопутствующее осложнение 2", "comp", 0},
	{"U", "[ДОП] ТКРС 1", "repair", 1},
	{"V", "[ДОП] ТКРС 2", "repair", 1},
	{"W", "[ДОП] Вид повреждения", "damage_all", 1},
	{"X", "[ДОП] Итоговая причина", "cause_all", 1},
	{"Z", "[ДОП] Метод до отказа 1", "method", 1},
	{"AA", "[ДОП] Метод до отказа 2", "method", 1},
	{"AB", "[ДОП] Метод после отказа 1", "method", 1},
	{"AC", "[ДОП] Метод после отказа 2", "method", 1},
	{"AD", "[ДОП] Подтверждение", "proof", 1},
	{"AF", "[ДОП] Достоверность", "confidence", 1},
	{"AG", "Тип / типоразмер НКТ", "equip_nkt", 2},
	{"AH", "Длина НКТ, м", "", 2},
	{"AI", "Год НКТ", "equip_years", 2},
	{"AJ", "СПО", "workover", 2},
	{"AK", "Состояние НКТ до спуска", "condition", 2},
	{"AL", "Тип насоса", "pump_type_all", 3},
	{"AM", "Условный диаметр / размер ШГН, мм", "nominal", 3},
	{"AN", "Производитель насоса", "equip_pump_all", 3},
	{"AO", "Глубина спуска насоса, м", "", 3},
	{"AP", "Производитель двигателя", "equip_motor_maker", 4},
	{"AQ", "Типоразмер двигателя (габарит), мм", "equip_motor_size", 4},
	{"AR", "Год двигателя", "equip_years", 4},
	{"AS", "Состояние двигателя до спуска", "condition", 4},
	{"AT", "Типоразмер кабеля (сечение), мм²", "equip_cable", 4},
	{"AU", "Длина кабеля, м", "", 4},
	{"AV", "Состояние кабеля до спуска", "condition", 4},
	{"AW", "Типоразмер удлинителя (сечение), мм²", "equip_extension", 4},
	{"AX", "Длина удлинителя, м", "", 4},
	{"AY", "Состояние удлинителя до спуска", "condition", 4},
	{"AZ", "Производитель гидрозащиты", "equip_hydro_maker", 4},
	{"BA", "Типоразмер гидрозащиты (габарит), мм", "equip_hydro_size", 4},
	{"BB", "Год гидрозащиты", "equip_years", 4},
	{"BC", "Состояние гидрозащиты до спуска", "condition", 4},
	{"BD", "Производитель ТМС", "equip_tms_maker", 4},
	{"BE", "Типоразмер ТМС", "equip_tms_size", 4},
	{"BF", "Год ТМС", "equip_years", 4},
	{"BG", "Состояние ТМС до спуска", "condition", 4},
	{"BH", "Состояние штанг до спуска", "condition", 5},
	{"BI", "Производитель штанг", "equip_rod_maker", 5},
	{"BJ", "Штанги 19, шт.", "", 5},
	{"BK", "Штанги 22, шт.", "", 5},
	{"BL", "Штанги 25, шт.", "", 5},
	{"BM", "Год штанг", "equip_years", 5},
	{"BN", "СПО штанг", "", 5},
}

var wellFailureCatalog map[string][]string
var wellExtraLabels [len(wellExtraSpecs)]syscall.Handle
var wellExtraFields [len(wellExtraSpecs)]syscall.Handle
var wellSectionButtons [7]syscall.Handle
var wellCatalogButton, wellReferenceButton, wellManualEntryButton, wellFormModeNote syscall.Handle
var wellFormSection int
var wellActiveFailureField syscall.Handle

func wellExtraUsesFreeText(column string) bool {
	switch column {
	case "AH", "AO", "AU", "AX":
		return true
	case "BJ", "BK", "BL", "BN":
		return true
	}
	return false
}

func rememberWellFailureField(handle syscall.Handle) {
	if !wellFormActive || handle == 0 {
		return
	}
	for _, field := range wellFormFields {
		if handle == field {
			wellActiveFailureField = handle
			return
		}
	}
	for _, field := range wellExtraFields {
		if handle == field {
			wellActiveFailureField = handle
			return
		}
	}
}

func wellVisibleCatalogField() syscall.Handle {
	for i, spec := range wellExtraSpecs {
		if wellFormSection == spec.section && spec.catalog != "" && wellExtraFields[i] == wellActiveFailureField {
			return wellActiveFailureField
		}
	}
	for i, field := range wellFormFields {
		if wellCoreSection(i) == wellFormSection && field == wellActiveFailureField && (wellFormUsesChoice(i) || i == 3 || i == 6 || i == 16 || i == 17) {
			return field
		}
	}
	return 0
}

func wellCoreSection(index int) int {
	switch index {
	case 10, 12, 15:
		return 1
	case 11, 13, 14, 19:
		return 6
	default:
		return 0
	}
}

func wellExtraText(column string) string {
	for i, spec := range wellExtraSpecs {
		if spec.column == column && wellExtraFields[i] != 0 {
			return strings.TrimSpace(getText(wellExtraFields[i]))
		}
	}
	return ""
}

func wellCurrentOperationFamily() string {
	e := FailureEvent{
		OperationMode:  strings.TrimSpace(getText(wellFormFields[3])),
		Equipment:      strings.TrimSpace(getText(wellFormFields[4])),
		EquipmentSize:  strings.TrimSpace(getText(wellFormFields[17])),
		EquipmentSize2: wellExtraText("H"),
	}
	return failureOperationFamily(e)
}

func wellORDUsesElectricEquipment() bool {
	text := strings.ToUpper(strings.Join([]string{
		strings.TrimSpace(getText(wellFormFields[4])), strings.TrimSpace(getText(wellFormFields[17])),
		wellExtraText("H"),
	}, " "))
	return strings.Contains(text, "ЭЦН") || strings.Contains(text, "ЭВН") || strings.Contains(text, "УЭЦН") || strings.Contains(text, "УЭВН")
}

func wellORDUsesRodEquipment() bool {
	text := strings.ToUpper(strings.Join([]string{
		strings.TrimSpace(getText(wellFormFields[4])), strings.TrimSpace(getText(wellFormFields[17])),
		wellExtraText("H"),
	}, " "))
	return strings.Contains(text, "ШГН") || strings.Contains(text, "УШГН")
}

func wellExtraApplicable(column string) bool {
	family := wellCurrentOperationFamily()
	switch column {
	case "H":
		return family == "ОРД"
	case "AG", "AH", "AI", "AJ", "AK":
		return family == "ШГН" || family == "ЭЦН" || family == "ЭВН" || family == "ШВН" || family == "НПУ-ВД" || family == "УПЛД" || family == "ФОН" || family == "ОРД"
	case "AL", "AM", "AN", "AO":
		return family == "ШГН" || family == "ЭЦН" || family == "ЭВН" || family == "ШВН" || family == "НПУ-ВД" || family == "УПЛД" || family == "ОРД"
	case "AP", "AQ", "AR", "AS", "AT", "AU", "AV", "AW", "AX", "AY", "AZ", "BA", "BB", "BC", "BD", "BE", "BF", "BG":
		return family == "ЭЦН" || family == "ЭВН" || (family == "ОРД" && wellORDUsesElectricEquipment())
	case "BH", "BI", "BJ", "BK", "BL", "BM", "BN":
		return family == "ШГН" || (family == "ОРД" && wellORDUsesRodEquipment())
	default:
		return true
	}
}

func wellSectionApplicable(section int) bool {
	if section < 0 || section > 6 {
		return false
	}
	if section == 0 || section == 1 || section == 6 {
		return true
	}
	family := wellCurrentOperationFamily()
	switch section {
	case 2: // НКТ
		return family != ""
	case 3: // насос
		return family != "ФОН" && family != ""
	case 4: // электрика
		return family == "ЭЦН" || family == "ЭВН" || (family == "ОРД" && wellORDUsesElectricEquipment())
	case 5: // штанги
		return family == "ШГН" || (family == "ОРД" && wellORDUsesRodEquipment())
	}
	return true
}

func wellSectionHeading(section int) string {
	switch section {
	case 0:
		return "ПРЕДВАРИТЕЛЬНО · Исходные сведения, признаки, причина и первичная категория осложнения"
	case 1:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · Дополняется технологом; предварительные сведения уже перенесены"
	case 2:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · НКТ"
	case 3:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · Насос"
	case 4:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · Двигатель, кабель, удлинитель, гидрозащита и ТМС"
	case 5:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · Штанги"
	default:
		return "РАСШИРЕННОЕ РАССЛЕДОВАНИЕ · Комиссия, источник и итог"
	}
}

func loadWellFailureCatalog() {
	if err := json.Unmarshal(embeddedFailureCatalog, &wellFailureCatalog); err != nil {
		wellFailureCatalog = make(map[string][]string)
	}
}

func wellCatalogName(name string) string {
	// The reference separates equipment-specific suggestions; never mix families.
	if name == "pump_type_all" {
		family := wellCurrentOperationFamily()
		switch family {
		case "ШГН":
			return "pump_type_shgn"
		case "ЭЦН":
			return "pump_type_esp"
		case "ЭВН":
			return "pump_type_evn"
		case "ШВН":
			return "pump_type_shvn"
		case "НПУ-ВД":
			return "pump_type_npu"
		case "УПЛД":
			return "pump_type_upld"
		case "ОРД":
			return "pump_type_ord"
		default:
			return "pump_type_all"
		}
	}
	if name == "sign_all" || name == "node_all" || name == "damage_all" || name == "cause_all" || name == "size_all" || name == "brand_all" || name == "equip_pump_all" {
		pump := strings.ToUpper(firstNonEmpty(strings.TrimSpace(getText(wellFormFields[3])), strings.TrimSpace(wellChoiceValue(4))))
		if strings.Contains(pump, "ШГН") || strings.Contains(pump, "ШТАНГ") {
			return map[string]string{"sign_all": "sign_shgn", "node_all": "node_shgn", "damage_all": "damage_shgn", "cause_all": "cause_shgn", "size_all": "size_shgn", "brand_all": "brand_shgn", "equip_pump_all": "equip_pump_shgn"}[name]
		}
		if strings.Contains(pump, "ШВН") {
			return map[string]string{"sign_all": "sign_shvn", "node_all": "node_shvn", "damage_all": "damage_shvn", "cause_all": "cause_shgn", "size_all": "size_shvn", "brand_all": "brand_shvn", "equip_pump_all": "equip_pump_shvn"}[name]
		}
		if strings.Contains(pump, "ЭВН") {
			return map[string]string{"sign_all": "sign_evn", "node_all": "node_evn", "damage_all": "damage_evn", "cause_all": "cause_esp", "size_all": "size_evn", "brand_all": "brand_evn", "equip_pump_all": "equip_pump_evn"}[name]
		}
		if strings.Contains(pump, "НПУ") {
			return map[string]string{"sign_all": "sign_npu", "node_all": "node_npu", "damage_all": "damage_npu", "cause_all": "cause_npu", "size_all": "size_npu", "brand_all": "brand_npu", "equip_pump_all": "equip_pump_npu"}[name]
		}
		if strings.Contains(pump, "ЭЦН") {
			return map[string]string{"sign_all": "sign_esp", "node_all": "node_esp", "damage_all": "damage_esp", "cause_all": "cause_esp", "size_all": "size_esp", "brand_all": "brand_esp", "equip_pump_all": "equip_pump_esp"}[name]
		}
	}
	return name
}

func wellFillEditableCombo(combo syscall.Handle, options []string, value string) {
	pSendMessageW.Call(uintptr(combo), CB_RESETCONTENT, 0, 0)
	seen := make(map[string]bool)
	pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(failureNoSelection))))
	seen[strings.ToLower(failureNoSelection)] = true
	for _, option := range options {
		option = strings.TrimSpace(option)
		if option == "" || seen[strings.ToLower(option)] {
			continue
		}
		seen[strings.ToLower(option)] = true
		pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(option))))
	}
	if failureHasValue(value) {
		setText(combo, value)
	} else {
		pSendMessageW.Call(uintptr(combo), CB_SETCURSEL, 0, 0)
	}
	pSendMessageW.Call(uintptr(combo), 0x0160, 600, 0) // CB_SETDROPPEDWIDTH
}

func refreshWellEditableSuggestions() {
	for i, spec := range wellExtraSpecs {
		if wellExtraFields[i] == 0 || wellExtraUsesFreeText(spec.column) {
			continue
		}
		key := "extra_" + spec.column
		value := strings.TrimSpace(getText(wellExtraFields[i]))
		wellFillEditableCombo(wellExtraFields[i], failurePickerSuggestions(key, spec.catalog), value)
	}
	catalogs := map[int]string{3: "pump", 4: "pump", 5: "node_all", 6: "sign_all", 9: "comp", 12: "status", 15: "comp", 16: "brand_all", 17: "size_all"}
	for i, field := range wellFormFields {
		if !wellFormUsesEditableCombo(i) || field == 0 {
			continue
		}
		value := strings.TrimSpace(getText(field))
		wellFillEditableCombo(field, failurePickerSuggestions("core_"+strings.TrimSpace(wellFieldCaptions[i]), catalogs[i]), value)
	}
}
