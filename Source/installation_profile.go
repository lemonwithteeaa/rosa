//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// InstallationProfile supplies equipment context shared by telemetry and
// dynamometry. Empty fields are valid: older datasets remain readable.
type InstallationProfile struct {
	ObjectKey      string  `json:"object_key"`
	EquipmentType  string  `json:"equipment_type,omitempty"`
	PumpModel      string  `json:"pump_model,omitempty"`
	DriveModel     string  `json:"drive_model,omitempty"`
	TubingDiameter float64 `json:"tubing_diameter_mm,omitempty"`
	PumpDepth      float64 `json:"pump_depth_m,omitempty"`
	StrokeLength   float64 `json:"stroke_length_m,omitempty"`
	StrokesPerMin  float64 `json:"strokes_per_min,omitempty"`
	NominalCurrent float64 `json:"nominal_current_a,omitempty"`
	NominalRate    float64 `json:"nominal_rate_m3_day,omitempty"`
	RodDiameter    float64 `json:"rod_diameter_mm,omitempty"`
	UpdatedAt      string  `json:"updated_at"`
}

var installationProfiles map[string]InstallationProfile

func installationProfilesPath() string { return filepath.Join(dataDir, "installation_profiles.json") }

func loadInstallationProfiles() {
	installationProfiles = map[string]InstallationProfile{}
	loadJSON(installationProfilesPath(), &installationProfiles)
}

func installationProfileForObject(objectKey string) InstallationProfile {
	if installationProfiles == nil {
		loadInstallationProfiles()
	}
	return installationProfiles[normalizeLaboratoryObjectKey(objectKey)]
}

func saveInstallationProfile(profile InstallationProfile) error {
	if err := validateInstallationProfile(profile); err != nil {
		return err
	}
	if installationProfiles == nil {
		loadInstallationProfiles()
	}
	profile.ObjectKey = strings.TrimSpace(profile.ObjectKey)
	profile.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	installationProfiles[normalizeLaboratoryObjectKey(profile.ObjectKey)] = profile
	return saveJSON(installationProfilesPath(), installationProfiles)
}

func validateInstallationProfile(p InstallationProfile) error {
	if strings.TrimSpace(p.ObjectKey) == "" {
		return fmt.Errorf("не задан ключ объекта")
	}
	checks := []struct {
		name       string
		value, max float64
	}{
		{"Диаметр НКТ", p.TubingDiameter, 1000},
		{"Глубина спуска насоса", p.PumpDepth, 20000},
		{"Длина хода", p.StrokeLength, 20},
		{"Число качаний", p.StrokesPerMin, 100},
		{"Номинальный ток", p.NominalCurrent, 10000},
		{"Номинальная подача", p.NominalRate, 100000},
		{"Диаметр штанг", p.RodDiameter, 200},
	}
	for _, c := range checks {
		if c.value < 0 || c.value > c.max {
			return fmt.Errorf("%s: допустимо значение от 0 до %g", c.name, c.max)
		}
	}
	return nil
}

func installationProfileValueLines(p InstallationProfile) (used, missing []string) {
	texts := []struct{ name, value string }{
		{"тип оборудования", strings.TrimSpace(p.EquipmentType)},
		{"модель насоса", strings.TrimSpace(p.PumpModel)},
		{"модель привода", strings.TrimSpace(p.DriveModel)},
	}
	for _, x := range texts {
		if x.value == "" {
			missing = append(missing, x.name)
		} else {
			used = append(used, x.name+" = "+x.value)
		}
	}
	numbers := []struct {
		name, unit string
		value      float64
	}{
		{"диаметр НКТ", "мм", p.TubingDiameter}, {"глубина спуска насоса", "м", p.PumpDepth},
		{"длина хода", "м", p.StrokeLength}, {"число качаний", "мин⁻¹", p.StrokesPerMin},
		{"номинальный ток", "А", p.NominalCurrent}, {"номинальная подача", "м³/сут", p.NominalRate},
		{"диаметр штанг", "мм", p.RodDiameter},
	}
	for _, x := range numbers {
		if x.value <= 0 {
			missing = append(missing, x.name)
		} else {
			used = append(used, fmt.Sprintf("%s = %g %s", x.name, x.value, x.unit))
		}
	}
	return used, missing
}

func installationProfileContext(p InstallationProfile, source string) string {
	used, missing := installationProfileValueLines(p)
	if strings.TrimSpace(source) == "" {
		source = "профиль установки"
	}
	date := strings.TrimSpace(p.UpdatedAt)
	if date == "" {
		date = "дата отсутствует"
	}
	if len(used) == 0 {
		return fmt.Sprintf("Параметры установки: отсутствуют. Источник: %s; %s.", source, date)
	}
	return fmt.Sprintf("Параметры установки — источник: %s; дата: %s. Использовано: %s. Отсутствует: %s.", source, date, strings.Join(used, "; "), strings.Join(missing, "; "))
}
