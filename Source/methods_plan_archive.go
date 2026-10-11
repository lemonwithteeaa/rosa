//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	methodPlanEditorStatus         = 9690
	methodPlanEditorDue            = 9691
	methodPlanEditorOwner          = 9692
	methodPlanEditorControlAt      = 9693
	methodPlanEditorCriterion      = 9694
	methodPlanEditorParams         = 9695
	methodPlanEditorSave           = 9696
	methodPlanEditorCancel         = 9697
	methodPlanEditorCompletedAt    = 9710
	methodPlanEditorExecution      = 9711
	methodPlanEditorOutcome        = 9712
	methodPlanEditorOutcomeComment = 9713

	methodArchiveRestore = 9700
	methodArchiveClose   = 9701
)

var methodPlanStatuses = []string{"Черновик", "Выбран", "Утверждён", "Назначен", "Выполнен", "Отменён"}
var methodPlanControlOutcomes = []string{"Оценка ещё не завершена", "Эффект подтверждён", "Подтверждён частично", "Не подтверждён"}

var (
	methodPlanDialogDone         bool
	methodPlanDialogIndex        int
	methodPlanStatusCombo        syscall.Handle
	methodPlanDueEdit            syscall.Handle
	methodPlanOwnerEdit          syscall.Handle
	methodPlanControlDateEdit    syscall.Handle
	methodPlanCriterionEdit      syscall.Handle
	methodPlanCompletedAtEdit    syscall.Handle
	methodPlanExecutionEdit      syscall.Handle
	methodPlanOutcomeCombo       syscall.Handle
	methodPlanOutcomeCommentEdit syscall.Handle

	methodArchiveDialogDone     bool
	methodArchiveDialogDecision approvedMethodDecision
	methodArchiveHeaderBand     syscall.Handle
	methodArchiveAccent         syscall.Handle
	methodArchiveTitleHandle    syscall.Handle
	methodArchiveSubtitle       syscall.Handle
	methodArchiveSection        syscall.Handle
	methodArchiveBody           syscall.Handle
	methodArchiveCloseButton    syscall.Handle
	methodArchiveRestoreButton  syscall.Handle
	objectHistoryTitle          string
	objectHistoryBody           string
)

type methodPlanAuditEntry struct {
	ObjectKey            string `json:"object_key"`
	Method               string `json:"method"`
	DecisionID           string `json:"decision_id,omitempty"`
	ChangedAt            string `json:"changed_at"`
	ChangedBy            string `json:"changed_by,omitempty"`
	BeforeStatus         string `json:"before_status,omitempty"`
	AfterStatus          string `json:"after_status,omitempty"`
	BeforeDueDate        string `json:"before_due_date,omitempty"`
	AfterDueDate         string `json:"after_due_date,omitempty"`
	BeforeResponsible    string `json:"before_responsible,omitempty"`
	AfterResponsible     string `json:"after_responsible,omitempty"`
	BeforeControlDate    string `json:"before_control_date,omitempty"`
	AfterControlDate     string `json:"after_control_date,omitempty"`
	BeforeCriterion      string `json:"before_criterion,omitempty"`
	AfterCriterion       string `json:"after_criterion,omitempty"`
	BeforeCompletedAt    string `json:"before_completed_at,omitempty"`
	AfterCompletedAt     string `json:"after_completed_at,omitempty"`
	BeforeControlOutcome string `json:"before_control_outcome,omitempty"`
	AfterControlOutcome  string `json:"after_control_outcome,omitempty"`
}

func methodPlanAuditPath() string { return sharedPath("method_plan_journal.jsonl") }

func appendMethodPlanAudit(entry methodPlanAuditEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	path := methodPlanAuditPath()
	if err := os.MkdirAll(filepathDir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func methodPlanCanonicalStatus(s string) string {
	s = strings.TrimSpace(s)
	for _, allowed := range methodPlanStatuses {
		if strings.EqualFold(s, allowed) {
			return allowed
		}
	}
	switch strings.ToLower(s) {
	case "в плане", "выбрано":
		return "Выбран"
	case "утверждено", "утверждено технологом":
		return "Утверждён"
	}
	if s == "" {
		return "Черновик"
	}
	return s
}

func methodPlanItemMethod(p PlanItem) string {
	if strings.TrimSpace(p.MethodName) != "" {
		return strings.TrimSpace(p.MethodName)
	}
	a := strings.TrimSpace(p.Action)
	for _, prefix := range []string{"Утверждённый метод: ", "Метод / мероприятие: ", "Метод: "} {
		if strings.HasPrefix(strings.ToLower(a), strings.ToLower(prefix)) {
			return strings.TrimSpace(a[len(prefix):])
		}
	}
	return ""
}

func methodPlanDateDisplay(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return strings.TrimSpace(v)
}

func methodPlanResponsibleDisplay(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return strings.TrimSpace(v)
}

func methodPlanControlDisplay(p PlanItem) string {
	parts := make([]string, 0, 4)
	if d := methodPlanDateDisplay(p.ControlDate); d != "—" {
		parts = append(parts, d)
	}
	if c := strings.TrimSpace(p.ControlCriterion); c != "" {
		parts = append(parts, c)
	}
	if o := strings.TrimSpace(p.ControlOutcome); o != "" {
		parts = append(parts, o)
	}
	if len(parts) == 0 {
		return "контроль не задан"
	}
	return strings.Join(parts, " · ")
}

func methodPlanParametersDisplay(p PlanItem) string {
	if strings.TrimSpace(p.DecisionID) != "" {
		return "Снимок версии"
	}
	return "Текущие"
}

func methodDecisionContainsMethod(d approvedMethodDecision, method string) bool {
	n := normalizeMethodText(method)
	for _, x := range normalizeMethodList(d.Methods) {
		if normalizeMethodText(x) == n {
			return true
		}
	}
	if len(d.Methods) == 0 && normalizeMethodText(d.Method) == n {
		return true
	}
	return false
}

func methodPlanFindLatestIndex(objectKey, method, decisionID string) int {
	n := normalizeMethodText(method)
	key := strings.TrimSpace(objectKey)
	decisionID = strings.TrimSpace(decisionID)
	if decisionID != "" {
		for i := len(plans) - 1; i >= 0; i-- {
			if strings.EqualFold(strings.TrimSpace(plans[i].ObjectKey), key) &&
				normalizeMethodText(methodPlanItemMethod(plans[i])) == n &&
				strings.EqualFold(strings.TrimSpace(plans[i].DecisionID), decisionID) {
				return i
			}
		}
	}
	for i := len(plans) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(plans[i].ObjectKey), key) && normalizeMethodText(methodPlanItemMethod(plans[i])) == n {
			return i
		}
	}
	return -1
}

func methodPlanDefaultCriterion(objectKey, method string) string {
	r := methodImplementationSnapshotForName(objectKey, method)
	for _, f := range r.Fields {
		if f.Key == "control" && strings.TrimSpace(f.Value) != "" {
			return strings.TrimSpace(f.Value)
		}
	}
	return "Контроль фактического эффекта и наработки после мероприятия"
}

func methodPlanVirtualItem(objectKey, method string) PlanItem {
	p := PlanItem{
		ObjectKey: strings.TrimSpace(objectKey), MethodName: strings.TrimSpace(method),
		Action: "Метод / мероприятие: " + strings.TrimSpace(method), Status: "Выбран",
		DueDate: "—", Responsible: "—", ControlDate: "—",
		ControlCriterion: methodPlanDefaultCriterion(objectKey, method),
	}
	if d, ok := latestApprovedMethodDecision(objectKey); ok && methodDecisionContainsMethod(d, method) {
		p.Status = "Утверждён"
		p.DecisionID = d.ID
		p.ParametersRef = d.ID + "::" + strings.TrimSpace(method)
	}
	return p
}

func methodPlanDisplayItem(objectKey, method string) PlanItem {
	decisionID := ""
	if d, ok := latestApprovedMethodDecision(objectKey); ok && methodDecisionContainsMethod(d, method) {
		decisionID = d.ID
	}
	if idx := methodPlanFindLatestIndex(objectKey, method, decisionID); idx >= 0 {
		p := plans[idx]
		p.Status = methodPlanCanonicalStatus(p.Status)
		if strings.TrimSpace(p.MethodName) == "" {
			p.MethodName = strings.TrimSpace(method)
		}
		return p
	}
	return methodPlanVirtualItem(objectKey, method)
}

func ensureMethodsCurrentPlanItems() error {
	key := strings.TrimSpace(selectedObjectKey)
	if key == "" {
		return fmt.Errorf("сначала закрепите объект на Главной")
	}
	names := currentMethodsPlanNames()
	if len(names) == 0 {
		return fmt.Errorf("сначала выберите метод или сформируйте комплекс мероприятий")
	}
	latest, hasApproved := latestApprovedMethodDecision(key)
	changed := false
	for _, name := range names {
		decisionID := ""
		if hasApproved && methodDecisionContainsMethod(latest, name) {
			decisionID = latest.ID
		}
		if methodPlanFindLatestIndex(key, name, decisionID) >= 0 {
			continue
		}
		p := methodPlanVirtualItem(key, name)
		p.ID = "method-plan-" + strconvSafeNano() + fmt.Sprintf("-%d", len(plans)+1)
		p.UpdatedAt = time.Now().Format("02.01.2006 15:04")
		p.UpdatedBy = methodImplementationAuditUser()
		plans = append(plans, p)
		changed = true
	}
	if changed {
		return saveJSON(sharedPath("plan.json"), plans)
	}
	return nil
}

func strconvSafeNano() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

func methodPlanIndexForDisplayRow(row int) int {
	names := currentMethodsPlanNames()
	if row < 0 || row >= len(names) {
		return -1
	}
	key := strings.TrimSpace(selectedObjectKey)
	decisionID := ""
	if d, ok := latestApprovedMethodDecision(key); ok && methodDecisionContainsMethod(d, names[row]) {
		decisionID = d.ID
	}
	return methodPlanFindLatestIndex(key, names[row], decisionID)
}

func methodPlanValidateDate(label, value string) error {
	value = strings.TrimSpace(value)
	if value == "" || value == "—" {
		return nil
	}
	if _, err := time.ParseInLocation("02.01.2006", value, time.Local); err != nil {
		return fmt.Errorf("%s: используйте формат ДД.ММ.ГГГГ или «—»", label)
	}
	return nil
}

func methodPlanSnapshotText(p PlanItem) string {
	method := methodPlanItemMethod(p)
	var r MethodImplementationRecord
	found := false
	if strings.TrimSpace(p.DecisionID) != "" {
		ensureApprovedMethodsLoaded()
		for i := len(approvedMethodDecisions) - 1; i >= 0; i-- {
			d := approvedMethodDecisions[i]
			if d.ID != p.DecisionID {
				continue
			}
			for _, snap := range d.Implementation {
				if normalizeMethodText(snap.Method) == normalizeMethodText(method) {
					r = snap
					found = true
					break
				}
			}
			break
		}
	}
	if !found {
		r = methodImplementationSnapshotForName(p.ObjectKey, method)
	}
	var b strings.Builder
	b.WriteString("ПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\n\r\n")
	fmt.Fprintf(&b, "Метод: %s\r\n", dash(method))
	if strings.TrimSpace(p.DecisionID) != "" {
		fmt.Fprintf(&b, "Версия решения: %s\r\n", p.DecisionID)
	} else {
		b.WriteString("Источник: текущие параметры до утверждения версии.\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(methodImplementationReportText(r))
	return b.String()
}

func openMethodPlanParametersAtRow(row int) {
	if err := ensureMethodsCurrentPlanItems(); err != nil {
		msg("План мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	idx := methodPlanIndexForDisplayRow(row)
	if idx < 0 || idx >= len(plans) {
		return
	}
	openMethodsReferenceDialog("РОСА — параметры реализации", methodPlanSnapshotText(plans[idx]))
}

func methodPlanEditorWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		if methodPlanDialogIndex < 0 || methodPlanDialogIndex >= len(plans) {
			return 0
		}
		item := plans[methodPlanDialogIndex]
		title := modalCreate("STATIC", "ПЛАН ИСПОЛНЕНИЯ МЕРОПРИЯТИЯ", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 18, 760, 32, parent, 0)
		if hFontBold != 0 {
			pSendMessageW.Call(uintptr(title), WM_SETFONT, uintptr(hFontBold), 1)
		}
		sub := modalCreate("STATIC", methodPlanItemMethod(item), WS_CHILD|WS_VISIBLE|SS_LEFT|SS_ENDELLIPSIS, 24, 52, 760, 26, parent, 0)
		if hFont != 0 {
			pSendMessageW.Call(uintptr(sub), WM_SETFONT, uintptr(hFont), 1)
		}

		modalCreate("STATIC", "Состояние", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 96, 170, 24, parent, 0)
		methodPlanStatusCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 210, 90, 250, 230, parent, methodPlanEditorStatus)
		selected := 0
		current := methodPlanCanonicalStatus(item.Status)
		for i, s := range methodPlanStatuses {
			pSendMessageW.Call(uintptr(methodPlanStatusCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(s))))
			if s == current {
				selected = i
			}
		}
		pSendMessageW.Call(uintptr(methodPlanStatusCombo), CB_SETCURSEL, uintptr(selected), 0)

		modalCreate("STATIC", "Плановая дата", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 142, 170, 24, parent, 0)
		methodPlanDueEdit = modalCreate("EDIT", methodPlanDateDisplay(item.DueDate), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 210, 136, 250, 32, parent, methodPlanEditorDue)
		modalCreate("STATIC", "Ответственный", WS_CHILD|WS_VISIBLE|SS_LEFT, 486, 142, 135, 24, parent, 0)
		methodPlanOwnerEdit = modalCreate("EDIT", methodPlanResponsibleDisplay(item.Responsible), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 622, 136, 250, 32, parent, methodPlanEditorOwner)

		modalCreate("STATIC", "Контрольная дата", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 190, 170, 24, parent, 0)
		methodPlanControlDateEdit = modalCreate("EDIT", methodPlanDateDisplay(item.ControlDate), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 210, 184, 250, 32, parent, methodPlanEditorControlAt)
		modalCreate("STATIC", "Критерий контроля", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 238, 170, 24, parent, 0)
		methodPlanCriterionEdit = modalCreate("EDIT", methodPlanControlCriterionOrDefault(item), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 210, 232, 662, 72, parent, methodPlanEditorCriterion)

		modalCreate("STATIC", "Фактическая дата выполнения", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 326, 180, 24, parent, 0)
		methodPlanCompletedAtEdit = modalCreate("EDIT", methodPlanDateDisplay(item.CompletedAt), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL, 210, 320, 250, 32, parent, methodPlanEditorCompletedAt)
		modalCreate("STATIC", "Факт выполнения / результат", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 372, 180, 24, parent, 0)
		methodPlanExecutionEdit = modalCreate("EDIT", strings.TrimSpace(item.ExecutionResult), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 210, 366, 662, 64, parent, methodPlanEditorExecution)

		modalCreate("STATIC", "Итог контроля", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 450, 180, 24, parent, 0)
		methodPlanOutcomeCombo = modalCreate("COMBOBOX", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST, 210, 444, 330, 220, parent, methodPlanEditorOutcome)
		outcomeSelected := 0
		currentOutcome := strings.TrimSpace(item.ControlOutcome)
		for i, outcome := range methodPlanControlOutcomes {
			pSendMessageW.Call(uintptr(methodPlanOutcomeCombo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(outcome))))
			if strings.EqualFold(currentOutcome, outcome) {
				outcomeSelected = i
			}
		}
		pSendMessageW.Call(uintptr(methodPlanOutcomeCombo), CB_SETCURSEL, uintptr(outcomeSelected), 0)
		modalCreate("STATIC", "Комментарий контроля", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 496, 180, 24, parent, 0)
		methodPlanOutcomeCommentEdit = modalCreate("EDIT", strings.TrimSpace(item.ControlComment), WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|WS_VSCROLL, 210, 490, 662, 72, parent, methodPlanEditorOutcomeComment)

		refText := "Текущие параметры реализации"
		if strings.TrimSpace(item.DecisionID) != "" {
			refText = "Сохранённый снимок параметров версии решения"
		}
		modalCreate("STATIC", refText, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_ENDELLIPSIS, 24, 580, 520, 26, parent, 0)
		params := modalCreate("BUTTON", "ОТКРЫТЬ ПАРАМЕТРЫ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 622, 572, 250, 38, parent, methodPlanEditorParams)
		cancel := modalCreate("BUTTON", "ОТМЕНА", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 622, 640, 118, 40, parent, methodPlanEditorCancel)
		save := modalCreate("BUTTON", "СОХРАНИТЬ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 754, 640, 118, 40, parent, methodPlanEditorSave)
		for _, h := range []syscall.Handle{params, cancel, save} {
			if hFontButton != 0 {
				pSendMessageW.Call(uintptr(h), WM_SETFONT, uintptr(hFontButton), 1)
			}
		}
		note := modalCreate("STATIC", "Дата — ДД.ММ.ГГГГ. Для «Назначен» обязательны срок и ответственный; для «Выполнен» — фактическая дата и результат. Итог контроля хранится отдельно от комментария.", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 632, 570, 52, parent, 0)
		if hFontHeaderSmall != 0 {
			pSendMessageW.Call(uintptr(note), WM_SETFONT, uintptr(hFontHeaderSmall), 1)
		}
		return 0
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case methodPlanEditorParams:
			if methodPlanDialogIndex >= 0 && methodPlanDialogIndex < len(plans) {
				openMethodsReferenceDialog("РОСА — параметры реализации", methodPlanSnapshotText(plans[methodPlanDialogIndex]))
			}
			return 0
		case methodPlanEditorCancel:
			methodPlanDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case methodPlanEditorSave:
			if methodPlanDialogIndex < 0 || methodPlanDialogIndex >= len(plans) {
				return 0
			}
			old := plans[methodPlanDialogIndex]
			idx, _, _ := pSendMessageW.Call(uintptr(methodPlanStatusCombo), CB_GETCURSEL, 0, 0)
			status := "Черновик"
			if int(idx) >= 0 && int(idx) < len(methodPlanStatuses) {
				status = methodPlanStatuses[int(idx)]
			}
			due := strings.TrimSpace(getText(methodPlanDueEdit))
			if due == "" {
				due = "—"
			}
			owner := strings.TrimSpace(getText(methodPlanOwnerEdit))
			if owner == "" {
				owner = "—"
			}
			controlDate := strings.TrimSpace(getText(methodPlanControlDateEdit))
			if controlDate == "" {
				controlDate = "—"
			}
			criterion := strings.TrimSpace(getText(methodPlanCriterionEdit))
			completedAt := strings.TrimSpace(getText(methodPlanCompletedAtEdit))
			if completedAt == "" {
				completedAt = "—"
			}
			executionResult := strings.TrimSpace(getText(methodPlanExecutionEdit))
			outcomeIdx, _, _ := pSendMessageW.Call(uintptr(methodPlanOutcomeCombo), CB_GETCURSEL, 0, 0)
			controlOutcome := methodPlanControlOutcomes[0]
			if int(outcomeIdx) >= 0 && int(outcomeIdx) < len(methodPlanControlOutcomes) {
				controlOutcome = methodPlanControlOutcomes[int(outcomeIdx)]
			}
			controlComment := strings.TrimSpace(getText(methodPlanOutcomeCommentEdit))
			if err := methodPlanValidateDate("Плановая дата", due); err != nil {
				msg("План мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			if err := methodPlanValidateDate("Контрольная дата", controlDate); err != nil {
				msg("План мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			if err := methodPlanValidateDate("Фактическая дата выполнения", completedAt); err != nil {
				msg("План мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			if (status == "Утверждён" || status == "Назначен" || status == "Выполнен") && strings.TrimSpace(old.DecisionID) == "" {
				msg("План мероприятий", "Сначала утвердите метод/комплекс. До утверждения доступны состояния «Черновик», «Выбран» и «Отменён».", MB_OK|MB_ICONWARNING)
				return 0
			}
			if status == "Назначен" && (due == "—" || owner == "—") {
				msg("План мероприятий", "Для состояния «Назначен» укажите плановую дату и ответственного.", MB_OK|MB_ICONWARNING)
				return 0
			}
			if status == "Выполнен" && (completedAt == "—" || executionResult == "") {
				msg("План мероприятий", "Для состояния «Выполнен» укажите фактическую дату выполнения и результат.", MB_OK|MB_ICONWARNING)
				return 0
			}
			if controlOutcome == "Оценка ещё не завершена" && status == "Выполнен" && controlComment == "" {
				msg("План мероприятий", "Если контроль ещё не завершён, добавьте пояснение в комментарии контроля.", MB_OK|MB_ICONWARNING)
				return 0
			}
			if criterion == "" {
				criterion = methodPlanDefaultCriterion(old.ObjectKey, methodPlanItemMethod(old))
			}
			updated := old
			updated.Status, updated.DueDate, updated.Responsible = status, due, owner
			updated.ControlDate, updated.ControlCriterion = controlDate, criterion
			updated.CompletedAt, updated.ExecutionResult = completedAt, executionResult
			updated.ControlOutcome, updated.ControlComment = controlOutcome, controlComment
			updated.UpdatedAt, updated.UpdatedBy = time.Now().Format("02.01.2006 15:04"), methodImplementationAuditUser()
			plans[methodPlanDialogIndex] = updated
			if err := saveJSON(sharedPath("plan.json"), plans); err != nil {
				plans[methodPlanDialogIndex] = old
				msg("План мероприятий", "Не удалось сохранить план: "+err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			audit := methodPlanAuditEntry{ObjectKey: updated.ObjectKey, Method: methodPlanItemMethod(updated), DecisionID: updated.DecisionID, ChangedAt: updated.UpdatedAt, ChangedBy: updated.UpdatedBy,
				BeforeStatus: old.Status, AfterStatus: updated.Status, BeforeDueDate: old.DueDate, AfterDueDate: updated.DueDate,
				BeforeResponsible: old.Responsible, AfterResponsible: updated.Responsible, BeforeControlDate: old.ControlDate, AfterControlDate: updated.ControlDate,
				BeforeCriterion: old.ControlCriterion, AfterCriterion: updated.ControlCriterion,
				BeforeCompletedAt: old.CompletedAt, AfterCompletedAt: updated.CompletedAt, BeforeControlOutcome: old.ControlOutcome, AfterControlOutcome: updated.ControlOutcome}
			_ = appendMethodPlanAudit(audit)
			methodPlanDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		methodPlanDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func methodPlanControlCriterionOrDefault(p PlanItem) string {
	if strings.TrimSpace(p.ControlCriterion) != "" {
		return strings.TrimSpace(p.ControlCriterion)
	}
	return methodPlanDefaultCriterion(p.ObjectKey, methodPlanItemMethod(p))
}

func openMethodsPlanEditorAtRow(row int) {
	if err := ensureMethodsCurrentPlanItems(); err != nil {
		msg("План мероприятий", err.Error(), MB_OK|MB_ICONWARNING)
		return
	}
	if row < 0 {
		row = 0
	}
	idx := methodPlanIndexForDisplayRow(row)
	if idx < 0 || idx >= len(plans) {
		msg("План мероприятий", "Не удалось определить выбранное мероприятие.", MB_OK|MB_ICONWARNING)
		return
	}
	methodPlanDialogIndex = idx
	methodPlanDialogDone = false
	cls := p("ROSAMethodPlanEditorV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodPlanEditorWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — План исполнения"))), 0x00C80000|WS_VISIBLE, 0x80000000, 0x80000000, 920, 735, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("План мероприятий", "Не удалось открыть редактор плана.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !methodPlanDialogDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	refreshMethodsVisualTables()
	refreshMethodsPlanTable(methodsClassificationReady)
	refreshPlan()
}

func openMethodsPlanEditorSelected() {
	row := -1
	if methodsPlanList != 0 {
		row = selectedRow(methodsPlanList)
	}
	openMethodsPlanEditorAtRow(row)
}

func methodDecisionByID(id string) (approvedMethodDecision, bool) {
	ensureApprovedMethodsLoaded()
	for i := len(approvedMethodDecisions) - 1; i >= 0; i-- {
		if approvedMethodDecisions[i].ID == id {
			return approvedMethodDecisions[i], true
		}
	}
	return approvedMethodDecision{}, false
}

func methodDecisionStoredVersion(d approvedMethodDecision) int {
	if d.VersionNumber > 0 {
		return d.VersionNumber
	}
	version := 0
	for _, x := range approvedMethodDecisions {
		if strings.EqualFold(strings.TrimSpace(x.ObjectKey), strings.TrimSpace(d.ObjectKey)) {
			version++
		}
		if x.ID == d.ID {
			return version
		}
	}
	return version
}

func nextMethodDecisionVersion(objectKey string) int {
	ensureApprovedMethodsLoaded()
	n := 0
	for _, d := range approvedMethodDecisions {
		if strings.EqualFold(strings.TrimSpace(d.ObjectKey), strings.TrimSpace(objectKey)) {
			if d.VersionNumber > n {
				n = d.VersionNumber
			}
			if d.VersionNumber == 0 {
				n++
			}
		}
	}
	if n < 0 {
		n = 0
	}
	return n + 1
}

func methodArchiveDecisionAtRow(row int) (approvedMethodDecision, bool) {
	if row < 0 || row >= len(methodsArchiveRows) {
		return approvedMethodDecision{}, false
	}
	return methodsArchiveRows[row], true
}

func openMethodsArchiveObjectHistoryAtRow(row int) {
	d, ok := methodArchiveDecisionAtRow(row)
	if !ok {
		return
	}
	objectKey := strings.TrimSpace(d.ObjectKey)
	if objectKey == "" {
		return
	}
	ensureApprovedMethodsLoaded()
	var rows []approvedMethodDecision
	for _, item := range approvedMethodDecisions {
		if methodsObjectKeysEquivalent(item.ObjectKey, objectKey) {
			rows = append(rows, item)
		}
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(d.ID)), "teaching-") {
		rows = append(rows, teachingMethodArchiveDecisions(objectKey, time.Now())...)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return methodDecisionStoredVersion(rows[i]) > methodDecisionStoredVersion(rows[j])
	})
	var b strings.Builder
	fmt.Fprintf(&b, "ОБЪЕКТ: %s\r\n\r\n", readableObjectKey(objectKey))
	if len(rows) == 0 {
		b.WriteString("Утверждённых решений по объекту пока нет.\r\n")
	} else {
		b.WriteString("ИСТОРИЯ РЕШЕНИЙ (учебные данные отдельно отмечены)\r\n\r\n")
		for i, item := range rows {
			if i > 0 {
				b.WriteString("\r\n")
			}
			forecast := "—"
			if item.ForecastBaseDays > 0 {
				forecast = fmt.Sprintf("%.0f сут.", item.ForecastBaseDays)
				if item.ForecastLowDays > 0 && item.ForecastHighDays > 0 {
					forecast = fmt.Sprintf("%.0f / %.0f / %.0f сут.", item.ForecastLowDays, item.ForecastBaseDays, item.ForecastHighDays)
				}
			}
			fmt.Fprintf(&b, "v%d · %s · %s\r\nОсложнения: %s\r\nМетоды: %s\r\nПрогноз: %s\r\nДостоверность истории: %s\r\nОтветственный: %s\r\n",
				methodDecisionStoredVersion(item), dash(item.ApprovedAt), methodArchiveVersionStatus(item), dash(item.Classification), dash(item.Method), forecast, dash(item.ForecastPrecision), dash(item.ApprovedBy))
			for _, implementation := range item.Implementation {
				b.WriteString(methodImplementationReportText(implementation) + "\r\n")
			}
			for _, plan := range item.Plan {
				fmt.Fprintf(&b, "План: %s · %s; срок %s; контроль %s.\r\n", dash(methodPlanItemMethod(plan)), methodPlanCanonicalStatus(plan.Status), methodPlanDateDisplay(plan.DueDate), methodPlanControlDisplay(plan))
			}
		}
	}
	scenarios := alternativeScenariosForObject(objectKey)
	b.WriteString("\r\nВСЕ РАССМОТРЕННЫЕ СХЕМЫ\r\n")
	if len(scenarios) == 0 {
		b.WriteString("Сохранённых альтернативных схем нет.\r\n")
	}
	for _, s := range scenarios {
		fmt.Fprintf(&b, "\r\n%s · %s · %s\r\nОсложнения: %s\r\nМетоды: %s\r\nДоступность: %s. %s\r\nМРП: %.0f / %.0f / %.0f сут.\r\nОснование: %s\r\n",
			dash(s.Name), dash(s.Status), dash(s.UpdatedAt), dash(s.Classification), strings.Join(alternativeScenarioMethodsForCalculation(s), " + "), dash(s.Availability), dash(s.AvailabilityNote), s.ForecastLowDays, s.ForecastBaseDays, s.ForecastHighDays, dash(s.SelectionBasis))
	}
	openObjectHistoryDialog("МЕТОДЫ И МЕРОПРИЯТИЯ · ИСТОРИЯ ОБЪЕКТА", b.String())
}

func methodArchiveVersionStatus(d approvedMethodDecision) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(d.ID)), "teaching-") {
		return "УЧЕБНЫЙ ПРИМЕР"
	}
	v := methodDecisionStoredVersion(d)
	if strings.TrimSpace(d.RestoredFromID) != "" {
		from := "?"
		if old, ok := methodDecisionByID(d.RestoredFromID); ok {
			from = fmt.Sprintf("%d", methodDecisionStoredVersion(old))
		}
		return fmt.Sprintf("Версия %d · восстановлено из v%s", v, from)
	}
	return fmt.Sprintf("Версия %d · утверждено", v)
}

func methodArchivePreviewText(d approvedMethodDecision) string {
	var b strings.Builder
	fmt.Fprintf(&b, "АРХИВНАЯ ВЕРСИЯ РЕШЕНИЯ\r\n\r\nВерсия: %d\r\nДата: %s\r\nАвтор: %s\r\nОбъект: %s\r\nКлассификация: %s\r\nМетод / комплекс: %s\r\nСтатус: %s\r\n", methodDecisionStoredVersion(d), dash(d.ApprovedAt), dash(d.ApprovedBy), dash(d.ObjectKey), dash(d.Classification), dash(d.Method), methodArchiveVersionStatus(d))
	if strings.TrimSpace(d.RestoredFromID) != "" {
		b.WriteString("Источник восстановления: " + d.RestoredFromID + "\r\n")
	}
	fmt.Fprintf(&b, "\r\nПрогноз наработки: %.0f / %.0f / %.0f сут.\r\nДостаточность истории: %s\r\n", d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays, dash(d.ForecastPrecision))
	if len(d.Implementation) > 0 {
		b.WriteString("\r\nПАРАМЕТРЫ РЕАЛИЗАЦИИ\r\n")
		for _, r := range d.Implementation {
			b.WriteString(methodImplementationReportText(r) + "\r\n")
		}
	}
	if len(d.Plan) > 0 {
		b.WriteString("\r\nПЛАН В ЭТОЙ ВЕРСИИ\r\n")
		for _, p0 := range d.Plan {
			fmt.Fprintf(&b, "• %s — %s; план %s; ответственный %s; контроль %s.\r\n", dash(methodPlanItemMethod(p0)), methodPlanCanonicalStatus(p0.Status), methodPlanDateDisplay(p0.DueDate), methodPlanResponsibleDisplay(p0.Responsible), methodPlanControlDisplay(p0))
		}
	}
	b.WriteString("\r\nПросмотр архива ничего не меняет. Восстановление создаёт НОВУЮ версию и не перезаписывает текущую или архивную запись.")
	return b.String()
}

func cloneMethodImplementationRecords(in []MethodImplementationRecord) []MethodImplementationRecord {
	out := make([]MethodImplementationRecord, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].Fields = append([]methodImplementationField(nil), in[i].Fields...)
	}
	return out
}

func applyMethodImplementationSnapshotsToMemory(objectKey string, snapshots []MethodImplementationRecord, user, at string) {
	ensureMethodImplementationRecordsLoaded()
	for _, snap := range snapshots {
		r := snap
		r.ObjectKey = strings.TrimSpace(objectKey)
		r.UpdatedAt = at
		r.UpdatedBy = user + " · восстановлено из архива"
		idx := methodImplementationRecordIndex(r.ObjectKey, r.Method)
		if idx >= 0 {
			methodImplementationRecords[idx] = r
		} else {
			methodImplementationRecords = append(methodImplementationRecords, r)
		}
	}
}

func clonePlanItems(in []PlanItem) []PlanItem {
	out := make([]PlanItem, len(in))
	copy(out, in)
	return out
}

func methodPlanItemsForRestoredDecision(d approvedMethodDecision, old approvedMethodDecision) []PlanItem {
	methods := normalizeMethodList(d.Methods)
	if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
		methods = []string{strings.TrimSpace(d.Method)}
	}
	byMethod := map[string]PlanItem{}
	for _, p0 := range old.Plan {
		byMethod[normalizeMethodText(methodPlanItemMethod(p0))] = p0
	}
	out := make([]PlanItem, 0, len(methods))
	for i, method := range methods {
		p0, ok := byMethod[normalizeMethodText(method)]
		if !ok {
			p0 = methodPlanVirtualItem(d.ObjectKey, method)
			p0.Status = "Утверждён"
		}
		p0.ID = fmt.Sprintf("method-%s-%d", d.ID, i+1)
		p0.ObjectKey, p0.MethodName, p0.DecisionID = d.ObjectKey, method, d.ID
		p0.ParametersRef = d.ID + "::" + method
		p0.Action = "Метод / мероприятие: " + method
		// A restored decision is a new version. Historical execution/control facts
		// stay in the old version and must never be inherited as current facts.
		p0.Status = "Утверждён"
		p0.CompletedAt, p0.ExecutionResult = "", ""
		p0.ControlDate, p0.ControlOutcome, p0.ControlComment = "—", "Оценка ещё не завершена", ""
		p0.UpdatedAt, p0.UpdatedBy = d.ApprovedAt, d.ApprovedBy
		out = append(out, p0)
	}
	return out
}

func restoreMethodDecisionAsNewVersion(old approvedMethodDecision) error {
	key := strings.TrimSpace(old.ObjectKey)
	if key == "" {
		return fmt.Errorf("у архивной версии отсутствует ключ объекта")
	}
	ensureApprovedMethodsLoaded()
	ensureMethodImplementationRecordsLoaded()
	oldApproved := append([]approvedMethodDecision(nil), approvedMethodDecisions...)
	oldPlans := clonePlanItems(plans)
	oldImplementation := cloneMethodImplementationRecords(methodImplementationRecords)

	latest, hasLatest := latestApprovedMethodDecision(key)
	now := time.Now().Format("02.01.2006 15:04")
	user := methodImplementationAuditUser()
	d := old
	d.ID = strconvSafeNano()
	d.ApprovedAt, d.ApprovedBy = now, user
	d.SchemaVersion = 2
	d.VersionNumber = nextMethodDecisionVersion(key)
	d.RestoredFromID = old.ID
	if hasLatest {
		d.SupersedesID = latest.ID
	} else {
		d.SupersedesID = ""
	}
	d.MethodStatus = "Восстановлено из архивной версии"
	d.Plan = methodPlanItemsForRestoredDecision(d, old)

	approvedMethodDecisions = append(approvedMethodDecisions, d)
	plans = append(plans, d.Plan...)
	applyMethodImplementationSnapshotsToMemory(key, old.Implementation, user, now)

	rollback := func() {
		approvedMethodDecisions = oldApproved
		plans = oldPlans
		methodImplementationRecords = oldImplementation
		_ = saveApprovedMethods()
		_ = saveJSON(sharedPath("plan.json"), plans)
		_ = saveJSON(methodImplementationPath(), methodImplementationRecords)
	}
	if err := saveApprovedMethods(); err != nil {
		rollback()
		return err
	}
	if err := saveJSON(sharedPath("plan.json"), plans); err != nil {
		rollback()
		return err
	}
	if err := saveJSON(methodImplementationPath(), methodImplementationRecords); err != nil {
		rollback()
		return err
	}
	if err := markEconomicsNeedsRecalcForDecision(d, "восстановлена архивная версия технологического решения"); err != nil {
		rollback()
		return fmt.Errorf("версия не восстановлена: не удалось обновить статус экономики: %v", err)
	}

	methods := normalizeMethodList(d.Methods)
	if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
		methods = []string{d.Method}
	}
	setCombinedMethods(key, methods)
	if len(methods) > 0 {
		technicalMethodSelection[key] = methods[0]
	}
	return nil
}

func layoutMethodArchiveDialog(hwnd uintptr) {
	var rc RECT
	if r, _, _ := pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	margin := int32(24)
	headerH := int32(78)
	setPos(methodArchiveHeaderBand, 0, 0, w, headerH)
	setPos(methodArchiveAccent, 0, headerH-4, w, 4)
	setPos(methodArchiveTitleHandle, margin, 12, max32(320, w-margin*2), 30)
	setPos(methodArchiveSubtitle, margin, 44, max32(320, w-margin*2), 24)
	setPos(methodArchiveSection, margin, 94, max32(320, w-margin*2), 28)
	buttonY := max32(430, h-margin-42)
	setPos(methodArchiveBody, margin, 126, max32(320, w-margin*2), max32(250, buttonY-140))
	if objectHistoryTitle != "" {
		setPos(methodArchiveBody, margin, 126, max32(320, w-margin*2), max32(250, h-150))
	}
	restoreW := min32(360, max32(300, (w-margin*2)*40/100))
	setPos(methodArchiveRestoreButton, w-margin-restoreW, buttonY, restoreW, 42)
}

func methodArchiveDialogWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		if hIconBig != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, uintptr(hIconBig))
		}
		if hIconSmall != 0 {
			pSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, uintptr(hIconSmall))
		}
		methodArchiveHeaderBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1080, 78, parent, 0)
		methodArchiveAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 74, 1080, 4, parent, 0)
		title := "АРХИВ РЕШЕНИЙ · ПРОСМОТР ВЕРСИИ"
		if objectHistoryTitle != "" {
			title = objectHistoryTitle
		}
		methodArchiveTitleHandle = modalCreate("STATIC", title, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 12, 1020, 30, parent, 0)
		subtitle := "Архивная запись открыта только для просмотра · восстановление всегда создаёт новую версию"
		if objectHistoryTitle != "" {
			subtitle = "Все сохранённые варианты по выбранному объекту · просмотр без изменения решения"
		}
		methodArchiveSubtitle = modalCreate("STATIC", subtitle, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 44, 1020, 24, parent, 0)
		if hFontSystemName != 0 {
			pSendMessageW.Call(uintptr(methodArchiveTitleHandle), WM_SETFONT, uintptr(hFontSystemName), 1)
		}
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodArchiveSubtitle), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		section := "СОСТАВ И ПАРАМЕТРЫ СОХРАНЁННОГО РЕШЕНИЯ"
		if objectHistoryTitle != "" {
			section = "ИСТОРИЯ ВАРИАНТОВ ПО ОБЪЕКТУ"
		}
		methodArchiveSection = modalCreate("STATIC", section, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 94, 1020, 28, parent, 0)
		pSendMessageW.Call(uintptr(methodArchiveSection), WM_SETFONT, uintptr(methodsFont(methodsTypeSection, true)), 1)
		_ = riched20.Load()
		archiveText := methodArchivePreviewText(methodArchiveDialogDecision)
		if objectHistoryTitle != "" {
			archiveText = objectHistoryBody
		}
		methodArchiveBody = modalCreate("RichEdit20W", archiveText, WS_CHILD|WS_VISIBLE|WS_BORDER|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 24, 126, 1020, 490, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodArchiveBody), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		pSendMessageW.Call(uintptr(methodArchiveBody), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		margins := uintptr(uint32(methodsLeftMargin) | uint32(methodsLeftMargin)<<16)
		pSendMessageW.Call(uintptr(methodArchiveBody), emSetMargins, 3, margins)
		formatMethodsReferenceText(methodArchiveBody, archiveText)
		methodArchiveRestoreButton = modalCreate("BUTTON", "ВОССТАНОВИТЬ КАК НОВУЮ ВЕРСИЮ", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 692, 634, 352, 42, parent, methodArchiveRestore)
		if objectHistoryTitle != "" {
			show(methodArchiveRestoreButton, false)
		}
		if hFontButton != 0 {
			pSendMessageW.Call(uintptr(methodArchiveRestoreButton), WM_SETFONT, uintptr(hFontButton), 1)
		}
		layoutMethodArchiveDialog(hwnd)
		return 0
	case WM_SIZE:
		layoutMethodArchiveDialog(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 900
			mmi.PtMinTrackSize.Y = 650
		}
		return 0
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case methodArchiveHeaderBand:
			return uintptr(brushNav)
		case methodArchiveAccent:
			return uintptr(brushHeader)
		case methodArchiveTitleHandle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodArchiveSubtitle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		case methodArchiveSection:
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(232, 242, 253))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(methodsSectionBrush())
		}
	case WM_CTLCOLOREDIT:
		if syscall.Handle(lParam) == methodArchiveBody {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(43, 46, 50))
			return uintptr(brushWhite)
		}
	case WM_COMMAND:
		switch int(loWord(wParam)) {
		case methodArchiveClose:
			methodArchiveDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		case methodArchiveRestore:
			if objectHistoryTitle != "" {
				return 0
			}
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(methodArchiveDialogDecision.ID)), "teaching-") {
				msg("Учебная история", "Синтетическая учебная версия не восстанавливается как производственное решение.", MB_OK|MB_ICONINFORMATION)
				return 0
			}
			text := "Восстановить выбранную архивную версию?\n\nТекущая запись НЕ будет перезаписана. РОСА создаст новую версию решения, восстановит выбранные методы и сохранённые параметры реализации."
			r, _, _ := pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(p(text))), uintptr(unsafe.Pointer(p("Восстановление версии"))), 0x00000004|MB_ICONWARNING)
			if r != 6 {
				return 0
			}
			if err := restoreMethodDecisionAsNewVersion(methodArchiveDialogDecision); err != nil {
				msg("Восстановление версии", err.Error(), MB_OK|MB_ICONWARNING)
				return 0
			}
			methodArchiveDialogDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		methodArchiveDialogDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openMethodsArchiveVersionAtRow(row int) {
	d, ok := methodArchiveDecisionAtRow(row)
	if !ok {
		return
	}
	methodArchiveDialogDecision = d
	objectHistoryTitle, objectHistoryBody = "", ""
	methodArchiveDialogDone = false
	cls := p("ROSAMethodArchivePreviewV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodArchiveDialogWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — Архив решений"))), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1080, 720, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg("Архив решений", "Не удалось открыть архивную версию.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !methodArchiveDialogDone {
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
	refreshMethodsWorkflowUI()
	refreshMRPForecastPage()
	refreshEconomicsPage()
	refreshResultPage()
	refreshPlan()
}

// openObjectHistoryDialog shows a read-only object history. It deliberately
// reuses the archive frame but never exposes the restore action or catalogue.
func openObjectHistoryDialog(title, body string) {
	objectHistoryTitle, objectHistoryBody = title, body
	methodArchiveDialogDone = false
	cls := p("ROSAMethodArchivePreviewV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodArchiveDialogWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p("РОСА — История по объекту"))), WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000, 0x80000000, 1080, 720, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		objectHistoryTitle, objectHistoryBody = "", ""
		msg("История по объекту", "Не удалось открыть историю.", MB_OK|MB_ICONWARNING)
		return
	}
	var message MSG
	for !methodArchiveDialogDone {
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
	objectHistoryTitle, objectHistoryBody = "", ""
}

func methodPlanItemsForApprovedDecision(d approvedMethodDecision, basis string) []PlanItem {
	methods := normalizeMethodList(d.Methods)
	if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
		methods = []string{strings.TrimSpace(d.Method)}
	}
	out := make([]PlanItem, 0, len(methods))
	for i, method := range methods {
		prev := methodPlanDisplayItem(d.ObjectKey, method)
		p0 := PlanItem{
			ID: fmt.Sprintf("method-%s-%d", d.ID, i+1), ObjectKey: d.ObjectKey, MethodName: method,
			Action: "Метод / мероприятие: " + method, Basis: basis, Status: "Утверждён", DecisionID: d.ID,
			ParametersRef: d.ID + "::" + method, DueDate: methodPlanDateDisplay(prev.DueDate), Responsible: methodPlanResponsibleDisplay(prev.Responsible),
			ControlDate: methodPlanDateDisplay(prev.ControlDate), ControlCriterion: methodPlanControlCriterionOrDefault(prev),
			UpdatedAt: d.ApprovedAt, UpdatedBy: d.ApprovedBy,
		}
		if p0.Responsible == "" {
			p0.Responsible = "—"
		}
		out = append(out, p0)
	}
	return out
}
