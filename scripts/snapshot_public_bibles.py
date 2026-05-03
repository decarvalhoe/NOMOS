#!/usr/bin/env python3
"""snapshot_public_bibles.py — Download and hash public reference bibles.

Downloads official public reference documents (NIST SSDF, NASA NPR,
FDA guidance) from their canonical URLs, computes SHA-256 hashes, and
generates sidecar YAML files. NEVER stores full text in Git.

Usage:
    python3 scripts/snapshot_public_bibles.py \
        --output docs/regulated/reference-basis/public-snapshots/ \
        [--dry-run] [--timeout 30]
"""

import argparse
import hashlib
import json
import os
import sys
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


@dataclass
class PublicBible:
    """Definition of a public reference bible to snapshot."""
    ref_id: str
    title: str
    authority: str
    url: str
    version: str
    category: str
    format: str = "pdf"
    notes: str = ""


# Official public reference bibles with stable URLs.
PUBLIC_BIBLES: list[PublicBible] = [
    PublicBible(
        ref_id="NIST-SSDF-1.1",
        title="NIST Secure Software Development Framework (SSDF) v1.1",
        authority="NIST",
        url="https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-218.pdf",
        version="1.1",
        category="software_security",
        notes="SP 800-218. Practices for secure software development lifecycle.",
    ),
    PublicBible(
        ref_id="NIST-CSF-2.0",
        title="NIST Cybersecurity Framework 2.0",
        authority="NIST",
        url="https://nvlpubs.nist.gov/nistpubs/CSWP/NIST.CSWP.29.pdf",
        version="2.0",
        category="cybersecurity",
        notes="CSWP 29. Cybersecurity risk management framework.",
    ),
    PublicBible(
        ref_id="NASA-NPR-7150.2D",
        title="NASA Software Engineering Requirements (NPR 7150.2D)",
        authority="NASA",
        url="https://nodis3.gsfc.nasa.gov/npg_img/N_PR_7150_002D_/N_PR_7150_002D_.pdf",
        version="D",
        category="software_engineering",
        notes="NASA procedural requirements for software engineering.",
    ),
    PublicBible(
        ref_id="FDA-GPSV-2002",
        title="FDA General Principles of Software Validation",
        authority="FDA",
        url="https://www.fda.gov/media/73141/download",
        version="2002",
        category="software_validation",
        notes="Guidance for industry and FDA staff on software validation.",
    ),
    PublicBible(
        ref_id="FDA-CSA-2023",
        title="FDA Computer Software Assurance for Production and Quality System Software",
        authority="FDA",
        url="https://www.fda.gov/media/166496/download",
        version="2023-draft",
        category="software_assurance",
        notes="Draft guidance on risk-based computer software assurance.",
    ),
]


@dataclass
class SnapshotResult:
    """Result of snapshotting a single bible."""
    ref_id: str
    success: bool
    sha256: str = ""
    size_bytes: int = 0
    error: str = ""
    sidecar_path: str = ""


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Snapshot public reference bibles and generate sidecar YAML."
    )
    parser.add_argument("--output", required=True, help="Output directory for sidecar YAML files")
    parser.add_argument("--dry-run", action="store_true", help="Show plan without downloading")
    parser.add_argument("--timeout", type=int, default=30, help="HTTP timeout in seconds")
    parser.add_argument("--ref", action="append", help="Limit to specific ref IDs (may repeat)")
    return parser.parse_args(argv)


def compute_sha256_from_url(url: str, timeout: int = 30) -> tuple[str, int]:
    """Download URL content, compute SHA-256 hash and size WITHOUT storing content.

    Returns (sha256_hex, size_bytes). Never writes full content to disk.
    """
    h = hashlib.sha256()
    size = 0
    req = urllib.request.Request(url, headers={"User-Agent": "Nomos-BibleSnapshot/0.1"})

    with urllib.request.urlopen(req, timeout=timeout) as resp:
        while True:
            chunk = resp.read(65536)
            if not chunk:
                break
            h.update(chunk)
            size += len(chunk)

    return "sha256:" + h.hexdigest(), size


def generate_sidecar(bible: PublicBible, sha256: str, size_bytes: int) -> str:
    """Generate sidecar YAML content for a bible snapshot."""
    now = datetime.now(timezone.utc).isoformat()
    lines = [
        f'ref_id: "{bible.ref_id}"',
        f'title: "{bible.title}"',
        f'authority: "{bible.authority}"',
        f'version: "{bible.version}"',
        f'category: "{bible.category}"',
        f'format: "{bible.format}"',
        f'url: "{bible.url}"',
        f'snapshot_hash: "{sha256}"',
        f'snapshot_size_bytes: {size_bytes}',
        f'snapshot_at: "{now}"',
        f'stored_in_git: false',
        f'storage_policy: "hash_only_no_full_text"',
    ]
    if bible.notes:
        lines.append(f'notes: "{bible.notes}"')
    return "\n".join(lines) + "\n"


def write_sidecar(bible: PublicBible, sha256: str, size_bytes: int, output_dir: Path) -> str:
    """Write sidecar YAML file and return its path."""
    output_dir.mkdir(parents=True, exist_ok=True)
    filename = f"{bible.ref_id.lower()}.sidecar.yaml"
    path = output_dir / filename
    content = generate_sidecar(bible, sha256, size_bytes)
    path.write_text(content, encoding="utf-8")
    return str(path)


def snapshot_bible(bible: PublicBible, output_dir: Path, timeout: int) -> SnapshotResult:
    """Download, hash, and create sidecar for a single bible."""
    try:
        sha256, size = compute_sha256_from_url(bible.url, timeout)
        sidecar_path = write_sidecar(bible, sha256, size, output_dir)
        return SnapshotResult(
            ref_id=bible.ref_id,
            success=True,
            sha256=sha256,
            size_bytes=size,
            sidecar_path=sidecar_path,
        )
    except (urllib.error.HTTPError, urllib.error.URLError, OSError) as e:
        return SnapshotResult(
            ref_id=bible.ref_id,
            success=False,
            error=str(e),
        )


def get_bibles(ref_filter: list[str] | None) -> list[PublicBible]:
    """Filter bibles by ref IDs if specified."""
    if not ref_filter:
        return PUBLIC_BIBLES
    filter_set = {r.upper() for r in ref_filter}
    return [b for b in PUBLIC_BIBLES if b.ref_id.upper() in filter_set]


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    output_dir = Path(args.output)
    bibles = get_bibles(args.ref)

    print(f"Public bible snapshot: {len(bibles)} reference(s)")
    print(f"  Output: {output_dir}")

    if args.dry_run:
        print("  [DRY RUN] Plan:")
        for bible in bibles:
            print(f"    {bible.ref_id}: {bible.title}")
            print(f"      URL: {bible.url}")
            print(f"      -> {bible.ref_id.lower()}.sidecar.yaml")
        return 0

    results: list[SnapshotResult] = []
    for bible in bibles:
        print(f"  Snapshotting {bible.ref_id}...", end=" ", flush=True)
        result = snapshot_bible(bible, output_dir, args.timeout)
        results.append(result)
        if result.success:
            print(f"OK ({result.size_bytes} bytes, {result.sha256[:20]}...)")
        else:
            print(f"FAILED: {result.error}")

    succeeded = sum(1 for r in results if r.success)
    failed = sum(1 for r in results if not r.success)
    print(f"\nDone: {succeeded} succeeded, {failed} failed")

    if failed > 0:
        print("WARNING: Some downloads failed. Re-run or check URLs.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
