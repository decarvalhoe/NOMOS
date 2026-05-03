# Nomos Coverage Report

Generated: 2026-05-03
Release context: `v0.1.0-ALPHA`

## Command

```powershell
cd C:\Dev\nomos-main-verify-20260503-110433\cli
$env:PATH='C:\Dev\canonical-first-method\.tools\go\bin;' + $env:PATH
$env:GOCACHE='C:\Dev\canonical-first-method\.tools\go-cache'
$env:GOMODCACHE='C:\Dev\canonical-first-method\.tools\go-mod-cache'
$env:GOPATH='C:\Dev\canonical-first-method\.tools\gopath'
C:\Dev\canonical-first-method\.tools\go\bin\go.exe test ./... -coverprofile ..\reports\cli-coverage.out
C:\Dev\canonical-first-method\.tools\go\bin\go.exe tool cover -func ..\reports\cli-coverage.out
```

## Result

Overall CLI statement coverage:

```text
total: (statements) 87.2%
```

The coverage profile is generated locally and is not committed as release evidence. This Markdown report records the release-preparation result without storing the raw `.out` profile in Git.

## Notes

- Coverage is a quality signal, not a regulated validation claim.
- Coverage does not prove semantic fidelity of a corpus.
- Regulated qualification still requires intended-use validation, traceability, challenge cases, approvals, retention, and independent reconstruction evidence.
