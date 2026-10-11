//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC422VisualBalanceAndHeaderNoClipGuards(t *testing.T) {
	authB, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	auth := string(authB)
	mainB, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	mainS := string(mainB)
	brandB, err := os.ReadFile("brand.go")
	if err != nil {
		t.Fatal(err)
	}
	brand := string(brandB)

	for _, want := range []string{
		`add(authNGDOEdit, []string{"—", "РИТЭК"})`,
		`add(authCITSEdit, []string{"—",`,
		`add(authCDNGEdit, []string{"—",`,
		`add(authPositionEdit, []string{"—",`,
		`featureIconSize := int32(34)`,
		`buildLoginStandaloneIconBitmap(true, i, 58, 58)`,
	} {
		if !strings.Contains(auth, want) {
			t.Fatalf("missing RC422 auth guard %q", want)
		}
	}
	if strings.Contains(auth, `rightInnerX, 688, rightInnerW, 1`) {
		t.Fatal("decorative separator must stay removed to reduce visual noise")
	}
	for _, want := range []string{
		`const appBuildVersion = "7.3.11.422-RC"`,
		`rightBlockW := int32(520)`,
		`lukoilW = 160`,
		`Единая инженерная среда\r\nдля работы с осложнённым фондом скважин`,
	} {
		if !strings.Contains(mainS, want) {
			t.Fatalf("missing RC422 main header guard %q", want)
		}
	}
	for _, want := range []string{
		`_rosa_header_brand_v10.bmp`, `image.Rect(0, 0, 174, 68)`,
		`_lukoil_header_brand_v11.bmp`, `image.Rect(0, 0, 156, 52)`,
	} {
		if !strings.Contains(brand, want) {
			t.Fatalf("missing RC422 bitmap no-clip guard %q", want)
		}
	}
}
