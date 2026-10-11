"""Audit saved CO2 references; optionally repeat with an explicit PHREEQC executable."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent
NAMES = ('external_reservoir', 'closed_volume', 'insufficient_reservoir', 'closed_small_volume', 'closed_large_volume')

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def final_row(path):
    lines = path.read_text().splitlines()
    headers = [x.strip() for x in lines[0].split('\t') if x.strip()]
    rows = [[x.strip() for x in line.split('\t') if x.strip()] for line in lines[1:] if line.strip()]
    rows = [dict(zip(headers, row)) for row in rows if len(row) == len(headers)]
    row = [r for r in rows if r['state'] == 'react'][-1]
    return {k: float(v) for k, v in row.items() if k != 'state'}

def audit(root):
    a = final_row(root / 'external_reservoir' / 'selected.tsv')
    b = final_row(root / 'closed_volume' / 'selected.tsv')
    dump = (root / 'closed_volume' / 'initial_gas.dump').read_text()
    initial = float(re.search(r'^\s+-moles\s+(\S+)', dump, re.M).group(1))
    residual = b['total_carbon_mol'] - initial
    assert abs(residual) < 1e-12, ('carbon balance', residual)
    assert math.isclose(a['co2_partial_atm'], 10**-3.5, rel_tol=1e-9), 'reservoir pressure not reached'
    assert abs(a['co2_phase_si'] - math.log10(a['co2_partial_atm'] * a['co2_phi'])) < 1e-9, 'pressure/fugacity mismatch'
    assert a['carbon_water_mol'] > 0 and b['carbon_water_mol'] > 0
    assert 0 < b['co2_gas_mol'] < initial
    assert abs(b['volume'] - 1) < 1e-10 and 0 < b['pressure'] < .01
    for row in (a, b):
        assert all(math.isfinite(value) for value in row.values())
        for element in ('Na', 'Cl'):
            assert abs(row[element] * row['water_kg'] - .01) < 1e-12
        assert 0 < row['water_activity'] <= 1
    volumes = []
    for name, volume in (('closed_small_volume', .1), ('closed_volume', 1), ('closed_large_volume', 10)):
        row = final_row(root / name / 'selected.tsv')
        raw = (root / name / 'initial_gas.dump').read_text()
        carbon = float(re.search(r'^\s+-moles\s+(\S+)', raw, re.M).group(1))
        assert abs(row['total_carbon_mol'] - carbon) < 1e-12, (name, 'carbon balance')
        assert abs(row['volume'] - volume) < 1e-10, (name, 'volume')
        assert 0 < row['co2_gas_mol'] < carbon
        assert 0 < row['pressure'] < .01
        assert all(math.isfinite(value) for value in row.values())
        for element in ('Na', 'Cl'):
            assert abs(row[element] * row['water_kg'] - .01) < 1e-12
        volumes.append(row)
    assert volumes[0]['carbon_water_mol'] < volumes[1]['carbon_water_mol'] < volumes[2]['carbon_water_mol']
    assert volumes[0]['pH'] > volumes[1]['pH'] > volumes[2]['pH']
    poor = final_row(root / 'insufficient_reservoir' / 'selected.tsv')
    assert abs(poor['reservoir_stock_mol']) < 1e-16
    assert abs(poor['carbon_water_mol'] - 1e-8) < 1e-14
    assert poor['co2_phase_si'] < -3.5 - 1, 'unattained target mistaken for equilibrium'
    assert poor['pH'] > a['pH']
    return {'closed_initial_carbon_mol': initial, 'closed_final_carbon_mol': b['total_carbon_mol'],
            'closed_carbon_residual_mol': residual, 'reservoir_phase_si': a['co2_phase_si'],
            'reservoir_final_pH': a['pH'], 'closed_final_pH': b['pH'], 'insufficient_reservoir_pH': poor['pH'],
            'insufficient_reservoir_phase_si': poor['co2_phase_si'],
            'volume_series_pH': [row['pH'] for row in volumes],
            'closed_pressure_vs_water_SI_log_residual': [row['co2_phase_si']-math.log10(row['co2_partial_atm']*row['co2_phi']) for row in volumes]}

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--engine', help='Path to PHREEQC executable; omit to audit saved results only')
    args = parser.parse_args()
    manifest = json.loads((ROOT / 'RUN_MANIFEST.json').read_text())
    assert sha(ROOT / 'phreeqc.dat') == manifest['database_sha256'], 'database changed'
    for name in NAMES:
        assert sha(ROOT / (name + '.pqi')) == manifest['inputs'][name], 'input changed'
        saved = final_row(ROOT / name / 'selected.tsv')
        golden = final_row(ROOT / name / 'golden.tsv')
        assert saved == golden, 'saved output changed'
    result = audit(ROOT)
    if args.engine:
        engine = str(Path(args.engine).resolve())
        with tempfile.TemporaryDirectory() as temp:
            target = Path(temp)
            shutil.copy2(ROOT / 'phreeqc.dat', target / 'phreeqc.dat')
            for name in NAMES:
                d = target / name
                d.mkdir()
                shutil.copy2(ROOT / (name + '.pqi'), d / 'input.pqi')
                run = subprocess.run([engine, 'input.pqi', 'full_output.txt', '../phreeqc.dat'], cwd=d, capture_output=True, text=True)
                assert run.returncode == 0, run.stderr
                actual = final_row(d / 'selected.tsv')
                expected = final_row(ROOT / name / 'golden.tsv')
                for key in expected:
                    assert math.isclose(actual[key], expected[key], rel_tol=1e-9, abs_tol=1e-12), (name, key, actual[key], expected[key])
            audit(target)
        result['replay'] = 'passed'
    print(json.dumps(result, indent=2))

if __name__ == '__main__':
    main()
