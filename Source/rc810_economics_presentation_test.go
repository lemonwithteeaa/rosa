//go:build windows

package main

import "testing"

func TestEconomicsFundRussianCountForms(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "объектов"}, {1, "объект"}, {2, "объекта"}, {5, "объектов"},
		{11, "объектов"}, {14, "объектов"}, {21, "объект"}, {23, "объекта"}, {111, "объектов"},
	} {
		if got := economicsFundCountWord(tc.n, "объект", "объекта", "объектов"); got != tc.want {
			t.Errorf("count %d: got %q; want %q", tc.n, got, tc.want)
		}
	}
}

func TestEconomicsMetricUnits(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{115000, "115 тыс. руб."}, {283500, "283,5 тыс. руб."}, {1000000, "1,00 млн руб."},
	}
	for _, tc := range cases {
		if got := econMetricRubThousands(tc.in); got != tc.want {
			t.Errorf("%.1f -> %q, want %q", tc.in, got, tc.want)
		}
	}
}
