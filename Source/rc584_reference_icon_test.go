//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC584ReferenceUsesStandardCaptionAndCalmHeader(t *testing.T) {
	b, err := os.ReadFile("methods_reference_dialog.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	required := []string{
		"headerH := int32(76)",
		"tabsY := int32(88)",
		"contentY := int32(150)",
		"methodsReferenceTitleHandle), WM_SETFONT, uintptr(hFontSystemName)",
		"methodsReferenceSubtitleHandle), WM_SETFONT, uintptr(hFont)",
		"HIcon:         hIconBig",
		"HIconSm:       hIconSmall",
		"RC584: one strong card title only",
	}
	for _, q := range required {
		if !strings.Contains(s, q) {
			t.Fatalf("RC584 reference contract missing %q", q)
		}
	}
	if !strings.Contains(s, "hwnd, _, _ := pCreateWindowExW.Call(\n\t\t0,") {
		t.Fatal("methods reference window must use normal Windows caption, not WS_EX_TOOLWINDOW")
	}
}

func TestRC584ReleaseBuildRequiresApprovedIcon(t *testing.T) {
	b, err := os.ReadFile("build_windows_gui.ps1")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, q := range []string{"embed_icon_resource.py", "verify_icon_resource.py", "OF_Model_icon_round.ico"} {
		if !strings.Contains(s, q) {
			t.Fatalf("RC584 icon release guard missing %q", q)
		}
	}
	if !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
