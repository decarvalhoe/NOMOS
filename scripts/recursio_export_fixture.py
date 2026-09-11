#!/usr/bin/env python3
"""#612 — the "Recursio export of reference", regenerated deterministically.

Recursio (the crawler) is not in this repository, and the E2E must run without
it and without a network. What IS here is the shape of what Recursio hands NOMOS:
for each captured page, a normalised Markdown export and one JSONL record with
#610 provenance. This script produces that export from the static fixture site,
deterministically, so a test can prove two things:

  1. HTML → normalised text → Markdown is REPRODUCIBLE: regenerating from the
     committed site yields the committed export byte for byte (``--check``).
  2. one changed byte in one HTML page changes that page's content hash, its
     normalised hash, its Markdown, and therefore the snapshot root — the
     "diff/version détectable" acceptance criterion.

This is deliberately a fixture normaliser, not an HTML engine. It strips tags,
drops elements marked as page chrome (nav/footer with class "chrome"), keeps
h1/h2 as Markdown headings and paragraphs as paragraphs, and collapses
whitespace. Anything a real crawler does beyond that is Recursio's job; the
contract NOMOS consumes is the JSONL, not the normaliser.

Robots and licence: the fixture site is synthetic, so both decisions are
recorded as `allowed` with a note that they were not adjudicated for any real
site — exactly what #610 says such a record must carry.
"""

from __future__ import annotations

import argparse
import hashlib
import html
import json
import re
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FIXTURE_REL = Path("tests") / "fixtures" / "recursio-e2e"
BASE_URL = "https://reglement.example.invalid"
FETCHED_AT = "2026-09-05T09:00:00Z"  # a fixture is a fixed instant, or it is not reproducible
CRAWLER = "recursio-fixture/0.1"

_CHROME = re.compile(r"<(nav|footer)\b[^>]*class=\"chrome\"[^>]*>.*?</\1>", re.S | re.I)
_HEAD = re.compile(r"<head\b.*?</head>", re.S | re.I)
_H = re.compile(r"<h([12])\b[^>]*>(.*?)</h\1>", re.S | re.I)
_P = re.compile(r"<p\b[^>]*>(.*?)</p>", re.S | re.I)
_TAG = re.compile(r"<[^>]+>")
_WS = re.compile(r"\s+")


def sha256_bytes(data: bytes) -> str:
    return "sha256:" + hashlib.sha256(data).hexdigest()


def clean(fragment: str) -> str:
    return _WS.sub(" ", html.unescape(_TAG.sub("", fragment))).strip()


def normalise(html_bytes: bytes) -> tuple[str, str]:
    """Return (normalised_text, markdown). The normalised text is what the
    normalized_content_hash covers: headings and paragraphs, chrome removed,
    whitespace collapsed — the same page served with different boilerplate
    hashes the same here."""
    doc = html_bytes.decode("utf-8")
    doc = _HEAD.sub("", doc)
    doc = _CHROME.sub("", doc)
    blocks: list[tuple[str, str]] = []
    for m in re.finditer(r"<h([12])\b[^>]*>.*?</h\1>|<p\b[^>]*>.*?</p>", doc, re.S | re.I):
        frag = m.group(0)
        hm = _H.match(frag)
        if hm:
            blocks.append(("h" + hm.group(1), clean(hm.group(2))))
        else:
            pm = _P.match(frag)
            if pm:
                blocks.append(("p", clean(pm.group(1))))
    text = "\n".join(t for _, t in blocks if t)
    md_lines = []
    for kind, t in blocks:
        if not t:
            continue
        md_lines.append(("# " if kind == "h1" else "## " if kind == "h2" else "") + t)
        md_lines.append("")
    return text, "\n".join(md_lines).rstrip("\n") + "\n"


def pages(site: Path) -> list[Path]:
    return sorted(p for p in site.rglob("*.html"))


def build_export(site: Path, out: Path) -> list[dict]:
    """Write captures/*.md and sources.jsonl under `out`; return the records."""
    captures = out / "captures"
    captures.mkdir(parents=True, exist_ok=True)
    records = []
    for page in pages(site):
        rel = page.relative_to(site).as_posix()
        raw = page.read_bytes()
        text, md = normalise(raw)
        md_name = rel[:-len(".html")].replace("/", "__") + ".md"
        (captures / md_name).write_text(md, encoding="utf-8")
        url = f"{BASE_URL}/{rel}"
        depth = rel.count("/")
        parent = f"{BASE_URL}/index.html" if depth else ""
        source_id = rel[:-len(".html")].replace("/", "-")
        records.append({
            "source_id": source_id,
            "version_id": "2026-09",
            "locator": url,
            "content_hash": sha256_bytes(raw),
            "size_bytes": len(raw),
            "captured_at": FETCHED_AT,
            "source_type": "html",
            "export_path": f"captures/{md_name}",
            "web_source": {
                "canonical_url": url,
                "fetched_url": url,
                "http_status": 200,
                "content_type": "text/html; charset=utf-8",
                "content_hash": sha256_bytes(raw),
                "normalized_content_hash": sha256_bytes(text.encode("utf-8")),
                "fetched_at": FETCHED_AT,
                "crawler_version": CRAWLER,
                "scope_policy": "seed" if depth == 0 else "in_scope",
                "robots_decision": "allowed",
                "licence_decision": "allowed",
                **({"parent_url": parent} if parent else {}),
                "depth": depth,
                "claim_boundary": "Synthetic fixture site: robots and licence recorded as allowed by construction, not adjudicated for any real site.",
            },
        })
    (out / "sources.jsonl").write_text("".join(json.dumps(r, ensure_ascii=False, sort_keys=True) + "\n" for r in records), encoding="utf-8")
    return records


def check(site: Path, export: Path) -> int:
    """Regenerate into a temp dir and compare with the committed export.
    Exit 4 on any difference: either the site changed and the export is stale,
    or the normaliser changed — both must be deliberate."""
    with tempfile.TemporaryDirectory() as tmp:
        fresh = Path(tmp) / "export"
        build_export(site, fresh)
        drift = []
        for f in sorted(fresh.rglob("*")):
            if f.is_file():
                rel = f.relative_to(fresh)
                committed = export / rel
                if not committed.exists() or committed.read_bytes() != f.read_bytes():
                    drift.append(rel.as_posix())
        for f in sorted(export.rglob("*")):
            if f.is_file() and f.name != "snapshot.json" and not (fresh / f.relative_to(export)).exists():
                drift.append(f.relative_to(export).as_posix() + " (orphan)")
    if drift:
        print("recursio-export: DRIFT — committed export does not match a fresh normalisation of the site:", file=sys.stderr)
        for d in drift:
            print("  -", d, file=sys.stderr)
        return 4
    print(f"recursio-export: OK — {len(pages(site))} page(s) re-normalise byte-for-byte to the committed export")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="verify the committed export is a fresh normalisation of the site (exit 4 on drift)")
    parser.add_argument("--root", default=str(ROOT), help="repository root holding tests/fixtures/recursio-e2e (tests point this at a mutated copy)")
    parser.add_argument("--out", default="", help="where to write the export (default: the fixture's export/ under --root)")
    args = parser.parse_args()
    fixture = Path(args.root).resolve() / FIXTURE_REL
    site, export = fixture / "site", fixture / "export"
    if args.check:
        return check(site, export)
    out = Path(args.out) if args.out else export
    records = build_export(site, out)
    print(f"recursio-export: wrote {len(records)} record(s) to {out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
