#!/usr/bin/env python3
"""NRT-025 (#678) — the security process, executable.

SECURITY.md used to be alpha prose. This gate turns the process into checks
that fail closed, and nothing more:

1. process  — ``docs/security/security-process.yaml`` (nomos-security-process-v1)
   validates: intake channels, triage targets (declared as targets, never
   measured SLAs), disclosure rule, supported-versions source, scanner
   configuration, Dependabot configuration, SOP references — every referenced
   file must exist.
2. allowlist — ``docs/security/vulnerability-allowlist.yaml``: every accepted
   finding carries id, ecosystem, package, justification, owner, accepted_on
   and expires_on. Undated, expired, over-long or duplicated entries turn the
   gate red. The gate reads the allowlist; the allowlist never reads the gate.
3. supported_versions — the "Supported Versions" section of SECURITY.md is
   GENERATED (from the support model of NRT-026 when it exists, from
   CHANGELOG.md until then) between markers; drift is red, ``--write``
   regenerates it.
4. govulncheck (``--scan govulncheck``) — every declared Go module is scanned at
   symbol level; a vulnerability the code CALLS (standard library included)
   is red unless an unexpired allowlist entry names it; imported-only and
   required-only findings are reported, never hidden.
5. pip-audit (``--scan pip-audit``) — the declared, pinned sidecar requirements
   are audited; any known vulnerability is red unless allowlisted.
6. manifests (#696) — every dependency manifest tracked by git (package.json,
   pyproject.toml, go.mod, Cargo.toml, Gemfile, pom.xml, requirements*.txt) is
   in a scanner's scope, in a Dependabot directory, or listed by name with a
   reason under ``manifests.not_scanned``. The gate enumerates the manifests
   itself: an unlisted one is red, a stale exclusion is red. A manifest nobody
   scans is a manifest nobody sees.

A requested scanner that is missing is a failed check, not a skipped one.

Exit 0 = every check passed; 1 = a check failed (named on stderr and in the
JSON verdict); 2 = usage error.

Claim boundary: "dependencies are scanned in CI and accepted findings expire".
Never "secure", never "certified", never a guarantee about a deployment.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from datetime import date, datetime
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for the security process gate.", file=sys.stderr)
    raise SystemExit(2) from exc


PROCESS_SCHEMA = "nomos-security-process-v1"
ALLOWLIST_SCHEMA = "nomos-vulnerability-allowlist-v1"
SUPPORT_MODEL_SCHEMA = "nomos-support-model-v1"
VERDICT_SCHEMA = "nomos-security-gate-v1"
DEFAULT_PROCESS = Path("docs/security/security-process.yaml")
MARK_BEGIN = "<!-- supported-versions:begin -->"
MARK_END = "<!-- supported-versions:end -->"
SEVERITIES = ("critical", "high", "medium", "low")
DISCLOSURE_RULES = ("coordinated", "private_until_fixed")
SUPPORTED_SOURCES = ("changelog", "support_model")
ECOSYSTEMS = ("go", "pypi", "github-actions")
SCAN_LEVELS = ("symbol", "package", "module")
FAIL_ON = ("called", "any")
KNOWN_SCANNERS = ("govulncheck", "pip-audit")
ADVISORY_ID = re.compile(r"^(GO-\d{4}-\d+|GHSA-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}|CVE-\d{4}-\d{4,}|PYSEC-\d{4}-\d+)$")
CHANGELOG_HEADING = re.compile(r"^## (v\S+) - (\d{4}-\d{2}-\d{2})\s*$", re.MULTILINE)
DEFAULT_TIMEOUT = 900.0
MIN_JUSTIFICATION = 20


# --- helpers ---------------------------------------------------------------------


class UsageError(RuntimeError):
    pass


def load_yaml(path: Path) -> Any:
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def check(name: str, problems: list[str], detail: dict[str, Any] | None = None) -> dict[str, Any]:
    return {"name": name, "status": "fail" if problems else "pass", "problems": problems, "detail": detail or {}}


def _date(value: Any) -> date | None:
    if isinstance(value, date) and not isinstance(value, datetime):
        return value
    if isinstance(value, datetime):
        return value.date()
    if isinstance(value, str):
        try:
            return date.fromisoformat(value)
        except ValueError:
            return None
    return None


def _rel(root: Path, value: Any) -> Path | None:
    if not isinstance(value, str) or not value.strip():
        return None
    return root / value


# --- 1. process --------------------------------------------------------------------


def check_process(root: Path, process: Any, process_path: Path) -> dict[str, Any]:
    problems: list[str] = []
    if not isinstance(process, dict):
        return check("process", [f"{process_path}: not a mapping"])
    if process.get("schema_version") != PROCESS_SCHEMA:
        problems.append(f"schema_version must be {PROCESS_SCHEMA!r}, got {process.get('schema_version')!r}")
    if not str(process.get("claim_boundary") or "").strip():
        problems.append("claim_boundary is required")

    intake = process.get("intake") if isinstance(process.get("intake"), dict) else {}
    channels = intake.get("channels")
    if not isinstance(channels, list) or not channels:
        problems.append("intake.channels must list at least one channel")
    else:
        for index, channel in enumerate(channels):
            if not isinstance(channel, dict) or not channel.get("kind") or not channel.get("ref"):
                problems.append(f"intake.channels[{index}] needs kind and ref")
    if intake.get("no_public_disclosure_before_triage") is not True:
        problems.append("intake.no_public_disclosure_before_triage must be true")

    triage = process.get("triage") if isinstance(process.get("triage"), dict) else {}
    targets = triage.get("targets")
    seen: set[str] = set()
    if not isinstance(targets, list) or not targets:
        problems.append("triage.targets must list one target per severity")
    else:
        for index, target in enumerate(targets):
            if not isinstance(target, dict):
                problems.append(f"triage.targets[{index}] is not a mapping")
                continue
            severity = target.get("severity")
            if severity not in SEVERITIES:
                problems.append(f"triage.targets[{index}].severity must be one of {SEVERITIES}, got {severity!r}")
            elif severity in seen:
                problems.append(f"triage.targets: severity {severity!r} declared twice")
            else:
                seen.add(severity)
            for field in ("acknowledge_within_days", "remediate_or_mitigate_within_days"):
                value = target.get(field)
                if not isinstance(value, int) or isinstance(value, bool) or value <= 0:
                    problems.append(f"triage.targets[{index}].{field} must be a positive integer")
        missing = [s for s in SEVERITIES if s not in seen]
        if missing:
            problems.append("triage.targets: missing severity target(s): " + ", ".join(missing))
    if triage.get("status") != "declared_not_measured":
        problems.append(
            "triage.status must be 'declared_not_measured': targets are declared, no measurement record backs them"
        )

    disclosure = process.get("disclosure") if isinstance(process.get("disclosure"), dict) else {}
    if disclosure.get("rule") not in DISCLOSURE_RULES:
        problems.append(f"disclosure.rule must be one of {DISCLOSURE_RULES}, got {disclosure.get('rule')!r}")
    embargo = disclosure.get("embargo_days_max")
    if not isinstance(embargo, int) or isinstance(embargo, bool) or embargo <= 0:
        problems.append("disclosure.embargo_days_max must be a positive integer")

    supported = process.get("supported_versions") if isinstance(process.get("supported_versions"), dict) else {}
    source = supported.get("source")
    if source not in SUPPORTED_SOURCES:
        problems.append(f"supported_versions.source must be one of {SUPPORTED_SOURCES}, got {source!r}")
    changelog = _rel(root, supported.get("changelog_ref"))
    if changelog is None or not changelog.is_file():
        problems.append("supported_versions.changelog_ref must point at an existing file")
    model_ref = _rel(root, supported.get("support_model_ref"))
    if source == "support_model" and (model_ref is None or not model_ref.is_file()):
        problems.append("supported_versions.source is support_model but support_model_ref does not exist")
    rendered = supported.get("rendered_in")
    if not isinstance(rendered, list) or not rendered:
        problems.append("supported_versions.rendered_in must list the files carrying the generated section")
    else:
        for target in rendered:
            path = _rel(root, target)
            if path is None or not path.is_file():
                problems.append(f"supported_versions.rendered_in: {target!r} does not exist")
            elif MARK_BEGIN not in path.read_text(encoding="utf-8") or MARK_END not in path.read_text(encoding="utf-8"):
                problems.append(f"supported_versions.rendered_in: {target!r} lacks the generated-section markers")

    scanners = process.get("scanners") if isinstance(process.get("scanners"), dict) else {}
    govuln = scanners.get("govulncheck") if isinstance(scanners.get("govulncheck"), dict) else {}
    modules = govuln.get("modules")
    if not isinstance(modules, list) or not modules:
        problems.append("scanners.govulncheck.modules must list at least one Go module directory")
    else:
        for module in modules:
            path = _rel(root, module)
            if path is None or not (path / "go.mod").is_file():
                problems.append(f"scanners.govulncheck.modules: {module!r} has no go.mod")
    if govuln.get("scan_level") not in SCAN_LEVELS:
        problems.append(f"scanners.govulncheck.scan_level must be one of {SCAN_LEVELS}")
    if govuln.get("fail_on") not in FAIL_ON:
        problems.append(f"scanners.govulncheck.fail_on must be one of {FAIL_ON}")
    pip = scanners.get("pip_audit") if isinstance(scanners.get("pip_audit"), dict) else {}
    requirements = _rel(root, pip.get("requirements"))
    if requirements is None or not requirements.is_file():
        problems.append("scanners.pip_audit.requirements must point at an existing requirements file")
    else:
        for lineno, line in enumerate(requirements.read_text(encoding="utf-8").splitlines(), 1):
            stripped = line.split("#", 1)[0].strip()
            if stripped and "==" not in stripped:
                problems.append(f"{pip.get('requirements')}:{lineno}: {stripped!r} is not pinned with '==' (pip-audit runs --no-deps on exact pins)")
    dependabot = scanners.get("dependabot") if isinstance(scanners.get("dependabot"), dict) else {}
    config = _rel(root, dependabot.get("config"))
    ecosystems = dependabot.get("ecosystems")
    if config is None or not config.is_file():
        problems.append("scanners.dependabot.config must point at an existing Dependabot configuration")
    else:
        try:
            loaded = load_yaml(config)
        except yaml.YAMLError as exc:
            loaded = None
            problems.append(f"{dependabot.get('config')}: not YAML ({exc})")
        if isinstance(loaded, dict):
            if loaded.get("version") != 2:
                problems.append(f"{dependabot.get('config')}: version must be 2")
            declared = {str(u.get("package-ecosystem")) for u in (loaded.get("updates") or []) if isinstance(u, dict)}
            if isinstance(ecosystems, list):
                for eco in ecosystems:
                    if str(eco) not in declared:
                        problems.append(f"{dependabot.get('config')}: declared ecosystem {eco!r} has no updates entry")
    if not isinstance(ecosystems, list) or not ecosystems:
        problems.append("scanners.dependabot.ecosystems must list the ecosystems Dependabot covers")

    allowlist = process.get("allowlist") if isinstance(process.get("allowlist"), dict) else {}
    allow_path = _rel(root, allowlist.get("path"))
    if allow_path is None or not allow_path.is_file():
        problems.append("allowlist.path must point at an existing allowlist file")
    max_days = allowlist.get("max_validity_days")
    if not isinstance(max_days, int) or isinstance(max_days, bool) or max_days <= 0:
        problems.append("allowlist.max_validity_days must be a positive integer")

    sops = process.get("sops") if isinstance(process.get("sops"), dict) else {}
    if not sops:
        problems.append("sops must reference the vulnerability-management SOP at least")
    for key, ref in sops.items():
        path = _rel(root, ref)
        if path is None or not path.is_file():
            problems.append(f"sops.{key}: {ref!r} does not exist")
    if "vulnerability_management" not in sops:
        problems.append("sops.vulnerability_management is required")

    return check("process", problems, {"path": str(process_path.relative_to(root)) if process_path.is_relative_to(root) else str(process_path)})


# --- 2. allowlist ------------------------------------------------------------------


def load_allowlist(root: Path, process: dict[str, Any]) -> tuple[Path | None, Any]:
    allowlist = process.get("allowlist") if isinstance(process.get("allowlist"), dict) else {}
    path = _rel(root, allowlist.get("path"))
    if path is None or not path.is_file():
        return path, None
    return path, load_yaml(path)


def check_allowlist(root: Path, process: dict[str, Any], today: date, override: Path | None = None) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    """Returns the check and the ACTIVE entries keyed by advisory id."""
    problems: list[str] = []
    allowlist = process.get("allowlist") if isinstance(process.get("allowlist"), dict) else {}
    max_days = allowlist.get("max_validity_days") if isinstance(allowlist.get("max_validity_days"), int) else 180
    path = override or _rel(root, allowlist.get("path"))
    if path is None or not path.is_file():
        return check("allowlist", [f"allowlist file missing: {path}"]), {}
    doc = load_yaml(path)
    if not isinstance(doc, dict):
        return check("allowlist", [f"{path}: not a mapping"]), {}
    if doc.get("schema_version") != ALLOWLIST_SCHEMA:
        problems.append(f"schema_version must be {ALLOWLIST_SCHEMA!r}, got {doc.get('schema_version')!r}")
    entries = doc.get("entries")
    if entries is None:
        entries = []
    if not isinstance(entries, list):
        return check("allowlist", ["entries must be a list"]), {}
    active: dict[str, dict[str, Any]] = {}
    seen: set[str] = set()
    for index, entry in enumerate(entries):
        label = f"entries[{index}]"
        if not isinstance(entry, dict):
            problems.append(f"{label}: not a mapping")
            continue
        advisory = str(entry.get("id") or "")
        label = advisory or label
        if not ADVISORY_ID.match(advisory):
            problems.append(f"{label}: id must be a GO-/GHSA-/CVE-/PYSEC- advisory id")
        if advisory in seen:
            problems.append(f"{label}: declared twice")
        seen.add(advisory)
        if entry.get("ecosystem") not in ECOSYSTEMS:
            problems.append(f"{label}: ecosystem must be one of {ECOSYSTEMS}")
        for field in ("package", "owner"):
            if not str(entry.get(field) or "").strip():
                problems.append(f"{label}: {field} is required")
        if len(str(entry.get("justification") or "").strip()) < MIN_JUSTIFICATION:
            problems.append(f"{label}: justification must explain the acceptance (at least {MIN_JUSTIFICATION} characters)")
        accepted = _date(entry.get("accepted_on"))
        expires = _date(entry.get("expires_on"))
        if accepted is None:
            problems.append(f"{label}: accepted_on (YYYY-MM-DD) is required")
        if expires is None:
            problems.append(f"{label}: expires_on (YYYY-MM-DD) is required — an acceptance without an expiry is not an acceptance")
            continue
        if expires < today:
            problems.append(f"{label}: expired on {expires.isoformat()} (today {today.isoformat()}) — re-assess or remove")
            continue
        if accepted is not None and (expires - accepted).days > max_days:
            problems.append(f"{label}: validity {(expires - accepted).days} days exceeds max_validity_days={max_days}")
            continue
        if accepted is not None and accepted > today:
            problems.append(f"{label}: accepted_on {accepted.isoformat()} is in the future")
            continue
        if not problems or all(not p.startswith(label + ":") for p in problems):
            active[advisory] = entry
    return check("allowlist", problems, {"entries": len(entries), "active": sorted(active)}), active


# --- 3. supported versions ---------------------------------------------------------


def changelog_versions(changelog: Path) -> list[tuple[str, str]]:
    return CHANGELOG_HEADING.findall(changelog.read_text(encoding="utf-8"))


def support_model_versions(model_path: Path) -> list[tuple[str, str, str, str]]:
    doc = load_yaml(model_path)
    if not isinstance(doc, dict) or doc.get("schema_version") != SUPPORT_MODEL_SCHEMA:
        raise UsageError(f"{model_path}: not a {SUPPORT_MODEL_SCHEMA} document")
    rows = []
    for entry in doc.get("supported_versions") or []:
        if not isinstance(entry, dict):
            continue
        rows.append((str(entry.get("version")), str(entry.get("released_on") or "—"), str(entry.get("state")), str(entry.get("security_support") or "")))
    return rows


def render_supported_versions(root: Path, process: dict[str, Any]) -> str:
    supported = process.get("supported_versions") if isinstance(process.get("supported_versions"), dict) else {}
    source = supported.get("source")
    lines = [MARK_BEGIN]
    if source == "support_model":
        model_path = _rel(root, supported.get("support_model_ref"))
        if model_path is None or not model_path.is_file():
            raise UsageError("supported_versions.source is support_model but the model file is missing")
        lines.append(f"<!-- GENERATED from {supported.get('support_model_ref')} by scripts/security_process_gate.py --write; do not edit by hand, CI fails on drift -->")
        lines += ["", "| Version | Released | State | Security support |", "|---|---|---|---|"]
        for version, released, state, support in support_model_versions(model_path):
            lines.append(f"| `{version}` | {released} | {state} | {support} |")
    else:
        changelog = _rel(root, supported.get("changelog_ref"))
        if changelog is None or not changelog.is_file():
            raise UsageError("supported_versions.changelog_ref is missing")
        versions = changelog_versions(changelog)
        if not versions:
            raise UsageError(f"{changelog}: no '## vX - YYYY-MM-DD' heading to derive supported versions from")
        lines.append(f"<!-- GENERATED from {supported.get('changelog_ref')} by scripts/security_process_gate.py --write (source: changelog until the support model of NRT-026 exists); do not edit by hand, CI fails on drift -->")
        lines += ["", "| Version | Released | Security support |", "|---|---|---|"]
        for index, (version, released) in enumerate(versions):
            if index == 0:
                support = "Supported — best-effort alpha triage (current release)"
            else:
                support = "Superseded — not supported"
            lines.append(f"| `{version}` | {released} | {support} |")
        lines.append(f"| older than `{versions[-1][0]}` | — | Not supported |")
    lines += ["", MARK_END]
    return "\n".join(lines)


def _replace_section(text: str, rendered: str) -> str | None:
    start = text.find(MARK_BEGIN)
    end = text.find(MARK_END)
    if start == -1 or end == -1 or end < start:
        return None
    end += len(MARK_END)
    return text[:start] + rendered + text[end:]


def check_supported_versions(root: Path, process: dict[str, Any], write: bool) -> dict[str, Any]:
    problems: list[str] = []
    supported = process.get("supported_versions") if isinstance(process.get("supported_versions"), dict) else {}
    try:
        rendered = render_supported_versions(root, process)
    except UsageError as exc:
        return check("supported_versions", [str(exc)])
    written: list[str] = []
    for target in supported.get("rendered_in") or []:
        path = _rel(root, target)
        if path is None or not path.is_file():
            problems.append(f"{target}: missing")
            continue
        text = path.read_text(encoding="utf-8")
        updated = _replace_section(text, rendered)
        if updated is None:
            problems.append(f"{target}: generated-section markers missing")
            continue
        if updated != text:
            if write:
                path.write_text(updated, encoding="utf-8")
                written.append(str(target))
            else:
                problems.append(f"{target}: the Supported Versions section drifted from its source — run scripts/security_process_gate.py --write")
    return check("supported_versions", problems, {"source": supported.get("source"), "written": written})


# --- 4. govulncheck ----------------------------------------------------------------


def resolve_govulncheck(explicit: str | None) -> list[str] | None:
    if explicit:
        return explicit.split()
    found = shutil.which("govulncheck")
    if found:
        return [found]
    gopath = ""
    if shutil.which("go"):
        proc = subprocess.run(["go", "env", "GOPATH"], text=True, capture_output=True, check=False)
        gopath = proc.stdout.strip()
    candidate = Path(gopath) / "bin" / "govulncheck" if gopath else None
    if candidate and candidate.is_file() and os.access(candidate, os.X_OK):
        return [str(candidate)]
    return None


def parse_json_stream(text: str) -> list[dict[str, Any]]:
    decoder = json.JSONDecoder()
    index, length, out = 0, len(text), []
    while index < length:
        while index < length and text[index].isspace():
            index += 1
        if index >= length:
            break
        obj, index = decoder.raw_decode(text, index)
        if isinstance(obj, dict):
            out.append(obj)
    return out


def scan_go_module(command: list[str], root: Path, module: str, level: str, timeout: float) -> dict[str, Any]:
    cwd = root / module
    argv = [*command, "-json", "-scan", level, "./..."]
    try:
        proc = subprocess.run(argv, cwd=cwd, text=True, capture_output=True, timeout=timeout, check=False)
    except subprocess.TimeoutExpired:
        return {"module": module, "error": f"govulncheck timed out after {timeout:.0f}s"}
    except OSError as exc:
        return {"module": module, "error": f"govulncheck could not run: {exc}"}
    try:
        messages = parse_json_stream(proc.stdout)
    except ValueError as exc:
        return {"module": module, "error": f"govulncheck output is not a JSON stream (exit {proc.returncode}): {exc}; stderr: {proc.stderr.strip()[-400:]}"}
    if proc.returncode not in (0, 3) and not messages:
        return {"module": module, "error": f"govulncheck exited {proc.returncode}: {proc.stderr.strip()[-400:]}"}
    summaries = {m["osv"]["id"]: m["osv"] for m in messages if isinstance(m.get("osv"), dict) and m["osv"].get("id")}
    called: dict[str, dict[str, Any]] = {}
    reported: dict[str, dict[str, Any]] = {}
    for message in messages:
        finding = message.get("finding")
        if not isinstance(finding, dict) or not finding.get("osv"):
            continue
        advisory = str(finding["osv"])
        trace = finding.get("trace") or []
        first = trace[0] if trace and isinstance(trace[0], dict) else {}
        entry = reported.setdefault(advisory, {
            "id": advisory,
            "module": str(first.get("module") or ""),
            "found_in": str(first.get("version") or ""),
            "fixed_in": str(finding.get("fixed_version") or ""),
            "summary": str((summaries.get(advisory) or {}).get("summary") or ""),
            "called": False,
        })
        if first.get("function"):
            entry["called"] = True
            entry["example_trace"] = f"{first.get('package', '')}.{first.get('function')}".strip(".")
            called[advisory] = entry
    return {
        "module": module,
        "scan_level": level,
        "findings": sorted(reported.values(), key=lambda e: e["id"]),
        "called": sorted(called),
        "not_called": sorted(set(reported) - set(called)),
    }


def check_govulncheck(root: Path, process: dict[str, Any], active: dict[str, dict[str, Any]], command: list[str] | None, modules_override: list[str] | None, timeout: float) -> dict[str, Any]:
    scanners = process.get("scanners") if isinstance(process.get("scanners"), dict) else {}
    cfg = scanners.get("govulncheck") if isinstance(scanners.get("govulncheck"), dict) else {}
    level = cfg.get("scan_level") if cfg.get("scan_level") in SCAN_LEVELS else "symbol"
    fail_on = cfg.get("fail_on") if cfg.get("fail_on") in FAIL_ON else "called"
    modules = modules_override or [str(m) for m in (cfg.get("modules") or [])]
    if command is None:
        return check("govulncheck", ["govulncheck is not available (install: go install golang.org/x/vuln/cmd/govulncheck@latest) — a requested scanner that is missing is a failed check"])
    problems: list[str] = []
    results = []
    for module in modules:
        result = scan_go_module(command, root, module, level, timeout)
        results.append(result)
        if result.get("error"):
            problems.append(f"{module}: {result['error']}")
            continue
        failing = result["called"] if fail_on == "called" else [f["id"] for f in result["findings"]]
        for advisory in failing:
            entry = next(f for f in result["findings"] if f["id"] == advisory)
            allow = active.get(advisory)
            if allow and allow.get("ecosystem") == "go":
                entry["allowlisted_until"] = str(allow.get("expires_on"))
                continue
            where = f"{entry['module']}@{entry['found_in']}" if entry.get("module") else module
            fix = f", fixed in {entry['fixed_in']}" if entry.get("fixed_in") else ""
            problems.append(f"{module}: {advisory} called via {entry.get('example_trace', '?')} in {where}{fix} — {entry.get('summary', '')}".rstrip(" —"))
    detail = {"command": command, "fail_on": fail_on, "modules": results}
    return check("govulncheck", problems, detail)


# --- 5. pip-audit ------------------------------------------------------------------


def resolve_pip_audit(explicit: str | None) -> list[str] | None:
    if explicit:
        return explicit.split()
    found = shutil.which("pip-audit")
    if found:
        return [found]
    probe = subprocess.run([sys.executable, "-c", "import pip_audit"], capture_output=True, check=False)
    if probe.returncode == 0:
        return [sys.executable, "-m", "pip_audit"]
    if shutil.which("uvx"):
        return ["uvx", "pip-audit"]
    return None


def check_pip_audit(root: Path, process: dict[str, Any], active: dict[str, dict[str, Any]], command: list[str] | None, requirements_override: Path | None, timeout: float) -> dict[str, Any]:
    scanners = process.get("scanners") if isinstance(process.get("scanners"), dict) else {}
    cfg = scanners.get("pip_audit") if isinstance(scanners.get("pip_audit"), dict) else {}
    requirements = requirements_override or _rel(root, cfg.get("requirements"))
    if command is None:
        return check("pip-audit", ["pip-audit is not available (install: pip install pip-audit) — a requested scanner that is missing is a failed check"])
    if requirements is None or not requirements.is_file():
        return check("pip-audit", [f"requirements file missing: {requirements}"])
    argv = [*command, "--no-deps", "--progress-spinner", "off", "--format", "json", "-r", str(requirements)]
    try:
        proc = subprocess.run(argv, cwd=root, text=True, capture_output=True, timeout=timeout, check=False)
    except subprocess.TimeoutExpired:
        return check("pip-audit", [f"pip-audit timed out after {timeout:.0f}s"])
    except OSError as exc:
        return check("pip-audit", [f"pip-audit could not run: {exc}"])
    try:
        report = json.loads(proc.stdout)
    except ValueError:
        return check("pip-audit", [f"pip-audit produced no JSON report (exit {proc.returncode}): {proc.stderr.strip()[-400:]}"])
    problems: list[str] = []
    dependencies = []
    for dependency in report.get("dependencies") or []:
        name = dependency.get("name")
        version = dependency.get("version")
        vulns = []
        for vuln in dependency.get("vulns") or []:
            ids = [str(vuln.get("id") or "")] + [str(a) for a in (vuln.get("aliases") or [])]
            allow = next((active[i] for i in ids if i in active and active[i].get("ecosystem") == "pypi"), None)
            record = {"id": ids[0], "aliases": ids[1:], "fix_versions": vuln.get("fix_versions") or [], "allowlisted_until": str(allow.get("expires_on")) if allow else None}
            if record in vulns:
                continue
            vulns.append(record)
            if allow is None:
                fix = f", fixed in {', '.join(record['fix_versions'])}" if record["fix_versions"] else ""
                problems.append(f"{name}=={version}: {ids[0]} ({', '.join(ids[1:]) or 'no alias'}){fix}")
        dependencies.append({"name": name, "version": version, "vulns": vulns})
    return check("pip-audit", problems, {"command": command, "requirements": str(requirements), "dependencies": dependencies})


# --- 6. manifests (#696) -------------------------------------------------------------


MANIFEST_NAMES = ("package.json", "pyproject.toml", "go.mod", "Cargo.toml", "Gemfile", "pom.xml")
MANIFEST_REQUIREMENTS = re.compile(r"^requirements[^/]*\.txt$")
WALK_SKIP = ("node_modules", "dist", "vendor", "__pycache__")


def _is_manifest(name: str) -> bool:
    return name in MANIFEST_NAMES or bool(MANIFEST_REQUIREMENTS.match(name))


def repo_manifests(root: Path) -> tuple[list[str], str]:
    """Dependency manifests that are PART OF THE REPOSITORY.

    When ``root`` is the top level of a git work tree, the tracked files are the
    manifests we ship — a module cache or a virtualenv inside the tree is not a
    manifest anyone delivers. Anywhere else (a test tree, an export) every
    non-hidden file is walked. Returns the sorted relative paths and the
    enumeration source, which the verdict reports so nobody has to guess.
    """
    names: list[str] | None = None
    source = "walk"
    try:
        top = subprocess.run(["git", "-C", str(root), "rev-parse", "--show-toplevel"], text=True, capture_output=True, check=True).stdout.strip()
        if top and Path(top).resolve() == root.resolve():
            listed = subprocess.run(["git", "-C", str(root), "ls-files", "-z"], capture_output=True, check=True)
            names = [n for n in listed.stdout.decode("utf-8", "replace").split("\0") if n]
            source = "git ls-files"
    except (OSError, subprocess.CalledProcessError):
        names = None
    if names is None:
        names = []
        for path in root.rglob("*"):
            if not path.is_file():
                continue
            rel = path.relative_to(root).as_posix()
            if any(segment.startswith(".") or segment in WALK_SKIP for segment in rel.split("/")):
                continue
            names.append(rel)
    found = sorted(rel for rel in names if _is_manifest(rel.rsplit("/", 1)[-1]) and (root / rel).is_file())
    return found, source


def check_manifests(root: Path, process: dict[str, Any]) -> dict[str, Any]:
    """Every dependency manifest in the tree is in a scanner's scope, in a
    Dependabot directory, or excluded by name with a reason.

    The gate enumerates the manifests itself: an unlisted one is red, an
    exclusion whose file no longer exists is red. Learned on 2026-09-06 (#696):
    three clean scans while GitHub held 8 advisories on an unscanned fixture.
    """
    problems: list[str] = []
    scanners = process.get("scanners") if isinstance(process.get("scanners"), dict) else {}
    govuln = scanners.get("govulncheck") if isinstance(scanners.get("govulncheck"), dict) else {}
    pip = scanners.get("pip_audit") if isinstance(scanners.get("pip_audit"), dict) else {}
    dependabot = scanners.get("dependabot") if isinstance(scanners.get("dependabot"), dict) else {}
    scanned_dirs = {str(module).strip("/") for module in (govuln.get("modules") or []) if isinstance(module, str)}
    scanned_files = {str(pip.get("requirements")).strip("/")} if isinstance(pip.get("requirements"), str) else set()
    watched_dirs: set[str] = set()
    config = _rel(root, dependabot.get("config"))
    if config is not None and config.is_file():
        try:
            loaded = load_yaml(config)
        except yaml.YAMLError:
            loaded = None  # named by the process check; nothing is watched then
        for update in (loaded.get("updates") or []) if isinstance(loaded, dict) else []:
            if isinstance(update, dict) and isinstance(update.get("directory"), str):
                watched_dirs.add(update["directory"].strip("/"))

    manifests = process.get("manifests") if isinstance(process.get("manifests"), dict) else {}
    excluded: dict[str, str] = {}
    for index, item in enumerate(manifests.get("not_scanned") or []):
        if not isinstance(item, dict) or not str(item.get("path") or "").strip():
            problems.append(f"manifests.not_scanned[{index}]: path is required")
            continue
        path = str(item["path"]).strip("/")
        if len(str(item.get("reason") or "").strip()) < MIN_JUSTIFICATION:
            problems.append(f"manifests.not_scanned: {path}: reason must say why it is not scanned (at least {MIN_JUSTIFICATION} characters)")
        if path in excluded:
            problems.append(f"manifests.not_scanned: {path} listed twice")
        excluded[path] = str(item.get("reason") or "")
        if not (root / path).is_file():
            problems.append(f"manifests.not_scanned: {path} does not exist — remove the stale exclusion")

    found, source = repo_manifests(root)
    coverage: list[dict[str, str]] = []
    for rel in found:
        parent = rel.rsplit("/", 1)[0] if "/" in rel else ""
        if rel in scanned_files or parent in scanned_dirs:
            how = "scanner"
        elif parent in watched_dirs:
            how = "dependabot"
        elif rel in excluded:
            how = "not_scanned"
        else:
            how = "uncovered"
            problems.append(f"{rel}: neither scanned, watched by Dependabot, nor listed in manifests.not_scanned with a reason")
        coverage.append({"path": rel, "covered_by": how})
    detail = {
        "source": source,
        "manifests": coverage,
        "uncovered": [c["path"] for c in coverage if c["covered_by"] == "uncovered"],
    }
    return check("manifests", problems, detail)


# --- main ---------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate the security process, the expiring allowlist, the generated Supported Versions section, and run the dependency scanners.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--process", default=str(DEFAULT_PROCESS), help="Security process file.")
    parser.add_argument("--allowlist", default=None, help="Override the allowlist path declared by the process (tests).")
    parser.add_argument("--today", default=None, help="Reference date YYYY-MM-DD for expiry checks (default: today).")
    parser.add_argument("--check", action="store_true", help="Run the static checks (process, allowlist, supported versions). Always on; kept for readability of CI steps.")
    parser.add_argument("--write", action="store_true", help="Regenerate the Supported Versions section instead of failing on drift.")
    parser.add_argument("--scan", default="", help="Comma-separated scanners to run: govulncheck,pip-audit.")
    parser.add_argument("--go-module", action="append", default=None, help="Override the Go module directories to scan (repeatable).")
    parser.add_argument("--requirements", default=None, help="Override the requirements file pip-audit audits.")
    parser.add_argument("--govulncheck-cmd", default=None, help="Command for govulncheck (default: PATH, then $GOPATH/bin).")
    parser.add_argument("--pip-audit-cmd", default=None, help="Command for pip-audit (default: PATH, then python -m pip_audit, then uvx pip-audit).")
    parser.add_argument("--timeout", type=float, default=DEFAULT_TIMEOUT, help="Seconds allowed per scanner invocation.")
    parser.add_argument("--report", default=None, help="Write the JSON verdict here as well as stdout.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    process_path = root / args.process if not Path(args.process).is_absolute() else Path(args.process)
    if not process_path.is_file():
        print(f"security gate: process file not found: {process_path}", file=sys.stderr)
        return 2
    today = date.today()
    if args.today:
        try:
            today = date.fromisoformat(args.today)
        except ValueError:
            print(f"security gate: --today must be YYYY-MM-DD, got {args.today!r}", file=sys.stderr)
            return 2
    scanners = [s.strip() for s in args.scan.split(",") if s.strip()]
    unknown = [s for s in scanners if s not in KNOWN_SCANNERS]
    if unknown:
        print(f"security gate: unknown scanner(s) {unknown}; known: {KNOWN_SCANNERS}", file=sys.stderr)
        return 2

    try:
        process = load_yaml(process_path)
    except yaml.YAMLError as exc:
        print(f"security gate: {process_path}: not YAML ({exc})", file=sys.stderr)
        return 2
    checks = [check_process(root, process, process_path)]
    process_dict = process if isinstance(process, dict) else {}
    allowlist_check, active = check_allowlist(root, process_dict, today, Path(args.allowlist) if args.allowlist else None)
    checks.append(allowlist_check)
    checks.append(check_supported_versions(root, process_dict, args.write))
    checks.append(check_manifests(root, process_dict))
    if "govulncheck" in scanners:
        checks.append(check_govulncheck(root, process_dict, active, resolve_govulncheck(args.govulncheck_cmd), args.go_module, args.timeout))
    if "pip-audit" in scanners:
        checks.append(check_pip_audit(root, process_dict, active, resolve_pip_audit(args.pip_audit_cmd), Path(args.requirements) if args.requirements else None, args.timeout))

    status = "pass" if all(c["status"] == "pass" for c in checks) else "fail"
    verdict = {
        "schema_version": VERDICT_SCHEMA,
        "status": status,
        "today": today.isoformat(),
        "claim_boundary": str(process_dict.get("claim_boundary") or "dependencies are scanned and accepted findings expire; no security certification"),
        "process": str(process_path),
        "scanners_requested": scanners,
        "checks": checks,
    }
    encoded = json.dumps(verdict, indent=2, sort_keys=True)
    if args.report:
        Path(args.report).write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    if status == "fail":
        for item in checks:
            for problem in item["problems"]:
                print(f"security gate: {item['name']}: {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
