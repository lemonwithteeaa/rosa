from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

build_ui = report[report.index("func buildResultWorkspaceUI()") : report.index("func selectedResultScenario()")]
control_lines = "\n".join(line for line in build_ui.splitlines() if "= button(" in line)
single_line = report[report.index("func isResultSingleLineButton") : report.index("func applyResultListReferenceStyle")]
layout = report[report.index("func layoutResultWorkspace") :]

captions = (
    "КАРТОЧКИ МЕТОДОВ",
    "РАСЧЁТ ЭФФЕКТА",
    "СНИМОК АНАЛИЗА",
    "УТВЕРДИТЬ РЕШЕНИЕ",
)
legacy = (
    "ИНФОРМАЦИЯ ПО ВЫБРАННЫМ МЕРОПРИЯТИЯМ",
    "В ЭКОНОМИКУ",
    "СОХРАНИТЬ ПРОХОДКУ",
    "УТВЕРДИТЬ В РЕЕСТР",
)

checks = {
    "version 710": 'const appBuildVersion = "7.3.11.710-RC"' in main,
    "four distinct action captions": all(f'"{caption}"' in build_ui for caption in captions),
    "legacy captions removed from live controls": all(f'"{caption}"' not in control_lines for caption in legacy),
    "decision actions preserve wrapped long captions": all(name not in single_line for name in (
        "resultSelectedInfoButton", "resultApproveButton"
    )) and "resultUseSelectedButton" in single_line,
    "decision buttons receive 50 px target": "min32(50, max32(44, decisionH-4))" in layout,
    "wrapped owner draw measures before painting": "DT_WORDBREAK | DT_NOPREFIX" in main and "DT_CALCRECT" in main,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
