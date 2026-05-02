# Evidence Index

This folder is the governed index for objective evidence.

## Files

- `evidence-ledger.yaml` - initial ledger of evidence categories and current gaps.
- `.regulated-evidence-pack/evidence-pack.json` - generated local hash inventory, uploaded by CI when the workflow runs.
- `.regulated-evidence-pack/github-qms-audit.json` - generated GitHub QMS controls audit, uploaded by CI when the workflow runs.
- `.regulated-doc-gate/reference-canon-report.json` - generated list of all Nomos bible references and their public/licensed processing status.

## Rule

The evidence ledger records actual evidence only. Missing evidence is explicit and blocks claims according to risk.

## Automation

Generate the evidence locally:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

Use `--offline` for a repo-file-only audit. Live GitHub settings that cannot be read remain `requires_live_evidence`.
