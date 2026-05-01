# Adapter Contract

This contract defines what an adapter must publish before Nomos can load it as
a versioned integration point. The machine-readable source of truth is
`specs/adapter-manifest.cue`, validated through the `#AdapterManifest`
definition.

## Manifest File

Each adapter publishes one manifest named `adapter.nomos.yaml` at the adapter
root. The manifest declares:

- adapter identity, owner, lifecycle status, package coordinates, and version;
- compatible Nomos core and schema versions;
- supported languages, frameworks, file globs, excluded paths, and surfaces;
- capabilities, inputs, outputs, and evidence kinds;
- executable command protocol for detection and self-checks;
- limitations and required compatibility checks.

The example `specs/examples/adapter-manifest.node-typescript.yaml` shows the
minimum concrete shape expected for a Node / TypeScript adapter.

## Capability Contract

Capabilities are stable identifiers, not prose labels. A capability declaration
must include:

- `id`: one value from `#AdapterCapabilityID`;
- `category`: detection, extraction, validation, evidence, or integration;
- `status`: experimental, stable, or deprecated;
- `surfaces`: affected product surfaces;
- `inputs`: the data classes the adapter reads;
- `outputs`: the result classes the adapter emits;
- `evidence`: the evidence kinds that make findings auditable.

Adapters must not claim support for a surface unless at least one capability
emits evidence for that surface. Experimental capabilities are allowed, but
strict gates should treat their confidence as lower unless policy says
otherwise.

## Versioning

Adapter versions use SemVer and are declared as `<adapter-id>@<semver>`.

- `MAJOR`: removes or renames a capability, command, output kind, evidence kind,
  or supported surface; changes command output incompatibly.
- `MINOR`: adds a compatible capability, command, surface, framework, or
  evidence kind.
- `PATCH`: fixes detection behavior or documentation without changing public
  fields.

The manifest also declares `compatibility.nomos_core.min_version` and optional
`max_version`. Nomos core may refuse an adapter when the running core version is
outside that range, even if the manifest itself validates.

## Command Protocol

Commands are declared, not inferred. Each command entry has an `id`, `argv`,
input transport, output transport, timeout, and `required` flag.

Required commands for a production adapter:

- `detect`: inspect a repository and emit adapter detection results;
- `self-check`: verify adapter dependencies, grammars, and fixture support.

Command output schemas are referenced by name in `schema_ref`. This issue does
not define those result schemas and does not modify Nomos report formats; it
only defines the adapter manifest contract that future CLI work can load.

## Compatibility Test Contract

`test_contract.required_checks` is the adapter-side compatibility checklist that
future CLI validation must execute:

- `manifest-validates`
- `capabilities-declared`
- `version-compatibility-declared`
- `commands-smoke-tested`
- `fixtures-pass`
- `limitations-declared`

An adapter is not considered compatible if it only ships code. It must also ship
the manifest, fixtures or declared fixture paths, limitations, and version
matrix needed to reproduce support claims.
