//go:build windows

package main

import (
	"strings"
	"testing"
)

func TestRC372OutcomeDetailsPreservedWhenBasicFactIsResaved(t *testing.T) {
	old := Outcome{
		FailureCause:       "коррозионное повреждение",
		DefectFinding:      "локальная язва",
		EvidenceSource:     "Акт дефектовки",
		EvidenceComment:    "подтверждено осмотром",
		NextCycleStartDate: "21.09.2026",
		NextCycleStatus:    "Работает",
		ObservedEffect:     "Наблюдается улучшение",
		EffectMetric:       "МРП",
		EffectBefore:       "420 сут.",
		EffectAfter:        "610 сут.",
		EffectAttribution:  "Есть временная связь; причинность не подтверждена",
	}
	fresh := Outcome{ActualMRP: "610"}
	preserveOutcomeDetails(&fresh, &old)
	if fresh.FailureCause != old.FailureCause || fresh.DefectFinding != old.DefectFinding || fresh.EffectMetric != old.EffectMetric || fresh.EffectAttribution != old.EffectAttribution {
		t.Fatalf("structured outcome fields were not preserved: %+v", fresh)
	}
}

func TestRC372StructuredSummarySeparatesObservationFromAttribution(t *testing.T) {
	o := Outcome{
		FailureCause:      "засорение",
		ObservedEffect:    "Наблюдается улучшение",
		EffectMetric:      "межремонтный период",
		EffectBefore:      "300",
		EffectAfter:       "500",
		EffectAttribution: "Есть временная связь; причинность не подтверждена",
	}
	s := outcomeStructuredSummary(o)
	for _, want := range []string{"Причина:", "Наблюдаемый эффект:", "Причинная интерпретация:", "причинность не подтверждена"} {
		if !strings.Contains(s, want) {
			t.Fatalf("summary missing %q: %s", want, s)
		}
	}
}

func TestRC372BuildVersion(t *testing.T) {
	if !strings.HasPrefix(appBuildVersion, "7.3.11.") || !strings.HasSuffix(appBuildVersion, "-RC") || appBuildVersion == "7.3.11.371-RC" {
		t.Fatalf("build version rolled back: %s", appBuildVersion)
	}
}
