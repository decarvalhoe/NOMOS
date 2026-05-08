# Install Guide Template — `<pack-id>` Domain Pack

> Replace every `replace_with_*` token before publishing. Keep section
> headings stable so customers and reviewers can diff between packs.

## 0. Pack Identity

- **Pack id:** replace_with_pack_id
- **Underlying DOR issue:** replace_with_dor_issue_url
- **Pack version:** replace_with_pack_version
- **Status:** alpha / beta / general-availability — replace_with_status
- **Repository path:** `docs/regulated/domain-packs/<pack-id>/`
- **Maintainer:** replace_with_maintainer

## 1. Intended Use And Out-Of-Scope

- **Intended use.** replace_with_paragraph describing the regulated or
  quasi-regulated context the pack targets, the kinds of evidence it
  helps a customer assemble, and the deployment shape (CLI, GitHub
  workflow, on-prem, hybrid) where the pack is supported.
- **Out of scope.** replace_with_paragraph naming what the pack does
  not address — for example, customer SOPs, training records, supplier
  audits, regulator filings, certification.

## 2. Required Customer Inputs

The customer must own the items below before installing the pack.
Nomos provides automation; the customer remains accountable for content
correctness and applicability.

### 2.1 Required References

| Reference id | Title | Acquisition path | Hash captured | Required by gate |
| --- | --- | --- | --- | --- |
| replace_with_ref_id | replace_with_ref_title | replace_with_publisher_url_or_internal_path | replace_with_yes_or_no | replace_with_gate_id |

The pack does not redistribute licensed standards. The customer
acquires a licensed copy through the publisher and points the pack's
reference loader at the local artefact through
`references/<pack-id>.yaml` (see
[`templates/regulated/licensed-reference-intake.yaml`](../../../../templates/regulated/licensed-reference-intake.yaml)).

### 2.2 Required Workflow Configuration

| Item | Value owned by customer |
| --- | --- |
| Provider | GitHub / GitLab / other (replace_with_provider) |
| Branch protection on `main` (or equivalent) | required reviews, required-status checks, signed commits — replace_with_setting |
| Approval reviewers | replace_with_reviewer_team_or_codeowners_path |
| Secrets / variables | replace_with_secret_names |
| Self-hosted runners (if any) | replace_with_runner_label |

### 2.3 Required Customer Records

- Customer-integration checklist:
  [`templates/regulated/customer-integration-checklist.md`](../../../../templates/regulated/customer-integration-checklist.md).
- Intended-use record (regulated impact, electronic-record scope,
  electronic-signature scope, AI/RAG scope) signed by the
  customer-side accountable owner.
- Supplier qualification record covering Nomos as a supplier under the
  customer's QMS.

## 3. Install Steps

> The pack must work on a clean checkout of Nomos at the version named
> in section 0. Each step below should map to a single command or PR
> the customer can review before applying.

1. **Clone or update Nomos** at the supported version.

   ```bash
   git -C <nomos-checkout> fetch origin
   git -C <nomos-checkout> checkout v<pack-supported-nomos-version>
   ```

2. **Configure references.** Copy the reference template into the
   customer repository and edit it to point at the licensed artefacts
   acquired in step 2.1.

   ```bash
   cp templates/regulated/licensed-reference-intake.yaml \
      references/<pack-id>.yaml
   $EDITOR references/<pack-id>.yaml
   ```

3. **Add the pack's workflow file(s).** replace_with_paragraph naming
   the workflow files the pack ships and where they should live in the
   customer's repository. Document any minimum permissions, concurrency
   settings, and required-status checks.

4. **Wire the pack's gates into branch protection.** Add the workflow
   jobs from step 3 to the customer's required-status checks for the
   protected branch.

5. **Run the gate locally** to confirm the pack's structure is reachable
   from the customer's checkout.

   ```bash
   python scripts/regulated_docs_gate.py \
     --report .regulated-doc-gate/regulated-doc-gate-report.json
   ```

6. **Run the pack's smoke verification.** replace_with_paragraph
   describing the pack-specific smoke command (for example, an
   atomization run, a fidelity gate, an evidence-bundle render).

7. **Open a PR that adds `references/<pack-id>.yaml` and any
   pack-required workflow files.** The customer's CODEOWNERS for the
   pack's evidence directory must include the customer's quality and
   technical owners. Approval recorded under the existing approval
   workflow at `docs/regulated/validation-pack/approval-workflow.yaml`.

## 4. Expected Outputs

After a green install, the customer should observe:

| Artefact | Path | Producer | Consumer |
| --- | --- | --- | --- |
| Regulated docs gate report | `.regulated-doc-gate/regulated-doc-gate-report.json` | `scripts/regulated_docs_gate.py` | release-evidence bundle |
| replace_with_artefact | replace_with_path | replace_with_producer | replace_with_consumer |

## 5. Gates

The pack expects the following gates to be configured. List exit codes
and the meaning of pass / fail for each.

| Gate | Command | Pass meaning | Fail meaning |
| --- | --- | --- | --- |
| Regulated docs gate | `python scripts/regulated_docs_gate.py` | YAML and controlled-doc structure pass; no overclaiming patterns. | Findings printed to stdout; exit 1; install incomplete. |
| replace_with_gate | replace_with_command | replace_with_pass_meaning | replace_with_fail_meaning |

## 6. Claim Boundary

This pack makes pack-specific evidence assembly possible inside the
customer's environment. It does not convert Nomos into a certified,
validated, or regulator-approved system. The pack's allowed and
prohibited claims are documented in [`claim-boundary.md`](../claim-boundary.md)
and remain bounded by the project-wide
[`docs/public-claim-boundary.md`](../../../public-claim-boundary.md).

## 7. Validation Checklist

Customers complete [`validation-checklist.md`](../validation-checklist.md)
once per environment install. The checklist must be signed by the
customer-side accountable owner before the pack is treated as
operational.

## 8. Rollback And Recovery

- **Rollback path.** replace_with_paragraph describing how the customer
  removes the pack: revert the install PR, drop
  `references/<pack-id>.yaml`, remove the pack's required-status
  checks from branch protection.
- **Recovery on partial install.** replace_with_paragraph naming which
  evidence persists after a partial rollback and what must be
  re-generated before the pack is reinstalled.

## 9. Maintenance

- **Periodic review cadence.** replace_with_value (for example,
  `quarterly`, `at every Nomos minor release`).
- **Owner.** replace_with_role.
- **Change log.** Each material change to this guide is recorded in
  `docs/regulated/decisions/` with a decision-record entry.
