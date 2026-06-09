from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import yaml


def load_yaml(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        loaded = yaml.safe_load(handle)
    if not isinstance(loaded, dict):
        raise SystemExit(f"{path} must contain a YAML object")
    return loaded


def as_list(value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return value
    return [value]


def facets_for(candidate: dict[str, Any]) -> dict[str, Any]:
    metadata = candidate.get("metadata") or {}
    facets = metadata.get("facets") or {}
    if not isinstance(facets, dict):
        return {}
    return facets


def selection_matches(facets: dict[str, Any], selection: dict[str, Any]) -> bool:
    for axis, expected in selection.items():
        actual_values = set(as_list(facets.get(axis)))
        expected_values = set(as_list(expected))
        if not actual_values.intersection(expected_values):
            return False
    return True


def first_matching_axis(facets: dict[str, Any], selections: list[dict[str, Any]]) -> str | None:
    for selection in selections:
        if selection_matches(facets, selection):
            return next(iter(selection.keys()), None)
    return None


def find_lens(bundle: dict[str, Any], preset_id: str) -> tuple[dict[str, Any], dict[str, Any]]:
    presets = {preset["id"]: preset for preset in bundle.get("presets", [])}
    preset = presets.get(preset_id)
    if preset is None:
        raise SystemExit(f"unknown preset: {preset_id}")
    lenses = {lens["id"]: lens for lens in bundle.get("lenses", [])}
    lens = lenses.get(preset["lens_id"])
    if lens is None:
        raise SystemExit(f"preset {preset_id} references unknown lens: {preset['lens_id']}")
    return preset, lens


def include_candidate(candidate: dict[str, Any], lens: dict[str, Any]) -> tuple[bool, str | None]:
    facets = facets_for(candidate)

    excluded_axis = first_matching_axis(facets, lens.get("exclude", {}).get("any_of", []))
    if excluded_axis:
        return False, f"excluded_by_facets.{excluded_axis}"

    include = lens.get("include", {})
    all_of = include.get("all_of", [])
    if all_of and not all(selection_matches(facets, selection) for selection in all_of):
        return False, "not_selected_by_lens"

    any_of = include.get("any_of", [])
    if any_of and first_matching_axis(facets, any_of) is None:
        return False, "not_selected_by_lens"

    none_of = include.get("none_of", [])
    if none_of and first_matching_axis(facets, none_of) is not None:
        return False, "not_selected_by_lens"

    return True, None


def filter_candidates(
    candidates: list[dict[str, Any]],
    lens_bundle: dict[str, Any] | None,
    preset_id: str | None,
) -> dict[str, Any]:
    if lens_bundle is None:
        return {
            "mode": "no_lens",
            "stage": "base_filter",
            "included_ids": [candidate["id"] for candidate in candidates],
            "excluded": [],
        }

    if not preset_id:
        raise SystemExit("--preset is required when --lens is provided")
    preset, lens = find_lens(lens_bundle, preset_id)

    included: list[str] = []
    excluded: list[dict[str, str]] = []
    for candidate in candidates:
        keep, reason = include_candidate(candidate, lens)
        if keep:
            included.append(candidate["id"])
        else:
            excluded.append({"id": candidate["id"], "reason": reason or "not_selected_by_lens"})

    return {
        "mode": "lens",
        "stage": "base_filter",
        "preset": preset["id"],
        "lens_id": lens["id"],
        "included_ids": included,
        "excluded": excluded,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidates", required=True, type=Path)
    parser.add_argument("--lens", type=Path)
    parser.add_argument("--preset")
    args = parser.parse_args()

    candidate_doc = load_yaml(args.candidates)
    candidates = candidate_doc.get("candidates", [])
    if not isinstance(candidates, list):
        raise SystemExit("candidates must be a list")

    lens_bundle = load_yaml(args.lens) if args.lens else None
    result = filter_candidates(candidates, lens_bundle, args.preset)
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
