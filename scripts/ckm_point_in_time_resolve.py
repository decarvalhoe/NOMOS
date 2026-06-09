#!/usr/bin/env python3
"""Resolve the regulatory atom expression in force at a project date."""

from __future__ import annotations

import argparse
import json
from datetime import date
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    raise SystemExit("PyYAML is required for point-in-time resolution.") from exc


def parse_day(value: str) -> date:
    return date.fromisoformat(value)


def load_atoms(path: Path) -> list[dict[str, Any]]:
    doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    atoms = doc.get("atoms")
    return atoms if isinstance(atoms, list) else []


def temporal(atom: dict[str, Any]) -> dict[str, Any]:
    metadata = atom.get("metadata") if isinstance(atom.get("metadata"), dict) else {}
    value = metadata.get("temporal") if isinstance(metadata.get("temporal"), dict) else {}
    return value


def in_force(meta: dict[str, Any], as_of: date) -> bool:
    effective_from = parse_day(str(meta["effective_from"]))
    effective_to = meta.get("effective_to")
    if as_of < effective_from:
        return False
    if effective_to and as_of > parse_day(str(effective_to)):
        return False
    return True


def resolve(atoms: list[dict[str, Any]], work_id: str, as_of: date) -> dict[str, Any]:
    candidates: list[tuple[date, dict[str, Any], dict[str, Any]]] = []
    for atom in atoms:
        if not isinstance(atom, dict):
            continue
        meta = temporal(atom)
        if meta.get("work_id") != work_id:
            continue
        if not meta.get("effective_from"):
            continue
        if in_force(meta, as_of):
            candidates.append((parse_day(str(meta["effective_from"])), atom, meta))

    if not candidates:
        return {
            "status": "not_in_force",
            "work_id": work_id,
            "as_of": as_of.isoformat(),
            "claim_boundary": "No atom version is in force for the requested date; Nomos refuses to cite a stale or future expression.",
        }

    _, atom, meta = sorted(candidates, key=lambda item: item[0], reverse=True)[0]
    return {
        "status": "resolved",
        "work_id": work_id,
        "as_of": as_of.isoformat(),
        "selected_atom_id": atom.get("atom_id"),
        "expression_id": meta.get("expression_id"),
        "effective_from": meta.get("effective_from"),
        "effective_to": meta.get("effective_to"),
        "source_hash": atom.get("source_span", {}).get("hash"),
        "claim_boundary": "Point-in-time resolver selects the recorded expression in force; it does not prove legal sufficiency.",
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Resolve point-in-time atom version.")
    parser.add_argument("--atoms", required=True, help="YAML atom set.")
    parser.add_argument("--work-id", required=True, help="FRBR/ELI work identifier.")
    parser.add_argument("--as-of", required=True, help="Project date YYYY-MM-DD.")
    args = parser.parse_args()

    report = resolve(load_atoms(Path(args.atoms)), args.work_id, parse_day(args.as_of))
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if report["status"] == "resolved" else 1


if __name__ == "__main__":
    raise SystemExit(main())
