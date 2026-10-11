from pathlib import Path
root = Path(__file__).resolve().parent
report = (root/'report_workspace.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
checks = {
    'version 730 or newer': ('const appBuildVersion = "7.3.11.730-RC"' in main or 'const appBuildVersion = "7.3.11.731-RC"' in main),
    'section card command base': 'ID_RESULT_SECTION_CARD_BASE = 9900' in report,
    'cards notify clicks': 'SS_OWNERDRAW|SS_NOPREFIX|SS_NOTIFY' in report,
    'six cards use clickable constructor': 'resultSectionCards[i] = resultSectionCard' in report,
    'card page mapping': all(x in report for x in ['return ID_OBJECTS','return ID_LAB','return ID_ASSESS','return ID_ACTIVITIES','return ID_MRP','return ID_ECON']),
    'click dispatch': 'id >= ID_RESULT_SECTION_CARD_BASE' in report and 'switchPage(page)' in report,
    'laboratory summary concise': 'labPrimary = "Заполнено и проверено"' in report,
    'assessment summary concise': 'readyPrimary = "Утверждено"' in report,
    'primary typography scales with card': 'cardW >= 245 && cardH >= 90' in report and 'primaryFont = hFontSection' in report and 'cardW >= 188 && cardH >= 80' in report and 'primaryFont = hFontBold' in report and 'primaryFont = hFontHeaderSmall' in report and 'primaryFont = hFontResultHint' not in report,
    'click affordance': 'drawReportText(di.HDC, chev, "›"' in report,
}
failed = [k for k,v in checks.items() if not v]
for k,v in checks.items(): print(('PASS' if v else 'FAIL') + ': ' + k)
if failed: raise SystemExit('RC730 gate failed: ' + ', '.join(failed))
print(f'RC730 gate passed: {len(checks)}/{len(checks)}')
