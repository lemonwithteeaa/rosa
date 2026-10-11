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

// ReplicationAssessment answers a separate question from method applicability:
// whether an already selected/approved approach has enough comparable targets
// and repeatable evidence to justify a controlled pilot or broader rollout.
// It is deliberately NOT a probability of success.
type ReplicationAssessment struct {
	Method                     string
	Direction                  string
	Classification             string
	Score                      int
	Category                   string
	MechanismFit               int
	OperatingFit               int
	EquipmentFit               int
	Repeatability              int
	Durability                 int
	OperationalReproducibility int
	PilotAnalogCount           int
	QualifiedAnalogCount       int
	InternalProbability        int
	InternalProbabilityLow     int
	InternalProbabilityHigh    int
	InternalSample             int
	InternalObjects            int
	RepeatabilityBasis         string
	DurabilityBasis            string
	Warnings                   []string
	Analogs                    []ReplicationAnalog
}

type ReplicationAnalog struct {
	ObjectKey          string
	Field              string
	Well               string
	ObjectPlast        string
	Equipment          string
	Class              string
	Score              int
	MechanismFit       int
	OperatingFit       int
	EquipmentFit       int
	Basis              string
	OperatingProfile   string
	PreviouslyAnalyzed bool
}

type replicationCandidate struct {
	State              TechObjectState
	PreviouslyAnalyzed bool
}

type replicationEvidence struct {
	Matches  int
	Approved int
	Archive  bool
}

func replicationCandidates() []replicationCandidate {
	// Build the candidate set once. Current technical states are preferred, while
	// objects that were analysed earlier in ROSA remain visible even if they are
	// absent from the latest imported operating-mode file.
	analyzed := map[string]bool{}
	latestVersion := map[string]VersionRecord{}
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		k := strings.ToLower(strings.TrimSpace(v.ObjectKey))
		if k == "" {
			continue
		}
		analyzed[k] = true
		if _, ok := latestVersion[k]; !ok {
			latestVersion[k] = v
		}
	}
	seen := map[string]bool{}
	out := make([]replicationCandidate, 0, len(techCurrent)+len(latestVersion))
	for _, st := range techCurrent {
		k := strings.ToLower(strings.TrimSpace(st.ObjectKey))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, replicationCandidate{State: st, PreviouslyAnalyzed: analyzed[k]})
	}
	for k, v := range latestVersion {
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, replicationCandidate{State: TechObjectState{
			ObjectKey: v.ObjectKey, Field: v.Field, Well: v.Well, Deposit: v.Deposit,
			ObjectPlast: v.ObjectPlast, Equipment: v.Equipment, CurrentMRP: v.CurrentMRP,
			SourceFile: "Архив РОСА",
		}, PreviouslyAnalyzed: true})
	}
	return out
}

func replicationEvidenceIndex(d assessmentClassDirection, now time.Time) map[string]replicationEvidence {
	// Previous implementation called historyForWell for every candidate. On a
	// large imported fund that repeatedly rescanned the same failure/archive data
	// on the UI thread and made the АНАЛОГИ button appear frozen. Index the same
	// evidence in linear passes instead.
	out := map[string]replicationEvidence{}
	addFailure := func(key string, e FailureEvent) {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" || !failureMatchesMethodDirection(e, d.Name) {
			return
		}
		x := out[k]
		x.Matches++
		if e.ConclusionApproved || strings.TrimSpace(e.ApprovedConclusion) != "" || strings.TrimSpace(e.CommissionConclusion) != "" {
			x.Approved++
		}
		out[k] = x
	}
	for _, e := range failureEvents {
		if e.Draft || !inFailureWindow(e.Date, now) {
			continue
		}
		addFailure(e.ObjectKey, e)
	}
	versionByID := make(map[string]VersionRecord, len(versions))
	for _, v := range versions {
		versionByID[v.VersionID] = v
		k := strings.ToLower(strings.TrimSpace(v.ObjectKey))
		if k != "" && methodTextMatchesDirection(v.Complication, d.Name) {
			x := out[k]
			x.Archive = true
			out[k] = x
		}
	}
	for _, outcome := range outcomes {
		v, found := versionByID[outcome.VersionID]
		if !found || !strings.EqualFold(strings.TrimSpace(outcome.EventType), "отказ") || !inFailureWindow(outcome.Date, now) {
			continue
		}
		e := FailureEvent{ObjectKey: v.ObjectKey, Date: outcome.Date, EventType: outcome.EventType,
			FailedUnit: outcome.FailureMode, Source: outcome.EvidenceSource, SourceCause: outcome.FailureCause,
			Inspection: outcome.DefectFinding, Comment: outcome.Comment}
		addFailure(v.ObjectKey, e)
	}
	return out
}

func replicationDirectionClassIndexed(objectKey string, d assessmentClassDirection, evidence map[string]replicationEvidence) (string, bool, string) {
	r := assessmentRecord(objectKey, d)
	if r.FullClassificationConfirmed {
		if c := strings.TrimSpace(r.FinalClass); c != "" && !strings.EqualFold(c, "НД") && !strings.EqualFold(c, "Не относится") {
			return c, true, "утверждённая классификация"
		}
	}
	e := evidence[strings.ToLower(strings.TrimSpace(objectKey))]
	if e.Approved > 0 {
		return d.Prefix, false, fmt.Sprintf("%d подтверждённ. отказ(а/ов) того же механизма", e.Approved)
	}
	if e.Matches > 0 {
		return d.Prefix, false, fmt.Sprintf("%d совпадающ. отказ(а/ов), причина не полностью утверждена", e.Matches)
	}
	if e.Archive {
		return d.Prefix, false, "ранее анализировался в РОСА по тому же семейству осложнения"
	}
	return "", false, ""
}

func replicationOperatingProfile(st TechObjectState) string {
	parts := []string{}
	if v := strings.TrimSpace(st.WaterCut); v != "" {
		parts = append(parts, "обводнённость "+v+"%")
	}
	if v := strings.TrimSpace(st.LiquidRate); v != "" {
		parts = append(parts, "жидкость "+v+" м³/сут")
	}
	if v := strings.TrimSpace(st.TMSTemperature); v != "" {
		parts = append(parts, "T "+v+" °C")
	}
	if v := strings.TrimSpace(st.BottomholePressure); v != "" {
		parts = append(parts, "Pзаб "+v)
	}
	if v := strings.TrimSpace(st.DynamicLevel); v != "" {
		parts = append(parts, "Hдин "+v+" м")
	}
	if v := strings.TrimSpace(st.Equipment); v != "" {
		parts = append(parts, "ГНО: "+v)
	}
	if len(parts) == 0 {
		return "режимные параметры в архиве не сохранены"
	}
	return strings.Join(parts, "; ")
}

func parseReplicationNumber(v string) (float64, bool) {
	v = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(v, "\u00a0", ""), " ", ""))
	v = strings.ReplaceAll(v, ",", ".")
	if v == "" || v == "—" || v == "-" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
}

func replicationStateForObject(objectKey string) (TechObjectState, bool) {
	key := strings.TrimSpace(objectKey)
	for _, st := range techCurrent {
		if strings.EqualFold(strings.TrimSpace(st.ObjectKey), key) {
			return st, true
		}
	}
	if st, ok := teachingWorkflowStateForObject(key); ok {
		return st, true
	}
	return TechObjectState{}, false
}

func replicationDirectionClass(objectKey string, d assessmentClassDirection) (string, bool, string) {
	r := assessmentRecord(objectKey, d)
	if r.FullClassificationConfirmed {
		if c := strings.TrimSpace(r.FinalClass); c != "" && !strings.EqualFold(c, "НД") && !strings.EqualFold(c, "Не относится") {
			return c, true, "утверждённая классификация"
		}
	}
	rows := historyForWell(objectKey, time.Now())
	matches, approved, _ := methodFailureEvidence(rows, d.Name)
	if approved > 0 {
		return d.Prefix, false, fmt.Sprintf("%d подтверждённ. отказ(а/ов) того же механизма", approved)
	}
	if matches > 0 {
		return d.Prefix, false, fmt.Sprintf("%d совпадающ. отказ(а/ов), причина не полностью утверждена", matches)
	}
	for i := len(versions) - 1; i >= 0; i-- {
		v := versions[i]
		if !strings.EqualFold(strings.TrimSpace(v.ObjectKey), strings.TrimSpace(objectKey)) {
			continue
		}
		if methodTextMatchesDirection(v.Complication, d.Name) {
			return d.Prefix, false, "архивная запись того же семейства осложнения"
		}
	}
	return "", false, ""
}

func replicationMechanismFit(targetClass, candidateClass string, confirmed bool, hasWeakEvidence bool) int {
	if strings.TrimSpace(candidateClass) == "" {
		return 0
	}
	if !confirmed {
		if hasWeakEvidence {
			return 48
		}
		return 35
	}
	ts := methodClassSeverity(targetClass)
	cs := methodClassSeverity(candidateClass)
	if ts == 0 || cs == 0 {
		return 70
	}
	d := ts - cs
	if d < 0 {
		d = -d
	}
	switch d {
	case 0:
		return 100
	case 1:
		return 82
	default:
		return 62
	}
}

func replicationAbsSimilarity(a, b, best, acceptable float64) (float64, bool) {
	if math.IsNaN(a) || math.IsNaN(b) {
		return 0, false
	}
	d := math.Abs(a - b)
	switch {
	case d <= best:
		return 100, true
	case d <= acceptable:
		return 100 - 55*(d-best)/(acceptable-best), true
	default:
		return math.Max(10, 45-35*math.Min(1, (d-acceptable)/math.Max(acceptable, 1))), true
	}
}

func replicationRatioSimilarity(a, b float64) (float64, bool) {
	if a <= 0 || b <= 0 {
		return 0, false
	}
	r := a / b
	if r < 1 {
		r = 1 / r
	}
	switch {
	case r <= 1.25:
		return 100, true
	case r <= 1.6:
		return 82, true
	case r <= 2.0:
		return 65, true
	case r <= 3.0:
		return 40, true
	default:
		return 15, true
	}
}

func replicationOperatingFit(a, b TechObjectState) (int, string) {
	type component struct {
		name  string
		score float64
	}
	parts := []component{}
	if av, ok := parseReplicationNumber(a.WaterCut); ok {
		if bv, ok2 := parseReplicationNumber(b.WaterCut); ok2 {
			if s, yes := replicationAbsSimilarity(av, bv, 8, 25); yes {
				parts = append(parts, component{"обводнённость", s})
			}
		}
	}
	if av, ok := parseReplicationNumber(a.TMSTemperature); ok {
		if bv, ok2 := parseReplicationNumber(b.TMSTemperature); ok2 {
			if s, yes := replicationAbsSimilarity(av, bv, 10, 35); yes {
				parts = append(parts, component{"температура", s})
			}
		}
	}
	if av, ok := parseReplicationNumber(a.LiquidRate); ok {
		if bv, ok2 := parseReplicationNumber(b.LiquidRate); ok2 {
			if s, yes := replicationRatioSimilarity(av, bv); yes {
				parts = append(parts, component{"дебит жидкости", s})
			}
		}
	}
	if av, ok := parseReplicationNumber(a.BottomholePressure); ok {
		if bv, ok2 := parseReplicationNumber(b.BottomholePressure); ok2 {
			if s, yes := replicationRatioSimilarity(av, bv); yes {
				parts = append(parts, component{"забойное давление", s})
			}
		}
	}
	if av, ok := parseReplicationNumber(a.DynamicLevel); ok {
		if bv, ok2 := parseReplicationNumber(b.DynamicLevel); ok2 {
			if s, yes := replicationRatioSimilarity(av, bv); yes {
				parts = append(parts, component{"динамический уровень", s})
			}
		}
	}
	contextBonus := 0.0
	context := []string{}
	if strings.TrimSpace(a.Field) != "" && strings.EqualFold(strings.TrimSpace(a.Field), strings.TrimSpace(b.Field)) {
		contextBonus += 8
		context = append(context, "то же месторождение")
	}
	if strings.TrimSpace(a.Deposit) != "" && strings.EqualFold(strings.TrimSpace(a.Deposit), strings.TrimSpace(b.Deposit)) {
		contextBonus += 7
		context = append(context, "та же залежь")
	}
	if strings.TrimSpace(a.ObjectPlast) != "" && strings.EqualFold(strings.TrimSpace(a.ObjectPlast), strings.TrimSpace(b.ObjectPlast)) {
		contextBonus += 5
		context = append(context, "тот же объект/пласт")
	}
	if len(parts) == 0 {
		base := 30 + contextBonus
		return clampInt(int(math.Round(base)), 20, 60), "режимные численные параметры неполны" + func() string {
			if len(context) == 0 {
				return ""
			}
			return "; " + strings.Join(context, ", ")
		}()
	}
	sum := 0.0
	names := []string{}
	for _, p := range parts {
		sum += p.score
		names = append(names, p.name)
	}
	avg := sum / float64(len(parts))
	avg = clampFloat(avg+contextBonus, 0, 100)
	basis := "сопоставлены: " + strings.Join(names, ", ")
	if len(context) > 0 {
		basis += "; " + strings.Join(context, ", ")
	}
	return int(math.Round(avg)), basis
}

func replicationEquipmentFit(method string, a, b TechObjectState) (int, string) {
	ta := normalizeTargetLiftType(a.Equipment)
	tb := normalizeTargetLiftType(b.Equipment)
	if ta == "" || tb == "" {
		return 50, "тип ГНО указан не полностью"
	}
	if ta == tb {
		return 100, "одно семейство ГНО: " + ta
	}
	if assessmentMethodIsEquipmentDesign(method) {
		return 5, "разное семейство ГНО; оборудование требует отдельного подбора"
	}
	return 60, "разное семейство ГНО, но метод не является прямым подбором компоновки"
}

func replicationExternalRepeatability(method string) (int, string) {
	n := normalizeMethodText(method)
	switch {
	case strings.Contains(n, "медленно высвобож"):
		return 95, "внешняя многоскважинная база: расширение от пилота до ~70 скважин с устойчивым превосходством длительности защитного цикла после пластовой обработки ингибитором"
	case strings.Contains(n, "ингибитор") && strings.Contains(n, "соле"):
		return 78, "внешние полевые серии подтверждают повторяемость контроля солеотложений; перенос на новый фонд требует проверки минералогии/вод"
	case (strings.Contains(n, "аспо") || strings.Contains(n, "парафин") || strings.Contains(n, "асфальтен")) && strings.Contains(n, "ингиб"):
		return 72, "полевые серии на нескольких проблемных скважинах показывают повторяемый рост наработки до отказа после лабораторного подбора"
	case strings.Contains(n, "корроз") && strings.Contains(n, "ингиб"):
		return 70, "многоскважинные программы показывают снижение коррозионных отказов при стабильной подаче и контроле"
	case strings.Contains(n, "газ") && (strings.Contains(n, "режим") || strings.Contains(n, "ток") || strings.Contains(n, "давлен")):
		return 88, "полевое тиражирование по данным Общества инженеров-нефтяников режима частотно-регулируемого привода на 60 скважинах показало снижение числа остановок и срывов УЭЦН и рост коэффициента технической готовности"
	case strings.Contains(n, "мехприм") || strings.Contains(n, "песк"):
		return 75, "есть многоскважинные пескозащитные кейсы, но конкретная техника должна проверяться отдельно"
	case strings.Contains(n, "гидрат") || strings.Contains(n, "метанол") || strings.Contains(n, "гликол"):
		return 68, "повторяемость контроля гидратов подтверждается полевыми программами при соблюдении условий давления–температуры и режима дозирования"
	case strings.Contains(n, "монитор") || strings.Contains(n, "контроль"):
		return 76, "контроль/мониторинг хорошо тиражируется организационно, но сам по себе не является устранением механизма отказа"
	default:
		return 48, "внешняя повторяемость ограничена или индивидуальный вклад метода не отделён"
	}
}

func replicationInternalDurability(method string) (int, string) {
	pairs := 0
	improved := 0
	ratios := []float64{}
	for _, v := range versions {
		if !methodNamesComparable(method, v.Method) {
			continue
		}
		o := outcomeFor(v.VersionID)
		if o == nil {
			continue
		}
		known, success := methodProbabilityObservation(method, v, *o)
		if !known || !success {
			continue
		}
		actual, okA := parseReplicationNumber(o.ActualMRP)
		before, okB := parseReplicationNumber(v.CurrentMRP)
		if !okA || actual <= 0 {
			continue
		}
		pairs++
		if okB && before > 0 {
			r := actual / before
			ratios = append(ratios, r)
			if r >= 1.0 {
				improved++
			}
		} else if actual >= 180 {
			improved++
		}
	}
	if pairs == 0 {
		_, _, _, _, quantified, _ := methodEffectReferenceBand(method)
		if quantified {
			return 62, "прямые внешние данные по наработке до отказа есть, но внутренняя длительность эффекта ещё не накоплена"
		}
		return 42, "нет достаточной внутренней серии с фактическим МРП после применения"
	}
	rate := float64(improved) / float64(pairs)
	score := 45 + int(math.Round(45*rate))
	if pairs >= 5 {
		score += 10
	} else if pairs >= 3 {
		score += 5
	}
	score = clampInt(score, 35, 100)
	basis := fmt.Sprintf("внутренние подтверждённые положительные исходы с фактическим МРП: %d; устойчивых относительно базового уровня: %d", pairs, improved)
	if len(ratios) > 0 {
		sort.Float64s(ratios)
		basis += fmt.Sprintf("; медиана отношения МРП после/до %.2f", ratios[len(ratios)/2])
	}
	return score, basis
}

func replicationOperationalReproducibility(method string) int {
	n := normalizeMethodText(method)
	score := 62
	switch {
	case strings.Contains(n, "монитор") || strings.Contains(n, "контроль") || strings.Contains(n, "режим"):
		score = 90
	case strings.Contains(n, "дозир") || strings.Contains(n, "ингибитор"):
		score = 84
	case strings.Contains(n, "периодичес") || strings.Contains(n, "squeeze") || strings.Contains(n, "обработк"):
		score = 74
	case strings.Contains(n, "промыв") || strings.Contains(n, "очистк"):
		score = 72
	case assessmentMethodIsEquipmentDesign(method):
		score = 48
	}
	if assessmentMethodNeedsLabSelection(method) {
		score -= 12
	}
	return clampInt(score, 25, 95)
}

func replicationAnalogSimilarity(mechanism, operating, equipment int) int {
	return clampInt(int(math.Round(0.46*float64(mechanism)+0.31*float64(operating)+0.23*float64(equipment))), 0, 100)
}

func methodReplicationAssessmentForSuggestion(s methodWorkflowSuggestion, objectKey string, now time.Time) ReplicationAssessment {
	out := ReplicationAssessment{Method: s.Method.Name, Direction: s.Direction, Classification: s.Classification}
	target, ok := replicationStateForObject(objectKey)
	if !ok {
		out.Category = "Нет данных фонда для поиска аналогов"
		out.Warnings = append(out.Warnings, "технический режим выбранного объекта не найден")
		return out
	}
	d, ok := methodDirectionByName(s.Direction)
	if !ok {
		out.Category = "Направление осложнения не распознано"
		return out
	}
	analogs := []ReplicationAnalog{}
	evidence := replicationEvidenceIndex(d, now)
	for _, candidate := range replicationCandidates() {
		cand := candidate.State
		if strings.TrimSpace(cand.ObjectKey) == "" || strings.EqualFold(strings.TrimSpace(cand.ObjectKey), strings.TrimSpace(objectKey)) {
			continue
		}
		class, confirmed, classBasis := replicationDirectionClassIndexed(cand.ObjectKey, d, evidence)
		mechanism := replicationMechanismFit(s.Classification, class, confirmed, class != "")
		if mechanism <= 0 {
			continue
		}
		operating, operatingBasis := replicationOperatingFit(target, cand)
		equipment, equipmentBasis := replicationEquipmentFit(s.Method.Name, target, cand)
		score := replicationAnalogSimilarity(mechanism, operating, equipment)
		basis := strings.Join([]string{classBasis, operatingBasis, equipmentBasis}, "; ")
		analogs = append(analogs, ReplicationAnalog{
			ObjectKey: cand.ObjectKey, Field: cand.Field, Well: cand.Well, ObjectPlast: cand.ObjectPlast, Equipment: cand.Equipment,
			Class: class, Score: score, MechanismFit: mechanism, OperatingFit: operating, EquipmentFit: equipment, Basis: basis,
			OperatingProfile: replicationOperatingProfile(cand), PreviouslyAnalyzed: candidate.PreviouslyAnalyzed,
		})
	}
	sort.SliceStable(analogs, func(i, j int) bool {
		if analogs[i].Score != analogs[j].Score {
			return analogs[i].Score > analogs[j].Score
		}
		return strings.ToLower(analogs[i].ObjectKey) < strings.ToLower(analogs[j].ObjectKey)
	})
	if len(analogs) > 25 {
		analogs = analogs[:25]
	}
	out.Analogs = analogs
	for _, a := range analogs {
		if a.Score >= 60 {
			out.PilotAnalogCount++
		}
		if a.Score >= 70 && a.MechanismFit >= 62 {
			out.QualifiedAnalogCount++
		}
	}

	// Use the best comparable slice for the transferability components. A large
	// low-quality population must not dilute a small set of genuinely analogous
	// wells, but one excellent analogue also cannot unlock broad rollout by itself.
	top := analogs
	if len(top) > 10 {
		top = top[:10]
	}
	if len(top) > 0 {
		var sm, so, se int
		for _, a := range top {
			sm += a.MechanismFit
			so += a.OperatingFit
			se += a.EquipmentFit
		}
		out.MechanismFit = sm / len(top)
		out.OperatingFit = so / len(top)
		out.EquipmentFit = se / len(top)
	}

	a := methodApplicabilityForSuggestion(s, objectKey, now)
	out.InternalProbability = a.EffectProbability
	out.InternalProbabilityLow = a.ProbabilityLow
	out.InternalProbabilityHigh = a.ProbabilityHigh
	out.InternalSample = a.ProbabilitySample
	out.InternalObjects = a.ProbabilityObjects
	if a.ProbabilityAvailable {
		coverage := 55
		if a.ProbabilitySample >= 10 {
			coverage = 80
		}
		if a.ProbabilitySample >= 15 && a.ProbabilityObjects >= 5 {
			coverage = 100
		}
		out.Repeatability = clampInt(int(math.Round(0.72*float64(a.EffectProbability)+0.28*float64(coverage))), 0, 100)
		out.RepeatabilityBasis = fmt.Sprintf("внутренняя выборка: %d исходов на %d объектах; вероятность подтверждения эффекта %d%% (%d–%d%%)", a.ProbabilitySample, a.ProbabilityObjects, a.EffectProbability, a.ProbabilityLow, a.ProbabilityHigh)
	} else {
		out.Repeatability, out.RepeatabilityBasis = replicationExternalRepeatability(s.Method.Name)
		out.Warnings = append(out.Warnings, "внутренняя повторяемость ещё не подтверждена достаточной выборкой; компонент повторяемости опирается на внешний полевой опыт")
	}
	out.Durability, out.DurabilityBasis = replicationInternalDurability(s.Method.Name)
	out.OperationalReproducibility = replicationOperationalReproducibility(s.Method.Name)

	score := 0.30*float64(out.MechanismFit) + 0.20*float64(out.OperatingFit) + 0.15*float64(out.EquipmentFit) + 0.20*float64(out.Repeatability) + 0.10*float64(out.Durability) + 0.05*float64(out.OperationalReproducibility)
	out.Score = clampInt(int(math.Round(score)), 0, 100)

	// Hard gates prevent a mathematically attractive score from being interpreted
	// as authorization to mass-rollout when diversity or factual outcomes are weak.
	if out.QualifiedAnalogCount == 0 {
		out.Score = minInt(out.Score, 44)
	} else if out.QualifiedAnalogCount < 3 {
		out.Score = minInt(out.Score, 59)
	}
	if !a.ProbabilityAvailable {
		out.Score = minInt(out.Score, 74)
	}
	if assessmentMethodNeedsLabSelection(s.Method.Name) {
		out.Score = minInt(out.Score, 79)
		out.Warnings = append(out.Warnings, "для химического метода перед тиражированием обязателен лабораторный подбор/совместимость на репрезентативных пробах группы")
	}
	if assessmentMethodIsEquipmentDesign(s.Method.Name) {
		out.Score = minInt(out.Score, 69)
		out.Warnings = append(out.Warnings, "конкретная компоновка/оборудование не тиражируется РОСА автоматически; требуется проверка в «Автотехнологе»")
	}

	switch {
	case a.ProbabilityAvailable && a.ProbabilitySample >= 10 && a.ProbabilityObjects >= 5 && a.EffectProbability >= 75 && a.ProbabilityLow >= 55 && out.QualifiedAnalogCount >= 10 && out.Score >= 88:
		out.Category = "Высокий потенциал переноса после контрольного пилота"
	case a.ProbabilityAvailable && out.QualifiedAnalogCount >= 3 && a.EffectProbability >= 65 && out.Score >= 78:
		out.Category = "Подтверждено для группы близких объектов"
	case out.PilotAnalogCount >= 3 && out.Score >= 62:
		out.Category = "Проверить на группе близких объектов"
	case out.PilotAnalogCount >= 1 && out.Score >= 45:
		out.Category = "Кандидат для пилота на близком объекте"
	default:
		out.Category = "Только для текущего объекта / недостаточно данных для переноса"
	}
	return out
}

func methodReplicationAssessment(objectKey, method string, now time.Time) ReplicationAssessment {
	if s, ok := methodSuggestionByName(objectKey, method); ok {
		return methodReplicationAssessmentForSuggestion(s, objectKey, now)
	}
	return ReplicationAssessment{Method: method, Score: 0, Category: "Метод не сопоставлен с утверждённой классификацией", Warnings: []string{"сначала требуется привязка метода к утверждённому механизму осложнения"}}
}

func methodReplicationText(r ReplicationAssessment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "АНАЛОГИ И ПРИМЕНИМОСТЬ МЕТОДА\r\n%s\r\n\r\n", dash(r.Method))
	fmt.Fprintf(&b, "ПОТЕНЦИАЛ ПЕРЕНОСА: %d%% — %s.\r\n", r.Score, r.Category)
	b.WriteString("Это индекс сходства и переносимости решения, а НЕ вероятность успеха отдельной скважины и НЕ автоматическое решение о внедрении.\r\n\r\n")
	b.WriteString("ГДЕ ЕЩЁ МЕТОД МОЖЕТ БЫТЬ ПОЛЕЗЕН\r\n")
	b.WriteString("РОСА ищет близкие режимы в текущем фонде и среди ранее анализированных объектов. Сначала смотрите на механизм осложнения, затем на режим и ГНО.\r\n")
	fmt.Fprintf(&b, "Подходящих для дополнительной проверки объектов (сходство ≥60): %d; близких аналогов (≥70): %d.\r\n\r\n", r.PilotAnalogCount, r.QualifiedAnalogCount)

	if len(r.Analogs) > 0 {
		b.WriteString("НАИБОЛЕЕ БЛИЗКИЕ ОБЪЕКТЫ И РЕЖИМЫ\r\n")
		limit := len(r.Analogs)
		if limit > 12 {
			limit = 12
		}
		for i := 0; i < limit; i++ {
			a := r.Analogs[i]
			fmt.Fprintf(&b, "%d. %s", i+1, dash(a.ObjectKey))
			if strings.TrimSpace(a.Field) != "" || strings.TrimSpace(a.Well) != "" {
				fmt.Fprintf(&b, " — %s / скв. %s", dash(a.Field), dash(a.Well))
			}
			if a.PreviouslyAnalyzed {
				b.WriteString(" — ранее анализировался в РОСА")
			}
			fmt.Fprintf(&b, "\r\n   Сходство %d%%: механизм %d; режим %d; ГНО %d.\r\n", a.Score, a.MechanismFit, a.OperatingFit, a.EquipmentFit)
			fmt.Fprintf(&b, "   Режим: %s.\r\n", dash(a.OperatingProfile))
			fmt.Fprintf(&b, "   Основание: %s.\r\n", dash(a.Basis))
		}
	} else {
		b.WriteString("НАИБОЛЕЕ БЛИЗКИЕ ОБЪЕКТЫ И РЕЖИМЫ\r\n")
		b.WriteString("Близкие объекты в текущем фонде и архиве РОСА не найдены.\r\n")
	}

	b.WriteString("\r\nПОЧЕМУ РОСА ТАК ОЦЕНИЛА ПЕРЕНОС\r\n")
	fmt.Fprintf(&b, "• сходство механизма осложнения — %d/100;\r\n• сходство режима/условий — %d/100;\r\n• сопоставимость ГНО — %d/100;\r\n• повторяемость результата — %d/100;\r\n• устойчивость эффекта во времени — %d/100;\r\n• операционная воспроизводимость — %d/100.\r\n", r.MechanismFit, r.OperatingFit, r.EquipmentFit, r.Repeatability, r.Durability, r.OperationalReproducibility)
	if r.InternalSample > 0 {
		if r.InternalObjects >= 3 && r.InternalSample >= 5 {
			fmt.Fprintf(&b, "Внутренняя повторяемость: вероятность подтверждения эффекта %d%% (%d–%d%%), %d исходов / %d объектов.\r\n", r.InternalProbability, r.InternalProbabilityLow, r.InternalProbabilityHigh, r.InternalSample, r.InternalObjects)
		} else {
			fmt.Fprintf(&b, "Внутренней выборки пока недостаточно: %d исходов / %d объектов. Для численной вероятности подтверждения эффекта требуется ≥5 исходов на ≥3 объектах.\r\n", r.InternalSample, r.InternalObjects)
		}
	}
	b.WriteString("Повторяемость: " + dash(r.RepeatabilityBasis) + ".\r\n")
	b.WriteString("Устойчивость: " + dash(r.DurabilityBasis) + ".\r\n")
	if len(r.Warnings) > 0 {
		b.WriteString("\r\nОГРАНИЧЕНИЯ ПЕРЕНОСА\r\n")
		for _, w := range r.Warnings {
			b.WriteString("• " + w + ".\r\n")
		}
	}
	b.WriteString("\r\nКАК ИСПОЛЬЗОВАТЬ РЕЗУЛЬТАТ\r\n")
	b.WriteString("Выберите 1–3 наиболее близких объекта, проверьте реальные ограничения режима и технологическую совместимость, затем используйте их как кандидатов для контролируемого пилота. Расширение применения выполняется только после подтверждения фактического эффекта.")
	return b.String()
}
