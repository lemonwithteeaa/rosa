package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func finiteNumber(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func parseEconomicNumber(text, field string, required bool) (float64, error) {
	s := strings.TrimSpace(text)
	if s == "" {
		if required {
			return 0, fmt.Errorf("не заполнено поле «%s»", field)
		}
		return 0, nil
	}
	s = strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "", ",", ".").Replace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || !finiteNumber(v) {
		return 0, fmt.Errorf("поле «%s»: требуется конечное число", field)
	}
	return v, nil
}

func validateEconomicsHorizon(months float64) error {
	if !finiteNumber(months) || months < 1 || months > 60 || math.Trunc(months) != months {
		return fmt.Errorf("горизонт должен быть целым числом от 1 до 60 месяцев")
	}
	return nil
}

func economicsBindingChanged(bound, selected string) bool {
	return !strings.EqualFold(strings.TrimSpace(bound), strings.TrimSpace(selected))
}

func economicsCurrentMRP(current, historical string) string {
	if s := strings.TrimSpace(current); s != "" {
		return s
	}
	return strings.TrimSpace(historical)
}

func economicsFieldPosition(index int, contentWidth int32) (x, y, width int32) {
	width = (contentWidth - 32) / 3
	x = 242 + int32(index%3)*(width+16)
	y = 306 + int32(index/3)*72
	return
}
