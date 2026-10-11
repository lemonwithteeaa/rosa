// Package nnocore contains the isolated, fail-closed calculation kernel for
// the rolling 12-month fleet run life. The UI must not call it until verified
// Ti observations and an approved methodology snapshot are available.
package nnocore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Action string

const (
	Include Action = "include"
	Exclude Action = "exclude"
)

type Rule struct {
	ClassifierVersion string `json:"classifier_version"`
	Branch            string `json:"branch"`
	Code              string `json:"code"`
	Action            Action `json:"action"`
	Reason            string `json:"reason"`
}

type Methodology struct {
	ID           string    `json:"id"`
	Version      string    `json:"version"`
	ValidFrom    time.Time `json:"valid_from"`
	ValidTo      time.Time `json:"valid_to"`
	Approved     bool      `json:"approved"`
	WindowMonths int       `json:"window_months"`
	Rules        []Rule    `json:"rules"`
}

type Failure struct {
	ID                string
	WellID            string
	Group             string
	Month             time.Time
	ClassifierVersion string
	Branch            string
	Code              string
	Confirmed         bool
	WorkedDays        float64
	ProductionData    bool
}

type Decision struct {
	EventID    string
	Group      string
	Reason     string
	RuleKey    string
	Included   bool
	WorkedDays float64
}

type Result struct {
	Group              string
	Month              string
	MethodologyID      string
	MethodologyVersion string
	Days               float64
	Failures           int
	NNO                float64
	Defined            bool
	Decisions          []Decision
}

func key(version, branch, code string) string {
	return strings.TrimSpace(version) + "|" + strings.TrimSpace(branch) + "|" + strings.TrimSpace(code)
}

func validate(m Methodology) error {
	if m.ID == "" || m.Version == "" || m.WindowMonths != 12 || !m.Approved || m.ValidFrom.IsZero() {
		return errors.New("methodology is not an approved 12-month version")
	}
	if !m.ValidTo.IsZero() && !m.ValidTo.After(m.ValidFrom) {
		return errors.New("invalid effective range")
	}
	found := map[string]bool{}
	for _, rule := range m.Rules {
		k := key(rule.ClassifierVersion, rule.Branch, rule.Code)
		if rule.ClassifierVersion == "" || rule.Branch == "" || rule.Code == "" || (rule.Action != Include && rule.Action != Exclude) || found[k] {
			return fmt.Errorf("ambiguous or invalid rule: %s", k)
		}
		found[k] = true
	}
	return nil
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func inWindow(eventMonth, reportMonth time.Time) bool {
	first := monthStart(reportMonth).AddDate(0, -11, 0)
	return !eventMonth.Before(first) && !eventMonth.After(monthStart(reportMonth))
}

func effective(m Methodology, reportMonth time.Time) bool {
	report := monthStart(reportMonth)
	return !report.Before(monthStart(m.ValidFrom)) && (m.ValidTo.IsZero() || report.Before(monthStart(m.ValidTo)))
}

// Calculate uses H12 = sum(Ti) / sum(Oi), never the mean of monthly ratios.
// Unverified, non-production and unmatched observations are excluded with an
// auditable reason; duplicate IDs and negative worked time fail the run.
func Calculate(m Methodology, failures []Failure, reportMonth time.Time, group string) (Result, error) {
	if err := validate(m); err != nil {
		return Result{}, err
	}
	if !effective(m, reportMonth) {
		return Result{}, errors.New("version not effective in the report month")
	}
	if strings.TrimSpace(group) == "" {
		return Result{}, errors.New("group must be selected")
	}
	rules := make(map[string]Rule, len(m.Rules))
	for _, rule := range m.Rules {
		rules[key(rule.ClassifierVersion, rule.Branch, rule.Code)] = rule
	}
	result := Result{Group: group, Month: monthStart(reportMonth).Format("2006-01"), MethodologyID: m.ID, MethodologyVersion: m.Version}
	seen := map[string]bool{}
	for _, failure := range failures {
		if failure.Group != group || !inWindow(monthStart(failure.Month), reportMonth) {
			continue
		}
		if failure.ID == "" || seen[failure.ID] {
			return Result{}, fmt.Errorf("missing or duplicate event id: %q", failure.ID)
		}
		seen[failure.ID] = true
		decision := Decision{EventID: failure.ID, Group: failure.Group, RuleKey: key(failure.ClassifierVersion, failure.Branch, failure.Code)}
		switch {
		case !failure.ProductionData:
			decision.Reason = "учебная запись — не учитывается"
		case !failure.Confirmed:
			decision.Reason = "классификация не утверждена"
		case failure.WorkedDays < 0:
			return Result{}, fmt.Errorf("negative worked days: %s", failure.ID)
		default:
			rule, ok := rules[decision.RuleKey]
			switch {
			case !ok:
				decision.Reason = "нет утверждённого правила для кода"
			case rule.Action == Exclude:
				decision.Reason = "исключено методикой: " + rule.Reason
			default:
				decision.Included = true
				decision.WorkedDays = failure.WorkedDays
				decision.Reason = "включено: " + rule.Reason
				result.Days += failure.WorkedDays
				result.Failures++
			}
		}
		result.Decisions = append(result.Decisions, decision)
	}
	sort.Slice(result.Decisions, func(i, j int) bool { return result.Decisions[i].EventID < result.Decisions[j].EventID })
	if result.Failures > 0 {
		result.Defined = true
		result.NNO = result.Days / float64(result.Failures)
	}
	return result, nil
}
