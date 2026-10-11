from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
snap=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
teach=(root/'teaching_alternative_scenarios.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
checks={
 'version 733': 'const appBuildVersion = "7.3.11.733-RC"' in main,
 'snapshot schema v2': 'RC733-SECTIONS-V2' in snap,
 'lab raw indicators': 'ИСХОДНЫЕ ЛАБОРАТОРНЫЕ ПОКАЗАТЕЛИ' in snap and 'reportLaboratoryRawData(pass)' in snap,
 'lab qc passport': 'laboratoryPassQCText(pass, time.Now())' in snap,
 'lab export qc': 'laboratoryExportQCText(pass, time.Now())' in snap,
 'lab visual summary': 'laboratoryReportVisualizationForPass(pass)' in snap,
 'lab brief and sources': 'laboratoryReportBrief(pass)' in snap and 'laboratoryReportSourceRows(pass)' in snap,
 'lab full formulas': 'calculationPassReport(pass, 0, true)' in snap and 'РАСЧЁТЫ, ФОРМУЛЫ, ПОДСТАНОВКА И РЕЗУЛЬТАТЫ' in snap,
 'method per-method detail': 'reportMethodAssessmentDetail(a)' in snap,
 'method reliability and probability': 'Достоверность объектовой оценки' in snap and 'Вероятность подтверждения эффекта' in snap,
 'method implementation': 'methodImplementationReportText' in snap,
 'method engineering card': 'methodEngineeringReferenceText(method)' in snap,
 'method practice card': 'methodsPracticeDetailedCard(method)' in snap,
 'parallel variants': 'ПАРАЛЛЕЛЬНЫЕ ВАРИАНТЫ ДЛЯ СРАВНЕНИЯ' in snap,
 'runlife transparent deltas': 'Прирост к базовому МРП' in snap,
 'economics formulas': 'РАСЧЁТНАЯ ЛОГИКА' in snap and 'Benefit/Cost = валовая выгода' in snap,
 'teaching legacy migration': 'len(record.Sections) == 0 || record.SnapshotVersion != fullReportSectionSnapshotVersion' in teach,
 'teaching new rows save sections': 'Sections:' in teach and 'buildFullReportSectionSnapshot(objectKey, scenarioRecord, snapshot)' in teach,
 'approved pdf docx detailed route': 'report := approvedRegistryDetailedReport(r)' in report,
 'approved xlsx section workbook': 'writeFullReportSectionXLSX(path, r, true)' in report,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC733 detailed export QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
