//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const dynThresholdDecisionNotMade = "not_decided"

type DynamogramReferenceQualificationReport struct {
	SchemaVersion          int                                 `json:"schema_version"`
	SetID                  string                              `json:"set_id"`
	RunID                  string                              `json:"run_id"`
	Status                 string                              `json:"status"`
	ThresholdStatus        string                              `json:"threshold_status"`
	ReadyForExpertDecision bool                                `json:"ready_for_expert_decision"`
	FileValidation         DynamogramReferenceValidationReport `json:"file_validation"`
	Comparison             DynamogramReferenceComparisonReport `json:"comparison"`
	Aggregate              DynamogramReferenceAggregateReport  `json:"aggregate"`
	Messages               []string                            `json:"messages,omitempty"`
}

type DynamogramReferenceExpertDecision struct {
	DecisionID                string `json:"decision_id"`
	SetID                     string `json:"set_id"`
	RunID                     string `json:"run_id"`
	CreatedAt                 string `json:"created_at"`
	Author                    string `json:"author"`
	AuthorRole                string `json:"author_role"`
	Status                    string `json:"status"`
	Rationale                 string `json:"rationale"`
	QualificationReportSHA256 string `json:"qualification_report_sha256"`
	ThresholdDecision         string `json:"threshold_decision"`
}

type DynamogramReferenceExpertDecisionJournal struct {
	SchemaVersion int                                 `json:"schema_version"`
	SetID         string                              `json:"set_id"`
	Decisions     []DynamogramReferenceExpertDecision `json:"decisions"`
}

func buildDynamogramReferenceQualificationReport(manifest DynamogramReferenceManifest, files DynamogramReferenceValidationReport, comparison DynamogramReferenceComparisonReport, aggregate DynamogramReferenceAggregateReport) DynamogramReferenceQualificationReport {
	report := DynamogramReferenceQualificationReport{
		SchemaVersion:   1,
		SetID:           manifest.SetID,
		RunID:           comparison.RunID,
		Status:          "not_ready",
		ThresholdStatus: manifest.ThresholdStatus,
		FileValidation:  files,
		Comparison:      comparison,
		Aggregate:       aggregate,
	}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		report.Messages = append(report.Messages, "Манифест отклонён: "+err.Error())
		return report
	}
	idsMatch := strings.EqualFold(strings.TrimSpace(manifest.SetID), strings.TrimSpace(files.SetID)) &&
		strings.EqualFold(strings.TrimSpace(manifest.SetID), strings.TrimSpace(comparison.SetID)) &&
		strings.EqualFold(strings.TrimSpace(manifest.SetID), strings.TrimSpace(aggregate.SetID)) &&
		strings.EqualFold(strings.TrimSpace(comparison.RunID), strings.TrimSpace(aggregate.RunID))
	if !idsMatch {
		report.Messages = append(report.Messages, "Входные отчёты относятся к разным наборам или прогонам.")
		return report
	}
	report.ReadyForExpertDecision = files.ReadyForQualification && comparison.ComparisonComplete && aggregate.ReadyForExpertAssessment
	if report.ReadyForExpertDecision {
		report.Status = "ready_for_expert_decision"
	} else {
		report.Messages = append(report.Messages, "Пакет неполный: экспертное решение о квалификации набора принимать рано.")
	}
	report.Messages = append(report.Messages, "Квалификационный отчёт не изменяет и не утверждает пороги автоматически.")
	return report
}

func hashDynamogramReferenceQualificationReport(report DynamogramReferenceQualificationReport) (string, error) {
	raw, err := json.Marshal(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func validateDynamogramReferenceExpertDecision(decision DynamogramReferenceExpertDecision, report DynamogramReferenceQualificationReport) error {
	if strings.TrimSpace(decision.DecisionID) == "" || strings.TrimSpace(decision.SetID) == "" || strings.TrimSpace(decision.RunID) == "" {
		return errors.New("не заполнены идентификаторы экспертного решения")
	}
	if !strings.EqualFold(strings.TrimSpace(decision.SetID), strings.TrimSpace(report.SetID)) || !strings.EqualFold(strings.TrimSpace(decision.RunID), strings.TrimSpace(report.RunID)) {
		return errors.New("экспертное решение относится к другому набору или прогону")
	}
	if strings.TrimSpace(decision.CreatedAt) == "" || strings.TrimSpace(decision.Author) == "" || strings.TrimSpace(decision.AuthorRole) == "" || strings.TrimSpace(decision.Rationale) == "" {
		return errors.New("нужны дата, автор, роль и обоснование экспертного решения")
	}
	if decision.Status != "needs_more_data" && decision.Status != "reference_set_qualified" && decision.Status != "reference_set_rejected" {
		return errors.New("неизвестный статус экспертного решения")
	}
	if decision.Status == "reference_set_qualified" && !report.ReadyForExpertDecision {
		return errors.New("неполный квалификационный отчёт нельзя принять как квалифицированный набор")
	}
	if decision.ThresholdDecision != dynThresholdDecisionNotMade {
		return errors.New("журнал квалификации набора не может утверждать пороги")
	}
	expectedHash, err := hashDynamogramReferenceQualificationReport(report)
	if err != nil {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(decision.QualificationReportSHA256), expectedHash) {
		return errors.New("SHA-256 квалификационного отчёта не совпадает")
	}
	return nil
}

func validateDynamogramReferenceExpertDecisionJournal(journal DynamogramReferenceExpertDecisionJournal, report DynamogramReferenceQualificationReport) error {
	if journal.SchemaVersion != 1 || !strings.EqualFold(strings.TrimSpace(journal.SetID), strings.TrimSpace(report.SetID)) {
		return errors.New("журнал имеет неверную версию или относится к другому набору")
	}
	seen := make(map[string]bool, len(journal.Decisions))
	for i, decision := range journal.Decisions {
		key := strings.ToLower(strings.TrimSpace(decision.DecisionID))
		if key == "" || seen[key] {
			return fmt.Errorf("решение %d: decision_id отсутствует или повторяется", i+1)
		}
		seen[key] = true
		if err := validateDynamogramReferenceExpertDecision(decision, report); err != nil {
			return fmt.Errorf("решение %d: %w", i+1, err)
		}
	}
	return nil
}

func formatDynamogramReferenceQualificationReport(report DynamogramReferenceQualificationReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "КВАЛИФИКАЦИОННЫЙ ОТЧЁТ %s / %s\r\n", dash(report.SetID), dash(report.RunID))
	fmt.Fprintf(&b, "Проверка файлов: %s; сравнение полное: %s; поклассовая оценка готова: %s.\r\n", yesNo(report.FileValidation.ReadyForQualification), yesNo(report.Comparison.ComparisonComplete), yesNo(report.Aggregate.ReadyForExpertAssessment))
	fmt.Fprintf(&b, "Статус пакета: %s. Готово к экспертному решению: %s.\r\n", report.Status, yesNo(report.ReadyForExpertDecision))
	fmt.Fprintf(&b, "Статус порогов: %s.\r\n", report.ThresholdStatus)
	for _, message := range report.Messages {
		fmt.Fprintf(&b, "- %s\r\n", message)
	}
	b.WriteString("Решение фиксируется отдельно с автором, ролью, датой, обоснованием и SHA-256 этого отчёта. Автоматического утверждения порогов нет.")
	return b.String()
}
