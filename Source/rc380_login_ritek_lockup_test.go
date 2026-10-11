//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC380RitekUsesOriginalStackedBrandLockup(t *testing.T) {
	auth, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	brand, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	a := string(auth)
	b := string(brand)
	for _, want := range []string{
		`authRitekLogo = authCreate("STATIC", "", WS_CHILD|WS_VISIBLE|SS_BITMAP|SS_CENTERIMAGE`,
		`clientW-48-150`,
		`clientH-145`,
		`150, 136`,
		`buildFooterRitekBitmap()`,
	} {
		if !strings.Contains(a, want) {
			t.Fatalf("missing RC380 stacked RITEK layout requirement: %s", want)
		}
	}
	for _, want := range []string{
		`//go:embed RITEK_footer_label.png`,
		`_ritek_footer_label_v3.bmp`,
		`resizeContainOnBackground(clean, 150, 136, panel)`,
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("missing RC380 original RITEK asset requirement: %s", want)
		}
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}
