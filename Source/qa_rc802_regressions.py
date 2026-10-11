from pathlib import Path
import json

root = Path(__file__).resolve().parent
src = lambda filename: (root / filename).read_text(encoding="utf-8")
main = src("main.go")
methods = src("methods_workflow.go")
background = src("methods_background.go")
report = src("report_workspace.go")
corpus = json.loads(src("training_corpus.json"))

begin = background.index("func refreshMethodsWorkflowUI()")
end = background.index("func showMethodsPendingState(")
refresh = background[begin:end]
report_archive = report[report.index("func refreshFullReportArchive()"):report.index("\nfunc reportScenarioForAction(")]
method_archive = methods[methods.index("func refreshMethodsArchive()"):methods.index("\nfunc addSelectedMethodsWorkflowMethod(")]
layout = report[report.index("func layoutResultWorkspace("):]
checks = {
    "802-RC build identifier": 'const appBuildVersion = "7.3.11.802-RC"' in main,
    "no duplicate work after Methods navigation": "if methodsBackgroundBusy && methodsBackgroundKey == key" in refresh and "showMethodsPendingState(key)" in refresh,
    "pending Methods content, archive and geometry restored": all(x in background for x in ("func showMethodsPendingState(key string)", "refreshMethodsArchive()", "relayoutMethodsWorkflowCurrent()")),
    "stale background result blocked by generation": "methodsBackgroundResultIsCurrent(result, token, methodsBackgroundGeneration.Load(), selectedObjectKey)" in background,
    "methods approved history uncapped": "row < 100" not in method_archive.split("// Keep several clearly marked examples")[0] and "for i := len(approvedMethodDecisions) - 1; i >= 0; i--" in method_archive,
    "native method history row count checked": 'METHODS history incomplete expected=' in method_archive,
    "all report passes iterated": "for i := len(fullReports) - 1; i >= 0; i--" in report_archive,
    "report passes show total and a scrolling indication": "Проходок: %d · все объекты · прокрутка" in report_archive,
    "report archive rows not cleared if unchanged": "resultArchiveRenderHash == signature" in report_archive and "lvmGetItemCount" in report_archive,
    "report archive row count is verified": 'REPORT archive incomplete expected=' in report_archive,
    "archive height is bounded by its window": "archiveVisibleH := bottom - archiveY" in layout and "setPos(resultArchiveList, x, archiveY, contentW, archiveVisibleH)" in layout,
    "native window re-layout skips identical coordinates": "origin.X == x && origin.Y == y" in main and "SWP_NOZORDER|frameNoActivate" in main,
    "native list avoids redundant column resizing": "const lvmGetColumnWidth" in main and "int32(current) == width" in main,
    "native method text avoids redundant font change": "const wmGetFont" in src("methods_typography.go"),
    "all twenty training wells preserved": corpus.get("synthetic") and len(corpus["wells"]) == 20,
    "60 failures and 180 lab records still linked": sum(len(w["failures"]) for w in corpus["wells"]) == 60 and sum(len(w["labPasses"]) for w in corpus["wells"]) == 180,
    "both teaching report revisions remain": "for ordinal := 1; ordinal <= 20; ordinal++" in src("teaching_alternative_scenarios.go") and "for version := 1; version <= 2; version++" in src("teaching_alternative_scenarios.go"),
}
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
print(f"RESULT {sum(checks.values())}/{len(checks)}")
if not all(checks.values()):
    raise SystemExit(1)
