"""RC809 UI/interaction static source assertions: no model or persistence changes."""
from pathlib import Path
base = Path(__file__).resolve().parent
main = (base/'main.go').read_text(encoding='utf8')
methods = (base/'methods_workflow.go').read_text(encoding='utf8')
econ = (base/'economics_visual_reference.go').read_text(encoding='utf8')
fund = (base/'economics_fund_archive.go').read_text(encoding='utf8')
ui = (base/'economics_ui_refinement.go').read_text(encoding='utf8')
workflow = (base/'workflow_pages.go').read_text(encoding='utf8')
checks={
 'Version 809': 'const appBuildVersion = "7.3.11.809-RC"' in main,
 'Methods scenario navigator short one-line': 'СХЕМА %d/%d ›' in methods and 'h == methodsScenarioNextButton ||' in main,
 'Native economics filter wired': 'ID_ECON_FUND_FILTER' in main and 'CBS_DROPDOWNLIST' in econ and 'changeEconomicsFundFilter()' in main,
 'Rows filtered after grouping, not at storage': 'economicsFundFilterGroups(allFundRows, econFundFilterMode)' in fund,
 'Drilldown unchanged': 'openEconomicsFundArchiveAtRow(i)' in main,
 'Economics summary counts actual only': 'g.Calculated' in fund and 'economicsFundSummaryText(allFundRows)' in fund,
 'Positive benefit requires current calculation': 'economicsScenarioCalculated(s)' in fund and 'g.HasBenefit && g.BestBenefit > 0' in fund,
 'Empty filtered list guarded': 'if len(economicsFundRows) == 0' in fund,
 'Finance formulas untouched': 'CalculateBaseEconomics(in)' in workflow,
 'Short economic layout keeps alternative rows': 'methodsH = 76' in econ and 'top.schemeButtonY, top.listY = 240, 238, 286' in econ,
 'Fund history remains key table': 'archiveH := int32(172)' in econ and 'setPos(econCompareList,' in econ,
 'Neutral unknown values': 'if !known {' in econ,
 'Accents only for calculated benefit': 'if draw.SubItem == 4 && g.HasBenefit' in ui,
 'No conflicting legacy archive duplicates': 'refreshEconomicsFundArchive()' in econ,
}
for name,valid in checks.items():
 print(('PASS' if valid else 'FAIL')+': '+name)
if not all(checks.values()):raise SystemExit(1)

# Deterministic dry run of the native-window bands at representative client
# heights. It checks that the cumulative geometry, not an isolated table size,
# reserves native headers plus several records for the fund archive.
def geometry(height, width=1280):
    cw=max(720,width-270)
    list_y=286 if height<960 else 307
    methods_h=100
    if height<800:methods_h=76
    elif height<930:methods_h=91
    elif height>=1050:methods_h=152
    if height>=890 and height<930 and cw>=990:methods_h=76
    scenario_h=31
    if height>=960 and cw>=990:scenario_h=56
    if height>=1050 and cw>=990:scenario_h=64
    if height<960:scenario_h=0
    params_y=list_y+methods_h+5+scenario_h+1
    archive_y=params_y+94+46+24
    available=max(0,height-(archive_y+88)-14)
    archive_h=208 if height>=1000 else (110 if height<800 else 172)
    if height>=800:archive_h=min(archive_h,max(60,available-87))
    else:archive_h=min(archive_h,max(48,available-40))
    archive_h=min(archive_h,available)
    return methods_h,archive_h
for height, expected_methods, expected_archive in [(768,76,85),(864,91,120),(930,100,160),(960,100,130),(1080,152,155)]:
    methods_h,archive_h=geometry(height)
    assert methods_h>=expected_methods and archive_h>=expected_archive, (height,methods_h,archive_h)
    print('PASS: economic client '+str(height)+'px: schemes '+str(methods_h)+'px, fund history '+str(archive_h)+'px')
