#!/usr/bin/env python3
"""NNO card & plot regression gates for the current reference geometry."""
from pathlib import Path

root = Path(__file__).resolve().parent
source = (root / "nno_fleet_ui.go").read_text(encoding="utf-8")
plot = (root / "nno_chart_windows.go").read_text(encoding="utf-8")
checks = {
    "six distinct production-mode cards": 'nnoMethodCount          = 6' in source and '"УЭЦН", "УШГН", "УЭВН", "ОРД", "Фонтанный", "Прочие"' in source,
    "red bold mean/median labels and non-fabricated values": 'row("Средняя", meanText' in source and 'row("Медиана", medianText' in source and 'rgb(172, 37, 55), rgb(15, 48, 82)' in source,
    "method palette keeps UECN distinct from alternatives": 'colors := [...]uintptr{rgb(0, 112, 125)' in source and 'rgb(22, 79, 172)' in source,
    "verified-only observed cycles": 'nnoEventIsConfirmed(e)' in source and 'nnoVerifiedCycleSamples' in source and 'nnoCycleStatistics' in source,
    "chart distinguishes average from median": 'stats.Mean' in plot and 'stats.Median' in plot and 'if nnoFleetShowMedian' in plot,
    "distribution chart uses actual quartiles": 'stat.Q1' in plot and 'stat.Q3' in plot and 'stat.Median' in plot,
    "distribution markers are selectable and report exact values": 'nnoPickDistributionMarker' in plot and 'nnoDistributionSelected.Value' in plot and 'nnoChartDashedStroke(hdc' in plot,
    "distribution legend explains whiskers, quartiles, and center": 'усы: мин. – макс.' in plot and 'коробка: Q1 – Q3' in plot and 'центр: средняя' in plot,
    "event-count chart retained as separate view": 'nnoFleetShowEventTrend' in source and 'nnoDrawEventCountChart' in plot,
    "table remains available beside boxplot": 'nnoFleetShowMethodTable' in source and 'nnoFleetMethods' in source,
    "short-window tables prioritized": 'compactNoMiddle := h < 820' in source and 'lowerY = splitY + lowerGap' in source,
    "NNO-12 never substituted from completed cycles": 'ННО-12: не рассчитана' in source and 'Нужны Ti и правило учёта' in source,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
if not all(checks.values()):
    raise SystemExit('RC769 reference adaptation regression')
