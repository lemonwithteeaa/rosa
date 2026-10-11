from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
lab = (root / "laboratory.go").read_text(encoding="utf-8")

atomic = lab[lab.index("func beginLaboratoryAtomicUpdate"):lab.index("func laboratoryRedraw")]
page_batch = main[main.index("func beginPageVisualUpdate"):main.index("func layoutCurrentPage")]
group_by_id = lab[lab.index("func switchLabGroupByID"):lab.index("func saveLaboratoryRecord")]
object_change = lab[lab.index("func handleLaboratoryObjectChange"):lab.index("func techStateForLaboratoryObject")]
analyze = lab[lab.index("func analyzeLaboratoryInputs"):lab.index("func chooseLaboratoryReportPath")]
toggle = lab[lab.index("func toggleLaboratoryConclusion"):lab.index("func normalizeLabHeader")]
list_end = main[main.index("func listEndUpdate"):main.index("func listClear")]
set_pos = main[main.index("func setPos"):main.index("func layoutDashboardFundColumns")]

checks = {
    "version 714 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(714, 1000)),
    "713 regression tag retained": 'RC713 baseline tag retained for regression QA' in main,
    "laboratory changes are prepared as one page batch": "beginPageVisualUpdate(ID_LAB)" in atomic and "endPageVisualUpdate(ID_LAB)" in atomic,
    "page batching never disables native redraw": "WM_SETREDRAW" not in atomic and "pSendMessageW.Call(uintptr(hMain), WM_SETREDRAW" not in page_batch,
    "page visibility reconciled before deferred frame": page_batch.index("applyActivePageVisibility(page)") < page_batch.index("invalidateVisibleFrame(page)") and "RDW_UPDATENOW" not in page_batch,
    "nested list refresh cannot thaw laboratory": "WM_SETREDRAW" not in list_end and "pInvalidateRect.Call" in list_end,
    "laboratory geometry preserves native invalidation": "SWP_NOZORDER)" in set_pos and "frameNoRedraw" not in set_pos,
    "repeated active complication click is no-op": 'g.Code == labActiveGroup && g.Code != "overall"' in group_by_id,
    "object change is atomic": "beginLaboratoryAtomicUpdate()" in object_change and "endLaboratoryAtomicUpdate(atomic)" in object_change,
    "analysis expansion is atomic": "beginLaboratoryAtomicUpdate()" in analyze and "endLaboratoryAtomicUpdate(atomic)" in analyze,
    "conclusion expansion is atomic": "beginLaboratoryAtomicUpdate()" in toggle and "endLaboratoryAtomicUpdate(atomic)" in toggle,
    "object change no longer forces independent main redraw": "pRedrawWindow.Call(uintptr(hMain)" not in object_change,
    "analysis no longer forces independent main redraw": "pRedrawWindow.Call(uintptr(hMain)" not in analyze,
    "conclusion no longer forces independent main redraw": "pRedrawWindow.Call(uintptr(hMain)" not in toggle,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
