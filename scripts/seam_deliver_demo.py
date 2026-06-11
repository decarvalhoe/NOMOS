"""W23-3 (#592) — deliver the emitted canonical bundle to the Aedifica demo.

The publisher pushes: this runs inside the NOMOS release workflow with the
demo ops credentials (DEMO_OPS_TOKEN / DEMO_OPS_PROJECT secrets), so no
cross-repo PAT ever exists. Tolerant by design: re-delivering the SAME feed
version (same emitting commit) answers 422 immutable — that is success, the
demo already carries it.
"""
from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request

DEMO_API = os.environ.get("DEMO_API", "https://aedifica-demo.fly.dev/api")


def main(bundle_path: str) -> int:
    token = os.environ["DEMO_OPS_TOKEN"]
    project = os.environ["DEMO_OPS_PROJECT"]
    with open(bundle_path, encoding="utf-8") as handle:
        bundle = json.load(handle)
    if bundle.get("schema_version") != "ckm-bundle-v1":
        print(f"refusing delivery: unexpected schema_version {bundle.get('schema_version')!r}", file=sys.stderr)
        return 1

    req = urllib.request.Request(
        f"{DEMO_API}/projects/{project}/nomos/import",
        data=json.dumps({"bundle": bundle, "activate": True}).encode(),
        method="POST",
    )
    req.add_header("Content-Type", "application/json")
    req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=300) as resp:
            imported = json.load(resp)["imported"]
            print(
                "demo import:",
                imported["packs"], "pack(s) ·",
                imported["chunks"], "chunks ·",
                imported["embedded"], "embedded ·",
                "versions:", imported["versions"],
            )
            return 0
    except urllib.error.HTTPError as exc:
        try:
            detail = json.load(exc).get("detail", {})
        except Exception:
            detail = {}
        if exc.code == 422 and "immutable" in str(detail):
            print("demo already carries this feed version (immutable) — nothing to do.")
            return 0
        print(f"delivery failed: HTTP {exc.code} {detail}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("usage: seam_deliver_demo.py <bundle.json>", file=sys.stderr)
        raise SystemExit(2)
    raise SystemExit(main(sys.argv[1]))
