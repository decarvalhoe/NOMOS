# ADR-REQIF-001 - ReqIF and ALM/QMS Import-Export Boundaries

## Statut

Accepted

## Date

2026-05-02

## Decision ID

ADR-REQIF-001

## Owner

RBOK Team (engineering-owner)

## Affected Products

- Nomos (primary)
- Praxis (downstream consumer)

## Affected Controls

- CTL-VAL-001 (Risk-Based Validation Strategy)
- CTL-VAL-002 (Risk-Based Testing)
- CTL-DOC-001 (Quality Risk Management)
- CTL-CC-001 (Configuration And Change Control)

## Contexte

Regulated environments commonly use ALM (Application Lifecycle Management)
and QMS (Quality Management System) tools that exchange requirements and
traceability data via ReqIF (Requirements Interchange Format, OMG standard).

Nomos produces canonical atoms, traceability matrices, evidence bundles,
and control matrices that must interoperate with these external systems
without losing the canonical-first invariants:

1. Nomos is the source of truth for canonical content.
2. External tools must not silently override canonical hashes or references.
3. Import from external tools must be treated as a controlled intake, not
   an automatic merge.

The question is: what should Nomos export, what should it import, and
where is the boundary?

## Sources Et Unités

- Source IDs: ISPE-GAMP5-2E-2022, ISO-IEC-IEEE-12207-2026, FDA-CSA-2025
- Unit IDs: NOMOS-CLI-SCAFFOLD, NOMOS-SPEC-SCHEMAS
- Issues: RBOKproject/NOMOS#156

## Options

### Option A — Full bidirectional ReqIF sync

Nomos imports and exports ReqIF, maintaining round-trip fidelity with
external ALM/QMS tools.

Consequences:
- Requires ReqIF parser and serializer in Go.
- Risk of canonical drift: an ALM tool could modify a requirement that
  Nomos considers immutable.
- Complexity of merge conflict resolution between Nomos atoms and ALM
  objects.
- Ongoing maintenance burden for ReqIF schema compatibility.

### Option B — Export-only ReqIF with controlled intake boundary

Nomos exports its canonical data as ReqIF for consumption by external
tools, but does not import ReqIF back into its canonical chain. External
requirements enter Nomos only through the existing source intake process
(source manifest, sidecar validation, hash verification).

Consequences:
- Simpler implementation: export adapter only.
- Clear boundary: Nomos canonical content is never mutated by external
  tooling.
- External tools can consume Nomos requirements but cannot push changes
  back without going through governed intake.
- ALM-side traceability links to Nomos canonical refs, not the reverse.

### Option C — No ReqIF support

Nomos uses only its own JSON/YAML formats and does not support ReqIF.

Consequences:
- Simplest implementation.
- Organizations using ALM/QMS tools must manually map Nomos outputs.
- Limits adoption in regulated environments where ReqIF is contractual.

## Décision

We adopt **Option B — Export-only ReqIF with controlled intake boundary**.

Nomos will:

### Export (Nomos → External)

1. **Requirements export**: canonical atoms serialized as ReqIF SpecObjects
   with stable IDs, canonical refs, content hashes, review states, and
   source spans.

2. **Traceability matrix export**: atom parent-child relationships and
   cross-references serialized as ReqIF SpecRelations.

3. **Control matrix export**: regulated controls with evidence status,
   gate assignments, and claim boundaries.

4. **Evidence summary export**: coverage ratios, validation status, and
   deviation/waiver records as ReqIF attributes.

5. **Metadata propagation**: every exported ReqIF object carries:
   - `nomos:canonical_ref` (stable identifier)
   - `nomos:content_hash` (SHA256 of source content)
   - `nomos:review_state` (draft/pending/approved)
   - `nomos:source_span` (file:line:col)
   - `nomos:export_timestamp` (ISO 8601)

### Not Exported

- Raw source file content (only hashes and references).
- Internal AST block structure (implementation detail).
- Lockfile or attestation signatures (separate evidence channel).

### Not Imported

1. **No ReqIF import into canonical chain**: external requirements do not
   enter Nomos through ReqIF. They enter through the standard source
   intake process (source manifest registration, sidecar validation,
   hash verification, owner assignment).

2. **No ALM-originated requirement IDs**: Nomos generates its own stable
   atom IDs from canonical references. ALM tool IDs may be stored as
   metadata but never replace Nomos IDs.

3. **No automatic merge from external tools**: changes made in an ALM
   tool to Nomos-exported requirements do not flow back. The ALM tool
   is a downstream consumer, not a peer source of truth.

4. **No QMS record import**: CAPA, deviation, and audit records from
   external QMS tools are not imported into Nomos. They are referenced
   by ID in Nomos evidence records but managed in the QMS tool.

### Integration Pattern

```text
Nomos (canonical source of truth)
  ├── export → ReqIF → ALM tool (read-only consumption)
  ├── export → JSON → QMS dashboard (read-only consumption)
  └── export → Evidence bundle → Audit archive

External source
  └── intake → source manifest → sidecar validation → Nomos canonical chain
```

### Boundary Rules

1. Any external system that consumes Nomos ReqIF exports must not modify
   the `nomos:canonical_ref` or `nomos:content_hash` attributes.

2. If an external system needs to add requirements, those requirements
   must be registered as new sources in Nomos through the standard intake
   process before they can appear in the canonical chain.

3. Nomos ReqIF exports are timestamped and hashed. A consuming system
   can verify it holds the latest export by comparing hashes.

4. The ReqIF export adapter is a Nomos adapter (versioned, declared in
   adapter manifest) and not a core CLI feature. It may be implemented
   as `nomos export --format reqif`.

## Conséquences

- Impact contrats: ReqIF export adapter spec to be added to adapter manifest.
- Impact schémas: ReqIF attribute mapping to be documented in specs/.
- Impact DB: No database changes (export-only).
- Impact core: No core changes. Export adapter consumes AtomSet and
  BundleManifest through public API.
- Impact API/UI: No API/UI changes.
- Impact tests: ReqIF export adapter requires golden-file tests with
  validated ReqIF XML output.
- Impact release: ReqIF adapter is optional; its absence does not block
  release. Core Nomos release does not depend on ReqIF support.

## Risk Impact

- Low risk: export-only means no canonical drift from external tools.
- Medium risk: ReqIF XML format has quirks across tool vendors. Golden
  tests must cover at least DOORS, Polarion, and Jama Connect imports.

## Evidence Impact

- No change to existing evidence chain.
- ReqIF exports become an additional evidence artifact type when the
  adapter is implemented.

## Approval Status

Accepted. No regulated claim depends on this decision today. The decision
becomes binding when the ReqIF adapter is implemented.

## Follow-Up Issues

- Implement ReqIF export adapter (future ticket, not yet scheduled).
- Document ReqIF attribute mapping in specs/reqif-mapping.yaml.
- Add golden-file tests for DOORS/Polarion/Jama ReqIF import validation.
- Update adapter manifest to declare ReqIF export capability.

## Expiration Ou Révision

Review this decision if:
- A customer contractually requires bidirectional ReqIF sync.
- The OMG ReqIF standard undergoes a major revision that changes the
  cost-benefit of bidirectional support.
- A regulated audit finding requires ALM-originated requirements to be
  treated as canonical sources.
