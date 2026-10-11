//go:build windows

package main

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestDynamogramQualityGateBlocksGrossDefect(t *testing.T) {
	a := DynamogramAnalysis{
		Confidence:          0.72,
		MissingSourceRatio:  0.34,
		MeanSourceDeviation: 0.02,
		MaxSourceDeviation:  0.07,
		MaxBranchJump:       0.05,
	}
	gate, issues := dynamogramQualityGate(&a)
	if gate != dynQualityGateBlocked {
		t.Fatalf("expected blocked gate, got %q", gate)
	}
	if len(issues) == 0 || !strings.Contains(strings.Join(issues, " "), "не найден видимый штрих") {
		t.Fatalf("missing explainable blocker: %#v", issues)
	}
}

func TestDynamogramQualityGatePassesSupportedContour(t *testing.T) {
	a := DynamogramAnalysis{
		Confidence:          0.76,
		MissingSourceRatio:  0.03,
		MeanSourceDeviation: 0.008,
		MaxSourceDeviation:  0.025,
		MaxBranchJump:       0.035,
		FlatRunRatio:        0.08,
		FrameCaptureRatio:   0.04,
	}
	gate, issues := dynamogramQualityGate(&a)
	if gate != dynQualityGatePass || len(issues) != 0 {
		t.Fatalf("expected pass without issues, got %q %#v", gate, issues)
	}
}

func TestContourSourceQualityMeasuresVisibleStroke(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 90))
	white := color.RGBA{255, 255, 255, 255}
	black := color.RGBA{0, 0, 0, 255}
	for y := 0; y < 90; y++ {
		for x := 0; x < 120; x++ {
			img.SetRGBA(x, y, white)
		}
	}
	c := component{minX: 10, minY: 10, maxX: 109, maxY: 79}
	pts := make([]DynamogramPoint, 60)
	for i := range pts {
		xn := float64(i) / float64(len(pts)-1)
		pts[i] = DynamogramPoint{XNorm: xn, UpperNorm: 0.72, LowerNorm: 0.28}
		x := c.minX + int(xn*float64(c.maxX-c.minX))
		yu := c.minY + int((1-pts[i].UpperNorm)*float64(c.maxY-c.minY))
		yl := c.minY + int((1-pts[i].LowerNorm)*float64(c.maxY-c.minY))
		img.SetRGBA(x, yu, black)
		img.SetRGBA(x, yl, black)
	}
	mean, maxDev, missing, _, _, _ := contourSourceQuality(img, c, pts)
	if mean > 0.02 || maxDev > 0.03 || missing > 0.05 {
		t.Fatalf("visible contour should be supported: mean=%g max=%g missing=%g", mean, maxDev, missing)
	}
}
