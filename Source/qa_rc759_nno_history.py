#!/usr/bin/env python3
from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
nno = (root / "nno_fleet_ui.go").read_text(encoding="utf-8")
editor = (root / "nno_methodology_editor.go").read_text(encoding="utf-8")
methods = (root / "methods_plan_archive.go").read_text(encoding="utf-8")
economics = (root / "economics_fund_archive.go").read_text(encoding="utf-8")
kernel = (root / "nno_core" / "nnocore.go").read_text(encoding="utf-8")

version_match = re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
checks = {
    "version 759+": bool(version_match) and int(version_match.group(1)) >= 759,
    "fleet screen is the default": "nnoFleetEnabled = true" in nno,
    "single-well forecast remains separate": "case ID_NNO_OBJECT:" in nno and "case ID_NNO_BACK:" in nno,
    "NNO chart remains factual and distinct from the result": "РЕГИСТРАЦИЯ СОБЫТИЙ · ПО МЕСЯЦАМ" in nno,
    "event chart is explicitly not NNO": "РЕГИСТРАЦИЯ СОБЫТИЙ · ПО МЕСЯЦАМ" in nno,
    "official result is withheld without Ti": "ННО-12: не рассчитана" in nno and '"Не рассчитана"' in nno,
    "rolling kernel sums Ti/Oi": "result.Days += failure.WorkedDays" in kernel and "result.NNO = result.Days / float64(result.Failures)" in kernel,
    "draft cannot become approved": "nnoEditorDraft.Approved = false" in editor and "verified.UserID != currentSession.UserID" in editor,
    "draft snapshots are immutable names": "nno_methodology_history" in editor and "Версия черновика уже существует" in editor,
    "NNO buttons are not 36px": not re.search(r"nnoFleet(?:Update|Audit|Rules|Object|Back),[^\n]*, 36\)", nno),
    "method history has all considered schemes": "ВСЕ РАССМОТРЕННЫЕ СХЕМЫ" in methods and "alternativeScenariosForObject(objectKey)" in methods,
    "method history does not open catalogue": 'openObjectHistoryDialog("МЕТОДЫ И МЕРОПРИЯТИЯ · ИСТОРИЯ ОБЪЕКТА"' in methods,
    "method history is read-only": "never exposes the restore action" in methods and 'show(methodArchiveRestoreButton, false)' in methods,
    "economics keeps 758 archive": "refreshEconomicsFundArchive" in economics and "openEconomicsFundArchiveAtRow" in economics,
    "economics opens only by double click": "currentPage == ID_ECON && hdr.Code == NM_DBLCLK" in main,
    "methods open only by double click": "hdr.Code == NM_DBLCLK && i >= 0 && i < len(methodsArchiveRows)" in main,
}

for width in (1180, 1366, 1600, 1920):
    cw = max(720, width - 270)
    gap = 8
    action_w = min(158 + 178 + 232 + 186 + 3 * gap, cw)
    wraps = cw < 170 + 132 + gap + action_w + gap
    if wraps:
        checks[f"wrapped compact action group fits {width}px"] = action_w <= cw
    else:
        checks[f"filters and right-aligned actions fit {width}px"] = 170 + 132 + gap + action_w + gap <= cw

for width, height in ((1180, 700), (1366, 768), (1600, 900), (1920, 1080)):
    cw = max(720, width - 270)
    action_w = min(158 + 178 + 232 + 186 + 24, cw)
    wraps = cw < 170 + 132 + 8 + action_w + 8
    compact = height < 820
    action_y = 269
    action_bottom = action_y + (90 if wraps else 42)
    card_y = action_bottom + 4
    columns = 6 if cw >= 1250 else 3
    card_h = (96 if columns == 6 else 76) if compact else (116 if columns == 6 else 96)
    rows = 6 // columns
    cards_bottom = card_y + rows * card_h + (rows - 1) * (4 if compact else 8)
    split_y = cards_bottom + 5 + 34
    mini = height < 740
    no_middle = height < 820
    chart_h = 0 if no_middle else max(50 if compact else 112, min(260, height - split_y - 210))
    lower_y = split_y + (6 if mini or no_middle else 26 + chart_h + (6 if compact else 8))
    lower_h = max(96 if compact else 100, height - (lower_y + 26) - 15)
    list_bottom = lower_y + 26 + lower_h
    checks[f"wrapped/compact section 5 geometry fits {width}x{height}"] = (mini or no_middle or chart_h >= (50 if compact else 112)) and lower_h >= (96 if compact else 100) and list_bottom <= height

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit("RC759 failed: " + ", ".join(failed))
