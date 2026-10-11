from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
economics = (root / "workflow_pages.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
methods = (root / "methods_workflow.go").read_text(encoding="utf-8")

activate = main[main.index("func activatePage"):main.index("func switchPage")]
resize = main[main.index("func resize()"):main.index("func wndProc")]

checks = {
    "version 702 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(702, 1000)),
    "layout cache removed": "pageLayoutStamp" not in main,
    "deferred page refresh removed": "WM_APP_PAGE_REFRESH" not in main,
    "every resize performs geometry": "stamp :=" not in resize and "setPos(hTopBG" in resize,
    "page refresh stays inside redraw lock": "refreshPage(id)" in activate and "pPostMessageW.Call" not in activate,
    "completed frame and visible children are invalidated after visibility commit": "invalidateVisibleFrame(currentPage)" in activate and "RDW_UPDATENOW" not in activate,
    "main window clips child painting during frame transitions": "WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN_FRAME" in main,
    "child positioning preserves native invalidation": "SWP_NOZORDER)" in main[main.index("func setPos("):main.index("func layoutDashboardFundColumns")],
    "last complete page remains visible through target preparation": (
        "hidePageControls(previousPage)" in activate
        and activate.index("refreshPage(id)") < activate.index("hidePageControls(previousPage)")
        and "if !mainLayoutReady" in activate
        and activate.index("refreshPage(id)") < activate.index("if !mainLayoutReady")
        and activate.index("if !mainLayoutReady") < activate.index("layoutCurrentPage()")
        and activate.index("layoutCurrentPage()") < activate.index("commitPageVisibility(id)")
    ),
    "tab switches avoid full-window relayout after startup": "mainLayoutReady = true" in (root / "stability_frame_windows.go").read_text(encoding="utf-8") and "if !mainLayoutReady" in activate,
    "only the target page is committed after refresh": activate.index("refreshPage(id)") < activate.index("commitPageVisibility(id)") and "hideInactivePageControls" not in activate,
    "short alternative button retained": "+ ДОБАВИТЬ АЛЬТЕРНАТИВУ" in methods,
    "simplified economics retained": "show(econExtendedButton, false)" in economics and "BENEFIT MODELING · УПРОЩЁННОЕ" in economics,
    "strengthened report retained": all(x in report for x in ["resultTraceCard", "Факт / контроль", "СВОДНОЕ ЗАКЛЮЧЕНИЕ И КОНТУР КОНТРОЛЯ"]),
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
