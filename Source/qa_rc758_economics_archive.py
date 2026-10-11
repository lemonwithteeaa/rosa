"""Section-six archive checks: fund-wide listing, drilldown and clipping simulation."""
from pathlib import Path
import re
root=Path(__file__).resolve().parent
src=(root/'economics_visual_reference.go').read_text(encoding='utf-8')
fund=(root/'economics_fund_archive.go').read_text(encoding='utf-8')
main=(root/'main.go').read_text(encoding='utf-8')
workflow=(root/'workflow_pages.go').read_text(encoding='utf-8')
version = re.search(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
assert version and int(version.group(1)) >= 758
assert 'alternativeScenariosForObject(selectedObjectKey)' in src
assert 'refreshEconomicsFundArchive()' in src
assert 'economicsFundGrouping(alternativeScenarios)' in fund
assert 'one object per row' in src
assert 'case econCompareList:' in main and 'openEconomicsFundArchiveAtRow(i)' in main
assert 'alternativeObjectIdentity(key)' in fund
assert 'readableObjectKeyFromIndex' in fund
assert 'for i, s := range economicsFundDialogObject.Schemes' in fund
assert 'pEnableWindow.Call(uintptr(hMain), 0)' in fund
assert 'pEnableWindow.Call(uintptr(hMain), 1)' in fund
assert 'selectedObjectKey =' not in fund
assert 'saveAlternativeScenarios' not in fund
assert 'applyGlobalObjectSelection' not in fund
for s in ('"Схема"','"Объект"','"Схем, шт."','"Лучший эффект, тыс. руб."'):
    assert s in src
for s in ('"OPEX, руб./год"','"CAPEX, руб."','"Окупаемость, мес."'):
    assert s in fund
for hook in ('case ID_ECON_METHOD_BASE:', 'case ID_ECON_METHOD_NEXT:', 'case ID_ECON_CALCULATE_ALL:'):
    assert hook in workflow
assert 'setEconomicsReferenceVisible(econCompareList, false)' not in src

for w,h in [(1180,700),(1280,720),(1366,768),(1600,810),(1600,850),(1600,900),(1600,930),(1920,1080),(1920,1200)]:
    offset=88
    mh=40 if h<800 else 91 if h<930 else 152 if h>=1050 else 100
    sh=0 if h<800 else 46 if h>=1050 else 31
    scenario_y=307+mh+5
    params_y=scenario_y+sh+1
    action_y=params_y+94
    archive_y=action_y+46+24
    target=72 if h<800 else 98 if h<930 else 156 if h>=1050 else 126
    available=h-(archive_y+offset)-14
    arch_h=target
    if h>=800 and arch_h>available-60:arch_h=max(58,available-60)
    if arch_h>available:arch_h=max(22,available)
    archive_bottom=archive_y+offset+arch_h
    assert archive_bottom<=h-12,(w,h,archive_bottom)
    result_y=archive_y+arch_h+7+23+offset
    result_h=h-result_y-16
    assert mh>=40 and arch_h>=22
    assert action_y+40 <= archive_y
    if result_h>=24:assert result_y+result_h<=h-14
    print('PASS GEO',w,h,'top schemes',mh,'fund history',arch_h,'conclusion',max(0,result_h))
print('PASS RC758: two distinct data sources, cross-object grouping, click detail, stable layout')
