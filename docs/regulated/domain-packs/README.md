# Domain Pack Packaging And Customer Install Guides

This folder is the canonical home for **domain pack** packaging — the
customer-facing install material for the domain-specific evidence packs
defined in the domain-opportunity roadmap
(`docs/38-domain-opportunity-roadmap.md`, DOR-021).

A **domain pack** is a scoped, non-certified package of references,
workflow configuration, validation checklist, expected outputs, gate
configuration, and claim boundary that a customer can install on top of
the Nomos baseline to make evidence specific to one regulated or
quasi-regulated context (for example, AI/RAG governance, GxP/CSV
controls, medical/SaMD evidence, finance/RegTech, legal/eDiscovery, cyber
supplier assurance).

This page defines:

- what every domain pack must ship before it is offered to a customer;
- where each artefact lives;
- the boundary between what Nomos provides and what the customer remains
  accountable for.

It does not define the domain profile schema (DOR-001), the claim ladder
(DOR-002), or any specific domain's controls. Those are owned by their
respective DOR issues. This page is the install/packaging contract that
every pack adopts once the underlying schema is ready.

## Required Pack Contents

A domain pack is "install-guide-ready" when its folder under
`docs/regulated/domain-packs/<pack-id>/` contains every item below. Each
item has a reusable template under
`docs/regulated/domain-packs/templates/`. Packs may add domain-specific
material; they may not omit any of the items below.

| Artefact | Purpose | Template |
| --- | --- | --- |
| `install-guide.md` | Step-by-step customer install: prerequisites, references, workflow config, expected outputs, gates, smoke verification. | [`templates/install-guide-template.md`](templates/install-guide-template.md) |
| `validation-checklist.md` | Per-install acceptance checklist customers complete before claiming the pack is operational in their environment. | [`templates/validation-checklist-template.md`](templates/validation-checklist-template.md) |
| `claim-boundary.md` | Pack-specific public claim boundary — what the pack supports claiming, what it does not, and which prohibited claims still apply. | [`templates/claim-boundary-template.md`](templates/claim-boundary-template.md) |

The templates are README-exempt informational documents under
`docs/regulated/domain-packs/`; they intentionally live outside the
controlled-document roots policed by `scripts/regulated_docs_gate.py`
(`docs/regulated/{quality-system,lifecycle,data-integrity,security-privacy,github-operating-model}/`).
A pack that needs controlled-document records (for example, an approved
SOP) must place those under the appropriate controlled root with the
required markers, not under `domain-packs/`.

## Required Inputs From The Customer

Every install guide enumerates what the customer must own before the
pack can be considered operational:

- **Required references.** The licensed standards, regulatory
  guidance, internal SOPs, and supplier records the pack consumes. The
  pack lists references by canonical identifier; the customer is
  responsible for licensed-reference acquisition (see "Licensed
  reference policy" below).
- **Workflow configuration.** Provider, branch protection, approval
  reviewers, secrets, and runners that the pack's GitHub workflows or
  scripts expect.
- **Acceptance evidence.** A signed customer-integration checklist
  (template at
  [`templates/regulated/customer-integration-checklist.md`](../../../templates/regulated/customer-integration-checklist.md))
  recording who is accountable for the regulated impact, electronic
  record scope, electronic signature scope, AI/RAG scope, data
  retention, audit-trail review, and supplier qualification in the
  customer's deployment.

The pack itself ships only the structure, automation, and
documentation; the customer remains accountable for content correctness,
applicability, and approval in their context.

## Licensed Reference Policy

Domain packs may not redistribute licensed standards. The install guide
records the canonical identifier (for example,
`ICH Q9(R1)`, `21 CFR Part 11`, `IEC 62304:2006+AMD1:2015`) and instructs
the customer to acquire a licensed copy through the appropriate
publisher. The customer points the pack's reference loader at their
local copy through configuration (`references/<pack-id>.yaml` or
equivalent) so Nomos can hash and cite the customer-supplied artefact
without copying it. See
[`templates/regulated/licensed-reference-intake.yaml`](../../../templates/regulated/licensed-reference-intake.yaml).

## Workflow Config

Each pack documents the GitHub workflow that exercises the pack's gates
in CI:

- the workflow files the pack adds (under `.github/workflows/`);
- which jobs the customer must adopt as required-status checks;
- which secrets / variables / runners the workflow reads;
- the expected exit codes and report artefacts;
- the path the customer's CODEOWNERS file should claim for the pack's
  evidence directory so approvals run through the documented reviewers.

## Expected Outputs And Gates

Every pack's install guide names the artefacts the customer should
expect to see after a green run. Typical examples include a control
matrix, an evidence index, a release evidence bundle reference, an
ALCOA+ envelope, and a `regulated-doc-gate-report.json`. The validation
checklist references those artefact paths so the customer can audit the
install without re-running everything from scratch.

## Claim Boundary

Domain packs make Nomos's regulated-readiness usable in a customer's
context. They do **not** convert Nomos into a certified, validated, or
regulator-approved system in that context. Every pack ships its own
`claim-boundary.md` that:

- restates the project-wide [public claim boundary](../../public-claim-boundary.md);
- adds the pack-specific allowed claims (always conditional on
  customer-owned validation);
- calls out the pack-specific prohibited claims (for example, an AI/RAG
  pack must explicitly refuse Part 11 e-signature platform claims,
  certified-AI claims, and bias-free-AI claims).

Closing DOR-021 or installing any domain pack does **not** authorise a
claim of certification, legal compliance, regulated validation, Part 11
compliance, GxP compliance, medical-device compliance, financial
regulatory compliance, or legal sufficiency. Any public claim must
remain bounded by [`docs/public-claim-boundary.md`](../../public-claim-boundary.md)
and the pack's own `claim-boundary.md`.

## Currently Packaged Domains

| Pack | Folder | Underlying DOR issue | Status |
| --- | --- | --- | --- |
| AI/RAG governance | [`ai-rag/`](ai-rag/) | [DOR-010](https://github.com/RBOKproject/NOMOS/issues/422) | first instantiated example for DOR-021 |

Additional packs (GxP/CSV — DOR-005, medical/SaMD — DOR-009, finance/RegTech
— DOR-013, legal/eDiscovery — DOR-014, cyber supplier assurance — DOR-018,
high-assurance engineering — DOR-019, AI governance review board — DOR-011)
adopt the same template once their underlying DOR issues land.

## Verification

The packaging gate that DOR-021's "Verification" step references is the
existing regulated documentation gate:

```bash
python scripts/regulated_docs_gate.py \
  --report .regulated-doc-gate/regulated-doc-gate-report.json
```

The gate validates that this folder's YAML is well-formed and free of
overclaiming patterns. It does not certify that any individual pack's
content is approved by an accountable human; that remains the
customer's responsibility per the boundary above.
