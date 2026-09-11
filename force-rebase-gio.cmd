@echo off
setlocal

rem ============================================
rem  强制将 gio 子仓库对齐远程最新 (origin/main)
rem  步骤: fetch -> 创建时间戳备份分支 -> reset --hard origin/main
rem ============================================

set "REPO=%~dp0gio"

if not exist "%REPO%\.git" (
    echo [ERROR] git repo not found: %REPO%
    exit /b 1
)

pushd "%REPO%"

echo [*] Fetching origin...
git fetch origin
if errorlevel 1 goto :fail

rem 时间戳备份分支, 重复执行不会重名
for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "TS=%%i"
set "BACKUP=backup/pre-force-rebase-%TS%"

echo [*] Creating backup branch: %BACKUP%
git branch "%BACKUP%"
if errorlevel 1 goto :fail

echo [*] Hard resetting to origin/main...
git reset --hard origin/main
if errorlevel 1 goto :fail

echo.
echo [OK] main is now at:
git log --oneline -3
echo.
echo [OK] Backup branch: %BACKUP%

popd
exit /b 0

:fail
echo.
echo [ERROR] command failed
popd
exit /b 1
