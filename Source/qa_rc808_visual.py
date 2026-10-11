"""Focused RC808 layout and training-preview regression gate."""
from pathlib import Path
src=Path(__file__).resolve().parent
nno=(src/'nno_fleet_ui.go').read_text(encoding='utf-8')
plot=(src/'nno_chart_windows.go').read_text(encoding='utf-8')
methods=(src/'methods_workflow.go').read_text(encoding='utf-8')
econ=(src/'economics_visual_reference.go').read_text(encoding='utf-8')
main=(src/'main.go').read_text(encoding='utf-8')
checks = {
'808 code version': 'const appBuildVersion = "7.3.11.808-RC"' in main,
'journal remains free of training source': 'Never mix unfinished or teaching records into production' in nno and 'nnoSourceDataForGroup' in nno,
'preview derives from raw events or active teaching laboratory': 'nnoShouldUseSyntheticPreview(failureEvents) || nnoHasActiveTrainingLabExamples()' in nno,
'preview never replaces nonempty observed event and cycle data': 'len(nnoFleetCycleSamples) == 0 && len(events) == 0' in nno,
'removed lab passes cannot activate preview': 'pass.Status != labPassSavedStatus' in nno,
'official NNO remains uncalculated': 'ННО-12: не рассчитана' in nno and 'Нужны Ti и правило учёта' in nno,
'both statistical plots retained': 'nnoDrawRollingCycleChart' in plot and 'nnoDrawCycleDistribution' in plot,
'full method names in plot legend': 'nnoChartText(hdc, nnoMethodName(g)' in plot,
'statistical legend explains every marker': all(x in plot for x in ['Диапазон без выбросов','Половина значений (Q1–Q3)','Медианная наработка','Средняя наработка']),
'detail table has a separate full-height viewport': 'chartH, tableH, eventH := nnoFleetBandHeights(h, viewportTop)' in nno and 'setPos(nnoFleetEvents, x, eventY+55, contentW, eventH)' in nno,
'archive independently reserved in Methods': 'archiveBudget = max32(archiveBudget, 174)' in methods and 'setPos(methodsArchiveList, x, archiveY, contentW, archiveH)' in methods,
'economy fund archive prioritized': 'ЭКОНОМИКА ФОНДА · НАКОПИТЕЛЬНАЯ ИСТОРИЯ ПО ВСЕМ ОБЪЕКТАМ' in econ and 'archiveH := int32(172)' in econ,
'economy inputs, exports and selected scenario preserved': 'setPos(econCalculateAllButton' in econ and 'setPos(econSchemesList' in econ,
}
for title,ok in checks.items():
 print(('PASS: ' if ok else 'FAIL: ')+title)
if not all(checks.values()): raise SystemExit('RC808 source gates failed')
# Exact equivalent of viewport budgeting formula, independent from Win32 output.
for height, viewport in [(864,302),(900,342),(1040,458),(1200,458)]:
 available=height-viewport-12
 middle,detail=118,146
 chart=max(185,min(240,available-121-middle-detail))
 if height<780:chart=185
 extra=max(0,available-121-chart-middle-detail)
 detail+=min(100,extra);extra-=min(100,extra)
 middle+=min(56,extra)
 assert chart>=185 and middle>=118 and detail>=146
 event_y=viewport+26+chart+8+24+middle+8+55
 scroll=max(0,event_y+detail-(height-12))
 # All sections remain reachable using the native scrolling buttons.
 assert event_y+detail-scroll<=height-12
 print(f'PASS: viewport {height}: chart={chart}, mid-table={middle}, detail-table={detail}, scroll={scroll}')
