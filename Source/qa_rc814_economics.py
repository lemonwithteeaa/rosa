from pathlib import Path
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text()
view=(root/'economics_visual_reference.go').read_text()
cards=(root/'economics_reference_812.go').read_text()
helper=(root/'economics_reference_814.go').read_text()
parent=(root/'economics_reference_813.go').read_text()
assert '7.3.11.814-RC' in main
for word in ['econ814RoundedRect', 'econ814Badge', 'economics814HasCurrentResult']:
 assert word in helper
assert 'econ814Badge(di.HDC, badge, card.state)' in cards
assert 'economicsScenarioCalculated(s)' in cards
assert 'state := "Не рассчитано"' in cards
assert 'state = "На пересчёт"' in cards
assert 'Нет актуального расчёта' in cards
assert 'breakGap, breakH, fundGap, fundHeader = 7, 84, 6, 32' in view
assert 'setEconomicsReferenceVisible(econSchemesList, false)' in view
assert 'setPos(econCompareList' in view
assert 'econ814RoundedRect(dc, b,' in parent
assert 'WS_CHILD' not in helper
assert 'newList' not in helper
assert 'econRefMetricSnapshot[index][1] != "—"' in (root/'economics_ui_refinement.go').read_text()
for w,h in [(1920,1080),(1658,925),(1658,864),(1658,805),(1366,768),(1280,768),(1152,864),(1024,768)]:
 cw=max(760,w-270);compact=h<=1020;ultra=h<=805
 cy,ch=(198,60) if ultra else (203,67) if compact else (209,69)
 area=cy+ch+11
 left=max(420,cw*54//100);left=min(left,cw-350)
 right=cw-left-12
 if ultra:rh,hh,ho,bg,pf,bkg,bh,fg,fhdr=(23,21,44,5,43,6,64,6,30)
 elif compact:rh,hh,ho,bg,pf,bkg,bh,fg,fhdr=(27,22,44,5,44,7,84,6,32)
 else:rh,hh,ho,bg,pf,bkg,bh,fg,fhdr=(34,24,43,10,48,8,100,8,34)
 row=area+ho+hh+1;buttons=row+5*rh+bg
 cardstart=area+(44 if ultra else 46 if compact else 53)
 cH=41 if ultra else 48 if compact else 53
 cg=4 if ultra else 6 if compact else 7
 cardend=cardstart+3*cH+2*cg
 assert cardend <= buttons,(w,h,'scenario footer overlap',cardend,buttons)
 assert right>=330,(w,h,'right pane narrow',right)
 inner=left-24; name=inner*42//100;value=max(96,inner*22//100);unit=max(64,inner*13//100)
 status=inner-name-value-unit
 if status<73:name=inner-value-unit-73;status=73
 assert min(name,value,unit,status)>0,(w,h,'input cols')
 fundstart=buttons+pf+bkg+bh+fg+fhdr+88
 avail=h-fundstart-10
 assert avail>=65 if h<=805 else avail>=99, (w,h,'fund rows too short',avail)
 fundH=min(325,max(66,avail))
 if h>=864:assert fundH>=104,(w,h,'four fund rows',fundH)
 assert 242+cw<=w,(w,h,'work area overflow')
 print(f'PASS {w}x{h}: scheme cards non-overlapping; bottom archive {min(fundH, h-fundstart)}px; full inputs')
print('PASS 814 rendering, status semantics and fund geometry')
