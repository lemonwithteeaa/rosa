//go:build windows

package main

import (
	"fmt"
	"sort"
	"strings"
)

type laboratoryTrendPoint struct {
	Date  string
	Value float64
}

func laboratoryTrendKeys(groupCode string) []string {
	switch groupCode {
	case "corrosion":
		return []string{"corrosion_rate", "ph", "chlorides", "iron_total", "orp"}
	case "scale":
		return []string{"mineralization", "ca", "so4", "ba", "sr", "ph"}
	case "aspo":
		return []string{"paraffins", "asphaltenes", "viscosity", "pour_point"}
	case "solids":
		return []string{"suspended", "sand", "size_d50", "turbidity"}
	case "microbiology":
		return []string{"gmb", "sulfide", "orp"}
	case "gases":
		return []string{"co2", "h2s", "o2", "gas_factor"}
	}
	return nil
}

func laboratoryTrendPoints(group LabGroupDef, key, objectKey, beforeDate string, passes []LabPassRecord) []laboratoryTrendPoint {
	objectKey = normalizeLaboratoryObjectKey(objectKey)
	cutoff := parseDate(beforeDate)
	var out []laboratoryTrendPoint
	for _, p := range passes {
		if p.Status != labPassSavedStatus || normalizeLaboratoryObjectKey(p.ObjectKey) != objectKey {
			continue
		}
		d := parseDate(p.Date)
		if !cutoff.IsZero() && !d.IsZero() && d.After(cutoff) {
			continue
		}
		vals := canonicalizeLaboratoryValues(group, p.Groups[group.Code], p.Units[group.Code])
		v, ok := parseLabNumber(vals[key])
		if !ok {
			continue
		}
		out = append(out, laboratoryTrendPoint{Date: p.Date, Value: v})
	}
	sort.SliceStable(out, func(i, j int) bool { return parseDate(out[i].Date).Before(parseDate(out[j].Date)) })
	if len(out) > 5 {
		out = out[len(out)-5:]
	}
	return out
}

func laboratoryGroupTrendShort(group LabGroupDef, objectKey, beforeDate string, passes []LabPassRecord) string {
	var parts []string
	for _, key := range laboratoryTrendKeys(group.Code) {
		pts := laboratoryTrendPoints(group, key, objectKey, beforeDate, passes)
		if len(pts) < 2 {
			continue
		}
		first, last := pts[0].Value, pts[len(pts)-1].Value
		label := key
		unit := ""
		for _, f := range group.Fields {
			if f.Key == key {
				label, unit = laboratoryFieldDisplayLabel(f), f.Unit
				break
			}
		}
		if first == 0 {
			parts = append(parts, fmt.Sprintf("%s: %g → %g %s", label, first, last, unit))
		} else {
			parts = append(parts, fmt.Sprintf("%s: %+.0f %% (%g → %g %s)", label, (last-first)/first*100, first, last, unit))
		}
		if len(parts) >= 3 {
			break
		}
	}
	if len(parts) == 0 {
		return "ТРЕНД ИСТОРИИ\nНедостаточно сопоставимых сохранённых анализов для тренда."
	}
	return "ТРЕНД ИСТОРИИ\nПоследние сопоставимые точки: " + strings.Join(parts, "; ") + ". Тренд описывает наблюдение и не доказывает причину изменения."
}
