#!/usr/bin/env python3
"""
Verify and optionally apply GitHub branch protection rules.

Reads the declarative config from
  docs/regulated/github-operating-model/branch-protection-config.yaml
and compares it against the live GitHub repository settings via `gh api`.

Usage:
  python3 scripts/regulated_branch_protection.py --verify
  python3 scripts/regulated_branch_protection.py --apply --confirm
  python3 scripts/regulated_branch_protection.py --verify --format json
"""

import argparse
import json
import subprocess
import sys
from pathlib import Path

try:
    import yaml
except ImportError:
    yaml = None  # handled at runtime


CONFIG_PATH = "docs/regulated/github-operating-model/branch-protection-config.yaml"


def load_config(path: str) -> dict:
    """Load the branch protection YAML config."""
    config_file = Path(path)
    if not config_file.exists():
        print(f"ERROR: config not found: {path}", file=sys.stderr)
        sys.exit(1)
    if yaml is None:
        print("ERROR: PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)
        sys.exit(1)
    with open(config_file) as f:
        return yaml.safe_load(f)


def gh_api(endpoint: str, method: str = "GET", data: dict | None = None) -> dict | None:
    """Call the GitHub API via `gh api`."""
    cmd = ["gh", "api", endpoint, "--method", method]
    if data is not None:
        cmd.extend(["--input", "-"])
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            input=json.dumps(data) if data else None,
            timeout=30,
        )
        if result.returncode != 0:
            return None
        if result.stdout.strip():
            return json.loads(result.stdout)
        return {}
    except (subprocess.TimeoutExpired, FileNotFoundError, json.JSONDecodeError):
        return None


def verify_branch(owner: str, repo: str, rule: dict) -> list[dict]:
    """Verify a single branch protection rule. Returns findings."""
    branch = rule["branch"]
    protection = rule["protection"]
    findings = []

    endpoint = f"/repos/{owner}/{repo}/branches/{branch}/protection"
    live = gh_api(endpoint)

    if live is None:
        findings.append({
            "control": "BRANCH-PROTECTION-EXISTS",
            "branch": branch,
            "severity": "critical",
            "blocking": True,
            "message": f"Cannot read branch protection for {branch}. Either not configured or insufficient permissions.",
            "remediation": f"Configure branch protection for {branch} in repository settings.",
        })
        return findings

    # Require PR
    pr_reviews = live.get("required_pull_request_reviews")
    if protection.get("require_pull_request") and pr_reviews is None:
        findings.append({
            "control": "REQUIRE-PR",
            "branch": branch,
            "severity": "critical",
            "blocking": True,
            "message": f"Branch {branch}: pull request reviews not required.",
            "remediation": "Enable 'Require a pull request before merging'.",
        })

    # Required approvals count
    if pr_reviews and protection.get("required_approving_review_count"):
        actual = pr_reviews.get("required_approving_review_count", 0)
        expected = protection["required_approving_review_count"]
        if actual < expected:
            findings.append({
                "control": "REVIEW-COUNT",
                "branch": branch,
                "severity": "high",
                "blocking": True,
                "message": f"Branch {branch}: {actual} approvals required, expected >= {expected}.",
                "remediation": f"Set required approving reviews to at least {expected}.",
            })

    # Dismiss stale reviews
    if pr_reviews and protection.get("dismiss_stale_reviews"):
        if not pr_reviews.get("dismiss_stale_reviews", False):
            findings.append({
                "control": "DISMISS-STALE",
                "branch": branch,
                "severity": "medium",
                "blocking": False,
                "message": f"Branch {branch}: stale review dismissal not enabled.",
                "remediation": "Enable 'Dismiss stale pull request approvals when new commits are pushed'.",
            })

    # Force pushes
    if not protection.get("allow_force_pushes", False):
        allow_fp = live.get("allow_force_pushes", {})
        if allow_fp.get("enabled", False):
            findings.append({
                "control": "NO-FORCE-PUSH",
                "branch": branch,
                "severity": "critical",
                "blocking": True,
                "message": f"Branch {branch}: force pushes are allowed.",
                "remediation": "Disable 'Allow force pushes'.",
            })

    # Deletions
    if not protection.get("allow_deletions", False):
        allow_del = live.get("allow_deletions", {})
        if allow_del.get("enabled", False):
            findings.append({
                "control": "NO-DELETION",
                "branch": branch,
                "severity": "critical",
                "blocking": True,
                "message": f"Branch {branch}: branch deletion is allowed.",
                "remediation": "Disable 'Allow deletions'.",
            })

    # Linear history
    if protection.get("require_linear_history"):
        linear = live.get("required_linear_history", {})
        if not linear.get("enabled", False):
            findings.append({
                "control": "LINEAR-HISTORY",
                "branch": branch,
                "severity": "medium",
                "blocking": False,
                "message": f"Branch {branch}: linear history not required.",
                "remediation": "Enable 'Require linear history'.",
            })

    # Required status checks
    checks = protection.get("required_status_checks", {})
    if checks:
        live_checks = live.get("required_status_checks")
        if live_checks is None:
            findings.append({
                "control": "STATUS-CHECKS",
                "branch": branch,
                "severity": "high",
                "blocking": True,
                "message": f"Branch {branch}: no required status checks configured.",
                "remediation": f"Add required status checks: {', '.join(checks.get('contexts', []))}.",
            })
        else:
            live_contexts = set()
            for ctx in live_checks.get("contexts", []):
                live_contexts.add(ctx)
            for ctx in live_checks.get("checks", []):
                live_contexts.add(ctx.get("context", ""))
            for expected_ctx in checks.get("contexts", []):
                if expected_ctx not in live_contexts:
                    findings.append({
                        "control": "STATUS-CHECK-CONTEXT",
                        "branch": branch,
                        "severity": "high",
                        "blocking": False,
                        "message": f"Branch {branch}: missing required check '{expected_ctx}'.",
                        "remediation": f"Add '{expected_ctx}' to required status checks.",
                    })

    # Enforce admins
    if protection.get("enforce_admins"):
        enforce = live.get("enforce_admins", {})
        if not enforce.get("enabled", False):
            findings.append({
                "control": "ENFORCE-ADMINS",
                "branch": branch,
                "severity": "medium",
                "blocking": False,
                "message": f"Branch {branch}: admins can bypass protection.",
                "remediation": "Enable 'Do not allow bypassing the above settings'.",
            })

    return findings


def apply_branch(owner: str, repo: str, rule: dict) -> bool:
    """Apply branch protection to a single branch. Returns success."""
    branch = rule["branch"]
    protection = rule["protection"]

    payload = {
        "required_pull_request_reviews": None,
        "required_status_checks": None,
        "enforce_admins": protection.get("enforce_admins", False),
        "restrictions": protection.get("restrictions"),
        "required_linear_history": protection.get("require_linear_history", False),
        "allow_force_pushes": protection.get("allow_force_pushes", False),
        "allow_deletions": protection.get("allow_deletions", False),
        "lock_branch": protection.get("lock_branch", False),
    }

    if protection.get("require_pull_request"):
        payload["required_pull_request_reviews"] = {
            "required_approving_review_count": protection.get("required_approving_review_count", 1),
            "dismiss_stale_reviews": protection.get("dismiss_stale_reviews", False),
            "require_code_owner_reviews": protection.get("require_code_owner_reviews", False),
        }

    checks = protection.get("required_status_checks")
    if checks:
        payload["required_status_checks"] = {
            "strict": checks.get("strict", True),
            "contexts": checks.get("contexts", []),
        }

    endpoint = f"/repos/{owner}/{repo}/branches/{branch}/protection"
    result = gh_api(endpoint, method="PUT", data=payload)
    return result is not None


def verify_all(config: dict) -> list[dict]:
    """Verify all branch rules and return findings."""
    repo_cfg = config.get("repository", {})
    owner = repo_cfg.get("owner", "")
    repo = repo_cfg.get("name", "")
    all_findings = []

    for rule in config.get("branch_rules", []):
        findings = verify_branch(owner, repo, rule)
        all_findings.extend(findings)

    return all_findings


def print_findings(findings: list[dict], fmt: str = "text") -> None:
    """Print findings in text or JSON format."""
    if fmt == "json":
        json.dump({"findings": findings, "total": len(findings),
                    "blocking": sum(1 for f in findings if f.get("blocking"))},
                   sys.stdout, indent=2)
        print()
        return

    if not findings:
        print("Branch protection: ALL CHECKS PASSED")
        return

    blocking = sum(1 for f in findings if f.get("blocking"))
    print(f"Branch protection: {len(findings)} findings ({blocking} blocking)\n")
    for f in findings:
        marker = "BLOCK" if f.get("blocking") else "WARN "
        print(f"  [{marker}] [{f['control']}] {f['branch']}: {f['message']}")
        print(f"         Fix: {f['remediation']}")


def main():
    parser = argparse.ArgumentParser(description="Verify/apply GitHub branch protection")
    parser.add_argument("--config", default=CONFIG_PATH, help="Config YAML path")
    parser.add_argument("--verify", action="store_true", help="Verify current settings")
    parser.add_argument("--apply", action="store_true", help="Apply settings via gh api")
    parser.add_argument("--confirm", action="store_true", help="Required with --apply")
    parser.add_argument("--format", choices=["text", "json"], default="text")
    args = parser.parse_args()

    if not args.verify and not args.apply:
        parser.print_help()
        sys.exit(2)

    config = load_config(args.config)

    if args.verify:
        findings = verify_all(config)
        print_findings(findings, args.format)
        blocking = sum(1 for f in findings if f.get("blocking"))
        sys.exit(1 if blocking > 0 else 0)

    if args.apply:
        if not args.confirm:
            print("ERROR: --apply requires --confirm", file=sys.stderr)
            sys.exit(2)
        repo_cfg = config.get("repository", {})
        owner = repo_cfg.get("owner", "")
        repo = repo_cfg.get("name", "")
        for rule in config.get("branch_rules", []):
            branch = rule["branch"]
            if "*" in branch:
                print(f"SKIP: wildcard branch {branch} (apply manually or via rulesets)")
                continue
            ok = apply_branch(owner, repo, rule)
            print(f"{'OK  ' if ok else 'FAIL'}: {branch}")


if __name__ == "__main__":
    main()
