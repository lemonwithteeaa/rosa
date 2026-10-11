//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRC367OverallInfographicNeverDisappearsAtCommonDesktopHeights(t *testing.T) {
	for _, h := range []int32{720, 768, 800, 900, 1080} {
		got := laboratoryOverallInfographicHeight(h, 156)
		if got < 136 {
			t.Fatalf("height %d: infographic collapsed to %d", h, got)
		}
	}
}

func TestRC367OverallInfographicKeepsSixDirectionEnvelope(t *testing.T) {
	natural := laboratoryInfluenceChartHeight(900, 6)
	got := laboratoryOverallInfographicHeight(768, natural)
	if got < 136 {
		t.Fatalf("six-direction infographic too small: natural=%d got=%d", natural, got)
	}
}

func TestRC367PowerShellExportIsHidden(t *testing.T) {
	b, err := os.ReadFile("laboratory_report_export.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"-WindowStyle", "Hidden"`, `cmd.SysProcAttr = hiddenProcessAttributes()`} {
		if !strings.Contains(s, want) {
			t.Fatalf("hidden export guard missing %q", want)
		}
	}
}

func TestRC367BuildVersionLineContinues(t *testing.T) {
	if !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") {
		t.Fatalf("unexpected build version %q", appBuildVersion)
	}
}
