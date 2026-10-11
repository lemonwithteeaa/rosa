//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var recentObjectKeys []string
var candidateFromRecent bool

type dashboardHistoryRow struct {
	Number         int
	ObjectKey      string
	PassID         string
	Date           string
	Code           string
	LabConclusion  string
	Classes        string
	Methods        string
	Economics      string
	FullConclusion string
	CompletionPct  int
	FullyApproved  bool
}

var dashboardHistoryRows []dashboardHistoryRow

func activateReleaseTeachingExample() {
	if teachingExampleReleaseSeen == appBuildVersion {
		return
	}
	well, ok := trainingWellByOrdinal(1)
	if !ok {
		return
	}
	// Start on the current corpus identity, not an older "Учебный пример 001"
	// key left in user preferences by the retired 100-workbook demo set.
	selectedObjectKey, candidateObjectKey = well.ObjectKey, well.ObjectKey
	keptRecent := make([]string, 0, len(recentObjectKeys))
	for _, key := range recentObjectKeys {
		if !isRetiredLegacyTeachingObjectKey(key) {
			keptRecent = append(keptRecent, key)
		}
	}
	recentObjectKeys = keptRecent
	rememberSelectedObject(well.ObjectKey)
	teachingExampleReleaseSeen = appBuildVersion
	saveUserPreferences()
}

func rememberSelectedObject(key string) {
	keys := []string{key}
	for _, old := range recentObjectKeys {
		if old != key && strings.TrimSpace(old) != "" {
			keys = append(keys, old)
		}
		if len(keys) == 25 {
			break
		}
	}
	recentObjectKeys = keys
}

func objectViewKey(v ObjectView) string {
	if strings.TrimSpace(v.State.ObjectKey) != "" {
		return strings.TrimSpace(v.State.ObjectKey)
	}
	return strings.TrimSpace(v.Eval.ObjectKey)
}

func findObjectViewByKey(key string) (ObjectView, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return ObjectView{}, false
	}
	for _, v := range allObjectViews() {
		if objectViewKey(v) == key || laboratoryObjectKeysEquivalent(objectViewKey(v), key) {
			return v, true
		}
	}
	return ObjectView{}, false
}

// readableObjectKeyIndex builds the human captions once for callers that need
// to render or sort many rows. Calling readableObjectKey for every row rebuilds
// allObjectViews each time and becomes noticeably quadratic as saved laboratory
// examples accumulate.
func readableObjectKeyIndex() map[string]string {
	index := make(map[string]string)
	for _, view := range allObjectViews() {
		key := objectViewKey(view)
		fingerprint := laboratoryObjectKeyFingerprint(key)
		if fingerprint != "" {
			index[fingerprint] = objectViewLabel(view)
		}
	}
	for _, event := range failureEvents {
		key := strings.TrimSpace(event.ObjectKey)
		fingerprint := laboratoryObjectKeyFingerprint(key)
		if fingerprint != "" && index[fingerprint] == "" && strings.TrimSpace(event.Field+event.Well+event.ObjectPlast) != "" {
			index[fingerprint] = wellFailureObjectCaption(key)
		}
	}
	return index
}

func readableObjectKeyFromIndex(key string, index map[string]string) string {
	key = strings.TrimSpace(key)
	if caption := strings.TrimSpace(index[laboratoryObjectKeyFingerprint(key)]); caption != "" {
		return caption
	}
	if strings.HasPrefix(strings.ToLower(key), "failure-well-") {
		return "Объект из архива отказов · " + key[len("failure-well-"):min(len(key), len("failure-well-")+8)]
	}
	return key
}

// The persistent key remains unchanged; this caption is only for people.
func readableObjectKey(key string) string {
	return readableObjectKeyFromIndex(key, readableObjectKeyIndex())
}

// applyGlobalObjectSelection is the single state transition for approving a
// working object. keepOpenLaboratory is used by a saved/teaching laboratory
// record: its visible measurements remain open, while every other object-bound
// page is invalidated and must reload for the new global key.
func applyGlobalObjectSelection(key string, keepOpenLaboratory bool) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if wellFormActive && !laboratoryObjectKeysEquivalent(wellDraftKey, key) && !confirmFailureFormDiscard() {
		return false
	}
	if !laboratoryObjectKeysEquivalent(wellDraftKey, key) {
		wellFormActive = false
		wellEditingID = ""
	}
	changed := strings.TrimSpace(selectedObjectKey) == "" || !laboratoryObjectKeysEquivalent(selectedObjectKey, key)
	if changed && !keepOpenLaboratory {
		labPinnedDraftID = ""
	}
	selectedObjectKey = key
	candidateObjectKey = key
	candidateFromRecent = keepOpenLaboratory
	rememberSelectedObject(key)
	// These pages cache values by object; force them to reload instead of
	// retaining controls from the previously approved object.
	techProfileBoundKey = ""
	econBoundObjectKey = ""
	if keepOpenLaboratory {
		labPinnedDraftID = labActivePassID
	} else {
		restoreSelectedObjectWorkspace(key)
	}
	saveUserPreferences()
	// Native Win32 ListView controls can emit notifications while rows are rebuilt.
	// Never synchronously refresh hidden pages from an object-selection handler:
	// RC539 did this for both Dashboard and Well Card and could leave the UI in a
	// re-entrant notification storm (the window rendered, then stopped responding).
	// Refresh only the page which is actually visible; other object-bound pages
	// are already invalidated above and will reload when the user opens them.
	switch currentPage {
	case ID_DASH:
		refreshDashboard()
	case ID_OBJECTS:
		refreshWellCard()
	}
	return laboratoryObjectKeysEquivalent(selectedObjectKey, key)
}

func confirmSelectedObject() {
	if currentPage != ID_DASH {
		return
	}
	key := strings.TrimSpace(candidateObjectKey)
	if !candidateFromRecent && objectsList != 0 {
		rows := filteredObjects()
		i := selectedRow(objectsList)
		if i >= 0 && i < len(rows) {
			key = objectViewKey(rows[i])
		}
	}
	if key == "" {
		msg("Выбор объекта", "Выделите строку объекта, затем нажмите «Выбрать объект».", MB_OK|MB_ICONWARNING)
		return
	}
	v, ok := findObjectViewByKey(key)
	if !ok {
		msg("Выбор объекта", "Выбранный объект больше не найден в текущем Техрежиме или истории.", MB_OK|MB_ICONWARNING)
		return
	}
	if !applyGlobalObjectSelection(key, false) {
		return
	}
	candidateFromRecent = false
	msg("Выбор объекта", fmt.Sprintf("Текущий объект закреплён:\n%s", objectViewLabel(v)), MB_OK|MB_ICONINFORMATION)
}

func confirmSelectedObjectAfterQuestion() {
	key := strings.TrimSpace(candidateObjectKey)
	if key == "" {
		return
	}
	v, ok := findObjectViewByKey(key)
	if !ok {
		msg("Утверждение объекта", "Выбранный объект больше не найден в текущем Техрежиме или истории.", MB_OK|MB_ICONWARNING)
		return
	}
	text := fmt.Sprintf("Утвердить объект?\n\n%s\n\nПосле утверждения РОСА восстановит ранее сохранённые лабораторные данные, классы, методы и расчёты этого объекта.", objectViewLabel(v))
	if askYesNo("Утвердить объект?", text) {
		confirmSelectedObject()
	}
}

func latestSavedLaboratoryPassForObject(objectKey string) (LabPassRecord, bool) {
	passes := repositoryLabPassesForObject(objectKey)
	var best LabPassRecord
	found := false
	bestTime := time.Time{}
	for _, pass := range passes {
		if pass.Status != labPassSavedStatus {
			continue
		}
		t := parseDate(pass.Date)
		if parsed, err := time.Parse(time.RFC3339, pass.UpdatedAt); err == nil && (t.IsZero() || parsed.After(t)) {
			t = parsed
		}
		if !found || t.After(bestTime) {
			best, bestTime, found = pass, t, true
		}
	}
	return best, found
}

func restoreSelectedObjectWorkspace(objectKey string) {
	// Force object-bound pages to reload their saved values next time they are
	// opened instead of retaining controls from the previously approved object.
	techProfileBoundKey = ""
	econBoundObjectKey = ""
	if pass, ok := latestSavedLaboratoryPassForObject(objectKey); ok {
		openLaboratoryPass(pass, "overall")
		labPinnedDraftID = labActivePassID
		return
	}
	labObjectSwitching = true
	setText(labObjectEdit, objectKey)
	labObjectSwitching = false
	handleLaboratoryObjectChange()
}

func continueSelectedObject() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		msg("Продолжить работу", "Для этого пользователя ещё не закреплён объект. Выберите его на Главной.", MB_OK|MB_ICONWARNING)
		return
	}
	if v, ok := findObjectViewByKey(selectedObjectKey); ok {
		candidateObjectKey = selectedObjectKey
		setText(dashboardNote, dashboardObjectSummary())
		msg("Закреплённый объект", objectViewLabel(v), MB_OK|MB_ICONINFORMATION)
		return
	}
	msg("Продолжить работу", "Сохранённый объект отсутствует в текущем Техрежиме. Выберите другой на Главной; история остаётся в архиве.", MB_OK|MB_ICONWARNING)
}

func dashboardObjectSummary() string {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return "Объект ещё не выбран. Нажмите «Выбрать объект» и подтвердите конкретную строку из текущего Техрежима."
	}
	if scenario, full := teachingFullScenarioForObject(selectedObjectKey); full {
		return fmt.Sprintf("АКТИВНЫЙ ЗАПОЛНЕННЫЙ УЧЕБНЫЙ ПРИМЕР %03d · %s\r\nОбъект закреплён для вкладок 1–5; два других сценария стоят первыми в истории ниже.\r\n%s", scenario.Ordinal, scenario.Name, readableObjectKey(selectedObjectKey))
	}
	v, ok := findObjectViewByKey(selectedObjectKey)
	if !ok {
		return fmt.Sprintf("Сохранённый объект: %s\r\nОбъект отсутствует в текущей выгрузке Техрежима. Исторические расчёты не удалены.", readableObjectKey(selectedObjectKey))
	}
	eval := "оценка ещё не выполнена"
	if v.HasEval {
		eval = fmt.Sprintf("последняя оценка %s; осложнение %s; метод %s", dash(v.Eval.Created), dash(v.Eval.Complication), dash(v.Eval.Method))
	}
	failureCount := len(failuresForObject(selectedObjectKey, time.Now()))
	return fmt.Sprintf("%s\r\nОбъект закреплён на Главной\r\nОборудование: %s; текущий МРП: %s сут.; отказов за 36 мес.: %d; %s", objectViewLabel(v), dash(v.State.Equipment), dash(v.State.CurrentMRP), failureCount, eval)
}

func dashboardHistoryStatus(ok bool) string {
	if ok {
		return "Готово"
	}
	return "Не готово"
}

func dashboardHistoryDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "—" || value == "-" {
		return "—"
	}
	return parseDate(value).Format("02.01.2006")
}

func dashboardLatestVersionForObject(objectKey string) (VersionRecord, bool) {
	key := strings.TrimSpace(objectKey)
	var latest VersionRecord
	found := false
	for _, version := range versions {
		if !strings.EqualFold(strings.TrimSpace(version.ObjectKey), key) {
			continue
		}
		if !found || parseDate(version.Created).After(parseDate(latest.Created)) {
			latest = version
			found = true
		}
	}
	return latest, found
}

func dashboardClassesApproved(objectKey string) bool {
	ensureAssessmentClassStore()
	if strings.TrimSpace(objectKey) == "" || len(assessmentClassDirections) == 0 {
		return false
	}
	for _, direction := range assessmentClassDirections {
		record := assessmentRecord(objectKey, direction)
		if strings.TrimSpace(record.FinalClass) == "" || strings.TrimSpace(record.ConfirmedAt) == "" {
			return false
		}
	}
	return true
}

func dashboardMethodsApproved(objectKey string, version VersionRecord, hasVersion bool) bool {
	if hasVersion && strings.TrimSpace(version.Method) != "" {
		return true
	}
	for _, direction := range assessmentClassDirections {
		if strings.TrimSpace(assessmentMethodSelection[assessmentMethodSelectionKey(objectKey, direction.Name)]) != "" {
			return true
		}
	}
	return false
}

func dashboardEconomicsCalculated(version VersionRecord, hasVersion bool) bool {
	if !hasVersion {
		return false
	}
	return strings.TrimSpace(version.EconStatus) != "" || strings.TrimSpace(version.EconResult) != ""
}

func dashboardFullConclusionApproved(version VersionRecord, hasVersion bool) bool {
	if !hasVersion {
		return false
	}
	if version.Archived {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(version.Decision)) {
	case "переносить", "не переносить", "утверждено", "утверждена", "утвержден", "approved":
		return true
	default:
		return false
	}
}

func dashboardLabConclusionApproved(pass LabPassRecord) bool {
	conclusion := strings.ToLower(strings.TrimSpace(pass.Conclusion))
	return pass.Status == labPassSavedStatus && conclusion != "" && !strings.Contains(conclusion, "не сформировано")
}

func dashboardHistoryRowsForMain() []dashboardHistoryRow {
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	passes := make([]LabPassRecord, 0, len(labPasses))
	seenPass := map[string]bool{}
	for _, pass := range labPasses {
		if pass.Status != labPassSavedStatus {
			continue
		}
		if scenario, full := teachingFullScenarioForObject(pass.ObjectKey); full && scenario.Ordinal == 1 && laboratoryObjectKeysEquivalent(selectedObjectKey, pass.ObjectKey) {
			continue
		}
		if strings.TrimSpace(pass.PassID) == "" || seenPass[pass.PassID] {
			continue
		}
		seenPass[pass.PassID] = true
		passes = append(passes, pass)
	}
	sort.SliceStable(passes, func(i, j int) bool {
		leftScenario, leftFull := teachingFullScenarioForObject(passes[i].ObjectKey)
		rightScenario, rightFull := teachingFullScenarioForObject(passes[j].ObjectKey)
		if leftFull != rightFull {
			return leftFull
		}
		if leftFull && leftScenario.Ordinal != rightScenario.Ordinal {
			if leftScenario.Ordinal == 1 {
				return false
			}
			if rightScenario.Ordinal == 1 {
				return true
			}
			return leftScenario.Ordinal < rightScenario.Ordinal
		}
		left, right := parseDate(passes[i].Date), parseDate(passes[j].Date)
		if !left.Equal(right) {
			return left.After(right)
		}
		return strings.TrimSpace(passes[i].PassID) > strings.TrimSpace(passes[j].PassID)
	})
	if len(passes) > 100 {
		passes = passes[:100]
	}
	rows := make([]dashboardHistoryRow, 0, len(passes))
	for i, pass := range passes {
		version, hasVersion := dashboardLatestVersionForObject(pass.ObjectKey)
		statuses := []bool{
			dashboardLabConclusionApproved(pass),
			dashboardClassesApproved(pass.ObjectKey),
			dashboardMethodsApproved(pass.ObjectKey, version, hasVersion),
			dashboardEconomicsCalculated(version, hasVersion),
			dashboardFullConclusionApproved(version, hasVersion),
		}
		if _, full := teachingFullScenarioForObject(pass.ObjectKey); full {
			statuses = []bool{true, true, true, true, true}
		}
		done := 0
		for _, status := range statuses {
			if status {
				done++
			}
		}
		code := strings.TrimSpace(pass.Source)
		if scenario, full := teachingFullScenarioForObject(pass.ObjectKey); full {
			code = fmt.Sprintf("УЧЕБНАЯ ИСТОРИЯ %03d · %s", scenario.Ordinal, scenario.Name)
		}
		if code == "" {
			code = pass.PassID
		}
		rows = append(rows, dashboardHistoryRow{
			Number:         i + 1,
			ObjectKey:      pass.ObjectKey,
			PassID:         pass.PassID,
			Date:           dashboardHistoryDate(pass.Date),
			Code:           code,
			LabConclusion:  dashboardHistoryStatus(statuses[0]),
			Classes:        dashboardHistoryStatus(statuses[1]),
			Methods:        dashboardHistoryStatus(statuses[2]),
			Economics:      dashboardHistoryStatus(statuses[3]),
			FullConclusion: dashboardHistoryStatus(statuses[4]),
			CompletionPct:  done * 100 / len(statuses),
			FullyApproved:  done == len(statuses),
		})
	}
	return rows
}

func openDashboardHistoryPass(index int) {
	if currentPage != ID_DASH || index < 0 || index >= len(dashboardHistoryRows) {
		return
	}
	row := dashboardHistoryRows[index]
	passID := strings.TrimSpace(row.PassID)
	if passID == "" {
		msg("Журнал работ", "Для выбранной строки нет сохранённой лабораторной записи для загрузки.", MB_OK|MB_ICONWARNING)
		return
	}
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	var source LabPassRecord
	found := false
	for _, pass := range labPasses {
		if pass.PassID == passID && pass.Status == labPassSavedStatus {
			source = pass
			found = true
			break
		}
	}
	if !found {
		msg("Журнал работ", "Сохранённая лабораторная запись больше не найдена. Обновите Главную и повторите выбор.", MB_OK|MB_ICONWARNING)
		return
	}
	if _, full := teachingFullScenarioForObject(source.ObjectKey); full {
		if !applyGlobalObjectSelection(source.ObjectKey, true) {
			return
		}
		activatePage(ID_LAB, false)
		openLaboratoryPass(source, "overall")
		return
	}
	if !confirmLaboratoryHistoryRestore(source.PassID, source.ObjectKey, source.Date, "все лабораторные данные записи") {
		return
	}
	activatePage(ID_LAB, false)
	openLaboratoryPass(source, "overall")
	setText(labInfo, "Открыта рабочая копия "+source.PassID+". Предыдущий черновик сохранён; кнопка «Вернуться к закреплённому объекту» восстановит его.")
}
