from pathlib import Path

root = Path(__file__).resolve().parent
econ = (root / "economics_fund_archive.go").read_text(encoding="utf-8")
methods = (root / "methods_plan_archive.go").read_text(encoding="utf-8")
main = (root / "main.go").read_text(encoding="utf-8")

checks = {
    "economics archive has comparison and detail tables":
        'economicsFundDialogList = modalCreate("SysListView32"' in econ and
        'economicsFundDialogDetail = modalCreate("SysListView32"' in econ,
    "resizable economics window supports maximize and minimum track size":
        'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in econ and
        'mmi.PtMinTrackSize.X = 900' in econ and
        'mmi.PtMinTrackSize.Y = 650' in econ,
    "economics method heading is red bold italic":
        'economicsFundMethodLabel(index)+" · "+strings.TrimSpace(s.Name)' in econ and
        'pSetTextColor.Call(wParam, rgb(183, 20, 37))' in econ and
        'pCreateFontW.Call(^uintptr(19-1), 0, 0, 0, 700, 1' in econ,
    "details show stored economics fields and status without recalculation":
        'rows := [][]string{' in econ and
        '{"Чистый эффект, тыс. руб.", econReferenceMoney(s.NetBenefitK, known)}' in econ and
        'econReferenceStatus(s)' in econ,
    "methods object history window is resizable too":
        methods.count('WS_OVERLAPPEDWINDOW|WS_VISIBLE') >= 2,
    "navigation arrows use thin polylines instead of filled block arrows":
        'pPolyline.Call(uintptr(di.HDC)' in main and
        'pCreatePen.Call(0, 2, uintptr(arrowColor))' in main and
        'pPolygon.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&points[0]))' not in main,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
