#!/usr/bin/env python3
"""NRT-026 (#679) — the support model, declared and checked.

``docs/support-model.yaml`` (nomos-support-model-v1) declares what Nomos
supports: versions and their lifecycle state, channels, response targets
(declared, never measured), tested platforms, toolchain versions, the surfaces
explicitly NOT supported, and the end-of-support rule. This guard refuses a
declaration the repository does not honour:

1. model     — schema and required fields; response targets must say
   ``declared_not_measured``; the hosted service, the control plane and the
   GitHub App must be declared unsupported; every rendered file exists and
   carries the generated-section markers.
2. platforms — ``tested_platforms.runners`` equals the CI matrix
   (``strategy.matrix.os`` of ci.yml), as a set.
3. toolchain — Go language and toolchain versions equal ``cli/go.mod``; CUE
   and Python versions equal what ci.yml sets up.
4. versions  — ``current_candidate`` equals the CLI ``Version`` constant;
   every declared version is a git tag or the candidate; every ``v*`` tag is
   declared; ``released_on`` equals the CHANGELOG heading date.
5. rendered  — the Support section GENERATED between markers into every file
   of ``rendered_in`` matches the model; ``--write`` regenerates it.

Exit 0 = every check passed; 1 = a check failed (named on stderr and in the
JSON verdict); 2 = usage error.

Claim boundary: "support is declared and consistent with what CI tests" —
never "contractually guaranteed", never an SLA.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from datetime import date, datetime
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for the support model guard.", file=sys.stderr)
    raise SystemExit(2) from exc


MODEL_SCHEMA = "nomos-support-model-v1"
VERDICT_SCHEMA = "nomos-support-model-guard-v1"
DEFAULT_MODEL = Path("docs/support-model.yaml")
DEFAULT_CI = Path(".github/workflows/ci.yml")
DEFAULT_GO_MOD = Path("cli/go.mod")
DEFAULT_CHANGELOG = Path("CHANGELOG.md")
DEFAULT_VERSION_FILE = Path("cli/internal/app/app.go")
MARK_BEGIN = "<!-- support-model:begin -->"
MARK_END = "<!-- support-model:end -->"
STATES = ("supported", "superseded", "unsupported", "candidate")
REQUIRED_UNSUPPORTED = ("hosted service", "control plane", "github app")
CHANGELOG_HEADING = re.compile(r"^## (v\S+) - (\d{4}-\d{2}-\d{2})\s*$", re.MULTILINE)
VERSION_CONST = re.compile(r'const Version = "([^"]+)"')
GO_DIRECTIVE = re.compile(r"^go (\S+)\s*$", re.MULTILINE)
TOOLCHAIN_DIRECTIVE = re.compile(r"^toolchain (\S+)\s*$", re.MULTILINE)


# --- helpers ---------------------------------------------------------------------


def load_yaml(path: Path) -> Any:
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def check(name: str, problems: list[str], detail: dict[str, Any] | None = None) -> dict[str, Any]:
    return {"name": name, "status": "fail" if problems else "pass", "problems": problems, "detail": detail or {}}


def _date_str(value: Any) -> str:
    if isinstance(value, datetime):
        return value.date().isoformat()
    if isinstance(value, date):
        return value.isoformat()
    return str(value) if value is not None else ""


def _versions(model: dict[str, Any]) -> list[dict[str, Any]]:
    return [v for v in (model.get("supported_versions") or []) if isinstance(v, dict)]


# --- 1. model ------------------------------------------------------------------------


def check_model(root: Path, model: Any) -> dict[str, Any]:
    problems: list[str] = []
    if not isinstance(model, dict):
        return check("model", ["support model is not a mapping"])
    if model.get("schema_version") != MODEL_SCHEMA:
        problems.append(f"schema_version must be {MODEL_SCHEMA!r}, got {model.get('schema_version')!r}")
    if not str(model.get("claim_boundary") or "").strip():
        problems.append("claim_boundary is required")
    candidate = str(model.get("current_candidate") or "")
    if not candidate.startswith("v"):
        problems.append("current_candidate must be the candidate version with a 'v' prefix")
    versions = _versions(model)
    if not versions:
        problems.append("supported_versions must list at least one version")
    seen: set[str] = set()
    for index, entry in enumerate(versions):
        label = str(entry.get("version") or f"supported_versions[{index}]")
        if not str(entry.get("version") or "").startswith("v"):
            problems.append(f"{label}: version must carry a 'v' prefix")
        if label in seen:
            problems.append(f"{label}: declared twice")
        seen.add(label)
        if entry.get("state") not in STATES:
            problems.append(f"{label}: state must be one of {STATES}, got {entry.get('state')!r}")
        for field in ("released_on", "security_support", "end_of_support"):
            if not _date_str(entry.get(field)).strip():
                problems.append(f"{label}: {field} is required")
    if not str(model.get("end_of_support_rule") or "").strip():
        problems.append("end_of_support_rule is required")

    channels = model.get("channels")
    if not isinstance(channels, list) or not channels:
        problems.append("channels must list at least one channel")
    else:
        for index, channel in enumerate(channels):
            if not isinstance(channel, dict) or not channel.get("kind") or not channel.get("ref"):
                problems.append(f"channels[{index}] needs kind and ref")
    targets = model.get("response_targets") if isinstance(model.get("response_targets"), dict) else {}
    if targets.get("status") != "declared_not_measured":
        problems.append("response_targets.status must be 'declared_not_measured': targets are declared, no measurement record backs them")
    known_channels = {str(c.get("kind")) for c in channels or [] if isinstance(c, dict)}
    for index, target in enumerate(targets.get("targets") or []):
        if not isinstance(target, dict):
            problems.append(f"response_targets.targets[{index}] is not a mapping")
            continue
        if str(target.get("channel")) not in known_channels:
            problems.append(f"response_targets.targets[{index}]: channel {target.get('channel')!r} is not declared in channels")
        days = target.get("first_response_within_days")
        defined_in = target.get("defined_in")
        if defined_in:
            if not (root / str(defined_in)).is_file():
                problems.append(f"response_targets.targets[{index}]: defined_in {defined_in!r} does not exist")
        elif not isinstance(days, int) or isinstance(days, bool) or days <= 0:
            problems.append(f"response_targets.targets[{index}]: first_response_within_days must be a positive integer or defined_in must point at a file")

    platforms = model.get("tested_platforms") if isinstance(model.get("tested_platforms"), dict) else {}
    if not isinstance(platforms.get("runners"), list) or not platforms.get("runners"):
        problems.append("tested_platforms.runners must list the CI runners")
    toolchain = model.get("toolchain") if isinstance(model.get("toolchain"), dict) else {}
    go = toolchain.get("go") if isinstance(toolchain.get("go"), dict) else {}
    if not str(go.get("language_version") or "").strip():
        problems.append("toolchain.go.language_version is required")
    cue = toolchain.get("cue") if isinstance(toolchain.get("cue"), dict) else {}
    if not str(cue.get("version") or "").strip():
        problems.append("toolchain.cue.version is required")
    python = toolchain.get("python") if isinstance(toolchain.get("python"), dict) else {}
    if not isinstance(python.get("versions"), list) or not python.get("versions"):
        problems.append("toolchain.python.versions is required")

    surfaces = model.get("unsupported_surfaces")
    names: list[str] = []
    if not isinstance(surfaces, list) or not surfaces:
        problems.append("unsupported_surfaces must list the surfaces that are explicitly not supported")
    else:
        for index, surface in enumerate(surfaces):
            if not isinstance(surface, dict) or not surface.get("surface") or not surface.get("reason"):
                problems.append(f"unsupported_surfaces[{index}] needs surface and reason")
            else:
                names.append(str(surface["surface"]).lower())
    for required in REQUIRED_UNSUPPORTED:
        if not any(required in name for name in names):
            problems.append(f"unsupported_surfaces must declare '{required}' unsupported")

    rendered_in = model.get("rendered_in")
    if not isinstance(rendered_in, list) or not rendered_in:
        problems.append("rendered_in must list the files carrying the generated Support section")
    else:
        for target in rendered_in:
            path = root / str(target)
            if not path.is_file():
                problems.append(f"rendered_in: {target!r} does not exist")
            else:
                text = path.read_text(encoding="utf-8")
                if MARK_BEGIN not in text or MARK_END not in text:
                    problems.append(f"rendered_in: {target!r} lacks the generated-section markers")
    return check("model", problems, {"versions": len(versions), "channels": len(channels or []), "unsupported_surfaces": names})


# --- 2. platforms ------------------------------------------------------------------


def ci_matrix_runners(ci: dict[str, Any]) -> set[str]:
    runners: set[str] = set()
    for job in (ci.get("jobs") or {}).values():
        if not isinstance(job, dict):
            continue
        matrix = ((job.get("strategy") or {}).get("matrix") or {})
        for runner in matrix.get("os") or []:
            runners.add(str(runner))
    return runners


def check_platforms(model: dict[str, Any], ci: dict[str, Any] | None, ci_path: Path) -> dict[str, Any]:
    if ci is None:
        return check("platforms", [f"{ci_path}: CI workflow unreadable"])
    declared = {str(r) for r in ((model.get("tested_platforms") or {}).get("runners") or [])}
    actual = ci_matrix_runners(ci)
    problems: list[str] = []
    if not actual:
        problems.append(f"{ci_path}: no strategy.matrix.os found")
    elif declared != actual:
        problems.append(f"tested_platforms.runners {sorted(declared)} differ from the CI matrix {sorted(actual)}")
    return check("platforms", problems, {"declared": sorted(declared), "ci_matrix": sorted(actual)})


# --- 3. toolchain ------------------------------------------------------------------


def ci_setup_versions(ci: dict[str, Any], action_prefix: str, key: str) -> set[str]:
    found: set[str] = set()
    for job in (ci.get("jobs") or {}).values():
        if not isinstance(job, dict):
            continue
        for step in job.get("steps") or []:
            if not isinstance(step, dict):
                continue
            if str(step.get("uses", "")).startswith(action_prefix):
                value = (step.get("with") or {}).get(key)
                if value is not None:
                    found.add(str(value))
    return found


def check_toolchain(root: Path, model: dict[str, Any], ci: dict[str, Any] | None, go_mod_path: Path) -> dict[str, Any]:
    problems: list[str] = []
    toolchain = model.get("toolchain") if isinstance(model.get("toolchain"), dict) else {}
    go = toolchain.get("go") if isinstance(toolchain.get("go"), dict) else {}
    detail: dict[str, Any] = {}
    if not go_mod_path.is_file():
        problems.append(f"{go_mod_path}: missing")
    else:
        text = go_mod_path.read_text(encoding="utf-8")
        language = GO_DIRECTIVE.search(text)
        directive = TOOLCHAIN_DIRECTIVE.search(text)
        actual_language = language.group(1) if language else ""
        actual_toolchain = directive.group(1) if directive else ""
        detail["go_mod"] = {"go": actual_language, "toolchain": actual_toolchain}
        if str(go.get("language_version") or "") != actual_language:
            problems.append(f"toolchain.go.language_version {str(go.get('language_version'))!r} differs from {go_mod_path.name} 'go {actual_language}'")
        declared_toolchain = str(go.get("toolchain_version") or "")
        if declared_toolchain != actual_toolchain:
            problems.append(f"toolchain.go.toolchain_version {declared_toolchain!r} differs from {go_mod_path.name} 'toolchain {actual_toolchain or '(none)'}'")
    if ci is None:
        problems.append("CI workflow unreadable: CUE and Python versions cannot be checked")
    else:
        cue_versions = ci_setup_versions(ci, "cue-lang/setup-cue", "version")
        cue = toolchain.get("cue") if isinstance(toolchain.get("cue"), dict) else {}
        detail["ci_cue"] = sorted(cue_versions)
        if cue_versions != {str(cue.get("version") or "")}:
            problems.append(f"toolchain.cue.version {str(cue.get('version'))!r} differs from the CI setup-cue version(s) {sorted(cue_versions)}")
        python_versions = ci_setup_versions(ci, "actions/setup-python", "python-version")
        python = toolchain.get("python") if isinstance(toolchain.get("python"), dict) else {}
        declared_python = {str(v) for v in (python.get("versions") or [])}
        detail["ci_python"] = sorted(python_versions)
        if python_versions != declared_python:
            problems.append(f"toolchain.python.versions {sorted(declared_python)} differ from the CI setup-python version(s) {sorted(python_versions)}")
    return check("toolchain", problems, detail)


# --- 4. versions -------------------------------------------------------------------


def git_tags(root: Path) -> list[str] | None:
    try:
        proc = subprocess.run(["git", "-C", str(root), "tag", "-l", "v*"], text=True, capture_output=True, check=False)
    except OSError:
        return None
    if proc.returncode != 0:
        return None
    return sorted(t.strip() for t in proc.stdout.splitlines() if t.strip())


def cli_candidate(version_file: Path) -> str | None:
    if not version_file.is_file():
        return None
    match = VERSION_CONST.search(version_file.read_text(encoding="utf-8"))
    if not match:
        return None
    value = match.group(1)
    return value if value.startswith("v") else "v" + value


def check_versions(root: Path, model: dict[str, Any], tags: list[str] | None, version_file: Path, changelog: Path) -> dict[str, Any]:
    problems: list[str] = []
    candidate = cli_candidate(version_file)
    declared_candidate = str(model.get("current_candidate") or "")
    if candidate is None:
        problems.append(f"{version_file}: CLI Version constant not found")
    elif declared_candidate != candidate:
        problems.append(f"current_candidate {declared_candidate!r} differs from the CLI Version constant {candidate!r} in {version_file}")
    if tags is None:
        problems.append("git tags unavailable: run inside a git checkout with tags fetched (git fetch --tags) or pass --tags")
        tags = []
    headings = dict(CHANGELOG_HEADING.findall(changelog.read_text(encoding="utf-8"))) if changelog.is_file() else {}
    declared_versions = {str(v.get("version")): v for v in _versions(model)}
    for version, entry in declared_versions.items():
        if version not in tags and version != candidate:
            problems.append(f"{version}: neither a git tag nor the current candidate ({candidate})")
        released = _date_str(entry.get("released_on"))
        if version in headings:
            if released != headings[version]:
                problems.append(f"{version}: released_on {released} differs from the {changelog.name} heading date {headings[version]}")
        elif entry.get("state") != "candidate":
            problems.append(f"{version}: no '## {version} - YYYY-MM-DD' heading in {changelog.name}")
    for tag in tags:
        if tag not in declared_versions:
            problems.append(f"tag {tag} exists but is not declared in supported_versions")
    return check("versions", problems, {"tags": tags, "candidate": candidate, "declared": sorted(declared_versions)})


# --- 5. rendered -------------------------------------------------------------------


def render(model: dict[str, Any]) -> str:
    lines = [MARK_BEGIN, "<!-- GENERATED from docs/support-model.yaml by scripts/support_model_guard.py --write; do not edit by hand, CI fails on drift -->", ""]
    lines += ["| Version | Released | State | Security support | End of support |", "|---|---|---|---|---|"]
    for entry in _versions(model):
        lines.append(f"| `{entry.get('version')}` | {_date_str(entry.get('released_on'))} | {entry.get('state')} | {entry.get('security_support')} | {_date_str(entry.get('end_of_support'))} |")
    lines.append("")
    lines.append(f"- Current candidate: `{model.get('current_candidate')}` (the CLI `Version` constant).")
    channels = "; ".join(f"{c.get('kind')} — {c.get('ref')} ({c.get('scope')})" for c in (model.get("channels") or []) if isinstance(c, dict))
    lines.append(f"- Channels: {channels}.")
    targets = model.get("response_targets") if isinstance(model.get("response_targets"), dict) else {}
    rendered_targets = []
    for target in targets.get("targets") or []:
        if not isinstance(target, dict):
            continue
        if target.get("defined_in"):
            rendered_targets.append(f"{target.get('channel')} — per {target.get('defined_in')}")
        else:
            rendered_targets.append(f"{target.get('channel')} — first response within {target.get('first_response_within_days')} days")
    lines.append(f"- Response targets ({str(targets.get('status', '')).replace('_', ', ')}): {'; '.join(rendered_targets)}.")
    runners = ", ".join(str(r) for r in ((model.get("tested_platforms") or {}).get("runners") or []))
    lines.append(f"- Tested platforms (CI matrix): {runners}.")
    toolchain = model.get("toolchain") if isinstance(model.get("toolchain"), dict) else {}
    go = toolchain.get("go") if isinstance(toolchain.get("go"), dict) else {}
    cue = toolchain.get("cue") if isinstance(toolchain.get("cue"), dict) else {}
    python = toolchain.get("python") if isinstance(toolchain.get("python"), dict) else {}
    go_text = f"Go {go.get('language_version')} (language)"
    if go.get("toolchain_version"):
        go_text += f" / {go.get('toolchain_version')} (toolchain)"
    lines.append(f"- Toolchain: {go_text} from {go.get('source', 'cli/go.mod')}; CUE {cue.get('version')}; Python {', '.join(str(v) for v in (python.get('versions') or []))}.")
    surfaces = "; ".join(f"{s.get('surface')} ({s.get('reason')})" for s in (model.get("unsupported_surfaces") or []) if isinstance(s, dict))
    lines.append(f"- Not supported: {surfaces}.")
    lines.append(f"- End of support: {str(model.get('end_of_support_rule') or '').strip()}")
    lines.append(MARK_END)
    return "\n".join(lines)


def _replace_section(text: str, rendered: str) -> str | None:
    start = text.find(MARK_BEGIN)
    end = text.find(MARK_END)
    if start == -1 or end == -1 or end < start:
        return None
    return text[:start] + rendered + text[end + len(MARK_END):]


def check_rendered(root: Path, model: dict[str, Any], write: bool) -> dict[str, Any]:
    problems: list[str] = []
    written: list[str] = []
    rendered = render(model)
    for target in model.get("rendered_in") or []:
        path = root / str(target)
        if not path.is_file():
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
                problems.append(f"{target}: the generated Support section drifted from docs/support-model.yaml — run scripts/support_model_guard.py --write")
    return check("rendered", problems, {"written": written})


# --- main ---------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(description="Check (or regenerate) the declared support model against the CI matrix, the toolchain, the tags and the generated Support sections.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--model", default=str(DEFAULT_MODEL), help="Support model YAML.")
    parser.add_argument("--ci", default=str(DEFAULT_CI), help="CI workflow carrying the matrix and tool setup steps.")
    parser.add_argument("--go-mod", default=str(DEFAULT_GO_MOD), help="Go module whose directives the model must match.")
    parser.add_argument("--changelog", default=str(DEFAULT_CHANGELOG), help="Changelog carrying the release headings.")
    parser.add_argument("--version-file", default=str(DEFAULT_VERSION_FILE), help="Go file carrying the CLI Version constant.")
    parser.add_argument("--tags", default=None, help="Comma-separated tag list overriding `git tag` (tests, shallow checkouts).")
    parser.add_argument("--check", action="store_true", help="Run the checks (always on; kept for readability of CI steps).")
    parser.add_argument("--write", action="store_true", help="Regenerate the Support sections instead of failing on drift.")
    parser.add_argument("--report", default=None, help="Write the JSON verdict here as well as stdout.")
    args = parser.parse_args()

    root = Path(args.root).resolve()

    def resolve(value: str) -> Path:
        path = Path(value)
        return path if path.is_absolute() else root / path

    model_path = resolve(args.model)
    if not model_path.is_file():
        print(f"support model guard: model not found: {model_path}", file=sys.stderr)
        return 2
    try:
        model = load_yaml(model_path)
    except yaml.YAMLError as exc:
        print(f"support model guard: {model_path}: not YAML ({exc})", file=sys.stderr)
        return 2
    ci_path = resolve(args.ci)
    ci: dict[str, Any] | None = None
    if ci_path.is_file():
        try:
            loaded = load_yaml(ci_path)
            ci = loaded if isinstance(loaded, dict) else None
        except yaml.YAMLError:
            ci = None
    tags = [t.strip() for t in args.tags.split(",") if t.strip()] if args.tags is not None else git_tags(root)

    model_dict = model if isinstance(model, dict) else {}
    checks = [
        check_model(root, model),
        check_platforms(model_dict, ci, ci_path),
        check_toolchain(root, model_dict, ci, resolve(args.go_mod)),
        check_versions(root, model_dict, tags, resolve(args.version_file), resolve(args.changelog)),
        check_rendered(root, model_dict, args.write),
    ]
    status = "pass" if all(c["status"] == "pass" for c in checks) else "fail"
    verdict = {
        "schema_version": VERDICT_SCHEMA,
        "status": status,
        "model": str(model_path),
        "claim_boundary": str(model_dict.get("claim_boundary") or "support is declared and checked; no contractual guarantee"),
        "checks": checks,
    }
    encoded = json.dumps(verdict, indent=2, sort_keys=True)
    if args.report:
        Path(args.report).write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    if status == "fail":
        for item in checks:
            for problem in item["problems"]:
                print(f"support model guard: {item['name']}: {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
