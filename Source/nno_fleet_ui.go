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
	ID_NNO_DATE_FROM        = 9817
	ID_NNO_DATE_TO          = 9818
	ID_NNO_DATA             = 9819
	ID_NNO_FILTER           = 9820
	ID_NNO_REFRESH          = 9821
	ID_NNO_RULES            = 9822
	ID_NNO_AUDIT            = 9823
	ID_NNO_OBJECT           = 9824
	ID_NNO_TREND            = 9825
	ID_NNO_BACK             = 9826
	ID_NNO_DISTRIBUTION     = 9827
	ID_NNO_VIEW_SWAP        = 9828
	ID_NNO_TREND_TOGGLE     = 9829
	ID_NNO_GROUP_CARD_FIRST = 9830
	nnoMethodCount          = 6
	ID_NNO_SECTION_FIRST    = 9920
	ID_NNO_STATUS_BAND      = 9924
	ID_NNO_MEAN_VIEW        = 9926
	ID_NNO_MEDIAN_VIEW      = 9927
	ID_NNO_BOTH_VIEW        = 9928
	ID_NNO_SCROLL_UP        = 9929
	ID_NNO_SCROLL_DOWN      = 9930
	ID_NNO_STATS_BAND       = 9931
	ID_NNO_TARGET           = 9932
	ID_NNO_EVENT_PREV       = 9933
	ID_NNO_EVENT_NEXT       = 9934
	ID_NNO_ZOOM_OUT         = 9935
	ID_NNO_ZOOM_RESET       = 9936
	ID_NNO_ZOOM_IN          = 9937
	ID_NNO_ZOOM_DIALOG      = 9938
	nnoEventPageSize        = 200
)

const (
	DTM_FIRST           = 0x1000
	DTM_GETSYSTEMTIME   = DTM_FIRST + 1
	DTM_SETSYSTEMTIME   = DTM_FIRST + 2
	DTM_SETFORMATW      = DTM_FIRST + 50
	DTS_SHORTDATEFORMAT = 0x0000
	DTN_DATETIMECHANGE  = int32(-759)
)

type nnoSystemTime struct {
	Year, Month, DayOfWeek, Day        uint16
	Hour, Minute, Second, Milliseconds uint16
}

func isNNOActionButton(h syscall.Handle) bool {
	return h == nnoFleetUpdate || h == nnoFleetAudit || h == nnoFleetRules || h == nnoFleetObject
}

func isNNOActionButtonID(id int) bool {
	return id == ID_NNO_REFRESH || id == ID_NNO_AUDIT || id == ID_NNO_RULES || id == ID_NNO_OBJECT
}

// The fund dashboard is deliberately separated from the single-well MRP forecast.
// Fleet NNO is NOT derived from the observed failure-cycle means.
var (
	nnoFleetEnabled                                                                                                    = true
	nnoFleetTitle, nnoFleetSubtitle, nnoFleetScope, nnoFleetStatus                                                     syscall.Handle
	nnoFleetPeriodLabel, nnoFleetFilterLabel, nnoFleetDataLabel, nnoFleetDateSeparator                                 syscall.Handle
	nnoFleetFromDate, nnoFleetToDate, nnoFleetFilter, nnoFleetData                                                     syscall.Handle
	nnoFleetUpdate, nnoFleetRules, nnoFleetAudit, nnoFleetObject, nnoFleetBack                                         syscall.Handle
	nnoFleetMeanView, nnoFleetMedianView, nnoFleetBothView, nnoFleetTargetButton, nnoFleetScrollUp, nnoFleetScrollDown syscall.Handle
	nnoFleetZoomOut, nnoFleetZoomReset, nnoFleetZoomIn, nnoFleetZoomDialog                                             syscall.Handle
	nnoFleetMonthTitle, nnoFleetMethodTitle, nnoFleetEventTitle, nnoFleetChartTitle, nnoFleetStatsTitle                syscall.Handle
	nnoFleetMonths, nnoFleetMethods, nnoFleetFundMethods, nnoFleetEvents, nnoFleetChart                                syscall.Handle
	nnoFleetDistribution, nnoFleetViewSwap, nnoFleetTrendToggle, nnoFleetEventPrev, nnoFleetEventNext                  syscall.Handle
	nnoFleetMethodCards                                                                                                [nnoMethodCount]syscall.Handle
	nnoFleetMethodSummaries                                                                                            [nnoMethodCount]nnoFleetMethodSummary
	nnoFleetHandles                                                                                                    []syscall.Handle
	nnoFleetMonthData                                                                                                  []nnoMonth
	nnoFleetCycleSamples                                                                                               []nnoCycleSample
	nnoFleetCycleSummaries                                                                                             [nnoMethodCount]nnoCycleStats
	nnoFleetSeries                                                                                                     [nnoMethodCount][]nnoCycleStats
	nnoFleetShowMethodTable                                                                                            bool
	nnoFleetShowEventTrend                                                                                             bool
	nnoFleetShowMedian                                                                                                 bool
	nnoFleetShowBoth                                                                                                   bool
	nnoFleetScroll                                                                                                     int32
	nnoFleetScrollMax                                                                                                  int32
	nnoFleetScrollRelayout                                                                                             bool
	nnoFleetEventData                                                                                                  []FailureEvent
	nnoFleetEventPage                                                                                                  int
	nnoFleetGroupFilter                                                                                                = "Все способы"
	nnoFleetDateFrom, nnoFleetDateTo                                                                                   time.Time
	nnoFleetDataSourceFilter                                                                                           string
	nnoFleetDataChoices                                                                                                []string
	nnoUpdatingDateControls                                                                                            bool
	nnoFleetCompactScope                                                                                               string
	nnoFleetSyntheticPreview                                                                                           bool
	nnoFleetRollingZoom                                                                                                = 1.0
	nnoFleetDistributionZoom                                                                                           = 1.0
	nnoZoomActiveChart                                                                                                 = 0
	nnoFleetFilterNames                                                                                                = []string{"Все способы", "УЭЦН", "УШГН", "УЭВН", "ОРД", "Фонтанный", "Прочие"}
)

type nnoMonth struct {
	Month                        time.Time
	Recorded, Confirmed, Pending int
	Wells                        map[string]bool
}

type nnoFleetMethodSummary struct {
	Name                         string
	Wells, Registered, Confirmed int
}

func nnoMethodName(i int) string {
	if i < 0 || i >= nnoMethodCount {
		return "Прочие"
	}
	return nnoFleetFilterNames[i+1]
}

func nnoMethodAccent(i int) uintptr {
	// The same method order is used by cards and the chart legend.
	colors := [...]uintptr{rgb(32, 91, 175), rgb(184, 55, 72), rgb(31, 137, 92), rgb(207, 125, 42), rgb(126, 73, 164), rgb(94, 107, 123)}
	if i < 0 || i >= len(colors) {
		return rgb(110, 120, 128)
	}
	return colors[i]
}

func nnoAttach(h syscall.Handle) syscall.Handle {
	nnoFleetHandles = append(nnoFleetHandles, h)
	return h
}

func nnoSetDatePicker(h syscall.Handle, value time.Time) {
	if h == 0 || value.IsZero() {
		return
	}
	st := nnoSystemTime{Year: uint16(value.Year()), Month: uint16(value.Month()), Day: uint16(value.Day())}
	pSendMessageW.Call(uintptr(h), DTM_SETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&st)))
}

func nnoReadDatePicker(h syscall.Handle) (time.Time, bool) {
	if h == 0 {
		return time.Time{}, false
	}
	var st nnoSystemTime
	ok, _, _ := pSendMessageW.Call(uintptr(h), DTM_GETSYSTEMTIME, 0, uintptr(unsafe.Pointer(&st)))
	if ok != 0 || st.Year == 0 || st.Month < 1 || st.Month > 12 || st.Day < 1 || st.Day > 31 {
		return time.Time{}, false
	}
	return time.Date(int(st.Year), time.Month(st.Month), int(st.Day), 0, 0, 0, 0, time.Local), true
}

func nnoSelectedDataSource() string {
	if nnoFleetData == 0 {
		return nnoFleetDataSourceFilter
	}
	idx, _, _ := pSendMessageW.Call(uintptr(nnoFleetData), CB_GETCURSEL, 0, 0)
	if int(idx) < 0 || int(idx) >= len(nnoFleetDataChoices) {
		return nnoFleetDataSourceFilter
	}
	if int(idx) == 0 {
		return ""
	}
	return nnoFleetDataChoices[int(idx)]
}

func nnoRefreshDataChoices() {
	if nnoFleetData == 0 {
		return
	}
	selected := nnoFleetDataSourceFilter
	choices := []string{"Журнал отказов"}
	seen := map[string]bool{}
	for _, e := range failureEvents {
		source := strings.TrimSpace(e.Source)
		if source == "" || strings.Contains(strings.ToLower(source), "учебн") || seen[source] {
			continue
		}
		seen[source] = true
		choices = append(choices, source)
	}
	if selected != "" && !seen[selected] {
		selected = ""
	}
	sort.Strings(choices[1:])
	nnoFleetDataChoices = choices
	pSendMessageW.Call(uintptr(nnoFleetData), CB_RESETCONTENT, 0, 0)
	selectedIndex := 0
	for i, name := range choices {
		pSendMessageW.Call(uintptr(nnoFleetData), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
		if i > 0 && name == selected {
			selectedIndex = i
		}
	}
	pSendMessageW.Call(uintptr(nnoFleetData), CB_SETCURSEL, uintptr(selectedIndex), 0)
	nnoFleetDataSourceFilter = selected
}
func nnoLabel(text string, bold bool) syscall.Handle {
	h := nnoAttach(label(ID_MRP, text, 242, 94, 200, 24))
	if bold && hFontBold != 0 {
		pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(hFontBold), 1)
	}
	return h
}
func nnoSectionBand(id int, caption string) syscall.Handle {
	h := nnoAttach(create("STATIC", caption, WS_CHILD|SS_OWNERDRAW, 242, py(315), 700, 24, hMain, id))
	addPageControl(ID_MRP, h)
	return h
}
func nnoStatusBand(text string) syscall.Handle {
	h := nnoAttach(create("STATIC", text, WS_CHILD|SS_OWNERDRAW, 242, py(315), 700, 28, hMain, ID_NNO_STATUS_BAND))
	addPageControl(ID_MRP, h)
	return h
}
func nnoMethodOfFailure(e FailureEvent) string {
	s := strings.ToUpper(strings.TrimSpace(e.Equipment + " " + e.OperationMode))
	switch {
	case strings.Contains(s, "ОРД") || strings.Contains(s, "ОДНОВРЕМ"):
		return "ОРД"
	case strings.Contains(s, "ЭЦН") || strings.Contains(s, "ESP"):
		return "УЭЦН"
	case strings.Contains(s, "ШГН") || strings.Contains(s, "СКН"):
		return "УШГН"
	case strings.Contains(s, "ЭВН"):
		return "УЭВН"
	case strings.Contains(s, "ФОНТАН"):
		return "Фонтанный"
	default:
		return "Прочие"
	}
}
func nnoEventIsConfirmed(e FailureEvent) bool {
	return !e.Draft && e.ConclusionApproved && normalizeMRPEventType(e.EventType, false) == "отказ"
}
func buildNNOFleetUI() {
	nnoLoadTarget()
	nnoFleetTitle = nnoAttach(pageTitle(ID_MRP, "Наработка на отказ — аналитика фонда", 242, 94, 970, 35))
	nnoFleetSubtitle = nnoLabel("Наработка на отказ по способам эксплуатации", true)
	nnoFleetScope = nnoLabel("", false)
	nnoFleetStatus = nnoStatusBand("ННО-12 не рассчитана · требуются проверенные месячные Ti и утверждённые правила включения отказов.")
	nnoFleetPeriodLabel = nnoLabel("Период анализа:", true)
	nnoFleetFilterLabel = nnoLabel("Способ эксплуатации:", true)
	nnoFleetDataLabel = nnoLabel("Данные:", true)
	nnoFleetDateSeparator = nnoLabel("—", false)
	today := time.Now()
	if nnoFleetDateTo.IsZero() {
		nnoFleetDateTo = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	}
	if nnoFleetDateFrom.IsZero() {
		nnoFleetDateFrom = nnoMonthStart(nnoFleetDateTo).AddDate(0, -11, 0)
	}
	nnoFleetFromDate = nnoAttach(create("SysDateTimePick32", "", WS_CHILD|WS_TABSTOP|DTS_SHORTDATEFORMAT, 242, py(151), 92, 26, hMain, ID_NNO_DATE_FROM))
	addPageControl(ID_MRP, nnoFleetFromDate)
	nnoFleetToDate = nnoAttach(create("SysDateTimePick32", "", WS_CHILD|WS_TABSTOP|DTS_SHORTDATEFORMAT, 242, py(151), 92, 26, hMain, ID_NNO_DATE_TO))
	addPageControl(ID_MRP, nnoFleetToDate)
	for _, dateControl := range []syscall.Handle{nnoFleetFromDate, nnoFleetToDate} {
		pSendMessageW.Call(uintptr(dateControl), DTM_SETFORMATW, 0, uintptr(unsafe.Pointer(p("dd.MM.yyyy"))))
		pSendMessageW.Call(uintptr(dateControl), WM_SETFONT, uintptr(hFont), 1)
	}
	nnoSetDatePicker(nnoFleetFromDate, nnoFleetDateFrom)
	nnoSetDatePicker(nnoFleetToDate, nnoFleetDateTo)
	nnoFleetFilter = nnoAttach(create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 242, py(166), 200, 180, hMain, ID_NNO_FILTER))
	addPageControl(ID_MRP, nnoFleetFilter)
	pSendMessageW.Call(uintptr(nnoFleetFilter), WM_SETFONT, uintptr(hFont), 1)
	for _, name := range nnoFleetFilterNames {
		pSendMessageW.Call(uintptr(nnoFleetFilter), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(name))))
	}
	pSendMessageW.Call(uintptr(nnoFleetFilter), CB_SETCURSEL, 0, 0)
	nnoFleetData = nnoAttach(create("COMBOBOX", "", WS_CHILD|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 242, py(166), 125, 180, hMain, ID_NNO_DATA))
	addPageControl(ID_MRP, nnoFleetData)
	pSendMessageW.Call(uintptr(nnoFleetData), WM_SETFONT, uintptr(hFont), 1)
	pSendMessageW.Call(uintptr(nnoFleetData), CB_SETDROPPEDWIDTH, 300, 0)
	nnoRefreshDataChoices()
	nnoFleetUpdate = nnoAttach(button(ID_MRP, "Обновить фонд", 242, 166, 100, 22, ID_NNO_REFRESH))
	nnoFleetAudit = nnoAttach(button(ID_MRP, "Проверить состав", 242, 166, 113, 22, ID_NNO_AUDIT))
	nnoFleetRules = nnoAttach(button(ID_MRP, "Методика · доступ по паролю", 242, 166, 140, 22, ID_NNO_RULES))
	nnoFleetObject = nnoAttach(button(ID_MRP, "Прогноз объекта ›", 242, 166, 116, 22, ID_NNO_OBJECT))
	nnoFleetBack = nnoAttach(button(ID_MRP, "‹ К аналитике фонда", 242, 166, 195, 42, ID_NNO_BACK))
	for i := range nnoFleetMethodCards {
		h := create("BUTTON", "", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 242, py(216), 200, 66, hMain, ID_NNO_GROUP_CARD_FIRST+i)
		nnoFleetMethodCards[i] = nnoAttach(h)
		addPageControl(ID_MRP, h)
		nnoFleetMethodSummaries[i] = nnoFleetMethodSummary{Name: nnoMethodName(i)}
	}
	nnoFleetChartTitle = nnoSectionBand(ID_NNO_SECTION_FIRST, "НАРАБОТКА · 12 МЕС.")
	nnoFleetMeanView = nnoAttach(button(ID_MRP, "Средняя", 242, 315, 82, 22, ID_NNO_MEAN_VIEW))
	nnoFleetMedianView = nnoAttach(button(ID_MRP, "Медиана", 242, 315, 82, 22, ID_NNO_MEDIAN_VIEW))
	nnoFleetBothView = nnoAttach(button(ID_MRP, "Обе", 242, 315, 60, 22, ID_NNO_BOTH_VIEW))
	nnoFleetTargetButton = nnoAttach(button(ID_MRP, "Ориентир наработки", 242, 315, 168, 22, ID_NNO_TARGET))
	nnoFleetScrollUp = nnoAttach(button(ID_MRP, "▲", 242, 315, 28, 30, ID_NNO_SCROLL_UP))
	nnoFleetScrollDown = nnoAttach(button(ID_MRP, "▼", 242, 315, 28, 30, ID_NNO_SCROLL_DOWN))
	nnoFleetTrendToggle = nnoAttach(button(ID_MRP, "События ›", 242, 315, 102, 22, ID_NNO_TREND_TOGGLE))
	nnoFleetZoomOut = nnoAttach(button(ID_MRP, "−", 242, 315, 26, 22, ID_NNO_ZOOM_OUT))
	nnoFleetZoomReset = nnoAttach(button(ID_MRP, "100%", 242, 315, 54, 22, ID_NNO_ZOOM_RESET))
	nnoFleetZoomIn = nnoAttach(button(ID_MRP, "+", 242, 315, 26, 22, ID_NNO_ZOOM_IN))
	nnoFleetZoomDialog = nnoAttach(button(ID_MRP, "Масштаб…", 242, 315, 112, 28, ID_NNO_ZOOM_DIALOG))
	nnoFleetChart = nnoAttach(create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW, 242, py(315), 700, 200, hMain, ID_NNO_TREND))
	addPageControl(ID_MRP, nnoFleetChart)
	initNNORollingInteraction()
	nnoFleetMethodTitle = nnoSectionBand(ID_NNO_SECTION_FIRST+1, "РАСПРЕДЕЛЕНИЕ НАРАБОТКИ · ПО СПОСОБАМ")
	nnoFleetDistribution = nnoAttach(create("STATIC", "", WS_CHILD|WS_BORDER|SS_OWNERDRAW|SS_NOTIFY, 242, py(315), 450, 190, hMain, ID_NNO_DISTRIBUTION))
	initNNODistributionInteraction()
	addPageControl(ID_MRP, nnoFleetDistribution)
	nnoFleetViewSwap = nnoAttach(button(ID_MRP, "Свод ›", 242, 315, 94, 22, ID_NNO_VIEW_SWAP))
	nnoFleetMethods = nnoAttach(newList(ID_MRP, 242, 315, 450, 190, []struct {
		n string
		w int32
	}{
		{"Способ", 110}, {"n", 48}, {"Мин.", 65}, {"Q1", 65}, {"Медиана", 74}, {"Q3", 65}, {"Макс.", 65}, {"Средняя", 74}, {"Выбросы", 76}, {"Ориентир", 82}, {"Δ ср.", 75}, {"Δ мед.", 75}, {"Интервалы ≥ цели", 134},
	}))
	nnoFleetFundMethods = nnoAttach(newList(ID_MRP, 242, 315, 450, 190, []struct {
		n string
		w int32
	}{
		{"Способ", 182}, {"Скв.", 84}, {"Событ.", 84}, {"Подтв.", 88}, {"ННО-12", 98}, {"Статус Ti", 154},
	}))
	nnoFleetMonthTitle = nnoSectionBand(ID_NNO_SECTION_FIRST+2, "ПОМЕСЯЧНО · СРЕДНЯЯ ПО СПОСОБАМ")
	nnoFleetStatsTitle = nnoSectionBand(ID_NNO_STATS_BAND, "СТАТИСТИЧЕСКИЕ ХАРАКТЕРИСТИКИ НАРАБОТКИ")
	nnoFleetMonths = nnoAttach(newList(ID_MRP, 242, 545, 620, 196, []struct {
		n string
		w int32
	}{
		{"Месяц", 96}, {"Событий", 96}, {"Подтв.", 84}, {"На проверке", 118}, {"Σ Ti, сут.", 110}, {"ННО-12, сут.", 128}, {"Комментарий", 240},
		{"УЭЦН ср.", 95}, {"УШГН ср.", 95}, {"УЭВН ср.", 95}, {"ОРД ср.", 95}, {"Фонт. ср.", 98}, {"Проч. ср.", 95},
		{"УЭЦН мед.", 98}, {"УШГН мед.", 98}, {"УЭВН мед.", 98}, {"ОРД мед.", 98}, {"Фонт. мед.", 101}, {"Проч. мед.", 98},
	}))
	nnoFleetEventTitle = nnoSectionBand(ID_NNO_SECTION_FIRST+3, "ИСХОДНЫЕ СОБЫТИЯ · ДВОЙНОЙ КЛИК")
	nnoFleetEvents = nnoAttach(newList(ID_MRP, 242, 545, 610, 196, []struct {
		n string
		w int32
	}{
		{"Месторождение", 150}, {"Скважина", 112}, {"Способ", 110}, {"Начало", 96}, {"Отказ", 96}, {"Интервал, сут.", 110}, {"Классификация", 145}, {"Цикл", 105}, {"Причина исключения", 160}, {"Источник", 200}, {"Качество", 115}, {"Причина", 126},
	}))
	nnoFleetEventPrev = nnoAttach(button(ID_MRP, "‹ Ранее", 242, 545, 85, 22, ID_NNO_EVENT_PREV))
	nnoFleetEventNext = nnoAttach(button(ID_MRP, "Далее ›", 242, 545, 85, 22, ID_NNO_EVENT_NEXT))
}
func nnoSelectedGroup() string {
	idx, _, _ := pSendMessageW.Call(uintptr(nnoFleetFilter), CB_GETCURSEL, 0, 0)
	if int(idx) >= 0 && int(idx) < len(nnoFleetFilterNames) {
		return nnoFleetFilterNames[int(idx)]
	}
	return "Все способы"
}
func nnoMonthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
func nnoMonthIndex(t, ref time.Time) int {
	return (t.Year()-ref.Year())*12 + int(t.Month()) - int(ref.Month())
}
func nnoSourceData() ([]nnoMonth, []FailureEvent) {
	return nnoSourceDataForGroup(nnoSelectedGroup())
}

func nnoSourceDataForGroup(group string) ([]nnoMonth, []FailureEvent) {
	from, to := nnoFleetDateFrom, nnoFleetDateTo
	if from.IsZero() || to.IsZero() {
		today := time.Now()
		to = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
		from = nnoMonthStart(to).AddDate(0, -11, 0)
	}
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())
	if from.After(to) {
		return nil, nil
	}
	first, last := nnoMonthStart(from), nnoMonthStart(to)
	monthCount := nnoMonthIndex(last, first) + 1
	if monthCount < 1 || monthCount > 1200 {
		return nil, nil
	}
	months := make([]nnoMonth, monthCount)
	for i := range months {
		months[i] = nnoMonth{Month: first.AddDate(0, i, 0), Wells: map[string]bool{}}
	}
	events := make([]FailureEvent, 0)
	seen := make(map[string]bool)
	selectedSource := nnoSelectedDataSource()
	for _, e := range failureEvents {
		// Never mix unfinished or teaching records into production. A missing
		// source class remains visible for audit but is not counted as approved.
		if e.Draft || strings.Contains(strings.ToLower(e.Source), "учебн") {
			continue
		}
		if group != "Все способы" && nnoMethodOfFailure(e) != group {
			continue
		}
		if selectedSource != "" && strings.TrimSpace(e.Source) != selectedSource {
			continue
		}
		d, err := failureDate(e.Date)
		if err != nil {
			continue
		}
		if d.Before(from) || d.After(to) {
			continue
		}
		pos := nnoMonthIndex(nnoMonthStart(d), first)
		if pos < 0 || pos >= len(months) {
			continue
		}
		key := strings.TrimSpace(e.ID)
		if key == "" {
			key = e.ObjectKey + "|" + e.Date + "|" + e.SourceCauseCode
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		m := &months[pos]
		m.Recorded++
		if nnoEventIsConfirmed(e) {
			m.Confirmed++
		} else {
			m.Pending++
		}
		if e.ObjectKey != "" {
			m.Wells[e.ObjectKey] = true
		}
		events = append(events, e)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Date > events[j].Date })
	return months, events
}
func refreshNNOFleetUI() {
	nnoDistributionSelected.Valid = false
	nnoRefreshDataChoices()
	months, events := nnoSourceData()
	_, allGroupEvents := nnoSourceDataForGroup("Все способы")
	nnoFleetMonthData, nnoFleetEventData = months, events
	nnoFleetCycleSamples = nnoVerifiedCycleSamples()
	nnoFleetSyntheticPreview = false
	// Only explicitly labelled teaching rows may enable the preview. The
	// filtered journal can contain DEMO rows: requiring an empty journal hid
	// both training charts even when every visible record was a demo record.
	// The source-scope guard rejects mixed production/teaching data; laboratory
	// fallback still requires an empty production journal.
	trainingOnly := nnoShouldUseSyntheticPreview(failureEvents)
	if len(nnoFleetCycleSamples) == 0 && (trainingOnly || (len(events) == 0 && nnoHasActiveTrainingLabExamples())) {
		nnoFleetCycleSamples = nnoSyntheticCycleSamples(months)
		nnoFleetSyntheticPreview = len(nnoFleetCycleSamples) > 0
	}
	for i := range nnoFleetCycleSummaries {
		nnoFleetCycleSummaries[i] = nnoCycleStatistics(nnoFleetCycleSamples, nnoMethodName(i), nnoFleetDateFrom, nnoFleetDateTo)
		nnoFleetSeries[i] = make([]nnoCycleStats, len(months))
		for j, m := range months {
			nnoFleetSeries[i][j] = nnoRollingCycleStats(nnoFleetCycleSamples, nnoMethodName(i), m.Month)
		}
	}
	recorded, confirmed := 0, 0
	wells := map[string]bool{}
	for _, m := range months {
		recorded += m.Recorded
		confirmed += m.Confirmed
		for k := range m.Wells {
			wells[k] = true
		}
	}
	groupCounts := [nnoMethodCount]nnoFleetMethodSummary{}
	groupWells := [nnoMethodCount]map[string]bool{}
	for i := range groupCounts {
		groupCounts[i].Name = nnoMethodName(i)
		groupWells[i] = map[string]bool{}
	}
	for _, e := range allGroupEvents {
		for i := range groupCounts {
			if groupCounts[i].Name != nnoMethodOfFailure(e) {
				continue
			}
			groupCounts[i].Registered++
			if nnoEventIsConfirmed(e) {
				groupCounts[i].Confirmed++
			}
			if strings.TrimSpace(e.ObjectKey) != "" {
				groupWells[i][e.ObjectKey] = true
			}
			break
		}
	}
	for i := range groupCounts {
		groupCounts[i].Wells = len(groupWells[i])
		nnoFleetMethodSummaries[i] = groupCounts[i]
		if nnoFleetMethodCards[i] != 0 {
			pInvalidateRect.Call(uintptr(nnoFleetMethodCards[i]), 0, 0)
		}
	}
	if len(months) == 0 {
		return
	}
	periodFrom, periodTo := nnoFleetDateFrom, nnoFleetDateTo
	if periodFrom.IsZero() || periodTo.IsZero() {
		periodFrom, periodTo = months[0].Month, months[len(months)-1].Month
	}
	sourceName := "Журнал отказов"
	if selected := nnoSelectedDataSource(); selected != "" {
		sourceName = selected
	}
	setText(nnoFleetScope, fmt.Sprintf("Период: %s — %s · способ: %s · данные: %s", periodFrom.Format("02.01.2006"), periodTo.Format("02.01.2006"), nnoSelectedGroup(), sourceName))
	if nnoFleetSyntheticPreview {
		setText(nnoFleetStatus, fmt.Sprintf("Событий: %d · подтверждено: %d · скважин в истории: %d · визуализация: учебные циклы · ННО-12: не рассчитана", recorded, confirmed, len(wells)))
		nnoFleetCompactScope = fmt.Sprintf("Период: %s — %s · %s · %s · события: %d · подтверждено: %d · скважины в истории: %d · визуализация: учебные циклы", periodFrom.Format("02.01.2006"), periodTo.Format("02.01.2006"), nnoSelectedGroup(), sourceName, recorded, confirmed, len(wells))
	} else {
		setText(nnoFleetStatus, fmt.Sprintf("Событий: %d · подтверждено: %d · скважин в истории: %d · учтённые Ti: нет · методика: не утверждена · ННО-12: не рассчитана", recorded, confirmed, len(wells)))
		nnoFleetCompactScope = fmt.Sprintf("Период: %s — %s · %s · %s · события: %d · подтверждено: %d · скважины в истории: %d · Ti / методика не подтверждены", periodFrom.Format("02.01.2006"), periodTo.Format("02.01.2006"), nnoSelectedGroup(), sourceName, recorded, confirmed, len(wells))
	}
	listBeginUpdate(nnoFleetMonths)
	listClear(nnoFleetMonths)
	for i, m := range months {
		cells := []string{m.Month.Format("01.2006"), fmt.Sprint(m.Recorded), fmt.Sprint(m.Confirmed), fmt.Sprint(m.Pending), "—", "Не рассчитана", "Нужны Ti и правило учёта"}
		for group := 0; group < nnoMethodCount; group++ {
			stats := nnoFleetSeries[group][i]
			if stats.Defined {
				cells = append(cells, fmt.Sprintf("%.0f", stats.Mean))
			} else {
				cells = append(cells, "—")
			}
		}
		for group := 0; group < nnoMethodCount; group++ {
			stats := nnoFleetSeries[group][i]
			if stats.Defined {
				cells = append(cells, fmt.Sprintf("%.0f", stats.Median))
			} else {
				cells = append(cells, "—")
			}
		}
		listAdd(nnoFleetMonths, i, cells)
	}
	listEndUpdate(nnoFleetMonths)
	listBeginUpdate(nnoFleetMethods)
	listClear(nnoFleetMethods)
	idx := 0
	for group := 0; group < nnoMethodCount; group++ {
		name := nnoMethodName(group)
		if nnoSelectedGroup() != "Все способы" && name != nnoSelectedGroup() {
			continue
		}
		stat := nnoFleetCycleSummaries[group]
		cells := []string{name, "—", "—", "—", "—", "—", "—", "—", "—", fmt.Sprintf("%.0f", nnoTarget.Days), "—", "—", "—"}
		if stat.Defined {
			achieved := 0
			for _, sample := range nnoFleetCycleSamples {
				if sample.Group == name && nnoSampleInWindow(sample, nnoFleetDateFrom, nnoFleetDateTo) && sample.Days >= nnoTarget.Days {
					achieved++
				}
			}
			cells = []string{name, fmt.Sprint(stat.Count), fmt.Sprintf("%.1f", stat.Min), fmt.Sprintf("%.1f", stat.Q1), fmt.Sprintf("%.1f", stat.Median), fmt.Sprintf("%.1f", stat.Q3), fmt.Sprintf("%.1f", stat.Max), fmt.Sprintf("%.1f", stat.Mean), fmt.Sprint(stat.Outliers), fmt.Sprintf("%.0f", nnoTarget.Days), fmt.Sprintf("%+.1f", stat.Mean-nnoTarget.Days), fmt.Sprintf("%+.1f", stat.Median-nnoTarget.Days), fmt.Sprintf("%d/%d (%.0f%%)", achieved, stat.Count, float64(achieved)/float64(stat.Count)*100)}
		}
		listAdd(nnoFleetMethods, idx, cells)
		idx++
	}
	listEndUpdate(nnoFleetMethods)
	listBeginUpdate(nnoFleetFundMethods)
	listClear(nnoFleetFundMethods)
	for group := 0; group < nnoMethodCount; group++ {
		name := nnoMethodName(group)
		if nnoSelectedGroup() != "Все способы" && name != nnoSelectedGroup() {
			continue
		}
		summary := nnoFleetMethodSummaries[group]
		listAdd(nnoFleetFundMethods, group, []string{name, fmt.Sprint(summary.Wells), fmt.Sprint(summary.Registered), fmt.Sprint(summary.Confirmed), "—", "Ti не подтверждены"})
	}
	listEndUpdate(nnoFleetFundMethods)
	nnoFleetEventPage = 0
	nnoRenderEventPage()
	if nnoFleetChart != 0 {
		pInvalidateRect.Call(uintptr(nnoFleetChart), 0, 1)
	}
	if nnoFleetDistribution != 0 {
		pInvalidateRect.Call(uintptr(nnoFleetDistribution), 0, 1)
	}
}

func nnoRenderEventPage() {
	count := len(nnoFleetEventData)
	last := 0
	if count > 0 {
		last = (count - 1) / nnoEventPageSize
	}
	if nnoFleetEventPage > last {
		nnoFleetEventPage = last
	}
	start := nnoFleetEventPage * nnoEventPageSize
	end := start + nnoEventPageSize
	if end > count {
		end = count
	}
	listBeginUpdate(nnoFleetEvents)
	listClear(nnoFleetEvents)
	for i := start; i < end; i++ {
		e := nnoFleetEventData[i]
		status := "На проверке"
		if nnoEventIsConfirmed(e) {
			status = "Утверждена"
		}
		include, reason, interval := "Нет", "Не утверждён отказ", "—"
		if nnoEventIsConfirmed(e) {
			include, reason = "Да", "—"
		}
		startDate, startErr := failureDate(e.CycleStartDate)
		end, endErr := failureDate(e.Date)
		if startErr == nil && endErr == nil && end.After(startDate) {
			interval = fmt.Sprintf("%.1f", end.Sub(startDate).Hours()/24)
		} else if include == "Да" {
			include = "Нет"
			reason = "Нет полного интервала"
		}
		code := strings.TrimSpace(e.SourceCauseCode)
		if code == "" {
			code = "—"
		}
		listAdd(nnoFleetEvents, i-start, []string{dash(e.Field), dash(e.Well), nnoMethodOfFailure(e), dash(e.CycleStartDate), e.Date, interval, dash(e.NormalizedClass), include, reason, dash(e.Source), status, code})
	}
	listEndUpdate(nnoFleetEvents)
	if count == 0 {
		setText(nnoFleetEventTitle, "ДЕТАЛИЗАЦИЯ ПО СКВАЖИНАМ И ОТКАЗАМ · 0 ЗАПИСЕЙ")
	} else {
		setText(nnoFleetEventTitle, fmt.Sprintf("ДЕТАЛИЗАЦИЯ ПО СКВАЖИНАМ И ОТКАЗАМ · %d–%d ИЗ %d", start+1, end, count))
	}
}

func drawNNOFleetMethodCard(di *DRAWITEMSTRUCT) bool {
	if di == nil || int(di.CtlID) < ID_NNO_GROUP_CARD_FIRST || int(di.CtlID) >= ID_NNO_GROUP_CARD_FIRST+nnoMethodCount {
		return false
	}
	i := int(di.CtlID) - ID_NNO_GROUP_CARD_FIRST
	bounds := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(brushWhite))
	frame := brushBorder
	if nnoSelectedGroup() == nnoMethodName(i) {
		frame = brushRedPressed
	}
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&bounds)), uintptr(frame))
	band := bounds
	band.Left++
	band.Top++
	band.Right--
	band.Bottom = bounds.Top + 25
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&band)), uintptr(brushLight))
	accent := bounds
	accent.Right = accent.Left + 4
	accent.Top++
	accent.Bottom--
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(nnoMethodAccent(i)))

	text := func(value string, top, bottom int32, font syscall.Handle, color uintptr) {
		r := bounds
		r.Left += 11
		r.Right -= 8
		r.Top += top
		r.Bottom = bounds.Top + bottom
		pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
		pSetTextColor.Call(uintptr(di.HDC), color)
		old := uintptr(0)
		if font != 0 {
			old, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		}
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(value))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	summary := nnoFleetMethodSummaries[i]
	cardName := summary.Name
	if i == nnoMethodCount-1 {
		cardName = "Прочие / не опр."
	}
	height := bounds.Bottom - bounds.Top
	nameBottom := int32(27)
	if height < 100 {
		nameBottom = 24
	}
	text(cardName, 5, nameBottom, hFontBold, nnoMethodAccent(i))
	// Preserve the approved vertical metric rows: each label and value has room
	// to read without colliding with its neighbour on the six method cards.
	row := func(label, value string, top, bottom int32, labelColor, valueColor uintptr) {
		labelRect := RECT{Left: bounds.Left + 11, Top: bounds.Top + top, Right: bounds.Left + 88, Bottom: bounds.Top + bottom}
		valueRect := RECT{Left: bounds.Left + 91, Top: bounds.Top + top, Right: bounds.Right - 8, Bottom: bounds.Top + bottom}
		pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
		pSetTextColor.Call(uintptr(di.HDC), labelColor)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(label))), ^uintptr(0), uintptr(unsafe.Pointer(&labelRect)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
		valueFont := hFontHeaderSmall
		if valueFont == 0 {
			valueFont = hFontBold
		}
		old, _, _ = pSelectObject.Call(uintptr(di.HDC), uintptr(valueFont))
		pSetTextColor.Call(uintptr(di.HDC), valueColor)
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(value))), ^uintptr(0), uintptr(unsafe.Pointer(&valueRect)), DT_SINGLELINE|DT_VCENTER|DT_NOPREFIX|0x8000)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
	}
	rowH := int32(21)
	if height < 100 {
		rowH = 19
	}
	if height < 84 {
		rowH = 17
	}
	meanText, medianText := "Нет циклов", "Нет циклов"
	if stat := nnoFleetCycleSummaries[i]; stat.Defined {
		meanText = fmt.Sprintf("%.0f сут.", stat.Mean)
		medianText = fmt.Sprintf("%.0f сут.", stat.Median)
	}
	if bounds.Right-bounds.Left < 175 {
		text("Ср. "+meanText, 25, 45, hFontButtonCompact, rgb(15, 48, 82))
		text("Мед. "+medianText, 46, 66, hFontButtonCompact, rgb(15, 48, 82))
		return true
	}
	row("Средняя", meanText, nameBottom, nameBottom+rowH, rgb(172, 37, 55), rgb(15, 48, 82))
	divider := RECT{Left: bounds.Left + 11, Top: bounds.Top + nameBottom + rowH, Right: bounds.Right - 8, Bottom: bounds.Top + nameBottom + rowH + 1}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&divider)), uintptr(brushBorder))
	row("Медиана", medianText, nameBottom+rowH+1, nameBottom+rowH*2, rgb(172, 37, 55), rgb(15, 48, 82))
	if height >= 112 {
		text("Циклы · не ННО-12", 70, 87, hFontButtonCompact, rgb(76, 94, 114))
		text(fmt.Sprintf("История: %d скв. · подтв. %d/%d", summary.Wells, summary.Confirmed, summary.Registered), 89, 111, hFontButtonCompact, rgb(76, 94, 114))
	} else if height >= 92 {
		text(fmt.Sprintf("История: %d скв. · %d/%d подтв.", summary.Wells, summary.Confirmed, summary.Registered), 66, height-3, hFontButtonCompact, rgb(76, 94, 114))
	}
	return true
}

func drawNNOFleetBand(di *DRAWITEMSTRUCT) bool {
	if di == nil || !((int(di.CtlID) >= ID_NNO_SECTION_FIRST && int(di.CtlID) < ID_NNO_SECTION_FIRST+4) || int(di.CtlID) == ID_NNO_STATUS_BAND || int(di.CtlID) == ID_NNO_STATS_BAND) {
		return false
	}
	r := di.RcItem
	fillBrush := brushLight
	frameBrush := brushBorder
	accentBrush := brushGraphite
	textColor := rgb(31, 67, 101)
	font := hFontHeaderSmall
	if int(di.CtlID) == ID_NNO_STATUS_BAND {
		accentBrush = brushBorder
		textColor = rgb(54, 79, 103)
		font = hFontButtonCompact
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), uintptr(fillBrush))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), uintptr(frameBrush))
	accent := r
	accent.Right = accent.Left + 4
	accent.Top++
	accent.Bottom--
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(accentBrush))
	text := getText(di.HwndItem)
	textRect := r
	textRect.Left += 12
	textRect.Right -= 8
	textRect.Top += 1
	textRect.Bottom -= 1
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), textColor)
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	flags := uintptr(DT_VCENTER | DT_NOPREFIX | 0x8000)
	flags |= DT_SINGLELINE
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(text))), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), flags)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return true
}

func drawNNOFleetMetricButton(di *DRAWITEMSTRUCT) bool {
	if di == nil || (int(di.CtlID) != ID_NNO_MEAN_VIEW && int(di.CtlID) != ID_NNO_MEDIAN_VIEW && int(di.CtlID) != ID_NNO_BOTH_VIEW) {
		return false
	}
	selected := (int(di.CtlID) == ID_NNO_BOTH_VIEW && nnoFleetShowBoth) || (!nnoFleetShowBoth && (int(di.CtlID) == ID_NNO_MEDIAN_VIEW) == nnoFleetShowMedian && int(di.CtlID) != ID_NNO_BOTH_VIEW)
	fill, color := brushLight, rgb(26, 61, 98)
	if selected {
		fill, color = brushWarning, rgb(172, 37, 55)
	}
	r := di.RcItem
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), uintptr(fill))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&r)), uintptr(brushBorder))
	if selected {
		accent := r
		accent.Right = accent.Left + 3
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&accent)), uintptr(brushRedPressed))
	}
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), color)
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontButtonCompact))
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(getText(di.HwndItem)))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_SINGLELINE|DT_CENTER|DT_VCENTER|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return true
}

func nnoOldHandles() []syscall.Handle {
	hs := []syscall.Handle{mrpPageTitle, mrpIntroLabel, mrpObjectLabel, mrpEvidenceLabel, mrpScenarioBandTitle, mrpCurrentHeader, mrpBaseHeader, mrpTargetHeader, mrpRangeHeader, mrpCurrentLabel, mrpBaseLabel, mrpTargetStatusLabel, mrpRangeLabel, mrpHistoryList, mrpMethodsList, mrpHistoryHeader, mrpMethodsHeader, mrpDetailHeader, mrpDetail, mrpDetailButton, mrpCalculateButton, mrpGuideButton, mrpDrillButton, mrpRollingHeader, mrpRollingLabel, mrpRollingBackground, mrpTargetLabel, mrpTargetEdit, mrpTargetScopeLabel, mrpTargetScopeCombo, mrpTargetSourceLabel, mrpCauseSummaryLabel, mrpTargetSaveButton, mrpTargetResetButton}
	hs = append(hs, mrpMetricBackgrounds[:]...)
	return hs
}
func setNNOFleetVisibility(visible bool) {
	for _, h := range nnoFleetHandles {
		setMRPLayoutVisibility(h, visible)
	}
}

// Return section sizes without moving existing HWNDs or reallocating lists.
// The content may exceed a small window, in which case the existing native
// dashboard scroller exposes the complete rows without overlapping bands.
func nnoFleetBandHeights(h, viewportTop int32) (chart, middle, detail int32) {
	available := h - viewportTop - 12
	const overhead = int32(121) // title/spacing bands of the two tables and plots
	middle, detail = 118, 146   // header plus >=3 native rows in each ListView
	chart = max32(185, min32(240, available-overhead-middle-detail))
	if h < 780 {
		chart = 185
	}
	// Taller windows first grow the drill-down history, not empty graphs.
	extra := max32(0, available-overhead-chart-middle-detail)
	detail += min32(100, extra)
	extra -= min32(100, extra)
	middle += min32(56, extra)
	return
}

var nnoFleetToolbarCaptionMode = -1
var nnoFleetToolbarFullCaption bool

func layoutNNOFleetUI(w, h int32) {
	beginMRPAtomicLayout()
	defer endMRPAtomicLayout()
	for _, old := range nnoOldHandles() {
		setMRPLayoutVisibility(old, false)
	}
	if !nnoFleetScrollRelayout {
		setNNOFleetVisibility(true)
	}
	x, cw := int32(242), max32(720, w-270)
	setPos(nnoFleetTitle, x, py(94), cw, 40)
	setMRPLayoutVisibility(nnoFleetSubtitle, false)
	setMRPLayoutVisibility(nnoFleetScope, false)
	compact := h < 820
	labelY, controlY := py(138), py(165)
	// A single, fully occupied rail: each native control receives a measured
	// share of the entire content width. No re-creation or handler changes.
	a := nnoToolbarArrange(x, cw)
	boxes := a.Cells
	dateGap := a.Gaps[0]
	setPos(nnoFleetPeriodLabel, boxes[0].X, labelY, boxes[1].X+boxes[1].W-boxes[0].X, 22)
	setPos(nnoFleetFromDate, boxes[0].X, controlY, boxes[0].W, 34)
	setPos(nnoFleetDateSeparator, boxes[0].X+boxes[0].W, controlY+8, dateGap, 20)
	setPos(nnoFleetToDate, boxes[1].X, controlY, boxes[1].W, 34)
	setPos(nnoFleetFilterLabel, boxes[2].X, labelY, boxes[2].W, 22)
	setPos(nnoFleetFilter, boxes[2].X, controlY, boxes[2].W, 258)
	setPos(nnoFleetDataLabel, boxes[3].X, labelY, boxes[3].W, 22)
	setPos(nnoFleetData, boxes[3].X, controlY, boxes[3].W, 258)
	setPos(nnoFleetUpdate, boxes[4].X, controlY, boxes[4].W, 34)
	setPos(nnoFleetAudit, boxes[5].X, controlY, boxes[5].W, 34)
	setPos(nnoFleetRules, boxes[6].X, controlY, boxes[6].W, 34)
	setPos(nnoFleetObject, boxes[7].X, controlY, boxes[7].W, 34)
	setPos(nnoFleetBack, boxes[7].X, controlY, boxes[7].W, 34)
	setMRPLayoutVisibility(nnoFleetBack, false)
	// Full captions on standard/wide monitors. Only genuinely narrow windows
	// receive shorter but unambiguous captions instead of truncated words.
	if nnoFleetToolbarCaptionMode != a.Mode || (a.Mode == 2 && nnoFleetToolbarFullCaption != (boxes[6].W >= 252)) {
		nnoFleetToolbarFullCaption = boxes[6].W >= 252
		// Use full-size text on wide monitors, with a modest font reduction
		// where native date arrows leave less room for the date value.
		inputFont := hFont
		if a.Mode < 2 {
			inputFont = hFontHeaderSmall
		}
		for _, input := range []syscall.Handle{nnoFleetFromDate, nnoFleetToDate, nnoFleetFilter, nnoFleetData} {
			pSendMessageW.Call(uintptr(input), WM_SETFONT, uintptr(inputFont), 1)
		}
		if a.Mode < 2 {
			pSendMessageW.Call(uintptr(nnoFleetFilterLabel), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		} else {
			pSendMessageW.Call(uintptr(nnoFleetFilterLabel), WM_SETFONT, uintptr(hFontBold), 1)
		}
		nnoFleetToolbarCaptionMode = a.Mode
		switch a.Mode {
		case 0:
			setText(nnoFleetFilterLabel, "Способ:")
			setText(nnoFleetRules, "Методика ›")
			setText(nnoFleetObject, "Прогноз ›")
		case 1:
			setText(nnoFleetFilterLabel, "Способ эксплуатации:")
			setText(nnoFleetRules, "Методика (пароль)")
			setText(nnoFleetObject, "Прогноз объекта ›")
		default:
			setText(nnoFleetFilterLabel, "Способ эксплуатации:")
			if boxes[6].W >= 252 {
				setText(nnoFleetRules, "Методика · доступ по паролю")
			} else {
				setText(nnoFleetRules, "Методика (пароль)")
			}
			setText(nnoFleetObject, "Прогноз объекта ›")
		}
	}
	actionBottom := controlY + 34
	cardY := actionBottom + 8
	columns, rows := int32(nnoMethodCount), int32(1)
	cardGap, cardH := int32(6), int32(96)
	if compact {
		cardH = 68
	}
	cardW := (cw - (columns-1)*cardGap) / columns
	for i, control := range nnoFleetMethodCards {
		row, col := int32(i)/columns, int32(i)%columns
		cy := cardY + row*(cardH+cardGap)
		cx := x + col*(cardW+cardGap)
		actualW := cardW
		if col == columns-1 {
			actualW = x + cw - cx
		}
		setPos(control, cx, cy, actualW, cardH)
	}
	cardsBottom := cardY + rows*cardH + (rows-1)*cardGap
	setMRPLayoutVisibility(nnoFleetStatus, true)
	setText(nnoFleetScope, nnoFleetCompactScope)
	statusY := cardsBottom + 5
	setPos(nnoFleetStatus, x, statusY, cw, 28)
	// Keep the controls above the fold. The lower bands scroll in fixed steps;
	// moving an HWND above the band boundary never covers the toolbar/header.
	viewportTop := statusY + 34
	contentW := cw - 36
	leftW := (contentW - 12) / 2
	rightW := contentW - 12 - leftW
	// Three separate independent viewports.  Reserve their minimum sizes
	// before assigning spare client height to the two charts. On an 864 px
	// screen the event table must no longer collapse to a single visible row.
	chartH, tableH, eventH := nnoFleetBandHeights(h, viewportTop)
	// The actual lower ListView bottom is +55 from eventY, not +24.
	contentEnd := viewportTop + 26 + chartH + 8 + 24 + tableH + 8 + 55 + eventH
	nnoFleetScrollMax = max32(0, contentEnd-(h-12))
	nnoFleetScroll = max32(0, min32(nnoFleetScroll, nnoFleetScrollMax))
	bandY := viewportTop - nnoFleetScroll
	chartY := bandY + 26
	tableY := chartY + chartH + 8
	eventY := tableY + 24 + tableH + 8
	setPos(nnoFleetScrollUp, x+cw-31, viewportTop, 29, 30)
	setPos(nnoFleetScrollDown, x+cw-31, viewportTop+34, 29, 30)
	setMRPLayoutVisibility(nnoFleetScrollUp, nnoFleetScroll > 0)
	setMRPLayoutVisibility(nnoFleetScrollDown, nnoFleetScroll < nnoFleetScrollMax)
	setPos(nnoFleetMeanView, x, viewportTop, 70, 28)
	setPos(nnoFleetMedianView, x+74, viewportTop, 74, 28)
	setPos(nnoFleetBothView, x+152, viewportTop, 56, 28)
	setPos(nnoFleetTrendToggle, x+212, viewportTop, 92, 28)
	setPos(nnoFleetTargetButton, x+308, viewportTop, 146, 28)
	setText(nnoFleetTargetButton, "Ориентир")
	setPos(nnoFleetZoomOut, x+458, viewportTop, 28, 28)
	setPos(nnoFleetZoomReset, x+489, viewportTop, 55, 28)
	setPos(nnoFleetZoomIn, x+547, viewportTop, 28, 28)
	setMRPLayoutVisibility(nnoFleetTargetButton, true)
	setMRPLayoutVisibility(nnoFleetZoomOut, true)
	setMRPLayoutVisibility(nnoFleetZoomReset, true)
	setMRPLayoutVisibility(nnoFleetZoomIn, true)
	setMRPLayoutVisibility(nnoFleetMeanView, !nnoFleetShowEventTrend)
	setMRPLayoutVisibility(nnoFleetMedianView, !nnoFleetShowEventTrend)
	setMRPLayoutVisibility(nnoFleetBothView, !nnoFleetShowEventTrend)
	setMRPLayoutVisibility(nnoFleetTrendToggle, true)
	setPos(nnoFleetZoomDialog, x+580, viewportTop, 105, 28)
	setMRPLayoutVisibility(nnoFleetZoomDialog, true)
	setPos(nnoFleetViewSwap, x+690, viewportTop, 104, 28)
	// All existing controls remain available on narrow windows.
	// The scroller begins at cw-31; the last action ends at x+682.
	if cw < 827 {
		setPos(nnoFleetMeanView, x, viewportTop, 64, 28)
		setPos(nnoFleetMedianView, x+67, viewportTop, 72, 28)
		setPos(nnoFleetBothView, x+142, viewportTop, 42, 28)
		setPos(nnoFleetTrendToggle, x+187, viewportTop, 72, 28)
		setPos(nnoFleetTargetButton, x+262, viewportTop, 90, 28)
		setText(nnoFleetTargetButton, "Цель")
		setPos(nnoFleetZoomOut, x+355, viewportTop, 28, 28)
		setPos(nnoFleetZoomReset, x+386, viewportTop, 54, 28)
		setPos(nnoFleetZoomIn, x+443, viewportTop, 28, 28)
		setPos(nnoFleetZoomDialog, x+474, viewportTop, 94, 28)
		setPos(nnoFleetViewSwap, x+571, viewportTop, 111, 28)
	}
	setMRPLayoutVisibility(nnoFleetViewSwap, true)
	if nnoFleetShowMethodTable {
		setText(nnoFleetViewSwap, "Статистика ›")
	} else {
		setText(nnoFleetViewSwap, "Свод фонда ›")
	}
	setText(nnoFleetZoomReset, nnoZoomCaption())
	if nnoFleetShowEventTrend {
		setText(nnoFleetTrendToggle, "Наработка ›")
	} else {
		setText(nnoFleetTrendToggle, "События ›")
	}
	setText(nnoFleetChartTitle, "ДИНАМИКА НАРАБОТКИ · 12 МЕС.")
	setText(nnoFleetMethodTitle, "РАСПРЕДЕЛЕНИЕ · ПО СПОСОБАМ")
	setText(nnoFleetMonthTitle, "ПОМЕСЯЧНАЯ ДИНАМИКА · СРЕДНЯЯ / МЕДИАНА")
	setPos(nnoFleetChartTitle, x, bandY+29, leftW, 24)
	setPos(nnoFleetMethodTitle, x+leftW+12, bandY+29, rightW, 24)
	setPos(nnoFleetChart, x, chartY+29, leftW, chartH)
	setPos(nnoFleetDistribution, x+leftW+12, chartY+29, rightW, chartH)
	setPos(nnoFleetMonthTitle, x, tableY+29, leftW, 24)
	setPos(nnoFleetStatsTitle, x+leftW+12, tableY+29, rightW, 24)
	setPos(nnoFleetMethods, x+leftW+12, tableY+55, rightW, tableH)
	setPos(nnoFleetFundMethods, x+leftW+12, tableY+55, rightW, tableH)
	setPos(nnoFleetMonths, x, tableY+55, leftW, tableH)
	setPos(nnoFleetEventTitle, x, eventY+29, contentW, 24)
	setPos(nnoFleetEventPrev, x+contentW-182, eventY+30, 85, 22)
	setPos(nnoFleetEventNext, x+contentW-92, eventY+30, 85, 22)
	setPos(nnoFleetEvents, x, eventY+55, contentW, eventH)
	for _, item := range []struct {
		h syscall.Handle
		y int32
	}{
		{nnoFleetChartTitle, bandY + 29}, {nnoFleetMethodTitle, bandY + 29}, {nnoFleetChart, chartY + 29}, {nnoFleetDistribution, chartY + 29},
		{nnoFleetMonthTitle, tableY + 29}, {nnoFleetStatsTitle, tableY + 29}, {nnoFleetMethods, tableY + 55}, {nnoFleetFundMethods, tableY + 55}, {nnoFleetMonths, tableY + 55}, {nnoFleetEventTitle, eventY + 29}, {nnoFleetEvents, eventY + 55},
		{nnoFleetEventPrev, eventY + 30}, {nnoFleetEventNext, eventY + 30},
	} {
		visible := item.y >= viewportTop+25 && item.y < h-15
		if item.h == nnoFleetMethods {
			visible = visible && !nnoFleetShowMethodTable
		}
		if item.h == nnoFleetFundMethods {
			visible = visible && nnoFleetShowMethodTable
		}
		if item.h == nnoFleetEventPrev {
			visible = visible && nnoFleetEventPage > 0
		}
		if item.h == nnoFleetEventNext {
			visible = visible && (nnoFleetEventPage+1)*nnoEventPageSize < len(nnoFleetEventData)
		}
		setMRPLayoutVisibility(item.h, visible)
	}
	// Consume spare client width in the final real column. Otherwise native
	// ListViews show a misleading empty strip that looks like an extra column.
	layoutNNOFleetColumns(nnoFleetMethods, rightW, true, []int32{110, 48, 65, 65, 74, 65, 65, 74, 76, 82, 75, 75, 134})
	layoutNNOFleetColumns(nnoFleetFundMethods, rightW, true, []int32{182, 84, 84, 88, 98, 154})
	layoutNNOFleetColumns(nnoFleetMonths, leftW, true, []int32{96, 96, 84, 118, 110, 128, 240, 95, 95, 95, 95, 98, 95, 98, 98, 98, 98, 101, 98})
	for col := 7; col < 19; col++ {
		if (col >= 13) != nnoFleetShowMedian {
			pSendMessageW.Call(uintptr(nnoFleetMonths), LVM_SETCOLUMNWIDTH, uintptr(col), 0)
		}
	}
	layoutNNOFleetColumns(nnoFleetEvents, contentW, true, []int32{150, 112, 110, 96, 96, 110, 145, 105, 160, 200, 115, 126})
}

func layoutNNOFleetColumns(list syscall.Handle, total int32, hasVerticalScroll bool, widths []int32) {
	if list == 0 || len(widths) == 0 {
		return
	}
	available := max32(0, total-3) // reserve only the list border unless a scrollbar is present
	if hasVerticalScroll {
		available = max32(0, total-20)
	}
	var used int32
	for _, width := range widths {
		used += width
	}
	if used < available {
		widths = append([]int32(nil), widths...)
		widths[len(widths)-1] += available - used
	}
	for i, width := range widths {
		setListColumnWidth(list, i, width)
	}
}

func nnoFleetList(h syscall.Handle) bool {
	return h != 0 && (h == nnoFleetMethods || h == nnoFleetFundMethods || h == nnoFleetMonths || h == nnoFleetEvents)
}

func nnoFleetHeaderCustomDraw(param uintptr, header syscall.Handle) uintptr {
	if param == 0 || header == 0 {
		return CDRF_DODEFAULT
	}
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
		r.Left += 6
		r.Right -= 6
		pDrawTextW.Call(uintptr(draw.HDC), uintptr(unsafe.Pointer(&buf[0])), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX|0x8000)
		if old != 0 {
			pSelectObject.Call(uintptr(draw.HDC), old)
		}
		return 4 // CDRF_SKIPDEFAULT
	}
	return CDRF_DODEFAULT
}

func nnoFleetListCustomDraw(param uintptr) uintptr {
	if param == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(param))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		if draw.NMCD.ItemState&0x0001 != 0 {
			return CDRF_DODEFAULT
		}
		row := int(draw.NMCD.ItemSpec)
		draw.ClrText = uint32(rgb(15, 48, 82))
		draw.ClrTextBk = uint32(rgb(255, 255, 255))
		if row%2 == 1 {
			draw.ClrTextBk = uint32(rgb(246, 248, 251))
		}
		if (draw.NMCD.Hdr.HwndFrom == nnoFleetMethods || draw.NMCD.Hdr.HwndFrom == nnoFleetFundMethods) && draw.SubItem == 0 && row >= 0 && row < nnoMethodCount {
			group := row
			if selected := nnoSelectedGroup(); selected != "Все способы" {
				for i := 0; i < nnoMethodCount; i++ {
					if nnoMethodName(i) == selected {
						group = i
						break
					}
				}
			}
			draw.ClrText = uint32(nnoMethodAccent(group))
		}
		return CDRF_NEWFONT
	}
	return CDRF_DODEFAULT
}
func handleNNOFleetCommand(id, code int) bool {
	if code == 1 && id == ID_NNO_FILTER {
		nnoFleetGroupFilter = nnoSelectedGroup()
		refreshNNOFleetUI()
		return true
	}
	if code == 1 && id == ID_NNO_DATA {
		nnoFleetDataSourceFilter = nnoSelectedDataSource()
		refreshNNOFleetUI()
		return true
	}
	if code != 0 && !(id >= ID_NNO_GROUP_CARD_FIRST && id < ID_NNO_GROUP_CARD_FIRST+nnoMethodCount) {
		return false
	}
	if id >= ID_NNO_GROUP_CARD_FIRST && id < ID_NNO_GROUP_CARD_FIRST+nnoMethodCount {
		groupIndex := id - ID_NNO_GROUP_CARD_FIRST
		target := groupIndex + 1
		if nnoSelectedGroup() == nnoMethodName(groupIndex) {
			target = 0
		}
		pSendMessageW.Call(uintptr(nnoFleetFilter), CB_SETCURSEL, uintptr(target), 0)
		nnoFleetGroupFilter = nnoSelectedGroup()
		refreshNNOFleetUI()
		return true
	}
	switch id {
	case ID_NNO_REFRESH:
		refreshNNOFleetUI()
		return true
	case ID_NNO_FILTER, ID_NNO_DATA:
		return true
	case ID_NNO_TREND_TOGGLE:
		nnoFleetShowEventTrend = !nnoFleetShowEventTrend
		layoutMRPForecastCurrentClient()
		return true
	case ID_NNO_MEAN_VIEW, ID_NNO_MEDIAN_VIEW, ID_NNO_BOTH_VIEW:
		nnoFleetShowBoth = id == ID_NNO_BOTH_VIEW
		nnoFleetShowMedian = id == ID_NNO_MEDIAN_VIEW
		nnoDistributionSelected.Valid = false
		layoutMRPForecastCurrentClient()
		pInvalidateRect.Call(uintptr(nnoFleetChart), 0, 0)
		pInvalidateRect.Call(uintptr(nnoFleetMeanView), 0, 0)
		pInvalidateRect.Call(uintptr(nnoFleetMedianView), 0, 0)
		pInvalidateRect.Call(uintptr(nnoFleetBothView), 0, 0)
		return true
	case ID_NNO_SCROLL_UP, ID_NNO_SCROLL_DOWN:
		if id == ID_NNO_SCROLL_UP {
			nnoScrollBy(-140)
		} else {
			nnoScrollBy(140)
		}
		return true
	case ID_NNO_TARGET:
		nnoOpenTarget()
		return true
	case ID_NNO_ZOOM_DIALOG:
		nnoOpenZoomDialog()
		return true
	case ID_NNO_ZOOM_OUT, ID_NNO_ZOOM_RESET, ID_NNO_ZOOM_IN:
		switch id {
		case ID_NNO_ZOOM_OUT:
			nnoSetChartZoom(nnoZoomActiveChart, nnoCurrentZoom()/1.25)
		case ID_NNO_ZOOM_IN:
			nnoSetChartZoom(nnoZoomActiveChart, nnoCurrentZoom()*1.25)
		default:
			nnoSetChartZoom(nnoZoomActiveChart, 1)
		}
		nnoZoomRepaint()
		return true
	case ID_NNO_EVENT_PREV, ID_NNO_EVENT_NEXT:
		if id == ID_NNO_EVENT_PREV && nnoFleetEventPage > 0 {
			nnoFleetEventPage--
		}
		if id == ID_NNO_EVENT_NEXT && (nnoFleetEventPage+1)*nnoEventPageSize < len(nnoFleetEventData) {
			nnoFleetEventPage++
		}
		nnoRenderEventPage()
		nnoFleetScrollRelayout = true
		layoutMRPForecastCurrentClient()
		nnoFleetScrollRelayout = false
		return true
	case ID_NNO_VIEW_SWAP:
		nnoFleetShowMethodTable = !nnoFleetShowMethodTable
		layoutMRPForecastCurrentClient()
		return true
	case ID_NNO_AUDIT:
		nnoOpenAudit()
		return true
	case ID_NNO_RULES:
		openNNOMethodologyEditor()
		return true
	case ID_NNO_OBJECT:
		beginPageVisualUpdate(ID_MRP)
		defer endPageSubviewUpdate(ID_MRP)
		nnoFleetEnabled = false
		setPageSubview(ID_MRP, pageSubviewMRPObject)
		refreshMRPForecastPage()
		layoutMRPForecastCurrentClient()
		return true
	case ID_NNO_BACK:
		beginPageVisualUpdate(ID_MRP)
		defer endPageSubviewUpdate(ID_MRP)
		nnoFleetEnabled = true
		setPageSubview(ID_MRP, pageSubviewMRPFleet)
		refreshMRPForecastPage()
		layoutMRPForecastCurrentClient()
		return true
	}
	return false
}
func layoutMRPForecastCurrentClient() {
	var r RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&r)))
	layoutMRPForecast(r.Right-r.Left, r.Bottom-r.Top)
}
func nnoScrollBy(delta int32) {
	next := max32(0, min32(nnoFleetScroll+delta, nnoFleetScrollMax))
	if next == nnoFleetScroll {
		return
	}
	nnoFleetScroll = next
	nnoFleetScrollRelayout = true
	layoutMRPForecastCurrentClient()
	nnoFleetScrollRelayout = false
}
func handleNNOFleetNotify(lparam uintptr) bool {
	hdr := (*NMHDR)(unsafe.Pointer(lparam))
	if hdr == nil || currentPage != ID_MRP || !nnoFleetEnabled {
		return false
	}
	if hdr.Code == DTN_DATETIMECHANGE && (hdr.HwndFrom == nnoFleetFromDate || hdr.HwndFrom == nnoFleetToDate) {
		if nnoUpdatingDateControls {
			return true
		}
		value, ok := nnoReadDatePicker(hdr.HwndFrom)
		if !ok {
			return true
		}
		nnoUpdatingDateControls = true
		if hdr.HwndFrom == nnoFleetFromDate {
			nnoFleetDateFrom = value
			if nnoFleetDateFrom.After(nnoFleetDateTo) {
				nnoFleetDateTo = nnoFleetDateFrom
				nnoSetDatePicker(nnoFleetToDate, nnoFleetDateTo)
			}
		} else {
			nnoFleetDateTo = value
			if nnoFleetDateTo.Before(nnoFleetDateFrom) {
				nnoFleetDateFrom = nnoFleetDateTo
				nnoSetDatePicker(nnoFleetFromDate, nnoFleetDateFrom)
			}
		}
		nnoUpdatingDateControls = false
		refreshNNOFleetUI()
		return true
	}
	if hdr.Code == NM_DBLCLK && hdr.HwndFrom == nnoFleetEvents {
		selected := selectedRow(nnoFleetEvents)
		if selected < 0 {
			return true
		}
		row := nnoFleetEventPage*nnoEventPageSize + selected
		if row >= 0 && row < len(nnoFleetEventData) {
			e := nnoFleetEventData[row]
			text := fmt.Sprintf("Дата: %s\r\nСкважина: %s\r\nСпособ: %s\r\nКод: %s\r\nТип события: %s\r\nПричина: %s\r\nЗаключение подтверждено: %t\r\nИсточник: %s\r\n\r\nСтатус для официального ННО: не определён — требуется утверждённое правило версии методики.\r\nИзменения классификации выполняются в карточке отказа; исходная запись здесь не редактируется.", e.Date, dash(e.Well), nnoMethodOfFailure(e), dash(e.SourceCauseCode), dash(e.EventType), dash(e.FinalCause), e.ConclusionApproved, dash(e.Source))
			openMRPTextDialog("СОСТАВ РАСЧЁТА · ИСХОДНЫЙ ОТКАЗ", text)
		}
		return true
	}
	return false
}
func nnoOpenAudit() {
	_, events := nnoSourceData()
	total, pending := 0, 0
	for _, e := range events {
		total++
		if !nnoEventIsConfirmed(e) {
			pending++
		}
	}
	openMRPTextDialog("КОНТРОЛЬ НАПОЛНЕНИЯ ННО-12", fmt.Sprintf("Журнал событий за выбранный период %s — %s: %d записей.\r\nНеутверждённые или неотказные события: %d.\r\nИсточник: %s. Способ эксплуатации: %s.\r\n\r\nДля расчёта ННО требуются:\r\n1. Заверенные помесячные сведения Ti по группам.\r\n2. Утверждённая версия методики с датами действия.\r\n3. Правила включения отказов по классификатору.\r\n4. Протокол исключений и список событий.\r\n\r\nСуществующий журнал позволяет проверить классификацию; сам по себе он не является достаточным источником Ti. Не рассчитывайте ННО через среднее МРП скважин.", nnoFleetDateFrom.Format("02.01.2006"), nnoFleetDateTo.Format("02.01.2006"), total, pending, func() string {
		if nnoSelectedDataSource() == "" {
			return "Журнал отказов"
		}
		return nnoSelectedDataSource()
	}(), nnoSelectedGroup()))
}

// Genuine 12-month mean/median durations of completed verified cycles.
// These advisory values are never substituted for the official NNO-12.
func drawNNOFleetTrend(di *DRAWITEMSTRUCT) {
	if nnoFleetShowEventTrend {
		nnoDrawEventCountChart(di)
	} else {
		nnoDrawRollingCycleChart(di)
	}
}

// Uses the source journal, including earlier months needed for rolling windows.
// No inferred Ti, unapproved events or training cases enter these statistics.
func nnoVerifiedCycleSamples() []nnoCycleSample {
	samples := make([]nnoCycleSample, 0)
	seen := map[string]bool{}
	selectedSource := nnoSelectedDataSource()
	for _, e := range failureEvents {
		if !nnoEventIsConfirmed(e) || strings.Contains(strings.ToLower(e.Source), "учебн") || (selectedSource != "" && strings.TrimSpace(e.Source) != selectedSource) {
			continue
		}
		start, errStart := failureDate(e.CycleStartDate)
		end, errEnd := failureDate(e.Date)
		if errStart != nil || errEnd != nil || !end.After(start) {
			continue
		}
		key := strings.TrimSpace(e.ID)
		if key == "" {
			key = e.ObjectKey + "|" + e.Date + "|" + e.SourceCauseCode
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		samples = append(samples, nnoCycleSample{Group: nnoMethodOfFailure(e), End: end, Days: end.Sub(start).Hours() / 24})
	}
	return samples
}

// Training wells in the bundled corpus normally live in the laboratory and
// object history, not in the official failure journal. Their presence is
// explicitly checked so the preview survives the journal's teaching filter.
// A user who discards all teaching passes no longer sees this preview.
func nnoHasActiveTrainingLabExamples() bool {
	if nnoSelectedDataSource() != "" {
		return false
	}
	// A real journal takes precedence over an unrelated teaching laboratory.
	for _, event := range failureEvents {
		source := strings.ToLower(strings.TrimSpace(event.Source))
		if !strings.Contains(source, "учеб") && !strings.Contains(source, "демо") && !strings.Contains(source, "demo") {
			return false
		}
	}
	for _, pass := range labPasses {
		if pass.Status != labPassSavedStatus {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(pass.Source))
		if strings.Contains(source, "учеб") || strings.Contains(source, "демо") || strings.Contains(source, "demo") {
			return true
		}
	}
	return false
}

func nnoShouldUseSyntheticPreview(events []FailureEvent) bool {
	// Preview is opt-in through the presence of labelled teaching source rows.
	// Respect the current period, group and source filter, and never mix a
	// synthetic preview with production/ambiguous events in the same scope.
	if nnoSelectedDataSource() != "" || len(events) == 0 {
		return false
	}
	training, other := 0, 0
	for _, e := range events {
		if e.Draft || (nnoSelectedGroup() != "Все способы" && nnoMethodOfFailure(e) != nnoSelectedGroup()) {
			continue
		}
		date, err := failureDate(e.Date)
		if err != nil || date.Before(nnoFleetDateFrom) || date.After(nnoFleetDateTo) {
			continue
		}
		src := strings.ToLower(strings.TrimSpace(e.Source))
		if strings.Contains(src, "демо") || strings.Contains(src, "учеб") || strings.Contains(src, "demo") {
			training++
		} else {
			other++
		}
	}
	return training > 0 && other == 0
}

func nnoSyntheticCycleSamples(months []nnoMonth) []nnoCycleSample {
	if len(months) == 0 {
		return nil
	}
	series := map[string][]float64{
		"УЭЦН":      {612, 640, 655, 670, 688, 701, 716, 732, 748, 760, 774, 792},
		"УШГН":      {860, 878, 905, 924, 941, 963, 981, 1002, 1018, 1036, 1055, 1070},
		"УЭВН":      {540, 562, 585, 603, 622, 645, 661, 680, 698, 715, 733, 752},
		"ОРД":       {780, 804, 826, 845, 868, 886, 904, 921, 939, 956, 975, 992},
		"Фонтанный": {980, 1008, 1032, 1059, 1084, 1105, 1124, 1146, 1168, 1186, 1204, 1225},
		"Прочие":    {505, 522, 541, 555, 573, 591, 604, 618, 637, 652, 668, 684},
	}
	out := make([]nnoCycleSample, 0, len(months)*len(series))
	for _, group := range nnoFleetFilterNames[1:] {
		values := series[group]
		for i, m := range months {
			value := values[i%len(values)]
			end := time.Date(m.Month.Year(), m.Month.Month(), 20, 0, 0, 0, 0, time.UTC)
			out = append(out, nnoCycleSample{Group: group, End: end, Days: value})
		}
	}
	return out
}

func nnoMin32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
