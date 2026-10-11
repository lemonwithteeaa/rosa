from pathlib import Path

root=Path(__file__).resolve().parent
raw=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
word=(root/'laboratory_report_export.go').read_text(encoding='utf-8')
workspace=(root/'report_workspace.go').read_text(encoding='utf-8')
calc=(root/'report_excel_calculations.go').read_text(encoding='utf-8')
checks={
 'shared fields use definitions and skip repeated values': 'reportSharedFieldDefinition(key)' in raw and 'reportSharedValueRepeatedInGroup(pass, key, value)' in raw,
 'unfilled fields are appended at the end': 'reportMissingLaboratoryFields(fullReportSection(r, "section_2"))' in raw and 'ПРИЛОЖЕНИЕ 1. НЕЗАПОЛНЕННЫЕ' in raw,
 'Excel appendix is the last sheet': '"Расчёты", "Приложение 1"' in raw,
 'Word hierarchy uses bold/italic headings and centered report title': '<w:b/>' in word and '<w:i/>' in word and 'w:jc w:val="center"' in word,
 'PDF fallback Excel uses section workbook': 'fallback.XLSXErr = writeFullReportSectionXLSX' in workspace,
 'formula results and expressions are both in workbook': 'calculationFormulaCell("F"+ref, formula' in calc and 'xlsxInlineCell("G"+ref, "="+formula' in calc,
}
for label, ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+label)
if not all(checks.values()): raise SystemExit(1)
