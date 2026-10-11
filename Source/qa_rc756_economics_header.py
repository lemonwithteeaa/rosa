"""Checks the visible section-6 header grid and all preserved data links."""
from pathlib import Path
import re
root = Path(__file__).resolve().parent
text = (root/'economics_visual_reference.go').read_text(encoding='utf-8')
workflow = (root/'workflow_pages.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
assert '7.3.11.756-RC' in main
assert 'econPageTitle = pageTitle(ID_ECON' in workflow
assert 'econObjectLabel = labelEllipsis' in workflow
assert 'layoutEconomicsVisualReference(w, h)' in workflow
assert 'defer refreshEconomicsVisualReference()' in workflow
assert 'alternativeScenariosForObject(selectedObjectKey)' in text
assert 'listBeginUpdate(econSchemesList)' in text
assert 'econAtomicLayoutDepth > 0 && pageControlOwner[h] == ID_ECON' in main
assert 'case ID_ECON_USE_SELECTED:' in workflow
assert 'case ID_ECON_METHOD_BASE:' in workflow
assert 'case ID_ECON_METHOD_NEXT:' in workflow
assert 'case ID_ECON_CALCULATE_ALL:' in workflow
assert 'setEconomicsReferenceVisible(econCompareList, false)' in text
# Title/sidebar geometry and action rectangle positions are taken from source values.
chunk = text.split('func economicsHeaderGeometry()',1)[1].split('}',1)[0]
g = {key:int(value) for key,value in re.findall(r'(titleY|subtitleY|statusY|objectY|sectionY|listY|objectButtonY|schemeButtonY):\s*(\d+)', chunk)}
assert len(g)==8,g
rect = lambda x,y,w,h: (x,y,x+w,y+h)
def overlaps(a,b):
    return min(a[2],b[2])>max(a[0],b[0]) and min(a[3],b[3])>max(a[1],b[1])
for w,h in [(1180,700),(1280,720),(1366,768),(1600,900),(1600,930),(1920,1080),(1920,1200)]:
    x,cw = 242,max(720,w-270)
    right=x+cw
    deferral=max(234,cw*27//100);status=cw-deferral-16
    title=rect(x,g['titleY'],cw,42)
    subtitle=rect(x,g['subtitleY'],cw,21)
    decision=rect(x,g['statusY'],status,21)
    repair=rect(x+status+16,g['statusY'],deferral,21)
    obj=rect(x,g['objectY'],cw-266-12,32)
    insert=rect(right-266,g['objectButtonY'],266,36)
    table_title=rect(x,g['sectionY'],cw-153*2-8-14,25)
    base=rect(right-314,g['schemeButtonY'],153,35)
    next_=rect(right-153,g['schemeButtonY'],153,35)
    if overlaps(base,next_) or overlaps(table_title,base) or overlaps(table_title,next_): raise AssertionError((w,h,'scheme row'))
    for upper,lower in [(title,subtitle),(subtitle,decision),(subtitle,repair),(decision,obj),(decision,insert),(repair,insert),(obj,base),(insert,base),(insert,next_)]:
        assert not overlaps(upper,lower),(w,h,upper,lower)
    assert not overlaps(obj,insert)
    assert not overlaps(decision,repair)
    assert all(r[0]>=x and r[2]<=right for r in (title,subtitle,decision,repair,obj,insert,table_title,base,next_))
    assert g['listY']>=max(table_title[3],base[3],next_[3])+4
    small=h<930; tiny=h<760
    methods=105 if tiny else 115 if small else 172 if h>=1050 else 145
    table_end=g['listY']+methods
    scen_start=table_end+5
    scenario_h=31 if small else 46
    params=scen_start+scenario_h+1
    edit_top=params+60+88
    action=params+94+88
    assert edit_top+30<=action
    if small:
        detail=action-88+46+23+88
        result_h=max(20,h-detail-16)
    else:
        compare_y=action-88+47+23+88
        compare_h=145 if h>=1050 else 102
        detail=compare_y+compare_h+7+23
        result_h=max(24,h-detail-16)
    assert detail+result_h<=h-12, (w,h,'result extends beyond client')
    assert x+cw<=w-16
    print(f'PASS {w}x{h}: header rows no overlap, results fit ({result_h}px)')
# The placeholder row must match the 8 real columns and must not corrupt last status.
header=text.split('econSchemesList = newList',1)[1].split('})',1)[0]
assert header.count('{"')==8
empty=text.split('if len(schemes) == 0 {',1)[1].split('return',1)[0]
assert re.search(r'listAdd\(econSchemesList, 0, \[\]string\{[^}]*\}\)',empty)
assert len(re.findall('"',empty.split('listAdd(econSchemesList, 0, []string{',1)[1].split('}',1)[0]))//2==8
print('PASS section 6 header, 8-column empty state and preserved actions')
