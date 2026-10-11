from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "laboratory.go", "laboratory_infographic_excel.go",
    "laboratory_history_chart.go", "laboratory_complex_infographic.go",
    "assessment_classification.go",
]}
excel = files["laboratory_infographic_excel.go"]
checks = {
    "version_538": 'const appBuildVersion = "7.3.11.538-RC"' in files["main.go"],
    "native_line_chart": all(x in excel for x in ['<c:lineChart>', 'drawingml/2006/chart', 'xl/charts/chart']),
    "direct_category_cells": '<c:cat><c:numRef><c:f>%s!$A$%d:$A$%d' in excel,
    "direct_value_cells": '<c:val><c:numRef><c:f>%s!$%s$%d:$%s$%d' in excel,
    "header_cell_names": '<c:strRef><c:f>%s!$%s$4' in excel,
    "excel_date_axis": all(x in excel for x in ['<c:dateAx>', '<c:baseTimeUnit val="days"/>']),
    "excel_absolute_relationships": all(x in excel for x in ['Target="/xl/drawings/drawing%d.xml"', 'Target="/xl/charts/chart%d.xml"']),
    "not_a_picture": all(x not in excel for x in ['relationships/image', 'xl/media/image', '<xdr:pic>', 'renderInfographicExcelChartPNG']),
    "visible_trendline": all(x in excel for x in ['val="CB1B2B"', 'w="44450"']),
    "threshold_caption_right": all('plot.Right-360' in files[name] for name in ["laboratory_history_chart.go", "laboratory_complex_infographic.go"]),
    "assessment_bold": all(x in files["assessment_classification.go"] for x in ['setAssessmentTabDetail', 'EM_SETCHARFORMAT', 'CFE_BOLD']),
    "assessment_object_red": 'child == objectSelectedValue || child == assessObjectInfo' in files["main.go"],
    "other_object_warning": all(x in files["laboratory.go"] for x in ['⚠ ДРУГОЙ ОБЪЕКТ:', 'ВНИМАНИЕ: в лаборатории будет открыт другой объект']),
    "cross_page_warning": 'В ЛАБОРАТОРНОМ РАЗДЕЛЕ ОТКРЫТ ДРУГОЙ ОБЪЕКТ' in files["main.go"],
    "teaching_conclusion_open": 'A selected saved or teaching example' in files["laboratory.go"] and 'labConclusionOpen = true' in files["laboratory.go"],
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks)-len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
