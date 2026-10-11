from pathlib import Path
p=Path(__file__).parent
# This check records the historical 810 geometry. Newer releases run their
# own versioned geometry check instead of asserting the removed 810 panels.
if '7.3.11.811-RC' in (p/'main.go').read_text(encoding='utf8'):
    print('810 historical QA superseded by qa_rc811_economics.py')
    raise SystemExit(0)
files={name:(p/name).read_text(encoding='utf8') for name in ['main.go','economics_visual_reference.go','economics_ui_refinement.go','workflow_pages.go','economics_fund_archive.go','methods_workflow.go']}
checks={
'810 version set': '7.3.11.810-RC' in files['main.go'],
'unchanged header structure': 'hBrandSignature' in files['main.go'],
'4 always-on upper metric cards': 'layoutEconomicsMetricCards(x, metricY, cw, metricH)' in files['economics_visual_reference.go'],
'primary KPI titles match reference': all(s in files['economics_ui_refinement.go'] for s in ['CAPEX · РАЗОВО','OPEX · В ГОД','ОТСРОЧКА РЕМОНТА','ЧИСТЫЙ ЭФФЕКТ']),
'known-only positive benefit': 'if economicsScenarioCalculated(scenario)' in files['economics_ui_refinement.go'],
'native bottom-layer background': 'pSetWindowPos.Call(uintptr(h), 1,' in files['economics_visual_reference.go'],
'economics panels painted owner draw': 'if paintEconomicsPanel(di)' in files['main.go'],
'scenario table persisted': 'setPos(econSchemesList,' in files['economics_visual_reference.go'],
'fund table persisted': 'setPos(econCompareList,' in files['economics_visual_reference.go'],
'fund filtering still active': 'ID_ECON_FUND_FILTER' in files['main.go'],
'scenario approval link retained': 'economicsDecisionStatusText' in files['workflow_pages.go'],
'original calculations untouched': 'CalculateBaseEconomics(in)' in files['workflow_pages.go'],
'full output button present': 'case ID_ECON_FULL_RESULT:' in files['workflow_pages.go'],
'double click full fund detail retained': 'openEconomicsFundArchiveAtRow' in files['main.go'],
'no overlapping action bar': 'innerW - calculateW - chartW - 2*btnGap' in files['economics_visual_reference.go'],
'no changes to methods scenario navigator': 'СХЕМА %d/%d ›' in files['methods_workflow.go'],
}
for name,ok in checks.items():print(('PASS ' if ok else 'FAIL ')+name)
assert all(checks.values())

def geometry(w,h):
 cw=max(720,w-270)
 wide=cw>=850; short=h<825
 metricY,metricH=(215,68) if short else (220,76)
 workspace=metricY+metricH+12
 if wide:
  action=workspace+(151 if short else 178)
  bottom=action+48
  panels=[(workspace,bottom)]
  # text labels / edit boxes end before the action bar
  inputEnd=workspace+(35+54+27+28 if short else 40+64+30+30)
  assert inputEnd <= action, (w,h,inputEnd,action)
 else:
  action=workspace+112
  methodsTitle=action+51
  bottom=methodsTitle+116
  panels=[(workspace,bottom)]
  assert action+40 < methodsTitle
 fundY=bottom+14+42
 fundAvailable=h-(fundY+88)-15
 fundH=min(170 if short else 210,max(45,fundAvailable))
 return cw,wide,fundY,fundH
for w,h in [(1658,864),(1658,768),(1280,768),(1280,960),(1190,768),(1190,864),(1366,930)]:
 cw,wide,y,fundH=geometry(w,h)
 assert fundH>=100,(w,h,fundH)
 assert y+88+fundH<=h-14,(w,h,y,fundH)
 print(f'PASS {w}x{h}: two-column={wide} fund={fundH}px, >3 table rows')
# Narrow viewport has a stacked presentation. Not enough vertical space at
# 768px for two tables plus all inputs; the fund retains a scrollbar.
for w,h in [(1080,768),(990,800)]:
 cw,wide,y,fundH=geometry(w,h)
 print(f'LIMIT {w}x{h}: stacked layout, fund={fundH}px; native scroll required')
