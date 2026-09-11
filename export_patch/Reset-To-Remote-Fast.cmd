@echo off
setlocal enabledelayedexpansion
 
set "scriptName=%~nx0"

for /d %%d in (*) do (
    if /i not "%%d"==".git" if /i not "%%d"=="!scriptName!" (
        rd /s /q "%%d" 2>nul
    )
)
for %%f in (*) do (
    if /i not "%%f"==".git" if /i not "%%f"=="!scriptName!" (
        del /f /q "%%f" 2>nul
    )
)

if exist ".git\rebase-merge" (
    git rebase --abort >nul 2>&1
)
if exist ".git\rebase-apply" (
    git rebase --abort >nul 2>&1
)

git checkout main >nul 2>&1 || git checkout -b main origin/main >nul 2>&1

git fetch origin main --unshallow >nul 2>&1
git reset --hard origin/main >nul 2>&1

pause