//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC400LoginWindowIsHiddenUntilFullyBuilt(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	visibleCreate := `WS_OVERLAPPEDWINDOW|WS_VISIBLE, 0x80000000`
	if strings.Contains(s, visibleCreate) {
		t.Fatal("login top-level window must not be visible during WM_CREATE")
	}
	if !strings.Contains(s, `WS_OVERLAPPEDWINDOW, 0x80000000`) {
		t.Fatal("hidden login window creation rule is missing")
	}
	if !strings.Contains(s, `pShowWindow.Call(hwnd, SW_MAXIMIZE)`) {
		t.Fatal("login window is not shown after initialization")
	}
	if !strings.Contains(s, `authFirstFrameRedraw`) || !strings.Contains(s, `pRedrawWindow.Call(hwnd, 0, 0, authFirstFrameRedraw)`) {
		t.Fatal("full first-frame redraw is missing")
	}
}

func TestRC400Version(t *testing.T) {
	if appBuildVersion != "7.3.11.400-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
