from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
word=(root/'laboratory_report_export.go').read_text(encoding='utf-8')
snap=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
calc=(root/'report_excel_calculations.go').read_text(encoding='utf-8')
ver=re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
if not ver or int(ver.group(1)) < 750:
    raise SystemExit('RC750 FAIL: version below 750')
footer=word[word.index('footer :='):word.index('return writeReportZip', word.index('footer :='))]
if 'w:jc w:val="right"' not in footer or 'w:instr="PAGE"' not in footer:
    raise SystemExit('RC750 FAIL: footer is not a right-aligned PAGE field')
if 'РОСА' in footer or 'страниц' in footer.lower():
    raise SystemExit('RC750 FAIL: footer branding/page word returned')
for marker in [
    'RC750-SECTIONS-V4', 'compactLaboratorySectionForExport', 'reportLaboratoryKeyInputs',
    'reportLaboratoryCalculatedSummary', 'КЛЮЧЕВЫЕ ИСХОДНЫЕ ДАННЫЕ', 'РАСЧЁТНЫЕ ПОКАЗАТЕЛИ',
    'Расчётная трассировка: затраты = CAPEX + OPEX',
]:
    if marker not in snap:
        raise SystemExit('RC750 FAIL: compact report marker missing: '+marker)
method_block=snap[snap.index('func reportSection4Methods'):snap.index('func reportSection5Runlife')]
if method_block.count('ПАРАМЕТРЫ РЕАЛИЗАЦИИ') != 1:
    raise SystemExit('RC750 FAIL: duplicated method implementation heading')
if 'formatMethodForecast(decision)' in method_block:
    raise SystemExit('RC750 FAIL: method section repeats full MRP forecast from section 5')
if 'Предыстория 36 мес.' in method_block or 'Ограничение прогноза:' in method_block:
    raise SystemExit('RC750 FAIL: method section repeats runlife history/forecast limitations')
section7=snap[snap.index('func reportSection7Final'):snap.index('func fullReportSection')]
if 'Основание выбора:' in section7 or 'Чистая выгода:' in section7 or 'Прогноз:' in section7:
    raise SystemExit('RC750 FAIL: final section repeats detail already owned by sections 4-6')
econ=snap[snap.index('func reportSection6EconomicsWithUI'):snap.index('func reportSection7Final')]
for duplicated in ['СЦЕНАРИЙ И ИСХОДНЫЕ ДАННЫЕ', 'РАСЧЁТНЫЙ ВЫВОД BENEFIT MODELING']:
    if duplicated in econ:
        raise SystemExit('RC750 FAIL: verbose economics duplicate returned: '+duplicated)
for marker in [
    'Ионная сила I', 'Баланс зарядов, %', 'Коэффициент активности γ₁', 'Коэффициент активности γ₂',
    ' · Ksp', ' · IAP', ' · Кнас', 'индекс насыщения SI', 'LOG10(F',
    'Средняя скорость коррозии по пластинке', 'Эффективность ингибиторной защиты',
    'Запас толщины стенки', 'Локальная скорость по язве', 'Температурный запас ΔT',
    'Расчётная жёсткость Ca²⁺+Mg²⁺', 'Расхождение TDS и суммы ионов', 'Коэффициент Сулина r(Na+K)/rCl', 'Тип воды по В. А. Сулину',
]:
    if marker not in calc:
        raise SystemExit('RC750 FAIL: calculation workbook marker missing: '+marker)
if calc.count('calculationFormulaCell(') < 10:
    raise SystemExit('RC750 FAIL: calculation workbook lost real Excel formulas')
if not any(x in snap for x in ('fgColor rgb="FFF2CC"','fgColor rgb="FFFFF2D4"')) or 'numFmtId="164" formatCode="0.###"' not in snap:
    raise SystemExit('RC750 FAIL: calculated rows are not highlighted/formatted')
print('PASS: RC750 compact PDF/DOCX structure, right page footer, laboratory calculation chain, highlighted Excel formulas')
