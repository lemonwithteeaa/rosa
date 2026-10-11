from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8")
economics = (root / "workflow_pages.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

checks = {
    "version 701 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(701, 1000)),
    "short alternative caption": '+ ДОБАВИТЬ АЛЬТЕРНАТИВУ' in methods,
    "extended economics is hidden": "show(econExtendedButton, false)" in economics,
    "only simplified benefit modeling remains": "BENEFIT MODELING · УПРОЩЁННОЕ СРАВНЕНИЕ" in economics,
    # RC762 expresses the same five-field contract as a visibility map so the
    # page can share geometry with the paired economics tables. Keep accepting
    # the earlier slice representation for historical source snapshots.
    "simplified economics uses five shared fields": (
        "visibleFields := []int{0, 5, 6, 7, 9}" in economics
        or "visible := map[int]bool{0: true, 5: true, 6: true, 7: true, 9: true}" in economics
    ),
    "report has forecast fact trace": all(x in report for x in ["resultTraceCard", "Факт / контроль", "reportTraceabilityText"]),
    "report states limitations": "СВОДНОЕ ЗАКЛЮЧЕНИЕ И КОНТУР КОНТРОЛЯ" in report and "не доказывают причинный эффект" in report,
    "report archive has three exports": all(x in report for x in ['exportSelectedFullReport("pdf")', 'exportSelectedFullReport("docx")', 'exportSelectedFullReport("xlsx")']),
    "archive keeps at least five visible rows": "max32(150, bottom-archiveY)" in report,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
