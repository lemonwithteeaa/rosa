//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

var (
	labPassBulkMode  bool
	labPassBulkQueue []LabPassRecord
)

func beginLabPassBulkWrite() {
	labPassBulkMode = true
	labPassBulkQueue = nil
}

func cancelLabPassBulkWrite() {
	labPassBulkMode = false
	labPassBulkQueue = nil
}

func appendLabPassLegacyJSONLBatch(passes []LabPassRecord) error {
	if len(passes) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, pass := range passes {
		b, err := json.Marshal(pass)
		if err != nil {
			return err
		}
		if personalPath(labPassPath()) {
			b, err = encryptPersonalLine(b)
			if err != nil {
				return err
			}
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(labPassPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(buf.Bytes()); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func flushLabPassBulkWrite() error {
	passes := append([]LabPassRecord(nil), labPassBulkQueue...)
	labPassBulkMode = false
	labPassBulkQueue = nil
	if len(passes) == 0 {
		return nil
	}
	var sqliteErr error
	if repo := activeLabPassRepository(); repo != nil {
		if batch, ok := repo.(LabPassBatchRepository); ok {
			sqliteErr = batch.SaveBatch(passes)
		} else {
			for _, pass := range passes {
				if sqliteErr = repo.Save(pass); sqliteErr != nil {
					break
				}
			}
		}
	}
	legacyErr := appendLabPassLegacyJSONLBatch(passes)
	if activeLabPassRepository() != nil && sqliteErr == nil {
		return legacyErr
	}
	if legacyErr == nil {
		return nil
	}
	if sqliteErr != nil {
		return fmt.Errorf("не удалось пакетно сохранить ни в SQLite (%v), ни в резервный JSONL (%v)", sqliteErr, legacyErr)
	}
	return legacyErr
}

const (
	labPassDraftStatus     = "draft"
	labPassSavedStatus     = "saved"
	labPassDiscardedStatus = "discarded"
)

// LabPassRecord is the backward-compatible aggregate introduced after the
// legacy per-section LabRecord format. Legacy records remain untouched and
// readable; a pass stores a coherent snapshot of every laboratory section.
type LabPassRecord struct {
	PassID         string                       `json:"pass_id"`
	BasedOn        string                       `json:"based_on_version,omitempty"`
	ObjectKey      string                       `json:"object_key"`
	Date           string                       `json:"date"`
	Source         string                       `json:"source,omitempty"`
	Comment        string                       `json:"comment,omitempty"`
	Author         string                       `json:"author,omitempty"`
	Status         string                       `json:"status"`
	Groups         map[string]map[string]string `json:"groups"`
	Units          map[string]map[string]string `json:"units,omitempty"`
	ScaleModel     string                       `json:"scale_model,omitempty"`
	SharedValues   map[string]string            `json:"shared_values,omitempty"`
	Completed      []string                     `json:"completed_groups,omitempty"`
	Conclusion     string                       `json:"conclusion,omitempty"`
	CreatedAt      string                       `json:"created_at"`
	UpdatedAt      string                       `json:"updated_at"`
	ImportedAt     string                       `json:"imported_at,omitempty"`
	ImportFileName string                       `json:"import_file_name,omitempty"`
	// ExtraValues keeps imported indicators that are not yet present in the
	// approved laboratory classifier. They remain part of the pass and are
	// shown to the user instead of being silently discarded.
	ExtraValues map[string]string `json:"extra_values,omitempty"`
}

func labPassPath() string { return filepath.Join(dataDir, "laboratory_passes.jsonl") }

func normalizeLaboratoryObjectKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

// Laboratory data can contain a human caption (with slashes, dots and dashes),
// while the Main page keeps a compact technical key.  Compare both by a stable
// letters-and-digits fingerprint so the same well is not shown as «нет данных».
func laboratoryObjectKeyFingerprint(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, strings.TrimSpace(value))
}

func laboratoryObjectKeysEquivalent(left, right string) bool {
	a, b := laboratoryObjectKeyFingerprint(left), laboratoryObjectKeyFingerprint(right)
	return a != "" && a == b
}

func newLabPassID(objectKey, date string, now time.Time) string {
	base := normalizeLaboratoryObjectKey(objectKey) + "|" + strings.TrimSpace(date) + "|" + now.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(base))
	return "LP-" + hex.EncodeToString(sum[:10])
}

func cloneLabGroups(groups map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(groups))
	for group, values := range groups {
		copyValues := make(map[string]string, len(values))
		for key, value := range values {
			copyValues[key] = value
		}
		out[group] = copyValues
	}
	return out
}

func completedLabGroupsForModel(groups map[string]map[string]string, modelCode string) []string {
	var out []string
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		filled, _ := laboratoryHistoryFieldCoverage(group, groups[group.Code], modelCode)
		if filled > 0 {
			out = append(out, group.Code)
		}
	}
	sort.Strings(out)
	return out
}

// completedLabGroups is retained for compatibility with older call sites and
// historical defaults. New persisted passes should always call the model-aware
// variant so a calculated/inactive scale field cannot mark a direction complete.
func completedLabGroups(groups map[string]map[string]string) []string {
	return completedLabGroupsForModel(groups, "davies")
}

func laboratoryImportedPassID(pass LabPassRecord) string {
	payload := struct {
		ObjectKey  string                       `json:"object_key"`
		Date       string                       `json:"date"`
		Source     string                       `json:"source"`
		Groups     map[string]map[string]string `json:"groups"`
		Units      map[string]map[string]string `json:"units"`
		ScaleModel string                       `json:"scale_model"`
	}{normalizeLaboratoryObjectKey(pass.ObjectKey), strings.TrimSpace(pass.Date), strings.TrimSpace(pass.Source), pass.Groups, pass.Units, pass.ScaleModel}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return "LP-IMP-" + hex.EncodeToString(sum[:10])
}

func laboratoryPassCompletenessSummary(pass LabPassRecord) string {
	var sections []string
	var missingBaseline []string
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := pass.Groups[group.Code]
		filled, total := laboratoryHistoryFieldCoverage(group, values, pass.ScaleModel)
		if filled > 0 {
			sections = append(sections, fmt.Sprintf("%s — %d из %d", group.Name, filled, total))
		}
		baseline := laboratoryBaselineKeys(group.Code)
		for _, field := range group.Fields {
			if !baseline[field.Key] || !laboratoryInputFieldEligible(group.Code, field.Key) {
				continue
			}
			if strings.TrimSpace(values[field.Key]) == "" {
				missingBaseline = append(missingBaseline, group.Name+": "+field.Label)
			}
		}
	}
	sectionText := "заполненных лабораторных разделов нет"
	if len(sections) > 0 {
		sectionText = strings.Join(sections, "; ")
	}
	missingText := "базовые пропуски по заполненным направлениям не выявлены"
	if len(missingBaseline) > 0 {
		const limit = 12
		shown := missingBaseline
		if len(shown) > limit {
			shown = shown[:limit]
		}
		missingText = "не заполнены базовые показатели: " + strings.Join(shown, "; ")
		if len(missingBaseline) > limit {
			missingText += fmt.Sprintf("; ещё %d", len(missingBaseline)-limit)
		}
	}
	return "Полнота диагностики: " + sectionText + ". " + missingText + ". Неполнота не запрещает осторожный предварительный вывод, если алгоритм конкретного направления допускает работу на имеющихся данных."
}

func (p *LabPassRecord) prepare(now time.Time) error {
	p.ObjectKey = strings.TrimSpace(p.ObjectKey)
	if p.ObjectKey == "" {
		return fmt.Errorf("не указан объект лабораторной диагностики")
	}
	if p.Date == "" {
		p.Date = now.Format("02.01.2006")
	}
	if p.PassID == "" {
		p.PassID = newLabPassID(p.ObjectKey, p.Date, now)
	}
	if p.Status == "" {
		p.Status = labPassDraftStatus
	}
	if p.Status != labPassDraftStatus && p.Status != labPassSavedStatus && p.Status != labPassDiscardedStatus {
		return fmt.Errorf("неподдерживаемое состояние лабораторной диагностики: %s", p.Status)
	}
	if p.Groups == nil {
		p.Groups = map[string]map[string]string{}
	}
	if p.Units == nil {
		p.Units = map[string]map[string]string{}
	}
	if p.ScaleModel == "" {
		p.ScaleModel = "davies"
	}
	if p.SharedValues == nil {
		p.SharedValues = map[string]string{}
	}
	p.Completed = completedLabGroupsForModel(p.Groups, p.ScaleModel)
	if p.CreatedAt == "" {
		p.CreatedAt = now.UTC().Format(time.RFC3339)
	}
	p.UpdatedAt = now.UTC().Format(time.RFC3339)
	return nil
}

func appendLabPass(pass LabPassRecord) error {
	if err := pass.prepare(time.Now()); err != nil {
		return err
	}
	if labPassBulkMode {
		labPassBulkQueue = append(labPassBulkQueue, pass)
		return nil
	}
	// RC184 transition policy: SQLite is the durable primary store. During the
	// migration window JSONL is still mirrored so rollback to an older RC remains
	// possible. A SQLite failure never silently loses the user's save.
	var sqliteErr error
	if repo := activeLabPassRepository(); repo != nil {
		sqliteErr = repo.Save(pass)
		if sqliteErr != nil {
			diagLog("STORAGE sqlite lab pass write failed pass=%s err=%v", pass.PassID, sqliteErr)
		}
	}
	legacyErr := appendLabPassLegacyJSONL(pass)
	if activeLabPassRepository() != nil && sqliteErr == nil {
		return legacyErr
	}
	if legacyErr == nil {
		return nil
	}
	if sqliteErr != nil {
		return fmt.Errorf("не удалось сохранить ни в SQLite (%v), ни в резервный JSONL (%v)", sqliteErr, legacyErr)
	}
	return legacyErr
}

func loadLabPasses() []LabPassRecord {
	if repo := activeLabPassRepository(); repo != nil {
		rows, err := repo.LoadAll()
		if err == nil && len(rows) > 0 {
			return rows
		}
		if err != nil {
			diagLog("STORAGE sqlite lab pass read failed: %v", err)
		}
	}
	return loadLabPassesLegacyJSONL()
}

func repositoryLabPassesForObject(objectKey string) []LabPassRecord {
	if repo := activeLabPassRepository(); repo != nil {
		rows, err := repo.LoadForObject(objectKey)
		if err == nil && len(rows) > 0 {
			return rows
		}
		// Existing databases may contain an older punctuation-sensitive normalized
		// key.  A bounded all-row fallback migrates the read path without rewriting
		// or duplicating the stored analyses.
		if all, allErr := repo.LoadAll(); allErr == nil {
			matched := make([]LabPassRecord, 0)
			for _, pass := range all {
				if laboratoryObjectKeysEquivalent(pass.ObjectKey, objectKey) {
					matched = append(matched, pass)
				}
			}
			if len(matched) > 0 {
				return matched
			}
		}
		if err != nil {
			diagLog("STORAGE object history query failed key=%s err=%v", objectKey, err)
		}
	}
	all := loadLabPassesLegacyJSONL()
	out := make([]LabPassRecord, 0)
	for _, pass := range all {
		if laboratoryObjectKeysEquivalent(pass.ObjectKey, objectKey) {
			out = append(out, pass)
		}
	}
	return out
}

func repositoryLatestSavedLabPasses() []LabPassRecord {
	if repo := activeLabPassRepository(); repo != nil {
		rows, err := repo.LoadLatestSaved()
		if err == nil {
			return rows
		}
		diagLog("STORAGE latest mass query failed: %v", err)
	}
	all := loadLabPassesLegacyJSONL()
	keys := make([]string, 0)
	seen := map[string]bool{}
	for _, pass := range all {
		if pass.Status != labPassSavedStatus {
			continue
		}
		key := normalizeLaboratoryObjectKey(pass.ObjectKey)
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return laboratoryLatestSavedPassesForKeys(keys, all)
}

func labPassForObject(passes []LabPassRecord, objectKey, passID string) (LabPassRecord, bool) {
	normalized := normalizeLaboratoryObjectKey(objectKey)
	for _, pass := range passes {
		if pass.PassID == passID && normalizeLaboratoryObjectKey(pass.ObjectKey) == normalized {
			pass.Groups = cloneLabGroups(pass.Groups)
			pass.Units = cloneLabGroups(pass.Units)
			return pass, true
		}
	}
	return LabPassRecord{}, false
}

// legacyLabPassView exposes old independent records through the new model
// without rewriting or deleting their source file.
func legacyLabPassView(record LabRecord) LabPassRecord {
	passID := "LEGACY-" + record.ID
	created := record.CreatedAt
	if created == "" {
		created = time.Now().UTC().Format(time.RFC3339)
	}
	groups := map[string]map[string]string{record.GroupCode: cloneLabGroups(map[string]map[string]string{"v": record.Values})["v"]}
	units := map[string]map[string]string{record.GroupCode: cloneLabGroups(map[string]map[string]string{"u": record.Units})["u"]}
	return LabPassRecord{
		PassID: passID, ObjectKey: record.ObjectKey, Date: record.Date,
		Source: record.Source, Comment: record.Comment, Status: labPassSavedStatus,
		Groups: groups, Units: units, ScaleModel: record.ScaleModel, Completed: completedLabGroupsForModel(groups, record.ScaleModel), CreatedAt: created, UpdatedAt: created,
		Conclusion: record.Conclusion,
	}
}

func buildLaboratoryPassConclusion(pass LabPassRecord) string {
	var sections, missing, attention, uncertain []string
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := pass.Groups[group.Code]
		canonicalValues := canonicalizeLaboratoryValues(group, values, pass.Units[group.Code])
		_, _, validationErrors := validateLaboratoryGroup(group, canonicalValues)
		filled, total := laboratoryHistoryFieldCoverage(group, values, pass.ScaleModel)
		if filled == 0 && len(validationErrors) == 0 {
			missing = append(missing, group.Name)
			continue
		}
		conclusion := buildLaboratoryConclusionWithScaleModel(group, canonicalValues, pass.ScaleModel)
		assessment := laboratoryGroupSummary(group, canonicalValues, pass.ScaleModel)
		if assessment.Level == 0 {
			uncertain = append(uncertain, group.Name)
		}
		if assessment.Level >= 2 {
			attention = append(attention, group.Name)
		}
		sections = append(sections, fmt.Sprintf("РАЗДЕЛ: %s\nЗаполнено показателей: %d из %d.\nИсточник раздела: %s. Дата данных: %s.\n\nИСХОДНЫЕ ДАННЫЕ РАЗДЕЛА\n%s\n\n%s", strings.ToUpper(group.Name), filled, total, emptyLabText(pass.Source), emptyLabText(pass.Date), laboratoryPassGroupValuesWithProvenance(pass, group, values), conclusion))
	}
	if len(sections) == 0 {
		return fmt.Sprintf("Общее заключение по объекту %s не сформировано: в текущей лабораторной диагностике нет заполненных показателей.", pass.ObjectKey)
	}
	priority := "По разделам, которые удалось оценить, признаки возможного осложнения не выявлены."
	if len(attention) > 0 {
		priority = "Направления, требующие приоритетной проверки: " + strings.Join(attention, ", ") + ". Совместное влияние следует оценивать по техрежиму, истории ремонтов и результатам ОЛИ/ОПИ."
	}
	if len(uncertain) > 0 {
		if len(uncertain) == len(sections) {
			priority = "Общий риск пока не определён: ни одно из представленных направлений не удалось оценить."
		}
		priority += " Оценить пока нельзя: " + strings.Join(uncertain, ", ") + ". Неполная или невыполненная оценка не означает отсутствия осложнения."
	}
	missingText := "Все лабораторные направления представлены."
	if len(missing) > 0 {
		missingText = "Не заполнены направления: " + strings.Join(missing, ", ") + ". Их отсутствие не блокирует предварительный вывод, но снижает полноту комплексной оценки."
	}
	historyPasses := repositoryLabPassesForObject(pass.ObjectKey)
	inhibitorTrials := laboratoryInhibitorDatabaseText(pass, historyPasses)
	return fmt.Sprintf("ЕДИНОЕ ПРЕДВАРИТЕЛЬНОЕ ЛАБОРАТОРНОЕ ЗАКЛЮЧЕНИЕ\n\nОбъект: %s\nИдентификатор диагностики: %s\nДата анализа: %s\nИсточник / протокол: %s\nАвтор: %s\nЗаполнено направлений: %d из 6.\n\nКОНТРОЛЬ ИСТОЧНИКОВ И АКТУАЛЬНОСТИ\n%s\n\n%s\n\n%s\n\n%s\n\n%s\n\nОБЩИЙ ВЫВОД\n%s\n\nЗаключение является предварительной инженерной оценкой и требует подтверждения лабораторией, промысловыми исследованиями, ОЛИ/ОПИ и производственной проверкой.",
		pass.ObjectKey, pass.PassID, pass.Date, emptyLabText(pass.Source), emptyLabText(pass.Author), len(sections), laboratoryPassProvenanceSummary(pass)+"\n"+laboratoryPassCompletenessSummary(pass), laboratoryPassQCText(pass, time.Now()), strings.Join(sections, "\n\n"), missingText, sharedLaboratoryPassSummary(pass)+"\n\n"+inhibitorTrials, priority+"\n\n"+laboratoryPassTechnologistSummary(pass))
}

func laboratoryPassProvenanceSummary(pass LabPassRecord) string {
	counts := map[string]int{"Б": 0, "Д": 0, "Т": 0, "О": 0}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		group = laboratoryScaleInputGroup(group, pass.ScaleModel)
		values := pass.Groups[group.Code]
		for _, field := range group.Fields {
			if strings.TrimSpace(values[field.Key]) == "" {
				continue
			}
			marker := strings.TrimSuffix(laboratoryFieldPrefixForModel(group.Code, field.Key, pass.ScaleModel), " · ")
			for _, code := range []string{"Б", "Д", "Т", "О"} {
				if strings.Contains(marker, code) {
					counts[code]++
				}
			}
		}
	}
	suffix := ""
	if strings.TrimSpace(pass.ImportedAt) != "" {
		suffix = "; импортировано: " + pass.ImportedAt
	}
	return fmt.Sprintf("Лабораторный источник/протокол: %s; дата: %s%s. Значения по происхождению: Б — %d, Д — %d, Т — %d, О — %d. Метка Т означает автоматический снимок Техрежима; О — единое значение текущей диагностики. Отсутствие номера протокола явно снижает прослеживаемость, но не удаляет введённые данные.", emptyLabText(pass.Source), emptyLabText(pass.Date), suffix, counts["Б"], counts["Д"], counts["Т"], counts["О"])
}

func laboratoryPassGroupValuesWithProvenance(pass LabPassRecord, group LabGroupDef, values map[string]string) string {
	var lines []string
	for _, field := range group.Fields {
		value := strings.TrimSpace(values[field.Key])
		if value == "" {
			continue
		}
		validation := validateLaboratoryValue(group.Code, field, value)
		if validation.State == labValueError {
			lines = append(lines, fmt.Sprintf("• [ОШИБКА — НЕ ИСПОЛЬЗОВАНО] %s: отклонено проверкой правдоподобия (%s)", field.Label, validation.Message))
			continue
		}
		marker := strings.TrimSuffix(laboratoryFieldPrefixForModel(group.Code, field.Key, pass.ScaleModel), " · ")
		if validation.State == labValueWarning {
			marker += "; ПРОВЕРИТЬ"
		}
		source := "протокол " + emptyLabText(pass.Source)
		if strings.Contains(marker, "Р") {
			source = "расчёт РОСА по сохранённому ионному составу"
		} else if strings.Contains(marker, "Т") {
			source = "Техрежим; значение автоматически зафиксировано в диагностике"
		} else if strings.Contains(marker, "О") {
			source = "общее поле диагностики; протокол " + emptyLabText(pass.Source)
		}
		unit := strings.TrimSpace(pass.Units[group.Code][field.Key])
		if unit == "" {
			unit = field.Unit
		}
		if group.Code == "scale" && pass.ScaleModel == "water_mixing" {
			source = "вода 1; протокол " + emptyLabText(values["mix1_source"])
			if strings.HasPrefix(field.Key, "mix2_") {
				source = "вода 2; протокол " + emptyLabText(values["mix2_source"])
			}
			if strings.HasPrefix(field.Key, "mix_") {
				source = "условия смешения; протокол " + emptyLabText(pass.Source)
			}
		}
		lines = append(lines, fmt.Sprintf("• [%s; источник: %s; дата: %s] %s: %s %s", marker, source, emptyLabText(pass.Date), field.Label, value, unit))
	}
	if len(lines) == 0 {
		return "заполненные значения отсутствуют"
	}
	return strings.Join(lines, "\n")
}

func laboratoryPassGroupValues(group LabGroupDef, values map[string]string) string {
	var lines []string
	for _, field := range group.Fields {
		value := strings.TrimSpace(values[field.Key])
		if value == "" {
			continue
		}
		validation := validateLaboratoryValue(group.Code, field, value)
		if validation.State == labValueError {
			lines = append(lines, fmt.Sprintf("• [ОШИБКА — НЕ ИСПОЛЬЗОВАНО] %s: отклонено проверкой правдоподобия (%s)", field.Label, validation.Message))
			continue
		}
		marker := strings.TrimSuffix(laboratoryFieldPrefix(group.Code, field.Key), " · ")
		if validation.State == labValueWarning {
			marker += "; ПРОВЕРИТЬ"
		}
		lines = append(lines, fmt.Sprintf("• [%s] %s: %s %s", marker, field.Label, value, field.Unit))
	}
	if len(lines) == 0 {
		return "заполненные значения отсутствуют"
	}
	return strings.Join(lines, "\n")
}

func emptyLabText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "не указан"
	}
	return strings.TrimSpace(value)
}

func sharedLaboratoryPassSummary(pass LabPassRecord) string {
	shared := pass.SharedValues
	if len(shared) == 0 {
		shared = collectLaboratorySharedValues(pass.Groups)
	}
	conflicts := laboratorySharedValueConflictsWithUnits(pass.Groups, pass.Units)
	if len(conflicts) > 0 {
		return fmt.Sprintf("Согласованность общих показателей: канонических значений %d. ОБНАРУЖЕНЫ РАСХОЖДЕНИЯ: %s. Конфликтующие значения не следует использовать как единый факт до сверки источника и даты.", len(shared), strings.Join(conflicts, "; "))
	}
	return fmt.Sprintf("Согласованность общих показателей: канонических значений %d. Расхождений между эквивалентными полями вкладок не выявлено; значения используются однократно.", len(shared))
}

func laboratorySharedValueConflictsWithUnits(groups, units map[string]map[string]string) []string {
	canonical := map[string]map[string]string{}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		canonical[group.Code] = canonicalizeLaboratoryValues(group, groups[group.Code], units[group.Code])
	}
	return laboratorySharedValueConflicts(canonical)
}

func laboratorySharedValueConflicts(groups map[string]map[string]string) []string {
	type observed struct {
		display string
		groups  []string
	}
	byShared := map[string]map[string]*observed{}
	for _, group := range labGroups {
		if group.Code == "overall" {
			continue
		}
		values := groups[group.Code]
		for _, field := range group.Fields {
			shared := laboratorySharedKey(group.Code, field.Key)
			value := strings.TrimSpace(values[field.Key])
			if shared == "" || value == "" {
				continue
			}
			normalized := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(value, ",", ".")), " "))
			if byShared[shared] == nil {
				byShared[shared] = map[string]*observed{}
			}
			item := byShared[shared][normalized]
			if item == nil {
				item = &observed{display: value}
				byShared[shared][normalized] = item
			}
			item.groups = append(item.groups, group.Name)
		}
	}
	var conflicts []string
	for shared, variants := range byShared {
		if len(variants) < 2 {
			continue
		}
		var descriptions []string
		for _, item := range variants {
			sort.Strings(item.groups)
			descriptions = append(descriptions, fmt.Sprintf("%s (%s)", item.display, strings.Join(item.groups, ", ")))
		}
		sort.Strings(descriptions)
		conflicts = append(conflicts, fmt.Sprintf("%s: %s", shared, strings.Join(descriptions, " / ")))
	}
	sort.Strings(conflicts)
	return conflicts
}

// Opening a historical pass always creates an independent draft.
func laboratoryDraftFromSavedPass(source LabPassRecord, now time.Time) (LabPassRecord, error) {
	pass := source
	pass.Groups = cloneLabGroups(source.Groups)
	pass.Units = cloneLabGroups(source.Units)
	pass.SharedValues = make(map[string]string, len(source.SharedValues))
	for key, value := range source.SharedValues {
		pass.SharedValues[key] = value
	}
	pass.Completed = nil
	pass.PassID = ""
	pass.BasedOn = source.PassID
	pass.Status = labPassDraftStatus
	pass.CreatedAt = ""
	pass.UpdatedAt = ""
	pass.Conclusion = ""
	err := pass.prepare(now)
	return pass, err
}
