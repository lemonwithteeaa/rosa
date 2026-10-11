from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
snap=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
checks={
 'version 726': 'const appBuildVersion = "7.3.11.726-RC"' in main,
 'record stores section snapshot': 'Sections        map[string]string' in report and 'SnapshotVersion string' in report,
 'snapshot built when pass is saved': 'buildFullReportSectionSnapshot(key, s, resultSnapshot)' in report,
 'all seven sections captured': all(f'sections["section_{i}"]' in snap for i in range(1,8)),
 'section1 pulls failure history': 'failuresForObject(objectKey, time.Now())' in snap and 'ПРЕДЫСТОРИЯ ОТКАЗОВ · 36 МЕС.' in snap,
 'section2 pulls full lab conclusion': 'buildLaboratoryPassConclusion(pass)' in snap,
 'section2 pulls calculated laboratory summary': 'reportLaboratoryCalculatedSummary(pass)' in snap and 'РАСЧЁТНЫЕ ПОКАЗАТЕЛИ' in snap,
 'section3 pulls approved assessment': 'assessmentFinalReportText(objectKey)' in snap,
 'section4 includes approved method decision': 'latestApprovedMethodDecision(objectKey)' in snap and 'MethodAssessments' in snap and 'Implementation' in snap,
 'section5 includes MRP survival detail': 'calculateMRPForecast(in)' in snap and 'Вероятность подтверждённого отказа к 365 сут.' in snap,
 'section6 captures compact Benefit Modeling': 'Расчётная трассировка' in snap and 'Benefit/Cost' in snap,
 'section7 stores full result summary': 'ПОЛНАЯ СВОДКА РАЗДЕЛА 7' in snap,
 'approved viewer uses section snapshot': 'storedSectionDetailedReport(r, true)' in report,
 'docx/pdf exports use detailed report': 'report := approvedRegistryDetailedReport(r)' in report,
 'xlsx has per-section workbook': ('writeFullReportSectionXLSX(path, r, true)' in report or 'writeFullReportSectionXLSX(tmp, r, true)' in report) and all(name in snap for name in ['1 Фонд','2 Лаборатория','3 Осложнения','4 Методы','5 Наработка','6 Экономика','7 Итог']),
 'saved section7 export also uses sections': 'storedSectionDetailedReport(r, strings.TrimSpace(r.ApprovedAt) != "")' in report,
 'archive export buttons use one-line contract': all(x in report for x in ['archiveExportPDFButton, archiveExportDOCXButton, archiveExportXLSXButton']),
 'archive export buttons have safe height': 'exportH := int32(42)' in report,
 'report cards have extra compact height': 'cardsH = 86' in report and 'cardsH = 72' in report,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC726 section export QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
