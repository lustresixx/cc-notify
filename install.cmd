@echo off
setlocal
set ROOT_DIR=%~dp0

if not exist "%ROOT_DIR%scripts\install.cmd" (
  echo scripts\install.cmd not found.
  exit /b 1
)

call "%ROOT_DIR%scripts\install.cmd"
exit /b %ERRORLEVEL%
