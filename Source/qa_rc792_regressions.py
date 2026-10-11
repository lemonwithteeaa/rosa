from pathlib import Path
import json

root = Path(__file__).resolve().parent
main = (root / 'main.go').read_text(encoding='utf-8')
seed = (root / 'demo_seed.go').read_text(encoding='utf-8')
lab = (root / 'laboratory.go').read_text(encoding='utf-8')
complex_ui = (root / 'laboratory_complex_analysis.go').read_text(encoding='utf-8')
infographic = (root / 'laboratory_infographic_dialog.go').read_text(encoding='utf-8')
canvas = (root / 'laboratory_infographic_canvas.go').read_text(encoding='utf-8')
corpus = json.loads((root / 'training_corpus.json').read_text(encoding='utf-8'))
ids = [p['passId'] for w in corpus['wells'] for p in w['labPasses']]
failure_ids = [f['id'] for w in corpus['wells'] for f in w['failures']]

checks = {
    'corpus is exactly 20 wells with 60 unique lab passes and failures': len(corpus['wells']) == 20 and len(ids) == 60 and len(set(ids)) == 60 and len(failure_ids) == 60 and len(set(failure_ids)) == 60,
    'prior imported demo passes and legacy lab records are retired': 'isSupersededTrainingPass(old)' in seed and 'isSupersededTrainingLabRecord(old)' in seed and 'old.Deleted = true' in seed,
    'old demo failure events and stale tech keys are retired at startup': 'retireLegacyTeachingFailures()' in main and 'isRetiredLegacyTeachingObjectKey(state.ObjectKey)' in main,
    'new receipt generation causes the current 20-well set to be reconciled': 'training20_receipts_791.json' in seed,
    'new release opens on the canonical first well': 'trainingWellByOrdinal(1)' in (root / 'object_selection.go').read_text(encoding='utf-8'),
    'overall view re-hides input fields after relayout': 'Reapply this after all geometry updates' in lab and 'setLaboratoryFieldsVisible(false)' in lab,
    'charts paint on dedicated WM_PAINT child canvases': 'case 0x000F: // WM_PAINT' in canvas and 'createLaboratoryInfographicCanvas' in infographic and 'createLaboratoryInfographicCanvas(h, idComplexChart' in complex_ui,
    'complex window uses native close/minimize/maximize caption controls': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN_FRAME' in complex_ui and 'modalCreate("BUTTON", "Закрыть"' not in complex_ui and 'modalCreate("BUTTON", "Развернуть"' not in complex_ui,
    'infographic window keeps native caption controls': 'WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN_FRAME' in infographic,
    'warning is a real owner-draw keyboard/mouse button': 'create("BUTTON", "ВНИМАНИЕ"' in (root / 'section_banner_settings.go').read_text(encoding='utf-8') and 'BS_OWNERDRAW' in (root / 'section_banner_settings.go').read_text(encoding='utf-8') and 'drawSectionNoticeButton(di)' in main,
    'warning pulse interval slowed to 1.2 seconds': 'sectionNoticePulseTimerID, 1200, 0' in (root / 'section_banner_settings.go').read_text(encoding='utf-8'),
    'one click opens the configured teaching message': 'if id == idSectionNoticeLabel && code == 0' in main and 'msg("Внимание", sectionNoticeMessage' in (root / 'section_banner_settings.go').read_text(encoding='utf-8'),
    'release version is 792': 'const appBuildVersion = "7.3.11.792-RC"' in main,
}
for label, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + label)
if not all(checks.values()):
    raise SystemExit(1)
