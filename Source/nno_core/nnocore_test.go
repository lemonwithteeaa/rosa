package nnocore

import (
	"testing"
	"time"
)

func testMonth(year int, month time.Month) time.Time {
	return time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
}

func testMethodology() Methodology {
	return Methodology{
		ID: "NNO-TEST", Version: "1.0", ValidFrom: testMonth(2025, time.January), Approved: true, WindowMonths: 12,
		Rules: []Rule{
			{ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.6", Action: Include, Reason: "подтверждённый отказ"},
			{ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.12", Action: Exclude, Reason: "ГТМ"},
		},
	}
}

func TestRolling12SumsNumeratorAndDenominator(t *testing.T) {
	failures := []Failure{
		{ID: "e1", Group: "УЭЦН", Month: testMonth(2026, time.January), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.6", Confirmed: true, WorkedDays: 100, ProductionData: true},
		{ID: "e2", Group: "УЭЦН", Month: testMonth(2026, time.June), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.6", Confirmed: true, WorkedDays: 300, ProductionData: true},
		{ID: "e3", Group: "УЭЦН", Month: testMonth(2026, time.September), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.12", Confirmed: true, WorkedDays: 900, ProductionData: true},
		{ID: "old", Group: "УЭЦН", Month: testMonth(2025, time.September), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.6", Confirmed: true, WorkedDays: 1000, ProductionData: true},
	}
	result, err := Calculate(testMethodology(), failures, testMonth(2026, time.October), "УЭЦН")
	if err != nil || result.Failures != 2 || result.Days != 400 || result.NNO != 200 || len(result.Decisions) != 3 {
		t.Fatalf("got %+v, err=%v", result, err)
	}
}

func TestDraftMethodologyAndZeroDenominatorAreNotPublished(t *testing.T) {
	result, err := Calculate(testMethodology(), nil, testMonth(2026, time.October), "УШГН")
	if err != nil || result.Defined || result.Failures != 0 {
		t.Fatalf("got %+v, err=%v", result, err)
	}
	m := testMethodology()
	m.Approved = false
	if _, err = Calculate(m, nil, testMonth(2026, time.October), "УЭЦН"); err == nil {
		t.Fatal("unapproved methodology must fail")
	}
}

func TestUnknownOrUnconfirmedClassificationIsExcluded(t *testing.T) {
	failures := []Failure{
		{ID: "unknown", Group: "УЭЦН", Month: testMonth(2026, time.June), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "unknown", Confirmed: true, WorkedDays: 900, ProductionData: true},
		{ID: "pending", Group: "УЭЦН", Month: testMonth(2026, time.July), ClassifierVersion: "V10", Branch: "УЭЦН", Code: "3.6", WorkedDays: 700, ProductionData: true},
	}
	result, err := Calculate(testMethodology(), failures, testMonth(2026, time.October), "УЭЦН")
	if err != nil || result.Failures != 0 || result.Defined || result.Decisions[0].Included || result.Decisions[1].Included {
		t.Fatalf("got %+v, err=%v", result, err)
	}
}
