@echo off
setlocal
cd /d "%~dp0"
where py >nul 2>nul
if %errorlevel%==0 (
  py -3 compare_pressure_golden_to_rosa.py
) else (
  where python >nul 2>nul
  if %errorlevel%==0 (
    python compare_pressure_golden_to_rosa.py
  ) else (
    echo Python 3 not found. Run compare_pressure_golden_to_rosa.py with Python 3.
    exit /b 3
  )
)
exit /b %errorlevel%
