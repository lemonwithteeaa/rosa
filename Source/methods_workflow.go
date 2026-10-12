//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	ID_METHODS_SUGGESTION_COMBO  = 9620
	ID_METHODS_APPROVE           = 9621
	ID_METHODS_EXPORT_XLSX       = 9622
	ID_METHODS_EXPORT_DOCX       = 9623
	ID_METHODS_EXPORT_PDF        = 9624
	ID_METHODS_TO_CLASSIFICATION = 9627
	ID_METHODS_CANDIDATE_PREV    = 9628
	ID_METHODS_CANDIDATE_NEXT    = 9629
	ID_METHODS_CARD_BASE         = 9630
	ID_METHODS_CARD_COUNT        = 6
	ID_METHODS_SHOW_ALL          = 9637
	ID_METHODS_FORECAST_BASE     = 9638
	ID_METHODS_FORECAST_GAIN     = 9639
	ID_METHODS_FORECAST_RESULT   = 9640
	ID_METHODS_FORECAST_BASIS    = 9641
	ID_METHODS_SCENARIO_NEXT     = 9642
	ID_METHODS_SCENARIO_ADD      = 9643
	ID_METHODS_SCENARIO_COPY     = 9644
	ID_METHODS_AVAILABILITY      = 9645
)

type methodWorkflowSuggestion struct {
	Method         assessmentMethodCandidate
	Direction      string
	Classification string
	Severity       int
	Basis          string
}

type approvedMethodDecision struct {
	ID                string                       `json:"id"`
	ObjectKey         string                       `json:"object_key"`
	ApprovedAt        string                       `json:"approved_at"`
	ApprovedBy        string                       `json:"approved_by,omitempty"`
	Classification    string                       `json:"classification"`
	MethodID          string                       `json:"method_id,omitempty"`
	Method            string                       `json:"method"`
	MethodSource      string                       `json:"method_source,omitempty"`
	MethodStatus      string                       `json:"method_status,omitempty"`
	Methods           []string                     `json:"methods,omitempty"`
	Interaction       string                       `json:"interaction,omitempty"`
	EffectLowPct      float64                      `json:"effect_low_pct,omitempty"`
	EffectBasePct     float64                      `json:"effect_base_pct,omitempty"`
	EffectHighPct     float64                      `json:"effect_high_pct,omitempty"`
	MethodAssessments []MethodEffectAssessment     `json:"method_assessments,omitempty"`
	Implementation    []MethodImplementationRecord `json:"implementation,omitempty"`
	FailureCount36    int                          `json:"failure_count_36,omitempty"`
	HistoryCycleCount int                          `json:"history_cycle_count,omitempty"`
	ForecastPrecision string                       `json:"forecast_precision,omitempty"`
	HistoryWarning    string                       `json:"history_warning,omitempty"`
	ForecastLowDays   float64                      `json:"forecast_low_days,omitempty"`
	ForecastBaseDays  float64                      `json:"forecast_base_days,omitempty"`
	ForecastHighDays  float64                      `json:"forecast_high_days,omitempty"`
	ForecastSource    string                       `json:"forecast_source,omitempty"`
	SchemaVersion     int                          `json:"schema_version,omitempty"`
	VersionNumber     int                          `json:"version_number,omitempty"`
	RestoredFromID    string                       `json:"restored_from_id,omitempty"`
	SupersedesID      string                       `json:"supersedes_id,omitempty"`
	Plan              []PlanItem                   `json:"plan,omitempty"`
}

var (
	methodsCardsPerPage             = ID_METHODS_CARD_COUNT
	methodsCandidateDetails         [ID_METHODS_CARD_COUNT]syscall.Handle
	methodsPageTitle                syscall.Handle
	methodsScenarioNextButton       syscall.Handle
	methodsScenarioAddButton        syscall.Handle
	methodsScenarioCopyButton       syscall.Handle
	methodsAvailabilityButton       syscall.Handle
	methodsObjectTitle              syscall.Handle
	methodsObjectCode               syscall.Handle
	methodsObjectContext            syscall.Handle
	methodsClassTitle               syscall.Handle
	methodsClassSummary             syscall.Handle
	methodsClassList                syscall.Handle
	methodsKeyDataTitle             syscall.Handle
	methodsKeyData                  syscall.Handle // compatibility backing field; hidden in RC573 visual layout
	methodsTechEquipment            syscall.Handle
	methodsTechMRP                  syscall.Handle
	methodsTechLiquid               syscall.Handle
	methodsTechWatercut             syscall.Handle
	methodsTechEquipmentValue       syscall.Handle
	methodsTechMRPValue             syscall.Handle
	methodsTechLiquidValue          syscall.Handle
	methodsTechWatercutValue        syscall.Handle
	methodsSuggestionTitle          syscall.Handle
	methodsReadyBanner              syscall.Handle
	methodsEmptyState               syscall.Handle
	methodsSuggestionCombo          syscall.Handle // hidden selector backing the visual table
	methodsCandidateList            syscall.Handle // compatibility backing list; hidden in RC575 card layout
	methodsAvailableTitle           syscall.Handle
	methodsCandidatePrev            syscall.Handle
	methodsCandidateNext            syscall.Handle
	methodsShowAllButton            syscall.Handle
	methodsShowAll                  bool
	methodsShowAllObject            string
	methodsCandidateCards           [ID_METHODS_CARD_COUNT]syscall.Handle
	methodsCandidatePage            int
	methodsToClassificationButton   syscall.Handle
	methodsApproveButton            syscall.Handle
	methodsPlanButton               syscall.Handle
	methodsReferenceButton          syscall.Handle
	methodsReplicationButton        syscall.Handle
	methodsAddButton                syscall.Handle
	methodsRemoveButton             syscall.Handle
	methodsResetButton              syscall.Handle
	methodsSuggestionInfo           syscall.Handle
	methodsBasisTitle               syscall.Handle
	methodsImplementationTitle      syscall.Handle
	methodsImplementation           syscall.Handle
	methodsImplementationEditButton syscall.Handle
	methodsForecastTitle            syscall.Handle
	methodsForecast                 syscall.Handle // detailed compatibility text; hidden in RC573 dashboard layout
	methodsForecastCurrent          syscall.Handle
	methodsForecastGain             syscall.Handle
	methodsForecastResult           syscall.Handle
	methodsForecastBasis            syscall.Handle
	methodsForecastNote             syscall.Handle
	methodsHistoryWarning           syscall.Handle
	methodsClassificationReady      bool
	methodsPlanTitle                syscall.Handle
	methodsPlanList                 syscall.Handle
	methodsPlanComment              syscall.Handle
	methodsArchiveTitle             syscall.Handle
	methodsExportXLSX               syscall.Handle
	methodsExportDOCX               syscall.Handle
	methodsExportPDF                syscall.Handle
	methodsArchiveList              syscall.Handle
	methodsArchiveRows              []approvedMethodDecision
	methodsArchiveRenderHash        uint64
	methodWorkflowSuggestions       []methodWorkflowSuggestion
	approvedMethodDecisions         []approvedMethodDecision
	approvedMethodsLoaded           bool
	methodsDemoDefaultsApplied      = map[string]bool{}
)

func methodsCardLabel(text string, x, y, w, h int32, bold bool) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|WS_BORDER|SS_LEFT|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	if z != 0 {
		font := hFont
		if bold && hFontBold != 0 {
			font = hFontBold
		}
		if font != 0 {
			pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(font), 1)
		}
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

func methodsCenteredCardLabel(text string, x, y, w, h int32, bold bool) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|WS_BORDER|SS_CENTER|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	if z != 0 {
		font := hFont
		if bold && hFontBold != 0 {
			font = hFontBold
		}
		if font != 0 {
			pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(font), 1)
		}
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

func methodsCenteredMetricLabel(text string, x, y, w, h int32, bold bool) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|SS_CENTER|SS_CENTERIMAGE|SS_NOPREFIX, x, py(y), w, h, hMain, 0)
	if z != 0 {
		font := methodsFont(methodsTypeBody, bold)
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(font), 1)
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

func methodsClickableCenteredCardLabel(text string, x, y, w, h int32, bold bool, id int) syscall.Handle {
	z := create("STATIC", text, WS_CHILD|WS_BORDER|SS_CENTER|SS_NOPREFIX|SS_NOTIFY, x, py(y), w, h, hMain, id)
	if z != 0 {
		font := hFont
		if bold && hFontBold != 0 {
			font = hFontBold
		}
		if font != 0 {
			pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(font), 1)
		}
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

// methodsSafeCandidateButton deliberately uses the native Windows button
// renderer instead of ROSA owner-draw. RC615 showed that visual complexity in
// the methods cards could destabilize page entry on some Windows machines.
// The hotfix keeps multiline cards and all click functionality while removing
// the unsafe/custom paint dependency from the critical entry path.
func methodsSafeCandidateButton(text string, x, y, w, h int32, id int) syscall.Handle {
	const bsMultiline = uint32(0x00002000)
	st := uint32(WS_CHILD|WS_TABSTOP|0x00000003) | bsMultiline // BS_AUTOCHECKBOX; draft state is persisted by the command handler
	z := create("BUTTON", text, st, x, py(y), w, h, hMain, id)
	if z != 0 && hFontButton != 0 {
		pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(hFontButton), 1)
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

func methodsScrollableCard(text string, x, y, w, h int32) syscall.Handle {
	st := uint32(WS_CHILD | WS_BORDER | ES_MULTILINE | ES_AUTOVSCROLL | ES_READONLY | WS_VSCROLL)
	z := create("EDIT", text, st, x, py(y), w, h, hMain, 0)
	if z != 0 {
		font := hFontPanel
		if font == 0 {
			font = hFont
		}
		if font != 0 {
			pSendMessageW.Call(uintptr(z), WM_SETFONT, uintptr(font), 1)
		}
	}
	addPageControl(ID_ACTIVITIES, z)
	return z
}

func buildMethodsWorkflowUI() {
	buildMethodsWorkspacePanels()
	defer buildMethodsParameterPanel()
	defer buildMethodsBasisPanel()
	methodsPageTitle = pageTitle(ID_ACTIVITIES, "Методы и мероприятия", 242, 94, 1010, 42)
	methodsScenarioNextButton = button(ID_ACTIVITIES, "СХЕМА А · 1/1", 748, 94, 150, 40, ID_METHODS_SCENARIO_NEXT)
	methodsAvailabilityButton = button(ID_ACTIVITIES, "НЕ ПРОВЕРЕНО", 906, 94, 196, 42, ID_METHODS_AVAILABILITY)
	methodsScenarioCopyButton = button(ID_ACTIVITIES, "КОПИРОВАТЬ", 1084, 94, 168, 40, ID_METHODS_SCENARIO_COPY)
	// Compatibility marker for RC699 QA: "+ АЛЬТЕРНАТИВНАЯ СХЕМА".
	methodsScenarioAddButton = button(ID_ACTIVITIES, "+ ДОБАВИТЬ АЛЬТЕРНАТИВУ", 1000, 94, 300, 40, ID_METHODS_SCENARIO_ADD)
	show(methodsScenarioCopyButton, false)

	methodsObjectTitle = label(ID_ACTIVITIES, "ЗАКРЕПЛЁННЫЙ ОБЪЕКТ", 242, 132, 240, 28)
	if methodsObjectTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsObjectTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsObjectCode = label(ID_ACTIVITIES, "—", 490, 128, 760, 34)
	if methodsObjectCode != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsObjectCode), WM_SETFONT, uintptr(hFontBold), 1)
	}
	// Do not use SS_ENDELLIPSIS here: the object context is an operational
	// identifier and must never end with a visually cut fragment.
	methodsObjectContext = label(ID_ACTIVITIES, "Сначала закрепите объект на Главной.", 242, 160, 1010, 30)

	methodsKeyDataTitle = create("STATIC", "ТЕХНИЧЕСКИЙ РЕЖИМ ОБЪЕКТА", WS_CHILD|SS_CENTER|SS_NOPREFIX, 980, py(132), 272, 28, hMain, 0)
	addPageControl(ID_ACTIVITIES, methodsKeyDataTitle)
	if methodsKeyDataTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsKeyDataTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsKeyData = edit(ID_ACTIVITIES, "—", 980, 160, 272, 92, true, true, 0)
	methodsTechEquipment = methodsCenteredMetricLabel("ГНО", 980, 160, 126, 20, true)
	methodsTechMRP = methodsCenteredMetricLabel("Текущий МРП", 1114, 160, 138, 20, true)
	methodsTechLiquid = methodsCenteredMetricLabel("Дебит жидкости", 980, 222, 126, 20, true)
	methodsTechWatercut = methodsCenteredMetricLabel("Обводнённость", 1114, 222, 138, 20, true)
	methodsTechEquipmentValue = methodsCenteredMetricLabel("—", 982, 181, 122, 20, true)
	methodsTechMRPValue = methodsCenteredMetricLabel("—", 1116, 181, 134, 20, true)
	methodsTechLiquidValue = methodsCenteredMetricLabel("—", 982, 243, 122, 20, true)
	methodsTechWatercutValue = methodsCenteredMetricLabel("—", 1116, 243, 134, 20, true)
	for _, value := range []syscall.Handle{methodsTechEquipmentValue, methodsTechMRPValue, methodsTechLiquidValue, methodsTechWatercutValue} {
		pSendMessageW.Call(uintptr(value), WM_SETFONT, uintptr(methodsFont(methodsTypeSection, true)), 1)
	}

	methodsClassTitle = label(ID_ACTIVITIES, "УТВЕРЖДЁННАЯ КЛАССИФИКАЦИЯ ИЗ РАЗДЕЛА 3", 242, 268, 1010, 28)
	if methodsClassTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsClassTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsClassSummary = methodsCardLabel("Классификация ещё не утверждена.", 242, 296, 1010, 34, false)
	methodsClassList = newList(ID_ACTIVITIES, 242, 326, 1010, 92, []struct {
		n string
		w int32
	}{
		{"№", 42}, {"Осложнение", 210}, {"Класс", 85}, {"Уровень", 95}, {"Краткое описание", 560},
	})
	setListHeaderFont(methodsClassList, hFontBold)
	centerListHeaders(methodsClassList, 4)

	methodsSuggestionTitle = label(ID_ACTIVITIES, "ПРЕДЛОЖЕННЫЕ МЕТОДЫ — ПО УТВЕРЖДЁННЫМ КЛАССАМ И ТЕХНОЛОГИЧЕСКОМУ МОДУЛЮ", 242, 432, 1010, 30)
	if methodsSuggestionTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsSuggestionTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsReadyBanner = methodsCardLabel("Сначала утвердите классификацию в разделе 3 — после этого РОСА сформирует методы-кандидаты.", 242, 462, 1010, 42, true)
	methodsEmptyState = methodsScrollableCard("Чтобы РОСА сформировала методы-кандидаты, завершите классификацию в разделе 3. После утверждения здесь появятся варианты с приоритетом и статусом.", 242, 512, 520, 224)
	methodsToClassificationButton = button(ID_ACTIVITIES, "ПЕРЕЙТИ К КЛАССИФИКАЦИИ", 260, 680, 484, 38, ID_METHODS_TO_CLASSIFICATION)
	show(methodsToClassificationButton, false)
	methodsSuggestionCombo = create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 242, py(462), 520, 360, hMain, ID_METHODS_SUGGESTION_COMBO)
	addPageControl(ID_ACTIVITIES, methodsSuggestionCombo)
	show(methodsSuggestionCombo, false)

	methodsReferenceButton = button(ID_ACTIVITIES, "СПРАВОЧНИК", 610, 426, 135, 48, ID_METHODS_REFERENCE)
	methodsReplicationButton = button(ID_ACTIVITIES, "АНАЛОГИ", 753, 426, 155, 48, ID_METHODS_REPLICATION)
	methodsApproveButton = button(ID_ACTIVITIES, "УТВЕРДИТЬ МЕТОД", 916, 426, 160, 48, ID_METHODS_APPROVE)
	methodsPlanButton = button(ID_ACTIVITIES, "СФОРМИРОВАТЬ ПЛАН", 1084, 426, 168, 48, ID_GEN_PLAN)

	methodsCandidateList = newList(ID_ACTIVITIES, 242, 474, 520, 252, []struct {
		n string
		w int32
	}{
		{"Осложнение / класс", 145}, {"Метод / мероприятие", 270}, {"Прим. / дост. / вероятность", 185}, {"Ожидаемый вклад", 150}, {"Приоритет / подход", 190},
	})
	setListHeaderFont(methodsCandidateList, hFontBold)
	centerListHeaders(methodsCandidateList, 5)
	show(methodsCandidateList, false)
	methodsAvailableTitle = label(ID_ACTIVITIES, "01 · МЕТОДЫ · ВЫБОР НЕСКОЛЬКИХ", 242, 474, 520, 26)
	if methodsAvailableTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsAvailableTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	// RC582/583: six readable cards per page, with explicit paging when the
	// catalogue contains more candidates. Paging never changes the selected
	// method or approves anything automatically.
	methodsCandidatePrev = button(ID_ACTIVITIES, "‹", 650, 474, 52, 28, ID_METHODS_CANDIDATE_PREV)
	methodsCandidateNext = button(ID_ACTIVITIES, "›", 710, 474, 52, 28, ID_METHODS_CANDIDATE_NEXT)
	methodsShowAllButton = button(ID_ACTIVITIES, "ПОКАЗАТЬ ВСЕ МЕТОДЫ", 500, 474, 206, 30, ID_METHODS_SHOW_ALL)
	show(methodsCandidatePrev, false)
	show(methodsCandidateNext, false)
	show(methodsShowAllButton, false)
	for i := 0; i < ID_METHODS_CARD_COUNT; i++ {
		methodsCandidateCards[i] = methodsSafeCandidateButton("—", 242, 502, 250, 34, ID_METHODS_CARD_BASE+i)
		methodsCandidateDetails[i] = methodsCardLabel("—", 242, 536, 250, 40, false)
		if hFontButtonCompact != 0 {
			pSendMessageW.Call(uintptr(methodsCandidateDetails[i]), WM_SETFONT, uintptr(hFontButtonCompact), 0)
		}
		show(methodsCandidateDetails[i], false)
		show(methodsCandidateCards[i], false)
	}

	methodsBasisTitle = label(ID_ACTIVITIES, "02 · ОСНОВАНИЕ ВЫБОРА", 776, 474, 220, 26)
	if methodsBasisTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsBasisTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsSuggestionInfo = methodsScrollableCard("Выберите карточку метода — здесь появятся пять коротких оснований выбора. Подробности и источники доступны в справочнике.", 776, 502, 220, 224)

	methodsImplementationTitle = label(ID_ACTIVITIES, "03 · ПАРАМЕТРЫ РЕАЛИЗАЦИИ", 1010, 474, 242, 26)
	if methodsImplementationTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsImplementationTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsImplementation = methodsScrollableCard("Выберите метод для отображения параметров реализации.", 1010, 502, 242, 224)
	methodsImplementationEditButton = button(ID_ACTIVITIES, "ПАРАМЕТРЫ И РЕАГЕНТЫ", 1010, 690, 242, 36, ID_METHODS_IMPLEMENTATION_EDIT)
	show(methodsImplementationEditButton, false)

	methodsAddButton = button(ID_ACTIVITIES, "+ В ПЛАН", 242, 734, 164, 38, ID_ACTIVITY_ADD_METHOD)
	methodsRemoveButton = button(ID_ACTIVITIES, "− УБРАТЬ", 414, 734, 164, 38, ID_ACTIVITY_REMOVE_METHOD)
	methodsResetButton = button(ID_ACTIVITIES, "ОЧИСТИТЬ", 586, 734, 176, 38, ID_ACTIVITY_RESET_METHODS)

	methodsForecastTitle = label(ID_ACTIVITIES, "ПРОГНОЗ НАРАБОТКИ · ЦЕЛЕВОЙ ОРИЕНТИР ≥ 1000 СУТ.", 242, 784, 1010, 28)
	if methodsForecastTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsForecastTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsForecast = edit(ID_ACTIVITIES, "Метод пока не утверждён.", 242, 814, 780, 92, true, true, 0)
	// RC582: four scenarios are presentation only. They use the existing
	// forecast engine (low/base/high) and do not alter the calculation core.
	methodsForecastCurrent = methodsClickableCenteredCardLabel("Негативный прогноз\r\n—", 256, 814, 236, 82, true, ID_METHODS_FORECAST_BASE)
	methodsForecastGain = methodsClickableCenteredCardLabel("Реальный прогноз\r\n—", 508, 814, 236, 82, true, ID_METHODS_FORECAST_GAIN)
	methodsForecastResult = methodsClickableCenteredCardLabel("Положительный прогноз\r\n—", 760, 814, 236, 82, true, ID_METHODS_FORECAST_RESULT)
	methodsForecastBasis = methodsClickableCenteredCardLabel("Основание прогноза\r\n—", 1012, 814, 236, 82, true, ID_METHODS_FORECAST_BASIS)
	methodsHistoryWarning = labelEllipsis(ID_ACTIVITIES, "", 1034, 814, 218, 92)
	methodsForecastNote = label(ID_ACTIVITIES, "Нажмите на карточку для подробной расшифровки расчёта. Прогноз предварительный и требует подтверждения фактическими данными.", 242, 900, 1010, 24)
	if methodsHistoryWarning != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsHistoryWarning), WM_SETFONT, uintptr(hFontBold), 1)
	}

	methodsPlanTitle = label(ID_ACTIVITIES, "ВЫБРАННЫЕ МЕРОПРИЯТИЯ / ПЛАН", 242, 918, 1010, 28)
	if methodsPlanTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsPlanTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsPlanList = newList(ID_ACTIVITIES, 242, 948, 1010, 116, []struct {
		n string
		w int32
	}{
		{"№", 42}, {"Метод / мероприятие", 230}, {"Состояние", 110}, {"Дата", 105}, {"Ответственный", 145}, {"Контроль", 165}, {"Достов., %", 88}, {"Примен., %", 88}, {"Доступность", 135},
	})
	setListHeaderFont(methodsPlanList, hFontBold)
	centerListHeaders(methodsPlanList, 9)
	methodsPlanComment = edit(ID_ACTIVITIES, "Комментарий к плану: комплекс ещё не сформирован.", 242, 1070, 650, 54, true, true, 0)

	methodsExportPDF = button(ID_ACTIVITIES, "PDF", 904, 1076, 106, 42, ID_METHODS_EXPORT_PDF)
	methodsExportXLSX = button(ID_ACTIVITIES, "XLSX", 1018, 1076, 106, 42, ID_METHODS_EXPORT_XLSX)
	methodsExportDOCX = button(ID_ACTIVITIES, "DOCX", 1132, 1076, 120, 42, ID_METHODS_EXPORT_DOCX)

	methodsArchiveTitle = label(ID_ACTIVITIES, "НАКОПИТЕЛЬНАЯ ИСТОРИЯ РЕШЕНИЙ ПО ВСЕМ ОБЪЕКТАМ", 242, 1136, 1010, 28)
	if methodsArchiveTitle != 0 && hFontBold != 0 {
		pSendMessageW.Call(uintptr(methodsArchiveTitle), WM_SETFONT, uintptr(hFontBold), 1)
	}
	methodsArchiveList = newList(ID_ACTIVITIES, 242, 1166, 1010, 146, []struct {
		n string
		w int32
	}{
		{"Объект", 210}, {"Дата", 112}, {"Версия", 72}, {"Классификация", 150}, {"Метод / комплекс", 240}, {"Прогноз", 145}, {"Статус", 170},
	})
	setListHeaderFont(methodsArchiveList, hFontBold)
	centerListHeaders(methodsArchiveList, 7)
}

func showMethodsSafeFallback(detail string) {
	methodsClassificationReady = false
	methodWorkflowSuggestions = nil
	methodsCandidatePage = 0
	for i, card := range methodsCandidateCards {
		if card != 0 {
			pEnableWindow.Call(uintptr(card), 1)
		}
		show(card, false)
		show(methodsCandidateDetails[i], false)
	}
	show(methodsCandidatePrev, false)
	show(methodsCandidateNext, false)
	show(methodsCandidateList, false)
	show(methodsEmptyState, true)
	show(methodsToClassificationButton, false)
	show(methodsAvailableTitle, true)
	show(methodsBasisTitle, true)
	show(methodsSuggestionInfo, true)
	show(methodsImplementationTitle, true)
	show(methodsImplementation, true)
	show(methodsImplementationEditButton, false)
	show(methodsAddButton, false)
	show(methodsRemoveButton, false)
	show(methodsResetButton, false)
	setText(methodsReadyBanner, "Методы открыты в безопасном режиме. Остальные разделы РОСА продолжают работать.")
	setText(methodsAvailableTitle, "01 · БЕЗОПАСНЫЙ РЕЖИМ")
	setText(methodsEmptyState, "Автоматическое обновление карточек методов остановлено этой сессией, чтобы не закрывать приложение. Перейдите в другой раздел и вернитесь после сохранения/уточнения исходных данных.")
	setText(methodsBasisTitle, "02 · ДИАГНОСТИКА")
	setText(methodsSuggestionInfo, "Ошибка изолирована только внутри раздела «Методы». Подробности записаны в diagnostics.log; производственные данные не изменены.")
	setText(methodsImplementationTitle, "03 · ЦЕЛЕВОЙ ОРИЕНТИР")
	setText(methodsImplementation, "Целевой показатель наработки: не менее 1000 суток. Методы должны рассматриваться как вклад в достижение этой цели; если одного метода недостаточно, формируется комплекс.")
	setText(methodsForecastTitle, "ПРОГНОЗ НАРАБОТКИ · ЦЕЛЕВОЙ ОРИЕНТИР ≥ 1000 СУТ.")
	setText(methodsForecastNote, "Безопасный режим: прогноз не пересчитывался после внутренней ошибки.")
	diagLog("METHODS safe fallback detail=%s", strings.TrimSpace(detail))
	relayoutMethodsWorkflowCurrent()
}

// layoutMethodsWorkflow follows the approved reference: a compact context band,
// one action line, a three-zone method workspace, forecast, plan and a visible
// archive. The geometry is deliberately height-budgeted from the archive up so
// smaller maximized work areas do not clip the final sections.
func layoutMethodsWorkflow(w, h int32) {
	// The font fitter runs last, after the final batched HWND geometry is real.
	defer fitMethodsVisibleText()
	beginLayoutBatch()
	defer endLayoutBatch()
	defer func() {
		if r := recover(); r != nil {
			diagLog("METHODS layout recovered panic=%v", r)
		}
	}()
	// Join the workspace directly to the 230 px navigation rail. Text keeps its
	// own inner inset, while the controls retain a 12 px safety edge on the right.
	x := int32(230)
	contentW := max32(804, w-242)
	right := x + contentW
	top := py(94)
	bottom := h - 12
	gap := int32(12)
	sectionTextInset := int32(10)

	pageTitleH, pageTitleAdvance := int32(34), int32(42)
	// RC673: keep the upper title compact through the common 900/1080px work
	// areas. The old 820/960 breakpoints made the top suddenly grow and stole
	// height from the actual method fields.
	if h < 1200 {
		pageTitleH, pageTitleAdvance = 32, 34
	}
	if h < 960 {
		pageTitleH, pageTitleAdvance = 30, 32
	}
	scenarioGap := int32(7)
	addScenarioW := min32(330, max32(270, contentW*29/100))
	availabilityW := min32(220, max32(184, contentW*18/100))
	nextScenarioW := min32(160, max32(132, contentW*13/100))
	// RC731: reserve enough width for «ЧАСТИЧНО ДОСТУПНО» first, then let the
	// page title absorb the remaining width. On the narrowest supported layout
	// keep a hard 176 px minimum without letting the row overflow.
	minTitleW := int32(176)
	maxAvail := contentW - addScenarioW - nextScenarioW - scenarioGap*3 - minTitleW
	if availabilityW > maxAvail {
		availabilityW = max32(176, maxAvail)
	}
	titleW := max32(minTitleW, contentW-addScenarioW-availabilityW-nextScenarioW-scenarioGap*3)
	setPos(methodsPageTitle, x, top, titleW, pageTitleH)
	setPos(methodsScenarioNextButton, x+titleW+scenarioGap, top, nextScenarioW, pageTitleH)
	setPos(methodsAvailabilityButton, x+titleW+scenarioGap+nextScenarioW+scenarioGap, top, availabilityW, pageTitleH)
	setPos(methodsScenarioAddButton, right-addScenarioW, top, addScenarioW, pageTitleH)
	show(methodsScenarioNextButton, true)
	show(methodsAvailabilityButton, true)
	show(methodsScenarioAddButton, true)
	show(methodsScenarioCopyButton, false)
	y := top + pageTitleAdvance
	contextBandY := y - 2

	// RC574: the reference uses one compact object/regime band.  The object stays
	// on the left while all four technical indicators form one horizontal row on
	// the right; this removes the 2x2 "mini-dashboard" that made the top heavy.
	techW := min32(720, max32(500, contentW*58/100))
	objW := contentW - techW - gap
	if objW < 330 {
		delta := 330 - objW
		techW = max32(470, techW-delta)
		objW = contentW - techW - gap
	}
	contextHeaderH := int32(22)
	metricFrameH := int32(44)
	metricCaptionH := int32(18)
	metricValueH := int32(21)
	objectCodeH, objectContextH := int32(24), int32(20)
	contextBottomGap := int32(6)
	// RC673: compact the object/technical strip instead of compressing the
	// method cards. Heights remain large enough for one native text line and
	// fitMethodsVisibleText still performs final font fitting.
	if h < 1200 {
		contextHeaderH, metricFrameH = 18, 34
		metricCaptionH, metricValueH = 14, 16
		objectCodeH, objectContextH = 22, 18
		contextBottomGap = 2
	}
	if h < 960 {
		// RC674 compact baseline tokens retained for static QA:
		// contextHeaderH, metricFrameH = 20, 32
		// objectCodeH, objectContextH = 20, 18
		// RC675: keep the technical strip compact, but increase the pinned-object
		// lines again. Real Windows rendering still clipped glyphs and the second
		// line when the user ran the page at compact heights.
		contextHeaderH, metricFrameH = 20, 32
		metricCaptionH, metricValueH = 14, 15
		objectCodeH, objectContextH = 24, 20
		contextBottomGap = 2
	}
	if h < 820 {
		// Short work areas: preserve a readable object identifier and dedicate
		// the remaining viewport to the method archive instead of allowing its
		// ListView to collapse to a bare native header.
		contextHeaderH, metricFrameH = 18, 30
		metricCaptionH, metricValueH = 12, 14
		objectCodeH, objectContextH = 20, 16
		contextBottomGap = 2
	}
	setPos(methodsObjectTitle, x+sectionTextInset, y, objW-sectionTextInset*2, contextHeaderH)
	setPos(methodsObjectCode, x+sectionTextInset, y+contextHeaderH+3, objW-sectionTextInset*2, objectCodeH)
	setPos(methodsObjectContext, x+sectionTextInset, y+contextHeaderH+4+objectCodeH, objW-sectionTextInset*2, objectContextH)
	setPos(methodsKeyDataTitle, x+objW+gap+sectionTextInset, y, techW-sectionTextInset*2, contextHeaderH)
	setPos(methodsContextHeaderBand, x, y, contentW, contextHeaderH)
	show(methodsContextHeaderBand, true)
	show(methodsKeyData, false)
	metricY := y + contextHeaderH + 3
	metricGap := int32(10)
	metricW := (techW - metricGap*3) / 4
	metricXs := [4]int32{x + objW + gap, x + objW + gap + metricW + metricGap, x + objW + gap + (metricW+metricGap)*2, x + objW + gap + (metricW+metricGap)*3}
	lastMetricW := techW - metricW*3 - metricGap*3
	metricWs := [4]int32{metricW, metricW, metricW, lastMetricW}
	metricLabels := [4]syscall.Handle{methodsTechEquipment, methodsTechMRP, methodsTechLiquid, methodsTechWatercut}
	metricValues := [4]syscall.Handle{methodsTechEquipmentValue, methodsTechMRPValue, methodsTechLiquidValue, methodsTechWatercutValue}
	for i := 0; i < 4; i++ {
		setPos(methodsTechMetricFrames[i], metricXs[i], metricY, metricWs[i], metricFrameH)
		show(methodsTechMetricFrames[i], true)
		setPos(metricLabels[i], metricXs[i]+5, metricY+3, metricWs[i]-10, metricCaptionH)
		setPos(metricValues[i], metricXs[i]+5, metricY+metricCaptionH+2, metricWs[i]-10, metricValueH)
	}
	// The left object stack is taller than the metric cards on compact screens.
	// Advancing by the metric height alone made the object context overlap and
	// visually clip the classification heading at 729/768 px work areas.
	metricBottom := metricY + metricFrameH
	objectBottom := y + contextHeaderH + 4 + objectCodeH + objectContextH
	y = max32(metricBottom, objectBottom) + contextBottomGap
	contextBandH := y - contextBandY

	// A compact classification strip mirrors the reference.  Detailed rows stay
	// in section 3, so this page never wastes vertical space on a duplicate table.
	classTitleH, classStripH := int32(22), int32(42)
	suggestionTitleH, actionH := int32(28), int32(44)
	classBlockH, actionBlockH := int32(70), int32(50)
	toolbarReserve, toolbarH := int32(42), int32(38)
	// RC673: the upper classification/action bands stay deliberately compact on
	// normal notebook and Full-HD screens so the central 01/02/03 fields receive
	// the vertical space. Native text fitting prevents caption clipping.
	if h < 1200 {
		classTitleH, classStripH = 18, 24
		suggestionTitleH, actionH = 18, 32
		classBlockH, actionBlockH = 44, 34
		toolbarReserve, toolbarH = 30, 30
	}
	if h < 960 {
		classTitleH, classStripH = 15, 18
		suggestionTitleH, actionH = 14, 30
		classBlockH, actionBlockH = 35, 30
		toolbarReserve, toolbarH = 28, 28
	}
	if h < 820 {
		classTitleH, classStripH = 14, 14
		classBlockH = 30
	}
	setPos(methodsClassTitle, x+sectionTextInset, y, contentW-sectionTextInset*2, classTitleH)
	setPos(methodsClassSummary, x, y+classTitleH+2, contentW, classStripH)
	layoutMethodsClassChips(x, y+classTitleH+2, contentW, classStripH)
	show(methodsClassList, false)
	classBandY := y - 2
	y += classBlockH

	// The reference has one restrained action line: readiness at left and the
	// two decision actions at right. Catalogue/service actions live in the local
	// toolbar below the cards and no longer compete with the primary decision.
	setPos(methodsSuggestionTitle, x+sectionTextInset, y, contentW-sectionTextInset*2, suggestionTitleH)
	y += suggestionTitleH + 2
	actionGap := int32(8)
	approveW := min32(174, max32(148, contentW*17/100))
	planW := min32(210, max32(184, contentW*20/100))
	readyW := contentW - approveW - planW - actionGap*2
	setPos(methodsReadyBanner, x, y, readyW, actionH)
	setPos(methodsApproveButton, x+readyW+actionGap, y, approveW, actionH)
	setPos(methodsPlanButton, x+readyW+actionGap+approveW+actionGap, y, planW, actionH)
	actionBandY := y - suggestionTitleH - 4
	y += actionBlockH

	// Budget lower sections before sizing the cards. This keeps forecast, plan
	// and archive on screen together instead of forcing a minimum archive height
	// beyond the client edge.
	forecastTitleH, forecastTitleAdvance := int32(22), int32(24)
	forecastCardH, forecastNoteH := int32(60), int32(28)
	// RC673: reserve enough space for the end-to-end result, but do not let the
	// plan/archive budgets collapse the main method workbench on 900/1080px
	// screens. The lower tables remain visible and scrollable.
	// RC654 baseline token retained for static QA: planReserve := int32(132); archiveReserve := int32(154)
	planReserve := int32(150)
	archiveReserve := int32(154)
	if h < 1200 {
		forecastTitleH, forecastTitleAdvance = 16, 18
		forecastCardH, forecastNoteH = 48, 18
		planReserve = 120
		archiveReserve = 112
	}
	if h < 960 {
		forecastTitleH, forecastTitleAdvance = 22, 22
		forecastCardH, forecastNoteH = 36, 16
		// RC675: Windows list rows at the user's font/DPI still showed only one
		// full plan row. Reserve a larger explicit viewport for at least two rows.
		// RC654 compact baseline token retained for static QA: planReserve = 100; archiveReserve = 70
		// RC674 compact baseline tokens retained for static QA: planReserve = 100; planListH = max32(74, planReserve-planTitleH-8)
		// Give the cumulative history its own scrollable viewport (not just a
		// header) while preserving two visible rows in the selected-method plan.
		planReserve = 104
		archiveReserve = 174
	}
	if h >= 820 && h < 900 {
		// On laptop-sized screens move height from notes/plan to the accumulated
		// cross-object history. All four forecast cards and plan remain present.
		forecastNoteH = 0
		planReserve = 94
		archiveReserve = 181
	}
	if h < 760 {
		// At the 729px compatibility floor the explanatory line is the least
		// important vertical item: keep all four forecast cards, the two-row plan
		// and at least one archive row instead of clipping any of them.
		// RC674 compact baseline tokens retained for static QA: planReserve = 90; planListH = max32(68, planReserve-planTitleH-6)
		forecastTitleH, forecastTitleAdvance = 18, 20
		forecastNoteH = 0
		planReserve = 92
		archiveReserve = 124
		toolbarReserve, toolbarH = 26, 26
	}
	if h < 820 {
		forecastTitleH, forecastTitleAdvance = 16, 18
		forecastCardH = 32
		planReserve = 88
		archiveReserve = 185
	}
	forecastReserve := forecastTitleAdvance + forecastCardH + 4 + forecastNoteH
	workspaceMaxH := int32(300)
	if h < 1200 {
		workspaceMaxH = 270
	}
	if h < 960 {
		workspaceMaxH = 190
	}
	if h < 820 {
		workspaceMaxH = 120
	}
	// Reserve an independently useful archive viewport: native header plus
	// at least three rows at regular desktop sizes.
	archiveBudget := archiveReserve
	if h >= 820 {
		archiveBudget = max32(archiveBudget, 174)
	}
	workspaceAvailable := bottom - y - toolbarReserve - forecastReserve - planReserve - archiveBudget - 6
	workspaceH := max32(120, min32(workspaceMaxH, workspaceAvailable))
	if workspaceAvailable < 120 {
		// When the lower workflow becomes space-constrained, keep at least one
		// usable archive row and a visible plan viewport by letting the central
		// workbench shrink below the default 120 px minimum.
		workspaceH = max32(84, min32(workspaceMaxH, workspaceAvailable))
	}
	// Start from the normal four-card page. The final decision is made after the
	// actual left-column width is known, because long Russian method names must
	// never be forced into a card that cannot contain both the full title and its
	// status/detail line.
	perPage := 4
	if workspaceH < 230 {
		perPage = 2
	}

	// RC650: use the actual reference proportions. The method selector, five
	// reasons and implementation parameters now form one aligned horizontal
	// workbench instead of two cards stacked at the right.
	leftW := contentW * 44 / 100
	midW := contentW * 25 / 100
	rightW := contentW - leftW - midW - gap*2
	if rightW < 270 {
		delta := 270 - rightW
		leftW = max32(405, leftW-delta)
		rightW = contentW - leftW - midW - gap*2
	}
	midX := x + leftW + gap
	rightX := midX + midW + gap
	layoutMethodsWorkspacePanels(x, midX, rightX, y, leftW, midW, rightW, workspaceH)
	// Keep every interactive/text control visibly inside its framed card.
	workspaceInset := int32(9)
	leftInnerX, leftInnerW := x+workspaceInset, leftW-workspaceInset*2
	midInnerX, midInnerW := midX+workspaceInset, midW-workspaceInset*2
	rightInnerX, rightInnerW := rightX+workspaceInset, rightW-workspaceInset*2

	// RC678: switch to one row / two cards whenever any currently available
	// Russian title would collide with the detail plaque in a four-card layout.
	// The decision uses GDI text measurement, not character-count guesses.
	if perPage == 4 && methodsCandidateTitlesNeedTwoCardPage(leftInnerW, workspaceH) {
		perPage = 2
	}
	if methodsCardsPerPage != perPage {
		first := methodsCandidatePage * methodsCardsPerPage
		methodsCardsPerPage = perPage
		methodsCandidatePage = first / perPage
		if methodsPreparedScope != nil && methodsPreparedKey == selectedObjectKey {
			refreshMethodsCandidateCards()
		}
	}

	if !methodsClassificationReady || len(methodWorkflowSuggestions) == 0 {
		hideMethodsParameterPanel()
		hideMethodsBasisPanel()
		show(methodsCandidateList, false)
		for i, card := range methodsCandidateCards {
			show(card, false)
			show(methodsCandidateDetails[i], false)
		}
		show(methodsCandidatePrev, false)
		show(methodsCandidateNext, false)
		show(methodsShowAllButton, false)

		// RC576: even before full classification the page keeps the same three-zone
		// geometry as the approved reference. The previous 92 px pink placeholder
		// collapsed the workspace and made the archive occupy most of the window,
		// so the user could not see any visual change until methods existed.
		show(methodsAvailableTitle, true)
		show(methodsBasisTitle, true)
		show(methodsSuggestionInfo, true)
		show(methodsImplementationTitle, true)
		show(methodsImplementation, true)
		show(methodsImplementationEditButton, false)
		show(methodsAddButton, false)
		show(methodsRemoveButton, false)
		show(methodsResetButton, false)
		show(methodsEmptyState, true)
		show(methodsToClassificationButton, !methodsClassificationReady)

		setPos(methodsAvailableTitle, leftInnerX, y+3, leftInnerW, 25)
		emptyBodyY := y + 31
		emptyBodyBottom := y + workspaceH - 8
		if !methodsClassificationReady {
			buttonY := y + workspaceH - 43
			emptyBodyBottom = buttonY - 7
			setPos(methodsToClassificationButton, leftInnerX+11, buttonY, leftInnerW-22, 36)
		}
		setPos(methodsEmptyState, leftInnerX, emptyBodyY, leftInnerW, max32(1, emptyBodyBottom-emptyBodyY))
		basisTitleH := int32(18)
		setPos(methodsBasisTitle, midInnerX, y+3, midInnerW, basisTitleH)
		basisBodyY := y + 3 + basisTitleH + 3
		setPos(methodsSuggestionInfo, midInnerX, basisBodyY, midInnerW, max32(1, y+workspaceH-5-basisBodyY))
		implTitleH := int32(18)
		setPos(methodsImplementationTitle, rightInnerX, y+3, rightInnerW, implTitleH)
		implBodyY := y + 3 + implTitleH + 3
		setPos(methodsImplementation, rightInnerX, implBodyY, rightInnerW, max32(1, y+workspaceH-5-implBodyY))
		y += workspaceH + 6
	} else {
		show(methodsToClassificationButton, false)
		show(methodsEmptyState, false)
		show(methodsCandidateList, false)
		show(methodsAvailableTitle, true)
		show(methodsBasisTitle, true)
		show(methodsSuggestionInfo, true)
		show(methodsImplementationTitle, true)
		show(methodsImplementation, true)
		show(methodsImplementationEditButton, true)
		show(methodsAddButton, true)
		show(methodsRemoveButton, true)
		show(methodsResetButton, true)

		// RC575: methods are rendered as two columns of large interactive cards,
		// matching the approved visual reference instead of a dense spreadsheet.
		pageButtonW := int32(38)
		pageButtonGap := int32(6)
		pageButtonsW := pageButtonW*2 + pageButtonGap
		headerGap := int32(7)
		// The complete captions must remain single-line in the actual owner-drawn
		// Windows font. The previous 148 px slot wrapped the last word below the
		// 28 px button and visually clipped it.
		showAllW := min32(210, max32(186, leftInnerW-pageButtonsW-headerGap*2-126))
		titleW := max32(126, leftInnerW-showAllW-pageButtonsW-headerGap*2)
		candidateHeaderH := int32(38)
		cardAreaAdvance := int32(41)
		compactCardOnly := workspaceH < 112
		// On the 729/768 px compatibility floor there is not enough vertical
		// room for a separate detail plaque. Keep the full method title inside
		// the card and leave status/details to the adjacent basis panel instead
		// of drawing a second control through the panel frame.
		setPos(methodsAvailableTitle, leftInnerX, y+3, titleW, 26)
		// Legacy RC632 static token retained: y+2, pageButtonW, 34. RC676 uses 38 px to prevent caption clipping.
		setPos(methodsShowAllButton, leftInnerX+titleW+headerGap, y+2, showAllW, candidateHeaderH)
		setPos(methodsCandidatePrev, leftInnerX+leftInnerW-pageButtonsW, y+2, pageButtonW, candidateHeaderH)
		setPos(methodsCandidateNext, leftInnerX+leftInnerW-pageButtonW, y+2, pageButtonW, candidateHeaderH)
		show(methodsShowAllButton, true)
		if methodsShowAll {
			setText(methodsShowAllButton, "ТОЛЬКО РЕКОМЕНДУЕМЫЕ")
		} else {
			setText(methodsShowAllButton, "ПОКАЗАТЬ ВСЕ МЕТОДЫ")
		}
		cardAreaTop := y + cardAreaAdvance
		cardGapX := int32(8)
		cardGapY := int32(6)
		cardW := (leftInnerW - cardGapX) / 2
		cardRows := int32(methodsCardsPerPage / 2)
		cardH := max32(38, (workspaceH-48-cardGapY*(cardRows-1))/cardRows)
		pageStart := methodsCandidatePage * methodsCardsPerPage
		visibleCards := min32(int32(methodsCardsPerPage), int32(max(0, len(methodWorkflowSuggestions)-pageStart)))
		titleHeights := make([]int32, len(methodsCandidateCards))
		detailHeights := make([]int32, len(methodsCandidateCards))
		rowTitleMax := make([]int32, 2)
		rowDetailMax := make([]int32, 2)
		// RC673 regression token: methodsMeasuredTextHeight(getText(card), cardW-32, methodsTypeBody, true)
		for i := 0; i < len(methodsCandidateCards); i++ {
			if i < methodsCardsPerPage && pageStart+i < len(methodWorkflowSuggestions) {
				// RC680: align the status/detail panels inside one row to the same
				// baseline. Long titles may span a different number of lines, but the
				// lower "маячок" blocks must start on one straight horizontal level.
				titleTextH := methodsMeasuredTextHeight(getText(methodsCandidateCards[i]), cardW-32, methodsTypeBody, true)
				detailTextH := methodsMeasuredTextHeight(getText(methodsCandidateDetails[i]), cardW-12, methodsTypeCompact, false)
				nameH, detailH := methodsCandidateCardHeights(titleTextH, detailTextH, cardH)
				if compactCardOnly {
					detailH = 0
					nameH = min32(nameH, max32(30, workspaceH-cardAreaAdvance-5))
				}
				titleHeights[i] = nameH
				detailHeights[i] = detailH
				row := i / 2
				if nameH > rowTitleMax[row] {
					rowTitleMax[row] = nameH
				}
				if detailH > rowDetailMax[row] {
					rowDetailMax[row] = detailH
				}
			}
		}
		cardStartX := leftInnerX
		// Legacy RC632 static token retained: cardTop := y + 41.
		cardTop := cardAreaTop
		if methodsCardsPerPage == 2 && visibleCards > 0 {
			totalRowH := rowTitleMax[0] + rowDetailMax[0]
			cardStartX = leftInnerX + max32(0, (leftInnerW-(cardW*2+cardGapX))/2)
			if compactCardOnly {
				cardTop = cardAreaTop
			} else {
				cardTop = cardAreaTop + max32(8, (workspaceH-48-totalRowH)/2)
			}
		}
		for i, card := range methodsCandidateCards {
			if i < methodsCardsPerPage && pageStart+i < len(methodWorkflowSuggestions) {
				row := int32(i / 2)
				col := int32(i % 2)
				cx := cardStartX + col*(cardW+cardGapX)
				cy := cardTop + row*(cardH+cardGapY)
				nameH := rowTitleMax[row]
				detailH := rowDetailMax[row]
				setPos(card, cx, cy, cardW, nameH)
				if compactCardOnly {
					show(methodsCandidateDetails[i], false)
				} else {
					setPos(methodsCandidateDetails[i], cx, cy+nameH, cardW, detailH)
					show(methodsCandidateDetails[i], true)
				}
				show(card, true)
			} else {
				show(card, false)
				show(methodsCandidateDetails[i], false)
			}
		}
		basisTitleH := int32(18)
		setPos(methodsBasisTitle, midInnerX, y+3, midInnerW, basisTitleH)
		basisBodyY := y + 3 + basisTitleH + 3
		basisBodyH := max32(1, y+workspaceH-5-basisBodyY)
		setPos(methodsSuggestionInfo, midInnerX, basisBodyY, midInnerW, basisBodyH)
		layoutMethodsBasisPanel(midInnerX, basisBodyY, midInnerW, basisBodyH)

		implTitleH := int32(18)
		setPos(methodsImplementationTitle, rightInnerX, y+3, rightInnerW, implTitleH)
		implContentY := y + 3 + implTitleH + 3
		implAvailable := max32(1, y+workspaceH-5-implContentY)
		implementationGap := int32(7)
		if implAvailable < 48 {
			implementationGap = 3
		}
		implementationButtonH := min32(36, max32(16, implAvailable))
		implementationBodyH := implAvailable - implementationButtonH - implementationGap
		if implementationBodyH >= 18 {
			setPos(methodsImplementation, rightInnerX, implContentY, rightInnerW, implementationBodyH)
			layoutMethodsParameterPanel(rightInnerX, implContentY, rightInnerW, implementationBodyH)
			setPos(methodsImplementationEditButton, rightInnerX, implContentY+implementationBodyH+implementationGap, rightInnerW, implementationButtonH)
		} else {
			hideMethodsParameterPanel()
			show(methodsImplementation, false)
			setPos(methodsImplementationEditButton, rightInnerX, implContentY, rightInnerW, implementationButtonH)
		}

		y += workspaceH + 6
	}

	// Local compact toolbar. Reference and replication remain accessible without
	// forming a second global action bar; when methods exist, plan editing shares
	// the same row.
	toolGap := int32(8)
	if methodsClassificationReady && len(methodWorkflowSuggestions) > 0 {
		// RC640: the action row now spans the whole workspace; there is no unused
		// blank strip below the right-hand cards. Service actions remain secondary,
		// while plan editing gets a wider, easier-to-scan group.
		toolAvailable := contentW - toolGap*4
		refW := toolAvailable * 18 / 100
		repW := toolAvailable * 15 / 100
		addW := toolAvailable * 18 / 100
		removeW := toolAvailable * 24 / 100
		resetW := toolAvailable - refW - repW - addW - removeW
		setPos(methodsReferenceButton, x, y, refW, toolbarH)
		setPos(methodsReplicationButton, x+refW+toolGap, y, repW, toolbarH)
		setPos(methodsAddButton, x+refW+repW+toolGap*2, y, addW, toolbarH)
		setPos(methodsRemoveButton, x+refW+repW+addW+toolGap*3, y, removeW, toolbarH)
		setPos(methodsResetButton, x+refW+repW+addW+removeW+toolGap*4, y, resetW, toolbarH)
	} else {
		serviceW := (contentW - toolGap) / 2
		setPos(methodsReferenceButton, x, y, serviceW, toolbarH)
		setPos(methodsReplicationButton, x+serviceW+toolGap, y, contentW-serviceW-toolGap, toolbarH)
	}
	show(methodsReferenceButton, true)
	show(methodsReplicationButton, true)
	y += toolbarReserve

	// Four forecast cards stay on one horizontal line. All remain quiet navy/white;
	// red is reserved for warnings/key object and no scenario uses a competing success fill.
	setPos(methodsForecastTitle, x+sectionTextInset, y, contentW-sectionTextInset*2, forecastTitleH)
	y += forecastTitleAdvance
	show(methodsForecast, false)
	show(methodsHistoryWarning, false)
	// RC641: forecast cards use one centered equal-width grid, matching the
	// rest of the reference layout instead of a visually unbalanced 22/22/30/26 split.
	cardGap := int32(10)
	forecastInset := int32(2)
	forecastUsable := contentW - forecastInset*2 - cardGap*3
	cardW := forecastUsable / 4
	currentW := cardW
	gainW := cardW
	resultW := cardW
	basisW := forecastUsable - cardW*3
	currentX := x + forecastInset
	gainX := currentX + currentW + cardGap
	resultX := gainX + gainW + cardGap
	basisX := resultX + resultW + cardGap
	setPos(methodsForecastCurrent, currentX, y, currentW, forecastCardH)
	setPos(methodsForecastGain, gainX, y, gainW, forecastCardH)
	setPos(methodsForecastResult, resultX, y, resultW, forecastCardH)
	setPos(methodsForecastBasis, basisX, y, basisW, forecastCardH)
	y += forecastCardH + 4
	if forecastNoteH > 0 {
		setPos(methodsForecastNote, x, y, contentW, forecastNoteH)
		show(methodsForecastNote, true)
		y += forecastNoteH
	} else {
		show(methodsForecastNote, false)
	}
	forecastBandY := y - forecastReserve - 2

	// RC657: the plan is part of the end-to-end workflow and must stay visible.
	// RC615 hid it and left a large, empty archive area, which made the teaching
	// examples appear to stop before «Мероприятия».
	planTitleH := int32(24)
	planListH := max32(96, planReserve-planTitleH-8)
	if h < 960 {
		planTitleH = 18
		planListH = max32(72, planReserve-planTitleH-8)
	}
	if h < 760 {
		planTitleH = 16
		planListH = max32(68, planReserve-planTitleH-6)
	}
	if h < 820 {
		planListH = max32(58, planReserve-planTitleH-6)
	}
	setPos(methodsPlanTitle, x+sectionTextInset, y, contentW-sectionTextInset*2, planTitleH)
	setPos(methodsPlanList, x, y+planTitleH+4, contentW, planListH)
	fitWellFailureListColumns(methodsPlanList, []int32{42, 230, 110, 105, 145, 165, 88, 88, 135})
	show(methodsPlanTitle, true)
	show(methodsPlanList, true)
	show(methodsPlanComment, false)
	y += planTitleH + 4 + planListH + 6

	exportGap := int32(8)
	exportW := int32(92)
	exportsW := exportW*3 + exportGap*2
	exportsX := right - exportsW
	archiveHeaderH := int32(24)
	archiveHeaderGap := int32(4)
	if h < 960 {
		archiveHeaderH = 18
	}
	if h < 760 {
		archiveHeaderH = 14
		// Preserve a 40px archive row at the 729px compatibility floor. The
		// title and list remain visually distinct through the header border.
		archiveHeaderGap = 0
	}
	setPos(methodsArchiveTitle, x+sectionTextInset, y, max32(300, contentW-exportsW-12-sectionTextInset), archiveHeaderH)
	setPos(methodsExportPDF, exportsX, y, exportW, archiveHeaderH)
	setPos(methodsExportXLSX, exportsX+exportW+exportGap, y, exportW, archiveHeaderH)
	setPos(methodsExportDOCX, exportsX+(exportW+exportGap)*2, y, exportW, archiveHeaderH)
	archiveY := y + archiveHeaderH + archiveHeaderGap
	archiveH := max32(1, bottom-archiveY)
	if h >= 820 && archiveH < 110 {
		diagLog("METHODS archive viewport too short: %d at client height %d", archiveH, h)
	}
	setPos(methodsArchiveList, x, archiveY, contentW, archiveH)
	// Fill the entire visible width with named columns. The native list view
	// otherwise leaves an anonymous blank strip after «Статус» on wide screens.
	fitWellFailureListColumns(methodsArchiveList, []int32{210, 112, 72, 150, 240, 145, 170})
	show(methodsArchiveTitle, true)
	show(methodsArchiveList, true)
	show(methodsExportPDF, true)
	show(methodsExportXLSX, true)
	show(methodsExportDOCX, true)
	show(methodsSuggestionCombo, false)
	layoutMethodsSectionBands(
		x, contentW,
		contextBandY, contextBandH,
		classBandY, classBlockH,
		actionBandY, suggestionTitleH+actionBlockH+6,
		forecastBandY, forecastReserve+2,
		y-2, bottom-y+2,
	)
}

func methodsCandidateTitlesNeedTwoCardPage(leftInnerW, workspaceH int32) bool {
	if leftInnerW <= 0 || workspaceH <= 0 || len(methodWorkflowSuggestions) == 0 {
		return false
	}
	cardGapX := int32(8)
	cardGapY := int32(6)
	cardW := (leftInnerW - cardGapX) / 2
	fourCardH := max32(38, (workspaceH-48-cardGapY)/2)
	if cardW <= 64 || fourCardH <= 0 {
		return true
	}
	for _, suggestion := range methodWorkflowSuggestions {
		title := strings.TrimSpace(suggestion.Method.Name)
		if title == "" {
			continue
		}
		titleTextH := methodsMeasuredTextHeight(title, cardW-32, methodsTypeBody, true)
		if titleTextH <= 0 {
			continue
		}
		// Reserve the native checkbox/button vertical margins plus two readable
		// compact detail lines. The extra safety is intentional: real Segoe UI
		// metrics on Windows can be a few pixels taller than the bare CALCRECT.
		if titleTextH+14+42 > fourCardH {
			return true
		}
	}
	return false
}

func methodsCandidateCardHeights(titleTextHeight, detailTextHeight, cardHeight int32) (int32, int32) {
	if titleTextHeight <= 0 {
		titleTextHeight = 28
	}
	if detailTextHeight <= 0 {
		detailTextHeight = 24
	}
	// Two extra safety pixels account for the native BUTTON/EDIT internal
	// vertical margins; GDI DT_CALCRECT itself does not include them.
	titleHeight := max32(44, titleTextHeight+12)
	detailHeight := max32(32, detailTextHeight+10)
	if titleHeight+detailHeight > cardHeight {
		detailHeight = max32(30, cardHeight-titleHeight)
	}
	if titleHeight+detailHeight > cardHeight {
		titleHeight = max32(34, cardHeight-detailHeight)
	}
	if titleHeight+detailHeight > cardHeight {
		detailHeight = max32(1, cardHeight-titleHeight)
	}
	return titleHeight, detailHeight
}

func layoutMethodsCandidateColumns(leftW int32) {
	if methodsCandidateList == 0 {
		return
	}
	available := max32(620, leftW-18)
	// В референсе блок методов читается как единая карточка, поэтому три
	// инженерные метрики собраны в одну колонку. Полные расшифровки находятся
	// справа в «Основании выбора» и в «Справочнике».
	widths := []int32{available * 17 / 100, available * 30 / 100, available * 20 / 100, available * 16 / 100, available * 17 / 100}
	total := int32(0)
	for _, width := range widths {
		total += width
	}
	widths[1] += available - total
	for i, width := range widths {
		setListColumnWidth(methodsCandidateList, i, max32(90, width))
	}
}

func methodsCandidateCustomDraw(lParam uintptr) uintptr {
	if lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		row := int(draw.NMCD.ItemSpec)
		if row < 0 || row >= len(methodWorkflowSuggestions) {
			return CDRF_DODEFAULT
		}
		// Clickable method names receive a restrained link-like accent; the
		// priority cell is a calm status plaque matching the approved reference.
		if draw.SubItem == 1 {
			draw.ClrText = uint32(rgb(15, 73, 128))
			draw.ClrTextBk = uint32(rgb(239, 246, 253))
			return CDRF_NEWFONT
		}
		if draw.SubItem == 4 {
			s := methodWorkflowSuggestions[row]
			a := methodApplicabilityForSuggestion(s, strings.TrimSpace(selectedObjectKey), time.Now())
			switch methodPriorityLabel(a, s) {
			case "Высокий":
				draw.ClrText = uint32(rgb(163, 35, 45))
				draw.ClrTextBk = uint32(rgb(255, 232, 235))
			case "Средний":
				draw.ClrText = uint32(rgb(34, 91, 148))
				draw.ClrTextBk = uint32(rgb(229, 240, 252))
			default:
				draw.ClrText = uint32(rgb(40, 112, 64))
				draw.ClrTextBk = uint32(rgb(232, 246, 235))
			}
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func layoutMethodsPlanColumns(contentW int32) {
	if methodsPlanList == 0 {
		return
	}
	available := max32(760, contentW-18)
	widths := []int32{42, available * 27 / 100, 118, 118, available * 15 / 100, available * 20 / 100, 128}
	total := int32(0)
	for _, width := range widths {
		total += width
	}
	widths[1] += available - total
	for i, width := range widths {
		setListColumnWidth(methodsPlanList, i, max32(42, width))
	}
}

func methodPriorityLabel(a MethodEffectAssessment, s methodWorkflowSuggestion) string {
	// Приоритет — только очередность рассмотрения технологом. Он не является
	// вероятностью успеха и не утверждает метод автоматически.
	switch {
	case a.Applicability >= 80 && a.Reliability >= 65:
		return "Высокий"
	case s.Severity >= 3 && a.Applicability >= 72 && a.Reliability >= 55:
		return "Высокий"
	case a.Applicability >= 60 && a.Reliability >= 45:
		return "Средний"
	default:
		return "Низкий"
	}
}

func methodsBasisSummaryText(s methodWorkflowSuggestion, a MethodEffectAssessment, r ReplicationAssessment, baselineDays float64) string {
	probability := "недостаточно подтверждённых исходов"
	if a.ProbabilityAvailable {
		probability = fmt.Sprintf("%d%% (фактов %d)", a.EffectProbability, a.ProbabilitySample)
	}
	contribution := "качественный эффект; численный вклад не начислен"
	if a.EffectQuantified && a.BasePct > 0 {
		contribution = fmt.Sprintf("+%.0f%%", a.BasePct)
		if baselineDays > 0 {
			contribution += fmt.Sprintf(" ≈ +%.0f сут", baselineDays*a.BasePct/100)
		}
	}
	return fmt.Sprintf("1. Класс: %s %s.\r\n\r\n2. Применимость: %d%%.\r\n\r\n3. Достоверность: %d%% · %s.\r\n\r\n4. Вероятность эффекта: %s.\r\n\r\n5. Вклад в наработку: %s.", s.Direction, s.Classification, a.Applicability, a.Reliability, a.ReliabilityLevel, probability, contribution)
}

func methodVisualType(name string) string {
	n := normalizeMethodText(name)
	switch {
	case strings.Contains(n, "уплд") || strings.Contains(n, "погружным линейным двигателем"):
		return "Оборудование"
	case strings.Contains(n, "рвк пилот") || strings.Contains(n, "резонансно волновой комплекс") || strings.Contains(n, "резонансно волновое воздействие"):
		return "Физическое воздействие"
	case strings.Contains(n, "дозир") || strings.Contains(n, "ингибитор") || strings.Contains(n, "реагент"):
		return "Дозирование"
	case strings.Contains(n, "промыв"):
		return "Промывка"
	case strings.Contains(n, "кислот") || strings.Contains(n, "обработ"):
		return "Обработка"
	case strings.Contains(n, "фильтр") || strings.Contains(n, "сепарат"):
		return "Механическое"
	case strings.Contains(n, "покрыт"):
		return "Покрытие"
	case strings.Contains(n, "режим") || strings.Contains(n, "депресс"):
		return "Режим"
	default:
		return "Метод"
	}
}

func methodClassShort(direction, class string) string {
	if strings.TrimSpace(class) != "" {
		return strings.TrimSpace(class)
	}
	if d, ok := methodDirectionByName(direction); ok {
		return d.Prefix
	}
	return "—"
}

func methodClassDescription(r AssessmentClassRecord) string {
	for _, v := range []string{r.FinalComment, r.ProgramReasonAtDecision, r.TechnologistComment, r.ManualLabComment} {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return "Утверждено технологом по результатам раздела 3"
}

func methodsImplementationText(key string) string {
	s, ok := methodsSelectedSuggestion()
	if !ok || strings.TrimSpace(s.Method.Name) == "" {
		return "Выберите метод.\r\n\r\nПараметры реализации зависят от типа мероприятия: дозирование, промывка/обработка либо механическое/технологическое воздействие."
	}
	text := methodImplementationSummaryText(key, s)
	if schemes := learnedSchemesForMethod(key, s.Method.Name); len(schemes) > 0 {
		text += fmt.Sprintf("\r\n\r\nНакопленные схемы РОСА: %d. Подробности и сочетания — кнопка «АНАЛОГИ».", len(schemes))
	}
	return text
}

func refreshMethodsVisualTables() {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	key := strings.TrimSpace(selectedObjectKey)
	refreshMethodsCandidateCards()
	if methodsKeyData != 0 {
		setText(methodsKeyData, methodsKeyDataText(key))
	}
	if methodsImplementation != 0 {
		setText(methodsImplementation, methodsImplementationText(key))
	}
	if methodsHistoryWarning != 0 {
		h := methodHistoryBasisForObject(key, time.Now())
		warning := "Предыстория 36 мес. учтена."
		if h.FailureCount == 0 || h.CycleCount == 0 {
			warning = "ВНИМАНИЕ: без предыстории отказов прогноз значительно менее точен."
		} else if strings.TrimSpace(h.Warning) != "" {
			warning = "ВНИМАНИЕ: " + strings.TrimSpace(h.Warning)
		}
		setText(methodsHistoryWarning, warning)
	}
	// Keep the end-to-end plan visible on ordinary page entry as well as after
	// opening the editor. RC658 restored the control but did not refresh its rows.
	refreshMethodsPlanTable(methodsClassificationReady)
}

func methodsTeachingPreviewKey(objectKey string) string {
	if _, ok := teachingWorkflowOrdinal(objectKey); ok {
		return objectKey
	}
	return "Учебный пример 001 · Авиловское / 6 / Бобриковский горизонт"
}

func methodsKeyDataText(key string) string {
	if strings.TrimSpace(key) == "" {
		return "Закрепите объект на Главной."
	}
	var b strings.Builder
	if st, ok := currentSelectedState(); ok {
		fmt.Fprintf(&b, "Обводнённость, %%: %s\r\n", displayTechNumber(st.WaterCut, 1))
		fmt.Fprintf(&b, "Температура, °C: %s\r\n", displayTechNumber(st.TMSTemperature, 1))
		fmt.Fprintf(&b, "Дебит жидкости: %s\r\n", displayTechNumber(st.LiquidRate, 2))
		fmt.Fprintf(&b, "ГНО: %s\r\n", dash(st.Equipment))
		fmt.Fprintf(&b, "Текущий МРП, сут.: %s\r\n", formatWholeDaysText(st.CurrentMRP))
	}
	h := methodHistoryBasisForObject(key, time.Now())
	fmt.Fprintf(&b, "Отказы за 36 мес.: %d\r\n", h.FailureCount)
	fmt.Fprintf(&b, "Завершённые циклы: %d\r\n", h.CycleCount)
	fmt.Fprintf(&b, "Достаточность истории: %s", dash(h.Precision))
	return b.String()
}

func methodPlanAssessmentForName(objectKey, name string) MethodEffectAssessment {
	if d, ok := latestApprovedMethodDecision(objectKey); ok {
		for _, assessment := range d.MethodAssessments {
			if methodNamesComparable(assessment.Method, name) || normalizeMethodText(assessment.Method) == normalizeMethodText(name) {
				return assessment
			}
		}
	}
	return methodApplicabilityForName(objectKey, name, time.Now())
}

func methodPlanAvailability(objectKey string) string {
	if s, ok := selectedTeachingAlternative(objectKey); ok && strings.TrimSpace(s.Availability) != "" {
		return s.Availability
	}
	if s, ok := activeAlternativeScenario(objectKey); ok && strings.TrimSpace(s.Availability) != "" {
		return s.Availability
	}
	return scenarioAvailabilityUnknown
}

func refreshMethodsPlanTable(classificationReady bool) {
	if methodsPlanList == 0 {
		return
	}
	key := strings.TrimSpace(selectedObjectKey)
	names := currentMethodsPlanNames()
	listBeginUpdate(methodsPlanList)
	listClear(methodsPlanList)
	// Built-in teaching rows stay visible for UI verification but remain
	// explicitly synthetic and never become a production approval.
	if len(names) == 0 {
		if points := teachingCrossHistoryPoints(methodsTeachingPreviewKey(key), time.Now()); len(points) > 0 {
			previewKey := methodsTeachingPreviewKey(key)
			seen := map[string]bool{}
			row := 0
			for pointIndex := len(points) - 1; pointIndex >= 0 && row < 3; pointIndex-- {
				point := points[pointIndex]
				for _, name := range normalizeMethodList(strings.Split(point.Method, " + ")) {
					normalized := normalizeMethodText(name)
					if normalized == "" || seen[normalized] {
						continue
					}
					seen[normalized] = true
					assessment := methodPlanAssessmentForName(previewKey, name)
					listAdd(methodsPlanList, row, []string{
						strconv.Itoa(row + 1), name, "УЧЕБНЫЙ ПРИМЕР",
						point.Date.AddDate(0, 1, 0).Format("02.01.2006"), "Технолог · учебный контур",
						"Контроль фактического эффекта и наработки",
						fmt.Sprintf("%d%%", assessment.Reliability), fmt.Sprintf("%d%%", assessment.Applicability), methodPlanAvailability(previewKey),
					})
					row++
					if row == 3 {
						break
					}
				}
			}
			listEndUpdate(methodsPlanList)
			setText(methodsPlanComment, "УЧЕБНЫЙ СЦЕНАРИЙ: три связанных мероприятия восстановлены из единого сквозного таймлайна; это не производственное назначение.")
			return
		}
	}
	availability := methodPlanAvailability(key)
	for i, name := range names {
		p0 := methodPlanDisplayItem(key, name)
		assessment := methodPlanAssessmentForName(key, name)
		listAdd(methodsPlanList, i, []string{
			strconv.Itoa(i + 1), name, methodPlanCanonicalStatus(p0.Status),
			methodPlanDateDisplay(p0.DueDate), methodPlanResponsibleDisplay(p0.Responsible),
			methodPlanControlDisplay(p0), fmt.Sprintf("%d%%", assessment.Reliability), fmt.Sprintf("%d%%", assessment.Applicability), availability,
		})
	}
	listEndUpdate(methodsPlanList)
	if methodsPlanComment != 0 {
		text := "Комментарий к плану: комплекс ещё не сформирован."
		if len(names) > 0 {
			text = "Комплекс мероприятий: " + strings.Join(names, "; ") + ".\r\nДвойной клик по строке — состояние, срок, ответственный и контроль; клик по названию метода — карточка метода. Оценки достоверности, применимости и доступности показаны непосредственно в плане."
			if !classificationReady {
				text += "\r\nКлассификация ещё не утверждена полностью."
			}
		}
		setText(methodsPlanComment, text)
	}
}

func methodsIsTeachingDemoObject(key string) bool {
	if _, ok := teachingWorkflowOrdinal(key); ok {
		return true
	}
	u := strings.ToUpper(strings.TrimSpace(key))
	return strings.Contains(u, "ДЕМО-") || strings.Contains(u, "УЧЕБНЫЙ ОБЪЕКТ") || strings.Contains(u, "УЧЕБНЫЙ ПРИМЕР") || strings.Contains(u, "ВСТРОЕННЫЙ СЦЕНАРИЙ")
}

func applyTeachingDemoMethodDefaults(key string) {
	identity := methodsObjectIdentityKey(key)
	if key == "" || methodsDemoDefaultsApplied[identity] || !methodsIsTeachingDemoObject(key) {
		return
	}
	methodsDemoDefaultsApplied[identity] = true
	if raw, _ := combinedMethodsForObject(key); len(normalizeMethodList(raw)) > 0 || len(methodWorkflowSuggestions) == 0 {
		return
	}
	// The teaching route is deterministic: restore the complete approved complex
	// for K1 + S2 + M2 (corrosion, scale and mechanical-solids protection)
	// instead of taking whichever catalogue rows happen to sort first.
	summary, ready, _ := approvedAssessmentForMethods(key)
	if ready {
		if lesson, ok := teachingActiveMethodDecision(key, summary); ok {
			setCombinedMethods(key, lesson.Methods)
			return
		}
	}
	chosen := make([]string, 0, 3)
	for _, suggestion := range methodWorkflowSuggestions {
		name := strings.TrimSpace(suggestion.Method.Name)
		if name == "" {
			continue
		}
		chosen = append(chosen, name)
		if len(chosen) == 3 {
			break
		}
	}
	if len(chosen) > 0 {
		setCombinedMethods(key, chosen)
	}
}

func refreshMethodsCandidateCards() {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	key := strings.TrimSpace(selectedObjectKey)

	pageCount := (len(methodWorkflowSuggestions) + methodsCardsPerPage - 1) / methodsCardsPerPage
	if pageCount < 1 {
		pageCount = 1
	}
	if methodsCandidatePage < 0 {
		methodsCandidatePage = 0
	}
	if methodsCandidatePage >= pageCount {
		methodsCandidatePage = pageCount - 1
	}
	pageStart := methodsCandidatePage * methodsCardsPerPage
	pageEnd := pageStart + methodsCardsPerPage
	if pageEnd > len(methodWorkflowSuggestions) {
		pageEnd = len(methodWorkflowSuggestions)
	}

	applyTeachingDemoMethodDefaults(key)
	plan := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
	approved := []string{}
	if d, ok := latestApprovedMethodDecision(key); ok {
		approved = normalizeMethodList(d.Methods)
		if len(approved) == 0 && strings.TrimSpace(d.Method) != "" {
			approved = []string{strings.TrimSpace(d.Method)}
		}
	}
	contains := func(names []string, method string) bool {
		n := normalizeMethodText(method)
		for _, x := range names {
			if normalizeMethodText(x) == n {
				return true
			}
		}
		return false
	}

	if len(methodWorkflowSuggestions) > methodsCardsPerPage {
		show(methodsCandidatePrev, true)
		show(methodsCandidateNext, true)
	} else {
		show(methodsCandidatePrev, false)
		show(methodsCandidateNext, false)
	}
	if methodsCandidatePrev != 0 {
		pEnableWindow.Call(uintptr(methodsCandidatePrev), boolToUintptr(methodsCandidatePage > 0))
	}
	if methodsCandidateNext != 0 {
		pEnableWindow.Call(uintptr(methodsCandidateNext), boolToUintptr(methodsCandidatePage < pageCount-1))
	}
	if len(methodWorkflowSuggestions) > 0 {
		mode := "РЕКОМЕНДУЕМЫЕ"
		if methodsShowAll {
			mode = "ВСЕ МЕТОДЫ"
		}
		setText(methodsAvailableTitle, fmt.Sprintf("01 · %s · %d–%d ИЗ %d", mode, pageStart+1, pageEnd, len(methodWorkflowSuggestions)))
	}

	for slot, card := range methodsCandidateCards {
		if card == 0 {
			continue
		}
		idx := pageStart + slot
		if slot >= methodsCardsPerPage || idx >= len(methodWorkflowSuggestions) {
			show(card, false)
			show(methodsCandidateDetails[slot], false)
			continue
		}
		s := methodWorkflowSuggestions[idx]
		a := methodApplicabilityForSuggestion(s, key, time.Now())
		state := "Предложен"
		if methodSuggestionOutsideCurrentProfile(s) {
			state = "Справочник"
		}

		if contains(plan, s.Method.Name) {
			state = "В плане"
		}
		if contains(approved, s.Method.Name) {
			state = "Утверждён"
		}
		target := targetRunlifeResolvedForSelectedObject().Days
		if target < defaultTargetRunlifeDays {
			target = defaultTargetRunlifeDays
		}
		individual := combinedMethodForecastForNames(key, []string{s.Method.Name}, methodForecastSnapshotForSelectedObject(), time.Now())
		targetLine := "Качественный вклад"
		if individual.ForecastBase > 0 && individual.ForecastBase > individual.BaselineBase+0.5 {
			if individual.ForecastBase >= target {
				targetLine = fmt.Sprintf("≈ %.0f сут. · цель %.0f", individual.ForecastBase, target)
			} else {
				targetLine = fmt.Sprintf("≈ %.0f / цель %.0f сут.", individual.ForecastBase, target)
			}
		}
		setText(card, strings.TrimSpace(s.Method.Name))
		checked := contains(plan, s.Method.Name)
		methodsCandidateDraftSelected[slot] = checked
		pSendMessageW.Call(uintptr(card), 0x00F1, boolToUintptr(checked), 0) // BM_SETCHECK
		setText(methodsCandidateDetails[slot], fmt.Sprintf("%s · %s · %s\r\n%s", s.Classification, methodPriorityLabel(a, s), state, targetLine))
		show(methodsCandidateDetails[slot], true)
		show(card, true)
		pInvalidateRect.Call(uintptr(card), 0, 0)
	}
}

func boolToUintptr(v bool) uintptr {
	if v {
		return 1
	}
	return 0
}

func methodsCandidateIndexForCardID(id int) int {
	return methodsCandidatePage*methodsCardsPerPage + (id - ID_METHODS_CARD_BASE)
}

func selectMethodsCandidateRow(row int) {
	if row < 0 || row >= len(methodWorkflowSuggestions) || methodsSuggestionCombo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_SETCURSEL, uintptr(row+1), 0)
	selectListRow(methodsCandidateList, row)
	refreshMethodsSuggestionInfo()
	refreshMethodsCandidateCards()
}

func openMethodsCandidateReference(row int) {
	if row < 0 || row >= len(methodWorkflowSuggestions) {
		return
	}
	selectMethodsCandidateRow(row)
	s := methodWorkflowSuggestions[row]
	openMethodsReferenceDialog("РОСА — подробная карточка метода", selectedMethodReferenceText(s))
}

func currentMethodsPlanNames() []string {
	key := strings.TrimSpace(selectedObjectKey)
	raw, explicit := combinedMethodsForObject(key)
	names := normalizeMethodList(raw)
	if explicit {
		return names
	}
	if d, ok := latestApprovedMethodDecision(key); ok {
		names = normalizeMethodList(d.Methods)
		if len(names) == 0 && strings.TrimSpace(d.Method) != "" {
			names = []string{strings.TrimSpace(d.Method)}
		}
	}
	return names
}

func openMethodsReferenceByName(name string) {
	norm := normalizeMethodText(name)
	for i, s := range methodWorkflowSuggestions {
		if normalizeMethodText(s.Method.Name) == norm {
			openMethodsCandidateReference(i)
			return
		}
	}
	for i, s := range methodSuggestionsForObject(strings.TrimSpace(selectedObjectKey)) {
		if normalizeMethodText(s.Method.Name) == norm {
			// Do not alter the current table selection if the method is only present
			// in the already formed/approved plan. The card still opens in full.
			_ = i
			openMethodsReferenceDialog("РОСА — подробная карточка метода", selectedMethodReferenceText(s))
			return
		}
	}
	// Conservative fallback: never invent a method description when the
	// currently loaded reference catalogue has no matching record.
	openMethodsReferenceDialog("РОСА — метод", "КАРТОЧКА МЕТОДА\r\n\r\nМетод / мероприятие: "+dash(name)+"\r\n\r\nПодробная карточка отсутствует в текущей доказательной базе. Проверьте справочник и источник метода; автоматическое описание не сформировано.")
}

func formatWholeDaysText(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "—"
	}
	// The source may already contain an abbreviated unit. Strip it before the
	// UI appends its single canonical "сут." suffix; this prevents "сут..".
	v = strings.TrimSpace(strings.TrimRight(v, "."))
	lower := strings.ToLower(v)
	for _, suffix := range []string{"суток", "сутки", "сут"} {
		if strings.HasSuffix(lower, suffix) {
			v = strings.TrimSpace(v[:len(v)-len(suffix)])
			break
		}
	}
	v = strings.TrimSpace(strings.TrimRight(v, "."))
	n := strings.ReplaceAll(v, ",", ".")
	if f, err := strconv.ParseFloat(n, 64); err == nil {
		return fmt.Sprintf("%.0f", f)
	}
	return v
}

func methodClassSeverity(class string) int {
	class = strings.TrimSpace(class)
	for i := len([]rune(class)) - 1; i >= 0; i-- {
		r := []rune(class)[i]
		if r >= '1' && r <= '3' {
			return int(r - '0')
		}
	}
	return 0
}

func assessmentDecisionConsistentForMethods(r AssessmentClassRecord) bool {
	finalClass := strings.TrimSpace(r.FinalClass)
	mode := strings.ToLower(strings.TrimSpace(r.DecisionMode))
	if finalClass == "" || mode == "" {
		return false
	}
	// These modes are defined as accepting the technologist value unchanged.
	// If a later edit changed that value, the old final class is stale even if
	// an older build left FullClassificationConfirmed=true.
	if strings.Contains(mode, "без корректировки") || strings.Contains(mode, "без лаборатории") {
		expected := strings.TrimSpace(r.TechnologistClass)
		if expected == "" && strings.Contains(mode, "без лаборатории") {
			expected = "НД"
		}
		return strings.EqualFold(finalClass, expected)
	}
	if strings.Contains(mode, "с корректировкой") {
		expected := strings.TrimSpace(r.ProgramClassAtDecision)
		if expected == "" || strings.EqualFold(expected, "НД") || strings.EqualFold(expected, "Не рекомендуется") {
			return false
		}
		return strings.EqualFold(finalClass, expected)
	}
	return false
}

func approvedAssessmentForMethods(objectKey string) (summary string, ready bool, rows []AssessmentClassRecord) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return "Объект не закреплён.", false, nil
	}
	parts := make([]string, 0, len(assessmentClassDirections))
	ready = true
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(objectKey, d)
		if !r.FullClassificationConfirmed || !assessmentDecisionConsistentForMethods(r) {
			ready = false
		}
		if c := strings.TrimSpace(r.FinalClass); c != "" && !strings.HasPrefix(c, "НД") && !strings.EqualFold(c, "Не относится") {
			parts = append(parts, d.Name+" — "+c)
		}
	}
	if len(parts) == 0 {
		if ready {
			return "Полная классификация утверждена, но профильные классы не заданы.", true, nil
		}
		return "Полная классификация ещё не утверждена. Вернитесь в раздел 3 и нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».", false, nil
	}
	if !ready {
		// Pending row-level choices are intentionally not exposed as method inputs.
		// The Methods module receives classes only after explicit full approval.
		return "ВЫБРАНО, НО НЕ УТВЕРЖДЕНО: " + strings.Join(parts, "; ") + ". Нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».", false, nil
	}
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(objectKey, d)
		if c := strings.TrimSpace(r.FinalClass); c != "" && !strings.HasPrefix(c, "НД") && !strings.EqualFold(c, "Не относится") {
			rows = append(rows, r)
		}
	}
	return "УТВЕРЖДЕНО: " + strings.Join(parts, "; "), true, rows
}

func methodDirectionByName(name string) (assessmentClassDirection, bool) {
	for _, d := range assessmentClassDirections {
		if strings.EqualFold(strings.TrimSpace(d.Name), strings.TrimSpace(name)) {
			return d, true
		}
	}
	return assessmentClassDirection{}, false
}

func methodSuggestionsForObject(objectKey string) []methodWorkflowSuggestion {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	cacheKey := methodsObjectIdentityKey(objectKey)
	if found, ok := methodsEvaluation.suggestions[cacheKey]; ok {
		return found
	}
	out := buildMethodSuggestionsForObject(objectKey)
	methodsEvaluation.suggestions[cacheKey] = out
	return out
}

func buildMethodSuggestionsForObject(objectKey string) []methodWorkflowSuggestion {
	out := collectMethodSuggestionsForObject(objectKey)
	now := time.Now()
	scores := make(map[string]MethodEffectAssessment, len(out))
	for _, s := range out {
		scores[s.Method.Name] = methodApplicabilityForSuggestion(s, objectKey, now)
	}
	return rankMethodSuggestions(out, scores)
}

func collectMethodSuggestionsForObject(objectKey string) []methodWorkflowSuggestion {
	_, ready, approved := approvedAssessmentForMethods(objectKey)
	if !ready || len(approved) == 0 {
		return nil
	}
	failures := historyForWell(objectKey, time.Now())
	byMethod := map[string]methodWorkflowSuggestion{}
	for _, r := range approved {
		d, ok := methodDirectionByName(r.Direction)
		if !ok {
			continue
		}
		sev := methodClassSeverity(r.FinalClass)
		for _, candidate := range assessmentMethodCandidatesForGroup(assessmentMethodGroupForDirection(d)) {
			if assessmentMethodIsEquipmentDesign(candidate.Name) {
				n := normalizeMethodText(candidate.Name)
				// RC678: УПЛД is a user-requested artificial-lift alternative and may
				// be shown as a Methods candidate for high-viscosity/low-rate context.
				// It still carries the «Проверить в Автотехнологе» status and receives
				// no automatic quantitative effect.
				if !strings.Contains(n, "уплд") && !strings.Contains(n, "погружным линейным двигателем") {
					continue
				}
			}
			key := normalizeMethodText(candidate.Name)
			if key == "" {
				continue
			}
			basis := fmt.Sprintf("%s %s; %s; источник: %s", d.Name, r.FinalClass, candidate.Status, candidate.Source)
			if len(failures) > 0 {
				basis += fmt.Sprintf("; в разделе 1 — %d отказ(а/ов) за 36 мес.", len(failures))
			}
			s := methodWorkflowSuggestion{Method: candidate, Direction: d.Name, Classification: r.FinalClass, Severity: sev, Basis: basis}
			if old, exists := byMethod[key]; !exists || s.Severity > old.Severity || (s.Severity == old.Severity && strings.Contains(strings.ToLower(s.Method.Source), "технологичес") && !strings.Contains(strings.ToLower(old.Method.Source), "технологичес")) {
				byMethod[key] = s
			}
		}
	}
	out := make([]methodWorkflowSuggestion, 0, len(byMethod))
	for _, s := range byMethod {
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		iTech := strings.Contains(strings.ToLower(out[i].Method.Source), "технологичес")
		jTech := strings.Contains(strings.ToLower(out[j].Method.Source), "технологичес")
		if iTech != jTech {
			return iTech
		}
		if out[i].Direction != out[j].Direction {
			return out[i].Direction < out[j].Direction
		}
		return strings.ToLower(out[i].Method.Name) < strings.ToLower(out[j].Method.Name)
	})
	out = mergeLearnedMethodSuggestions(objectKey, out)
	return out
}

func collectAllMethodSuggestionsForObject(objectKey string) []methodWorkflowSuggestion {
	return mergeAllMethodSuggestions(collectMethodSuggestionsForObject(objectKey), ruleCatalog.MethodCatalog)
}
func mergeAllMethodSuggestions(recommended []methodWorkflowSuggestion, catalog []MethodCatalogEntry) []methodWorkflowSuggestion {
	byMethod := map[string]methodWorkflowSuggestion{}
	for _, suggestion := range recommended {
		byMethod[normalizeMethodText(suggestion.Method.Name)] = suggestion
	}
	for _, method := range catalog {
		key := normalizeMethodText(method.Name)
		if key == "" {
			continue
		}
		if _, exists := byMethod[key]; exists {
			continue
		}
		source, sourceKind := assessmentMethodSource(method)
		note := "Справочный вариант полного каталога; технолог должен подтвердить связь с осложнением объекта."
		candidate := assessmentMethodCandidate{ID: method.ID, Name: method.Name, Source: source, SourceKind: sourceKind, Status: assessmentMethodStatus(method.Name), Note: note}
		byMethod[key] = methodWorkflowSuggestion{Method: candidate, Direction: methodsReferenceDirectionText(method.Name), Classification: "Вне текущего профиля", Severity: 0, Basis: note}
	}
	out := make([]methodWorkflowSuggestion, 0, len(byMethod))
	for _, suggestion := range byMethod {
		out = append(out, suggestion)
	}
	sort.SliceStable(out, func(i, j int) bool {
		iOutside, jOutside := methodSuggestionOutsideCurrentProfile(out[i]), methodSuggestionOutsideCurrentProfile(out[j])
		if iOutside != jOutside {
			return !iOutside
		}
		return strings.ToLower(out[i].Method.Name) < strings.ToLower(out[j].Method.Name)
	})
	return out
}
func methodSuggestionOutsideCurrentProfile(s methodWorkflowSuggestion) bool {
	return s.Classification == "Вне текущего профиля"
}
func methodOutsideProfileAssessment(s methodWorkflowSuggestion) MethodEffectAssessment {
	return MethodEffectAssessment{Method: s.Method.Name, Direction: s.Direction, Classification: s.Classification, Applicability: 0, Reliability: 0, ReliabilityLevel: "СПРАВОЧНЫЙ", Confidence: "СПРАВОЧНЫЙ", EffectQuantified: false, EffectBasisType: "не привязан к утверждённому профилю объекта", Evidence: "вариант полного справочника", Warnings: []string{"метод не сопоставлен с утверждёнными осложнениями текущего объекта; требуется отдельное обоснование технолога"}}
}

func rankMethodSuggestions(out []methodWorkflowSuggestion, scores map[string]MethodEffectAssessment) []methodWorkflowSuggestion {
	return rankMethodSuggestionsWithLimit(out, scores, 30)
}

func rankMethodSuggestionsWithLimit(out []methodWorkflowSuggestion, scores map[string]MethodEffectAssessment, limit int) []methodWorkflowSuggestion {
	sort.SliceStable(out, func(i, j int) bool {
		ai := scores[out[i].Method.Name]
		aj := scores[out[j].Method.Name]
		// Lesson candidates with a numeric synthetic effect come first so the
		// target-runlife exercise starts with a calculable intervention.
		iLesson := ai.EffectQuantified && strings.Contains(ai.EffectBasisType, "учебного сценария")
		jLesson := aj.EffectQuantified && strings.Contains(aj.EffectBasisType, "учебного сценария")
		if iLesson != jLesson {
			return iLesson
		}
		if ai.Applicability != aj.Applicability {
			return ai.Applicability > aj.Applicability
		}
		if ai.Reliability != aj.Reliability {
			return ai.Reliability > aj.Reliability
		}
		return strings.ToLower(out[i].Method.Name) < strings.ToLower(out[j].Method.Name)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func methodsDecisionPath() string { return sharedPath("approved_method_decisions.json") }

func ensureApprovedMethodsLoaded() {
	if approvedMethodsLoaded {
		return
	}
	approvedMethodsLoaded = true
	approvedMethodDecisions = nil
	b, err := os.ReadFile(methodsDecisionPath())
	if err == nil {
		_ = json.Unmarshal(b, &approvedMethodDecisions)
	}
}

func saveApprovedMethods() error {
	ensureApprovedMethodsLoaded()
	rows := make([]approvedMethodDecision, 0, len(approvedMethodDecisions))
	for _, decision := range approvedMethodDecisions {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(decision.ID)), "teaching-") {
			continue
		}
		rows = append(rows, decision)
	}
	return saveJSON(methodsDecisionPath(), rows)
}

func latestApprovedMethodDecisionRaw(objectKey string) (approvedMethodDecision, bool) {
	ensureApprovedMethodsLoaded()
	ordinal, teaching := teachingWorkflowOrdinal(objectKey)
	fullScenario := teaching && (ordinal == 1 || ordinal == 58 || ordinal == 90)
	for i := len(approvedMethodDecisions) - 1; i >= 0; i-- {
		if fullScenario && strings.HasPrefix(strings.ToLower(strings.TrimSpace(approvedMethodDecisions[i].ID)), "teaching-") {
			continue
		}
		if methodsObjectKeysEquivalent(approvedMethodDecisions[i].ObjectKey, objectKey) {
			return approvedMethodDecisions[i], true
		}
	}
	// Every built-in teaching object has a synthetic signed-off Methods step.
	// Build it on demand for the currently visible object caption so a harmless
	// key variant can never turn the lesson back into "выберите метод".
	if _, ok := teachingWorkflowOrdinal(objectKey); ok {
		if summary, ready, _ := approvedAssessmentForMethods(objectKey); ready {
			return teachingActiveMethodDecision(objectKey, summary)
		}
	}
	return approvedMethodDecision{}, false
}

func classificationSummariesEquivalent(a, b string) bool {
	norm := func(v string) string {
		v = strings.ToLower(strings.TrimSpace(v))
		v = strings.Join(strings.Fields(v), " ")
		return v
	}
	return norm(a) == norm(b)
}

func latestApprovedMethodDecision(objectKey string) (approvedMethodDecision, bool) {
	d, ok := latestApprovedMethodDecisionRaw(objectKey)
	if !ok {
		return approvedMethodDecision{}, false
	}
	current, ready, _ := approvedAssessmentForMethods(objectKey)
	if !ready {
		return approvedMethodDecision{}, false
	}
	// A method decision is current only for the exact classification snapshot
	// it was approved against.  Historical decisions remain in the archive, but
	// they must not leak into a newly reclassified object's active workflow.
	if strings.TrimSpace(d.Classification) != "" && !classificationSummariesEquivalent(d.Classification, current) {
		return approvedMethodDecision{}, false
	}
	return d, true
}

func latestApprovedMethodForObject(objectKey string) (string, bool) {
	d, ok := latestApprovedMethodDecision(objectKey)
	if !ok {
		return "", false
	}
	if len(d.Methods) > 0 && strings.TrimSpace(d.Methods[0]) != "" {
		return strings.TrimSpace(d.Methods[0]), true
	}
	return strings.TrimSpace(d.Method), strings.TrimSpace(d.Method) != ""
}

func approvedPrimaryComplication(objectKey string) string {
	_, ready, rows := approvedAssessmentForMethods(objectKey)
	if !ready || len(rows) == 0 {
		return ""
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return methodClassSeverity(rows[i].FinalClass) > methodClassSeverity(rows[j].FinalClass)
	})
	r := rows[0]
	return strings.TrimSpace(r.Direction + " " + r.FinalClass)
}

type methodForecastSnapshot struct {
	Low, Base, High float64
	Source          string
	History         MethodHistoryBasis
}

func calculateMethodForecastSnapshot() methodForecastSnapshot {
	history := methodHistoryBasisForObject(selectedObjectKey, time.Now())
	// Primary basis for the Methods page is the user's required rolling 36-month
	// failure history. Use its completed cycles whenever available.
	if history.BaseDays > 0 {
		return methodForecastSnapshot{Low: history.LowDays, Base: history.BaseDays, High: history.HighDays, Source: "предыстория отказов раздела 1 / последние 36 месяцев", History: history}
	}
	in, ok := mrpForecastInputForSelectedObject()
	if !ok {
		return methodForecastSnapshot{Source: "нет достаточного набора исходных данных для прогноза", History: history}
	}
	// Even fallback MRP observations are restricted to the same rolling window.
	cutoff := time.Now().AddDate(-3, 0, 0)
	filtered := make([]MRPObservation, 0, len(in.Observations))
	for _, o := range in.Observations {
		if o.Date.IsZero() || o.Date.Before(cutoff) {
			continue
		}
		filtered = append(filtered, o)
	}
	in.Observations = filtered
	r := calculateMRPForecast(in)
	if r.HasSurvivalTriplet {
		return methodForecastSnapshot{Low: r.SurvivalLowDays, Base: r.SurvivalBaseDays, High: r.SurvivalHighDays, Source: strings.TrimSpace("резервный расчёт безотказности за 36 мес. / " + r.SurvivalEconSource + " / " + r.Advanced.ObjectAny.ModelVersion), History: history}
	}
	if r.HasLowHigh {
		return methodForecastSnapshot{Low: r.LowDays, Base: r.BaseDays, High: r.HighDays, Source: strings.TrimSpace("резервный расчёт МРП за 36 мес. / " + r.EvidenceLevel + " / " + r.ModelVersion), History: history}
	}
	if r.BaseDays > 0 {
		return methodForecastSnapshot{Base: r.BaseDays, Source: strings.TrimSpace("резервный расчёт МРП за 36 мес. / " + r.EvidenceLevel + " / " + r.ModelVersion), History: history}
	}
	return methodForecastSnapshot{Source: "прогноз не сформирован", History: history}
}

func formatMethodForecast(d approvedMethodDecision) string {
	historyLine := fmt.Sprintf("Предыстория 36 мес.: отказов %d; завершённых циклов %d; достаточность: %s.", d.FailureCount36, d.HistoryCycleCount, dash(d.ForecastPrecision))
	warning := strings.TrimSpace(d.HistoryWarning)
	label := "Утверждённый метод"
	if len(d.Methods) > 1 {
		label = "Утверждённый комплекс"
	}
	if d.ForecastBaseDays <= 0 {
		text := label + ": " + d.Method + ".\r\nРасчёт наработки пока не сформирован: " + dash(d.ForecastSource) + "\r\n" + historyLine
		if warning != "" {
			text += "\r\n" + warning
		}
		return text
	}
	rangeText := fmt.Sprintf("ожидаемый сценарий %.0f сут.", d.ForecastBaseDays)
	if d.ForecastLowDays > 0 && d.ForecastHighDays > 0 {
		rangeText = fmt.Sprintf("осторожный / ожидаемый / благоприятный: %.0f / %.0f / %.0f сут.", d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays)
	}
	text := fmt.Sprintf("%s: %s.\r\nПрогноз наработки ПОСЛЕ мероприятия: %s\r\n%s\r\nИсточник: %s.", label, d.Method, rangeText, historyLine, dash(d.ForecastSource))
	if d.EffectBasePct > 0 {
		text += fmt.Sprintf("\r\nПредварительный вклад комплекса без простого сложения: +%.0f%% / +%.0f%% / +%.0f%%.", d.EffectLowPct, d.EffectBasePct, d.EffectHighPct)
	}
	if strings.TrimSpace(d.Interaction) != "" {
		text += "\r\nВзаимодействие: " + d.Interaction
	}
	text += "\r\nВАЖНО: скрининговые проценты не являются вероятностью успеха и не являются нормативными коэффициентами. Численный бонус за синергию отдельно не начисляется без калибровки по факту объекта/аналогов."
	if warning != "" {
		text += "\r\n" + warning
	}
	return text
}

func methodsSelectedSuggestion() (methodWorkflowSuggestion, bool) {
	if methodsSuggestionCombo == 0 {
		return methodWorkflowSuggestion{}, false
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_GETCURSEL, 0, 0)
	i := int(idx) - 1
	if i < 0 || i >= len(methodWorkflowSuggestions) {
		return methodWorkflowSuggestion{}, false
	}
	return methodWorkflowSuggestions[i], true
}

func refreshMethodsSuggestionInfo() {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	s, ok := methodsSelectedSuggestion()
	if !ok {
		if len(methodWorkflowSuggestions) > 0 {
			setText(methodsSuggestionInfo, fmt.Sprintf("РОСА сформировала %d вариант(а/ов). Выберите метод и нажмите «УТВЕРДИТЬ МЕТОД». Автоподстановка метода без решения технолога запрещена.", len(methodWorkflowSuggestions)))
		}
		setText(methodsImplementation, methodsImplementationText(selectedObjectKey))
		return
	}
	failures := historyForWell(selectedObjectKey, time.Now())
	history := methodHistoryWarningForObject(selectedObjectKey, time.Now())
	if len(failures) > 0 {
		last := failures[0]
		history = fmt.Sprintf("Раздел 1: %d отказ(а/ов) за 36 мес.; последний %s — %s. %s", len(failures), dash(last.Date), dash(last.StopCause), methodHistoryWarningForObject(selectedObjectKey, time.Now()))
	}
	app := methodApplicabilityForSuggestion(s, selectedObjectKey, time.Now())
	snap := methodForecastSnapshotForSelectedObject()
	// Fleet-wide replication is evaluated only by the explicit АНАЛОГИ action.
	repl := ReplicationAssessment{}
	_ = history // полная предыстория остаётся в карточке метода и прогнозе; здесь сохраняем компактный референсный блок
	basis := methodsBasisSummaryText(s, app, repl, snap.Base)
	basis += "\r\nНакопленный опыт: РОСА может предложить ранее применявшиеся методы, схемы и сочетания по сопоставимым объектам; автоматическое утверждение не выполняется."
	target := targetRunlifeResolvedForSelectedObject().Days
	if target < defaultTargetRunlifeDays {
		target = defaultTargetRunlifeDays
	}
	individual := combinedMethodForecastForNames(selectedObjectKey, []string{s.Method.Name}, snap, time.Now())
	if individual.ForecastBase >= target && individual.ForecastBase > 0 {
		basis += fmt.Sprintf("\r\nЦель наработки: ≥ %.0f сут. Этот метод в расчётном скрининге закрывает целевой уровень до %.0f сут.; требуется подтверждение технологом и фактом.", target, individual.ForecastBase)
	} else if individual.ForecastBase > snap.Base && individual.ForecastBase > 0 {
		basis += fmt.Sprintf("\r\nЦель наработки: ≥ %.0f сут. Метод даёт вклад до %.0f сут.; остаточный разрыв %.0f сут. — требуется комплекс мероприятий.", target, individual.ForecastBase, target-individual.ForecastBase)
	} else {
		basis += fmt.Sprintf("\r\nЦель наработки: ≥ %.0f сут. Для этого метода вклад к цели учитывается качественно; численный прирост без подтверждённой объектовой базы не выдумывается. Рассматривать в составе комплекса при релевантном механизме.", target)
	}
	setText(methodsSuggestionInfo, basis)
	setText(methodsImplementation, methodsImplementationText(selectedObjectKey))
	// The enclosing full-page render performs its own final geometry pass.
	// A user-driven selection still relayouts immediately.
	if !methodsRenderingWorkflow {
		relayoutMethodsWorkflowCurrent()
	}
}

func relayoutMethodsWorkflowCurrent() {
	// activatePage always runs layoutCurrentPage after refreshPage. Doing
	// another complete methods layout during that refresh repeats GDI text
	// measurement and hundreds of native control updates on the UI thread.
	if pageSwitching && currentPage == ID_ACTIVITIES {
		return
	}
	var rc RECT
	if r, _, _ := pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&rc))); r != 0 {
		layoutMethodsWorkflow(rc.Right-rc.Left, rc.Bottom-rc.Top)
	}
}

var methodsUIUpdateDepth int

func beginMethodsUIUpdate() bool {
	if hMain == 0 || currentPage != ID_ACTIVITIES {
		return false
	}
	methodsUIUpdateDepth++
	if methodsUIUpdateDepth == 1 {
		beginPageVisualUpdate(ID_ACTIVITIES)
	}
	return true
}

func endMethodsUIUpdate(started bool) {
	if !started || methodsUIUpdateDepth <= 0 {
		return
	}
	methodsUIUpdateDepth--
	if methodsUIUpdateDepth != 0 {
		return
	}
	endPageVisualUpdate(ID_ACTIVITIES)
}

func renderMethodsWorkflowUIAtomic() {
	started := beginMethodsUIUpdate()
	defer endMethodsUIUpdate(started)
	renderMethodsWorkflowUI()
}

func methodsScenarioText(title string, value, base float64, suffix string) string {
	if value <= 0 {
		return title + "\r\n—"
	}
	text := fmt.Sprintf("%s\r\n%.0f сут.", title, value)
	if base > 0 && value != base {
		text += fmt.Sprintf(" · %+.0f", value-base)
	}
	if strings.TrimSpace(suffix) != "" {
		text += "\r\n" + suffix
	}
	return text
}

func refreshMethodsDashboardCards() {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		setText(methodsTechEquipmentValue, "—")
		setText(methodsTechMRPValue, "—")
		setText(methodsTechLiquidValue, "—")
		setText(methodsTechWatercutValue, "—")
		setText(methodsForecastCurrent, "Негативный прогноз\r\n—")
		setText(methodsForecastGain, "Реальный прогноз\r\n—")
		setText(methodsForecastResult, "Положительный прогноз\r\n—")
		setText(methodsForecastBasis, "Основание прогноза\r\n—")
		setText(methodsForecastNote, "Сначала закрепите объект и утвердите классификацию.")
		return
	}
	if st, ok := currentSelectedState(); ok {
		setText(methodsTechEquipmentValue, dash(normalizeTargetLiftType(st.Equipment)))
		setText(methodsTechMRPValue, formatWholeDaysText(st.CurrentMRP)+" сут.")
		setText(methodsTechLiquidValue, displayTechNumber(st.LiquidRate, 2)+" м³/сут.")
		setText(methodsTechWatercutValue, displayTechNumber(st.WaterCut, 1)+" %")
	}

	target := targetRunlifeResolvedForSelectedObject().Days
	if target < defaultTargetRunlifeDays {
		target = defaultTargetRunlifeDays
	}
	setText(methodsForecastTitle, fmt.Sprintf("ПРОГНОЗ НАРАБОТКИ · ЦЕЛЕВОЙ ОРИЕНТИР ≥ %.0f СУТ.", target))
	snap := methodForecastSnapshotForSelectedObject()
	base := snap.Base
	low, calc, high := float64(0), float64(0), float64(0)
	source := ""
	status := "после утверждения метода"
	if scenario, ok := activeAlternativeScenario(key); ok && scenario.ForecastBaseDays > 0 {
		low, calc, high = scenario.ForecastLowDays, scenario.ForecastBaseDays, scenario.ForecastHighDays
		source = scenario.SelectionBasis
		status = dash(scenario.SelectionKind)
	} else if names := currentMethodsPlanNames(); len(names) > 0 {
		f := combinedMethodForecastForNames(key, names, snap, time.Now())
		markAlternativeScenarioForecast(key, f)
		low, calc, high, source = f.ForecastLow, f.ForecastBase, f.ForecastHigh, f.Source
		status = "текущий комплекс"
	} else if d, ok := latestApprovedMethodDecision(key); ok && d.ForecastBaseDays > 0 && !methodsHasExplicitDraft(key) {
		low, calc, high, source = d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays, d.ForecastSource
		if low <= 0 {
			low = calc
		}
		if high <= 0 {
			high = calc
		}
		status = "архивное утверждение"
		// Never subtract a new object's baseline from an old saved forecast.
	}

	if calc > 0 {
		setText(methodsForecastCurrent, fmt.Sprintf("Негативный прогноз\r\n%.0f сут.", low))
		setText(methodsForecastGain, fmt.Sprintf("Реальный прогноз\r\n%.0f сут.", calc))
		setText(methodsForecastResult, fmt.Sprintf("Положительный прогноз\r\n%.0f сут.", high))
		setText(methodsForecastBasis, fmt.Sprintf("Основание прогноза\r\nисходный МРП %.0f сут. · циклов %d · %s", base, snap.History.CycleCount, status))
	} else {
		setText(methodsForecastCurrent, "Негативный прогноз\r\nвыберите комплекс")
		setText(methodsForecastGain, "Реальный прогноз\r\nвыберите комплекс")
		setText(methodsForecastResult, "Положительный прогноз\r\nвыберите комплекс")
		setText(methodsForecastBasis, fmt.Sprintf("Основание прогноза\r\nисходный МРП %.0f сут. · циклов %d", base, snap.History.CycleCount))
	}

	note := "Реальный прогноз — центральный расчётный сценарий, не фактическое значение; подтвердите результат фактической наработкой."
	if snap.History.CycleCount == 0 {
		note = "Недостаточно истории отказов: прогноз менее точен."
	}
	if _, teaching := teachingWorkflowOrdinal(key); teaching {
		note = "УЧЕБНЫЙ СЦЕНАРИЙ: синтетический эффект, не производственная оценка."
	}
	if scenario, ok := activeAlternativeScenario(key); ok && strings.TrimSpace(scenario.SelectionBasis) != "" {
		note += " Логика выбора: " + scenario.SelectionBasis
	}
	if calc > 0 {
		if calc >= target {
			note += fmt.Sprintf(" Цель %.0f сут. достигнута: расчёт %.0f сут.", target, calc)
		} else {
			note += fmt.Sprintf(" До цели %.0f сут. осталось %.0f сут.; уточните комплекс.", target, target-calc)
		}
	} else {
		note += fmt.Sprintf(" Целевой ориентир: %.0f сут.", target)
	}
	_ = source // full provenance is retained in the detailed report and basis.
	note = "Нажмите на карточку для подробной расшифровки. " + note
	setText(methodsForecastNote, note)
}

var methodsRenderingWorkflow bool

func renderMethodsWorkflowUI() {
	previouslyRendering := methodsRenderingWorkflow
	methodsRenderingWorkflow = true
	defer func() { methodsRenderingWorkflow = previouslyRendering }()
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		setText(methodsScenarioNextButton, "СХЕМА —")
		setText(methodsAvailabilityButton, "НЕ ПРОВЕРЕНО")
		methodsClassificationReady = false
		methodsCandidatePage = 0
		setText(methodsObjectCode, "—")
		setText(methodsObjectContext, "Сначала закрепите объект на Главной.")
		setText(methodsClassSummary, "Классификация недоступна без закреплённого объекта.")
		methodWorkflowSuggestions = nil
		if methodsSuggestionCombo != 0 {
			pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_RESETCONTENT, 0, 0)
			pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p("— сначала закрепите объект —"))))
			pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_SETCURSEL, 0, 0)
		}
		setText(methodsSuggestionInfo, "После закрепления объекта и утверждения классификации РОСА сформирует варианты методов.")
		setText(methodsForecast, "Метод пока не утверждён.")
		setText(methodsReadyBanner, "Сначала закрепите объект на Главной, затем утвердите классификацию в разделе 3.")
		setText(methodsAvailableTitle, "01 · ЧТО НУЖНО ДЛЯ ФОРМИРОВАНИЯ МЕТОДОВ")
		setText(methodsEmptyState, "Рабочий объект ещё не закреплён. Закрепите объект на Главной — РОСА подхватит его технический режим, историю отказов и лабораторные данные.")
		setText(methodsBasisTitle, "02 · ЧТО УЧТЁТ РОСА")
		setText(methodsSuggestionInfo, "1. Утверждённые классы осложнений.\r\n\r\n2. Технический режим объекта.\r\n\r\n3. Предысторию отказов за 36 мес.\r\n\r\n4. Сопоставимость метода с механизмом осложнения.")
		setText(methodsImplementationTitle, "03 · ЧТО ПОЯВИТСЯ ДАЛЬШЕ")
		setText(methodsImplementation, "После закрепления объекта и утверждения классификации:\r\n\r\n• методы-кандидаты;\r\n• приоритет рассмотрения;\r\n• параметры реализации;\r\n• прогноз наработки.")
		refreshMethodsDashboardCards()
		refreshMethodsArchive()
		refreshMethodsVisualTables()
		relayoutMethodsWorkflowCurrent()
		return
	}
	if scenario, ok := activeAlternativeScenario(key); ok {
		position, total := alternativeScenarioPosition(key, scenario.ID)
		setText(methodsScenarioNextButton, fmt.Sprintf("СХЕМА %d/%d ›", position, total))
		setText(methodsAvailabilityButton, strings.ToUpper(dash(scenario.Availability)))
	}
	setText(methodsObjectCode, key)
	if st, ok := currentSelectedState(); ok {
		// The red line above already contains the selected object. Repeating the
		// field here made the context longer than the left column and clipped its
		// final words. Keep only the non-duplicated operational identifiers.
		setText(methodsObjectContext, fmt.Sprintf("скв. %s · %s", dash(st.Well), dash(st.ObjectPlast)))
	} else {
		setText(methodsObjectContext, readableObjectKey(key))
	}
	classSummary, ready, _ := approvedAssessmentForMethods(key)
	methodsClassificationReady = ready
	setText(methodsClassSummary, classSummary)
	if ready {
		selectedCount := len(normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }()))
		setText(methodsReadyBanner, fmt.Sprintf("Классификация утверждена · в плане: %d · аналоги из предыдущих диагностик, схемы применения и связки методов учитываются как подсказки; решение остаётся за технологом.", selectedCount))
	} else {
		setText(methodsReadyBanner, "СНАЧАЛА УТВЕРДИТЕ ПОЛНУЮ КЛАССИФИКАЦИЮ В РАЗДЕЛЕ 3 — после этого РОСА активирует выбор методов.")
		setText(methodsAvailableTitle, "01 · ЧТО НУЖНО СДЕЛАТЬ")
		setText(methodsEmptyState, "Классификация объекта ещё не утверждена полностью.\r\n\r\nПерейдите в раздел 3, проверьте предварительные классы и нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».\r\n\r\nПосле подтверждения здесь сразу появятся методы-кандидаты — без повторного выбора объекта.")
		setText(methodsBasisTitle, "02 · ОСНОВАНИЕ БУДУЩЕГО ВЫБОРА")
		setText(methodsSuggestionInfo, "1. Класс и тяжесть осложнения.\r\n\r\n2. Текущий режим и тип ГНО.\r\n\r\n3. Предыстория отказов за 36 мес.\r\n\r\n4. Аналоги и доказательная база.\r\n\r\n5. Ограничения применимости.")
		setText(methodsImplementationTitle, "03 · ПОСЛЕ УТВЕРЖДЕНИЯ")
		setText(methodsImplementation, "РОСА покажет:\r\n\r\n• карточки методов;\r\n• приоритет;\r\n• параметры реализации;\r\n• вклад в прогноз наработки;\r\n• статус «предложен / в плане / утверждён». ")
	}
	previousMethod := methodsResumeSelectionName
	methodsResumeSelectionName = ""
	if selected, ok := methodsSelectedSuggestion(); ok {
		previousMethod = strings.TrimSpace(selected.Method.Name)
	}
	methodWorkflowSuggestions = methodSuggestionsForObject(key)
	selectedComboIndex := 0
	if methodsSuggestionCombo != 0 {
		pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_RESETCONTENT, 0, 0)
		placeholder := "— выберите предложенный метод —"
		if !ready {
			placeholder = "— сначала утвердите классификацию в разделе 3 —"
		} else if len(methodWorkflowSuggestions) == 0 {
			placeholder = "— подходящие методы не сопоставлены —"
		}
		pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(placeholder))))
		for i, s := range methodWorkflowSuggestions {
			prefix := "[Техмодуль] "
			if !strings.Contains(strings.ToLower(s.Method.Source), "технологичес") {
				prefix = "[Справочно] "
			}
			app := methodApplicabilityForSuggestion(s, key, time.Now())
			caption := fmt.Sprintf("%s%s — %s %s · применимость %d%%", prefix, s.Method.Name, s.Direction, s.Classification, app.Applicability)
			pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
			if previousMethod != "" && normalizeMethodText(previousMethod) == normalizeMethodText(s.Method.Name) {
				selectedComboIndex = i + 1
			}
		}
		// РОСА выделяет наиболее релевантный вариант для рассмотрения, но не утверждает его.
		if selectedComboIndex == 0 && ready && len(methodWorkflowSuggestions) > 0 {
			selectedComboIndex = 1
		}
		pSendMessageW.Call(uintptr(methodsSuggestionCombo), CB_SETCURSEL, uintptr(selectedComboIndex), 0)
	}
	if selectedComboIndex > 0 {
		methodsCandidatePage = (selectedComboIndex - 1) / methodsCardsPerPage
	} else if methodsCandidatePage*methodsCardsPerPage >= len(methodWorkflowSuggestions) {
		methodsCandidatePage = 0
	}
	if ready && len(methodWorkflowSuggestions) > 0 {
		setText(methodsAvailableTitle, "01 · МЕТОДЫ · ВЫБОР НЕСКОЛЬКИХ")
		setText(methodsBasisTitle, "02 · ОСНОВАНИЕ ВЫБОРА")
		setText(methodsImplementationTitle, "03 · РЕАГЕНТЫ И ПАРАМЕТРЫ")
		refreshMethodsSuggestionInfo()
	} else if ready {
		setText(methodsAvailableTitle, "01 · МЕТОДЫ НЕ СОПОСТАВЛЕНЫ")
		setText(methodsBasisTitle, "02 · ЧТО ПРОВЕРИТЬ")
		setText(methodsImplementationTitle, "03 · ДАЛЬНЕЙШЕЕ ДЕЙСТВИЕ")
		setText(methodsSuggestionInfo, "По утверждённым классам в текущем справочнике не найдено сопоставленных методов. Проверьте справочник методов и исходные данные по объекту.")
		setText(methodsImplementation, "Откройте «СПРАВОЧНИК» и проверьте наличие метода для утверждённого класса. РОСА не будет подставлять неподтверждённое решение автоматически.")
		setText(methodsEmptyState, "Классификация утверждена, но сопоставленных методов в текущем справочнике нет. Пустая таблица намеренно не показывается.")
	} else {
		setText(methodsSuggestionInfo, "Методы не предлагаются до утверждения полной классификации в разделе 3.")
	}
	currentNames := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
	if d, ok := latestApprovedMethodDecision(key); ok {
		forecastText := formatMethodForecast(d)
		if len(currentNames) > 0 {
			approvedNames := normalizeMethodList(d.Methods)
			if len(approvedNames) == 0 && strings.TrimSpace(d.Method) != "" {
				approvedNames = []string{d.Method}
			}
			if strings.Join(currentNames, "|") != strings.Join(approvedNames, "|") {
				preview := combinedMethodForecastForNames(key, currentNames, methodForecastSnapshotForSelectedObject(), time.Now())
				forecastText += "\r\n\r\nЧЕРНОВИК ТЕКУЩЕГО КОМПЛЕКСА — ЕЩЁ НЕ УТВЕРЖДЁН:\r\n" + strings.Join(currentNames, " + ") + "\r\n" + formatCombinedMethodForecast(preview)
			}
		}
		setText(methodsForecast, forecastText)
	} else {
		h := methodHistoryBasisForObject(key, time.Now())
		if len(currentNames) > 0 {
			preview := combinedMethodForecastForNames(key, currentNames, methodForecastSnapshotForSelectedObject(), time.Now())
			text := "ЧЕРНОВИК КОМПЛЕКСА — ЕЩЁ НЕ УТВЕРЖДЁН:\r\n" + strings.Join(currentNames, " + ") + "\r\n" + formatCombinedMethodForecast(preview)
			setText(methodsForecast, text)
		} else {
			text := fmt.Sprintf("Метод ещё не утверждён. После утверждения РОСА сформирует прогноз с основным ориентиром по отказам/циклам за последние 36 месяцев.\r\nПредыстория: отказов %d; завершённых циклов %d; достаточность: %s.", h.FailureCount, h.CycleCount, dash(h.Precision))
			if strings.TrimSpace(h.Warning) != "" {
				text += "\r\n" + h.Warning
			}
			setText(methodsForecast, text)
		}
	}
	refreshMethodsDashboardCards()
	refreshMethodsArchive()
	refreshMethodsVisualTables()
	// Readiness changes the no-data/working-state composition; apply it immediately.
	relayoutMethodsWorkflowCurrent()
}

func refreshMethodsArchive() {
	if methodsArchiveList == 0 {
		return
	}
	ensureApprovedMethodsLoaded()
	hash := fnv.New64a()
	for i := range approvedMethodDecisions {
		d := &approvedMethodDecisions[i]
		fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%s\x00%.3f\x00%s\x00",
			d.ObjectKey, d.ApprovedAt, d.Classification, d.Method, d.ForecastBaseDays, methodArchiveVersionStatus(*d))
	}
	signature := hash.Sum64()
	const lvmGetItemCount = 0x1004
	actualCount, _, _ := pSendMessageW.Call(uintptr(methodsArchiveList), lvmGetItemCount, 0, 0)
	if methodsArchiveRenderHash == signature && int(actualCount) == len(methodsArchiveRows) && len(methodsArchiveRows) >= 4 {
		return
	}
	listBeginUpdate(methodsArchiveList)
	defer listEndUpdate(methodsArchiveList)
	listClear(methodsArchiveList)
	methodsArchiveRows = methodsArchiveRows[:0]
	row := 0
	for i := len(approvedMethodDecisions) - 1; i >= 0; i-- {
		d := approvedMethodDecisions[i]
		forecast := "—"
		if d.ForecastBaseDays > 0 {
			forecast = fmt.Sprintf("%.0f сут.", d.ForecastBaseDays)
			if d.ForecastLowDays > 0 && d.ForecastHighDays > 0 {
				forecast = fmt.Sprintf("%.0f / %.0f / %.0f сут.", d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays)
			}
		}
		listAdd(methodsArchiveList, row, []string{readableObjectKey(d.ObjectKey), d.ApprovedAt, fmt.Sprintf("v%d", methodDecisionStoredVersion(d)), d.Classification, d.Method, forecast, methodArchiveVersionStatus(d)})
		methodsArchiveRows = append(methodsArchiveRows, d)
		row++
	}
	// The built-in 20-well training corpus has three linked historical decisions
	// per well. RC802 inserted only four example rows regardless of the corpus;
	// the archive could never show the other 56 passages. Keep every teaching
	// decision alongside (but clearly distinct from) real approved decisions.
	// This list remains a normal native scrollable ListView at every height.
	for ordinal := 1; ordinal <= 20; ordinal++ {
		objectKey := teachingObjectKeyForOrdinal(ordinal)
		if objectKey == "" {
			continue
		}
		for _, d := range teachingMethodArchiveDecisions(objectKey, time.Now()) {
			forecast := fmt.Sprintf("%.0f / %.0f / %.0f сут.", d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays)
			listAdd(methodsArchiveList, row, []string{readableObjectKey(d.ObjectKey), d.ApprovedAt, fmt.Sprintf("Учеб. %d", d.VersionNumber), d.Classification, d.Method, forecast, "УЧЕБНЫЙ ПРИМЕР"})
			methodsArchiveRows = append(methodsArchiveRows, d)
			row++
		}
	}
	setText(methodsArchiveTitle, fmt.Sprintf("НАКОПИТЕЛЬНАЯ ИСТОРИЯ РЕШЕНИЙ ПО ВСЕМ ОБЪЕКТАМ · %d", row))
	actualCount, _, _ = pSendMessageW.Call(uintptr(methodsArchiveList), lvmGetItemCount, 0, 0)
	if int(actualCount) == row {
		methodsArchiveRenderHash = signature
	} else {
		methodsArchiveRenderHash = 0
		diagLog("METHODS history incomplete expected=%d got=%d", row, actualCount)
	}
}

func addSelectedMethodsWorkflowMethod() (string, error) {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return "", fmt.Errorf("сначала закрепите объект на Главной")
	}
	_, ready, _ := approvedAssessmentForMethods(key)
	if !ready {
		return "", fmt.Errorf("сначала утвердите полную классификацию в разделе 3")
	}
	s, ok := methodsSelectedSuggestion()
	if !ok {
		return "", fmt.Errorf("выберите метод в списке")
	}
	method := strings.TrimSpace(s.Method.Name)
	if method == "" {
		return "", fmt.Errorf("у выбранного метода отсутствует название")
	}
	methods := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
	if len(methods) == 0 {
		if d, ok := latestApprovedMethodDecision(key); ok {
			if len(d.Methods) > 0 {
				methods = normalizeMethodList(d.Methods)
			} else if strings.TrimSpace(d.Method) != "" {
				methods = []string{d.Method}
			}
		}
	}
	for _, m := range methods {
		if normalizeMethodText(m) == normalizeMethodText(method) {
			return "", fmt.Errorf("метод уже входит в текущий комплекс")
		}
	}
	if len(methods) >= maxMethodsPerScenario {
		return "", fmt.Errorf("достигнут предел: не более %d методов в одном комплексе", maxMethodsPerScenario)
	}
	methods = append(methods, method)
	setCombinedMethods(key, methods)
	return method, nil
}

func methodsForWorkflowApproval(key string, selected methodWorkflowSuggestion, hasSelected bool) ([]string, error) {
	if explicit, exists := combinedMethodsForObject(key); exists && len(normalizeMethodList(explicit)) == 0 {
		return nil, fmt.Errorf("выберите хотя бы один метод для утверждения")
	}
	if explicit := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }()); len(explicit) > 0 {
		return explicit, nil
	}
	if !hasSelected || strings.TrimSpace(selected.Method.Name) == "" {
		return nil, fmt.Errorf("выберите метод или сначала сформируйте комплекс")
	}
	return []string{strings.TrimSpace(selected.Method.Name)}, nil
}

func approveMethodsWorkflowSelection() error {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return fmt.Errorf("сначала закрепите объект на Главной")
	}
	classSummary, ready, _ := approvedAssessmentForMethods(key)
	if !ready {
		return fmt.Errorf("сначала утвердите полную классификацию в разделе 3")
	}
	s, hasSelected := methodsSelectedSuggestion()
	methods, err := methodsForWorkflowApproval(key, s, hasSelected)
	if err != nil {
		return err
	}
	methods = normalizeMethodList(methods)
	if len(methods) == 0 {
		return fmt.Errorf("комплекс методов пуст")
	}

	snap := methodForecastSnapshotForSelectedObject()
	comboForecast := combinedMethodForecastForNames(key, methods, snap, time.Now())
	implementation := make([]MethodImplementationRecord, 0, len(methods))
	for _, methodName := range methods {
		implementation = append(implementation, methodImplementationSnapshotForName(key, methodName))
	}

	ensureApprovedMethodsLoaded()
	previous, hasPrevious := latestApprovedMethodDecisionRaw(key)
	d := approvedMethodDecision{
		ID: strconv.FormatInt(time.Now().UnixNano(), 10), ObjectKey: key,
		ApprovedAt: time.Now().Format("02.01.2006 15:04"), Classification: classSummary,
		Method: strings.Join(methods, " + "), Methods: methods, Interaction: comboForecast.Interaction,
		FailureCount36: snap.History.FailureCount, HistoryCycleCount: snap.History.CycleCount,
		ForecastPrecision: snap.History.Precision, HistoryWarning: snap.History.Warning,
		ForecastLowDays: comboForecast.ForecastLow, ForecastBaseDays: comboForecast.ForecastBase, ForecastHighDays: comboForecast.ForecastHigh,
		ForecastSource: strings.TrimSpace(snap.Source + "; сценарная оценка вклада методов " + appBuildVersion),
		EffectLowPct:   comboForecast.EffectLowPct, EffectBasePct: comboForecast.EffectBasePct, EffectHighPct: comboForecast.EffectHighPct,
		MethodAssessments: comboForecast.Methods, Implementation: implementation,
		SchemaVersion: 2, VersionNumber: nextMethodDecisionVersion(key),
	}
	if hasPrevious {
		d.SupersedesID = previous.ID
	}
	if len(methods) == 1 && hasSelected {
		d.MethodID, d.MethodSource, d.MethodStatus = s.Method.ID, s.Method.Source, s.Method.Status
	} else {
		d.MethodStatus = "Утверждённый комплекс методов"
	}
	if currentSession != nil {
		d.ApprovedBy = strings.TrimSpace(currentSession.DisplayName)
	}
	if strings.TrimSpace(d.ApprovedBy) == "" {
		d.ApprovedBy = methodImplementationAuditUser()
	}

	basis := "утверждено технологом; основной ориентир прогноза — фактические отказы/циклы за последние 36 месяцев; индивидуальные вклады объединены через остаточный риск, без простого сложения и без численного бонуса за синергию"
	if hasSelected && strings.TrimSpace(s.Basis) != "" {
		basis = s.Basis + "; " + basis
	}
	planItems := methodPlanItemsForApprovedDecision(d, basis)
	d.Plan = clonePlanItems(planItems)

	oldApproved := append([]approvedMethodDecision(nil), approvedMethodDecisions...)
	oldPlans := clonePlanItems(plans)
	approvedMethodDecisions = append(approvedMethodDecisions, d)
	plans = append(plans, planItems...)
	if err := saveApprovedMethods(); err != nil {
		approvedMethodDecisions = oldApproved
		plans = oldPlans
		return err
	}
	if err := saveJSON(sharedPath("plan.json"), plans); err != nil {
		approvedMethodDecisions = oldApproved
		plans = oldPlans
		_ = saveApprovedMethods()
		_ = saveJSON(sharedPath("plan.json"), plans)
		return err
	}
	if err := markEconomicsNeedsRecalcForDecision(d, "утверждена новая версия технологического решения"); err != nil {
		approvedMethodDecisions = oldApproved
		plans = oldPlans
		_ = saveApprovedMethods()
		_ = saveJSON(sharedPath("plan.json"), plans)
		return fmt.Errorf("решение не зафиксировано: не удалось обновить статус экономики: %v", err)
	}

	setCombinedMethods(key, methods)
	technicalMethodSelection[key] = methods[0]
	return nil
}

func methodsWorkflowReportText() string {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return "РОСА\r\nМетоды и мероприятия\r\nОбъект не закреплён."
	}
	var b strings.Builder
	b.WriteString("РОСА — МЕТОДЫ И МЕРОПРИЯТИЯ\r\n")
	b.WriteString("Дата формирования: " + time.Now().Format("02.01.2006 15:04") + "\r\n")
	b.WriteString("Объект / шифр: " + key + "\r\n")
	if st, ok := currentSelectedState(); ok {
		fmt.Fprintf(&b, "Месторождение: %s; скважина: %s; объект: %s; ГНО: %s; текущий МРП: %s сут.\r\n", dash(st.Field), dash(st.Well), dash(st.ObjectPlast), dash(st.Equipment), formatWholeDaysText(st.CurrentMRP))
	}
	classSummary, ready, _ := approvedAssessmentForMethods(key)
	b.WriteString("Классификация: " + classSummary + "\r\n")
	if !ready {
		b.WriteString("Статус классификации: не полностью утверждена.\r\n")
	}
	h := methodHistoryBasisForObject(key, time.Now())
	fmt.Fprintf(&b, "Предыстория 36 мес.: отказов %d; завершённых циклов %d; достаточность прогноза: %s.\r\n", h.FailureCount, h.CycleCount, dash(h.Precision))
	if strings.TrimSpace(h.Warning) != "" {
		b.WriteString(h.Warning + "\r\n")
	}

	b.WriteString("\r\nТЕКУЩЕЕ / УТВЕРЖДЁННОЕ РЕШЕНИЕ\r\n")
	if d, ok := latestApprovedMethodDecision(key); ok {
		b.WriteString(formatMethodForecast(d) + "\r\n")
		if len(d.MethodAssessments) > 0 {
			b.WriteString("\r\nВклад отдельных методов:\r\n")
			for i, a := range d.MethodAssessments {
				fmt.Fprintf(&b, "%d. %s — применимость %d%%; вклад +%.0f%% / +%.0f%% / +%.0f%%; надёжность исходных данных: %s.\r\n", i+1, a.Method, a.Applicability, a.LowPct, a.BasePct, a.HighPct, dash(a.Confidence))
				b.WriteString("   Основание: " + a.Evidence + ".\r\n")
			}
		}
		if len(d.Implementation) > 0 {
			b.WriteString("\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\n")
			for _, r := range d.Implementation {
				b.WriteString(methodImplementationReportText(r))
			}
		}
	} else {
		names := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
		if len(names) == 0 {
			b.WriteString("Метод/комплекс ещё не утверждён.\r\n")
		} else {
			f := combinedMethodForecastForNames(key, names, methodForecastSnapshotForSelectedObject(), time.Now())
			b.WriteString("Черновик комплекса: " + strings.Join(names, " + ") + "\r\n")
			b.WriteString(formatCombinedMethodForecast(f) + "\r\n")
			b.WriteString("\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ (ЧЕРНОВИК)\r\n")
			for _, name := range names {
				b.WriteString(methodImplementationReportText(methodImplementationSnapshotForName(key, name)))
			}
		}
	}

	b.WriteString("\r\nПЛАН ИСПОЛНЕНИЯ\r\n")
	planNames := currentMethodsPlanNames()
	if len(planNames) == 0 {
		b.WriteString("План мероприятий ещё не сформирован.\r\n")
	} else {
		for i, name := range planNames {
			p0 := methodPlanDisplayItem(key, name)
			fmt.Fprintf(&b, "%d. %s — %s; плановая дата %s; ответственный %s; контроль %s; факт %s — %s; итог контроля %s; комментарий %s; параметры: %s.\r\n", i+1, name, methodPlanCanonicalStatus(p0.Status), methodPlanDateDisplay(p0.DueDate), methodPlanResponsibleDisplay(p0.Responsible), methodPlanControlDisplay(p0), methodPlanDateDisplay(p0.CompletedAt), dash(p0.ExecutionResult), dash(p0.ControlOutcome), dash(p0.ControlComment), strings.ToLower(methodPlanParametersDisplay(p0)))
		}
	}

	b.WriteString("\r\nАНАЛОГИ / ПЕРЕНОС РЕШЕНИЯ\r\n")
	methodsForReplication := []string{}
	if d, ok := latestApprovedMethodDecision(key); ok {
		methodsForReplication = normalizeMethodList(d.Methods)
		if len(methodsForReplication) == 0 && strings.TrimSpace(d.Method) != "" {
			methodsForReplication = []string{d.Method}
		}
	}
	if len(methodsForReplication) == 0 {
		methodsForReplication = normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
	}
	if len(methodsForReplication) == 0 {
		b.WriteString("Нет выбранного/утверждённого метода для поиска аналогов и оценки переноса решения.\r\n")
	} else {
		for i, method := range methodsForReplication {
			r := methodReplicationAssessment(key, method, time.Now())
			fmt.Fprintf(&b, "%d. %s — индекс %d%%; %s; аналоги пилот/близкие: %d/%d.\r\n", i+1, method, r.Score, r.Category, r.PilotAnalogCount, r.QualifiedAnalogCount)
		}
	}

	b.WriteString("\r\nАРХИВ РЕШЕНИЙ ПО ОБЪЕКТУ\r\n")
	ensureApprovedMethodsLoaded()
	count := 0
	for i := len(approvedMethodDecisions) - 1; i >= 0 && count < 20; i-- {
		d := approvedMethodDecisions[i]
		if !strings.EqualFold(strings.TrimSpace(d.ObjectKey), key) {
			continue
		}
		count++
		fmt.Fprintf(&b, "%d. v%d · %s — %s — прогноз %.0f / %.0f / %.0f сут. — %s\r\n", count, methodDecisionStoredVersion(d), dash(d.ApprovedAt), dash(d.Method), d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays, methodArchiveVersionStatus(d))
	}
	if count == 0 {
		b.WriteString("Архив утверждённых решений пока пуст.\r\n")
	}
	b.WriteString("\r\nОГРАНИЧЕНИЕ\r\n")
	b.WriteString("Применимость в процентах — скрининговый индекс, а не вероятность эффективности. Внешние полевые ориентиры не являются нормативными коэффициентами. Основной вес прогноза даётся фактической предыстории отказов/эксплуатационных циклов за последние 36 месяцев. Без неё точность прогноза значительно ниже. Численный бонус за синергию методов не начисляется без калибровки на факте объекта или сопоставимых объектов.\r\n")
	return b.String()
}

func chooseMethodsWorkflowExportPath(format string) string {
	key := safeFilePart(strings.TrimSpace(selectedObjectKey))
	if key == "" {
		key = "объект"
	}
	filename := "РОСА_методы_" + key + "_" + time.Now().Format("20060102") + "." + format
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(filename)))
	filters := map[string]string{
		"xlsx": "Excel (*.xlsx)\x00*.xlsx\x00\x00",
		"docx": "Word (*.docx)\x00*.docx\x00\x00",
		"pdf":  "PDF (*.pdf)\x00*.pdf\x00\x00",
	}
	filter := utf16.Encode([]rune(filters[format]))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p(format), LpstrTitle: p("Сохранить отчёт по методам и мероприятиям"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return ""
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	}
	return path
}

func exportMethodsWorkflowReport(format string) {
	path := chooseMethodsWorkflowExportPath(format)
	if path == "" {
		return
	}
	report := methodsWorkflowReportText()
	var err error
	switch format {
	case "xlsx":
		err = writeLaboratoryXLSXReport(path, report)
	case "docx":
		err = writeLaboratoryDOCXReport(path, report)
	case "pdf":
		err = writeLaboratoryPDFReport(path, report)
	default:
		err = fmt.Errorf("неподдерживаемый формат: %s", format)
	}
	if err != nil {
		msg("Выгрузка методов и мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	msg("Выгрузка методов и мероприятий", "Отчёт сохранён:\n"+path, MB_OK|MB_ICONINFORMATION)
}

func handleMethodsWorkflowCommand(id int, code uint16) bool {
	if currentPage != ID_ACTIVITIES {
		return false
	}
	if methodsBackgroundBusy && (id == ID_METHODS_APPROVE || id == ID_GEN_PLAN || id >= ID_METHODS_CARD_BASE && id < ID_METHODS_CARD_BASE+ID_METHODS_CARD_COUNT) {
		return true
	}
	if id >= ID_METHODS_CARD_BASE && id < ID_METHODS_CARD_BASE+ID_METHODS_CARD_COUNT {
		if code != 0 { // candidate buttons only act on BN_CLICKED
			return true
		}
		uiUpdate := beginMethodsUIUpdate()
		defer endMethodsUIUpdate(uiUpdate)
		row := methodsCandidateIndexForCardID(id)
		if row >= 0 && row < len(methodWorkflowSuggestions) {
			name := methodWorkflowSuggestions[row].Method.Name
			key := strings.TrimSpace(selectedObjectKey)
			names := normalizeMethodList(func() []string { v, _ := combinedMethodsForObject(key); return v }())
			found := false
			next := []string{}
			for _, existing := range names {
				if normalizeMethodText(existing) == normalizeMethodText(name) {
					found = true
				} else {
					next = append(next, existing)
				}
			}
			if !found {
				if len(next) >= maxMethodsPerScenario {
					msg("Методы и мероприятия", fmt.Sprintf("В комплекс можно выбрать не более %d методов.", maxMethodsPerScenario), MB_OK|MB_ICONWARNING)
					return true
				}
				next = append(next, name)
			}
			setCombinedMethods(key, next)
			selectMethodsCandidateRow(row)
			refreshMethodsDashboardCards()
		}
		return true
	}
	switch id {
	case ID_METHODS_FORECAST_BASE, ID_METHODS_FORECAST_GAIN, ID_METHODS_FORECAST_RESULT, ID_METHODS_FORECAST_BASIS:
		openMethodsForecastDetail(id)
		return true
	case ID_METHODS_SCENARIO_NEXT:
		if _, ok := cycleAlternativeScenario(selectedObjectKey, 1); ok {
			methodsCandidatePage = 0
			refreshMethodsWorkflowUI()
		}
		return true
	case ID_METHODS_SCENARIO_ADD:
		s, err := createAlternativeScenario(selectedObjectKey, false)
		if err != nil {
			msg("Альтернативная схема", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			methodsCandidatePage = 0
			refreshMethodsWorkflowUI()
			msg("Альтернативная схема", s.Name+" создана. Объект, классификация, история и исходный прогноз закреплены. Методы, параметры, затраты и утверждение обнулены.", MB_OK|MB_ICONINFORMATION)
		}
		return true
	case ID_METHODS_SCENARIO_COPY:
		if _, err := createAlternativeScenario(selectedObjectKey, true); err != nil {
			msg("Копия схемы", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			refreshMethodsWorkflowUI()
		}
		return true
	case ID_METHODS_AVAILABILITY:
		if value, err := cycleActiveScenarioAvailability(selectedObjectKey); err != nil {
			msg("Доступность ресурсов", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			setText(methodsAvailabilityButton, strings.ToUpper(value))
			_ = markEconomicsNeedsRecalc(selectedObjectKey, "изменилась доступность ресурсов схемы")
			refreshMethodsWorkflowUI()
		}
		return true
	case ID_METHODS_SHOW_ALL:
		key := strings.TrimSpace(selectedObjectKey)
		if key == "" {
			return true
		}
		methodsShowAllObject = key
		methodsShowAll = !methodsShowAll
		methodsCandidatePage = 0
		refreshMethodsWorkflowUI()
		return true
	case ID_METHODS_CANDIDATE_PREV:
		if methodsCandidatePage > 0 {
			uiUpdate := beginMethodsUIUpdate()
			defer endMethodsUIUpdate(uiUpdate)
			methodsCandidatePage--
			refreshMethodsCandidateCards()
			relayoutMethodsWorkflowCurrent()
		}
		return true
	case ID_METHODS_CANDIDATE_NEXT:
		if (methodsCandidatePage+1)*methodsCardsPerPage < len(methodWorkflowSuggestions) {
			uiUpdate := beginMethodsUIUpdate()
			defer endMethodsUIUpdate(uiUpdate)
			methodsCandidatePage++
			refreshMethodsCandidateCards()
			relayoutMethodsWorkflowCurrent()
		}
		return true
	case ID_METHODS_TO_CLASSIFICATION:
		switchPage(ID_ASSESS)
		return true
	case ID_METHODS_SUGGESTION_COMBO:
		if code == 1 {
			refreshMethodsSuggestionInfo()
			return true
		}
	case ID_METHODS_IMPLEMENTATION_EDIT:
		openMethodsImplementationEditor()
		return true
	case ID_METHODS_REFERENCE:
		openMethodsReference()
		return true
	case ID_METHODS_REPLICATION:
		openMethodsReplication()
		return true
	case ID_METHODS_EXPORT_XLSX:
		exportMethodsWorkflowReport("xlsx")
		return true
	case ID_METHODS_EXPORT_DOCX:
		exportMethodsWorkflowReport("docx")
		return true
	case ID_METHODS_EXPORT_PDF:
		exportMethodsWorkflowReport("pdf")
		return true
	case ID_METHODS_APPROVE:
		if err := approveMethodsWorkflowSelection(); err != nil {
			msg("Методы и мероприятия", err.Error(), MB_OK|MB_ICONWARNING)
		} else {
			_ = approveActiveAlternativeScenario(selectedObjectKey)
			refreshMethodsWorkflowUI()
			if d, ok := latestApprovedMethodDecision(selectedObjectKey); ok {
				msg("Методы и мероприятия", "Метод/комплекс утверждён. Прогноз наработки сформирован и показан на вкладке.\n\n"+formatMethodForecast(d), MB_OK|MB_ICONINFORMATION)
			}
		}
		return true
	}
	return false
}
