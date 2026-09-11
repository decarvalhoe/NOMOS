# SDK

The SDK area is reserved for client libraries and integration helpers.

## Status in `v1.0.0-BETA.1`

No SDK is shipped in `v1.0.0-BETA.1`: this directory holds this README and nothing else, and no code, package or client library lives here. The integration surface is the CLI (`cli/`) plus the generated JSON/YAML artifacts under the contracts in `specs/`; the replayed integration path is `docs/48-customer-integration-guide.md`. This status is computed from the tree by `scripts/unshipped_surfaces_guard.py`: the root READMEs may not describe an SDK as shipped while this directory stays README-only.

## Intended Future Uses

- helpers for reading Nomos evidence packs;
- CI integrations;
- typed clients for reports and attestations;
- customer integration helpers;
- downstream Praxis compatibility.

## Compatibility Rule

Until SDK packages are versioned and documented, downstream systems should integrate through explicit artifact contracts in `specs/` rather than importing unstable internal code.
