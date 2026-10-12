//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

// The fund history is read-only. The calculation model and persistence format
// remain unchanged; rows are grouped solely for presentation.
type economicsFundObjectRow struct {
	ObjectKey   string
	Schemes     []AlternativeMethodScenario
	Updated     string
	BestBenefit float64
	HasBenefit  bool
	MaxMRP      float64
	Calculated  int
}

var economicsFundRows []economicsFundObjectRow
var econFundFilterMode int // 0 all, 1 needs calculation, 2 calculated, 3 positive benefit

func economicsFundFilterGroups(groups []economicsFundObjectRow, mode int) []economicsFundObjectRow {
	filtered := make([]economicsFundObjectRow, 0, len(groups))
	for _, g := range groups {
		include := false
		switch mode {
		case 1:
			include = g.Calculated < len(g.Schemes)
		case 2:
			include = g.Calculated > 0
		case 3:
			include = g.HasBenefit && g.BestBenefit > 0
		default:
			include = true
		}
		if include {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

func economicsFundCountWord(n int, one, few, many string) string {
	if n%100 >= 11 && n%100 <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

func economicsFundSummaryText(groups []economicsFundObjectRow) string {
	allSchemes, calculated, positiveObjects := 0, 0, 0
	for _, g := range groups {
		allSchemes += len(g.Schemes)
		calculated += g.Calculated
		if g.HasBenefit && g.BestBenefit > 0 {
			positiveObjects++
		}
	}
	return fmt.Sprintf("ФОНД: %d %s · %d %s · %d %s · положительный эффект: %d",
		len(groups), economicsFundCountWord(len(groups), "объект", "объекта", "объектов"),
		allSchemes, economicsFundCountWord(allSchemes, "схема", "схемы", "схем"),
		calculated, economicsFundCountWord(calculated, "расчёт", "расчёта", "расчётов"), positiveObjects)
}

func changeEconomicsFundFilter() {
	if econFundFilterCombo == 0 {
		return
	}
	selected, _, _ := pSendMessageW.Call(uintptr(econFundFilterCombo), CB_GETCURSEL, 0, 0)
	if selected > 3 {
		return
	}
	econFundFilterMode = int(selected)
	refreshEconomicsFundArchive()
}

func economicsFundGrouping(rows []AlternativeMethodScenario) []economicsFundObjectRow {
	groups := make([]economicsFundObjectRow, 0)
	byKey := make(map[string]int)
	for _, s := range rows {
		key := strings.TrimSpace(s.ObjectKey)
		if key == "" {
			continue
		}
		id := alternativeObjectIdentity(key)
		if id == "" {
			id = key
		}
		index, found := byKey[id]
		if !found {
			index = len(groups)
			byKey[id] = index
			groups = append(groups, economicsFundObjectRow{ObjectKey: key})
		}
		group := &groups[index]
		group.Schemes = append(group.Schemes, s)
		if s.UpdatedAt > group.Updated {
			group.Updated = s.UpdatedAt
		}
		if s.ForecastBaseDays > group.MaxMRP {
			group.MaxMRP = s.ForecastBaseDays
		}
		if economicsScenarioCalculated(s) {
			group.Calculated++
			if !group.HasBenefit || s.NetBenefitK > group.BestBenefit {
				group.HasBenefit = true
				group.BestBenefit = s.NetBenefitK
			}
		}
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Updated != groups[j].Updated {
			return groups[i].Updated > groups[j].Updated
		}
		return groups[i].ObjectKey < groups[j].ObjectKey
	})
	return groups
}

func economicsScenarioCalculated(s AlternativeMethodScenario) bool {
	return strings.TrimSpace(s.EconomicsCalculated) != "" &&
		strings.EqualFold(strings.TrimSpace(s.EconomicsStatus), "Расчёт актуален") &&
		!strings.EqualFold(strings.TrimSpace(s.Status), "Исходные данные изменились")
}

func refreshEconomicsFundArchive() {
	if econCompareList == 0 {
		return
	}
	ensureAlternativeScenariosLoaded()
	allFundRows := economicsFundGrouping(alternativeScenarios)
	setText(econModeInfo, economicsFundSummaryText(allFundRows))
	economicsFundRows = economicsFundFilterGroups(allFundRows, econFundFilterMode)
	names := readableObjectKeyIndex()
	listBeginUpdate(econCompareList)
	defer listEndUpdate(econCompareList)
	listClear(econCompareList)
	for i, g := range economicsFundRows {
		mrp, benefit := econReferenceMRP(g.MaxMRP), "—"
		if g.HasBenefit {
			benefit = fmt.Sprintf("%.0f", g.BestBenefit)
		}
		status := "Требует расчёта"
		if g.Calculated == len(g.Schemes) {
			status = "Все схемы рассчитаны"
		} else if g.Calculated > 0 {
			status = "Частично рассчитано"
		}
		date := "—"
		if len(g.Updated) >= 10 {
			date = g.Updated[:10]
		} else if g.Updated != "" {
			date = g.Updated
		}
		listAdd(econCompareList, i, []string{
			readableObjectKeyFromIndex(g.ObjectKey, names), fmt.Sprintf("%d", len(g.Schemes)),
			fmt.Sprintf("%d из %d", g.Calculated, len(g.Schemes)), mrp, benefit, date, status,
		})
	}
	if len(economicsFundRows) == 0 {
		message := "По выбранному фильтру объектов нет"
		if len(allFundRows) == 0 {
			message = "История пока не накоплена: сохраните схемы в разделе 4"
		}
		listAdd(econCompareList, 0, []string{message, "—", "—", "—", "—", "—", "—"})
	}
}

const (
	economicsFundArchiveCloseID = 9758
	economicsFundArchiveRestoreID = 14969
)

var (
	economicsFundDialogDone        bool
	economicsFundDialogObject      economicsFundObjectRow
	economicsFundDialogList        syscall.Handle
	economicsFundDialogDetail      syscall.Handle
	economicsFundDialogDetailTitle syscall.Handle
	economicsFundDialogAccentFont  syscall.Handle
	economicsFundDialogTitle       syscall.Handle
	economicsFundDialogSubtitle    syscall.Handle
	economicsFundDialogClose       syscall.Handle
	economicsFundDialogRestore     syscall.Handle
	economicsFundDialogSelected    int
)

func layoutEconomicsFundDialog(hwnd uintptr, w, h int32) {
	if w <= 0 || h <= 0 {
		return
	}
	margin := int32(22)
	setPos(economicsFundDialogTitle, margin, 18, max32(100, w-margin*2), 32)
	setPos(economicsFundDialogSubtitle, margin, 55, max32(100, w-margin*2), 44)
	listY := int32(112)
	closeY := max32(108, h-margin-40)
	usable := max32(330, closeY-listY-72)
	listHeight := max32(170, usable*53/100)
	setPos(economicsFundDialogList, margin, listY, max32(100, w-margin*2), listHeight)
	detailTitleY := listY + listHeight + 12
	setPos(economicsFundDialogDetailTitle, margin, detailTitleY, max32(100, w-margin*2), 28)
	detailY := detailTitleY + 32
	setPos(economicsFundDialogDetail, margin, detailY, max32(100, w-margin*2), max32(115, closeY-detailY-10))
	setPos(economicsFundDialogRestore, margin, closeY, 250, 38)
	setPos(economicsFundDialogClose, max32(margin, w-182), closeY, 160, 38)
	layoutEconomicsFundArchiveColumns(w)
}

func layoutEconomicsFundArchiveColumns(w int32) {
	if economicsFundDialogList == 0 {
		return
	}
	// Preserve the intended visual proportions while fitting all nine values in
	// the dialog. The former fixed widths totaled 1,363 px in a ~1,060 px client.
	base := [...]int32{100, 300, 108, 148, 130, 175, 153, 151, 198}
	var total int32
	for _, width := range base {
		total += width
	}
	available := max32(100, w-44)
	var used int32
	for i, width := range base {
		columnWidth := int32(int64(available) * int64(width) / int64(total))
		if i == len(base)-1 {
			columnWidth = available - used
		}
		setListColumnWidth(economicsFundDialogList, i, columnWidth)
		used += columnWidth
	}
	// The lower details pane is a word-wrapped EDIT, not a ListView.
	// Its entire client width is used for paragraphs; no column widths needed.
}

func economicsFundMethodLabel(index int) string {
	letters := [...]string{"А", "Б", "В", "Г", "Д", "Е", "Ж", "З", "И", "К", "Л", "М", "Н", "О", "П", "Р", "С", "Т", "У", "Ф", "Х", "Ц", "Ч", "Ш", "Щ", "Э", "Ю", "Я"}
	if index >= 0 && index < len(letters) {
		return "МЕТОД " + letters[index]
	}
	return fmt.Sprintf("МЕТОД %d", index+1)
}

// Full-value rendering for the archived method's economics, including long
// sources and dated teaching history. A multiline EDIT wraps long values, and
// the vertical scroll retains every paragraph without truncation.
func economicsFundDetailsText(rows [][]string) string {
	var result strings.Builder
	for _, row := range rows {
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		result.WriteString(strings.TrimSpace(row[0]))
		if len(row) > 1 {
			result.WriteString(":\r\n")
			result.WriteString(strings.TrimSpace(row[1]))
		}
		result.WriteString("\r\n\r\n")
	}
	return strings.TrimSpace(result.String())
}

func economicsFundDialogSelectScheme(index int) {
	if economicsFundDialogDetail == 0 || index < 0 || index >= len(economicsFundDialogObject.Schemes) {
		return
	}
	economicsFundDialogSelected = index
	s := economicsFundDialogObject.Schemes[index]
	methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
	if methods == "" {
		methods = "Не выбраны"
	}
	known := economicsScenarioCalculated(s)
	setText(economicsFundDialogDetailTitle, economicsFundMethodLabel(index)+" · "+strings.TrimSpace(s.Name))
	rows := [][]string{
		{"Статус расчёта", econReferenceStatus(s)},
		{"Методы / мероприятия", methods},
		{"Доступность", dash(s.Availability)},
		{"Пояснение доступности", dash(s.AvailabilityNote)},
		{"Прогноз МРП, сут.", econReferenceMRP(s.ForecastBaseDays)},
		{"OPEX, руб./год", econReferenceMoney(s.AnnualOPEXRub, known || s.AnnualOPEXRub != 0)},
		{"CAPEX, руб.", econReferenceMoney(s.OneTimeCAPEXRub, known || s.OneTimeCAPEXRub != 0)},
		{"Чистый эффект, тыс. руб.", econReferenceMoney(s.NetBenefitK, known)},
		{"Выгода / затраты", "—"},
		{"Окупаемость, мес.", "—"},
		{"Основание выбора", dash(s.SelectionBasis)},
	}
	if known {
		rows[8][1] = fmt.Sprintf("%.2f", s.BenefitCostRatio)
		if s.PaybackMonths != nil {
			rows[9][1] = fmt.Sprintf("%.1f", *s.PaybackMonths)
		}
	}
	// Reuse the same 20-well failure IDs and lab-pass IDs used in the other
	// sections. This is a synthetic, read-only three-year drilldown.
	if history := teachingEconomicsHistory816(s.ObjectKey, s.ID); len(history) > 0 {
		rows = append(rows, []string{"ИСТОРИЯ ЗА 3 ГОДА", "Учебные расчёты; не производственные данные"})
		for _, point := range history {
			status := "Расчёт не выполнен"
			if point.Result.Status == "PASS" {
				status = fmt.Sprintf("МРП %.0f → %.0f сут.; эффект %+.1f тыс. руб. / 12 мес.; CAPEX %.0f; OPEX %.0f руб./год", point.BaselineMRP, point.ForecastMRP, point.Result.Selected.NetBenefitForHorizonK, point.CAPEXRub, point.OPEXRub)
			}
			rows = append(rows, []string{point.Date + " · " + point.FailureID, status})
			rows = append(rows, []string{"Основание: " + point.LabPassID, "Формула " + point.Result.FormulaVersion + " · синтетические значения"})
		}
	}
	setText(economicsFundDialogDetail, economicsFundDetailsText(rows))
	// Switching schemes starts at the beginning of the readable report.
	pSendMessageW.Call(uintptr(economicsFundDialogDetail), EM_SETSEL, 0, 0)
}

func economicsFundDialogApplySelected(hwnd uintptr) {
	index := economicsFundDialogSelected
	if index < 0 || index >= len(economicsFundDialogObject.Schemes) {
		return
	}
	stored, err := restoreEconomicsFromArchive(economicsFundDialogObject.Schemes[index])
	if err != nil {
		msg("Предыдущие расчёты", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	if !stored {
		// Old records have forecasts and CAPEX/OPEX but may lack the exact
		// historical repair cost, horizon or inflation. Do not make them up.
		msg("Предыдущие расчёты", "Прогноз и затраты перенесены. Полный снимок исходных параметров для этого старого расчёта не сохранён; проверьте оставшиеся поля.", MB_OK|MB_ICONINFORMATION)
	}
	economicsFundDialogDone = true
	pDestroyWindowAuth.Call(hwnd)
}

func economicsFundArchiveWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		economicsFundDialogTitle = modalCreate("STATIC", "ЭКОНОМИЧЕСКИЕ СХЕМЫ ОБЪЕКТА", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 22, 18, 1000, 32, parent, 0)
		economicsFundDialogSubtitle = modalCreate("STATIC", readableObjectKey(economicsFundDialogObject.ObjectKey)+" · архив альтернатив А–Д · двойной щелчок по схеме — подставить данные при совпадении объекта", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 22, 55, 1000, 44, parent, 0)
		economicsFundDialogDetailTitle = modalCreate("STATIC", "ВЫБЕРИТЕ МЕТОД ДЛЯ ПРОСМОТРА ЭКОНОМИКИ", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 22, 438, 1000, 28, parent, 0)
		font, _, _ := pCreateFontW.Call(^uintptr(19-1), 0, 0, 0, 700, 1, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
		economicsFundDialogAccentFont = syscall.Handle(font)
		if hFontSystemName != 0 {
			pSendMessageW.Call(uintptr(economicsFundDialogTitle), WM_SETFONT, uintptr(hFontSystemName), 1)
		}
		if economicsFundDialogAccentFont != 0 {
			pSendMessageW.Call(uintptr(economicsFundDialogDetailTitle), WM_SETFONT, uintptr(economicsFundDialogAccentFont), 1)
		} else if hFontHeaderBold != 0 {
			pSendMessageW.Call(uintptr(economicsFundDialogDetailTitle), WM_SETFONT, uintptr(hFontHeaderBold), 1)
		}
		columns := []struct {
			name  string
			width int32
		}{
			{"Схема", 100}, {"Методы / мероприятия", 300}, {"МРП, сут.", 108},
			{"OPEX, руб./год", 148}, {"CAPEX, руб.", 130}, {"Чистый эффект, тыс. руб.", 175},
			{"Выгода / затраты", 153}, {"Окупаемость, мес.", 151}, {"Статус", 198},
		}
		economicsFundDialogList = modalCreate("SysListView32", "", WS_CHILD|WS_VISIBLE|WS_BORDER|LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS, 22, 108, 1000, 325, parent, 0)
		pSendMessageW.Call(uintptr(economicsFundDialogList), LVM_SETEXTENDEDLISTVIEWSTYLE,
			LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER,
			LVS_EX_FULLROWSELECT|LVS_EX_GRIDLINES|LVS_EX_DOUBLEBUFFER)
		for i, c := range columns {
			col := LVCOLUMN{Mask: LVCF_TEXT | LVCF_WIDTH | LVCF_FMT, Fmt: LVCFMT_LEFT, Cx: c.width, PszText: p(c.name)}
			pSendMessageW.Call(uintptr(economicsFundDialogList), LVM_INSERTCOLUMNW, uintptr(i), uintptr(unsafe.Pointer(&col)))
		}
		for i, s := range economicsFundDialogObject.Schemes {
			costKnown := economicsScenarioCalculated(s)
			methods := strings.Join(alternativeScenarioMethodsForCalculation(s), " + ")
			if methods == "" {
				methods = "Не указаны"
			}
			ratio, payback := "—", "—"
			if costKnown {
				ratio = fmt.Sprintf("%.2f", s.BenefitCostRatio)
				if s.PaybackMonths != nil {
					payback = fmt.Sprintf("%.1f", *s.PaybackMonths)
				}
			}
			listAdd(economicsFundDialogList, i, []string{
				economicsFundMethodLabel(i), methods, econReferenceMRP(s.ForecastBaseDays),
				econReferenceMoney(s.AnnualOPEXRub, costKnown || s.AnnualOPEXRub != 0),
				econReferenceMoney(s.OneTimeCAPEXRub, costKnown || s.OneTimeCAPEXRub != 0),
				econReferenceMoney(s.NetBenefitK, costKnown), ratio, payback, econReferenceStatus(s),
			})
		}
		// Native multiline EDIT wraps long descriptions and retains all their
		// content behind a vertical scroll bar. ListView cannot wrap cells and
		// previously selected an oversized bold font into 20-pixel rows.
		economicsFundDialogDetail = modalCreate("EDIT", "", WS_CHILD|WS_VISIBLE|WS_BORDER|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY, 22, 468, 1000, 160, parent, 0)
		if hFontButtonCompact != 0 {
			pSendMessageW.Call(uintptr(economicsFundDialogDetail), WM_SETFONT, uintptr(hFontButtonCompact), 1)
		} else if hFont != 0 {
			pSendMessageW.Call(uintptr(economicsFundDialogDetail), WM_SETFONT, uintptr(hFont), 1)
		}
		if len(economicsFundDialogObject.Schemes) > 0 {
			economicsFundDialogSelectScheme(0)
		}
		economicsFundDialogRestore = modalCreate("BUTTON", "Подставить прежний расчёт", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 22, 668, 250, 38, parent, economicsFundArchiveRestoreID)
		economicsFundDialogClose = modalCreate("BUTTON", "Закрыть", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 850, 668, 160, 38, parent, economicsFundArchiveCloseID)
		layoutEconomicsFundDialog(hwnd, 1160, 700)
		return 0
	case WM_SIZE:
		layoutEconomicsFundDialog(hwnd, int32(uint16(lParam)), int32(uint16(lParam>>16)))
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 900
			mmi.PtMinTrackSize.Y = 650
		}
		return 0
	case WM_CTLCOLORSTATIC:
		if syscall.Handle(lParam) == economicsFundDialogDetailTitle {
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(183, 20, 37))
			return uintptr(brushLight)
		}
	case WM_NOTIFY:
		if lParam != 0 {
			hdr := (*NMHDR)(unsafe.Pointer(lParam))
			if hdr.HwndFrom == economicsFundDialogList && hdr.Code == NM_CUSTOMDRAW {
				draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
				switch draw.NMCD.DrawStage {
				case CDDS_PREPAINT:
					return CDRF_NOTIFYITEMDRAW
				case CDDS_ITEMPREPAINT:
					return CDRF_NOTIFYSUBITEMDRAW
				case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
					if draw.SubItem == 0 {
						draw.ClrText = uint32(rgb(183, 20, 37))
						// Respect native row height: do not select an oversized font
						// into a ListView DC (it clips above/below the row borders).
						return CDRF_NEWFONT
					}
				}
			}
			if hdr.HwndFrom == economicsFundDialogList && (hdr.Code == NM_CLICK || hdr.Code == NM_DBLCLK) {
				act := (*NMITEMACTIVATE)(unsafe.Pointer(lParam))
				index := selectedRow(economicsFundDialogList)
				if act != nil && act.IItem >= 0 {
					index = int(act.IItem)
				}
				economicsFundDialogSelectScheme(index)
				if hdr.Code == NM_DBLCLK {
					economicsFundDialogApplySelected(hwnd)
				}
				return 0
			}
		}
	case WM_COMMAND:
		if int(wParam&0xffff) == economicsFundArchiveRestoreID {
			economicsFundDialogApplySelected(hwnd)
			return 0
		}
		if int(wParam&0xffff) == economicsFundArchiveCloseID {
			economicsFundDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case 0x0010: // WM_CLOSE
		economicsFundDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	case WM_DESTROY:
		economicsFundDialogDone = true
		if economicsFundDialogAccentFont != 0 {
			pDeleteObject.Call(uintptr(economicsFundDialogAccentFont))
			economicsFundDialogAccentFont = 0
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openEconomicsFundArchiveAtRow(row int) {
	if row < 0 || row >= len(economicsFundRows) {
		return
	}
	economicsFundDialogObject = economicsFundRows[row]
	// Viewing a different object is deliberately not the same as selecting it.
	economicsFundDialogDone = false
	cls := p("ROSAEconomicsFundArchiveV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc: syscall.NewCallback(economicsFundArchiveWndProc),
		HInstance:   syscall.Handle(inst), HbrBackground: syscall.Handle(5 + 1),
		LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — экономические схемы объекта"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1160, 740, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Экономическая история", "Не удалось открыть сохранённые схемы.", MB_OK|MB_ICONWARNING)
		return
	}
	var m MSG
	for !economicsFundDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &m) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
}
