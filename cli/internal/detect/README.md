# Detection Package

`internal/detect` implements the NOM-301 repository detector without coupling it
to CLI command dispatch or later admission verdict schemas.

It scans a repository tree and emits a deterministic `nomos.detect.v1` JSON
report containing:

- languages;
- package/build/CI tools;
- CI providers;
- product surfaces: `api`, `ui`, `worker`, `data`, `infra`, `docs`.

The detector is heuristic by design. Findings include evidence paths and
reasons so `diagnose` and adapter work can decide whether a signal is strong
enough for admission.

`nomos diagnose` consumes this package but keeps admission classification in
`internal/diagnose`. This keeps NOM-301 owned by the detection package while
still making the raw detection report exportable through `WriteJSON`.
