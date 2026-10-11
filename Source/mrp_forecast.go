package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// MRP forecast is intentionally evidence-first. RC196 does not introduce an
// opaque composite score and does not assign invented effects to lab or
// process variables. The baseline is built only from factual completed MRP
// observations for the same object. Current process/lab data are reported as
// context and become predictors only after a separately validated model exists.
const mrpForecastModelVersion = "MRP-base-1.4-ru"

type MRPRegimeSnapshot struct {
	CapturedAt         time.Time
	Equipment          string
	Complication       string
	WaterCut           string
	LiquidRate         string
	OilRate            string
	BottomholePressure string
	DynamicLevel       string
	Temperature        string
	Completeness       string
}

type MRPObservation struct {
	ObjectKey               string
	Field                   string
	Well                    string
	ObjectPlast             string
	Date                    time.Time
	CycleStartDate          time.Time
	CycleEndDate            time.Time
	MRPDays                 float64
	DurationSource          string
	DurationWarning         string
	Source                  string
	VersionID               string
	Equipment               string
	Method                  string
	Complication            string
	EventType               string
	Planned                 bool
	FailureMode             string
	ClassificationSource    string
	ClassificationConfirmed bool
	Snapshot                MRPRegimeSnapshot
}

type MRPForecastAudit struct {
	VersionID     string
	ForecastDate  time.Time
	BaseDays      float64
	ActualDate    time.Time
	ActualMRPDays float64
	ErrorDays     float64
	AbsoluteError float64
}

type MRPBacktestSummary struct {
	Eligible       bool
	TrainingCount  int
	TestCount      int
	MedianAbsError float64
	Note           string
}

type MRPForecastFactor struct {
	Name     string
	Value    string
	Evidence string
	Note     string
}

type MRPForecastInput struct {
	ObjectKey       string
	CurrentMRPDays  float64
	CurrentDate     time.Time
	Observations    []MRPObservation
	Equipment       string
	Complication    string
	WaterCut        string
	LiquidRate      string
	OilRate         string
	BottomholePress string
	DynamicLevel    string
	Temperature     string
	LabClasses      []string
	Method          string
	TargetMRPDays   float64
	TargetSource    string
	TargetScope     string
	TargetLiftType  string
	TargetNearPct   float64
}

type MRPForecastResult struct {
	ModelVersion        string
	ObjectKey           string
	CurrentMRPDays      float64
	LowDays             float64
	BaseDays            float64
	HighDays            float64
	HasLowHigh          bool
	ObservationCount    int
	LatestObservation   time.Time
	LatestActualMRPDays float64
	EvidenceLevel       string
	Status              string
	Basis               []string
	Warnings            []string
	Factors             []MRPForecastFactor
	ComparableCount     int
	NonComparableCount  int
	UnknownComparable   int
	Backtest            MRPBacktestSummary
	ComparableBaseDays  float64
	HasComparableBase   bool
	AuditCount          int
	AuditActualCount    int
	AuditMedianAbsError float64
	EconomicsReady      bool
	MethodContext       string
	Advanced            MRPAdvancedSummary
	SurvivalLowDays     float64
	SurvivalBaseDays    float64
	SurvivalHighDays    float64
	HasSurvivalTriplet  bool
	SurvivalEconSource  string
	TargetMRPDays       float64
	TargetGapDays       float64
	TargetRatioPct      float64
	TargetStatus        string
	TargetForecastDays  float64
	TargetSource        string
	TargetScope         string
	TargetLiftType      string
	TargetNearPct       float64
}

func validMRPObservations(in []MRPObservation) []MRPObservation {
	out := make([]MRPObservation, 0, len(in))
	for _, o := range in {
		if o.MRPDays <= 0 || math.IsNaN(o.MRPDays) || math.IsInf(o.MRPDays, 0) {
			continue
		}
		out = append(out, o)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Date.Equal(out[j].Date) {
			return out[i].MRPDays < out[j].MRPDays
		}
		return out[i].Date.Before(out[j].Date)
	})
	return out
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	mid := len(v) / 2
	if len(v)%2 == 1 {
		return v[mid]
	}
	return (v[mid-1] + v[mid]) / 2
}

// empiricalQuantile uses linear interpolation between ordered factual values.
// It is used only to describe the historical empirical spread; RC196 does not
// call this a statistical confidence interval.
func empiricalQuantile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	if p <= 0 {
		return v[0]
	}
	if p >= 1 {
		return v[len(v)-1]
	}
	pos := p * float64(len(v)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return v[lo]
	}
	frac := pos - float64(lo)
	return v[lo] + frac*(v[hi]-v[lo])
}

func mrpEvidenceLevel(n int) string {
	switch {
	case n >= 5:
		return "достаточная история"
	case n >= 3:
		return "ограниченная история"
	case n >= 1:
		return "низкая достаточность"
	default:
		return "фактическая история отсутствует"
	}
}

func normalizedText(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func mrpObservationComparability(current MRPForecastInput, o MRPObservation) string {
	s := o.Snapshot
	if strings.TrimSpace(s.Equipment) == "" {
		s.Equipment = o.Equipment
	}
	if strings.TrimSpace(s.Complication) == "" {
		s.Complication = o.Complication
	}
	known := 0
	mismatch := false
	if normalizedText(s.Equipment) != "" && normalizedText(current.Equipment) != "" {
		known++
		if normalizedText(s.Equipment) != normalizedText(current.Equipment) {
			mismatch = true
		}
	}
	if normalizedText(s.Complication) != "" && normalizedText(current.Complication) != "" {
		known++
		if normalizedText(s.Complication) != normalizedText(current.Complication) {
			mismatch = true
		}
	}
	if mismatch {
		return "несопоставимо"
	}
	if known >= 2 {
		return "сопоставимо"
	}
	return "недостаточно данных"
}

func mrpTimeSplitBacktest(obs []MRPObservation) MRPBacktestSummary {
	valid := validMRPObservations(obs)
	if len(valid) < 6 {
		return MRPBacktestSummary{Eligible: false, TrainingCount: maxInt(0, len(valid)-1), TestCount: 0, Note: "Для последовательной проверки по времени требуется минимум 6 завершённых МРП одного объекта; количественную модель факторов не включать."}
	}
	errs := make([]float64, 0, len(valid)-5)
	for i := 5; i < len(valid); i++ {
		train := make([]float64, 0, i)
		for _, o := range valid[:i] {
			train = append(train, o.MRPDays)
		}
		pred := medianFloat64(train)
		errs = append(errs, math.Abs(pred-valid[i].MRPDays))
	}
	return MRPBacktestSummary{Eligible: true, TrainingCount: 5, TestCount: len(errs), MedianAbsError: medianFloat64(errs), Note: "Последовательная проверка по времени: каждый факт тестируется только на более ранней истории. Это проверка базового расчёта, а не разрешение численных весов факторов."}
}

func mrpForecastAuditsForObject(objectKey string) []MRPForecastAudit {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return nil
	}
	out := make([]MRPForecastAudit, 0)
	actualByVersion := map[string]Outcome{}
	for _, o := range outcomes {
		if toFloat(o.ActualMRP) > 0 {
			actualByVersion[o.VersionID] = o
		}
	}
	for _, v := range versions {
		if normalizeLaboratoryObjectKey(v.ObjectKey) != key {
			continue
		}
		base := toFloat(v.MRPBase)
		if base <= 0 {
			base = toFloat(v.ForecastMRP)
		}
		if base <= 0 {
			continue
		}
		a := MRPForecastAudit{VersionID: v.VersionID, ForecastDate: parseDate(v.Created), BaseDays: base}
		if o, ok := actualByVersion[v.VersionID]; ok {
			a.ActualDate = parseDate(o.Date)
			a.ActualMRPDays = toFloat(o.ActualMRP)
			a.ErrorDays = base - a.ActualMRPDays
			a.AbsoluteError = math.Abs(a.ErrorDays)
		}
		out = append(out, a)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ForecastDate.Before(out[j].ForecastDate) })
	return out
}

func calculateMRPForecast(in MRPForecastInput) MRPForecastResult {
	obs := validMRPObservations(in.Observations)
	r := MRPForecastResult{
		ModelVersion:     mrpForecastModelVersion,
		ObjectKey:        strings.TrimSpace(in.ObjectKey),
		CurrentMRPDays:   in.CurrentMRPDays,
		ObservationCount: len(obs),
		EvidenceLevel:    mrpEvidenceLevel(len(obs)),
		Status:           "предварительный прогноз; требуется инженерная проверка",
		TargetMRPDays:    normalizedTargetRunlifeDays(in.TargetMRPDays),
		TargetSource:     strings.TrimSpace(in.TargetSource),
		TargetScope:      strings.TrimSpace(in.TargetScope),
		TargetLiftType:   strings.TrimSpace(in.TargetLiftType),
		TargetNearPct:    normalizedTargetNearPct(in.TargetNearPct),
	}

	for _, o := range obs {
		switch mrpObservationComparability(in, o) {
		case "сопоставимо":
			r.ComparableCount++
		case "несопоставимо":
			r.NonComparableCount++
		default:
			r.UnknownComparable++
		}
	}
	r.Backtest = mrpTimeSplitBacktest(obs)
	comparableValues := make([]float64, 0, len(obs))
	for _, o := range obs {
		if mrpObservationComparability(in, o) == "сопоставимо" {
			comparableValues = append(comparableValues, o.MRPDays)
		}
	}
	if len(comparableValues) >= 2 {
		r.ComparableBaseDays = medianFloat64(comparableValues)
		r.HasComparableBase = true
	}
	values := make([]float64, 0, len(obs))
	for _, o := range obs {
		values = append(values, o.MRPDays)
	}
	if len(obs) > 0 {
		latest := obs[len(obs)-1]
		r.LatestObservation = latest.Date
		r.LatestActualMRPDays = latest.MRPDays
		r.BaseDays = medianFloat64(values)
		switch {
		case len(values) >= 5:
			r.LowDays = empiricalQuantile(values, 0.25)
			r.HighDays = empiricalQuantile(values, 0.75)
			r.HasLowHigh = true
			r.Basis = append(r.Basis, "Базовая оценка = медиана фактических МРП этого объекта; нижний/верхний исторический ориентир = 25-й/75-й процентили фактической истории.")
		case len(values) >= 2:
			sorted := append([]float64(nil), values...)
			sort.Float64s(sorted)
			r.LowDays, r.HighDays = sorted[0], sorted[len(sorted)-1]
			r.HasLowHigh = true
			r.Basis = append(r.Basis, "Базовая оценка = медиана фактических МРП; из-за малого числа наблюдений нижняя/верхняя границы показывают только фактический минимум/максимум, а не доверительный интервал.")
		default:
			r.BaseDays = values[0]
			r.Basis = append(r.Basis, "Есть только один фактический МРП: он используется как ориентир без расчёта низкого/высокого сценария.")
		}
		if !in.CurrentDate.IsZero() && !latest.Date.IsZero() {
			age := int(in.CurrentDate.Sub(latest.Date).Hours() / 24)
			if age >= 0 {
				r.Basis = append(r.Basis, fmt.Sprintf("Последний фактический МРП зарегистрирован %d сут. назад.", age))
				if age > 730 {
					r.Warnings = append(r.Warnings, "Последний фактический МРП старше двух лет; применимость исторического базиса к текущему режиму нужно подтвердить.")
				}
			}
		}
	} else if in.CurrentMRPDays > 0 {
		r.BaseDays = in.CurrentMRPDays
		r.Basis = append(r.Basis, "Фактических завершённых МРП в истории нет; базовый ориентир временно принят равным текущему МРП из Техрежима.")
		r.Warnings = append(r.Warnings, "Исторические нижняя/верхняя границы не рассчитаны: недостаточно фактических завершённых МРП.")
	} else {
		r.Status = "прогноз не сформирован"
		r.Warnings = append(r.Warnings, "Нет ни текущего МРП, ни фактической истории завершённых МРП по объекту.")
	}

	// Историческая база показывается отдельно от численного прогноза: смена ГНО,
	// ранее применявшиеся методы и полнота снимков режима не получают скрытых весов.
	equipmentSet := map[string]bool{}
	methodSet := map[string]bool{}
	complicationSet := map[string]bool{}
	regimeSnapshots := 0
	for _, o := range obs {
		if v := strings.TrimSpace(o.Equipment); v != "" {
			equipmentSet[v] = true
		}
		if v := strings.TrimSpace(o.Method); v != "" {
			methodSet[v] = true
		}
		if v := strings.TrimSpace(o.Complication); v != "" {
			complicationSet[v] = true
		}
		snap := o.Snapshot
		if strings.TrimSpace(snap.Equipment) != "" || strings.TrimSpace(snap.Complication) != "" || strings.TrimSpace(snap.WaterCut) != "" || strings.TrimSpace(snap.LiquidRate) != "" || strings.TrimSpace(snap.BottomholePressure) != "" || strings.TrimSpace(snap.DynamicLevel) != "" {
			regimeSnapshots++
		}
	}
	if len(equipmentSet) > 1 {
		r.Basis = append(r.Basis, fmt.Sprintf("В истории есть смена ГНО: %d вариантов. Сопоставимость циклов проверяется отдельно; разные эпохи эксплуатации не смешиваются скрытым коэффициентом.", len(equipmentSet)))
	} else if len(equipmentSet) == 1 {
		r.Basis = append(r.Basis, "Исторические циклы содержат один зафиксированный тип ГНО; смена оборудования по доступной истории не выявлена.")
	}
	if len(methodSet) > 0 {
		r.Basis = append(r.Basis, fmt.Sprintf("В завершённых циклах зафиксировано применявшихся методов/комплексов: %d. Их наличие показывается как контекст; численный эффект переносится только через подтверждённый сценарий методов.", len(methodSet)))
	}
	if len(complicationSet) > 0 {
		r.Basis = append(r.Basis, fmt.Sprintf("В истории зафиксировано классов/типов осложнений: %d; текущая классификация сопоставляется с каждым циклом.", len(complicationSet)))
	}
	if len(obs) > 0 {
		r.Basis = append(r.Basis, fmt.Sprintf("Снимок режима для проверки сопоставимости доступен по %d из %d завершённых циклов.", regimeSnapshots, len(obs)))
	}

	if r.BaseDays > 0 && in.CurrentMRPDays > 0 {
		delta := (r.BaseDays/in.CurrentMRPDays - 1) * 100
		r.Factors = append(r.Factors, MRPForecastFactor{
			Name:     "История МРП",
			Value:    fmt.Sprintf("медиана %.0f сут.; текущий %.0f сут.; разница %+.1f%%", r.BaseDays, in.CurrentMRPDays, delta),
			Evidence: "фактические ремонты / Техрежим",
			Note:     "Разница описательная; коэффициент влияния на будущий МРП не назначается без калибровки.",
		})
	}
	context := []struct{ name, value string }{
		{"ГНО", strings.TrimSpace(in.Equipment)},
		{"Осложнение", strings.TrimSpace(in.Complication)},
		{"Обводнённость", strings.TrimSpace(in.WaterCut)},
		{"Дебит жидкости", strings.TrimSpace(in.LiquidRate)},
		{"Дебит нефти", strings.TrimSpace(in.OilRate)},
		{"Забойное давление", strings.TrimSpace(in.BottomholePress)},
		{"Динамический уровень", strings.TrimSpace(in.DynamicLevel)},
		{"Температура ТМС", strings.TrimSpace(in.Temperature)},
	}
	for _, c := range context {
		if c.value == "" {
			continue
		}
		r.Factors = append(r.Factors, MRPForecastFactor{Name: c.name, Value: c.value, Evidence: "текущий режим", Note: "Показатель отображается как контекст; RC194 не приписывает ему численный вклад без валидированной модели."})
	}
	if len(in.LabClasses) > 0 {
		r.Factors = append(r.Factors, MRPForecastFactor{Name: "Лабораторные классы/признаки", Value: strings.Join(in.LabClasses, ", "), Evidence: "сохранённое заключение", Note: "Только объясняющий контекст. Численные веса запрещены до последовательной проверки по времени и отдельной калибровки."})
	}
	audits := mrpForecastAuditsForObject(in.ObjectKey)
	r.AuditCount = len(audits)
	auditErrors := make([]float64, 0, len(audits))
	for _, a := range audits {
		if a.ActualMRPDays > 0 {
			r.AuditActualCount++
			auditErrors = append(auditErrors, a.AbsoluteError)
		}
	}
	if len(auditErrors) > 0 {
		r.AuditMedianAbsError = medianFloat64(auditErrors)
	}
	r.EconomicsReady = r.BaseDays > 0 && r.CurrentMRPDays > 0
	if strings.TrimSpace(in.Method) != "" {
		r.MethodContext = strings.TrimSpace(in.Method)
		r.Factors = append(r.Factors, MRPForecastFactor{Name: "Выбранный метод", Value: r.MethodContext, Evidence: "последняя сохранённая версия", Note: "Метод отображается как контекст и не изменяет базовый прогноз МРП без подтверждённой фактической эффективности."})
	}
	if r.NonComparableCount > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d исторических наблюдений имеют отличающееся ГНО/класс осложнения и требуют отдельной инженерной сверки.", r.NonComparableCount))
	}
	if r.UnknownComparable > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("Для %d наблюдений недостаточно сохранённого снимка режима для подтверждения сопоставимости.", r.UnknownComparable))
	}
	r.Advanced = calculateMRPAdvanced(in)
	if lo, base, hi, ok, source := mrpSurvivalEconomicTriplet(r.Advanced); ok {
		r.SurvivalLowDays, r.SurvivalBaseDays, r.SurvivalHighDays = lo, base, hi
		r.HasSurvivalTriplet = true
		r.SurvivalEconSource = source
	}
	r.TargetForecastDays = r.BaseDays
	if r.HasSurvivalTriplet && r.SurvivalBaseDays > 0 {
		r.TargetForecastDays = r.SurvivalBaseDays
	}
	if r.TargetForecastDays > 0 {
		r.TargetGapDays, r.TargetRatioPct, r.TargetStatus = targetRunlifeComparisonWithTolerance(r.TargetForecastDays, r.TargetMRPDays, r.TargetNearPct)
		r.Factors = append(r.Factors, MRPForecastFactor{
			Name:     "Целевой ориентир наработки",
			Value:    fmt.Sprintf("%.0f сут.; основной прогноз %.0f сут.; %s", r.TargetMRPDays, r.TargetForecastDays, r.TargetStatus),
			Evidence: "настраиваемый инженерный ориентир",
			Note:     fmt.Sprintf("Источник: %s. Допуск статуса «вблизи»: ±%.0f%%. Ориентир не является жёстким нормативом и не подменяет статистический прогноз МРП.", dash(r.TargetSource), r.TargetNearPct),
		})
	}
	return r
}

func mrpObservationHistoryForObject(objectKey string) []MRPObservation {
	key := normalizeLaboratoryObjectKey(objectKey)
	if key == "" {
		return nil
	}
	versionByID := make(map[string]VersionRecord, len(versions))
	for _, v := range versions {
		versionByID[v.VersionID] = v
	}
	out := make([]MRPObservation, 0)
	seen := map[string]bool{}
	for _, o := range outcomes {
		v, ok := versionByID[o.VersionID]
		if !ok || normalizeLaboratoryObjectKey(v.ObjectKey) != key {
			continue
		}
		mrp, durationSource, durationWarning, cycleStart, cycleEnd := mrpDurationFromOutcome(o)
		if mrp <= 0 {
			continue
		}
		d := parseDate(o.Date)
		if !cycleEnd.IsZero() {
			d = cycleEnd
		}
		fingerprint := fmt.Sprintf("%s|%.6f|%s", d.Format("2006-01-02"), mrp, o.VersionID)
		if seen[fingerprint] {
			continue
		}
		seen[fingerprint] = true
		eventType, planned, failureMode, classSource, confirmed := mrpOutcomeClassification(o)
		out = append(out, MRPObservation{
			ObjectKey: v.ObjectKey, Field: v.Field, Well: v.Well, ObjectPlast: v.ObjectPlast, Date: d, CycleStartDate: cycleStart, CycleEndDate: cycleEnd, MRPDays: mrp, DurationSource: durationSource, DurationWarning: durationWarning, Source: "Фактический результат", VersionID: v.VersionID, Equipment: v.Equipment, Method: v.Method, Complication: v.Complication,
			EventType: eventType, Planned: planned, FailureMode: failureMode, ClassificationSource: classSource, ClassificationConfirmed: confirmed,
			Snapshot: MRPRegimeSnapshot{CapturedAt: d, Equipment: v.Equipment, Complication: v.Complication, Completeness: "частичный снимок: ГНО и класс осложнения из сохранённой версии; режимные численные параметры старых версий не восстанавливаются задним числом"},
		})
	}
	if len(out) == 0 {
		out = teachingWorkflowObservations(objectKey)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func mrpForecastInputForSelectedObject() (MRPForecastInput, bool) {
	st, ok := currentSelectedState()
	if !ok {
		return MRPForecastInput{}, false
	}
	// Для прогноза приоритет имеют текущие решения технолога: утверждённая
	// классификация из раздела 3 и утверждённый метод из раздела 4.
	// Сохранённая версия остаётся fallback для ранее накопленных данных.
	complication := approvedPrimaryComplication(st.ObjectKey)
	method, _ := latestApprovedMethodForObject(st.ObjectKey)
	if v, found := latestVersionForSelectedObject(); found {
		if strings.TrimSpace(complication) == "" {
			complication = v.Complication
		}
		if strings.TrimSpace(method) == "" {
			method = v.Method
		}
	}
	target := targetRunlifeResolved(st.ObjectKey, st.Equipment)
	return MRPForecastInput{
		ObjectKey:       st.ObjectKey,
		CurrentMRPDays:  toFloat(st.CurrentMRP),
		CurrentDate:     time.Now(),
		Observations:    mrpObservationHistoryForObject(st.ObjectKey),
		Equipment:       st.Equipment,
		Complication:    complication,
		WaterCut:        st.WaterCut,
		LiquidRate:      st.LiquidRate,
		OilRate:         st.OilRate,
		BottomholePress: st.BottomholePressure,
		DynamicLevel:    st.DynamicLevel,
		Temperature:     st.TMSTemperature,
		LabClasses:      nonEmptyStrings(complication),
		Method:          method,
		TargetMRPDays:   target.Days,
		TargetSource:    target.Source,
		TargetScope:     target.Scope,
		TargetLiftType:  target.LiftType,
		TargetNearPct:   target.NearPct,
	}, true
}

func nonEmptyStrings(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return []string{strings.TrimSpace(v)}
}

func formatMRPForecast(r MRPForecastResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Модель: %s\r\nСтатус: %s\r\nДостаточность фактической истории: %s\r\nФактических завершённых МРП: %d\r\n", mrpUserModelName(r.ModelVersion), r.Status, r.EvidenceLevel, r.ObservationCount)
	if r.CurrentMRPDays > 0 {
		fmt.Fprintf(&b, "Текущий МРП из Техрежима: %.0f сут.\r\n", r.CurrentMRPDays)
	}
	if r.BaseDays > 0 {
		if r.ObservationCount < 3 {
			fmt.Fprintf(&b, "Предварительный ориентир (прогноз не подтверждён): %.0f сут.\r\n", r.BaseDays)
		} else {
			fmt.Fprintf(&b, "Базовый прогноз следующего МРП: %.0f сут.\r\n", r.BaseDays)
		}
	}
	fmt.Fprintf(&b, "%s\r\n", targetRunlifeSummary(r.TargetForecastDays, r.TargetMRPDays))
	if strings.TrimSpace(r.TargetSource) != "" {
		fmt.Fprintf(&b, "Источник целевого ориентира: %s. Допуск статуса «вблизи»: ±%.0f%%.\r\n", r.TargetSource, r.TargetNearPct)
	}
	if r.HasLowHigh {
		fmt.Fprintf(&b, "Исторический нижний / верхний ориентир (справочно): %.0f / %.0f сут.\r\n", r.LowDays, r.HighDays)
	} else {
		b.WriteString("Исторические нижняя / верхняя границы не рассчитаны из-за недостатка фактических наблюдений.\r\n")
	}
	if !r.LatestObservation.IsZero() {
		fmt.Fprintf(&b, "Последний факт: %s — %.0f сут.\r\n", r.LatestObservation.Format("02.01.2006"), r.LatestActualMRPDays)
	}
	fmt.Fprintf(&b, "Сопоставимость истории: сопоставимо %d; отличается %d; недостаточно снимка %d.\r\n", r.ComparableCount, r.NonComparableCount, r.UnknownComparable)
	if r.HasComparableBase {
		fmt.Fprintf(&b, "Ориентир только по сопоставимой истории: медиана %.0f сут. (справочно; основной базовый расчёт автоматически не заменяется).\r\n", r.ComparableBaseDays)
	}
	if r.AuditCount > 0 {
		fmt.Fprintf(&b, "Аудит прогноз → факт: сохранено прогнозов %d; с подтверждённым фактом %d", r.AuditCount, r.AuditActualCount)
		if r.AuditActualCount > 0 {
			fmt.Fprintf(&b, "; медианная абсолютная ошибка %.0f сут.", r.AuditMedianAbsError)
		}
		b.WriteString("\r\n")
	}
	if r.EconomicsReady {
		b.WriteString("Передача в экономику: базовые данные МРП присутствуют; сценарии должны сохранять происхождение и уровень достаточности истории.\r\n")
	} else {
		b.WriteString("Передача в экономику: недостаточно данных для расчёта на основе МРП.\r\n")
	}
	if r.Backtest.Eligible {
		fmt.Fprintf(&b, "Последовательная проверка базового расчёта по времени: %d тестовых точек; медианная абсолютная ошибка %.0f сут.\r\n", r.Backtest.TestCount, r.Backtest.MedianAbsError)
	} else {
		fmt.Fprintf(&b, "Последовательная проверка по времени пока не активирована. %s\r\n", r.Backtest.Note)
	}
	if len(r.Basis) > 0 {
		b.WriteString("\r\nОСНОВАНИЕ\r\n")
		for _, x := range r.Basis {
			b.WriteString("• " + x + "\r\n")
		}
	}
	if len(r.Factors) > 0 {
		b.WriteString("\r\nФАКТОРЫ И КОНТЕКСТ\r\n")
		for _, f := range r.Factors {
			fmt.Fprintf(&b, "• %s: %s [%s]. %s\r\n", f.Name, f.Value, f.Evidence, f.Note)
		}
	}
	if len(r.Warnings) > 0 {
		b.WriteString("\r\nОГРАНИЧЕНИЯ\r\n")
		for _, w := range r.Warnings {
			b.WriteString("• " + w + "\r\n")
		}
	}
	b.WriteString(formatMRPAdvanced(r.Advanced))
	b.WriteString("\r\n\r\nВажно: прозрачный базовый расчёт сохраняется как контрольная линия, а анализ безотказной работы с учётом незавершённых циклов выполняется отдельно. Лабораторные и режимные показатели не получают численных весов без отдельной калибровки на подтверждённой истории отказов и ремонтов и последовательной проверки по времени.")
	return b.String()
}
