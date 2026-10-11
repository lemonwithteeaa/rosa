from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
checks = {
    "version 715": 'const appBuildVersion = "7.3.11.715-RC"' in main,
    "initial hint width expanded": 'py(656), 760, 16' in report,
    "dynamic hint gets extra width": 'hintW := min32(contentW, actionsW+140)' in report,
    "hint stays right anchored": 'hintX := right - hintW' in report and 'setPos(resultDecisionHint, hintX, decisionY, hintW, 16)' in report,
}
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if not all(checks.values()):
    raise SystemExit("RC715 hint layout failed")
print(f"{len(checks)}/{len(checks)} PASS")
