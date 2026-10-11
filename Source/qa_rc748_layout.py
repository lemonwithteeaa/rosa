from pathlib import Path
root=Path(__file__).resolve().parent
ui=(root/'mrp_forecast_ui.go').read_text(encoding='utf-8')
for marker in [
    'mrpMetricBackgrounds [4]syscall.Handle',
    'cardW := (contentW - 3*cardGap) / 4',
    'setPos(mrpMetricBackgrounds[i]',
    'setPos(mrpEvidenceLabel',
]:
    if marker not in ui:
        raise SystemExit('RC748 LAYOUT FAIL: '+marker)
for width in [1180,1280,1366,1600,1920]:
    content=max(720,width-270)
    for gap in [8,10]:
        card=(content-3*gap)//4
        body=card-16
        if card < 215 or body < 195:
            raise SystemExit(f'RC748 LAYOUT FAIL {width}: card={card} body={body}')
    print(f'PASS {width}: four scenario cards remain readable')
print('RC748 runlife scenario layout PASS')
