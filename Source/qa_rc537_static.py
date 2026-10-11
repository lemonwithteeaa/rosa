from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "object_selection.go", "assessment_classification.go",
    "laboratory_infographic_excel.go",
]}
excel = files["laboratory_infographic_excel.go"]
all_source = "\n".join(files.values())
checks = {
    "version_538": 'const appBuildVersion = "7.3.11.538-RC"' in files["main.go"],
    "fixed_x_y_axes": all(x in excel for x in ["infographicExcelAxisBounds", '<c:min val=', '<c:max val=']),
    "straight_lines": '<c:smooth val="0"/>' in excel,
    "legend_outside_plot": all(x in excel for x in ['<c:legendPos val="b"/>', '<c:overlay val="0"/>']),
    "threshold_text_only": 'strings.ToUpper(threshold.caption(metric.Unit))' in excel and 'xlsxInlineCell("B1"' in excel and 'lastColumnName' not in excel,
    "native_editable_chart": all(x in excel for x in ['<c:lineChart>', '<c:dateAx>', '<c:numRef>']),
    "double_click_confirmation": all(x in all_source for x in ["confirmSelectedObjectAfterQuestion", "Утвердить объект?", "askYesNo"]),
    "restore_object_workspace": all(x in files["object_selection.go"] for x in ["restoreSelectedObjectWorkspace", "latestSavedLaboratoryPassForObject", "econBoundObjectKey = \"\""]),
    "selected_lab_drives_classes": all(x in files["main.go"] for x in ["labDrafts[groupCode]", "выбранная лабораторная запись"]),
    "top_object_change_warning": '⚠ ОБЪЕКТ ИЗМЕНЁН\\r\\nГлавная:' in files["main.go"],
    "no_empty_fund_rows": all(x in files["main.go"] for x in ["desiredObjectH", "rowCount := int32(len(filteredObjects()))"]),
    "history_not_offscreen": all(x in files["assessment_classification.go"] for x in ["historyVisible := false", "h-historyY >= 150"]),
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks)-len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
