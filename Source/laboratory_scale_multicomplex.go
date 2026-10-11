//go:build windows

package main

import (
	"fmt"
	"math"
	"strings"
)

const laboratoryMultiComplexBasis = "USGS PHREEQC 3.8.6-17100/phreeqc.dat; 25 °C; Davies; I≤0,1; a(H₂O)=1; γ нейтральных форм=1"
const laboratoryMolalWaterActivityBasis = "Для моляльного маршрута a(H₂O)=1−0,017·Σm по сумме всех растворённых частиц; это формула стандартного не-Pitzer/SIT ядра USGS PHREEQC."
const laboratoryBaSrComplexBasis = "Для Ba/Sr учтены сульфатные, карбонатные, гидрокарбонатные и гидроксокомплексы по той же базе PHREEQC."

type laboratoryMultiComplexSolution struct {
	Forms                                                                                                                                          laboratoryCarbonForms
	Ca, Mg, Ba, Sr, SO4, CaCO3, CaHCO3, CaSO4, CaOH, MgCO3, MgHCO3, MgSO4, MgSO42, MgOH, BaSO4, SrSO4, BaCO3, BaHCO3, BaOH, SrCO3, SrHCO3, SrOH, I float64
	CalciteSI, GypsumSI, BariteSI, CelestiteSI                                                                                                     float64
	WaterActivity                                                                                                                                  float64
	Iterations                                                                                                                                     int
}

func laboratoryMultiComplexSolve(ph, cat, mgt, ct, st, background float64) (laboratoryMultiComplexSolution, error) {
	return laboratoryMultiComplexSolveExtended(ph, cat, mgt, 0, 0, ct, st, background)
}

func laboratoryMultiComplexSolveExtended(ph, cat, mgt, bat, srt, ct, st, background float64) (laboratoryMultiComplexSolution, error) {
	return laboratoryMultiComplexSolveCore(ph, cat, mgt, bat, srt, ct, st, background, 2*background, false)
}

func laboratoryMultiComplexSolveExtendedMolal(ph, cat, mgt, bat, srt, ct, st, background, backgroundSpecies float64) (laboratoryMultiComplexSolution, error) {
	return laboratoryMultiComplexSolveCore(ph, cat, mgt, bat, srt, ct, st, background, backgroundSpecies, true)
}

func laboratoryMultiComplexSolveCore(ph, cat, mgt, bat, srt, ct, st, background, backgroundSpecies float64, calculateWaterActivity bool) (laboratoryMultiComplexSolution, error) {
	for _, x := range []float64{ph, cat, mgt, bat, srt, ct, st, background, backgroundSpecies} {
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S нужны конечные неотрицательные данные")
		}
	}
	if ph > 14 || cat <= 0 || ct <= 0 || cat > 10 || mgt > 10 || bat > 10 || srt > 10 || ct > 10 || st > 10 {
		return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S нужны pH 0–14, положительные общие Ca/C и концентрации не выше 10 моль/л")
	}
	t := 298.15
	assoc, _ := laboratoryCarbonateAssociationLogK(25)
	ideal, _ := laboratoryCarbonFormsFromPH(25, ph, 1)
	caK0 := math.Pow(10, -1228.732-.29944*t+35512.75/t+485.818*math.Log10(t))
	caK1 := math.Pow(10, -6.009+.03377*t+2044/t)
	mgK0 := math.Pow(10, .991+.00667*t)
	mgK1 := math.Pow(10, 48.6721+.03252849*t-2614.335/t-18.00263*math.Log10(t)+563713.9/(t*t))
	mgKS := math.Pow(10, .00964*t-136/t)
	// SO4-2 + MgSO4(aq) = Mg(SO4)2-2. The reactant and product have
	// the same charge, so their Davies factors cancel; MgSO4 is neutral.
	mgKS2 := math.Pow(10, -.00151*t+86040/(t*t))
	baKS := math.Pow(10, 2.7)
	srKS := math.Pow(10, 2.29)
	i := math.Min(.1, background+2*(cat+mgt+bat+srt+st)+.5*ct)
	aw := 1.0
	for step := 1; step <= 150; step++ {
		g1 := laboratoryCarbonDaviesGamma(1, i)
		g2 := laboratoryCarbonDaviesGamma(2, i)
		carbon := g1 * math.Pow(10, ph-assoc)
		qca0 := caK0 * g2 * carbon
		qca1 := caK1 * g2 * math.Pow(10, -assoc)
		qmg0 := mgK0 * g2 * carbon
		qmg1 := mgK1 * g2 * math.Pow(10, -assoc)
		qca := qca0 + qca1
		qmg := qmg0 + qmg1
		rca := math.Pow(10, 2.25) * g2 * g2
		rmg := mgKS * g2 * g2
		oca := math.Pow(10, ph-12.78) * g2 / g1 * aw
		omg := math.Pow(10, ph-11.44) * g2 / g1 * aw
		qba0 := math.Pow(10, 2.71) * g2 * carbon
		qba1 := math.Pow(10, .982) * g2
		qsr0 := math.Pow(10, 2.81) * g2 * carbon
		qsr1 := math.Pow(10, 11.509) * g2 * math.Pow(10, -assoc)
		qba := qba0 + qba1
		qsr := qsr0 + qsr1
		oba := math.Pow(10, ph-13.47) * g2 / g1 * aw
		osr := math.Pow(10, ph-13.29) * g2 / g1 * aw
		d := 1 + ideal.CO3*g1/g2 + ideal.CO2*g1/aw
		hc := ct / d
		sf := st
		ca, mg, ba, sr := 0.0, 0.0, 0.0, 0.0
		closed := false
		for inner := 0; inner < 300; inner++ {
			// Coordinate bisection preserves positivity and avoids logarithms of zero sulfate.
			lo, hi := 0.0, ct/d
			for n := 0; n < 70; n++ {
				b := .5 * (lo + hi)
				c := cat / (1 + b*qca + sf*rca + oca)
				m := mgt / (1 + b*qmg + sf*rmg + sf*sf*rmg*mgKS2 + omg)
				ba0 := bat / (1 + b*qba + sf*baKS*g2*g2 + oba)
				sr0 := srt / (1 + b*qsr + sf*srKS*g2*g2 + osr)
				if b*(d+c*qca+m*qmg+ba0*qba+sr0*qsr) > ct {
					hi = b
				} else {
					lo = b
				}
			}
			hc = .5 * (lo + hi)
			lo, hi = 0, st
			for n := 0; n < 70; n++ {
				s := .5 * (lo + hi)
				c := cat / (1 + hc*qca + s*rca + oca)
				m := mgt / (1 + hc*qmg + s*rmg + s*s*rmg*mgKS2 + omg)
				b := bat / (1 + hc*qba + s*baKS*g2*g2 + oba)
				r := srt / (1 + hc*qsr + s*srKS*g2*g2 + osr)
				if s*(1+c*rca+m*rmg+2*m*s*rmg*mgKS2+b*baKS*g2*g2+r*srKS*g2*g2) > st {
					hi = s
				} else {
					lo = s
				}
			}
			sf = .5 * (lo + hi)
			ca = cat / (1 + hc*qca + sf*rca + oca)
			mg = mgt / (1 + hc*qmg + sf*rmg + sf*sf*rmg*mgKS2 + omg)
			ba = bat / (1 + hc*qba + sf*baKS*g2*g2 + oba)
			sr = srt / (1 + hc*qsr + sf*srKS*g2*g2 + osr)
			cError := math.Abs(hc*(d+ca*qca+mg*qmg+ba*qba+sr*qsr)/ct - 1)
			sError := 0.0
			if st > 0 {
				sError = math.Abs(sf*(1+ca*rca+mg*rmg+2*mg*sf*rmg*mgKS2+ba*baKS*g2*g2+sr*srKS*g2*g2)/st - 1)
			}
			if cError < 1e-11 && sError < 1e-11 {
				closed = true
				break
			}
		}
		if !closed {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S не замкнуты внутренние балансы")
		}
		f := ideal
		f.HCO3 = hc
		f.CO3 = hc * ideal.CO3 * g1 / g2
		f.CO2 = hc * ideal.CO2 * g1 / aw
		f.Hydrogen = ideal.Hydrogen / g1
		f.Hydroxide = ideal.Hydroxide * aw / g1
		f.TotalCarbon = f.CO2 + hc + f.CO3
		s := laboratoryMultiComplexSolution{Forms: f, Ca: ca, Mg: mg, Ba: ba, Sr: sr, SO4: sf, CaCO3: ca * hc * qca0, CaHCO3: ca * hc * qca1, CaSO4: ca * sf * rca, CaOH: ca * oca, MgCO3: mg * hc * qmg0, MgHCO3: mg * hc * qmg1, MgSO4: mg * sf * rmg, MgSO42: mg * sf * sf * rmg * mgKS2, MgOH: mg * omg, BaSO4: ba * sf * baKS * g2 * g2, SrSO4: sr * sf * srKS * g2 * g2, BaCO3: ba * hc * qba0, BaHCO3: ba * hc * qba1, BaOH: ba * oba, SrCO3: sr * hc * qsr0, SrHCO3: sr * hc * qsr1, SrOH: sr * osr, I: i, WaterActivity: aw, Iterations: step}
		s.Forms.Alkalinity = hc + 2*f.CO3 + f.Hydroxide - f.Hydrogen + 2*(s.CaCO3+s.MgCO3+s.BaCO3+s.SrCO3) + s.CaHCO3 + s.MgHCO3 + s.BaHCO3 + s.SrHCO3 + s.CaOH + s.MgOH + s.BaOH + s.SrOH
		target := background + .5*(4*(ca+mg+ba+sr+sf+s.MgSO42)+hc+4*f.CO3+f.Hydrogen+f.Hydroxide+s.CaHCO3+s.MgHCO3+s.BaHCO3+s.SrHCO3+s.CaOH+s.MgOH+s.BaOH+s.SrOH)
		if math.IsNaN(target) || math.IsInf(target, 0) || target > .1 {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S I выше 0,1 моль/л либо произошло переполнение")
		}
		awTarget := 1.0
		if calculateWaterActivity {
			species := backgroundSpecies + ca + mg + ba + sr + sf + f.CO2 + hc + f.CO3 + f.Hydrogen + f.Hydroxide + s.CaCO3 + s.CaHCO3 + s.CaSO4 + s.CaOH + s.MgCO3 + s.MgHCO3 + s.MgSO4 + s.MgSO42 + s.MgOH + s.BaSO4 + s.BaCO3 + s.BaHCO3 + s.BaOH + s.SrSO4 + s.SrCO3 + s.SrHCO3 + s.SrOH
			awTarget = 1 - .017*species
			if awTarget <= 0 || awTarget > 1 || math.IsNaN(awTarget) || math.IsInf(awTarget, 0) {
				return laboratoryMultiComplexSolution{}, fmt.Errorf("Для моляльного равновесия недопустима расчётная активность воды")
			}
		}
		if math.Abs(target-i) <= 1e-12 && math.Abs(awTarget-aw) <= 1e-12 {
			s.WaterActivity = awTarget
			k, _ := laboratoryTemperatureLogK("Кальцит", 25)
			s.CalciteSI = math.Log10(ca*g2) + math.Log10(f.CO3*g2) - k
			if sf > 0 {
				k, _ = laboratoryTemperatureLogK("Гипс", 25)
				s.GypsumSI = math.Log10(ca*g2) + math.Log10(sf*g2) + 2*math.Log10(awTarget) - k
				if ba > 0 {
					k, _ = laboratoryTemperatureLogK("Барит", 25)
					s.BariteSI = math.Log10(ba*g2) + math.Log10(sf*g2) - k
				}
				if sr > 0 {
					k, _ = laboratoryTemperatureLogK("Целестин", 25)
					s.CelestiteSI = math.Log10(sr*g2) + math.Log10(sf*g2) - k
				}
			}
			return s, nil
		}
		i = .5 * (i + target)
		aw = .5 * (aw + awTarget)
	}
	return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S не достигнута сходимость")
}
func laboratoryMultiComplexFromValues(v map[string]string) (laboratoryMultiComplexSolution, error) {
	return laboratoryMultiComplexFromValuesMode(v, false)
}

func laboratoryMultiComplexFromValuesMolal(v map[string]string) (laboratoryMultiComplexSolution, error) {
	return laboratoryMultiComplexFromValuesMode(v, true)
}

func laboratoryMultiComplexFromValuesMode(v map[string]string, molalWaterActivity bool) (laboratoryMultiComplexSolution, error) {
	t, ok := parseLabNumber(v["sample_temp"])
	if !ok || math.Abs(t-25) > .001 {
		return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S требуется 25 °C")
	}
	if strings.TrimSpace(v["ph_measurement_temp"]) != "" {
		mt, ok := parseLabNumber(v["ph_measurement_temp"])
		if !ok || math.Abs(mt-t) > .5 {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S не согласована температура измерения pH")
		}
	}
	ph, ok := parseLabNumber(v["ph"])
	if !ok {
		return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S нужен pH")
	}
	amounts := map[string]float64{}
	for _, ion := range []struct {
		key, label string
		mass       float64
	}{{"ca_total", "общий кальций", 40.078}, {"mg_total", "общий магний", 24.305}, {"tic", "общий неорганический углерод", 12.0111}, {"so4_total", "общий сульфат как SO₄", 96.06}, {"na", "отдельный натрий", 22.99}, {"k", "отдельный калий", 39.0983}, {"cl", "хлорид", 35.45}} {
		c, ok := scaleIonMolar(v, ion.key, ion.mass)
		if !ok {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S нужен %s; явный нуль допустим, кроме Ca/C", ion.label)
		}
		amounts[ion.key] = c
	}
	background := .5 * (amounts["na"] + amounts["k"] + amounts["cl"])
	backgroundSpecies := amounts["na"] + amounts["k"] + amounts["cl"]
	ba, sr := 0.0, 0.0
	for _, ion := range []struct {
		key  string
		mass float64
	}{{"ba", 137.327}, {"sr", 87.62}} {
		if strings.TrimSpace(v[ion.key]) == "" {
			continue
		}
		x, ok := scaleIonMolar(v, ion.key, ion.mass)
		if !ok {
			return laboratoryMultiComplexSolution{}, fmt.Errorf("Для равновесия Ca/Mg/C/S некорректен анализ %s", ion.key)
		}
		if ion.key == "ba" {
			ba = x
		} else {
			sr = x
		}
	}
	if molalWaterActivity {
		return laboratoryMultiComplexSolveExtendedMolal(ph, amounts["ca_total"], amounts["mg_total"], ba, sr, amounts["tic"], amounts["so4_total"], background, backgroundSpecies)
	}
	return laboratoryMultiComplexSolveExtended(ph, amounts["ca_total"], amounts["mg_total"], ba, sr, amounts["tic"], amounts["so4_total"], background)
}
func laboratoryMultiComplexText(v map[string]string) string {
	s, e := laboratoryMultiComplexFromValues(v)
	if e != nil {
		return e.Error()
	}
	return fmt.Sprintf("Свободные ионы: Ca²⁺ %.6g мг Ca/л; Mg²⁺ %.6g мг Mg/л; Ba²⁺ %.6g мг Ba/л; Sr²⁺ %.6g мг Sr/л; SO₄²⁻ %.6g мг SO₄/л. Комплексы, моль/л: CaCO₃(aq) %.6g; CaHCO₃⁺ %.6g; CaSO₄(aq) %.6g; CaOH⁺ %.6g; MgCO₃(aq) %.6g; MgHCO₃⁺ %.6g; MgSO₄(aq) %.6g; Mg(SO₄)₂²⁻ %.6g; MgOH⁺ %.6g; BaSO₄(aq) %.6g; BaCO₃(aq) %.6g; BaHCO₃⁺ %.6g; BaOH⁺ %.6g; SrSO₄(aq) %.6g; SrCO₃(aq) %.6g; SrHCO₃⁺ %.6g; SrOH⁺ %.6g. Балансы общего Ca/Mg/Ba/Sr/C/S замкнуты расчётом; общий C является входом. Щёлочность %.6g мг/л как CaCO₃; I %.8g моль/л; итераций %d. Исходные значения сохранены.", s.Ca*40078, s.Mg*24305, s.Ba*137327, s.Sr*87620, s.SO4*96060, s.CaCO3, s.CaHCO3, s.CaSO4, s.CaOH, s.MgCO3, s.MgHCO3, s.MgSO4, s.MgSO42, s.MgOH, s.BaSO4, s.BaCO3, s.BaHCO3, s.BaOH, s.SrSO4, s.SrCO3, s.SrHCO3, s.SrOH, s.Forms.Alkalinity*50043.45, s.I, s.Iterations)
}
func laboratoryMultiComplexResults(v map[string]string) ([]laboratorySaturationResult, float64, []string) {
	s, e := laboratoryMultiComplexFromValues(v)
	if e != nil {
		return nil, 0, []string{e.Error()}
	}
	w := []string{"Сокращённое равновесие Ca/Mg/Ba/Sr/C/S, семнадцать комплексов. База: " + laboratoryMultiComplexBasis + ". " + laboratoryBaSrComplexBasis + " Давление/плотность и газовая фаза не учтены; это не полный PHREEQC/Питцер."}
	if strings.TrimSpace(v["ph_measurement_temp"]) == "" {
		w = append(w, "Температура измерения pH отсутствует; подтвердите 25 °C.")
	}
	// TIC is a balance input here, not an independent carbon check.
	measured, ok := parseLabNumber(v["alkalinity"])
	if !ok || measured < 0 {
		w = append(w, "Независимая сверка щёлочности не выполнена: анализ отсутствует или некорректен.")
	} else {
		calc := s.Forms.Alkalinity * 50043.45
		den := math.Max(math.Abs(calc), math.Abs(measured))
		if den > 0 && math.Abs(calc-measured)/den > .1 {
			w = append(w, fmt.Sprintf("Щёлочность: расчёт %.6g, анализ %.6g мг/л как CaCO₃; разность выше рабочего уровня 10 %%, не нормативного.", calc, measured))
		}
	}
	if b, ok := laboratoryScaleModelIonBalance(v, "multicomplex"); ok && math.Abs(b) > 5 {
		w = append(w, fmt.Sprintf("Невязка зарядов %+.2f %% выше рабочего контроля 5 %%.", b))
	}
	k, _ := laboratoryTemperatureLogK("Кальцит", 25)
	r := []laboratorySaturationResult{{Mineral: "Кальцит", Formula: "CaCO₃", SI: s.CalciteSI, Ratio: math.Pow(10, s.CalciteSI), TemperatureC: 25, LogK: k}}
	if s.SO4 > 0 {
		k, _ = laboratoryTemperatureLogK("Гипс", 25)
		r = append(r, laboratorySaturationResult{Mineral: "Гипс", Formula: "CaSO₄·2H₂O", SI: s.GypsumSI, Ratio: math.Pow(10, s.GypsumSI), TemperatureC: 25, LogK: k})
	}
	if s.Ba > 0 && s.SO4 > 0 {
		k, _ = laboratoryTemperatureLogK("Барит", 25)
		r = append(r, laboratorySaturationResult{Mineral: "Барит", Formula: "BaSO₄", SI: s.BariteSI, Ratio: math.Pow(10, s.BariteSI), TemperatureC: 25, LogK: k})
	}
	if s.Sr > 0 && s.SO4 > 0 {
		k, _ = laboratoryTemperatureLogK("Целестин", 25)
		r = append(r, laboratorySaturationResult{Mineral: "Целестин", Formula: "SrSO₄", SI: s.CelestiteSI, Ratio: math.Pow(10, s.CelestiteSI), TemperatureC: 25, LogK: k})
	}
	return r, s.I, w
}
