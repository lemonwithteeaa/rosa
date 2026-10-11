//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type laboratoryDataQualitySummary struct {
	BaseFilled, BaseTotal         int
	OptionalFilled, OptionalTotal int
	FreshnessDays                 int
	FreshnessKnown                bool
	SourcePresent                 bool
	ConsistencyLabel              string
	FreshnessLabel                string
	SourceLabel                   string
}

func laboratoryAnalysisDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"02.01.2006", "2006-01-02", "02/01/2006", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func laboratoryDataQualitySummaryFor(group LabGroupDef, values map[string]string, scaleModel, date, source string, now time.Time, warnings, errors int) laboratoryDataQualitySummary {
	bf, bt, of, ot := laboratoryDataCoverageForModel(group, values, scaleModel)
	out := laboratoryDataQualitySummary{BaseFilled: bf, BaseTotal: bt, OptionalFilled: of, OptionalTotal: ot, SourcePresent: strings.TrimSpace(source) != ""}
	switch {
	case errors > 0:
		out.ConsistencyLabel = fmt.Sprintf("есть ошибки ввода: %d", errors)
	case warnings > 0:
		out.ConsistencyLabel = fmt.Sprintf("есть предупреждения: %d", warnings)
	default:
		out.ConsistencyLabel = "явных ошибок ввода нет"
	}
	if group.Code == "scale" {
		qc := laboratoryWaterQualityQC(values)
		var flags []string
		if qc.ChargeBalanceOK {
			a := math.Abs(qc.ChargeBalancePct)
			if a > 10 {
				flags = append(flags, "ионный баланс требует проверки")
			} else if a > 5 {
				flags = append(flags, "ионный баланс пограничный")
			}
		}
		if qc.TDSComparisonOK {
			if qc.TDSDifferencePct > 20 {
				flags = append(flags, "TDS существенно расходится с суммой ионов")
			} else if qc.TDSDifferencePct > 10 {
				flags = append(flags, "TDS требует сверки")
			}
		}
		if len(flags) > 0 {
			out.ConsistencyLabel = strings.Join(flags, "; ")
		} else if qc.ChargeBalanceOK || qc.TDSComparisonOK {
			out.ConsistencyLabel = "QC воды без существенных расхождений"
		}
	}
	if t, ok := laboratoryAnalysisDate(date); ok {
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
	if out.SourcePresent {
		out.SourceLabel = strings.TrimSpace(source)
	} else {
		out.SourceLabel = "не указан — прослеживаемость снижена"
	}
	return out
}

func laboratoryDataQualityCardText(group LabGroupDef, values map[string]string, scaleModel, date, source string, now time.Time, warnings, errors int) string {
	q := laboratoryDataQualitySummaryFor(group, values, scaleModel, date, source, now, warnings, errors)
	completeness := "нет обязательных полей"
	if q.BaseTotal > 0 {
		completeness = fmt.Sprintf("%d из %d обязательных; %d из %d дополнительных", q.BaseFilled, q.BaseTotal, q.OptionalFilled, q.OptionalTotal)
	}
	return fmt.Sprintf("КАЧЕСТВО ИСХОДНЫХ ДАННЫХ\r\nПолнота: %s\r\nСогласованность: %s\r\nСвежесть: %s\r\nИсточник: %s\r\nЭто паспорт качества исходных данных, а не итоговый балл и не доказательство осложнения.", completeness, q.ConsistencyLabel, q.FreshnessLabel, q.SourceLabel)
}
