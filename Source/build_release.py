"""Single release entry point. Version and icon identity are source-controlled."""
from pathlib import Path
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys

root = Path(__file__).resolve().parent
package = root.parent
main = (root / 'main.go').read_text(encoding='utf-8')
version = re.search(r'^const appBuildVersion = "([^"]+)"', main, re.M).group(1)
approved_icon_sha256 = 'd63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a'
icon = root / 'OF_Model_icon_round.ico'
icon_syso = root / 'icon_windows_amd64.syso'
if hashlib.sha256(icon.read_bytes()).hexdigest() != approved_icon_sha256:
    raise SystemExit('Release refused: approved application icon changed')
if '//go:embed OF_Model_icon_round.ico' not in main:
    raise SystemExit('Release refused: runtime icon no longer embedded')
if not icon_syso.exists():
    raise SystemExit('Release refused: native link-time icon resource object missing')
switch = (root / 'user_switch.go').read_text(encoding='utf-8')
for marker in ['ensureDataWithVisibleTransition()', 'activatePage(ID_DASH, true)', 'SW_MAXIMIZE']:
    if marker not in switch:
        raise SystemExit('Release refused: incomplete user-switch lifecycle: ' + marker)
for marker in ['if id == ID_HEADER_EXIT {', 'case ID_HEADER_APPROVE:']:
    if 'switchCurrentUser()' not in main[main.index(marker):main.index(marker)+130]:
        raise SystemExit('Release refused: user-switch commands diverged')
if 'ID_ASSESS_CONFIRM_FULL    = 9610' not in main or 'ID_HEADER_EXIT            = 9441' not in main:
    raise SystemExit('Release refused: classification approval can collide with Exit')
if 'const userSwitchCaption = "Смена пользователя"' not in switch:
    raise SystemExit('Release refused: user-switch caption changed')

background = (root/'methods_background.go').read_text(encoding='utf-8')
methods = (root/'methods_workflow.go').read_text(encoding='utf-8')
for marker in ['go func()', 'methodsBackgroundResultIsCurrent', 'empiricalMethodProbabilityFromCohort']:
    if marker not in background:
        raise SystemExit('Release refused: Methods background pipeline missing: ' + marker)
if 'invalidateMethodsBackground()' not in switch:
    raise SystemExit('Release refused: a Methods task survives user switch')
info = methods[methods.index('func refreshMethodsSuggestionInfo'):methods.index('func relayoutMethodsWorkflowCurrent')]
if 'methodReplicationAssessmentForSuggestion(' in info:
    raise SystemExit('Release refused: fleet replication returned to the page-entry path')
reference = (root/'methods_reference.go').read_text(encoding='utf-8')
selected_card = reference[reference.index('func selectedMethodReferenceText'):reference.index('func methodsReferenceText')]
if 'methodReplicationAssessmentForSuggestion(' in selected_card:
    raise SystemExit('Release refused: fleet search returned to Reference opening')
if 'ID_METHODS_SHOW_ALL' not in methods or 'mergeAllMethodSuggestions' not in methods:
    raise SystemExit('Release refused: full Methods catalogue switch missing')
if 'rankMethodSuggestionsWithLimit(candidates, scores, 0)' not in background:
    raise SystemExit('Release refused: full Methods catalogue is capped')
if 'workspaceInset := int32(9)' not in methods or 'layoutMethodsWorkspacePanels(x, midX, rightX' not in methods:
    raise SystemExit('Release refused: RC650 aligned Methods workspace missing')
if 'showAllW := min32(210, max32(186' not in methods or 'forecastUsable / 4' not in methods:
    raise SystemExit('Release refused: Methods visual hierarchy regressed')
if 'methodsCandidateCardHeights(titleTextH, detailTextH, cardH)' not in methods or 'perPage := 4' not in methods:
    raise SystemExit('Release refused: long method plaques can be clipped')
forecast_detail = (root/'methods_forecast_details.go').read_text(encoding='utf-8')
for marker in ['ID_METHODS_FORECAST_BASE', 'methodsClickableCenteredCardLabel', 'openMethodsForecastDetail(id)']:
    if marker not in methods:
        raise SystemExit('Release refused: clickable forecast detail cards missing: ' + marker)
for marker in ['Фактические завершённые циклы, вошедшие в базу', 'остаточный риск', 'ЧТО РОСА НЕ ДЕЛАЕТ']:
    if marker not in forecast_detail:
        raise SystemExit('Release refused: forecast explanation detail missing: ' + marker)
typography = (root/'methods_typography.go').read_text(encoding='utf-8')
for marker in ['methodsTypeDisplay = 26', 'methodsTypeSection = 19', 'methodsTypeBody    = 16', 'methodsTypeCompact = 14']:
    if marker not in typography:
        raise SystemExit('Release refused: Methods typography scale regressed: ' + marker)
subprocess.run([sys.executable, str(root/'qa_rc632_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc647_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc648_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc649_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc650_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc651_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc652_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc653_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc654_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc655_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc656_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc657_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc658_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc659_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc660_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc661_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc661_layout_matrix.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc663_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc673_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc674_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc677_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc678_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc699_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc700_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc701_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc702_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc703_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc704_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc705_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc705_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc706_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc706_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc707_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc708_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc708_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc709_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc709_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc710_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc710_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc711_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc711_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc712_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc712_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc713_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc713_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc714_static.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc714_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc716_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc717_layout.py')], check=True)
# RC718/RC719 caption-position gates were superseded by the unified button contract.
subprocess.run([sys.executable, str(root/'qa_rc721_variants.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc722_comparison.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc723_button_help.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc724_report_registry_style.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc724_registry_exports.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc724_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc725_report_density.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc725_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc726_section_exports.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc727_registry_no_clip.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc727_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc728_report_cards_no_clip.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc729_variant_title.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc730_clickable_report_cards.py')], check=True)
# RC731 clickable archive KPI cards were intentionally superseded in RC732.
subprocess.run([sys.executable, str(root/'qa_rc732_archive_cards_static.py')], check=True)
# RC736/RC737 validated the superseded Save As + staging/rename pipeline.
# Section 8 now uses direct final-format worker output; RC739/RC740 gates below
# cover the active behavior and its process-stability boundary.
subprocess.run([sys.executable, str(root/'qa_rc739_archive_interactions.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc740_stability.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc741_visibility_export.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc742_detailed_registry.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc743_report_format.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc744_report_content.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc747_release.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc748_mrp.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc748_layout.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc749_mrp_visual_stability.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc750_report.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc760_stability_economics.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc761_economics_window.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc762_metric_emphasis.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc763_responsiveness.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc764_economics_archive_visual.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc765_ui_stability.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc759_nno_history.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc771_nno_reference.py')], check=True)
subprocess.run([sys.executable, str(root/'qa_rc797_stability_corpus.py')], check=True)

go_name = 'go.exe' if platform.system() == 'Windows' else 'go'
bundled = package / 'Компилятор' / 'go-1.23.2' / 'bin' / go_name
go = str(bundled) if bundled.exists() else shutil.which(go_name)
if not go:
    raise SystemExit('Go compiler for this operating system was not found. The bundle contains Linux Go 1.23.2 for cross-compilation; native Windows requires Go for Windows.')
compiler_version = subprocess.check_output([go, 'version'], text=True).strip()
env = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0')
if bundled.exists():
    env['GOROOT'] = str(bundled.parent.parent)
    if platform.system() != 'Windows':
        for directory in [bundled.parent, bundled.parent.parent / 'pkg' / 'tool' / 'linux_amd64']:
            for path in directory.iterdir():
                if path.is_file(): path.chmod(path.stat().st_mode | 0o111)

if platform.system() == 'Linux':
    subprocess.run([sys.executable, str(root/'qa_rc632_portable.py')], check=True)
    subprocess.run([sys.executable, str(root/'qa_rc632_classification_link.py')], check=True)

outdir = package / 'Windows_x64'
outdir.mkdir(exist_ok=True)
final = outdir / ('РОСА_' + version + '.exe')
raw = outdir / ('РОСА_' + version + '.raw.exe')
try:
    # icon_windows_amd64.syso is consumed by the Go linker itself. This produces
    # a normal native PE resource section instead of mutating the finished EXE.
    subprocess.run([go, 'build', '-buildvcs=false', '-trimpath', '-ldflags=-H=windowsgui', '-o', str(final), '.'], cwd=root, env=env, check=True)
    subprocess.run([sys.executable, str(root/'verify_icon_resource.py'), str(final), str(icon)], check=True)
    tests = package / 'Тесты'
    tests.mkdir(exist_ok=True)
    subprocess.run([go, 'test', '-buildvcs=false', '-c', '-o', str(tests/('РОСА_'+version+'_tests_windows.exe'))], cwd=root, env=env, check=True)
    manifest = {'version':version, 'icon_sha256':approved_icon_sha256, 'exe_sha256':hashlib.sha256(final.read_bytes()).hexdigest(),
                'compiler':compiler_version, 'target':'Windows x64 GUI', 'runtime_icon':'embedded approved ICO', 'test_execution':'see TEST_REPORT'}
    (outdir/'release_manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
finally:
    raw.unlink(missing_ok=True)
print('Release built and icon verified:', final)
