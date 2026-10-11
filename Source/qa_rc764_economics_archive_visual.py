from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
economics = (root / "economics_visual_reference.go").read_text(encoding="utf-8")
cards = (root / "economics_ui_refinement.go").read_text(encoding="utf-8")
workflow = (root / "workflow_pages.go").read_text(encoding="utf-8")
methods = (root / "methods_reference_dialog.go").read_text(encoding="utf-8")

checks = {
    "selected object value uses ROSA red while label remains navy bold":
        'strings.HasPrefix(strings.ToUpper(t), "ОБЪЕКТ:")' in methods and
        'CrTextColor: uint32(rgb(15, 48, 82))' in methods and
        'CrTextColor: uint32(rgb(183, 20, 37))' in methods,
    "four metric cards use existing selected-scheme values only":
        'activeAlternativeScenario(selectedObjectKey)' in cards and
        'economicsScenarioCalculated(scenario)' in cards and
        'scenario.NetBenefitK' in cards and
        'CalculateBaseEconomics' not in cards,
    "metric snapshot refreshes before repaint":
        'econRefMetricSnapshot = economicsMetricStrings()' in cards and
        'pInvalidateRect.Call(uintptr(h), 0, 0)' in cards,
    "card controls are created once with the economics page":
        'buildEconomicsMetricCards()' in economics and
        'func buildEconomicsMetricCards()' in cards,
    "card strip collapses below supported width and preserves scenario text":
        'canShow := width >= 990 && height >= 54' in cards and
        'setPos(econScenarioCard, x, py(scenarioY), descriptionWidth, scenarioH)' in economics,
    "archive and scheme list styles are handled before click routing":
        'return economicsVisualCustomDraw(hdr.HwndFrom, lParam)' in main and
        main.index('economicsVisualCustomDraw(hdr.HwndFrom, lParam)') < main.index('if hdr.Code != NM_CLICK && hdr.Code != NM_DBLCLK'),
    "metric owner draw is handled before generic buttons":
        'if paintEconomicsMetricCard(di)' in main and
        main.index('if paintEconomicsMetricCard(di)') < main.index('if di != nil && di.CtlType == ODT_BUTTON', main.index('case WM_DRAWITEM')),
    "single and parallel recalculation refresh the fund archive and KPI values":
        workflow.count('refreshEconomicsMetricCards()') >= 4 and
        'refreshEconomicsVisualReference()' in workflow,
    "wide windows preserve four archive rows at the target 900px layout":
        'if h >= 890 && h < 930 && cw >= 990' in economics and
        'archiveH = 115' in economics,
    "double-click still opens the selected object's economics archive":
        'openEconomicsFundArchiveAtRow(i)' in main,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
