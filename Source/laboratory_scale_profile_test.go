//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func profileTestValues() map[string]string {
	return map[string]string{
		"ca": "40.078", "co3": "60.01", "so4": "96.06", "ba": "1.37327", "sr": "0.8762",
		"sample_temp": "25", "profile_temp_min": "10", "profile_pressure_min": "5", "profile_temp_max": "90", "profile_pressure_max": "25",
	}
}

func TestTemperatureProfileThreePointsAndNoPressureEffect(t *testing.T) {
	v := profileTestValues()
	pts, err := laboratoryTemperatureProfile(v)
	if err != nil || len(pts) != 3 {
		t.Fatal(pts, err)
	}
	if pts[0].Temperature != 10 || pts[1].Temperature != 50 || pts[2].Temperature != 90 || pts[0].PressureMPa != 5 || pts[1].PressureMPa != 15 || pts[2].PressureMPa != 25 {
		t.Fatal(pts)
	}
	base := pts[1].Results[0].SI
	v["profile_pressure_min"], v["profile_pressure_max"] = "50", "150"
	pts2, err := laboratoryTemperatureProfile(v)
	if err != nil || math.Abs(pts2[1].Results[0].SI-base) > 1e-12 {
		t.Fatal("pressure leaked into unverified SI", base, pts2, err)
	}
}

func TestTemperatureProfileValidation(t *testing.T) {
	for key, val := range map[string]string{"profile_pressure_max": "", "profile_temp_min": "101", "profile_pressure_min": "-1"} {
		v := profileTestValues()
		v[key] = val
		if _, err := laboratoryTemperatureProfile(v); err == nil {
			t.Fatal("accepted", key, val)
		}
	}
	v := profileTestValues()
	v["profile_temp_min"], v["profile_temp_max"] = "90", "10"
	if _, err := laboratoryTemperatureProfile(v); err == nil {
		t.Fatal("accepted reversed temperatures")
	}
}

func TestTemperatureProfileTextIsExplicitAboutPressure(t *testing.T) {
	text := laboratoryTemperatureProfileText(profileTestValues())
	for _, want := range []string{"ПРОВЕРЕНА ТОЛЬКО ТЕМПЕРАТУРНАЯ ЧАСТЬ", "НЕ участвует в SI", "Tmin", "середина", "Tmax", "не применено"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	full := laboratorySaturationTextForModel(profileTestValues(), "temperature_ideal")
	if !strings.Contains(full, "три температурные контрольные точки") || !strings.Contains(full, "ПРОФИЛЬ T/P") {
		t.Fatal(full)
	}
}

func TestPressureReferencePreparedButNotEnabled(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("reference", "tp_profile", "pressure_reference_manifest.txt"))
	if err != nil {
		t.Fatal(err)
	}
	m := string(manifest)
	for _, want := range []string{"status=PREPARED_NOT_EXECUTED", "pressure_effect_in_ROSA_remains_disabled"} {
		if !strings.Contains(m, want) {
			t.Fatalf("pressure reference manifest lost safety marker %q: %s", want, m)
		}
	}
	if _, err := os.Stat(filepath.Join("reference", "tp_profile", "golden.tsv")); err == nil {
		t.Fatal("golden.tsv must not be bundled before an actual external PHREEQC run")
	}
}
