from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
versions=re.findall(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
version_ok=bool(versions and int(versions[-1])>=725)
checks={
 'version 725 or newer': version_ok,
 'object card enlarged': 'objectH := int32(66)' in report and 'objectH = 60' in report and 'objectH = 54' in report,
 'six report cards enlarged': 'cardsH := int32(98)' in report and ('cardsH = 78' in report or 'cardsH = 86' in report) and ('cardsH = 64' in report or 'cardsH = 72' in report),
 'object heading larger': 'headerR.Bottom = min32(r.Bottom, r.Top+20)' in report and 'hFontHeaderSmall' in report,
 'metric captions larger': 'labelH := int32(19)' in report and 'hFontBold' in report,
 'variant subtitle protected': 'setPos(resultVariantsSubtitle, x, variantTitleY+24, variantTextW, 22)' in report,
 'archive subtitle protected': 'setPos(resultArchiveSubtitle, x, archiveTitleY+24, archiveTextW, 22)' in report,
 'registry banner enlarged': 'bannerH := int32(82)' in report and ('bannerH = 74' in report or 'bannerH = 78' in report),
 'registry stats enlarged': 'statsH := int32(94)' in report and ('statsH = 82' in report or 'statsH = 90' in report),
 'registry hint protected': ('setPos(archiveRegistryHint, x, titleY+36, contentW, 24)' in report or 'setPos(archiveRegistryHint, x, titleY+46, contentW, 24)' in report),
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC725 density/style QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
