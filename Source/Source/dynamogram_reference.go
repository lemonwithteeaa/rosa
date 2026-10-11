//go:build windows

package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	dynReferenceSetDraft     = "draft"
	dynReferenceSetQualified = "qualified"
	dynThresholdsScreening   = "screening_not_approved"
	dynThresholdsApproved    = "approved"
)

type DynamogramReferenceCase struct {
	ID                     string   `json:"id"`
	SourceFile             string   `json:"source_file"`
	SourceSHA256           string   `json:"source_sha256"`
	SourceKind             string   `json:"source_kind"`
	LiftType               string   `json:"lift_type"`
	ExpectedClasses        []string `json:"expected_classes"`
	ExpectedInterpretation string   `json:"expected_interpretation"`
	ExpectedHypotheses     []string `json:"expected_hypotheses,omitempty"`
	ExpertStatus           string   `json:"expert_status"`
	ExpertRole             string   `json:"expert_role,omitempty"`
	ReviewedAt             string   `json:"reviewed_at,omitempty"`
}

type DynamogramReferenceManifest struct {
	SchemaVersion       int                       `json:"schema_version"`
	SetID               string                    `json:"set_id"`
	SetStatus           string                    `json:"set_status"`
	ThresholdStatus     string                    `json:"threshold_status"`
	QualificationReport string                    `json:"qualification_report,omitempty"`
	Cases               []DynamogramReferenceCase `json:"cases"`
}

type DynamogramReferenceFileResult struct {
	ID           string `json:"id"`
	SourceFile   string `json:"source_file"`
	Status       string `json:"status"`
	ActualSHA256 string `json:"actual_sha256,omitempty"`
	Details      string `json:"details,omitempty"`
}

type DynamogramReferenceValidationReport struct {
	SetID                   string                          `json:"set_id"`
	ManifestValid           bool                            `json:"manifest_valid"`
	FilesReady              bool                            `json:"files_ready"`
	ReadyForQualification   bool                            `json:"ready_for_qualification"`
	ReadyForThresholdReview bool                            `json:"ready_for_threshold_review"`
	Total                   int                             `json:"total"`
	Passed                  int                             `json:"passed"`
	Missing                 int                             `json:"missing"`
	HashMismatch            int                             `json:"hash_mismatch"`
	Invalid                 int                             `json:"invalid"`
	UnexpectedFiles         []string                        `json:"unexpected_files,omitempty"`
	Items                   []DynamogramReferenceFileResult `json:"items,omitempty"`
	Messages                []string                        `json:"messages,omitempty"`
}

//go:embed dynamogram_reference_manifest.json
var embeddedDynamogramReferenceManifest []byte

func loadEmbeddedDynamogramReferenceManifest() (DynamogramReferenceManifest, error) {
	var manifest DynamogramReferenceManifest
	if err := json.Unmarshal(embeddedDynamogramReferenceManifest, &manifest); err != nil {
		return manifest, fmt.Errorf("не удалось прочитать манифест эталонных динамограмм: %w", err)
	}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func loadDynamogramReferenceManifestFile(path string) (DynamogramReferenceManifest, error) {
	var manifest DynamogramReferenceManifest
	info, err := os.Stat(path)
	if err != nil {
		return manifest, err
	}
	if info.Size() > 2*1024*1024 {
		return manifest, errors.New("манифест эталонных динамограмм превышает 2 МБ")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("некорректный JSON манифеста: %w", err)
	}
	if err = validateDynamogramReferenceManifest(manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func validateDynamogramReferenceManifest(manifest DynamogramReferenceManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("неподдерживаемая версия манифеста: %d", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.SetID) == "" {
		return errors.New("не задан идентификатор эталонного набора")
	}
	if manifest.SetStatus != dynReferenceSetDraft && manifest.SetStatus != dynReferenceSetQualified {
		return fmt.Errorf("неизвестный статус эталонного набора: %s", manifest.SetStatus)
	}
	if manifest.ThresholdStatus != dynThresholdsScreening && manifest.ThresholdStatus != dynThresholdsApproved {
		return fmt.Errorf("неизвестный статус порогов: %s", manifest.ThresholdStatus)
	}
	if manifest.ThresholdStatus == dynThresholdsApproved {
		if manifest.SetStatus != dynReferenceSetQualified {
			return errors.New("пороги нельзя утвердить до квалификации эталонного набора")
		}
		if strings.TrimSpace(manifest.QualificationReport) == "" {
			return errors.New("для утверждения порогов нужен отчёт о проверке эталонного набора")
		}
		if len(manifest.Cases) == 0 {
			return errors.New("пороги нельзя утвердить на пустом эталонном наборе")
		}
	}

	seen := make(map[string]bool, len(manifest.Cases))
	for i, item := range manifest.Cases {
		if err := validateDynamogramReferenceCase(item, manifest.SetStatus == dynReferenceSetQualified); err != nil {
			return fmt.Errorf("эталон %d: %w", i+1, err)
		}
		key := strings.ToLower(strings.TrimSpace(item.ID))
		if seen[key] {
			return fmt.Errorf("эталон %d: повторяется идентификатор %s", i+1, item.ID)
		}
		seen[key] = true
	}
	return nil
}

var dynamogramReferenceSHA256 = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func validateDynamogramReferenceCase(item DynamogramReferenceCase, requireConfirmed bool) error {
	if strings.TrimSpace(item.ID) == "" {
		return errors.New("не задан идентификатор")
	}
	cleanSource := filepath.ToSlash(filepath.Clean(strings.TrimSpace(item.SourceFile)))
	if filepath.IsAbs(item.SourceFile) || cleanSource == "." || strings.HasPrefix(cleanSource, "../") || !strings.HasPrefix(cleanSource, "samples/") {
		return errors.New("исходник должен иметь относительный путь внутри каталога samples")
	}
	if !dynamogramReferenceSHA256.MatchString(strings.TrimSpace(item.SourceSHA256)) {
		return errors.New("не задан корректный SHA-256 исходника")
	}
	if item.SourceKind != "real_image" && item.SourceKind != "digital_export" {
		return errors.New("допускаются только реальные изображения или цифровые выгрузки; синтетические образцы не являются эталонами")
	}
	if strings.TrimSpace(item.LiftType) == "" {
		return errors.New("не указан тип ГНО")
	}
	if len(item.ExpectedClasses) == 0 {
		return errors.New("не указан ни один ожидаемый диагностический класс")
	}
	classes := make(map[string]bool, len(item.ExpectedClasses))
	for _, className := range item.ExpectedClasses {
		key := strings.ToLower(strings.TrimSpace(className))
		if key == "" || classes[key] {
			return errors.New("ожидаемые диагностические классы не должны быть пустыми или повторяться")
		}
		classes[key] = true
	}
	if strings.TrimSpace(item.ExpectedInterpretation) == "" {
		return errors.New("не указана ожидаемая экспертная интерпретация")
	}
	if item.ExpertStatus != "pending" && item.ExpertStatus != "confirmed" {
		return errors.New("статус экспертизы должен быть «ожидает экспертизы» (pending) или «подтверждено» (confirmed)")
	}
	if requireConfirmed && item.ExpertStatus != "confirmed" {
		return errors.New("квалифицированный набор может содержать только подтверждённые эталоны")
	}
	if item.ExpertStatus == "confirmed" && strings.TrimSpace(item.ExpertRole) == "" {
		return errors.New("для подтверждённого эталона должна быть указана роль эксперта")
	}
	return nil
}

func dynamogramReferenceExtensionAllowed(sourceKind, path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if sourceKind == "real_image" {
		return ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".bmp"
	}
	if sourceKind == "digital_export" {
		return ext == ".csv" || ext == ".txt"
	}
	return false
}

func hashDynamogramReferenceFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("исходник должен быть обычным файлом, символические ссылки запрещены")
	}
	const maxReferenceBytes int64 = 50 * 1024 * 1024
	if info.Size() > maxReferenceBytes {
		return "", errors.New("исходник превышает допустимый размер 50 МБ")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validateDynamogramReferenceFiles(manifest DynamogramReferenceManifest, root string) DynamogramReferenceValidationReport {
	report := DynamogramReferenceValidationReport{SetID: manifest.SetID, Total: len(manifest.Cases)}
	if err := validateDynamogramReferenceManifest(manifest); err != nil {
		report.Messages = append(report.Messages, "Манифест отклонён: "+err.Error())
		return report
	}
	report.ManifestValid = true
	registered := make(map[string]bool, len(manifest.Cases))
	allConfirmed := true
	for _, item := range manifest.Cases {
		clean := filepath.ToSlash(filepath.Clean(item.SourceFile))
		registered[strings.ToLower(clean)] = true
		result := DynamogramReferenceFileResult{ID: item.ID, SourceFile: clean}
		if item.ExpertStatus != "confirmed" {
			allConfirmed = false
		}
		if !dynamogramReferenceExtensionAllowed(item.SourceKind, clean) {
			result.Status = "invalid_extension"
			result.Details = "расширение файла не соответствует типу источника"
			report.Invalid++
			report.Items = append(report.Items, result)
			continue
		}
		actual, err := hashDynamogramReferenceFile(filepath.Join(root, filepath.FromSlash(clean)))
		if errors.Is(err, os.ErrNotExist) {
			result.Status = "missing"
			result.Details = "файл из манифеста не найден"
			report.Missing++
		} else if err != nil {
			result.Status = "invalid_file"
			result.Details = err.Error()
			report.Invalid++
		} else if !strings.EqualFold(actual, item.SourceSHA256) {
			result.Status = "hash_mismatch"
			result.ActualSHA256 = actual
			result.Details = "SHA-256 файла не совпадает с манифестом"
			report.HashMismatch++
		} else {
			result.Status = "ok"
			result.ActualSHA256 = actual
			report.Passed++
		}
		report.Items = append(report.Items, result)
	}

	samplesRoot := filepath.Join(root, "samples")
	_ = filepath.WalkDir(samplesRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || strings.EqualFold(entry.Name(), "README.txt") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !registered[strings.ToLower(rel)] {
			report.UnexpectedFiles = append(report.UnexpectedFiles, rel)
		}
		return nil
	})
	sort.Strings(report.UnexpectedFiles)

	report.FilesReady = report.Total > 0 && report.Passed == report.Total && len(report.UnexpectedFiles) == 0
	report.ReadyForQualification = report.FilesReady && allConfirmed
	report.ReadyForThresholdReview = report.ReadyForQualification && manifest.SetStatus == dynReferenceSetQualified && strings.TrimSpace(manifest.QualificationReport) != ""
	if report.Total == 0 {
		report.Messages = append(report.Messages, "Эталонные случаи отсутствуют: набор остаётся пустым черновиком.")
	}
	if len(report.UnexpectedFiles) > 0 {
		report.Messages = append(report.Messages, "В samples есть файлы без карточек манифеста.")
	}
	if !allConfirmed && report.Total > 0 {
		report.Messages = append(report.Messages, "Не все случаи подтверждены экспертом.")
	}
	return report
}

func formatDynamogramReferenceValidationReport(report DynamogramReferenceValidationReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ПРОВЕРКА ЭТАЛОННОЙ БИБЛИОТЕКИ %s\r\n", dash(report.SetID))
	fmt.Fprintf(&b, "Манифест: %s. Случаев: %d; успешно: %d; отсутствует: %d; SHA-256 не совпал: %d; некорректно: %d.\r\n", yesNo(report.ManifestValid), report.Total, report.Passed, report.Missing, report.HashMismatch, report.Invalid)
	for _, item := range report.Items {
		fmt.Fprintf(&b, "- %s / %s: %s", dash(item.ID), dash(item.SourceFile), dynamogramReferenceStatusLabel(item.Status))
		if item.Details != "" {
			fmt.Fprintf(&b, " — %s", item.Details)
		}
		b.WriteString(".\r\n")
	}
	for _, path := range report.UnexpectedFiles {
		fmt.Fprintf(&b, "- Без карточки манифеста: %s.\r\n", path)
	}
	for _, message := range report.Messages {
		fmt.Fprintf(&b, "- %s\r\n", message)
	}
	fmt.Fprintf(&b, "Файлы готовы: %s. Готово к квалификации: %s. Готово к рассмотрению порогов: %s.", yesNo(report.FilesReady), yesNo(report.ReadyForQualification), yesNo(report.ReadyForThresholdReview))
	return b.String()
}

func dynamogramReferenceStatusLabel(status string) string {
	switch status {
	case "ok":
		return "файл найден, SHA-256 совпадает"
	case "missing":
		return "файл отсутствует"
	case "hash_mismatch":
		return "SHA-256 не совпадает"
	case "invalid_extension":
		return "неверный тип файла"
	case "invalid_file":
		return "файл отклонён"
	default:
		return "неизвестный результат проверки"
	}
}

func yesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}
