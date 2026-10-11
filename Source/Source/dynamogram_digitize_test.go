//go:build windows

package main

import (
	"image"
	"image/color"
	"testing"
)

func TestSuppressInteriorBranchExcursionsRemovesGridRectangle(t *testing.T) {
	values := make([]int, 240)
	for i := range values {
		values[i] = 92 + i/80
	}
	for i := 104; i < 128; i++ {
		values[i] = 38 // short inner edge of a table/grid rectangle
	}

	got := suppressInteriorBranchExcursions(values, 180)
	for i := 108; i < 124; i++ {
		if got[i] < 75 {
			t.Fatalf("grid excursion remained at x=%d: got y=%d", i, got[i])
		}
	}
}

func TestSuppressInteriorBranchExcursionsPreservesBroadWorkingBend(t *testing.T) {
	values := make([]int, 300)
	for i := range values {
		values[i] = 118
	}
	// A broad smooth bend is part of the working branch, not a grid artefact.
	for i := 85; i < 205; i++ {
		distance := i - 145
		values[i] = 72 + distance*distance/260
	}

	got := suppressInteriorBranchExcursions(values, 200)
	if got[145] > 88 {
		t.Fatalf("broad working bend was flattened: center y=%d", got[145])
	}
}

func TestDigitizeComponentKeepsMoreThan240Samples(t *testing.T) {
	c := component{minX: 0, minY: 20, maxX: 399, maxY: 160}
	for x := 0; x < 400; x++ {
		c.pixels = append(c.pixels, (40+x/40)*400+x, (140-x/50)*400+x)
	}
	points, _, _, _, _, _ := digitizeComponent(c, 400, 200)
	if len(points) != 400 {
		t.Fatalf("expected one point per source column, got %d", len(points))
	}
}

func TestDetectDynamogramPlotDoesNotCutAtLowerGridLine(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 500, 700))
	for y := 0; y < 700; y++ {
		for x := 0; x < 500; x++ {
			img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	black := color.RGBA{0, 0, 0, 255}
	for y := 300; y <= 600; y++ {
		img.SetRGBA(50, y, black)
		img.SetRGBA(450, y, black)
	}
	for x := 50; x <= 450; x++ {
		img.SetRGBA(x, 600, black)
	}
	// Strong intermediate grid line that the former 62% search mistook for
	// the plot baseline.
	for x := 50; x <= 450; x++ {
		img.SetRGBA(x, 510, black)
	}

	plot, ok := detectDynamogramPlot(img)
	if !ok {
		t.Fatal("synthetic QUANTOR-like plot frame was not detected")
	}
	if plot.Bounds().Dy() < 270 {
		t.Fatalf("plot was cut at the lower grid line: height=%d", plot.Bounds().Dy())
	}
}
