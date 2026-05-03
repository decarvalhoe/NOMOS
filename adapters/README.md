# Adapters

Adapters describe how Nomos observes application stacks without making the core depend on a specific framework.

## Alpha Status

The adapter area is a contract and early-profile workspace. It is useful for design validation and controlled pilots, but it is not yet a stable plugin marketplace.

Current material:

- `adapter-contract.md` defines the public adapter contract.
- `node-typescript/` contains a Node/TypeScript profile and fixtures.
- `python/` contains a Python profile and fixtures.
- `jvm/` contains a JVM profile.
- `../specs/adapter-manifest.cue` defines the machine-readable adapter manifest schema.
- `../specs/examples/adapter-manifest.node-typescript.yaml` shows an example manifest.

## Adapter Requirements

Every adapter must declare:

- supported stacks and surfaces;
- capabilities and maturity status;
- commands exposed to the Nomos CLI;
- forbidden patterns;
- known limitations;
- compatible Nomos core version;
- test fixtures and evidence expectations.

## Regulated-Readiness Rule

Adapters may help detect source-to-product drift, but an adapter alone is not validation evidence. Any regulated claim needs traceability from source to implementation to verification to approved evidence.
