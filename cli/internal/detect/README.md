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
- Node / TypeScript adapter signals for routes, services, mocks, fixtures,
  dependencies, test tooling, and likely hardcoded catalogues.

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

## Node / TypeScript Adapter Signals

NOM-402 adds a first adapter-specific report section named `nodeTypeScript`.
It is heuristic but evidence-backed:

- routes: Next.js `app/api/**/route.ts`, Next.js `pages/api/**`, Next.js UI
  pages, Express/Fastify handlers, and route/controller modules;
- services: `services/` modules, `service.ts` suffixes, and exported service
  symbols;
- mocks: `__mocks__/`, `mocks/`, MSW handlers, `jest.mock`, and `vi.mock`;
- fixtures: `fixtures/`, `__fixtures__/`, `testdata/`, and Cypress fixtures;
- hardcoded catalogues: catalogue paths and TypeScript constants that look like
  static business enumerations.

The official compatibility corpus lives in
`adapters/node-typescript/fixtures/nextjs-api-ui`.
