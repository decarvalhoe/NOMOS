#!/usr/bin/env python3
"""Audit GitHub-hosted QMS controls for regulated-by-design work.

The audit separates repository-file evidence from live forge configuration
evidence. Missing live evidence remains a gap; it is not inferred.

Live reads go through the forge provider (`scripts/forge_provider.py`,
selected by `NOMOS_FORGE_PROVIDER` / `NOMOS_FORGE_URL` /
`NOMOS_FORGE_TOKEN_FILE`; on GitHub Actions the runner's token reaches it
through `GH_TOKEN`). The endpoints audited (rulesets, branch protection,
environments, security features) are GitHub-shaped: on another forge the
provider's refusal or 404 is recorded in `api_detail`, never turned into a
pass. Without `--offline`, a missing provider configuration is an error
(docs/43 §2.8).
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

# Le module frère vit dans scripts/ ; le script peut être chargé depuis
# ailleurs (tests, importlib), d'où l'ajout explicite au chemin.
_SCRIPTS_DIR = str(Path(__file__).resolve().parent)
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from forge_provider import ForgeError, Provider, provider_from_env  # noqa: E402


LIVE_CONTROL_NAMES = [
    "branch_protection",
    "rulesets",
    "protected_environments",
    "required_status_checks",
    "required_reviews",
    "artifact_retention_export",
    "audit_log_export",
    "security_features",
]

REQUIRED_SECURITY_FEATURES = [
    "code_security",
    "dependabot_security_updates",
    "secret_scanning",
    "secret_scanning_non_provider_patterns",
    "secret_scanning_push_protection",
    "secret_scanning_validity_checks",
]


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def present(path: Path, detail: str | None = None) -> dict[str, object]:
    payload: dict[str, object] = {"status": "present"}
    if detail:
        payload["detail"] = detail
    return payload


def missing(path: Path, detail: str | None = None) -> dict[str, object]:
    payload: dict[str, object] = {
        "status": "missing",
        "severity": "major",
        "path": path.as_posix(),
    }
    if detail:
        payload["detail"] = detail
    return payload


def requires_live(detail: str) -> dict[str, object]:
    return {
        "status": "requires_live_evidence",
        "severity": "major",
        "detail": detail,
    }


def codeowners_check(path: Path) -> dict[str, object]:
    if not path.is_file():
        return missing(path)
    active_lines = [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    if not active_lines:
        return {
            "status": "requires_human_review",
            "severity": "major",
            "path": path.as_posix(),
            "detail": "CODEOWNERS exists but has no active owner rule.",
        }
    return present(path, f"{len(active_lines)} active owner rule(s)")


def local_checks(root: Path) -> dict[str, dict[str, object]]:
    issue_forms = [
        path
        for path in (root / ".github/ISSUE_TEMPLATE").glob("*.y*ml")
        if path.name.lower() != "config.yml"
    ] if (root / ".github/ISSUE_TEMPLATE").exists() else []

    checks = {
        "issue_forms": present(
            root / ".github/ISSUE_TEMPLATE",
            f"{len(issue_forms)} regulated issue form(s)",
        )
        if issue_forms
        else missing(root / ".github/ISSUE_TEMPLATE", "No regulated YAML issue form found."),
        "pull_request_template": present(root / ".github/PULL_REQUEST_TEMPLATE.md")
        if (root / ".github/PULL_REQUEST_TEMPLATE.md").is_file()
        else missing(root / ".github/PULL_REQUEST_TEMPLATE.md"),
        "codeowners": codeowners_check(root / ".github/CODEOWNERS"),
        "regulated_documentation_gate": present(root / ".github/workflows/regulated-documentation-gate.yml")
        if (root / ".github/workflows/regulated-documentation-gate.yml").is_file()
        else missing(root / ".github/workflows/regulated-documentation-gate.yml"),
        "regulated_evidence_pack_workflow": present(root / ".github/workflows/regulated-evidence-pack.yml")
        if (root / ".github/workflows/regulated-evidence-pack.yml").is_file()
        else missing(root / ".github/workflows/regulated-evidence-pack.yml"),
    }
    return checks


def forge_api(repo: str, endpoint: str, provider: Provider) -> tuple[bool, Any]:
    """GET `/repos/{repo}{endpoint}` through the provider; a refusal is data, not a pass."""
    try:
        payload = provider.get_json(f"/repos/{repo}{endpoint}")
    except ForgeError as exc:
        return False, {"error": str(exc), "status": exc.status, "provider": provider.name}
    return True, payload if payload is not None else {}


def live_checks(repo: str, provider: Provider) -> dict[str, dict[str, object]]:
    checks: dict[str, dict[str, object]] = {}

    ok, rulesets = forge_api(repo, "/rulesets", provider)
    if ok and isinstance(rulesets, list) and rulesets:
        checks["rulesets"] = {"status": "verified", "count": len(rulesets)}
    else:
        checks["rulesets"] = requires_live("No live ruleset evidence was collected.")
        checks["rulesets"]["api_detail"] = rulesets

    branch_results = {}
    protected = 0
    for branch in ("main", "develop"):
        ok, protection = forge_api(repo, f"/branches/{branch}/protection", provider)
        branch_results[branch] = protection
        if ok:
            protected += 1
    if protected:
        checks["branch_protection"] = {
            "status": "verified",
            "protected_branches_checked": protected,
            "branches": sorted(branch for branch, detail in branch_results.items() if "error" not in detail),
        }
        checks["required_status_checks"] = {
            "status": "requires_human_review",
            "detail": "Branch protection exists; required check names must be reviewed against release policy.",
        }
        checks["required_reviews"] = {
            "status": "requires_human_review",
            "detail": "Branch protection exists; reviewer count and CODEOWNER review settings must be reviewed.",
        }
    else:
        checks["branch_protection"] = requires_live("No live branch protection evidence was collected.")
        checks["branch_protection"]["api_detail"] = branch_results
        checks["required_status_checks"] = requires_live("Required status checks depend on branch protection evidence.")
        checks["required_reviews"] = requires_live("Required reviews depend on branch protection evidence.")

    ok, environments = forge_api(repo, "/environments", provider)
    if ok and isinstance(environments, dict) and environments.get("total_count", 0):
        checks["protected_environments"] = {
            "status": "requires_human_review",
            "count": environments.get("total_count", 0),
            "detail": "Environment names and reviewer rules must be reviewed against regulated-release policy.",
        }
    else:
        checks["protected_environments"] = requires_live("No live protected environment evidence was collected.")
        checks["protected_environments"]["api_detail"] = environments

    ok, repo_detail = forge_api(repo, "", provider)
    if ok and isinstance(repo_detail, dict):
        security = repo_detail.get("security_and_analysis", {})
        disabled = [
            feature
            for feature in REQUIRED_SECURITY_FEATURES
            if security.get(feature, {}).get("status") != "enabled"
        ]
        if disabled:
            checks["security_features"] = {
                "status": "requires_human_review",
                "detail": "Some required GitHub security features are not enabled.",
                "disabled_features": disabled,
                "security_and_analysis": security,
            }
        else:
            checks["security_features"] = {
                "status": "verified",
                "enabled_features": REQUIRED_SECURITY_FEATURES,
                "security_and_analysis": security,
            }
    else:
        checks["security_features"] = requires_live("No repository security configuration evidence was collected.")
        checks["security_features"]["api_detail"] = repo_detail

    checks["artifact_retention_export"] = requires_live(
        "GitHub workflow artifact retention/export evidence requires policy plus retained export artifact."
    )
    checks["audit_log_export"] = requires_live(
        "Organization audit-log export evidence must be collected outside repository files."
    )
    return checks


def offline_live_checks() -> dict[str, dict[str, object]]:
    return {
        name: requires_live(f"{name} requires live GitHub settings or exported audit evidence.")
        for name in LIVE_CONTROL_NAMES
    }


def overall_status(checks: dict[str, dict[str, object]]) -> str:
    statuses = {str(check.get("status")) for check in checks.values()}
    if "missing" in statuses:
        return "failed"
    if any(status in statuses for status in ("requires_live_evidence", "requires_human_review")):
        return "requires_evidence"
    return "verified"


def build_report(
    root: Path, repo: str, offline: bool, provider: Provider | None = None
) -> dict[str, object]:
    """Assemble the report; live mode needs a provider (injected or resolved from the environment)."""
    checks = local_checks(root)
    if offline:
        checks.update(offline_live_checks())
    else:
        forge = provider if provider is not None else provider_from_env()
        checks.update(live_checks(repo, forge))
    return {
        "schema_version": "0.1.0",
        "status": overall_status(checks),
        "generated_at_utc": utc_now(),
        "repo": repo,
        "root": str(root.resolve()),
        "offline": offline,
        "claim_boundary": "GitHub QMS audit evidence only; no compliance certification.",
        "checks": checks,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Audit GitHub regulated operating controls.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--repo", required=True, help="GitHub repository, e.g. RBOKproject/NOMOS.")
    parser.add_argument("--offline", action="store_true", help="Do not call the forge API.")
    parser.add_argument("--strict", action="store_true", help="Return non-zero unless all checks are verified.")
    parser.add_argument(
        "--output",
        default=".regulated-evidence-pack/github-qms-audit.json",
        help="JSON report path.",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = Path(args.output)
    if not output.is_absolute():
        output = root / output
    output.parent.mkdir(parents=True, exist_ok=True)

    try:
        report = build_report(root, args.repo, args.offline)
    except ForgeError as exc:
        # Sans --offline, l'absence de configuration est une erreur nommée :
        # l'audit ne rend pas un rapport « sans preuve vivante » en silence.
        print(f"ERROR: forge provider not configured: {exc}", file=sys.stderr)
        return 2
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"]}, indent=2, sort_keys=True))
    if args.strict and report["status"] != "verified":
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
