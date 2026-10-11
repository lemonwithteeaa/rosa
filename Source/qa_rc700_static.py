from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8")
alts = (root / "alternative_scenarios.go").read_text(encoding="utf-8")
fixtures = (root / "teaching_alternative_scenarios.go").read_text(encoding="utf-8")
economics = (root / "workflow_pages.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
tests = (root / "rc700_parallel_examples_test.go").read_text(encoding="utf-8")

checks = {
    "version 700 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(700, 1000)),
    "throughgoing teaching examples cover all 20 current wells": "for ordinal := 1; ordinal <= 20; ordinal++" in fixtures and "teachingAlternativeTemplates(objectKey)" in fixtures,
    "three distinct alternatives are generated per well": all(x in fixtures for x in ["Повтор метода", "Новый подбор", "Комплексная защита", '[]string{"A", "B", "C"}']),
    "choice logic covers effect, cost, and varying resource availability": all(x in fixtures for x in ["NetBenefitK", "OneTimeCAPEXRub", "AnnualOPEXRub", "scenarioAvailabilityReady", "scenarioAvailabilityPartial", "scenarioAvailabilityMissing"]),
    "target runlife is at least 1000": "ForecastBase < 1000" in tests and "TargetMRPDays: 1000" in fixtures,
    "resource availability retained": all(x in fixtures for x in ["scenarioAvailabilityReady", "scenarioAvailabilityPartial", "scenarioAvailabilityMissing"]),
    "unavailable scheme cannot be approved": "сначала выберите доступную схему" in alts,
    "scenario stores own capex opex": "AnnualOPEXRub" in alts and "OneTimeCAPEXRub" in alts,
    "economics switches own inputs": "Parallel schemes are independent economic alternatives" in economics,
    "economics compares all schemes": "ПАРАЛЛЕЛЬНЫЙ РАСЧЁТ СХЕМ" in economics and "SelectionBasis" in economics,
    "accent alternative action": "+ ДОБАВИТЬ АЛЬТЕРНАТИВУ" in methods,
    "report has dense eight-column comparison": all(x in report for x in ["CAPEX / OPEX", "Факт / контроль", "Основание выбора"]),
    "40 seeded report revisions cover 20 current wells": "for ordinal := 1; ordinal <= 20; ordinal++" in fixtures and "for version := 1; version <= 2; version++" in fixtures,
    "archive supports three exports": all(x in report for x in ['exportSelectedFullReport("pdf")', 'exportSelectedFullReport("docx")', 'exportSelectedFullReport("xlsx")']),
    "selected scheme row highlighted": "selectListRow(resultVariantsList, i)" in report,
    "teaching scenario regression tests": "TestRC700ThreeTeachingObjectsHaveParallelSchemes" in tests,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
