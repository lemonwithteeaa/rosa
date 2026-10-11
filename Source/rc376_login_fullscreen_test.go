//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC376LoginFullscreenAndCleanFooter(t *testing.T) {
	b, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `pShowWindow.Call(hwnd, SW_MAXIMIZE)`) {
		t.Fatal("login window is not maximized on open")
	}
	for _, obsolete := range []string{"Локальная работа", "История пользователя", "Резервирование данных"} {
		if strings.Contains(s, obsolete) {
			t.Fatalf("obsolete bottom text remains: %s", obsolete)
		}
	}
	if !strings.Contains(s, `buildLoginThemesBitmap()`) || !strings.Contains(s, `rightInnerX, 674, rightInnerW, 120`) {
		t.Fatal("thematic illustration block is not present in the expanded login card")
	}
	if !strings.Contains(s, `authApplyCenteredLayout(clientW)`) || !strings.Contains(s, `footerY := max32(801, clientH-58)`) {
		t.Fatal("fullscreen login layout is not anchored to the actual client area")
	}
	if !strings.Contains(s, `authRitekLogo = authCreate`) || !strings.Contains(s, `buildFooterRitekBitmap()`) {
		t.Fatal("RITEK footer label is missing")
	}
	if !strings.Contains(s, `authLukoilLogo = authCreate`) {
		t.Fatal("LUKOIL header logo must remain")
	}
}

func TestRC376Version(t *testing.T) {
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
