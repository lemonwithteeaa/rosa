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

func chooseComplexReportPath(owner syscall.Handle) (string, string) {
	buf := make([]uint16, 32768)
	name, _ := syscall.UTF16FromString("РОСА_комплексный_анализ_диагностик.docx")
	copy(buf, name)
	filter := utf16.Encode([]rune("Word (*.docx)\x00*.docx\x00Excel (*.xlsx)\x00*.xlsx\x00\x00"))
	of := OPENFILENAMEW{LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})), HwndOwner: owner, LpstrFilter: &filter[0], NFilterIndex: 1, LpstrFile: &buf[0], NMaxFile: uint32(len(buf)), LpstrDefExt: p("docx"), LpstrTitle: p("Сохранить комплексная диагностика"), Flags: OFN_EXPLORER | OFN_PATHMUSTEXIST | OFN_OVERWRITEPROMPT}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&of)))
	if r == 0 {
		return "", ""
	}
	format := "docx"
	if of.NFilterIndex == 2 {
		format = "xlsx"
	}
	path := syscall.UTF16ToString(buf)
	ext := strings.ToLower(filepath.Ext(path))
	wanted := "." + format
	if ext != ".docx" && ext != ".xlsx" {
		path += wanted
	} else if ext != wanted {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + wanted
	}
	return path, format
}

const (
	idComplexFind         = 7391
	idComplexAll          = 7392
	idComplexRun          = 7393
	idComplexClose        = 7394
	idComplexVisual       = 7395
	idComplexChart        = 7396
	idComplexExport       = 7397
	idComplexMetric       = 7398
	idComplexObject       = 7399
	idComplexObjectsAll   = 7400
	idComplexObjectsClear = 7401
	idComplexMaximize     = 7402
	idComplexZoomOut      = 7403
	idComplexZoomIn       = 7404
	idComplexBack         = 7405
	idComplexForward      = 7406
	idComplexTrend        = 7407
	idComplexTrendType    = 7408
	idComplexExcel        = 7409
	idComplexLegendScroll = 7410
	idComplexHistoryBack  = 7411
	idComplexHistoryNext  = 7412
	wmComplexExportDone   = 0x8006
	wmComplexAnalysisDone = 0x8007
	wmComplexExcelDone    = 0x8008
)

var (
	complexDone                                                                                                                          bool
	complexSearch, complexList, complexResult, complexChart, complexVisualButton                                                         syscall.Handle
	complexPasses, complexSelected                                                                                                       []LabPassRecord
	complexCatalog, complexChosenKeys                                                                                                    []string
	complexSavedKeys                                                                                                                     []string
	complexCaptionIndex                                                                                                                  map[string]string
	complexPassIndex                                                                                                                     map[string][]LabPassRecord
	complexInitialPassID                                                                                                                 string
	complexShowChart                                                                                                                     bool
	complexTitle, complexObjectLabel, complexFindButton, complexListLabel                                                                syscall.Handle
	complexAllButton, complexRunButton, complexCloseButton, complexExportButton, complexExcelButton, complexMetricCombo                  syscall.Handle
	complexObjectCombo, complexComment, complexCommentHeading, complexObjectsAllButton, complexObjectsClearButton, complexMaximizeButton syscall.Handle
	complexZoomOutButton, complexZoomInButton                                                                                            syscall.Handle
	complexHistoryBackButton, complexHistoryNextButton                                                                                   syscall.Handle
	complexBackButton, complexForwardButton                                                                                              syscall.Handle
	complexTrendCheck, complexTrendTypeCombo                                                                                             syscall.Handle
	complexLegendScrollbar                                                                                                               syscall.Handle
	complexTrendEnabled                                                                                                                  bool
	complexTrendObjectKey                                                                                                                string
	complexZoom                                                                                                                          float64 = 1
	complexLegendScrollRow                                                                                                               int
	complexHighlightedGroup                                                                                                              int = -1
	complexHighlightX, complexHighlightY                                                                                                 int32
	complexLastRightClick                                                                                                                time.Time
	complexExportBusy                                                                                                                    bool
	complexExportDone                                                                                                                    chan complexExportResult
	complexAnalysisBusy                                                                                                                  bool
	complexAnalysisDone                                                                                                                  chan complexAnalysisResult
	complexExcelDone                                                                                                                     chan complexExportResult
	complexExcelBusy                                                                                                                     bool
	complexLoadingWindow                                                                                                                 syscall.Handle
	complexWindowHandle                                                                                                                  syscall.Handle
	complexSelectionUndo, complexSelectionRedo                                                                                           [][]LabPassRecord
)

func complexReadableObjectKey(key string) string {
	return readableObjectKeyFromIndex(key, complexCaptionIndex)
}

type complexExportResult struct {
	path, content string
	err           error
}

type complexAnalysisResult struct {
	content string
}

func complexChartPointFromCursor() (POINT, bool) {
	if complexChart == 0 {
		return POINT{}, false
	}
	var point POINT
	if result, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&point))); result == 0 {
		return POINT{}, false
	}
	pScreenToClient.Call(uintptr(complexChart), uintptr(unsafe.Pointer(&point)))
	return point, true
}

func complexSelectSeriesAtPoint(point POINT) int {
	var rc RECT
	pGetClientRect.Call(uintptr(complexChart), uintptr(unsafe.Pointer(&rc)))
	return complexSeriesHitTest(complexChartGroups(), complexSelectedMetric(), rc, point.X, point.Y)
}

func complexLegendGroupAtPoint(point POINT) int {
	groups := complexChartGroups()
	if len(groups) == 0 {
		return -1
	}
	var rc RECT
	pGetClientRect.Call(uintptr(complexChart), uintptr(unsafe.Pointer(&rc)))
	columns, totalRows, visibleRows := complexLegendViewport(len(groups), rc.Bottom-rc.Top)
	if visibleRows <= 0 {
		visibleRows = totalRows
	}
	plot, ok := complexChartPlotRect(rc, visibleRows)
	if !ok {
		return -1
	}
	gap := int32(18)
	available := rc.Right - rc.Left - 32 - int32(columns-1)*gap
	columnWidth := available / int32(columns)
	for i := range groups {
		row, column := i/columns, i%columns
		if row < complexLegendScrollRow || row >= complexLegendScrollRow+visibleRows {
			continue
		}
		left := rc.Left + 16 + int32(column)*(columnWidth+gap)
		top := plot.Bottom + 100 + int32(row-complexLegendScrollRow)*24
		if point.X >= left && point.X <= left+columnWidth && point.Y >= top && point.Y <= top+22 {
			return i
		}
	}
	return -1
}

func cloneComplexSelection(source []LabPassRecord) []LabPassRecord {
	return append([]LabPassRecord(nil), source...)
}

func complexEnabledFlag(enabled bool) uintptr {
	if enabled {
		return 1
	}
	return 0
}

func updateComplexHistoryButtons() {
	if complexHistoryBackButton != 0 {
		pEnableWindow.Call(uintptr(complexHistoryBackButton), complexEnabledFlag(len(complexSelectionUndo) > 0))
	}
	if complexHistoryNextButton != 0 {
		pEnableWindow.Call(uintptr(complexHistoryNextButton), complexEnabledFlag(len(complexSelectionRedo) > 0))
	}
}

func resetComplexSelectionHistory() {
	complexSelectionUndo = nil
	complexSelectionRedo = nil
	updateComplexHistoryButtons()
}

func pushComplexSelectionHistory() {
	complexSelectionUndo = append(complexSelectionUndo, cloneComplexSelection(complexSelected))
	if len(complexSelectionUndo) > 30 {
		complexSelectionUndo = complexSelectionUndo[len(complexSelectionUndo)-30:]
	}
	complexSelectionRedo = nil
	updateComplexHistoryButtons()
}

func refreshComplexSelectionView() {
	invalidateComplexChartGroups()
	complexHighlightedGroup = -1
	complexLegendScrollRow = 0
	complexZoom = 1
	if len(complexSelected) >= 2 {
		setComplexResultText(complexConclusion(cloneComplexSelection(complexSelected)))
	}
	refreshComplexComment()
	var client RECT
	pGetClientRect.Call(uintptr(complexChart), uintptr(unsafe.Pointer(&client)))
	updateComplexLegendScrollbar(client.Bottom - client.Top)
	pInvalidateRect.Call(uintptr(complexChart), 0, 1)
	updateComplexHistoryButtons()
}

func complexIsolateObjectAt(index int) {
	groups := complexChartGroups()
	if index < 0 || index >= len(groups) || len(groups[index]) == 0 || len(groups) == 1 {
		return
	}
	pushComplexSelectionHistory()
	complexSelected = cloneComplexSelection(groups[index])
	refreshComplexSelectionView()
}

func complexUndoSelection() {
	if len(complexSelectionUndo) == 0 {
		return
	}
	complexSelectionRedo = append(complexSelectionRedo, cloneComplexSelection(complexSelected))
	complexSelected = complexSelectionUndo[len(complexSelectionUndo)-1]
	complexSelectionUndo = complexSelectionUndo[:len(complexSelectionUndo)-1]
	refreshComplexSelectionView()
}

func complexRedoSelection() {
	if len(complexSelectionRedo) == 0 {
		return
	}
	complexSelectionUndo = append(complexSelectionUndo, cloneComplexSelection(complexSelected))
	complexSelected = complexSelectionRedo[len(complexSelectionRedo)-1]
	complexSelectionRedo = complexSelectionRedo[:len(complexSelectionRedo)-1]
	refreshComplexSelectionView()
}

func complexShowSeriesLabel(point POINT) {
	index := complexSelectSeriesAtPoint(point)
	if index < 0 {
		return
	}
	complexHighlightedGroup = index
	complexHighlightX, complexHighlightY = point.X, point.Y
	pInvalidateRect.Call(uintptr(complexChart), 0, 1)
}

func complexRemoveObjectAt(index int) {
	groups := complexChartGroups()
	if index < 0 || index >= len(groups) || len(groups[index]) == 0 {
		return
	}
	key := complexAnalysisGroupKey(groups[index][0].ObjectKey)
	caption := complexReadableObjectKey(groups[index][0].ObjectKey)
	pushComplexSelectionHistory()
	removed := make([]LabPassRecord, 0, len(groups[index]))
	filtered := complexSelected[:0]
	for _, pass := range complexSelected {
		if complexAnalysisGroupKey(pass.ObjectKey) == key {
			removed = append(removed, pass)
		} else {
			filtered = append(filtered, pass)
		}
	}
	complexSelected = append([]LabPassRecord(nil), filtered...)
	invalidateComplexChartGroups()
	addSessionTrashItem("Объект графика · "+caption, "Удалён только из текущего анализа; исходные лабораторные данные сохранены.", func() error {
		for _, pass := range complexSelected {
			if complexAnalysisGroupKey(pass.ObjectKey) == key {
				return nil
			}
		}
		complexSelected = append(complexSelected, removed...)
		refreshComplexSelectionView()
		return nil
	}, nil)
	complexHighlightedGroup = -1
	complexLegendScrollRow = 0
	refreshComplexSelectionView()
}

func complexHandleRightDoubleClick(owner syscall.Handle, point POINT) {
	index := complexSelectSeriesAtPoint(point)
	groups := complexChartGroups()
	if index < 0 || index >= len(groups) || len(groups[index]) == 0 {
		return
	}
	objectKey := groups[index][0].ObjectKey
	caption := complexReadableObjectKey(objectKey)
	createPopupMenu := user32.NewProc("CreatePopupMenu")
	appendMenu := user32.NewProc("AppendMenuW")
	trackPopupMenu := user32.NewProc("TrackPopupMenu")
	destroyMenu := user32.NewProc("DestroyMenu")
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	appendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(p("Удалить из анализа…"))))
	trendIsSelected := complexTrendEnabled && complexAnalysisGroupKey(complexTrendObjectKey) == complexAnalysisGroupKey(objectKey)
	trendCaption := "Добавить линию тренда…"
	if trendIsSelected {
		trendCaption = "Убрать линию тренда…"
	}
	appendMenu.Call(menu, 0, 2, uintptr(unsafe.Pointer(p(trendCaption))))
	var screen POINT
	if result, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&screen))); result == 0 {
		return
	}
	command, _, _ := trackPopupMenu.Call(menu, 0x0100|0x0002, uintptr(screen.X), uintptr(screen.Y), 0, uintptr(owner), 0)
	switch command {
	case 1:
		confirm, _, _ := pMessageBoxW.Call(uintptr(owner), uintptr(unsafe.Pointer(p("Удалить из текущего анализа?\n\n"+caption+"\n\nИсходные данные останутся, объект можно восстановить из сессионной корзины."))), uintptr(unsafe.Pointer(p("Удалить из анализа?"))), 0x00000004|MB_ICONWARNING)
		if confirm == 6 {
			complexRemoveObjectAt(index)
		}
	case 2:
		action := "Добавить линию тренда для объекта?"
		if trendIsSelected {
			action = "Убрать линию тренда для объекта?"
		}
		confirm, _, _ := pMessageBoxW.Call(uintptr(owner), uintptr(unsafe.Pointer(p(action+"\n\n"+caption))), uintptr(unsafe.Pointer(p("Линия тренда"))), 0x00000004|0x00000020)
		if confirm == 6 {
			complexTrendEnabled = !trendIsSelected
			if complexTrendEnabled {
				complexTrendObjectKey = complexAnalysisGroupKey(objectKey)
			} else {
				complexTrendObjectKey = ""
			}
			check := uintptr(0)
			if complexTrendEnabled {
				check = 1
			}
			pSendMessageW.Call(uintptr(complexTrendCheck), BM_SETCHECK, check, 0)
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
		}
	}
}

func updateComplexLegendScrollbar(chartHeight int32) {
	_, totalRows, visibleRows := complexLegendViewport(len(complexChartGroups()), chartHeight)
	maximum := maxInt(0, totalRows-visibleRows)
	if complexLegendScrollRow > maximum {
		complexLegendScrollRow = maximum
	}
	pSetScrollRange.Call(uintptr(complexLegendScrollbar), 1, 0, uintptr(maximum), 1)
	pSetScrollPos.Call(uintptr(complexLegendScrollbar), 1, uintptr(complexLegendScrollRow), 1)
	show(complexLegendScrollbar, maximum > 0 && complexShowChart)
}

func complexSavedObjectKeys() []string {
	return append([]string(nil), complexSavedKeys...)
}

// prepareComplexAnalysisIndex takes one snapshot of the laboratory history.
// The dialog then sorts, searches and selects against that snapshot instead of
// rebuilding all object views and querying the repository once per object.
func prepareComplexAnalysisIndex() {
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	complexCaptionIndex = readableObjectKeyIndex()
	complexPassIndex = make(map[string][]LabPassRecord)
	seen := map[string]bool{}
	complexSavedKeys = nil
	for _, pass := range labPasses {
		key := strings.TrimSpace(pass.ObjectKey)
		normalized := normalizeLaboratoryObjectKey(key)
		if pass.Status != labPassSavedStatus || normalized == "" {
			continue
		}
		complexPassIndex[normalized] = append(complexPassIndex[normalized], pass)
		if !seen[normalized] {
			seen[normalized] = true
			complexSavedKeys = append(complexSavedKeys, key)
		}
	}
	sort.Slice(complexSavedKeys, func(i, j int) bool {
		return strings.ToLower(complexReadableObjectKey(complexSavedKeys[i])) < strings.ToLower(complexReadableObjectKey(complexSavedKeys[j]))
	})
	complexCatalog = append([]string(nil), complexSavedKeys...)
}

func complexSelectedMetric() complexMetric {
	index := laboratoryComboSelection(complexMetricCombo)
	if index < 0 || index >= len(complexMetrics) {
		index = 3
	}
	return complexMetrics[index]
}

func complexCombinedFactors(passes []LabPassRecord) string {
	var rising, falling, attention []string
	changed := map[string]int{}
	for _, metric := range complexMetrics {
		values := complexMetricValues(passes, metric)
		if len(values) < 2 {
			continue
		}
		first, last := values[0].value, values[len(values)-1].value
		if last > first {
			rising = append(rising, metric.label)
			changed[metric.key] = 1
		}
		if last < first {
			falling = append(falling, metric.label)
			changed[metric.key] = -1
		}
		if boundary, ok := laboratoryMetricThreshold(metric.group, metric.key); ok && boundary.exceeded(last) {
			attention = append(attention, fmt.Sprintf("%s %.3g %s (%s)", metric.label, last, metric.unit, boundary.caption(metric.unit)))
		}
	}
	var b strings.Builder
	b.WriteString("\r\nСОВОКУПНОЕ ВЛИЯНИЕ И ПРОВЕРКА ПРЕДПОСЫЛОК\r\n")
	if len(rising) > 0 {
		b.WriteString("Одновременно растут: " + strings.Join(rising, ", ") + ". ")
	}
	if len(falling) > 0 {
		b.WriteString("Снижаются: " + strings.Join(falling, ", ") + ". ")
	}
	if len(rising)+len(falling) == 0 {
		b.WriteString("По сопоставимым датам выраженного изменения перечисленных показателей не выявлено. ")
	}
	if changed["watercut"] == 1 {
		b.WriteString("Рост обводнённости увеличивает контакт оборудования с водной фазой; при одновременном росте минерализации, CO₂/H₂S/O₂ или скорости коррозии проверьте режим, защиту и состав воды по каждой дате. Значение обводнённости может происходить из техрежима; сверяйте дату записи. ")
	} else if changed["watercut"] == -1 {
		b.WriteString("Снижение обводнённости учитывайте при сравнении концентраций и скорости коррозии; подтвердите режимный показатель по каждой дате. ")
	} else {
		b.WriteString("Изменение обводнённости по датам не подтверждено; проверьте журнал техрежима, прежде чем связывать динамику химии с долей воды. ")
	}
	if changed["co2"] == 1 || changed["h2s"] == 1 || changed["o2"] == 1 {
		b.WriteString("Рост CO₂, H₂S или O₂ при наличии водной фазы усиливает повод проверить механизм коррозии и ингибирование; сопоставьте pH и фактическую скорость коррозии. ")
	}
	if changed["ca"] == 1 && changed["so4"] == 1 {
		b.WriteString("Рост кальция вместе с сульфатами усиливает основание проверить пересыщение сульфатных солей, состав смешиваемых вод и эффективность ингибитора. Сами концентрации ещё не доказывают выпадение осадка. ")
	} else if changed["ca"] == 1 || changed["so4"] == 1 {
		b.WriteString("Изменение кальция или сульфатов оцените вместе с парным ионом, pH и расчётом насыщения; отдельный показатель не подтверждает осадок. ")
	}
	if changed["mineralization"] == 1 && changed["corrosion_rate"] == 1 {
		b.WriteString("Совместный рост минерализации и измеренной скорости коррозии требует проверить состав воды, CO₂/H₂S/O₂, pH, материалы и дозирование ингибитора; причинная связь по двум рядам не установлена. ")
	} else if changed["corrosion_rate"] == 1 {
		b.WriteString("Рост скорости коррозии проверьте повторным измерением, данными по pH, газам, обводнённости и защите оборудования. ")
	}
	if changed["paraffins"] == 1 {
		b.WriteString("При росте парафинов сопоставьте температуру потока с температурой начала кристаллизации, вязкость и историю АСПО. ")
	}
	if changed["suspended"] == 1 {
		b.WriteString("При росте взвеси определите состав частиц и проверьте фильтры, эрозию и недавние операции на скважине. ")
	}
	if len(attention) > 0 {
		b.WriteString("\r\nВЫШЕ РАБОЧИХ ГРАНИЦ: " + strings.Join(attention, "; ") + ". Приоритет — подтвердить значения и проверить соответствующие механизмы. ")
	}
	b.WriteString("\r\nСопоставьте диагностики с датами изменения температуры, давления, дебита, обводнённости, дозировки реагентов и ремонтов. Если этих сведений нет, предпосылки остаются гипотезами. Следующая проба тем же методом покажет, сохраняется ли тенденция.")
	return b.String()
}

func refreshComplexComment() {
	if complexComment == 0 {
		return
	}
	if len(complexSelected) < 2 {
		setComplexCommentText("Выберите минимум две сохранённые диагностики одного объекта с разными датами и нажмите «Проанализировать выбранные». Тогда здесь появится сопоставление и рекомендации.")
		return
	}
	if len(complexPassGroups(complexSelected)) > 1 {
		metric := complexSelectedMetric()
		var b strings.Builder
		b.WriteString("ВЫВОД ДЛЯ ТЕХНОЛОГА — НЕСКОЛЬКО ОБЪЕКТОВ\r\n")
		for _, group := range complexPassGroups(complexSelected) {
			values := complexMetricValues(group, metric)
			fmt.Fprintf(&b, "№%d %s: ", complexObjectNumber(group[0].ObjectKey), complexReadableObjectKey(group[0].ObjectKey))
			if len(values) < 2 {
				if len(values) == 0 {
					b.WriteString("по выбранному показателю нет сохранённых значений; линия отсутствует.\r\n")
				} else {
					b.WriteString("есть только одна дата; для тренда нужны две.\r\n")
				}
				continue
			}
			first, last := values[0], values[len(values)-1]
			fmt.Fprintf(&b, "%s %.3g → %s %.3g %s. ", first.date, first.value, last.date, last.value, metric.unit)
			if last.value > first.value {
				b.WriteString("Рост; проверьте условия и защиту. ")
			} else if last.value < first.value {
				b.WriteString("Снижение; подтвердите повторной пробой. ")
			} else {
				b.WriteString("Без изменения. ")
			}
			if t, ok := laboratoryMetricThreshold(metric.group, metric.key); ok && t.exceeded(last.value) {
				b.WriteString("За рабочей границей — приоритет проверки. ")
			}
			b.WriteString("\r\n")
			b.WriteString(complexCombinedFactors(group))
			b.WriteString("\r\n")
		}
		b.WriteString("Различия между объектами не устанавливают общую причину; сопоставьте температуру, давление, состав воды, дозирование и историю ремонтов каждого объекта.")
		setComplexCommentText(b.String())
		return
	}
	metric := complexSelectedMetric()
	values := complexMetricValues(complexSelected, metric)
	if len(values) < 2 {
		setComplexCommentText("По показателю «" + metric.label + "» нет значений на двух разных датах. Выберите другой показатель либо внесите следующую диагностику. Причину и направление изменения по одному измерению установить нельзя.")
		return
	}
	first, last := values[0], values[len(values)-1]
	threshold, hasThreshold := laboratoryMetricThreshold(metric.group, metric.key)
	change := last.value - first.value
	assessment := "показатель без изменения"
	action := "Продолжите контроль тем же методом, сопоставьте результаты с режимом работы скважины."
	if change > 0 {
		assessment = "показатель вырос"
		action = "Возможная предпосылка ухудшения: " + metric.implication + ". Проверьте условия отбора и методику измерения, затем сопоставьте изменение с температурой, давлением, дозированием реагентов и историей ремонтов. Если условия сохранятся, рост может продолжиться; подтвердите это следующей пробой."
	} else if change < 0 {
		assessment = "показатель снизился"
		action = "Снижение требует сопоставления с изменениями режима и обработки. Подтвердите динамику повторной пробой; при сохранении условий снижение может продолжиться, но численный прогноз по немногим датам ненадёжен."
	}
	if hasThreshold && threshold.exceeded(last.value) {
		action = "ПОСЛЕДНЕЕ ЗНАЧЕНИЕ ЗА РАБОЧЕЙ ГРАНИЦЕЙ (" + threshold.caption(metric.unit) + "). Требуется проверить результат и режим. " + action
	} else if hasThreshold {
		action = threshold.caption(metric.unit) + "; последнее значение не вышло за эту границу. " + action
	}
	setComplexCommentText(fmt.Sprintf("%s: %s — %.3g %s; %s — %.3g %s. Между этими датами %s.\r\n%s\r\nЭто предположение по данным диагностик, а не установленная причина осложнения.", metric.label, first.date, first.value, metric.unit, last.date, last.value, metric.unit, assessment, action) + complexCombinedFactors(complexSelected))
}
func setComplexCommentText(s string) {
	s = strings.TrimPrefix(s, "ВЫВОД ДЛЯ ТЕХНОЛОГА — НЕСКОЛЬКО ОБЪЕКТОВ\r\n")
	formatComplexText(complexComment, s)
}

func formatComplexText(control syscall.Handle, s string) {
	if control == 0 {
		return
	}
	setText(control, s)
	plain := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, CrTextColor: uint32(rgb(43, 46, 50))}
	pSendMessageW.Call(uintptr(control), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(control), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&plain)))
	bold := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82))}
	offset := 0
	for _, line := range strings.SplitAfter(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		label := strings.TrimSpace(line)
		if label != "" {
			length := 0
			if label == strings.ToUpper(label) && laboratoryUTF16Len(label) < 100 {
				length = laboratoryUTF16Len(label)
			} else if strings.HasPrefix(label, "№") {
				length = laboratoryUTF16Len(label)
			} else if colon := strings.Index(label, ":"); colon > 0 && colon < 65 {
				length = laboratoryUTF16Len(label[:colon+1])
			}
			if length > 0 {
				start := offset + laboratoryUTF16Len(line[:len(line)-len(strings.TrimLeft(line, " \t"))])
				pSendMessageW.Call(uintptr(control), EM_SETSEL, uintptr(start), uintptr(start+length))
				pSendMessageW.Call(uintptr(control), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&bold)))
			}
		}
		offset += laboratoryUTF16Len(line)
	}
	pSendMessageW.Call(uintptr(control), EM_SETSEL, 0, 0)
}

func setComplexResultText(s string) { formatComplexText(complexResult, s) }

func layoutComplexDialog(w, h int32) {
	if complexList == 0 || w < 740 || h < 570 {
		return
	}
	contentW := w - 48
	// Navigation, title and window actions have separate horizontal zones. The
	// previous full-width title started under the navigation buttons and produced
	// the overlap visible on ordinary 1366 px screens.
	// Keep navigation labels complete. The former narrow controls clipped both
	// captions to "Назад к объек..." / "Вперёд к лаборат...".
	setPos(complexBackButton, 24, 14, 200, 36)
	setPos(complexForwardButton, 232, 14, 242, 36)
	setPos(complexTitle, 492, 15, max32(300, w-520), 38)
	if complexShowChart {
		for _, control := range []syscall.Handle{complexObjectLabel, complexSearch, complexFindButton, complexObjectCombo, complexObjectsAllButton, complexObjectsClearButton, complexList} {
			show(control, false)
		}
		setText(complexListLabel, "ПОКАЗАТЕЛЬ И ДИНАМИКА ВЫБРАННЫХ ДИАГНОСТИК")
		setPos(complexListLabel, 24, 68, contentW-460, 28)
		setPos(complexMetricCombo, w-448, 65, 424, 390)
		show(complexTrendCheck, true)
		show(complexTrendTypeCombo, true)
		setPos(complexTrendCheck, 24, 208, 170, 28)
		setPos(complexTrendTypeCombo, 200, 205, 230, 300)
		halfW := (contentW - 10) / 2
		setPos(complexAllButton, 24, 112, halfW, 42)
		setPos(complexRunButton, 34+halfW, 112, contentW-10-halfW, 42)
		thirdW := (contentW - 20) / 3
		setPos(complexVisualButton, 24, 162, thirdW, 42)
		setPos(complexExportButton, 34+thirdW, 162, thirdW, 42)
		setPos(complexExcelButton, 44+2*thirdW, 162, contentW-20-2*thirdW, 42)
		resultH := max32(160, h-270)
		_, legendRowCount := complexLegendLayout(len(complexChartGroups()))
		legendRows := int32(legendRowCount)
		// Reserve readable space for the technologist's conclusion while the
		// plot still has room for its axes and the numbered object legend.
		// Give the chart enough room for every legend row. The previous min/max
		// order could cap the control below its own required height and the drawing
		// code then incorrectly asked the user to enlarge an already maximized window.
		maxChartH := max32(220, resultH-88)
		chartH := min32(maxChartH, max32(212+legendRows*24, resultH*52/100))
		setPos(complexChart, 24, 244, contentW-22, chartH)
		setPos(complexLegendScrollbar, w-44, 244, 20, chartH)
		setPos(complexHistoryBackButton, w-256, 252, 38, 34)
		setPos(complexHistoryNextButton, w-210, 252, 38, 34)
		setPos(complexZoomOutButton, w-164, 252, 38, 34)
		setPos(complexZoomInButton, w-118, 252, 38, 34)
		show(complexHistoryBackButton, true)
		show(complexHistoryNextButton, true)
		show(complexZoomOutButton, true)
		show(complexZoomInButton, true)
		updateComplexLegendScrollbar(chartH)
		setPos(complexCommentHeading, 24, 252+chartH, contentW, 26)
		setPos(complexComment, 24, 278+chartH, contentW, max32(50, resultH-chartH-34))
		return
	}
	for _, control := range []syscall.Handle{complexObjectLabel, complexSearch, complexFindButton, complexObjectCombo, complexObjectsAllButton, complexObjectsClearButton, complexList} {
		show(control, true)
	}
	show(complexTrendCheck, false)
	show(complexTrendTypeCombo, false)
	show(complexZoomOutButton, false)
	show(complexZoomInButton, false)
	show(complexHistoryBackButton, false)
	show(complexHistoryNextButton, false)
	show(complexLegendScrollbar, false)
	setText(complexListLabel, fmt.Sprintf("СОХРАНЁННЫЕ ДИАГНОСТИКИ — %d ОБЪЕКТ(ОВ), ВЫБЕРИТЕ ДАТЫ", len(complexChosenKeys)))
	setPos(complexObjectLabel, 24, 58, contentW, 25)
	searchW := max32(220, contentW-220)
	setPos(complexSearch, 24, 88, searchW, 34)
	setPos(complexFindButton, w-212, 88, 188, 34)
	setPos(complexObjectCombo, 24, 156, contentW, 94)
	setPos(complexObjectsAllButton, w-330, 126, 140, 28)
	setPos(complexObjectsClearButton, w-180, 126, 156, 28)
	setPos(complexListLabel, 24, 255, contentW-460, 27)
	setPos(complexMetricCombo, w-448, 252, 424, 390)
	listH := max32(105, min32(160, h/4-65))
	setPos(complexList, 24, 282, contentW, listH)
	buttonsY := 282 + listH + 12
	// Two rows keep the controls readable on a normal desktop and at high DPI.
	halfW := (contentW - 10) / 2
	setPos(complexAllButton, 24, buttonsY, halfW, 42)
	setPos(complexRunButton, 34+halfW, buttonsY, contentW-10-halfW, 42)
	thirdW := (contentW - 20) / 3
	setPos(complexVisualButton, 24, buttonsY+50, thirdW, 42)
	setPos(complexExportButton, 34+thirdW, buttonsY+50, thirdW, 42)
	setPos(complexExcelButton, 44+2*thirdW, buttonsY+50, contentW-20-2*thirdW, 42)
	resultY := buttonsY + 104
	resultH := max32(105, h-resultY-24)
	setPos(complexResult, 24, resultY, contentW, resultH)
	chartH := max32(120, resultH*46/100)
	setPos(complexChart, 24, resultY, contentW, chartH)
	setPos(complexCommentHeading, 24, resultY+chartH+8, contentW, 26)
	setPos(complexComment, 24, resultY+chartH+34, contentW, max32(45, resultH-chartH-34))
	// Moving overlapped child windows can leave copied pixels on the parent.
	pRedrawWindow.Call(uintptr(complexTitle), 0, 0, 0x0001|0x0004)
}

func complexLoadObjects(keys []string) {
	complexChosenKeys = append([]string(nil), keys...)
	pSendMessageW.Call(uintptr(complexList), LB_RESETCONTENT, 0, 0)
	complexPasses = nil
	complexSelected = nil
	invalidateComplexChartGroups()
	resetComplexSelectionHistory()
	refreshComplexComment()
	complexShowChart = false
	show(complexChart, false)
	show(complexComment, false)
	show(complexCommentHeading, false)
	show(complexResult, true)
	setText(complexVisualButton, "Инфографика")
	for _, key := range keys {
		complexPasses = append(complexPasses, complexPassIndex[normalizeLaboratoryObjectKey(key)]...)
	}
	sort.SliceStable(complexPasses, func(i, j int) bool {
		a, b := normalizeLaboratoryObjectKey(complexPasses[i].ObjectKey), normalizeLaboratoryObjectKey(complexPasses[j].ObjectKey)
		if a != b {
			return complexObjectNumber(complexPasses[i].ObjectKey) < complexObjectNumber(complexPasses[j].ObjectKey)
		}
		return parseDate(complexPasses[i].Date).Before(parseDate(complexPasses[j].Date))
	})
	for i, pass := range complexPasses {
		line := fmt.Sprintf("№%d · %s · %s · %s", complexObjectNumber(pass.ObjectKey), pass.Date, pass.Source, pass.PassID)
		pSendMessageW.Call(uintptr(complexList), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(line))))
		if pass.PassID == complexInitialPassID {
			pSendMessageW.Call(uintptr(complexList), 0x0185, 1, uintptr(i))
		}
	}
	setText(complexObjectLabel, fmt.Sprintf("ОБЪЕКТЫ — ВЫБРАНО: %d. ЩЕЛЧОК ВКЛЮЧАЕТ ИЛИ ИСКЛЮЧАЕТ ОБЪЕКТ", len(keys)))
	setText(complexListLabel, fmt.Sprintf("СОХРАНЁННЫЕ ДИАГНОСТИКИ — %d ОБЪЕКТ(ОВ), ВЫБЕРИТЕ ДАТЫ", len(keys)))
	if len(keys) == 0 {
		setComplexResultText("Щёлкните по объектам в списке выше или найдите их по шифру. Выделенные строки включены в анализ. Затем выберите диагностики и даты.")
	} else {
		setComplexResultText(fmt.Sprintf("Выбрано объектов: %d; сохранённых диагностик: %d. Выделите минимум две диагностики на объект для тренда. Номера объектов показаны в легенде графика.", len(keys), len(complexPasses)))
	}
	pRedrawWindow.Call(uintptr(complexTitle), 0, 0, 0x0001|0x0004)
}
func complexObjectNumber(key string) int {
	normalized := normalizeLaboratoryObjectKey(key)
	for i, s := range complexChosenKeys {
		if normalizeLaboratoryObjectKey(s) == normalized {
			return i + 1
		}
	}
	return 0
}
func complexFind(query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	key := ""
	for _, s := range complexCatalog {
		if normalizeLaboratoryObjectKey(s) == normalizeLaboratoryObjectKey(query) {
			key = s
			break
		}
	}
	if key == "" {
		var matches []string
		for _, s := range complexCatalog {
			if q != "" && strings.Contains(strings.ToLower(complexReadableObjectKey(s)), q) {
				matches = append(matches, s)
			}
		}
		if len(matches) == 1 {
			key = matches[0]
		}
	}
	if key == "" {
		setComplexResultText("Объект по шифру не найден. Выбранные объекты сохранены; проверьте запрос или щёлкните объект в списке.")
		return
	}
	for i, candidate := range complexSavedObjectKeys() {
		if normalizeLaboratoryObjectKey(candidate) == normalizeLaboratoryObjectKey(key) {
			pSendMessageW.Call(uintptr(complexObjectCombo), 0x0185, 1, uintptr(i)) // LB_SETSEL
			break
		}
	}
	complexLoadObjects(complexObjectSelection())
}
func complexObjectSelection() []string {
	keys := complexSavedObjectKeys()
	chosen := make([]string, 0, len(keys))
	for i, key := range keys {
		selected, _, _ := pSendMessageW.Call(uintptr(complexObjectCombo), 0x0187, uintptr(i), 0) // LB_GETSEL
		if selected != 0 {
			chosen = append(chosen, key)
		}
	}
	return chosen
}
func complexConclusion(selected []LabPassRecord) string {
	sort.SliceStable(selected, func(i, j int) bool { return parseDate(selected[i].Date).Before(parseDate(selected[j].Date)) })
	groups := complexPassGroups(selected)
	var b strings.Builder
	fmt.Fprintf(&b, "КОМПЛЕКСНАЯ ДИАГНОСТИКА\r\nОбъектов: %d. Диагностик: %d. Период: %s — %s.\r\n\r\n", len(groups), len(selected), selected[0].Date, selected[len(selected)-1].Date)
	if len(groups) > 1 {
		b.WriteString("\r\nСОПОСТАВЛЕНИЕ ПО ОБЪЕКТАМ\r\n")
		for _, group := range groups {
			fmt.Fprintf(&b, "№%d — %s; диагностик: %d.\r\n", complexObjectNumber(group[0].ObjectKey), complexReadableObjectKey(group[0].ObjectKey), len(group))
			b.WriteString(complexDateAssessment(group))
			b.WriteString(complexCombinedFactors(group))
		}
		b.WriteString("\r\nМежду разными объектами абсолютные значения сравнивайте только при одинаковых методиках и единицах.\r\n")
		return b.String()
	}
	for _, p0 := range selected {
		fmt.Fprintf(&b, "№%d · %s · %s · %s\r\n", complexObjectNumber(p0.ObjectKey), p0.Date, p0.Source, p0.PassID)
		for _, g := range labGroups {
			if g.Code != "overall" && laboratoryPassHasGroupData(p0, g.Code) {
				fmt.Fprintf(&b, "  %s\r\n", laboratoryPassGroupShortConclusion(p0, g.Code))
			}
		}
	}
	b.WriteString("\r\nСОПОСТАВЛЕНИЕ\r\n")
	measures := []struct{ group, key, label, unit string }{{"corrosion", "corrosion_rate", "Скорость коррозии", "мм/год"}, {"corrosion", "mineralization", "Минерализация", "г/л"}, {"scale", "ca", "Кальций", "мг/л"}, {"scale", "so4", "Сульфаты", "мг/л"}, {"aspo", "paraffins", "Парафины", "% масс."}, {"solids", "suspended", "Взвешенные вещества", "мг/л"}}
	compared := 0
	for _, m := range measures {
		var first, last float64
		unit := ""
		count := 0
		for _, p0 := range selected {
			v, ok := parseLabNumber(p0.Groups[m.group][m.key])
			if !ok {
				continue
			}
			u := p0.Units[m.group][m.key]
			if u == "" {
				u = m.unit
			}
			if count == 0 {
				first = v
				unit = u
			}
			if strings.EqualFold(unit, u) {
				last = v
				count++
			}
		}
		if count >= 2 {
			fmt.Fprintf(&b, "%s: %.3g → %.3g %s (%d значений).\r\n", m.label, first, last, unit, count)
			compared++
		}
	}
	if compared == 0 {
		b.WriteString("Нет двух значений с совпадающими единицами для сравнения.\r\n")
	}
	b.WriteString(complexDateAssessment(selected))
	b.WriteString("\r\nРЕКОМЕНДАЦИИ ТЕХНОЛОГУ\r\nСверьте методики и даты отбора проб. При росте скорости коррозии проверьте эффективность ингибирования и подтвердите результат повторным измерением. При изменении Ca²⁺, SO₄²⁻ или минерализации проверьте пересыщение с учётом температуры и давления. Сопоставьте тенденции с режимом скважины и историей ремонтов; один показатель не устанавливает причину осложнения.")
	return b.String()
}
func drawComplexButton(di *DRAWITEMSTRUCT) bool {
	switch int(di.CtlID) {
	case idComplexFind, idComplexAll, idComplexRun, idComplexClose, idComplexVisual, idComplexExport, idComplexExcel, idComplexObjectsAll, idComplexObjectsClear, idComplexMaximize, idComplexZoomOut, idComplexZoomIn, idComplexHistoryBack, idComplexHistoryNext, idComplexBack, idComplexForward:
	default:
		return false
	}
	primary := int(di.CtlID) == idComplexRun || int(di.CtlID) == idComplexFind || int(di.CtlID) == idComplexExport || int(di.CtlID) == idComplexExcel
	background, foreground := rgb(222, 231, 242), rgb(18, 47, 80)
	if primary {
		background, foreground = rgb(205, 24, 45), rgb(255, 255, 255)
	}
	if di.ItemState&0x0001 != 0 {
		background = rgb(104, 124, 148)
	} // ODS_SELECTED
	brush, _, _ := pCreateSolidBrush.Call(background)
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), brush)
	pDeleteObject.Call(brush)
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(hFontBold))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), foreground)
	label := getText(di.HwndItem)
	textRect := di.RcItem
	textRect.Left += 5
	textRect.Right -= 5
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(label))), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), DT_SINGLELINE|DT_VCENTER|DT_CENTER|DT_NOPREFIX|0x8000) // DT_END_ELLIPSIS
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return true
}
func complexWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		h := syscall.Handle(hwnd)
		complexTitle = modalCreate("STATIC", "Комплексная диагностика", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 15, 870, 38, h, 0)
		if complexTitle != 0 && hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(complexTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		complexObjectLabel = modalCreate("STATIC", "ОБЪЕКТЫ — ЩЁЛКНИТЕ ДЛЯ ВЫБОРА НЕСКОЛЬКИХ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 54, 870, 25, h, 0)
		if complexObjectLabel != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(complexObjectLabel), WM_SETFONT, uintptr(hFontBold), 1)
		}
		complexSearch = modalCreate("EDIT", strings.TrimSpace(getText(labObjectEdit)), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP, 24, 82, 650, 34, h, 0)
		complexObjectCombo = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|0x00100000|0x0008|LBS_NOTIFY, 24, 156, 862, 94, h, idComplexObject) // LBS_MULTIPLESEL; WS_HSCROLL
		for _, key := range complexSavedObjectKeys() {
			pSendMessageW.Call(uintptr(complexObjectCombo), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(complexReadableObjectKey(key)))))
		}
		pSendMessageW.Call(uintptr(complexObjectCombo), 0x0194, 2200, 0) // LB_SETHORIZONTALEXTENT: full long names can be scrolled
		complexObjectsAllButton = modalCreate("BUTTON", "Все объекты", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 930, 126, 140, 28, h, idComplexObjectsAll)
		complexObjectsClearButton = modalCreate("BUTTON", "Очистить выбор", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 1080, 126, 156, 28, h, idComplexObjectsClear)
		complexFindButton = modalCreate("BUTTON", "Найти диагностики", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 686, 82, 200, 34, h, idComplexFind)
		complexListLabel = modalCreate("STATIC", "СОХРАНЁННЫЕ ДИАГНОСТИКИ — ВЫБЕРИТЕ ДАТЫ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 128, 800, 26, h, 0)
		if complexListLabel != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(complexListLabel), WM_SETFONT, uintptr(hFontBold), 1)
		}
		complexMetricCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 700, 128, 186, 220, h, idComplexMetric)
		for _, metric := range complexMetrics {
			pSendMessageW.Call(uintptr(complexMetricCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(metric.label))))
		}
		pSendMessageW.Call(uintptr(complexMetricCombo), CB_SETCURSEL, 3, 0) // corrosion rate has a visible threshold in the examples
		complexTrendCheck = modalCreate("BUTTON", "Линия тренда", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX, 24, 208, 170, 28, h, idComplexTrend)
		complexTrendTypeCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 200, 205, 230, 300, h, idComplexTrendType)
		for _, label := range []string{"Линейная", "Логарифмическая", "Экспоненциальная", "Полиномиальная (2)", "Скользящее среднее"} {
			pSendMessageW.Call(uintptr(complexTrendTypeCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(label))))
		}
		pSendMessageW.Call(uintptr(complexTrendTypeCombo), CB_SETCURSEL, 0, 0)
		pSendMessageW.Call(uintptr(complexTrendTypeCombo), 0x0160, 300, 0)
		complexZoomOutButton = modalCreate("BUTTON", "−", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 38, 34, h, idComplexZoomOut)
		complexZoomInButton = modalCreate("BUTTON", "+", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 38, 34, h, idComplexZoomIn)
		complexHistoryBackButton = modalCreate("BUTTON", "←", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 38, 34, h, idComplexHistoryBack)
		complexHistoryNextButton = modalCreate("BUTTON", "→", WS_CHILD|WS_TABSTOP|BS_OWNERDRAW, 0, 0, 38, 34, h, idComplexHistoryNext)
		updateComplexHistoryButtons()
		complexBackButton = modalCreate("BUTTON", "Назад к объектам", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 12, 170, 34, h, idComplexBack)
		complexForwardButton = modalCreate("BUTTON", "Вперёд к лаборатории", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 200, 12, 190, 34, h, idComplexForward)
		complexList = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|WS_TABSTOP|0x0008, 24, 156, 862, 226, h, 0)
		complexAllButton = modalCreate("BUTTON", "Выбрать все даты", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 394, 170, 38, h, idComplexAll)
		complexRunButton = modalCreate("BUTTON", "Анализ выбранных", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 206, 394, 280, 38, h, idComplexRun)
		complexVisualButton = modalCreate("BUTTON", "Инфографика", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 494, 394, 214, 38, h, idComplexVisual)
		complexExportButton = modalCreate("BUTTON", "Выгрузить DOCX / Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 714, 394, 200, 38, h, idComplexExport)
		complexExcelButton = modalCreate("BUTTON", "Графики и данные в Excel", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 714, 438, 200, 38, h, idComplexExcel)
		// Closing, minimizing and maximizing use the standard Windows caption icons.
		complexCloseButton = 0
		complexMaximizeButton = 0
		complexResult = modalCreate("RichEdit20W", "", WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL|ES_READONLY, 24, 444, 862, 290, h, 0)
		if complexResult != 0 && hFont != 0 {
			pSendMessageW.Call(uintptr(complexResult), WM_SETFONT, uintptr(hFont), 1)
		}
		complexChart = createLaboratoryInfographicCanvas(h, idComplexChart, 24, 444, 862, 290, false)
		complexLegendScrollbar = modalCreate("SCROLLBAR", "", WS_CHILD|0x0001, 0, 0, 20, 290, h, idComplexLegendScroll)
		complexComment = modalCreate("RichEdit20W", "", WS_CHILD|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL|ES_READONLY, 24, 600, 862, 130, h, 0)
		complexCommentHeading = modalCreate("STATIC", "ВЫВОД ДЛЯ ТЕХНОЛОГА", WS_CHILD|SS_LEFT, 24, 574, 862, 26, h, 0)
		show(complexComment, false)
		if complexComment != 0 && hFont != 0 {
			pSendMessageW.Call(uintptr(complexComment), WM_SETFONT, uintptr(hFont), 1)
		}
		if complexCommentHeading != 0 && hFontBold != 0 {
			pSendMessageW.Call(uintptr(complexCommentHeading), WM_SETFONT, uintptr(hFontBold), 1)
		}
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		layoutComplexDialog(rc.Right, rc.Bottom)
		complexFind(getText(complexSearch))
		return 0
	case WM_SIZE:
		layoutComplexDialog(int32(loWord(lParam)), int32(hiWord(lParam)))
		pRedrawWindow.Call(hwnd, 0, 0, 0x0001|0x0004|0x0080)
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case idComplexFind:
			complexFind(getText(complexSearch))
			return 0
		case idComplexObject:
			if hiWord(wParam) == 1 { // LBN_SELCHANGE
				complexLoadObjects(complexObjectSelection())
			}
			return 0
		case idComplexObjectsAll, idComplexObjectsClear:
			selectAll := int(loWord(wParam)) == idComplexObjectsAll
			pSendMessageW.Call(uintptr(complexObjectCombo), 0x0185, uintptr(0), ^uintptr(0)) // LB_SETSEL(-1) clears all
			if selectAll {
				pSendMessageW.Call(uintptr(complexObjectCombo), 0x0185, 1, ^uintptr(0))
			}
			complexLoadObjects(complexObjectSelection())
			return 0
		case idComplexAll:
			for i := range complexPasses {
				pSendMessageW.Call(uintptr(complexList), 0x0185, 1, uintptr(i))
			}
			return 0
		case idComplexRun:
			if complexAnalysisBusy {
				return 0
			}
			var selected []LabPassRecord
			for i, p0 := range complexPasses {
				v, _, _ := pSendMessageW.Call(uintptr(complexList), 0x0187, uintptr(i), 0)
				if v != 0 {
					selected = append(selected, p0)
				}
			}
			complexSelected = selected
			invalidateComplexChartGroups()
			resetComplexSelectionHistory()
			if len(selected) < 2 {
				setComplexResultText("Выберите минимум две сохранённые диагностики одного объекта.")
			} else {
				complexAnalysisBusy = true
				pEnableWindow.Call(uintptr(complexRunButton), 0)
				setText(complexRunButton, "Анализ…")
				setComplexResultText("Выполняется комплексный анализ выбранных диагностик. Окно остаётся доступным.")
				done := complexAnalysisDone
				selection := append([]LabPassRecord(nil), selected...)
				go func() {
					done <- complexAnalysisResult{content: complexConclusion(selection)}
					pPostMessageW.Call(hwnd, wmComplexAnalysisDone, 0, 0)
				}()
			}
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			pRedrawWindow.Call(hwnd, 0, 0, 0x0001|0x0004|0x0080)
			return 0
		case idComplexVisual:
			complexShowChart = !complexShowChart
			show(complexChart, complexShowChart)
			show(complexResult, !complexShowChart)
			show(complexComment, complexShowChart)
			show(complexCommentHeading, complexShowChart)
			if complexShowChart {
				setText(complexVisualButton, "Текстовый вывод")
				pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			} else {
				setText(complexVisualButton, "Инфографика")
			}
			var client RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
			layoutComplexDialog(client.Right-client.Left, client.Bottom-client.Top)
			pRedrawWindow.Call(hwnd, 0, 0, 0x0001|0x0004|0x0080|0x0100)
			return 0
		case idComplexMetric:
			if hiWord(wParam) == 1 {
				invalidateComplexChartGroups()
				complexHighlightedGroup = -1
				complexZoom = 1
				refreshComplexComment()
				pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			}
			return 0
		case idComplexTrend:
			v, _, _ := pSendMessageW.Call(uintptr(complexTrendCheck), BM_GETCHECK, 0, 0)
			complexTrendEnabled = v != 0
			complexTrendObjectKey = ""
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			return 0
		case idComplexChart:
			if hiWord(wParam) == STN_DBLCLK {
				if point, ok := complexChartPointFromCursor(); ok {
					if index := complexLegendGroupAtPoint(point); index >= 0 {
						complexIsolateObjectAt(index)
					} else {
						complexShowSeriesLabel(point)
					}
				}
			} else if hiWord(wParam) == 0 {
				if point, ok := complexChartPointFromCursor(); ok && complexSelectSeriesAtPoint(point) < 0 {
					complexHighlightedGroup = -1
					pInvalidateRect.Call(uintptr(complexChart), 0, 1)
				}
			}
			return 0
		case idComplexTrendType:
			if hiWord(wParam) == 1 {
				pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			}
			return 0
		case idComplexExcel:
			if len(complexSelected) == 0 {
				msg("Экспорт инфографики", "Сначала выберите диагностики и нажмите «Анализ выбранных».", MB_OK|MB_ICONWARNING)
				return 0
			}
			if complexExcelBusy {
				return 0
			}
			complexExcelBusy = true
			pEnableWindow.Call(uintptr(complexExcelButton), 0)
			setText(complexExcelButton, "Формирование…")
			startupLoadingHeadline = "ФОРМИРОВАНИЕ EXCEL"
			complexLoadingWindow = showStartupLoadingWindow()
			setStartupLoadingStage(3)
			setStartupLoadingProgress("Создаём таблицы, графики и формулы Excel…")
			exportComplexInfographicExcel(append([]LabPassRecord(nil), complexSelected...))
			return 0
		case idComplexBack:
			complexDone = true
			pDestroyWindowAuth.Call(hwnd)
			activatePage(ID_OBJECTS, false)
			return 0
		case idComplexForward:
			complexDone = true
			pDestroyWindowAuth.Call(hwnd)
			activatePage(ID_LAB, false)
			return 0
		case idComplexZoomOut, idComplexZoomIn:
			if int(loWord(wParam)) == idComplexZoomIn {
				complexZoom = minFloat(complexZoom*1.35, 8)
			} else {
				complexZoom /= 1.35
				if complexZoom < 1 {
					complexZoom = 1
				}
			}
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			return 0
		case idComplexHistoryBack:
			complexUndoSelection()
			return 0
		case idComplexHistoryNext:
			complexRedoSelection()
			return 0
		case idComplexExport:
			if complexExportBusy {
				return 0
			}
			if len(complexSelected) < 2 {
				setComplexResultText("Сначала выберите минимум две диагностики и нажмите «Проанализировать выбранные».")
				show(complexResult, true)
				show(complexChart, false)
				show(complexComment, false)
				show(complexCommentHeading, false)
				return 0
			}
			path, format := chooseComplexReportPath(syscall.Handle(hwnd))
			if path == "" {
				return 0
			}
			content := complexConclusion(append([]LabPassRecord(nil), complexSelected...))
			complexExportBusy = true
			pEnableWindow.Call(uintptr(complexExportButton), 0)
			setText(complexExportButton, "Сохранение…")
			setComplexResultText("Формирование документа выполняется. Окно остаётся доступным.")
			show(complexResult, true)
			show(complexChart, false)
			show(complexComment, false)
			show(complexCommentHeading, false)
			complexShowChart = false
			done := complexExportDone
			go func() {
				var err error
				if format == "docx" {
					err = writeLaboratoryDOCXReport(path, content)
				} else {
					err = writeLaboratoryXLSXReport(path, content)
				}
				done <- complexExportResult{path, content, err}
				pPostMessageW.Call(hwnd, wmComplexExportDone, 0, 0)
			}()
			return 0
		}
	case WM_VSCROLL:
		if syscall.Handle(lParam) == complexLegendScrollbar {
			var rc RECT
			pGetClientRect.Call(uintptr(complexChart), uintptr(unsafe.Pointer(&rc)))
			_, totalRows, visibleRows := complexLegendViewport(len(complexChartGroups()), rc.Bottom-rc.Top)
			maximum := maxInt(0, totalRows-visibleRows)
			switch int(loWord(wParam)) {
			case 0: // SB_LINEUP
				complexLegendScrollRow--
			case 1: // SB_LINEDOWN
				complexLegendScrollRow++
			case 2: // SB_PAGEUP
				complexLegendScrollRow -= maxInt(1, visibleRows)
			case 3: // SB_PAGEDOWN
				complexLegendScrollRow += maxInt(1, visibleRows)
			case 4, 5: // SB_THUMBPOSITION / SB_THUMBTRACK
				complexLegendScrollRow = int(hiWord(wParam))
			}
			complexLegendScrollRow = maxInt(0, minInt(complexLegendScrollRow, maximum))
			pSetScrollPos.Call(uintptr(complexLegendScrollbar), 1, uintptr(complexLegendScrollRow), 1)
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
			return 0
		}
	case WM_CONTEXTMENU:
		if syscall.Handle(wParam) == complexChart {
			point, ok := complexChartPointFromCursor()
			now := time.Now()
			if ok && !complexLastRightClick.IsZero() && now.Sub(complexLastRightClick) <= 550*time.Millisecond {
				complexLastRightClick = time.Time{}
				complexHandleRightDoubleClick(syscall.Handle(hwnd), point)
			} else {
				complexLastRightClick = now
			}
			return 0
		}
	case wmComplexExcelDone:
		select {
		case result := <-complexExcelDone:
			complexExcelBusy = false
			if complexLoadingWindow != 0 {
				pDestroyWindowAuth.Call(uintptr(complexLoadingWindow))
				complexLoadingWindow = 0
			}
			startupLoadingHeadline = "ВХОД ВЫПОЛНЕН"
			setText(complexExcelButton, "Графики и данные в Excel")
			pEnableWindow.Call(uintptr(complexExcelButton), 1)
			if result.err != nil {
				msg("Графики в Excel", "Не удалось создать Excel: "+result.err.Error(), MB_OK|MB_ICONERROR)
			} else {
				notifyDetachedExcelCopy(result.path)
				openExternal(result.path)
			}
		default:
		}
		return 0
	case wmComplexExportDone:
		select {
		case result := <-complexExportDone:
			complexExportBusy = false
			setText(complexExportButton, "Выгрузить DOCX / Excel")
			pEnableWindow.Call(uintptr(complexExportButton), 1)
			if result.err != nil {
				setComplexResultText("Не удалось выгрузить отчёт: " + result.err.Error())
			} else {
				setComplexResultText("Отчёт сохранён: " + result.path + "\r\n\r\n" + result.content)
			}
			pRedrawWindow.Call(hwnd, 0, 0, 0x0001|0x0004|0x0080)
		default:
		}
		return 0
	case wmComplexAnalysisDone:
		select {
		case result := <-complexAnalysisDone:
			complexAnalysisBusy = false
			setText(complexRunButton, "Анализ выбранных")
			pEnableWindow.Call(uintptr(complexRunButton), 1)
			setComplexResultText(result.content)
			refreshComplexComment()
			pInvalidateRect.Call(uintptr(complexChart), 0, 1)
		default:
		}
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil && drawComplexButton(di) {
			return 1
		}
		if di != nil && int(di.CtlID) == idComplexChart {
			drawComplexInfographic(di, complexSelected)
			return 1
		}
	case 0x0010: // WM_CLOSE from the native caption close icon.
		complexDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 780
			mmi.PtMinTrackSize.Y = 680
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}
func openLaboratoryComplexAnalysisDialog() {
	complexDone = false
	complexShowChart = false
	complexExportBusy = false
	complexAnalysisBusy = false
	complexLegendScrollRow = 0
	complexHighlightedGroup = -1
	invalidateComplexChartGroups()
	resetComplexSelectionHistory()
	complexTrendObjectKey = ""
	complexLastRightClick = time.Time{}
	complexExportDone = make(chan complexExportResult, 1)
	complexAnalysisDone = make(chan complexAnalysisResult, 1)
	complexExcelDone = make(chan complexExportResult, 1)
	complexExcelBusy = false
	complexLoadingWindow = 0
	complexInitialPassID = ""
	if i := selectedRow(labHistoryList); i >= 0 && i < len(labHistoryEntries) && labHistoryEntries[i].Kind == "pass" {
		complexInitialPassID = labHistoryEntries[i].Pass.PassID
	}
	prepareComplexAnalysisIndex()
	metrics := user32.NewProc("GetSystemMetrics")
	screenW, _, _ := metrics.Call(0)
	screenH, _, _ := metrics.Call(1)
	windowW := min32(1240, max32(820, int32(screenW)-64))
	windowH := min32(900, max32(680, int32(screenH)-72))
	cls := p("ROSALaboratoryComplexDialogV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(complexWndProc), HInstance: syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1), LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	registerLaboratoryInfographicCanvas()
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — комплексная диагностика"))), WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN_FRAME, 0x80000000, 0x80000000, uintptr(windowW), uintptr(windowH), uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		return
	}
	complexWindowHandle = syscall.Handle(hwnd)
	var m MSG
	for !complexDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	complexWindowHandle = 0
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
