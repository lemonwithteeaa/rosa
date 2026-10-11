//go:build windows

package main

import (
	"archive/zip"
	_ "embed"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

//go:embed РОСА_БЛАНК_ЭКОНОМИКИ.xlsx
var economicsBlankTemplateXLSX []byte

type economicsWorkbookVariant struct {
	ordinal                  int
	mrpLow, mrpBase, mrpHigh *float64
	opex, capex              *float64
}

type economicsWorkbookInput struct {
	objectKey string
	uiValues  map[string]string
	variants  []economicsWorkbookVariant
}

func chooseEconomicsTemplatePath(defaultName, title string, save bool) string {
	filter := utf16.Encode([]rune("Экономический бланк Excel (*.xlsx)\x00*.xlsx\x00Все файлы (*.*)\x00*.*\x00\x00"))
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(defaultName)))
	of := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain,
		LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)),
		LpstrTitle: p(title), LpstrDefExt: p("xlsx"),
	}
	if save {
		of.Flags = OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT
		r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
		if r == 0 {
			return ""
		}
	} else {
		of.Flags = OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_FILEMUSTEXIST
		r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&of)))
		if r == 0 {
			return ""
		}
	}
	path := syscall.UTF16ToString(buf)
	if save && !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	return path
}

func downloadEconomicsTemplate() {
	path := chooseEconomicsTemplatePath("РОСА_БЛАНК_ЭКОНОМИКИ.xlsx", "Сохранить бланк экономической оценки", true)
	if path == "" {
		return
	}
	if len(economicsBlankTemplateXLSX) == 0 {
		msg("Бланк экономики", "Встроенный экономический бланк недоступен.", MB_OK|MB_ICONERROR)
		return
	}
	if err := os.WriteFile(path, economicsBlankTemplateXLSX, 0o600); err != nil {
		msg("Бланк экономики", err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	msg("Бланк экономики", "Сохранён отдельный бланк исходных экономических параметров и вариантов. Заполните листы «Параметры» и «Варианты», затем выберите «Загрузить бланк».", MB_OK|MB_ICONINFORMATION)
}

func economicsWorkbookSheetRows(path, sheetName string) ([][]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 16<<20 {
		return nil, fmt.Errorf("экономический бланк больше 16 МБ")
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	var shared []string
	if b, readErr := readZipEntry(z, "xl/sharedStrings.xml"); readErr == nil {
		var ss xlsxSharedStrings
		if xml.Unmarshal(b, &ss) == nil {
			for _, item := range ss.Items {
				shared = append(shared, sharedText(item))
			}
		}
	}
	sheetPath, err := xlsxWorksheetPath(z, sheetName)
	if err != nil {
		return nil, err
	}
	b, err := readZipEntry(z, sheetPath)
	if err != nil {
		return nil, err
	}
	var sheet xlsxSheet
	if err = xml.Unmarshal(b, &sheet); err != nil {
		return nil, err
	}
	rows := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		if row.R > 500 {
			return nil, fmt.Errorf("лист «%s» содержит слишком много строк", sheetName)
		}
		values := map[int]string{}
		maxCol := 0
		for _, cell := range row.Cells {
			col := colNumber(cell.R)
			if col < 1 || col > 32 {
				continue
			}
			if col > maxCol {
				maxCol = col
			}
			value := cell.V
			if cell.T == "s" {
				i, _ := strconv.Atoi(cell.V)
				if i >= 0 && i < len(shared) {
					value = shared[i]
				}
			} else if cell.T == "inlineStr" {
				value = cell.Inline.T
			}
			values[col] = strings.TrimSpace(value)
		}
		if maxCol == 0 {
			rows = append(rows, nil)
			continue
		}
		valuesRow := make([]string, maxCol)
		for col := 1; col <= maxCol; col++ {
			valuesRow[col-1] = values[col]
		}
		rows = append(rows, valuesRow)
	}
	return rows, nil
}

func economicsHeaderColumns(rows [][]string, required ...string) (int, map[string]int, error) {
	for rowIndex, row := range rows {
		columns := map[string]int{}
		for col, value := range row {
			key := strings.ToLower(strings.TrimSpace(value))
			if key != "" {
				columns[key] = col
			}
		}
		matched := true
		for _, key := range required {
			if _, ok := columns[strings.ToLower(key)]; !ok {
				matched = false
				break
			}
		}
		if matched {
			return rowIndex, columns, nil
		}
	}
	return -1, nil, fmt.Errorf("не найдена шапка обязательных колонок: %s", strings.Join(required, ", "))
}

func economicsCell(row []string, columns map[string]int, name string) string {
	if col, ok := columns[strings.ToLower(name)]; ok && col >= 0 && col < len(row) {
		return strings.TrimSpace(row[col])
	}
	return ""
}

func economicsOptionalNumber(raw, field string) (*float64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	value, err := parseEconomicNumber(raw, field, true)
	if err != nil {
		return nil, err
	}
	if !finiteNumber(value) {
		return nil, fmt.Errorf("поле «%s»: требуется конечное число", field)
	}
	return &value, nil
}

func parseEconomicsWorkbookRows(parameterRows, variantRows [][]string) (economicsWorkbookInput, error) {
	parameterHeader, parameterColumns, err := economicsHeaderColumns(parameterRows, "Параметр", "Значение")
	if err != nil {
		return economicsWorkbookInput{}, fmt.Errorf("лист «Параметры»: %w", err)
	}
	values := map[string]string{}
	for i := parameterHeader + 1; i < len(parameterRows); i++ {
		row := parameterRows[i]
		if len(row) == 0 {
			continue
		}
		name := strings.ToLower(economicsCell(row, parameterColumns, "Параметр"))
		if name != "" {
			values[name] = economicsCell(row, parameterColumns, "Значение")
		}
	}
	get := func(name string) string { return strings.TrimSpace(values[strings.ToLower(name)]) }
	objectKey := get("Объект")
	current, err := parseEconomicNumber(get("Текущий МРП"), "Текущий МРП", true)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	low, err := parseEconomicNumber(get("МРП осторожный"), "МРП осторожный", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	base, err := parseEconomicNumber(get("МРП центральный"), "МРП центральный", true)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	high, err := parseEconomicNumber(get("МРП благоприятный"), "МРП благоприятный", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	horizonValue, err := parseEconomicNumber(get("Горизонт"), "Горизонт", true)
	if err != nil || horizonValue != math.Trunc(horizonValue) || horizonValue < 1 || horizonValue > 60 {
		return economicsWorkbookInput{}, fmt.Errorf("горизонт: требуется целое число от 1 до 60 месяцев")
	}
	repairDays, err := parseEconomicNumber(get("Длительность ремонта"), "Длительность ремонта", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	repairCost, err := parseEconomicNumber(get("Стоимость ремонта"), "Стоимость ремонта", true)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	opex, err := parseEconomicNumber(get("OPEX"), "OPEX", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	inflation, err := parseEconomicNumber(get("Инфляция OPEX"), "Инфляция OPEX", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	capex, err := parseEconomicNumber(get("CAPEX"), "CAPEX", false)
	if err != nil {
		return economicsWorkbookInput{}, err
	}
	selected := normalizeScenarioName(get("Выбранный сценарий"))
	if selected == "" {
		selected = "Центральный"
	}
	if low < 0 || base <= 0 || high < 0 || repairDays < 0 || repairCost <= 0 || opex < 0 || capex < 0 || inflation <= -100 {
		return economicsWorkbookInput{}, fmt.Errorf("проверьте МРП и затраты: МРП должны быть положительными, затраты — неотрицательными")
	}
	in := BaseEconomicsInput{CurrentMRPDays: current, MRPLowDays: low, MRPBaseDays: base, MRPHighDays: high,
		SelectedScenario: selected, HorizonMonths: int(horizonValue), RepairDurationDays: repairDays,
		RepairCostK: repairCost / 1000, AnnualOPEXK: opex / 1000, OPEXInflationPct: inflation, OneTimeMTRK: capex / 1000}
	if errs := validateBaseEconomicsInput(in); len(errs) > 0 {
		return economicsWorkbookInput{}, fmt.Errorf("проверьте исходные данные: %s", strings.Join(errs, "; "))
	}
	uiValues := map[string]string{
		"current_mrp": strconv.FormatFloat(current, 'f', -1, 64),
		"mrp_low":     strings.TrimSpace(get("МРП осторожный")),
		"mrp_base":    strconv.FormatFloat(base, 'f', -1, 64),
		"mrp_high":    strings.TrimSpace(get("МРП благоприятный")),
		"scenario":    selected,
		"horizon":     strconv.Itoa(int(horizonValue)),
		"repair_days": strings.TrimSpace(get("Длительность ремонта")),
		"repair_cost": strconv.FormatFloat(repairCost, 'f', -1, 64),
		"annual_opex": strconv.FormatFloat(opex, 'f', -1, 64),
		"inflation":   strconv.FormatFloat(inflation, 'f', -1, 64),
		"one_time":    strconv.FormatFloat(capex, 'f', -1, 64),
	}
	variants := make([]economicsWorkbookVariant, 0, 3)
	if len(variantRows) > 0 {
		header, columns, headerErr := economicsHeaderColumns(variantRows, "№", "МРП осторожный", "МРП центральный", "МРП благоприятный", "OPEX, руб./год", "CAPEX, руб.")
		if headerErr != nil {
			return economicsWorkbookInput{}, fmt.Errorf("лист «Варианты»: %w", headerErr)
		}
		for i := header + 1; i < len(variantRows); i++ {
			row := variantRows[i]
			if len(row) == 0 {
				continue
			}
			ordText := economicsCell(row, columns, "№")
			ord, parseErr := strconv.Atoi(strings.TrimSpace(ordText))
			if parseErr != nil || ord < 1 || ord > 3 {
				continue
			}
			v := economicsWorkbookVariant{ordinal: ord}
			v.mrpLow, err = economicsOptionalNumber(economicsCell(row, columns, "МРП осторожный"), "вариант: МРП осторожный")
			if err != nil {
				return economicsWorkbookInput{}, err
			}
			v.mrpBase, err = economicsOptionalNumber(economicsCell(row, columns, "МРП центральный"), "вариант: МРП центральный")
			if err != nil {
				return economicsWorkbookInput{}, err
			}
			v.mrpHigh, err = economicsOptionalNumber(economicsCell(row, columns, "МРП благоприятный"), "вариант: МРП благоприятный")
			if err != nil {
				return economicsWorkbookInput{}, err
			}
			v.opex, err = economicsOptionalNumber(economicsCell(row, columns, "OPEX, руб./год"), "вариант: OPEX")
			if err != nil {
				return economicsWorkbookInput{}, err
			}
			v.capex, err = economicsOptionalNumber(economicsCell(row, columns, "CAPEX, руб."), "вариант: CAPEX")
			if err != nil {
				return economicsWorkbookInput{}, err
			}
			for _, number := range []*float64{v.mrpLow, v.mrpBase, v.mrpHigh, v.opex, v.capex} {
				if number != nil && *number < 0 {
					return economicsWorkbookInput{}, fmt.Errorf("значения сценария не могут быть отрицательными")
				}
			}
			for _, mrp := range []*float64{v.mrpLow, v.mrpBase, v.mrpHigh} {
				if mrp != nil && *mrp <= 0 {
					return economicsWorkbookInput{}, fmt.Errorf("прогноз МРП в вариантах должен быть больше нуля")
				}
			}
			if v.mrpLow != nil || v.mrpBase != nil || v.mrpHigh != nil || v.opex != nil || v.capex != nil {
				variants = append(variants, v)
			}
		}
	}
	return economicsWorkbookInput{objectKey: objectKey, uiValues: uiValues, variants: variants}, nil
}

func parseEconomicsWorkbook(path string) (economicsWorkbookInput, error) {
	parameters, err := economicsWorkbookSheetRows(path, "Параметры")
	if err != nil {
		return economicsWorkbookInput{}, fmt.Errorf("лист «Параметры» не найден: %w", err)
	}
	variants, err := economicsWorkbookSheetRows(path, "Варианты")
	if err != nil {
		return economicsWorkbookInput{}, fmt.Errorf("лист «Варианты» не найден: %w", err)
	}
	return parseEconomicsWorkbookRows(parameters, variants)
}

func importEconomicsTemplate() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Бланк экономики", "Сначала закрепите объект на Главной, затем загружайте экономические исходные данные.", MB_OK|MB_ICONWARNING)
		return
	}
	path := chooseEconomicsTemplatePath("", "Загрузить экономический бланк", false)
	if path == "" {
		return
	}
	imported, err := parseEconomicsWorkbook(path)
	if err != nil {
		msg("Бланк экономики", "Не удалось прочитать экономические исходные данные.\n\n"+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	if strings.TrimSpace(imported.objectKey) != "" && !laboratoryObjectKeysEquivalent(imported.objectKey, selectedObjectKey) && !strings.EqualFold(strings.TrimSpace(imported.objectKey), strings.TrimSpace(selectedObjectKey)) {
		msg("Бланк экономики", "В файле указан другой объект. Закрепите соответствующий объект или очистите поле «Объект» в бланке.", MB_OK|MB_ICONWARNING)
		return
	}
	if economicsBindingChanged(econBoundObjectKey, selectedObjectKey) {
		useSelectedObjectForEconomics()
	}
	schemes := alternativeScenariosForObject(selectedObjectKey)
	for _, variant := range imported.variants {
		if variant.ordinal > len(schemes) {
			msg("Бланк экономики", fmt.Sprintf("В бланке заполнен вариант %d, но в разделе 4 для выбранного объекта подготовлено только %d схем. Сначала сформируйте нужные схемы.", variant.ordinal, len(schemes)), MB_OK|MB_ICONWARNING)
			return
		}
	}
	setText(econCurrentMRP, imported.uiValues["current_mrp"])
	setText(econMRPLow, imported.uiValues["mrp_low"])
	setText(econMRPBase, imported.uiValues["mrp_base"])
	setText(econMRPHigh, imported.uiValues["mrp_high"])
	setText(econScenario, imported.uiValues["scenario"])
	setText(econHorizon, imported.uiValues["horizon"])
	setText(econRepairDays, imported.uiValues["repair_days"])
	setText(econRepairCost, imported.uiValues["repair_cost"])
	setText(econAnnualOPEX, imported.uiValues["annual_opex"])
	setText(econInflation, imported.uiValues["inflation"])
	setText(econOneTimeMTR, imported.uiValues["one_time"])

	activeID := activeAlternativeScenarioID(selectedObjectKey)
	oldScenarios := append([]AlternativeMethodScenario(nil), alternativeScenarios...)
	indices := alternativeScenarioIndices(selectedObjectKey)
	byID := make(map[string]int, len(indices))
	for _, index := range indices {
		byID[strings.ToUpper(strings.TrimSpace(alternativeScenarios[index].ID))] = index
	}
	markInputsChanged := func(s *AlternativeMethodScenario) {
		s.EconomicsStatus = "Исходные данные изменены"
		s.EconomicsCalculated = ""
		s.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	if index, ok := byID[strings.ToUpper(strings.TrimSpace(activeID))]; ok {
		s := &alternativeScenarios[index]
		if v, e := strconv.ParseFloat(imported.uiValues["mrp_low"], 64); e == nil && v > 0 {
			s.ForecastLowDays = v
		}
		if v, e := strconv.ParseFloat(imported.uiValues["mrp_base"], 64); e == nil && v > 0 {
			s.ForecastBaseDays = v
		}
		if v, e := strconv.ParseFloat(imported.uiValues["mrp_high"], 64); e == nil && v > 0 {
			s.ForecastHighDays = v
		}
		if v, e := strconv.ParseFloat(imported.uiValues["annual_opex"], 64); e == nil {
			s.AnnualOPEXRub = v
		}
		if v, e := strconv.ParseFloat(imported.uiValues["one_time"], 64); e == nil {
			s.OneTimeCAPEXRub = v
		}
		markInputsChanged(s)
	}
	for _, variant := range imported.variants {
		scenarioID := strings.ToUpper(strings.TrimSpace(schemes[variant.ordinal-1].ID))
		index, ok := byID[scenarioID]
		if !ok {
			continue
		}
		s := &alternativeScenarios[index]
		if variant.mrpLow != nil {
			s.ForecastLowDays = *variant.mrpLow
		}
		if variant.mrpBase != nil {
			s.ForecastBaseDays = *variant.mrpBase
		}
		if variant.mrpHigh != nil {
			s.ForecastHighDays = *variant.mrpHigh
		}
		if variant.opex != nil {
			s.AnnualOPEXRub = *variant.opex
		}
		if variant.capex != nil {
			s.OneTimeCAPEXRub = *variant.capex
		}
		markInputsChanged(s)
	}
	if err := saveAlternativeScenarios(); err != nil {
		alternativeScenarios = oldScenarios
		msg("Бланк экономики", "Параметры не сохранены в накопительной экономической истории: "+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	econBoundObjectKey = strings.TrimSpace(selectedObjectKey)
	setText(econResult, fmt.Sprintf("Экономические параметры загружены из %s. Реестр отказов не затронут. Проверьте исходные данные и выполните расчёт схем.", filepath.Base(path)))
	refreshEconomicsDecisionIndicators()
	refreshEconomicsVisualReference()
	refreshEconomicsMetricCards()
}
