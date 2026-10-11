from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
ui=(root/'mrp_forecast_ui.go').read_text(encoding='utf-8')
model=(root/'mrp_scenarios.go').read_text(encoding='utf-8')
forecast=(root/'mrp_forecast.go').read_text(encoding='utf-8')
report=(root/'report_section_snapshot.go').read_text(encoding='utf-8')
workspace=(root/'report_workspace.go').read_text(encoding='utf-8')

m=re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
if not m or int(m.group(1)) < 748:
    raise SystemExit('RC748 FAIL: app version below 748-RC')
for marker in ['ПРОГНОЗ НАРАБОТКИ', 'Базовая оценка', 'Сдержанный', 'Расчётный', 'Благоприятный']:
    if marker not in ui:
        raise SystemExit('RC748 FAIL: scenario UI marker missing: '+marker)
if 'mrpMetricBackgrounds [4]syscall.Handle' not in ui:
    raise SystemExit('RC748 FAIL: scenario strip is not four cards')
for marker in ['currentMethodsForecastDetailState()', 'без нового мероприятия', 'к базе:', 'уверенность:', 'сценарий появится после выбора или утверждения комплекса мероприятий']:
    if marker not in model:
        raise SystemExit('RC748 FAIL: scenario semantics missing: '+marker)
if 'days > 0 && baseline > 0' not in model:
    raise SystemExit('RC748 FAIL: unsupported method scenario can be rendered as available')
for marker in ['смена ГНО', 'применявшихся методов/комплексов', 'Снимок режима для проверки сопоставимости']:
    if marker not in forecast:
        raise SystemExit('RC748 FAIL: historical basis marker missing: '+marker)
for marker in ['Базовая оценка без нового мероприятия', 'Сдержанный сценарий', 'Расчётный сценарий', 'Благоприятный сценарий', 'уровень уверенности сценарной оценки']:
    if marker not in report:
        raise SystemExit('RC748 FAIL: section-5 export missing: '+marker)
if 'база %.0f · сдерж. %.0f · расч. %.0f · благ. %.0f' not in workspace:
    raise SystemExit('RC748 FAIL: final report card does not reflect four-scenario model')
print('PASS: RC748 runlife four-scenario strip, evidence basis, no invented method forecast, report alignment')
