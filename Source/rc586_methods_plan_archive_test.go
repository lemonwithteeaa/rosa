//go:build windows

package main

import "testing"

func TestRC586MethodPlanCanonicalStatus(t *testing.T) {
	cases := map[string]string{
		"В плане": "Выбран",
		"утверждено технологом": "Утверждён",
		"Выполнен":              "Выполнен",
		"":                      "Черновик",
	}
	for in, want := range cases {
		if got := methodPlanCanonicalStatus(in); got != want {
			t.Fatalf("status %q: got %q want %q", in, got, want)
		}
	}
}

func TestRC586DecisionVersionSequence(t *testing.T) {
	oldLoaded := approvedMethodsLoaded
	old := approvedMethodDecisions
	defer func() { approvedMethodsLoaded, approvedMethodDecisions = oldLoaded, old }()
	approvedMethodsLoaded = true
	approvedMethodDecisions = []approvedMethodDecision{
		{ID: "old1", ObjectKey: "OBJ"},
		{ID: "old2", ObjectKey: "OBJ"},
		{ID: "new3", ObjectKey: "OBJ", VersionNumber: 3},
	}
	if got := nextMethodDecisionVersion("OBJ"); got != 4 {
		t.Fatalf("next version=%d want 4", got)
	}
}

func TestRC586ArchiveRestoreStatus(t *testing.T) {
	oldLoaded := approvedMethodsLoaded
	old := approvedMethodDecisions
	defer func() { approvedMethodsLoaded, approvedMethodDecisions = oldLoaded, old }()
	approvedMethodsLoaded = true
	approvedMethodDecisions = []approvedMethodDecision{
		{ID: "v1", ObjectKey: "OBJ", VersionNumber: 1},
		{ID: "v2", ObjectKey: "OBJ", VersionNumber: 2, RestoredFromID: "v1"},
	}
	got := methodArchiveVersionStatus(approvedMethodDecisions[1])
	want := "Версия 2 · восстановлено из v1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
