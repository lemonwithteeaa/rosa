from pathlib import Path
root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
checks = {
    "version 719": 'const appBuildVersion = "7.3.11.719-RC"' in main,
    "variant helper exists": "func isResultVariantCaptionButton" in report,
    "only three upper buttons in helper": "case resultAddVariantButton, resultCopyVariantButton, resultCompareButton:" in report,
    "extra optical lift applied": "if isResultVariantCaptionButton(h)" in main and main.count("textRect.Top -= 2") >= 2 and main.count("textRect.Bottom -= 2") >= 2,
    "base RC718 lift preserved": "if isResultRaisedCaptionButton(h)" in main,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(("PASS" if v else "FAIL") + " - " + k)
if failed: raise SystemExit("RC719 layout QA failed: " + ", ".join(failed))
