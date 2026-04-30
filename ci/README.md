# Nomos CI Integration

Reusable CI jobs for running `nomos strict` as a gate in GitHub Actions and GitLab CI.

## GitHub Actions

```yaml
# .github/workflows/nomos.yml
name: Nomos Gate
on:
  pull_request:
  push:
    branches: [main]

jobs:
  nomos:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: ./ci/github
        with:
          fail-mode: closed       # "closed" (default) or "open"
          annotate-pr: "true"     # Post findings as PR annotations
          write-report: nomos-report.json
```

### Inputs

| Input | Default | Description |
|---|---|---|
| `nomos-version` | `latest` | Version of nomos CLI to install |
| `project-path` | `.` | Path to project root |
| `fail-mode` | `closed` | `closed` fails the workflow, `open` warns only |
| `write-report` | `nomos-report.json` | Report output path (empty to disable) |
| `annotate-pr` | `true` | Post findings as PR review annotations |
| `extra-args` | | Additional args for `nomos strict` |

### Outputs

| Output | Description |
|---|---|
| `verdict` | `pass` or `fail` |
| `report-path` | Path to the generated report |

## GitLab CI

```yaml
# .gitlab-ci.yml
include:
  - project: 'nomos/ci-templates'
    ref: main
    file: '/ci/gitlab/nomos-strict.gitlab-ci.yml'

nomos-gate:
  extends: .nomos-strict
  variables:
    NOMOS_FAIL_MODE: "closed"
```

### Variables

| Variable | Default | Description |
|---|---|---|
| `NOMOS_VERSION` | `latest` | Version of nomos CLI to install |
| `NOMOS_PROJECT_PATH` | `.` | Path to project root |
| `NOMOS_FAIL_MODE` | `closed` | `closed` fails pipeline, `open` warns only |
| `NOMOS_WRITE_REPORT` | `nomos-report.json` | Report output path |
| `NOMOS_EXTRA_ARGS` | | Additional args for `nomos strict` |

### MR Annotations

Set `GITLAB_TOKEN` as a CI/CD variable with API access to enable merge request annotations.
The `annotate-mr.py` script posts findings as a discussion note on the MR.

## Fail-closed vs Fail-open

- **closed** (default): The pipeline/workflow fails when `nomos strict` finds blocking issues. This is the recommended mode for production gates.
- **open**: The pipeline/workflow succeeds with warnings. Use during initial adoption.
