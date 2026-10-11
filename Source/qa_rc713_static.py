from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
lab = (root / "laboratory.go").read_text(encoding="utf-8")

switch_page = lab[lab.index("func switchLabFieldsPage"):lab.index("func switchLabGroup")]
switch_group = lab[lab.index("func switchLabGroup(code string, refresh bool)"):lab.index("func switchLabGroupByID")]
layout = lab[lab.index("func layoutLaboratory"):]
atomic = lab[lab.index("func beginLaboratoryAtomicUpdate"):lab.index("func switchLabFieldsPage")]

checks = {
    "version 713 or newer": any(f'const appBuildVersion = "7.3.11.{n}-RC"' in main for n in range(713, 1000)),
    "laboratory visual transaction avoids native redraw toggles": "beginPageVisualUpdate(ID_LAB)" in atomic and "endPageVisualUpdate(ID_LAB)" in atomic and "WM_SETREDRAW" not in atomic,
    "transaction respects outer page switch": "pageSwitching" in atomic and "currentPage != ID_LAB" in atomic,
    "field-page switch is atomic": "beginLaboratoryAtomicUpdate()" in switch_page and "defer endLaboratoryAtomicUpdate(atomic)" in switch_page,
    "complication switch is atomic": "beginLaboratoryAtomicUpdate()" in switch_group and "defer endLaboratoryAtomicUpdate(atomic)" in switch_group,
    "new complication opens first indicator page": "previousGroup := labActiveGroup" in switch_group and "previousGroup != code" in switch_group and "labFieldsPage = 0" in switch_group,
    "layout redraw is transaction aware": "laboratoryRedraw(RDW_INVALIDATE | RDW_ALLCHILDREN)" in layout or "laboratoryRedraw(RDW_INVALIDATE | RDW_ERASE | RDW_ALLCHILDREN)" in layout,
    "dynamic caption wrapper exists": "func laboratoryWrapFieldCaptionForWidth" in lab and "laboratoryCaptionTokenChunks" in lab,
    "layout uses actual label width": "labelWidth := max32(120, colW-34)" in layout and "laboratoryWrapFieldCaptionForWidth" in layout,
    "caption geometry is measured before placement": "laboratoryFieldLabelHeightForWidth" in layout and "fieldCaptions := [10]string{}" in layout,
    "long captions get compact font": "if ownH > 42 && hFontHeaderSmall != 0" in layout,
    "top laboratory actions have safe height": lab.count("TopActionH: 42") >= 2 and "TopActionH: 44" in lab,
}

failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(("PASS" if ok else "FAIL") + ": " + name)
if failed:
    raise SystemExit(f"{len(failed)}/{len(checks)} failed: {', '.join(failed)}")
print(f"{len(checks)}/{len(checks)} PASS")
