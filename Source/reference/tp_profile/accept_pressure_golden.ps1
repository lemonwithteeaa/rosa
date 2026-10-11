param(
    [string]$Selected = "",
    [string]$ExecutionManifest = ""
)
$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here
if (-not $Selected) { $Selected = Join-Path $here "pressure_selected.tsv" }
if (-not $ExecutionManifest) { $ExecutionManifest = Join-Path $here "pressure_execution_manifest.txt" }
$plannedManifest = Join-Path $here "pressure_reference_manifest.txt"
$verifier = Join-Path $here "verify_pressure_reference.py"
$golden = Join-Path $here "pressure_golden.tsv"
$goldenManifest = Join-Path $here "pressure_golden_manifest.txt"

foreach ($p in @($Selected,$ExecutionManifest,$plannedManifest,$verifier)) {
    if (-not (Test-Path $p)) { throw "Не найден обязательный файл: $p" }
}

function Read-KV([string]$Path) {
    $h = @{}
    Get-Content $Path | ForEach-Object {
        if ($_ -match '^([^=]+)=(.*)$') { $h[$matches[1].Trim()] = $matches[2].Trim() }
    }
    return $h
}
$plan = Read-KV $plannedManifest
$exec = Read-KV $ExecutionManifest

$python = Get-Command python.exe -ErrorAction SilentlyContinue
if (-not $python) { $python = Get-Command py.exe -ErrorAction SilentlyContinue }
if (-not $python) { throw "Python не найден: принятие golden запрещено без verify_pressure_reference.py." }
if ($python.Name -ieq "py.exe") { & $python.Source -3 $verifier }
else { & $python.Source $verifier }
if ($LASTEXITCODE -ne 0) { throw "Verifier pressure_reference не прошёл." }

if ($exec['verification'] -ne 'PRESSURE_REFERENCE_OK') {
    throw "В execution manifest нет verification=PRESSURE_REFERENCE_OK. Сначала выполните RUN_PRESSURE_REFERENCE.cmd."
}
if ($exec['database_sha256'] -ne $plan['database_sha256']) {
    throw "SHA-256 phreeqc.dat не совпадает с подготовленным эталоном."
}
if ($exec['input_sha256'] -ne $plan['input_sha256']) {
    throw "SHA-256 pressure_reference.pqi не совпадает с подготовленным эталоном."
}
$ver = (($exec['engine_product_version'] + ' ' + $exec['engine_file_version']).Trim())
if ($ver -notmatch '3\.8\.6' -and $ver -notmatch '17100') {
    throw "Версия PHREEQC не подтверждена как 3.8.6-17100: $ver"
}

Copy-Item -Force $Selected $golden
$goldenHash = (Get-FileHash -Algorithm SHA256 $golden).Hash.ToLower()
@(
    "accepted_at=$(Get-Date -Format o)",
    "status=ACCEPTED_EXTERNAL_GOLDEN",
    "source_selected=$(Split-Path -Leaf $Selected)",
    "golden=pressure_golden.tsv",
    "golden_sha256=$goldenHash",
    "engine_product_version=$($exec['engine_product_version'])",
    "engine_file_version=$($exec['engine_file_version'])",
    "engine_sha256=$($exec['engine_sha256'])",
    "database_sha256=$($exec['database_sha256'])",
    "input_sha256=$($exec['input_sha256'])",
    "verification=PRESSURE_REFERENCE_OK",
    "rule=golden_is_external_reference_only_pressure_effect_in_ROSA_still_requires_explicit_implementation_and_comparison"
) | Set-Content -Encoding UTF8 $goldenManifest
Write-Host "PRESSURE_GOLDEN_ACCEPTED" -ForegroundColor Green
Write-Host "Создано: $golden"
Write-Host "Manifest: $goldenManifest"
