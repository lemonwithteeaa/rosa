from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
flow = (root/'methods_workflow.go').read_text(encoding='utf-8')
detail = (root/'methods_forecast_details.go').read_text(encoding='utf-8')

m = re.search(r'const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main)
checks = {
    'version 649 or newer': bool(m) and int(m.group(1)) >= 649,
    'four forecast cards have command ids': all(x in flow for x in ['ID_METHODS_FORECAST_BASE', 'ID_METHODS_FORECAST_GAIN', 'ID_METHODS_FORECAST_RESULT', 'ID_METHODS_FORECAST_BASIS']),
    'forecast cards notify parent on click': 'SS_NOTIFY' in flow and 'methodsClickableCenteredCardLabel' in flow,
    'all four forecast cards use clickable helper': flow.count('methodsClickableCenteredCardLabel(') >= 5,
    'forecast clicks open detailed dialog': 'openMethodsForecastDetail(id)' in flow,
    'visible click hint exists': 'Нажмите на карточку для подробной расшифровки' in flow,
    'baseline detail explains 36 month cycles': 'Фактические завершённые циклы, вошедшие в базу' in detail and 'P25/P75' in detail,
    'gain detail explains residual risk': 'остаточный риск' in detail and 'коэффициентом 0,65' in detail,
    'result detail exposes formulas': 'РАСЧЁТ ПО ТЕКУЩЕМУ КОМПЛЕКСУ' in detail and '× (1 + %.0f%%)' in detail,
    'basis detail lists safeguards': 'ЧТО РОСА НЕ ДЕЛАЕТ' in detail and 'не складывает проценты методов напрямую' in detail,
    'detail dialog is scrollable': 'RichEdit20W' in detail and 'WS_VSCROLL' in detail and 'ES_READONLY' in detail,
    'archive/current calculation separated': 'архивное утверждение' in detail and 'текущий комплекс' in detail,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
