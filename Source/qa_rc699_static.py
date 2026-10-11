from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8")
alts = (root / "alternative_scenarios.go").read_text(encoding="utf-8")
technical = (root / "technical_scenario.go").read_text(encoding="utf-8")
economics = (root / "workflow_pages.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

checks = {
    "version 699 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(699, 1000)),
    "accent add-alternative button": "+ АЛЬТЕРНАТИВНАЯ СХЕМА" in methods and "ID_METHODS_SCENARIO_ADD" in main,
    "new alternative resets method data": "MethodsInitialized: true" in alts and "if copyCurrent" in alts,
    "object source snapshot is retained": "SourceSignature: alternativeSourceSignature(objectKey)" in alts and "Classification: base.Classification" in alts,
    "scenario-specific cost key": "scenarioDraftKeyForID" in technical and "activeAlternativeScenarioID" in technical,
    "four availability states": all(x in alts for x in ["Не проверено", "Доступно", "Частично доступно", "Недоступно"]),
    "economics calculates every scheme": "calculateAllAlternativeEconomics" in economics and "for _, scenario := range scenarios" in economics,
    "economics uses common assumptions and own costs": "loadScenarioCostDraftForID" in economics and "CurrentMRPDays: currentMRP" in economics,
    "report has six section cards": "resultSectionCards" in report and "[6]syscall.Handle" in report,
    "report compares alternatives": "resultVariantsList" in report and "alternativeScenariosForObject" in report,
    "full report archive persisted": "full_reports_archive.json" in report and "saveCurrentFullReport" in report,
    "archive exports three formats": all(x in report for x in ['exportSelectedFullReport("pdf")', 'exportSelectedFullReport("docx")', 'exportSelectedFullReport("xlsx")']),
    "archive reserves five rows": "max32(150, bottom-archiveY)" in report,
    "limited resources produce conditional approval": "Условно утверждено" in alts,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
