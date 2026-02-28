@echo off
setlocal
set SCRIPT_DIR=%~dp0

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%uninstall.ps1"
set EXIT_CODE=%ERRORLEVEL%

if not "%EXIT_CODE%"=="0" (
  echo.
  echo Uninstall failed with exit code %EXIT_CODE%.
) else (
  echo.
  echo Uninstall completed.
)

pause
exit /b %EXIT_CODE%
