# local use only

param(
    [switch]$Build
)

$ErrorActionPreference = "Stop"

Write-Host "`n====================================" -ForegroundColor Cyan
Write-Host " OBOX APP - WINDOWS BUILD SETUP"
Write-Host "====================================`n" -ForegroundColor Cyan

function Install-WingetPackage($Id) {
    Write-Host "`nInstalling $Id ..." -ForegroundColor Yellow

    winget install --id $Id --exact `
        --accept-package-agreements `
        --accept-source-agreements

    if ($LASTEXITCODE -ne 0) {
        throw "Installation failed: $Id"
    }
}

# 1. Check Winget
if (!(Get-Command winget -ErrorAction SilentlyContinue)) {
    throw "Winget is missing. Install Windows App Installer first."
}

# 2. Install development tools
Install-WingetPackage "GoLang.Go"
Install-WingetPackage "OpenJS.NodeJS.LTS"
Install-WingetPackage "Task.Task"
Install-WingetPackage "MSYS2.MSYS2"

# 3. Refresh environment PATH
$GoBin = "C:\Program Files\Go\bin"
$NodeBin = "C:\Program Files\nodejs"
$MsysBin = "C:\msys64\mingw64\bin"
$GoTools = "$env:USERPROFILE\go\bin"
$WingetLinks = "$env:LOCALAPPDATA\Microsoft\WinGet\Links"

$RequiredPaths = @(
    $GoBin,
    $NodeBin,
    $MsysBin,
    $GoTools,
    $WingetLinks
)

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
$PathList = @($UserPath -split ";" | Where-Object { $_ })

foreach ($PathItem in $RequiredPaths) {
    if (!(Test-Path $PathItem)) {
        continue
    }

    if ($PathList -notcontains $PathItem) {
        $PathList += $PathItem
    }
}

[Environment]::SetEnvironmentVariable(
    "Path",
    ($PathList -join ";"),
    "User"
)

$env:Path = ($RequiredPaths -join ";") + ";" + $env:Path

# 4. Install MSYS2 GCC and NSIS
$Bash = "C:\msys64\usr\bin\bash.exe"

if (!(Test-Path $Bash)) {
    throw "MSYS2 not found at C:\msys64"
}

Write-Host "`nUpdating MSYS2 packages..." -ForegroundColor Yellow

& $Bash -lc "pacman -Syu --noconfirm"

if ($LASTEXITCODE -ne 0) {
    throw @"
MSYS2 update did not finish.
Close this terminal, reopen PowerShell and rerun this script.
"@
}

Write-Host "`nInstalling GCC and NSIS..." -ForegroundColor Yellow

& $Bash -lc "pacman -S --needed --noconfirm mingw-w64-x86_64-gcc mingw-w64-x86_64-nsis"

if ($LASTEXITCODE -ne 0) {
    throw "Could not install MSYS2 GCC/NSIS. Check pacman output."
}

# 5. Refresh PATH for compiler
$env:Path = "$MsysBin;$GoTools;$GoBin;$NodeBin;$WingetLinks;$env:Path"

# 6. Install Wails 3 CLI
Write-Host "`nInstalling Wails 3 CLI..." -ForegroundColor Yellow

go install github.com/wailsapp/wails/v3/cmd/wails3@latest

if ($LASTEXITCODE -ne 0) {
    throw "Wails 3 installation failed."
}

# 7. Enable CGO
$env:CGO_ENABLED = "1"
$env:CC = "gcc"

# 8. Verify tools
$Tools = @(
    "go",
    "node",
    "npm",
    "task",
    "gcc",
    "makensis",
    "wails3"
)

Write-Host "`n====================================" -ForegroundColor Cyan
Write-Host " TOOL VERIFICATION"
Write-Host "====================================" -ForegroundColor Cyan

foreach ($Tool in $Tools) {
    $Found = Get-Command $Tool -ErrorAction SilentlyContinue

    if ($Found) {
        Write-Host "[OK] $Tool : $($Found.Source)" -ForegroundColor Green
    }
    else {
        Write-Host "[MISSING] $Tool" -ForegroundColor Red
    }
}

# 9. Check compiler architecture
Write-Host "`nGCC target:"
gcc -dumpmachine

# 10. Build
if ($Build) {
    Write-Host "`nStarting Windows build..." -ForegroundColor Cyan

    if (!(Get-Command task -ErrorAction SilentlyContinue)) {
        throw "Task command not found."
    }

    $TaskFiles = @(
        "Taskfile.yml",
        "Taskfile.yaml",
        "taskfile.yml",
        "taskfile.yaml"
    )

    $TaskFile = $TaskFiles |
        Where-Object { Test-Path $_ } |
        Select-Object -First 1

    if (!$TaskFile) {
        throw "Taskfile not found in current directory."
    }

    $TaskContent = Get-Content $TaskFile -Raw

    if ($TaskContent -match 'INFO_PRODUCTVERSION=dev') {
        throw @"
Your NSIS installer has an invalid file version.
Change INFO_PRODUCTVERSION=dev to INFO_PRODUCTVERSION=1.0.0.0
in $TaskFile, then rerun with -Build.
"@
    }

    task build:windows

    if ($LASTEXITCODE -ne 0) {
        throw "Windows build failed."
    }

    Write-Host "`nBUILD SUCCESSFUL!" -ForegroundColor Green
}

Write-Host "`n====================================" -ForegroundColor Green
Write-Host " SETUP COMPLETED"
Write-Host "====================================" -ForegroundColor Green

Write-Host "`nTo build later, reopen PowerShell and run:"
Write-Host ".\setup-local-build.ps1 -Build" -ForegroundColor Yellow

