//go:build windows

package main

import "time"

// A refresh-local cache. Never retained between user actions, data changes,
// object changes or sessions. All callers run on the native UI thread.
type methodsEvaluationKey struct {
	Object     string
	Suggestion methodWorkflowSuggestion
}
type methodsEvaluationScope struct {
	assessments map[methodsEvaluationKey]MethodEffectAssessment
	suggestions map[string][]methodWorkflowSuggestion
	histories   map[string]MethodHistoryBasis
	rows        map[string][]FailureEvent
	snapshots   map[string]methodForecastSnapshot
}

var methodsEvaluation *methodsEvaluationScope

func beginMethodsEvaluation() func() {
	if methodsEvaluation != nil {
		return func() {}
	}
	if methodsPreparedScope != nil && methodsPreparedKey == selectedObjectKey {
		methodsEvaluation = methodsPreparedScope
	} else {
		methodsEvaluation = newMethodsEvaluationScope()
	}
	return func() { methodsEvaluation = nil }
}

func newMethodsEvaluationScope() *methodsEvaluationScope {
	return &methodsEvaluationScope{
		assessments: map[methodsEvaluationKey]MethodEffectAssessment{},
		suggestions: map[string][]methodWorkflowSuggestion{},
		histories:   map[string]MethodHistoryBasis{}, rows: map[string][]FailureEvent{},
		snapshots: map[string]methodForecastSnapshot{},
	}
}

func methodApplicabilityForSuggestion(s methodWorkflowSuggestion, objectKey string, now time.Time) MethodEffectAssessment {
	key := methodsEvaluationKey{objectKey, s}
	if methodsEvaluation != nil {
		if a, ok := methodsEvaluation.assessments[key]; ok {
			return a
		}
	}
	a := calculateMethodApplicability(s, objectKey, now)
	if methodsEvaluation != nil {
		methodsEvaluation.assessments[key] = a
	}
	return a
}

func methodHistoryBasisForObject(objectKey string, now time.Time) MethodHistoryBasis {
	if methodsEvaluation != nil {
		if h, ok := methodsEvaluation.histories[objectKey]; ok {
			return h
		}
	}
	h := calculateMethodHistoryBasis(objectKey, now)
	if methodsEvaluation != nil {
		methodsEvaluation.histories[objectKey] = h
	}
	return h
}

func historyForWell(objectKey string, now time.Time) []FailureEvent {
	if methodsEvaluation != nil {
		if rows, ok := methodsEvaluation.rows[objectKey]; ok {
			return rows
		}
	}
	rows := collectHistoryForWell(objectKey, now)
	if methodsEvaluation != nil {
		methodsEvaluation.rows[objectKey] = rows
	}
	return rows
}

func methodForecastSnapshotForSelectedObject() methodForecastSnapshot {
	if methodsEvaluation != nil {
		if snap, ok := methodsEvaluation.snapshots[selectedObjectKey]; ok {
			return snap
		}
	}
	snap := calculateMethodForecastSnapshot()
	if methodsEvaluation != nil {
		methodsEvaluation.snapshots[selectedObjectKey] = snap
	}
	return snap
}
