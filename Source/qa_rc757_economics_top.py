"""Verifies the 757 section-six header: controls, safe rows, scrollable narrow mode."""
from pathlib import Path
import re
import subprocess
r = Path(__file__).resolve().parent
s = (r/'economics_visual_reference.go').read_text(encoding='utf-8')
w = (r/'workflow_pages.go').read_text(encoding='utf-8')
m = (r/'main.go').read_text(encoding='utf-8')
version = re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', m, re.M)
assert version and int(version.group(1)) >= 757
assert 'econHeaderDivider' in s and 'SS_ETCHEDHORZ' in s
assert 'econMethodNextButton' in s and 'econMethodBaseButton' in s
assert 'setPos(econMethodNextButton, x+cw-buttonW, py(top.schemeButtonY), buttonW, 44)' in s
assert 'setPos(econMethodBaseButton, x+cw-buttonW*2-buttonGap, py(top.schemeButtonY), buttonW, 44)' in s
assert 'buttonW := int32(194)' in s
assert 'setPos(econUseObjectButton, x+cw-objectButtonW, py(top.objectButtonY), objectButtonW, 44)' in s
assert 'SS_RIGHT|SS_CENTERIMAGE|SS_NOPREFIX' in w
assert 'SS_LEFT|SS_CENTERIMAGE|SS_ENDELLIPSIS|SS_NOPREFIX' in w
assert 'econAtomicLayoutDepth > 0 && pageControlOwner[h] == ID_ECON' in m
assert 'refreshEconomicsVisualReference()' in w
assert 'alternativeScenariosForObject(selectedObjectKey)' in s
assert 'case ID_ECON_METHOD_NEXT:' in w and 'case ID_ECON_METHOD_BASE:' in w
assert 'case ID_ECON_CALCULATE_ALL:' in w

chunk = s.split('func economicsHeaderGeometry()',1)[1].split('\n}',1)[0]
g = {k:int(v) for k,v in re.findall(r'\b(titleY|subtitleY|statusY|objectY|dividerY|sectionY|listY|objectButtonY|schemeButtonY):\s*(\d+)',chunk)}
assert len(g)==9, g

# Model exact coordinate constants in layoutEconomicsVisualReference (py = y+88).
def box(x,y,width,height): return x,y,x+width,y+height
def collides(a,b): return min(a[2],b[2])>max(a[0],b[0]) and min(a[3],b[3])>max(a[1],b[1])
for size in [(1180,700),(1280,720),(1366,768),(1600,850),(1600,900),(1600,930),(1920,1080),(1920,1200)]:
    win_h,win_v=size; left=242; cw=max(720,win_h-270); right=left+cw
    deferral=max(240,cw*27//100); status_w=cw-deferral-18
    title=box(left,g['titleY'],cw,40)
    subtitle=box(left,g['subtitleY'],cw,24)
    status=box(left,g['statusY'],status_w,26)
    defer=box(left+status_w+18,g['statusY'],deferral,26)
    obj=box(left,g['objectY'],cw-294-16,38)
    put=box(right-294,g['objectButtonY'],294,44)
    divider=box(left,g['dividerY'],cw,2)
    sections=box(left,g['sectionY'],cw-194*2-12-20,30)
    prev=box(right-400,g['schemeButtonY'],194,44)
    next=box(right-194,g['schemeButtonY'],194,44)
    table=box(left,g['listY'],cw,57 if win_v<740 else 88 if win_v<810 else 115 if win_v<930 else 152 if win_v>=1050 else 122)
    for a,b in [(title,subtitle),(subtitle,status),(status,obj),(defer,put),(status,defer),(obj,put),(put,divider),(divider,prev),(divider,next),(sections,prev),(sections,next),(prev,next),(prev,table),(next,table),(sections,table)]:
        assert not collides(a,b), ('overlap',size,a,b)
    assert all(q[0]>=left and q[2]<=right for q in (title,subtitle,status,defer,obj,put,divider,sections,prev,next,table))
    assert (prev[3]-prev[1])>=44 and (next[2]-next[0])>=194
    methods=table[3]-table[1]
    compact=win_v<930
    card_h=31 if compact else 46
    scenario_y=g['listY']+methods+5
    params_y=scenario_y+card_h+1
    action_y=params_y+94
    if compact:
        result_top=action_y+46+23+88
        rh=max(20,win_v-result_top-16)
    else:
        cmp_h=124 if win_v>=1050 else 80
        cmp_y=action_y+47+23
        result_top=cmp_y+cmp_h+7+23+88
        rh=max(24,win_v-result_top-16)
    assert result_top+rh<=win_v-12, ('result clipping',size,result_top,rh)
    print('PASS GEOMETRY',f'{win_h}x{win_v}',f'action=194x44',f'result={rh}')

# This pass must be visual-only: no calculation, persistence, database or report files.
changed=subprocess.check_output(['git','diff','--name-only'],cwd=r,text=True).splitlines()
assert set(changed)<={'main.go','workflow_pages.go','economics_visual_reference.go','build_windows_release.sh','economics_fund_archive.go','methods_plan_archive.go','methods_reference_dialog.go','build_release.py','qa_rc658_static.py','qa_rc677_static.py','qa_rc757_economics_top.py','qa_rc759_nno_history.py','qa_rc760_stability_economics.py'},changed
print('PASS PRE-RELEASE SCOPE + ACTION HOOKS')
