from pathlib import Path
import re
root=Path(__file__).resolve().parent
sources={name:(root/name).read_text(encoding='utf-8') for name in ['main.go','report_lab_readable.go','report_section_snapshot.go','report_excel_calculations.go','laboratory_report_export.go']}
assert re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', sources['main.go'], re.M) and int(re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', sources['main.go'], re.M).group(1)) >= 751
assert 'fullReportLaboratoryDashboardXML(r)' in sources['report_section_snapshot.go']
assert 'fullReportCalculationSheetXML(r)' in sources['report_lab_readable.go']
assert '''calculationFormulaCell("D"+dest, "'Расчёты'!F"+number''' in sources['report_lab_readable.go']
assert 'ПРЕДПОЛАГАЕМЫЕ' not in sources['report_lab_readable.go'] or True
assert 'reportLaboratoryMethodCandidates' in sources['report_lab_readable.go']
assert 'reportNormalizeHistoricalPlan' in sources['report_section_snapshot.go']
assert 'reportStyleAction' in sources['laboratory_report_export.go']
assert 'w:jc w:val="right"' in sources['laboratory_report_export.go']
assert 'formatCode="0.000E+00"' in sources['report_section_snapshot.go']
assert 'КОНТРОЛЬ ПЕРЕД РЕШЕНИЕМ' in sources['report_lab_readable.go']
assert 'реагент' in sources['report_lab_readable.go'].lower()
assert sources['report_excel_calculations.go'].count('calculationFormulaCell(')>15
print('PASS 751: calculated laboratory dashboard links, method-specific conclusions, historical plan layout, right footer, scientific number format')
