//go:build windows

package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type methodProbabilityResult struct {
	Available                                    bool
	Value, Low, High, Sample, Successes, Objects int
}

func indexMethodOutcomes(rows []Outcome) map[string]Outcome {
	out := make(map[string]Outcome, len(rows))
	for _, row := range rows {
		if _, exists := out[row.VersionID]; !exists {
			out[row.VersionID] = row
		}
	}
	return out
}

type methodsBackgroundResult struct {
	token    uint64
	key      string
	previous string
	scope    *methodsEvaluationScope
	err      string
}

var methodsBackgroundGeneration atomic.Uint64
var methodsBackgroundMu sync.Mutex
var methodsBackgroundReady *methodsBackgroundResult
var methodsPreparedScope *methodsEvaluationScope
var methodsPreparedKey string
var methodsPreparedShowAll bool
var methodsBackgroundBusy bool
var methodsBackgroundKey string
var methodsBackgroundShowAll bool
var methodsResumeSelectionName string

func invalidateMethodsBackground() {
	methodsBackgroundGeneration.Add(1)
	methodsPreparedScope, methodsPreparedKey = nil, ""
	methodsPreparedShowAll = false
	methodsBackgroundBusy = false
	methodsBackgroundKey = ""
	methodsBackgroundMu.Lock()
	methodsBackgroundReady = nil
	methodsBackgroundMu.Unlock()
}

// All native calls and mutable application reads are on the UI thread. The
// worker receives value snapshots and never uses refresh-local/global caches.
func refreshMethodsWorkflowUI() {
	uiUpdate := beginMethodsUIUpdate()
	defer endMethodsUIUpdate(uiUpdate)
	key := strings.TrimSpace(selectedObjectKey)
	if methodsShowAllObject != key {
		methodsShowAll = false
		methodsShowAllObject = key
	}
	showAll := methodsShowAll
	if key == "" {
		invalidateMethodsBackground()
		renderMethodsWorkflowUI()
		return
	}
	if methodsPreparedScope != nil && methodsPreparedKey == key && methodsPreparedShowAll == showAll {
		// Reuse the completed result when returning to this page or changing a
		// scenario. Data/classification mutations invalidate it explicitly.
		renderMethodsWorkflowUI()
		return
	}
	if methodsBackgroundBusy && methodsBackgroundKey == key && methodsBackgroundShowAll == showAll {
		// A navigation round trip must not cancel a calculation already in flight.
		// It must still reconstruct the visible placeholder and the independent
		// cumulative history: previously this early return could leave the page
		// partially blank until the worker happened to finish.
		showMethodsPendingState(key)
		return
	}
	invalidateMethodsBackground()
	methodsBackgroundKey, methodsBackgroundShowAll = key, showAll
	previousMethod := ""
	if previous, ok := methodsSelectedSuggestion(); ok {
		previousMethod = previous.Method.Name
	}
	token := methodsBackgroundGeneration.Load()
	started := time.Now()
	finish := beginMethodsEvaluation()
	candidates := collectMethodSuggestionsForObject(key)
	if showAll {
		candidates = collectAllMethodSuggestionsForObject(key)
	}
	rows := append([]FailureEvent(nil), historyForWell(key, started)...)
	history := methodHistoryBasisForObject(key, started)
	current := 0.0
	if st, ok := currentSelectedState(); ok {
		current = toFloat(st.CurrentMRP)
	}
	observations := append([]MRPObservation(nil), mrpObservationHistoryForObject(key)...)
	finish()
	records := append([]VersionRecord(nil), versions...)
	for i := range records {
		records[i].Raw = nil
	} // worker does not use payloads
	outcomeIndex := indexMethodOutcomes(outcomes)
	lift := methodObjectEquipment(key)
	methodWorkflowSuggestions = nil
	methodsBackgroundBusy = true
	showMethodsPendingState(key)
	diagLog("METHODS snapshot key=%s candidates=%d versions=%d elapsed=%s", key, len(candidates), len(records), time.Since(started))
	mainWindow := hMain
	go func() {
		result := &methodsBackgroundResult{token: token, key: key, previous: previousMethod}
		defer func() {
			if failure := recover(); failure != nil {
				result.err = fmt.Sprint(failure)
			}
			if methodsBackgroundGeneration.Load() != token {
				return
			}
			methodsBackgroundMu.Lock()
			if methodsBackgroundGeneration.Load() == token {
				methodsBackgroundReady = result
			}
			methodsBackgroundMu.Unlock()
			pPostMessageW.Call(uintptr(mainWindow), WM_APP_METHODS_DONE, uintptr(token), 0)
		}()
		snapshot := methodsBaselineFromSnapshot(history, current, observations, started)
		scope := newMethodsEvaluationScope()
		scope.rows[key], scope.histories[key], scope.snapshots[key] = rows, history, snapshot
		cohort := prepareMethodProbabilityCohort(records, outcomeIndex)
		scores := make(map[string]MethodEffectAssessment, len(candidates))
		for _, candidate := range candidates {
			if methodsBackgroundGeneration.Load() != token {
				return
			}
			if methodSuggestionOutsideCurrentProfile(candidate) {
				grade := methodOutsideProfileAssessment(candidate)
				scope.assessments[methodsEvaluationKey{key, candidate}] = grade
				scores[candidate.Method.Name] = grade
				continue
			}
			p := methodProbabilityResult{}
			p.Available, p.Value, p.Low, p.High, p.Sample, p.Successes, p.Objects = empiricalMethodProbabilityFromCohort(candidate.Method.Name, candidate.Direction, lift, cohort)
			grade := calculateMethodApplicabilityFromSnapshot(candidate, key, history, rows, p)
			scope.assessments[methodsEvaluationKey{key, candidate}] = grade
			scores[candidate.Method.Name] = grade
		}
		if showAll {
			scope.suggestions[key] = rankMethodSuggestionsWithLimit(candidates, scores, 0)
		} else {
			scope.suggestions[key] = rankMethodSuggestions(candidates, scores)
		}
		result.scope = scope
	}()
}

func showMethodsPendingState(key string) {
	// Always write a self-consistent interim state. Old suggestion controls must
	// not overlap the calculation indicator after switching from another object.
	methodsClassificationReady = false
	methodWorkflowSuggestions = nil
	setText(methodsObjectCode, key)
	setText(methodsClassSummary, "Классификация подготовлена. Рассчитываются методы…")
	setText(methodsReadyBanner, "Расчёт методов в фоне. Можно переходить в другие разделы.")
	setText(methodsAvailableTitle, "01 · РАСЧЁТ МЕТОДОВ")
	setText(methodsEmptyState, "Сопоставляем методы с утверждёнными классами и историей объекта…")
	setText(methodsSuggestionInfo, "Результат появится после расчёта.")
	setText(methodsImplementation, "Выберите метод после завершения расчёта.")
	setText(methodsForecastNote, "Расчёт текущего комплекса…")
	pEnableWindow.Call(uintptr(methodsApproveButton), 0)
	pEnableWindow.Call(uintptr(methodsPlanButton), 0)
	for _, card := range methodsCandidateCards {
		if card != 0 {
			pEnableWindow.Call(uintptr(card), 0)
		}
	}
	refreshMethodsArchive()
	relayoutMethodsWorkflowCurrent()
}

func completeMethodsEvaluation(token uint64) {
	methodsBackgroundMu.Lock()
	result := methodsBackgroundReady
	if result != nil && result.token == token {
		methodsBackgroundReady = nil
	}
	methodsBackgroundMu.Unlock()
	if !methodsBackgroundResultIsCurrent(result, token, methodsBackgroundGeneration.Load(), selectedObjectKey) {
		return
	}
	methodsBackgroundBusy = false
	methodsBackgroundKey = ""
	if result.err != "" {
		showMethodsSafeFallback(result.err)
		return
	}
	methodsPreparedScope, methodsPreparedKey = result.scope, result.key
	methodsPreparedShowAll = methodsBackgroundShowAll
	methodsResumeSelectionName = result.previous
	// Hidden pages are never re-shown by an obsolete completion.
	if currentPage != ID_ACTIVITIES {
		return
	}
	renderMethodsWorkflowUIAtomic()
	for _, card := range methodsCandidateCards {
		if card != 0 {
			pEnableWindow.Call(uintptr(card), boolToUintptr(methodsClassificationReady))
		}
	}
	pEnableWindow.Call(uintptr(methodsApproveButton), boolToUintptr(methodsClassificationReady))
	pEnableWindow.Call(uintptr(methodsPlanButton), boolToUintptr(methodsClassificationReady))
	pInvalidateRect.Call(uintptr(hMain), 0, 0)
}

// Methods needs an object baseline, not the fleet survival-model training
// performed on the dedicated forecast page. Keep the same 36-month boundary.
func methodsBaselineFromSnapshot(h MethodHistoryBasis, current float64, observations []MRPObservation, now time.Time) methodForecastSnapshot {
	if h.BaseDays > 0 {
		return methodForecastSnapshot{Low: h.LowDays, Base: h.BaseDays, High: h.HighDays, History: h, Source: "предыстория отказов раздела 1 / последние 36 месяцев"}
	}
	values := []float64{}
	cutoff := now.AddDate(-3, 0, 0)
	for _, o := range observations {
		if o.MRPDays > 0 && !o.Date.IsZero() && !o.Date.Before(cutoff) && !o.Date.After(now) {
			values = append(values, o.MRPDays)
		}
	}
	if len(values) > 0 {
		snapshot := methodForecastSnapshot{Base: medianFloat64(values), History: h, Source: "резервная объектовая история МРП / последние 36 месяцев"}
		if len(values) >= 2 {
			snapshot.Low = empiricalQuantile(values, 0.25)
			snapshot.High = empiricalQuantile(values, 0.75)
		}
		return snapshot
	}
	return methodForecastSnapshot{Base: current, History: h, Source: "текущий МРП; нет завершённых циклов за 36 месяцев"}
}

func methodsBackgroundResultIsCurrent(result *methodsBackgroundResult, token, generation uint64, key string) bool {
	return result != nil && result.token == token && generation == token && result.key == strings.TrimSpace(key)
}

func methodsHasExplicitDraft(key string) bool {
	_, exists := combinedMethodsForObject(key)
	return exists
}
