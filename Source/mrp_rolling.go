//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

type MRPObjectRolling struct {
	ObjectKey          string
	Name               string
	Cycles             []MRPObservation
	TotalDays, MeanAll float64
	Mean3, Mean4       float64
}

// These are descriptive, completed failure cycles. They are deliberately
// separate from both the approved rolling-year fleet NNO and the forecast.
type MRPRollingRow struct {
	Method         string
	Cycles         []MRPObservation
	Objects        []string
	ObjectRows     []MRPObjectRolling
	TotalDays      float64
	MeanAll        float64
	Sum3, Sum4     float64
	Mean3, Mean4   float64
	Ready3, Ready4 int
}

// Rebuild method history in one pass. The selected example is the only
// synthetic fallback; other demo objects are included only when their actual
// result is present in the archive.
func mrpAllMethodObservations(in MRPForecastInput) []MRPObservation {
	_, selectedDemo := teachingWorkflowOrdinal(in.ObjectKey)
	versionByID := make(map[string]VersionRecord, len(versions))
	for _, v := range versions {
		versionByID[v.VersionID] = v
	}
	seen := map[string]bool{}
	rows := []MRPObservation{}
	add := func(o MRPObservation) {
		_, demo := teachingWorkflowOrdinal(o.ObjectKey)
		if demo != selectedDemo {
			return
		}
		if o.Date.IsZero() || o.MRPDays <= 0 {
			return
		}
		key := normalizeLaboratoryObjectKey(o.ObjectKey) + "|" + o.Date.Format("2006-01-02")
		if seen[key] {
			return
		}
		seen[key] = true
		rows = append(rows, o)
	}
	// A confirmed failure-card record is the closest source for the scheme
	// actually in effect before the failure.
	for _, e := range failureEvents {
		if e.Draft {
			continue
		}
		date, err := failureDate(e.Date)
		if err != nil {
			continue
		}
		start, err := failureDate(e.CycleStartDate)
		if err != nil || start.After(date) {
			continue
		}
		method := strings.TrimSpace(e.MethodBeforeFailure)
		if second := strings.TrimSpace(e.MethodBeforeFailure2); second != "" && second != method {
			if method == "" {
				method = second
			} else {
				method += " + " + second
			}
		}
		approved := e.ConclusionApproved
		if !approved || !failureHasValue(method) || normalizeMRPEventType(e.EventType, false) != "отказ" {
			continue
		}
		add(MRPObservation{ObjectKey: e.ObjectKey, Field: e.Field, Well: e.Well, ObjectPlast: e.ObjectPlast,
			Date: date, CycleStartDate: start, CycleEndDate: date, MRPDays: date.Sub(start).Hours() / 24,
			Method: method, Equipment: e.Equipment, Complication: e.PrimaryComplication, EventType: e.EventType,
			ClassificationConfirmed: approved, ClassificationSource: e.CommissionSource,
			Source: "Журнал отказов · " + dash(e.Source), VersionID: e.ID})
	}
	for _, o := range outcomes {
		v, ok := versionByID[o.VersionID]
		if !ok {
			continue
		}
		days, durationSource, durationWarning, start, end := mrpDurationFromOutcome(o)
		if days <= 0 {
			continue
		}
		date := parseDate(o.Date)
		if !end.IsZero() {
			date = end
		}
		eventType, planned, mode, source, confirmed := mrpOutcomeClassification(o)
		add(MRPObservation{ObjectKey: v.ObjectKey, Field: v.Field, Well: v.Well, ObjectPlast: v.ObjectPlast,
			Date: date, CycleStartDate: start, CycleEndDate: end, MRPDays: days, DurationSource: durationSource,
			DurationWarning: durationWarning, Method: v.Method, Equipment: v.Equipment, Complication: v.Complication,
			EventType: eventType, Planned: planned, FailureMode: mode, ClassificationSource: source,
			ClassificationConfirmed: confirmed, Source: "Фактический результат", VersionID: v.VersionID})
	}
	for _, o := range in.Observations {
		add(o)
	}
	return rows
}

func mrpRollingObjectName(o MRPObservation) string {
	parts := []string{}
	well := strings.TrimSpace(o.Well)
	if well != "" {
		well = "скв. " + well
	}
	for _, value := range []string{o.Field, well, o.ObjectPlast} {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, strings.TrimSpace(value))
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " / ")
	}
	return dash(o.ObjectKey)
}

func mrpRollingRows(in MRPForecastInput) (rows []MRPRollingRow, excluded []string) {
	byMethod := map[string]map[string][]MRPObservation{}
	_, demoObject := teachingWorkflowOrdinal(in.ObjectKey)
	for _, o := range validMRPObservations(mrpAllMethodObservations(in)) {
		teaching := demoObject && strings.Contains(o.Source, "УЧЕБНЫЙ ПРИМЕР")
		reason := ""
		switch {
		case normalizeMRPEventType(o.EventType, o.Planned) != "отказ":
			reason = "завершение не является отказом"
		case !o.ClassificationConfirmed && !teaching:
			reason = "тип события не подтверждён"
		case o.CycleStartDate.IsZero() || o.CycleEndDate.IsZero() || o.CycleEndDate.Before(o.CycleStartDate):
			reason = "нет корректных дат начала и окончания"
		case strings.TrimSpace(o.Method) == "":
			reason = "неизвестен способ до отказа"
		}
		if reason != "" {
			excluded = append(excluded, fmt.Sprintf("%s · %s · %.0f сут. · %s · %s", mrpRollingObjectName(o), o.Date.Format("02.01.2006"), o.MRPDays, dash(o.Method), reason))
			continue
		}
		method, key := strings.TrimSpace(o.Method), normalizeLaboratoryObjectKey(o.ObjectKey)
		if byMethod[method] == nil {
			byMethod[method] = map[string][]MRPObservation{}
		}
		byMethod[method][key] = append(byMethod[method][key], o)
	}
	for method, objects := range byMethod {
		row := MRPRollingRow{Method: method}
		for key, cycles := range objects {
			sort.SliceStable(cycles, func(i, j int) bool { return cycles[i].Date.After(cycles[j].Date) })
			object := MRPObjectRolling{ObjectKey: key, Name: mrpRollingObjectName(cycles[0]), Cycles: cycles}
			for _, o := range cycles {
				object.TotalDays += o.MRPDays
				row.Cycles = append(row.Cycles, o)
			}
			object.MeanAll = object.TotalDays / float64(len(cycles))
			row.TotalDays += object.TotalDays
			row.MeanAll += object.MeanAll
			if len(cycles) >= 3 {
				object.Mean3 = (cycles[0].MRPDays + cycles[1].MRPDays + cycles[2].MRPDays) / 3
				row.Sum3 += object.Mean3
				row.Ready3++
			}
			if len(cycles) >= 4 {
				object.Mean4 = (cycles[0].MRPDays + cycles[1].MRPDays + cycles[2].MRPDays + cycles[3].MRPDays) / 4
				row.Sum4 += object.Mean4
				row.Ready4++
			}
			row.Objects = append(row.Objects, object.Name)
			row.ObjectRows = append(row.ObjectRows, object)
		}
		row.MeanAll /= float64(len(row.ObjectRows))
		if row.Ready3 > 0 {
			row.Mean3 = row.Sum3 / float64(row.Ready3)
		}
		if row.Ready4 > 0 {
			row.Mean4 = row.Sum4 / float64(row.Ready4)
		}
		sort.Strings(row.Objects)
		sort.Slice(row.ObjectRows, func(i, j int) bool { return row.ObjectRows[i].Name < row.ObjectRows[j].Name })
		sort.SliceStable(row.Cycles, func(i, j int) bool { return row.Cycles[i].Date.After(row.Cycles[j].Date) })
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if len(rows[i].ObjectRows) == len(rows[j].ObjectRows) {
			return rows[i].Method < rows[j].Method
		}
		return len(rows[i].ObjectRows) > len(rows[j].ObjectRows)
	})
	return
}

func mrpRollingHeadline(in MRPForecastInput) string {
	rows, _ := mrpRollingRows(in)
	if len(rows) == 0 {
		return "Нет подтверждённых циклов с датами и способом до отказа."
	}
	ready := 0
	for _, row := range rows {
		if row.Mean3 > 0 {
			ready++
		}
	}
	if ready == 0 {
		return fmt.Sprintf("%d схемы · пока нет трёх отказов одной схемы. Состав — кнопка «Проверить расчёт».", len(rows))
	}
	first := rows[0]
	if first.Mean3 > 0 {
		name := []rune(first.Method)
		if len(name) > 30 {
			name = append(name[:29], '…')
		}
		return fmt.Sprintf("%s · последние 3: %.0f сут. · циклов: %d. Все схемы — в расшифровке.", string(name), first.Mean3, len(first.Cycles))
	}
	return fmt.Sprintf("Схем: %d · полный ряд из трёх есть у %d. Откройте расчёт по способам.", len(rows), ready)
}

func mrpRollingDetail(in MRPForecastInput) string {
	rows, excluded := mrpRollingRows(in)
	var b strings.Builder
	b.WriteString("НАРАБОТКА ПО СПОСОБАМ · состав расчёта\r\n")
	b.WriteString("Две ступени: (1) среднее всех подтверждённых отказов каждого объекта с этим способом; (2) среднее полученных средних по объектам с одинаковым весом. Скользящая: у каждого объекта последние 3 или 4 отказа, затем среднее по объектам с полным окном.\r\n")
	if _, teaching := teachingWorkflowOrdinal(in.ObjectKey); teaching {
		b.WriteString("УЧЕБНЫЕ ДАННЫЕ — отдельно от производственных.\r\n")
	}
	b.WriteString("Отбор: подтверждённый отказ с датами цикла и известным способом ДО отказа. Плановые и неподтверждённые ремонты исключаются. Разные методы или комплексы не сливаются.\r\n\r\n")
	if len(rows) == 0 {
		b.WriteString("Подходящих циклов нет.\r\n")
	}
	for _, row := range rows {
		fmt.Fprintf(&b, "СПОСОБ: %s · объектов %d · отказов %d\r\n", row.Method, len(row.ObjectRows), len(row.Cycles))
		fmt.Fprintf(&b, "Накопительная: сумма средних объектов / %d = %.1f / %d = %.1f сут. (Σ суток всех циклов: %.0f, это проверочная сумма, а не числитель второй ступени).\r\n", len(row.ObjectRows), row.MeanAll*float64(len(row.ObjectRows)), len(row.ObjectRows), row.MeanAll, row.TotalDays)
		if row.Ready3 > 0 {
			fmt.Fprintf(&b, "Скользящая 3: сумма средних последних трёх по %d объектам %.1f / %d = %.1f сут.\r\n", row.Ready3, row.Sum3, row.Ready3, row.Mean3)
		} else {
			b.WriteString("Скользящая 3: нет объектов с тремя отказами по способу.\r\n")
		}
		if row.Ready4 > 0 {
			fmt.Fprintf(&b, "Скользящая 4: сумма средних последних четырёх по %d объектам %.1f / %d = %.1f сут.\r\n", row.Ready4, row.Sum4, row.Ready4, row.Mean4)
		} else {
			b.WriteString("Скользящая 4: нет объектов с четырьмя отказами по способу.\r\n")
		}
		for _, obj := range row.ObjectRows {
			fmt.Fprintf(&b, "\r\nОБЪЕКТ: %s · отказов %d · накоплено %.0f сут. / %d = %.1f сут.\r\n", obj.Name, len(obj.Cycles), obj.TotalDays, len(obj.Cycles), obj.MeanAll)
			if obj.Mean3 > 0 {
				fmt.Fprintf(&b, "  Последние 3: (%.0f + %.0f + %.0f) / 3 = %.1f сут.\r\n", obj.Cycles[0].MRPDays, obj.Cycles[1].MRPDays, obj.Cycles[2].MRPDays, obj.Mean3)
			}
			if obj.Mean4 > 0 {
				fmt.Fprintf(&b, "  Последние 4: %.0f / 4 = %.1f сут.\r\n", obj.Cycles[0].MRPDays+obj.Cycles[1].MRPDays+obj.Cycles[2].MRPDays+obj.Cycles[3].MRPDays, obj.Mean4)
			}
			running := float64(0)
			for i := len(obj.Cycles) - 1; i >= 0; i-- {
				o := obj.Cycles[i]
				running += o.MRPDays
				count := len(obj.Cycles) - i
				fmt.Fprintf(&b, "  %d. %s — %s · %.0f сут. · накопительная %.0f/%d=%.1f · %s / %s · %s · источник: %s · запись: %s\r\n", count, o.CycleStartDate.Format("02.01.2006"), o.CycleEndDate.Format("02.01.2006"), o.MRPDays, running, count, running/float64(count), dash(o.Equipment), dash(o.Complication), dash(o.ClassificationSource), dash(o.Source), dash(o.VersionID))
			}
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "ИСКЛЮЧЕНО: %d записей\r\n", len(excluded))
	for i, item := range excluded {
		if i >= 200 {
			fmt.Fprintf(&b, "… ещё %d исключённых записей.\r\n", len(excluded)-i)
			break
		}
		b.WriteString("  " + item + "\r\n")
	}
	b.WriteString("\r\nОфициальная ННО фонда за 12 месяцев отдельно: пока не рассчитана без помесячных Ti/O и учёта простоев. Исправляйте исходное событие с протоколом, не итоговое число.")
	return b.String()
}
