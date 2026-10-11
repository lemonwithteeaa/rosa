from pathlib import Path
import re, sys
main = Path('main.go').read_text(encoding='utf-8')
calc = Path('laboratory_calculation_basis.go').read_text(encoding='utf-8')
lab = Path('laboratory.go').read_text(encoding='utf-8')
vis = Path('laboratory_visualization.go').read_text(encoding='utf-8')
load = Path('startup_loading.go').read_text(encoding='utf-8')

def body(src, fn):
    m = re.search(r'func\s+'+re.escape(fn)+r'\([^\)]*\)[^{]*\{', src)
    if not m: return ''
    i=m.end(); depth=1; j=i
    while j < len(src) and depth:
        if src[j]=='{': depth+=1
        elif src[j]=='}': depth-=1
        j+=1
    return src[i:j-1]

open_body = body(calc, 'openLaboratoryCalculationBasisDialog')
risk_body = body(calc, 'calculationRisk')
checks = {
    'version 556': 'const appBuildVersion = "7.3.11.556-RC"' in main,
    'open does not persist draft': 'saveLabDraft()' not in open_body,
    'open does not scan all passes': 'loadLabPasses()' not in open_body and 'sort.Slice' not in open_body,
    'zero-write calculation snapshot': 'func calculationBasisCurrentSnapshot()' in calc and 'currentLaboratoryUnits()' in calc,
    'window starts with preview': 'showCalculationBasisPreview()' in calc and 'case idBasisRun:\n\t\t\tcalculationBasisRefresh()' in calc,
    'heavy work background guarded': 'go func(pass LabPassRecord' in calc and 'recover()' in calc and 'wmBasisCalculationDone' in calc,
    'single current salt model default': 'model.Code == labScaleModelCode' in calc and 'uintptr(i+1)' in calc,
    'scale risk not recomputed via all-model report': 'calculationScaleAllModels(values)' not in risk_body,
    'legend measured at active dpi': 'laboratoryLegendMeasuredCaptionWidth' in vis and 'DT_CALCRECT' in vis,
    'legend wraps to two rows': 'items[:3], items[3:]' in vis,
    'abrupt-change legend has room': '"резкое изменение > 30 %": 196' in vis,
    'legend band tall enough': all(x in lab for x in ['LegendH: 44', 'LegendH: 46', 'LegendH: 48']),
    'loading redesigned light surface': 'rgb(247, 249, 252)' in load and 'No large progress "bricks"' in load,
    'loading progress uses nodes': 'startupEllipse.Call' in load and 'stages := []string{"Профиль", "Данные", "Модели", "Готово"}' in load,
    'loading dimensions updated': 'const width, height = 920, 356' in load and 'ROSAStartupLoadingV3' in load,
}
failed=[name for name,ok in checks.items() if not ok]
for name,ok in checks.items(): print(('PASS' if ok else 'FAIL')+': '+name)
if failed:
    print('FAILED:', ', '.join(failed))
    sys.exit(1)
