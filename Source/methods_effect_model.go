//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MethodEffectAssessment is a transparent engineering screening estimate.
// Applicability is NOT a probability of success. Reliability describes how
// trustworthy the object-specific estimate is. EffectProbability is shown only
// when ROSA has enough comparable factual applications with an explicitly
// registered observed effect.
type MethodEffectAssessment struct {
	Method               string
	Direction            string
	Classification       string
	Applicability        int
	Reliability          int
	ReliabilityLevel     string
	Confidence           string // retained for compatibility with saved/exported RC568 structures
	HistoryMatches       int
	HistoryApprovedMatch int
	CauseRiskShare       float64
	ProbabilityAvailable bool
	EffectProbability    int
	ProbabilityLow       int
	ProbabilityHigh      int
	ProbabilitySample    int
	ProbabilitySuccesses int
	ProbabilityObjects   int
	LowPct               float64
	BasePct              float64
	HighPct              float64
	EffectQuantified     bool
	EffectBasisType      string
	Evidence             string
	Reasons              []string
	Warnings             []string
}

type CombinedMethodForecast struct {
	Methods       []MethodEffectAssessment
	TargetDays    float64
	BaselineLow   float64
	BaselineBase  float64
	BaselineHigh  float64
	EffectLowPct  float64
	EffectBasePct float64
	EffectHighPct float64
	ForecastLow   float64
	ForecastBase  float64
	ForecastHigh  float64
	Interaction   string
	History       MethodHistoryBasis
	Source        string
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func methodEffectReferenceBand(name string) (low, base, high float64, evidence string, quantified bool, basis string) {
	n := normalizeMethodText(name)
	switch {
	case strings.Contains(n, "мониторинг") || strings.Contains(n, "контроль остаточ") || strings.Contains(n, "контроль эффективности"):
		return 0, 0, 0, "мониторинг повышает управляемость и достоверность программы, но сам по себе не является самостоятельным источником прироста МРП", false, "контроль качества и обратная связь; эффект не начисляется отдельно"
	case strings.Contains(n, "депресс") || strings.Contains(n, "плавн") && strings.Contains(n, "режим"):
		return 0, 0, 0, "режимное управление может уменьшать вынос мехпримесей и число неустойчивых режимов, но без объектовой калибровки не переводится в универсальный процент МРП", false, "эксплуатационное управление риском; качественный эффект"
	case strings.Contains(n, "медленно высвобож") || strings.Contains(n, "slow release"):
		return 20, 50, 100, "ингибирование солеотложений пролонгированного действия: опубликованные полевые кейсы показывают многомесячное предотвращение отложений и кратное продление отдельных циклов; внешний ориентир, не норматив", true, "прямые полевые данные по наработке до отказа / времени без солеотложений"
	case strings.Contains(n, "ингибитор") && (strings.Contains(n, "соле") || strings.Contains(n, "соли") || strings.Contains(n, "комплексон")):
		return 10, 35, 100, "ингибирование солеотложений: полевые кейсы УЭЦН показывают от умеренного до кратного роста наработки при подтверждённой солевой причине; переносить цифры между объектами без калибровки нельзя", true, "прямые полевые данные по наработке до отказа"
	case strings.Contains(n, "ингибитор") && strings.Contains(n, "корроз"):
		return 10, 30, 70, "ингибиторная защита: полевые кейсы показывают снижение коррозионных отказов и рост наработки до отказа, но эффект зависит от химии среды, непрерывности подачи и остаточной защитной концентрации", true, "полевые данные по отказам / наработке до отказа + механизм коррозионной защиты"
	case strings.Contains(n, "покрыт") || strings.Contains(n, "эпокс") || strings.Contains(n, "гидрофоб"):
		return 0, 0, 0, "покрытия имеют положительный промысловый опыт при коррозии/отложениях, однако в доступных комплексных кейсах индивидуальный вклад покрытия часто не отделён от химии и конструкции оборудования", false, "качественный/комплексный полевой эффект; индивидуальный вклад не отделён"
	case strings.Contains(n, "аспо") || strings.Contains(n, "парафин"):
		if strings.Contains(n, "ингиб") || strings.Contains(n, "депресс") {
			return 10, 30, 80, "химическая профилактика АСПО/асфальтенов: опубликованы полевые программы с существенным увеличением наработки до отказа; эффект требует лабораторного подбора, корректной точки ввода и подтверждения доминирующей причины отказа", true, "полевые данные по наработке до отказа для химических программ"
		}
		return 0, 0, 0, "тепловые, механические и растворяющие операции способны восстановить проходимость и увеличить межочистной период, но универсальный перевод операции в процент МРП не подтверждён", false, "технологический эффект без универсального коэффициента МРП"
	case strings.Contains(n, "растворител") || strings.Contains(n, "кислот") || strings.Contains(n, "промыв"):
		return 0, 0, 0, "восстановительное мероприятие снимает уже сформированное ограничение; устойчивость результата определяется последующей профилактикой и фактическим межоперационным периодом", false, "восстановление режима; процент МРП только после объектовой калибровки"
	case strings.Contains(n, "биоцид") || strings.Contains(n, "бактер"):
		return 0, 0, 0, "биоцидная программа может снижать микробиологическую активность и коррозию, но опубликованные показатели коррозии нельзя автоматически переводить в процент МРП", false, "косвенный эффект через микробиологически обусловленную коррозию / коррозию"
	case strings.Contains(n, "мехприм") || strings.Contains(n, "песк") || strings.Contains(n, "фильтр") || strings.Contains(n, "износостой"):
		return 0, 0, 0, "для пескозащиты и износостойких насосов есть полевые кейсы с многократным ростом наработки до отказа, но это оборудование/компоновка и должно проверяться в «Автотехнологе», а не начисляться РОСА автоматически", false, "прямые полевые данные, но решение относится к оборудованию/компоновке"
	case strings.Contains(n, "газосеп") || strings.Contains(n, "дисперг") || strings.Contains(n, "мультифаз") || strings.Contains(n, "газ handler"):
		return 0, 0, 0, "средства работы со свободным газом имеют сильные полевые данные по росту наработки УЭЦН до отказа, но подбор конкретной компоновки относится к «Автотехнологу»", false, "прямые полевые данные, но решение относится к оборудованию/компоновке"
	case strings.Contains(n, "деэмульг") || strings.Contains(n, "вязкост") || strings.Contains(n, "эмульс"):
		return 0, 0, 0, "снижение вязкости/разрушение эмульсии улучшает гидравлику и нагрузку оборудования, но без объектовой статистики нельзя приравнивать процент снижения вязкости или роста добычи к проценту МРП", false, "косвенный технологический эффект"
	case strings.Contains(n, "гидрат") || strings.Contains(n, "метанол") || strings.Contains(n, "гликол") || strings.Contains(n, "антиаглом"):
		return 0, 0, 0, "гидратные ингибиторы и тепловые меры предотвращают/устраняют гидратные инциденты; доступные кейсы подтверждают повышение устойчивости / коэффициента технической готовности, но не универсальный процент МРП", false, "эффект на риск гидратного инцидента / коэффициент технической готовности"
	case strings.Contains(n, "термостой") || strings.Contains(n, "высокотемп"):
		return 0, 0, 0, "высокотемпературные исполнения повышают надёжность в термических условиях, однако подбор оборудования относится к «Автотехнологу»", false, "оборудование/компоновка"
	default:
		return 0, 0, 0, "для метода нет достаточной прямой доказательной базы, позволяющей автоматически переводить применимость в процент прироста МРП", false, "качественная инженерная оценка"
	}
}

func failureTextForMethodMatch(e FailureEvent) string {
	return normalizeMethodText(strings.Join([]string{
		e.StopCause, e.SourceClass, e.UserClass, e.NormalizedClass, e.ComplicationFactor,
		e.PrimaryComplication, e.AssociatedComplication1, e.AssociatedComplication2, e.RefinedComplication,
		e.SourceCause, e.NormalizedCause, e.PreliminaryConclusion, e.ApprovedConclusion,
		e.CommissionConclusion, e.Comment,
	}, " "))
}

func failureMatchesMethodDirection(e FailureEvent, direction string) bool {
	t := failureTextForMethodMatch(e)
	d := normalizeMethodText(direction)
	hasAny := func(values ...string) bool {
		for _, v := range values {
			if strings.Contains(t, normalizeMethodText(v)) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.Contains(d, "сол"):
		return hasAny("солеотлож", "соли", "кальцит", "барит", "гипс", "карбонат", "сульфат")
	case strings.Contains(d, "корроз"):
		return hasAny("корроз", "co2", "h2s", "сквозн", "питтинг", "скрн", "вир")
	case strings.Contains(d, "аспо") || strings.Contains(d, "парафин"):
		return hasAny("аспо", "парафин", "смол", "асфальтен")
	case strings.Contains(d, "мех"):
		return hasAny("мехприм", "песок", "абразив", "эроз", "засор", "проппант")
	case strings.Contains(d, "микроб") || strings.Contains(d, "бактер"):
		return hasAny("бактер", "микроб", "свб", "биокорроз")
	case strings.Contains(d, "газ") && !strings.Contains(d, "гидрат"):
		return hasAny("свободн газ", "газосодерж", "газ lock", "газлок", "срыв подачи", "влияние газа")
	case strings.Contains(d, "гидрат"):
		return hasAny("гидрат", "гидратообраз")
	case strings.Contains(d, "вяз") || strings.Contains(d, "эмуль"):
		return hasAny("вязк", "эмуль", "перегруз", "ток")
	}
	return false
}

func methodEvidenceScore(s methodWorkflowSuggestion) int {
	src := strings.ToLower(strings.TrimSpace(s.Method.Source))
	kind := strings.ToLower(strings.TrimSpace(s.Method.SourceKind))
	switch {
	case strings.Contains(src, "технологичес") || strings.Contains(src, "лукойл") || strings.Contains(kind, "действующ") || strings.Contains(kind, "производствен"):
		return 90
	case strings.Contains(src, "spe") || strings.Contains(kind, "научн") || strings.Contains(kind, "technical paper"):
		return 82
	case strings.Contains(src, "ampp") || strings.Contains(kind, "стандарт") || strings.Contains(kind, "отраслев"):
		return 78
	case strings.Contains(src, "slb") || strings.Contains(src, "baker") || strings.Contains(src, "weatherford") || strings.Contains(src, "champion"):
		return 68
	default:
		return 55
	}
}

func historyScoreForMethod(h MethodHistoryBasis) int {
	cycle := 0
	switch {
	case h.CycleCount >= 5:
		cycle = 100
	case h.CycleCount == 4:
		cycle = 90
	case h.CycleCount == 3:
		cycle = 78
	case h.CycleCount == 2:
		cycle = 60
	case h.CycleCount == 1:
		cycle = 40
	default:
		cycle = 15
	}
	fail := 0
	switch {
	case h.FailureCount >= 5:
		fail = 100
	case h.FailureCount >= 3:
		fail = 80
	case h.FailureCount == 2:
		fail = 60
	case h.FailureCount == 1:
		fail = 40
	default:
		fail = 15
	}
	return int(math.Round(0.65*float64(cycle) + 0.35*float64(fail)))
}

func methodReliabilityLevel(v int) string {
	switch {
	case v >= 80:
		return "высокая"
	case v >= 65:
		return "хорошая"
	case v >= 50:
		return "умеренная"
	case v >= 35:
		return "ограниченная"
	default:
		return "низкая"
	}
}

func methodFailureEvidence(rows []FailureEvent, direction string) (matches, approved int, share float64) {
	if len(rows) == 0 {
		return 0, 0, 0
	}
	for _, e := range rows {
		if !failureMatchesMethodDirection(e, direction) {
			continue
		}
		matches++
		if e.ConclusionApproved || strings.TrimSpace(e.ApprovedConclusion) != "" || strings.TrimSpace(e.CommissionConclusion) != "" {
			approved++
		}
	}
	share = float64(matches) / float64(len(rows))
	return matches, approved, clampFloat(share, 0, 1)
}

func methodTextMatchesDirection(text, direction string) bool {
	return methodTextMatchesNormalizedDirection(normalizeMethodText(text), normalizeMethodText(direction))
}

func methodTextMatchesNormalizedDirection(t, d string) bool {
	hasAny := func(values ...string) bool {
		for _, v := range values {
			if strings.Contains(t, v) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.Contains(d, "сол"):
		return hasAny("солеотлож", "соли", "кальцит", "барит", "гипс", "карбонат", "сульфат")
	case strings.Contains(d, "корроз"):
		return hasAny("корроз", "co2", "h2s", "скрн", "вир")
	case strings.Contains(d, "аспо") || strings.Contains(d, "парафин"):
		return hasAny("аспо", "парафин", "смол", "асфальтен")
	case strings.Contains(d, "мех"):
		return hasAny("мехприм", "песок", "абразив", "эроз", "засор", "проппант")
	case strings.Contains(d, "микроб") || strings.Contains(d, "бактер"):
		return hasAny("бактер", "микроб", "свб", "биокорроз")
	case strings.Contains(d, "газ") && !strings.Contains(d, "гидрат"):
		return hasAny("свободн газ", "газосодерж", "газлок", "газ lock", "срыв подачи", "влияние газа")
	case strings.Contains(d, "гидрат"):
		return hasAny("гидрат", "гидратообраз")
	case strings.Contains(d, "вяз") || strings.Contains(d, "эмуль"):
		return hasAny("вязк", "эмуль")
	}
	return d == ""
}

func methodNamesComparable(method, recorded string) bool {
	return methodNamesComparableNormalized(normalizeMethodText(method), normalizeMethodText(recorded))
}

func methodNamesComparableNormalized(mn, vm string) bool {
	if mn == "" || vm == "" {
		return false
	}
	if strings.Contains(vm, mn) || strings.Contains(mn, vm) {
		return true
	}
	// Fallback only on a distinctive first token; short generic words are not
	// sufficient to pool different technologies into one empirical sample.
	parts := strings.Fields(mn)
	return len(parts) > 0 && len([]rune(parts[0])) >= 5 && strings.Contains(vm, parts[0])
}

func methodProbabilityObservation(method string, v VersionRecord, o Outcome) (known, success bool) {
	if !methodNamesComparable(method, v.Method) {
		return false, false
	}
	return methodProbabilityOutcomeObservation(o)
}

func methodProbabilityOutcomeObservation(o Outcome) (known, success bool) {
	// A "probability of confirmed effect" must not be trained on records where
	// causal attribution was never evaluated. Temporal association is useful in
	// the archive, but is not enough for this percentage.
	attribution := normalizeMethodText(o.EffectAttribution)
	if !strings.Contains(attribution, normalizeMethodText("Подтверждено специалистом")) {
		return false, false
	}
	effect := normalizeMethodText(o.ObservedEffect)
	switch {
	case strings.Contains(effect, normalizeMethodText("Наблюдается улучшение")):
		return true, true
	case strings.Contains(effect, normalizeMethodText("Без заметного изменения")),
		strings.Contains(effect, normalizeMethodText("Наблюдается ухудшение")):
		return true, false
	case strings.Contains(effect, normalizeMethodText("Смешанный результат")):
		return false, false
	}
	// Legacy outcome rows may contain the explicit conclusion in Result.
	result := normalizeMethodText(o.Result)
	for _, n := range []string{"нет эффекта", "не подтверж", "отриц", "ухудш", "без эффекта", "неэффект"} {
		if strings.Contains(result, normalizeMethodText(n)) {
			return true, false
		}
	}
	for _, pos := range []string{"эффект подтвержден", "эффект подтверждён", "положительный эффект", "успешно", "рост наработки"} {
		if strings.Contains(result, normalizeMethodText(pos)) {
			return true, true
		}
	}
	return false, false
}

func methodObjectEquipment(objectKey string) string {
	key := strings.TrimSpace(objectKey)
	for _, st := range techCurrent {
		if strings.EqualFold(strings.TrimSpace(st.ObjectKey), key) {
			return normalizeTargetLiftType(st.Equipment)
		}
	}
	if st, ok := teachingWorkflowStateForObject(key); ok {
		return normalizeTargetLiftType(st.Equipment)
	}
	return ""
}

func empiricalMethodEffectProbability(method, direction, objectKey string) (available bool, probability, low, high, sample, successes, objects int) {
	return empiricalMethodProbabilityFromSnapshot(method, direction, methodObjectEquipment(objectKey), versions, indexMethodOutcomes(outcomes))
}

// Immutable data only: also used by the Methods worker.
func empiricalMethodProbabilityFromSnapshot(method, direction, targetLift string, records []VersionRecord, outcomeIndex map[string]Outcome) (available bool, probability, low, high, sample, successes, objects int) {
	return empiricalMethodProbabilityFromCohort(method, direction, targetLift, prepareMethodProbabilityCohort(records, outcomeIndex))
}

type methodProbabilityCohortRow struct {
	Method, Complication, Lift, Object string
	Success                            bool
}

func prepareMethodProbabilityCohort(records []VersionRecord, outcomeIndex map[string]Outcome) []methodProbabilityCohortRow {
	rows := make([]methodProbabilityCohortRow, 0, len(records))
	for _, v := range records {
		o, exists := outcomeIndex[v.VersionID]
		if !exists {
			continue
		}
		known, success := methodProbabilityOutcomeObservation(o)
		if !known {
			continue
		}
		rows = append(rows, methodProbabilityCohortRow{normalizeMethodText(v.Method), normalizeMethodText(v.Complication), normalizeTargetLiftType(v.Equipment), strings.ToLower(strings.TrimSpace(v.ObjectKey)), success})
	}
	return rows
}

func empiricalMethodProbabilityFromCohort(method, direction, targetLift string, rows []methodProbabilityCohortRow) (available bool, probability, low, high, sample, successes, objects int) {
	mn, dn := normalizeMethodText(method), normalizeMethodText(direction)
	seenObjects := map[string]struct{}{}
	for _, row := range rows {
		if !methodNamesComparableNormalized(mn, row.Method) {
			continue
		}
		if dn != "" && !methodTextMatchesNormalizedDirection(row.Complication, dn) {
			continue
		}
		if targetLift != "" && (row.Lift == "" || row.Lift != targetLift) {
			continue
		}
		sample++
		if row.Success {
			successes++
		}
		if row.Object != "" {
			seenObjects[row.Object] = struct{}{}
		}
	}

	objects = len(seenObjects)
	// Both event count and object diversity are required. Five cycles from one
	// well are useful history, but they are not a sufficiently transferable
	// empirical probability for another well.
	if sample < 5 || objects < 3 {
		return false, 0, 0, 0, sample, successes, objects
	}
	// Beta(1,1) posterior. The point estimate is the posterior mean. The
	// interval is an intentionally conservative normal approximation of the
	// posterior uncertainty so a small cohort cannot look artificially precise.
	alpha := float64(successes + 1)
	beta := float64(sample - successes + 1)
	mean := alpha / (alpha + beta)
	variance := alpha * beta / ((alpha + beta) * (alpha + beta) * (alpha + beta + 1))
	sd := math.Sqrt(variance)
	lo := clampFloat(mean-1.96*sd, 0, 1)
	hi := clampFloat(mean+1.96*sd, 0, 1)
	return true, int(math.Round(mean * 100)), int(math.Round(lo * 100)), int(math.Round(hi * 100)), sample, successes, objects
}

func calculateMethodApplicability(s methodWorkflowSuggestion, objectKey string, now time.Time) MethodEffectAssessment {
	h := methodHistoryBasisForObject(objectKey, now)
	rows := historyForWell(objectKey, now)
	p := methodProbabilityResult{}
	p.Available, p.Value, p.Low, p.High, p.Sample, p.Successes, p.Objects = empiricalMethodEffectProbability(s.Method.Name, s.Direction, objectKey)
	return calculateMethodApplicabilityFromSnapshot(s, objectKey, h, rows, p)
}

func calculateMethodApplicabilityFromSnapshot(s methodWorkflowSuggestion, objectKey string, h MethodHistoryBasis, rows []FailureEvent, p methodProbabilityResult) MethodEffectAssessment {
	matches, approved, causeShare := methodFailureEvidence(rows, s.Direction)
	confirmedCauseShare := 0.0
	if len(rows) > 0 {
		confirmedCauseShare = float64(approved) / float64(len(rows))
	}

	classFit := 100.0 // suggestion exists only for an approved direction/class
	severityNeed := map[int]float64{1: 45, 2: 70, 3: 92}[clampInt(s.Severity, 1, 3)]
	historyFit := 50.0 // neutral when there is no history; reliability carries the uncertainty
	if len(rows) > 0 {
		historyFit = clampFloat(causeShare*100, 5, 100)
	}
	evidenceFit := float64(methodEvidenceScore(s))
	feasibility := 78.0
	if assessmentMethodNeedsLabSelection(s.Method.Name) {
		feasibility = 58
	}
	if assessmentMethodIsEquipmentDesign(s.Method.Name) {
		feasibility = 50
	}
	applicability := int(math.Round(0.35*classFit + 0.20*severityNeed + 0.25*historyFit + 0.10*evidenceFit + 0.10*feasibility))
	if len(rows) == 0 {
		applicability = minInt(applicability, 80)
	}
	applicability = clampInt(applicability, 25, 95)

	historyQuality := historyScoreForMethod(h)
	causeCertainty := 25.0
	if matches > 0 {
		causeCertainty = 55 + 45*float64(approved)/float64(matches)
	} else if len(rows) > 0 {
		causeCertainty = 20
	}
	classificationQuality := 100.0
	verificationQuality := 75.0
	if assessmentMethodNeedsLabSelection(s.Method.Name) {
		verificationQuality = 50
	}
	reliability := int(math.Round(0.30*float64(historyQuality) + 0.25*causeCertainty + 0.20*classificationQuality + 0.15*evidenceFit + 0.10*verificationQuality))
	if len(rows) == 0 {
		reliability = minInt(reliability, 58)
	}
	if assessmentMethodNeedsLabSelection(s.Method.Name) {
		reliability = minInt(reliability, 76)
	}
	reliability = clampInt(reliability, 20, 95)

	reasons := []string{
		fmt.Sprintf("утверждённый класс %s (%s)", dash(s.Classification), dash(s.Direction)),
		fmt.Sprintf("доля отказов 36 мес., совпадающих с направлением: %.0f%% (%d из %d)", causeShare*100, matches, len(rows)),
		fmt.Sprintf("качество доказательной базы источника: %d/100", int(evidenceFit)),
	}
	warnings := []string{}
	if len(rows) == 0 {
		warnings = append(warnings, "нет предыстории отказов за 36 месяцев — достоверность ограничена")
	}
	if len(rows) > 0 && matches == 0 {
		warnings = append(warnings, "в отказах нет явного подтверждения этой причины; численный вклад в МРП не начисляется")
	}
	if matches > 0 && approved < matches {
		warnings = append(warnings, "часть совпадающих отказов не имеет утверждённого заключения комиссии/технолога")
	}
	if assessmentMethodNeedsLabSelection(s.Method.Name) {
		warnings = append(warnings, "до лабораторного подбора/совместимости достоверность ограничена")
	}

	refLow, refBase, refHigh, evidence, quantified, basis := methodEffectReferenceBand(s.Method.Name)
	low, base, high := 0.0, 0.0, 0.0
	if quantified && len(rows) > 0 && approved > 0 {
		dataFactor := clampFloat(float64(reliability)/100.0, 0.2, 0.95)
		suitabilityFactor := clampFloat(float64(applicability)/100.0, 0.25, 0.95)
		// A preliminary/free-text match may justify a warning or an investigation,
		// but only a cause approved by the commission/technologist may add a
		// numerical MRP contribution.
		causeFactor := clampFloat(confirmedCauseShare, 0, 1)
		low = refLow * suitabilityFactor * dataFactor * causeFactor
		base = refBase * suitabilityFactor * dataFactor * causeFactor
		high = refHigh * suitabilityFactor * dataFactor * causeFactor
		low = clampFloat(low, 0, 35)
		base = clampFloat(base, low, 60)
		high = clampFloat(high, base, 90)
	} else if quantified {
		warnings = append(warnings, "внешний потенциал метода показан только в справочнике; без совпадающей истории он не переносится в прогноз МРП")
	}

	// Lesson coefficients stay confined to explicitly named built-in objects.
	// Never treat synthetic effects as observed industrial evidence/probability.
	if _, teaching := teachingWorkflowOrdinal(objectKey); teaching {
		if lesson, ok := teachingMethodEffect(s.Method.Name); ok {
			low, base, high = lesson.LowPct, lesson.BasePct, lesson.HighPct
			quantified = true
			evidence, basis = lesson.Evidence, lesson.EffectBasisType
			warnings = append(warnings, "УЧЕБНЫЙ СЦЕНАРИЙ: прирост задан синтетическим коэффициентом; не производственный прогноз")
		}
	}

	probAvailable, probability, probabilityLow, probabilityHigh, sample, successes, probObjects := p.Available, p.Value, p.Low, p.High, p.Sample, p.Successes, p.Objects
	if !probAvailable {
		warnings = append(warnings, fmt.Sprintf("вероятность подтверждения эффекта не рассчитывается: сопоставимых подтверждённых исходов %d на %d объект(ах); требуется минимум 5 исходов и 3 объекта", sample, probObjects))
	}
	return MethodEffectAssessment{
		Method: s.Method.Name, Direction: s.Direction, Classification: s.Classification,
		Applicability: applicability, Reliability: reliability, ReliabilityLevel: methodReliabilityLevel(reliability), Confidence: methodReliabilityLevel(reliability),
		HistoryMatches: matches, HistoryApprovedMatch: approved, CauseRiskShare: confirmedCauseShare,
		ProbabilityAvailable: probAvailable, EffectProbability: probability, ProbabilityLow: probabilityLow, ProbabilityHigh: probabilityHigh, ProbabilitySample: sample, ProbabilitySuccesses: successes, ProbabilityObjects: probObjects,
		LowPct: low, BasePct: base, HighPct: high, EffectQuantified: quantified && len(rows) > 0 && approved > 0, EffectBasisType: basis,
		Evidence: evidence, Reasons: reasons, Warnings: warnings,
	}
}

func methodSuggestionByName(objectKey, name string) (methodWorkflowSuggestion, bool) {
	key := normalizeMethodText(name)
	for _, s := range methodSuggestionsForObject(objectKey) {
		if normalizeMethodText(s.Method.Name) == key {
			return s, true
		}
	}
	return methodWorkflowSuggestion{}, false
}

func methodApplicabilityForName(objectKey, name string, now time.Time) MethodEffectAssessment {
	if s, ok := methodSuggestionByName(objectKey, name); ok {
		return methodApplicabilityForSuggestion(s, objectKey, now)
	}
	h := methodHistoryBasisForObject(objectKey, now)
	_, _, _, evidence, _, basis := methodEffectReferenceBand(name)
	reliability := clampInt(int(math.Round(0.6*float64(historyScoreForMethod(h))+25)), 20, 70)
	return MethodEffectAssessment{
		Method: name, Applicability: 45, Reliability: reliability, ReliabilityLevel: methodReliabilityLevel(reliability), Confidence: methodReliabilityLevel(reliability),
		ProbabilityAvailable: false, EffectProbability: 0, ProbabilityLow: 0, ProbabilityHigh: 0, ProbabilitySample: 0, ProbabilitySuccesses: 0, ProbabilityObjects: 0,
		EffectQuantified: false, EffectBasisType: basis, Evidence: evidence,
		Warnings: []string{"метод не сопоставлен напрямую с текущим утверждённым классом; численный вклад в МРП не начисляется до инженерной проверки"},
	}
}

func sameMethodFamily(a, b string) bool {
	family := func(name string) string {
		n := normalizeMethodText(name)
		switch {
		case strings.Contains(n, "соле") || strings.Contains(n, "соли") || strings.Contains(n, "комплексон"):
			return "scale"
		case strings.Contains(n, "корроз"):
			return "corrosion"
		case strings.Contains(n, "аспо") || strings.Contains(n, "парафин") || strings.Contains(n, "асфальтен"):
			return "aspo"
		case strings.Contains(n, "мехприм") || strings.Contains(n, "песк"):
			return "solids"
		case strings.Contains(n, "эмуль") || strings.Contains(n, "вязк"):
			return "viscosity"
		case strings.Contains(n, "гидрат"):
			return "hydrate"
		}
		return ""
	}
	fa, fb := family(a), family(b)
	return fa != "" && fa == fb
}

func combineMethodEffectPercents(methods []MethodEffectAssessment, selector func(MethodEffectAssessment) float64) float64 {
	// Residual-risk combination. Methods for the same mechanism receive a
	// duplication discount. No numerical synergy bonus is allowed until it is
	// calibrated on the object or a sufficiently comparable factual cohort.
	residual := 1.0
	seenNames := []string{}
	for _, m := range methods {
		if !m.EffectQuantified {
			continue
		}
		r := clampFloat(selector(m)/100.0, 0, 0.75)
		duplicated := false
		for _, prev := range seenNames {
			if sameMethodFamily(prev, m.Method) {
				duplicated = true
				break
			}
		}
		if duplicated {
			r *= 0.65
		}
		residual *= 1 - r
		seenNames = append(seenNames, m.Method)
	}
	return clampFloat((1-residual)*100, 0, 90)
}

func combinedMethodForecastForNames(objectKey string, names []string, snap methodForecastSnapshot, now time.Time) CombinedMethodForecast {
	finishEvaluation := beginMethodsEvaluation()
	defer finishEvaluation()
	target := targetRunlifeDaysForObject(objectKey)
	if target < defaultTargetRunlifeDays {
		target = defaultTargetRunlifeDays
	}
	out := CombinedMethodForecast{History: snap.History, Source: snap.Source, TargetDays: target}
	for _, name := range normalizeMethodList(names) {
		out.Methods = append(out.Methods, methodApplicabilityForName(objectKey, name, now))
	}
	out.Interaction = methodInteractionSummary(names)
	if snap.Base <= 0 || len(out.Methods) == 0 {
		return out
	}
	out.BaselineBase = snap.Base
	out.BaselineLow = snap.Low
	if out.BaselineLow <= 0 {
		out.BaselineLow = snap.Base
	}
	out.BaselineHigh = snap.High
	if out.BaselineHigh <= 0 {
		out.BaselineHigh = snap.Base
	}
	out.EffectLowPct = combineMethodEffectPercents(out.Methods, func(m MethodEffectAssessment) float64 { return m.LowPct })
	out.EffectBasePct = combineMethodEffectPercents(out.Methods, func(m MethodEffectAssessment) float64 { return m.BasePct })
	out.EffectHighPct = combineMethodEffectPercents(out.Methods, func(m MethodEffectAssessment) float64 { return m.HighPct })
	out.ForecastLow = out.BaselineLow * (1 + out.EffectLowPct/100)
	out.ForecastBase = out.BaselineBase * (1 + out.EffectBasePct/100)
	out.ForecastHigh = out.BaselineHigh * (1 + out.EffectHighPct/100)
	vals := []float64{out.ForecastLow, out.ForecastBase, out.ForecastHigh}
	if vals[0] > vals[1] {
		vals[0] = vals[1]
	}
	if vals[2] < vals[1] {
		vals[2] = vals[1]
	}
	out.ForecastLow, out.ForecastBase, out.ForecastHigh = vals[0], vals[1], vals[2]
	return out
}

func formatMethodApplicability(a MethodEffectAssessment, baselineDays float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ПРИМЕНИМОСТЬ: %d%% — инженерный индекс соответствия условиям, НЕ вероятность успеха.\r\n", a.Applicability)
	fmt.Fprintf(&b, "ДОСТОВЕРНОСТЬ ОЦЕНКИ: %d%% (%s). Основана на полноте 36-месячной истории, подтверждённости причин отказов, утверждённой классификации и качестве источника.\r\n", a.Reliability, a.ReliabilityLevel)
	if a.ProbabilityAvailable {
		fmt.Fprintf(&b, "ЭМПИРИЧЕСКАЯ ВЕРОЯТНОСТЬ ПОДТВЕРЖДЕНИЯ ЭФФЕКТА: %d%%; ориентировочный 95%% диапазон %d–%d%% (явных фактов %d на %d объект(ах), положительных %d; Beta(1,1)-сглаживание). Это не гарантия результата на следующей скважине.\r\n", a.EffectProbability, a.ProbabilityLow, a.ProbabilityHigh, a.ProbabilitySample, a.ProbabilityObjects, a.ProbabilitySuccesses)
	} else {
		fmt.Fprintf(&b, "ВЕРОЯТНОСТЬ ПОДТВЕРЖДЕНИЯ ЭФФЕКТА: не рассчитывается — сопоставимых подтверждённых исходов %d; требуется минимум 5 исходов на 3 объектах.\r\n", a.ProbabilitySample)
	}
	if a.EffectQuantified {
		if baselineDays > 0 {
			fmt.Fprintf(&b, "ПРЕДВАРИТЕЛЬНЫЙ ВКЛАД В НАРАБОТКУ: +%.0f%% / +%.0f%% / +%.0f%% (осторожный / ожидаемый / благоприятный), ≈ +%.0f / +%.0f / +%.0f сут. относительно %.0f сут.\r\n", a.LowPct, a.BasePct, a.HighPct, baselineDays*a.LowPct/100, baselineDays*a.BasePct/100, baselineDays*a.HighPct/100, baselineDays)
		} else {
			fmt.Fprintf(&b, "ПРЕДВАРИТЕЛЬНЫЙ ВКЛАД: +%.0f%% / +%.0f%% / +%.0f%%; в сутках не пересчитан без базового прогноза.\r\n", a.LowPct, a.BasePct, a.HighPct)
		}
		fmt.Fprintf(&b, "Доля исторического риска, на которую воздействует метод: %.0f%%.\r\n", a.CauseRiskShare*100)
	} else {
		b.WriteString("ВКЛАД В МРП: численно не начисляется. Метод может быть технологически полезен, но прямой перенос внешнего эффекта в наработку не обоснован либо нет совпадающей истории отказов.\r\n")
	}
	if len(a.Reasons) > 0 {
		b.WriteString("Почему подходит: " + strings.Join(a.Reasons, "; ") + ".\r\n")
	}
	if len(a.Warnings) > 0 {
		b.WriteString("Проверить: " + strings.Join(a.Warnings, "; ") + ".\r\n")
	}
	b.WriteString("Тип доказательной базы: " + a.EffectBasisType + ".\r\n")
	b.WriteString("Доказательная база: " + a.Evidence + ".")
	return b.String()
}

func formatCombinedMethodForecast(f CombinedMethodForecast) string {
	if f.ForecastBase <= 0 {
		return "Прогноз после комплекса пока не рассчитан: отсутствует базовый ориентир наработки."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "БАЗА ДО МЕРОПРИЯТИЙ (36 мес.): %.0f / %.0f / %.0f сут.\r\n", f.BaselineLow, f.BaselineBase, f.BaselineHigh)
	fmt.Fprintf(&b, "ПРОГНОЗ ПОСЛЕ КОМПЛЕКСА: осторожный %.0f сут.; ожидаемый %.0f сут.; благоприятный %.0f сут.\r\n", f.ForecastLow, f.ForecastBase, f.ForecastHigh)
	target := f.TargetDays
	if target < defaultTargetRunlifeDays {
		target = defaultTargetRunlifeDays
	}
	if f.ForecastBase >= target {
		fmt.Fprintf(&b, "ЦЕЛЬ: ≥ %.0f сут. — расчётный сценарий достигает целевого уровня (%.0f сут.).\r\n", target, f.ForecastBase)
	} else {
		fmt.Fprintf(&b, "ЦЕЛЬ: ≥ %.0f сут. — текущий комплекс даёт %.0f сут.; остаточный разрыв %.0f сут. Требуется усиление комплекса, а не искусственное завышение эффекта.\r\n", target, f.ForecastBase, target-f.ForecastBase)
	}
	fmt.Fprintf(&b, "Совокупный предварительный вклад без простого сложения: +%.0f%% / +%.0f%% / +%.0f%%.\r\n", f.EffectLowPct, f.EffectBasePct, f.EffectHighPct)
	quantified := 0
	for _, m := range f.Methods {
		if m.EffectQuantified {
			quantified++
		}
	}
	fmt.Fprintf(&b, "В численный прогноз вошли %d из %d методов; остальные показаны качественно без искусственного процента МРП.\r\n", quantified, len(f.Methods))
	b.WriteString("ОБОСНОВАНИЕ СЦЕНАРИЕВ:\r\n")
	b.WriteString("• Осторожный — нижняя граница фактической 36-месячной наработки и нижний подтверждаемый вклад методов по релевантной доле причин отказа.\r\n")
	b.WriteString("• Ожидаемый — медианный ориентир 36-месячной истории и центральная оценка вклада с учётом применимости и достоверности.\r\n")
	b.WriteString("• Благоприятный — верхняя граница фактической истории и верхний внешний ориентир только для методов с прямыми данными по наработке до отказа; это не гарантия.\r\n")
	b.WriteString("Взаимодействие: " + f.Interaction + "\r\n")
	b.WriteString("Важно: численный бонус за синергию отдельно НЕ начислен; он появится только после калибровки по факту объекта или сопоставимой выборке.")
	if strings.TrimSpace(f.History.Warning) != "" {
		b.WriteString("\r\n" + f.History.Warning)
	}
	return b.String()
}

func roundFinite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v)
}

func formatCombinedMethodDetails(objectKey string, names []string, snap methodForecastSnapshot, now time.Time) string {
	names = normalizeMethodList(names)
	if len(names) == 0 {
		return "Комплекс методов ещё не сформирован."
	}
	var b strings.Builder
	b.WriteString("ТЕКУЩИЙ КОМПЛЕКС:\r\n")
	for i, name := range names {
		a := methodApplicabilityForName(objectKey, name, now)
		contribution := "; вклад в МРП не начислен"
		if a.EffectQuantified {
			if snap.Base > 0 {
				contribution = fmt.Sprintf("; ожидаемый предварительный вклад +%.0f%% (≈ +%.0f сут.)", a.BasePct, snap.Base*a.BasePct/100)
			} else {
				contribution = fmt.Sprintf("; ожидаемый предварительный вклад +%.0f%%", a.BasePct)
			}
		}
		prob := "вероятность эффекта — НД"
		if a.ProbabilityAvailable {
			prob = "вероятность подтверждения эффекта " + strconv.Itoa(a.EffectProbability) + "%"
		}
		fmt.Fprintf(&b, "%d. %s — применимость %d%%; достоверность %d%%; %s%s\r\n", i+1, name, a.Applicability, a.Reliability, prob, contribution)
	}
	b.WriteString("ВЗАИМОДЕЙСТВИЕ: " + methodInteractionSummary(names))
	return b.String()
}

func sortedMethodAssessmentsByApplicability(in []MethodEffectAssessment) []MethodEffectAssessment {
	out := append([]MethodEffectAssessment(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Applicability != out[j].Applicability {
			return out[i].Applicability > out[j].Applicability
		}
		if out[i].Reliability != out[j].Reliability {
			return out[i].Reliability > out[j].Reliability
		}
		return out[i].Method < out[j].Method
	})
	return out
}
