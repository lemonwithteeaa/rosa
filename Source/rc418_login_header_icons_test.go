package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC418LoginHeaderAndFeatureMarkersArePresent(t *testing.T) {
	raw, err := os.ReadFile("auth.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	wants := []string{
		"gap = (usableW - totalWordsW) / int32(len(words)-1)",
		"buildLoginStandaloneIconBitmap(false, i",
		"featureIconSize := int32(28)",
		"Cards first, accents second",
	}
	for _, want := range wants {
		if !strings.Contains(src, want) {
			t.Fatalf("RC418 marker missing: %s", want)
		}
	}
}
