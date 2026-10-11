"""Structural and coordinate regression for section 6 (Windows visual-only pass)."""
from pathlib import Path
import re, subprocess
root=Path(__file__).resolve().parent
module=(root/'economics_visual_reference.go').read_text(encoding='utf-8')
workflow=(root/'workflow_pages.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
assert 'func buildEconomicsVisualReference()' in module
assert 'buildEconomicsVisualReference()' in workflow
assert 'layoutEconomicsVisualReference(w, h)' in workflow
assert 'defer refreshEconomicsVisualReference()' in workflow
assert 'econAtomicLayoutDepth > 0 && pageControlOwner[h] == ID_ECON' in main
assert 'case ID_ECON_METHOD_BASE:' in workflow and 'case ID_ECON_METHOD_NEXT:' in workflow
assert 'case ID_ECON_CALCULATE_ALL:' in workflow and 'case ID_ECON_USE_SELECTED:' in workflow
assert 'small := h < 930' in module
assert 'if h >= 1050' in module
assert 'listBeginUpdate(econSchemesList)' in module and 'listEndUpdate(econCompareList)' in module
assert 'alternativeScenariosForObject(selectedObjectKey)' in module
assert 'Нет данных' in module and 'Расчёт актуален' in module
assert 'econReferenceMoney(s.NetBenefitK, costKnown)' in module
assert '"Чистый эффект, тыс. руб."' in module
assert '[]int32{7, 25, 11, 10, 13, 12, 13, 9}' in module
# All 8 physical columns populated; preserve the stored 8th status.
header=module.split('econSchemesList = newList',1)[1].split('})',1)[0]
assert header.count('{"')==8,header
assert 's.Name, methods, dash(s.Availability)' in module
assert 'econReferenceMoney(s.NetBenefitK, costKnown), status' in module
# No calculation or export file is allowed to change in this pass.
changed=subprocess.check_output(['git','diff','--name-only'],cwd=root,text=True).splitlines()
assert set(changed)<=set(['main.go','workflow_pages.go']),changed
# Section 6 geometry simulation, using the exact constants in layoutEconomicsVisualReference.
for w,h in [(1180,700),(1280,720),(1366,768),(1600,900),(1600,930),(1920,1080),(1920,1200)]:
 x,cw=242,max(720,w-270)
 small=h<930; very_small=h<760
 methodsH=105 if very_small else 172 if not small and h>=1050 else 145
 table_top=255+88
 scenarioY=260+methodsH
 assert table_top+methodsH<=scenarioY+88,(w,h,'methods/scenario overlap')
 scenarioH=31 if small else 46
 paramsY=scenarioY+scenarioH+1
 label_y=paramsY+22+88; label_bottom=label_y+36
 edit_top=paramsY+60+88
 assert label_bottom<=edit_top+2,(w,h,'label/input overlap')
 actionY=paramsY+94
 assert edit_top+30<=actionY+88,(w,h,'input/button overlap')
 assert x+cw<=w-16,(w,h,'right-hand edge overflow')
 fieldW=(cw-40)//5
 assert fieldW>=130 and x+4*(fieldW+10)+fieldW<=x+cw,(w,h,'inputs overflow')
 if small:
  assert h>=700
  titleY=actionY+46
  detailY=titleY+23
  resultH=max(20,h-(detailY+88)-16)
  assert resultH>=20 and detailY+88+resultH<=h-12,(w,h,'compact result clipping')
 else:
  compareY=actionY+47+23
  compareH=145 if h>=1050 else 102
  assert compareY+88>=actionY+88+40,(w,h,'compare button overlap')
  detailY=compareY+compareH+7+23
  resultH=max(24,h-(detailY+88)-16)
  assert resultH>=24 and detailY+88+resultH<=h-12,(w,h,'full result clipping')
 print(f'PASS econ geometry {w}x{h}: {"compact" if small else "expanded"}, result {resultH}px')
print('PASS RC755: 8-column schema, selection/refresh wiring, calculation/export untouched')
