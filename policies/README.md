# Policies

This area is reserved for a future declarative policy framework for Nomos gates.

## Status in `v1.0.0-BETA.1`

No executable policy lives here in `v1.0.0-BETA.1`: this directory holds this README and nothing else, and nothing in it is read by the CLI or by CI. The gates NOMOS actually enforces are code: the guards and gates in `scripts/` (run by CI and by `.forgejo/portes.sh`) and the fidelity and compliance engines in `cli/`. This status is computed from the tree by `scripts/unshipped_surfaces_guard.py`: the root READMEs may not describe policies as executable or operational while this directory stays README-only.

## Intended Future Role

A policy framework here would support:

- release gates;
- strict checks;
- scope and claim boundaries;
- exception handling;
- attestation verification;
- regulated-readiness evidence controls.

## Authoring Rule

A policy must be:

- explicit about fail-open vs fail-closed behavior;
- testable;
- linked to the evidence or risk it controls;
- documented when it affects release or customer claims.

## Regulated-Readiness Boundary

A policy can enforce evidence structure, but it is not by itself proof of regulatory compliance. Compliance claims require approved procedures, executed records, review evidence, and intended-use validation.
