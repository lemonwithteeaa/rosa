//go:build windows

package main

import (
	"strings"
	"time"
)

// OrganizationProfile describes the organizational context independently from
// calculation logic. RОSA must be portable across the LUKOIL Group, therefore
// company/division names are configuration data rather than hard-coded rules.
type OrganizationProfile struct {
	GroupName        string `json:"group_name"`
	OrganizationName string `json:"organization_name"`
	UnitLabel        string `json:"unit_label"` // ЦИТС for RITEK, ТПП for other LUKOIL companies by current convention
	UnitPlural       string `json:"unit_plural"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

var organizationProfile OrganizationProfile

func defaultOrganizationProfile() OrganizationProfile {
	return OrganizationProfile{
		GroupName:        "Группа ЛУКОЙЛ",
		OrganizationName: "ООО «РИТЭК»",
		UnitLabel:        "ЦИТС",
		UnitPlural:       "ЦИТС",
		UpdatedAt:        time.Now().Format(time.RFC3339),
	}
}

func normalizeOrganizationProfile(p OrganizationProfile) OrganizationProfile {
	if strings.TrimSpace(p.GroupName) == "" {
		p.GroupName = "Группа ЛУКОЙЛ"
	}
	if strings.TrimSpace(p.OrganizationName) == "" {
		p.OrganizationName = "ООО «РИТЭК»"
	}
	// Current organizational convention supplied by the project owner:
	// RITEK uses ЦИТС; other LUKOIL companies use ТПП. Keep the database field
	// neutral, while the UI label follows the profile.
	if strings.TrimSpace(p.UnitLabel) == "" {
		if strings.Contains(strings.ToLower(p.OrganizationName), "ритэк") {
			p.UnitLabel = "ЦИТС"
			p.UnitPlural = "ЦИТС"
		} else {
			p.UnitLabel = "ТПП"
			p.UnitPlural = "ТПП"
		}
	}
	if strings.TrimSpace(p.UnitPlural) == "" {
		p.UnitPlural = p.UnitLabel
	}
	if strings.TrimSpace(p.UpdatedAt) == "" {
		p.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	return p
}

func loadOrganizationProfile() OrganizationProfile {
	p := defaultOrganizationProfile()
	var stored OrganizationProfile
	loadJSON(sharedPath("organization_profile.json"), &stored)
	if strings.TrimSpace(stored.OrganizationName) != "" || strings.TrimSpace(stored.UnitLabel) != "" {
		p = stored
	}
	p = normalizeOrganizationProfile(p)
	organizationProfile = p
	_ = saveJSON(sharedPath("organization_profile.json"), p)
	return p
}

func currentOrganizationName() string {
	if strings.TrimSpace(organizationProfile.OrganizationName) == "" {
		loadOrganizationProfile()
	}
	return organizationProfile.OrganizationName
}

func currentUnitLabel() string {
	if strings.TrimSpace(organizationProfile.UnitLabel) == "" {
		loadOrganizationProfile()
	}
	return organizationProfile.UnitLabel
}

func stateProductionUnit(s TechObjectState) string {
	if strings.TrimSpace(s.ProductionUnit) != "" {
		return strings.TrimSpace(s.ProductionUnit)
	}
	return strings.TrimSpace(s.TPP) // compatibility with archives created before neutral field was added
}
