from pathlib import Path
p=Path(__file__).parent
m=(p/'main.go').read_text(encoding='utf8')
ui=(p/'economics_visual_reference.go').read_text(encoding='utf8')
metrics=(p/'economics_ui_refinement.go').read_text(encoding='utf8')
assert '7.3.11.811-RC' in m
assert 'econPanels' not in ui
assert 'paintEconomicsPanel' not in m+metrics
assert '15010+i' not in ui
assert 'layoutEconomicsMetricCards(x, metricY, cw, metricH)' in ui
assert 'setPos(econCompareList,' in ui and 'setPos(econSchemesList,' in ui
assert 'setPos(econFieldLabels[index],' in ui and 'setPos(inputs[index],' in ui
assert 'refreshEconomicsFundArchive()' in ui
assert 'setPos(econChartButton,' in ui and 'setPos(econFullResultButton,' in ui
assert 'beginEconomicsAtomicLayout()' in ui
assert 'setEconomicsReferenceVisible(econConclusionTitle, false)' in ui
assert 'econFundFilterCombo' in ui
assert 'openEconomicsFundArchiveAtRow' in m
assert 'if economicsScenarioCalculated(scenario)' in metrics
for w,h in [(1920,1080),(1658,864),(1366,768),(1280,768),(1152,864)]:
 cw=max(720,w-270);wide=cw>=950;short=h<825
 my,mh=(215,68) if short else (220,76)
 work=my+mh+12
 if wide:
  left=int(cw*.53);action=work+(151 if short else 178);bottom=action+48
  fieldW=(left-28-20)//3
  chart=min(176,max(126, (left-28-192-16)//2))
  full=left-28-192-chart-16
  assert fieldW>=150, (w,h,fieldW)
  assert full>=125,(w,h,full)
  assert work+(35+54+27+28 if short else 40+64+30+30)<=action
 else:
  action=work+112;bottom=action+51+116
 fy=bottom+14+42
 fh=min(170 if short else 210,max(45,h-(fy+88)-15))
 assert fh>=75,(w,h,fh)
 print(f'{w}x{h}: cards=4, fund-table={fh}px, wide={wide}')
print('PASS 811 layout and control visibility checks')
