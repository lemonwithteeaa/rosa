//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

func laboratoryExportQCText(pass LabPassRecord, now time.Time) string {
	q := laboratoryPassQCSummaryFor(pass, now)
	var issues []string
	if !q.ObjectPresent {
		issues = append(issues, "не указан объект")
	}
	if !q.DatePresent {
		issues = append(issues, "не указана дата анализа")
	}
	if !q.SourcePresent {
		issues = append(issues, "не указан источник / протокол")
	}
	state := "отчёт прослеживаем"
	if q.ValidationErrors > 0 || len(q.SharedConflicts) > 0 {
		state = "есть ошибки или противоречия исходных данных — расчётные выводы требуют сверки"
	} else if len(issues) > 0 || q.ValidationWarnings > 0 || !q.FreshnessKnown || q.FreshnessDays > 180 {
		state = "есть ограничения / неполные метаданные / вопросы актуальности"
	}
	issueText := "критичные метаданные заполнены"
	if len(issues) > 0 {
		issueText = strings.Join(issues, "; ")
	}
	conflictText := "расхождений общих показателей не выявлено"
	if len(q.SharedConflicts) > 0 {
		conflictText = "расхождения: " + strings.Join(q.SharedConflicts, "; ")
	}
	return fmt.Sprintf("QC-ГЕЙТ ЭКСПОРТА\nСтатус: %s.\nПокрытие: данные есть по %d из 6 направлений; базовый набор полностью заполнен по %d направлениям.\nВалидация: ошибок %d; предупреждений %d; %s.\nПрослеживаемость: %s; %s.\nОтчёт сохраняет исходные значения, единицы, модель, ограничения и недостающие данные; QC не заменяет лабораторный протокол.", state, q.DirectionsWithData, q.DirectionsBaselineComplete, q.ValidationErrors, q.ValidationWarnings, conflictText, issueText, q.FreshnessLabel)
}
