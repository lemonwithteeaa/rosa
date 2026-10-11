package main

import "testing"

func TestGNOCommandTargets(t *testing.T) {
	tests := []struct {
		command int
		page    int
	}{
		{ID_GNO_DYNAMOMETRY, ID_DYN},
		{ID_GNO_TELEMETRY, ID_TELEMETRY},
	}
	for _, test := range tests {
		page, ok := gnoPageForCommand(test.command)
		if !ok || page != test.page {
			t.Fatalf("command %d: got page=%d ok=%v; want page=%d", test.command, page, ok, test.page)
		}
	}
	if _, ok := gnoPageForCommand(ID_DASH); ok {
		t.Fatal("non-GNO command was accepted as a GNO tab")
	}
}

func TestGNOActiveStates(t *testing.T) {
	for _, page := range []int{ID_DYN, ID_TELEMETRY} {
		if !isGNOPage(page) {
			t.Fatalf("page %d must belong to GNO diagnostics", page)
		}
		if !isNavigationButtonActive(ID_TECHPARAM, page) {
			t.Fatalf("technology-parameters navigation must stay active on page %d", page)
		}
	}
	if isNavigationButtonActive(ID_LAB, ID_TELEMETRY) {
		t.Fatal("unrelated primary navigation item became active on monitoring page")
	}
	if !isGNOSubmoduleTabActive(ID_GNO_DYNAMOMETRY, ID_DYN) ||
		isGNOSubmoduleTabActive(ID_GNO_TELEMETRY, ID_DYN) {
		t.Fatal("wrong active tab state on dynamometry page")
	}
	if !isGNOSubmoduleTabActive(ID_GNO_TELEMETRY, ID_TELEMETRY) ||
		isGNOSubmoduleTabActive(ID_GNO_DYNAMOMETRY, ID_TELEMETRY) {
		t.Fatal("wrong active tab state on monitoring page")
	}
}

func TestTechParameterActionTargets(t *testing.T) {
	tests := []struct{ command, page int }{
		{ID_NAV_TELEMETRY, ID_TELEMETRY},
		{ID_NAV_DYNAMOMETRY, ID_DYN},
		{ID_NAV_DATA, ID_DATA},
		{ID_NAV_TECHBANK, ID_TECH},
	}
	for _, test := range tests {
		page, ok := techParameterActionPage(test.command)
		if !ok || page != test.page {
			t.Fatalf("command %d: got page=%d ok=%v; want page=%d", test.command, page, ok, test.page)
		}
	}
	if _, ok := techParameterActionPage(ID_DASH); ok {
		t.Fatal("unrelated command was accepted as a technology-parameters action")
	}
}
