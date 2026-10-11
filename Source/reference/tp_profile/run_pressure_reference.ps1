param(
    [string]$PhreeqcExe = "",
    [string]$Database = ""
)
$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here

function Find-Phreeqc {
    param([string]$Requested)
    if ($Requested -and (Test-Path $Requested)) { return (Resolve-Path $Requested).Path }
    if ($env:PHREEQC_EXE -and (Test-Path $env:PHREEQC_EXE)) { return (Resolve-Path $env:PHREEQC_EXE).Path }
    $localCandidates = @(
        (Join-Path $here "phreeqc.exe"),
        (Join-Path $here "engine\phreeqc.exe"),
        (Join-Path $here "portable\phreeqc.exe")
    )
    foreach ($p in $localCandidates) { if (Test-Path $p) { return (Resolve-Path $p).Path } }
    $cmd = Get-Command phreeqc.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $candidates = @(
        "C:\Program Files\USGS\phreeqc-3.8.6-17100\bin\phreeqc.exe",
        "C:\Program Files\USGS\PHREEQC 3.8.6-17100\bin\phreeqc.exe",
        "C:\Program Files\USGS\phreeqc\bin\phreeqc.exe",
        "C:\Program Files (x86)\USGS\phreeqc-3.8.6-17100\bin\phreeqc.exe"
    )
    foreach ($p in $candidates) { if (Test-Path $p) { return $p } }
    return ""
}

$exe = Find-Phreeqc $PhreeqcExe
if (-not $exe) {
    Write-Host "PHREEQC не найден." -ForegroundColor Yellow
    Write-Host "Установите официальный USGS PHREEQC 3.8.6-17100, положите phreeqc.exe рядом со скриптом/в engine\, либо запустите:" -ForegroundColor Yellow
    Write-Host "  powershell -ExecutionPolicy Bypass -File .\run_pressure_reference.ps1 -PhreeqcExe C:\path\to\phreeqc.exe"
    Write-Host "Официальная страница: https://water.usgs.gov/water-resources/software/PHREEQC/"
    exit 2
}

if (-not $Database) { $Database = Join-Path $here "phreeqc.dat" }
if (-not (Test-Path $Database)) { throw "База PHREEQC не найдена: $Database" }
$input = Join-Path $here "pressure_reference.pqi"
$full = Join-Path $here "pressure_full_output.txt"
$selected = Join-Path $here "pressure_selected.tsv"
$manifest = Join-Path $here "pressure_execution_manifest.txt"
Remove-Item $full,$selected -ErrorAction SilentlyContinue

$exeItem = Get-Item $exe
$exeHash = (Get-FileHash -Algorithm SHA256 $exe).Hash.ToLower()
$dbHash = (Get-FileHash -Algorithm SHA256 $Database).Hash.ToLower()
$inputHash = (Get-FileHash -Algorithm SHA256 $input).Hash.ToLower()
$productVersion = $exeItem.VersionInfo.ProductVersion
$fileVersion = $exeItem.VersionInfo.FileVersion

Write-Host "Запуск официального PHREEQC:" $exe
& $exe $input $full $Database
$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) { throw "PHREEQC завершился с кодом $exitCode" }
if (-not (Test-Path $selected)) { throw "PHREEQC завершился, но pressure_selected.tsv не создан." }

@(
    "executed_at=$(Get-Date -Format o)",
    "engine_path=$exe",
    "engine_product_version=$productVersion",
    "engine_file_version=$fileVersion",
    "engine_sha256=$exeHash",
    "database_path=$Database",
    "database_sha256=$dbHash",
    "input_sha256=$inputHash",
    "full_output=pressure_full_output.txt",
    "selected_output=pressure_selected.tsv",
    "status=EXECUTED_NOT_YET_ACCEPTED"
) | Set-Content -Encoding UTF8 $manifest

$python = Get-Command python.exe -ErrorAction SilentlyContinue
if (-not $python) { $python = Get-Command py.exe -ErrorAction SilentlyContinue }
if ($python) {
    if ($python.Name -ieq "py.exe") { & $python.Source -3 (Join-Path $here "verify_pressure_reference.py") }
    else { & $python.Source (Join-Path $here "verify_pressure_reference.py") }
    if ($LASTEXITCODE -ne 0) { throw "verify_pressure_reference.py не прошёл проверку." }
    Add-Content -Encoding UTF8 $manifest "verification=PRESSURE_REFERENCE_OK"
    Write-Host "PRESSURE_REFERENCE_OK" -ForegroundColor Green
} else {
    Write-Host "Python не найден: selected/full output и хеши сохранены, но verifier не запущен." -ForegroundColor Yellow
    Add-Content -Encoding UTF8 $manifest "verification=NOT_RUN_NO_PYTHON"
}
