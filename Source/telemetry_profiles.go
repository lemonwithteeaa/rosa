//go:build windows

package main

import "strings"

// TelemetrySourceProfile deliberately stores aliases as configuration. We know
// which systems are used (Телескоп, MegaWeb, WebSCADA), but tag names and export
// layouts can differ between deployments. RОSA therefore never hard-codes a
// vendor-specific tag as a universal truth: a local profile can be adjusted
// once and then reused for subsequent offline exports.
type TelemetryChannelAlias struct {
	Canonical string   `json:"canonical"`
	Display   string   `json:"display"`
	Unit      string   `json:"unit,omitempty"`
	Aliases   []string `json:"aliases"`
}

type TelemetrySourceProfile struct {
	Name             string                  `json:"name"`
	TimestampAliases []string                `json:"timestamp_aliases,omitempty"`
	DateAliases      []string                `json:"date_aliases,omitempty"`
	TimeAliases      []string                `json:"time_aliases,omitempty"`
	WellAliases      []string                `json:"well_aliases,omitempty"`
	Channels         []TelemetryChannelAlias `json:"channels,omitempty"`
	Note             string                  `json:"note,omitempty"`
}

var telemetryProfiles []TelemetrySourceProfile

func defaultTelemetryProfiles() []TelemetrySourceProfile {
	commonTS := []string{"datetime", "timestamp", "dateandtime", "датаивремя", "временнаяметка", "времяизмерения", "дата время"}
	commonDate := []string{"date", "дата", "measurementdate", "датазамера"}
	commonTime := []string{"time", "время", "measurementtime", "времязамера"}
	commonWell := []string{"well", "wellno", "wellnumber", "скважина", "номерскважины", "скв", "объект", "object"}
	note := "Профиль содержит только подтверждённые общие обозначения. Точные теги конкретной установки добавляются после проверки реальной выгрузки и не распространяются автоматически на другие общества."
	return []TelemetrySourceProfile{
		{Name: "Телескоп", TimestampAliases: commonTS, DateAliases: commonDate, TimeAliases: commonTime, WellAliases: commonWell, Note: note},
		{Name: "MegaWeb", TimestampAliases: commonTS, DateAliases: commonDate, TimeAliases: commonTime, WellAliases: commonWell, Note: note},
		{Name: "WebSCADA", TimestampAliases: commonTS, DateAliases: commonDate, TimeAliases: commonTime, WellAliases: commonWell, Note: note},
		{Name: "Универсальный", TimestampAliases: commonTS, DateAliases: commonDate, TimeAliases: commonTime, WellAliases: commonWell, Note: "Универсальный профиль CSV/XLSX."},
	}
}

func loadTelemetryProfiles() {
	var stored []TelemetrySourceProfile
	loadJSON(sharedPath("telemetry_profiles.json"), &stored)
	if len(stored) == 0 {
		stored = defaultTelemetryProfiles()
		_ = saveJSON(sharedPath("telemetry_profiles.json"), stored)
	}
	telemetryProfiles = stored
}

func telemetryProfileFor(source string) TelemetrySourceProfile {
	if len(telemetryProfiles) == 0 {
		loadTelemetryProfiles()
	}
	for _, p := range telemetryProfiles {
		if strings.EqualFold(strings.TrimSpace(p.Name), strings.TrimSpace(source)) {
			return p
		}
	}
	for _, p := range telemetryProfiles {
		if strings.EqualFold(p.Name, "Универсальный") {
			return p
		}
	}
	return TelemetrySourceProfile{Name: source}
}

func profileAliasMatch(n string, aliases []string) bool {
	for _, a := range aliases {
		if n == normalizeTelemetryName(a) {
			return true
		}
	}
	return false
}

func profileChannelMatch(p TelemetrySourceProfile, n string) (string, string, string, bool) {
	for _, c := range p.Channels {
		for _, a := range c.Aliases {
			an := normalizeTelemetryName(a)
			if n == an || (len(an) >= 4 && strings.Contains(n, an)) {
				return c.Canonical, c.Display, c.Unit, true
			}
		}
	}
	return "", "", "", false
}
