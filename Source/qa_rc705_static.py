from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
well = (root / "well_card_ui.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
alt = (root / "alternative_scenarios.go").read_text(encoding="utf-8")

checks = {
    "version 705+": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(705, 1000)),
    "failure chooser frame cannot draw shifted stub": "show(failureObjectFrame, false)" in well and "setPos(failureObjectFrame, 242, selectorY, 1, 1)" in well,
    "failure chooser row shares one top edge": "setPos(failureObjectCombo, 242, selectorY" in well and "setPos(failureObjectOpenButton, 242+contentW-selectorButtonW, selectorY" in well,
    "failure table follows selector row": "historyY = selectorY + 38" in well,
    "result variant management controls": all(x in report for x in ["ID_RESULT_ADD_SCHEME", "ID_RESULT_COPY_SCHEME", "ID_RESULT_COMPARE_SCHEMES", "ID_RESULT_USE_SELECTED"]),
    "result variants activate explicitly": "activateSelectedResultScenario" in report and "setAlternativeScenarioActiveByID" in alt,
    "result comparison detail exists": "resultScenarioComparisonText" in report and ("СРАВНЕНИЕ ПАРАЛЛЕЛЬНЫХ СХЕМ" in report or "СРАВНЕНИЕ ВАРИАНТОВ" in report),
    "result readiness is visible": "готовность данных %d/6" in report,
    "result archive stores real user": "Author: currentUserLabel()" in report,
    "result list active row is highlighted": "resultVariantCustomDraw" in report and "resultScenarioRows" in report,
    "result double click hooks retained": "case resultVariantsList:" in main and "case resultArchiveList:" in main,
    "final report reference hierarchy retained": all(x in report for x in ["Параллельные варианты методов и мероприятий", "Все проходки итогового анализа", "ПРОГНОЗ → ФАКТ"]),
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
