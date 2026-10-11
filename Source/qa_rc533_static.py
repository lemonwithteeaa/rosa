from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "laboratory_infographic_excel.go",
    "laboratory_calculation_basis.go", "assessment_classification.go",
]}

checks = {
    "version_533": 'const appBuildVersion = "7.3.11.533-RC"' in files["main.go"],
    "excel_minimal_chart": all(x in files["laboratory_infographic_excel.go"] for x in [
        '<c:style val="13"/>', 'trendline must precede xVal/yVal',
        'infographicNumberCacheXML', '<c:scatterChart>',
    ]),
    "desktop_graph_copy": all(x in files["laboratory_infographic_excel.go"] for x in [
        "rosaDesktopDir", "ДЛЯ_ПРОВЕРКИ_И_ИЗМЕНЕНИЙ", "не загружаются обратно",
    ]),
    "resizable_calculation_window": "WS_OVERLAPPEDWINDOW|WS_VISIBLE" in files["laboratory_calculation_basis.go"],
    "visible_excel_formula": all(x in files["laboratory_calculation_basis.go"] for x in [
        '"Excel-формула"', '"Результат Excel"', 'xlsxInlineCell("E"+ref, "="+formula',
        'calculationFormulaCell("F"+ref',
    ]),
    "detailed_textbook_calculation": all(x in files["laboratory_calculation_basis.go"] for x in [
        "I = ½·(", "Ksp = 10^", "IAP =", "SI = log₁₀",
    ]),
    "desktop_calculation_copy": all(x in files["laboratory_calculation_basis.go"] for x in [
        "rosaDesktopDir", "ДЛЯ_ПРОВЕРКИ_И_ИЗМЕНЕНИЙ", "не загружаются обратно",
    ]),
    "program_tab_read_only": all(x in files["assessment_classification.go"] for x in [
        "show(assessClassCombo, tech)", "сохранённая лабораторная диагностика",
        "На этой вкладке пользователь ничего не назначает",
    ]),
    "skip_then_accept": all(x in files["assessment_classification.go"] for x in [
        "skipAssessmentLaboratoryReview", "ПРОПУСТИТЬ ЛАБОРАТОРИЮ",
        "ПРИНЯТЬ КЛАССЫ ТЕХНОЛОГА", "assessmentLaboratoryReviewSkipped",
    ]),
    "first_tab_save_isolated": "idx := assessTechSelected" in files["assessment_classification.go"] and "if assessInnerTab == 1 {\n\t\tidx" not in files["assessment_classification.go"],
    "adaptive_assessment_columns": "assessmentResizeColumns(contentW)" in files["assessment_classification.go"],
    "accent_direction_columns": all(x in files["assessment_classification.go"] for x in [
        "assessmentDirectionCustomDraw", "draw.SubItem == 0", "221, 233, 244",
    ]),
    "accent_headers": "setListHeaderFont(table, hFontBold)" in files["main.go"],
    "program_justification_header": "Обоснование класса" in files["main.go"],
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks) - len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
