# Windows Corpus Setup

## Build Profiles

Nomos CLI has two build profiles due to `go-tree-sitter` requiring CGO:

| Profile | CGO | C Compiler | Packages Available |
|---|---|---|---|
| **Corpus-only** | `CGO_ENABLED=0` | Not needed | corpus, checks, validate, strict, exceptions, productcheck, compliance |
| **Full** | `CGO_ENABLED=1` | Required (GCC) | All packages including detect, diagnose, report |

Most corpus workflows (scan, manifest, extract, checks) run without CGO.

## Quick Start (Corpus-Only)

```powershell
# 1. Install Go (if not present)
winget install GoLang.Go

# 2. Verify
go version

# 3. Build and test corpus commands (no CGO needed)
cd cli
$env:CGO_ENABLED = "0"
go test ./internal/corpus/...
go test ./internal/checks/...
go test ./internal/validate/...
go test ./internal/strict/...
go test ./internal/exceptions/...
go test ./internal/productcheck/...
```

## Automated Setup

Run the setup script from the repo root:

```powershell
# Verify environment
.\scripts\nomos-corpus-windows.ps1

# Verify + install missing dependencies
.\scripts\nomos-corpus-windows.ps1 -Install

# Verify full build (including tree-sitter)
.\scripts\nomos-corpus-windows.ps1 -BuildFull
```

## Full Build (with Tree-Sitter)

Tree-sitter provides AST parsing for detect/diagnose. It requires a C compiler.

### Option 1: MSYS2 (Recommended)

```powershell
# Install MSYS2 from https://www.msys2.org/
# Then in MSYS2 terminal:
pacman -S mingw-w64-x86_64-gcc

# Add to PATH (PowerShell)
$env:Path += ";C:\msys64\mingw64\bin"

# Enable CGO
go env -w CGO_ENABLED=1

# Build
cd cli
go build ./...
```

### Option 2: TDM-GCC

Download from https://jmeubank.github.io/tdm-gcc/ and install. GCC is added to PATH automatically.

```powershell
go env -w CGO_ENABLED=1
cd cli
go build ./...
```

### Option 3: Scoop

```powershell
scoop install gcc
go env -w CGO_ENABLED=1
cd cli
go build ./...
```

## CUE Schema Validation

CUE validates `specs/*.cue` schemas. It is required for schema contribution but not for corpus commands.

```powershell
# Install CUE
go install cuelang.org/go/cmd/cue@latest

# Validate schemas
cue vet specs/*.cue
```

## tree-sitter Grammars

tree-sitter grammars compile automatically with `go build` when CGO is enabled. No separate installation needed — the Go bindings (`github.com/smacker/go-tree-sitter`) bundle grammar C sources for:

- Go, Java, JavaScript, Python, TypeScript

## Package Dependency Map

```
CGO_ENABLED=0 (pure Go, no C compiler):
  checks/         sources, contracts, matrix
  validate/       schema validation
  strict/         cross-manifest checks
  exceptions/     expiring exceptions
  productcheck/   project manifest checks
  compliance/     claims governance
  attestation/    in-toto, SLSA, cosign
  output/         JSON, markdown output
  partial/        partial mode
  guard/          output + push guards
  corpus/         scan, manifest, extract, feed, governance, release-gate

CGO_ENABLED=1 (requires GCC, tree-sitter C bindings):
  detect/         tree-sitter AST parsing
  diagnose/       repository diagnosis (imports detect)
  report/         report generation (imports detect)
  export/         SPDX, CycloneDX (imports report -> detect)
  admit/          admission logic (imports detect)
  remediation/    gap remediation (imports detect)
  app/            CLI entrypoint (imports all)
```

## CI on Windows

For GitHub Actions on `windows-latest`:

```yaml
jobs:
  corpus-test-windows:
    runs-on: windows-latest
    env:
      CGO_ENABLED: "0"
    defaults:
      run:
        working-directory: cli
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: cli/go.mod
      - run: go test ./internal/corpus/...
      - run: go test ./internal/checks/...
      - run: go test ./internal/validate/...
      - run: go test ./internal/strict/...
```

For full build on Windows CI, add GCC via chocolatey:

```yaml
jobs:
  full-test-windows:
    runs-on: windows-latest
    env:
      CGO_ENABLED: "1"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: cli/go.mod
      - run: choco install mingw -y
      - run: go test ./...
        working-directory: cli
```

## Troubleshooting

| Error | Cause | Fix |
|---|---|---|
| `exec: "gcc": executable file not found` | CGO enabled but no C compiler | Set `CGO_ENABLED=0` for corpus-only, or install GCC |
| `cgo: C compiler "gcc" not found` | Same | Same |
| `undefined: sitter.NewParser` | Building detect without CGO | Use `CGO_ENABLED=1` + GCC, or skip detect/diagnose/report packages |
| `go-tree-sitter compilation errors` | GCC version mismatch | Update GCC or use TDM-GCC |
