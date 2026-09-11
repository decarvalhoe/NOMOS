#!/usr/bin/env python3
"""Seed the tracker taxonomy (labels, milestones, issue assignments) of the Nomos backlog.

Every write goes through the forge provider (`scripts/forge_provider.py`,
selected by `NOMOS_FORGE_PROVIDER` / `NOMOS_FORGE_URL` /
`NOMOS_FORGE_TOKEN_FILE`). The repository comes from `--repo` or
`NOMOS_FORGE_REPO`. A missing configuration is an error (docs/43 §2.8); a
forge that does not support labels or milestones refuses explicitly.
"""

from __future__ import annotations

import argparse
import sys
from dataclasses import dataclass
from pathlib import Path

# Le module frère vit dans scripts/ ; le script peut être chargé depuis
# ailleurs (tests, importlib), d'où l'ajout explicite au chemin.
_SCRIPTS_DIR = str(Path(__file__).resolve().parent)
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from forge_provider import ForgeError, Provider, provider_from_env, repo_from_env  # noqa: E402


DEFAULT_REPO = "RBOKproject/Nomos"
EPIC_MILESTONE = "v1.0 Productized Platform"


@dataclass(frozen=True)
class Label:
    name: str
    color: str
    description: str


LABELS = [
    Label("type:epic", "5319E7", "Cross-cutting epic issue"),
    Label("type:backlog", "0E8A16", "Backlog implementation issue"),
    Label("status:in-progress", "FBCA04", "Work has started locally or on a branch"),
    Label("status:seeded", "BFDADC", "Backlog seeded from product roadmap"),
    Label("area:foundation", "1D76DB", "Repo structure, conventions, versioning"),
    Label("area:spec", "0052CC", "Schemas, manifests, spec model"),
    Label("area:cli", "0E8A16", "CLI core and command behavior"),
    Label("area:admission", "5319E7", "Project admission and diagnosis"),
    Label("area:adapters", "C2E0C6", "Stack adapters and parsing"),
    Label("area:checks", "D4C5F9", "Canonical validation and reporting"),
    Label("area:product-check", "F9D0C4", "Static product anti-bypass checks"),
    Label("area:brownfield", "F9D0C4", "Legacy migration and partial mode"),
    Label("area:cicd", "BFD4F2", "CI/CD integration and policy gates"),
    Label("area:attestations", "B60205", "Provenance, SBOM, signed evidence"),
    Label("area:control-plane", "006B75", "Portfolio supervision and evidence storage"),
]


MILESTONES = [
    ("v0.1 Core Spec", "Universal spec and schema baseline"),
    ("v0.2 CLI Minimal", "Minimal executable Nomos CLI"),
    ("v0.3 Admission Engine", "Diagnose and admit workflows"),
    ("v0.4 Adapters v1", "Polyglot detection and first adapters"),
    ("v0.5 Canonical Checks", "Manifest, matrix, contracts and reports"),
    ("v0.6 Brownfield Migration Pack", "Partial mode and migration tooling"),
    ("v0.7 CI-CD And Policy", "Pipeline gates and reusable CI integration"),
    ("v0.8 Provenance And Attestations", "SLSA, in-toto, signatures, SBOM"),
    ("v0.9 Control Plane", "Portfolio-level registry and dashboard"),
    ("v1.0 Productized Platform", "Stable platform release"),
]


EPIC_TO_LABEL = {
    "1": "area:foundation",
    "2": "area:spec",
    "3": "area:cli",
    "4": "area:admission",
    "5": "area:adapters",
    "6": "area:checks",
    "7": "area:product-check",
    "8": "area:brownfield",
    "9": "area:cicd",
    "10": "area:control-plane",
}


ISSUE_CONFIG = {
    11: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:foundation"]),
    12: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:foundation"]),
    13: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:spec"]),
    14: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:spec"]),
    15: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:spec"]),
    16: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:spec"]),
    17: ("v0.1 Core Spec", ["type:backlog", "status:seeded", "area:spec"]),
    18: ("v0.2 CLI Minimal", ["type:backlog", "status:seeded", "area:cli"]),
    19: ("v0.2 CLI Minimal", ["type:backlog", "status:seeded", "area:cli"]),
    20: ("v0.2 CLI Minimal", ["type:backlog", "status:seeded", "area:cli"]),
    21: ("v0.2 CLI Minimal", ["type:backlog", "status:seeded", "area:cli"]),
    22: ("v0.3 Admission Engine", ["type:backlog", "status:seeded", "area:admission"]),
    23: ("v0.3 Admission Engine", ["type:backlog", "status:seeded", "area:admission"]),
    24: ("v0.3 Admission Engine", ["type:backlog", "status:seeded", "area:admission"]),
    25: ("v0.4 Adapters v1", ["type:backlog", "status:seeded", "area:adapters"]),
    26: ("v0.4 Adapters v1", ["type:backlog", "status:seeded", "area:adapters"]),
    27: ("v0.4 Adapters v1", ["type:backlog", "status:seeded", "area:adapters"]),
    28: ("v0.4 Adapters v1", ["type:backlog", "status:seeded", "area:adapters"]),
    29: ("v0.4 Adapters v1", ["type:backlog", "status:seeded", "area:adapters"]),
    30: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:checks"]),
    31: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:checks"]),
    32: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:checks"]),
    33: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:checks"]),
    34: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:product-check"]),
    35: ("v0.5 Canonical Checks", ["type:backlog", "status:seeded", "area:product-check"]),
    36: ("v0.6 Brownfield Migration Pack", ["type:backlog", "status:seeded", "area:brownfield"]),
    37: ("v0.6 Brownfield Migration Pack", ["type:backlog", "status:seeded", "area:brownfield"]),
    38: ("v0.6 Brownfield Migration Pack", ["type:backlog", "status:seeded", "area:brownfield"]),
    39: ("v0.7 CI-CD And Policy", ["type:backlog", "status:seeded", "area:cicd"]),
    40: ("v0.7 CI-CD And Policy", ["type:backlog", "status:seeded", "area:cicd"]),
    41: ("v0.8 Provenance And Attestations", ["type:backlog", "status:seeded", "area:attestations"]),
    42: ("v0.8 Provenance And Attestations", ["type:backlog", "status:seeded", "area:attestations"]),
    43: ("v0.9 Control Plane", ["type:backlog", "status:seeded", "area:control-plane"]),
    44: ("v0.9 Control Plane", ["type:backlog", "status:seeded", "area:control-plane"]),
    45: ("v0.9 Control Plane", ["type:backlog", "status:seeded", "area:control-plane"]),
}


STARTED_ISSUES = {11, 12, 13, 14, 15, 18}


def ensure_labels(repo: str, provider: Provider) -> None:
    """Create the missing labels; realign colour/description of the existing ones."""
    existing = {item["name"]: item for item in provider.list_labels(repo)}
    for label in LABELS:
        current = existing.get(label.name)
        if current is None:
            provider.create_label(repo, label.name, label.color, label.description)
            print(f"LABEL created {label.name}")
            continue
        if current["color"] != label.color.lower() or current["description"] != label.description:
            provider.update_label(repo, label.name, color=label.color, description=label.description)
            print(f"LABEL updated {label.name}")


def ensure_milestones(repo: str, provider: Provider) -> dict[str, int]:
    """Create the missing milestones; return title → identifier for assignments."""
    existing = {item["title"]: item["id"] for item in provider.list_milestones(repo, state="all")}
    for title, description in MILESTONES:
        if title in existing:
            continue
        created = provider.create_milestone(repo, title, description)
        existing[title] = created["id"]
        print(f"MILESTONE created {title}")
    return existing


def _milestone_id(milestones: dict[str, int], title: str) -> int:
    if title not in milestones:
        # Un jalon inconnu est une erreur nommée, pas une affectation ignorée.
        raise ForgeError(f"milestone {title!r} does not exist; run ensure_milestones first")
    return int(milestones[title])


def sync_epics(repo: str, milestones: dict[str, int], provider: Provider) -> None:
    for issue_number, area_label in EPIC_TO_LABEL.items():
        provider.edit_issue(
            repo,
            int(issue_number),
            add_labels=["type:epic", "status:seeded", area_label],
            milestone=_milestone_id(milestones, EPIC_MILESTONE),
        )


def sync_backlog_issues(repo: str, milestones: dict[str, int], provider: Provider) -> None:
    for issue_number, (milestone, labels) in ISSUE_CONFIG.items():
        add_labels = list(labels)
        if issue_number in STARTED_ISSUES:
            add_labels.append("status:in-progress")
        provider.edit_issue(
            repo,
            int(issue_number),
            add_labels=add_labels,
            milestone=_milestone_id(milestones, milestone),
        )


def sync_taxonomy(repo: str, provider: Provider) -> None:
    ensure_labels(repo, provider)
    milestones = ensure_milestones(repo, provider)
    sync_epics(repo, milestones, provider)
    sync_backlog_issues(repo, milestones, provider)


def main(argv: list[str] | None = None, provider: Provider | None = None) -> int:
    parser = argparse.ArgumentParser(description="Seed the tracker taxonomy of the Nomos backlog.")
    parser.add_argument(
        "--repo",
        default="",
        help=f"Repository as owner/name (default: NOMOS_FORGE_REPO, then {DEFAULT_REPO})",
    )
    args = parser.parse_args(argv)

    try:
        try:
            repo = repo_from_env(explicit=args.repo)
        except ForgeError:
            repo = DEFAULT_REPO
        forge = provider if provider is not None else provider_from_env()
        sync_taxonomy(repo, forge)
    except ForgeError as exc:
        # Configuration absente ou refus de la forge : dit, jamais tu.
        print(f"ERROR: {exc}", file=sys.stderr)
        return 2
    print(f"Taxonomy sync completed on {repo} via {forge.name}.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
