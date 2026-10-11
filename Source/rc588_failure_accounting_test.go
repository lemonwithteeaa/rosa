//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC588NoSelectionDoesNotIncreaseCompletion(t *testing.T) {
	e := FailureEvent{OperationMode: "ФОН", Extra: map[string]string{}}
	for _, f := range failureFullFields(e) {
		e.Extra[f.Key] = failureNoSelection
	}
	e.InvestigationStatus = failureNoSelection
	e.TKRS1 = failureNoSelection
	e.TKRS2 = failureNoSelection
	e.DamageType = failureNoSelection
	e.FinalCause = failureNoSelection
	e.RefinedComplication = failureNoSelection
	e.MethodBeforeFailure = failureNoSelection
	e.MethodBeforeFailure2 = failureNoSelection
	e.MethodAfterFailure = failureNoSelection
	e.MethodAfterFailure2 = failureNoSelection
	e.Confirmation = failureNoSelection
	e.Confidence = failureNoSelection
	e.CommissionConclusion = failureNoSelection
	e.CommissionSource = failureNoSelection
	e.FountainPumpType = failureNoSelection
	e.FountainPumpDepth = failureNoSelection
	refreshFailureDerived(&e)
	if e.FullCompletionPct != 0 {
		t.Fatalf("Не выбрано must not increase completion: %d", e.FullCompletionPct)
	}
	if !e.HasFullData || e.NoDataCount == 0 {
		t.Fatalf("explicit no-data must be visible in full-card state: %+v", e)
	}
}

func TestRC588ORDKeepsFourGNOSizes(t *testing.T) {
	e := FailureEvent{OperationMode: "ОРД", EquipmentSize: "ЭЦН5-50", EquipmentSize2: "ШГН 44", EquipmentSize3: "ЭЦН5-80", EquipmentSize4: "ШГН 57"}
	got := failureGNOCompact(e)
	if !strings.Contains(got, "(+3)") || !strings.Contains(got, "ЭЦН5-50") {
		t.Fatalf("ORD compact GNO lost additional sizes: %q", got)
	}
}

func TestRC588ImportMergePreservesExistingFilledValue(t *testing.T) {
	old := FailureEvent{ID: "x", ObjectKey: "o", Field: "F", Well: "1", SourceCause: "ручное уточнение", Comment: "", Extra: map[string]string{"AG": "50"}}
	incoming := old
	incoming.SourceCause = "значение из файла"
	incoming.Comment = "добавлено из файла"
	incoming.Extra = map[string]string{"AG": "60", "AH": "НОВОМЕТ"}
	merged, added, conflicts := mergeImportedFailure(old, incoming)
	if merged.SourceCause != old.SourceCause {
		t.Fatalf("import overwrote user value: %q", merged.SourceCause)
	}
	if merged.Comment != incoming.Comment || merged.Extra["AH"] != "НОВОМЕТ" || added == 0 {
		t.Fatalf("blank fields were not safely supplemented: %+v", merged)
	}
	if len(conflicts) == 0 {
		t.Fatal("filled-value conflict must be reported")
	}
}

func TestRC588OperationalOnlyAndFullStatusAreSeparate(t *testing.T) {
	e := FailureEvent{Field: "F", Well: "1", CycleStartDate: "01.01.2026", Date: "02.01.2026", OperationMode: "ЭЦН", Equipment: "ЭЦН", EquipmentBrand: "НОВОМЕТ", EquipmentSize: "ЭЦН5-50", EventType: "Отказ", StopCause: "Снижение подачи", FailedUnit: "Насос", PreliminaryCause: "Износ", PrimaryComplication: "Механические примеси"}
	refreshFailureDerived(&e)
	if !e.OperationalOnly || failureDataStatus(e) != "Только оперативный ввод" {
		t.Fatalf("unexpected operational-only status: %+v / %q", e, failureDataStatus(e))
	}
	e.TKRS1 = "ПРС"
	refreshFailureDerived(&e)
	if e.OperationalOnly || e.FullCompletionPct <= 0 || e.FullCompletionPct >= 100 {
		t.Fatalf("partial full card not detected: %+v", e)
	}
}
