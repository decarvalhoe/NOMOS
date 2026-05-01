# Output Package

`internal/output` provides the standard NOM-204 renderers for Nomos reports.

The package does not implement `validate`, `diagnose`, schemas, or adapter
contracts. It renders the existing `nomos-report` contract as:

- stable machine-readable JSON with `WriteJSON`;
- readable Markdown with `WriteMarkdown`.

The renderers normalize check, finding, evidence, waiver, and reference ordering
before writing. That keeps CLI output deterministic even when future collectors
append results in filesystem, adapter, or execution order.
