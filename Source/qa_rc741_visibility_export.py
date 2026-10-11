from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
lab = (root / "laboratory.go").read_text(encoding="utf-8")
report = (root / "report_workspace.go").read_text(encoding="utf-8")

end_lab = lab[lab.index("func endLaboratoryAtomicUpdate"):lab.index("func laboratoryRedraw")]
end_list = main[main.index("func listEndUpdate"):main.index("func listClear")]
export = report[report.index("func runApprovedRegistryExport("):report.index("func setApprovedRegistryExportBusy")]

checks = {
    "laboratory finishes through the shared page visual transaction":
        "endPageVisualUpdate(ID_LAB)" in end_lab and "WM_SETREDRAW" not in lab[lab.index("func beginLaboratoryAtomicUpdate"):lab.index("func laboratoryRedraw")],
    "shared page transaction reconciles visibility before repaint":
        main.index("applyActivePageVisibility(page)", main.index("func endPageVisualUpdate")) < main.index("invalidateVisibleFrame(page)", main.index("func endPageVisualUpdate")),
    "list refresh does not alter native visibility":
        "WM_SETREDRAW" not in end_list and "pInvalidateRect.Call" in end_list,
    "registry export enriches sparse teaching records safely":
        "r = enrichApprovedRegistryRecordDetached(r)" in export and "report := approvedRegistryDetailedReport(r)" in export,
    "export still validates the final file": "writeApprovedRegistryExportDirectValidated" in export,
}
for name, passed in checks.items():
    print(("PASS" if passed else "FAIL") + ": " + name)
print(f"{sum(checks.values())}/{len(checks)} PASS")
if not all(checks.values()):
    raise SystemExit(1)
