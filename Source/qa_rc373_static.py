from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
stat=(root/'outcome_statistics.go').read_text(encoding='utf-8')
mrp=(root/'mrp_forecast_ui.go').read_text(encoding='utf-8')
workflow=(root/'workflow_pages.go').read_text(encoding='utf-8')
checks={
'version RC373':'const appBuildVersion = "7.3.11.373-RC"' in main,
'separate confirmed event types':'stats.Failures++' in stat and 'stats.Repairs++' in stat and 'stats.PlannedInterventions++' in stat,
'unconfirmed separated':'stats.Unconfirmed++' in stat,
'cause exact grouping': 'no semantic synonym merging' in stat,
'MRP compact stats visible':'mrpCauseSummaryLabel' in mrp and 'outcomeCycleStatisticsCompact' in mrp,
'MRP detail stats':'ФАКТИЧЕСКАЯ СТРУКТУРА ЗАВЕРШЕНИЙ ЦИКЛОВ' in mrp,
'forecast fact comparison':'outcomeForecastFactComparison' in stat and 'СОПОСТАВЛЕНИЕ ПРОГНОЗ → ФАКТ' in main,
'result page includes stats':'НАКОПИТЕЛЬНАЯ СТАТИСТИКА ЗАВЕРШЕНИЙ ЦИКЛОВ' in workflow,
'no automatic ranking':'не используется для автоматического рейтинга методов' in stat,
'no automatic MRP weighting':'не получают вес в прогнозе МРП' in stat,
}
for name,ok in checks.items():
    print(('PASS' if ok else 'FAIL')+' | '+name)
if not all(checks.values()):
    raise SystemExit(1)
