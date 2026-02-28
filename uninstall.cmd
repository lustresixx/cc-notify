@echo off
setlocal
set ROOT_DIR=%~dp0

if not exist "%ROOT_DIR%scripts\uninstall.cmd" (
  echo scripts\uninstall.cmd not found.
  exit /b 1
)

call "%ROOT_DIR%scripts\uninstall.cmd"
exit /b %ERRORLEVEL%
