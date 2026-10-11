from pathlib import Path

root = Path(__file__).resolve().parent
files = {name: (root / name).read_text(encoding="utf-8") for name in [
    "main.go", "auth.go", "startup_loading.go", "laboratory_complex_analysis.go",
    "laboratory_complex_multi.go", "laboratory_infographic_excel.go",
    "laboratory_calculation_basis.go", "laboratory_report_export.go",
]}

checks = {
    "version": 'const appBuildVersion = "7.3.11.531-RC"' in files["main.go"],
    "password_mask": "ES_PASSWORD|ES_CENTER|ES_MULTILINE" in files["auth.go"] and "authSetPasswordMask(true)" in files["auth.go"],
    "password_toggle": all(x in files["auth.go"] for x in ["ID_AUTH_SHOW_PASSWORD", '"Показать"', '"Скрыть"']),
    "pumpjack": all(x in files["startup_loading.go"] for x in ["startupDrawPumpjack", "startupPolygon", "startupEllipse"]),
    "label_close": "complexSelectSeriesAtPoint(point) < 0" in files["laboratory_complex_analysis.go"],
    "context_menu": all(x in files["laboratory_complex_analysis.go"] for x in ["CreatePopupMenu", "Удалить из анализа…", "Добавить линию тренда…"]),
    "session_trash": "addSessionTrashItem" in files["laboratory_complex_analysis.go"],
    "zoom": all(x in files["laboratory_complex_analysis.go"] for x in ['modalCreate("BUTTON", "−"', 'modalCreate("BUTTON", "+"']),
    "threshold_below_plot": all(x in files["laboratory_complex_multi.go"] for x in ["plot.Bottom + 50", "plot.Bottom + 74", "plot.Bottom + 100"]),
    "excel_single_point": "len(chartSeriesValues[i]) >= 1" in files["laboratory_infographic_excel.go"],
    "excel_numeric_date": "dateCell = xlsxNumberCell" in files["laboratory_infographic_excel.go"],
    "calculation_modes": all(x in files["laboratory_calculation_basis.go"] for x in ["Индивидуальная диагностика", "Полный расчёт по объекту"]),
    "model_choice": "Все доступные модели (сравнение)" in files["laboratory_calculation_basis.go"],
    "structured_xlsx": all(x in files["laboratory_calculation_basis.go"] for x in ["writeCalculationWorkbookXLSX", 'name="Сводка"', "calculationWorkbookSheet"]),
    "docx_footer": all(x in files["laboratory_report_export.go"] for x in ["footer1.xml", 'w:instr="PAGE"']),
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + " | " + name)
print(f"RESULT | {len(checks) - len(failed)}/{len(checks)} passed")
raise SystemExit(1 if failed else 0)
