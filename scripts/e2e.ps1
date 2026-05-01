#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
$RootDir = Split-Path -Parent (Split-Path -Parent $PSCommandPath)

# 1. Verify toolchain
Write-Host "=== 1/3 — Verify toolchain ==="
foreach ($cmd in @('go', 'cue')) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Write-Error "FATAL: $cmd not found in PATH"
        exit 1
    }
    $ver = & $cmd version 2>&1 | Select-Object -First 1
    Write-Host "  ${cmd}: $ver"
}

# 2. Go vet + test
Write-Host ""
Write-Host "=== 2/3 — Go vet & test (cli/) ==="
Push-Location (Join-Path $RootDir 'cli')
& go vet ./...;   if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& go test ./...;  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Get-ChildItem -Path (Join-Path $RootDir 'control-plane') -Filter 'go.mod' -Recurse | ForEach-Object {
    $dir = $_.DirectoryName
    Write-Host "  $(Split-Path -Leaf $dir)/"
    Push-Location $dir
    & go vet ./...;   if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & go test ./...;  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Pop-Location
}

# 3. CUE validation
Write-Host ""
Write-Host "=== 3/3 — CUE schema validation ==="
Push-Location $RootDir
$cueFiles = Get-ChildItem -Path 'specs' -Filter '*.cue' | ForEach-Object { $_.FullName }
& cue vet @cueFiles; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host ""
Write-Host "All checks passed."
exit 0
