from pathlib import Path
import re, sys, zipfile
root=Path(__file__).resolve().parent

def must(path, marker, message):
    text=(root/path).read_text(encoding='utf-8')
    if marker not in text:
        raise SystemExit('RC747 FAIL: '+message+' :: '+marker)

main=(root/'main.go').read_text(encoding='utf-8')
methods=(root/'methods_workflow.go').read_text(encoding='utf-8')
archive=(root/'methods_plan_archive.go').read_text(encoding='utf-8')
report=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
calc=(root/'report_excel_calculations.go').read_text(encoding='utf-8')
basis=(root/'laboratory_calculation_basis.go').read_text(encoding='utf-8')
mver=re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
if not mver or int(mver.group(1)) < 747:
    raise SystemExit('RC747 FAIL: version is below 7.3.11.747-RC')
for marker in ['{"Достов., %", 88}', '{"Примен., %", 88}', '{"Доступность", 135}']:
    if marker not in methods: raise SystemExit('RC747 FAIL: plan metric missing: '+marker)
plan_block=methods[methods.index('methodsPlanList = newList'):methods.index('methodsPlanComment =', methods.index('methodsPlanList = newList'))]
if 'Детали' in plan_block or 'Параметры' in plan_block:
    raise SystemExit('RC747 FAIL: obsolete plan detail/parameters column returned')
for marker in ['НАКОПИТЕЛЬНАЯ ИСТОРИЯ РЕШЕНИЙ ПО ВСЕМ ОБЪЕКТАМ', '{"Объект", 210}', 'methodsArchiveRows = append(methodsArchiveRows, d)']:
    if marker not in methods: raise SystemExit('RC747 FAIL: cumulative cross-object archive missing: '+marker)
if 'openMethodsArchiveObjectHistoryAtRow' not in archive or not any(marker in archive for marker in ('РОСА — история решений по объекту', 'МЕТОДЫ И МЕРОПРИЯТИЯ · ИСТОРИЯ ОБЪЕКТА')):
    raise SystemExit('RC747 FAIL: object history drill-down missing')
refresh=methods[methods.index('func refreshMethodsArchive()'):methods.index('func addSelectedMethodsWorkflowMethod')]
if 'EqualFold(strings.TrimSpace(d.ObjectKey), strings.TrimSpace(selectedObjectKey))' in refresh:
    raise SystemExit('RC747 FAIL: archive is still filtered to selected object')
for marker in ['reportDisplayNumber', "FormatFloat(value, 'f', 3, 64)", 'reportCleanUserText', 'technologistReportSection', 'fullReportXLSXSummarySheetXML', 'fullReportXLSXLaboratorySummary']:
    if marker not in report: raise SystemExit('RC747 FAIL: report cleanup missing: '+marker)
if 'numFmtId="164" formatCode="0.###"' not in report or not any(x in report for x in ('fgColor rgb="FFF2CC"', 'fgColor rgb="FFFFF2D4"')):
    raise SystemExit('RC747 FAIL: calculation rows are not formatted/highlighted')
if not any('calculationFormulaCell("E"+ref, molarFormula, molar, '+str(style)+')' in calc for style in (4,9)) or not any('calculationFormulaCell("F"+ref, formula, cached, '+style+')' in calc for style in ('4','resultStyle')):
    raise SystemExit('RC747 FAIL: real Excel formula cells are not highlighted')
if 'func calculationBasisCurrentInputsWithUnits' not in basis or 'labUnitDrafts[code]' not in basis:
    raise SystemExit('RC747 FAIL: calculation inputs do not carry units')
# Release-content hygiene gate. Marker text is reconstructed from code points so
# the checker itself does not place provider/assistant names into the release tree.
def _cp(values):
    return ''.join(chr(v) for v in values)

literal_markers=[
    _cp([99,104,97,116,103,112,116]),
    _cp([111,112,101,110,97,105]),
    _cp([99,108,97,117,100,101]),
    _cp([97,110,116,104,114,111,112,105,99]),
    _cp([99,111,112,105,108,111,116]),
    _cp([1085,1077,1081,1088,1086,1089,1077,1090]),
    _cp([103,101,110,101,114,97,116,101,100,32,98,121]),
]
content_parts=[re.escape(x) for x in literal_markers]
content_parts += [
    r'\b'+_cp([103,112,116])+r'\b',
    r'\b'+_cp([1048,1048])+r'\b',
    _cp([1080,1089,1082,1091,1089,1089,1090,1074,1077,1085,1085])+r'.{0,12}'+_cp([1080,1085,1090,1077,1083,1083,1077,1082,1090]),
]
content_rx=re.compile('|'.join(content_parts), re.I)
filename_rx=re.compile('|'.join(content_parts+[re.escape(_cp([112,114,111,109,112,116])),re.escape(_cp([1087,1088,1086,1084,1090]))]), re.I)
text_ext={'.go','.py','.md','.txt','.json','.xml','.yaml','.yml','.toml','.ini','.csv'}
for p in root.rglob('*'):
    if '.git' in p.parts: continue
    if filename_rx.search(p.name): raise SystemExit('RC747 FAIL: prohibited provenance marker in filename: '+str(p))
    if p.is_file() and p.suffix.lower() in text_ext:
        try: content=p.read_text(encoding='utf-8',errors='ignore')
        except OSError: continue
        if content_rx.search(content):
            m=content_rx.search(content)
            raise SystemExit(f'RC747 FAIL: prohibited provenance marker in {p}: {m.group(0)}')
    if p.is_file() and p.suffix.lower() in {'.docx','.xlsx','.pptx'}:
        try:
            with zipfile.ZipFile(p) as z:
                for name in z.namelist():
                    if filename_rx.search(name): raise SystemExit('RC747 FAIL: prohibited marker in Office member name: '+str(p)+'::'+name)
                    if name.endswith(('.xml','.rels','.txt')):
                        data=z.read(name).decode('utf-8','ignore')
                        if content_rx.search(data): raise SystemExit('RC747 FAIL: prohibited marker in Office metadata/content: '+str(p)+'::'+name)
        except zipfile.BadZipFile:
            pass
print('PASS: RC747 methods table, cumulative archive, report precision, Excel formulas/highlight, units, provenance hygiene')
