#!/usr/bin/env python3
"""Generate intended-use-specific IQ/OQ/PQ template packs.

The pack gives regulated customers structured prompts for installation,
operational, and performance qualification evidence. It is a preparation
artifact only and does not validate any deployment.
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for IQ/OQ/PQ template generation.", file=sys.stderr)
    raise SystemExit(2) from exc


CLAIM_BOUNDARY = "IQ/OQ/PQ template preparation only; no validation, compliance, or certification claim."
DEFAULT_DOMAIN_PROFILE = Path("specs/examples/nomos-domain-profile.gxp.valid.yaml")
DEFAULT_OUTPUT = Path(".regulated-evidence-pack/iq-oq-pq-template-pack.json")

DEPLOYMENT_BLUEPRINTS = [
    {
        "deployment_model": "cli-only",
        "description": "Local command-line execution against read-only source corpora.",
        "iq_focus": "local executable baseline, source checkout, tool versions, and read-only input paths",
        "oq_focus": "command behavior, deterministic exits, current gate status, and retained logs",
        "pq_focus": "user-owned corpus journey with generated report, source hashes, and documented deviations",
        "iq_checks": [
            "record executable version and source commit",
            "record operating system, shell, Python, Go, and CUE versions when applicable",
            "record local source paths and read-only guard configuration",
            "record operator or automation identity",
        ],
        "oq_checks": [
            "run required commands with clean inputs and capture exit code",
            "verify invalid input produces a controlled failure",
            "verify stale historical failures remain informational and do not override current command result",
        ],
        "pq_checks": [
            "execute intended-use corpus workflow",
            "verify report paths, hashes, and non-claim boundary",
            "record PASS/FAIL/PENDING result with blocker link when applicable",
        ],
    },
    {
        "deployment_model": "github-workflow",
        "description": "Repository workflow execution with checks, logs, and uploaded evidence artifacts.",
        "iq_focus": "workflow file, protected branch target, runner image, permissions, and artifact retention",
        "oq_focus": "latest run per workflow/check name, stale failure downgrade, newest failure block, and in-progress pending state",
        "pq_focus": "pull-request or scheduled workflow execution with retained evidence artifact",
        "iq_checks": [
            "record workflow path, trigger set, permissions, and runner image",
            "record target branch policy and required checks",
            "record artifact name, retention days, and retrieval procedure",
        ],
        "oq_checks": [
            "latest run per workflow/check name wins",
            "superseded failures are warnings, not blocking failures",
            "newest failure remains red",
            "newest in_progress remains pending/external_wait",
        ],
        "pq_checks": [
            "execute workflow for the intended profile",
            "verify uploaded regulated evidence artifact is retrievable",
            "record workflow URL, commit SHA, and result",
        ],
    },
    {
        "deployment_model": "output-repo",
        "description": "Generated outputs published to a controlled repository or branch.",
        "iq_focus": "output repository, target branch, path allowlist, credentials boundary, and publish mode",
        "oq_focus": "publish dry-run, path guard, anti-loop marker, and PR-only change route",
        "pq_focus": "published artifact availability, trace manifest, and reviewer-ready diff",
        "iq_checks": [
            "record output repository URL and target branch",
            "record allowed output paths and forbidden path patterns",
            "record publication credential boundary without storing secrets",
        ],
        "oq_checks": [
            "verify dry-run plan before publishing",
            "verify path guard rejects traversal or absolute paths",
            "verify direct push is disabled unless explicitly authorized",
        ],
        "pq_checks": [
            "verify published artifact exists at expected path",
            "verify trace manifest links source commit, output commit, and generated files",
            "record review or PR link for human approval",
        ],
    },
    {
        "deployment_model": "control-plane",
        "description": "Portfolio or administrative supervision of multiple projects and evidence streams.",
        "iq_focus": "control-plane service baseline, portfolio bindings, roles, and evidence storage locations",
        "oq_focus": "portfolio supervision, current-state health rollups, stale signal handling, and permission gates",
        "pq_focus": "operator journey through project status, blockers, and evidence retrieval",
        "iq_checks": [
            "record service version, configuration source, and deployment target",
            "record portfolio binding count and readiness status",
            "record role and permission model for supervisory actions",
        ],
        "oq_checks": [
            "verify health rollup uses current-state signals",
            "verify stale failures remain historical telemetry",
            "verify red current gates block unsafe promotion",
        ],
        "pq_checks": [
            "execute operator journey for status review",
            "verify blocker triage and evidence retrieval paths",
            "record screenshots or traces without secrets",
        ],
    },
    {
        "deployment_model": "downstream-rag",
        "description": "Downstream retrieval or answer-generation system consuming generated evidence.",
        "iq_focus": "retrieval corpus baseline, index version, source hashes, model/provider inventory, and refusal policy",
        "oq_focus": "citation integrity, refusal behavior, prompt-injection controls, and answer evidence envelope",
        "pq_focus": "citation-bearing answer journey, denied-answer path, and empty-result path",
        "iq_checks": [
            "record corpus and index version",
            "record model, provider, prompt template, and retrieval settings",
            "record source hash manifest and approved citation policy",
        ],
        "oq_checks": [
            "verify every answer cites retained sources",
            "verify prompt-injection and unsupported-claim refusals",
            "verify answer evidence envelope includes source and artifact hashes",
        ],
        "pq_checks": [
            "execute representative citation answer",
            "execute permission-denied or unsupported-claim path",
            "execute empty retrieval path without hallucinated filler content",
        ],
    },
]


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def resolve(root: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def section(focus: str, checks: list[str], evidence_fields: list[str]) -> dict[str, Any]:
    return {
        "focus": focus,
        "checks": checks,
        "evidence_fields": evidence_fields,
        "approval_state": "template_only_pending_customer_execution",
    }


def build_template(blueprint: dict[str, Any], intended_use: dict[str, Any]) -> dict[str, Any]:
    return {
        "deployment_model": blueprint["deployment_model"],
        "description": blueprint["description"],
        "intended_use": intended_use,
        "claim_boundary": CLAIM_BOUNDARY,
        "iq": section(
            blueprint["iq_focus"],
            blueprint["iq_checks"],
            ["environment", "source_commit", "tool_versions", "operator_or_automation", "evidence_path"],
        ),
        "oq": section(
            blueprint["oq_focus"],
            blueprint["oq_checks"],
            ["test_case", "current_status", "workflow_or_command_url", "result", "deviation_ref"],
        ),
        "pq": section(
            blueprint["pq_focus"],
            blueprint["pq_checks"],
            ["journey", "user_or_profile", "input_dataset", "artifact_ref", "pass_fail_pending"],
        ),
    }


def build_report(root: Path, domain_profile_path: Path) -> dict[str, Any]:
    domain_profile = load_yaml(domain_profile_path)
    intended_use = domain_profile.get("intended_use")
    if not isinstance(intended_use, dict):
        intended_use = {"statement": "Customer-owned intended use must be supplied before execution."}

    templates = [build_template(blueprint, intended_use) for blueprint in DEPLOYMENT_BLUEPRINTS]
    return {
        "schema_version": "0.1.0",
        "status": "generated",
        "generated_at_utc": utc_now(),
        "claim_boundary": CLAIM_BOUNDARY,
        "domain": {
            "profile": str(domain_profile.get("domain_profile", "")).strip(),
            "name": str(domain_profile.get("name", "")).strip(),
            "risk_class": domain_profile.get("risk_class", {}),
        },
        "source_documents": {
            "domain_profile": rel(domain_profile_path, root),
        },
        "summary": {
            "deployment_models": [template["deployment_model"] for template in templates],
            "templates_generated": len(templates),
            "iq_oq_pq_sections_per_template": True,
            "non_claim_boundary_enforced": True,
        },
        "templates": templates,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate intended-use-specific IQ/OQ/PQ templates.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--domain-profile", default=str(DEFAULT_DOMAIN_PROFILE), help="Domain profile YAML path.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="JSON template pack path.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = resolve(root, args.output)
    output.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, resolve(root, args.domain_profile))
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
