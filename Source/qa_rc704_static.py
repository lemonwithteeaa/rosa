from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
well = (root / "well_card_ui.go").read_text(encoding="utf-8")

show = main[main.index("func show("):main.index("func beginBusyLoading")]
registry = main[main.index("func addPageControl("):main.index("func isPersistentChromeControl")]
apply = main[main.index("func applyActivePageVisibility("):main.index("func activatePage(")]
activate = main[main.index("func activatePage("):main.index("func switchPage(")]

checks = {
    "version 704 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(704, 1000)),
    "logical visibility map exists": "pageControlVisible" in main,
    "show records page-local intent": "pageControlVisible[h] = yes" in show,
    "inactive page show is remembered but not leaked": "owner != currentPage" in show and "return" in show,
    "new controls default logically visible": "pageControlVisible[h] = true" in registry,
    "outgoing page isolation does not overwrite logical visibility": "func hidePageControls(page int)" in main and "show(h, false)" not in main[main.index("func hidePageControls("):main.index("func applyActivePageVisibility(")],
    "activation applies logical visibility": "commitPageVisibility(id)" in activate and "applyActivePageVisibility(page)" in main,
    "activation no longer force-shows whole page": "show(h, true)" not in activate,
    "active page apply respects hidden controls": "setPhysicalControlVisibility(h, wantVisible)" in apply and "if wantVisible" in apply,
    "well form explicitly hides nonactive sections": "visible := form && (i == 0 || wellFullCardMode) && wellSectionApplicable(i)" in well,
    "well form mutually excludes list and form": "manualList := hasKey && wellManualMode && !wellFormActive" in well and "form := hasKey && wellManualMode && wellFormActive" in well,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
