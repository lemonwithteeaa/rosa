//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC402LoginFirstFrameAndRecoveryGeometry(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	checks := []string{
		`WS_EX_COMPOSITED_AUTH`,
		`WS_OVERLAPPEDWINDOW|WS_MAXIMIZE_AUTH|WS_CLIPCHILDREN_AUTH`,
		`pShowWindow.Call(hwnd, SW_SHOW)`,
		`panelW, panelH := int32(462), int32(248)`,
		`authSetStatus("")`,
		`authRosaSubtitle = authCreate("STATIC", "ИНФОРМАЦИОННО-АНАЛИТИЧЕСКАЯ СИСТЕМА`,
		`authBrandCaptionFont`,
		`SS_CENTER|SS_NOPREFIX|SS_ENDELLIPSIS`,
	}
	for _, want := range checks {
		if !strings.Contains(s, want) {
			t.Fatalf("missing RC402 rule: %s", want)
		}
	}
	if strings.Contains(s, `pSendMessageW.Call(hwnd, WM_SETREDRAW, 0, 0)\n\tpShowWindow.Call(hwnd, SW_MAXIMIZE)`) {
		t.Fatal("old first-frame redraw toggle returned")
	}
	if appBuildVersion != "7.3.11.402-RC" {
		t.Fatalf("version=%s", appBuildVersion)
	}
}
