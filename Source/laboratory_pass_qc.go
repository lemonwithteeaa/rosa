//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

type laboratoryPassQCDirection struct {
	Code, Name                    string
	BaseFilled, BaseTotal         int
	OptionalFilled, OptionalTotal int
	Warnings, Errors              int
}

type laboratoryPassQCSummary struct {
	DirectionsWithData         int
	DirectionsBaselineComplete int
	ValidationWarnings         int
	ValidationErrors           int
	SharedConflicts            []string
	ObjectPresent              bool
	DatePresent                bool
	SourcePresent              bool
	FreshnessLabel             string
	FreshnessKnown             bool
	FreshnessDays              int
	Directions                 []laboratoryPassQCDirection
}

func laboratoryPassQCSummaryFor(pass LabPassRecord, now time.Time) laboratoryPassQCSummary {
	out := laboratoryPassQCSummary{
		ObjectPresent: strings.TrimSpace(pass.ObjectKey) != "",
		DatePresent:   strings.TrimSpace(pass.Date) != "",
		SourcePresent: strings.TrimSpace(pass.Source) != "",
	}
	if t, ok := laboratoryAnalysisDate(pass.Date); ok {
		days := int(now.Sub(t).Hours() / 24)
		out.FreshnessKnown = true
		out.FreshnessDays = days
		switch {
		case days < -1:
			out.FreshnessLabel = "дата анализа находится в будущем — проверить"
		case days <= 30:
			if days < 0 {
				days = 0
			}
			out.FreshnessLabel = fmt.Sprintf("свежий анализ: %d дн.", days)
		case days <= 180:
			out.FreshnessLabel = fmt.Sprintf("анализу %d дн. — учитывать изменение режима", days)
		default:
			out.FreshnessLabel = fmt.Sprintf("анализу %d дн. — желательно подтвердить актуальность", days)
		}
	} else {
		out.FreshnessLabel = "дата не распознана"
	}
	for _, baseGroup := range labGroups {
		if baseGroup.Code == "overall" {
			continue
		}
		group := laboratoryScaleInputGroup(baseGroup, pass.ScaleModel)
		raw := pass.Groups[group.Code]
		canonical := canonicalizeLaboratoryValues(group, raw, pass.Units[group.Code])
		bf, bt, of, ot := laboratoryDataCoverageForModel(group, raw, pass.ScaleModel)
		filled, _ := laboratoryHistoryFieldCoverage(group, raw, pass.ScaleModel)
		_, warnings, errors := validateLaboratoryGroup(group, canonical)
		row := laboratoryPassQCDirection{Code: group.Code, Name: group.Name, BaseFilled: bf, BaseTotal: bt, OptionalFilled: of, OptionalTotal: ot, Warnings: len(warnings), Errors: len(errors)}
		out.Directions = append(out.Directions, row)
		if filled > 0 || len(errors) > 0 || len(warnings) > 0 {
			out.DirectionsWithData++
		}
		if bt > 0 && bf >= bt {
			out.DirectionsBaselineComplete++
		}
		out.ValidationWarnings += len(warnings)
		out.ValidationErrors += len(errors)
	}
	out.SharedConflicts = laboratorySharedValueConflictsWithUnits(pass.Groups, pass.Units)
	return out
}

func laboratoryPassQCText(pass LabPassRecord, now time.Time) string {
	q := laboratoryPassQCSummaryFor(pass, now)
	meta := []string{}
	if !q.ObjectPresent {
		meta = append(meta, "не указан объект")
	}
	if !q.DatePresent {
		meta = append(meta, "не указана дата анализа")
	}
	if !q.SourcePresent {
		meta = append(meta, "не указан источник / протокол")
	}
	metadata := "критичные метаданные заполнены"
	if len(meta) > 0 {
		metadata = strings.Join(meta, "; ")
	}

	dirLines := make([]string, 0, len(q.Directions))
	for _, d := range q.Directions {
		status := fmt.Sprintf("база %d/%d; доп. %d/%d", d.BaseFilled, d.BaseTotal, d.OptionalFilled, d.OptionalTotal)
		if d.Errors > 0 || d.Warnings > 0 {
			status += fmt.Sprintf("; ошибок %d; предупреждений %d", d.Errors, d.Warnings)
		}
		dirLines = append(dirLines, fmt.Sprintf("• %s: %s", d.Name, status))
	}
	conflicts := "расхождений общих показателей не выявлено"
	if len(q.SharedConflicts) > 0 {
		conflicts = "есть расхождения общих показателей: " + strings.Join(q.SharedConflicts, "; ")
	}
	state := "исходные данные прослеживаемы; явных блокирующих ошибок не выявлено"
	if q.ValidationErrors > 0 || len(q.SharedConflicts) > 0 {
		state = "есть блокирующие или противоречивые исходные данные — расчётные выводы требуют сверки"
	} else if len(meta) > 0 || q.ValidationWarnings > 0 || !q.FreshnessKnown || q.FreshnessDays > 180 {
		state = "есть ограничения качества / актуальности, которые нужно учитывать при интерпретации"
	}
	return fmt.Sprintf("ПАСПОРТ КАЧЕСТВА ЛАБОРАТОРНОЙ ДИАГНОСТИКИ\nСтатус: %s.\nМетаданные: %s.\nСвежесть: %s.\nПокрытие: данные есть по %d из 6 направлений; базовый набор полностью заполнен по %d направлениям.\nВалидация: ошибок %d; предупреждений %d; %s.\n%s\nПримечание: паспорт качества не является баллом риска и не доказывает наличие или отсутствие осложнения.", state, metadata, q.FreshnessLabel, q.DirectionsWithData, q.DirectionsBaselineComplete, q.ValidationErrors, q.ValidationWarnings, conflicts, strings.Join(dirLines, "\n"))
}
