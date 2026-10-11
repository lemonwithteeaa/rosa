//go:build windows

package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	methodsReferenceMethods     = 9969
	methodsReferenceCalc        = 9970
	methodsReferenceSources     = 9971
	methodsReferenceReplication = 9972
	methodsReferenceMethodList  = 9973
	methodsReferenceSourceList  = 9974
	methodsSourcePopupCopy      = 9975
	methodsReferenceMinimize    = 9976
	methodsReferenceMaxRestore  = 9977
	methodsReferenceClose       = 9978
	methodsReferenceReagents    = 9979
	methodsReferenceReagentList = 9980
	methodsReferenceReagentAdd  = 9981
	methodsReferenceReagentEdit = 9982
	methodsReferenceTargetEdit  = 9983
	methodsReferenceClassCombo  = 9984
	methodsReferenceTocList     = 9985
)

const (
	methodsReferenceTabMethods = iota
	methodsReferenceTabCalc
	methodsReferenceTabReplication
	methodsReferenceTabSources
	methodsReferenceTabReagents
)

const (
	emSetMargins      = 0x00D3
	emSetBkColor      = 0x0443
	cfmSize           = 0x80000000
	cfmItalic         = 0x00000002
	cfeItalic         = 0x00000002
	methodsLeftMargin = 18
	swMinimize        = 6
	swRestore         = 9
)

var (
	methodsReferenceDone              bool
	methodsReferenceTitle             string
	methodsReferenceBody              string
	methodsReferenceEdit              syscall.Handle
	methodsReferenceHeaderBand        syscall.Handle
	methodsReferenceAccent            syscall.Handle
	methodsReferenceTitleHandle       syscall.Handle
	methodsReferenceSubtitleHandle    syscall.Handle
	methodsReferenceFootnoteHandle    syscall.Handle
	methodsReferenceMethodsButton     syscall.Handle
	methodsReferenceCalcButton        syscall.Handle
	methodsReferenceReplicationButton syscall.Handle
	methodsReferenceSourcesButton     syscall.Handle
	methodsReferenceReagentsButton    syscall.Handle
	methodsReferenceTargetButton      syscall.Handle
	methodsReferenceMethodListHandle  syscall.Handle
	methodsReferenceMethodListTitle   syscall.Handle
	methodsReferenceClassLabel        syscall.Handle
	methodsReferenceClassComboHandle  syscall.Handle
	methodsReferenceTocTitle          syscall.Handle
	methodsReferenceTocListHandle     syscall.Handle
	methodsReferenceMethodCardTitle   syscall.Handle
	methodsReferenceSourceListHandle  syscall.Handle
	methodsReferenceSourceListTitle   syscall.Handle
	methodsReferenceReagentListHandle syscall.Handle
	methodsReferenceReagentListTitle  syscall.Handle
	methodsReferenceReagentInputLabel syscall.Handle
	methodsReferenceReagentInput      syscall.Handle
	methodsReferenceReagentAddButton  syscall.Handle
	methodsReferenceReagentNote       syscall.Handle
	methodsReferenceMinimizeButton    syscall.Handle
	methodsReferenceMaxRestoreButton  syscall.Handle
	methodsReferenceCloseButton       syscall.Handle
	methodsReferenceMethodsBody       string
	methodsReferenceReplicationBody   string
	methodsReferenceActiveSection     int
	methodsReferenceCatalog           []assessmentMethodCandidate
	methodsReferenceCurrentSources    []RuleSource
	methodsReferenceCurrentSchemes    []MethodChemicalScheme
	methodsReferenceFilteredCatalog   []assessmentMethodCandidate
	methodsReferenceClassCodes        []string
	methodsReferenceInitialMethodName string
	methodsReferenceInitialCard       string
	methodsSourcePopupDone            bool
	methodsSourcePopupEdit            syscall.Handle
	methodsSourcePopupText            string
	methodsSourcePopupCaption         string
	methodsSourcePopupValueLabel      string
	methodsReferenceOpenTarget        bool
	methodsReferenceHeadingFont       syscall.Handle
	methodsReferenceTitleFont         syscall.Handle
)

type methodsReferenceClassOption struct {
	Code  string
	Title string
}

func methodsReferenceClassOptions() []methodsReferenceClassOption {
	return []methodsReferenceClassOption{
		{Code: "", Title: "Все осложнения"},
		{Code: "aspo", Title: "АСПО / парафин / асфальтены"},
		{Code: "scale", Title: "Солеотложения / FeS / галит"},
		{Code: "corrosion", Title: "Коррозия"},
		{Code: "microbiology", Title: "Микробиология / СВБ"},
		{Code: "solids", Title: "Механические примеси / песок"},
		{Code: "viscosity", Title: "Высокая вязкость / эмульсии"},
		{Code: "free_gas", Title: "Свободный газ"},
		{Code: "hydrates", Title: "Гидраты"},
		{Code: "high_temp", Title: "Высокая температура"},
	}
}

func methodsReferenceButtonHandles() []syscall.Handle {
	return []syscall.Handle{
		methodsReferenceMethodsButton,
		methodsReferenceCalcButton,
		methodsReferenceReplicationButton,
		methodsReferenceSourcesButton,
		methodsReferenceReagentsButton,
	}
}

func invalidateMethodsReferenceTabs() {
	for _, h := range methodsReferenceButtonHandles() {
		if h != 0 {
			pInvalidateRect.Call(uintptr(h), 0, 1)
		}
	}
}

func showMethodsReferenceControl(h syscall.Handle, show bool) {
	if h == 0 {
		return
	}
	cmd := uintptr(SW_HIDE)
	if show {
		cmd = uintptr(SW_SHOW)
	}
	pShowWindow.Call(uintptr(h), cmd)
}

func methodsReferenceMethodMode() bool {
	return methodsReferenceActiveSection == methodsReferenceTabMethods
}

func methodsReferenceSourceMode() bool {
	return methodsReferenceActiveSection == methodsReferenceTabSources
}

func methodsReferenceReagentMode() bool {
	return methodsReferenceActiveSection == methodsReferenceTabReagents
}

func ensureMethodsReferenceHeadingFonts() {
	if methodsReferenceHeadingFont == 0 {
		h, _, _ := pCreateFontW.Call(^uintptr(uiTextHeight-1), 0, 0, 0, 600, 1, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
		methodsReferenceHeadingFont = syscall.Handle(h)
	}
	if methodsReferenceTitleFont == 0 {
		h, _, _ := pCreateFontW.Call(^uintptr(uiSystemNameHeight-1), 0, 0, 0, 600, 1, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
		methodsReferenceTitleFont = syscall.Handle(h)
	}
}

func deleteMethodsReferenceHeadingFonts() {
	if methodsReferenceHeadingFont != 0 {
		pDeleteObject.Call(uintptr(methodsReferenceHeadingFont))
		methodsReferenceHeadingFont = 0
	}
	if methodsReferenceTitleFont != 0 {
		pDeleteObject.Call(uintptr(methodsReferenceTitleFont))
		methodsReferenceTitleFont = 0
	}
}

func layoutMethodsReferenceDialog(hwnd uintptr) {
	var rc RECT
	if r, _, _ := pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	margin := int32(24)

	// RC583: only the native Windows caption controls remain. The additional
	// in-client — / □ / × controls from RC581 duplicated the system title bar
	// and were explicitly rejected in live Windows review.
	headerH := int32(76)
	setPos(methodsReferenceHeaderBand, 0, 0, w, headerH)
	setPos(methodsReferenceAccent, 0, headerH-4, w, 4)
	targetButtonW := min32(390, max32(310, w/4))
	headerTextW := max32(420, w-margin*3-targetButtonW)
	setPos(methodsReferenceTitleHandle, margin, 12, headerTextW, 28)
	setPos(methodsReferenceSubtitleHandle, margin, 42, headerTextW, 24)
	setPos(methodsReferenceTargetButton, w-margin-targetButtonW, 16, targetButtonW, 44)

	tabsY := int32(88)
	tabsH := int32(44)
	gap := int32(8)
	available := max32(720, w-margin*2-gap*4)
	tabW := available / 5
	x := margin
	for _, btn := range methodsReferenceButtonHandles() {
		setPos(btn, x, tabsY, tabW, tabsH)
		x += tabW + gap
	}

	footH := int32(30)
	footY := max32(620, h-margin-footH)
	contentY := int32(150)
	contentH := max32(360, footY-contentY-12)

	if methodsReferenceMethodMode() {
		headingH := int32(30)
		filterLabelH := int32(22)
		filterComboH := int32(26)
		tocTitleH := int32(24)
		paneY := contentY + headingH + 6
		paneH := max32(320, contentH-headingH-6)
		listW := min32(500, max32(360, (w-margin*2)*39/100))
		paneGap := int32(14)
		cardX := margin + listW + paneGap
		cardW := max32(420, w-margin-cardX)
		sourceTitleH := int32(28)
		sourceListH := min32(180, max32(116, paneH/4))
		cardEditH := max32(220, paneH-sourceTitleH-sourceListH-12)
		filterTop := paneY
		filterLabelY := filterTop
		filterComboY := filterLabelY + filterLabelH + 2
		tocTitleY := filterComboY + filterComboH + 10
		tocListY := tocTitleY + tocTitleH + 4
		tocH := min32(180, max32(126, paneH/4))
		methodsTitleY := tocListY + tocH + 12
		listY := methodsTitleY + headingH + 4
		listH := max32(140, paneY+paneH-listY)

		setPos(methodsReferenceTocTitle, margin, contentY, listW, headingH)
		setPos(methodsReferenceClassLabel, margin, filterLabelY, listW, filterLabelH)
		setPos(methodsReferenceClassComboHandle, margin, filterComboY, listW, 260)
		setPos(methodsReferenceTocListHandle, margin, tocListY, listW, tocH)
		setPos(methodsReferenceMethodListTitle, margin, methodsTitleY, listW, headingH)
		setPos(methodsReferenceMethodCardTitle, cardX, contentY, cardW, headingH)
		setPos(methodsReferenceMethodListHandle, margin, listY, listW, listH)
		setPos(methodsReferenceEdit, cardX, paneY, cardW, cardEditH)
		setPos(methodsReferenceSourceListTitle, cardX, paneY+cardEditH+8, cardW, sourceTitleH)
		setPos(methodsReferenceSourceListHandle, cardX, paneY+cardEditH+sourceTitleH+8, cardW, sourceListH)
	} else if methodsReferenceSourceMode() {
		headingH := int32(30)
		paneY := contentY + headingH + 6
		paneH := max32(320, contentH-headingH-6)
		listW := min32(560, max32(410, (w-margin*2)*42/100))
		paneGap := int32(14)
		detailX := margin + listW + paneGap
		detailW := max32(400, w-margin-detailX)
		setPos(methodsReferenceSourceListTitle, margin, contentY, listW, headingH)
		setPos(methodsReferenceMethodCardTitle, detailX, contentY, detailW, headingH)
		setPos(methodsReferenceSourceListHandle, margin, paneY, listW, paneH)
		setPos(methodsReferenceEdit, detailX, paneY, detailW, paneH)
	} else if methodsReferenceReagentMode() {
		headingH := int32(30)
		paneY := contentY + headingH + 6
		paneH := max32(320, contentH-headingH-6)
		listW := min32(560, max32(410, (w-margin*2)*42/100))
		paneGap := int32(14)
		detailX := margin + listW + paneGap
		detailW := max32(400, w-margin-detailX)
		setPos(methodsReferenceReagentListTitle, margin, contentY, listW, headingH)
		setPos(methodsReferenceMethodCardTitle, detailX, contentY, detailW, headingH)
		setPos(methodsReferenceReagentListHandle, margin, paneY, listW, paneH)
		setPos(methodsReferenceEdit, detailX, paneY, detailW, paneH)
	} else {
		setPos(methodsReferenceTocTitle, 0, 0, 0, 0)
		setPos(methodsReferenceClassLabel, 0, 0, 0, 0)
		setPos(methodsReferenceClassComboHandle, 0, 0, 0, 0)
		setPos(methodsReferenceTocListHandle, 0, 0, 0, 0)
		setPos(methodsReferenceEdit, margin, contentY, max32(720, w-margin*2), contentH)
	}

	setPos(methodsReferenceFootnoteHandle, margin, footY, max32(720, w-margin*2), footH)
}

func formatMethodsReferenceText(h syscall.Handle, s string) {
	if h == 0 {
		return
	}
	// Body is deliberately calm. Structural headings and exact field labels get
	// emphasis; values, bullets and fragments inside prose stay regular-weight.
	normal := CHARFORMATW{
		CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
		DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
		CrTextColor: uint32(rgb(43, 46, 50)),
		YHeight:     230,
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&normal)))

	// RichEdit stores a paragraph break as one character even when the source
	// text uses CRLF. Count positions in that native model so formatting never
	// drifts into the first letters of later paragraphs.
	offset := 0
	firstHeading := true
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	fieldLabels := []string{
		"Осложнения:", "Методы:", "Методы и мероприятия:", "Доступность:", "МРП:", "Основание:",
		"План:", "Контроль:", "Критерии прекращения:", "Дополнительное условие:",
		"Достоверность истории:", "Ответственный:", "Прогноз:", "Версия:", "Дата:", "Автор:",
		"Объект:", "Классификация:", "Статус:", "Источник:", "Параметры реализации:",
		"Последняя ручная корректировка:",
	}
	for i, line := range lines {
		t := strings.TrimSpace(line)
		trimLen := len(utf16.Encode([]rune(t)))
		if t != "" {
			leadingBytes := len(line) - len(strings.TrimLeft(line, " \t"))
			lineStart := offset + len(utf16.Encode([]rune(line[:leadingBytes])))
			objectHeader := strings.HasPrefix(strings.ToUpper(t), "ОБЪЕКТ:")
			schemeHeading := strings.HasPrefix(t, "Схема ") || strings.HasPrefix(t, "МЕТОД ")
			if schemeHeading {
				end := strings.Index(t, " · ")
				if end < 0 {
					end = len(t)
				}
				cf := CHARFORMATW{
					CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
					DwEffects: CFE_BOLD | cfeItalic, CrTextColor: uint32(rgb(183, 20, 37)), YHeight: 245,
				}
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(lineStart), uintptr(lineStart+len(utf16.Encode([]rune(t[:end])))))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
			}
			if objectHeader {
				// Keep the field name in navy and apply the application's red
				// accent only to the selected object's value.
				colon := strings.IndexByte(t, ':')
				labelCF := CHARFORMATW{
					CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR,
					DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82)),
				}
				labelEnd := lineStart + len(utf16.Encode([]rune(t[:colon+1])))
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(lineStart), uintptr(labelEnd))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&labelCF)))
				valueStart := colon + 1
				for valueStart < len(t) && (t[valueStart] == ' ' || t[valueStart] == '\t') {
					valueStart++
				}
				if valueStart < len(t) {
					redCF := CHARFORMATW{
						CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR,
						DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(183, 20, 37)),
					}
					valueStartOffset := lineStart + len(utf16.Encode([]rune(t[:valueStart])))
					pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(valueStartOffset), uintptr(lineStart+trimLen))
					pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&redCF)))
				}
			}
			if strings.HasPrefix(t, "• ") {
				// Implementation details use a bullet followed by a label and a
				// value. Emphasize the label only so long values remain easy to scan.
				if colon := strings.IndexByte(t[2:], ':'); colon >= 0 {
					labelEnd := 2 + colon + 1
					labelStart := lineStart + len(utf16.Encode([]rune(t[:2])))
					labelLength := len(utf16.Encode([]rune(t[2:labelEnd])))
					cf := CHARFORMATW{
						CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR,
						DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82)),
					}
					pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(labelStart), uintptr(labelStart+labelLength))
					pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
				}
			}
			upperHeading := t == strings.ToUpper(t) && !strings.HasPrefix(t, "•") && len([]rune(t)) <= 72
			if upperHeading && !schemeHeading {
				// Structural headings stay distinct from the exact field labels below.
				cf := CHARFORMATW{
					CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
					DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
					DwEffects:   CFE_BOLD | cfeItalic,
					CrTextColor: uint32(rgb(15, 48, 82)),
					YHeight:     240,
				}
				// Legacy regression token: cf.DwEffects = CFE_BOLD; headings also italic.
				if firstHeading {
					cf.YHeight = 260
					firstHeading = false
				}
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(lineStart), uintptr(lineStart+trimLen))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
			}
			for _, label := range fieldLabels {
				if !strings.HasPrefix(t, label) {
					continue
				}
				cf := CHARFORMATW{
					CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR,
					DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82)),
				}
				labelLength := len(utf16.Encode([]rune(label)))
				pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(lineStart), uintptr(lineStart+labelLength))
				pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
				break
			}
		}
		offset += len(utf16.Encode([]rune(line)))
		if i < len(lines)-1 {
			offset++
		}
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, 0)
}

func methodsReferenceCandidateFromCatalog(m MethodCatalogEntry) assessmentMethodCandidate {
	source, sourceKind := assessmentMethodSource(m)
	note := "Кандидат из справочника методов. Выбор строки открывает инженерную справку и не означает автоматического назначения."
	if assessmentMethodIsEquipmentDesign(m.Name) {
		note = "РОСА фиксирует диагностический контекст. Подбор и проверка конкретной компоновки выполняются в «Автотехнологе»."
	}
	return assessmentMethodCandidate{
		ID: m.ID, Name: m.Name, Source: source, SourceKind: sourceKind,
		Status: assessmentMethodStatus(m.Name), Note: note,
	}
}

func methodsReferenceDirectionText(name string) string {
	groups := []struct{ code, title string }{
		{"aspo", "АСПО"},
		{"scale", "солеотложения"},
		{"corrosion", "коррозия"},
		{"microbiology", "микробиологически обусловленная коррозия"},
		{"solids", "механические примеси"},
		{"viscosity", "высокая вязкость"},
		{"free_gas", "свободный газ"},
		{"hydrates", "гидраты"},
		{"high_temp", "высокая температура"},
	}
	var out []string
	for _, g := range groups {
		if assessmentMethodMatchesGroup(g.code, name) {
			out = append(out, g.title)
		}
	}
	if len(out) == 0 {
		return "требует уточнения по объекту"
	}
	return strings.Join(out, ", ")
}

func methodsReferenceMethodCardText(method assessmentMethodCandidate) string {
	var b strings.Builder
	b.WriteString("СПРАВКА ПО МЕТОДУ\r\n\r\n")
	fmt.Fprintf(&b, "Метод: %s\r\n", dash(method.Name))
	fmt.Fprintf(&b, "Направление: %s\r\n", methodsReferenceDirectionText(method.Name))
	fmt.Fprintf(&b, "Тип решения: %s\r\n", methodsPracticeStage(method.Name))
	fmt.Fprintf(&b, "Статус проверки: %s\r\n\r\n", dash(method.Status))

	b.WriteString("НАЗНАЧЕНИЕ И УСЛОВИЯ ПРИМЕНЕНИЯ\r\n")
	rules := methodRulesForName(method.Name)
	if len(rules) == 0 {
		b.WriteString("• Конкретное назначение подтверждается по механизму осложнения и данным выбранного объекта.\r\n")
	} else {
		seen := map[string]bool{}
		for _, r := range rules {
			if strings.TrimSpace(r.Purpose) != "" && !seen["p:"+r.Purpose] {
				fmt.Fprintf(&b, "• Назначение: %s.\r\n", strings.TrimSpace(r.Purpose))
				seen["p:"+r.Purpose] = true
			}
			if len(r.Criteria) > 0 {
				fmt.Fprintf(&b, "• Условия применимости: %s.\r\n", strings.Join(r.Criteria, "; "))
			}
		}
	}

	// RC681: every card receives the full engineering minimum even when an older
	// corporate rule did not carry every modern field explicitly.
	b.WriteString(methodsPracticeDetailedCard(method.Name))

	// Explicit rule-level details refine the generic complication profile.
	required, contraindications, controls, limitations := []string{}, []string{}, []string{}, []string{}
	seenText := map[string]bool{}
	baseProfile := methodsPracticeProfileFor(method.Name)
	for _, value := range append(append(append([]string{}, baseProfile.Inputs...), baseProfile.Contraindications...), baseProfile.Control...) {
		seenText[strings.TrimSpace(value)] = true
	}
	appendUnique := func(dst *[]string, values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seenText[value] {
				continue
			}
			seenText[value] = true
			*dst = append(*dst, value)
		}
	}
	for _, r := range rules {
		appendUnique(&required, r.RequiredInputs...)
		appendUnique(&contraindications, r.Contraindications...)
		appendUnique(&controls, r.Control...)
		appendUnique(&limitations, r.Limitations...)
	}
	if len(required) > 0 {
		b.WriteString("\r\nДОПОЛНИТЕЛЬНЫЕ ИСХОДНЫЕ ДАННЫЕ ДЛЯ ЭТОГО МЕТОДА\r\n")
		b.WriteString("• " + strings.Join(required, "; ") + ".\r\n")
	}
	if len(contraindications) > 0 || len(limitations) > 0 {
		b.WriteString("\r\nДОПОЛНИТЕЛЬНЫЕ ОГРАНИЧЕНИЯ ДЛЯ ЭТОГО МЕТОДА\r\n")
		items := append([]string{}, contraindications...)
		items = append(items, limitations...)
		b.WriteString("• " + strings.Join(items, "; ") + ".\r\n")
	}
	if len(controls) > 0 {
		b.WriteString("\r\nДОПОЛНИТЕЛЬНЫЙ КОНТРОЛЬ ДЛЯ ЭТОГО МЕТОДА\r\n")
		b.WriteString("• " + strings.Join(controls, "; ") + ".\r\n")
	}

	// Preserve the method-specific handbook paragraphs accumulated in earlier
	// releases (mechanism, deployment, risks, etc.).
	b.WriteString(methodEngineeringReferenceText(method.Name))

	sources := methodsReferenceSourcesForMethod(method.Name)
	if len(sources) > 0 {
		b.WriteString("\r\n\r\nИСТОЧНИКИ И ТЕХНИЧЕСКАЯ БАЗА\r\n")
		for _, src := range sources {
			label := strings.TrimSpace(src.Title)
			if strings.TrimSpace(src.Organization) != "" {
				label += " — " + strings.TrimSpace(src.Organization)
			}
			if src.Year > 0 {
				label += fmt.Sprintf(", %d", src.Year)
			}
			fmt.Fprintf(&b, "• %s.\r\n", label)
		}
	}

	b.WriteString("\r\nСЛЕДУЮЩЕЕ ДЕЙСТВИЕ\r\n")
	switch {
	case assessmentMethodNeedsLabSelection(method.Name):
		b.WriteString("Лабораторный подбор, проверка совместимости и уточнение режима/концентрации; затем объектовая проверка и ОПИ при необходимости.\r\n")
	case assessmentMethodIsEquipmentDesign(method.Name):
		b.WriteString("Передать фактические параметры объекта в «Автотехнолог» для проверки оборудования и компоновки. РОСА не подбирает ГНО автоматически.\r\n")
	default:
		b.WriteString("Проверить ограничения на данных выбранного объекта, сопоставить с предысторией отказов и зафиксировать основание решения технолога.\r\n")
	}

	b.WriteString("\r\nВАЖНО\r\n")
	b.WriteString("Справка поддерживает инженерный выбор, но не является автоматическим назначением. Численный эффект на МРП допускается только при подходящей объектовой доказательной базе; проценты разных эффектов не складываются арифметически. Фирменные изделия в источниках и примерах не заменяют технологическое название метода.\r\n")
	return b.String()
}

func rebuildMethodsReferenceCatalog() {
	seen := map[string]bool{}
	methodsReferenceCatalog = methodsReferenceCatalog[:0]
	for _, m := range ruleCatalog.MethodCatalog {
		key := normalizeMethodText(m.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		methodsReferenceCatalog = append(methodsReferenceCatalog, methodsReferenceCandidateFromCatalog(m))
	}
	// Keep the canonical Technology Module first; verified additions follow it.
	sort.SliceStable(methodsReferenceCatalog, func(i, j int) bool {
		iTech := strings.Contains(strings.ToLower(methodsReferenceCatalog[i].Source), "технологичес")
		jTech := strings.Contains(strings.ToLower(methodsReferenceCatalog[j].Source), "технологичес")
		if iTech != jTech {
			return iTech
		}
		return methodsReferenceCatalog[i].Name < methodsReferenceCatalog[j].Name
	})
}

func setMethodsReferenceCard(text string) {
	methodsReferenceBody = text
	if methodsReferenceEdit != 0 {
		setText(methodsReferenceEdit, text)
		formatMethodsReferenceText(methodsReferenceEdit, text)
	}
}

func methodsReferenceSelectedClassCode() string {
	if methodsReferenceClassComboHandle == 0 || len(methodsReferenceClassCodes) == 0 {
		return ""
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_GETCURSEL, 0, 0)
	i := int(idx)
	if i < 0 || i >= len(methodsReferenceClassCodes) {
		return ""
	}
	return methodsReferenceClassCodes[i]
}

func methodsReferenceClassTitle(code string) string {
	for _, option := range methodsReferenceClassOptions() {
		if option.Code == code {
			return option.Title
		}
	}
	return "Все осложнения"
}

func methodsReferenceMethodInClass(method assessmentMethodCandidate, code string) bool {
	if strings.TrimSpace(code) == "" {
		return true
	}
	return assessmentMethodMatchesGroup(code, method.Name)
}

func populateMethodsReferenceClassControls() {
	options := methodsReferenceClassOptions()
	selectedCode := methodsReferenceSelectedClassCode()
	methodsReferenceClassCodes = methodsReferenceClassCodes[:0]
	if methodsReferenceClassComboHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_RESETCONTENT, 0, 0)
	}
	if methodsReferenceTocListHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), LB_RESETCONTENT, 0, 0)
	}
	selectedIndex := 0
	for i, option := range options {
		count := 0
		for _, method := range methodsReferenceCatalog {
			if methodsReferenceMethodInClass(method, option.Code) {
				count++
			}
		}
		caption := option.Title
		if option.Code == "" {
			caption = fmt.Sprintf("%s · %d", option.Title, len(methodsReferenceCatalog))
		} else {
			caption = fmt.Sprintf("%s · %d", option.Title, count)
		}
		methodsReferenceClassCodes = append(methodsReferenceClassCodes, option.Code)
		if methodsReferenceClassComboHandle != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
		}
		if methodsReferenceTocListHandle != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
		}
		if option.Code == selectedCode {
			selectedIndex = i
		}
	}
	if methodsReferenceClassComboHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_SETCURSEL, uintptr(selectedIndex), 0)
	}
	if methodsReferenceTocListHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), LB_SETCURSEL, uintptr(selectedIndex), 0)
	}
}

func populateMethodsReferenceFilteredList(preferredName string) {
	if methodsReferenceMethodListHandle == 0 {
		return
	}
	code := methodsReferenceSelectedClassCode()
	methodsReferenceFilteredCatalog = methodsReferenceFilteredCatalog[:0]
	pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), LB_RESETCONTENT, 0, 0)
	selected := 0
	for _, m := range methodsReferenceCatalog {
		if !methodsReferenceMethodInClass(m, code) {
			continue
		}
		methodsReferenceFilteredCatalog = append(methodsReferenceFilteredCatalog, m)
		pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(m.Name))))
		if preferredName != "" && normalizeMethodText(m.Name) == normalizeMethodText(preferredName) {
			selected = len(methodsReferenceFilteredCatalog) - 1
		}
	}
	if methodsReferenceMethodListTitle != 0 {
		setText(methodsReferenceMethodListTitle, fmt.Sprintf("МЕТОДЫ · %s · %d", strings.ToUpper(methodsReferenceClassTitle(code)), len(methodsReferenceFilteredCatalog)))
	}
	if len(methodsReferenceFilteredCatalog) == 0 {
		setMethodsReferenceCard("МЕТОДЫ\r\n\r\nДля выбранного класса осложнения методы пока не найдены.")
		populateMethodsReferenceSourceList(nil)
		return
	}
	pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), LB_SETCURSEL, uintptr(selected), 0)
	method := methodsReferenceFilteredCatalog[selected]
	methodsReferenceInitialMethodName = method.Name
	if methodsReferenceInitialCard != "" && normalizeMethodText(method.Name) == normalizeMethodText(preferredName) {
		setMethodsReferenceCard(methodsReferenceInitialCard)
	} else {
		setMethodsReferenceCard(methodsReferenceMethodCardText(method))
	}
	populateMethodsReferenceSourceList(methodsReferenceSourcesForMethod(method.Name))
}

func refreshMethodsReferenceMethodView(preferredName string) {
	populateMethodsReferenceClassControls()
	populateMethodsReferenceFilteredList(preferredName)
}

func setMethodsReferenceClassSelection(index int) {
	if index < 0 || index >= len(methodsReferenceClassCodes) {
		return
	}
	if methodsReferenceClassComboHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_SETCURSEL, uintptr(index), 0)
	}
	if methodsReferenceTocListHandle != 0 {
		pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), LB_SETCURSEL, uintptr(index), 0)
	}
	refreshMethodsReferenceMethodView(methodsReferenceInitialMethodName)
}

func populateMethodsReferenceList() {
	if methodsReferenceMethodListHandle == 0 {
		return
	}
	rebuildMethodsReferenceCatalog()
	if len(methodsReferenceCatalog) == 0 {
		setMethodsReferenceCard("МЕТОДЫ\r\n\r\nСправочник методов пока пуст. Проверьте загрузку корпоративной базы правил.")
		return
	}
	refreshMethodsReferenceMethodView(methodsReferenceInitialMethodName)
}

func methodsReferenceSourcesForMethod(name string) []RuleSource {
	seen := map[string]bool{}
	out := []RuleSource{}
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		if src, ok := ruleSource(id); ok {
			seen[id] = true
			out = append(out, src)
		}
	}
	for _, m := range ruleCatalog.MethodCatalog {
		if normalizeMethodText(m.Name) == normalizeMethodText(name) {
			add(m.SourceID)
		}
	}
	for _, r := range methodRulesForName(name) {
		for _, id := range r.SourceIDs {
			add(id)
		}
	}
	return out
}

func methodsReferenceSourceLabel(src RuleSource) string {
	prefix := "ФАЙЛ"
	if methodsReferenceSourceIsWeb(src) {
		prefix = "ОНЛАЙН"
	}
	// RC585: the list is a navigator, not the full source card.  Organisation,
	// year and document metadata stay in the detail pane on the right.  Keeping
	// only the source title here prevents native LISTBOX text from being clipped
	// at 125–150% DPI while preserving the complete information one click away.
	title := strings.TrimSpace(src.Title)
	if title == "" {
		title = strings.TrimSpace(src.Organization)
	}
	if title == "" {
		title = "Источник РОСА"
	}
	return prefix + " · " + title
}

func methodsReferenceSourceLink(src RuleSource) string {
	if strings.TrimSpace(src.URL) != "" {
		return strings.TrimSpace(src.URL)
	}
	if strings.TrimSpace(src.Reference) != "" {
		return strings.TrimSpace(src.Reference)
	}
	return strings.TrimSpace(src.Title)
}

func methodsReferenceSourceIsWeb(src RuleSource) bool {
	u := strings.ToLower(strings.TrimSpace(src.URL))
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

func methodsReferenceDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		for _, candidate := range []string{
			filepath.Join(home, "Downloads"),
			filepath.Join(home, "Загрузки"),
			filepath.Join(home, "OneDrive", "Downloads"),
		} {
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return candidate
			}
		}
		candidate := filepath.Join(home, "Downloads")
		if os.MkdirAll(candidate, 0700) == nil {
			return candidate
		}
	}
	fallback := filepath.Join(appRootDir(), "exports", "Источники")
	_ = os.MkdirAll(fallback, 0700)
	return fallback
}

func methodsReferenceWriteCSV(path string, header []string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// UTF-8 BOM + semicolon delimiter: opens cleanly in Russian Excel without an import wizard.
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Comma = ';'
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func methodsReferenceExportLocalSource(src RuleSource) (string, error) {
	dir := methodsReferenceDownloadsDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102_150405")
	base := safeFilePart(src.Title)
	if base == "" {
		base = "Источник_РОСА"
	}
	switch src.ID {
	case "excel_tech_methods":
		path := filepath.Join(dir, "РОСА_"+base+"_"+stamp+".csv")
		rows := make([][]string, 0, len(ruleCatalog.MethodCatalog))
		for _, m := range ruleCatalog.MethodCatalog {
			if m.SourceID != src.ID {
				continue
			}
			rows = append(rows, []string{m.ID, m.Name, m.Status, methodsReferenceDirectionText(m.Name)})
		}
		return path, methodsReferenceWriteCSV(path, []string{"ID", "Метод", "Статус", "Направление"}, rows)
	case "excel_prelim_matrix":
		path := filepath.Join(dir, "РОСА_"+base+"_"+stamp+".csv")
		rows := [][]string{}
		for _, r := range ruleCatalog.Rules {
			match := false
			for _, sid := range r.SourceIDs {
				if sid == src.ID {
					match = true
					break
				}
			}
			if !match {
				continue
			}
			rows = append(rows, []string{
				r.ID, r.Complication, r.Method, r.Purpose,
				strings.Join(r.Criteria, " | "), strings.Join(r.Limitations, " | "),
				r.CriterionStatus, r.EconomicEffectBasis, r.PreliminaryStatus, r.Comment,
			})
		}
		return path, methodsReferenceWriteCSV(path, []string{"ID", "Осложнение", "Метод", "Назначение", "Критерии", "Ограничения", "Статус критерия", "Основа эффекта", "Предварительный статус", "Комментарий"}, rows)
	default:
		path := filepath.Join(dir, "РОСА_"+base+"_"+stamp+".txt")
		body := methodsReferenceSourceDetailText(src) + "\r\n"
		if strings.TrimSpace(src.Reference) != "" {
			body += "Исходное обозначение материала: " + strings.TrimSpace(src.Reference) + "\r\n"
		}
		return path, os.WriteFile(path, []byte(body), 0600)
	}
}

func methodsReferenceOpenURL(owner uintptr, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("у источника отсутствует интернет-ссылка")
	}
	r, _, _ := pShellExecuteW.Call(owner, uintptr(unsafe.Pointer(p("open"))), uintptr(unsafe.Pointer(p(rawURL))), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("Windows не смог открыть ссылку (код %d)", r)
	}
	return nil
}

func handleMethodsReferenceSourceAction(owner uintptr, src RuleSource) {
	if methodsReferenceSourceIsWeb(src) {
		if err := methodsReferenceOpenURL(owner, src.URL); err != nil {
			msg("Источник", err.Error()+"\n\n"+strings.TrimSpace(src.URL), MB_OK|MB_ICONERROR)
			return
		}
		openMethodsReferenceSourceLinkPopup(owner, src, strings.TrimSpace(src.URL), "Интернет-ссылка")
		return
	}
	path, err := methodsReferenceExportLocalSource(src)
	if err != nil {
		msg("Источник", "Не удалось выгрузить материал:\n"+err.Error(), MB_OK|MB_ICONERROR)
		return
	}
	// The download is the primary action. Open the exported copy after saving so
	// the user immediately sees the actual file, not an internal pseudo-reference.
	pShellExecuteW.Call(owner, uintptr(unsafe.Pointer(p("open"))), uintptr(unsafe.Pointer(p(path))), 0, 0, 1)
	openMethodsReferenceSourceLinkPopup(owner, src, path, "Скачанный файл")
}

func methodsReferenceSourceDetailText(src RuleSource) string {
	var b strings.Builder
	b.WriteString("КАРТОЧКА ИСТОЧНИКА\r\n\r\n")
	fmt.Fprintf(&b, "Название: %s\r\n", dash(src.Title))
	if strings.TrimSpace(src.Organization) != "" {
		fmt.Fprintf(&b, "Организация: %s\r\n", strings.TrimSpace(src.Organization))
	}
	if src.Year > 0 {
		fmt.Fprintf(&b, "Год: %d\r\n", src.Year)
	}
	if strings.TrimSpace(src.Kind) != "" {
		fmt.Fprintf(&b, "Тип: %s\r\n", strings.TrimSpace(src.Kind))
	}
	if strings.TrimSpace(src.Scope) != "" {
		fmt.Fprintf(&b, "Область применения: %s\r\n", strings.TrimSpace(src.Scope))
	}
	if strings.TrimSpace(src.Status) != "" {
		fmt.Fprintf(&b, "Статус: %s\r\n", strings.TrimSpace(src.Status))
	}
	if strings.TrimSpace(src.Reference) != "" {
		fmt.Fprintf(&b, "Документ: %s\r\n", strings.TrimSpace(src.Reference))
	}
	b.WriteString("\r\nДвойной щелчок: интернет-источник открывается в браузере; накопленный материал выгружается в папку «Загрузки» как отдельный файл. После действия показывается копируемая ссылка или путь к скачанному файлу.\r\n")
	return b.String()
}

func populateMethodsReferenceSourceList(sources []RuleSource) {
	methodsReferenceCurrentSources = append(methodsReferenceCurrentSources[:0], sources...)
	if methodsReferenceSourceListHandle == 0 {
		return
	}
	pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), 0x0184, 0, 0) // LB_RESETCONTENT
	for _, src := range methodsReferenceCurrentSources {
		label := methodsReferenceSourceLabel(src)
		pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(label))))
	}
	if len(methodsReferenceCurrentSources) > 0 {
		pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), LB_SETCURSEL, 0, 0)
	}
}

func populateAllMethodsReferenceSources() {
	sources := append([]RuleSource(nil), ruleCatalog.Sources...)
	sort.SliceStable(sources, func(i, j int) bool {
		return methodsReferenceSourceLabel(sources[i]) < methodsReferenceSourceLabel(sources[j])
	})
	populateMethodsReferenceSourceList(sources)
	if len(sources) > 0 {
		setMethodsReferenceCard(methodsReferenceSourceDetailText(sources[0]))
	} else {
		setMethodsReferenceCard("ИСТОЧНИКИ\r\n\r\nСписок источников пока пуст.")
	}
}

func selectedMethodsReferenceSource() (RuleSource, bool) {
	if methodsReferenceSourceListHandle == 0 || len(methodsReferenceCurrentSources) == 0 {
		return RuleSource{}, false
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), LB_GETCURSEL, 0, 0)
	i := int(idx)
	if i < 0 || i >= len(methodsReferenceCurrentSources) {
		return RuleSource{}, false
	}
	return methodsReferenceCurrentSources[i], true
}

func openSelectedMethodsReferenceCard() {
	if methodsReferenceMethodListHandle == 0 || len(methodsReferenceFilteredCatalog) == 0 {
		return
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), LB_GETCURSEL, 0, 0)
	i := int(idx)
	if i < 0 || i >= len(methodsReferenceFilteredCatalog) {
		return
	}
	methodsReferenceInitialMethodName = methodsReferenceFilteredCatalog[i].Name
	methodsReferenceInitialCard = ""
	setMethodsReferenceCard(methodsReferenceMethodCardText(methodsReferenceFilteredCatalog[i]))
	populateMethodsReferenceSourceList(methodsReferenceSourcesForMethod(methodsReferenceFilteredCatalog[i].Name))
}

func populateMethodsReferenceReagentList() {
	if methodsReferenceReagentListHandle == 0 {
		return
	}
	pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), 0x0184, 0, 0) // LB_RESETCONTENT
	methodsReferenceCurrentSchemes = methodChemicalSchemes()
	setText(methodsReferenceReagentListTitle, fmt.Sprintf("СОХРАНЁННЫЕ СХЕМЫ · %d", len(methodsReferenceCurrentSchemes)))
	for _, scheme := range methodsReferenceCurrentSchemes {
		caption := methodChemicalSchemeCaption(scheme)
		pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), LB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(caption))))
	}
	if len(methodsReferenceCurrentSchemes) > 0 {
		pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), LB_SETCURSEL, 0, 0)
		setMethodsReferenceCard(methodChemicalSchemeDetails(methodsReferenceCurrentSchemes[0]))
		return
	}
	body := "СПРАВОЧНИК СХЕМ ХИМИЗАЦИИ\r\n\r\n"
	body += "Сохранённых схем пока нет.\r\n\r\n"
	body += "Откройте параметры реализации метода, заполните все показатели вручную и нажмите «СОХРАНИТЬ СХЕМУ В СПРАВОЧНИК». После этого полную схему можно выбрать для другого объекта.\r\n\r\n"
	body += "Точное совпадение всей совокупности параметров повторно не сохраняется."
	setMethodsReferenceCard(body)
}

func openSelectedMethodsReferenceChemicalScheme() {
	if methodsReferenceReagentListHandle == 0 || len(methodsReferenceCurrentSchemes) == 0 {
		return
	}
	idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), LB_GETCURSEL, 0, 0)
	i := int(idx)
	if i < 0 || i >= len(methodsReferenceCurrentSchemes) {
		return
	}
	setMethodsReferenceCard(methodChemicalSchemeDetails(methodsReferenceCurrentSchemes[i]))
}

func updateMethodsReferenceModeVisibility() {
	methodMode := methodsReferenceMethodMode()
	sourceMode := methodsReferenceSourceMode()
	reagentMode := methodsReferenceReagentMode()
	showMethodsReferenceControl(methodsReferenceMethodListHandle, methodMode)
	showMethodsReferenceControl(methodsReferenceMethodListTitle, methodMode)
	showMethodsReferenceControl(methodsReferenceClassLabel, methodMode)
	showMethodsReferenceControl(methodsReferenceClassComboHandle, methodMode)
	showMethodsReferenceControl(methodsReferenceTocTitle, methodMode)
	showMethodsReferenceControl(methodsReferenceTocListHandle, methodMode)
	showMethodsReferenceControl(methodsReferenceMethodCardTitle, methodMode || sourceMode || reagentMode)
	showMethodsReferenceControl(methodsReferenceSourceListHandle, methodMode || sourceMode)
	showMethodsReferenceControl(methodsReferenceSourceListTitle, methodMode || sourceMode)
	showMethodsReferenceControl(methodsReferenceReagentListHandle, reagentMode)
	showMethodsReferenceControl(methodsReferenceReagentListTitle, reagentMode)
	showMethodsReferenceControl(methodsReferenceReagentInputLabel, false)
	showMethodsReferenceControl(methodsReferenceReagentInput, false)
	showMethodsReferenceControl(methodsReferenceReagentAddButton, false)
	showMethodsReferenceControl(methodsReferenceReagentNote, false)
	if methodMode {
		setText(methodsReferenceTocTitle, "ОГЛАВЛЕНИЕ ПО КЛАССАМ ОСЛОЖНЕНИЙ")
		setText(methodsReferenceClassLabel, "ФИЛЬТР · раскрывающийся список по классам осложнений")
		setText(methodsReferenceMethodCardTitle, "КАРТОЧКА МЕТОДА")
		setText(methodsReferenceSourceListTitle, "ИСТОЧНИКИ · двойной щелчок — открыть / скачать")
	} else if sourceMode {
		setText(methodsReferenceMethodCardTitle, "КАРТОЧКА ИСТОЧНИКА")
		setText(methodsReferenceSourceListTitle, "ВСЕ ИСТОЧНИКИ · двойной щелчок")
	} else if reagentMode {
		setText(methodsReferenceMethodCardTitle, "ПОЛНАЯ СХЕМА ХИМИЗАЦИИ")
		setText(methodsReferenceReagentListTitle, fmt.Sprintf("СОХРАНЁННЫЕ СХЕМЫ · %d", len(methodsReferenceCurrentSchemes)))
	}
}

func setMethodsReferenceSection(section int, body string) {
	methodsReferenceActiveSection = section
	switch section {
	case methodsReferenceTabMethods:
		if methodsReferenceMethodListHandle != 0 {
			refreshMethodsReferenceMethodView(methodsReferenceInitialMethodName)
		}
	case methodsReferenceTabSources:
		populateAllMethodsReferenceSources()
	case methodsReferenceTabReagents:
		populateMethodsReferenceReagentList()
	default:
		setMethodsReferenceCard(body)
	}
	updateMethodsReferenceModeVisibility()
	invalidateMethodsReferenceTabs()
}

func methodsSourcePopupWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		title := modalCreate("STATIC", methodsSourcePopupCaption, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_ENDELLIPSIS, 24, 18, 790, 36, parent, 0)
		if hFontBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontBold), 1)
		}
		label := methodsSourcePopupValueLabel
		if strings.TrimSpace(label) == "" {
			label = "Ссылка / путь"
		}
		modalCreate("STATIC", label, WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 66, 340, 24, parent, 0)
		methodsSourcePopupEdit = modalCreate("EDIT", methodsSourcePopupText, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL|ES_READONLY, 24, 92, 650, 34, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsSourcePopupEdit), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		copyBtn := modalCreate("BUTTON", "КОПИРОВАТЬ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 686, 92, 132, 34, parent, methodsSourcePopupCopy)
		if hFontButton != 0 {
			pSendMessageW.Call(uintptr(copyBtn), WM_SETFONT, uintptr(hFontButton), 1)
		}
		return 0
	case WM_COMMAND:
		if int(loWord(wParam)) == methodsSourcePopupCopy && methodsSourcePopupEdit != 0 {
			pSendMessageW.Call(uintptr(methodsSourcePopupEdit), EM_SETSEL, 0, ^uintptr(0))
			pSendMessageW.Call(uintptr(methodsSourcePopupEdit), WM_COPY, 0, 0)
			pSendMessageW.Call(uintptr(methodsSourcePopupEdit), EM_SETSEL, 0, 0)
			return 0
		}
	case labMappingClose:
		methodsSourcePopupDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openMethodsReferenceSourceLinkPopup(owner uintptr, src RuleSource, value, valueLabel string) {
	methodsSourcePopupCaption = methodsReferenceSourceLabel(src)
	methodsSourcePopupText = strings.TrimSpace(value)
	if methodsSourcePopupText == "" {
		methodsSourcePopupText = methodsReferenceSourceLink(src)
	}
	methodsSourcePopupValueLabel = strings.TrimSpace(valueLabel)
	methodsSourcePopupDone = false
	methodsSourcePopupEdit = 0
	cls := p("ROSAMethodSourcePopupV2")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodsSourcePopupWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(owner, 0)
	// RC587: use the normal caption metrics instead of WS_EX_TOOLWINDOW.  The
	// compact tool-window caption clipped the native close button at 125–150% DPI.
	// The wider client area also keeps the read-only path and COPY button apart.
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — источник"))), 0x00C80000|WS_VISIBLE, 0x80000000, 0x80000000, 860, 215, owner, 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(owner, 1)
		msg("Источник", methodsSourcePopupText, MB_OK|MB_ICONINFORMATION)
		return
	}
	var message MSG
	for !methodsSourcePopupDone {
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
	methodsSourcePopupEdit = 0
	pEnableWindow.Call(owner, 1)
}

func drawMethodsReferenceButton(di *DRAWITEMSTRUCT) uintptr {
	if di == nil {
		return 0
	}
	id := int(di.CtlID)
	if id == methodsReferenceTargetEdit {
		fill := brushHeader
		if di.ItemState&ODS_SELECTED != 0 {
			fill = brushGraphitePressed
		}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(fill))
		pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(brushWhite))
		pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
		pSetTextColor.Call(uintptr(di.HDC), rgb(255, 255, 255))
		font := hFontButton
		if font == 0 {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		caption := getText(di.HwndItem)
		r := di.RcItem
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
		return 1
	}
	if id == methodsReferenceMinimize || id == methodsReferenceMaxRestore || id == methodsReferenceClose {
		fill := brushNav
		if id == methodsReferenceClose {
			fill = brushHeader
		}
		pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(fill))
		pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
		pSetTextColor.Call(uintptr(di.HDC), rgb(255, 255, 255))
		font := hFontButton
		if font == 0 {
			font = hFontBold
		}
		old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
		caption := getText(di.HwndItem)
		r := di.RcItem
		pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
		if old != 0 {
			pSelectObject.Call(uintptr(di.HDC), old)
		}
		return 1
	}
	active := false
	section := -1
	switch id {
	case methodsReferenceMethods:
		section = methodsReferenceTabMethods
	case methodsReferenceCalc:
		section = methodsReferenceTabCalc
	case methodsReferenceReplication:
		section = methodsReferenceTabReplication
	case methodsReferenceSources:
		section = methodsReferenceTabSources
	case methodsReferenceReagents:
		section = methodsReferenceTabReagents
	}
	if section >= 0 {
		active = section == methodsReferenceActiveSection
	}

	fill := brushWhite
	fg := rgb(15, 48, 82)
	border := brushBorder
	switch id {
	default:
		if active {
			fill = brushHeader
			fg = rgb(255, 255, 255)
			border = brushHeader
		}
	}
	if di.ItemState&ODS_SELECTED != 0 {
		fill = brushGraphitePressed
		fg = rgb(255, 255, 255)
		border = brushGraphitePressed
	}
	pFillRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(fill))
	pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&di.RcItem)), uintptr(border))
	pSetBkMode.Call(uintptr(di.HDC), TRANSPARENT)
	pSetTextColor.Call(uintptr(di.HDC), fg)
	font := hFontButton
	if active {
		font = hFontBold
	}
	old, _, _ := pSelectObject.Call(uintptr(di.HDC), uintptr(font))
	caption := getText(di.HwndItem)
	r := di.RcItem
	pDrawTextW.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(p(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	if old != 0 {
		pSelectObject.Call(uintptr(di.HDC), old)
	}
	return 1
}

func methodsReferenceDialogWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		methodsReferenceHeaderBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1120, 76, parent, 0)
		methodsReferenceAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 72, 1120, 4, parent, 0)
		methodsReferenceTitleHandle = modalCreate("STATIC", "СПРАВОЧНИК МЕТОДОВ И РАСЧЁТОВ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 12, 1050, 28, parent, 0)
		methodsReferenceSubtitleHandle = modalCreate("STATIC", "Подробная инженерная база · каждый метод открывается по клику", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 42, 900, 24, parent, 0)
		methodsReferenceTargetButton = modalCreate("BUTTON", "ИЗМЕНИТЬ ЦЕЛЕВОЙ ОРИЕНТИР", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 760, 16, 336, 44, parent, methodsReferenceTargetEdit)
		if methodsReferenceTitleFont != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceTitleHandle), WM_SETFONT, uintptr(methodsReferenceTitleFont), 1)
		}
		if hFont != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceSubtitleHandle), WM_SETFONT, uintptr(hFont), 1)
		}
		methodsReferenceMethodsButton = modalCreate("BUTTON", "МЕТОДЫ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 24, 88, 220, 44, parent, methodsReferenceMethods)
		methodsReferenceCalcButton = modalCreate("BUTTON", "КАК СЧИТАЕТСЯ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 252, 88, 220, 44, parent, methodsReferenceCalc)
		methodsReferenceReplicationButton = modalCreate("BUTTON", "АНАЛОГИ / ПЕРЕНОС", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 480, 88, 220, 44, parent, methodsReferenceReplication)
		methodsReferenceSourcesButton = modalCreate("BUTTON", "ИСТОЧНИКИ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 708, 88, 220, 44, parent, methodsReferenceSources)
		methodsReferenceReagentsButton = modalCreate("BUTTON", "РЕАГЕНТЫ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_OWNERDRAW, 936, 88, 220, 44, parent, methodsReferenceReagents)

		methodsReferenceTocTitle = modalCreate("STATIC", "ОГЛАВЛЕНИЕ ПО КЛАССАМ ОСЛОЖНЕНИЙ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 158, 380, 30, parent, 0)
		methodsReferenceClassLabel = modalCreate("STATIC", "ФИЛЬТР · раскрывающийся список по классам осложнений", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 190, 380, 22, parent, 0)
		methodsReferenceClassComboHandle = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 24, 214, 380, 240, parent, methodsReferenceClassCombo)
		methodsReferenceTocListHandle = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 24, 248, 380, 150, parent, methodsReferenceTocList)
		methodsReferenceMethodListTitle = modalCreate("STATIC", "МЕТОДЫ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 410, 380, 30, parent, 0)
		methodsReferenceMethodCardTitle = modalCreate("STATIC", "КАРТОЧКА МЕТОДА", WS_CHILD|WS_VISIBLE|SS_LEFT, 420, 158, 600, 30, parent, 0)
		if methodsReferenceHeadingFont != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceTocTitle), WM_SETFONT, uintptr(methodsReferenceHeadingFont), 1)
			pSendMessageW.Call(uintptr(methodsReferenceClassLabel), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
			pSendMessageW.Call(uintptr(methodsReferenceMethodListTitle), WM_SETFONT, uintptr(methodsReferenceHeadingFont), 1)
			pSendMessageW.Call(uintptr(methodsReferenceMethodCardTitle), WM_SETFONT, uintptr(methodsReferenceHeadingFont), 1)
		}
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), WM_SETFONT, uintptr(hFontPanel), 1)
			pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), WM_SETFONT, uintptr(hFontPanel), 1)
			pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), 0x01A0, 0, 24) // LB_SETITEMHEIGHT
		}
		methodsReferenceMethodListHandle = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 24, 190, 380, 470, parent, methodsReferenceMethodList)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), WM_SETFONT, uintptr(hFontPanel), 1)
			pSendMessageW.Call(uintptr(methodsReferenceMethodListHandle), 0x01A0, 0, 26) // LB_SETITEMHEIGHT
		}

		_ = riched20.Load()
		methodsReferenceEdit = modalCreate("RichEdit20W", methodsReferenceBody, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 420, 190, 674, 470, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceEdit), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		pSendMessageW.Call(uintptr(methodsReferenceEdit), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		margins := uintptr(uint32(methodsLeftMargin) | uint32(methodsLeftMargin)<<16)
		pSendMessageW.Call(uintptr(methodsReferenceEdit), emSetMargins, 3, margins)

		methodsReferenceSourceListTitle = modalCreate("STATIC", "ИСТОЧНИКИ · двойной щелчок — открыть / скачать", WS_CHILD|WS_VISIBLE|SS_LEFT, 420, 560, 674, 28, parent, 0)
		if methodsReferenceHeadingFont != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceSourceListTitle), WM_SETFONT, uintptr(methodsReferenceHeadingFont), 1)
		}
		methodsReferenceSourceListHandle = modalCreate("LISTBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 420, 588, 674, 120, parent, methodsReferenceSourceList)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), WM_SETFONT, uintptr(hFontPanel), 1)
			pSendMessageW.Call(uintptr(methodsReferenceSourceListHandle), 0x01A0, 0, 26) // LB_SETITEMHEIGHT
		}

		methodsReferenceReagentListTitle = modalCreate("STATIC", "СОХРАНЁННЫЕ СХЕМЫ ХИМИЗАЦИИ", WS_CHILD|SS_LEFT, 24, 158, 420, 30, parent, 0)
		methodsReferenceReagentListHandle = modalCreate("LISTBOX", "", WS_CHILD|WS_TABSTOP|WS_BORDER|WS_VSCROLL|LBS_NOTIFY|LBS_OWNERDRAWFIXED|LBS_HASSTRINGS, 24, 190, 420, 470, parent, methodsReferenceReagentList)
		if methodsReferenceHeadingFont != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceReagentListTitle), WM_SETFONT, uintptr(methodsReferenceHeadingFont), 1)
		}
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), WM_SETFONT, uintptr(hFontPanel), 1)
			pSendMessageW.Call(uintptr(methodsReferenceReagentListHandle), 0x01A0, 0, 58) // LB_SETITEMHEIGHT
		}

		methodsReferenceFootnoteHandle = modalCreate("STATIC", "РОСА сопровождает решение технолога; подбор конструкции ГНО остаётся в «Автотехнологе».", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 720, 1060, 30, parent, 0)
		if hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(methodsReferenceFootnoteHandle), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}

		populateMethodsReferenceList()
		updateMethodsReferenceModeVisibility()
		layoutMethodsReferenceDialog(hwnd)
		return 0
	case WM_SIZE:
		layoutMethodsReferenceDialog(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 1080
			mmi.PtMinTrackSize.Y = 700
		}
		return 0
	case WM_DRAWITEM:
		di := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
		if di != nil {
			switch int(di.CtlID) {
			case methodsReferenceReagentList:
				return drawMethodChemicalSchemeListItem(di, methodsReferenceCurrentSchemes)
			case methodsReferenceMethods, methodsReferenceCalc, methodsReferenceReplication, methodsReferenceSources, methodsReferenceReagents, methodsReferenceTargetEdit:
				return drawMethodsReferenceButton(di)
			}
		}
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case methodsReferenceHeaderBand:
			return uintptr(brushNav)
		case methodsReferenceAccent:
			return uintptr(brushHeader)
		case methodsReferenceTitleHandle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodsReferenceSubtitleHandle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		case methodsReferenceFootnoteHandle, methodsReferenceTocTitle, methodsReferenceClassLabel, methodsReferenceMethodListTitle, methodsReferenceMethodCardTitle, methodsReferenceSourceListTitle, methodsReferenceReagentListTitle, methodsReferenceReagentInputLabel, methodsReferenceReagentNote:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(brushLight)
		}
	case WM_COMMAND:
		id := int(loWord(wParam))
		notify := int(hiWord(wParam))
		if id == methodsReferenceMethodList && notify == LBN_SELCHANGE {
			openSelectedMethodsReferenceCard()
			layoutMethodsReferenceDialog(hwnd)
			return 0
		}
		if id == methodsReferenceTocList && notify == LBN_SELCHANGE {
			idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceTocListHandle), LB_GETCURSEL, 0, 0)
			setMethodsReferenceClassSelection(int(idx))
			return 0
		}
		if id == methodsReferenceClassCombo && notify == 1 {
			idx, _, _ := pSendMessageW.Call(uintptr(methodsReferenceClassComboHandle), CB_GETCURSEL, 0, 0)
			setMethodsReferenceClassSelection(int(idx))
			return 0
		}
		if id == methodsReferenceReagentList && notify == LBN_SELCHANGE {
			openSelectedMethodsReferenceChemicalScheme()
			return 0
		}
		if id == methodsReferenceSourceList {
			if notify == 1 && methodsReferenceSourceMode() { // LBN_SELCHANGE
				if src, ok := selectedMethodsReferenceSource(); ok {
					setMethodsReferenceCard(methodsReferenceSourceDetailText(src))
				}
				return 0
			}
			if notify == 2 { // LBN_DBLCLK
				if src, ok := selectedMethodsReferenceSource(); ok {
					handleMethodsReferenceSourceAction(hwnd, src)
				}
				return 0
			}
		}
		switch id {
		case methodsReferenceTargetEdit:
			methodsReferenceOpenTarget = true
			methodsReferenceDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case methodsReferenceMethods:
			setMethodsReferenceSection(methodsReferenceTabMethods, "")
			layoutMethodsReferenceDialog(hwnd)
			return 0
		case methodsReferenceCalc:
			setMethodsReferenceSection(methodsReferenceTabCalc, methodsReferenceCalculationText())
			layoutMethodsReferenceDialog(hwnd)
			return 0
		case methodsReferenceReplication:
			if strings.TrimSpace(methodsReferenceReplicationBody) != "" {
				setMethodsReferenceSection(methodsReferenceTabReplication, methodsReferenceReplicationBody)
			} else {
				setMethodsReferenceSection(methodsReferenceTabReplication, methodsReferenceReplicationText())
			}
			layoutMethodsReferenceDialog(hwnd)
			return 0
		case methodsReferenceSources:
			setMethodsReferenceSection(methodsReferenceTabSources, "")
			layoutMethodsReferenceDialog(hwnd)
			return 0
		case methodsReferenceReagents:
			setMethodsReferenceSection(methodsReferenceTabReagents, "")
			layoutMethodsReferenceDialog(hwnd)
			return 0
		}
	case labMappingClose:
		methodsReferenceDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func extractInitialMethodName(body string) string {
	for _, prefix := range []string{"Метод / мероприятие:", "Метод:"} {
		if i := strings.Index(body, prefix); i >= 0 {
			rest := body[i+len(prefix):]
			if j := strings.IndexAny(rest, "\r\n"); j >= 0 {
				rest = rest[:j]
			}
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

func openMethodsReferenceDialog(title, body string) {
	methodsReferenceTitle = title
	methodsReferenceMethodsBody = methodsReferenceMethodsText()
	methodsReferenceBody = ""
	methodsReferenceReplicationBody = ""
	methodsReferenceActiveSection = methodsReferenceTabMethods
	methodsReferenceInitialMethodName = ""
	methodsReferenceInitialCard = ""
	if strings.TrimSpace(body) != "" {
		if strings.Contains(body, "КАРТОЧКА МЕТОДА") {
			methodsReferenceInitialCard = body
			methodsReferenceInitialMethodName = extractInitialMethodName(body)
			methodsReferenceActiveSection = methodsReferenceTabMethods
		} else if strings.Contains(body, "ТИРАЖИРОВАНИЕ") {
			methodsReferenceReplicationBody = body
			methodsReferenceBody = body
			methodsReferenceActiveSection = methodsReferenceTabReplication
		}
	}
	methodsReferenceEdit = 0
	methodsReferenceHeaderBand = 0
	methodsReferenceAccent = 0
	methodsReferenceTitleHandle = 0
	methodsReferenceSubtitleHandle = 0
	methodsReferenceFootnoteHandle = 0
	methodsReferenceMethodsButton = 0
	methodsReferenceCalcButton = 0
	methodsReferenceReplicationButton = 0
	methodsReferenceSourcesButton = 0
	methodsReferenceReagentsButton = 0
	methodsReferenceTargetButton = 0
	methodsReferenceMethodListHandle = 0
	methodsReferenceMethodListTitle = 0
	methodsReferenceClassLabel = 0
	methodsReferenceClassComboHandle = 0
	methodsReferenceTocTitle = 0
	methodsReferenceTocListHandle = 0
	methodsReferenceMethodCardTitle = 0
	methodsReferenceSourceListHandle = 0
	methodsReferenceSourceListTitle = 0
	methodsReferenceReagentListHandle = 0
	methodsReferenceReagentListTitle = 0
	methodsReferenceReagentInputLabel = 0
	methodsReferenceReagentInput = 0
	methodsReferenceReagentAddButton = 0
	methodsReferenceReagentNote = 0
	methodsReferenceMinimizeButton = 0
	methodsReferenceMaxRestoreButton = 0
	methodsReferenceCloseButton = 0
	methodsReferenceCurrentSources = nil
	methodsReferenceCurrentSchemes = nil
	methodsReferenceCatalog = nil
	methodsReferenceFilteredCatalog = nil
	methodsReferenceClassCodes = nil
	methodsReferenceDone = false
	methodsReferenceOpenTarget = false
	ensureMethodsReferenceHeadingFonts()

	cls := p("ROSAMethodsReferenceDialogV6")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(methodsReferenceDialogWndProc),
		HInstance:     syscall.Handle(inst),
		HbrBackground: background,
		LpszClassName: cls,
		HIcon:         hIconBig,
		HIconSm:       hIconSmall,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	metrics := user32.NewProc("GetSystemMetrics")
	screenW, _, _ := metrics.Call(0)
	screenH, _, _ := metrics.Call(1)
	windowW := min32(1500, max32(1120, int32(screenW)-160))
	windowH := min32(900, max32(740, int32(screenH)-130))
	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(p("РОСА — Справочник"))),
		WS_OVERLAPPEDWINDOW|WS_VISIBLE|0x02000000,
		0x80000000, 0x80000000,
		uintptr(windowW), uintptr(windowH),
		uintptr(hMain), 0, inst, 0,
	)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		deleteMethodsReferenceHeadingFonts()
		msg(title, methodsReferenceText(), MB_OK|MB_ICONINFORMATION)
		return
	}
	var message MSG
	for !methodsReferenceDone {
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
	methodsReferenceEdit = 0
	methodsReferenceHeaderBand = 0
	methodsReferenceAccent = 0
	methodsReferenceTitleHandle = 0
	methodsReferenceSubtitleHandle = 0
	methodsReferenceFootnoteHandle = 0
	methodsReferenceMethodsButton = 0
	methodsReferenceCalcButton = 0
	methodsReferenceReplicationButton = 0
	methodsReferenceSourcesButton = 0
	methodsReferenceReagentsButton = 0
	methodsReferenceTargetButton = 0
	methodsReferenceMethodListHandle = 0
	methodsReferenceMethodListTitle = 0
	methodsReferenceMethodCardTitle = 0
	methodsReferenceSourceListHandle = 0
	methodsReferenceSourceListTitle = 0
	methodsReferenceReagentListHandle = 0
	methodsReferenceReagentListTitle = 0
	methodsReferenceReagentInputLabel = 0
	methodsReferenceReagentInput = 0
	methodsReferenceReagentAddButton = 0
	methodsReferenceReagentNote = 0
	methodsReferenceMinimizeButton = 0
	methodsReferenceMaxRestoreButton = 0
	methodsReferenceCloseButton = 0
	methodsReferenceCurrentSources = nil
	methodsReferenceCurrentSchemes = nil
	methodsReferenceCatalog = nil
	deleteMethodsReferenceHeadingFonts()
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
	if methodsReferenceOpenTarget {
		activatePage(ID_MRP, true)
	}
}
