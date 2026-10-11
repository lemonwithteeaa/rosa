from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
stability = (root / "stability_frame_windows.go").read_text(encoding="utf-8")

show = main[main.index("func show("):main.index("func beginBusyLoading")]
registry = main[main.index("func addPageControl("):main.index("func label(")]
activate = main[main.index("func activatePage("):main.index("func switchPage(")]

checks = {
    "version 703 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(703, 1000)),
    "unowned main controls cannot be shown on a page; modal children can": "!isPersistentChromeControl(h)" in show and "parent == uintptr(hMain)" in show and "UI refused unowned main control show" in show,
    "page control registry rejects cross-page ownership": "UI control owner conflict" in registry,
    "page registry deduplicates handles": "pageControls[page] = append(pageControls[page], h)" in registry,
    "persistent shell explicitly whitelisted": "func isPersistentChromeControl" in main and "for _, nav := range navButtons" in main,
    "tab switch only updates outgoing and incoming page controls": "hidePageControls(previousPage)" in activate and "commitPageVisibility(id)" in activate and "sweepInactivePageControls" not in activate,
    "per-navigation EnumChildWindows scan removed": "pEnumChildWindows.Call" not in main and "stablePageSweepProc" not in stability,
    "manual page append uses owner-aware registry": "addPageControl(ID_TECH, factEventType)" in main and "pageControls[ID_TECH] = append" not in main,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
