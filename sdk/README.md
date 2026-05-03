# SDK

The SDK area is reserved for client libraries and integration helpers.

## Alpha Status

No stable public SDK is shipped in `v0.1.0-ALPHA`. The canonical integration surface is currently the CLI plus generated JSON/YAML artifacts.

## Intended Future Uses

- helpers for reading Nomos evidence packs;
- CI integrations;
- typed clients for reports and attestations;
- customer integration helpers;
- downstream Praxis compatibility.

## Compatibility Rule

Until SDK packages are versioned and documented, downstream systems should integrate through explicit artifact contracts in `specs/` rather than importing unstable internal code.
