# Control Plane

The control plane is the optional multi-project supervision layer for Nomos.

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
