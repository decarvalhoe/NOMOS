#Requires -Version 5.1
<#
.SYNOPSIS
    Verify and configure Windows environment for Nomos corpus commands.

.DESCRIPTION
    Nomos CLI has two build profiles:
    - Full build: includes tree-sitter (requires CGO + C compiler)
    - Corpus-only build: scan, manifest, extract (pure Go, no CGO)

    This script verifies the environment for corpus-only operations
    and optionally installs missing dependencies.

.PARAMETER Install
    If set, installs missing dependencies via winget/scoop.

.PARAMETER BuildFull
    If set, verifies full build requirements including CGO.

.EXAMPLE
    .\nomos-corpus-windows.ps1
    .\nomos-corpus-windows.ps1 -Install
    .\nomos-corpus-windows.ps1 -BuildFull
#>

param(
    [switch]$Install,
    [switch]$BuildFull
)

$ErrorActionPreference = "Stop"

function Write-Check {
    param([string]$Name, [bool]$Ok, [string]$Detail)
    if ($Ok) {
        Write-Host "  [PASS] $Name" -ForegroundColor Green
        if ($Detail) { Write-Host "         $Detail" -ForegroundColor DarkGray }
    } else {
        Write-Host "  [FAIL] $Name" -ForegroundColor Red
        if ($Detail) { Write-Host "         $Detail" -ForegroundColor Yellow }
    }
    return $Ok
}

function Write-Section {
    param([string]$Title)
    Write-Host ""
    Write-Host "=== $Title ===" -ForegroundColor Cyan
}

# ---------- Go ----------

Write-Section "Go Runtime"

$goOk = $false
$goVersion = ""
try {
    $goVersion = (go version 2>$null) -replace "go version ", ""
    $goOk = $true
} catch {}

Write-Check "Go installed" $goOk $goVersion

if (-not $goOk -and $Install) {
    Write-Host "  Installing Go via winget..." -ForegroundColor Yellow
    winget install GoLang.Go --accept-package-agreements --accept-source-agreements
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [System.Environment]::GetEnvironmentVariable("Path", "User")
    try {
        $goVersion = (go version 2>$null) -replace "go version ", ""
        $goOk = $true
        Write-Host "  Go installed: $goVersion" -ForegroundColor Green
    } catch {
        Write-Host "  Go installation failed. Install manually from https://go.dev/dl/" -ForegroundColor Red
    }
}

# ---------- CGO / C Compiler (full build only) ----------

if ($BuildFull) {
    Write-Section "CGO Requirements (full build)"

    $cgoEnv = go env CGO_ENABLED 2>$null
    $cgoEnabled = $cgoEnv -eq "1"
    Write-Check "CGO_ENABLED=1" $cgoEnabled "Current: CGO_ENABLED=$cgoEnv"

    $gccOk = $false
    $gccVersion = ""
    try {
        $gccVersion = (gcc --version 2>$null | Select-Object -First 1)
        $gccOk = $true
    } catch {}
    Write-Check "GCC / C compiler" $gccOk $gccVersion

    if (-not $gccOk) {
        Write-Host ""
        Write-Host "  Tree-sitter requires a C compiler. Options:" -ForegroundColor Yellow
        Write-Host "    1. Install MSYS2: https://www.msys2.org/" -ForegroundColor Yellow
        Write-Host "       Then: pacman -S mingw-w64-x86_64-gcc" -ForegroundColor Yellow
        Write-Host "    2. Install TDM-GCC: https://jmeubank.github.io/tdm-gcc/" -ForegroundColor Yellow
        Write-Host "    3. Use scoop: scoop install gcc" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "  After installing, ensure gcc is in PATH and run:" -ForegroundColor Yellow
        Write-Host "    go env -w CGO_ENABLED=1" -ForegroundColor Yellow
    }

    if (-not $gccOk -and $Install) {
        Write-Host "  Attempting scoop install gcc..." -ForegroundColor Yellow
        try {
            scoop install gcc 2>$null
            $gccVersion = (gcc --version 2>$null | Select-Object -First 1)
            $gccOk = $true
            Write-Host "  GCC installed: $gccVersion" -ForegroundColor Green
        } catch {
            Write-Host "  scoop not available. Install GCC manually." -ForegroundColor Red
        }
    }
} else {
    Write-Section "CGO Status (informational)"

    $cgoEnv = go env CGO_ENABLED 2>$null
    Write-Host "  CGO_ENABLED=$cgoEnv (not required for corpus commands)" -ForegroundColor DarkGray
    Write-Host "  Corpus commands (scan, manifest, extract) are pure Go." -ForegroundColor DarkGray
}

# ---------- Corpus build test ----------

Write-Section "Corpus Build Verification"

$cliRoot = Join-Path $PSScriptRoot ".." "cli"
if (-not (Test-Path (Join-Path $cliRoot "go.mod"))) {
    $cliRoot = Join-Path (Get-Location) "cli"
}

if (Test-Path (Join-Path $cliRoot "go.mod")) {
    Write-Host "  Building corpus package (CGO_ENABLED=0)..." -ForegroundColor DarkGray

    $env:CGO_ENABLED = "0"
    $buildResult = & go build "$cliRoot/internal/corpus" 2>&1
    $buildOk = $LASTEXITCODE -eq 0
    Write-Check "corpus package builds (no CGO)" $buildOk ($buildResult | Out-String).Trim()

    if ($buildOk) {
        Write-Host "  Running corpus tests..." -ForegroundColor DarkGray
        Push-Location $cliRoot
        $testResult = & go test ./internal/corpus/... 2>&1
        $testOk = $LASTEXITCODE -eq 0
        Pop-Location
        Write-Check "corpus tests pass" $testOk ($testResult | Select-Object -Last 1)
    }

    if ($BuildFull) {
        $env:CGO_ENABLED = "1"
        Write-Host "  Building full CLI (CGO_ENABLED=1)..." -ForegroundColor DarkGray
        Push-Location $cliRoot
        $fullResult = & go build ./... 2>&1
        $fullOk = $LASTEXITCODE -eq 0
        Pop-Location
        Write-Check "full CLI builds (with tree-sitter)" $fullOk ($fullResult | Out-String).Trim()
    }

    # Restore
    Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
} else {
    Write-Host "  cli/go.mod not found — run from repo root" -ForegroundColor Red
}

# ---------- Summary ----------

Write-Section "Summary"

if ($BuildFull) {
    Write-Host "  Full build mode: requires Go + GCC + CGO_ENABLED=1" -ForegroundColor White
    Write-Host "  tree-sitter grammars: Go, Java, JavaScript, Python, TypeScript" -ForegroundColor DarkGray
} else {
    Write-Host "  Corpus-only mode: requires Go only (no CGO, no C compiler)" -ForegroundColor White
    Write-Host "  Available commands: corpus scan, manifest generate, parcours extract" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "  Build split:" -ForegroundColor White
Write-Host "    CGO_ENABLED=0  ->  corpus, checks, validate, strict, exceptions" -ForegroundColor DarkGray
Write-Host "    CGO_ENABLED=1  ->  detect (tree-sitter), diagnose, report" -ForegroundColor DarkGray
Write-Host ""
