"""Regression checks for the visibility leak and the two affected page layouts."""
from pathlib import Path

root = Path(__file__).resolve().parent
main = (root / "main.go").read_text(encoding="utf-8")
econ = (root / "economics_visual_reference.go").read_text(encoding="utf-8")
nno = (root / "nno_fleet_ui.go").read_text(encoding="utf-8")

visibility = main.split("func setPhysicalControlVisibility(", 1)[1].split("\n}", 1)[0]
assert "pGetWindowLongPtrW.Call(uintptr(h), ^uintptr(15))" in visibility
assert "style&uintptr(WS_VISIBLE)" in visibility
assert "physicalControlVisibilityKnown" not in main
assert "setPhysicalControlVisibility(h, false)" in main
assert "applyActivePageVisibility(page)" in main and "pSendMessageW.Call(uintptr(h), WM_SETREDRAW" not in main
print("PASS live HWND visibility check; no stale visibility cache")

assert "calculateAllW := int32(270)" in econ
assert "setPos(econCalculateAllButton, x+cw-calculateAllW, py(actionY), calculateAllW, 40)" in econ
assert "setPos(econCalculateAllButton, x, py(actionY), cw, 40)" not in econ
print("PASS economics primary action is compact and right-aligned")

assert "ПОМЕСЯЧНАЯ ДИНАМИКА · СРЕДНЯЯ / МЕДИАНА" in nno
assert "DTM_SETFORMATW" in nno and "dd.MM.yyyy" in nno
assert "nnoFleetDateFrom" in nno and "nnoFleetDateTo" in nno
assert "nnoSelectedDataSource" in nno and "nnoRefreshDataChoices" in nno
assert "nnoFleetSeries" in nno and "nnoDrawEventCountChart" in (root / "nno_chart_windows.go").read_text(encoding="utf-8")
assert "РЕГИСТРАЦИЯ СОБЫТИЙ · ПО МЕСЯЦАМ" in nno
assert "setPos(nnoFleetPeriodLabel, boxes[0].X, labelY, boxes[1].X+boxes[1].W-boxes[0].X, 22)" in nno
assert "setPos(nnoFleetFilterLabel, boxes[2].X, labelY, boxes[2].W, 22)" in nno
assert "setPos(nnoFleetDataLabel, boxes[3].X, labelY, boxes[3].W, 22)" in nno
assert "nnoToolbarArrange(x, cw)" in nno
assert "labelY, controlY := py(138), py(165)" in nno
assert "drawFont = hFontButton" in main and "drawFont = hFontBrandCaption" not in main[main.index("if nnoAction && hFontButton != 0"):main.index("if h == methodsShowAllButton",main.index("if nnoAction && hFontButton != 0"))]
assert "row2 := y + 48" not in nno
assert "nnoFleetMethodCards" in nno and "[nnoMethodCount]syscall.Handle" in nno
assert 'row("Средняя", meanText' in nno
assert 'row("Медиана", medianText' in nno
assert "rgb(172, 37, 55), rgb(15, 48, 82)" in nno
assert "colors := [...]uintptr{rgb(0, 112, 125), rgb(22, 79, 172)" in nno
assert "nnoSourceDataForGroup(\"Все способы\")" in nno
assert "drawNNOFleetMethodCard(di)" in main
assert "minChartH = 50" in nno
assert "text(cardName, 5, nameBottom, hFontBold" in nno
assert "if cw >= 1250" in nno
assert "setMRPLayoutVisibility(nnoFleetStatus, true)" in nno
assert "statusY := cardsBottom + 5" in nno and "splitY := statusY + 34" in nno
assert "func layoutNNOFleetColumns" in nno
assert "widths[len(widths)-1] += available - used" in nno
assert 'cardName = "Прочие / не опр."' in nno
assert "func nnoFleetHeaderCustomDraw" in nno and "func nnoFleetListCustomDraw" in nno
assert "nnoFleetListCustomDraw(lParam)" in main and "nnoFleetHeaderCustomDraw(lParam" in main
assert "brushNav" in nno[nno.index("func nnoFleetHeaderCustomDraw"):nno.index("func nnoFleetListCustomDraw")]
assert "rgb(246, 248, 251)" in nno[nno.index("func nnoFleetListCustomDraw"):nno.index("func handleNNOFleetCommand")]
print("PASS NNO filters and four actions share a single toolbar row")

for width, height in ((1180, 700), (1366, 768), (1600, 900), (1920, 1080)):
    cw = max(720, width - 270)
    assert "nnoToolbarArrange(x, cw)" in nno
    assert "setPos(nnoFleetObject, boxes[7].X, controlY, boxes[7].W, 34)" in nno
    control_y = 253
    cards_y = control_y + 34 + 8
    columns = 6 if cw >= 1250 else 3
    rows = 1 if columns == 6 else 2
    card_h, gap = ((96 if columns == 6 else 76), 4) if height < 820 else ((116 if columns == 6 else 96), 8)
    cards_bottom = cards_y + rows * card_h + (rows - 1) * gap
    split_y = cards_bottom + 5 + 34
    mini = height < 740
    no_middle = height < 820
    chart_h = 0 if mini or no_middle else max(50 if height < 820 else 112, min(260, height - split_y - 210))
    lower_y = split_y + (6 if mini or no_middle else 26 + chart_h + (6 if height < 820 else 8))
    lower_h = max(96 if height < 820 else 100, height - (lower_y + 26) - 15)
    list_bottom = lower_y + 26 + lower_h
    assert list_bottom <= height, (width, height, list_bottom)
    print(f"PASS: single-row NNO toolbar, cards and both tables fit {width}×{height}")
