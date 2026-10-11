//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_WELL_HISTORY_TAB = 9500 + iota
	ID_WELL_MANUAL_TAB
	ID_WELL_OPEN_EVENT
	ID_WELL_ADD_FAILURE
	ID_WELL_EDIT_FAILURE
	ID_WELL_DELETE_FAILURE
	ID_WELL_SAVE_FAILURE
	ID_WELL_CLEAR_FAILURE
	ID_WELL_CANCEL_FAILURE
	ID_WELL_BACK_MAIN
	ID_WELL_IMPORT_FORM
	ID_WELL_DOWNLOAD_TEMPLATE
	ID_WELL_APPROVE_FAILURE
	ID_WELL_SAVE_DRAFT
)

const (
	ID_WELL_FORM_MODE_OPERATIONAL = 9558
	ID_WELL_FORM_MODE_FULL        = 9559
)

const ID_WELL_FORM_COMBO_BASE = 9570

var (
	wellHistoryTab, wellManualTab, wellBackMain                                                                                                                                          syscall.Handle
	wellMetrics                                                                                                                                                                          [4]syscall.Handle
	wellMetricTitles                                                                                                                                                                     [4]syscall.Handle
	wellHistoryList, wellOpenEvent, wellImportForm, wellDownloadTemplate                                                                                                                 syscall.Handle
	wellManualCounter, wellManualList, wellAddFailure, wellEditFailure, wellDeleteFailure                                                                                                syscall.Handle
	wellFormHeading, wellFormProgress, wellFormModeOperational, wellFormModeFull, wellFormError, wellSaveDraft, wellSaveFailure, wellApproveFailure, wellClearFailure, wellCancelFailure syscall.Handle
	wellFormLabels                                                                                                                                                                       [20]syscall.Handle
	wellFormFields                                                                                                                                                                       [20]syscall.Handle
	wellCauseChoices                                                                                                                                                                     []OfficialFailureCause2024
	wellManualMode                                                                                                                                                                       bool
	wellFormActive                                                                                                                                                                       bool
	wellFullCardMode                                                                                                                                                                     bool
	wellEditingID                                                                                                                                                                        string
	wellEditingOriginal                                                                                                                                                                  FailureEvent
	wellDraftKey                                                                                                                                                                         string
	wellHistoryRows, wellManualRows                                                                                                                                                      []FailureEvent
)

var wellMetricDividers [3]syscall.Handle
var hFontWellMetricValue syscall.Handle

var wellFieldCaptions = [20]string{
	"Дата отказа / окончания *", "Дата пуска *", "Дата окончания ремонта",
	"Способ эксплуатации *", "Тип / компоновка ГНО", "Отказавший узел *",
	"Признак / предпосылка *", "Код причины по классификатору", "Причина по классификатору",
	"Основная категория осложнения *", "Осмотр / дефектовка", "Заключение комиссии / акт",
	"Статус расследования", "Источник / документ", "Комментарий",
	"Уточнённая категория осложнения", "Марка ГНО *", "Типоразмер ГНО 1 *",
	"Предварительное заключение", "Утверждённое заключение",
}

func buildWellCardFailureUI() {
	loadWellFailureCatalog()
	wellBackMain = button(ID_OBJECTS, "← На главную", 242, 224, 225, 44, ID_WELL_BACK_MAIN)
	wellHistoryTab = button(ID_OBJECTS, "История отказов · 36 мес.", 242, 224, 285, 44, ID_WELL_HISTORY_TAB)
	wellManualTab = button(ID_OBJECTS, "Ручной ввод отказа", 539, 224, 285, 44, ID_WELL_MANUAL_TAB)
	hFontWellMetricValue = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(31-1), 0, 0, 0, 700, 0, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))))
	for i, title := range []string{"СОБЫТИЙ · 36 МЕС.", "С УКАЗАННЫМ ОСЛОЖНЕНИЕМ", "ПОСЛЕДНИЙ ОТКАЗ", "ПОСЛЕДНИЙ ИСХОДНЫЙ КЛАСС"} {
		wellMetricTitles[i] = create("STATIC", title, WS_CHILD|SS_CENTER|SS_NOPREFIX, 242+int32(i)*250, py(274), 238, 36, hMain, 0)
		addPageControl(ID_OBJECTS, wellMetricTitles[i])
		wellMetrics[i] = create("STATIC", "—", WS_CHILD|SS_CENTER|SS_NOPREFIX, 242+int32(i)*250, py(312), 238, 74, hMain, 0)
		addPageControl(ID_OBJECTS, wellMetrics[i])
		pSendMessageW.Call(uintptr(wellMetricTitles[i]), WM_SETFONT, uintptr(hFontSection), 1)
		pSendMessageW.Call(uintptr(wellMetrics[i]), WM_SETFONT, uintptr(hFontWellMetricValue), 1)
	}
	for i := range wellMetricDividers {
		wellMetricDividers[i] = create("STATIC", "", WS_CHILD|0x11, 242, py(280), 2, 100, hMain, 0) // SS_ETCHEDVERT
		addPageControl(ID_OBJECTS, wellMetricDividers[i])
	}
	wellHistoryList = newList(ID_OBJECTS, 242, 340, 1010, 285, []struct {
		n string
		w int32
	}{
		{"Дата отказа / окончания", 130}, {"Тип события", 130}, {"ГНО", 190},
		{"Осложнение", 180}, {"Статус данных", 230}, {"Статус расследования", 170}, {"Источник", 250},
	})
	setListHeaderFont(wellHistoryList, hFontHeaderSmall)
	centerListHeaders(wellHistoryList, 7)
	wellOpenEvent = button(ID_OBJECTS, "АНАЛИЗ ПО ОБЪЕКТУ", 242, 630, 240, 42, ID_WELL_OPEN_EVENT)
	wellImportForm = button(ID_OBJECTS, "Подгрузить форму отказов", 242, 630, 240, 42, ID_WELL_IMPORT_FORM)
	// The cumulative failure database is handled directly on the first module.
	// Keeping the three steps together prevents the old jump to the service page.
	wellDownloadTemplate = button(ID_OBJECTS, "1  СКАЧАТЬ БЛАНК РЕЕСТРА ОТКАЗОВ", 242, 274, 300, 42, ID_WELL_DOWNLOAD_TEMPLATE)
	failureChooseButton = button(ID_OBJECTS, "2  ЗАГРУЗИТЬ ЗАПОЛНЕННЫЙ\r\nРЕЕСТР ОТКАЗОВ", 492, 274, 280, 54, ID_FAILURE_CHOOSE)
	failurePreviewButton = button(ID_OBJECTS, "3  ПРОВЕРИТЬ", 782, 274, 220, 40, ID_FAILURE_PREVIEW)
	failureApplyButton = button(ID_OBJECTS, "4  ПРИНЯТЬ", 1012, 274, 220, 40, ID_FAILURE_APPLY)
	failureObjectFrame = create("STATIC", "", WS_CHILD|0x12, 242, py(384), 1010, 36, hMain, 0) // SS_ETCHEDFRAME
	addPageControl(ID_OBJECTS, failureObjectFrame)
	failureObjectCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST,
		242, py(274), 700, 300, hMain, ID_FAILURE_OBJECT_COMBO)
	addPageControl(ID_OBJECTS, failureObjectCombo)
	failureObjectOpenButton = button(ID_OBJECTS, "ВЫБРАТЬ ОБЪЕКТ", 952, 274, 300, 42, ID_FAILURE_OBJECT_OPEN)
	failureAnalysisButton = button(ID_OBJECTS, "АНАЛИЗ ОТКАЗОВ", 952, 274, 300, 42, ID_FAILURE_ANALYSIS)
	failureImportFileLabel = label(ID_OBJECTS, "Файл: не выбран", 242, 318, 1010, 20)
	failureImportSummary = label(ID_OBJECTS, "Выберите утверждённую Excel-форму базы отказов .xlsx", 242, 340, 1010, 20)
	failureImportList = newList(ID_OBJECTS, 242, 370, 1010, 200, []struct {
		n string
		w int32
	}{{"Статус", 110}, {"Строка", 65}, {"Объект", 210}, {"Дата", 105}, {"Причина", 350}, {"Результат проверки", 250}})
	setListHeaderFont(failureImportList, hFontHeaderSmall)
	centerListHeaders(failureImportList, 6)
	wellManualCounter = label(ID_OBJECTS, "История событий и ручной ввод · 36 мес.", 242, 274, 600, 32)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(wellManualCounter), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	}
	wellManualList = newList(ID_OBJECTS, 242, 326, 1010, 320, []struct {
		n string
		w int32
	}{
		{"Дата отказа / окончания", 130}, {"ГНО", 210}, {"Причина", 250}, {"Статус данных", 250}, {"Статус расследования", 180}, {"Источник", 250},
	})
	setListHeaderFont(wellManualList, hFontHeaderSmall)
	centerListHeaders(wellManualList, 6)
	wellAddFailure = button(ID_OBJECTS, "ДОБАВИТЬ ОТКАЗ", 242, 274, 225, 44, ID_WELL_ADD_FAILURE)
	wellEditFailure = button(ID_OBJECTS, "Дополнить / изменить", 479, 630, 225, 44, ID_WELL_EDIT_FAILURE)
	wellDeleteFailure = button(ID_OBJECTS, "Удалить запись", 716, 630, 225, 44, ID_WELL_DELETE_FAILURE)
	wellFormHeading = label(ID_OBJECTS, "Новый отказ", 242, 270, 810, 22)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(wellFormHeading), WM_SETFONT, uintptr(hFontBold), 1)
	}
	wellFormProgress = label(ID_OBJECTS, "Статус данных: Только оперативный ввод", 242, 270, 430, 30)
	if hFontBold != 0 {
		pSendMessageW.Call(uintptr(wellFormProgress), WM_SETFONT, uintptr(hFontBold), 1)
	}
	wellFormModeOperational = button(ID_OBJECTS, "ПРЕДВАРИТЕЛЬНО", 242, 270, 170, 28, ID_WELL_FORM_MODE_OPERATIONAL)
	wellFormModeFull = button(ID_OBJECTS, "РАСШИРЕННОЕ", 242, 270, 190, 28, ID_WELL_FORM_MODE_FULL)
	wellFormError = label(ID_OBJECTS, "", 242, 602, 1010, 20)
	for i, name := range wellFieldCaptions {
		wellFormLabels[i] = label(ID_OBJECTS, name, 242, 292, 350, 20)
		if hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(wellFormLabels[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}
		if wellFormUsesChoice(i) || wellFormUsesEditableCombo(i) {
			wellFormFields[i] = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWN,
				242, py(315), 350, 260, hMain, ID_WELL_FORM_COMBO_BASE+i)
			addPageControl(ID_OBJECTS, wellFormFields[i])
		} else {
			wellFormFields[i] = edit(ID_OBJECTS, "", 242, 315, 350, 27, wellFormUsesMultilineEdit(i), i == 7, 0)
		}
	}
	for i, spec := range wellExtraSpecs {
		wellExtraLabels[i] = label(ID_OBJECTS, spec.title, 242, 365, 270, 20)
		pSendMessageW.Call(uintptr(wellExtraLabels[i]), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		if wellExtraUsesFreeText(spec.column) {
			wellExtraFields[i] = edit(ID_OBJECTS, "", 242, 388, 270, 27, false, false, 9900+i)
		} else {
			wellExtraFields[i] = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWN,
				242, py(388), 270, 260, hMain, 9900+i)
			addPageControl(ID_OBJECTS, wellExtraFields[i])
		}
	}
	for i, name := range []string{"1 ● ПРЕДВАРИТЕЛЬНО", "2 ● РАССЛЕДОВАНИЕ", "3 ● НКТ", "4 ● НАСОС", "5 ● ЭЛЕКТРИКА", "6 ● ШТАНГИ", "7 ● КОМИССИЯ/ИТОГ"} {
		wellSectionButtons[i] = button(ID_OBJECTS, name, 242, 297, 170, 38, ID_WELL_SECTION_BASE+i)
	}
	wellCatalogButton = button(ID_OBJECTS, "ИЗ СПРАВОЧНИКА", 770, 295, 180, 38, ID_WELL_CATALOG)
	wellReferenceButton = button(ID_OBJECTS, "СПРАВОЧНИК", 770, 295, 180, 38, ID_WELL_REFERENCE)
	wellManualEntryButton = button(ID_OBJECTS, "ВВЕСТИ САМОМУ", 960, 295, 160, 38, ID_WELL_MANUAL_ENTRY)
	wellFormModeNote = label(ID_OBJECTS, wellSectionHeading(0), 242, 337, 1010, 20)
	pSendMessageW.Call(uintptr(wellFormModeNote), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
	wellSaveDraft = button(ID_OBJECTS, "Сохранить черновик", 242, 630, 245, 42, ID_WELL_SAVE_DRAFT)
	wellSaveFailure = button(ID_OBJECTS, "Сохранить предварительное", 242, 630, 245, 42, ID_WELL_SAVE_FAILURE)
	wellApproveFailure = button(ID_OBJECTS, "Утвердить заключение", 242, 630, 245, 42, ID_WELL_APPROVE_FAILURE)
	wellClearFailure = button(ID_OBJECTS, "Очистить форму", 499, 630, 220, 42, ID_WELL_CLEAR_FAILURE)
	wellCancelFailure = button(ID_OBJECTS, "К списку отказов", 731, 630, 220, 42, ID_WELL_CANCEL_FAILURE)
}

func layoutWellCardFailureUI(w, h int32) {
	contentW := max32(720, w-270)

	// Two navigation tabs always span the same content width.
	setPos(wellBackMain, 242, py(224), 225, 44)
	tabGap := int32(10)
	tabW := (contentW - tabGap) / 2
	setPos(wellHistoryTab, 242, py(224), tabW, 44)
	setPos(wellManualTab, 242+tabW+tabGap, py(224), contentW-tabW-tabGap, 44)

	cardGap := int32(12)
	cardW := (contentW - 3*cardGap) / 4
	for i, handle := range wellMetrics {
		x := int32(242) + int32(i)*(cardW+cardGap)
		setPos(wellMetricTitles[i], x, py(274), cardW, 36)
		setPos(handle, x, py(312), cardW, 74)
	}

	for i, divider := range wellMetricDividers {
		setPos(divider, 242+int32(i+1)*cardW+int32(i)*cardGap+cardGap/2, py(282), 2, 100)
	}

	// Import controls belong to the first module and occupy the toolbar above
	// the history table. The preview list uses the same table area while a file
	// is being checked, then disappears after acceptance.
	// No object chooser here: the only working-object selector lives on Main.
	// The import can ingest the whole workbook, while this module displays the
	// history of the globally pinned object.
	selectorButtonW := min32(255, max32(220, contentW/4))
	selectorGap := int32(10)
	selectorY := py(384)
	// RC705: the etched frame around this row was the source of the visible
	// one-pixel/side-stub shift at 125–150% Windows scaling.  The native combo
	// and the adjacent action now own the row directly and share one top edge.
	// Keep the old frame physically out of the layout so it cannot draw through
	// the gap between the controls.
	setPos(failureObjectFrame, 242, selectorY, 1, 1)
	setPos(failureObjectCombo, 242, selectorY, contentW-selectorButtonW-selectorGap, 280)
	setPos(failureObjectOpenButton, 242+contentW-selectorButtonW, selectorY, selectorButtonW, 30)
	setPos(failureAnalysisButton, 242, py(274), 1, 1)
	gap := int32(10)
	stepW := (contentW - 3*gap) / 4
	toolbarH := int32(54)
	setPos(wellDownloadTemplate, 242, py(274), stepW, toolbarH)
	setPos(failureChooseButton, 242+stepW+gap, py(274), stepW, toolbarH)
	setPos(failurePreviewButton, 242+2*(stepW+gap), py(274), stepW, toolbarH)
	setPos(failureApplyButton, 242+3*(stepW+gap), py(274), contentW-3*(stepW+gap), toolbarH)
	setPos(failureImportFileLabel, 242, py(334), contentW, 20)
	setPos(failureImportSummary, 242, py(356), contentW, 20)

	// Action rows are positioned independently of the active tab.  The previous
	// implementation laid out only the currently visible set of buttons; after a
	// tab change a hidden button could reappear at its old build-time coordinate
	// and look as if it were floating in the middle of the table.
	actionY := max32(py(590), h-54)
	actionGap := int32(12)

	historyY := py(384)
	if len(failureObjectKeys) > 0 {
		historyY = selectorY + 38
	}
	historyH := max32(88, actionY-historyY-12)
	setPos(wellHistoryList, 242, historyY, contentW, historyH)
	setPos(failureImportList, 242, historyY, contentW, max32(88, actionY-historyY-12))

	historyButtonW := (contentW - 3*actionGap) / 4
	setPos(wellOpenEvent, 242, actionY, historyButtonW, 42)
	setPos(wellImportForm, 242+3*(historyButtonW+actionGap), actionY, contentW-3*(historyButtonW+actionGap), 42)

	addW := min32(300, (contentW-12)/2)
	setPos(wellManualCounter, 242, py(393), contentW, 28)
	if wellManualMode {
		pairX := int32(242) + contentW - addW
		setPos(wellAddFailure, pairX, py(429), addW, 46)

	}
	manualY := py(489)
	manualH := max32(85, actionY-manualY-12)
	setPos(wellManualList, 242, manualY, contentW, manualH)

	manualButtonW := (contentW - 2*actionGap) / 3
	if wellManualMode {
		setPos(wellOpenEvent, 242, actionY, manualButtonW, 42)
		setPos(wellEditFailure, 242+manualButtonW+actionGap, actionY, manualButtonW, 42)
		setPos(wellDeleteFailure, 242+2*(manualButtonW+actionGap), actionY, contentW-2*(manualButtonW+actionGap), 42)
	} else {
		setPos(wellEditFailure, 242+historyButtonW+actionGap, actionY, historyButtonW, 42)
		setPos(wellDeleteFailure, 242+2*(historyButtonW+actionGap), actionY, historyButtonW, 42)
	}

	// Keep all named columns fitted exactly to the visible table area.  This is
	// the same rule used by the object-selection table: the rightmost named
	// column receives the remainder and no anonymous spacer column is created.
	fitWellFailureListColumns(wellHistoryList, []int32{130, 130, 190, 180, 230, 170, 250})
	fitWellFailureListColumns(wellManualList, []int32{130, 210, 250, 250, 180, 250})
	fitWellFailureListColumns(failureImportList, []int32{110, 65, 210, 105, 350, 250})

	// At regular height the form follows the laboratory two-column layout.
	// At the minimum window height four columns keep full-height labels and
	// input fields visible above the separate validation/action footer.
	modeGap := int32(12)
	modeOpW := min32(184, max32(158, contentW/6))
	modeFullW := min32(194, max32(158, contentW/6))
	// The form title owns its own line.  Status and mode selectors are deliberately
	// placed on the next line so a long object/status text can never cover the
	// heading or each other at Windows scaling above 100%.
	setPos(wellFormHeading, 242, py(270), contentW, 28)
	statusY := py(302)
	progressW := max32(280, contentW-modeOpW-modeFullW-2*modeGap)
	setPos(wellFormProgress, 242, statusY, progressW, 32)
	setPos(wellFormModeOperational, 242+progressW+modeGap, statusY-2, modeOpW, 36)
	setPos(wellFormModeFull, 242+progressW+modeGap+modeOpW+modeGap, statusY-2, modeFullW, 36)
	toolbarGap := int32(14)
	sectionControls := []syscall.Handle{wellSectionButtons[0]}
	if wellFullCardMode {
		for i := 1; i < len(wellSectionButtons); i++ {
			if wellSectionApplicable(i) {
				sectionControls = append(sectionControls, wellSectionButtons[i])
			}
		}
	}
	sectionW := (contentW - int32(len(sectionControls)-1)*toolbarGap) / int32(len(sectionControls))
	for i, control := range sectionControls {
		x := int32(242) + int32(i)*(sectionW+toolbarGap)
		width := sectionW
		if i == len(sectionControls)-1 {
			width = int32(242) + contentW - x
		}
		setPos(control, x, py(344), width, 42)
	}
	actionControls := []syscall.Handle{wellCatalogButton, wellReferenceButton, wellManualEntryButton}
	actionInset := int32(18)
	actionUsableW := contentW - 2*actionInset
	actionW := (actionUsableW - 2*toolbarGap) / 3
	for i, control := range actionControls {
		x := int32(242) + actionInset + int32(i)*(actionW+toolbarGap)
		width := actionW
		if i == len(actionControls)-1 {
			width = int32(242) + contentW - actionInset - x
		}
		setPos(control, x, py(394), width, 42)
	}
	setPos(wellFormModeNote, 250, py(442), contentW-16, 22)
	formCols := int32(4)
	formGap := int32(16)
	formInset := int32(8)
	formTop := py(470)
	formContentW := contentW - 2*formInset
	formColW := (formContentW - (formCols-1)*formGap) / formCols
	for section := 0; section <= 6; section++ {
		if section == 6 {
			sectionX := int32(242) + formInset
			fullW := formContentW
			pairW := (fullW - formGap) / 2
			topY := formTop
			setPos(wellFormLabels[11], sectionX, topY, pairW, 20)
			setPos(wellFormFields[11], sectionX, topY+23, pairW, 27)
			setPos(wellFormLabels[19], sectionX+pairW+formGap, topY, pairW, 20)
			setPos(wellFormFields[19], sectionX+pairW+formGap, topY+23, pairW, 27)

			available := max32(172, h-formTop-108)
			sourceH := min32(54, max32(38, available/5))
			commentH := min32(84, max32(44, available-96-sourceH))
			if sourceH+commentH > available-88 {
				commentH = max32(44, available-88-sourceH)
			}
			sourceY := topY + 56
			commentY := sourceY + 23 + sourceH + 8
			setPos(wellFormLabels[13], sectionX, sourceY, fullW, 20)
			setPos(wellFormFields[13], sectionX, sourceY+23, fullW, sourceH)
			setPos(wellFormLabels[14], sectionX, commentY, fullW, 20)
			setPos(wellFormFields[14], sectionX, commentY+23, fullW, commentH)
			continue
		}
		fieldCount := int32(0)
		for i := range wellFormFields {
			if wellCoreSection(i) == section {
				fieldCount++
			}
		}
		for _, spec := range wellExtraSpecs {
			if spec.section == section {
				fieldCount++
			}
		}
		rows := max32(1, (fieldCount+formCols-1)/formCols)
		rowStep := min32(135, max32(50, (h-84-formTop-50)/max32(1, rows-1)))
		position := int32(0)
		for i := range wellFormFields {
			if wellCoreSection(i) != section {
				continue
			}
			col, row := position%formCols, position/formCols
			x := int32(242) + formInset + col*(formColW+formGap)
			y := formTop + row*rowStep
			setPos(wellFormLabels[i], x, y, formColW, 20)
			fieldH := int32(27)
			if wellFormUsesChoice(i) || wellFormUsesEditableCombo(i) {
				fieldH = 260
			}
			setPos(wellFormFields[i], x, y+23, formColW, fieldH)
			position++
		}
		for i, spec := range wellExtraSpecs {
			if spec.section != section {
				continue
			}
			col, row := position%formCols, position/formCols
			x := int32(242) + formInset + col*(formColW+formGap)
			y := formTop + row*rowStep
			setPos(wellExtraLabels[i], x, y, formColW, 20)
			fieldH := int32(260)
			if wellExtraUsesFreeText(spec.column) {
				fieldH = 27
			}
			setPos(wellExtraFields[i], x, y+23, formColW, fieldH)
			position++
		}
	}

	setPos(wellFormError, 242, h-74, contentW, 20)
	footerControls := []syscall.Handle{wellSaveDraft, wellSaveFailure}
	if wellFullCardMode {
		footerControls = append(footerControls, wellApproveFailure)
	}
	footerControls = append(footerControls, wellClearFailure, wellCancelFailure)
	formButtonW := (contentW - int32(len(footerControls)-1)*actionGap) / int32(len(footerControls))
	for i, control := range footerControls {
		x := int32(242) + int32(i)*(formButtonW+actionGap)
		width := formButtonW
		if i == len(footerControls)-1 {
			width = int32(242) + contentW - x
		}
		setPos(control, x, h-48, width, 42)
	}

	updateWellCardVisibility()
}

// relayoutWellCardFailureUI closes the first-open timing gap between layout and
// data refresh. On initial entry resize() runs before refreshFailureImportUI()
// has populated failureObjectKeys, so the history table used to keep the old
// selector-free Y coordinate until the user switched tabs. Reapplying only this
// page layout after the chooser data is known makes the first frame identical to
// every later return to the tab.
func relayoutWellCardFailureUI() {
	if hMain == 0 || currentPage != ID_OBJECTS {
		return
	}
	var client RECT
	if result, _, _ := pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&client))); result == 0 {
		return
	}
	w, h := client.Right-client.Left, client.Bottom-client.Top
	if w > 0 && h > 0 {
		layoutWellCardFailureUI(w, h)
	}
}

// fitWellFailureListColumns follows the same sizing contract as the object-selection
// table. The final named column absorbs the rounding remainder; space occupied by
// the native vertical scrollbar is reserved, so no anonymous blank column appears.
func fitWellFailureListColumns(list syscall.Handle, base []int32) {
	if list == 0 || len(base) == 0 {
		return
	}
	var client RECT
	if result, _, _ := pGetClientRect.Call(uintptr(list), uintptr(unsafe.Pointer(&client))); result == 0 {
		return
	}
	available := client.Right - client.Left - 18
	if available <= 0 {
		return
	}
	total := int32(0)
	for _, width := range base {
		total += width
	}
	widths := make([]int32, len(base))
	used := int32(0)
	for i, width := range base {
		widths[i] = max32(42, available*width/total)
		used += widths[i]
	}
	widths[len(widths)-1] += available - used
	for i, width := range widths {
		setListColumnWidth(list, i, max32(42, width))
	}
}

func wellFailureCustomDraw(lParam uintptr, list syscall.Handle) uintptr {
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		if list == failureImportList {
			row := int(draw.NMCD.ItemSpec)
			visible := 0
			filterKey := selectedFailureObjectKey()
			for _, candidate := range pendingFailureImport {
				if filterKey != "" && !failureCandidateMatchesObject(candidate, filterKey) {
					continue
				}
				if visible == row && candidate.OperationalOnly {
					draw.ClrText = uint32(rgb(125, 82, 0))
					draw.ClrTextBk = uint32(rgb(255, 246, 210))
					return CDRF_NEWFONT
				}
				visible++
			}
		}
		rows := wellHistoryRows
		if list == wellManualList {
			rows = wellManualRows
		}
		row := int(draw.NMCD.ItemSpec)
		if row >= 0 && row < len(rows) {
			e := rows[row]
			refreshFailureDerived(&e)
			if e.OperationalOnly {
				draw.ClrText = uint32(rgb(125, 82, 0))
				draw.ClrTextBk = uint32(rgb(255, 246, 210))
				return CDRF_NEWFONT
			}
			if e.FullCompletionPct >= 100 {
				draw.ClrText = uint32(rgb(20, 92, 48))
				draw.ClrTextBk = uint32(rgb(226, 246, 232))
				return CDRF_NEWFONT
			}
		}
		if row >= 0 && row < len(rows) && rows[row].ConclusionApproved {
			draw.ClrText = uint32(rgb(20, 92, 48))
			draw.ClrTextBk = uint32(rgb(226, 246, 232))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func updateWellCardVisibility() {
	if currentPage != ID_OBJECTS {
		return
	}
	hasKey := strings.TrimSpace(selectedObjectKey) != ""
	show(wellBackMain, false)
	show(wellHistoryTab, true)
	show(wellManualTab, hasKey)
	for _, handle := range wellMetrics {
		show(handle, hasKey && wellManualMode && !wellFormActive)
	}
	for _, handle := range wellMetricTitles {
		show(handle, hasKey && wellManualMode && !wellFormActive)
	}
	for _, divider := range wellMetricDividers {
		show(divider, hasKey && wellManualMode && !wellFormActive)
	}
	historyList := !wellManualMode && len(pendingFailureImport) == 0
	importPreview := !wellManualMode && len(pendingFailureImport) > 0
	manualList := hasKey && wellManualMode && !wellFormActive
	show(wellHistoryList, historyList)
	show(wellOpenEvent, historyList || manualList)
	show(wellImportForm, false)
	// The chooser filters only the loaded/persisted failure register. It does
	// not replace the globally pinned working object used by other sections.
	registerChooser := !wellManualMode && !wellFormActive && len(failureObjectKeys) > 0
	show(failureObjectFrame, false)
	show(failureObjectCombo, registerChooser)
	show(failureObjectOpenButton, registerChooser)
	show(failureAnalysisButton, false)
	show(wellDownloadTemplate, !wellManualMode && !wellFormActive)
	show(failureChooseButton, !wellManualMode && !wellFormActive)
	show(failurePreviewButton, !wellManualMode && !wellFormActive)
	show(failureApplyButton, !wellManualMode && !wellFormActive)
	show(failureImportFileLabel, !wellManualMode && !wellFormActive)
	show(failureImportSummary, !wellManualMode && !wellFormActive)
	show(failureImportList, importPreview)
	for _, handle := range []syscall.Handle{wellManualCounter, wellManualList, wellAddFailure} {
		show(handle, manualList)
	}
	show(wellEditFailure, historyList || manualList)
	show(wellDeleteFailure, historyList || manualList)
	form := hasKey && wellManualMode && wellFormActive
	show(wellFormHeading, form)
	show(wellFormProgress, form)
	show(wellFormModeOperational, form)
	show(wellFormModeFull, form)
	show(wellFormError, form)
	for i, handle := range wellFormLabels {
		show(handle, form && wellFormSection == wellCoreSection(i))
	}
	for i, handle := range wellFormFields {
		show(handle, form && wellFormSection == wellCoreSection(i))
	}
	for i, spec := range wellExtraSpecs {
		// RC684: show only fields that are meaningful for the selected exploitation family.
		visible := form && wellFormSection == spec.section && wellExtraApplicable(spec.column)
		show(wellExtraLabels[i], visible)
		show(wellExtraFields[i], visible)
	}
	for i, handle := range wellSectionButtons {
		visible := form && (i == 0 || wellFullCardMode) && wellSectionApplicable(i)
		show(handle, visible)
		pInvalidateRect.Call(uintptr(handle), 0, 1)
	}
	show(wellCatalogButton, form)
	show(wellReferenceButton, form)
	show(wellManualEntryButton, form)
	show(wellFormModeNote, form)
	show(wellSaveDraft, form)
	show(wellSaveFailure, form)
	show(wellApproveFailure, form && wellFullCardMode)
	show(wellClearFailure, form)
	show(wellCancelFailure, form)
	for _, handle := range []syscall.Handle{wellOpenEvent, wellImportForm, wellDownloadTemplate, wellAddFailure, wellEditFailure, wellDeleteFailure, wellSaveDraft, wellSaveFailure, wellApproveFailure, wellClearFailure, wellCancelFailure} {
		if handle != 0 {
			pInvalidateRect.Call(uintptr(handle), 0, 1)
		}
	}
	for _, handle := range []syscall.Handle{wellHistoryTab, wellManualTab} {
		pInvalidateRect.Call(uintptr(handle), 0, 1)
	}
}

func collectHistoryForWell(key string, now time.Time) []FailureEvent {
	rows := failuresForObject(key, now)
	// Five control examples carry an explicit synthetic 36-month failure history.
	// It is generated in memory, never written into the production failure log, and
	// is labelled as teaching data in every row.
	rows = append(rows, teachingFailureHistoryForObject(key, now)...)
	versionByID := map[string]VersionRecord{}
	for _, v := range versions {
		versionByID[v.VersionID] = v
	}
	for _, outcome := range outcomes {
		v, found := versionByID[outcome.VersionID]
		if !found || v.ObjectKey != key || !strings.EqualFold(strings.TrimSpace(outcome.EventType), "отказ") || !inFailureWindow(outcome.Date, now) {
			continue
		}
		rows = append(rows, FailureEvent{ID: outcome.ID, ObjectKey: key, Date: outcome.Date, EventType: outcome.EventType,
			FailedUnit: outcome.FailureMode, Source: outcome.EvidenceSource, SourceCause: outcome.FailureCause,
			Inspection: outcome.DefectFinding, Comment: outcome.Comment, Manual: false})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, _ := failureDate(rows[i].Date)
		right, _ := failureDate(rows[j].Date)
		return left.After(right)
	})
	return rows
}

func wrapWellMetricClass(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= 22 {
		return value
	}
	for _, separator := range []string{" — ", " – ", " - "} {
		if parts := strings.SplitN(value, separator, 2); len(parts) == 2 {
			return strings.TrimSpace(parts[0]) + "\r\n" + strings.TrimSpace(parts[1])
		}
	}
	words := strings.Fields(value)
	if len(words) < 2 {
		return value
	}
	midpoint := len([]rune(value)) / 2
	line := words[0]
	index := 1
	for index < len(words) && len([]rune(line+" "+words[index])) <= midpoint {
		line += " " + words[index]
		index++
	}
	return line + "\r\n" + strings.Join(words[index:], " ")
}

func failureFirstDisplay(values ...string) string {
	for _, value := range values {
		if failureHasValue(value) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func refreshWellCardFailures() {
	if wellHistoryList == 0 {
		return
	}
	key := selectedFailureObjectKey()
	if wellManualMode {
		key = strings.TrimSpace(selectedObjectKey)
	}
	if wellFormActive && wellDraftKey != "" && wellDraftKey != key {
		wellFormActive = false
		wellEditingID = ""
	}
	now := time.Now()
	wellHistoryRows = historyForWell(key, now)
	wellManualRows = failureEntriesForObject(key, now)

	listBeginUpdate(wellHistoryList)
	listClear(wellHistoryList)
	lastDate, lastClass := "—", "—"
	withClass := 0
	for i, sourceEvent := range wellHistoryRows {
		event := sourceEvent
		refreshFailureDerived(&event)
		wellHistoryRows[i] = event
		if lastDate == "—" {
			lastDate = dash(event.Date)
		}
		complication := failureFirstDisplay(event.RefinedComplication, event.PrimaryComplication, event.ComplicationFactor, event.SourceClass, event.UserClass)
		if complication != "" {
			withClass++
			if lastClass == "—" {
				lastClass = complication
			}
		}
		listAdd(wellHistoryList, i, []string{
			dash(event.Date),
			dash(event.EventType),
			failureGNOCompact(event),
			dash(complication),
			failureDataStatus(event),
			dash(event.InvestigationStatus),
			failureSourceLabel(event),
		})
	}
	listEndUpdate(wellHistoryList)
	for i, value := range []string{fmt.Sprint(len(wellHistoryRows)), fmt.Sprint(withClass), lastDate, wrapWellMetricClass(lastClass)} {
		setText(wellMetrics[i], value)
	}

	listBeginUpdate(wellManualList)
	listClear(wellManualList)
	for i, sourceEvent := range wellManualRows {
		event := sourceEvent
		refreshFailureDerived(&event)
		wellManualRows[i] = event
		cause := failureFirstDisplay(event.FinalCause, event.PreliminaryCause, event.SourceCause, event.StopCause)
		investigation := strings.TrimSpace(event.InvestigationStatus)
		if event.Draft {
			investigation = "Черновик"
		} else if event.ConclusionApproved {
			if investigation == "" || strings.EqualFold(investigation, failureNoSelection) {
				investigation = "Утверждено"
			} else {
				investigation = "Утверждено · " + investigation
			}
		}
		listAdd(wellManualList, i, []string{
			dash(event.Date), failureGNOCompact(event), dash(cause), failureDataStatus(event), dash(investigation), failureSourceLabel(event),
		})
	}
	listEndUpdate(wellManualList)

	fitWellFailureListColumns(wellHistoryList, []int32{130, 130, 190, 180, 230, 170, 250})
	fitWellFailureListColumns(wellManualList, []int32{130, 210, 250, 250, 180, 250})
	archiveCaption := fmt.Sprintf("ЖУРНАЛ ОТКАЗОВ · %d ЗАПИСЕЙ     |     РУЧНЫХ ЗАПИСЕЙ: %d", len(wellManualRows), manualFailureCount(key, now))
	if len(wellManualRows) == 0 {
		archiveCaption = "ЖУРНАЛ ОТКАЗОВ ПУСТ"
	}
	setText(wellManualCounter, archiveCaption)
	refreshFailureImportUI()
	// failureObjectKeys is finalized by refreshFailureImportUI(). Position the
	// selector and history table from that final state before the suspended page
	// redraw is released; this prevents the first-open overlap seen in 708-RC.
	relayoutWellCardFailureUI()
	updateWellCardVisibility()
}

func wellFormUsesEditableCombo(index int) bool {
	if wellFormUsesMultilineEdit(index) {
		return false
	}
	return index >= 3 && index < len(wellFormFields) && !wellFormUsesChoice(index)
}

func wellFormUsesMultilineEdit(index int) bool {
	switch index {
	case 13, 14:
		return true
	}
	return false
}

func wellFormUsesChoice(index int) bool {
	switch index {
	case 4, 5, 8, 9, 12, 15:
		return true
	}
	return false
}

const wellChoicePrompt = failureNoSelection

// Keep an imported value even if an older archive used terminology absent from
// the current reference. It remains selectable but is never silently remapped.
func wellSetChoice(index int, options []string, selected string) {
	combo := wellFormFields[index]
	pSendMessageW.Call(uintptr(combo), CB_RESETCONTENT, 0, 0)
	pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(wellChoicePrompt))))
	selection := 0
	for _, option := range options {
		pos, _, _ := pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(option))))
		if strings.EqualFold(strings.TrimSpace(option), strings.TrimSpace(selected)) {
			selection = int(pos)
		}
	}
	if strings.TrimSpace(selected) != "" && selection == 0 {
		pos, _, _ := pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(selected))))
		selection = int(pos)
	}
	pSendMessageW.Call(uintptr(combo), CB_SETCURSEL, uintptr(selection), 0)
	pSendMessageW.Call(uintptr(combo), 0x0153, ^uintptr(0), 27) // CB_SETITEMHEIGHT, selection
	pSendMessageW.Call(uintptr(combo), 0x0153, 0, 26)           // popup rows
	if index == 8 {
		pSendMessageW.Call(uintptr(combo), 0x0160, 720, 0) // CB_SETDROPPEDWIDTH
	} else if index == 5 || index == 9 {
		pSendMessageW.Call(uintptr(combo), 0x0160, 540, 0)
	}
}

func wellChoiceValue(index int) string {
	return strings.TrimSpace(getText(wellFormFields[index]))
}

func wellPopulateCauses(code, title string) {
	branch := wellClassifierBranch(firstNonEmpty(strings.TrimSpace(getText(wellFormFields[3])), wellChoiceValue(4)))
	items := OfficialFailureCauses2024(branch)
	wellCauseChoices = []OfficialFailureCause2024{{}}
	labels := make([]string, 0, len(items)+1)
	selection := 0
	for _, item := range items {
		wellCauseChoices = append(wellCauseChoices, item)
		label := item.Code + " · " + item.Title
		if branch == "" {
			label = item.Branch + " | " + label
		}
		labels = append(labels, label)
		if code != "" && strings.EqualFold(code, item.Code) && strings.EqualFold(title, item.Title) {
			selection = len(labels)
		}
	}
	if title != "" && selection == 0 {
		wellCauseChoices = append(wellCauseChoices, OfficialFailureCause2024{Code: code, Title: title})
		labels = append(labels, "Из архива | "+code+" · "+title)
		selection = len(labels)
	}
	wellSetChoice(8, labels, "")
	pSendMessageW.Call(uintptr(wellFormFields[8]), CB_SETCURSEL, uintptr(selection), 0)
	setText(wellFormFields[7], code)
}

func handleWellCardComboChange(id int) bool {
	if currentPage != ID_OBJECTS || !wellFormActive {
		return false
	}
	switch id {
	case ID_WELL_FORM_COMBO_BASE + 3:
		operation := strings.TrimSpace(getText(wellFormFields[3]))
		if failureHasValue(operation) {
			wellSetChoice(4, wellEquipmentChoices, operation)
		}
		wellSetChoice(5, wellNodesForEquipment(firstNonEmpty(operation, wellChoiceValue(4))), wellChoiceValue(5))
		wellPopulateCauses("", "")
		refreshWellEditableSuggestions()
	case ID_WELL_FORM_COMBO_BASE + 4:
		equipment := wellChoiceValue(4)
		if !failureHasValue(strings.TrimSpace(getText(wellFormFields[3]))) && failureHasValue(equipment) {
			setText(wellFormFields[3], equipment)
		}
		wellSetChoice(5, wellNodesForEquipment(firstNonEmpty(strings.TrimSpace(getText(wellFormFields[3])), equipment)), wellChoiceValue(5))
		wellPopulateCauses("", "")
		refreshWellEditableSuggestions()
	case ID_WELL_FORM_COMBO_BASE + 8:
		index, _, _ := pSendMessageW.Call(uintptr(wellFormFields[8]), CB_GETCURSEL, 0, 0)
		code := ""
		if index < uintptr(len(wellCauseChoices)) && !strings.Contains(getText(wellFormFields[8]), "; ") {
			code = wellCauseChoices[index].Code
		}
		setText(wellFormFields[7], code)
	default:
		// Editable catalog fields also affect completion/applicability, so a
		// generic change while the form is active is still handled below.
		if id < ID_WELL_FORM_COMBO_BASE || id >= ID_WELL_FORM_COMBO_BASE+len(wellFormFields) {
			return false
		}
	}
	refreshFailureFormProgress()
	updateWellCardVisibility()
	return true
}

func failureEventExtraForForm(event FailureEvent, column string) string {
	switch column {
	case "H":
		return firstNonEmpty(event.EquipmentSize2, failureExtraValue(event, column))
	case "I":
		return firstNonEmpty(event.EquipmentSize3, failureExtraValue(event, column))
	case "J":
		return firstNonEmpty(event.EquipmentSize4, failureExtraValue(event, column))
	case "L":
		return firstNonEmpty(event.EventType, failureExtraValue(event, column))
	case "Q":
		return firstNonEmpty(event.AssociatedComplication1, failureExtraValue(event, column))
	case "R":
		return firstNonEmpty(event.AssociatedComplication2, failureExtraValue(event, column))
	case "U":
		return firstNonEmpty(event.TKRS1, failureExtraValue(event, column))
	case "V":
		return firstNonEmpty(event.TKRS2, failureExtraValue(event, column))
	case "W":
		return firstNonEmpty(event.DamageType, failureExtraValue(event, column))
	case "X":
		return firstNonEmpty(event.FinalCause, failureExtraValue(event, column))
	case "Z":
		return firstNonEmpty(event.MethodBeforeFailure, failureExtraValue(event, column))
	case "AA":
		return firstNonEmpty(event.MethodBeforeFailure2, failureExtraValue(event, column))
	case "AB":
		return firstNonEmpty(event.MethodAfterFailure, failureExtraValue(event, column))
	case "AC":
		return firstNonEmpty(event.MethodAfterFailure2, failureExtraValue(event, column))
	case "AD":
		return firstNonEmpty(event.Confirmation, failureExtraValue(event, column))
	case "AF":
		return firstNonEmpty(event.Confidence, failureExtraValue(event, column))
	case "AL":
		return firstNonEmpty(event.FountainPumpType, failureExtraValue(event, column))
	case "AO":
		return firstNonEmpty(event.FountainPumpDepth, failureExtraValue(event, column))
	default:
		return failureExtraValue(event, column)
	}
}

func refreshFailureFormModeUI() {
	if !wellFormActive {
		return
	}
	if wellFullCardMode {
		setText(wellFormModeOperational, "ПРЕДВАРИТЕЛЬНО")
		setText(wellFormModeFull, "● РАСШИРЕННОЕ")
		setText(wellSaveFailure, "Сохранить полную карточку")
	} else {
		setText(wellFormModeOperational, "● ПРЕДВАРИТЕЛЬНО")
		setText(wellFormModeFull, "РАСШИРЕННОЕ")
		setText(wellSaveFailure, "Сохранить предварительное")
	}
	setText(wellFormModeNote, wellSectionHeading(wellFormSection))
}

func refreshFailureFormProgress() {
	if !wellFormActive || wellFormProgress == 0 {
		return
	}
	event := readFailureForm()
	refreshFailureDerived(&event)
	setText(wellFormProgress, "Статус данных: "+failureDataStatus(event))
	refreshFailureFormModeUI()
}

func beginFailureForm(event *FailureEvent) {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return
	}
	wellDraftKey = selectedObjectKey
	wellEditingID = ""
	wellEditingOriginal = FailureEvent{}
	values := [20]string{}
	if event != nil {
		copyEvent := *event
		refreshFailureDerived(&copyEvent)
		wellEditingID = copyEvent.ID
		wellEditingOriginal = copyEvent
		normalizeFailureEquipmentSchema(&copyEvent)
		values = [20]string{
			copyEvent.Date, copyEvent.CycleStartDate, copyEvent.RepairEndDate,
			failureCanonicalOperationMode(copyEvent.OperationMode), failureCanonicalOperationMode(copyEvent.Equipment), copyEvent.FailedUnit,
			copyEvent.StopCause, copyEvent.SourceCauseCode, firstNonEmpty(copyEvent.PreliminaryCause, copyEvent.SourceCause),
			firstNonEmpty(copyEvent.PrimaryComplication, copyEvent.ComplicationFactor, copyEvent.SourceClass),
			copyEvent.Inspection, copyEvent.CommissionConclusion, copyEvent.InvestigationStatus,
			firstNonEmpty(copyEvent.CommissionSource, failureExtraValue(copyEvent, "BP")), copyEvent.Comment,
			firstNonEmpty(copyEvent.RefinedComplication, copyEvent.UserClass), copyEvent.EquipmentBrand, copyEvent.EquipmentSize,
			copyEvent.PreliminaryConclusion, copyEvent.ApprovedConclusion,
		}
		wellFullCardMode = copyEvent.HasFullData
	} else {
		wellFullCardMode = false
		if view, ok := findObjectViewByKey(selectedObjectKey); ok {
			canonical := failureCanonicalOperationMode(view.State.Equipment)
			values[3] = canonical
			values[4] = canonical
		}
	}
	wellFormActive = true
	wellFormSection = 0
	wellActiveFailureField = 0

	for i, value := range values {
		if !wellFormUsesChoice(i) {
			setText(wellFormFields[i], value)
		}
	}
	wellSetChoice(4, wellEquipmentChoices, values[4])
	wellSetChoice(5, wellNodesForEquipment(firstNonEmpty(values[3], values[4])), values[5])
	wellPopulateCauses(values[7], values[8])
	wellSetChoice(9, wellComplicationChoices, values[9])
	wellSetChoice(12, wellInvestigationChoices, values[12])
	wellSetChoice(15, []string{"К1", "К2", "К3", "С1", "С2", "С3", "А1", "А2", "А3", "М1", "М2", "М3", "В1", "В2", "В3", "Прочее"}, values[15])

	for i, spec := range wellExtraSpecs {
		value := ""
		if event != nil {
			value = failureEventExtraForForm(*event, spec.column)
		}
		if wellExtraUsesFreeText(spec.column) {
			setText(wellExtraFields[i], value)
		} else {
			wellFillEditableCombo(wellExtraFields[i], failurePickerSuggestions("extra_"+spec.column, spec.catalog), value)
		}
	}
	refreshWellEditableSuggestions()
	setText(wellFormError, "")
	heading := "Новый отказ"
	if event != nil {
		heading = "Отказ от " + dash(event.Date) + " · изменение"
		if event.Draft {
			heading = "Черновик отказа · дополнение"
		}
		if event.ConclusionApproved {
			heading = "Утверждённое событие · изменение"
		}
	}
	setText(wellFormHeading, heading)
	refreshFailureFormProgress()
	updateWellCardVisibility()
	resize()
}

func readFailureForm() FailureEvent {
	values := [20]string{}
	for i, field := range wellFormFields {
		if wellFormUsesChoice(i) {
			values[i] = wellChoiceValue(i)
		} else {
			values[i] = strings.TrimSpace(getText(field))
		}
	}
	index, _, _ := pSendMessageW.Call(uintptr(wellFormFields[8]), CB_GETCURSEL, 0, 0)
	if index < uintptr(len(wellCauseChoices)) && !strings.Contains(values[8], "; ") && failureHasValue(values[8]) {
		values[7] = wellCauseChoices[index].Code
		values[8] = wellCauseChoices[index].Title
	}

	event := wellEditingOriginal
	if event.Extra == nil {
		event.Extra = make(map[string]string)
	} else {
		clone := make(map[string]string, len(event.Extra))
		for key, value := range event.Extra {
			clone[key] = value
		}
		event.Extra = clone
	}
	for i, spec := range wellExtraSpecs {
		value := strings.TrimSpace(getText(wellExtraFields[i]))
		if value == "" {
			delete(event.Extra, spec.column)
		} else {
			event.Extra[spec.column] = value
		}
	}

	event.ID, event.ObjectKey = wellEditingID, wellDraftKey
	event.Date, event.CycleStartDate, event.RepairEndDate = values[0], values[1], values[2]
	event.OperationMode, event.Equipment, event.FailedUnit = values[3], values[4], values[5]
	event.StopCause, event.SourceCauseCode = values[6], values[7]
	event.PreliminaryCause, event.SourceCause = values[8], values[8]
	event.NormalizedCause = normalizeFailureCause(values[8])
	event.PrimaryComplication, event.ComplicationFactor = values[9], values[9]
	event.Inspection = values[10]
	event.CommissionConclusion, event.InvestigationStatus = values[11], values[12]
	event.CommissionSource, event.Comment = values[13], values[14]
	event.RefinedComplication, event.UserClass = values[15], values[15]
	event.EquipmentBrand, event.EquipmentSize = values[16], values[17]
	event.PreliminaryConclusion, event.ApprovedConclusion = values[18], values[19]

	event.EquipmentSize2 = wellExtraText("H")
	// GNO 3/4 were removed from the approved ORD form. Preserve neither stale
	// UI values nor imported legacy values when the event is saved again.
	event.EquipmentSize3 = ""
	event.EquipmentSize4 = ""
	event.EventType = wellExtraText("L")
	event.AssociatedComplication1 = wellExtraText("Q")
	event.AssociatedComplication2 = wellExtraText("R")
	event.TKRS1, event.TKRS2 = wellExtraText("U"), wellExtraText("V")
	event.DamageType, event.FinalCause = wellExtraText("W"), wellExtraText("X")
	event.MethodBeforeFailure, event.MethodBeforeFailure2 = wellExtraText("Z"), wellExtraText("AA")
	event.MethodAfterFailure, event.MethodAfterFailure2 = wellExtraText("AB"), wellExtraText("AC")
	event.Confirmation, event.Confidence = wellExtraText("AD"), wellExtraText("AF")
	event.FountainPumpType, event.FountainPumpDepth = wellExtraText("AL"), wellExtraText("AO")
	normalizeFailureEquipmentSchema(&event)

	// Mirror the current approved Excel schema so a manual event and an imported
	// event have the same logical shape without showing duplicate controls.
	mirrors := map[string]string{
		"M": event.StopCause, "O": event.PreliminaryCause, "P": event.PrimaryComplication,
		"Y": event.RefinedComplication, "AE": event.InvestigationStatus,
		"BO": event.CommissionConclusion, "BP": event.CommissionSource,
	}
	for column, value := range mirrors {
		if strings.TrimSpace(value) == "" {
			delete(event.Extra, column)
		} else {
			event.Extra[column] = strings.TrimSpace(value)
		}
	}

	if !failureHasValue(event.EventType) {
		event.EventType = "Отказ"
	}
	if event.ID == "" {
		event.Manual = true
		event.SourceType = "manual"
		event.Source = "Ручной ввод"
		if view, ok := findObjectViewByKey(wellDraftKey); ok {
			event.Department = currentOrganizationName()
			event.ProductionUnit = stateProductionUnit(view.State)
			event.Field = view.State.Field
			event.Well = view.State.Well
			event.ObjectPlast = view.State.ObjectPlast
		} else {
			for _, old := range failureEvents {
				if old.ObjectKey == wellDraftKey && old.Field != "" && old.Well != "" {
					event.Field, event.Well, event.ObjectPlast = old.Field, old.Well, old.ObjectPlast
					break
				}
			}
		}
	}
	refreshFailureDerived(&event)
	return event
}

func storedFailureEvent(id, key string) (FailureEvent, bool) {
	for _, event := range failureEvents {
		if event.ID == id && event.ObjectKey == key {
			return event, true
		}
	}
	return FailureEvent{}, false
}

func failureFormHasText() bool {
	for _, field := range wellExtraFields {
		if strings.TrimSpace(getText(field)) != "" {
			return true
		}
	}
	for i, field := range wellFormFields {
		value := strings.TrimSpace(getText(field))
		if wellFormUsesChoice(i) && value == wellChoicePrompt {
			continue
		}
		if value != "" {
			return true
		}
	}
	return false
}

func confirmFailureFormDiscard() bool {
	if !failureFormHasText() {
		return true
	}
	message := p("Несохранённые сведения об отказе будут потеряны. Продолжить?")
	title := p("РОСА — черновик отказа")
	response, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x4|MB_ICONWARNING)
	return response == 6
}

func handleWellCardCommand(id int) bool {
	switch id {
	case ID_WELL_FORM_MODE_OPERATIONAL:
		wellFullCardMode = false
		if wellFormSection > 0 {
			wellFormSection = 0
		}
		refreshFailureFormProgress()
		resize()
		return true
	case ID_WELL_FORM_MODE_FULL:
		wellFullCardMode = true
		refreshFailureFormProgress()
		resize()
		return true
	case ID_WELL_SECTION_BASE, ID_WELL_SECTION_BASE + 1, ID_WELL_SECTION_BASE + 2, ID_WELL_SECTION_BASE + 3, ID_WELL_SECTION_BASE + 4, ID_WELL_SECTION_BASE + 5, ID_WELL_SECTION_BASE + 6:
		wellFormSection = id - ID_WELL_SECTION_BASE
		setText(wellFormModeNote, wellSectionHeading(wellFormSection))
		updateWellCardVisibility()
		return true
	case ID_WELL_CATALOG:
		openFailureMultiPicker()
		return true
	case ID_WELL_REFERENCE:
		openFailureCatalogBrowser()
		return true
	case ID_WELL_MANUAL_ENTRY:
		if field := wellVisibleCatalogField(); field != 0 {
			pSendMessageW.Call(uintptr(field), 0x014F, 0, 0)
			pSetFocus.Call(uintptr(field))
			return true
		}
		for i, spec := range wellExtraSpecs {
			if wellFormSection == spec.section && spec.catalog != "" {
				pSetFocus.Call(uintptr(wellExtraFields[i]))
				return true
			}
		}
		for i, field := range wellFormFields {
			if wellCoreSection(i) == wellFormSection {
				pSetFocus.Call(uintptr(field))
				return true
			}
		}
		return true
	case ID_FAILURE_IMPORT, ID_FAILURE_CHOOSE:
		// Keep the import action inside the first module.  This direct branch is
		// intentionally before the card-specific tabs so a native owner-drawn
		// button cannot be swallowed by the generic command router.
		if currentPage != ID_OBJECTS {
			return false
		}
		chooseFailureWorkbookUI()
		refreshWellCardFailures()
		return true
	case ID_FAILURE_PREVIEW:
		if currentPage != ID_OBJECTS {
			return false
		}
		previewFailureWorkbookUI()
		refreshWellCardFailures()
		return true
	case ID_FAILURE_APPLY:
		if currentPage != ID_OBJECTS {
			return false
		}
		applyFailureWorkbookUI()
		return true
	case ID_FAILURE_OBJECT_OPEN:
		if currentPage != ID_OBJECTS {
			return false
		}
		openFailureObjectHistoryUI()
		return true
	case ID_FAILURE_ANALYSIS:
		if currentPage != ID_OBJECTS {
			return false
		}
		openFailureAnalysisUI()
		return true
	case ID_WELL_BACK_MAIN:
		switchPage(ID_DASH)
	case ID_WELL_HISTORY_TAB:
		if wellFormActive && !confirmFailureFormDiscard() {
			return true
		}
		wellFormActive = false
		wellEditingID = ""
		wellEditingOriginal = FailureEvent{}
		wellManualMode = false
		refreshWellCardFailures()
		resize()
	case ID_WELL_MANUAL_TAB:
		if wellFormActive && !confirmFailureFormDiscard() {
			return true
		}
		wellFormActive = false
		wellManualMode = true
		refreshWellCardFailures()
		resize()
	case ID_WELL_OPEN_EVENT:
		key := strings.TrimSpace(selectedObjectKey)
		if key == "" {
			key = selectedFailureObjectKey()
		}
		rows := historyForWell(key, time.Now())
		if len(rows) == 0 {
			msg("Анализ по объекту", "Для выбранного объекта пока нет отказов за 36 месяцев.", MB_OK|MB_ICONINFORMATION)
			return true
		}
		openFailureReportDialog("Анализ отказов · "+wellFailureObjectCaption(key), failureAnalysisReport(rows, key), "анализ_отказов")
	case ID_WELL_IMPORT_FORM:
		// Compatibility command: importing is now performed entirely in this
		// first module; never redirect to the ninth service page.
		chooseFailureWorkbookUI()
		refreshWellCardFailures()
	case ID_WELL_DOWNLOAD_TEMPLATE:
		downloadFailureTemplate()
	case ID_WELL_ADD_FAILURE:
		beginFailureForm(nil)
		resize()
	case ID_WELL_EDIT_FAILURE:
		list := wellManualList
		rows := wellManualRows
		if !wellManualMode {
			list, rows = wellHistoryList, wellHistoryRows
		}
		i := selectedRow(list)
		if i < 0 || i >= len(rows) {
			msg("Журнал отказов", "Выберите запись в списке.", MB_OK|MB_ICONWARNING)
			return true
		}
		e, ok := storedFailureEvent(rows[i].ID, selectedObjectKey)
		if !ok {
			msg("Журнал отказов", "Эта строка сформирована из другого раздела и не является отдельной записью отказа. Измените исходные данные в соответствующем разделе.", MB_OK|MB_ICONINFORMATION)
			return true
		}
		wellManualMode = true
		beginFailureForm(&e)
		resize()
	case ID_WELL_DELETE_FAILURE:
		list := wellManualList
		rows := wellManualRows
		if !wellManualMode {
			list, rows = wellHistoryList, wellHistoryRows
		}
		i := selectedRow(list)
		if i < 0 || i >= len(rows) {
			msg("Журнал отказов", "Выберите запись для удаления.", MB_OK|MB_ICONWARNING)
			return true
		}
		e, ok := storedFailureEvent(rows[i].ID, selectedObjectKey)
		if !ok {
			msg("Журнал отказов", "Эта строка сформирована из другого раздела и не хранится как отдельная запись отказа; удалить её здесь нельзя.", MB_OK|MB_ICONINFORMATION)
			return true
		}
		text := p(fmt.Sprintf("Удалить отказ от %s (%s)? Изменение останется в журнале.", e.Date, dash(e.FailedUnit)))
		title := p("РОСА — удалить отказ")
		r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x4|MB_ICONWARNING)
		if r == 6 {
			if err := deleteFailureEvent(e.ID, selectedObjectKey); err != nil {
				msg("Удаление отказа", err.Error(), MB_OK|MB_ICONERROR)
			} else {
				addSessionTrashItem("Запись отказа", e.ObjectKey+"\r\nДата: "+e.Date+"\r\nУзел: "+dash(e.FailedUnit), func() error {
					if err := recordFailureChange("restore", e); err != nil {
						return err
					}
					failureEvents = append(failureEvents, e)
					refreshWellCardFailures()
					resize()
					return nil
				}, nil)
				refreshWellCardFailures()
				resize()
			}
		}
	case ID_WELL_CLEAR_FAILURE:
		if confirmFailureFormDiscard() {
			for i, field := range wellFormFields {
				if !wellFormUsesChoice(i) {
					setText(field, "")
				}
			}
			wellSetChoice(4, wellEquipmentChoices, "")
			wellSetChoice(5, wellNodesForEquipment(""), "")
			wellPopulateCauses("", "")
			wellSetChoice(9, wellComplicationChoices, "")
			wellSetChoice(12, wellInvestigationChoices, "")
			wellSetChoice(15, []string{"К1", "К2", "К3", "С1", "С2", "С3", "А1", "А2", "А3", "М1", "М2", "М3", "В1", "В2", "В3", "Прочее"}, "")
			setText(wellFormError, "")
			for _, field := range wellExtraFields {
				setText(field, "")
			}
			refreshWellEditableSuggestions()
			refreshFailureFormProgress()
			updateWellCardVisibility()
		}
	case ID_WELL_CANCEL_FAILURE:
		if confirmFailureFormDiscard() {
			wellFormActive = false
			wellEditingID = ""
			wellEditingOriginal = FailureEvent{}
			refreshWellCardFailures()
			resize()
		}
	case ID_WELL_SAVE_DRAFT, ID_WELL_SAVE_FAILURE, ID_WELL_APPROVE_FAILURE:
		if !wellFormActive || wellDraftKey != selectedObjectKey {
			setText(wellFormError, "Объект изменился. Вернитесь на Главную и откройте запись заново.")
			return true
		}
		e := readFailureForm()
		if id == ID_WELL_SAVE_DRAFT {
			if wellEditingOriginal.ID != "" && !wellEditingOriginal.Draft {
				setText(wellFormError, "Сохранённый отказ нельзя перевести в черновик. Используйте предварительное заключение.")
				return true
			}
			e.Draft = true
			e.ConclusionApproved = false
			if err := saveFailureEvent(e, time.Now()); err != nil {
				setText(wellFormError, err.Error())
				return true
			}
			wellFormActive = false
			wellEditingID = ""
			wellEditingOriginal = FailureEvent{}
			refreshWellCardFailures()
			msg("Карточка скважины", "Черновик сохранён. Откройте его через «Изменить запись» для дополнения.", MB_OK|MB_ICONINFORMATION)
			return true
		}
		e.Draft = false
		approved := id == ID_WELL_APPROVE_FAILURE
		if approved && !failureHasValue(e.ApprovedConclusion) {
			setText(wellFormError, "Для утверждения заполните утверждённое заключение. Процент полной карточки при этом может быть меньше 100%.")
			return true
		}
		e.ConclusionApproved = approved
		confirmAndStoreFailureFormCustomValues()
		if old, duplicate := duplicateFailure(e); duplicate {
			note := p(fmt.Sprintf("У объекта уже есть событие от %s с типом «%s» (источник: %s). Сохранить вторую запись?", old.Date, old.EventType, old.Source))
			title := p("РОСА — возможный дубль")
			r, _, _ := pMessageBoxW.Call(uintptr(hMain), uintptr(unsafe.Pointer(note)), uintptr(unsafe.Pointer(title)), 0x4|MB_ICONWARNING)
			if r != 6 {
				return true
			}
		}
		if err := saveFailureEvent(e, time.Now()); err != nil {
			setText(wellFormError, err.Error())
			return true
		}
		wellFormActive = false
		wellEditingID = ""
		wellEditingOriginal = FailureEvent{}
		refreshWellCardFailures()
		if approved {
			msg("Карточка скважины", "Событие утверждено. Полнота карточки сохраняется как отдельный статус и не подменяется утверждением.", MB_OK|MB_ICONINFORMATION)
		} else if wellFullCardMode {
			msg("Карточка скважины", "Полная карточка сохранена. Текущий статус: "+failureDataStatus(e)+".", MB_OK|MB_ICONINFORMATION)
		} else {
			msg("Карточка скважины", "Оперативные сведения сохранены. Текущий статус: "+failureDataStatus(e)+".", MB_OK|MB_ICONINFORMATION)
		}
	default:
		return false
	}
	return true
}
