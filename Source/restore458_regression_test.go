//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestHeaderBrandCaptionFitsAboveSeparator(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`setPos(hBrandSignature, 8, 86, 178, 34)`,
		`hFontBrandCaption = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(10-1)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing caption no-clip guard %q", want)
		}
	}
}

func TestDashboardUsesOrderedFourSectionVisualHierarchy(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`1. ОБНОВЛЕНИЕ ТЕХРЕЖИМА`, `2. ЗАКРЕПЛЁННЫЙ ОБЪЕКТ`, `3. ВЫБОР ОБЪЕКТА`,
		`4. ЖУРНАЛ РАБОТ`, `Лабораторные данные`, `Классы осложнений`, `Экономическая оценка`, `Общее заключение`, `Заполнение, %`,
		`Двойной щелчок — открыть сохранённую запись`, `openDashboardHistoryPass`,
		`dashboardImportAccent`, `dashboardSelectionAccent`, `dashboardObjectAccent`,
		`techImportList = 0`,
		`setPos(objectSelectFrame, confirmX-4, py(188), buttonW+8, 50)`,
		`setPos(dashboardNote, 242, py(286), contentW, 52)`,
		`buttonW := min32(280, max32(180, (contentW-buttonGap*3)/4))`,
		`rowX := int32(242) + max32(0, (contentW-rowW)/2)`,
		`techImportFileLabel = 0`, `techImportSummary = 0`,
		`setPos(dashboardObjectTitle, 258, py(252), contentW-16, 30)`,
		`setPos(objectsList, 242, objectY, contentW, objectH)`,
		`setPos(recentList, 242, historyY, contentW, max32(90, h-historyY-12))`,
		`layoutDashboardFundColumns(contentW)`,
		`layoutDashboardHistoryColumns(contentW)`,
		`dashboardHistoryCustomDraw`,
		`hFontSection = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(20-1)`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing dashboard visual guard %q", want)
		}
	}
	var buttonPriority string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "primary := id ==") && strings.Contains(line, "ID_OBJECT_CONFIRM") {
			buttonPriority = line
			break
		}
	}
	if buttonPriority == "" || strings.Contains(buttonPriority, "ID_TECH_CHOOSE") || strings.Contains(buttonPriority, "ID_TECH_PREVIEW") || strings.Contains(buttonPriority, "ID_TECH_APPLY") {
		t.Fatalf("only the final object-confirmation action should use the primary red style: %s", buttonPriority)
	}
	if got := displayMRP("97.999999"); got != "98" {
		t.Fatalf("displayMRP = %q", got)
	}
}

func TestDashboardJournalHasMatchingAccentCenteredHeadersAndNoSpacerColumn(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`dashboardHistoryAccent = label(ID_DASH, "", 242, 620, 6, 30)`,
		`setPos(dashboardHistoryAccent, 242, bottomY, 6, 30)`,
		`centerListHeaders(objectsList, 7)`,
		`centerListHeaders(recentList, 9)`,
		`HDM_SETITEMW`,
		`Fmt: HDF_CENTER | HDF_STRING`,
		`available = width`,
		`{"Заполнение, %", 105}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing journal layout guard %q", want)
		}
	}
	if strings.Contains(s, `{"",`) || strings.Contains(s, `{" ",`) {
		t.Fatal("dashboard journal must not create an unnamed spacer column")
	}
}

func TestFailureAndGNOListsReuseObjectSelectionTableStyle(t *testing.T) {
	well, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	ws := string(well)
	for _, want := range []string{
		`setListHeaderFont(wellHistoryList, hFontHeaderSmall)`,
		`centerListHeaders(wellHistoryList, 7)`,
		`setListHeaderFont(wellManualList, hFontHeaderSmall)`,
		`centerListHeaders(wellManualList, 6)`,
		`fitWellFailureListColumns(wellHistoryList`,
		`fitWellFailureListColumns(wellManualList`,
		`final named column absorbs`,
	} {
		if !strings.Contains(ws, want) {
			t.Fatalf("failure journal is missing object-selection table styling %q", want)
		}
	}
	telemetry, err := os.ReadFile("telemetry.go")
	if err != nil {
		t.Fatal(err)
	}
	ts := string(telemetry)
	for _, want := range []string{
		`setListHeaderFont(telHistoryList, hFontHeaderSmall)`,
		`centerListHeaders(telHistoryList, 7)`,
		`setListHeaderFont(telChannelList, hFontHeaderSmall)`,
		`centerListHeaders(telChannelList, 8)`,
		`setListHeaderFont(telEventList, hFontHeaderSmall)`,
		`centerListHeaders(telEventList, 3)`,
		`fitTelemetryListColumns(telHistoryList`,
	} {
		if !strings.Contains(ts, want) {
			t.Fatalf("GNO table is missing object-selection table styling %q", want)
		}
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	ms := string(main)
	style := strings.LastIndex(ms, `if gnoTab {`)
	if style < 0 || !strings.Contains(ms[style:style+700], `br = brushLight`) || !strings.Contains(ms[style:style+700], `br = brushHeader`) {
		t.Fatal("GNO tabs do not reassert the quiet/active object-selection style")
	}
}

func TestWellCardSecondTabKeepsManualFormAndActionsSeparated(t *testing.T) {
	b, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`wellManualTab = button(ID_OBJECTS, "Архив отказов · 36 мес."`,
		`setPos(wellManualTab, 242+tabW+tabGap, py(224),`,
		`manualY := py(326)`,
		`setPos(wellAddFailure, 242+contentW-addW, py(274), addW, 44)`,
		`wellManualRows = failuresForObject(key, now)`,
		`if wellManualMode {`,
		`formTop := py(292)`,
		`if h >= 960 {`,
		`else if h >= 810 {`,
		`setPos(wellFormLabels[i], x, y, formColW, 20)`,
		`setPos(wellFormFields[i], x, y+23, formColW, fieldH)`,
		`fieldH = 260`,
		`wellFormUsesChoice(i)`,
		`ПОСЛЕДНИЙ ИСХОДНЫЙ КЛАСС`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("well card visual guard missing %q", want)
		}
	}
	if strings.Contains(s, `event.ObjectKey == key && event.Manual && inFailureWindow`) {
		t.Fatal("imported archive events must not be hidden from the second tab")
	}
	// All twenty fields must stay above the validation line at each breakpoint.
	for _, clientH := range []int32{720, 810, 960, 1040} {
		columns := int32(4)
		if clientH >= 960 {
			columns = 2
		} else if clientH >= 810 {
			columns = 3
		}
		lastRow := int32(19) / columns
		formBottom := int32(292+PAGE_Y_OFFSET) + lastRow*50 + 23 + 27
		if formBottom > clientH-78 {
			t.Fatalf("form overlaps validation at client height %d: bottom=%d errorY=%d", clientH, formBottom, clientH-78)
		}
	}
}

func TestFailureFormChoiceCatalogBranchesAndUserClassSeparateFromSource(t *testing.T) {
	if got := wellClassifierBranch("ЭЦН"); got != "УЭЦН/УЭВН" {
		t.Fatalf("ЭЦН branch = %q", got)
	}
	if got := wellClassifierBranch("УШГН"); got != "УШГН/УШВН/ОРЭ-УШГН" {
		t.Fatalf("УШГН branch = %q", got)
	}
	nodes := wellNodesForEquipment("УШГН")
	if len(nodes) == 0 || !strings.HasPrefix(nodes[0], "УШГН | ") {
		t.Fatalf("missing UШГН node choices: %v", nodes)
	}
	if len(OfficialFailureCauses2024(wellClassifierBranch("УЭЦН"))) == 0 {
		t.Fatal("official classifier branch is empty")
	}
	b, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`wellFormFields[8]), CB_GETCURSEL`,
		`values[7] = wellCauseChoices[index].Code`,
		`values[8] = wellCauseChoices[index].Title`,
		`event.UserClass = values[15]`,
		`"К1", "К2", "К3", "С1", "С2", "С3"`,
		`"Прочее"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("choice mapping missing %q", want)
		}
	}
}

func TestFailureConclusionsHaveSeparateApprovalAndEquipmentFields(t *testing.T) {
	data, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`"Марка ГНО *", "Типоразмер ГНО *"`,
		`"Предварительное заключение *", "Утверждённое заключение"`,
		`ID_WELL_SAVE_FAILURE, ID_WELL_APPROVE_FAILURE:`,
		`if approved && strings.TrimSpace(e.ApprovedConclusion) == ""`,
		`e.ConclusionApproved = approved`,
		`rows[row].ConclusionApproved`,
		`draw.ClrTextBk = uint32(rgb(226, 246, 232))`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("failure approval guard missing %q", want)
		}
	}
}

func TestLoginHeaderKeeps444ReferenceComposition(t *testing.T) {
	data, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`authHeaderHeight           int32 = 132`,
		`authBrandFont = syscall.Handle(mustCall(pCreateFontW.Call(^uintptr(38-1)`,
		`authDeferMove(hdwp, authHeaderRule, 0, 128, clientW, 4)`,
		`authDeferMove(hdwp, authRosaLogo, 28, 6, 256, 96)`,
		`authDeferMove(hdwp, authProductLabel, 292, 30, rightEdge-292, 72)`,
		`drawAuthInfoLead(di)`,
		`featureY := []int32{280, 390, 500, 610}`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("login header rollback missing %q", want)
		}
	}
}

func TestOldPassRestoreWarnsBeforeReplacingLaboratoryDraft(t *testing.T) {
	b, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`confirmLaboratoryHistoryRestore`,
		`все уже введённые сейчас лабораторные данные и текущий черновик будут очищены`,
		`Вместо них загрузятся данные выбранной записи`,
		`Исходная сохранённая запись не изменится`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing old-pass restore safeguard %q", want)
		}
	}
}

func TestFailureWindowIncludesExactThreeYearBoundary(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.Local)
	if !inFailureWindow("23.09.2023", now) {
		t.Fatal("exact 36-month boundary must be included")
	}
	if inFailureWindow("22.09.2023", now) {
		t.Fatal("older event must remain stored but leave the 36-month view")
	}
	if inFailureWindow("24.09.2026", now) {
		t.Fatal("future failure must be rejected")
	}
}

func TestEquipmentCardFormatsMeasurementsForDisplay(t *testing.T) {
	if got := displayTechNumber("16.100000000000001", 2); got != "16.1" {
		t.Fatalf("pressure display = %q, want 16.1", got)
	}
	if got := displayTechNumber("—", 2); got != "—" {
		t.Fatalf("missing measurement display = %q, want em dash", got)
	}
	if got := displayTechNumber("плановый", 2); got != "плановый" {
		t.Fatalf("non-numeric text should be preserved, got %q", got)
	}
}

func TestWellFailureActionsStayInDedicatedRow(t *testing.T) {
	b, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`actionY := max32(py(530), h-54)`,
		`historyH := max32(88, actionY-historyY-12)`,
		`manualH := max32(85, actionY-manualY-12)`,
		`setPos(wellAddFailure, 242+contentW-addW, py(274), addW, 44)`,
		`setPos(wellEditFailure, 242+manualButtonW+actionGap, actionY`,
		`setPos(wellDeleteFailure, 242+2*(manualButtonW+actionGap), actionY`,
		`col, row := int32(i)%formCols, int32(i)/formCols`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("failure card can regress to floating/overlapping actions: missing %q", want)
		}
	}
}

func TestEquipmentPageRestoresStableSinglePanelGeometry(t *testing.T) {
	b, err := os.ReadFile("workflow_pages.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`techParamTitle = pageTitle(ID_TECHPARAM, "Состояние оборудования"`,
		`techParamDescription = label(ID_TECHPARAM, "Фактическая компоновка, Техрежим и диагностика оборудования выбранной скважины.`,
		`techParamObjectInfo = label(ID_TECHPARAM, "Объект не выбран.", 242, 172, 1010, 28)`,
		`pSendMessageW.Call(uintptr(techParamObjectInfo), WM_SETFONT, uintptr(hFontHeaderBold), 1)`,
		`py(210), buttonW, 48`,
		`int32(270)+row*36`,
		`techProfileSave = button(ID_TECHPARAM, "Сохранить фактическую компоновку"`,
		`setPos(techParamDetail, 242, py(504), contentW`,
		`techParamDetailSecondary = 0`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("equipment page must stay on the restored stable geometry: missing %q", want)
		}
	}
	if strings.Contains(s, `techParamDetailSecondary = edit(ID_TECHPARAM`) {
		t.Fatal("equipment page must not recreate the split lower detail panel")
	}
}

func TestEquipmentActionsKeepSingleRedAccentAfterGenericButtonRules(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	style := strings.LastIndex(s, `if techAction {`)
	if style < 0 {
		t.Fatal("equipment action colors are not re-applied after generic button styles")
	}
	for _, want := range []string{`else if techActionPrimary {`, `br = brushLight`, `br = brushHeader`} {
		if !strings.Contains(s[style:style+700], want) {
			t.Fatalf("missing equipment button style branch %q", want)
		}
	}
}

func TestOfficialFailureClassifier2024UsesOnlyCanonicalSheets(t *testing.T) {
	const esp = "УЭЦН/УЭВН"
	const shgn = "УШГН/УШВН/ОРЭ-УШГН"
	if officialFailureClassifier2024SHA256 != "547e47e14150e4205a7678b278294d9e7d78faac79b95e48503e2dfec1adf236" {
		t.Fatalf("unexpected source workbook fingerprint: %s", officialFailureClassifier2024SHA256)
	}
	if got := len(OfficialFailureCauses2024(esp)); got != 88 {
		t.Fatalf("УЭЦН canonical entries = %d, want 88", got)
	}
	if got := len(OfficialFailureCauses2024(shgn)); got != 56 {
		t.Fatalf("УШГН canonical entries = %d, want 56", got)
	}
	if got := len(OfficialFailureCauses2024("ПО 01")); got != 0 {
		t.Fatalf("auxiliary sheet unexpectedly mapped to %d official causes", got)
	}
	want := map[string]string{
		esp + "|3.6.2":   "в т.ч. отложение солей карбоната кальция",
		shgn + "|3.10.1": " - усталостный (нагрузка, наработка)",
		shgn + "|3.10.3": " - коррозионный",
	}
	for _, entry := range OfficialFailureCauses2024("") {
		key := entry.Branch + "|" + entry.Code
		if title, ok := want[key]; ok {
			if entry.Title != title || entry.Version != "2024 V10" || entry.WorkbookSHA256 != officialFailureClassifier2024SHA256 {
				t.Errorf("canonical entry %s not preserved: %+v", key, entry)
			}
			delete(want, key)
		}
		if entry.SourceSheet != "УЭЦН" && entry.SourceSheet != "УШГН" {
			t.Errorf("non-canonical source sheet entered official catalog: %q", entry.SourceSheet)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing canonical entries: %v", want)
	}
	copy := OfficialFailureCauses2024(esp)
	copy[0].Title = "modified by caller"
	if OfficialFailureCauses2024(esp)[0].Title == "modified by caller" {
		t.Fatal("caller can mutate the canonical classifier")
	}
}

func TestSixManualFailuresArePerObjectAndEditsDoNotConsumeSlot(t *testing.T) {
	oldEvents, oldError := failureEvents, failureLoadError
	defer func() { failureEvents, failureLoadError = oldEvents, oldError }()
	failureLoadError = nil
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.Local)
	failureEvents = []FailureEvent{
		{ID: "1", ObjectKey: "A", Date: "23.09.2026", Manual: true},
		{ID: "2", ObjectKey: "A", Date: "01.09.2026", Manual: true},
		{ID: "3", ObjectKey: "A", Date: "01.08.2026", Manual: true},
		{ID: "4", ObjectKey: "A", Date: "01.07.2026", Manual: true},
		{ID: "5", ObjectKey: "A", Date: "01.06.2026", Manual: true},
		{ID: "6", ObjectKey: "A", Date: "01.05.2026", Manual: true},
		{ID: "old", ObjectKey: "A", Date: "22.09.2023", Manual: true},
		{ID: "other", ObjectKey: "B", Date: "23.09.2026", Manual: true},
		{ID: "import", ObjectKey: "A", Date: "23.09.2026", Manual: false},
	}
	if got := manualFailureCount("A", now); got != 6 {
		t.Fatalf("manual count = %d, want 6", got)
	}
}

func TestImportedFailureRequiresUniqueTechObjectMatch(t *testing.T) {
	old := techCurrent
	defer func() { techCurrent = old }()
	techCurrent = []TechObjectState{
		{ObjectKey: "one", Field: "Поле", Well: "7", ObjectPlast: "А"},
		{ObjectKey: "two", Field: "Поле", Well: "7", ObjectPlast: "Б"},
	}
	e := FailureEvent{Field: "Поле", Well: "7"}
	if failureBelongsToObject(e, "one") || failureBelongsToObject(e, "two") {
		t.Fatal("ambiguous well-only event must not be silently attached")
	}
	e.ObjectPlast = "Б"
	if !failureBelongsToObject(e, "two") || failureBelongsToObject(e, "one") {
		t.Fatal("deposit/plast-specific event must attach to one row")
	}
}

func TestFailureNaturalIDSurvivesSourceAndClassificationCorrection(t *testing.T) {
	base := FailureEvent{Field: "Поле", Well: "7", ObjectPlast: "Б", Date: "01.09.2026", Equipment: "УЭЦН", FailedUnit: "кабель", Source: "выгрузка-1.xlsx", SourceClass: "K2", SourceCause: "старая"}
	changed := base
	changed.Source = "выгрузка-2.xlsx"
	changed.SourceClass = "К-2"
	changed.SourceCause = "уточнённая"
	if failureNaturalID(base) != failureNaturalID(changed) {
		t.Fatal("stable import ID must not change with source filename or reviewed classification")
	}
}

func TestFailurePreviewSeparatesDuplicateUpdateConflictAndKeyError(t *testing.T) {
	oldEvents := failureEvents
	defer func() { failureEvents = oldEvents }()
	base := FailureEvent{ID: "same", ObjectKey: "obj", Field: "Поле", Well: "7", Date: "01.09.2026", EventType: "Отказ", Source: "акт", SourceCause: "обрыв"}
	failureEvents = []FailureEvent{base}
	update := base
	update.Comment = "уточнение"
	conflict := base
	conflict.SourceCause = "износ"
	rows := classifyFailureCandidates([]FailureImportCandidate{
		{Row: 2, Event: base},
		{Row: 3, Event: update},
		{Row: 4, Event: conflict},
		{Row: 5, Event: FailureEvent{ID: "bad"}},
	})
	if rows[0].Status != failureStatusDuplicate || rows[1].Status != failureStatusDuplicate || rows[2].Status != failureStatusDuplicate {
		t.Fatalf("same key within one file must be marked duplicate after first occurrence: %+v", rows)
	}
	if rows[3].Status != failureStatusKeyError {
		t.Fatalf("unbound row status = %q", rows[3].Status)
	}
	for i := 1; i <= 2; i++ {
		one := classifyFailureCandidates([]FailureImportCandidate{{Row: i + 2, Event: []FailureEvent{update, conflict}[i-1]}})
		want := []string{failureStatusUpdate, failureStatusConflict}[i-1]
		if one[0].Status != want {
			t.Fatalf("status = %q, want %q", one[0].Status, want)
		}
	}
}

func TestFailureWorkbookMapsTheNamedSourceColumnsWithoutShifting(t *testing.T) {
	header := map[int]string{
		1: "Подразделение", 2: "ЦДНГ", 3: "Месторождение", 4: "№ скважины",
		5: "Объект / пласт (если известен)", 6: "Дата запуска цикла", 7: "Дата отказа",
		8: "Дата окончания ремонта", 9: "Способ эксплуатации", 10: "Тип / компоновка ГНО",
		11: "Отказавший узел", 12: "Причина остановки / симптом",
		13: "Код причины по исходному классификатору", 14: "Причина по исходному классификатору",
		15: "Осложняющий фактор указан", 16: "Обстоятельства отказа / осмотр / дефектовка",
		17: "Причина по заключению комиссии", 18: "Статус расследования", 19: "Источник / файл", 20: "Комментарий",
	}
	row := map[int]string{
		1: "Подразделение-1", 2: "ЦДНГ-2", 3: "Поле-3", 4: "Скважина-4", 5: "Пласт-5",
		6: "01.02.2025", 7: "03.02.2025", 8: "04.02.2025", 9: "Способ-9", 10: "ГНО-10",
		11: "Узел-11", 12: "Симптом-12", 13: "Код-13", 14: "Причина-14", 15: "Фактор-15",
		16: "Осмотр-16", 17: "Комиссия-17", 18: "Расследование-18", 19: "Источник-19", 20: "Комментарий-20",
	}
	event := failureEventFromWorkbookRow(row, failureWorkbookSchema(header), "fallback.xlsx")
	if event.Department != "Подразделение-1" || event.ProductionUnit != "ЦДНГ-2" || event.CycleStartDate != "01.02.2025" || event.Date != "03.02.2025" || event.RepairEndDate != "04.02.2025" {
		t.Fatalf("identity/date columns mapped incorrectly: %+v", event)
	}
	if event.CommissionConclusion != "Комиссия-17" || event.InvestigationStatus != "Расследование-18" || event.Source != "Источник-19" || event.Comment != "Комментарий-20" {
		t.Fatalf("trailing columns shifted: %+v", event)
	}
	if event.SourceClass != "" {
		t.Fatalf("template has no source class column; must not infer one: %q", event.SourceClass)
	}
}

func TestUserCorrectedImportedFailureIsProtectedFromChangedReimport(t *testing.T) {
	oldEvents := failureEvents
	defer func() { failureEvents = oldEvents }()
	corrected := FailureEvent{ID: "stable", ObjectKey: "object", Field: "Поле", Well: "7", Date: "01.09.2026", EventType: "Отказ", Source: "источник", SourceCause: "исправленная причина", CorrectedByUser: true}
	failureEvents = []FailureEvent{corrected}
	incoming := corrected
	incoming.SourceCause = "причина из выгрузки"
	rows := classifyFailureCandidates([]FailureImportCandidate{{Row: 2, Event: incoming}})
	if rows[0].Status != failureStatusConflict {
		t.Fatalf("changed import must preserve manual correction as conflict; got %q", rows[0].Status)
	}
}

func TestVisualV14FailureGeometryAndBuildIdentity(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	wellSource, err := os.ReadFile("well_card_ui.go")
	if err != nil {
		t.Fatal(err)
	}
	mainText := string(mainSource)
	wellText := string(wellSource)
	for _, want := range []string{
		"Автоматизация · \"+appBuildVersion",
		`const appBuildVersion = "7.3.11.465-RESTORE-DEV-VISUAL-20"`,
		"if id == ID_WELL_ADD_FAILURE",
		"techActionPrimary = i == len(techActionButtons)-1",
	} {
		if !strings.Contains(mainText, want) {
			t.Fatalf("main visual v14 marker missing: %s", want)
		}
	}
	for _, want := range []string{
		`wellAddFailure = button(ID_OBJECTS, "Добавить отказ", 242, 274`,
		`wellManualList = newList(ID_OBJECTS, 242, 326, 1010, 320`,
		`setPos(wellAddFailure, 242+contentW-addW, py(274), addW, 44)`,
		`fitWellFailureListColumns(wellManualList`,
		`pInvalidateRect.Call(uintptr(handle), 0, 1)`,
	} {
		if !strings.Contains(wellText, want) {
			t.Fatalf("well-card visual v14 marker missing: %s", want)
		}
	}
}
