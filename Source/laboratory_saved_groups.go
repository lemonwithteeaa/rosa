//go:build windows

package main

import (
	"sort"
	"strings"
	"time"
)

type laboratorySavedGroup struct {
	Name       string   `json:"name"`
	ObjectKeys []string `json:"object_keys"`
	UpdatedAt  string   `json:"updated_at"`
}

var laboratorySavedGroups []laboratorySavedGroup

func loadLaboratorySavedGroups() []laboratorySavedGroup {
	var groups []laboratorySavedGroup
	loadJSON(sharedPath("laboratory_saved_groups.json"), &groups)
	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	return groups
}

func saveLaboratorySavedGroup(name, rawKeys string) bool {
	name = strings.TrimSpace(name)
	keys := laboratorySelectedObjectKeysText(rawKeys)
	if name == "" || len(keys) == 0 {
		return false
	}
	groups := loadLaboratorySavedGroups()
	updated := false
	for i := range groups {
		if strings.EqualFold(strings.TrimSpace(groups[i].Name), name) {
			groups[i].ObjectKeys = keys
			groups[i].UpdatedAt = time.Now().Format(time.RFC3339)
			updated = true
			break
		}
	}
	if !updated {
		groups = append(groups, laboratorySavedGroup{
			Name:       name,
			ObjectKeys: keys,
			UpdatedAt:  time.Now().Format(time.RFC3339),
		})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return strings.ToLower(groups[i].Name) < strings.ToLower(groups[j].Name)
	})
	if saveJSON(sharedPath("laboratory_saved_groups.json"), groups) != nil {
		return false
	}
	laboratorySavedGroups = groups
	return true
}

func laboratorySavedGroupByIndex(index int) (laboratorySavedGroup, bool) {
	if index < 0 || index >= len(laboratorySavedGroups) {
		return laboratorySavedGroup{}, false
	}
	return laboratorySavedGroups[index], true
}

func laboratorySavedGroupText(g laboratorySavedGroup) string {
	return strings.Join(g.ObjectKeys, "; ")
}
