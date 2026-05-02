# Reference Basis

This folder controls the external references Nomos uses for regulated IT claims.

The register does not certify Nomos. It records which external sources are being used, how they apply, which Nomos documents must address them, and which evidence is still missing.

## Records

- `external-reference-register.yaml` - source register for regulatory, standards, and high-practice references.
- `nomos-bible-corpus-policy.md` - policy that makes every registered reference a Nomos canonical bible and defines public vs licensed processing.

## Conduct

1. Add a reference only with an official or authoritative URL.
2. Record the date checked.
3. Mark paid standards as `summary_reference_only` unless a licensed clause mapping exists.
4. Map each reference to a Nomos control and evidence expectation.
5. Do not use a reference in public claims if it is not mapped to the control matrix or an explicit non-applicability record.
6. Process public bibles from official snapshots and licensed bibles from `NOMOS_LICENSED_REFERENCE_ROOT` only.
