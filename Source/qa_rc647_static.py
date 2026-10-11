from pathlib import Path
import re

root = Path(__file__).resolve().parent
main = (root/'main.go').read_text(encoding='utf-8')
rep = (root/'methods_replication.go').read_text(encoding='utf-8')
ref = (root/'methods_reference.go').read_text(encoding='utf-8')
dlg = (root/'methods_reference_dialog.go').read_text(encoding='utf-8')
rules = (root/'rules_seed.json').read_text(encoding='utf-8')

checks = {
    'version 647 or newer': bool(re.search(r'const appBuildVersion = \"7\.3\.11\.(\d+)-RC\"', main)) and int(re.search(r'const appBuildVersion = \"7\.3\.11\.(\d+)-RC\"', main).group(1)) >= 647,
    'analogs use one evidence index': 'replicationEvidenceIndex(d, now)' in rep,
    'analogs include ROSA archive': 'replicationCandidates()' in rep and 'PreviouslyAnalyzed' in rep and 'Архив РОСА' in rep,
    'candidate loop avoids per-object history scan': 'replicationDirectionClassIndexed(cand.ObjectKey, d, evidence)' in rep,
    'clear applicability wording': 'ГДЕ ЕЩЁ МЕТОД МОЖЕТ БЫТЬ ПОЛЕЗЕН' in rep and 'АНАЛОГИ И ПЕРЕНОС РЕШЕНИЯ' in ref,
    'analog rows show operating mode': 'OperatingProfile' in rep and 'Режим: %s' in rep,
    'prior analysis marker': 'ранее анализировался в РОСА' in rep,
    'replication tab renamed': 'АНАЛОГИ / ПЕРЕНОС' in dlg,
    'all structural headings bold': 'DwEffects:   CFE_BOLD' in dlg,
    'generic submerged container wording in catalog': 'Погружной контейнер от АСПО, коррозии, солеотложений (ТРИЛ-К, ТРИЛ-П-См, ТРИЛ-СВ)' in rules,
    'generic submerged container wording in technologies': 'Погружной контейнер от АСПО, коррозии, солеотложений (ТРИЛ-К, ТРИЛ-П-См, ТРИЛ-СВ)' in main,
    'old brand-bound names removed': 'Погружной контейнер ТРИЛ-П-СМ (АСПО)' not in rules and 'Погружной контейнер ТРИЛ-К (коррозия)' not in rules,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL') + ': ' + name)
print(f'{sum(checks.values())}/{len(checks)} PASS')
raise SystemExit(0 if all(checks.values()) else 1)
