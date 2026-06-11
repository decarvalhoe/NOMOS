# Control Plane

The control plane is the optional multi-project supervision layer for Nomos.

## Status: archived (ADR-0006)

These packages are **archived exploratory code** ([ADR-0006](../docs/decisions/0006-control-plane-archive.md), VRC-04 #550): functional, internally tested, and with **zero production callers**. They are frozen in place to inform the v0.9.x portfolio-governance design and are no longer gated in CI. Revival requires a capability-claim issue with a declared production caller; reactivation restores CI gating in the same PR.

## Alpha Status

The current release includes Go packages for dashboard, registry, and storage tests. It is not yet a hosted product, production API, or regulated system boundary.

## Intended Responsibilities

- project registry;
- execution history;
- report and evidence browsing;
- exception and deviation visibility;
- portfolio-level views;
- future customer integration surfaces.

## Current Release Boundary

For `v0.1.0-ALPHA`, the CLI and generated artifacts remain the primary product surface. Control-plane work should not be presented as production-ready until authentication, authorization, audit trail, retention, deployment, backup, and validation evidence are implemented.
