from pathlib import Path
import re
root = Path(__file__).resolve().parent
number = (root/'report_numeric_tabs.go').read_text(encoding='utf-8')
chemical = (root/'report_chemical_overview.go').read_text(encoding='utf-8')
sections = (root/'report_section_snapshot.go').read_text(encoding='utf-8')
word = (root/'laboratory_report_export.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
assert re.search(r'^const appBuildVersion = "7\.3\.11\.(?:75[2-9]|7[6-9]\d|[89]\d\d)-RC"', main, re.M)
assert 'reportScenarioTabXML(r)' in sections and 'reportEconomyTabXML(r)' in sections
assert 'fullReportLaboratoryDashboardXML(r)' in sections and 'fullReportCalculationSheetXML(r)' in sections
assert 'numFmtId="166" formatCode="0.0%"' in sections
for value in ['Базовая оценка без нового мероприятия', 'Сдержанный сценарий', 'Расчётный сценарий', 'Благоприятный сценарий',
              'Целевой ориентир', '"B"+n+"-$B$6"', 'B10-B8', 'SUM(B6:B7)', 'B11-B12']:
    assert re.sub(r'\s+', '', value) in re.sub(r'\s+', '', number), value
assert 'fullReportSection(r, "section_5")' in number and 'fullReportSection(r, "section_6")' in number
assert 'reportAddChemicalOverview(text)' in sections
for value in ['Норма подачи / обработки:', 'Частота / режим подачи:', 'Химреагент:', 'База расчёта:', 'periodicity', 'rate', 'reagent']:
    assert value in chemical, value
assert 'w:jc w:val="right"' in word and 'w:instr="PAGE"' in word
assert 'Шаблон' not in number
print('PASS RC752: workbook calculated sheets, percent styles, chemical PDF/DOCX summaries and right page-number footer')
