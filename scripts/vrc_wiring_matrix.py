#!/usr/bin/env python3
"""VRC-00 — generated wiring matrix: capability statuses computed from the tree.

Doctrine (docs/43-development-doctrine.md §2.3, docs/45-vision-reality-closure-plan.md
§8 G3, docs/46-vrc-epic-issue-list.md VRC-00): a pipeline result is CALCULATED, never
declared. The audited failure class behind this guard is "implemented but never
wired" (#539/#540/#543): ``nomos atomize`` existed but was not registered in the CLI
command map; ``ApplyLens`` had zero production callers; a connector manifest carried
synthetic hashes. Those gaps were found by a *second* audit pass — this guard catches
them at PR time instead.

Model:

* ``scripts/vrc_wiring_matrix_registry.json`` declares, per capability, WHERE to look
  (anchor files + required tokens) and the EXPECTED status. It never declares the
  computed status itself.
* This script COMPUTES each capability status from the tree:
    - ``real``    — engine anchors + declared production caller(s) + adversarial
                    test(s) all present (and declared CI-gate anchors, when any);
    - ``partial`` — engine present but a production caller / adversarial test /
                    declared CI gate is missing (the #540 class);
    - ``sidecar`` — no Go engine; the capability lives in Python sidecars / CUE specs
                    only (doctrine: "un schéma CUE + script Python sidecar = PARTIAL,
                    pas done");
    - ``stub``    — an explicit honest placeholder (e.g. ``PlaceholderAdapter``);
    - ``absent``  — nothing found, or declared anchors rotted (loud on purpose).
* ``must_be_absent`` probes guard the OTHER direction: when engine markers appear in
  the tree for a capability the registry still lists as sidecar/stub/absent, the
  matrix turns red until the registry entry is flipped (with its new anchors) in the
  same PR. Promotions cannot land silently.
* Generic checks (registry-independent): every ``*Command`` function defined in
  ``cli/internal/app`` must be registered in the ``app.go`` command map or called
  from another production file, and every registered command must be advertised in
  the help text. This is the exact #543 ``atomize`` failure, generalized.

Any mismatch between computed and expected — in either direction — exits 1: the
registry must track the truth, and the truth must not outrun the registry.

Outputs are deterministic (sorted, relative POSIX paths, no timestamps) and written
to ``.vrc-wiring-matrix/wiring-matrix.json`` and ``wiring-matrix.md``. Both files are
GENERATED; editing them by hand is forbidden (RBOK dossier rule: "editing the
'Actual' column without a corresponding evidence pack is forbidden"). CI re-runs the
script and fails on ``git diff`` drift.

Claim boundary: the matrix proves WIRING PRESENCE (files, symbols, callers, tests,
gates), not functional correctness, and certifies nothing by itself.

Run (exit 0 = matrix matches registry and generic checks pass, 1 otherwise):

    python3 scripts/vrc_wiring_matrix.py --root .

Exercised by ``tests/test_vrc_wiring_matrix.py`` (adversarial: a forged ``real``
expectation without an engine turns red; unwiring a production caller turns red;
engine markers appearing under a ``sidecar`` entry turn red; an unregistered command
with a doc-comment decoy turns red).
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

REGISTRY_DEFAULT = "scripts/vrc_wiring_matrix_registry.json"
OUT_DIR_DEFAULT = ".vrc-wiring-matrix"
MATRIX_SCHEMA_VERSION = "vrc-wiring-matrix-v1"
STATUSES = ("real", "partial", "sidecar", "stub", "absent")

GENERATED_BANNER = (
    "GENERATED FILE — do not edit by hand. "
    "Regenerate with: python3 scripts/vrc_wiring_matrix.py --root ."
)
CLAIM_BOUNDARY = (
    "Wiring presence computed from tree anchors; not a proof of functional "
    "correctness; no compliance certification."
)

# func XxxCommand(args []string, stdout io.Writer, stderr io.Writer) int
_CMD_FUNC = re.compile(
    r"func\s+([A-Za-z]\w*Command)\s*\(\s*\w+\s+\[\]string\s*,\s*"
    r"\w+\s+io\.Writer\s*,\s*\w+\s+io\.Writer\s*\)\s*int"
)
_MAP_ENTRY = re.compile(r'"([\w-]+)"\s*:\s*([\w.]+)\s*,')


def _read_text(path: Path) -> str | None:
    try:
        return path.read_text(encoding="utf-8")
    except OSError:
        return None


def check_anchor(root: Path, anchor: dict) -> dict:
    """An anchor passes when its file exists and contains every declared token."""
    rel = anchor["path"]
    tokens = anchor.get("contains", [])
    text = _read_text(root / rel)
    if text is None:
        return {"path": rel, "ok": False, "missing": ["<file missing>"] + list(tokens)}
    missing = [t for t in tokens if t not in text]
    return {"path": rel, "ok": not missing, "missing": missing}


def _anchor_group(root: Path, anchors: list[dict]) -> tuple[bool, list[dict]]:
    results = [check_anchor(root, a) for a in anchors]
    return bool(results) and all(r["ok"] for r in results), results


def probe_absent(root: Path, probe: dict) -> list[str]:
    """Return up to 10 ``path:line`` hits for a marker that must NOT exist."""
    base = root / probe["dir"]
    if not base.is_dir():
        return []
    suffixes = tuple(probe.get("suffixes", [".go"]))
    exclude_tests = bool(probe.get("exclude_tests", False))
    flags = re.IGNORECASE if probe.get("ignore_case") else 0
    rx = re.compile(probe["regex"], flags)
    hits: list[str] = []
    for path in sorted(base.rglob("*")):
        if not path.is_file() or path.suffix not in suffixes:
            continue
        if exclude_tests and path.name.endswith("_test.go"):
            continue
        text = _read_text(path)
        if text is None:
            continue
        for lineno, line in enumerate(text.splitlines(), start=1):
            if rx.search(line):
                hits.append(f"{path.relative_to(root).as_posix()}:{lineno}")
                if len(hits) >= 10:
                    return hits
    return hits


def compute_capability(root: Path, cap: dict) -> dict:
    """Compute one capability status from the tree. Never trusts ``expected``."""
    reasons: list[str] = []
    drift: list[str] = []
    for probe in cap.get("must_be_absent", []):
        for hit in probe_absent(root, probe):
            drift.append(
                f"marker {probe['regex']!r} (declared must-be-absent) found at {hit} "
                "— promote the capability: flip this registry entry and add its anchors"
            )

    engine_ok, engine_res = _anchor_group(root, cap.get("engine", []))
    callers_ok, callers_res = _anchor_group(root, cap.get("production_callers", []))
    adv_ok, adv_res = _anchor_group(root, cap.get("adversarial_tests", []))
    gates = cap.get("ci_gates", [])
    gates_ok, gates_res = _anchor_group(root, gates) if gates else (True, [])
    sidecar = cap.get("sidecar", [])
    sidecar_ok, sidecar_res = _anchor_group(root, sidecar) if sidecar else (False, [])
    stubs = cap.get("stub_markers", [])
    stub_ok, stub_res = _anchor_group(root, stubs) if stubs else (False, [])

    if cap.get("engine"):
        if not engine_ok:
            computed = "absent"
            reasons.append("declared engine anchors are missing (anchor rot or removal)")
        else:
            missing: list[str] = []
            if not cap.get("production_callers"):
                missing.append("no production caller declared (the #540 class)")
            elif not callers_ok:
                missing.append("a declared production caller is missing")
            if not cap.get("adversarial_tests"):
                missing.append("no adversarial test declared (doctrine §2.3)")
            elif not adv_ok:
                missing.append("a declared adversarial test is missing")
            if gates and not gates_ok:
                missing.append("a declared CI gate anchor is missing")
            computed = "real" if not missing else "partial"
            reasons.extend(missing)
    elif sidecar:
        if sidecar_ok and gates_ok:
            computed = "sidecar"
            reasons.append(
                "implementation lives in sidecar scripts/specs only "
                "(doctrine: sidecar = PARTIAL, not done)"
            )
        else:
            computed = "absent"
            reasons.append("declared sidecar/gate anchors are missing (anchor rot or removal)")
    elif stubs:
        computed = "stub" if stub_ok else "absent"
        if not stub_ok:
            reasons.append("declared stub markers are missing (anchor rot or removal)")
    else:
        computed = "absent"

    expected = cap.get("expected", "")
    mismatch = computed != expected or bool(drift)
    return {
        "id": cap["id"],
        "title": cap.get("title", cap["id"]),
        "pillar": cap.get("pillar", ""),
        "claim_level": cap.get("claim_level", ""),
        "promotion_issue": cap.get("promotion_issue", ""),
        "expected": expected,
        "computed": computed,
        "mismatch": mismatch,
        "reasons": reasons,
        "drift": drift,
        "checks": {
            "engine": engine_res,
            "production_callers": callers_res,
            "adversarial_tests": adv_res,
            "ci_gates": gates_res,
            "sidecar": sidecar_res,
            "stub_markers": stub_res,
        },
    }


def generic_command_checks(root: Path, allowlist: list[dict] | None = None) -> dict:
    """Every *Command func in cli/internal/app is registered or called; every
    registered command is advertised in the help text (the #543 class).

    ``allowlist`` (registry ``generic_allowlist.command_registration.known_unwired``)
    records ALREADY-KNOWN unwired commands, each bound to a promotion issue. They are
    reported (never hidden) but do not fail CI; any NEW unwired command fails. A
    stale allowlist entry — the command became wired, or no longer exists — fails
    too: the allowlist must track the truth in both directions, like the registry.
    """
    allow = {entry["name"]: entry for entry in (allowlist or [])}
    app_dir = root / "cli" / "internal" / "app"
    app_go_text = _read_text(app_dir / "app.go")
    if app_go_text is None:
        return {
            "name": "command_registration",
            "status": "skipped",
            "detail": "cli/internal/app/app.go not present under this root",
            "failures": [],
            "known_unwired": [],
        }

    start = app_go_text.find("map[string]commandFunc{")
    block = ""
    if start != -1:
        end = app_go_text.find("\n\t}", start)
        block = app_go_text[start : end if end != -1 else len(app_go_text)]
    registered = dict(_MAP_ENTRY.findall(block))
    registered_values = {value.split(".")[-1] for value in registered.values()}

    files: dict[str, str] = {}
    for path in sorted(app_dir.glob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = _read_text(path)
        if text is not None:
            files[path.name] = text

    defined: list[tuple[str, str]] = []
    for fname in sorted(files):
        for match in _CMD_FUNC.finditer(files[fname]):
            defined.append((fname, match.group(1)))

    failures: list[str] = []
    known_unwired: list[str] = []
    unwired_names: set[str] = set()
    for fname, name in defined:
        if name in registered_values:
            continue
        call_rx = re.compile(r"(?<!func )\b" + re.escape(name) + r"\s*\(")
        if any(call_rx.search(text) for text in files.values()):
            continue
        unwired_names.add(name)
        finding = (
            f"{fname}: {name} is implemented but neither registered in the app.go "
            "command map nor called from production code (the #543 'atomize' class)"
        )
        if name in allow:
            entry = allow[name]
            known_unwired.append(
                f"{finding} — known, tracked by {entry.get('issue', '?')}"
                + (f": {entry['note']}" if entry.get("note") else "")
            )
        else:
            failures.append(finding)

    defined_names = {name for _, name in defined}
    for name in sorted(allow):
        if name in unwired_names:
            continue
        if name in defined_names:
            failures.append(
                f"allowlist: {name} is wired now — remove its known_unwired entry "
                "(the allowlist must track the truth)"
            )
        else:
            failures.append(
                f"allowlist: {name} no longer exists — remove its known_unwired entry"
            )

    for key in sorted(registered):
        if f'"  {key}' not in app_go_text:
            failures.append(
                f'app.go: command "{key}" is registered but not advertised in the help text'
            )

    return {
        "name": "command_registration",
        "status": "fail" if failures else "pass",
        "registered_commands": sorted(registered),
        "failures": failures,
        "known_unwired": known_unwired,
    }


def build_matrix(root: Path, registry: dict, registry_rel: str) -> tuple[dict, bool]:
    capabilities = [compute_capability(root, cap) for cap in registry.get("capabilities", [])]
    cmd_allowlist = (
        registry.get("generic_allowlist", {})
        .get("command_registration", {})
        .get("known_unwired", [])
    )
    generic = [generic_command_checks(root, cmd_allowlist)]
    mismatches = [c for c in capabilities if c["mismatch"]]
    generic_failures = [g for g in generic if g.get("status") == "fail"]
    matrix = {
        "schema_version": MATRIX_SCHEMA_VERSION,
        "generated_by": "scripts/vrc_wiring_matrix.py",
        "generated_note": GENERATED_BANNER,
        "registry": registry_rel,
        "registry_schema_version": registry.get("schema_version", ""),
        "claim_boundary": CLAIM_BOUNDARY,
        "capabilities": capabilities,
        "generic_checks": generic,
        "summary": {
            "capabilities": len(capabilities),
            "computed": {status: sum(1 for c in capabilities if c["computed"] == status) for status in STATUSES},
            "mismatches": len(mismatches),
            "generic_check_failures": sum(len(g.get("failures", [])) for g in generic_failures),
            "known_unwired": sum(len(g.get("known_unwired", [])) for g in generic),
        },
    }
    return matrix, bool(mismatches or generic_failures)


def render_markdown(matrix: dict) -> str:
    lines = [
        "# VRC Wiring Matrix",
        "",
        f"> {GENERATED_BANNER}",
        "> Statuses are COMPUTED from tree anchors, never declared. The registry",
        "> (`" + matrix["registry"] + "`) only says where to look and what status is expected;",
        "> any divergence — in either direction — fails CI.",
        f"> Claim boundary: {matrix['claim_boundary']}",
        "",
        "| Capability | Pillar | Expected | Computed | OK | Promotion | Notes |",
        "|---|---|---|---|---|---|---|",
    ]
    for cap in matrix["capabilities"]:
        notes = "; ".join(cap["reasons"] + cap["drift"]) or "—"
        if len(notes) > 220:
            notes = notes[:217] + "..."
        ok = "❌" if cap["mismatch"] else "✅"
        lines.append(
            f"| `{cap['id']}` | {cap['pillar'] or '—'} | {cap['expected']} | "
            f"{cap['computed']} | {ok} | {cap['promotion_issue'] or '—'} | {notes} |"
        )
    lines += ["", "## Generic checks", ""]
    for check in matrix["generic_checks"]:
        lines.append(f"- `{check['name']}`: **{check['status']}**")
        for failure in check.get("failures", []):
            lines.append(f"  - FAIL: {failure}")
        for known in check.get("known_unwired", []):
            lines.append(f"  - known-unwired: {known}")
    summary = matrix["summary"]
    computed = ", ".join(f"{k}={v}" for k, v in summary["computed"].items())
    lines += [
        "",
        "## Summary",
        "",
        f"- capabilities: {summary['capabilities']} ({computed})",
        f"- mismatches: {summary['mismatches']}",
        f"- generic check failures: {summary['generic_check_failures']}",
        f"- known unwired commands (tracked, not hidden): {summary['known_unwired']}",
        "",
    ]
    return "\n".join(lines)


def write_outputs(matrix: dict, out_dir: Path) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    with open(out_dir / "wiring-matrix.json", "w", encoding="utf-8", newline="\n") as fp:
        json.dump(matrix, fp, indent=2, ensure_ascii=False)
        fp.write("\n")
    with open(out_dir / "wiring-matrix.md", "w", encoding="utf-8", newline="\n") as fp:
        fp.write(render_markdown(matrix))


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root to scan.")
    parser.add_argument(
        "--registry",
        default=None,
        help=f"Capability registry path (default: <root>/{REGISTRY_DEFAULT}).",
    )
    parser.add_argument(
        "--out-dir",
        default=None,
        help=f"Output directory (default: <root>/{OUT_DIR_DEFAULT}; pass '' to skip writing).",
    )
    parser.add_argument("--quiet", action="store_true", help="Only print the summary line.")
    args = parser.parse_args(argv)

    root = Path(args.root).resolve()
    registry_path = Path(args.registry) if args.registry else root / REGISTRY_DEFAULT
    try:
        registry = json.loads(registry_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as err:
        print(f"wiring matrix: cannot load registry {registry_path}: {err}", file=sys.stderr)
        return 2

    try:
        registry_rel = registry_path.resolve().relative_to(root).as_posix()
    except ValueError:
        registry_rel = registry_path.as_posix()

    matrix, failed = build_matrix(root, registry, registry_rel)

    if args.out_dir != "":
        out_dir = Path(args.out_dir) if args.out_dir else root / OUT_DIR_DEFAULT
        write_outputs(matrix, out_dir)

    if failed and not args.quiet:
        for cap in matrix["capabilities"]:
            if not cap["mismatch"]:
                continue
            print(
                f"{cap['id']}: expected {cap['expected']!r}, computed {cap['computed']!r}",
                file=sys.stderr,
            )
            for reason in cap["reasons"] + cap["drift"]:
                print(f"    - {reason}", file=sys.stderr)
        for check in matrix["generic_checks"]:
            for failure in check.get("failures", []):
                print(f"{check['name']}: {failure}", file=sys.stderr)

    summary = matrix["summary"]
    verdict = "FAIL" if failed else "OK"
    stream = sys.stderr if failed else sys.stdout
    print(
        f"wiring matrix: {verdict} — {summary['capabilities']} capabilities, "
        f"{summary['mismatches']} mismatch(es), "
        f"{summary['generic_check_failures']} generic failure(s)",
        file=stream,
    )
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
