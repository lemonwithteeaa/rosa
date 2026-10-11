from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
well = (root / "well_card_ui.go").read_text(encoding="utf-8")

info_body = report[report.index("func openResultSelectedMeasuresInfo()") : report.index("func openSelectedResultScenarioEconomics()")]
econ_body = report[report.index("func openSelectedResultScenarioEconomics()") : report.index("func activateSelectedResultScenario()")]

checks = {
    "version 709": 'const appBuildVersion = "7.3.11.709-RC"' in main,
    "section 8 renamed": 'ID_ARCHIVE, "8. Утверждённый реестр"' in main and 'pageTitle(ID_ARCHIVE, "Утверждённый реестр"' in main,
    "methods action navigates": "setAlternativeScenarioActiveByID" in info_body and "switchPage(ID_ACTIVITIES)" in info_body and "openMethodsReferenceDialog" not in info_body,
    "economics action navigates": "setAlternativeScenarioActiveByID" in econ_body and "switchPage(ID_ECON)" in econ_body,
    "pass list caption": '"Все проходки итогового анализа"' in report and '"СОХРАНИТЬ ПРОХОДКУ"' in report,
    "registry approval caption": '"УТВЕРДИТЬ В РЕЕСТР"' in report,
    "approval marker persisted": "ApprovedAt" in report and "appendFullReportRecord" in report,
    "registry contains only explicit approvals": "approvedFullReportRecords" in report and 'strings.TrimSpace(fullReports[i].ApprovedAt) != ""' in report,
    "registry opens frozen snapshot": "openSelectedApprovedRegistryReport" in report and "case archiveList:" in main,
    "first-open relayout": "refreshFailureImportUI()" in well and "relayoutWellCardFailureUI()" in well,
    "relayout happens after chooser refresh": well.index("refreshFailureImportUI()", well.index("func refreshWellCardFailures")) < well.index("relayoutWellCardFailureUI()", well.index("func refreshWellCardFailures")),
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
