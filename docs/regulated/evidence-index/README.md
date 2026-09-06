# Evidence Index

This folder is the governed index for objective evidence.

## Files

- `evidence-ledger.yaml` - GENERATED index of evidence categories and current gaps (NRT-033 #716). `status: effective` means the index is in force: computed from the tree by `scripts/evidence_ledger_guard.py` and checked in CI. It says nothing about the effectiveness of the indexed documents, whose `status:` markers are recounted under each category's `observed` block, never softened.
- `.regulated-evidence-pack/evidence-pack.json` - generated local hash inventory, uploaded by CI when the workflow runs.
- `.regulated-evidence-pack/github-qms-audit.json` - generated GitHub QMS controls audit, uploaded by CI when the workflow runs.
- `.regulated-doc-gate/reference-canon-report.json` - generated list of all Nomos bible references and their public/licensed processing status.
- `repeated-ci-evidence/` - VRC-14 (#560): versioned policy, dated index and measurement of the repeated CI evidence chain on the private corpus. Measured 2026-09-04: 4 of the 8 consecutive green scheduled runs the target requires, so the claim stays locked (`EV-CI-REPEAT-001` requires evidence, `GAP-REPEATED-CI-EVIDENCE` open).

## Rule

The evidence ledger records actual evidence only. Missing evidence is explicit and blocks claims according to risk.

## Recurring DevOps Action

The index goes stale after 90 days (portfolio freshness policy) and CI turns red. Regenerate it deliberately, review the diff of the `observed` blocks, and commit:

```bash
python3 scripts/evidence_ledger_guard.py --root . --write
python3 scripts/evidence_ledger_guard.py --root . --check
```

The declarations (`current_status`, `claim_allowed`, gaps) stay human-owned; the guard refuses a declaration the files contradict.

The Nomos/Praxis exchange fixture (`specs/examples/nomos-praxis-evidence.valid.yaml`) binds this ledger by hash: after a regeneration, `nomos evidence praxis-verify` refuses the fixture until its `NOMOS-EVIDENCE-LEDGER` hash is updated to the new file — that refusal is the contract working, and the update is part of this action.

## Automation

Generate the evidence locally:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
python3 scripts/repeated_ci_evidence.py --root .            # verify the published repeated-evidence index (offline)
python3 scripts/repeated_ci_evidence.py --root . --collect  # re-measure live from the Actions API
```

Use `--offline` for a repo-file-only audit. Live GitHub settings that cannot be read remain `requires_live_evidence`.
