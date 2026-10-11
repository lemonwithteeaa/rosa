//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestDynamogramHypothesesHaveFourSourceChecks(t *testing.T) {
	base := DynamogramAnalysis{
		Confidence: 0.90, AreaRatio: 0.30, MeanSeparation: 0.25,
		LeftSeparation: 0.25, MiddleSeparation: 0.25, RightSeparation: 0.25,
		FillUniformity: 0.80, ClosureQuality: 0.90, Roughness: 0.01,
		Symmetry: 0.90, ReviewStatus: "confirmed",
	}
	type scenario struct {
		name  string
		card  DynamogramAnalysis
		title string
	}
	scenarios := []scenario{
		{"нормальная форма", base, "выраженный доминирующий дефект"},
	}
	narrow := base
	narrow.AreaRatio = 0.10
	scenarios = append(scenarios, scenario{"узкая петля", narrow, "узкая рабочая петля"})
	right := base
	right.RightSeparation = 0.10
	scenarios = append(scenarios, scenario{"неполное заполнение", right, "неполное заполнение насоса или газовое влияние"})
	left := base
	left.LeftSeparation = 0.10
	scenarios = append(scenarios, scenario{"начало хода", left, "нарушение формирования нагрузки в начале хода"})
	uneven := base
	uneven.FillUniformity = 0.40
	scenarios = append(scenarios, scenario{"неравномерность", uneven, "неравномерное заполнение"})
	leak := base
	leak.ClosureQuality = 0.40
	scenarios = append(scenarios, scenario{"клапаны", leak, "негерметичность клапанов"})
	friction := base
	friction.Roughness = 0.08
	scenarios = append(scenarios, scenario{"трение", friction, "повышенное механическое трение"})
	asymmetry := base
	asymmetry.Symmetry = 0.50
	scenarios = append(scenarios, scenario{"асимметрия", asymmetry, "асимметрия ветвей"})
	lowQuality := base
	lowQuality.Confidence = 0.40
	scenarios = append(scenarios, scenario{"качество", lowQuality, "качество исходника недостаточно"})

	markers := []string{
		"Предварительная гипотеза:", "Основание:",
		"- Техрежим:", "- Телеметрия:", "- Ремонт:", "- Лаборатория:",
	}
	for _, tc := range scenarios {
		_, hypotheses := dynamogramOperatingAssessment(&tc.card)
		if len(hypotheses) == 0 {
			t.Fatalf("scenario %s has no hypothesis", tc.name)
		}
		joined := strings.Join(hypotheses, "\n")
		if !strings.Contains(joined, tc.title) {
			t.Fatalf("scenario %s has no title %q: %s", tc.name, tc.title, joined)
		}
		for _, hypothesis := range hypotheses {
			for _, marker := range markers {
				if !strings.Contains(hypothesis, marker) {
					t.Fatalf("scenario %s hypothesis has no %q: %s", tc.name, marker, hypothesis)
				}
			}
		}
	}
}

func TestRejectedDynamogramDoesNotCreateHypotheses(t *testing.T) {
	a := DynamogramAnalysis{Confidence: 0.90, ReviewStatus: "rejected"}
	status, messages := dynamogramOperatingAssessment(&a)
	if status != "техническая оценка отключена" || len(messages) != 1 {
		t.Fatalf("unexpected rejected result: status=%s messages=%v", status, messages)
	}
	if strings.Contains(messages[0], "Предварительная гипотеза:") || !strings.Contains(messages[0], "гипотезы по нему не формируются") {
		t.Fatalf("rejected contour created a hypothesis: %s", messages[0])
	}
}
