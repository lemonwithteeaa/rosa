from pathlib import Path
import hashlib
import re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
methods = (root/'methods_workflow.go').read_text(encoding='utf-8')
switch = (root/'user_switch.go').read_text(encoding='utf-8')
release = (root/'build_release.py').read_text(encoding='utf-8')
checks = {
    'version 617': 'const appBuildVersion = "7.3.11.617-RC"' in main,
    'approved ICO hash': hashlib.sha256((root/'OF_Model_icon_round.ico').read_bytes()).hexdigest() == 'd63b3dea920b18114fd62da4ddaa92cad91979663e732e523242909d06ac244a',
    'runtime ICO embedded': '//go:embed OF_Model_icon_round.ico' in main,
    'shared switch handler': all('switchCurrentUser()' in main[main.index(x):main.index(x)+130] for x in ['if id == ID_HEADER_EXIT {','case ID_HEADER_APPROVE:']),
    'user caption': 'const userSwitchCaption = "Смена\\r\\nпользователя"' in switch,
    'user switch returns visible dashboard': all(x in switch for x in ['ensureDataWithVisibleTransition()','activatePage(ID_DASH, true)','SW_MAXIMIZE','WM_SETICON']),
    'release uses source version': 're.search' in release and 'appBuildVersion' in release,
    'mandatory binary icon verification': 'verify_icon_resource.py' in release,
    'grades outside comparator': 'ai := scores[out[i].Method.Name]' in methods and 'aj := scores[out[j].Method.Name]' in methods,
    'hidden tables not repopulated': 'listAdd(' not in methods[methods.index('func refreshMethodsVisualTables'):methods.index('func methodsKeyDataText')],
    'native checkboxes': 'BS_CHECKBOX' in methods and 'BM_SETCHECK' in methods,
    'native cards have no owner draw': 'BS_OWNERDRAW' not in methods[methods.index('func methodsSafeCandidateButton'):methods.index('func methodsScrollableCard')],
    'draft toggle': 'setCombinedMethods(key, next)' in methods,
    'explicit empty draft cannot be approved': 'выберите хотя бы один метод для утверждения' in methods,
    'reference proportions': 'leftW := contentW * 44 / 100' in methods and 'midW := contentW * 27 / 100' in methods,
    'adaptive paging': 'perPage = 2' in methods and 'perPage = 4' in methods,
    'plan, archive and formats remain': all(x in methods for x in ['openMethodsPlanEditor','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']) or all(x in methods for x in ['methodsPlanButton','methodsArchiveList','exportMethodsWorkflowReport("xlsx")','exportMethodsWorkflowReport("docx")','exportMethodsWorkflowReport("pdf")']),
}
for name, ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
