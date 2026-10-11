package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"reflect"
	"testing"
)

func TestEconomicNumberInput(t *testing.T) {
	for _, tc := range []struct {
		text     string
		required bool
		want     float64
		valid    bool
	}{
		{"12,5", true, 12.5, true}, {"1 200,75", true, 1200.75, true},
		{"1\u00a0200,75", true, 1200.75, true}, {"1\u202f200,75", true, 1200.75, true},
		{"", false, 0, true}, {"", true, 0, false}, {"0", true, 0, true},
		{"NaN", true, 0, false}, {"+Inf", true, 0, false}, {"-Inf", true, 0, false},
		{"1e309", true, 0, false}, {"abc", true, 0, false}, {"1,2,3", true, 0, false},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got, err := parseEconomicNumber(tc.text, "Показатель", tc.required)
			if (err == nil) != tc.valid || (tc.valid && got != tc.want) {
				t.Fatalf("got %v, %v; want %v, valid=%v", got, err, tc.want, tc.valid)
			}
		})
	}
}

func TestHorizonDoesNotTruncate(t *testing.T) {
	for _, n := range []float64{1, 36, 60} {
		if err := validateEconomicsHorizon(n); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []float64{0, -1, 36.5, 60.1, 61, math.NaN(), math.Inf(1)} {
		if err := validateEconomicsHorizon(n); err == nil {
			t.Errorf("accepted %v", n)
		}
	}
}

func TestEconomicsFormFitsClientWidth(t *testing.T) {
	for _, clientWidth := range []int32{1164, 1264, 1366, 1920} {
		contentWidth := clientWidth - 270
		for i := 0; i < 12; i++ {
			x, y, width := economicsFieldPosition(i, contentWidth)
			if width < 260 || x < 242 || x+width > clientWidth-28 || y+70 > 512 {
				t.Errorf("field %d outside form at width %d: %d,%d,%d", i, clientWidth, x, y, width)
			}
		}
	}
}

func TestEconomicsObjectBinding(t *testing.T) {
	if !economicsBindingChanged("скв1пластА", "скв1пластБ") {
		t.Fatal("different formations treated as same object")
	}
	if !economicsBindingChanged("скв1", "скв2") {
		t.Fatal("different wells treated as same object")
	}
	if economicsBindingChanged(" Скв1 ", "скв1") {
		t.Fatal("same object should preserve manual inputs")
	}
	if !economicsBindingChanged("скв1", "") {
		t.Fatal("cleared selection must invalidate binding")
	}
}

func TestCurrentTechRegimeHasPriority(t *testing.T) {
	for _, tc := range []struct{ current, history, want string }{
		{"1100", "900", "1100"}, {"", "900", "900"}, {"", "", ""}, {" 1200 ", "900", "1200"},
	} {
		if got := economicsCurrentMRP(tc.current, tc.history); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func validBaseInput() BaseEconomicsInput {
	return BaseEconomicsInput{CurrentMRPDays: 1000, MRPLowDays: 1200, MRPBaseDays: 1500,
		MRPHighDays: 1800, SelectedScenario: "Базовый", HorizonMonths: 24,
		RepairDurationDays: 5, RepairCostK: 1000, AnnualOPEXK: 100, OPEXInflationPct: 5, OneTimeMTRK: 50}
}

func TestBaseRejectsAllNonfiniteInputs(t *testing.T) {
	for i := 0; i < reflect.TypeOf(BaseEconomicsInput{}).NumField(); i++ {
		field := reflect.TypeOf(BaseEconomicsInput{}).Field(i)
		if field.Type.Kind() != reflect.Float64 {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			for _, n := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
				in := validBaseInput()
				reflect.ValueOf(&in).Elem().Field(i).SetFloat(n)
				if r := CalculateBaseEconomics(in); r.Status == "PASS" || len(r.Errors) == 0 {
					t.Fatalf("nonfinite value accepted: %+v", r)
				}
			}
		})
	}
}

func TestUnknownScenarioIsNotSilentlyBase(t *testing.T) {
	in := validBaseInput()
	in.SelectedScenario = "базовй"
	if r := CalculateBaseEconomics(in); r.Status == "PASS" {
		t.Fatal("typo accepted")
	}
}

func TestExistingCalculationChecks(t *testing.T) {
	if err := selfCheckBaseEconomics(); err != nil {
		t.Fatal(err)
	}
	if err := selfCheckExtendedEconomics(); err != nil {
		t.Fatal(err)
	}
	base := CalculateBaseEconomics(validBaseInput())
	if math.Abs(base.Selected.NetBenefitForHorizonK-(-11.6666666667)) > 1e-6 {
		t.Fatalf("baseline formula changed: %+v", base.Selected)
	}
}

// These are source wiring checks, not substitutes for a Windows UI test.
func TestWorkflowWiring(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"buildUI", "buildWorkflowPagesUI"}, {"resize", "layoutWorkflowPage"},
		{"wndProc", "handleWorkflowCommand"}, {"wndProc", "invalidateEconomicsForEdit"},
		{"refreshPage", "refreshTechParametersPage"}, {"refreshPage", "refreshEconomicsPage"},
		{"refreshPage", "refreshResultPage"},
	} {
		found := false
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != pair[0] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if id, ok := c.Fun.(*ast.Ident); ok && id.Name == pair[1] {
						found = true
					}
				}
				return true
			})
		}
		if !found {
			t.Errorf("missing %s -> %s", pair[0], pair[1])
		}
	}
}
