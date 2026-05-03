#!/usr/bin/env python3
"""regulated_audit_log_export.py — Export GitHub audit log events.

Usage:
    python3 scripts/regulated_audit_log_export.py \
        --org RBOKproject \
        --output .regulated-audit-logs/ \
        [--token $GITHUB_TOKEN] \
        [--since 2026-04-26] \
        [--until 2026-05-03] \
        [--dry-run]

Exports GitHub organization audit log events to JSON files with
SHA-256 integrity hashes. Designed for regulated evidence retention.
"""

import argparse
import hashlib
import json
import os
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Export GitHub audit log events for regulated retention."
    )
    parser.add_argument("--org", required=True, help="GitHub organization name")
    parser.add_argument("--output", required=True, help="Output directory for exports")
    parser.add_argument("--token", default=os.environ.get("GITHUB_TOKEN", ""), help="GitHub token")
    parser.add_argument("--since", help="Start date (YYYY-MM-DD), default: 7 days ago")
    parser.add_argument("--until", help="End date (YYYY-MM-DD), default: today")
    parser.add_argument("--dry-run", action="store_true", help="Print plan without exporting")
    parser.add_argument("--api-url", default="https://api.github.com", help="GitHub API base URL")
    return parser.parse_args(argv)


def resolve_date_range(since: str | None, until: str | None) -> tuple[str, str]:
    """Resolve date range, defaulting to last 7 days."""
    now = datetime.now(timezone.utc)
    if until:
        end = until
    else:
        end = now.strftime("%Y-%m-%d")
    if since:
        start = since
    else:
        start = (now - timedelta(days=7)).strftime("%Y-%m-%d")
    return start, end


def compute_sha256(data: bytes) -> str:
    """Compute SHA-256 hash of bytes."""
    return "sha256:" + hashlib.sha256(data).hexdigest()


def fetch_audit_log(org: str, token: str, api_url: str, since: str, until: str) -> list[dict[str, Any]]:
    """Fetch audit log events from GitHub API.

    Returns a list of audit log entries. Handles pagination via cursors.
    Requires organization admin or audit log read permissions.
    """
    try:
        import urllib.request
        import urllib.error
    except ImportError:
        print("ERROR: urllib required for API calls", file=sys.stderr)
        return []

    events: list[dict[str, Any]] = []
    url = f"{api_url}/orgs/{org}/audit-log?include=all&per_page=100&phrase=created:{since}..{until}"
    headers = {
        "Accept": "application/json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
    }

    while url:
        req = urllib.request.Request(url, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                page_data = json.loads(resp.read().decode("utf-8"))
                if isinstance(page_data, list):
                    events.extend(page_data)
                else:
                    break

                # Check for pagination via Link header
                link_header = resp.headers.get("Link", "")
                url = extract_next_link(link_header)
        except urllib.error.HTTPError as e:
            print(f"ERROR: GitHub API returned {e.code}: {e.reason}", file=sys.stderr)
            if e.code == 403:
                print("HINT: Token may lack audit_log:read scope or org admin access.", file=sys.stderr)
            break
        except urllib.error.URLError as e:
            print(f"ERROR: Network error: {e.reason}", file=sys.stderr)
            break

    return events


def extract_next_link(link_header: str) -> str | None:
    """Extract 'next' URL from GitHub Link header."""
    if not link_header:
        return None
    for part in link_header.split(","):
        if 'rel="next"' in part:
            url = part.split(";")[0].strip().strip("<>")
            return url
    return None


def write_export(events: list[dict[str, Any]], output_dir: Path, since: str, until: str) -> dict[str, Any]:
    """Write audit log export with integrity hash and manifest."""
    output_dir.mkdir(parents=True, exist_ok=True)

    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    export_filename = f"audit-log-{since}-to-{until}-{timestamp}.json"
    export_path = output_dir / export_filename

    export_data = {
        "export_version": "0.1.0",
        "exported_at": datetime.now(timezone.utc).isoformat(),
        "date_range": {"since": since, "until": until},
        "event_count": len(events),
        "events": events,
    }

    raw_bytes = json.dumps(export_data, indent=2, ensure_ascii=False).encode("utf-8")
    export_hash = compute_sha256(raw_bytes)

    export_path.write_bytes(raw_bytes)

    # Write manifest entry
    manifest_path = output_dir / "export-manifest.json"
    manifest = load_manifest(manifest_path)
    manifest["exports"].append({
        "filename": export_filename,
        "date_range": {"since": since, "until": until},
        "event_count": len(events),
        "hash": export_hash,
        "exported_at": export_data["exported_at"],
    })
    manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False), encoding="utf-8")

    return {
        "filename": export_filename,
        "path": str(export_path),
        "event_count": len(events),
        "hash": export_hash,
    }


def load_manifest(path: Path) -> dict[str, Any]:
    """Load or initialize the export manifest."""
    if path.exists():
        try:
            return json.loads(path.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            pass
    return {
        "schema_version": "0.1.0",
        "policy_ref": "RCP-004",
        "exports": [],
    }


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    since, until = resolve_date_range(args.since, args.until)
    output_dir = Path(args.output)

    print(f"Audit log export: {args.org}")
    print(f"  Date range: {since} to {until}")
    print(f"  Output: {output_dir}")

    if args.dry_run:
        print("  [DRY RUN] No export performed.")
        return 0

    if not args.token:
        print("ERROR: --token or GITHUB_TOKEN environment variable is required.", file=sys.stderr)
        return 1

    events = fetch_audit_log(args.org, args.token, args.api_url, since, until)
    print(f"  Events fetched: {len(events)}")

    result = write_export(events, output_dir, since, until)
    print(f"  Written: {result['filename']}")
    print(f"  Hash: {result['hash']}")
    print(f"  Events: {result['event_count']}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
