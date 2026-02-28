@echo off
setlocal
set SCRIPT_DIR=%~dp0

powershell -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%install.ps1"
set EXIT_CODE=%ERRORLEVEL%

if not "%EXIT_CODE%"=="0" (
  echo.
  echo Installation failed with exit code %EXIT_CODE%.
) else (
  echo.
  echo Installation completed. Hooks are configured.
)

pause
exit /b %EXIT_CODE%
