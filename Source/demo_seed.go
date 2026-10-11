//go:build windows

package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed training_corpus.json
var trainingCorpusFile []byte

// Per-file receipts survive deletions. A legacy success flag cannot suppress
// this repair, and a deleted record is never recreated on the next launch.
type demoReceipt struct {
	PassID string `json:"pass_id"`
}

var bundledDemoWatercutPattern = regexp.MustCompile(`(?i)_(\d{2,3})_процентов`)
var bundledDemoOrdinalPattern = regexp.MustCompile(`^(\d{2,3})_`)
var bundledDemoObjectPattern = regexp.MustCompile(`(?i)^Учебный пример\s+(\d{3})\s+·\s+(.+)$`)
var bundledDemoLegacyObjectPattern = regexp.MustCompile(`(?i)^Учебный объект\s+\d+\s*/\s*ДЕМО-(\d{3})\s*/\s*.+$`)

func bundledDemoOrdinal(name string) (int, bool) {
	m := bundledDemoOrdinalPattern.FindStringSubmatch(strings.TrimSpace(name))
	if len(m) != 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil && n >= 1 && n <= 100
}

func bundledDemoObjectKey(name string, original ...string) (string, bool) {
	n, ok := bundledDemoOrdinal(name)
	if !ok {
		return "", false
	}
	base := "Штатные бланки"
	if len(original) > 0 && strings.TrimSpace(original[0]) != "" {
		candidate := strings.TrimSpace(original[0])
		if m := bundledDemoObjectPattern.FindStringSubmatch(candidate); len(m) == 3 {
			if existing, err := strconv.Atoi(m[1]); err == nil && existing == n {
				return candidate, true
			}
			candidate = strings.TrimSpace(m[2])
		}
		base = candidate
	}
	return fmt.Sprintf("Учебный пример %03d · %s", n, base), true
}

func applyBundledDemoObjectKey(rows [][]string, name string) error {
	header := detectLaboratoryHeaderRow(rows)
	if header < 0 || header >= len(rows) {
		return fmt.Errorf("%s: не найден заголовок лабораторного бланка", name)
	}
	col := labHeaderIndex(rows[header], "ключ объекта", "шифр объекта", "код объекта", "объект", "object key", "object_key", "скважина", "номер скважины", "№ скв")
	if col < 0 {
		return fmt.Errorf("%s: не найден столбец ключа объекта", name)
	}
	original := ""
	for i := header + 1; i < len(rows); i++ {
		if len(rows[i]) > col && strings.TrimSpace(rows[i][col]) != "" {
			original = strings.TrimSpace(rows[i][col])
			break
		}
	}
	key, ok := bundledDemoObjectKey(name, original)
	if !ok {
		return fmt.Errorf("%s: не удалось определить номер учебного примера", name)
	}
	for i := header + 1; i < len(rows); i++ {
		for len(rows[i]) <= col {
			rows[i] = append(rows[i], "")
		}
		if strings.TrimSpace(rows[i][col]) != "" {
			rows[i][col] = key
		}
	}
	return nil
}

func repairBundledDemoIdentity(receipts map[string]demoReceipt) error {
	passes := bundledDemoPassSnapshot()
	byID := map[string]int{}
	for i := range passes {
		byID[passes[i].PassID] = i
	}
	for name, receipt := range receipts {
		i, exists := byID[receipt.PassID]
		if !exists {
			continue // manually deleted examples stay deleted
		}
		pass := passes[i]
		expected, ok := bundledDemoObjectKey(name, pass.ObjectKey)
		if !ok {
			continue
		}
		if laboratoryObjectKeysEquivalent(pass.ObjectKey, expected) {
			continue
		}
		pass.ObjectKey = expected
		if strings.TrimSpace(pass.Comment) == "" {
			pass.Comment = "Встроенный учебный пример РОСА. Технологические и экономические данные на следующих шагах являются синтетическими тестовыми исходными данными, а не фактом по реальной скважине."
		} else if !strings.Contains(strings.ToLower(pass.Comment), "учебн") {
			pass.Comment = "Встроенный учебный пример РОСА. " + pass.Comment
		}
		if err := appendLabPass(pass); err != nil {
			return fmt.Errorf("%s: не удалось разделить учебные объекты: %w", name, err)
		}
		passes[i] = pass
	}
	return nil
}

func teachingWorkflowOrdinal(objectKey string) (int, bool) {
	w, ok := trainingWellForObject(objectKey)
	return w.Ordinal, ok
}

const canonicalCrossExampleObject = "Авиловское / скв. 6 / Бобриковский горизонт"

func isCanonicalCrossExampleObject(objectKey string) bool {
	value := strings.TrimSpace(objectKey)
	if m := bundledDemoObjectPattern.FindStringSubmatch(value); len(m) == 3 {
		// Example 001 is the named cross-section lesson; compare its original
		// object caption instead of the synthetic prefix.
		if n, err := strconv.Atoi(m[1]); err == nil && n == 1 {
			value = strings.TrimSpace(m[2])
		}
	}
	parts := strings.Split(value, "/")
	if len(parts) < 3 {
		return false
	}
	field := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(parts[0])), " "))
	plast := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(strings.Join(parts[2:], "/"))), " "))
	well := strings.ToLower(strings.TrimSpace(parts[1]))
	for _, token := range []string{"скважина", "скв.", "скв", "№", "#"} {
		well = strings.ReplaceAll(well, token, "")
	}
	well = strings.TrimSpace(strings.Trim(well, ".,;:-"))
	return field == "авиловское" && well == "6" && plast == "бобриковский горизонт"
}

func teachingWorkflowOriginalObject(objectKey string) string {
	w, ok := trainingWellForObject(objectKey)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s / скв. %s / %s", w.Field, w.Well, w.Layer)
}

// teachingWorkflowRegimeValues provides deterministic synthetic production
// rates for every built-in teaching object. They are deliberately varied so
// cards, charts and comparisons exercise liquid-rate and water-cut paths.
func teachingWorkflowRegimeValues(n int) (liquidRate, waterCut, oilRate string) {
	if w, ok := trainingWellByOrdinal(n); ok {
		return fmt.Sprintf("%.1f", w.LiquidRate), strconv.FormatFloat(w.WaterCut, 'f', 0, 64), fmt.Sprintf("%.1f", w.OilRate)
	}
	if n < 1 {
		n = 1
	}
	wc := 24 + ((n*7 + (n/10)*3) % 70)       // 24..93 %
	liq := 24.0 + float64((n*11+(n/7)*5)%92) // 24..115 m3/day
	oil := liq * (100.0 - float64(wc)) / 100.0
	return fmt.Sprintf("%.1f", liq), strconv.Itoa(wc), fmt.Sprintf("%.1f", oil)
}

// teachingWorkflowStateForObject provides an explicitly synthetic tech-regime
// context only for the 20 built-in teaching wells. It exists so every example
// can exercise MRP/economics/result pages without masquerading missing real-world
// measurements as observed facts.
func teachingWorkflowStateForObject(objectKey string) (TechObjectState, bool) {
	w, ok := trainingWellForObject(objectKey)
	if !ok {
		return TechObjectState{}, false
	}
	currentMRP := 350
	if len(w.Failures) > 0 {
		currentMRP = w.Failures[len(w.Failures)-1].ResultingMRP
	}
	// IMPORTANT: keep this function pure and O(1). It is called while object lists
	// and captions are rebuilt. RC539 scanned every laboratory pass here; because
	// readableObjectKey -> findObjectViewByKey -> allObjectViews calls this for many
	// rows, that turned ordinary painting into an O(N^3)-like UI stall.
	return TechObjectState{
		ObjectKey: objectKey, Organization: "УЧЕБНЫЙ НАБОР РОСА", ProductionUnit: "Учебный контур",
		TPP: "Учебный контур", Workshop: "ЦДНГ-1", DNS: "ДНС Авиловская",
		Field: w.Field, Deposit: "Учебная залежь", Well: w.Well, Block: fmt.Sprintf("Учебный блок %02d", (w.Ordinal-1)/5+1),
		ObjectPlast: w.Layer, ElementDevelopment: "Учебный участок", Equipment: w.LiftMethod,
		PumpModel: w.PumpModel, CurrentMRP: strconv.Itoa(currentMRP),
		BufferPressure: "1.8", LinePressure: "1.2", DynamicLevel: strconv.Itoa(w.PumpDepth - 120), TMSTemperature: "62",
		BottomholePressure: strconv.FormatFloat(w.BottomholePressure, 'f', 1, 64), ReservoirPressure: strconv.FormatFloat(w.ReservoirPressure, 'f', 1, 64),
		LiquidRate: strconv.FormatFloat(w.LiquidRate, 'f', 1, 64), WaterCut: strconv.FormatFloat(w.WaterCut, 'f', 0, 64), OilRate: strconv.FormatFloat(w.OilRate, 'f', 1, 64),
		PlannedActivities: "Учебный синтетический техрежим за три года. Не использовать как производственный режим.",
		SourceFile:        "Встроенный учебный сценарий РОСА — синтетический техрежим только для тестирования",
	}, true
}

func teachingWorkflowObservations(objectKey string) []MRPObservation {
	if _, ok := teachingWorkflowOrdinal(objectKey); !ok {
		return nil
	}
	points := teachingCrossHistoryPoints(objectKey, time.Now())
	state, _ := teachingWorkflowStateForObject(objectKey)
	out := make([]MRPObservation, 0, len(points))
	for _, point := range points {
		end := point.Date
		start := end.AddDate(0, 0, -point.MRPDays)
		out = append(out, MRPObservation{
			ObjectKey: objectKey, Field: state.Field, Well: state.Well, ObjectPlast: state.ObjectPlast, Date: end, CycleStartDate: start, CycleEndDate: end, MRPDays: float64(point.MRPDays),
			DurationSource: "учебный синтетический цикл", Source: "УЧЕБНЫЙ ПРИМЕР · единый сквозной таймлайн РОСА",
			EventType: "Учебный отказ", Planned: false, FailureMode: point.Cause,
			Equipment: point.Equipment, Method: point.Method, Complication: point.Complication,
			ClassificationSource: "Учебный набор; не экспертное заключение", ClassificationConfirmed: false,
			Snapshot: MRPRegimeSnapshot{CapturedAt: end, Equipment: point.Equipment, Completeness: "синтетический учебный снимок; не производственные данные"},
		})
	}
	return out
}

func teachingEconomicsDefaultsForObject(objectKey string) (repairDays, repairCostRub, annualOpexRub, oneTimeRub float64, ok bool) {
	n, ok := teachingWorkflowOrdinal(objectKey)
	if !ok {
		return 0, 0, 0, 0, false
	}
	return float64(4 + n%4), float64(1100000 + (n%6)*150000), float64(180000 + (n%5)*45000), float64(350000 + (n%7)*75000), true
}

func bundledDemoWatercut(name string) (string, bool) {
	w, ok := trainingWellForObject(name)
	if !ok {
		return "", false
	}
	return strconv.FormatFloat(w.WaterCut, 'f', 0, 64), true
}

func repairBundledDemoWatercut(receipts map[string]demoReceipt) error {
	// Retained for source compatibility; new synthetic analyses are created from
	// the versioned 20-well corpus in seedBundledDemoChecked.
	return nil
}
func seedBundledDemo() {
	if err := seedBundledDemoChecked(); err != nil {
		diagLog("TRAINING20: %v", err)
		msg("Загрузка учебного фонда 20 скважин", err.Error(), MB_OK|MB_ICONERROR)
	}
	labPasses = loadLabPasses()
	labPassesLoaded = true
}
func seedBundledDemoChecked() error {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		return err
	}
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	legacyReceipts := map[string]demoReceipt{}
	legacyPath := filepath.Join(dataDir, "demo100_receipts_523.json")
	if b, e := os.ReadFile(legacyPath); e == nil {
		if e = json.Unmarshal(b, &legacyReceipts); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	receiptPath := filepath.Join(dataDir, "training20_receipts_791.json")
	receipts := map[string]demoReceipt{}
	if b, e := os.ReadFile(receiptPath); e == nil {
		if e = json.Unmarshal(b, &receipts); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	byID := map[string]LabPassRecord{}
	canonical := make(map[string]LabPassRecord, len(corpus.Wells)*9)
	for _, well := range corpus.Wells {
		failuresByID := make(map[string]trainingFailure, len(well.Failures))
		for _, event := range well.Failures {
			failuresByID[event.ID] = event
		}
		for _, pass := range well.LabPasses {
			event, linked := failuresByID[pass.FailureID]
			if !linked {
				return fmt.Errorf("лабораторный паспорт %s не связан с отказом скважины %s", pass.PassID, well.ID)
			}
			canonical[pass.PassID] = trainingLabPassRecord(well, event, pass)
		}
	}
	// Receipts are an optimization only; discard entries from prior sample sets
	// so an obsolete marker cannot survive indefinitely or hide a future repair.
	for id := range receipts {
		if _, current := canonical[id]; !current {
			delete(receipts, id)
		}
	}
	for _, p := range labPasses {
		byID[p.PassID] = p
	}
	beginLabPassBulkWrite()
	complete := false
	defer func() {
		if !complete {
			cancelLabPassBulkWrite()
		}
	}()
	for _, r := range legacyReceipts {
		if _, current := canonical[r.PassID]; current {
			continue
		}
		p, ok := byID[r.PassID]
		if !ok || p.Status == labPassDiscardedStatus {
			continue
		}
		p.Status = labPassDiscardedStatus
		p.Comment = strings.TrimSpace(p.Comment + " · прежний встроенный пример снят; учебный набор заменён 20 скважинами.")
		if e := appendLabPass(p); e != nil {
			return e
		}
	}
	// Replace any prior demonstration corpus, including imported workbook rows
	// that were not covered by the old receipt file. Keep real/user-entered
	// laboratory records untouched, and retain only IDs in the current 20-well
	// corpus among synthetic teaching passes.
	for _, old := range labPasses {
		if old.Status != labPassSavedStatus {
			continue
		}
		if expected, current := canonical[old.PassID]; current {
			if trainingLabPassMatchesCanonical(old, expected) {
				continue
			}
			// A reused PassID is not proof that the saved content is current. Older
			// releases reused IDs for different wells and dates; replace the full
			// synthetic snapshot so stale rows cannot survive under a valid ID.
			if e := appendLabPass(expected); e != nil {
				return fmt.Errorf("не удалось актуализировать учебный паспорт %s: %w", old.PassID, e)
			}
			byID[old.PassID] = expected
			receipts[old.PassID] = demoReceipt{PassID: old.PassID}
			continue
		}
		if !isSupersededTrainingPass(old) {
			continue
		}
		old.Status = labPassDiscardedStatus
		old.Comment = strings.TrimSpace(old.Comment + " · прежний учебный пример снят; активен только набор из 20 скважин.")
		if e := appendLabPass(old); e != nil {
			return fmt.Errorf("не удалось убрать прежний учебный пример %s: %w", old.PassID, e)
		}
	}
	if !labRecordsLoaded {
		labRecords = loadLabRecords()
	}
	for _, old := range labRecords {
		if !isSupersededTrainingLabRecord(old) {
			continue
		}
		old.Deleted = true
		if e := appendLabRecord(old); e != nil {
			return fmt.Errorf("не удалось убрать прежнюю лабораторную запись %s: %w", old.ID, e)
		}
	}
	if len(labRecords) > 0 {
		labRecords = loadLabRecords()
	}
	for _, w := range corpus.Wells {
		failuresByID := make(map[string]trainingFailure, len(w.Failures))
		for _, event := range w.Failures {
			failuresByID[event.ID] = event
		}
		for i, p := range w.LabPasses {
			event, linked := failuresByID[p.FailureID]
			if !linked && i < len(w.Failures) {
				event, linked = w.Failures[i], true // compatibility with older corpus files
			}
			if !linked {
				return fmt.Errorf("лабораторный паспорт %s не связан с отказом скважины %s", p.PassID, w.ID)
			}
			pass := trainingLabPassRecord(w, event, p)
			if old, ok := byID[p.PassID]; ok {
				if old.Status == labPassSavedStatus && trainingLabPassMatchesCanonical(old, pass) {
					receipts[p.PassID] = demoReceipt{PassID: p.PassID}
					continue
				}
			}
			if e := appendLabPass(pass); e != nil {
				return e
			}
			receipts[p.PassID] = demoReceipt{PassID: p.PassID}
		}
	}
	if e := flushLabPassBulkWrite(); e != nil {
		return e
	}
	complete = true
	b, e := json.MarshalIndent(receipts, "", "  ")
	if e != nil {
		return e
	}
	return atomicWriteFile(receiptPath, b, 0600)
}

func trainingLabPassMatchesCanonical(saved, canonical LabPassRecord) bool {
	return saved.PassID == canonical.PassID &&
		laboratoryObjectKeysEquivalent(saved.ObjectKey, canonical.ObjectKey) &&
		saved.Date == canonical.Date && saved.Source == canonical.Source &&
		saved.Comment == canonical.Comment && saved.Author == canonical.Author &&
		saved.Status == labPassSavedStatus && saved.ScaleModel == canonical.ScaleModel &&
		saved.Conclusion == canonical.Conclusion &&
		reflect.DeepEqual(saved.Groups, canonical.Groups) &&
		reflect.DeepEqual(saved.Units, canonical.Units) &&
		reflect.DeepEqual(saved.SharedValues, canonical.SharedValues)
}

func isSupersededTrainingPass(pass LabPassRecord) bool {
	key := strings.ToLower(strings.TrimSpace(pass.ObjectKey))
	source := strings.ToLower(strings.TrimSpace(pass.Source))
	comment := strings.ToLower(strings.TrimSpace(pass.Comment))
	id := strings.ToUpper(strings.TrimSpace(pass.PassID))
	if strings.HasPrefix(id, "TRN-") {
		return true // a TRN ID outside the canonical 60-pass set is obsolete
	}
	return strings.Contains(key, "учебный пример") || strings.Contains(key, "учебный объект") ||
		strings.Contains(key, "демо-") || strings.Contains(key, "учебная скважина") ||
		strings.Contains(source, "демо-протокол") || strings.Contains(source, "учебный пример") ||
		strings.Contains(source, "учебный набор") || strings.Contains(comment, "встроенный учебный пример")
}

func isSupersededTrainingLabRecord(record LabRecord) bool {
	key := strings.ToLower(strings.TrimSpace(record.ObjectKey))
	source := strings.ToLower(strings.TrimSpace(record.Source))
	comment := strings.ToLower(strings.TrimSpace(record.Comment + " " + record.Conclusion))
	return strings.Contains(key, "учебный пример") || strings.Contains(key, "учебный объект") ||
		strings.Contains(key, "демо-") || strings.Contains(key, "учебная скважина") ||
		strings.Contains(source, "демо-протокол") || strings.Contains(source, "учебный пример") ||
		strings.Contains(source, "учебный набор") || strings.Contains(comment, "встроенный учебный пример")
}

func isRetiredLegacyTeachingObjectKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "учебный пример") || strings.Contains(key, "учебный объект") ||
		strings.Contains(key, "демо-") || strings.Contains(key, "штатные бланки")
}

func retireLegacyTeachingFailures() {
	for _, event := range append([]FailureEvent(nil), failureEvents...) {
		key := event.ObjectKey
		if !isRetiredLegacyTeachingObjectKey(key) {
			continue
		}
		if err := deleteFailureEvent(event.ID, event.ObjectKey); err != nil {
			diagLog("TRAINING20: failed to retire old demo failure id=%s err=%v", event.ID, err)
		}
	}
}

func bundledDemoPassSnapshot() []LabPassRecord {
	if !labPassesLoaded {
		labPasses = loadLabPasses()
		labPassesLoaded = true
	}
	return labPasses
}
