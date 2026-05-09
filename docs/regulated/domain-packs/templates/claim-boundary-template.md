# Claim Boundary Template — `<pack-id>` Domain Pack

> Replace every `replace_with_*` token before publishing. This file
> sits alongside the pack's `install-guide.md` and
> `validation-checklist.md`. It must be reachable from any marketing
> page or customer-facing surface that mentions the pack.

## Scope

This page bounds public claims about the **`<pack-id>`** domain pack
when it is installed on top of Nomos. It does not replace the
project-wide [public claim boundary](../../../public-claim-boundary.md);
it adds pack-specific allowed and prohibited claims on top of it.

## Pack Allowed Claims

The pack supports the following claims when (and only when) the
customer's signed validation checklist is on file for the environment
being claimed about:

- replace_with_allowed_claim_1 (for example, "produces a traceable
  control crosswalk between the customer's intended use and the
  references named in `references/<pack-id>.yaml`").
- replace_with_allowed_claim_2.
- replace_with_allowed_claim_3.

Every allowed claim is conditional. The conditional clause must
appear next to the claim. Recommended pattern:

> "in the customer's environment described by signed validation
> checklist `replace_with_checklist_id`, with references at versions
> `replace_with_versions`."

## Pack Conditional Claims

The pack supports the following claims with the named context:

- **"<pack-id>-ready"** means the pack's structure is installed and
  the validation checklist passes for the environment claimed about.
  It does not mean certified, validated, or regulator-approved.
- replace_with_conditional_claim_2.

## Pack Prohibited Claims

In addition to every prohibited claim in the project-wide public
claim boundary, the pack must never be used to support the following
pack-specific claims:

- replace_with_prohibited_claim_1 (for example, "Part 11 e-signature
  platform").
- replace_with_prohibited_claim_2 (for example, "certified
  regulator-approved system").
- replace_with_prohibited_claim_3.

Closing the pack's underlying DOR issue, installing the pack, or
passing the regulated documentation gate does not authorise any of
the claims above.

## Evidence Rule

Every public statement about the pack must map to one of:

- a passing automated gate from the install guide;
- a generated artefact recorded in the validation checklist;
- a reviewed document in `docs/regulated/`;
- a controlled decision under `docs/regulated/decisions/`;
- a known gap with owner and next action.

If a public statement cannot be mapped to one of the above, it must
be removed or rewritten as a roadmap item.
