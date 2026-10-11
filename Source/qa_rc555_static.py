from pathlib import Path
s = Path('laboratory_calculation_basis.go').read_text(encoding='utf-8')
main = Path('main.go').read_text(encoding='utf-8')
checks = {
    'background calculation worker': 'go func(pass LabPassRecord' in s,
    'completion marshalled to UI': 'wmBasisCalculationDone' in s and 'pPostMessageW.Call' in s,
    'busy guard': 'setCalculationBasisBusy(true)' in s and 'setCalculationBasisBusy(false)' in s,
    'old synchronous hot path removed': 'text := calculationPassReportWithModel(calculationBasisSelectedPass(), index, full, calculationBasisSelectedModel(index))' not in s,
    'build version': 'const appBuildVersion = "7.3.11.555-RC"' in main,
}
failed = [name for name, ok in checks.items() if not ok]
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
if failed:
    raise SystemExit(1)
