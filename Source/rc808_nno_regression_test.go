//go:build windows

package main

import (
	"testing"
	"time"
)

// Screen-level geometry is calculated once per layout, rather than drawing a
// graph over the detail table and hoping the OS clips the previous controls.
func TestRC808NNOThreeTableRows(t *testing.T) {
	for _, c := range []struct{ height, viewport int32 }{
		{864, 302}, {900, 342}, {1040, 458}, {1200, 458},
	} {
		chart, middle, detail := nnoFleetBandHeights(c.height, c.viewport)
		if chart < 185 || middle < 118 || detail < 146 {
			t.Errorf("height=%d viewport=%d -> chart=%d middle=%d detail=%d", c.height, c.viewport, chart, middle, detail)
		}
		// ListView contains a native header and >=3 data rows even if a short
		// desktop window requires using the existing scrolling buttons.
	}
}

func TestRC808TrainingPreviewRequiresTeachingSource(t *testing.T) {
	oldFrom, oldTo, oldSource := nnoFleetDateFrom, nnoFleetDateTo, nnoFleetDataSourceFilter
	oldData, oldFilter := nnoFleetData, nnoFleetFilter
	defer func() {
		nnoFleetDateFrom, nnoFleetDateTo = oldFrom, oldTo
		nnoFleetDataSourceFilter, nnoFleetData, nnoFleetFilter = oldSource, oldData, oldFilter
	}()
	nnoFleetData, nnoFleetFilter = 0, 0
	nnoFleetDataSourceFilter = ""
	nnoFleetDateFrom = time.Date(2025, 11, 1, 0, 0, 0, 0, time.Local)
	nnoFleetDateTo = time.Date(2026, 10, 10, 0, 0, 0, 0, time.Local)
	demo := FailureEvent{Date: "10.01.2026", Source: "Учебный набор РОСА", Equipment: "УЭЦН"}
	if !nnoShouldUseSyntheticPreview([]FailureEvent{demo}) {
		t.Fatal("training event is suppressed by prefilter")
	}
	real := FailureEvent{Date: "01.01.2026", Source: "Журнал отказов", Equipment: "УЭЦН"}
	if nnoShouldUseSyntheticPreview([]FailureEvent{demo, real}) {
		t.Fatal("preview would mix real and training events")
	}
	if nnoShouldUseSyntheticPreview([]FailureEvent{{Date: "01.10.2024", Source: demo.Source}}) {
		t.Fatal("training source outside selected period activated preview")
	}
}
