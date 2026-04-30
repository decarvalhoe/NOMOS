#!/usr/bin/env python3

from __future__ import annotations

import json
import subprocess
import sys
from dataclasses import dataclass


REPO = "RBOKproject/Nomos"


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


def run(*args: str, capture_output: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, check=True, text=True, capture_output=capture_output)


def gh_json(*args: str):
    result = run("gh", *args)
    return json.loads(result.stdout or "[]")


def ensure_labels() -> None:
    existing = {item["name"] for item in gh_json("label", "list", "--repo", REPO, "--limit", "200", "--json", "name")}
    for label in LABELS:
        if label.name in existing:
            continue
        run(
            "gh", "label", "create",
            label.name,
            "--repo", REPO,
            "--color", label.color,
            "--description", label.description,
            capture_output=False,
        )


def ensure_milestones() -> dict[str, int]:
    existing = {
        item["title"]: item["number"]
        for item in gh_json("api", f"repos/{REPO}/milestones?state=all&per_page=100")
    }
    for title, description in MILESTONES:
        if title in existing:
            continue
        run(
            "gh", "api", f"repos/{REPO}/milestones",
            "--method", "POST",
            "-f", f"title={title}",
            "-f", f"description={description}",
            capture_output=False,
        )
    return {
        item["title"]: item["number"]
        for item in gh_json("api", f"repos/{REPO}/milestones?state=all&per_page=100")
    }


def sync_epics(milestones: dict[str, int]) -> None:
    for issue_number, area_label in EPIC_TO_LABEL.items():
        run(
            "gh", "issue", "edit", issue_number,
            "--repo", REPO,
            "--add-label", "type:epic",
            "--add-label", "status:seeded",
            "--add-label", area_label,
            "--milestone", "v1.0 Productized Platform",
            capture_output=False,
        )


def sync_backlog_issues(milestones: dict[str, int]) -> None:
    for issue_number, (milestone, labels) in ISSUE_CONFIG.items():
        cmd = ["gh", "issue", "edit", str(issue_number), "--repo", REPO, "--milestone", milestone]
        for label in labels:
            cmd.extend(["--add-label", label])
        if issue_number in STARTED_ISSUES:
            cmd.extend(["--add-label", "status:in-progress"])
        run(*cmd, capture_output=False)


def main() -> int:
    ensure_labels()
    milestones = ensure_milestones()
    sync_epics(milestones)
    sync_backlog_issues(milestones)
    print("GitHub taxonomy sync completed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
