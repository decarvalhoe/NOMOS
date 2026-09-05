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
| `public_surrogate_annex` | Temporary bridge for a required licensed bible when authoritative public regulations, regulator guidance, or agency standards cover enough adjacent process scope to keep Nomos work moving. | Process only the public sources named in `public-surrogate-annexes/`. Preserve the licensed gap and block all protected-standard clause, certification, equivalence, and redistribution claims. |

## Machine-Readable Reference Classification

Each registered bible may declare `reference_classification`. If the block is absent, `regulated_reference_canon.py` infers the class from `content_access_policy`, publisher and evidence status. The normalized report always emits the effective classification.

```yaml
reference_classification:
  source_class: public | licensed | private | confidential | customer_owned
  confidentiality: public | licensed_restricted | private_restricted | confidential_restricted | customer_confidential
  full_text_redistribution: allowed | source_terms_only | forbidden
  processing_mode: official_snapshot | licensed_read_only_local_artifact | private_read_only_local_artifact | confidential_read_only_local_artifact | customer_read_only_local_artifact
  retention_obligation: public_snapshot_retained_with_hash | license_terms | owner_policy | confidentiality_agreement | customer_contract
```

Class rules:

| Source class | Access rule | Retention rule | Redistribution rule |
|---|---|---|---|
| `public` | Official public source may be snapshotted and hashed. | Keep public snapshot metadata, hash and retrieval evidence. | Full text redistribution is only `allowed` when the source terms permit it; otherwise use `source_terms_only`. |
| `licensed` | Full text must live outside Git under `NOMOS_LICENSED_REFERENCE_ROOT` and be processed read-only. | Retain according to license terms and intake sidecar. | Full text redistribution and committed full text are forbidden by the gate. |
| `private` | Source must be supplied as a controlled local artifact. | Retain according to owner policy. | Full text redistribution and committed full text are forbidden by the gate. |
| `confidential` | Source must be supplied as a confidential controlled local artifact. | Retain according to confidentiality agreement. | Full text redistribution and committed full text are forbidden by the gate. |
| `customer_owned` | Source must be supplied as a customer-controlled local artifact. | Retain according to customer contract. | Full text redistribution and committed full text are forbidden by the gate. |

## Public Surrogate Annexes

A public surrogate annex is a legal temporary bypass, not a license bypass.

Nomos may use one only when all conditions are true:

- the required bible is registered and still requires licensed intake;
- the annex is stored under `docs/regulated/reference-basis/public-surrogate-annexes/`;
- the annex names official or authoritative public sources;
- the annex declares a claim boundary and blocked claims;
- the command is run with `--allow-public-surrogates`;
- the generated report status is `surrogate_ready_for_processing`, not `ready_for_processing`.

Surrogate processing may generate manifests, hashes, topic indexes, public-source chunks, traceability to public sources, and explicit gap reports to the missing licensed intake. It may not generate ISO/IEC/IEEE/ISPE clause-level coverage, certification evidence, hidden equivalence claims, or reconstructed protected text.

Current temporary annexes:

- `ISO-13485-2016` uses FDA QMSR, the FDA final rule, eCFR 21 CFR Part 820, and ANSI IBR access metadata until ISO 13485:2016 is acquired.
- `ISO-IEC-IEEE-12207-2026` uses NASA software engineering requirements/handbook material plus NIST SSDF and systems security engineering guidance until ISO/IEC/IEEE 12207:2026 is acquired.

## Licensed Reference Intake

Before Nomos processes a licensed bible:

1. Create a `templates/regulated/licensed-reference-intake.yaml` record.
2. Record reference ID, official URL, edition/date, license holder, allowed use, storage location, source hash and reviewer.
3. Store the source in `NOMOS_LICENSED_REFERENCE_ROOT/<reference-id>/`.
4. Run Nomos corpus scan/feed in read-only mode against that local artifact.
5. Commit only permitted sidecars, manifests, hashes, coverage, traceability and validation evidence.
6. Do not commit full text or substantial extracted chunks. For `licensed`, `private`, `confidential`, and `customer_owned` classifications, the reference canon gate fails if an intake sidecar authorizes full-text redistribution or committed full text.
7. **Artifact integrity is not permission (#641).** A matching `source_integrity.sha256` proves what is on disk; it says nothing about whether NOMOS may process it. The gate keeps the two facts apart — `artifact_hash_verified` and `license_use_approved` — and reports `verified_license_review_required` with `GAP-LICENSE-REVIEW` / `GAP-LICENSE-USE` / `GAP-LICENSE-SAFETY` until an **assigned** reviewer records an approved status, `allowed_use.internal_processing_by_nomos` authorises processing, and `commit_full_text_to_git` and `customer_redistribution` are explicitly `false`. A draft or reviewer-less sidecar is an honest blocked state: green without any claim, red only under `--strict`.
8. **Protected text actually present (#641).** `no-full-text-policy.yaml` drives a scan of the public trees (and, with `--staged`, of anything staged for commit): a file whose hash equals a registered licensed artifact is the artifact committed (`LICENSED_FULL_TEXT_COMMITTED`); a normalised sentence whose digest matches a registered sentinel is copied text (`LICENSED_TEXT_PRESENT`). Both are errors. The policy stores digests only, never the sentences; an empty sentinel list is reported as **uncovered**, never as clean, because nobody with the licensed copy has registered any yet.

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

Temporary surrogate processing is explicit:

```bash
python scripts/regulated_reference_canon.py --allow-public-surrogates --report .regulated-doc-gate/reference-canon-report.json
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

Both missing bibles have temporary public surrogate annexes. The surrogate annexes keep public-source processing available until acquisition, but do not close the licensed-intake gaps.
