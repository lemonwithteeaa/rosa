//go:build windows

package main

import "fmt"

type laboratoryChartThreshold struct {
	value float64
	below bool
}

// Working screening boundaries already used by laboratory conclusions.
func laboratoryMetricThreshold(group, key string) (laboratoryChartThreshold, bool) {
	switch group + "/" + key {
	case "corrosion/ph":
		return laboratoryChartThreshold{6.5, true}, true
	case "corrosion/chlorides":
		return laboratoryChartThreshold{10000, false}, true
	case "corrosion/h2s", "gases/h2s":
		return laboratoryChartThreshold{0, false}, true
	case "corrosion/corrosion_rate":
		return laboratoryChartThreshold{0.1, false}, true
	case "scale/ca":
		return laboratoryChartThreshold{2000, false}, true
	case "scale/hco3":
		return laboratoryChartThreshold{500, false}, true
	case "corrosion/co2":
		return laboratoryChartThreshold{50, false}, true
	case "gases/co2":
		return laboratoryChartThreshold{50, false}, true
	case "corrosion/o2", "gases/o2":
		return laboratoryChartThreshold{0.1, false}, true
	case "scale/mg":
		return laboratoryChartThreshold{500, false}, true
	case "scale/na":
		return laboratoryChartThreshold{10000, false}, true
	case "scale/k":
		return laboratoryChartThreshold{500, false}, true
	case "scale/ba":
		return laboratoryChartThreshold{10, false}, true
	case "scale/sr":
		return laboratoryChartThreshold{1000, false}, true
	case "scale/co3":
		return laboratoryChartThreshold{50, false}, true
	case "scale/so4":
		return laboratoryChartThreshold{500, false}, true
	case "aspo/paraffins":
		return laboratoryChartThreshold{3, false}, true
	case "aspo/viscosity":
		return laboratoryChartThreshold{50, false}, true
	case "solids/suspended":
		return laboratoryChartThreshold{100, false}, true
	}
	return laboratoryChartThreshold{}, false
}

func (t laboratoryChartThreshold) exceeded(value float64) bool {
	if t.below {
		return value < t.value
	}
	return value > t.value
}

func (t laboratoryChartThreshold) caption(unit string) string {
	direction := "выше"
	if t.below {
		direction = "ниже"
	}
	return fmt.Sprintf("Порог внимания: %s %.3g %s", direction, t.value, unit)
}
