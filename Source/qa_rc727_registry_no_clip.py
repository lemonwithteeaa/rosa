from pathlib import Path
import re
root=Path(__file__).resolve().parent
main=(root/'main.go').read_text(encoding='utf-8')
report=(root/'report_workspace.go').read_text(encoding='utf-8')
versions=re.findall(r'^const appBuildVersion = "7\.3\.11\.(\d+)-RC"', main, re.M)
checks={
 'version 727 or newer': bool(versions and int(versions[-1])>=727),
 'registry banner has wrap reserve': 'bannerH := int32(100)' in report and 'bannerH = 92' in report,
 'registry metric cards have third-line reserve': 'statsH := int32(112)' in report and 'statsH = 104' in report,
 'registry title has full height': 'setPos(archiveRegistryTitle, x, titleY, titleW, 34)' in report,
 'registry hint has full height': 'setPos(archiveRegistryHint, x, titleY+48, contentW, 28)' in report,
 'registry list starts after title and hint': 'listY := titleY + 80' in report,
 'export buttons are equal width': 'pdfW, docxW, xlsxW := int32(154), int32(154), int32(154)' in report,
 'export buttons are taller': 'exportH := int32(46)' in report,
 'metric evidence keeps bottom inset': 'Bottom: di.RcItem.Bottom - 8' in report,
 'registry evidence keeps bottom inset': 'Bottom: di.RcItem.Bottom - 10' in report,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL')+': '+k)
if failed: raise SystemExit('RC727 registry no-clipping QA failed: '+', '.join(failed))
print(f'{len(checks)}/{len(checks)} PASS')
