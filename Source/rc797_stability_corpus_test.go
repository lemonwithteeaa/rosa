//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC800PageRefreshKeepsOldFrameUntilTargetIsReady(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	start := strings.Index(s, "func activatePage(")
	end := strings.Index(s[start:], "\nfunc switchPage(")
	if start < 0 || end < 0 {
		t.Fatal("activatePage block not found")
	}
	body := s[start : start+end]
	hide := strings.Index(body, "hidePageControls(previousPage)")
	refresh := strings.Index(body, "refreshPage(id)")
	conditionalLayout := strings.Index(body, "if !mainLayoutReady")
	pageLayout := strings.Index(body, "layoutCurrentPage()")
	commit := strings.Index(body, "commitPageVisibility(id)")
	if hide < 0 || refresh < 0 || conditionalLayout < 0 || pageLayout < 0 || commit < 0 || !(refresh < conditionalLayout && conditionalLayout < pageLayout && pageLayout < hide && hide < commit) {
		t.Fatalf("old frame must remain until target data and geometry are ready: refresh=%d layout=%d pageLayout=%d hide=%d commit=%d", refresh, conditionalLayout, pageLayout, hide, commit)
	}
	if strings.Contains(s, "SC_MINIMIZE") || strings.Contains(s, "pageSwitchWasMaximized") {
		t.Fatal("page navigation must not intercept or synthesize window minimization")
	}
	if strings.Contains(body, "sweepInactivePageControls") || strings.Contains(s, "pEnumChildWindows.Call") {
		t.Fatal("page navigation must not rescan the full native child tree")
	}
	showStart := strings.Index(s, "func show(h syscall.Handle, yes bool)")
	showEnd := strings.Index(s[showStart:], "\nfunc beginBusyLoading(")
	showBody := s[showStart : showStart+showEnd]
	if strings.Index(showBody, "if pageSwitching") < 0 {
		t.Fatal("show() must defer physical visibility changes while a page is transitioning")
	}
}

func TestRC799StartupReconcilesLaboratoryBeforeDashboard(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	labSource, err := os.ReadFile("laboratory.go")
	if err != nil {
		t.Fatal(err)
	}
	build := string(mainSource)[strings.Index(string(mainSource), "func buildUI()"):strings.Index(string(mainSource), "\nfunc refreshPage(")]
	if !(strings.Index(build, "buildLaboratoryUI()") < strings.Index(build, "for page := range pageControls") &&
		strings.Index(build, "for page := range pageControls") < strings.Index(build, "activatePage(ID_DASH, false)")) {
		t.Fatal("startup must reconcile all built page controls before opening the dashboard")
	}
	if !strings.Contains(string(labSource), "switchLabGroup(\"corrosion\", false)") {
		t.Fatal("laboratory initialization must be represented in the startup isolation check")
	}
}

func TestRC800TransactionsAndMonitorChangesKeepWindowState(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	main := string(mainSource)
	anchor := main[strings.Index(main, "func anchorMainWindowToMonitor("):strings.Index(main, "type INITCOMMONCONTROLSEX")]
	if strings.Contains(anchor, "pShowWindow.Call") || !strings.Contains(anchor, "pIsIconic.Call") {
		t.Fatal("display and DPI changes must not restore or maximize an existing window")
	}
	for _, name := range []string{"laboratory.go", "methods_workflow.go"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "pSendMessageW.Call(uintptr(h), WM_SETREDRAW") {
			t.Fatalf("%s toggles native redraw on the entire page", name)
		}
	}
}

func TestRC797TwentyWellCorpusHasLinkedMultiYearLaboratoryCoverage(t *testing.T) {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(corpus.Wells) != 20 {
		t.Fatalf("expected exactly 20 wells, got %d", len(corpus.Wells))
	}
	seen := map[string]bool{}
	for _, well := range corpus.Wells {
		if well.ID == "" || seen[well.ID] {
			t.Fatalf("missing or duplicate well ID %q", well.ID)
		}
		seen[well.ID] = true
		if len(well.Failures) != 3 || len(well.LabPasses) < 9 {
			t.Fatalf("%s: expected 3 linked failures and at least 9 lab passes, got %d/%d", well.ID, len(well.Failures), len(well.LabPasses))
		}
		failureIDs := map[string]bool{}
		for _, event := range well.Failures {
			failureIDs[event.ID] = true
		}
		byYear := map[string]int{}
		passIDs := map[string]bool{}
		for _, pass := range well.LabPasses {
			if pass.PassID == "" || passIDs[pass.PassID] || !failureIDs[pass.FailureID] {
				t.Fatalf("%s: lab pass %q is duplicate or not linked to a failure", well.ID, pass.PassID)
			}
			passIDs[pass.PassID] = true
			if len(pass.Date) < 4 {
				t.Fatalf("%s: invalid lab date %q", well.ID, pass.Date)
			}
			byYear[pass.Date[:4]]++
		}
		for _, year := range []string{"2024", "2025", "2026"} {
			if byYear[year] < 2 {
				t.Fatalf("%s: expected at least two lab analyses in %s, got %d", well.ID, year, byYear[year])
			}
		}
	}
	if appBuildVersion != "7.3.11.803-RC" {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}

func TestRC797LaboratorySeedingUsesCorpusFailureLinks(t *testing.T) {
	b, err := os.ReadFile("demo_seed.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "failuresByID[p.FailureID]") {
		t.Fatal("lab seeding must resolve each passport's explicit failure ID")
	}
	if strings.Contains(s, "if i >= len(w.Failures) {\n\t\t\t\tcontinue") {
		t.Fatal("lab seeding must not discard periodic analysis rows after the failure count")
	}
}

func TestRC798CanonicalLabIdentityRepairsReusedIDsAndFiltersOldSamples(t *testing.T) {
	corpus, err := loadTrainingCorpus()
	if err != nil {
		t.Fatal(err)
	}
	ids := canonicalTrainingPassIDs()
	if len(ids) != 180 {
		t.Fatalf("expected 180 canonical lab IDs, got %d", len(ids))
	}
	well := corpus.Wells[13]
	event := well.Failures[0]
	var source trainingLabPass
	for _, candidate := range well.LabPasses {
		if candidate.FailureID == event.ID {
			source = candidate
			break
		}
	}
	canonical := trainingLabPassRecord(well, event, source)
	if !trainingLabPassMatchesCanonical(canonical, canonical) {
		t.Fatal("canonical record must match itself")
	}
	stale := canonical
	stale.ObjectKey = "Учебная скважина 99 · старый фонд"
	if trainingLabPassMatchesCanonical(stale, canonical) {
		t.Fatal("reused ID with an old object key must be repaired")
	}
	stale = canonical
	stale.Groups = map[string]map[string]string{"ph": {"ph": "1.00"}}
	if trainingLabPassMatchesCanonical(stale, canonical) {
		t.Fatal("canonical ID with stale measurements must be repaired")
	}
	stale = canonical
	stale.PassID = "TRN-OLD-42"
	if _, current := ids[stale.PassID]; current {
		t.Fatal("retired synthetic sample ID must not enter canonical history")
	}
}
