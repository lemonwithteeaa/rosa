from pathlib import Path

root = Path(__file__).resolve().parent
report = (root / 'report_workspace.go').read_text(encoding='utf-8')
detail = (root / 'report_detailed_export.go').read_text(encoding='utf-8')
lab = (root / 'report_section_snapshot.go').read_text(encoding='utf-8')
export = report[report.index('func runApprovedRegistryExport('):report.index('func runApprovedRegistryExportSafely(')]
checks = {
    'old teaching record is enriched before composing document': export.index('enrichApprovedRegistryRecordDetached(r)') < export.index('approvedRegistryDetailedReport(r)'),
    'all seven section builders are isolated': all(f'func() string {{ return reportSection{i}' in detail for i in (1, 2, 3, 4, 5, 6, 7)),
    'laboratory fixture fallback includes every recorded value': 'for key := range pass.Groups[code]' in detail and 'teachingCompleteLabDraft(&pass)' in detail,
    'calculations are isolated per direction': 'for i := range calculationBasisSections' in detail and 'reportSectionSafely(fmt.Sprintf("calculation_%d"' in detail,
    'worker skips live economics controls': 'reportSection6EconomicsDetached(key, scenario)' in detail and '_ = includeUI' in lab and 'getText(econScenarioCard)' not in lab[lab.index('func reportSection6EconomicsWithUI'):lab.index('func reportSection7Final')],
    'xlsx receives populated section record': 'writeFullReportSectionXLSX(path, r, true)' in report and 'r = enrichApprovedRegistryRecordDetached(r)' in export,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
if not all(checks.values()):
    raise SystemExit(1)
