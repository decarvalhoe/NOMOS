# Provenance Fixture

This synthetic fixture describes artifact provenance for a release evidence
bundle without claiming signature validity or external attestation acceptance.

## Evidence Chain

The record links source path, content hash, generator version, operator identity,
and timestamp as separate facts.

## Verification Step

The workflow compares expected hash, observed hash, artifact path, and reviewer
decision before marking evidence as accepted.

## Non-Claim Boundary

This fixture does not claim signed release assurance, legal attestation, or
regulated validation. It only exercises portable corpus artifact generation with
license-safe text.
