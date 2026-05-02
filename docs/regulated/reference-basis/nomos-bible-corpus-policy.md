# Nomos Bible Corpus Policy

document_id: NOMOS-REF-POL-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Define how external regulatory, standards, and industry good-practice references become the canonical bibles of Nomos.

The policy is strict:

```text
Every entry in external-reference-register.yaml is a Nomos bible.
If the full source cannot be legally obtained, the gap remains visible.
Nomos must not invent clauses, approvals, mappings, or compliance conclusions.
```

## Canon Rule

All references in `external-reference-register.yaml` are canonical prerequisites for regulated claims. Nomos may not claim regulated-grade CSV/CSA, GxP, data-integrity, e-record/e-signature, secure-SDLC, or high-assurance status unless the relevant bible is either:

- processed from an official public source snapshot;
- processed from a licensed local artifact;
- explicitly marked not applicable with rationale and approval;
- blocked with `requires_evidence`.

## GAMP 5 Rule

ISPE GAMP 5 Second Edition is a required Nomos bible for CSV/CSA and GxP computerized-system validation.

Current source status:

- official public metadata page: `https://ispe.org/publications/guidance-documents/gamp-5`;
- official public table-of-contents PDF: `https://ispe.org/sites/default/files/publications/guidance-documents/2022-TOC/ISPE-GAMP5-Ed2_TOC.pdf`;
- full guide: licensed ISPE digital publication, not a free public corpus;
- Nomos status: `licensed_reference_required_before_clause_mapping`.

Nomos may fetch or snapshot public metadata and the public TOC. Nomos must not fetch, scrape, commit, redistribute, or reconstruct the full guide unless a licensed copy is provided under terms that allow the intended internal processing.

## Processing Modes

| Mode | Use | Rule |
|---|---|---|
| `official_snapshot_allowed_with_hash` | Public official regulations, guidance pages, open standards pages, GitHub documentation. | Snapshot from official URL, hash the content, record retrieval time and tool, then atomize. |
| `licensed_local_artifact_required` | GAMP 5, ISO standards, paid industry guides, restricted customer standards. | Keep source outside Git, create a sidecar intake record, hash the artifact, process read-only, and commit only allowed manifests/evidence. |
| `metadata_only_until_licensed` | Public product page or table of contents exists but full text is restricted. | Register metadata and open a gap. No clause-level mapping is allowed. |

## Licensed Reference Intake

Before Nomos processes a licensed bible:

1. Create a `templates/regulated/licensed-reference-intake.yaml` record.
2. Record reference ID, official URL, edition/date, license holder, allowed use, storage location, source hash and reviewer.
3. Store the source in `NOMOS_LICENSED_REFERENCE_ROOT/<reference-id>/`.
4. Run Nomos corpus scan/feed in read-only mode against that local artifact.
5. Commit only permitted sidecars, manifests, hashes, coverage, traceability and validation evidence.
6. Do not commit full text or substantial extracted chunks unless the license explicitly permits it.

## Self-Processing Requirement

Nomos must process its own bibles with the same corpus rules it applies to customer sources:

```text
reference register
  -> bible corpus manifest
  -> source snapshot or licensed intake
  -> read-only scan
  -> atomization
  -> traceability matrix
  -> validation evidence
  -> release claim boundary
```

The canonical report is generated with:

```bash
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
```

The report may be `requires_evidence` while licensed bibles are absent. That status blocks clause-level claims; it does not block registering the bible itself.

## Current Local Intake Status

As of 2026-05-02, local sidecars exist for:

- `ISPE-GAMP5-2E-2022`
- `ISO-IEC-25010-2023`

Both records are still `license_review_required`; they prove artifact presence and integrity, not permission to redistribute or claim clause-level validation.

Missing licensed bibles still expected by the register:

- `ISO-13485-2016`
- `ISO-IEC-IEEE-12207-2026`
