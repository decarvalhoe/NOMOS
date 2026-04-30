# Detection Package

`internal/detect` implements the NOM-301 repository detector without coupling it
to CLI command dispatch or later admission verdict schemas.

It scans a repository tree and emits a deterministic `nomos.detect.v1` JSON
report containing:

- languages;
- package/build/CI tools;
- CI providers;
- product surfaces: `api`, `ui`, `worker`, `data`, `infra`, `docs`.
- Tree-sitter parsing coverage and diagnostics.

The detector is heuristic by design. Findings include evidence paths and
reasons so later `diagnose` and adapter work can decide whether a signal is
strong enough for admission.

Tree-sitter is used as the AST-backed parsing layer for the first adapter
families: Go, Java, JavaScript, Python, TSX, and TypeScript. The report includes
`treeSitter.parsedFiles`, per-language parse evidence, syntax parse diagnostics,
and clear missing-grammar diagnostics such as:

```text
tree-sitter grammar missing for language C#; registered grammars: Go, Java, JavaScript, Python, TSX, TypeScript
```

Parsing is bounded to the same file prefix limit as content heuristics, so a
medium repository can be scanned without unbounded AST work.

Command integration is intentionally deferred to NOM-302 (`nomos diagnose`).
This keeps NOM-301 owned by the detection package while still making the report
exportable through `WriteJSON`.
