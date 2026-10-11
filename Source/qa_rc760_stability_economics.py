from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
econ = (root / "economics_fund_archive.go").read_text(encoding="utf-8")

checks = {
    "page changes do not intercept Windows minimize commands":
        'SC_MINIMIZE' not in main and 'pageSwitchWasMaximized' not in main,
    "page change hides only outgoing controls and reveals incoming controls":
        'hidePageControls(previousPage)' in main and 'commitPageVisibility(id)' in main,
    "page switch no longer scans the application child tree":
        'sweepInactivePageControls' not in main and 'pEnumChildWindows.Call' not in main,
    "economics archive columns fit available dialog width":
        'layoutEconomicsFundArchiveColumns(w)' in econ and
        'available := max32(100, w-44)' in econ and
        'setListColumnWidth(economicsFundDialogList, i, columnWidth)' in econ,
    "economics archive keeps row detail interaction and read-only mode":
        'economicsFundDialogSelectScheme(index)' in econ and
        'LVS_REPORT|LVS_SINGLESEL|LVS_SHOWSELALWAYS' in econ and
        'LVS_EDITLABELS' not in econ and
        'hdr.Code == NM_CLICK || hdr.Code == NM_DBLCLK' in econ,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
