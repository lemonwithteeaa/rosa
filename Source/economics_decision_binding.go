//go:build windows

package main

import (
	"fmt"
	"strings"
	"time"
)

const (
	economicsDecisionStatusNeedsRecalc = "требует пересчёта"
	economicsDecisionStatusCurrent     = "расчёт актуален"
)

// EconomicsDecisionLink binds an economic calculation to one immutable
// approved technology-decision version.  A new approval/restoration never
// silently reuses an older economic result.
type EconomicsDecisionLink struct {
	ObjectKey       string `json:"object_key"`
	DecisionID      string `json:"decision_id,omitempty"`
	DecisionVersion int    `json:"decision_version,omitempty"`
	DecisionAt      string `json:"decision_at,omitempty"`
	ScenarioID      string `json:"scenario_id,omitempty"`
	Status          string `json:"status"`
	Reason          string `json:"reason,omitempty"`
	UpdatedAt       string `json:"updated_at"`
	CalculatedAt    string `json:"calculated_at,omitempty"`
}

var (
	economicsDecisionLinksLoaded bool
	economicsDecisionLinks       []EconomicsDecisionLink
)

func economicsDecisionLinksPath() string { return sharedPath("economics_decision_links.json") }

func ensureEconomicsDecisionLinksLoaded() {
	if economicsDecisionLinksLoaded {
		return
	}
	economicsDecisionLinksLoaded = true
	economicsDecisionLinks = nil
	loadJSON(economicsDecisionLinksPath(), &economicsDecisionLinks)
}

func economicsDecisionLinkIndex(objectKey string) int {
	ensureEconomicsDecisionLinksLoaded()
	key := strings.TrimSpace(objectKey)
	for i := len(economicsDecisionLinks) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(economicsDecisionLinks[i].ObjectKey), key) {
			return i
		}
	}
	return -1
}

func saveEconomicsDecisionLink(link EconomicsDecisionLink) error {
	ensureEconomicsDecisionLinksLoaded()
	backup := append([]EconomicsDecisionLink(nil), economicsDecisionLinks...)
	idx := economicsDecisionLinkIndex(link.ObjectKey)
	if idx >= 0 {
		economicsDecisionLinks[idx] = link
	} else {
		economicsDecisionLinks = append(economicsDecisionLinks, link)
	}
	if err := saveJSON(economicsDecisionLinksPath(), economicsDecisionLinks); err != nil {
		economicsDecisionLinks = backup
		return err
	}
	return nil
}

func markEconomicsNeedsRecalcForDecision(d approvedMethodDecision, reason string) error {
	if strings.TrimSpace(d.ObjectKey) == "" {
		return nil
	}
	return saveEconomicsDecisionLink(EconomicsDecisionLink{
		ObjectKey: strings.TrimSpace(d.ObjectKey), DecisionID: d.ID,
		DecisionVersion: methodDecisionStoredVersion(d), DecisionAt: d.ApprovedAt,
		Status: economicsDecisionStatusNeedsRecalc, Reason: strings.TrimSpace(reason),
		UpdatedAt: time.Now().Format("02.01.2006 15:04"),
	})
}

func markEconomicsNeedsRecalc(objectKey, reason string) error {
	d, ok := latestApprovedMethodDecision(objectKey)
	if !ok {
		return nil
	}
	return markEconomicsNeedsRecalcForDecision(d, reason)
}

func markEconomicsCalculatedForCurrentDecision(objectKey, scenarioID string) error {
	d, ok := latestApprovedMethodDecision(objectKey)
	if !ok {
		return nil
	}
	now := time.Now().Format("02.01.2006 15:04")
	return saveEconomicsDecisionLink(EconomicsDecisionLink{
		ObjectKey: strings.TrimSpace(objectKey), DecisionID: d.ID,
		DecisionVersion: methodDecisionStoredVersion(d), DecisionAt: d.ApprovedAt,
		ScenarioID: strings.TrimSpace(scenarioID), Status: economicsDecisionStatusCurrent,
		UpdatedAt: now, CalculatedAt: now,
	})
}

// economicsDecisionLinkForObject is intentionally defensive: even if a legacy
// file has no link row, a newer approved decision is treated as stale rather
// than as a reusable economic result.
func economicsDecisionLinkForObject(objectKey string) (EconomicsDecisionLink, bool) {
	d, ok := latestApprovedMethodDecision(objectKey)
	if !ok {
		return EconomicsDecisionLink{}, false
	}
	ensureEconomicsDecisionLinksLoaded()
	idx := economicsDecisionLinkIndex(objectKey)
	if idx < 0 {
		return EconomicsDecisionLink{
			ObjectKey: d.ObjectKey, DecisionID: d.ID,
			DecisionVersion: methodDecisionStoredVersion(d), DecisionAt: d.ApprovedAt,
			Status: economicsDecisionStatusNeedsRecalc,
			Reason: "утверждённое решение ещё не связано с экономическим расчётом",
		}, true
	}
	link := economicsDecisionLinks[idx]
	if strings.TrimSpace(link.DecisionID) != strings.TrimSpace(d.ID) {
		return EconomicsDecisionLink{
			ObjectKey: d.ObjectKey, DecisionID: d.ID,
			DecisionVersion: methodDecisionStoredVersion(d), DecisionAt: d.ApprovedAt,
			Status:    economicsDecisionStatusNeedsRecalc,
			Reason:    "изменена версия утверждённого технологического решения",
			UpdatedAt: link.UpdatedAt,
		}, true
	}
	if strings.TrimSpace(link.Status) == "" {
		link.Status = economicsDecisionStatusNeedsRecalc
	}
	return link, true
}

func economicsDecisionStatusText(objectKey string) string {
	link, ok := economicsDecisionLinkForObject(objectKey)
	if !ok {
		return "Связь с решением: утверждённое решение отсутствует"
	}
	status := strings.ToUpper(strings.TrimSpace(link.Status))
	if status == "" {
		status = strings.ToUpper(economicsDecisionStatusNeedsRecalc)
	}
	text := fmt.Sprintf("Связь с решением: v%d · %s", link.DecisionVersion, status)
	if strings.TrimSpace(link.Reason) != "" && link.Status != economicsDecisionStatusCurrent {
		text += " · " + strings.TrimSpace(link.Reason)
	}
	return text
}

func economicsApprovedImplementationText(d approvedMethodDecision) string {
	if len(d.Implementation) == 0 {
		return "Параметры реализации: в архивной версии отсутствуют; проверьте текущие параметры раздела 4."
	}
	var lines []string
	for _, record := range d.Implementation {
		fields := make([]string, 0, len(record.Fields))
		for _, f := range record.Fields {
			value := strings.TrimSpace(f.Value)
			if value == "" {
				value = strings.TrimSpace(f.Suggested)
			}
			if value != "" {
				fields = append(fields, f.Label+": "+value)
			}
		}
		line := strings.TrimSpace(record.Method)
		if line == "" {
			line = "Метод"
		}
		if len(fields) > 0 {
			line += " — " + strings.Join(fields, "; ")
		}
		lines = append(lines, line)
	}
	return "Параметры реализации из утверждённой версии:\r\n• " + strings.Join(lines, "\r\n• ")
}

func economicsApprovedDecisionText(d approvedMethodDecision) string {
	methods := normalizeMethodList(d.Methods)
	if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
		methods = []string{strings.TrimSpace(d.Method)}
	}
	return fmt.Sprintf(
		"Утверждённое решение v%d от %s\r\nМетод / комплекс: %s\r\n%s\r\nИсточник сценария: архив решений раздела 4; внешняя цена нефти в базовом режиме не используется.",
		methodDecisionStoredVersion(d), dash(d.ApprovedAt), dash(strings.Join(methods, " + ")), economicsApprovedImplementationText(d),
	)
}

func economicsDeferralMonths(currentMRP, scenarioMRP float64) float64 {
	if currentMRP <= 0 || scenarioMRP <= currentMRP {
		return 0
	}
	return (scenarioMRP - currentMRP) / 30.4375
}

func approvedDecisionContainsMethod(d approvedMethodDecision, method string) bool {
	n := normalizeMethodText(method)
	if n == "" {
		return false
	}
	methods := normalizeMethodList(d.Methods)
	if len(methods) == 0 && strings.TrimSpace(d.Method) != "" {
		methods = []string{d.Method}
	}
	for _, m := range methods {
		if normalizeMethodText(m) == n {
			return true
		}
	}
	return false
}
