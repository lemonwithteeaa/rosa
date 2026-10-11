//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

// learnedMethodSupport is deliberately advisory. Accumulated ROSA history may
// promote a method into the candidate list, but it never selects or approves it
// for the technologist.
type learnedMethodSupport struct {
	Method         string
	Direction      string
	Classification string
	Decisions      int
	Objects        int
	ObjectKeys     []string
}

type learnedSchemeSupport struct {
	Kind       string
	Summary    string
	Uses       int
	Objects    int
	ObjectKeys []string
}

type learnedComboSupport struct {
	Methods    []string
	Decisions  int
	Objects    int
	ObjectKeys []string
}

func approvedDecisionMethodNames(d approvedMethodDecision) []string {
	names := normalizeMethodList(d.Methods)
	if len(names) > 0 {
		return names
	}
	raw := strings.TrimSpace(d.Method)
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, " + ") {
		return normalizeMethodList(strings.Split(raw, " + "))
	}
	return []string{raw}
}

func learnedRelevantCurrentRow(objectKey, classification string) (AssessmentClassRecord, bool) {
	_, ready, rows := approvedAssessmentForMethods(objectKey)
	if !ready {
		return AssessmentClassRecord{}, false
	}
	classification = strings.ToLower(strings.TrimSpace(classification))
	for _, row := range rows {
		direction := strings.ToLower(strings.TrimSpace(row.Direction))
		class := strings.ToLower(strings.TrimSpace(row.FinalClass))
		if direction != "" && strings.Contains(classification, direction) {
			return row, true
		}
		if class != "" && strings.Contains(classification, class) {
			return row, true
		}
		if methodTextMatchesDirection(classification, row.Direction) {
			return row, true
		}
	}
	return AssessmentClassRecord{}, false
}

func learnedMethodSupportsForObject(objectKey string) []learnedMethodSupport {
	ensureApprovedMethodsLoaded()
	target := strings.TrimSpace(objectKey)
	type agg struct {
		learnedMethodSupport
		objects map[string]bool
	}
	byMethod := map[string]*agg{}
	for _, d := range approvedMethodDecisions {
		if strings.TrimSpace(d.ObjectKey) == "" || strings.EqualFold(strings.TrimSpace(d.ObjectKey), target) {
			continue
		}
		row, relevant := learnedRelevantCurrentRow(target, d.Classification)
		if !relevant {
			continue
		}
		for _, method := range approvedDecisionMethodNames(d) {
			key := normalizeMethodText(method)
			if key == "" || assessmentMethodIsEquipmentDesign(method) {
				continue
			}
			a := byMethod[key]
			if a == nil {
				a = &agg{learnedMethodSupport: learnedMethodSupport{Method: method, Direction: row.Direction, Classification: row.FinalClass}, objects: map[string]bool{}}
				byMethod[key] = a
			}
			a.Decisions++
			a.objects[strings.TrimSpace(d.ObjectKey)] = true
		}
	}
	out := make([]learnedMethodSupport, 0, len(byMethod))
	for _, a := range byMethod {
		for k := range a.objects {
			a.ObjectKeys = append(a.ObjectKeys, k)
		}
		sort.Strings(a.ObjectKeys)
		a.Objects = len(a.ObjectKeys)
		out = append(out, a.learnedMethodSupport)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Objects != out[j].Objects {
			return out[i].Objects > out[j].Objects
		}
		if out[i].Decisions != out[j].Decisions {
			return out[i].Decisions > out[j].Decisions
		}
		return strings.ToLower(out[i].Method) < strings.ToLower(out[j].Method)
	})
	return out
}

func learnedCatalogCandidate(method string) assessmentMethodCandidate {
	for _, item := range ruleCatalog.MethodCatalog {
		if normalizeMethodText(item.Name) != normalizeMethodText(method) {
			continue
		}
		source, sourceKind := assessmentMethodSource(item)
		if strings.TrimSpace(source) == "" {
			source = "Справочник РОСА"
		}
		return assessmentMethodCandidate{ID: item.ID, Name: item.Name, Source: source + "; накопленный опыт РОСА", SourceKind: sourceKind, Status: assessmentMethodStatus(item.Name), Note: "Метод поднят также по накопленным решениям на сопоставимых объектах; требуется проверка технологом."}
	}
	return assessmentMethodCandidate{Name: method, Source: "Накопленный опыт РОСА", SourceKind: "internal", Status: "Кандидат по аналогам", Note: "Ранее утверждался на сопоставимых объектах. Автоматически не выбирается и требует проверки технологом."}
}

func mergeLearnedMethodSuggestions(objectKey string, base []methodWorkflowSuggestion) []methodWorkflowSuggestion {
	out := append([]methodWorkflowSuggestion(nil), base...)
	seen := map[string]bool{}
	for _, s := range out {
		seen[normalizeMethodText(s.Method.Name)] = true
	}
	for _, support := range learnedMethodSupportsForObject(objectKey) {
		// One isolated historical decision is shown in the analog card but does
		// not modify the main candidate set. The candidate list learns only
		// after repetition on at least two distinct objects.
		if support.Objects < 2 || seen[normalizeMethodText(support.Method)] {
			continue
		}
		candidate := learnedCatalogCandidate(support.Method)
		basis := fmt.Sprintf("Накопленный опыт РОСА: метод ранее утверждался на %d сопоставимых объектах (%d решений); направление %s %s. Это подсказка по аналогам, а не автоматическое решение.", support.Objects, support.Decisions, support.Direction, support.Classification)
		out = append(out, methodWorkflowSuggestion{Method: candidate, Direction: support.Direction, Classification: support.Classification, Severity: methodClassSeverity(support.Classification), Basis: basis})
		seen[normalizeMethodText(support.Method)] = true
	}
	return out
}

func implementationSignature(r MethodImplementationRecord) string {
	parts := []string{strings.ToLower(strings.TrimSpace(r.Kind)), normalizeMethodText(r.Method)}
	fields := append([]methodImplementationField(nil), r.Fields...)
	sort.SliceStable(fields, func(i, j int) bool { return fields[i].Key < fields[j].Key })
	for _, f := range fields {
		v := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(f.Value)), " "))
		if v == "" || strings.EqualFold(v, "не выбрано") {
			continue
		}
		parts = append(parts, f.Key+"="+v)
	}
	return strings.Join(parts, "|")
}

func implementationCompactSummary(r MethodImplementationRecord) string {
	wanted := map[string]bool{"reagent": true, "basis": true, "unit": true, "rate": true, "point": true, "periodicity": true, "sequence": true, "control": true, "mode": true}
	parts := []string{}
	for _, f := range r.Fields {
		if !wanted[f.Key] {
			continue
		}
		v := strings.TrimSpace(f.Value)
		if v == "" || strings.EqualFold(v, "Не выбрано") {
			continue
		}
		parts = append(parts, f.Label+": "+v)
		if len(parts) >= 4 {
			break
		}
	}
	if len(parts) == 0 {
		return strings.TrimSpace(r.Kind)
	}
	return strings.Join(parts, "; ")
}

func learnedSchemesForMethod(objectKey, method string) []learnedSchemeSupport {
	ensureMethodImplementationRecordsLoaded()
	target := strings.TrimSpace(objectKey)
	type agg struct {
		learnedSchemeSupport
		objects map[string]bool
	}
	bySig := map[string]*agg{}
	for _, r := range methodImplementationRecords {
		if strings.TrimSpace(r.ObjectKey) == "" || strings.EqualFold(strings.TrimSpace(r.ObjectKey), target) || normalizeMethodText(r.Method) != normalizeMethodText(method) {
			continue
		}
		sig := implementationSignature(r)
		if sig == "" {
			continue
		}
		a := bySig[sig]
		if a == nil {
			a = &agg{learnedSchemeSupport: learnedSchemeSupport{Kind: r.Kind, Summary: implementationCompactSummary(r)}, objects: map[string]bool{}}
			bySig[sig] = a
		}
		a.Uses++
		a.objects[strings.TrimSpace(r.ObjectKey)] = true
	}
	out := []learnedSchemeSupport{}
	for _, a := range bySig {
		for k := range a.objects {
			a.ObjectKeys = append(a.ObjectKeys, k)
		}
		sort.Strings(a.ObjectKeys)
		a.Objects = len(a.ObjectKeys)
		out = append(out, a.learnedSchemeSupport)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Objects != out[j].Objects {
			return out[i].Objects > out[j].Objects
		}
		if out[i].Uses != out[j].Uses {
			return out[i].Uses > out[j].Uses
		}
		return out[i].Summary < out[j].Summary
	})
	return out
}

func learnedCombosForMethod(objectKey, method string) []learnedComboSupport {
	ensureApprovedMethodsLoaded()
	target := strings.TrimSpace(objectKey)
	type agg struct {
		learnedComboSupport
		objects map[string]bool
	}
	byCombo := map[string]*agg{}
	for _, d := range approvedMethodDecisions {
		if strings.TrimSpace(d.ObjectKey) == "" || strings.EqualFold(strings.TrimSpace(d.ObjectKey), target) {
			continue
		}
		if _, relevant := learnedRelevantCurrentRow(target, d.Classification); !relevant {
			continue
		}
		names := approvedDecisionMethodNames(d)
		has := false
		for _, name := range names {
			if normalizeMethodText(name) == normalizeMethodText(method) {
				has = true
				break
			}
		}
		if !has || len(names) < 2 {
			continue
		}
		sorted := append([]string(nil), names...)
		sort.SliceStable(sorted, func(i, j int) bool { return normalizeMethodText(sorted[i]) < normalizeMethodText(sorted[j]) })
		sigParts := make([]string, 0, len(sorted))
		for _, name := range sorted {
			sigParts = append(sigParts, normalizeMethodText(name))
		}
		sig := strings.Join(sigParts, "+")
		a := byCombo[sig]
		if a == nil {
			a = &agg{learnedComboSupport: learnedComboSupport{Methods: sorted}, objects: map[string]bool{}}
			byCombo[sig] = a
		}
		a.Decisions++
		a.objects[strings.TrimSpace(d.ObjectKey)] = true
	}
	out := []learnedComboSupport{}
	for _, a := range byCombo {
		for k := range a.objects {
			a.ObjectKeys = append(a.ObjectKeys, k)
		}
		sort.Strings(a.ObjectKeys)
		a.Objects = len(a.ObjectKeys)
		out = append(out, a.learnedComboSupport)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Objects != out[j].Objects {
			return out[i].Objects > out[j].Objects
		}
		if out[i].Decisions != out[j].Decisions {
			return out[i].Decisions > out[j].Decisions
		}
		return strings.Join(out[i].Methods, "+") < strings.Join(out[j].Methods, "+")
	})
	return out
}

func learnedExperienceText(objectKey, method string) string {
	var b strings.Builder
	b.WriteString("НАКОПЛЕННЫЙ ОПЫТ РОСА\r\n")
	supports := learnedMethodSupportsForObject(objectKey)
	found := false
	for _, s := range supports {
		if normalizeMethodText(s.Method) != normalizeMethodText(method) {
			continue
		}
		found = true
		fmt.Fprintf(&b, "Метод встречался на сопоставимых объектах: %d объект(а/ов), %d утверждённых решений.\r\n", s.Objects, s.Decisions)
		break
	}
	if !found {
		b.WriteString("По этому методу пока нет повторяющейся внутренней серии на сопоставимых объектах.\r\n")
	}

	combos := learnedCombosForMethod(objectKey, method)
	b.WriteString("\r\nВОЗМОЖНЫЕ СОЧЕТАНИЯ С ДРУГИМИ МЕТОДАМИ\r\n")
	if len(combos) == 0 {
		b.WriteString("Повторяющиеся связки пока не накоплены. Одиночный метод остаётся доступен для самостоятельного выбора технологом.\r\n")
	} else {
		limit := len(combos)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			c := combos[i]
			fmt.Fprintf(&b, "%d. %s — встречалось на %d объект(ах), %d решениях.\r\n", i+1, strings.Join(c.Methods, " + "), c.Objects, c.Decisions)
		}
	}

	schemes := learnedSchemesForMethod(objectKey, method)
	b.WriteString("\r\nВОЗМОЖНЫЕ СХЕМЫ ПРИМЕНЕНИЯ ИЗ ПРЕДЫДУЩИХ ДИАГНОСТИК\r\n")
	if len(schemes) == 0 {
		b.WriteString("Сохранённых объектовых схем по этому методу пока нет. Используйте параметры метода или справочник схем химизации.\r\n")
	} else {
		limit := len(schemes)
		if limit > 3 {
			limit = 3
		}
		for i := 0; i < limit; i++ {
			s := schemes[i]
			fmt.Fprintf(&b, "%d. %s — %s; использовано на %d объект(ах).\r\n", i+1, dash(s.Kind), dash(s.Summary), s.Objects)
		}
	}
	b.WriteString("\r\nРОСА использует накопленный опыт только как подсказку: схема/связка не применяется и не утверждается автоматически.")
	return b.String()
}
