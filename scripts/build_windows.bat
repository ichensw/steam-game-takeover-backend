@echo off
setlocal

cd /d "%~dp0\.."

where go >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Go was not found in PATH.
  echo Install Go or add it to PATH, then run this script again.
  exit /b 1
)

if not exist "bin" mkdir "bin"

echo [INFO] Building steam-game-takeover-backend...
go build -o "bin\steam-game-takeover-backend.exe" ".\cmd\server"
if errorlevel 1 (
  echo [ERROR] Build failed.
  exit /b 1
)

echo [OK] Built: %CD%\bin\steam-game-takeover-backend.exe

endlocal
