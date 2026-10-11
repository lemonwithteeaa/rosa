//go:build windows && legacy_rc_tests

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC382LoginExactReferenceComposition(t *testing.T) {
	auth, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	brand, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	a, b := string(auth), string(brand)
	checks := []string{
		`const authBaseWidth int32 = 1144`,
		`buildLoginReferenceCardBitmap(true)`,
		`buildLoginReferenceCardBitmap(false)`,
		`login_left_reference_v2.png`,
		`login_right_reference_v2.png`,
		`authUsersList = authCreate("LISTBOX", "", WS_CHILD|LBS_NOTIFY|LBS_HASSTRINGS`,
		`authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW|SS_NOTIFY, 36, cardY+524`,
		`authCreateCentered("STATIC", "", WS_CHILD|WS_VISIBLE|SS_OWNERDRAW|SS_NOTIFY, 36, cardY+584`,
		`authRitekBitmap, _ = buildFooterRitekBitmap()`,
		`{"РАСЧЁТ", false}, {"  •  ", true}`,
		`{"Вход в ", false}, {"РОСА", true}`,
	}
	for _, want := range checks {
		if !strings.Contains(a, want) && !strings.Contains(b, want) {
			t.Fatalf("missing RC382 exact-reference requirement: %s", want)
		}
	}
	if strings.Contains(a, `"СОХРАНЁННЫЙ ПРОФИЛЬ"`) {
		t.Fatal("obsolete visible saved-profile block returned")
	}
	if appBuildVersion != "7.3.11.382-RC" {
		t.Fatalf("unexpected build version: %s", appBuildVersion)
	}
}

func TestRC382UsesOriginalRitekAsset(t *testing.T) {
	b, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`//go:embed RITEK_footer_label.png`, `_ritek_footer_label_v3.bmp`, `resizeContainOnBackground(clean, 150, 136, panel)`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing original RITEK asset rule: %s", want)
		}
	}
}
