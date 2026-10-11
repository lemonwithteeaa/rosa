from pathlib import Path

root = Path(__file__).resolve().parent
mw = (root/'methods_workflow.go').read_text(encoding='utf-8')
mr = (root/'methods_reference_dialog.go').read_text(encoding='utf-8')
main = (root/'main.go').read_text(encoding='utf-8')
checks = {
    'version_583': 'const appBuildVersion = "7.3.11.583-RC"' in main,
    'pager_ids': 'ID_METHODS_CANDIDATE_PREV' in mw and 'ID_METHODS_CANDIDATE_NEXT' in mw,
    'six_cards_page': 'ID_METHODS_CARD_COUNT        = 6' in mw,
    'pager_index_mapping': 'methodsCandidateIndexForCardID' in mw,
    'card_applicability_reliability': 'Прим. %d%% · дост. %d%%' in mw,
    'forecast_base': 'Базовая оценка' in mw,
    'forecast_conservative': 'Сдержанный' in mw,
    'forecast_calculated': 'Расчётный' in mw,
    'forecast_favourable': 'Благоприятный' in mw,
    'forecast_note': 'methodsForecastNote' in mw and 'Предыстория 36 мес. недостаточна' in mw,
    'compact_plan_visible': 'show(methodsPlanTitle, true)' in mw and 'show(methodsPlanComment, true)' in mw,
    'archive_min_height': 'archiveH := max32(100, bottom-archiveY)' in mw,
    'native_system_caption_kept': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE' in mr,
    'no_custom_close_creation': 'methodsReferenceCloseButton = modalCreate(' not in mr,
    'no_custom_min_creation': 'methodsReferenceMinimizeButton = modalCreate(' not in mr,
    'no_custom_max_creation': 'methodsReferenceMaxRestoreButton = modalCreate(' not in mr,
    'calm_reference_body': 'Only whole structural headings are bold' in mr and 'upperHeading :=' in mr,
    'no_red_reference_heading': 'color = rgb(190, 24, 49)' not in mr,
}
failed=[k for k,v in checks.items() if not v]
for k,v in checks.items():
    print(('PASS' if v else 'FAIL')+': '+k)
print(f'{sum(checks.values())}/{len(checks)} PASS')
if failed:
    raise SystemExit('failed: '+', '.join(failed))
