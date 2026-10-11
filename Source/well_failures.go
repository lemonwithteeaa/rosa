//go:build windows

package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// FailureEvent preserves the fields of the source document. No class or cause
// is inferred from free text while the user records a factual failure.
type FailureEvent struct {
	ID                      string            `json:"id"`
	ObjectKey               string            `json:"object_key"`
	Department              string            `json:"department,omitempty"`
	ProductionUnit          string            `json:"production_unit,omitempty"`
	Field                   string            `json:"field,omitempty"`
	Well                    string            `json:"well,omitempty"`
	ObjectPlast             string            `json:"object_plast,omitempty"`
	CycleStartDate          string            `json:"cycle_start_date,omitempty"`
	Date                    string            `json:"date"`
	RepairEndDate           string            `json:"repair_end_date,omitempty"`
	EventType               string            `json:"event_type"`
	FailedUnit              string            `json:"failed_unit,omitempty"`
	OperationMode           string            `json:"operation_mode,omitempty"`
	Equipment               string            `json:"equipment,omitempty"`
	EquipmentBrand          string            `json:"equipment_brand,omitempty"`
	EquipmentSize           string            `json:"equipment_size,omitempty"`
	EquipmentSize2          string            `json:"equipment_size_2,omitempty"`
	EquipmentSize3          string            `json:"equipment_size_3,omitempty"`
	EquipmentSize4          string            `json:"equipment_size_4,omitempty"`
	StopCause               string            `json:"stop_cause,omitempty"`
	PreliminaryCause        string            `json:"preliminary_cause,omitempty"`
	PrimaryComplication     string            `json:"primary_complication,omitempty"`
	AssociatedComplication1 string            `json:"associated_complication_1,omitempty"`
	AssociatedComplication2 string            `json:"associated_complication_2,omitempty"`
	Source                  string            `json:"source"`
	SourceClass             string            `json:"source_class,omitempty"`
	UserClass               string            `json:"user_class,omitempty"`
	NormalizedClass         string            `json:"normalized_class,omitempty"`
	ComplicationFactor      string            `json:"complication_factor,omitempty"`
	SourceCause             string            `json:"source_cause,omitempty"`
	SourceCauseCode         string            `json:"source_cause_code,omitempty"`
	NormalizedCause         string            `json:"normalized_cause,omitempty"`
	Inspection              string            `json:"inspection,omitempty"`
	InvestigationStatus     string            `json:"investigation_status,omitempty"`
	CommissionConclusion    string            `json:"commission_conclusion,omitempty"`
	TKRS1                   string            `json:"tkrs_1,omitempty"`
	TKRS2                   string            `json:"tkrs_2,omitempty"`
	MethodBeforeFailure     string            `json:"method_before_failure,omitempty"`
	MethodBeforeFailure2    string            `json:"method_before_failure_2,omitempty"`
	MethodAfterFailure      string            `json:"method_after_failure,omitempty"`
	MethodAfterFailure2     string            `json:"method_after_failure_2,omitempty"`
	DamageType              string            `json:"damage_type,omitempty"`
	FinalCause              string            `json:"final_cause,omitempty"`
	RefinedComplication     string            `json:"refined_complication,omitempty"`
	Confirmation            string            `json:"confirmation,omitempty"`
	Confidence              string            `json:"confidence,omitempty"`
	CommissionSource        string            `json:"commission_source,omitempty"`
	FountainPumpType        string            `json:"fountain_pump_type,omitempty"`
	FountainPumpDepth       string            `json:"fountain_pump_depth,omitempty"`
	PreliminaryConclusion   string            `json:"preliminary_conclusion,omitempty"`
	ApprovedConclusion      string            `json:"approved_conclusion,omitempty"`
	ConclusionApproved      bool              `json:"conclusion_approved,omitempty"`
	Draft                   bool              `json:"draft,omitempty"`
	Comment                 string            `json:"comment,omitempty"`
	Manual                  bool              `json:"manual"`
	CorrectedByUser         bool              `json:"corrected_by_user,omitempty"`
	UpdatedAt               string            `json:"updated_at"`
	SourceType              string            `json:"source_type,omitempty"`
	SourceFile              string            `json:"source_file,omitempty"`
	SourceSheet             string            `json:"source_sheet,omitempty"`
	SourceRow               int               `json:"source_row,omitempty"`
	SourceImportID          string            `json:"source_import_id,omitempty"`
	ImportedAt              string            `json:"imported_at,omitempty"`
	ImportedBy              string            `json:"imported_by,omitempty"`
	OperationalComplete     bool              `json:"operational_complete,omitempty"`
	HasFullData             bool              `json:"has_full_data,omitempty"`
	FullCompletionPct       int               `json:"full_completion_pct,omitempty"`
	NoDataCount             int               `json:"no_data_count,omitempty"`
	OperationalOnly         bool              `json:"operational_only,omitempty"`
	Extra                   map[string]string `json:"extra,omitempty"`
}

type failureChange struct {
	When   string       `json:"when"`
	Action string       `json:"action"`
	Event  FailureEvent `json:"event"`
}

var failureEvents []FailureEvent
var failureLoadError error

func failuresLogPath() string { return sharedPath("well_failure_changes.jsonl") }

func loadFailureEvents() {
	failureEvents = nil
	failureLoadError = nil
	f, err := os.Open(failuresLogPath())
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		failureLoadError = err
		return
	}
	defer f.Close()
	current := make(map[string]FailureEvent)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var change failureChange
		if err := json.Unmarshal(scanner.Bytes(), &change); err != nil {
			failureLoadError = fmt.Errorf("повреждена строка журнала отказов: %w", err)
			break
		}
		if change.Event.ID == "" || (change.Action != "save" && change.Action != "delete") {
			failureLoadError = errors.New("неизвестное действие в журнале отказов")
			break
		}
		if change.Action == "delete" {
			delete(current, change.Event.ID)
		} else {
			current[change.Event.ID] = change.Event
		}
	}
	if scanner.Err() != nil {
		failureLoadError = scanner.Err()
	}
	for _, event := range current {
		normalizeFailureEquipmentSchema(&event)
		refreshFailureDerived(&event)
		failureEvents = append(failureEvents, event)
	}
	sort.SliceStable(failureEvents, func(i, j int) bool { return failureEvents[i].Date > failureEvents[j].Date })
}

func failureDate(value string) (time.Time, error) {
	for _, layout := range []string{"02.01.2006", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("укажите дату отказа в формате ДД.ММ.ГГГГ")
}

func inFailureWindow(value string, now time.Time) bool {
	date, err := failureDate(value)
	if err != nil {
		return false
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return !date.Before(day.AddDate(-3, 0, 0)) && !date.After(day)
}

func validateFailureCycleDates(event FailureEvent) error {
	failure, err := failureDate(event.Date)
	if err != nil {
		return err
	}
	if strings.TrimSpace(event.CycleStartDate) != "" {
		cycleStart, err := failureDate(event.CycleStartDate)
		if err != nil {
			return fmt.Errorf("дата запуска цикла: %w", err)
		}
		if cycleStart.After(failure) {
			return errors.New("дата запуска цикла не может быть позже даты отказа")
		}
	}
	if strings.TrimSpace(event.RepairEndDate) != "" {
		repairEnd, err := failureDate(event.RepairEndDate)
		if err != nil {
			return fmt.Errorf("дата окончания ремонта: %w", err)
		}
		if repairEnd.Before(failure) {
			return errors.New("дата окончания ремонта не может быть раньше даты отказа")
		}
	}
	return nil
}

func manualFailureCount(key string, now time.Time) int {
	count := 0
	for _, e := range failureEvents {
		if e.Manual && !e.Draft && e.ObjectKey == key && inFailureWindow(e.Date, now) {
			count++
		}
	}
	return count
}

func failuresForObject(key string, now time.Time) []FailureEvent {
	var rows []FailureEvent
	for _, e := range failureEvents {
		if !e.Draft && failureBelongsToObject(e, key) && inFailureWindow(e.Date, now) {
			rows = append(rows, e)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Date > rows[j].Date })
	return rows
}

func failureEntriesForObject(key string, now time.Time) []FailureEvent {
	rows := failuresForObject(key, now)
	for _, event := range failureEvents {
		if event.Draft && event.ObjectKey == key {
			rows = append(rows, event)
		}
	}
	return rows
}

func duplicateFailure(event FailureEvent) (FailureEvent, bool) {
	date, err := failureDate(event.Date)
	if err != nil {
		return FailureEvent{}, false
	}
	for _, old := range failureEvents {
		oldDate, oldErr := failureDate(old.Date)
		if oldErr == nil && old.ObjectKey == event.ObjectKey && old.ID != event.ID && oldDate.Equal(date) && strings.EqualFold(old.EventType, event.EventType) {
			return old, true
		}
	}
	return FailureEvent{}, false
}

func recordFailureChange(action string, event FailureEvent) error {
	if failureLoadError != nil {
		return fmt.Errorf("запись запрещена: журнал требует восстановления: %w", failureLoadError)
	}
	if currentSession != nil && currentSession.IsGuest {
		return errors.New("гостевой режим: запись отказов недоступна")
	}
	path := failuresLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	line, err := json.Marshal(failureChange{When: time.Now().Format(time.RFC3339), Action: action, Event: event})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(line, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func saveFailureEvent(event FailureEvent, now time.Time) error {
	normalizeFailureEquipmentSchema(&event)
	if strings.TrimSpace(event.ObjectKey) == "" {
		return errors.New("сначала закрепите объект на Главной")
	}
	if !failureHasValue(event.Equipment) {
		event.Equipment = strings.TrimSpace(event.OperationMode)
	}
	if !failureHasValue(event.PreliminaryCause) {
		event.PreliminaryCause = firstNonEmpty(event.SourceCause, event.PreliminaryConclusion)
	}
	if !failureHasValue(event.PrimaryComplication) {
		event.PrimaryComplication = firstNonEmpty(event.ComplicationFactor, event.SourceClass, event.UserClass)
	}
	if !failureHasValue(event.PreliminaryConclusion) && failureHasValue(event.PreliminaryCause) {
		event.PreliminaryConclusion = event.PreliminaryCause
	}
	if event.Manual && strings.TrimSpace(event.Source) == "" {
		event.Source = "Ручной ввод"
	}
	if !event.Draft {
		if !failureHasValue(event.Field) || !failureHasValue(event.Well) {
			return errors.New("укажите месторождение и номер скважины")
		}
		if !failureHasValue(event.CycleStartDate) {
			return errors.New("укажите дату запуска цикла")
		}
		if !failureHasValue(event.EventType) {
			return errors.New("укажите фактический тип события")
		}
		if !failureHasValue(event.OperationMode) || !failureHasValue(event.EquipmentBrand) || !failureHasValue(event.EquipmentSize) {
			return errors.New("укажите способ эксплуатации, марку и основной типоразмер ГНО")
		}
		if !failureHasValue(event.FailedUnit) || !failureHasValue(event.StopCause) || !failureHasValue(event.PrimaryComplication) {
			return errors.New("укажите отказавший узел, признак отказа и основное осложнение")
		}
		if !failureHasValue(event.PreliminaryCause) {
			return errors.New("укажите предварительную причину")
		}
		if !inFailureWindow(event.Date, now) {
			return errors.New("дата отказа должна быть в пределах последних 36 месяцев и не в будущем")
		}
		if err := validateFailureCycleDates(event); err != nil {
			return err
		}
	}

	updating := false
	for _, old := range failureEvents {
		if old.ID == event.ID {
			if old.ObjectKey != event.ObjectKey {
				return errors.New("запись принадлежит другому объекту")
			}
			updating = true
			event.Manual = old.Manual
			event.CorrectedByUser = old.CorrectedByUser || !old.Manual
			break
		}
	}
	if !updating {
		event.Manual = true
	}
	if event.ID == "" {
		var bytes [16]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return err
		}
		event.ID = hex.EncodeToString(bytes[:])
	}
	event.UpdatedAt = now.Format(time.RFC3339)
	if event.Manual && strings.TrimSpace(event.SourceType) == "" {
		event.SourceType = "manual"
	}
	refreshFailureDerived(&event)
	if err := recordFailureChange("save", event); err != nil {
		return err
	}
	for i := range failureEvents {
		if failureEvents[i].ID == event.ID {
			failureEvents[i] = event
			invalidateMethodsBackground()
			return nil
		}
	}
	failureEvents = append(failureEvents, event)
	invalidateMethodsBackground()
	return nil
}

func deleteFailureEvent(id, key string) error {
	for i, e := range failureEvents {
		if e.ID != id || e.ObjectKey != key {
			continue
		}
		if err := recordFailureChange("delete", e); err != nil {
			return err
		}
		failureEvents = append(failureEvents[:i], failureEvents[i+1:]...)
		invalidateMethodsBackground()
		return nil
	}
	return errors.New("запись этого объекта не найдена")
}
