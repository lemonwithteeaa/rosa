from pathlib import Path

text = (Path(__file__).parent / "report_workspace.go").read_text(encoding="utf-8")
checks = {
    "title lifted": "setPos(resultVariantsTitle, x, variantTitleY-2, variantTextW, 26)" in text,
    "compact title protected": "setPos(resultVariantsTitle, x, variantTitleY-1, variantTextW, 26)" in text,
    "subtitle remains below title": "setPos(resultVariantsSubtitle, x, variantTitleY+24, variantTextW, 22)" in text,
}
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if not all(checks.values()):
    raise SystemExit(1)
