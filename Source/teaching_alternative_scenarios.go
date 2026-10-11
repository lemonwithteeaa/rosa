//go:build windows

package main

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// teachingAlternativeTemplate is the deterministic fixture behind the three
// complete lessons. Every row is a real parallel scheme: it has its own
// methods, resource status, forecast and economics and can be opened on the
// Methods, Economics and Result pages.
type teachingAlternativeTemplate struct {
	ID, Name                                string
	Methods                                 []string
	Availability, AvailabilityNote          string
	ForecastLow, ForecastBase, ForecastHigh float64
	AnnualOPEXRub, OneTimeCAPEXRub          float64
	NetBenefitK, BenefitCostRatio           float64
	PaybackMonths                           float64
	SelectionKind, SelectionBasis           string
	Selected                                bool
}

func teachingAlternativeTemplates(objectKey string) ([]teachingAlternativeTemplate, bool) {
	well, ok := trainingWellForObject(objectKey)
	if !ok {
		return nil, false
	}
	if len(well.Failures) == 0 {
		return nil, false
	}
	e := well.Failures[len(well.Failures)-1]
	base := float64(e.MRP)
	targets := []float64{float64(e.ResultingMRP), float64(e.ResultingMRP + 30), float64(e.ResultingMRP + 60)}
	methods := []string{e.MethodAfter, e.MethodAfter + " · лабораторная корректировка", "Комплекс: " + e.MethodAfter + " + мониторинг"}
	names := []string{"Повтор метода", "Новый подбор", "Комплексная защита"}
	caps := []float64{280000 + float64(well.Ordinal)*3500, 520000 + float64(well.Ordinal)*3500, 760000 + float64(well.Ordinal)*3500}
	opex := []float64{115000, 165000, 235000}
	out := make([]teachingAlternativeTemplate, 0, 3)
	for i := range names {
		extra := math.Max(0, targets[i]-base)
		benefit := extra*well.OilRate*well.NetbackRubPerM3 - caps[i] - opex[i]*extra/365
		availability := scenarioAvailabilityReady
		switch i {
		case 0:
			if well.Ordinal%3 == 0 {
				availability = scenarioAvailabilityPartial
			}
		case 1:
			if well.Ordinal%7 == 0 {
				availability = scenarioAvailabilityMissing
			}
		case 2:
			if well.Ordinal%4 == 0 {
				availability = scenarioAvailabilityPartial
			}
		}
		note := "Условная учебная доступность; фактический ресурс всё равно проверяется перед утверждением."
		if availability == scenarioAvailabilityPartial {
			note = "В учебном сценарии доступно частично; срок и комплектность требуют проверки."
		} else if availability == scenarioAvailabilityMissing {
			note = "В учебном сценарии недоступно до подтверждения оборудования и подрядчика."
		}
		out = append(out, teachingAlternativeTemplate{
			ID:   fmt.Sprintf("TRN-%02d-%s", well.Ordinal, []string{"A", "B", "C"}[i]),
			Name: names[i], Methods: []string{methods[i]}, Availability: availability, AvailabilityNote: note,
			ForecastLow: targets[i] - 20, ForecastBase: targets[i], ForecastHigh: targets[i] + 25,
			AnnualOPEXRub: opex[i], OneTimeCAPEXRub: caps[i], NetBenefitK: benefit / 1000,
			BenefitCostRatio: benefit / math.Max(1, caps[i]), PaybackMonths: math.Max(0, caps[i]/math.Max(1, benefit)*12),
			SelectionKind: names[i], SelectionBasis: "Сценарий связан с последним отказом и лабораторным паспортом " + e.LabPassID,
		})
	}
	selected := -1
	if out[1].Availability == scenarioAvailabilityReady {
		selected = 1
	} else {
		for i := range out {
			if out[i].Availability == scenarioAvailabilityReady {
				selected = i
				break
			}
		}
	}
	if selected >= 0 {
		out[selected].Selected = true
	}
	return out, true
}

func teachingAlternativeRecord(objectKey string, ordinal int, t teachingAlternativeTemplate) AlternativeMethodScenario {
	status := "Рассчитано"
	approvedAt := ""
	if t.Selected {
		status = "Утверждено"
		approvedAt = "06.10.2026 10:30"
	}
	result := AlternativeMethodScenario{
		SchemaVersion: alternativeScenarioSchemaVersion, ID: t.ID, ObjectKey: strings.TrimSpace(objectKey), Name: t.Name,
		CreatedAt: fmt.Sprintf("2026-10-06T%02d:00:00Z", 6+ordinal%5), UpdatedAt: "2026-10-06T07:30:00Z",
		SourceSignature: teachingEconomicsSourceSignature816(objectKey), Classification: teachingScenarioClassification(objectKey),
		BaselineMRPDays: teachingScenarioBaseline(objectKey), TargetMRPDays: 1000,
		Methods: append([]string(nil), t.Methods...), MethodsInitialized: true,
		Availability: t.Availability, AvailabilityNote: t.AvailabilityNote, Status: status, ApprovedAt: approvedAt,
		ForecastLowDays: t.ForecastLow, ForecastBaseDays: t.ForecastBase, ForecastHighDays: t.ForecastHigh,
		// Only a result from the same economics formula as the production
		// workflow may be labelled as calculated.
		EconomicsStatus: "Не рассчитано", EconomicsCalculated: "",
		AnnualOPEXRub: t.AnnualOPEXRub, OneTimeCAPEXRub: t.OneTimeCAPEXRub,
		NetBenefitK: 0, BenefitCostRatio: 0, PaybackMonths: nil,
		SelectionKind: t.SelectionKind, SelectionBasis: t.SelectionBasis,
	}
	if calculated, ok := teachingLatestEconomics816(objectKey, result.ID); ok {
		// Keep the displayed cost inputs identical to those used by the
		// 36-month reproducible calculation.
		if history := teachingEconomicsHistory816(objectKey, result.ID); len(history) > 0 {
			last := history[len(history)-1]
			result.AnnualOPEXRub = last.OPEXRub
			result.OneTimeCAPEXRub = last.CAPEXRub
		}
		result.EconomicsStatus = "Расчёт актуален"
		result.EconomicsCalculated = "2026-10-08T09:00:00Z"
		result.NetBenefitK = calculated.Selected.NetBenefitForHorizonK
		result.BenefitCostRatio = calculated.Selected.BenefitCostRatio
		result.PaybackMonths = calculated.PaybackMonths
		result.SelectionBasis += " · Учебный расчёт: " + calculated.FormulaVersion + ", горизонт 36 мес.; данные синтетические"
	}
	return result
}

func teachingScenarioClassification(objectKey string) string {
	scenario, ok := teachingFullScenarioForObject(objectKey)
	if !ok {
		return "УЧЕБНЫЙ ПРИМЕР"
	}
	return "УТВЕРЖДЕНО: " + scenario.Name
}

func teachingScenarioBaseline(objectKey string) float64 {
	state, ok := teachingWorkflowStateForObject(objectKey)
	if !ok {
		return 850
	}
	value, err := parseEconomicNumber(state.CurrentMRP, "МРП", false)
	if err != nil || value <= 0 {
		return 850
	}
	return value
}

// ensureTeachingAlternativeScenarios migrates an old singleton teaching draft
// from 699-RC and fills any missing fixture rows without touching user-created
// alternatives. It deliberately does not call alternativeScenarioIndices to
// avoid recursion.
func ensureTeachingAlternativeScenarios(objectKey string) {
	// Once the cross-fund fixture has been seeded, never silently recreate
	// teaching rows intentionally removed or edited in subsequent sessions.
	if _, err := os.Stat(sharedPath(trainingEconomicsSeed816Name)); err == nil {
		return
	}
	templates, ok := teachingAlternativeTemplates(objectKey)
	if !ok || len(templates) == 0 {
		return
	}
	identity := alternativeObjectIdentity(objectKey)
	existing := make([]int, 0, len(templates))
	byID := map[string]int{}
	for i := range alternativeScenarios {
		if alternativeObjectIdentity(alternativeScenarios[i].ObjectKey) != identity {
			continue
		}
		existing = append(existing, i)
		byID[strings.ToUpper(strings.TrimSpace(alternativeScenarios[i].ID))] = i
	}
	changed := false
	ordinal, _ := teachingWorkflowOrdinal(objectKey)
	if len(existing) == 1 {
		old := alternativeScenarios[existing[0]]
		if strings.EqualFold(old.ID, "SCHEME-A") && len(old.Methods) == 0 && old.Status == "Черновик" {
			alternativeScenarios[existing[0]] = teachingAlternativeRecord(objectKey, ordinal, templates[0])
			byID[strings.ToUpper(templates[0].ID)] = existing[0]
			changed = true
		}
	}
	selectedID := ""
	for _, template := range templates {
		key := strings.ToUpper(template.ID)
		if _, exists := byID[key]; !exists {
			alternativeScenarios = append(alternativeScenarios, teachingAlternativeRecord(objectKey, ordinal, template))
			byID[key] = len(alternativeScenarios) - 1
			changed = true
		}
		if template.Selected {
			selectedID = template.ID
		}
	}
	if current := strings.TrimSpace(alternativeScenarioActive[identity]); current == "" || strings.EqualFold(current, "SCHEME-A") {
		alternativeScenarioActive[identity] = selectedID
		changed = true
	}
	if changed {
		_ = saveAlternativeScenarios()
	}
}

func selectedTeachingAlternative(objectKey string) (AlternativeMethodScenario, bool) {
	templates, ok := teachingAlternativeTemplates(objectKey)
	if !ok {
		return AlternativeMethodScenario{}, false
	}
	for _, template := range templates {
		if !template.Selected {
			continue
		}
		return teachingAlternativeRecord(objectKey, func() int { n, _ := teachingWorkflowOrdinal(objectKey); return n }(), template), true
	}
	return AlternativeMethodScenario{}, false
}

func teachingObjectKeyForOrdinal(ordinal int) string {
	if well, ok := trainingWellByOrdinal(ordinal); ok {
		return well.ObjectKey
	}
	for _, pass := range bundledDemoPassSnapshot() {
		if n, ok := teachingWorkflowOrdinal(pass.ObjectKey); ok && n == ordinal {
			return pass.ObjectKey
		}
	}
	return ""
}

func teachingReportSnapshot(ordinal int, objectKey string, selected teachingAlternativeTemplate) string {
	var b strings.Builder
	fmt.Fprintf(&b, "РОСА — ПОЛНЫЙ УЧЕБНЫЙ ОТЧЁТ %03d\r\n", ordinal)
	b.WriteString("Синтетические данные предназначены только для проверки сквозного маршрута программы.\r\n\r\n")
	objectLabel := strings.TrimSpace(objectKey)
	if well, ok := trainingWellForObject(objectKey); ok {
		objectLabel = fmt.Sprintf("%s / скв. %s / %s", well.Field, well.Well, well.Layer)
	}
	fmt.Fprintf(&b, "Объект: %s\r\nВыбрана: %s · %s\r\nМетоды: %s\r\nДоступность: %s — %s\r\n", objectLabel, selected.Name, selected.SelectionKind, strings.Join(selected.Methods, " + "), selected.Availability, selected.AvailabilityNote)
	fmt.Fprintf(&b, "Прогноз наработки: %.0f / %.0f / %.0f сут.; целевой ориентир 1000 сут.\r\n", selected.ForecastLow, selected.ForecastBase, selected.ForecastHigh)
	fmt.Fprintf(&b, "CAPEX: %.0f руб.; OPEX: %.0f руб./год; чистый эффект: %+.1f млн руб.\r\n", selected.OneTimeCAPEXRub, selected.AnnualOPEXRub, selected.NetBenefitK/1000)
	fmt.Fprintf(&b, "Основание решения: %s\r\n\r\n", selected.SelectionBasis)
	if all, ok := teachingAlternativeTemplates(objectKey); ok {
		b.WriteString("ПАРАЛЛЕЛЬНЫЕ ВАРИАНТЫ\r\n")
		for _, item := range all {
			fmt.Fprintf(&b, "%s | %s | %.0f сут. | CAPEX %.2f млн | OPEX %.2f млн/год | %s\r\n", item.Name, item.Availability, item.ForecastBase, item.OneTimeCAPEXRub/1000000, item.AnnualOPEXRub/1000000, item.SelectionKind)
		}
	}
	return b.String()
}

func teachingFullReportSectionsNeedRefresh(sections map[string]string) bool {
	if len(sections) < 7 {
		return true
	}
	// RC734 content gate: a teaching report is considered migrated only when the
	// heavy sections are actually detailed, not merely present as short summaries.
	return len([]rune(sections["section_2"])) < 3500 ||
		len([]rune(sections["section_4"])) < 1800 ||
		len([]rune(sections["section_5"])) < 650 ||
		len([]rune(sections["section_6"])) < 900
}

func teachingScenarioForFullReportRecord(r FullReportRecord) (AlternativeMethodScenario, bool) {
	templates, ok := teachingAlternativeTemplates(r.ObjectKey)
	if !ok {
		return AlternativeMethodScenario{}, false
	}
	ordinal, _ := teachingWorkflowOrdinal(r.ObjectKey)
	for _, template := range templates {
		if strings.EqualFold(strings.TrimSpace(template.ID), strings.TrimSpace(r.ScenarioID)) ||
			strings.Contains(strings.ToLower(r.ScenarioName), strings.ToLower(template.Name)) {
			return teachingAlternativeRecord(r.ObjectKey, ordinal, template), true
		}
	}
	return selectedTeachingAlternative(r.ObjectKey)
}

// ensureTeachingFullReports makes the lower archive traceable for every current
// training well. Two revisions per object give one approved record per well;
// real saved reports stay in the same cumulative table and are never overwritten.
func ensureTeachingFullReports() {
	existing := map[string]bool{}
	changed := false
	active := fullReports[:0]
	for _, record := range fullReports {
		// Retire report fixtures from the previous teaching corpus. Real reports
		// remain untouched, and current 20-well records keep their IDs/history.
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(record.ID)), "УЧ-") {
			if well, ok := trainingWellForObject(record.ObjectKey); !ok || !laboratoryObjectKeysEquivalent(well.ObjectKey, record.ObjectKey) {
				changed = true
				continue
			}
		}
		active = append(active, record)
	}
	fullReports = active
	for i := range fullReports {
		record := fullReports[i]
		existing[record.ID] = true
		// RC709 migration: the second teaching pass was the explicitly approved
		// example in older builds. Mark it once so section 8 starts with useful
		// examples while ordinary saved/draft passes stay out of the registry.
		if strings.HasPrefix(record.ID, "УЧ-") && strings.HasSuffix(record.ID, "-v2") &&
			strings.EqualFold(strings.TrimSpace(record.DecisionStatus), "Утверждено") && strings.TrimSpace(record.ApprovedAt) == "" {
			fullReports[i].ApprovedAt = record.CreatedAt
			changed = true
		}
		// RC735 stability: do NOT rebuild detailed section snapshots while a page is
		// opening. RC734 performed this migration synchronously from the UI thread,
		// which could freeze sections 7/8 for several seconds. Teaching rows are
		// enriched lazily only when the user explicitly opens or exports one report.
	}
	for ordinal := 1; ordinal <= 20; ordinal++ {
		objectKey := teachingObjectKeyForOrdinal(ordinal)
		templates, ok := teachingAlternativeTemplates(objectKey)
		if !ok || len(templates) == 0 {
			continue
		}
		selectedIndex := 0
		for i, item := range templates {
			if item.Selected {
				selectedIndex = i
				break
			}
		}
		for version := 1; version <= 2; version++ {
			id := fmt.Sprintf("УЧ-%03d-v%d", ordinal, version)
			if existing[id] {
				continue
			}
			choice := templates[0]
			status := "Архивный"
			if version == 2 {
				choice = templates[selectedIndex]
				status = "Утверждено"
			}
			label := fmt.Sprintf("Учебный №%03d · %s", ordinal, teachingScenarioClassification(objectKey))
			// Detailed Sections are intentionally deferred (RC735): creating the archive
			// index must stay cheap and must never block page activation.
			snapshot := teachingReportSnapshot(ordinal, objectKey, choice)
			created := fmt.Sprintf("%02d.10.2026 %02d:%02d", 2+(ordinal-1)%5, 9+(ordinal*3+version)%12, (ordinal*7+version*13)%60)
			fullReports = append(fullReports, FullReportRecord{
				ID: id, CreatedAt: created, ObjectKey: objectKey, ObjectLabel: label,
				ScenarioID: choice.ID, ScenarioName: choice.Name + " · " + choice.SelectionKind,
				Methods: strings.Join(choice.Methods, " + "), Availability: choice.Availability,
				ForecastDays: choice.ForecastBase, NetBenefitK: choice.NetBenefitK,
				DecisionStatus: status, ApprovedAt: func() string {
					if version == 2 {
						return created
					}
					return ""
				}(), DecisionBasis: choice.SelectionBasis,
				Author: "Технолог · учебный контур", Snapshot: snapshot,
				SnapshotVersion: "",
				Sections:        nil,
			})
			existing[id] = true
			changed = true
		}
	}
	if changed {
		_ = saveJSON(fullReportsPath(), fullReports)
	}
}
