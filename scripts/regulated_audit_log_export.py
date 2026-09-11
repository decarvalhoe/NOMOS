#!/usr/bin/env python3
"""regulated_audit_log_export.py — Export organisation audit log events.

Usage:
    python3 scripts/regulated_audit_log_export.py \
        --org RBOKproject \
        --output .regulated-audit-logs/ \
        [--since 2026-04-26] \
        [--until 2026-05-03] \
        [--dry-run]

Exports organisation audit log events to JSON files with SHA-256
integrity hashes. Designed for regulated evidence retention.

The events are read through the forge provider (`scripts/forge_provider.py`,
selected by `NOMOS_FORGE_PROVIDER` / `NOMOS_FORGE_URL` /
`NOMOS_FORGE_TOKEN_FILE`). The organisation audit log is a GitHub concept:
on another forge the provider refuses (`NotSupported`) and the script exits
non-zero with that message — it never writes an empty export as if the log
had been read. `--token` / `--api-url` keep their meaning as an explicit
GitHub configuration that bypasses the environment.
"""

import argparse
import hashlib
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

# Le module frère vit dans scripts/ ; le script peut être chargé depuis
# ailleurs (tests, importlib), d'où l'ajout explicite au chemin.
_SCRIPTS_DIR = str(Path(__file__).resolve().parent)
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from forge_provider import (  # noqa: E402
    GITHUB_DEFAULT_URL,
    ForgeError,
    GitHubProvider,
    Provider,
    provider_from_env,
)


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Export GitHub audit log events for regulated retention."
    )
    parser.add_argument("--org", required=True, help="GitHub organization name")
    parser.add_argument("--output", required=True, help="Output directory for exports")
    parser.add_argument(
        "--token",
        default="",
        help="Explicit GitHub token (with read:audit_log); otherwise the forge provider "
        "is resolved from NOMOS_FORGE_* / GITHUB_TOKEN",
    )
    parser.add_argument("--since", help="Start date (YYYY-MM-DD), default: 7 days ago")
    parser.add_argument("--until", help="End date (YYYY-MM-DD), default: today")
    parser.add_argument("--dry-run", action="store_true", help="Print plan without exporting")
    parser.add_argument("--api-url", default="", help="GitHub API base URL (only with --token)")
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


def resolve_provider(token: str = "", api_url: str = "", provider: Provider | None = None) -> Provider:
    """Injected provider first; then an explicit GitHub token; then the environment.

    Une configuration absente lève `ForgeConfigError` (doctrine §2.8) :
    l'export ne se fait jamais « à vide » en silence.
    """
    if provider is not None:
        return provider
    if token:
        return GitHubProvider(api_url or GITHUB_DEFAULT_URL, token)
    return provider_from_env()


def fetch_audit_log(org: str, since: str, until: str, provider: Provider) -> list[dict[str, Any]]:
    """Fetch audit log events through the provider (pagination handled there).

    Requires organisation admin or audit log read permissions; a refusal
    (HTTP 403, or a forge without an organisation audit log) surfaces as
    `ForgeError` to the caller — it is not swallowed into an empty list.
    """
    return provider.export_org_audit_log(org, since=since, until=until)


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
        except (json.JSONDecodeError, OSError) as exc:
            # An audit export manifest that exists but cannot be read is never
            # reinitialised silently: that would erase the export chain
            # (docs/43 principle 8, the audit-chain lesson of docs/49).
            raise ValueError(
                f"audit export manifest {path} exists but cannot be read ({exc}); "
                "refusing to reinitialise the export chain silently — repair or move the manifest"
            ) from exc
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

    try:
        provider = resolve_provider(args.token, args.api_url)
        events = fetch_audit_log(args.org, since, until, provider)
    except ForgeError as exc:
        print(f"ERROR: audit log export refused: {exc}", file=sys.stderr)
        if getattr(exc, "status", None) == 403:
            print("HINT: Token may lack read:audit_log scope or org admin access.", file=sys.stderr)
        return 1
    print(f"  Events fetched: {len(events)}")

    result = write_export(events, output_dir, since, until)
    print(f"  Written: {result['filename']}")
    print(f"  Hash: {result['hash']}")
    print(f"  Events: {result['event_count']}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
