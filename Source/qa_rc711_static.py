from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

build_ui = report[report.index("func buildResultWorkspaceUI()") : report.index("func resultObjectLabel")]
control_lines = "\n".join(line for line in build_ui.splitlines() if "= button(" in line)
single_line = report[report.index("func isResultSingleLineButton") : report.index("func applyResultListReferenceStyle")]
layout = report[report.index("func layoutResultWorkspace") :]

checks = {
    "version 711": 'const appBuildVersion = "7.3.11.711-RC"' in main,
    "method card caption is singular": '"\u041a\u0410\u0420\u0422\u041e\u0427\u041a\u0410 \u041c\u0415\u0422\u041e\u0414\u0410"' in control_lines and '"\u041a\u0410\u0420\u0422\u041e\u0427\u041a\u0418 \u041c\u0415\u0422\u041e\u0414\u041e\u0412"' not in control_lines,
    "variant buttons equal and wide": ("addW, copyW, compareW := int32(208), int32(208), int32(208)" in layout) or ("copyW, compareW := int32(236), int32(236)" in layout),
    "variant captions stay centred single-line": all(name in single_line for name in (
        "resultCopyVariantButton", "resultCompareButton"
    )) and "DT_CENTER | DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX" in main,
    "add caption has no visual offset plus": ('button(ID_RESULT, "\u0414\u041e\u0411\u0410\u0412\u0418\u0422\u042c \u0412\u0410\u0420\u0418\u0410\u041d\u0422"' in build_ui) or ('resultAddVariantButton = 0' in report),
    "selection hint exists": "\u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0441\u0442\u0440\u043e\u043a\u0443 \u0441\u0446\u0435\u043d\u0430\u0440\u0438\u044f" in build_ui and "resultDecisionHint" in layout,
    "selection hint is visually quiet": "child == resultDecisionHint" in main and "rgb(116, 128, 140)" in main,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
