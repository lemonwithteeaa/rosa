//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

type teachingFailureProfile struct {
	Count         int
	Class         string
	Complication  string
	Causes        []string
	FailedUnits   []string
	MethodsBefore []string
	MethodsAfter  []string
}

var teachingFailureProfiles = map[int]teachingFailureProfile{
	1: {
		Count: 2, Class: "С2", Complication: "Солеотложения",
		Causes:        []string{"Рост перепада давления и признаки отложений", "Повторное снижение подачи после очистки"},
		FailedUnits:   []string{"НКТ / проточная часть", "Насосное оборудование"},
		MethodsBefore: []string{"Без постоянного ингибирования", "Периодическая промывка"},
		MethodsAfter:  []string{"Ингибитор солеотложений", "Ингибитор солеотложений + контроль совместимости"},
	},
	58: {
		Count: 3, Class: "К2", Complication: "Коррозия",
		Causes:        []string{"Коррозионное повреждение металла", "Локальная коррозия после роста обводнённости", "Повторное коррозионное повреждение"},
		FailedUnits:   []string{"НКТ", "Муфтовое соединение", "Корпус оборудования"},
		MethodsBefore: []string{"Без постоянной защиты", "Ингибитор коррозии", "Ингибитор коррозии"},
		MethodsAfter:  []string{"Ингибитор коррозии", "Корректировка дозирования ингибитора коррозии", "Ингибитор коррозии + контроль кислорода"},
	},
	62: {
		Count: 4, Class: "А2", Complication: "АСПО",
		Causes:        []string{"Рост нагрузки и парафиновые отложения", "Снижение проходного сечения", "Повторное накопление АСПО", "Рост вязкости и отложения"},
		FailedUnits:   []string{"НКТ", "Насос", "НКТ", "Приём насоса"},
		MethodsBefore: []string{"Без профилактики", "Периодическая горячая обработка", "Растворитель АСПО", "Растворитель АСПО"},
		MethodsAfter:  []string{"Тепловая обработка", "Растворитель АСПО", "Скребковая очистка + растворитель", "Корректировка профилактики АСПО"},
	},
	74: {
		Count: 3, Class: "М2", Complication: "Механические примеси",
		Causes:        []string{"Накопление механических примесей", "Абразивный износ", "Засорение приёмной части"},
		FailedUnits:   []string{"Приём насоса", "Рабочая пара", "Фильтрующий узел"},
		MethodsBefore: []string{"Без дополнительной очистки", "Промывка", "Фильтрация"},
		MethodsAfter:  []string{"Нормализация забоя", "Фильтрация / сепарация мехпримесей", "Корректировка фильтрации"},
	},
	97: {
		Count: 4, Class: "В2", Complication: "Высокая вязкость / эмульсия",
		Causes:        []string{"Рост вязкости эмульсии", "Нестабильная подача при эмульгировании", "Повторный рост вязкости", "Снижение эффективности деэмульсации"},
		FailedUnits:   []string{"Насос", "Приём насоса", "НКТ", "Насос"},
		MethodsBefore: []string{"Без реагентной обработки", "Деэмульгатор", "Деэмульгатор", "Подбор реагента не актуализирован"},
		MethodsAfter:  []string{"Деэмульгатор", "Корректировка дозирования деэмульгатора", "Подбор состава для снижения вязкости", "Повторный лабораторный подбор деэмульгатора"},
	},
}

func teachingFailureHistoryForObject(objectKey string, now time.Time) []FailureEvent {
	points := teachingCrossHistoryPoints(objectKey, now)
	if len(points) == 0 {
		return nil
	}
	state, _ := teachingWorkflowStateForObject(objectKey)
	rows := make([]FailureEvent, 0, len(points))
	for _, point := range points {
		cycleStart := point.CycleStartDate
		repairEnd := point.RepairEndDate
		rows = append(rows, FailureEvent{
			ID: fmt.Sprintf("teaching-cross-failure-%d", point.Index), ObjectKey: objectKey,
			Department: "УЧЕБНЫЙ НАБОР РОСА", ProductionUnit: "Учебный контур",
			Field: state.Field, Well: state.Well, ObjectPlast: state.ObjectPlast,
			CycleStartDate: cycleStart.Format("02.01.2006"), Date: point.Date.Format("02.01.2006"), RepairEndDate: repairEnd.Format("02.01.2006"),
			EventType: "Отказ", FailedUnit: point.FailedUnit, OperationMode: "Учебный режим эксплуатации",
			Equipment: point.Equipment, EquipmentBrand: "Учебная марка", EquipmentSize: "Учебный типоразмер",
			StopCause: point.Cause, Source: "УЧЕБНЫЙ ПРИМЕР · единый сквозной таймлайн Фонд → Лаборатория → Оценка → Методы → МРП",
			SourceClass: point.Classification, UserClass: point.Classification, NormalizedClass: point.Classification,
			ComplicationFactor: point.Complication, SourceCause: point.Cause, NormalizedCause: strings.ToLower(point.Cause),
			Inspection:           "Учебная дефектовка: синтетическое подтверждение проявления для проверки маршрута программы.",
			InvestigationStatus:  "Утверждено (учебный пример)",
			CommissionConclusion: "Учебное заключение. Не является производственным актом или экспертным заключением.",
			MethodBeforeFailure:  point.MethodBefore, MethodAfterFailure: point.Method,
			PreliminaryConclusion: point.Complication + " — учебная предварительная причина",
			ApprovedConclusion:    point.Complication + " — учебное утверждённое заключение",
			ConclusionApproved:    true, Draft: false, Manual: false,
			Comment:   fmt.Sprintf("УЧЕБНЫЙ ПРИМЕР · ГНО: %s; фактическая наработка: %d сут.; синтетическая запись.", point.Equipment, point.MRPDays),
			UpdatedAt: now.Format(time.RFC3339), OperationalOnly: false,
		})
	}
	return rows
}
