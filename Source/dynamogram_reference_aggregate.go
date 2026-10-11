//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

type DynamogramReferenceAggregateMetrics struct {
	ClassName                 string  `json:"class_name,omitempty"`
	TotalCases                int     `json:"total_cases"`
	ComparedCases             int     `json:"compared_cases"`
	UnprocessedCases          int     `json:"unprocessed_cases"`
	InterpretationMatches     int     `json:"interpretation_matches"`
	ExactMatches              int     `json:"exact_matches"`
	ExpectedHypotheses        int     `json:"expected_hypotheses"`
	MatchedHypotheses         int     `json:"matched_hypotheses"`
	MissingHypotheses         int     `json:"missing_hypotheses"`
	UnexpectedHypotheses      int     `json:"unexpected_hypotheses"`
	HypothesisRecall          float64 `json:"hypothesis_recall,omitempty"`
	HypothesisRecallDefined   bool    `json:"hypothesis_recall_defined"`
	UnexpectedHypothesisShare float64 `json:"unexpected_hypothesis_share,omitempty"`
	UnexpectedShareDefined    bool    `json:"unexpected_share_defined"`
	ExactMatchRate            float64 `json:"exact_match_rate,omitempty"`
	ExactMatchRateDefined     bool    `json:"exact_match_rate_defined"`
	InterpretationMatchRate   float64 `json:"interpretation_match_rate,omitempty"`
	InterpretationRateDefined bool    `json:"interpretation_rate_defined"`
}

type DynamogramReferenceAggregateReport struct {
	SetID                    string                                `json:"set_id"`
	RunID                    string                                `json:"run_id"`
	ComparisonComplete       bool                                  `json:"comparison_complete"`
	AllClassesCovered        bool                                  `json:"all_classes_covered"`
	ReadyForExpertAssessment bool                                  `json:"ready_for_expert_assessment"`
	Overall                  DynamogramReferenceAggregateMetrics   `json:"overall"`
	ByClass                  []DynamogramReferenceAggregateMetrics `json:"by_class"`
	Messages                 []string                              `json:"messages,omitempty"`
}

func finalizeDynamogramReferenceMetrics(metrics *DynamogramReferenceAggregateMetrics) {
	if metrics == nil {
		return
	}
	metrics.UnprocessedCases = metrics.TotalCases - metrics.ComparedCases
	if metrics.ExpectedHypotheses > 0 {
		metrics.HypothesisRecall = float64(metrics.MatchedHypotheses) / float64(metrics.ExpectedHypotheses)
		metrics.HypothesisRecallDefined = true
	}
	predicted := metrics.MatchedHypotheses + metrics.UnexpectedHypotheses
	if predicted > 0 {
		metrics.UnexpectedHypothesisShare = float64(metrics.UnexpectedHypotheses) / float64(predicted)
		metrics.UnexpectedShareDefined = true
	}
	if metrics.ComparedCases > 0 {
		metrics.ExactMatchRate = float64(metrics.ExactMatches) / float64(metrics.ComparedCases)
		metrics.ExactMatchRateDefined = true
		metrics.InterpretationMatchRate = float64(metrics.InterpretationMatches) / float64(metrics.ComparedCases)
		metrics.InterpretationRateDefined = true
	}
}

func addDynamogramReferenceComparison(metrics *DynamogramReferenceAggregateMetrics, item DynamogramReferenceComparisonCase) {
	if metrics == nil || item.Status != "compared" {
		return
	}
	metrics.ComparedCases++
	if item.InterpretationMatch {
		metrics.InterpretationMatches++
	}
	if item.InterpretationMatch && len(item.MissingHypotheses) == 0 && len(item.UnexpectedHypotheses) == 0 {
		metrics.ExactMatches++
	}
	metrics.MatchedHypotheses += len(item.MatchedHypotheses)
	metrics.MissingHypotheses += len(item.MissingHypotheses)
	metrics.ExpectedHypotheses += len(item.MatchedHypotheses) + len(item.MissingHypotheses)
	metrics.UnexpectedHypotheses += len(item.UnexpectedHypotheses)
}

func aggregateDynamogramReferenceComparison(manifest DynamogramReferenceManifest, comparison DynamogramReferenceComparisonReport) DynamogramReferenceAggregateReport {
	report := DynamogramReferenceAggregateReport{SetID: manifest.SetID, RunID: comparison.RunID}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		report.Messages = append(report.Messages, "Манифест отклонён: "+err.Error())
		return report
	}
	if !comparison.ProtocolValid || !strings.EqualFold(strings.TrimSpace(manifest.SetID), strings.TrimSpace(comparison.SetID)) {
		report.Messages = append(report.Messages, "Отчёт сравнения отсутствует, некорректен или относится к другому набору.")
		return report
	}

	comparisonByID := make(map[string]DynamogramReferenceComparisonCase, len(comparison.Cases))
	for _, item := range comparison.Cases {
		comparisonByID[strings.ToLower(strings.TrimSpace(item.ReferenceID))] = item
	}
	classMetrics := make(map[string]*DynamogramReferenceAggregateMetrics)
	classNames := make(map[string]string)
	report.Overall.TotalCases = len(manifest.Cases)
	for _, reference := range manifest.Cases {
		item, found := comparisonByID[strings.ToLower(strings.TrimSpace(reference.ID))]
		if found {
			addDynamogramReferenceComparison(&report.Overall, item)
		}
		for _, className := range reference.ExpectedClasses {
			key := strings.ToLower(strings.TrimSpace(className))
			metrics := classMetrics[key]
			if metrics == nil {
				metrics = &DynamogramReferenceAggregateMetrics{ClassName: strings.TrimSpace(className)}
				classMetrics[key] = metrics
				classNames[key] = strings.TrimSpace(className)
			}
			metrics.TotalCases++
			if found {
				addDynamogramReferenceComparison(metrics, item)
			}
		}
	}
	finalizeDynamogramReferenceMetrics(&report.Overall)
	keys := make([]string, 0, len(classMetrics))
	for key := range classMetrics {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return classNames[keys[i]] < classNames[keys[j]] })
	report.AllClassesCovered = len(keys) > 0
	for _, key := range keys {
		metrics := classMetrics[key]
		finalizeDynamogramReferenceMetrics(metrics)
		if metrics.ComparedCases == 0 {
			report.AllClassesCovered = false
		}
		report.ByClass = append(report.ByClass, *metrics)
	}
	report.ComparisonComplete = comparison.ComparisonComplete && report.Overall.TotalCases > 0 && report.Overall.ComparedCases == report.Overall.TotalCases
	report.ReadyForExpertAssessment = report.ComparisonComplete && report.AllClassesCovered
	report.Messages = append(report.Messages, "Один случай может относиться к нескольким классам; строки классов нельзя суммировать как независимые наблюдения.")
	report.Messages = append(report.Messages, "Метрики описывают набор, но не содержат автоматически назначенных норм, проходных границ или решения об утверждении порогов.")
	if !report.ComparisonComplete {
		report.Messages = append(report.Messages, "Сравнение неполное: показатели нельзя использовать как итоговую оценку набора.")
	}
	return report
}

func dynamogramReferencePercent(value float64, defined bool) string {
	if !defined {
		return "не рассчитывается"
	}
	return fmt.Sprintf("%.1f %%", value*100)
}

func formatDynamogramReferenceAggregateReport(report DynamogramReferenceAggregateReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "АГРЕГИРОВАННАЯ СВЕРКА %s / %s\r\n", dash(report.SetID), dash(report.RunID))
	writeDynamogramReferenceMetrics := func(title string, metrics DynamogramReferenceAggregateMetrics) {
		fmt.Fprintf(&b, "%s: случаев %d; сопоставлено %d; без результата %d; точных совпадений %d.\r\n", title, metrics.TotalCases, metrics.ComparedCases, metrics.UnprocessedCases, metrics.ExactMatches)
		fmt.Fprintf(&b, "  Полнота ожидаемых гипотез: %s; доля лишних гипотез: %s; совпадение интерпретации: %s; полное совпадение: %s.\r\n", dynamogramReferencePercent(metrics.HypothesisRecall, metrics.HypothesisRecallDefined), dynamogramReferencePercent(metrics.UnexpectedHypothesisShare, metrics.UnexpectedShareDefined), dynamogramReferencePercent(metrics.InterpretationMatchRate, metrics.InterpretationRateDefined), dynamogramReferencePercent(metrics.ExactMatchRate, metrics.ExactMatchRateDefined))
	}
	writeDynamogramReferenceMetrics("Общий итог", report.Overall)
	for _, metrics := range report.ByClass {
		writeDynamogramReferenceMetrics("Класс "+metrics.ClassName, metrics)
	}
	for _, message := range report.Messages {
		fmt.Fprintf(&b, "- %s\r\n", message)
	}
	fmt.Fprintf(&b, "Все классы представлены: %s. Готово к экспертной оценке: %s. Автоматического решения о порогах нет.", yesNo(report.AllClassesCovered), yesNo(report.ReadyForExpertAssessment))
	return b.String()
}
