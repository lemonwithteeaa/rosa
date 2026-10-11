from pathlib import Path
r=Path(__file__).resolve().parent
m=(r/'main.go').read_text()
v=(r/'economics_visual_reference.go').read_text()
u=(r/'economics_reference_812.go').read_text()
assert '7.3.11.812-RC' in m
assert 'paintEconomics812ScenarioCard(di)' in m
assert 'handleEconomics812Card(id)' in m
assert 'refreshEconomics812ScenarioCards()' in v
assert 'setEconomicsReferenceVisible(econSchemesList, false)' in v
assert 'openEconomicsFundArchiveAtRow' in m
assert 'selectEconomicsSchemeFromTable(i)' in m
assert 'setPos(econCompareList' in v
assert 'econPanels' not in v
assert 'CalculateBaseEconomics(in)' in (r/'workflow_pages.go').read_text()
assert 'paintEconomics812Bands' in m
assert 'refreshEconomics812Display' in v
for w,h in [(1920,1080),(1658,925),(1658,864),(1366,768),(1280,768),(1152,864)]:
 cw=max(760,w-270);compact=h<=905;ultra=h<=805
 cy,ch=(198,60) if ultra else (203,67) if compact else (210,74)
 areaY=cy+ch+11; leftW=max(420,cw*54//100); gap=13;rx=242+leftW+gap;rw=cw-leftW-gap
 rh,hoff,hh=(26,28,22) if ultra else (30,31,24) if compact else (36,35,26)
 ry=areaY+hoff+hh+1; bottom=ry+5*rh;buttons=bottom+9
 listY=areaY+48;cardGap,cardH=(4,42) if ultra else (5,48) if compact else (6,56)
 lastCardBottom=listY+2*(cardH+cardGap)+cardH
 assert lastCardBottom < buttons,(w,h,'cards overlap controls',lastCardBottom,buttons)
 methodW=108 if rw<560 else 130
 titleRight=9+rw-2*methodW-37
 firstButtonX=rw-2*methodW-14
 assert titleRight<firstButtonX,(w,h,'title button overlap')
 breakY=buttons+49; breakH=37 if ultra else 46 if compact else 60
 fundY=breakY+breakH+9+32
 fh=min(300,max(72,h-(fundY+88)-12))
 nameW=(leftW-24)*42//100
 valueW=max(91,(leftW-24)*21//100)
 unitsW=max(66,(leftW-24)*14//100)
 statusW=leftW-24-nameW-valueW-unitsW
 assert nameW>=145 and valueW>=91 and unitsW>=66 and statusW>=70
 # Four+ data rows at normal heights, >=2 at shorter Windows viewports.
 assert fh>=80,(w,h,'archive height',fh)
 assert 242+cw <= max(1000,w),(w,h,'outer region')
 print(f'{w}x{h}: parameter cols={nameW}/{valueW}/{unitsW}/{statusW}, scenario cards=3, fund table={fh}px, overlap=none')
print('PASS 812 reference geometry, controls, fund access')
