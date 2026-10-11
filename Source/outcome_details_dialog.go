//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	idOutcomeCause           = 7491
	idOutcomeEvidenceSource  = 7492
	idOutcomeDefect          = 7493
	idOutcomeEvidenceComment = 7494
	idOutcomeNextCycleStart  = 7495
	idOutcomeNextCycleStatus = 7496
	idOutcomeObservedEffect  = 7497
	idOutcomeEffectMetric    = 7498
	idOutcomeEffectBefore    = 7499
	idOutcomeEffectAfter     = 7500
	idOutcomeAttribution     = 7501
	idOutcomeDetailSave      = 7502
	idOutcomeDetailClose     = 7503
)

var (
	outcomeDetailsDone          bool
	outcomeDetailsVersionID     string
	outcomeCauseEdit            syscall.Handle
	outcomeEvidenceSourceCombo  syscall.Handle
	outcomeDefectEdit           syscall.Handle
	outcomeEvidenceCommentEdit  syscall.Handle
	outcomeNextCycleStartEdit   syscall.Handle
	outcomeNextCycleStatusCombo syscall.Handle
	outcomeObservedEffectCombo  syscall.Handle
	outcomeEffectMetricEdit     syscall.Handle
	outcomeEffectBeforeEdit     syscall.Handle
	outcomeEffectAfterEdit      syscall.Handle
	outcomeAttributionCombo     syscall.Handle
)

var outcomeEvidenceSources = []string{
	"Не указано",
	"Акт дефектовки",
	"Акт ТРС/КРС",
	"Осмотр оборудования",
	"Телеметрия",
	"Динамограмма",
	"Лабораторные данные",
	"Несколько источников",
	"Иное",
}

var outcomeNextCycleStatuses = []string{
	"Нет данных",
	"Работает",
	"Завершён",
	"Ожидает запуска",
}

var outcomeObservedEffects = []string{
	"Недостаточно данных",
	"Без заметного изменения",
	"Наблюдается улучшение",
	"Наблюдается ухудшение",
	"Смешанный результат",
}

var outcomeAttributionStatuses = []string{
	"Не оценивалась причинная связь",
	"Есть временная связь; причинность не подтверждена",
	"Подтверждено специалистом по совокупности данных",
}

func outcomeComboText(h syscall.Handle) string {
	if h == 0 {
		return ""
	}
	idx, _, _ := pSendMessageW.Call(uintptr(h), CB_GETCURSEL, 0, 0)
	if int32(idx) < 0 {
		return ""
	}
	buf := make([]uint16, 512)
	pSendMessageW.Call(uintptr(h), CB_GETLBTEXT, idx, uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func setOutcomeComboValue(h syscall.Handle, values []string, value string) {
	if h == 0 {
		return
	}
	target := strings.TrimSpace(value)
	index := 0
	for i, item := range values {
		if strings.EqualFold(strings.TrimSpace(item), target) {
			index = i
			break
		}
	}
	pSendMessageW.Call(uintptr(h), CB_SETCURSEL, uintptr(index), 0)
}

func addOutcomeComboItems(h syscall.Handle, items []string) {
	for _, item := range items {
		pSendMessageW.Call(uintptr(h), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(item))))
	}
	pSendMessageW.Call(uintptr(h), CB_SETCURSEL, 0, 0)
}

func preserveOutcomeDetails(dst *Outcome, src *Outcome) {
	if dst == nil || src == nil {
		return
	}
	dst.FailureCause = src.FailureCause
	dst.DefectFinding = src.DefectFinding
	dst.EvidenceSource = src.EvidenceSource
	dst.EvidenceComment = src.EvidenceComment
	dst.NextCycleStartDate = src.NextCycleStartDate
	dst.NextCycleStatus = src.NextCycleStatus
	dst.ObservedEffect = src.ObservedEffect
	dst.EffectMetric = src.EffectMetric
	dst.EffectBefore = src.EffectBefore
	dst.EffectAfter = src.EffectAfter
	dst.EffectAttribution = src.EffectAttribution
}

func outcomeStructuredSummary(o Outcome) string {
	parts := []string{}
	if strings.TrimSpace(o.FailureCause) != "" {
		parts = append(parts, "Причина: "+strings.TrimSpace(o.FailureCause))
	}
	if strings.TrimSpace(o.DefectFinding) != "" {
		parts = append(parts, "Дефектовка: "+strings.TrimSpace(o.DefectFinding))
	}
	if strings.TrimSpace(o.EvidenceSource) != "" && !strings.EqualFold(strings.TrimSpace(o.EvidenceSource), "Не указано") {
		evidence := "Подтверждение: " + strings.TrimSpace(o.EvidenceSource)
		if strings.TrimSpace(o.EvidenceComment) != "" {
			evidence += " — " + strings.TrimSpace(o.EvidenceComment)
		}
		parts = append(parts, evidence)
	}
	cycle := []string{}
	if strings.TrimSpace(o.NextCycleStartDate) != "" {
		cycle = append(cycle, "старт "+strings.TrimSpace(o.NextCycleStartDate))
	}
	if strings.TrimSpace(o.NextCycleStatus) != "" && !strings.EqualFold(strings.TrimSpace(o.NextCycleStatus), "Нет данных") {
		cycle = append(cycle, "статус "+strings.TrimSpace(o.NextCycleStatus))
	}
	if len(cycle) > 0 {
		parts = append(parts, "Следующий цикл: "+strings.Join(cycle, "; "))
	}
	if strings.TrimSpace(o.ObservedEffect) != "" {
		effect := "Наблюдаемый эффект: " + strings.TrimSpace(o.ObservedEffect)
		if strings.TrimSpace(o.EffectMetric) != "" {
			effect += "; показатель: " + strings.TrimSpace(o.EffectMetric)
		}
		if strings.TrimSpace(o.EffectBefore) != "" || strings.TrimSpace(o.EffectAfter) != "" {
			effect += fmt.Sprintf("; до: %s; после: %s", dash(o.EffectBefore), dash(o.EffectAfter))
		}
		parts = append(parts, effect)
	}
	if strings.TrimSpace(o.EffectAttribution) != "" {
		parts = append(parts, "Причинная интерпретация: "+strings.TrimSpace(o.EffectAttribution))
	}
	return strings.Join(parts, "\r\n")
}

func outcomeDetailsDialogWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		title := modalCreate("STATIC", "Причина ремонта, дефектовка и фактический эффект", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 18, 1020, 34, parent, 0)
		if title != 0 && hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		modalCreate("STATIC", "Здесь фиксируются факты после мероприятия. Наблюдаемое изменение не считается доказанным эффектом автоматически.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 56, 1050, 42, parent, 0)

		modalCreate("STATIC", "Причина завершения цикла / отказа", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 108, 330, 24, parent, 0)
		outcomeCauseEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 24, 134, 500, 30, parent, idOutcomeCause)
		modalCreate("STATIC", "Источник подтверждения", WS_CHILD|WS_VISIBLE|SS_LEFT, 548, 108, 260, 24, parent, 0)
		outcomeEvidenceSourceCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 548, 134, 526, 240, parent, idOutcomeEvidenceSource)
		addOutcomeComboItems(outcomeEvidenceSourceCombo, outcomeEvidenceSources)

		modalCreate("STATIC", "Дефектовка / фактически выявленное состояние оборудования", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 176, 700, 24, parent, 0)
		outcomeDefectEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 24, 202, 1050, 84, parent, idOutcomeDefect)

		modalCreate("STATIC", "Что подтверждает причину: документ, место повреждения, наблюдение, сопоставление", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 296, 900, 24, parent, 0)
		outcomeEvidenceCommentEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 24, 322, 1050, 64, parent, idOutcomeEvidenceComment)

		modalCreate("STATIC", "Следующий эксплуатационный цикл — дата начала", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 398, 350, 24, parent, 0)
		outcomeNextCycleStartEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 24, 424, 180, 30, parent, idOutcomeNextCycleStart)
		modalCreate("STATIC", "Статус следующего цикла", WS_CHILD|WS_VISIBLE|SS_LEFT, 228, 398, 240, 24, parent, 0)
		outcomeNextCycleStatusCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 228, 424, 290, 210, parent, idOutcomeNextCycleStatus)
		addOutcomeComboItems(outcomeNextCycleStatusCombo, outcomeNextCycleStatuses)

		modalCreate("STATIC", "Наблюдаемое изменение после мероприятия", WS_CHILD|WS_VISIBLE|SS_LEFT, 548, 398, 360, 24, parent, 0)
		outcomeObservedEffectCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 548, 424, 526, 220, parent, idOutcomeObservedEffect)
		addOutcomeComboItems(outcomeObservedEffectCombo, outcomeObservedEffects)

		modalCreate("STATIC", "Контрольный показатель", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 466, 230, 24, parent, 0)
		outcomeEffectMetricEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 24, 492, 360, 30, parent, idOutcomeEffectMetric)
		modalCreate("STATIC", "До", WS_CHILD|WS_VISIBLE|SS_LEFT, 408, 466, 80, 24, parent, 0)
		outcomeEffectBeforeEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 408, 492, 180, 30, parent, idOutcomeEffectBefore)
		modalCreate("STATIC", "После", WS_CHILD|WS_VISIBLE|SS_LEFT, 612, 466, 80, 24, parent, 0)
		outcomeEffectAfterEdit = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 612, 492, 180, 30, parent, idOutcomeEffectAfter)

		modalCreate("STATIC", "Интерпретация причинной связи — только ручное решение специалиста", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 534, 620, 24, parent, 0)
		outcomeAttributionCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 24, 560, 1050, 220, parent, idOutcomeAttribution)
		addOutcomeComboItems(outcomeAttributionCombo, outcomeAttributionStatuses)
		warning := modalCreate("STATIC", "Важно: эта запись не меняет прогноз МРП и не обучает модель автоматически. Для использования результата в статистике требуется подтверждённая классификация события и достаточная сопоставимая история.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 606, 1050, 48, parent, 0)
		if warning != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(warning), WM_SETFONT, uintptr(hFontBold), 1)
		}

		modalCreate("BUTTON", "Сохранить", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 774, 666, 145, 38, parent, idOutcomeDetailSave)
		modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 929, 666, 145, 38, parent, idOutcomeDetailClose)

		if o := outcomeFor(outcomeDetailsVersionID); o != nil {
			setText(outcomeCauseEdit, o.FailureCause)
			setText(outcomeDefectEdit, o.DefectFinding)
			setText(outcomeEvidenceCommentEdit, o.EvidenceComment)
			setText(outcomeNextCycleStartEdit, o.NextCycleStartDate)
			setText(outcomeEffectMetricEdit, o.EffectMetric)
			setText(outcomeEffectBeforeEdit, o.EffectBefore)
			setText(outcomeEffectAfterEdit, o.EffectAfter)
			setOutcomeComboValue(outcomeEvidenceSourceCombo, outcomeEvidenceSources, o.EvidenceSource)
			setOutcomeComboValue(outcomeNextCycleStatusCombo, outcomeNextCycleStatuses, o.NextCycleStatus)
			setOutcomeComboValue(outcomeObservedEffectCombo, outcomeObservedEffects, o.ObservedEffect)
			setOutcomeComboValue(outcomeAttributionCombo, outcomeAttributionStatuses, o.EffectAttribution)
		}
		return 0

	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idOutcomeDetailSave:
			o := outcomeFor(outcomeDetailsVersionID)
			if o == nil {
				msg("Фактический эффект", "Фактическая запись не найдена. Сначала сохраните фактический результат.", MB_OK|MB_ICONWARNING)
				return 0
			}
			nextStart := strings.TrimSpace(getText(outcomeNextCycleStartEdit))
			if nextStart != "" && parseDate(nextStart).IsZero() {
				msg("Фактический эффект", "Некорректная дата начала следующего цикла. Используйте ДД.ММ.ГГГГ.", MB_OK|MB_ICONWARNING)
				return 0
			}
			o.FailureCause = strings.TrimSpace(getText(outcomeCauseEdit))
			o.DefectFinding = strings.TrimSpace(getText(outcomeDefectEdit))
			o.EvidenceSource = strings.TrimSpace(outcomeComboText(outcomeEvidenceSourceCombo))
			o.EvidenceComment = strings.TrimSpace(getText(outcomeEvidenceCommentEdit))
			o.NextCycleStartDate = nextStart
			o.NextCycleStatus = strings.TrimSpace(outcomeComboText(outcomeNextCycleStatusCombo))
			o.ObservedEffect = strings.TrimSpace(outcomeComboText(outcomeObservedEffectCombo))
			o.EffectMetric = strings.TrimSpace(getText(outcomeEffectMetricEdit))
			o.EffectBefore = strings.TrimSpace(getText(outcomeEffectBeforeEdit))
			o.EffectAfter = strings.TrimSpace(getText(outcomeEffectAfterEdit))
			o.EffectAttribution = strings.TrimSpace(outcomeComboText(outcomeAttributionCombo))
			if e := saveJSON(filepathJoinData("outcomes.json"), outcomes); e != nil {
				msg("Ошибка", e.Error(), MB_OK|MB_ICONERROR)
				return 0
			}
			msg("Фактический эффект", "Структурированная причина, дефектовка и наблюдаемый эффект сохранены. Причинная связь не присваивается программой автоматически.", MB_OK|MB_ICONINFORMATION)
			refreshAll()
			return 0
		case idOutcomeDetailClose:
			outcomeDetailsDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}

	case labMappingClose:
		outcomeDetailsDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func filepathJoinData(name string) string {
	return filepath.Join(dataDir, name)
}

func openOutcomeDetailsDialog() {
	x := filteredVersions()
	i := selectedRow(historyList)
	if i < 0 || i >= len(x) {
		msg("Причина / дефектовка / эффект", "Сначала выберите расчётную версию на странице «История оценок».", MB_OK|MB_ICONWARNING)
		return
	}
	v := x[i]
	if outcomeFor(v.VersionID) == nil {
		msg("Причина / дефектовка / эффект", "Для выбранной версии ещё нет фактического результата. Сначала сохраните факт, затем заполните дефектовку и эффект.", MB_OK|MB_ICONWARNING)
		return
	}
	outcomeDetailsVersionID = v.VersionID
	outcomeDetailsDone = false

	cls := p("ROSAOutcomeDetailsDialogV1")
	hInstRaw, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(outcomeDetailsDialogWndProc),
		HInstance:     syscall.Handle(hInstRaw),
		HbrBackground: syscall.Handle(5 + 1),
		LpszClassName: cls,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(p("РОСА — причина, дефектовка и фактический эффект"))),
		0x00CA0000|WS_VISIBLE,
		0x80000000, 0x80000000,
		1120, 760,
		uintptr(hMain), 0, hInstRaw, 0,
	)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Причина / дефектовка / эффект", "Не удалось открыть окно.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !outcomeDetailsDone {
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
