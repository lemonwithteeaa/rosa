//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type assessmentClassDirection struct {
	Name     string
	Prefix   string
	LabGroup string
	Levels   []string
}

var assessmentClassDirections = []assessmentClassDirection{
	{Name: "АСПО", Prefix: "А", LabGroup: "aspo", Levels: []string{"А1", "А2", "А3"}},
	{Name: "Коррозия", Prefix: "К", LabGroup: "corrosion", Levels: []string{"К1", "К2", "К3"}},
	{Name: "Высокая вязкость", Prefix: "В", LabGroup: "aspo", Levels: []string{"В1", "В2", "В3"}},
	{Name: "Солеотложения", Prefix: "С", LabGroup: "scale", Levels: []string{"С1", "С2", "С3"}},
	{Name: "Свободный газ", Prefix: "Г", LabGroup: "gases", Levels: []string{"Г1", "Г2", "Г3"}},
	{Name: "Механические примеси", Prefix: "М", LabGroup: "solids", Levels: []string{"М1", "М2", "М3"}},
	{Name: "Гидраты", Prefix: "Гд", LabGroup: "gases", Levels: []string{"Гд1"}},
	{Name: "Высокая температура", Prefix: "Т", LabGroup: "gases", Levels: []string{"Т1", "Т2", "Т3"}},
}

var (
	assessmentCompareRowsCache      [][]string
	assessmentLabPassSnapshotObject string
	assessmentLabPassSnapshot       []LabPassRecord
	assessmentLabPassSnapshotValid  bool
)

func assessmentLabPassesForObjectCached(objectKey string) []LabPassRecord {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil
	}
	if assessmentLabPassSnapshotValid && laboratoryObjectKeysEquivalent(assessmentLabPassSnapshotObject, objectKey) {
		return assessmentLabPassSnapshot
	}
	rows := repositoryLabPassesForObject(objectKey)
	assessmentLabPassSnapshotObject = objectKey
	assessmentLabPassSnapshot = rows
	assessmentLabPassSnapshotValid = true
	return assessmentLabPassSnapshot
}

type AssessmentClassRecord struct {
	ObjectKey                   string `json:"object_key"`
	Direction                   string `json:"direction"`
	Prefix                      string `json:"prefix"`
	TechnologistClass           string `json:"technologist_class,omitempty"`
	TechnologistComment         string `json:"technologist_comment,omitempty"`
	ManualLabClass              string `json:"manual_lab_class,omitempty"`
	ManualLabComment            string `json:"manual_lab_comment,omitempty"`
	FinalClass                  string `json:"final_class,omitempty"`
	FinalComment                string `json:"final_comment,omitempty"`
	DecisionMode                string `json:"decision_mode,omitempty"`
	ProgramClassAtDecision      string `json:"program_class_at_decision,omitempty"`
	ProgramReasonAtDecision     string `json:"program_reason_at_decision,omitempty"`
	ConfirmedBy                 string `json:"confirmed_by,omitempty"`
	ConfirmedAt                 string `json:"confirmed_at,omitempty"`
	UpdatedAt                   string `json:"updated_at,omitempty"`
	LabReviewSkipped            bool   `json:"lab_review_skipped,omitempty"`
	FullClassificationConfirmed bool   `json:"full_classification_confirmed,omitempty"`
	RestoredFromArchive         bool   `json:"restored_from_archive,omitempty"`
	RestoredDecisionID          string `json:"restored_decision_id,omitempty"`
	ProgramSourceAtDecision     string `json:"program_source_at_decision,omitempty"`
}

type AssessmentDecisionRecord struct {
	DecisionID, ObjectKey, Direction, TechnologistClass, ProgramClass           string
	FinalClass, DecisionMode, Comment, Reason, Source, ConfirmedBy, ConfirmedAt string
}

func assessmentHasAnyLaboratoryData() bool {
	for _, direction := range assessmentClassDirections {
		if assessmentLatestLab(direction) != nil {
			return true
		}
	}
	return false
}

func assessmentVerificationPlan(direction assessmentClassDirection) string {
	switch direction.Prefix {
	case "А":
		return "повторный анализ нефти на парафины/смолы/асфальтены; температура насыщения парафином; профиль температуры; исследование фактических отложений (ИК-Фурье/ДСА)."
	case "К":
		return "скорость коррозии купонами или зондом в динамике; pH, Cl⁻, O₂, CO₂ и H₂S одной пробы; анализ продуктов коррозии; толщинометрия/дефектоскопия."
	case "В":
		return "вязкость при нескольких рабочих температурах; реология; плотность и состав нефти; фактический температурный профиль и условия запуска."
	case "С":
		return "полный ионный состав и баланс зарядов; pH и температура отбора; расчёт SI проверенной моделью; анализ осадка; при смешении — отдельные анализы обеих вод и смеси."
	case "Г":
		return "давление и температура на приёме; доля свободного газа; газовый фактор и данные о фазовом поведении флюида; динамический уровень; режим и паспорт газосепаратора."
	case "М":
		return "КВЧ/взвешенные вещества в повторных пробах; гранулометрия d₅₀/d₉₀; минералогия/элементный состав частиц; проверка пескопроявления и продуктов коррозии."
	case "Гд":
		return "состав газа и воды; совместные фактические T/P по профилю; равновесный расчёт гидратов; проверка локальных охлаждений и дросселирования."
	case "Т":
		return "поверенная глубинная температура и профиль T; привязка к глубине подвески; проверка термостойкости кабеля, эластомеров и реагента."
	}
	return "повторный профильный анализ и сопоставление с историей проявлений и отказов."
}

func assessmentReconciliationAdvice(direction assessmentClassDirection, tech, program, confidence string) string {
	tech, program = strings.TrimSpace(tech), strings.TrimSpace(program)
	if program == "" || program == "НД" || program == "Не рекомендуется" {
		if tech != "" && tech != "НД" {
			return "До получения лабораторного подтверждения корректнее оставить класс технолога как предварительный и явно зафиксировать основание."
		}
		return "Данных недостаточно: итоговый класс пока не подтверждать; сначала выполнить указанные исследования."
	}
	if tech == "" || tech == "НД" || tech == "Не относится" {
		return "Программное предположение лучше обеспечено доступными количественными данными; его можно принять как предварительное после проверки источника и единиц."
	}
	if strings.EqualFold(tech, program) {
		return "Обе оценки совпадают. Подтвердите класс без корректировки, если исходные данные и область применимости метода проверены."
	}
	if strings.EqualFold(confidence, "высокая") {
		return "Разногласие существенно: программный класс опирается на прямой количественный критерий. Корректнее перепроверить ввод и использовать его, если контрольное исследование подтвердит показатель."
	}
	return "Программная уверенность ограничена. До подтверждающих исследований корректнее сохранить решение технолога, а изменение оформить только «с корректировкой» и с комментарием."
}

var (
	assessmentClassStoreLoaded bool
	assessmentClassStore       = map[string]AssessmentClassRecord{}
	assessmentDecisionLoaded   bool
	assessmentDecisionStore    []AssessmentDecisionRecord
	assessInnerTab             = 0
	assessTechSelected         = 0
	assessLabSelected          = 0
	assessCompareSelected      = 0
)

const idAssessmentHistoryClose = 9490

var (
	assessmentHistoryDialogDone bool
	assessmentHistoryHeader     syscall.Handle
	assessmentHistoryAccent     syscall.Handle
	assessmentHistoryTitle      syscall.Handle
	assessmentHistoryText       syscall.Handle
	assessmentHistoryClose      syscall.Handle
)

func assessmentClassStoragePath() string {
	return filepath.Join(appDataDir(), "assessment_classification.json")
}

func assessmentDecisionStoragePath() string {
	return filepath.Join(appDataDir(), "assessment_decision_history.json")
}

func ensureAssessmentDecisionHistory() {
	if assessmentDecisionLoaded {
		return
	}
	assessmentDecisionLoaded = true
	loadJSON(assessmentDecisionStoragePath(), &assessmentDecisionStore)
}

func appendAssessmentDecisionHistory(r AssessmentClassRecord) error {
	ensureAssessmentDecisionHistory()
	source := "—"
	if d, ok := assessmentDirectionByName(r.Direction); ok {
		source = assessmentProgramSource(d)
	}
	assessmentDecisionStore = append(assessmentDecisionStore, AssessmentDecisionRecord{
		DecisionID:        fmt.Sprintf("AC-%d", time.Now().UnixNano()),
		ObjectKey:         r.ObjectKey,
		Direction:         r.Direction,
		TechnologistClass: r.TechnologistClass,
		ProgramClass:      r.ProgramClassAtDecision,
		FinalClass:        r.FinalClass,
		DecisionMode:      r.DecisionMode,
		Comment:           r.FinalComment,
		Reason:            r.ProgramReasonAtDecision,
		Source:            source,
		ConfirmedBy:       r.ConfirmedBy,
		ConfirmedAt:       r.ConfirmedAt,
	})
	return saveJSON(assessmentDecisionStoragePath(), assessmentDecisionStore)
}

func assessmentDecisionsForObject(objectKey string) []AssessmentDecisionRecord {
	ensureAssessmentDecisionHistory()
	rows := make([]AssessmentDecisionRecord, 0)
	for i := len(assessmentDecisionStore) - 1; i >= 0; i-- {
		if laboratoryObjectKeysEquivalent(assessmentDecisionStore[i].ObjectKey, objectKey) {
			rows = append(rows, assessmentDecisionStore[i])
		}
	}
	return rows
}

func refreshAssessmentDecisionHistory() {
	if assessDecisionHistoryList == 0 {
		return
	}
	if assessDecisionHistoryLabel != 0 {
		if strings.TrimSpace(selectedObjectKey) == "" {
			setText(assessDecisionHistoryLabel, "УЧЕБНЫЕ ПРИМЕРЫ · ИСТОРИЯ КЛАССИФИКАЦИЙ ПО 20 СКВАЖИНАМ")
		} else {
			setText(assessDecisionHistoryLabel, "РАНЕЕ СОХРАНЁННЫЕ КЛАССИФИКАЦИИ (УТВЕРЖДЁННЫЕ)")
		}
	}
	listBeginUpdate(assessDecisionHistoryList)
	defer listEndUpdate(assessDecisionHistoryList)
	listClear(assessDecisionHistoryList)
	realRows := assessmentDecisionsForObject(selectedObjectKey)
	row := 0
	for _, r := range realRows {
		if row >= 100 {
			break
		}
		source := strings.TrimSpace(r.Source)
		if source == "" {
			source = "источник не сохранён в ранней записи"
		}
		listAdd(assessDecisionHistoryList, row, []string{dash(r.ConfirmedAt), r.Direction, dash(r.FinalClass), source, "Утверждено", dash(r.ConfirmedBy)})
		row++
	}
	if strings.TrimSpace(selectedObjectKey) == "" {
		for _, r := range teachingAssessmentOverviewHistoryRows() {
			if row >= 100 {
				break
			}
			listAdd(assessDecisionHistoryList, row, []string{r.ConfirmedAt, r.Direction, r.FinalClass, r.Source, "УЧЕБНЫЙ ПРИМЕР", r.ConfirmedBy})
			row++
		}
	} else if row < 4 {
		for _, r := range teachingAssessmentHistoryRows(selectedObjectKey, time.Now()) {
			if row >= 4 {
				break
			}
			listAdd(assessDecisionHistoryList, row, []string{r.ConfirmedAt, r.Direction, r.FinalClass, r.Source, "УЧЕБНЫЙ ПРИМЕР", r.ConfirmedBy})
			row++
		}
	}
}

func teachingAssessmentOverviewHistoryRows() []AssessmentDecisionRecord {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		return nil
	}
	rows := make([]AssessmentDecisionRecord, 0, 64)
	for _, well := range corpus.Wells {
		for _, row := range teachingAssessmentHistoryRows(well.ObjectKey, time.Now()) {
			row.Source = fmt.Sprintf("Учебная скв. %02d · %s / № %s", well.Ordinal, well.Field, well.Well)
			rows = append(rows, row)
		}
	}
	return rows
}

func setAssessmentTabDetail(text string) {
	if assessTabDetail == 0 {
		return
	}
	display := strings.ReplaceAll(normalizeLaboratoryLineBreaks(text), "\n", "\r\n")
	setText(assessTabDetail, display)
	pSendMessageW.Call(uintptr(assessTabDetail), WM_SETFONT, uintptr(hFontPanel), 1)
	plain := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, CrTextColor: uint32(rgb(43, 46, 50))}
	pSendMessageW.Call(uintptr(assessTabDetail), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(assessTabDetail), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&plain)))
	bold := CHARFORMATW{CbSize: uint32(unsafe.Sizeof(CHARFORMATW{})), DwMask: CFM_BOLD | CFM_COLOR, DwEffects: CFE_BOLD, CrTextColor: uint32(rgb(15, 48, 82))}
	offset := 0
	for _, part := range strings.SplitAfter(strings.ReplaceAll(display, "\r\n", "\n"), "\n") {
		line := strings.TrimSuffix(part, "\n")
		trimmed := strings.TrimSpace(line)
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		length := 0
		upper := strings.ToUpper(trimmed)
		if trimmed != "" && upper == trimmed && laboratoryUTF16Len(trimmed) <= 120 {
			length = laboratoryUTF16Len(trimmed)
		} else if colon := strings.Index(trimmed, ":"); colon > 0 && colon < 52 {
			length = laboratoryUTF16Len(trimmed[:colon+1])
		}
		if length > 0 {
			start := offset + laboratoryUTF16Len(line[:leading])
			pSendMessageW.Call(uintptr(assessTabDetail), EM_SETSEL, uintptr(start), uintptr(start+length))
			pSendMessageW.Call(uintptr(assessTabDetail), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&bold)))
		}
		offset += laboratoryUTF16Len(part)
	}
	pSendMessageW.Call(uintptr(assessTabDetail), EM_SETSEL, 0, 0)
}

func assessmentClassKey(objectKey, direction string) string {
	return strings.TrimSpace(objectKey) + "|" + strings.TrimSpace(direction)
}

func ensureAssessmentClassStore() {
	if assessmentClassStoreLoaded {
		return
	}
	assessmentClassStoreLoaded = true
	var rows []AssessmentClassRecord
	loadJSON(assessmentClassStoragePath(), &rows)
	for _, r := range rows {
		if strings.TrimSpace(r.ObjectKey) == "" || strings.TrimSpace(r.Direction) == "" {
			continue
		}
		assessmentClassStore[assessmentClassKey(r.ObjectKey, r.Direction)] = r
	}
}

func saveAssessmentClassStore() error {
	ensureAssessmentClassStore()
	rows := make([]AssessmentClassRecord, 0, len(assessmentClassStore))
	for _, r := range assessmentClassStore {
		rows = append(rows, r)
	}
	return saveJSON(assessmentClassStoragePath(), rows)
}

func assessmentRecord(objectKey string, d assessmentClassDirection) AssessmentClassRecord {
	ensureAssessmentClassStore()
	k := assessmentClassKey(objectKey, d.Name)
	r := assessmentClassStore[k]
	if r.ObjectKey == "" {
		if teaching, ok := teachingAssessmentRecord(objectKey, d); ok {
			return teaching
		}
		r.ObjectKey = objectKey
		r.Direction = d.Name
		r.Prefix = d.Prefix
	}
	return r
}

func assessmentSetRecord(r AssessmentClassRecord) error {
	ensureAssessmentClassStore()
	r.UpdatedAt = time.Now().Format(time.RFC3339)
	assessmentClassStore[assessmentClassKey(r.ObjectKey, r.Direction)] = r
	// Classification is an upstream dependency of the Methods page.  Any edit
	// must invalidate a prepared/background method snapshot immediately;
	// otherwise section 4 can briefly reuse candidates from the previous class.
	invalidateMethodsBackground()
	return saveAssessmentClassStore()
}

func assessmentClassOptions(d assessmentClassDirection) []string {
	out := []string{"— не задано —", "Не относится", "НД — недостаточно данных"}
	out = append(out, d.Levels...)
	return out
}

func setAssessmentClassCombo(combo syscall.Handle, d assessmentClassDirection, selected string) {
	if combo == 0 {
		return
	}
	pSendMessageW.Call(uintptr(combo), CB_RESETCONTENT, 0, 0)
	opts := assessmentClassOptions(d)
	sel := 0
	for i, v := range opts {
		pSendMessageW.Call(uintptr(combo), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(p(v))))
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(selected)) {
			sel = i
		}
	}
	pSendMessageW.Call(uintptr(combo), CB_SETCURSEL, uintptr(sel), 0)
}

func assessmentComboValue(combo syscall.Handle) string {
	if combo == 0 {
		return ""
	}
	idx, _, _ := pSendMessageW.Call(uintptr(combo), CB_GETCURSEL, 0, 0)
	if int32(idx) < 0 {
		return ""
	}
	var buf [256]uint16
	pSendMessageW.Call(uintptr(combo), CB_GETLBTEXT, idx, uintptr(unsafe.Pointer(&buf[0])))
	v := strings.TrimSpace(syscall.UTF16ToString(buf[:]))
	if strings.HasPrefix(v, "—") {
		return ""
	}
	if strings.HasPrefix(v, "НД") {
		return "НД"
	}
	return v
}

func assessmentLatestLab(direction assessmentClassDirection) *LabRecord {
	return latestLabRecordForAssessment(selectedObjectKey, direction.LabGroup)
}

func assessmentProgramSource(direction assessmentClassDirection) string {
	rec := assessmentLatestLab(direction)
	if rec == nil {
		return "нет лабораторных данных"
	}
	source := strings.TrimSpace(rec.Source)
	if source == "" {
		source = "сохранённая лабораторная диагностика"
	}
	if date := strings.TrimSpace(rec.Date); date != "" {
		source += " · " + date
	}
	return source
}

func assessmentDirectionByName(name string) (assessmentClassDirection, bool) {
	for _, d := range assessmentClassDirections {
		if strings.EqualFold(strings.TrimSpace(d.Name), strings.TrimSpace(name)) {
			return d, true
		}
	}
	return assessmentClassDirection{}, false
}

func assessmentProgramClass(direction assessmentClassDirection) (class, confidence, reason string) {
	if scenario, full := teachingFullScenarioForObject(selectedObjectKey); full {
		if selected := scenario.ClassByDirection[direction.Name]; selected != "" {
			return selected, "учебная", "Синтетический сквозной сценарий: " + scenario.Name
		}
		return "Не относится", "учебная", "Осложнение не относится к данному учебному сценарию."
	}
	rec := assessmentLatestLab(direction)
	return assessmentProgramClassFromRecord(direction, rec)
}

func assessmentProgramClassFromRecord(direction assessmentClassDirection, rec *LabRecord) (class, confidence, reason string) {
	if rec == nil {
		return "НД", "нет данных", "Лабораторные данные по этому направлению не внесены."
	}
	values := rec.Values
	prefix := direction.Prefix
	summary := laboratoryResultSummary{}
	if g, ok := assessmentGroupByCode(direction.LabGroup); ok {
		summary = laboratoryGroupSummary(g, values, rec.ScaleModel)
	}
	switch direction.Prefix {
	case "К":
		if v, ok := parseLabNumber(values["corrosion_rate"]); ok {
			switch {
			case v > 1.0:
				return "К3", "высокая", "Фактическая скорость коррозии >1,0 мм/год соответствует критерию К3 базы классификации РОСА."
			case v > 0.5:
				return "К2", "высокая", "Фактическая скорость коррозии >0,5 мм/год соответствует критерию К2 базы классификации РОСА."
			case v >= 0.1:
				return "К1", "высокая", "Фактическая скорость коррозии 0,1–0,5 мм/год соответствует критерию К1 базы классификации РОСА."
			default:
				return "Не рекомендуется", "средняя", "По скорости коррозии критерий К1 не достигнут; другие коррозионные механизмы этим не исключаются."
			}
		}
	case "В":
		if v, ok := parseLabNumber(values["viscosity"]); ok {
			if v > 200 {
				return "В3", "средняя", "Вязкость >200 мПа·с соответствует лабораторной части критерия В3; подтверждение смолистости/эксплуатационных проявлений требуется отдельно."
			}
			if v >= 100 {
				return "В2", "средняя", "Вязкость 100–200 мПа·с соответствует лабораторной части критерия В2; подтвердите значение при рабочей температуре и эксплуатационные проявления."
			}
			if v >= 50 {
				return "В1", "средняя", "Вязкость 50–100 мПа·с соответствует лабораторной части критерия В1."
			}
			return "Не рекомендуется", "средняя", "Вязкость ниже 50 мПа·с; класс по высокой вязкости только по этому показателю не предлагается."
		}
	case "С":
		if v, ok := parseLabNumber(values["external_saturation_index"]); ok {
			if v > 1.0 {
				return "С2", "средняя", "Индекс/коэффициент насыщения >1,0 соответствует лабораторной части критерия С2. С3 требует фактического короткого МРП/повторных проявлений."
			}
			if v >= 0 {
				return "С1", "средняя", "Неотрицательный индекс насыщения указывает на термодинамическую предпосылку солеотложения и соответствует предварительному С1."
			}
			return "Не рекомендуется", "средняя", "Индекс насыщения ниже 0; термодинамическая предпосылка по этому расчёту не выявлена."
		}
		if summary.Level > 0 {
			lvl := summary.Level
			if lvl > 2 {
				lvl = 2
			}
			return fmt.Sprintf("%s%d", prefix, lvl), "низкая", "Есть лабораторные признаки солеотложения, но прямой индекс насыщения по протоколу отсутствует; класс предварительный и требует проверки."
		}
	case "М":
		if v, ok := parseLabNumber(values["suspended"]); ok {
			switch {
			case v > 500:
				return "М3", "средняя", "Взвешенные вещества >500 мг/л соответствуют количественной части критерия М3; твёрдость частиц и история ТРС требуют проверки."
			case v >= 200:
				return "М2", "средняя", "Взвешенные вещества 200–500 мг/л соответствуют количественной части критерия М2."
			case v >= 50:
				return "М1", "средняя", "Взвешенные вещества 50–200 мг/л соответствуют количественной части критерия М1."
			default:
				return "Не рекомендуется", "средняя", "Взвешенные вещества ниже 50 мг/л; количественный порог М1 по имеющимся данным не достигнут."
			}
		}
	case "Гд":
		ht, okH := parseLabNumber(values["hydrate_temp"])
		candidates := []string{"sample_temp", "pump_temp", "wellhead_temp", "profile_temp_min"}
		minT := 0.0
		haveT := false
		for _, k := range candidates {
			if v, ok := parseLabNumber(values[k]); ok && (!haveT || v < minT) {
				minT = v
				haveT = true
			}
		}
		if okH && haveT && minT <= ht {
			return "Гд1", "средняя", fmt.Sprintf("Минимальная доступная температура %.1f °C не выше расчётной температуры гидратообразования %.1f °C. Это предварительный признак Гд1, требующий проверки давления.", minT, ht)
		}
	case "Т":
		for _, k := range []string{"pump_temp", "profile_temp_max", "formation_temp"} {
			if v, ok := parseLabNumber(values[k]); ok {
				switch {
				case v > 160:
					return "Т3", "средняя", fmt.Sprintf("Температура %.1f °C выше 160 °C — количественный критерий Т3.", v)
				case v >= 120:
					return "Т2", "средняя", fmt.Sprintf("Температура %.1f °C находится в диапазоне 120–160 °C — критерий Т2.", v)
				case v >= 100:
					return "Т1", "средняя", fmt.Sprintf("Температура %.1f °C находится в диапазоне 100–120 °C — критерий Т1.", v)
				}
			}
		}
	case "А":
		triggered := 0
		var evidence []string
		for _, rule := range []struct {
			key, label string
			limit      float64
		}{{"paraffins", "парафины", 3}, {"asphaltenes", "асфальтены", 1}, {"pour_point", "температура застывания", 15}, {"viscosity", "вязкость", 50}} {
			if v, ok := parseLabNumber(values[rule.key]); ok && v > rule.limit {
				triggered++
				evidence = append(evidence, fmt.Sprintf("%s %.3g > %.3g", rule.label, v, rule.limit))
			}
		}
		if triggered >= 3 {
			return "А2", "средняя", "Одновременно выявлено не менее трёх лабораторных предпосылок АСПО: " + strings.Join(evidence, "; ") + ". А3 требует подтверждённых эксплуатационных/отказных проявлений."
		}
		if triggered > 0 {
			return "А1", "средняя", "Выявлены лабораторные предпосылки АСПО: " + strings.Join(evidence, "; ") + ". Для повышения класса нужны дополнительные признаки и фактические проявления."
		}
	case "Г":
		return "НД", "нет данных", "Класс Г1–Г3 в Технологическом модуле определяется долей свободного газа на приёме насосной установки. Лабораторный газовый фактор не заменяет этот показатель."
	}
	if summary.HasData && summary.Level > 0 && prefix != "Гд" && prefix != "Т" && prefix != "Г" {
		lvl := summary.Level
		if lvl > 3 {
			lvl = 3
		}
		return fmt.Sprintf("%s%d", prefix, lvl), "низкая", "Есть лабораторные признаки, но прямой критерий класса в базе классификации РОСА заполнен не полностью."
	}
	return "НД", "нет данных", "Данных недостаточно для предположения класса по логике классификации РОСА."
}

func assessmentComparisonStatus(tech, program string) string {
	tech = strings.TrimSpace(tech)
	program = strings.TrimSpace(program)
	if program == "" || program == "НД" || program == "Не рекомендуется" {
		return "— НЕТ НОВОЙ РЕКОМЕНДАЦИИ"
	}
	if tech == "" || tech == "НД" || tech == "Не относится" {
		return "⚠ РАЗНОГЛАСИЕ: НЕТ КЛАССА ТЕХНОЛОГА"
	}
	if strings.EqualFold(tech, program) {
		return "✓ СОВПАДАЕТ"
	}
	return "⚠ РАЗНОГЛАСИЕ КЛАССОВ"
}

func assessmentFinalSuggestion(r AssessmentClassRecord, program string) string {
	if strings.TrimSpace(r.FinalClass) != "" {
		return r.FinalClass
	}
	if strings.TrimSpace(r.TechnologistClass) != "" {
		return r.TechnologistClass
	}
	if program != "НД" && program != "Не рекомендуется" {
		return program
	}
	return ""
}

func assessmentTechnologistRows() [][]string {
	rows := make([][]string, 0, len(assessmentClassDirections))
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		status := "не заполнено"
		if r.TechnologistClass != "" {
			status = "задано технологом"
		}
		rows = append(rows, []string{d.Name, d.Prefix, dash(r.TechnologistClass), status, dash(r.TechnologistComment)})
	}
	return rows
}

func assessmentProgramRows() [][]string {
	rows := make([][]string, 0, len(assessmentClassDirections))
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		c, conf, reason := assessmentProgramClass(d)
		source := assessmentProgramSource(d)
		if r.RestoredFromArchive && strings.TrimSpace(r.ProgramClassAtDecision) != "" {
			c = r.ProgramClassAtDecision
			conf = "архив"
			reason = r.ProgramReasonAtDecision
			if strings.TrimSpace(r.ProgramSourceAtDecision) != "" {
				source = r.ProgramSourceAtDecision
			}
		} else if c == "НД" && assessmentLatestLab(d) != nil {
			source += " · данных недостаточно для класса"
		}
		rows = append(rows, []string{d.Name, c, conf, source, reason})
	}
	return rows
}

func assessmentCompareRows() [][]string {
	rows := make([][]string, 0, len(assessmentClassDirections))
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		prog, _, _ := assessmentProgramClass(d)
		if r.RestoredFromArchive && strings.TrimSpace(r.ProgramClassAtDecision) != "" {
			prog = r.ProgramClassAtDecision
		}
		status := assessmentComparisonStatus(r.TechnologistClass, prog)
		final := assessmentFinalSuggestion(r, prog)
		decision := dash(final)
		if strings.TrimSpace(r.DecisionMode) != "" {
			decision += " · " + r.DecisionMode
			if r.FullClassificationConfirmed {
				decision += " · УТВЕРЖДЕНО"
			} else {
				decision += " · ОЖИДАЕТ ОБЩЕГО УТВЕРЖДЕНИЯ"
			}
		}
		rows = append(rows, []string{d.Name, dash(r.TechnologistClass), dash(prog), status, decision})
	}
	return rows
}

func refreshAssessmentClassTabs() {
	if assessTechList == 0 {
		return
	}
	// RC562: calculate each table once, before Win32 starts painting cells.
	// CustomDraw must be a pure presentation path; recomputing rows there caused
	// repeated laboratory storage reads and UI hangs.
	techRows := assessmentTechnologistRows()
	programRows := assessmentProgramRows()
	compareRows := assessmentCompareRows()
	assessmentCompareRowsCache = compareRows

	fill := func(list syscall.Handle, rows [][]string) {
		if list == 0 {
			return
		}
		listBeginUpdate(list)
		defer listEndUpdate(list)
		listClear(list)
		for i, row := range rows {
			listAdd(list, i, row)
		}
	}
	fill(assessTechList, techRows)
	fill(assessProgramList, programRows)
	fill(assessCompareList, compareRows)

	if len(assessmentClassDirections) > 0 {
		if assessTechSelected >= len(assessmentClassDirections) {
			assessTechSelected = 0
		}
		if assessLabSelected >= len(assessmentClassDirections) {
			assessLabSelected = 0
		}
		if assessCompareSelected >= len(assessmentClassDirections) {
			assessCompareSelected = 0
		}
		updateAssessmentTabEditor()
	}
	refreshAssessmentDecisionHistory()
}

func updateAssessmentTabEditor() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		setAssessmentTabDetail("Сначала выберите объект. Классификация хранится отдельно для каждого объекта.")
		return
	}
	switch assessInnerTab {
	case 0:
		d := assessmentClassDirections[assessTechSelected]
		r := assessmentRecord(selectedObjectKey, d)
		pEnableWindow.Call(uintptr(assessClassCombo), 1)
		pEnableWindow.Call(uintptr(assessClassComment), 1)
		setAssessmentClassCombo(assessClassCombo, d, r.TechnologistClass)
		setText(assessClassComment, r.TechnologistComment)
		setAssessmentTabDetail(fmt.Sprintf("%s (%s): технолог вручную фиксирует класс по утверждённой логике классификации РОСА. Выбор технолога хранится отдельно от программного предположения и не перезаписывается лабораторией.", strings.ToUpper(d.Name), d.Prefix))
	case 1:
		d := assessmentClassDirections[assessLabSelected]
		r := assessmentRecord(selectedObjectKey, d)
		c, conf, reason := assessmentProgramClass(d)
		source := assessmentProgramSource(d)
		note := "Класс рассчитан автоматически по текущей сохранённой лабораторной диагностике. На этой вкладке пользователь ничего не назначает."
		if r.RestoredFromArchive && strings.TrimSpace(r.ProgramClassAtDecision) != "" {
			c = r.ProgramClassAtDecision
			conf = "архив"
			reason = r.ProgramReasonAtDecision
			if strings.TrimSpace(r.ProgramSourceAtDecision) != "" {
				source = r.ProgramSourceAtDecision
			}
			note = "Показано состояние программного расчёта из восстановленной архивной классификации."
		}
		if !r.RestoredFromArchive && !assessmentHasAnyLaboratoryData() {
			note = "Лабораторных данных по объекту нет. Нажмите «Пропустить лабораторию», затем в третьей вкладке примите классификацию технолога."
		}
		setAssessmentTabDetail(fmt.Sprintf("ПРЕДПОЛОЖЕНИЕ ПРОГРАММЫ: %s\r\nУверенность: %s\r\nИсточник: %s\r\n\r\nОБОСНОВАНИЕ\r\n%s\r\n\r\nЧТО НУЖНО ДЛЯ ПОДТВЕРЖДЕНИЯ\r\n%s\r\n\r\n%s", c, conf, source, reason, assessmentVerificationPlan(d), note))
	case 2:
		d := assessmentClassDirections[assessCompareSelected]
		r := assessmentRecord(selectedObjectKey, d)
		if !r.RestoredFromArchive && !assessmentHasAnyLaboratoryData() {
			setAssessmentTabDetail(fmt.Sprintf("%s\r\n\r\nЛабораторных данных по объекту нет. Класс технолога: %s.\r\n\r\nПосле проверки первой вкладки нажмите «Принять классы технолога». В итоговом отчёте будет отдельно отмечено, что классификация утверждена без лабораторного подтверждения.", strings.ToUpper(d.Name), dash(r.TechnologistClass)))
			return
		}
		// Keep the existing methods workflow linked to the direction selected in
		// the reconciliation table. This does not auto-select a method.
		for i := range assessDirectionRows {
			if assessDirectionRows[i].Group.Code == d.LabGroup {
				assessSelectedRow = i
				populateAssessmentMethodCombo(assessmentMethodGroupForDirection(d))
				break
			}
		}
		prog, conf, reason := assessmentProgramClass(d)
		if r.RestoredFromArchive && strings.TrimSpace(r.ProgramClassAtDecision) != "" {
			prog = r.ProgramClassAtDecision
			conf = "архив"
			reason = r.ProgramReasonAtDecision
		}
		status := assessmentComparisonStatus(r.TechnologistClass, prog)
		setAssessmentClassCombo(assessFinalClassCombo, d, assessmentFinalSuggestion(r, prog))
		setText(assessFinalComment, r.FinalComment)
		advice := assessmentReconciliationAdvice(d, r.TechnologistClass, prog, conf)
		setAssessmentTabDetail(fmt.Sprintf("%s — %s\r\n\r\nКласс технолога: %s\r\nПредположение программы: %s (%s)\r\n\r\nПОЧЕМУ ПРОГРАММА ТАК СЧИТАЕТ\r\n%s\r\n\r\nСОВЕТ\r\n%s\r\n\r\nЧТО ИССЛЕДОВАТЬ ДЛЯ ПОДТВЕРЖДЕНИЯ\r\n%s\r\n\r\nГРУППОВОЙ ВЫБОР: отметьте галочками нужные осложнения в таблице. «ПРИНЯТЬ РЕКЛАССИФИКАЦИЮ» применит предложение программы ко всем отмеченным строкам, а «РЕШЕНИЕ ТЕХНОЛОГА» — класс технолога. Без галочек действие применяется к выделенной строке. Эти кнопки НЕ утверждают классификацию. После выбора решений по всем направлениям обязательно нажмите «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ». При отклонении от предложения программы комментарий обязателен (не менее 10 знаков).", strings.ToUpper(d.Name), status, dash(r.TechnologistClass), dash(prog), conf, reason, advice, assessmentVerificationPlan(d)))
	}
}

func saveAssessmentTabValue() error {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сначала выберите объект")
	}
	idx := assessTechSelected
	if idx < 0 || idx >= len(assessmentClassDirections) {
		return fmt.Errorf("выберите направление")
	}
	d := assessmentClassDirections[idx]
	r := assessmentRecord(selectedObjectKey, d)
	value := assessmentComboValue(assessClassCombo)
	comment := strings.TrimSpace(getText(assessClassComment))
	changed := !strings.EqualFold(strings.TrimSpace(r.TechnologistClass), strings.TrimSpace(value))
	r.TechnologistClass = value
	r.TechnologistComment = comment
	if changed {
		r.FinalClass, r.FinalComment, r.DecisionMode = "", "", ""
		r.ProgramClassAtDecision, r.ProgramReasonAtDecision, r.ProgramSourceAtDecision = "", "", ""
		r.RestoredFromArchive, r.RestoredDecisionID = false, ""
		r.ConfirmedAt, r.ConfirmedBy = "", ""
		// Full approval belongs to the complete classification snapshot, not to
		// an individual row.  Editing even one technologist class makes the old
		// snapshot obsolete until it is confirmed again.
		r.FullClassificationConfirmed = false
	}
	if err := assessmentSetRecord(r); err != nil {
		return err
	}
	refreshAssessmentClassTabs()
	return nil
}

func skipAssessmentLaboratoryReview() error {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сначала выберите объект")
	}
	if assessmentHasAnyLaboratoryData() {
		return fmt.Errorf("лабораторные данные уже есть; просмотрите рекомендацию программы и выполните сверку")
	}
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		r.LabReviewSkipped = true
		if err := assessmentSetRecord(r); err != nil {
			return err
		}
	}
	switchAssessmentInnerTab(2)
	return nil
}

func assessmentFinalClassForDecision(mode, technologistClass, programClass string) (string, error) {
	modeNorm := strings.ToLower(strings.TrimSpace(mode))
	tech := strings.TrimSpace(technologistClass)
	program := strings.TrimSpace(programClass)
	switch {
	case strings.Contains(modeNorm, "без корректировки"):
		if tech == "" || strings.EqualFold(tech, "НД") {
			return "", fmt.Errorf("для решения технолога сначала задайте класс в первой вкладке")
		}
		return tech, nil
	case strings.Contains(modeNorm, "с корректировкой"):
		// The reclassification button accepts exactly the program proposal shown
		// in the comparison table. It must never reuse a previous final/tech class.
		if program == "" || strings.EqualFold(program, "НД") || strings.EqualFold(program, "Не рекомендуется") {
			return "", fmt.Errorf("программа не сформировала класс, который можно принять как реклассификацию")
		}
		return program, nil
	default:
		return "", fmt.Errorf("неизвестный режим решения классификации")
	}
}

func assessmentDecisionRequiresComment(mode, technologistClass, programClass string) bool {
	modeNorm := strings.ToLower(strings.TrimSpace(mode))
	if !strings.Contains(modeNorm, "без корректировки") {
		return false
	}
	tech := strings.TrimSpace(technologistClass)
	program := strings.TrimSpace(programClass)
	if program == "" || strings.EqualFold(program, "НД") || strings.EqualFold(program, "Не рекомендуется") {
		return false
	}
	return !strings.EqualFold(tech, program)
}

func assessmentCheckedCompareDirections() []int {
	checked := make([]int, 0, len(assessmentClassDirections))
	if assessCompareList != 0 {
		for i := range assessmentClassDirections {
			state, _, _ := pSendMessageW.Call(uintptr(assessCompareList), LVM_GETITEMSTATE, uintptr(i), LVIS_STATEIMAGEMASK)
			if ((state >> 12) & 0xF) == 2 { // state-image index 2 = checked
				checked = append(checked, i)
			}
		}
	}
	// Preserve the old single-row workflow: without explicit checkmarks the two
	// decision buttons act on the currently selected complication only.
	if len(checked) == 0 && assessCompareSelected >= 0 && assessCompareSelected < len(assessmentClassDirections) {
		checked = append(checked, assessCompareSelected)
	}
	return checked
}

func confirmAssessmentFinalClasses(mode string, indices []int) (int, error) {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return 0, fmt.Errorf("сначала выберите объект")
	}
	if len(indices) == 0 {
		return 0, fmt.Errorf("отметьте галочками хотя бы одно осложнение или выделите одну строку")
	}
	comment := strings.TrimSpace(getText(assessFinalComment))
	pending := make([]AssessmentClassRecord, 0, len(indices))
	seen := map[int]bool{}
	for _, index := range indices {
		if index < 0 || index >= len(assessmentClassDirections) || seen[index] {
			continue
		}
		seen[index] = true
		d := assessmentClassDirections[index]
		r := assessmentRecord(selectedObjectKey, d)
		program, _, reason := assessmentProgramClass(d)
		value, err := assessmentFinalClassForDecision(mode, r.TechnologistClass, program)
		if err != nil {
			return 0, fmt.Errorf("осложнение «%s»: %w", d.Name, err)
		}
		if assessmentDecisionRequiresComment(mode, r.TechnologistClass, program) && len([]rune(comment)) < 10 {
			return 0, fmt.Errorf("для осложнения «%s» при отклонении от реклассификации укажите причину не менее чем из 10 знаков", d.Name)
		}
		r.FinalClass = value
		r.FullClassificationConfirmed = false
		r.FinalComment = comment
		r.DecisionMode = mode
		r.ProgramClassAtDecision = program
		r.ProgramReasonAtDecision = reason
		r.ProgramSourceAtDecision = assessmentProgramSource(d)
		r.RestoredFromArchive = false
		r.RestoredDecisionID = ""
		r.ConfirmedAt = ""
		r.ConfirmedBy = ""
		pending = append(pending, r)
	}
	if len(pending) == 0 {
		return 0, fmt.Errorf("не выбрано ни одного осложнения")
	}

	// Commit the batch as one classification-store write. Validation above is
	// completed for every checked row first, so an invalid row cannot leave a
	// half-applied group operation behind.
	ensureAssessmentClassStore()
	old := make(map[string]AssessmentClassRecord, len(pending))
	existed := make(map[string]bool, len(pending))
	now := time.Now().Format(time.RFC3339)
	for i := range pending {
		key := assessmentClassKey(pending[i].ObjectKey, pending[i].Direction)
		old[key], existed[key] = assessmentClassStore[key]
		pending[i].UpdatedAt = now
		assessmentClassStore[key] = pending[i]
	}
	invalidateMethodsBackground()
	if err := saveAssessmentClassStore(); err != nil {
		for _, r := range pending {
			key := assessmentClassKey(r.ObjectKey, r.Direction)
			if existed[key] {
				assessmentClassStore[key] = old[key]
			} else {
				delete(assessmentClassStore, key)
			}
		}
		return 0, err
	}
	refreshAssessmentClassTabs()
	return len(pending), nil
}

func confirmAssessmentFinalClass(mode string) error {
	_, err := confirmAssessmentFinalClasses(mode, []int{assessCompareSelected})
	return err
}

func confirmAssessmentFullClassification() error {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сначала выберите объект")
	}

	// RC631: the row buttons select the decision source only. The final approval
	// resolves that source against the values currently visible in the comparison
	// table, then freezes one coherent snapshot. This also repairs pending choices
	// saved by older RC builds where FinalClass could lag behind the selected mode.
	pending := make([]AssessmentClassRecord, 0, len(assessmentClassDirections))
	classificationChanged := false
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		if !r.FullClassificationConfirmed {
			classificationChanged = true
		}
		if strings.TrimSpace(r.DecisionMode) == "" {
			return fmt.Errorf("сначала примите решение по осложнению «%s»", d.Name)
		}
		program, _, reason := assessmentProgramClass(d)
		resolved, err := assessmentFinalClassForDecision(r.DecisionMode, r.TechnologistClass, program)
		if err != nil {
			return fmt.Errorf("осложнение «%s»: %w", d.Name, err)
		}
		if assessmentDecisionRequiresComment(r.DecisionMode, r.TechnologistClass, program) && len([]rune(strings.TrimSpace(r.FinalComment))) < 10 {
			return fmt.Errorf("для осложнения «%s» при отклонении от реклассификации укажите причину не менее чем из 10 знаков", d.Name)
		}
		r.FinalClass = resolved
		r.ProgramClassAtDecision = program
		r.ProgramReasonAtDecision = reason
		r.ProgramSourceAtDecision = assessmentProgramSource(d)
		r.FullClassificationConfirmed = false
		r.ConfirmedAt = ""
		r.ConfirmedBy = ""
		pending = append(pending, r)
	}

	approvedAt := time.Now().Format("02.01.2006 15:04")
	approvedBy := ""
	if currentSession != nil {
		approvedBy = strings.TrimSpace(currentSession.DisplayName)
	}
	for _, r := range pending {
		r.FullClassificationConfirmed = true
		r.ConfirmedAt = approvedAt
		r.ConfirmedBy = approvedBy
		if err := assessmentSetRecord(r); err != nil {
			return err
		}
		if err := appendAssessmentDecisionHistory(r); err != nil {
			return err
		}
	}
	if classificationChanged {
		// A method draft belongs to the classification snapshot from which it was
		// built. Never carry an old draft into a newly confirmed set of classes.
		clearCombinedMethods(strings.TrimSpace(selectedObjectKey))
		methodWorkflowSuggestions = nil
		methodsResumeSelectionName = ""
		methodsCandidatePage = 0
		invalidateMethodsBackground()
	}
	refreshAssessmentClassTabs()
	return nil
}
func acceptTechnologistClassificationWithoutLab() error {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сначала выберите объект")
	}
	if assessmentHasAnyLaboratoryData() {
		return fmt.Errorf("по объекту уже есть лабораторные данные; выполните сверку в третьей вкладке")
	}
	if !assessmentLaboratoryReviewSkipped() {
		return fmt.Errorf("сначала откройте вторую вкладку и нажмите «Пропустить лабораторию»")
	}
	meaningful := false
	for _, d := range assessmentClassDirections {
		v := strings.TrimSpace(assessmentRecord(selectedObjectKey, d).TechnologistClass)
		if v != "" && !strings.EqualFold(v, "НД") && !strings.EqualFold(v, "Не относится") {
			meaningful = true
			break
		}
	}
	if !meaningful {
		return fmt.Errorf("в первой вкладке не задан ни один профильный класс технолога")
	}
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		finalClass := strings.TrimSpace(r.TechnologistClass)
		if finalClass == "" {
			// An omitted direction is explicitly stored as insufficient data.  This
			// prevents an old S3/G3/etc. decision from surviving a newer profile.
			finalClass = "НД"
		}
		r.FinalClass = finalClass
		r.FinalComment = r.TechnologistComment
		r.DecisionMode = "принято без лаборатории"
		r.ProgramClassAtDecision = "НД"
		r.ProgramReasonAtDecision = "Лабораторные данные отсутствовали; принят актуальный класс технолога, незаданные направления зафиксированы как НД."
		r.ProgramSourceAtDecision = "Лабораторные данные отсутствовали"
		r.RestoredFromArchive = false
		r.RestoredDecisionID = ""
		r.FullClassificationConfirmed = false
		// This shortcut only selects the technologist values for all directions.
		// The complete classification still requires the explicit full-approval button.
		r.ConfirmedAt = ""
		r.ConfirmedBy = ""
		if err := assessmentSetRecord(r); err != nil {
			return err
		}
	}
	refreshAssessmentClassTabs()
	return nil
}

func assessmentLaboratoryReviewSkipped() bool {
	if len(assessmentClassDirections) == 0 || strings.TrimSpace(selectedObjectKey) == "" {
		return false
	}
	return assessmentRecord(selectedObjectKey, assessmentClassDirections[0]).LabReviewSkipped
}

func assessmentDecisionSnapshotAt(objectKey, confirmedAt string) map[string]AssessmentDecisionRecord {
	ensureAssessmentDecisionHistory()
	result := map[string]AssessmentDecisionRecord{}
	latest := map[string]time.Time{}
	target, targetErr := time.Parse("02.01.2006 15:04", strings.TrimSpace(confirmedAt))
	for _, row := range assessmentDecisionStore {
		if !laboratoryObjectKeysEquivalent(row.ObjectKey, objectKey) {
			continue
		}
		rowTime, rowErr := time.Parse("02.01.2006 15:04", strings.TrimSpace(row.ConfirmedAt))
		if targetErr == nil && rowErr == nil {
			if rowTime.After(target) {
				continue
			}
			if prev, ok := latest[row.Direction]; ok && rowTime.Before(prev) {
				continue
			}
			latest[row.Direction] = rowTime
			result[row.Direction] = row
			continue
		}
		if strings.TrimSpace(row.ConfirmedAt) == strings.TrimSpace(confirmedAt) {
			result[row.Direction] = row
		}
	}
	return result
}

func restoreAssessmentDecisionSnapshot(historyRow int) error {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return fmt.Errorf("сначала закрепите объект")
	}
	rows := assessmentDecisionsForObject(selectedObjectKey)
	if historyRow < 0 || historyRow >= len(rows) {
		return fmt.Errorf("архивная запись не найдена")
	}
	selected := rows[historyRow]
	when := strings.TrimSpace(selected.ConfirmedAt)
	if when == "" {
		when = "без даты"
	}
	warning := fmt.Sprintf("Текущая классификация по закреплённому объекту будет полностью очищена и заменена состоянием архивного расчёта на %s.\n\nВсе классы, комментарии и принятые решения на этой вкладке будут перезаписаны. Продолжить?", when)
	if !askYesNo("Восстановить архивную классификацию?", warning) {
		return nil
	}
	snapshot := assessmentDecisionSnapshotAt(selectedObjectKey, selected.ConfirmedAt)
	if len(snapshot) == 0 {
		return fmt.Errorf("для выбранной архивной даты не найдено данных для восстановления")
	}
	ensureAssessmentClassStore()
	for _, d := range assessmentClassDirections {
		r := AssessmentClassRecord{ObjectKey: selectedObjectKey, Direction: d.Name, Prefix: d.Prefix}
		if old, ok := snapshot[d.Name]; ok {
			r.TechnologistClass = old.TechnologistClass
			r.FinalClass = old.FinalClass
			r.FinalComment = old.Comment
			r.DecisionMode = old.DecisionMode
			r.ProgramClassAtDecision = old.ProgramClass
			r.ProgramReasonAtDecision = old.Reason
			r.ProgramSourceAtDecision = old.Source
			r.ConfirmedBy = old.ConfirmedBy
			r.ConfirmedAt = old.ConfirmedAt
			r.FullClassificationConfirmed = true
			r.LabReviewSkipped = strings.Contains(strings.ToLower(old.DecisionMode), "без лаборатории")
			r.RestoredFromArchive = true
			r.RestoredDecisionID = old.DecisionID
		}
		r.UpdatedAt = time.Now().Format(time.RFC3339)
		assessmentClassStore[assessmentClassKey(selectedObjectKey, d.Name)] = r
	}
	if err := saveAssessmentClassStore(); err != nil {
		return err
	}
	refreshAssessmentClassTabs()
	setAssessmentTabDetail(fmt.Sprintf("Архивная классификация на %s восстановлена в рабочую форму. Текущие значения были заменены архивными; при новом подтверждении будет создана новая запись истории.", when))
	return nil
}

func assessmentDecisionHistoryCustomDraw(lParam uintptr) uintptr {
	if lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		if draw.SubItem == 4 { // Статус
			draw.ClrText = uint32(rgb(28, 112, 45))
			draw.ClrTextBk = uint32(rgb(232, 247, 233))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func assessmentCompareCustomDraw(lParam uintptr) uintptr {
	if lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		row := int(draw.NMCD.ItemSpec)
		if draw.SubItem == 0 {
			draw.ClrText = uint32(rgb(15, 48, 82))
			draw.ClrTextBk = uint32(rgb(221, 233, 244))
			return CDRF_NEWFONT
		}
		rows := assessmentCompareRowsCache
		if row >= 0 && row < len(rows) && len(rows[row]) >= 4 {
			status := rows[row][3]
			switch {
			case strings.Contains(status, "РАЗНОГЛАСИЕ"):
				draw.ClrText = uint32(rgb(120, 36, 18))
				draw.ClrTextBk = uint32(rgb(255, 232, 222))
			case strings.Contains(status, "СОВПАДАЕТ"):
				draw.ClrText = uint32(rgb(20, 92, 48))
				draw.ClrTextBk = uint32(rgb(226, 246, 232))
			default:
				draw.ClrText = uint32(rgb(82, 94, 109))
				draw.ClrTextBk = uint32(rgb(238, 243, 248))
			}
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func assessmentDirectionCustomDraw(lParam uintptr) uintptr {
	if lParam == 0 {
		return CDRF_DODEFAULT
	}
	draw := (*NMLVCUSTOMDRAW)(unsafe.Pointer(lParam))
	switch draw.NMCD.DrawStage {
	case CDDS_PREPAINT:
		return CDRF_NOTIFYITEMDRAW
	case CDDS_ITEMPREPAINT:
		return CDRF_NOTIFYSUBITEMDRAW
	case CDDS_ITEMPREPAINT | CDDS_SUBITEM:
		if draw.SubItem == 0 {
			draw.ClrText = uint32(rgb(15, 48, 82))
			draw.ClrTextBk = uint32(rgb(221, 233, 244))
			return CDRF_NEWFONT
		}
	}
	return CDRF_DODEFAULT
}

func switchAssessmentInnerTab(tab int) {
	started := time.Now()
	defer reportSlowFrame("assessment_inner_tab", ID_ASSESS, started, 150*time.Millisecond)
	if tab < 0 || tab > 2 {
		tab = 0
	}
	if currentPage == ID_ASSESS {
		beginPageVisualUpdate(ID_ASSESS)
		defer endPageSubviewUpdate(ID_ASSESS)
	}
	assessInnerTab = tab
	setPageSubview(ID_ASSESS, []int{pageSubviewAssessmentTech, pageSubviewAssessmentLab, pageSubviewAssessmentCompare}[tab])
	for _, h := range []syscall.Handle{assessTabTechButton, assessTabLabButton, assessTabCompareButton} {
		if h != 0 {
			pInvalidateRect.Call(uintptr(h), 0, 1)
		}
	}
	// Data tables are refreshed when classification/measurement inputs change
	// and on section activation. Clicking the three presentation tabs must not
	// erase/reinsert all rows of *all* three ListViews synchronously.
	updateAssessmentTabEditor()
	layoutAssessmentInnerTabs()
}

func showAssessmentInnerControls() {
	tech := assessInnerTab == 0
	lab := assessInnerTab == 1
	compare := assessInnerTab == 2
	show(assessTechList, tech)
	show(assessProgramList, lab)
	show(assessCompareList, compare)
	hasLab := assessmentHasAnyLaboratoryData()
	noLabCompare := compare && !hasLab
	show(assessClassCombo, tech)
	show(assessClassComment, tech)
	show(assessClassSaveButton, tech)
	show(assessAcceptTechButton, (lab && !hasLab) || noLabCompare)
	if lab {
		setText(assessAcceptTechButton, "ПРОПУСТИТЬ ЛАБОРАТОРИЮ")
	} else if noLabCompare {
		setText(assessAcceptTechButton, "ПРИНЯТЬ КЛАССЫ ТЕХНОЛОГА")
	}
	show(assessClassCaption, tech)
	show(assessFinalClassCombo, false)
	show(assessFinalComment, compare && hasLab)
	show(assessFinalCommentCaption, compare && hasLab)
	show(assessConfirmButton, false)
	show(assessConfirmKeepButton, compare && hasLab)
	show(assessConfirmCorrectButton, compare && hasLab)
	show(assessConfirmFullButton, compare && hasLab)
	show(assessFinalCaption, false)
	show(assessMethodsLabel, false)
	show(assessMethodsCombo, false)
	show(assessMethodInfo, false)
	show(assessToActivitiesButton, false)
	show(assessList, false)
	show(assessDetail, false)
}

func assessmentResizeColumns(width int32) {
	// Leave room for the vertical scrollbar, checkbox state image and native
	// ListView borders. Using width-20 forced a pointless horizontal scrollbar.
	usable := max32(620, width-44)
	apply := func(list syscall.Handle, percentages []int32) {
		used := int32(0)
		for i, percentage := range percentages {
			columnWidth := usable * percentage / 100
			if i == len(percentages)-1 {
				columnWidth = usable - used
			}
			setListColumnWidth(list, i, max32(80, columnWidth))
			used += columnWidth
		}
	}
	apply(assessTechList, []int32{20, 8, 15, 17, 40})
	apply(assessProgramList, []int32{19, 14, 12, 20, 35})
	apply(assessCompareList, []int32{20, 12, 12, 28, 28})
	apply(assessDecisionHistoryList, []int32{14, 17, 10, 35, 11, 13})
}

func assessmentObjectCaption(st TechObjectState, fallback string) string {
	field := strings.TrimSpace(st.Field)
	well := strings.TrimSpace(st.Well)
	horizon := strings.TrimSpace(st.ObjectPlast)
	if horizon == "" {
		horizon = strings.TrimSpace(st.Deposit)
	}
	if well != "" {
		lower := strings.ToLower(well)
		for _, prefix := range []string{"скважина", "скв.", "скв"} {
			if strings.HasPrefix(lower, prefix) {
				well = strings.TrimSpace(well[len(prefix):])
				break
			}
		}
	}
	parts := make([]string, 0, 3)
	if field != "" {
		parts = append(parts, field)
	}
	if well != "" {
		parts = append(parts, "скв. "+well)
	}
	if horizon != "" {
		parts = append(parts, horizon)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " / ")
	}
	return strings.TrimSpace(readableObjectKey(fallback))
}

func layoutAssessmentInnerTabs() {
	beginLayoutBatch()
	defer endLayoutBatch()
	// RC574 hard guard: this function changes visibility as well as geometry.
	// Never call it as an off-page data refresh, otherwise assessment controls
	// can appear on top of Laboratory after cross-page object re-pinning.
	if currentPage != ID_ASSESS {
		return
	}
	if assessTabTechButton == 0 {
		return
	}
	var cr RECT
	pGetClientRect.Call(uintptr(hMain), uintptr(unsafe.Pointer(&cr)))
	w, h := cr.Right-cr.Left, cr.Bottom-cr.Top
	contentW := max32(720, w-270)
	right := int32(242) + contentW

	// RC560: compact the complete classification workspace vertically instead of
	// hiding the lower block. The approved-history table is a permanent part of
	// all three inner tabs and receives its space first.
	// RC577: tabs start below the 50 px object/action band.  RC576 used py(158),
	// which overlapped that band by 8 px and caused the red action button, object
	// warning and tab captions to paint over one another.
	tabY := py(190)
	gap := int32(8)
	tabH := int32(44)
	tabW := (contentW - 2*gap) / 3
	setPos(assessTabTechButton, 242, tabY, tabW, tabH)
	setPos(assessTabLabButton, 242+tabW+gap, tabY, tabW, tabH)
	setPos(assessTabCompareButton, 242+2*(tabW+gap), tabY, right-(242+2*(tabW+gap)), tabH)

	overallY := tabY + tabH + 8
	overallH := int32(52)
	setPos(assessOverall, 242, overallY, contentW, overallH)
	listY := overallY + overallH + 6

	// Reserve the approved-history region from the bottom upward on every
	// supported resolution. It must not disappear at 1280x729/1366x768.
	historyVisible := true
	historyListH := max32(64, min32(112, h/9))
	if h < 820 {
		historyListH = 56
	}
	historyLabelH := int32(22)
	historyBottomPad := int32(12)
	historyY := h - historyBottomPad - historyListH - historyLabelH - 4
	setPos(assessDecisionHistoryLabel, 242, historyY, contentW, historyLabelH)
	setPos(assessDecisionHistoryList, 242, historyY+historyLabelH+4, contentW, historyListH)

	// The main grid is deliberately shorter than before. Most rows are only a few
	// lines high; a large empty list rectangle added no information and displaced
	// confirmation/history controls below the screen.
	detailAndActions := int32(120)
	if assessInnerTab == 2 && assessmentHasAnyLaboratoryData() {
		detailAndActions = 230
		if h < 820 {
			detailAndActions = 178
		}
	}
	desiredListH := int32(26 + min32(int32(len(assessmentClassDirections)), 7)*24)
	listH := min32(194, desiredListH)
	maxListH := historyY - listY - detailAndActions
	if maxListH < listH {
		minListH := int32(104)
		if h < 820 {
			minListH = 64
		}
		listH = max32(minListH, maxListH)
	}
	setPos(assessTechList, 242, listY, contentW, listH)
	setPos(assessProgramList, 242, listY, contentW, listH)
	setPos(assessCompareList, 242, listY, contentW, listH)
	assessmentResizeColumns(contentW)

	editorY := listY + listH + 8
	if assessInnerTab == 0 {
		// Technologist entry: one compact row, then the explanation uses all space
		// down to the fixed approved-history block.
		setPos(assessClassCaption, 242, editorY+5, 170, 22)
		setPos(assessClassCombo, 416, editorY, 210, 220)
		setPos(assessClassComment, 638, editorY, max32(170, right-638-176), 32)
		setPos(assessClassSaveButton, right-166, editorY, 166, 34)
		detailY := editorY + 42
		setPos(assessTabDetail, 242, detailY, contentW, max32(72, historyY-detailY-8))
	} else if assessInnerTab == 1 {
		// Laboratory proposal: keep the action above the explanation and leave the
		// full lower strip to earlier approved passes.
		buttonY := editorY
		setPos(assessAcceptTechButton, right-238, buttonY, 238, 34)
		detailY := buttonY + 42
		setPos(assessTabDetail, 242, detailY, contentW, max32(82, historyY-detailY-8))
	} else if !assessmentHasAnyLaboratoryData() {
		// The no-laboratory fallback has one action only. It must not inherit the
		// full reconciliation geometry and create a large empty rectangle.
		buttonH := int32(48)
		buttonW := min32(330, max32(270, contentW/3))
		setPos(assessAcceptTechButton, right-buttonW, editorY, buttonW, buttonH)
		detailY := editorY + buttonH + 8
		setPos(assessTabDetail, 242, detailY, contentW, max32(54, historyY-detailY-8))
		setPos(assessFinalCommentCaption, 0, 0, 0, 0)
		setPos(assessFinalComment, 0, 0, 0, 0)
		setPos(assessConfirmKeepButton, 0, 0, 0, 0)
		setPos(assessConfirmCorrectButton, 0, 0, 0, 0)
		setPos(assessConfirmFullButton, 0, 0, 0, 0)
	} else {
		// Reconciliation: row-level choice and final approval are intentionally
		// separated. Keep the final approval on its own full-width row so it never
		// gets clipped at Windows display scaling or on a narrower client area.
		captionH := int32(22)
		commentH := int32(42)
		choiceH := int32(40)
		approveH := int32(42)
		controlGap := int32(8)
		rowGap := int32(7)
		if h < 820 {
			captionH = 18
			commentH = 34
			choiceH = 38
			approveH = 42
			controlGap = 4
			rowGap = 4
		}
		setPos(assessFinalCommentCaption, 242, editorY, contentW, captionH)
		setPos(assessFinalComment, 242, editorY+captionH+2, contentW, commentH)
		setPos(assessFinalClassCombo, 0, 0, 0, 0)
		choicesY := editorY + captionH + commentH + controlGap
		buttonGap := int32(10)
		choiceW := (contentW - buttonGap) / 2
		setPos(assessConfirmKeepButton, 242, choicesY, choiceW, choiceH)
		setPos(assessConfirmCorrectButton, 242+choiceW+buttonGap, choicesY, right-(242+choiceW+buttonGap), choiceH)
		approveY := choicesY + choiceH + rowGap
		setPos(assessConfirmFullButton, 242, approveY, contentW, approveH)
		setPos(assessAcceptTechButton, 0, 0, 0, 0)
		detailY := approveY + approveH + rowGap
		detailH := historyY - detailY - 8
		if detailH < 20 {
			detailH = 20
		}
		setPos(assessTabDetail, 242, detailY, contentW, detailH)

		// Legacy method controls are not part of this inner view; keep them outside
		// the visible area rather than allowing stale coordinates to overlap history.
		setPos(assessMethodsLabel, 0, 0, 0, 0)
		setPos(assessToActivitiesButton, 0, 0, 0, 0)
		setPos(assessMethodsCombo, 0, 0, 0, 0)
		setPos(assessMethodInfo, 0, 0, 0, 0)
	}
	showAssessmentInnerControls()
	show(assessDecisionHistoryLabel, historyVisible)
	show(assessDecisionHistoryList, historyVisible)
}

func refreshAssessmentOverallClassificationSummary() {
	if strings.TrimSpace(selectedObjectKey) == "" {
		return
	}
	conflict, match, missing := 0, 0, 0
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(selectedObjectKey, d)
		prog, _, _ := assessmentProgramClass(d)
		switch assessmentComparisonStatus(r.TechnologistClass, prog) {
		case "✓ СОВПАДАЕТ":
			match++
		case "⚠ РАЗНОГЛАСИЕ КЛАССОВ", "⚠ РАЗНОГЛАСИЕ: НЕТ КЛАССА ТЕХНОЛОГА":
			conflict++
		default:
			missing++
		}
	}
	setText(assessOverall, fmt.Sprintf("Сверка: совпадает — %d; требует решения — %d; недостаточно данных — %d. Галочки — групповой выбор.\r\nИтог переносится в «Методы» только после нажатия «УТВЕРДИТЬ ПОЛНУЮ КЛАССИФИКАЦИЮ».", match, conflict, missing))
}

func assessmentFinalReportText(objectKey string) string {
	if strings.TrimSpace(objectKey) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("КЛАССИФИКАЦИЯ ОСЛОЖНЕНИЙ — ПОДТВЕРЖДЕНИЕ ТЕХНОЛОГА\r\n")
	written := 0
	for _, d := range assessmentClassDirections {
		r := assessmentRecord(objectKey, d)
		if strings.TrimSpace(r.FinalClass) == "" {
			continue
		}
		program := r.ProgramClassAtDecision
		if program == "" {
			program, _, _ = assessmentProgramClass(d)
		}
		marker := "✓"
		if r.DecisionMode == "с корректировкой" || (!strings.EqualFold(r.TechnologistClass, program) && program != "" && program != "НД" && program != "Не рекомендуется") {
			marker = "⚠"
		}
		if r.FullClassificationConfirmed {
			fmt.Fprintf(&b, "%s %s: утверждённый итог %s; технолог %s; программа %s; решение — %s.\r\n", marker, d.Name, dash(r.FinalClass), dash(r.TechnologistClass), dash(program), dash(r.DecisionMode))
		} else {
			fmt.Fprintf(&b, "… %s: выбранный вариант %s; технолог %s; программа %s; решение — %s; ОЖИДАЕТ ОБЩЕГО УТВЕРЖДЕНИЯ.\r\n", d.Name, dash(r.FinalClass), dash(r.TechnologistClass), dash(program), dash(r.DecisionMode))
		}
		if strings.TrimSpace(r.FinalComment) != "" {
			fmt.Fprintf(&b, "  Комментарий: %s\r\n", r.FinalComment)
		}
		if r.FullClassificationConfirmed {
			fmt.Fprintf(&b, "  Утвердил: %s; %s.\r\n", dash(r.ConfirmedBy), dash(r.ConfirmedAt))
		}
		written++
	}
	if written == 0 {
		b.WriteString("⚠ Итоговая классификация ещё не подтверждена.\r\n")
	}
	return b.String()
}
