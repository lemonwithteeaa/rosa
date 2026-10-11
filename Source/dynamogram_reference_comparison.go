//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

type DynamogramReferenceRunCase struct {
	ReferenceID          string   `json:"reference_id"`
	AnalysisID           string   `json:"analysis_id,omitempty"`
	SourceSHA256         string   `json:"source_sha256"`
	ProcessingStatus     string   `json:"processing_status"`
	ActualInterpretation string   `json:"actual_interpretation,omitempty"`
	ActualHypotheses     []string `json:"actual_hypotheses,omitempty"`
	Error                string   `json:"error,omitempty"`
}

type DynamogramReferenceRun struct {
	SchemaVersion        int                          `json:"schema_version"`
	RunID                string                       `json:"run_id"`
	SetID                string                       `json:"set_id"`
	CreatedAt            string                       `json:"created_at"`
	ApplicationVersion   string                       `json:"application_version"`
	ThresholdStatusAtRun string                       `json:"threshold_status_at_run"`
	Cases                []DynamogramReferenceRunCase `json:"cases"`
}

type DynamogramReferenceComparisonCase struct {
	ReferenceID            string   `json:"reference_id"`
	Status                 string   `json:"status"`
	InterpretationMatch    bool     `json:"interpretation_match"`
	ExpectedInterpretation string   `json:"expected_interpretation,omitempty"`
	ActualInterpretation   string   `json:"actual_interpretation,omitempty"`
	MatchedHypotheses      []string `json:"matched_hypotheses,omitempty"`
	MissingHypotheses      []string `json:"missing_hypotheses,omitempty"`
	UnexpectedHypotheses   []string `json:"unexpected_hypotheses,omitempty"`
	Details                string   `json:"details,omitempty"`
}

type DynamogramReferenceComparisonReport struct {
	SetID                 string                              `json:"set_id"`
	RunID                 string                              `json:"run_id"`
	ProtocolValid         bool                                `json:"protocol_valid"`
	ComparisonComplete    bool                                `json:"comparison_complete"`
	ReadyForExpertReview  bool                                `json:"ready_for_expert_review"`
	TotalReferences       int                                 `json:"total_references"`
	Compared              int                                 `json:"compared"`
	ExactMatches          int                                 `json:"exact_matches"`
	InterpretationMatches int                                 `json:"interpretation_matches"`
	MatchedHypotheses     int                                 `json:"matched_hypotheses"`
	MissingHypotheses     int                                 `json:"missing_hypotheses"`
	UnexpectedHypotheses  int                                 `json:"unexpected_hypotheses"`
	UnexpectedRunIDs      []string                            `json:"unexpected_run_ids,omitempty"`
	Cases                 []DynamogramReferenceComparisonCase `json:"cases,omitempty"`
	Messages              []string                            `json:"messages,omitempty"`
}

func loadDynamogramReferenceRun(path string) (DynamogramReferenceRun, error) {
	var run DynamogramReferenceRun
	info, err := os.Stat(path)
	if err != nil {
		return run, err
	}
	if info.Size() > 5*1024*1024 {
		return run, errors.New("протокол прогона превышает 5 МБ")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return run, err
	}
	if err = json.Unmarshal(raw, &run); err != nil {
		return run, fmt.Errorf("некорректный JSON протокола прогона: %w", err)
	}
	if err = validateDynamogramReferenceRun(run); err != nil {
		return run, err
	}
	return run, nil
}

func validateDynamogramReferenceRun(run DynamogramReferenceRun) error {
	if run.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия протокола: %d", run.SchemaVersion)
	}
	if strings.TrimSpace(run.RunID) == "" || strings.TrimSpace(run.SetID) == "" {
		return errors.New("в протоколе должны быть заданы run_id и set_id")
	}
	if strings.TrimSpace(run.CreatedAt) == "" || strings.TrimSpace(run.ApplicationVersion) == "" {
		return errors.New("в протоколе должны быть заданы дата и версия РОСА")
	}
	if run.ThresholdStatusAtRun != dynThresholdsScreening && run.ThresholdStatusAtRun != dynThresholdsApproved {
		return errors.New("неизвестный статус порогов в момент прогона")
	}
	seen := make(map[string]bool, len(run.Cases))
	for i, item := range run.Cases {
		key := strings.ToLower(strings.TrimSpace(item.ReferenceID))
		if key == "" || seen[key] {
			return fmt.Errorf("результат %d: reference_id отсутствует или повторяется", i+1)
		}
		seen[key] = true
		if !dynamogramReferenceSHA256.MatchString(strings.TrimSpace(item.SourceSHA256)) {
			return fmt.Errorf("результат %d: не задан корректный SHA-256 исходника", i+1)
		}
		if item.ProcessingStatus != "completed" && item.ProcessingStatus != "rejected" && item.ProcessingStatus != "error" {
			return fmt.Errorf("результат %d: неизвестный processing_status", i+1)
		}
		if item.ProcessingStatus == "completed" && strings.TrimSpace(item.ActualInterpretation) == "" {
			return fmt.Errorf("результат %d: отсутствует фактическая интерпретация", i+1)
		}
		if item.ProcessingStatus == "error" && strings.TrimSpace(item.Error) == "" {
			return fmt.Errorf("результат %d: для статуса error нужно описание", i+1)
		}
	}
	return nil
}

func normalizeDynamogramComparisonText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSpace(value)
	prefix := "предварительная гипотеза:"
	if strings.HasPrefix(strings.ToLower(value), prefix) {
		value = strings.TrimSpace(value[len(prefix):])
	}
	if index := strings.Index(strings.ToLower(value), "\nоснование:"); index >= 0 {
		value = value[:index]
	}
	value = strings.Trim(value, " .;:-—")
	value = strings.ReplaceAll(strings.ToLower(value), "ё", "е")
	return strings.Join(strings.Fields(value), " ")
}

func dynamogramHypothesisMap(values []string) map[string]string {
	out := make(map[string]string, len(values))
	for _, value := range values {
		key := normalizeDynamogramComparisonText(value)
		if key != "" {
			if _, exists := out[key]; !exists {
				out[key] = strings.TrimSpace(value)
			}
		}
	}
	return out
}

func compareDynamogramReferenceRun(manifest DynamogramReferenceManifest, run DynamogramReferenceRun) DynamogramReferenceComparisonReport {
	report := DynamogramReferenceComparisonReport{SetID: manifest.SetID, RunID: run.RunID, TotalReferences: len(manifest.Cases)}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		report.Messages = append(report.Messages, "Манифест отклонён: "+err.Error())
		return report
	}
	if err := validateDynamogramReferenceRun(run); err != nil {
		report.Messages = append(report.Messages, "Протокол отклонён: "+err.Error())
		return report
	}
	if !strings.EqualFold(strings.TrimSpace(manifest.SetID), strings.TrimSpace(run.SetID)) {
		report.Messages = append(report.Messages, "set_id протокола не совпадает с эталонным набором.")
		return report
	}
	report.ProtocolValid = true
	runByReference := make(map[string]DynamogramReferenceRunCase, len(run.Cases))
	referenceIDs := make(map[string]bool, len(manifest.Cases))
	for _, item := range manifest.Cases {
		referenceIDs[strings.ToLower(strings.TrimSpace(item.ID))] = true
	}
	for _, item := range run.Cases {
		key := strings.ToLower(strings.TrimSpace(item.ReferenceID))
		runByReference[key] = item
		if !referenceIDs[key] {
			report.UnexpectedRunIDs = append(report.UnexpectedRunIDs, item.ReferenceID)
		}
	}
	sort.Strings(report.UnexpectedRunIDs)

	for _, expected := range manifest.Cases {
		comparison := DynamogramReferenceComparisonCase{
			ReferenceID:            expected.ID,
			ExpectedInterpretation: expected.ExpectedInterpretation,
		}
		actual, exists := runByReference[strings.ToLower(strings.TrimSpace(expected.ID))]
		if !exists {
			comparison.Status = "missing_run"
			comparison.Details = "для эталона нет результата прогона"
			comparison.MissingHypotheses = append(comparison.MissingHypotheses, expected.ExpectedHypotheses...)
			report.MissingHypotheses += len(comparison.MissingHypotheses)
			report.Cases = append(report.Cases, comparison)
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(expected.SourceSHA256), strings.TrimSpace(actual.SourceSHA256)) {
			comparison.Status = "source_hash_mismatch"
			comparison.Details = "протокол сформирован не по тому файлу: SHA-256 отличается"
			report.Cases = append(report.Cases, comparison)
			continue
		}
		if actual.ProcessingStatus != "completed" {
			comparison.Status = actual.ProcessingStatus
			comparison.Details = strings.TrimSpace(actual.Error)
			report.Cases = append(report.Cases, comparison)
			continue
		}

		comparison.Status = "compared"
		comparison.ActualInterpretation = actual.ActualInterpretation
		comparison.InterpretationMatch = normalizeDynamogramComparisonText(expected.ExpectedInterpretation) == normalizeDynamogramComparisonText(actual.ActualInterpretation)
		if comparison.InterpretationMatch {
			report.InterpretationMatches++
		}
		expectedHypotheses := dynamogramHypothesisMap(expected.ExpectedHypotheses)
		actualHypotheses := dynamogramHypothesisMap(actual.ActualHypotheses)
		for key, original := range expectedHypotheses {
			if _, found := actualHypotheses[key]; found {
				comparison.MatchedHypotheses = append(comparison.MatchedHypotheses, original)
			} else {
				comparison.MissingHypotheses = append(comparison.MissingHypotheses, original)
			}
		}
		for key, original := range actualHypotheses {
			if _, found := expectedHypotheses[key]; !found {
				comparison.UnexpectedHypotheses = append(comparison.UnexpectedHypotheses, original)
			}
		}
		sort.Strings(comparison.MatchedHypotheses)
		sort.Strings(comparison.MissingHypotheses)
		sort.Strings(comparison.UnexpectedHypotheses)
		report.MatchedHypotheses += len(comparison.MatchedHypotheses)
		report.MissingHypotheses += len(comparison.MissingHypotheses)
		report.UnexpectedHypotheses += len(comparison.UnexpectedHypotheses)
		report.Compared++
		if comparison.InterpretationMatch && len(comparison.MissingHypotheses) == 0 && len(comparison.UnexpectedHypotheses) == 0 {
			report.ExactMatches++
		}
		report.Cases = append(report.Cases, comparison)
	}

	report.ComparisonComplete = report.TotalReferences > 0 && report.Compared == report.TotalReferences && len(report.UnexpectedRunIDs) == 0
	report.ReadyForExpertReview = report.ComparisonComplete
	if report.TotalReferences == 0 {
		report.Messages = append(report.Messages, "Эталонный набор пуст: сравнение не выполнено.")
	}
	if len(report.UnexpectedRunIDs) > 0 {
		report.Messages = append(report.Messages, "В протоколе есть результаты без эталонных карточек.")
	}
	return report
}

func formatDynamogramReferenceComparisonReport(report DynamogramReferenceComparisonReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "СВЕРКА ЭТАЛОННОГО ПРОГОНА %s / %s\r\n", dash(report.SetID), dash(report.RunID))
	fmt.Fprintf(&b, "Эталонов: %d; сопоставлено: %d; полных совпадений: %d.\r\n", report.TotalReferences, report.Compared, report.ExactMatches)
	fmt.Fprintf(&b, "Интерпретация совпала: %d; гипотез совпало: %d; пропущено: %d; лишних: %d.\r\n", report.InterpretationMatches, report.MatchedHypotheses, report.MissingHypotheses, report.UnexpectedHypotheses)
	for _, item := range report.Cases {
		fmt.Fprintf(&b, "- %s: %s", dash(item.ReferenceID), dynamogramComparisonStatusLabel(item.Status))
		if item.Details != "" {
			fmt.Fprintf(&b, " — %s", item.Details)
		}
		fmt.Fprintf(&b, "; пропущено гипотез %d; лишних %d.\r\n", len(item.MissingHypotheses), len(item.UnexpectedHypotheses))
	}
	for _, id := range report.UnexpectedRunIDs {
		fmt.Fprintf(&b, "- Результат без эталонной карточки: %s.\r\n", id)
	}
	for _, message := range report.Messages {
		fmt.Fprintf(&b, "- %s\r\n", message)
	}
	fmt.Fprintf(&b, "Сравнение полное: %s. Готово к экспертному рассмотрению: %s. Пороги автоматически не утверждаются.", yesNo(report.ComparisonComplete), yesNo(report.ReadyForExpertReview))
	return b.String()
}

func dynamogramComparisonStatusLabel(status string) string {
	switch status {
	case "compared":
		return "сопоставлено"
	case "missing_run":
		return "результат прогона отсутствует"
	case "source_hash_mismatch":
		return "не совпадает исходный файл"
	case "rejected":
		return "контур отклонён при проверке"
	case "error":
		return "ошибка обработки"
	default:
		return "неизвестный статус"
	}
}
