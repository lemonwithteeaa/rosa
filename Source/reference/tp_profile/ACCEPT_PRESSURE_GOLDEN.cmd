@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0accept_pressure_golden.ps1" %*
echo.
if errorlevel 1 (
  echo Pressure golden acceptance: ERROR. See message above.
) else (
  echo Pressure golden acceptance: completed.
)
pause
