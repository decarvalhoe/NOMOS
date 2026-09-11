# Recursio → NOMOS offline E2E fixture (#612)

A **synthetic** three-page static site and the export a crawler such as
Recursio would hand NOMOS for it. Nothing here is a real commune, a real
regulation or a real crawl.

| Path | What it is | Who writes it |
|---|---|---|
| `site/**/*.html` | the fixture site: an index and two "articles" | hand-written, versioned |
| `export/captures/*.md` | normalised Markdown export of each page | `scripts/recursio_export_fixture.py` (deterministic) |
| `export/sources.jsonl` | one record per page with `#610` web-source provenance and `export_path` | same script |
| `export/snapshot.json` | the immutable `#611` envelope sealing the records (Merkle root) | `nomos corpus snapshot seal` (real CLI) |

`scripts/recursio-e2e-fixture.sh` drives the chain offline: export ≡
normalise(site) → `snapshot verify` → `snapshot import` → `scan` → `feed` →
`body-ledger` → `attest --external-snapshot` → `strict`. It asserts that the
attestation carries the web source type and the snapshot coverage, and that
the fixture is never mutated. Exit codes: 0 pass · 1 stage failed · 4 drift or
mutation · 5 preflight.

## Regenerating

```bash
python3 scripts/recursio_export_fixture.py            # rewrite export/captures + sources.jsonl from site/
cd cli && go run . corpus snapshot seal --records ../tests/fixtures/recursio-e2e/export/sources.jsonl \
  --snapshot-id recursio-fixture-2026-09 --producer recursio-fixture/0.1 --db-schema recursio-export-v1 \
  --generated-at 2026-09-05T09:00:00Z --out ../tests/fixtures/recursio-e2e/export/snapshot.json
```

Any change to a page changes that page's `content_hash`, its
`normalized_content_hash`, its Markdown, and the snapshot root; the drift
check (`--check`, exit 4) names the stale files. Changing boilerplate marked
`class="chrome"` changes the raw hash but not the normalised one, by design.

## What this does not prove

- anything about a real site: `robots_decision` and `licence_decision` are
  `allowed` by construction here, not adjudicated;
- that Recursio (absent from this repository) produces exactly this export —
  the contract NOMOS consumes is the JSONL + envelope, and this fixture is one
  conforming producer of it;
- that the snapshot was complete relative to any operational store (see the
  `#611` claim boundary carried in the envelope and the attestation).
