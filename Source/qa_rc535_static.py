from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "laboratory.go", "laboratory_infographic_excel.go",
    "laboratory_excel_chart_image.go", "laboratory_history_chart.go",
    "laboratory_complex_infographic.go", "assessment_classification.go",
]}

checks = {
    "version_535": 'const appBuildVersion = "7.3.11.535-RC"' in files["main.go"],
    "excel_png_content_type": 'ContentType="image/png"' in files["laboratory_infographic_excel.go"],
    "excel_image_relationship": 'relationships/image' in files["laboratory_infographic_excel.go"],
    "excel_picture_anchor": all(x in files["laboratory_infographic_excel.go"] for x in ['<xdr:pic>', 'r:embed="rId1"', 'xl/media/image']),
    "no_fragile_chart_part": all(x not in files["laboratory_infographic_excel.go"] for x in ['drawingml/chart', 'xl/charts/chart']),
    "png_chart_renderer": all(x in files["laboratory_excel_chart_image.go"] for x in ['image.NewNRGBA', 'png.Encoder', 'Красная линия — тренд']),
    "threshold_caption_right": all('plot.Right-360' in files[name] for name in ["laboratory_history_chart.go", "laboratory_complex_infographic.go"]),
    "assessment_rich_edit": 'assessTabDetail = richEdit' in files["main.go"],
    "assessment_bold_headings": all(x in files["assessment_classification.go"] for x in ['setAssessmentTabDetail', 'EM_SETCHARFORMAT', 'CFE_BOLD']),
    "assessment_object_red": 'child == objectSelectedValue || child == assessObjectInfo' in files["main.go"],
    "lab_other_object_caption": '⚠ ДРУГОЙ ОБЪЕКТ:' in files["laboratory.go"],
    "lab_other_object_popup": 'ВНИМАНИЕ: в лаборатории будет открыт другой объект' in files["laboratory.go"],
    "cross_page_warning": 'В ЛАБОРАТОРНОМ РАЗДЕЛЕ ОТКРЫТ ДРУГОЙ ОБЪЕКТ' in files["main.go"],
    "teaching_conclusion_open": 'A selected saved or teaching example' in files["laboratory.go"] and 'labConclusionOpen = true' in files["laboratory.go"],
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks) - len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
