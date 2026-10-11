//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	failureReportExcel = 9821 + iota
	failureReportWord
	failureReportPDF
)

var failureReportDone bool
var failureReportTitle, failureReportBody, failureReportName string
var failureReportEdit syscall.Handle

func wellFailureObjectCaption(key string) string {
	if v, ok := findObjectViewByKey(key); ok {
		return objectViewLabel(v)
	}
	for _, e := range failureEvents {
		if e.ObjectKey == key {
			return strings.Trim(strings.Join([]string{e.Field, "скв. " + e.Well, e.ObjectPlast}, " / "), " / ")
		}
	}
	return "выбранный объект"
}
func failureReportValue(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Не указано в источнике"
	}
	return strings.TrimSpace(s)
}
func failureReportStatus(e FailureEvent) string {
	if e.ConclusionApproved || (strings.Contains(strings.ToLower(e.InvestigationStatus), "подтверждено комиссией") && strings.TrimSpace(e.CommissionConclusion) != "") {
		return "Подтверждено комиссией"
	}
	return "Предварительное заключение — утверждение не зарегистрировано"
}
func appendFailureEventReport(b *strings.Builder, e FailureEvent) {
	line := func(label, value string) { fmt.Fprintf(b, "%s: %s\n", label, failureReportValue(value)) }
	b.WriteString("ИСХОДНЫЕ СВЕДЕНИЯ\n")
	line("Месторождение", e.Field)
	line("Скважина", e.Well)
	line("Пласт", e.ObjectPlast)
	line("Цех", e.Department)
	line("Производственная единица", e.ProductionUnit)
	line("Дата отказа", e.Date)
	line("Начало цикла", e.CycleStartDate)
	line("Окончание ремонта", e.RepairEndDate)
	line("Событие", e.EventType)
	b.WriteString("\nГНО И РЕЖИМ\n")
	line("ГНО", e.Equipment)
	line("Марка", e.EquipmentBrand)
	line("Типоразмер", e.EquipmentSize)
	line("Режим работы", e.OperationMode)
	line("Отказавший узел", e.FailedUnit)
	b.WriteString("\nПРИЧИНА ОТКАЗА\n")
	line("Причина остановки", e.StopCause)
	line("Код причины", e.SourceCauseCode)
	line("Причина по источнику", e.SourceCause)
	line("Исходная классификация", e.SourceClass)
	line("Оценка специалиста", e.UserClass)
	line("Фактор осложнения", e.ComplicationFactor)
	b.WriteString("\nРАССЛЕДОВАНИЕ И ЗАКЛЮЧЕНИЕ\n")
	line("Статус", failureReportStatus(e))
	line("Осмотр", e.Inspection)
	line("Статус расследования", e.InvestigationStatus)
	line("Заключение комиссии", e.CommissionConclusion)
	line("Предварительное заключение", e.PreliminaryConclusion)
	line("Утверждённое заключение", e.ApprovedConclusion)
	line("Источник", e.Source)
	line("Метод до отказа", e.MethodBeforeFailure)
	line("Метод после отказа", e.MethodAfterFailure)
	line("Комментарий", e.Comment)
	sections := []string{"ПРИЗНАКИ И ПРЕДПОСЫЛКИ", "СОПУТСТВУЮЩИЕ ОСЛОЖНЕНИЯ", "РЕМОНТ, ПРИЧИНЫ И МЕТОДЫ", "КОМПЛЕКТАЦИЯ ГНО", "КАБЕЛЬ, ГИДРОЗАЩИТА И ТМС"}
	last := -1
	for _, spec := range wellExtraSpecs {
		if spec.section != last {
			last = spec.section
			fmt.Fprintf(b, "\n%s\n", sections[last])
		}
		line(strings.TrimSuffix(spec.title, " *"), e.Extra[spec.column])
	}
	b.WriteString("\nРЕКОМЕНДАЦИИ К МЕРОПРИЯТИЯМ\n")
	for i, rec := range failureNextActionRecommendations([]FailureEvent{e}) {
		fmt.Fprintf(b, "%d. %s\n", i+1, rec)
	}
	if !e.ConclusionApproved && !strings.Contains(strings.ToLower(e.InvestigationStatus), "подтверждено комиссией") {
		b.WriteString("Подтвердить причину и мероприятия после завершения расследования.\n")
	}
}
func failureEventReport(e FailureEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "РОСА — КАРТОЧКА ОТКАЗА\nСформировано: %s\n\n", time.Now().Format("02.01.2006 15:04"))
	appendFailureEventReport(&b, e)
	return b.String()
}
func failureAnalysisReport(rows []FailureEvent, key string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "РОСА — АНАЛИЗ ОТКАЗОВ ОБЪЕКТА\nОбъект: %s\nОтказов за 36 месяцев: %d\nСформировано: %s\n", wellFailureObjectCaption(key), len(rows), time.Now().Format("02.01.2006 15:04"))
	approved := 0
	causeCounts := map[string]int{}
	for _, e := range rows {
		if e.ConclusionApproved || (strings.Contains(strings.ToLower(e.InvestigationStatus), "подтверждено комиссией") && strings.TrimSpace(e.CommissionConclusion) != "") {
			approved++
		}
		cause := strings.TrimSpace(e.SourceClass)
		if cause == "" {
			cause = strings.TrimSpace(e.SourceCause)
		}
		if cause == "" {
			cause = strings.TrimSpace(e.StopCause)
		}
		if cause != "" {
			causeCounts[cause]++
		}
	}
	fmt.Fprintf(&b, "\nСВОДНЫЙ АНАЛИЗ\nПодтверждено комиссией: %d\nПредварительно / без утверждения: %d\n", approved, len(rows)-approved)
	if len(causeCounts) == 0 {
		b.WriteString("Причины по источникам: не указаны\n")
	}
	causes := make([]string, 0, len(causeCounts))
	for cause := range causeCounts {
		causes = append(causes, cause)
	}
	sort.Strings(causes)
	for _, cause := range causes {
		fmt.Fprintf(&b, "Причина / осложнение «%s»: %d событий\n", cause, causeCounts[cause])
	}
	if len(rows) > 1 {
		var dates []time.Time
		for _, e := range rows {
			if d, err := failureDate(e.Date); err == nil {
				dates = append(dates, d)
			}
		}
		sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
		if len(dates) > 1 {
			span := int(dates[len(dates)-1].Sub(dates[0]).Hours() / 24)
			fmt.Fprintf(&b, "Период между первым и последним зарегистрированным отказом: %d сут.\n", span)
		}
	}
	b.WriteString("\nРЕКОМЕНДАЦИИ ПО РЯДУ ОТКАЗОВ\n")
	for _, cause := range causes {
		if causeCounts[cause] > 1 {
			fmt.Fprintf(&b, "Проверить повторяемость причины «%s» (%d событий), сопоставить дефектовку, режим и эффективность мероприятий после каждого ремонта.\n", cause, causeCounts[cause])
		}
	}
	for i, rec := range failureNextActionRecommendations(rows) {
		fmt.Fprintf(&b, "%d. %s\n", i+1, rec)
	}
	b.WriteString("Сопоставить наработку до отказов и следующий цикл после мероприятия; оценивать эффект только при наличии подтверждённых данных.\n")
	for i, e := range rows {
		fmt.Fprintf(&b, "\nСОБЫТИЕ %d — %s\n", i+1, failureReportValue(e.Date))
		appendFailureEventReport(&b, e)
	}
	return b.String()
}
func formatFailureReportText(h syscall.Handle, s string) {
	normal := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, CrTextColor: uint32(rgb(15, 48, 82))}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&normal)))
	offset := 0
	for _, line := range strings.SplitAfter(s, "\n") {
		t := strings.TrimSpace(line)
		// Each field name is a bold blue anchor; its recorded value remains regular blue.
		if colon := strings.Index(t, ":"); colon > 0 && colon < 65 {
			label := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82))}
			pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(offset), uintptr(offset+len(utf16.Encode([]rune(t[:colon+1])))))
			pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&label)))
		}
		if t != "" && (t == strings.ToUpper(t) || strings.HasPrefix(t, "СОБЫТИЕ ")) {
			cf := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(180, 20, 38))}
			if strings.HasPrefix(t, "РОСА") {
				cf.CrTextColor = uint32(rgb(15, 48, 82))
			}
			pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(offset), uintptr(offset+len(utf16.Encode([]rune(t)))))
			pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
		}
		offset += len(utf16.Encode([]rune(line)))
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, 0)
}
func exportFailureReport(format string) {
	filename := "РОСА_" + failureReportName + "_" + time.Now().Format("20060102") + "." + format
	buf := make([]uint16, 32768)
	copy(buf, utf16.Encode([]rune(filename)))
	filters := map[string]string{"xlsx": "Excel (*.xlsx)\x00*.xlsx\x00\x00", "docx": "Word (*.docx)\x00*.docx\x00\x00", "pdf": "PDF (*.pdf)\x00*.pdf\x00\x00"}
	filter := utf16.Encode([]rune(filters[format]))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: hMain, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p(format), LpstrTitle: p("Сохранить отчёт об отказах"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return
	}
	path := syscall.UTF16ToString(buf)
	if !strings.EqualFold(filepath.Ext(path), "."+format) {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + "." + format
	}
	var err error
	switch format {
	case "xlsx":
		err = writeLaboratoryXLSXReport(path, failureReportBody)
	case "docx":
		err = writeLaboratoryDOCXReport(path, failureReportBody)
	case "pdf":
		err = writeLaboratoryPDFReport(path, failureReportBody)
	}
	if err != nil {
		msg("Выгрузка отчёта", err.Error(), MB_OK|MB_ICONWARNING)
	} else {
		msg("Выгрузка отчёта", "Отчёт сохранён:\n"+path, MB_OK|MB_ICONINFORMATION)
	}
}
func failureReportWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		title := modalCreate("STATIC", failureReportTitle, WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 20, 1010, 36, parent, 0)
		if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		_ = riched20.Load()
		failureReportEdit = modalCreate("RichEdit20W", failureReportBody, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 24, 66, 1020, 574, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(failureReportEdit), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		formatFailureReportText(failureReportEdit, failureReportBody)
		modalCreate("BUTTON", "Выгрузить Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 24, 645, 195, 40, parent, failureReportExcel)
		modalCreate("BUTTON", "Выгрузить Word", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 230, 645, 195, 40, parent, failureReportWord)
		modalCreate("BUTTON", "Выгрузить PDF", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 436, 645, 195, 40, parent, failureReportPDF)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case failureReportExcel:
			exportFailureReport("xlsx")
			return 0
		case failureReportWord:
			exportFailureReport("docx")
			return 0
		case failureReportPDF:
			exportFailureReport("pdf")
			return 0
		}
	case labMappingClose:
		failureReportDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
func openFailureReportDialog(title, body, name string) {
	failureReportTitle = title
	failureReportBody = body
	failureReportName = name
	failureReportDone = false
	cls := p("ROSAFailureReportDialogV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(failureReportWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(title))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 1090, 750, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg(title, "Не удалось открыть отчёт.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !failureReportDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &message) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}

func wellFailureHeaderCustomDraw(param uintptr, header syscall.Handle) uintptr {
	draw := (*NMCUSTOMDRAW)(unsafe.Pointer(param))
	switch draw.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		pFillRect.Call(uintptr(draw.HDC), uintptr(unsafe.Pointer(&draw.Rc)), uintptr(brushNav))
		buf := make([]uint16, 256)
		item := HDITEM{Mask: 0x0002, PszText: &buf[0], CchTextMax: int32(len(buf))}
		pSendMessageW.Call(uintptr(header), HDM_FIRST+11, draw.ItemSpec, uintptr(unsafe.Pointer(&item)))
		old, _, _ := pSelectObject.Call(uintptr(draw.HDC), uintptr(hFontHeaderSmall))
		pSetBkMode.Call(uintptr(draw.HDC), TRANSPARENT)
		pSetTextColor.Call(uintptr(draw.HDC), rgb(255, 255, 255))
		r := draw.Rc
		pDrawTextW.Call(uintptr(draw.HDC), uintptr(unsafe.Pointer(&buf[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		pSelectObject.Call(uintptr(draw.HDC), old)
		return 4
	}
	return CDRF_DODEFAULT
}
