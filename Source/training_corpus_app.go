//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type trainingWell struct {
	ID, ObjectKey, Field, Well, Layer, LiftMethod, GNO, PumpModel string
	Ordinal, PumpDepth                                            int
	LiquidRate, WaterCut, OilRate, NetbackRubPerM3                float64
	ReservoirPressure, BottomholePressure                         float64
	Failures                                                      []trainingFailure
	LabPasses                                                     []trainingLabPass
}
type trainingFailure struct {
	ID, Date, DateRu, CycleStart, CycleStartRu, RepairEnd, RepairEndRu                                          string
	EventNo, MRP, PumpDepth, ResultingMRP                                                                       int
	LiftMethod, Equipment, Brand, Size, Complication, Classification, AssociatedComplication, Cause, FailedUnit string
	MethodBefore, MethodAfter, DecisionType, LabGroup, LabPassID, Investigation, Commission, Comment            string
}
type trainingLabPass struct{ PassID, Date, DateRu, FailureID, Complication, GroupFocus, Conclusion, Status string }
type trainingCorpus struct {
	SchemaVersion          int
	Title                  string
	Synthetic              bool
	PeriodStart, PeriodEnd string
	Wells                  []trainingWell
}

var trainingCorpusOnce sync.Once
var trainingCorpusValue trainingCorpus
var trainingCorpusErr error

func loadTrainingCorpus() (trainingCorpus, error) {
	trainingCorpusOnce.Do(func() {
		trainingCorpusErr = json.Unmarshal(trainingCorpusFile, &trainingCorpusValue)
		if trainingCorpusErr == nil && (!trainingCorpusValue.Synthetic || len(trainingCorpusValue.Wells) != 20) {
			trainingCorpusErr = fmt.Errorf("учебный фонд должен содержать 20 скважин")
		}
	})
	return trainingCorpusValue, trainingCorpusErr
}
func trainingWellByOrdinal(n int) (trainingWell, bool) {
	c, e := loadTrainingCorpus()
	if e != nil || n < 1 || n > len(c.Wells) {
		return trainingWell{}, false
	}
	return c.Wells[n-1], true
}
func trainingWellForObject(key string) (trainingWell, bool) {
	c, e := loadTrainingCorpus()
	if e != nil {
		return trainingWell{}, false
	}
	for _, w := range c.Wells {
		if laboratoryObjectKeysEquivalent(w.ObjectKey, key) || strings.EqualFold(strings.TrimSpace(w.ID), strings.TrimSpace(key)) {
			return w, true
		}
	}
	return trainingWell{}, false
}

func canonicalTrainingPassIDs() map[string]struct{} {
	ids := make(map[string]struct{}, 180)
	corpus, err := loadTrainingCorpus()
	if err != nil {
		return ids
	}
	for _, well := range corpus.Wells {
		for _, pass := range well.LabPasses {
			ids[strings.ToUpper(strings.TrimSpace(pass.PassID))] = struct{}{}
		}
	}
	return ids
}

func trainingFailureDate(v string) time.Time {
	for _, layout := range []string{"2006-01-02", "02.01.2006"} {
		if t, e := time.ParseInLocation(layout, v, time.Local); e == nil {
			return t
		}
	}
	return time.Time{}
}
func trainingLabPassRecord(w trainingWell, e trainingFailure, p trainingLabPass) LabPassRecord {
	groups := map[string]map[string]string{}
	units := map[string]map[string]string{}
	for _, g := range labGroups {
		if g.Code == "overall" {
			continue
		}
		groups[g.Code] = map[string]string{}
		units[g.Code] = map[string]string{}
		for _, f := range g.Fields {
			groups[g.Code][f.Key] = teachingLabValue(f.Key, f.Unit, w, e)
			units[g.Code][f.Key] = f.Unit
		}
	}
	return LabPassRecord{PassID: p.PassID, ObjectKey: w.ObjectKey, Date: p.Date, Source: "Учебный набор РОСА · синтетические данные", Comment: "Значения созданы для обучения, не являются лабораторными измерениями.", Author: "Учебный контур", Status: labPassSavedStatus, Groups: groups, Units: units, ScaleModel: "davies", SharedValues: map[string]string{"watercut": strconv.FormatFloat(w.WaterCut, 'f', 0, 64), "liquid_rate_m3_day": strconv.FormatFloat(w.LiquidRate, 'f', 1, 64), "oil_rate_t_day": strconv.FormatFloat(w.OilRate, 'f', 1, 64)}, Conclusion: p.Conclusion, CreatedAt: p.Date, UpdatedAt: p.Date}
}
func teachingLabValue(key, unit string, w trainingWell, e trainingFailure) string {
	n, j := w.Ordinal, e.EventNo
	switch key {
	case "ph":
		return fmt.Sprintf("%.2f", 6.1+float64((n+j)%9)*.13)
	case "ca":
		return strconv.Itoa(280 + (n*37+j*61)%820)
	case "mg":
		return strconv.Itoa(60 + (n*19+j*31)%260)
	case "na_k":
		return strconv.Itoa(9000 + (n*317+j*541)%19000)
	case "so4":
		return strconv.Itoa(240 + (n*23+j*47)%1300)
	case "hco3":
		return strconv.Itoa(180 + (n*29+j*43)%980)
	case "cl", "chlorides":
		return strconv.Itoa(9000 + (n*911+j*1301)%33000)
	case "mineralization":
		return fmt.Sprintf("%.1f", 28+float64((n*13+j*17)%420)/10)
	case "corrosion_rate", "corrosion_rate_without":
		return fmt.Sprintf("%.2f", .08+float64((n*7+j*11)%59)/100)
	case "corrosion_rate_with":
		return fmt.Sprintf("%.2f", .03+float64((n*5+j*7)%20)/100)
	case "iron_total", "iron":
		return fmt.Sprintf("%.1f", 1.2+float64((n*11+j*17)%140)/10)
	case "paraffins":
		return fmt.Sprintf("%.1f", 3+float64((n*7+j*13)%105)/10)
	case "resins":
		return fmt.Sprintf("%.1f", 5+float64((n*9+j*11)%115)/10)
	case "asphaltenes":
		return fmt.Sprintf("%.1f", 1+float64((n*5+j*7)%49)/10)
	case "water":
		return strconv.Itoa(int(w.WaterCut))
	case "srb", "apb", "biofilm", "iron_bacteria":
		return []string{"Не выявлено", "Низкий уровень", "Выявлено · учебная оценка"}[(n+j)%3]
	case "gmb", "qpcr_srb", "qpcr_apb":
		return strconv.Itoa(120 + (n*47+j*89)%7800)
	case "sulfide", "h2s":
		return fmt.Sprintf("%.2f", float64((n*11+j*17)%420)/100)
	case "co2":
		return strconv.Itoa(18 + (n*29+j*41)%410)
	case "o2":
		return fmt.Sprintf("%.2f", float64((n*3+j*7)%28)/100)
	}
	if strings.Contains(strings.ToLower(unit), "результат") || strings.Contains(strings.ToLower(unit), "текст") || strings.Contains(strings.ToLower(unit), "протокол") {
		return fmt.Sprintf("Учебное значение · %s · %s", e.Complication, w.ID)
	}
	seed := n*23 + j*17 + len(key)*11
	switch {
	case strings.Contains(unit, "%"):
		return fmt.Sprintf("%.1f", float64(seed%760)/10)
	case strings.Contains(unit, "°C"):
		return strconv.Itoa(25 + seed%60)
	case strings.Contains(strings.ToLower(unit), "мпа"):
		return fmt.Sprintf("%.2f", .2+float64(seed%150)/10)
	case strings.Contains(unit, "мг"):
		return strconv.Itoa(2 + seed%9000)
	case strings.Contains(unit, "кг/м"):
		return strconv.Itoa(810 + seed%250)
	}
	return strconv.Itoa(1 + seed%1000)
}
