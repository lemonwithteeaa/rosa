//go:build windows

package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestRC590PlanHasAllCanonicalStatesAndRequiredDateFormat(t *testing.T) {
	want := []string{"Черновик", "Выбран", "Утверждён", "Назначен", "Выполнен", "Отменён"}
	if !reflect.DeepEqual(methodPlanStatuses, want) {
		t.Fatalf("plan state machine changed: got %#v want %#v", methodPlanStatuses, want)
	}
	for _, valid := range []string{"", "—", "01.10.2026", "29.02.2028"} {
		if err := methodPlanValidateDate("Дата", valid); err != nil {
			t.Fatalf("valid date %q rejected: %v", valid, err)
		}
	}
	for _, invalid := range []string{"2026-10-01", "31.02.2026", "1.10.2026"} {
		if err := methodPlanValidateDate("Дата", invalid); err == nil {
			t.Fatalf("invalid date %q accepted", invalid)
		}
	}
}

func TestRC590ApprovedPlanCreatesImmutableDecisionBoundSnapshot(t *testing.T) {
	oldLoaded, oldDecisions, oldPlans := approvedMethodsLoaded, approvedMethodDecisions, plans
	defer func() { approvedMethodsLoaded, approvedMethodDecisions, plans = oldLoaded, oldDecisions, oldPlans }()
	approvedMethodsLoaded = true
	approvedMethodDecisions = []approvedMethodDecision{{
		ID: "decision-v1", ObjectKey: "OBJ", VersionNumber: 1,
		Methods: []string{"Ингибитор"},
	}}
	plans = []PlanItem{{
		ID: "old-plan", ObjectKey: "OBJ", MethodName: "Ингибитор", DecisionID: "decision-v1",
		Status: "Назначен", DueDate: "01.10.2026", Responsible: "Технолог",
		ControlDate: "15.10.2026", ControlCriterion: "МРП не ниже цели",
	}}

	d := approvedMethodDecision{
		ID: "decision-v2", ObjectKey: "OBJ", VersionNumber: 2,
		Method: "Ингибитор", Methods: []string{"Ингибитор"}, ApprovedAt: "29.09.2026 18:00", ApprovedBy: "Технолог",
	}
	got := methodPlanItemsForApprovedDecision(d, "утверждено технологом")
	if len(got) != 1 {
		t.Fatalf("approved plan items=%d want 1", len(got))
	}
	p := got[0]
	if p.DecisionID != d.ID || p.ParametersRef != d.ID+"::Ингибитор" {
		t.Fatalf("plan is not bound to immutable decision snapshot: %+v", p)
	}
	if p.Status != "Утверждён" {
		t.Fatalf("new approved version inherited operational status %q", p.Status)
	}
	if p.DueDate != "01.10.2026" || p.Responsible != "Технолог" || p.ControlCriterion != "МРП не ниже цели" {
		t.Fatalf("technologist plan fields were lost on approval: %+v", p)
	}
}

func TestRC590ArchivePreviewIsReadOnlyAndRestorePlanRebindsVersion(t *testing.T) {
	oldLoaded, oldDecisions := approvedMethodsLoaded, approvedMethodDecisions
	defer func() { approvedMethodsLoaded, approvedMethodDecisions = oldLoaded, oldDecisions }()
	approvedMethodsLoaded = true
	old := approvedMethodDecision{
		ID: "decision-v1", ObjectKey: "OBJ", VersionNumber: 1,
		Method: "Ингибитор", Methods: []string{"Ингибитор"}, ApprovedAt: "01.09.2026 10:00",
		Plan: []PlanItem{{MethodName: "Ингибитор", Status: "Выполнен", DueDate: "05.09.2026", Responsible: "Технолог", ControlDate: "20.09.2026", ControlCriterion: "Факт подтверждён"}},
	}
	approvedMethodDecisions = []approvedMethodDecision{old}
	before := old
	preview := methodArchivePreviewText(old)
	if !reflect.DeepEqual(old, before) {
		t.Fatal("archive preview mutated stored decision")
	}
	if !strings.Contains(preview, "Просмотр архива ничего не меняет") || !strings.Contains(preview, "создаёт НОВУЮ версию") {
		t.Fatalf("archive preview lacks read-only/new-version contract: %q", preview)
	}

	restored := old
	restored.ID = "decision-v2"
	restored.VersionNumber = 2
	restored.RestoredFromID = old.ID
	items := methodPlanItemsForRestoredDecision(restored, old)
	if len(items) != 1 {
		t.Fatalf("restored plan items=%d want 1", len(items))
	}
	if items[0].DecisionID != restored.ID || items[0].ParametersRef != restored.ID+"::Ингибитор" {
		t.Fatalf("restored plan still points to archived decision: %+v", items[0])
	}
	if items[0].ID == old.Plan[0].ID {
		t.Fatalf("restored plan reused archived row id %q", items[0].ID)
	}
	if old.Plan[0].DecisionID != "" || old.Plan[0].ParametersRef != "" {
		t.Fatal("building restored plan mutated archived plan")
	}
}

func TestRC590EconomicsLinkIsVersionSpecificAndBecomesStaleForNewDecision(t *testing.T) {
	oldMethodsLoaded, oldDecisions := approvedMethodsLoaded, approvedMethodDecisions
	oldLinksLoaded, oldLinks := economicsDecisionLinksLoaded, economicsDecisionLinks
	defer func() {
		approvedMethodsLoaded, approvedMethodDecisions = oldMethodsLoaded, oldDecisions
		economicsDecisionLinksLoaded, economicsDecisionLinks = oldLinksLoaded, oldLinks
	}()
	approvedMethodsLoaded = true
	economicsDecisionLinksLoaded = true
	approvedMethodDecisions = []approvedMethodDecision{{ID: "decision-v1", ObjectKey: "OBJ", VersionNumber: 1}}
	economicsDecisionLinks = []EconomicsDecisionLink{{ObjectKey: "OBJ", DecisionID: "decision-v1", DecisionVersion: 1, Status: economicsDecisionStatusCurrent}}

	link, ok := economicsDecisionLinkForObject("OBJ")
	if !ok || link.Status != economicsDecisionStatusCurrent || link.DecisionID != "decision-v1" {
		t.Fatalf("current economics link rejected: %+v ok=%v", link, ok)
	}

	approvedMethodDecisions = append(approvedMethodDecisions, approvedMethodDecision{ID: "decision-v2", ObjectKey: "OBJ", VersionNumber: 2, RestoredFromID: "decision-v1"})
	link, ok = economicsDecisionLinkForObject("OBJ")
	if !ok || link.Status != economicsDecisionStatusNeedsRecalc || link.DecisionID != "decision-v2" {
		t.Fatalf("new/restored decision did not invalidate economics: %+v ok=%v", link, ok)
	}
	if !strings.Contains(link.Reason, "изменена версия") {
		t.Fatalf("unexpected stale-link reason %q", link.Reason)
	}
}

func TestRC590RequiredPlanFieldsAndEconomicsInvalidationStayWired(t *testing.T) {
	planSource, err := os.ReadFile("methods_plan_archive.go")
	if err != nil {
		t.Fatal(err)
	}
	plan := string(planSource)
	for _, token := range []string{
		`status == "Назначен" && (due == "—" || owner == "—")`,
		`status == "Выполнен" && (controlDate == "—" || criterion == "")`,
		`Восстановить выбранную архивную версию?`,
		`d.RestoredFromID = old.ID`,
		`d.VersionNumber = nextMethodDecisionVersion(key)`,
		`markEconomicsNeedsRecalcForDecision(d, "восстановлена архивная версия технологического решения")`,
	} {
		if !strings.Contains(plan, token) {
			t.Fatalf("plan/archive contract is no longer wired: missing %q", token)
		}
	}

	implementationSource, err := os.ReadFile("methods_implementation.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(implementationSource), `markEconomicsNeedsRecalcForDecision(decision, "изменены параметры реализации утверждённого метода")`) {
		t.Fatal("editing approved implementation parameters no longer invalidates economics")
	}

	workflowSource, err := os.ReadFile("methods_workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflowSource), `markEconomicsNeedsRecalcForDecision(d, "утверждена новая версия технологического решения")`) {
		t.Fatal("new approved decision no longer invalidates economics")
	}

	economicsSource, err := os.ReadFile("workflow_pages.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(economicsSource), `markEconomicsCalculatedForCurrentDecision(selectedObjectKey, scenarioID)`) {
		t.Fatal("successful economics calculation is no longer linked to the current decision version")
	}
}
