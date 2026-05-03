# Contributing To Nomos

Nomos is a canonical-first product intelligence project. Contributions must preserve the central rule: product claims, runtime artifacts, and generated evidence must stay traceable to governed sources.

## Contribution Rules

- Use issues for meaningful changes before implementation when the scope affects product claims, evidence, public docs, or regulated-readiness posture.
- Use pull requests for all changes to protected branches.
- Keep changes scoped and explain the evidence they affect.
- Do not add sample, mock, or invented business data to product surfaces without marking it as test-only.
- Do not claim regulated compliance unless the repository contains the evidence required for that claim.
- Update public documentation when behavior, gates, or claim boundaries change.

## Required Checks

Run the relevant local checks before opening a PR:

```bash
cd cli
go test ./...
```

For release or gate changes, also run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
```

For Python automation and regulated evidence scripts:

```bash
python -m unittest discover -s tests -v
```

## Regulated-Readiness Contributions

Changes touching `docs/regulated/`, `templates/regulated/`, evidence schemas, validation records, or compliance scripts must:

- state the intended use;
- state the claim boundary;
- link to evidence or create an explicit gap;
- avoid copying licensed standards into the repository unless license terms allow it;
- preserve ALCOA+ expectations for generated evidence where applicable.

## Commit Style

Use concise conventional commits:

- `feat:`
- `fix:`
- `docs:`
- `test:`
- `refactor:`
- `chore:`

## Review Standard

Reviewers should prioritize:

- hidden overclaiming;
- loss of source traceability;
- gates that pass without real evidence;
- mutation of source corpora;
- missing tests for evidence-affecting behavior;
- documentation that implies a higher assurance level than the current proof supports.
