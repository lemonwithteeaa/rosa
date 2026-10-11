from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")
workflow = (root / "workflow_pages.go").read_text(encoding="utf-8")

build_ui = report[report.index("func buildResultWorkspaceUI()") : report.index("func resultObjectLabel")]
live_buttons = "\n".join(line for line in build_ui.splitlines() if "= button(" in line)
layout = report[report.index("func layoutResultWorkspace") :]

checks = {
    "version 712": 'const appBuildVersion = "7.3.11.712-RC"' in main,
    "snapshot button removed": "resultSaveButton = button(" not in live_buttons and "setPos(resultSaveButton" not in layout,
    "three decision actions remain": all(name in live_buttons for name in (
        "resultSelectedInfoButton", "resultUseSelectedButton", "resultApproveButton"
    )),
    "hint moved above actions": "setPos(resultDecisionHint, actionsX, decisionY, actionsW, 16)" in layout and "buttonY := decisionY + 16" in layout,
    "decision card receives expanded geometry": "decisionCardH := int32(104)" in layout and all(marker in layout for marker in (
        "decisionCardH = 90", "decisionCardH = 82"
    )),
    "tables retain prior vertical budget": "archiveY := archiveToolsY + archiveButtonH + 6" in layout and "max32(150, bottom-archiveY)" in layout,
    "refresh control is retained": 'resultRefreshButton = button(ID_RESULT, "\u041e\u0431\u043d\u043e\u0432\u0438\u0442\u044c \u0441\u0432\u043e\u0434\u043a\u0443"' in workflow,
    "refresh control is right anchored": "setPos(resultRefreshButton, right-190, top, 190, 34)" in layout,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
