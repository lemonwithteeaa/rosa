from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
methods = (root / "methods_reference_dialog.go").read_text(encoding="utf-8")
background = (root / "methods_background.go").read_text(encoding="utf-8")

checks = {
    "object history label stays navy while its value uses the red accent":
        'objectHeader := strings.HasPrefix(strings.ToUpper(t), "ОБЪЕКТ:")' in methods and
        'uintptr(labelEnd)' in methods and
        'CrTextColor: uint32(rgb(183, 20, 37))' in methods and
        'uintptr(lineStart+trimLen)' in methods,
    "implementation detail labels are bold while values remain outside the selection":
        'if strings.HasPrefix(t, "• ")' in methods and
        "colon := strings.IndexByte(t[2:], ':')" in methods and
        't[2:labelEnd]' in methods,
    "manual correction label is emphasized":
        '"Последняя ручная корректировка:"' in methods,
    "visibility fast path reads the live Win32 style, not a stale Go cache":
        'pGetWindowLongPtrW.Call(uintptr(h), ^uintptr(15))' in main and
        '(style&uintptr(WS_VISIBLE) != 0) == yes' in main and
        'physicalControlVisibilityKnown' not in main,
    "page sweep uses visibility helper and ListView refresh preserves visibility":
        'setPhysicalControlVisibility(h, false)' in main and
        'pSendMessageW.Call(uintptr(h), WM_SETREDRAW' not in main[main.index('func listBeginUpdate'):main.index('func listClear')] and
        'pShowWindow.Call(hwnd, SW_HIDE)' not in main,
    "method probability cohort still runs in the background":
        'go func() {' in background and
        background.index('go func() {') < background.index('prepareMethodProbabilityCohort(records, outcomeIndex)') and
        'WM_APP_METHODS_DONE' in background,
}

failed = []
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
    if not ok:
        failed.append(name)
print(f"{len(checks) - len(failed)}/{len(checks)} PASS")
if failed:
    raise SystemExit(1)
