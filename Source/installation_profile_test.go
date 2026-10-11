//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestInstallationProfileUsesNormalizedObjectKey(t *testing.T) {
	installationProfiles = map[string]InstallationProfile{
		"fieldw1": {ObjectKey: "Field W1", PumpDepth: 1450},
	}
	profile := installationProfileForObject(" FIELD W1 ")
	if profile.PumpDepth != 1450 {
		t.Fatalf("profile was not resolved: %#v", profile)
	}
}

func TestInstallationProfileValidationAndContext(t *testing.T) {
	if err := validateInstallationProfile(InstallationProfile{PumpDepth: -1}); err == nil {
		t.Fatal("negative pump depth must be rejected")
	}
	p := InstallationProfile{ObjectKey: "W1", EquipmentType: "ШГН", PumpDepth: 1450, NominalCurrent: 30, UpdatedAt: "2026-09-16T10:00:00Z"}
	text := installationProfileContext(p, "сохранённый профиль")
	for _, want := range []string{"сохранённый профиль", "2026-09-16", "глубина спуска насоса = 1450 м", "Отсутствует:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("context %q misses %q", text, want)
		}
	}
}

func TestTelemetryUsesImportProfileSnapshot(t *testing.T) {
	x := TelemetryImport{Installation: InstallationProfile{NominalCurrent: 20, UpdatedAt: "2026-09-16"}}
	text := telemetryInstallationContext(x, []TelemetryChannelSummary{{Canonical: "current", Unit: "А", Mean: 10}})
	if !strings.Contains(text, "50.0 % номинального") {
		t.Fatalf("unexpected context: %s", text)
	}
	empty := telemetryInstallationContext(TelemetryImport{}, nil)
	if !strings.Contains(empty, "задним числом не подмешивается") {
		t.Fatalf("missing historical limitation: %s", empty)
	}
}

func TestTelemetryRC151ReportsMissingInstallationParameters(t *testing.T) {
	x := TelemetryImport{Installation: InstallationProfile{UpdatedAt: "2026-09-18", EquipmentType: "УЭЦН", NominalCurrent: 50}}
	text := telemetryInstallationContext(x, []TelemetryChannelSummary{{Canonical: "current", Unit: "А", Mean: 25}})
	if !strings.Contains(text, "50.0 % номинального") || !strings.Contains(text, "отсутствует:") || !strings.Contains(text, "номинальная подача") {
		t.Fatalf("unexpected installation context: %s", text)
	}
}
