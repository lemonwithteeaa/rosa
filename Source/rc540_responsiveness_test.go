//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC540GlobalObjectTransitionDoesNotRefreshHiddenNativePages(t *testing.T) {
	b, err := os.ReadFile("object_selection.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func applyGlobalObjectSelection")
	end := strings.Index(s[start:], "\nfunc confirmSelectedObject")
	if start < 0 || end < 0 {
		t.Fatal("applyGlobalObjectSelection not found")
	}
	body := s[start : start+end]
	if !strings.Contains(body, "switch currentPage") || !strings.Contains(body, "case ID_DASH") || !strings.Contains(body, "case ID_OBJECTS") {
		t.Fatalf("object transition must refresh only the visible page")
	}
	if strings.Contains(body, "refreshDashboard()\n\trefreshWellCard()") {
		t.Fatalf("hidden Dashboard/Well Card must never be refreshed as one synchronous chain")
	}
}

func TestRC540TeachingStateIsConstantTimeForListRendering(t *testing.T) {
	b, err := os.ReadFile("demo_seed.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func teachingWorkflowStateForObject")
	end := strings.Index(s[start:], "\nfunc teachingWorkflowObservations")
	if start < 0 || end < 0 {
		t.Fatal("teachingWorkflowStateForObject not found")
	}
	body := s[start : start+end]
	for _, forbidden := range []string{"loadLabPasses()", "range passes", "range labPasses", "repositoryLabPassesForObject"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("teaching state must stay O(1) during UI list rebuild; found %q", forbidden)
		}
	}
}

func TestRC540BuildVersion(t *testing.T) {
	if appBuildVersion != "7.3.11.540-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
