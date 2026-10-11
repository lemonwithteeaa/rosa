//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

type teachingCrossHistoryPoint struct {
	Index          int
	WellOrdinal    int
	Date           time.Time
	LabDate        time.Time
	CycleStartDate time.Time
	RepairEndDate  time.Time
	FailureID      string
	LabPassID      string
	MethodBefore   string
	DecisionType   string
	Equipment      string
	MRPDays        int
	Direction      string
	Classification string
	Complication   string
	Cause          string
	FailedUnit     string
	Method         string
	LabGroupCode   string
	LabGroupName   string
	LabValues      map[string]string
	ForecastLow    float64
	ForecastBase   float64
	ForecastHigh   float64
}

func teachingAssessmentRecord(objectKey string, d assessmentClassDirection) (AssessmentClassRecord, bool) {
	if _, ok := teachingWorkflowOrdinal(objectKey); !ok {
		return AssessmentClassRecord{}, false
	}
	classes := map[string]string{
		"АСПО": "Не относится", "Коррозия": "К1", "Высокая вязкость": "Не относится",
		"Солеотложения": "С2", "Свободный газ": "Не относится", "Механические примеси": "М2",
		"Гидраты": "Не относится", "Высокая температура": "Не относится",
	}
	class := classes[d.Name]
	if scenario, full := teachingFullScenarioForObject(objectKey); full {
		class = scenario.ClassByDirection[d.Name]
		if class == "" {
			class = "Не относится"
		}
	}
	if class == "" {
		class = "Не относится"
	}
	return AssessmentClassRecord{
		ObjectKey: objectKey, Direction: d.Name, Prefix: d.Prefix,
		TechnologistClass: class, TechnologistComment: "Сквозной учебный пример: класс заполнен для проверки маршрута.",
		ProgramClassAtDecision: class, ProgramReasonAtDecision: "Синтетические лабораторные показатели и история отказов учебного контура.",
		FinalClass: class, FinalComment: "Утверждено в сквозном учебном примере; не является производственным заключением.",
		DecisionMode: "без корректировки", ConfirmedBy: "Технолог · учебный контур",
		ConfirmedAt: "04.10.2026 12:00", UpdatedAt: "2026-10-04T12:00:00Z", FullClassificationConfirmed: true,
	}, true
}

func teachingLatestLabRecord(objectKey, groupCode string, now time.Time) (*LabRecord, bool) {
	points := teachingCrossHistoryPoints(objectKey, now)
	for i := len(points) - 1; i >= 0; i-- {
		point := points[i]
		if point.LabGroupCode != groupCode {
			continue
		}
		return &LabRecord{
			ID: fmt.Sprintf("teaching-active-lab-%d", point.Index), ObjectKey: objectKey,
			Date: point.Date.Format("02.01.2006"), GroupCode: groupCode, GroupName: point.LabGroupName,
			Source: "УЧЕБНЫЙ ПРИМЕР · единый сквозной таймлайн", Values: cloneAssessmentLabValues(point.LabValues),
			Units: map[string]string{}, ScaleModel: "Oddo-Thomson",
			Comment: "Синтетические данные для проверки всех разделов РОСА.", Conclusion: point.Complication,
			CreatedAt: point.Date.Format(time.RFC3339),
		}, true
	}
	return nil, false
}

func teachingActiveMethodDecision(objectKey, classification string) (approvedMethodDecision, bool) {
	ordinal, ok := teachingWorkflowOrdinal(objectKey)
	if !ok {
		return approvedMethodDecision{}, false
	}
	methods := []string{"Ингибитор коррозии (дозирование / закачка)", "Ингибитор солеотложений (дозирование / закачка)", "Фильтр / сепаратор мехпримесей"}
	low, base, high := float64(980), float64(1050), float64(1125)
	interaction := "Комбинация реагентной защиты и механической защиты применяется последовательно; контроль эффективности ведётся по лабораторным показателям и фактической наработке."
	if scenario, full := teachingFullScenarioForObject(objectKey); full {
		methods = scenario.Methods
		low, base, high = scenario.ForecastLow, scenario.ForecastBase, scenario.ForecastHigh
		switch scenario.Ordinal {
		case 1:
			interaction = "Профилактическое ингибирование снижает риск нового осадка; промывка удаляет ранее сформированные отложения; НКТ с покрытием рассматриваются как дополнительная ресурсная защита. Эффекты не складываются линейно."
		case 58:
			interaction = "Ингибитор коррозии — базовое мероприятие; контроль кислорода и состояния металла используется как обратная связь. Покрытие НКТ не заменяет контроль коррозионной среды."
		case 90:
			interaction = "Растворитель АСПО снимает/предупреждает органические отложения, деэмульгатор снижает устойчивость эмульсии и вязкостную нагрузку; совместимость проверяется лабораторно до ОПИ."
		}
	}
	assessments := make([]MethodEffectAssessment, 0, len(methods))
	implementation := make([]MethodImplementationRecord, 0, len(methods))
	plan := make([]PlanItem, 0, len(methods))
	for i, method := range methods {
		if a, exists := teachingMethodEffect(method); exists {
			assessments = append(assessments, a)
		}
		impl := methodImplementationSnapshotForName(objectKey, method)
		impl.UpdatedAt = "06.10.2026 10:20"
		impl.UpdatedBy = "Технолог · учебный контур"
		implementation = append(implementation, impl)
		plan = append(plan, PlanItem{
			ID: fmt.Sprintf("teaching-plan-%03d-%d", ordinal, i+1), ObjectKey: objectKey, MethodName: method,
			Action: "Выполнить мероприятие по утверждённой карточке метода и проконтролировать результат",
			Basis:  "Утверждённая классификация + лабораторный паспорт + история отказов 36 мес.", Status: "Запланировано",
			DueDate: fmt.Sprintf("%02d.10.2026", 15+i*7), Responsible: "Технолог / производственный участок",
			ControlDate: fmt.Sprintf("%02d.11.2026", 15+i*7), ControlCriterion: "Лабораторный контроль + отсутствие повторного проявления + динамика МРП",
			ControlComment: "Учебный план контроля; фактические параметры должны быть подтверждены перед ОПИ.", ParametersRef: "Карточка параметров реализации метода",
			UpdatedAt: "06.10.2026 10:20", UpdatedBy: "Технолог · учебный контур",
		})
	}
	return approvedMethodDecision{
		ID: fmt.Sprintf("teaching-active-method-%03d", ordinal), ObjectKey: objectKey, ApprovedAt: "06.10.2026 10:30",
		ApprovedBy: "Технолог · учебный контур", Classification: classification,
		Method: strings.Join(methods, " + "), Methods: methods, MethodStatus: "УТВЕРЖДЁН · УЧЕБНЫЙ ПРИМЕР",
		MethodSource: "Утверждённая классификация + история отказов + полный лабораторный паспорт",
		Interaction:  interaction, MethodAssessments: assessments, Implementation: implementation, Plan: plan,
		FailureCount36: 4, HistoryCycleCount: 4, ForecastPrecision: "учебная проверка полного маршрута; 4 синтетических цикла",
		HistoryWarning:  "Синтетические данные; не переносить проценты и параметры в производственное решение без верификации.",
		ForecastLowDays: low, ForecastBaseDays: base, ForecastHighDays: high,
		ForecastSource: "УЧЕБНЫЙ ПРИМЕР · выбранная параллельная схема · лабораторный паспорт + история 4 циклов + целевой МРП 1000 сут.",
		SchemaVersion:  6, VersionNumber: 2,
	}, true
}

func teachingCompletedVersionForObject(objectKey string) (VersionRecord, bool) {
	state, ok := teachingWorkflowStateForObject(objectKey)
	if !ok {
		return VersionRecord{}, false
	}
	versionID, complication, method := "ROSA-TEACHING-001-COMPLETE", "Солеотложения С2 + коррозия К1 + механические примеси М2", "Ингибитор коррозии + ингибитор солеотложений + фильтр / сепаратор мехпримесей"
	low, base, high := "980", "1050", "1125"
	if scenario, full := teachingFullScenarioForObject(objectKey); full {
		versionID = fmt.Sprintf("ROSA-TEACHING-%03d-COMPLETE", scenario.Ordinal)
		complication, method = scenario.Name, strings.Join(scenario.Methods, " + ")
		low, base, high = fmt.Sprintf("%.0f", scenario.ForecastLow), fmt.Sprintf("%.0f", scenario.ForecastBase), fmt.Sprintf("%.0f", scenario.ForecastHigh)
	}
	return VersionRecord{
		VersionID: versionID, ObjectKey: objectKey, Created: "04.10.2026 12:30",
		Field: state.Field, Well: state.Well, Deposit: state.Deposit, ObjectPlast: state.ObjectPlast,
		Complication: complication,
		Method:       method, Equipment: state.Equipment,
		ForecastMRP: base, MRPLow: low, MRPBase: base, MRPHigh: high, Scenario: "Центральный",
		CurrentMRP: state.CurrentMRP, LabStatus: "Заполнено и проверено · учебный пример",
		EconMode: "Базовая", EconStatus: "Рассчитано · учебный пример",
		EconResult: fmt.Sprintf("Целевой МРП 1000 сут. достигнут; центральный прогноз %s сут.", base),
		Decision:   "Принято", DecisionComment: "Сквозной пример принят во всех разделах; параллельные схемы сохранены для сравнения.",
		Responsible: "Технолог · учебный контур", SourceFile: "Встроенный сквозной пример РОСА", Archived: true,
		Quarter: "4 кв. 2026", ImportedAt: "2026-10-04T12:30:00Z",
	}, true
}

func teachingOutcomeForVersion(versionID string) (*Outcome, bool) {
	if versionID != "ROSA-TEACHING-001-COMPLETE" && versionID != "ROSA-TEACHING-058-COMPLETE" && versionID != "ROSA-TEACHING-090-COMPLETE" {
		return nil, false
	}
	return &Outcome{
		ID: "teaching-outcome-001", VersionID: versionID, Date: "04.10.2026",
		CycleStartDate: "20.11.2023", CycleEndDate: "15.08.2026", ActualMRP: "1000", ActualCost: "1680",
		Result: "Целевой МРП достигнут", Comment: "Синтетический факт для проверки сквозного отчёта.",
		EventType: "Отказ", FailureMode: "Загрязнение приёмной части и продукты коррозии",
		ClassificationSource: "Учебная дефектовка и лабораторная расшифровка", ClassificationConfirmed: true,
		FailureCause:   "Солеотложения, коррозия и механические примеси",
		DefectFinding:  "Карбонатные отложения, продукты коррозии и механические частицы подтверждены учебным актом.",
		EvidenceSource: "УЧЕБНЫЙ АКТ ТКРС-001", EvidenceComment: "Полный комплект синтетических подтверждений.",
		NextCycleStartDate: "20.08.2026", NextCycleStatus: "Запущен после ТКРС",
		ObservedEffect: "Наработка увеличена до целевого уровня", EffectMetric: "МРП, сут.",
		EffectBefore: "850", EffectAfter: "1000", EffectAttribution: "Учебная связь; причинность автоматически не утверждается.",
	}, true
}

// teachingCrossHistoryPoints is the single synthetic timeline used by the
// histories for the current 20-well training corpus.
func teachingCrossHistoryPoints(objectKey string, now time.Time) []teachingCrossHistoryPoint {
	well, ok := trainingWellForObject(objectKey)
	if !ok {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	out := make([]teachingCrossHistoryPoint, 0, len(well.Failures))
	for i, e := range well.Failures {
		lab := well.LabPasses[i]
		dateValue := trainingFailureDate(e.Date)
		labDate := trainingFailureDate(lab.Date)
		start := trainingFailureDate(e.CycleStart)
		repair := trainingFailureDate(e.RepairEnd)
		if dateValue.After(now) {
			continue
		}
		g, hasGroup := labGroupByCode(e.LabGroup)
		values := map[string]string{}
		if hasGroup {
			for _, f := range g.Fields {
				values[f.Key] = teachingLabValue(f.Key, f.Unit, well, e)
			}
		}
		p := teachingCrossHistoryPoint{Index: i + 1, WellOrdinal: well.Ordinal, Date: dateValue, LabDate: labDate, CycleStartDate: start, RepairEndDate: repair, FailureID: e.ID, LabPassID: lab.PassID, MethodBefore: e.MethodBefore, DecisionType: e.DecisionType, Equipment: e.Equipment, MRPDays: e.MRP, Direction: e.Complication, Classification: e.Classification, Complication: e.Complication, Cause: e.Cause, FailedUnit: e.FailedUnit, Method: e.MethodAfter, LabGroupCode: e.LabGroup, LabGroupName: e.Complication, LabValues: values}
		p.ForecastLow = float64(e.ResultingMRP - 20)
		p.ForecastBase = float64(e.ResultingMRP)
		p.ForecastHigh = float64(e.ResultingMRP + 25)
		out = append(out, p)
	}
	return out
}

func teachingAssessmentHistoryRows(objectKey string, now time.Time) []AssessmentDecisionRecord {
	points := teachingCrossHistoryPoints(objectKey, now)
	out := make([]AssessmentDecisionRecord, 0, len(points))
	for _, p := range points {
		out = append(out, AssessmentDecisionRecord{
			DecisionID: fmt.Sprintf("teaching-assessment-%d", p.Index), ObjectKey: objectKey,
			Direction: p.Direction, TechnologistClass: p.Classification, ProgramClass: p.Classification,
			FinalClass: p.Classification, DecisionMode: "Учебный пример",
			Comment: "Синтетическая история для проверки сквозного интерфейса; не производственное решение.",
			Reason:  p.Complication, Source: "УЧЕБНЫЙ ПРИМЕР · лаборатория + история отказов",
			ConfirmedBy: "РОСА · учебный контур", ConfirmedAt: p.Date.Format("02.01.2006"),
		})
	}
	return out
}

func teachingLaboratoryHistoryEntries(objectKey string, now time.Time) []laboratoryHistoryEntry {
	points := teachingCrossHistoryPoints(objectKey, now)
	out := make([]laboratoryHistoryEntry, 0, len(points))
	for _, p := range points {
		record := LabRecord{
			ID: fmt.Sprintf("teaching-lab-%d", p.Index), ObjectKey: objectKey, Date: p.Date.Format("02.01.2006"),
			GroupCode: p.LabGroupCode, GroupName: p.LabGroupName,
			Source: "УЧЕБНЫЙ ПРИМЕР · синтетическая лабораторная история",
			Values: p.LabValues, Units: map[string]string{}, ScaleModel: "Oddo-Thomson",
			Comment:    fmt.Sprintf("ГНО: %s; МРП: %d сут.; %s", p.Equipment, p.MRPDays, p.Complication),
			Conclusion: p.Complication + ". Учебная контрольная точка, не производственный анализ.",
			CreatedAt:  p.Date.Format(time.RFC3339),
		}
		out = append(out, laboratoryHistoryEntry{Kind: "teaching", Record: record, Version: record.ID, Date: record.Date, Created: record.CreatedAt})
	}
	return out
}

func teachingMethodArchiveDecisions(objectKey string, now time.Time) []approvedMethodDecision {
	points := teachingCrossHistoryPoints(objectKey, now)
	out := make([]approvedMethodDecision, 0, len(points))
	for _, p := range points {
		out = append(out, approvedMethodDecision{
			ID: fmt.Sprintf("teaching-method-%d", p.Index), ObjectKey: objectKey,
			ApprovedAt: p.Date.Format("02.01.2006"), ApprovedBy: "РОСА · учебный контур",
			Classification: p.Classification, Method: p.Method, Methods: strings.Split(p.Method, " + "),
			MethodStatus: "УЧЕБНЫЙ ПРИМЕР", ForecastLowDays: p.ForecastLow, ForecastBaseDays: p.ForecastBase, ForecastHighDays: p.ForecastHigh,
			ForecastSource: fmt.Sprintf("учебная предыстория: ГНО %s; МРП %d сут.; %s", p.Equipment, p.MRPDays, p.Complication),
			VersionNumber:  p.Index,
		})
	}
	return out
}
