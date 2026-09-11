#!/usr/bin/env python3
"""
Verify and optionally apply deployment environment protection rules.

Reads the declarative config from
  docs/regulated/github-operating-model/release-environment-config.yaml
and compares against the live settings read through the forge provider
(`scripts/forge_provider.py`, selected by `NOMOS_FORGE_PROVIDER` /
`NOMOS_FORGE_URL` / `NOMOS_FORGE_TOKEN_FILE`). Deployment environments
are a GitHub concept: on another forge the provider refuses explicitly
(`NotSupported`) and the refusal is reported as a blocking finding, never
as a pass. A missing provider configuration is an error (docs/43 §2.8).

Usage:
  python3 scripts/regulated_release_env.py --verify
  python3 scripts/regulated_release_env.py --apply --confirm
  python3 scripts/regulated_release_env.py --verify --format json
"""

import argparse
import json
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    yaml = None

# Le module frère vit dans scripts/ ; le script peut être chargé depuis
# ailleurs (tests, importlib), d'où l'ajout explicite au chemin.
_SCRIPTS_DIR = str(Path(__file__).resolve().parent)
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from forge_provider import ForgeError, Provider, provider_from_env  # noqa: E402

CONFIG_PATH = "docs/regulated/github-operating-model/release-environment-config.yaml"


def load_config(path: str) -> dict:
    config_file = Path(path)
    if not config_file.exists():
        print(f"ERROR: config not found: {path}", file=sys.stderr)
        sys.exit(1)
    if yaml is None:
        print("ERROR: PyYAML required. pip install pyyaml", file=sys.stderr)
        sys.exit(1)
    with open(config_file) as f:
        return yaml.safe_load(f)


def resolve_provider(provider: Provider | None = None) -> Provider:
    """Le fournisseur injecté prime ; sinon il est résolu depuis l'environnement.

    Une configuration absente lève `ForgeConfigError` (doctrine §2.8) : la
    vérification n'est jamais « sautée » en silence.
    """
    return provider if provider is not None else provider_from_env()


def read_live_environment(
    owner: str, repo: str, env_name: str, provider: Provider | None = None
) -> tuple[dict | None, str]:
    """Live environment payload, or (None, reason) when it cannot be read."""
    try:
        return resolve_provider(provider).get_environment(f"{owner}/{repo}", env_name), ""
    except ForgeError as exc:
        return None, str(exc)


def verify_environment(owner: str, repo: str, env_config: dict, provider: Provider | None = None) -> list[dict]:
    """Verify a single environment against the live forge settings."""
    env_name = env_config["name"]
    protection = env_config.get("protection", {})
    branch_policy = env_config.get("deployment_branch_policy", {})
    findings = []

    live, reason = read_live_environment(owner, repo, env_name, provider)

    if live is None:
        findings.append(finding(
            "ENV-EXISTS", env_name, "critical", True,
            f"Environment '{env_name}' does not exist or cannot be read ({reason}).",
            f"Create environment '{env_name}' in repository settings.",
        ))
        return findings

    # Protection rules
    live_rules = live.get("protection_rules", [])
    reviewer_rule = next((r for r in live_rules if r.get("type") == "required_reviewers"), None)
    wait_rule = next((r for r in live_rules if r.get("type") == "wait_timer"), None)

    # Self-review prevention
    if protection.get("prevent_self_review", False):
        prevent = live.get("prevent_self_review", False)
        if not prevent:
            findings.append(finding(
                "ENV-SELF-REVIEW", env_name, "critical", True,
                f"Environment '{env_name}': self-review prevention not enabled.",
                "Enable 'Prevent self-review' in environment protection rules.",
            ))

    # Wait timer
    expected_wait = protection.get("wait_timer_minutes", 0)
    if expected_wait > 0:
        actual_wait = wait_rule.get("wait_timer", 0) if wait_rule else 0
        if actual_wait < expected_wait:
            findings.append(finding(
                "ENV-WAIT-TIMER", env_name, "high", True,
                f"Environment '{env_name}': wait timer is {actual_wait}m, expected >= {expected_wait}m.",
                f"Set wait timer to at least {expected_wait} minutes.",
            ))

    # Required reviewers
    reviewers_cfg = protection.get("required_reviewers", {})
    expected_users = reviewers_cfg.get("users", [])
    expected_teams = reviewers_cfg.get("teams", [])
    if expected_users or expected_teams:
        if reviewer_rule is None:
            findings.append(finding(
                "ENV-REVIEWERS", env_name, "high", True,
                f"Environment '{env_name}': no required reviewers configured.",
                "Add required reviewers to environment protection rules.",
            ))
        else:
            live_reviewers = reviewer_rule.get("reviewers", [])
            live_logins = {r.get("reviewer", {}).get("login", "") for r in live_reviewers}
            for user in expected_users:
                if user not in live_logins:
                    findings.append(finding(
                        "ENV-REVIEWER-MISSING", env_name, "high", False,
                        f"Environment '{env_name}': expected reviewer '{user}' not configured.",
                        f"Add '{user}' as required reviewer.",
                    ))
    elif not expected_users and not expected_teams:
        # No reviewers declared — advisory warning
        findings.append(finding(
            "ENV-REVIEWERS-EMPTY", env_name, "medium", False,
            f"Environment '{env_name}': no reviewers declared in config (placeholder).",
            "Assign real reviewers when quality/regulatory owners are designated.",
        ))

    # Deployment branch policy
    if branch_policy.get("protected_branches_only", False):
        live_policy = live.get("deployment_branch_policy")
        if live_policy is None or not live_policy.get("protected_branches", False):
            findings.append(finding(
                "ENV-BRANCH-POLICY", env_name, "high", True,
                f"Environment '{env_name}': not restricted to protected branches.",
                "Set deployment branch policy to 'Protected branches only'.",
            ))

    return findings


def verify_governance_controls(config: dict) -> list[dict]:
    """Verify governance controls declared in the config."""
    findings = []
    controls = config.get("governance_controls", [])
    envs = {e["name"]: e for e in config.get("environments", [])}

    for ctrl in controls:
        # Only check the regulated-release environment
        env = envs.get("regulated-release")
        if env is None:
            findings.append(finding(
                ctrl["id"], "regulated-release", ctrl["severity"], ctrl["blocking"],
                f"Control {ctrl['id']}: regulated-release environment not defined in config.",
                ctrl["description"],
            ))
            continue

        field_path = ctrl.get("field", "")
        if not field_path:
            continue

        value = resolve_field(env, field_path)
        expected = ctrl.get("expected")
        expected_min = ctrl.get("expected_min")
        expected_min_count = ctrl.get("expected_min_count")

        if expected is not None and value != expected:
            findings.append(finding(
                ctrl["id"], "regulated-release", ctrl["severity"], ctrl["blocking"],
                f"Control {ctrl['id']}: {field_path} is {value!r}, expected {expected!r}.",
                ctrl["description"],
            ))
        elif expected_min is not None and (value is None or value < expected_min):
            findings.append(finding(
                ctrl["id"], "regulated-release", ctrl["severity"], ctrl["blocking"],
                f"Control {ctrl['id']}: {field_path} is {value!r}, expected >= {expected_min}.",
                ctrl["description"],
            ))
        elif expected_min_count is not None:
            count = len(value) if isinstance(value, (list, dict)) else 0
            if isinstance(value, dict):
                count = sum(len(v) for v in value.values() if isinstance(v, list))
            if count < expected_min_count:
                findings.append(finding(
                    ctrl["id"], "regulated-release", ctrl["severity"], ctrl["blocking"],
                    f"Control {ctrl['id']}: {field_path} has {count} items, expected >= {expected_min_count}.",
                    ctrl["description"],
                ))

    return findings


def resolve_field(obj: dict, path: str):
    """Resolve a dotted field path in a nested dict."""
    parts = path.split(".")
    current = obj
    for part in parts:
        if isinstance(current, dict):
            current = current.get(part)
        elif isinstance(current, list):
            for item in current:
                if isinstance(item, dict) and item.get("name") == part:
                    current = item.get("value")
                    break
            else:
                return None
        else:
            return None
    return current


def apply_environment(owner: str, repo: str, env_config: dict, provider: Provider | None = None) -> tuple[bool, str]:
    """Create or update a deployment environment. Returns (success, detail)."""
    env_name = env_config["name"]
    protection = env_config.get("protection", {})

    payload = {
        "prevent_self_review": protection.get("prevent_self_review", False),
        "deployment_branch_policy": None,
    }

    wait = protection.get("wait_timer_minutes", 0)
    if wait > 0:
        payload["wait_timer"] = wait

    reviewers_cfg = protection.get("required_reviewers", {})
    reviewers = []
    for user in reviewers_cfg.get("users", []):
        reviewers.append({"type": "User", "id": 0})  # requires actual user IDs
    for team in reviewers_cfg.get("teams", []):
        reviewers.append({"type": "Team", "id": 0})
    if reviewers:
        payload["reviewers"] = reviewers

    branch_policy = env_config.get("deployment_branch_policy", {})
    if branch_policy.get("protected_branches_only"):
        payload["deployment_branch_policy"] = {"protected_branches": True, "custom_branch_policies": False}
    elif branch_policy.get("custom_branch_policies"):
        payload["deployment_branch_policy"] = {"protected_branches": False, "custom_branch_policies": True}

    try:
        resolve_provider(provider).put_environment(f"{owner}/{repo}", env_name, payload)
    except ForgeError as exc:
        # Un refus (forge sans environnements, droits insuffisants) est
        # rapporté tel quel, jamais maquillé en succès.
        return False, str(exc)
    return True, ""


def verify_all(config: dict, provider: Provider | None = None) -> list[dict]:
    repo = config.get("repository", {})
    owner, name = repo.get("owner", ""), repo.get("name", "")
    all_findings = []
    forge = resolve_provider(provider)
    for env in config.get("environments", []):
        all_findings.extend(verify_environment(owner, name, env, forge))
    all_findings.extend(verify_governance_controls(config))
    return all_findings


def finding(control, env, severity, blocking, message, remediation):
    return {
        "control": control,
        "environment": env,
        "severity": severity,
        "blocking": blocking,
        "message": message,
        "remediation": remediation,
    }


def print_findings(findings: list[dict], fmt: str = "text") -> None:
    if fmt == "json":
        json.dump({"findings": findings, "total": len(findings),
                    "blocking": sum(1 for f in findings if f.get("blocking"))},
                   sys.stdout, indent=2)
        print()
        return
    if not findings:
        print("Release environment: ALL CHECKS PASSED")
        return
    blocking = sum(1 for f in findings if f.get("blocking"))
    print(f"Release environment: {len(findings)} findings ({blocking} blocking)\n")
    for f in findings:
        marker = "BLOCK" if f.get("blocking") else "WARN "
        print(f"  [{marker}] [{f['control']}] {f['environment']}: {f['message']}")
        print(f"         Fix: {f['remediation']}")


def main():
    parser = argparse.ArgumentParser(description="Verify/apply deployment environment protection through the forge provider")
    parser.add_argument("--config", default=CONFIG_PATH)
    parser.add_argument("--verify", action="store_true")
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--confirm", action="store_true")
    parser.add_argument("--format", choices=["text", "json"], default="text")
    args = parser.parse_args()

    if not args.verify and not args.apply:
        parser.print_help()
        sys.exit(2)

    config = load_config(args.config)

    try:
        provider = resolve_provider()
    except ForgeError as exc:
        print(f"ERROR: forge provider not configured: {exc}", file=sys.stderr)
        sys.exit(2)

    if args.verify:
        findings = verify_all(config, provider)
        print_findings(findings, args.format)
        sys.exit(1 if any(f["blocking"] for f in findings) else 0)

    if args.apply:
        if not args.confirm:
            print("ERROR: --apply requires --confirm", file=sys.stderr)
            sys.exit(2)
        repo = config.get("repository", {})
        owner, name = repo.get("owner", ""), repo.get("name", "")
        for env in config.get("environments", []):
            ok, detail = apply_environment(owner, name, env, provider)
            print(f"{'OK  ' if ok else 'FAIL'}: {env['name']}" + (f" ({detail})" if detail else ""))


if __name__ == "__main__":
    main()
