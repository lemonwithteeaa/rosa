//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC657DemoFastPathRepairsIdentity(t *testing.T) {
	b, err := os.ReadFile("demo_seed.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "if len(receipts) == len(entries)")
	end := strings.Index(s[start:], "temp, err := os.MkdirTemp")
	if start < 0 || end < 0 {
		t.Fatal("demo fast path not found")
	}
	fast := s[start : start+end]
	identity := strings.Index(fast, "repairBundledDemoIdentity(receipts)")
	watercut := strings.Index(fast, "repairBundledDemoWatercut(receipts)")
	if identity < 0 || watercut < 0 || identity > watercut {
		t.Fatal("demo identity must be repaired before regime values")
	}
}

func TestRC657MethodsPlanAndTargetActionAreRestored(t *testing.T) {
	flowBytes, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	flow := string(flowBytes)
	for _, want := range []string{"show(methodsPlanTitle, true)", "show(methodsPlanList, true)", "teachingCrossHistoryPoints(methodsTeachingPreviewKey(key), time.Now())"} {
		if !strings.Contains(flow, want) {
			t.Fatalf("restored plan marker missing: %s", want)
		}
	}
	refBytes, err := os.ReadFile("methods_reference_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	ref := string(refBytes)
	for _, want := range []string{"ИЗМЕНИТЬ ЦЕЛЕВОЙ ОРИЕНТИР", "activatePage(ID_MRP, true)"} {
		if !strings.Contains(ref, want) {
			t.Fatalf("target action marker missing: %s", want)
		}
	}
}
