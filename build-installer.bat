@echo off
setlocal

echo ========================================
echo  Deckstr Production Build
echo ========================================
echo.

:: Configuration - UPDATE THESE FOR YOUR REPO
set VERSION=1.0.0
set GITHUB_OWNER=ajanitshimanga
set GITHUB_REPO=OpenSmurfManager

:: Allow override via command line: build-installer.bat 1.2.0 myusername
if not "%~1"=="" set VERSION=%~1
if not "%~2"=="" set GITHUB_OWNER=%~2

echo Version: %VERSION%
echo GitHub: %GITHUB_OWNER%/%GITHUB_REPO%
echo.

:: Step 1: Build the Wails application with version info
echo [1/2] Building application...
set LDFLAGS=-X 'OpenSmurfManager/internal/updater.Version=%VERSION%' -X 'OpenSmurfManager/internal/updater.GitHubOwner=%GITHUB_OWNER%' -X 'OpenSmurfManager/internal/updater.GitHubRepo=%GITHUB_REPO%'
call wails build -clean -ldflags "%LDFLAGS%"
if errorlevel 1 (
    echo ERROR: Wails build failed!
    pause
    exit /b 1
)
echo Build complete.
echo.

:: Step 2: Build the installer with Inno Setup
echo [2/2] Building installer...

:: Try common Inno Setup locations
set ISCC=""
if exist "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" (
    set ISCC="C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
)
if exist "C:\Program Files\Inno Setup 6\ISCC.exe" (
    set ISCC="C:\Program Files\Inno Setup 6\ISCC.exe"
)

if %ISCC%=="" (
    echo ERROR: Inno Setup not found!
    echo Please install Inno Setup 6 from: https://jrsoftware.org/isdl.php
    pause
    exit /b 1
)

%ISCC% installer\setup.iss
if errorlevel 1 (
    echo ERROR: Installer build failed!
    pause
    exit /b 1
)

echo.
echo ========================================
echo  Build Complete!
echo ========================================
echo.
echo Installer: build\installer\Deckstr-Setup-1.0.0.exe
echo.
pause
