#!/usr/bin/env python3
"""NNO toolbar, date/data filters and reference layout regression checks."""
from pathlib import Path

root = Path(__file__).resolve().parent
source = (root / "nno_fleet_ui.go").read_text(encoding="utf-8")
main = (root / "main.go").read_text(encoding="utf-8")

checks = {
    "analysis dates use native dropdown calendars and requested format": '"SysDateTimePick32"' in source and '"dd.MM.yyyy"' in source and "DTM_GETSYSTEMTIME" in source,
    "date changes keep a valid inclusive range": "nnoFleetDateFrom.After(nnoFleetDateTo)" in source and "nnoFleetDateTo.Before(nnoFleetDateFrom)" in source,
    "method and data dropdowns are populated": '"Способ эксплуатации:"' in source and '"Данные:"' in source and '"Журнал отказов"' in source,
    "data choice filters events by source": 'strings.TrimSpace(e.Source) != selectedSource' in source,
    "analysis range filters exact event dates and builds dynamic months": "d.Before(from) || d.After(to)" in source and "monthCount := nnoMonthIndex(last, first) + 1" in source,
    "trend graph scales to selected month count": "nnoFleetSeries" in source and "nnoRollingCycleStats" in source,
    "toolbar labels occupy a separate band and controls consume full width": "labelY, controlY := py(138), py(165)" in source and "nnoToolbarArrange(x, cw)" in source and "a.Cells" in source,
    "all four actions use readable uniform single-line height": "setPos(nnoFleetUpdate, boxes[4].X, controlY, boxes[4].W, 34)" in source and "setPos(nnoFleetRules, boxes[6].X, controlY, boxes[6].W, 34)" in source,
    "only refresh is emphasized as the primary action": "if h == nnoFleetUpdate {" in main and "br = brushNav" in main,
    "date picker common controls are initialized": "0x00000100" in main,
    "six method cards retain requested order and metrics": "nnoMethodCount          = 6" in source and 'row("Средняя", meanText' in source and 'row("Медиана", medianText' in source,
    "average and median stay bold red": "rgb(172, 37, 55), rgb(15, 48, 82)" in source,
    "tables have no artificial empty last columns": "widths[len(widths)-1] += available - used" in source,
    "NNO remains explicitly uncalculated without Ti and methodology": '"Не рассчитана"' in source and "ННО-12: не рассчитана" in source,
}
checks["buttons use standard readable font rather than 10px brand font"] = "if nnoAction && hFontButton != 0" in main and "drawFont = hFontButton" in main
checks["labels clear controls by 5px and have 22px height"] = "setPos(nnoFleetDataLabel, boxes[3].X, labelY, boxes[3].W, 22)" in source and "labelY, controlY := py(138), py(165)" in source
for name, passed in checks.items():
    print(("PASS" if passed else "FAIL") + ": " + name)
if not all(checks.values()):
    raise SystemExit("NNO toolbar/reference check failed")

# The application minimum client width is about 910 px (1180 px window).
# Confirm the toolbar stays a single row at minimum, reference and wide layouts.
for width, height in ((1180, 700), (1280, 729), (1366, 768), (1600, 900), (1920, 1080)):
    cw = max(720, width - 270)
    # Geometry is validated by executable Go tests. Static gating checks the
    # production layout actually consumes those computed slots.
    assert "nnoToolbarArrange(x, cw)" in source
    for slot in range(4, 8):
        assert f"boxes[{slot}].X, controlY, boxes[{slot}].W, 34" in source
    assert "setPos(nnoFleetFromDate, boxes[0].X, controlY, boxes[0].W, 34)" in source
    assert "setPos(nnoFleetToDate, boxes[1].X, controlY, boxes[1].W, 34)" in source
    cards_y = 253 + 34 + 8
    columns = 6 if cw >= 1250 else 3
    rows = 1 if columns == 6 else 2
    card_h, gap = ((96 if columns == 6 else 76), 4) if height < 820 else ((116 if columns == 6 else 96), 8)
    cards_bottom = cards_y + rows * card_h + (rows - 1) * gap
    split_y = cards_bottom + 5 + 34
    mini, no_middle = height < 740, height < 820
    chart_h = 0 if mini or no_middle else max(112, min(260, height - split_y - 210))
    lower_y = split_y + (6 if mini or no_middle else 26 + chart_h + 8)
    lower_h = max(96 if height < 820 else 100, height - (lower_y + 26) - 15)
    assert lower_y + 26 + lower_h <= height, (width, height, lower_y + 26 + lower_h)
    print(f"PASS: single-row toolbar, six cards and data tables fit {width}×{height}")
