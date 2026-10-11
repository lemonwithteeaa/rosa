//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func mixingInputs() map[string]string {
	v := map[string]string{"sample_temp": "25", "mix2_sample_temp": "25", "mix_temp": "25", "mix_ph_temp": "25", "mix_percent": "25", "mix_ph": "8.542807540421", "brine_density": "1001", "mix2_brine_density": "1001", "solute_mass": "1", "mix2_solute_mass": "1"}
	for _, item := range []struct {
		key        string
		mass, a, b float64
	}{
		{"ca_total", 40.078, .001, .003}, {"mg_total", 24.305, .002, .001}, {"tic", 12.0111, .003, .009}, {"so4_total", 96.06, .001, .005}, {"na", 22.99, .001, .002}, {"k", 39.0983, 0, 0}, {"cl", 35.45, .001, .002}, {"ba", 137.327, 1e-5, 3e-5}, {"sr", 87.62, 2e-5, 1e-5},
	} {
		v[item.key] = fmt.Sprint(item.a * item.mass * 1000)
		v["mix2_"+item.key] = fmt.Sprint(item.b * item.mass * 1000)
	}
	return v
}

func TestMixingInputMatchesActualPHREEQCReference(t *testing.T) {
	raw, err := os.ReadFile("reference/mixing/golden.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	numbers := make([]float64, len(fields))
	for i, text := range fields {
		numbers[i], err = strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatal(err)
		}
	}
	v := mixingInputs()
	v["mix_ph"] = fields[0]
	m, err := laboratoryWaterMixingFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	for name, pair := range map[string][2]float64{"Ca": {m.Solution.Ca, numbers[1]}, "Mg": {m.Solution.Mg, numbers[2]}, "SO4": {m.Solution.SO4, numbers[3]}, "Ba": {m.Solution.Ba, numbers[4]}, "Sr": {m.Solution.Sr, numbers[5]}, "HCO3": {m.Solution.Forms.HCO3, numbers[6]}} {
		if math.Abs(pair[0]/pair[1]-1) > 1e-5 {
			t.Fatalf("%s: %v", name, pair)
		}
	}
	if math.Abs(m.Solution.CalciteSI-numbers[29]) > 1e-5 || math.Abs(m.Solution.GypsumSI-numbers[30]) > 1e-5 || math.Abs(m.Solution.BariteSI-numbers[31]) > 1e-5 || math.Abs(m.Solution.CelestiteSI-numbers[32]) > 1e-5 || math.Abs(m.Solution.WaterActivity-math.Pow(10, numbers[33])) > laboratoryMixingWaterActivityReferenceTolerance {
		t.Fatalf("external reference residuals: calcite=%g gypsum=%g barite=%g celestite=%g water_activity=%g", m.Solution.CalciteSI-numbers[29], m.Solution.GypsumSI-numbers[30], m.Solution.BariteSI-numbers[31], m.Solution.CelestiteSI-numbers[32], m.Solution.WaterActivity-math.Pow(10, numbers[33]))
	}
}

func TestMixingWeightsBySolventMassAndPreservesMaterialBalances(t *testing.T) {
	v := mixingInputs()
	v["mix2_brine_density"] = "951"
	before := v["ca_total"]
	m, err := laboratoryWaterMixingFromValues(v)
	if err != nil {
		t.Fatal(err)
	}
	expectedMassFraction := .25 * .95 / (.75 + .25*.95)
	if math.Abs(m.SecondWaterMassFraction-expectedMassFraction) > 1e-14 {
		t.Fatal(m.SecondWaterMassFraction)
	}
	// For a notional pre-mixing batch of 0.75 L + 0.25 L, conserve moles
	// over total solvent mass; do not average normalized molalities by volume.
	expectedCa := (.75*.001 + .25*.003) / (.75 + .25*.95)
	if math.Abs(m.Totals.Ca-expectedCa) > 1e-14 {
		t.Fatal(m.Totals.Ca, expectedCa)
	}
	s := m.Solution
	for name, pair := range map[string][2]float64{
		"Ca": {s.Ca + s.CaCO3 + s.CaHCO3 + s.CaSO4 + s.CaOH, m.Totals.Ca},
		"Mg": {s.Mg + s.MgCO3 + s.MgHCO3 + s.MgSO4 + s.MgSO42 + s.MgOH, m.Totals.Mg},
		"C":  {s.Forms.TotalCarbon + s.CaCO3 + s.CaHCO3 + s.MgCO3 + s.MgHCO3 + s.BaCO3 + s.BaHCO3 + s.SrCO3 + s.SrHCO3, m.Totals.Carbon},
		"S":  {s.SO4 + s.CaSO4 + s.MgSO4 + 2*s.MgSO42 + s.BaSO4 + s.SrSO4, m.Totals.Sulfate},
	} {
		if math.Abs(pair[0]/pair[1]-1) > 1e-10 {
			t.Fatal(name, pair)
		}
	}
	if v["ca_total"] != before {
		t.Fatal("input mutated")
	}
}

func TestMixingRequiresMeasuredMixturePHAndAllRequiredData(t *testing.T) {
	for _, test := range []struct{ key, value string }{{"mix_ph", ""}, {"mix_ph", "NaN"}, {"mix_ph", "15"}, {"mix_percent", "0"}, {"mix_percent", "100"}, {"mix2_ca_total", ""}, {"mix2_brine_density", ""}, {"mix2_solute_mass", ""}, {"mix_ph_temp", "40"}, {"mix2_sample_temp", "20"}, {"mix_temp", "35"}} {
		v := mixingInputs()
		v[test.key] = test.value
		v["ph"] = "8" // First-water pH must not fill in for mixed pH.
		if _, err := laboratoryWaterMixingFromValues(v); err == nil {
			t.Fatalf("accepted %s=%q", test.key, test.value)
		}
		results, _, warnings := laboratorySaturationResults(v, "water_mixing")
		if len(results) != 0 || len(warnings) == 0 {
			t.Fatalf("failure did not reach selected-model status for %s", test.key)
		}
	}
}

func TestMixingScreenSeparatesInputsWithoutMutatingCatalog(t *testing.T) {
	group, _ := labGroupByCode("scale")
	firstLabel := group.Fields[0].Label
	mixed := laboratoryScaleInputGroup(group, "water_mixing")
	regular := laboratoryScaleInputGroup(group, "multicomplex_molal")
	if len(mixed.Fields) != 30 || len(regular.Fields) != 45 {
		t.Fatal(len(mixed.Fields), len(regular.Fields))
	}
	for _, field := range regular.Fields {
		if laboratoryIsMixingField(field.Key) {
			t.Fatal("mixing field leaked into regular screen")
		}
	}
	for _, field := range mixed.Fields {
		if field.Key == "ph" {
			t.Fatal("first-water pH offered as mixture pH")
		}
	}
	mixed.Fields[0].Label = "changed"
	if group.Fields[0].Label != firstLabel {
		t.Fatal("catalog mutated")
	}
	filled, total, _, _ := laboratoryDataCoverageForModel(group, mixingInputs(), "water_mixing")
	if filled != 24 || total != 24 {
		t.Fatal(filled, total)
	}
}

func TestMixingSurvivesPassJSONAndExplainsLimits(t *testing.T) {
	pass := LabPassRecord{ScaleModel: "water_mixing", Groups: map[string]map[string]string{"scale": mixingInputs()}, Units: map[string]map[string]string{"scale": {"mix2_ca_total": "мг/л"}}}
	raw, err := json.Marshal(pass)
	if err != nil {
		t.Fatal(err)
	}
	var restored LabPassRecord
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ScaleModel != pass.ScaleModel || restored.Groups["scale"]["mix_ph"] != pass.Groups["scale"]["mix_ph"] {
		t.Fatal("mixing inputs lost")
	}
	group, _ := labGroupByCode("scale")
	text := buildLaboratoryConclusionWithScaleModel(group, restored.Groups["scale"], restored.ScaleModel)
	for _, want := range []string{"СМЕШЕНИЕ ДВУХ ВОД", "Измеренный pH смеси", "pH смеси нужно измерить", "моль/кг воды", "не определяет количество и скорость", "ЧТО ЭТО ЗНАЧИТ ДЛЯ ТЕХНОЛОГА"} {
		if !strings.Contains(text, want) {
			t.Fatal("missing explanation", want)
		}
	}
	if strings.Contains(text, "числовых показателей для скрининга не найдено") {
		t.Fatal("computed mixing reported as missing numerical data")
	}
}

func TestMixingMissingBaSrIsNotMeasuredZero(t *testing.T) {
	v := mixingInputs()
	delete(v, "mix2_ba")
	delete(v, "mix2_sr")
	if _, err := laboratoryWaterMixingFromValues(v); err != nil {
		t.Fatal("optional analysis blocked mixing", err)
	}
	_, _, warnings := laboratoryWaterMixingResults(v)
	text := strings.Join(warnings, "\n")
	if !strings.Contains(text, "Не хватает анализа BA") || !strings.Contains(text, "Не хватает анализа SR") {
		t.Fatal(text)
	}
}

func TestMixingValidationUnitsAndScopeOfFields(t *testing.T) {
	group, _ := labGroupByCode("scale")
	mixed := laboratoryScaleInputGroup(group, "water_mixing")
	units := map[string]string{"mix2_ca_total": "г/л"}
	v := mixingInputs()
	v["mix2_ca_total"] = "0.120234"
	canonical := canonicalizeLaboratoryValues(mixed, v, units)
	n, ok := parseLabNumber(canonical["mix2_ca_total"])
	if !ok || math.Abs(n-120.234) > 1e-10 {
		t.Fatal(canonical["mix2_ca_total"])
	}
	for _, field := range mixed.Fields {
		if field.Key == "mix_ph" && validateLaboratoryValue("scale", field, "15").State != labValueError {
			t.Fatal("pH range was not enforced")
		}
	}
	if laboratoryIsMixingField("mixture_density") {
		t.Fatal("regular mixture density was mistaken for new mixing inputs")
	}
}
