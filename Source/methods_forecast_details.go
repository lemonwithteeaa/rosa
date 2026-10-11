//go:build windows

package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const methodsForecastDetailClose = 9650

var (
	methodsForecastDetailDone        bool
	methodsForecastDetailTitle       string
	methodsForecastDetailBody        string
	methodsForecastDetailSubtitle    string
	methodsForecastDetailSummary     [3]string
	methodsForecastDetailHeaderBand  syscall.Handle
	methodsForecastDetailAccent      syscall.Handle
	methodsForecastDetailTitleHandle syscall.Handle
	methodsForecastDetailSubHandle   syscall.Handle
	methodsForecastDetailCards       [3]syscall.Handle
	methodsForecastDetailSection     syscall.Handle
	methodsForecastDetailEdit        syscall.Handle
	methodsForecastDetailCloseButton syscall.Handle
	methodsForecastDetailHeadingFont syscall.Handle
	methodsForecastDetailTitleFont   syscall.Handle
)

func ensureMethodsForecastDetailHeadingFonts() {
	if methodsForecastDetailHeadingFont == 0 {
		h, _, _ := pCreateFontW.Call(^uintptr(methodsTypeSection-1), 0, 0, 0, 600, 1, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
		methodsForecastDetailHeadingFont = syscall.Handle(h)
	}
	if methodsForecastDetailTitleFont == 0 {
		h, _, _ := pCreateFontW.Call(^uintptr(methodsTypeDisplay-1), 0, 0, 0, 600, 1, 0, 0, 204, 0, 0, 5, 0, uintptr(unsafe.Pointer(p("Segoe UI"))))
		methodsForecastDetailTitleFont = syscall.Handle(h)
	}
}

func deleteMethodsForecastDetailHeadingFonts() {
	if methodsForecastDetailHeadingFont != 0 {
		pDeleteObject.Call(uintptr(methodsForecastDetailHeadingFont))
		methodsForecastDetailHeadingFont = 0
	}
	if methodsForecastDetailTitleFont != 0 {
		pDeleteObject.Call(uintptr(methodsForecastDetailTitleFont))
		methodsForecastDetailTitleFont = 0
	}
}

type methodsForecastDetailState struct {
	Key        string
	Snap       methodForecastSnapshot
	Names      []string
	Combo      CombinedMethodForecast
	HasCombo   bool
	Archive    approvedMethodDecision
	HasArchive bool
	Low        float64
	Calc       float64
	High       float64
	Source     string
	Status     string
	Target     TargetRunlifeResolved
}

func currentMethodsForecastDetailState() methodsForecastDetailState {
	key := strings.TrimSpace(selectedObjectKey)
	st := methodsForecastDetailState{Key: key, Status: "после утверждения метода"}
	if key == "" {
		return st
	}
	st.Snap = methodForecastSnapshotForSelectedObject()
	st.Source = st.Snap.Source
	st.Target = targetRunlifeResolvedForSelectedObject()
	if st.Target.Days < defaultTargetRunlifeDays {
		st.Target.Days = defaultTargetRunlifeDays
		if strings.TrimSpace(st.Target.Source) == "" {
			st.Target.Source = "общий целевой ориентир"
		}
	}
	if names := currentMethodsPlanNames(); len(names) > 0 {
		st.Names = append([]string(nil), names...)
		st.Combo = combinedMethodForecastForNames(key, names, st.Snap, time.Now())
		st.HasCombo = true
		st.Low, st.Calc, st.High = st.Combo.ForecastLow, st.Combo.ForecastBase, st.Combo.ForecastHigh
		st.Source = st.Combo.Source
		st.Status = "текущий комплекс"
		return st
	}
	if d, ok := latestApprovedMethodDecision(key); ok && d.ForecastBaseDays > 0 && !methodsHasExplicitDraft(key) {
		st.Archive = d
		st.HasArchive = true
		st.Names = normalizeMethodList(d.Methods)
		if len(st.Names) == 0 && strings.TrimSpace(d.Method) != "" {
			st.Names = []string{strings.TrimSpace(d.Method)}
		}
		st.Low, st.Calc, st.High = d.ForecastLowDays, d.ForecastBaseDays, d.ForecastHighDays
		if st.Low <= 0 {
			st.Low = st.Calc
		}
		if st.High <= 0 {
			st.High = st.Calc
		}
		st.Source = d.ForecastSource
		st.Status = "архивное утверждение"
	}
	return st
}

type methodsForecastCycle struct {
	Start string
	End   string
	Days  float64
	Cause string
}

func methodsForecastCycles(objectKey string, now time.Time) []methodsForecastCycle {
	rows := historyForWell(objectKey, now)
	out := make([]methodsForecastCycle, 0, len(rows))
	for _, e := range rows {
		end, errEnd := failureDate(e.Date)
		start, errStart := failureDate(e.CycleStartDate)
		if errEnd != nil || errStart != nil || !end.After(start) {
			continue
		}
		days := end.Sub(start).Hours() / 24
		if days <= 0 || days > 3650 {
			continue
		}
		cause := strings.TrimSpace(e.StopCause)
		if cause == "" {
			cause = strings.TrimSpace(e.PrimaryComplication)
		}
		out = append(out, methodsForecastCycle{Start: e.CycleStartDate, End: e.Date, Days: days, Cause: cause})
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, ei := failureDate(out[i].End)
		dj, ej := failureDate(out[j].End)
		if ei == nil && ej == nil {
			return di.Before(dj)
		}
		return out[i].End < out[j].End
	})
	return out
}

func methodsForecastCyclesText(objectKey string, now time.Time) string {
	cycles := methodsForecastCycles(objectKey, now)
	if len(cycles) == 0 {
		return "Фактические завершённые циклы: нет циклов с корректными датами начала и окончания."
	}
	var b strings.Builder
	b.WriteString("Фактические завершённые циклы, вошедшие в базу:\r\n")
	for i, c := range cycles {
		fmt.Fprintf(&b, "%d. %s — %s = %.0f сут.", i+1, dash(c.Start), dash(c.End), c.Days)
		if strings.TrimSpace(c.Cause) != "" {
			fmt.Fprintf(&b, " Причина/осложнение: %s.", strings.TrimSpace(c.Cause))
		}
		b.WriteString("\r\n")
	}
	return strings.TrimRight(b.String(), "\r\n")
}

func methodsForecastBaseDetail(st methodsForecastDetailState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "БАЗОВАЯ ОЦЕНКА\r\n\r\n")
	fmt.Fprintf(&b, "Объект: %s.\r\n", dash(st.Key))
	if st.Snap.Base > 0 {
		fmt.Fprintf(&b, "Значение на карточке: %.0f сут.\r\n", st.Snap.Base)
	} else {
		b.WriteString("Значение на карточке: не сформировано.\r\n")
	}
	b.WriteString("Что означает: исходная ожидаемая наработка ДО применения выбранного метода/комплекса.\r\n\r\n")
	fmt.Fprintf(&b, "Период истории: последние %d мес.\r\n", methodForecastHistoryMonths)
	fmt.Fprintf(&b, "Отказов в периоде: %d. Завершённых циклов с корректными датами: %d.\r\n", st.Snap.History.FailureCount, st.Snap.History.CycleCount)
	fmt.Fprintf(&b, "Достаточность истории: %s.\r\n", dash(st.Snap.History.Precision))
	b.WriteString(methodsForecastCyclesText(st.Key, time.Now()))
	b.WriteString("\r\n\r\nАЛГОРИТМ ФОРМИРОВАНИЯ БАЗЫ\r\n")
	b.WriteString("• 1 завершённый цикл — используется его фактическая длительность.\r\n")
	b.WriteString("• 2 цикла — база = медиана; нижняя/верхняя границы = фактический минимум/максимум.\r\n")
	b.WriteString("• 3 и более циклов — база = медиана; осторожная/благоприятная границы = P25/P75 фактической 36-месячной истории.\r\n")
	if st.Snap.Base > 0 {
		fmt.Fprintf(&b, "Для текущего объекта получено: нижняя база %.0f сут.; медианная база %.0f сут.; верхняя база %.0f сут.\r\n", valueOr(st.Snap.Low, st.Snap.Base), st.Snap.Base, valueOr(st.Snap.High, st.Snap.Base))
	}
	fmt.Fprintf(&b, "Источник: %s.\r\n", dash(st.Snap.Source))
	if strings.TrimSpace(st.Snap.History.Warning) != "" {
		fmt.Fprintf(&b, "Ограничение: %s\r\n", st.Snap.History.Warning)
	}
	if _, teaching := teachingWorkflowOrdinal(st.Key); teaching {
		b.WriteString("Статус данных: УЧЕБНЫЙ сценарий; значения предназначены для проверки логики программы, а не для производственного решения.\r\n")
	}
	b.WriteString("\r\nОГРАНИЧЕНИЯ И ИНТЕРПРЕТАЦИЯ\r\n")
	b.WriteString("Лабораторные и режимные показатели не получают выдуманных численных весов. Если фактической истории недостаточно, РОСА явно показывает резервный источник и понижает достоверность. Базовая оценка описывает исходное состояние объекта и сама по себе не подтверждает эффективность выбранного мероприятия.")
	return b.String()
}

func valueOr(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}

func methodsForecastGainDetail(st methodsForecastDetailState) string {
	var b strings.Builder
	b.WriteString("ОЖИДАЕМЫЙ ПРИРОСТ\r\n\r\n")
	fmt.Fprintf(&b, "Объект: %s.\r\n", dash(st.Key))
	if st.Snap.Base <= 0 {
		b.WriteString("Прирост не рассчитывается: нет базовой оценки наработки.\r\n")
		fmt.Fprintf(&b, "Источник базы: %s.", dash(st.Snap.Source))
		return b.String()
	}
	if st.HasArchive && !st.HasCombo {
		b.WriteString("На экране используется архивное утверждение. РОСА намеренно НЕ вычитает текущую базу объекта из старого сохранённого прогноза, потому что условия/классификация могли измениться.\r\n")
		b.WriteString("Для корректного прироста сформируйте или подтвердите текущий комплекс — после этого карточка будет пересчитана относительно текущей базы.")
		return b.String()
	}
	if !st.HasCombo || st.Calc <= 0 {
		b.WriteString("Прирост пока не рассчитан: выберите хотя бы один метод, для которого есть численно подтверждаемый вклад, и сформируйте текущий комплекс.")
		return b.String()
	}
	lowGain := math.Max(0, st.Low-st.Snap.Base)
	highGain := math.Max(0, st.High-st.Snap.Base)
	fmt.Fprintf(&b, "Диапазон на карточке: +%.0f–%.0f сут.\r\n", lowGain, highGain)
	b.WriteString("Это разница между итоговой осторожной/благоприятной оценкой после комплекса и центральной базовой оценкой до мероприятий.\r\n\r\n")
	fmt.Fprintf(&b, "Совокупный рассчитанный вклад комплекса: +%.0f%% / +%.0f%% / +%.0f%% (осторожный / ожидаемый / благоприятный).\r\n", st.Combo.EffectLowPct, st.Combo.EffectBasePct, st.Combo.EffectHighPct)
	b.WriteString("\r\nПРАВИЛО ОБЪЕДИНЕНИЯ МЕТОДОВ\r\n")
	b.WriteString("Используется остаточный риск, а не арифметическое сложение процентов: общий эффект = 1 − произведение (1 − вклад метода).\r\n")
	b.WriteString("Если несколько методов относятся к одному механизму осложнения, вклад второго и последующих дополнительно дисконтируется коэффициентом 0,65, чтобы не задваивать эффект.\r\n")
	b.WriteString("Отдельный численный бонус за синергию НЕ начисляется без объектовой калибровки.\r\n\r\n")
	b.WriteString("ВКЛАД МЕТОДОВ ТЕКУЩЕГО КОМПЛЕКСА\r\n")
	for i, m := range st.Combo.Methods {
		fmt.Fprintf(&b, "%d. %s. ", i+1, dash(m.Method))
		if m.EffectQuantified {
			fmt.Fprintf(&b, "Вклад +%.0f%% / +%.0f%% / +%.0f%%; применимость %d%%; достоверность %d%% (%s).", m.LowPct, m.BasePct, m.HighPct, m.Applicability, m.Reliability, dash(m.ReliabilityLevel))
		} else {
			fmt.Fprintf(&b, "Численный вклад не начислен; учитывается качественно. Применимость %d%%; достоверность %d%% (%s).", m.Applicability, m.Reliability, dash(m.ReliabilityLevel))
		}
		if strings.TrimSpace(m.EffectBasisType) != "" {
			fmt.Fprintf(&b, " База эффекта: %s.", m.EffectBasisType)
		}
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "\r\nВзаимодействие методов: %s\r\n", dash(st.Combo.Interaction))
	b.WriteString("\r\nОГРАНИЧЕНИЯ ИНТЕРПРЕТАЦИИ\r\n")
	b.WriteString("Проценты применимости и достоверности не являются вероятностью успеха и не должны трактоваться как гарантированный рост МРП. До появления подтверждённого фактического исхода рассчитанный прирост используется как инженерный сценарий для решения технолога.")
	return b.String()
}

func methodsForecastResultDetail(st methodsForecastDetailState) string {
	var b strings.Builder
	b.WriteString("ПРОГНОЗ НАРАБОТКИ\r\n\r\n")
	fmt.Fprintf(&b, "Объект: %s.\r\n", dash(st.Key))
	if st.Calc <= 0 {
		b.WriteString("Итоговый прогноз пока не сформирован. Выберите текущий метод/комплекс или откройте сохранённое утверждённое решение.")
		return b.String()
	}
	fmt.Fprintf(&b, "Результат на карточке: %.0f–%.0f сут.; расчётная точка %.0f сут.\r\n", st.Low, st.High, st.Calc)
	fmt.Fprintf(&b, "Статус: %s.\r\n", st.Status)
	if st.HasCombo {
		lowBase := valueOr(st.Combo.BaselineLow, st.Snap.Base)
		highBase := valueOr(st.Combo.BaselineHigh, st.Snap.Base)
		fmt.Fprintf(&b, "\r\nРАСЧЁТ ПО ТЕКУЩЕМУ КОМПЛЕКСУ\r\n")
		fmt.Fprintf(&b, "• Осторожный: %.0f × (1 + %.0f%%) = %.0f сут.\r\n", lowBase, st.Combo.EffectLowPct, st.Combo.ForecastLow)
		fmt.Fprintf(&b, "• Расчётный: %.0f × (1 + %.0f%%) = %.0f сут.\r\n", st.Combo.BaselineBase, st.Combo.EffectBasePct, st.Combo.ForecastBase)
		fmt.Fprintf(&b, "• Благоприятный: %.0f × (1 + %.0f%%) = %.0f сут.\r\n", highBase, st.Combo.EffectHighPct, st.Combo.ForecastHigh)
		b.WriteString("После расчёта границы упорядочиваются так, чтобы осторожная оценка не была выше расчётной, а благоприятная — ниже неё.\r\n")
	} else if st.HasArchive {
		fmt.Fprintf(&b, "\r\nЭто сохранённый прогноз утверждённого решения от %s. Он показывается без повторного смешивания со свежей базой объекта.\r\n", dash(st.Archive.ApprovedAt))
		if st.Archive.EffectBasePct > 0 {
			fmt.Fprintf(&b, "Сохранённый вклад комплекса: +%.0f%% / +%.0f%% / +%.0f%%.\r\n", st.Archive.EffectLowPct, st.Archive.EffectBasePct, st.Archive.EffectHighPct)
		}
	}
	target := st.Target.Days
	if target <= 0 {
		target = defaultTargetRunlifeDays
	}
	if st.Calc >= target {
		fmt.Fprintf(&b, "\r\nСопоставление с целью: расчёт %.0f сут. ≥ ориентир %.0f сут.; целевой уровень достигается в расчётном сценарии.\r\n", st.Calc, target)
	} else {
		fmt.Fprintf(&b, "\r\nСопоставление с целью: расчёт %.0f сут. < ориентир %.0f сут.; остаточный разрыв %.0f сут.\r\n", st.Calc, target, target-st.Calc)
	}
	fmt.Fprintf(&b, "Источник прогноза: %s.\r\n", dash(st.Source))
	b.WriteString("\r\nОГРАНИЧЕНИЯ ИНТЕРПРЕТАЦИИ\r\n")
	b.WriteString("Диапазон не является статистическим доверительным интервалом. Это инженерный осторожный/расчётный/благоприятный сценарий на базе фактической истории и только тех эффектов методов, которые допускают численный перенос. После реализации решения прогноз необходимо сопоставить с фактической наработкой и причиной следующего отказа.")
	return b.String()
}

func methodsForecastBasisDetail(st methodsForecastDetailState) string {
	var b strings.Builder
	b.WriteString("ОСНОВАНИЕ ПРОГНОЗА\r\n\r\n")
	fmt.Fprintf(&b, "Объект: %s.\r\n", dash(st.Key))
	fmt.Fprintf(&b, "Статус расчёта: %s.\r\n", st.Status)
	fmt.Fprintf(&b, "История за %d мес.: отказов %d; завершённых циклов %d; достаточность %s.\r\n", methodForecastHistoryMonths, st.Snap.History.FailureCount, st.Snap.History.CycleCount, dash(st.Snap.History.Precision))
	fmt.Fprintf(&b, "База: %s\r\n", dash(st.Snap.History.Basis))
	fmt.Fprintf(&b, "Источник базового прогноза: %s.\r\n", dash(st.Snap.Source))
	if len(st.Names) > 0 {
		fmt.Fprintf(&b, "Метод/комплекс: %s.\r\n", strings.Join(st.Names, "; "))
	} else {
		b.WriteString("Метод/комплекс: не выбран.\r\n")
	}
	if st.HasCombo {
		quantified := 0
		for _, m := range st.Combo.Methods {
			if m.EffectQuantified {
				quantified++
			}
		}
		fmt.Fprintf(&b, "Численно вошло методов: %d из %d. Остальные не получают искусственного процента МРП.\r\n", quantified, len(st.Combo.Methods))
		fmt.Fprintf(&b, "Взаимодействие: %s\r\n", dash(st.Combo.Interaction))
	}
	target := st.Target.Days
	if target <= 0 {
		target = defaultTargetRunlifeDays
	}
	fmt.Fprintf(&b, "Целевой ориентир: %.0f сут.; источник: %s; область: %s.\r\n", target, dash(st.Target.Source), dash(st.Target.ScopeLabel))
	if strings.TrimSpace(st.Snap.History.Warning) != "" {
		fmt.Fprintf(&b, "Ограничение истории: %s\r\n", st.Snap.History.Warning)
	}
	if st.HasArchive && strings.TrimSpace(st.Archive.HistoryWarning) != "" {
		fmt.Fprintf(&b, "Ограничение сохранённого решения: %s\r\n", st.Archive.HistoryWarning)
	}
	b.WriteString("\r\nЧТО РОСА НЕ ДЕЛАЕТ\r\n")
	b.WriteString("• не считает применимость вероятностью успеха;\r\n")
	b.WriteString("• не складывает проценты методов напрямую;\r\n")
	b.WriteString("• не начисляет отдельный бонус за синергию без фактической калибровки;\r\n")
	b.WriteString("• не переносит старый архивный прогноз на изменившийся объект как новый расчёт;\r\n")
	b.WriteString("• не заменяет технологическое утверждение автоматическим решением.\r\n\r\n")
	b.WriteString("Для производственного применения прогноз должен быть подтверждён фактической наработкой после реализации и затем использован для последующей калибровки накопленного опыта РОСА.")
	return b.String()
}

func methodsForecastDetailHeader(cardID int, st methodsForecastDetailState) (string, [3]string) {
	var cards [3]string
	cards[0] = "ОБЪЕКТ\r\n" + dash(st.Key)
	switch cardID {
	case ID_METHODS_FORECAST_BASE:
		cards[1] = fmt.Sprintf("НЕГАТИВНЫЙ ПРОГНОЗ\r\n%.0f сут.", st.Low)
		cards[2] = fmt.Sprintf("ИСТОРИЯ\r\n%d мес. · %d циклов", methodForecastHistoryMonths, st.Snap.History.CycleCount)
		return "Нижний сценарий прогноза после текущего комплекса", cards
	case ID_METHODS_FORECAST_GAIN:
		cards[1] = fmt.Sprintf("РЕАЛЬНЫЙ ПРОГНОЗ\r\n%.0f сут.", st.Calc)
		cards[2] = "СТАТУС\r\n" + dash(st.Status)
		return "Центральный расчётный сценарий после текущего комплекса", cards
	case ID_METHODS_FORECAST_RESULT:
		cards[1] = fmt.Sprintf("ПОЛОЖИТЕЛЬНЫЙ ПРОГНОЗ\r\n%.0f сут.", st.High)
		cards[2] = fmt.Sprintf("ЦЕЛЕВОЙ ОРИЕНТИР\r\n≥ %.0f сут.", valueOr(st.Target.Days, defaultTargetRunlifeDays))
		return "Верхний сценарий прогноза после текущего комплекса", cards
	case ID_METHODS_FORECAST_BASIS:
		cards[1] = fmt.Sprintf("ИСХОДНАЯ ИСТОРИЯ\r\n%d отказов · %d циклов", st.Snap.History.FailureCount, st.Snap.History.CycleCount)
		cards[2] = "ДОСТАТОЧНОСТЬ\r\n" + dash(st.Snap.History.Precision)
		return "Источники, ограничения и правила интерпретации прогноза", cards
	}
	return "Подробная техническая расшифровка текущего расчёта", cards
}

func methodsForecastDetailText(cardID int) (string, string) {
	st := currentMethodsForecastDetailState()
	if st.Key == "" {
		return "РОСА — прогноз наработки", "Сначала закрепите объект. После этого карточки прогноза покажут подробную расшифровку исходных данных и формул."
	}
	switch cardID {
	case ID_METHODS_FORECAST_BASE:
		return "РОСА — Негативный прогноз", "НЕГАТИВНЫЙ СЦЕНАРИЙ\r\n\r\n" + methodsForecastResultDetail(st)
	case ID_METHODS_FORECAST_GAIN:
		return "РОСА — Реальный прогноз", "ЦЕНТРАЛЬНЫЙ РАСЧЁТНЫЙ СЦЕНАРИЙ\r\n\r\n" + methodsForecastResultDetail(st)
	case ID_METHODS_FORECAST_RESULT:
		return "РОСА — Положительный прогноз", "БЛАГОПРИЯТНЫЙ СЦЕНАРИЙ\r\n\r\n" + methodsForecastResultDetail(st)
	case ID_METHODS_FORECAST_BASIS:
		return "РОСА — Основание прогноза", methodsForecastBasisDetail(st)
	}
	return "РОСА — прогноз наработки", "Нет данных для выбранной карточки."
}

func layoutMethodsForecastDetail(hwnd uintptr) {
	var rc RECT
	if r, _, _ := pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc))); r == 0 {
		return
	}
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	margin := int32(24)
	headerH := int32(82)
	setPos(methodsForecastDetailHeaderBand, 0, 0, w, headerH)
	setPos(methodsForecastDetailAccent, 0, headerH-4, w, 4)
	setPos(methodsForecastDetailTitleHandle, margin, 12, max32(300, w-margin*2), 30)
	setPos(methodsForecastDetailSubHandle, margin, 44, max32(300, w-margin*2), 26)

	cardY := int32(96)
	cardH := int32(70)
	cardGap := int32(12)
	cardW := max32(190, (w-margin*2-cardGap*2)/3)
	for i, card := range methodsForecastDetailCards {
		cx := margin + int32(i)*(cardW+cardGap)
		cw := cardW
		if i == 2 {
			cw = max32(190, w-margin-cx)
		}
		setPos(card, cx, cardY, cw, cardH)
	}

	sectionY := cardY + cardH + 14
	setPos(methodsForecastDetailSection, margin, sectionY, max32(300, w-margin*2), 28)
	editY := sectionY + 32
	// The system caption already supplies Close. Give the former footer height
	// to the technical explanation so its final lines are not clipped.
	editH := max32(120, h-margin-editY)
	setPos(methodsForecastDetailEdit, margin, editY, max32(300, w-margin*2), editH)
}

// formatMethodsForecastDetailText resets the whole document first and then
// emphasizes only complete structural headings. Prefixes inside sentences are
// deliberately never formatted separately: this prevents the broken visual
// effect where only fragments of words or labels appeared bold.
func formatMethodsForecastDetailText(h syscall.Handle, s string) {
	if h == 0 {
		return
	}
	// Reset the complete document to plain body text first. Bold and italic are
	// both explicitly cleared, so emphasis can never leak into labels, bullets,
	// values or fragments after a previously formatted heading.
	normal := CHARFORMATW{
		CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
		DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
		CrTextColor: uint32(rgb(43, 46, 50)),
		YHeight:     240,
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, ^uintptr(0))
	pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&normal)))

	// RichEdit counts each paragraph separator as one character even when source
	// text is CRLF. Normalize first and format ONLY complete uppercase structural
	// headings. RC678 also makes those headings bold+italic, matching the handbook.
	offset := 0
	first := true
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimLen := len(utf16.Encode([]rune(trimmed)))
		wholeHeading := trimmed != "" && trimmed == strings.ToUpper(trimmed) && !strings.HasPrefix(trimmed, "•") && len([]rune(trimmed)) <= 76
		if wholeHeading {
			heading := CHARFORMATW{
				CbSize:      uint32(unsafe.Sizeof(CHARFORMATW{})),
				DwMask:      CFM_BOLD | cfmItalic | CFM_COLOR | cfmSize,
				DwEffects:   CFE_BOLD | cfeItalic,
				CrTextColor: uint32(rgb(15, 48, 82)),
				YHeight:     260,
			}
			if first {
				heading.YHeight = 300
				first = false
			}
			pSendMessageW.Call(uintptr(h), EM_SETSEL, uintptr(offset), uintptr(offset+trimLen))
			pSendMessageW.Call(uintptr(h), EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&heading)))
		}
		offset += len(utf16.Encode([]rune(line)))
		if i < len(lines)-1 {
			offset++
		}
	}
	pSendMessageW.Call(uintptr(h), EM_SETSEL, 0, 0)
}

func methodsForecastDetailWndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		parent := syscall.Handle(hwnd)
		methodsForecastDetailHeaderBand = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 0, 1120, 82, parent, 0)
		methodsForecastDetailAccent = modalCreate("STATIC", "", WS_CHILD|WS_VISIBLE, 0, 78, 1120, 4, parent, 0)
		methodsForecastDetailTitleHandle = modalCreate("STATIC", strings.ToUpper(methodsForecastDetailTitle), WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 12, 1050, 30, parent, 0)
		methodsForecastDetailSubHandle = modalCreate("STATIC", methodsForecastDetailSubtitle, WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 44, 1050, 26, parent, 0)
		ensureMethodsForecastDetailHeadingFonts()
		if methodsForecastDetailTitleFont != 0 {
			pSendMessageW.Call(uintptr(methodsForecastDetailTitleHandle), WM_SETFONT, uintptr(methodsForecastDetailTitleFont), 1)
		}
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsForecastDetailSubHandle), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		for i := range methodsForecastDetailCards {
			methodsForecastDetailCards[i] = modalCreate("STATIC", methodsForecastDetailSummary[i], WS_CHILD|WS_VISIBLE|WS_BORDER|SS_CENTER|SS_NOPREFIX, 24+int32(i)*340, 96, 326, 70, parent, 0)
			pSendMessageW.Call(uintptr(methodsForecastDetailCards[i]), WM_SETFONT, uintptr(methodsFont(methodsTypeBody, false)), 1)
		}
		methodsForecastDetailSection = modalCreate("STATIC", "ТЕХНИЧЕСКАЯ РАСШИФРОВКА", WS_CHILD|WS_VISIBLE|SS_LEFT|SS_NOPREFIX, 24, 180, 1050, 28, parent, 0)
		if methodsForecastDetailHeadingFont != 0 {
			pSendMessageW.Call(uintptr(methodsForecastDetailSection), WM_SETFONT, uintptr(methodsForecastDetailHeadingFont), 1)
		}
		_ = riched20.Load()
		methodsForecastDetailEdit = modalCreate("RichEdit20W", methodsForecastDetailBody, WS_CHILD|WS_VISIBLE|WS_BORDER|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY|WS_VSCROLL, 24, 212, 1050, 500, parent, 0)
		if hFontPanel != 0 {
			pSendMessageW.Call(uintptr(methodsForecastDetailEdit), WM_SETFONT, uintptr(hFontPanel), 1)
		}
		pSendMessageW.Call(uintptr(methodsForecastDetailEdit), emSetBkColor, 0, uintptr(rgb(255, 255, 255)))
		margins := uintptr(uint32(methodsLeftMargin) | uint32(methodsLeftMargin)<<16)
		pSendMessageW.Call(uintptr(methodsForecastDetailEdit), emSetMargins, 3, margins)
		formatMethodsForecastDetailText(methodsForecastDetailEdit, methodsForecastDetailBody)
		layoutMethodsForecastDetail(hwnd)
		return 0
	case WM_SIZE:
		layoutMethodsForecastDetail(hwnd)
		return 0
	case WM_GETMINMAXINFO:
		if lParam != 0 {
			mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
			mmi.PtMinTrackSize.X = 860
			mmi.PtMinTrackSize.Y = 640
		}
		return 0
	case WM_CTLCOLORSTATIC:
		child := syscall.Handle(lParam)
		switch child {
		case methodsForecastDetailHeaderBand:
			return uintptr(brushNav)
		case methodsForecastDetailAccent:
			return uintptr(brushHeader)
		case methodsForecastDetailTitleHandle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(255, 255, 255))
			return uintptr(brushNav)
		case methodsForecastDetailSubHandle:
			pSetBkMode.Call(wParam, TRANSPARENT)
			pSetTextColor.Call(wParam, rgb(220, 229, 238))
			return uintptr(brushNav)
		case methodsForecastDetailSection:
			pSetBkMode.Call(wParam, OPAQUE)
			pSetBkColor.Call(wParam, rgb(232, 242, 253))
			pSetTextColor.Call(wParam, rgb(15, 48, 82))
			return uintptr(methodsSectionBrush())
		}
		for _, card := range methodsForecastDetailCards {
			if child == card {
				pSetBkMode.Call(wParam, OPAQUE)
				pSetBkColor.Call(wParam, rgb(255, 255, 255))
				pSetTextColor.Call(wParam, rgb(15, 48, 82))
				return uintptr(brushWhite)
			}
		}
	case WM_CTLCOLOREDIT:
		if syscall.Handle(lParam) == methodsForecastDetailEdit {
			pSetBkColor.Call(wParam, rgb(255, 255, 255))
			pSetTextColor.Call(wParam, rgb(43, 46, 50))
			return uintptr(brushWhite)
		}
	case WM_DESTROY:
		deleteMethodsForecastDetailHeadingFonts()
		return 0
	case WM_COMMAND:
		if int(loWord(wParam)) == methodsForecastDetailClose {
			methodsForecastDetailDone = true
			pDestroyWindowAuth.Call(hwnd)
			return 0
		}
	case labMappingClose:
		methodsForecastDetailDone = true
		pDestroyWindowAuth.Call(hwnd)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func openMethodsForecastDetail(cardID int) {
	st := currentMethodsForecastDetailState()
	title, body := methodsForecastDetailText(cardID)
	methodsForecastDetailTitle = title
	methodsForecastDetailBody = body
	methodsForecastDetailSubtitle, methodsForecastDetailSummary = methodsForecastDetailHeader(cardID, st)
	methodsForecastDetailDone = false
	cls := p("ROSAMethodsForecastDetailV1")
	inst, _, _ := pGetModuleHandleW.Call(0)
	background := syscall.Handle(5 + 1)
	if brushLight != 0 {
		background = brushLight
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(methodsForecastDetailWndProc), HInstance: syscall.Handle(inst), HbrBackground: background, LpszClassName: cls, HIcon: hIconBig, HIconSm: hIconSmall}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	pEnableWindow.Call(uintptr(hMain), 0)
	hwnd, _, _ := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(p(title))), 0x00CA0000|WS_VISIBLE, 0x80000000, 0x80000000, 1120, 820, uintptr(hMain), 0, inst, 0)
	if hwnd == 0 {
		pEnableWindow.Call(uintptr(hMain), 1)
		msg(title, body, MB_OK|MB_ICONINFORMATION)
		return
	}
	var message MSG
	for !methodsForecastDetailDone {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleModalEscape(hwnd, &message) {
			continue
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
	}
	pEnableWindow.Call(uintptr(hMain), 1)
	pSetFocus.Call(uintptr(hMain))
}
