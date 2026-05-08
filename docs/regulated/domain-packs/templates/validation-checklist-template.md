# Validation Checklist Template — `<pack-id>` Domain Pack

> Replace every `replace_with_*` token before publishing. The checklist
> is completed once per customer environment install and is signed by
> the customer-side accountable owner before the pack is treated as
> operational.

## 0. Identity

- **Pack id:** replace_with_pack_id
- **Pack version:** replace_with_pack_version
- **Customer / deployment:** replace_with_customer_or_deployment_id
- **Environment:** replace_with_env (for example, dev / staging /
  production)
- **Date:** replace_with_iso_date
- **Accountable owner (customer side):** replace_with_owner_role_and_name

## 1. Prerequisites

- [ ] Nomos baseline at the pack-supported version is installed.
- [ ] Customer-integration checklist (per
  [`templates/regulated/customer-integration-checklist.md`](../../../../templates/regulated/customer-integration-checklist.md))
  is filled in and signed.
- [ ] Intended-use record names the pack and the customer's accountable
  owner.
- [ ] Supplier qualification record covers Nomos at the supported
  version.

## 2. References

- [ ] Every required reference from the install guide section 2.1 has
  a customer-acquired licensed copy.
- [ ] `references/<pack-id>.yaml` exists in the customer repository.
- [ ] Each reference entry records canonical id, title, customer-side
  storage path, and SHA-256 hash captured at intake.
- [ ] The pack's reference loader runs without error against
  `references/<pack-id>.yaml`.

## 3. Workflow Configuration

- [ ] Pack workflow files are present under `.github/workflows/` (or
  the customer-equivalent path) at the version specified by the
  install guide.
- [ ] Branch protection on the protected branch lists every gate the
  install guide marks as required.
- [ ] CODEOWNERS for the pack's evidence directory routes approvals to
  the customer's quality and technical owners.
- [ ] Approval workflow at
  `docs/regulated/validation-pack/approval-workflow.yaml` references
  the pack's evidence directory or has been extended to.

## 4. Outputs

For each row in the install guide section 4 ("Expected Outputs"):

- [ ] The artefact appears at the documented path after a green run.
- [ ] The artefact is consumed by the documented consumer (release
  evidence bundle, control matrix, evidence index, etc.).
- [ ] No artefact carries an overclaim warning from the regulated docs
  gate.

## 5. Gates

For each row in the install guide section 5 ("Gates"):

- [ ] The gate command runs without error on a clean checkout.
- [ ] The gate's pass / fail meaning matches the install guide.
- [ ] The gate is wired into the customer's required-status checks.

## 6. Claim Boundary

- [ ] The customer's public communication about this install matches
  [`claim-boundary.md`](../claim-boundary.md) and the project-wide
  [`docs/public-claim-boundary.md`](../../../public-claim-boundary.md).
- [ ] No marketing surface claims certification, regulator approval,
  Part 11 compliance as a platform, or any other prohibited claim
  documented in the pack's claim boundary.
- [ ] If the customer plans to make a conditional claim, the
  conditional clause appears verbatim alongside the claim.

## 7. Rollback Drill

- [ ] Rollback path from the install guide section 8 has been
  exercised at least once on the environment.
- [ ] Recovery on partial install has been documented for the
  environment.

## 8. Result

- **Result.** replace_with_pass / replace_with_fail
- **Open deviations.** replace_with_list_or_none.
- **Waivers.** replace_with_list_or_none. Each waiver records the
  controlled decision id under `docs/regulated/decisions/`.
- **Reviewer:** replace_with_role_and_name
- **Approval meaning.** Approval here records that the pack is
  operational in the customer's environment as scoped above. It is
  not, by itself, a regulated-grade approval, a certification, or a
  regulator filing.
