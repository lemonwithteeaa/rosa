package main

import (
	"math"
	"sort"
	"time"
)

// A completed, verified cycle is illustrative evidence, not a verified Ti
// contribution to the official fund-wide NNO-12 denominator/numerator.
type nnoCycleSample struct {
	Group string
	End   time.Time
	Days  float64
}
type nnoCycleStats struct {
	Count                          int
	Mean, Median, Min, Q1, Q3, Max float64
	WhiskerMin, WhiskerMax         float64
	Outliers                       int
	Defined                        bool
}

func nnoMonthFloor(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
func nnoSampleInWindow(s nnoCycleSample, from, to time.Time) bool {
	if s.End.IsZero() || s.Days <= 0 || math.IsNaN(s.Days) || math.IsInf(s.Days, 0) {
		return false
	}
	d := nnoMonthFloor(s.End)
	return !d.Before(nnoMonthFloor(from)) && !d.After(nnoMonthFloor(to))
}
func nnoPercentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	at := p * float64(len(sorted)-1)
	lo := int(math.Floor(at))
	hi := int(math.Ceil(at))
	return sorted[lo] + (sorted[hi]-sorted[lo])*(at-float64(lo))
}
func nnoCycleStatistics(samples []nnoCycleSample, group string, from, to time.Time) nnoCycleStats {
	values := make([]float64, 0)
	total := 0.0
	for _, s := range samples {
		if s.Group != group || !nnoSampleInWindow(s, from, to) {
			continue
		}
		values = append(values, s.Days)
		total += s.Days
	}
	if len(values) == 0 {
		return nnoCycleStats{}
	}
	sort.Float64s(values)
	q1, q3 := nnoPercentile(values, .25), nnoPercentile(values, .75)
	low, high := q1-1.5*(q3-q1), q3+1.5*(q3-q1)
	stats := nnoCycleStats{Count: len(values), Mean: total / float64(len(values)), Median: nnoPercentile(values, .5), Min: values[0], Q1: q1, Q3: q3, Max: values[len(values)-1], Defined: true}
	for _, value := range values {
		if value < low || value > high {
			stats.Outliers++
			continue
		}
		if stats.WhiskerMin == 0 {
			stats.WhiskerMin = value
		}
		stats.WhiskerMax = value
	}
	return stats
}
func nnoRollingCycleStats(samples []nnoCycleSample, group string, month time.Time) nnoCycleStats {
	last := nnoMonthFloor(month)
	return nnoCycleStatistics(samples, group, last.AddDate(0, -11, 0), last)
}
