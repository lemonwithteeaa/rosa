//go:build windows

package main

import (
	"fmt"
	"math"
)

type laboratoryMixTotals struct {
	Ca, Mg, Ba, Sr, Carbon, Sulfate, Background, BackgroundSpecies float64
}

func laboratoryMixConservativeTotals(first, second laboratoryMixTotals, secondFraction float64) (laboratoryMixTotals, error) {
	if math.IsNaN(secondFraction) || math.IsInf(secondFraction, 0) || secondFraction < 0 || secondFraction > 1 {
		return laboratoryMixTotals{}, fmt.Errorf("Доля второй воды должна быть от 0 до 1")
	}
	values := []float64{first.Ca, first.Mg, first.Ba, first.Sr, first.Carbon, first.Sulfate, first.Background, first.BackgroundSpecies, second.Ca, second.Mg, second.Ba, second.Sr, second.Carbon, second.Sulfate, second.Background, second.BackgroundSpecies}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return laboratoryMixTotals{}, fmt.Errorf("Для смешения нужны конечные неотрицательные общие концентрации")
		}
	}
	firstFraction := 1 - secondFraction
	return laboratoryMixTotals{
		Ca:                firstFraction*first.Ca + secondFraction*second.Ca,
		Mg:                firstFraction*first.Mg + secondFraction*second.Mg,
		Ba:                firstFraction*first.Ba + secondFraction*second.Ba,
		Sr:                firstFraction*first.Sr + secondFraction*second.Sr,
		Carbon:            firstFraction*first.Carbon + secondFraction*second.Carbon,
		Sulfate:           firstFraction*first.Sulfate + secondFraction*second.Sulfate,
		Background:        firstFraction*first.Background + secondFraction*second.Background,
		BackgroundSpecies: firstFraction*first.BackgroundSpecies + secondFraction*second.BackgroundSpecies,
	}, nil
}

func laboratoryMultiComplexSolveMixedMolal(measuredMixedPH float64, first, second laboratoryMixTotals, secondFraction float64) (laboratoryMultiComplexSolution, error) {
	mixed, err := laboratoryMixConservativeTotals(first, second, secondFraction)
	if err != nil {
		return laboratoryMultiComplexSolution{}, err
	}
	backgroundSpecies := mixed.BackgroundSpecies
	if backgroundSpecies == 0 {
		backgroundSpecies = 2 * mixed.Background
	}
	return laboratoryMultiComplexSolveExtendedMolal(measuredMixedPH, mixed.Ca, mixed.Mg, mixed.Ba, mixed.Sr, mixed.Carbon, mixed.Sulfate, mixed.Background, backgroundSpecies)
}

func laboratoryMultiComplexSolveMixed(measuredMixedPH float64, first, second laboratoryMixTotals, secondFraction float64) (laboratoryMultiComplexSolution, error) {
	mixed, err := laboratoryMixConservativeTotals(first, second, secondFraction)
	if err != nil {
		return laboratoryMultiComplexSolution{}, err
	}
	// pH is intentionally not averaged: it is an activity measure, not a conservative total.
	// Until charge balance and all acid/base systems are represented, use measured mixed-water pH.
	return laboratoryMultiComplexSolveExtended(measuredMixedPH, mixed.Ca, mixed.Mg, mixed.Ba, mixed.Sr, mixed.Carbon, mixed.Sulfate, mixed.Background)
}
