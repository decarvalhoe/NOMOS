#!/usr/bin/env python3
"""VRC-38 (#575, doc 45 §5 D6) — « reproductible = métrique, pas promesse ».

The core/pack coupling instrument, two modes:

`--manifest <pack.yaml>` — the metric AS THE PACK STANDS: every artifact the
manifest declares must live in a pack-allowed tree (the pack directory or the
corpus-testdata tree). The published metric is the count of core paths the
pack's artifacts require — target and expected value: **0**.

`--changed-files <file> --pack-label-present <true|false>` (one path per line,
e.g. `git diff --name-only --no-renames base...head`) — the always-reporting
CI guard. A PR that touches a domain-pack path MUST carry the exact `pack`
label; the missing-label case is blocking and an ADR cannot waive it. With
the label present, any touched path OUTSIDE the pack-allowed trees is a core
change. Core changes are BLOCKING unless the same PR touches an ADR
(docs/adr/*.md) that justifies them — the escape hatch is a written decision,
never silence. An unlabeled PR that touches no pack path is visibly
not-applicable instead of silently skipped.

Exit 0 when the metric is 0 (or justified); exit 1 otherwise, naming every
core path. The verdict is machine-readable JSON on stdout.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path, PurePosixPath
from typing import Any

import yaml

# A pack may own ONLY these trees (mirrors #PackLocalPath + the golden-corpus
# root rule of specs/domain-pack.cue).
PACK_ALLOWED_PREFIXES = (
    "docs/regulated/domain-packs/",
    "cli/internal/corpus/testdata/",
)
# Only the actual pack tree requires the label. Corpus testdata is shared by
# non-pack work and cannot by itself classify a PR as a pack change.
PACK_LABEL_REQUIRED_PREFIXES = (
    "docs/regulated/domain-packs/",
)
ADR_PREFIX = "docs/adr/"


def is_pack_path(path: str) -> bool:
    p = str(PurePosixPath(path))
    return any(p.startswith(prefix) for prefix in PACK_ALLOWED_PREFIXES)


def manifest_paths(manifest: dict[str, Any]) -> list[str]:
    paths = [
        manifest["profile_ref"],
        manifest["vocabularies"]["file"],
        manifest["source_register"]["file"],
        manifest["ontology"]["file"],
    ]
    paths += [preset["file"] for preset in manifest.get("lens_presets", [])]
    root = manifest["golden_corpus"]["root"].rstrip("/")
    paths.append(root + "/")
    return paths


def check_manifest(manifest_path: Path) -> dict[str, Any]:
    manifest = yaml.safe_load(manifest_path.read_text(encoding="utf-8"))
    declared = manifest_paths(manifest)
    # The profile instance is the DOR-001 contract instance the pack rides —
    # it lives in specs/examples by convention and is shared, not core code.
    core = [
        p for p in declared
        if not is_pack_path(p) and not p.startswith("specs/examples/")
    ]
    return {
        "mode": "manifest",
        "pack_id": manifest.get("pack_id"),
        "declared_paths": declared,
        "core_paths_required": sorted(core),
        "core_changes_required": len(core),
        "target": 0,
        "pass": len(core) == 0,
    }


def check_changed_files(listing: Path, *, pack_label_present: bool) -> dict[str, Any]:
    changed = [
        str(PurePosixPath(line.strip()))
        for line in listing.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]
    pack_marker_paths = sorted(
        path
        for path in changed
        if any(path.startswith(prefix) for prefix in PACK_LABEL_REQUIRED_PREFIXES)
    )
    missing_pack_label = bool(pack_marker_paths) and not pack_label_present
    core = sorted(p for p in changed if not is_pack_path(p))
    adrs = sorted(p for p in changed if p.startswith(ADR_PREFIX) and p.endswith(".md"))
    # ADRs themselves are the justification channel, not a core change.
    core = [p for p in core if p not in adrs]
    justified = bool(adrs)
    coupling_evaluated = pack_label_present
    coupling_pass = len(core) == 0 or justified

    if missing_pack_label:
        passed = False
        verdict = "pack paths touched WITHOUT required 'pack' label — blocking"
    elif not pack_label_present:
        passed = True
        verdict = "no pack paths touched; pack policy not applicable"
    else:
        passed = coupling_pass
        verdict = (
            "pack-only" if not core
            else ("core changes justified by ADR" if justified
                  else "core changes WITHOUT ADR justification — blocking")
        )
    return {
        "mode": "changed_files",
        "changed": changed,
        "pack_label_present": pack_label_present,
        "pack_marker_paths_touched": pack_marker_paths,
        "missing_pack_label": missing_pack_label,
        "coupling_evaluated": coupling_evaluated,
        "core_paths_touched": core,
        "core_changes": len(core),
        "adr_justifications": adrs,
        "target": 0,
        "pass": passed,
        "verdict": verdict,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--manifest", type=Path, help="pack manifest (metric as the pack stands)")
    group.add_argument("--changed-files", type=Path, help="newline-separated changed paths (CI guard)")
    parser.add_argument(
        "--pack-label-present",
        choices=("true", "false"),
        help="current state of the PR's exact 'pack' label; required with --changed-files",
    )
    args = parser.parse_args()

    if args.manifest:
        if args.pack_label_present is not None:
            parser.error("--pack-label-present is only valid with --changed-files")
        verdict = check_manifest(args.manifest)
    else:
        if args.pack_label_present is None:
            parser.error("--pack-label-present is required with --changed-files")
        verdict = check_changed_files(
            args.changed_files,
            pack_label_present=args.pack_label_present == "true",
        )
    print(json.dumps(verdict, indent=2, sort_keys=True))
    return 0 if verdict["pass"] else 1


if __name__ == "__main__":
    sys.exit(main())
