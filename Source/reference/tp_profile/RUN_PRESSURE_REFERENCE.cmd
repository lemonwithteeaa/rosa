@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0run_pressure_reference.ps1" %*
echo.
if errorlevel 1 (
  echo Pressure benchmark: ERROR. See message above.
) else (
  echo Pressure benchmark: completed.
)
pause
