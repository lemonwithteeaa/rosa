from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
flow = (root / "methods_workflow.go").read_text(encoding="utf-8")
background = (root / "methods_background.go").read_text(encoding="utf-8")
forecast = (root / "methods_forecast_details.go").read_text(encoding="utf-8")
version = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
frame = main[main.index("buttonFrame := di.RcItem"):main.index("if h == hHeaderBack", main.index("buttonFrame := di.RcItem"))]

checks = {
    "build version 668": bool(version) and int(version.group(1)) == 668,
    "button border is inset two pixels on every edge": all(
        x in frame for x in ["buttonFrame.Left += 2", "buttonFrame.Top += 2",
                             "buttonFrame.Right -= 2", "buttonFrame.Bottom -= 2"]),
    "all owner-drawn buttons use the inset frame": "pFrameRect.Call(uintptr(di.HDC), uintptr(unsafe.Pointer(&buttonFrame)), uintptr(frameBrush))" in frame,
    "methods layout retains original toolbar geometry":
        "toolbarReserve, toolbarH := int32(42), int32(38)" in flow and
        "toolbarReserve, toolbarH = 34, 30" in flow and
        "implementationButtonH := min32(36, max32(16, implAvailable))" in flow,
    "method background behavior is unchanged": "cacheKey := methodsObjectIdentityKey(key)" not in background,
    "scenario cards show negative, real and positive values": all(
        x in flow for x in ["Негативный прогноз", "Реальный прогноз", "Положительный прогноз",
                            "fmt.Sprintf(\"Негативный прогноз\\r\\n%.0f сут.\", low)",
                            "fmt.Sprintf(\"Реальный прогноз\\r\\n%.0f сут.\", calc)",
                            "fmt.Sprintf(\"Положительный прогноз\\r\\n%.0f сут.\", high)"]),
    "scenario detail dialogs match their cards": all(
        x in forecast for x in ["РОСА — Негативный прогноз", "РОСА — Реальный прогноз",
                                "РОСА — Положительный прогноз"]),
    "workbench grows while adapting to short windows": "workspaceMaxH = 150" in flow and
        "workspaceH := max32(100, min32(workspaceMaxH," in flow,
    "section headings are smaller than the body value scale":
        "fitMethodsControlText(handle, methodsTypeBody, true, 0)" in
        (root / "methods_typography.go").read_text(encoding="utf-8"),
}
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
print(f"{sum(checks.values())}/{len(checks)} PASS")
raise SystemExit(0 if all(checks.values()) else 1)
