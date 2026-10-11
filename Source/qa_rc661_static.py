from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
assessment = (root / "assessment_classification.go").read_text(encoding="utf-8")
forecast = (root / "methods_forecast_details.go").read_text(encoding="utf-8")
import_report = (root / "laboratory_import_report_dialog.go").read_text(encoding="utf-8")
failure_report = (root / "well_failure_report_dialog.go").read_text(encoding="utf-8")
archive = (root / "methods_plan_archive.go").read_text(encoding="utf-8")

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    "version 661 or newer": bool(m) and int(m.group(1)) >= 661,
    "assessment object uses a canonical readable formatter":
        'labelText = assessmentObjectCaption(st, key)' in main and
        'parts = append(parts, "скв. "+well)' in assessment and
        'return strings.Join(parts, " / ")' in assessment,
    "assessment columns reserve native scrollbar width":
        'usable := max32(620, width-44)' in assessment,
    "approved classification history remains visible":
        'historyVisible := true' in assessment and 'historyVisible := h >=' not in assessment,
    "no-laboratory action has compact deterministic geometry":
        'The no-laboratory fallback has one action only' in assessment and
        'setPos(assessAcceptTechButton, right-buttonW, editorY, buttonW, buttonH)' in assessment,
    "assessment tabs no longer compete in red":
        'br = brushNav' in main[main.index('assessmentTab :='):main.index('if !nav && primary')],
    "Esc closes owned information windows":
        'func handleModalEscape' in main and 'message.WParam != VK_ESCAPE' in main,
    "forecast detail has no duplicate close footer":
        'modalCreate("BUTTON", "ЗАКРЫТЬ"' not in forecast and
        'editH := max32(120, h-margin-editY)' in forecast,
    "import report has no duplicate close button":
        'modalCreate("BUTTON", "Закрыть"' not in import_report,
    "failure report has no duplicate close button":
        'modalCreate("BUTTON", "Закрыть"' not in failure_report,
    "archive detail has no duplicate close button":
        'modalCreate("BUTTON", "ЗАКРЫТЬ"' not in archive,
}

for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
print(f"{sum(checks.values())}/{len(checks)} PASS")
raise SystemExit(0 if all(checks.values()) else 1)
