from pathlib import Path

root = Path(__file__).resolve().parent
raw = (root/'report_section_snapshot.go').read_text(encoding='utf-8')
word = (root/'laboratory_report_export.go').read_text(encoding='utf-8')
calc = (root/'report_excel_calculations.go').read_text(encoding='utf-8')
checks = {
    'inactive model fields excluded from visible raw data': 'reportLaboratoryFieldBelongsToModel(key, pass.ScaleModel)' in raw,
    'known extra fields get catalog labels and units': 'allFields[field.Key] = field' in raw and 'reportLaboratoryIndicatorLine(label, strings.TrimSpace(values[key]), unit)' in raw,
    'value and unit use a nonbreaking space': 'strings.ReplaceAll(unit, " ", "\\u00a0")' in raw,
    'Word uses Times New Roman 12 pt with single paragraph spacing': 'w:ascii="Times New Roman"' in word and 'w:sz w:val="24"' in word and 'w:line="240"' in word,
    'headings and indicator names use restrained emphasis': '<w:b/>' in word and '<w:i/>' in word and '<w:keepNext/><w:keepLines/>' in word,
    'Excel sheets split label value unit into columns': 'xlsxInlineCell(fmt.Sprintf("C%d", row), unit, 0)' in raw,
    'Excel uses formula cells for concentration and ionic strength': 'calculationFormulaCell("E"+ref, molarFormula' in calc and 'SUM(F%d:F%d)' in calc,
    'Excel reads the archived section rather than current laboratory': 'reportLabSnapshotInputs(fullReportSection(r, "section_2"))' in calc,
    'workbook requests full formula recalculation': 'forceFullCalc="1"' in raw,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
if not all(checks.values()):
    raise SystemExit(1)
