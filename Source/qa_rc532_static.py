from pathlib import Path

root = Path(__file__).resolve().parent
names = [
    "main.go", "auth.go", "startup_loading.go", "laboratory_complex_multi.go",
    "laboratory_infographic_excel.go", "laboratory_calculation_basis.go",
    "assessment_classification.go", "workflow_pages.go",
]
files = {name: (root / name).read_text(encoding="utf-8") for name in names}

checks = {
    "version_532": 'const appBuildVersion = "7.3.11.532-RC"' in files["main.go"],
    "show_button_app_style": "ID_AUTH_SHOW_PASSWORD" in files["auth.go"] and "BS_OWNERDRAW" in files["auth.go"] and "authRoundWindow(authPasswordShowButton" in files["auth.go"],
    "recognisable_pumpjack": all(x in files["startup_loading.go"] for x in ["Walking beam", "Polished rod", "counterweight", "wellhead"]),
    "adaptive_y_axis": all(x in files["laboratory_complex_multi.go"] for x in ["tick <= 4", "value := maxV", "ratio < 0", "ratio > 1"]),
    "visible_trend": all(x in files["laboratory_complex_multi.go"] for x in ['draw("ТРЕНД"', "rgb(203, 27, 43)", "8, rgb(255, 255, 255)"]),
    "excel_chart_cache": all(x in files["laboratory_infographic_excel.go"] for x in ["infographicNumberCacheXML", "<c:numCache>", "<c:ptCount", "CB1B2B"]),
    "textbook_steps": all(x in files["laboratory_calculation_basis.go"] for x in ["ШАГ %d", "Формула проверки", "Подстановка", "calculationScaleTextbookForModel"]),
    "real_excel_formulas": all(x in files["laboratory_calculation_basis.go"] for x in ["calculationFormulaCell", "<f>%s</f>", "LOG10(B", "fullCalcOnLoad"]),
    "assessment_three_decisions": all(x in files["assessment_classification.go"] for x in ["принято без лаборатории", "без корректировки", "с корректировкой"]),
    "assessment_disagreement_highlight": all(x in files["assessment_classification.go"] for x in ["assessmentCompareCustomDraw", "РАЗНОГЛАСИЕ", "ClrTextBk"]),
    "assessment_research_advice": all(x in files["assessment_classification.go"] for x in ["assessmentVerificationPlan", "ЧТО ИССЛЕДОВАТЬ ДЛЯ ПОДТВЕРЖДЕНИЯ", "assessmentReconciliationAdvice"]),
    "final_report_decision": "assessmentFinalReportText" in files["assessment_classification.go"] and "assessmentFinalReportText(objectKey)" in files["workflow_pages.go"],
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks) - len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
