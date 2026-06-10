# Swiss live source connectors (CKM-H5)

This directory holds **point-in-time evidence** from real, read-only fetches of
open Swiss sources. It closes the audit gap (#518/#523) where CKM-10 carried a
CUE manifest and tests but only **synthetic hashes and no live fetch**.

## What the connector does

`nomos connector fetch` performs a genuine end-to-end pipeline:

1. **Read-only HTTP GET** of an open source (no writes anywhere).
2. **Real sha256** of the bytes that actually came back — never a fabricated digest.
3. **Line-span atoms**: the content is tiled into half-open `[start_byte, end_byte)`
   segments.
4. **Body ledger**: every byte (including each newline) belongs to exactly one
   segment, so `uncovered_bytes == 0` is proven, not asserted.

```bash
nomos connector sources
nomos connector fetch \
  --url "https://www.agvchapp.bfs.admin.ch/api/communes/snapshot?date=01-01-2026" \
  --connector-id ch-ofs-commune-register \
  --out docs/regulated/ch-connectors/ch-ofs-commune-register.evidence.json
```

## Scope discipline — open data only

The connector targets **open** government sources:

- **OFS / BFS** commune register (open statistical data);
- **Fedlex / ELI** (open federal law).

Paid norms (**SIA**, ISO, GAMP 5) are intentionally **excluded** — they are never
fetched or redistributed (doctrine §2.6, no-full-text). The committed evidence
records the **hash, byte count, span coverage, and a truncated atom sample** —
**not** the source text (`no_full_text: true`).

## Evidence is point-in-time

`*.evidence.json` records a specific fetch (URL + timestamp + real hash). Upstream
content legitimately changes over time, so the committed hash is a **receipt of a
real fetch**, not a pinned oracle. The offline test suite
(`cli/internal/connector`) proves the pipeline deterministically over a local
`httptest` server; the live fetch is exercised by a network-gated test:

```bash
NOMOS_LIVE_CH_FETCH=1 go test ./internal/connector/ -run TestLive -v
```
