//go:build windows

package main

import (
	"fmt"
	"math"
)

// Coefficients copied from the unmodified USGS PHREEQC 3.8.6-17100
// database/phreeqc.dat. Analytical expressions override log_k/delta_h.
// This computes dissolution log K only; it is not a speciation/Pitzer engine.
const laboratoryTemperatureDatabase = "USGS PHREEQC 3.8.6-17100, phreeqc.dat"

var laboratoryTemperaturePhaseCoefficients = map[string][6]float64{
	"Кальцит":  {17.118, -0.046528, -3496, 0, 0, 0},
	"Гипс":     {93.7, 5.99e-3, -4e3, -35.019, 0, 0},
	"Барит":    {-282.43, -8.972e-2, 5822, 113.08, 0, 0},
	"Целестин": {-7.14, 6.11e-3, 75, 0, 0, -1.79e-5},
}

func laboratoryTemperatureLogK(mineral string, temperatureC float64) (float64, error) {
	// Deliberately limited for this screening route; do not extrapolate.
	if math.IsNaN(temperatureC) || math.IsInf(temperatureC, 0) || temperatureC < 0 || temperatureC > 100 {
		return 0, fmt.Errorf("Температура пробы должна быть в диапазоне 0–100 °C для температурного скрининга")
	}
	a, ok := laboratoryTemperaturePhaseCoefficients[mineral]
	if !ok {
		return 0, fmt.Errorf("Нет температурной зависимости для минерала %s", mineral)
	}
	t := temperatureC + 273.15
	return a[0] + a[1]*t + a[2]/t + a[3]*math.Log10(t) + a[4]/(t*t) + a[5]*t*t, nil
}
