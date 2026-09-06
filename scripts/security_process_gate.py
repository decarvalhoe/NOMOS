#!/usr/bin/env python3
"""NRT-025 (#678) — the security process, executable.

  --check              validate docs/security/security-process.yaml and the
                       vulnerability allowlist (expired or undated entry → red),
                       and require SECURITY.md's generated section to be fresh.
  --govulncheck FILE   read govulncheck -json output; every finding not covered
                       by a live allowlist entry → red (named).
  --pip-audit FILE     read pip-audit -f json output; same rule.
  --emit-docs          regenerate the "Supported Versions" section of
                       SECURITY.md from CHANGELOG.md (latest release =
                       best-effort alpha triage, earlier = superseded).

Exit codes: 0 ok · 1 gate red · 4 generated docs drift · 2 usage.
The gate reads scanner output; it never edits the allowlist, and the allowlist
never silences a finding after its expiry.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import date, datetime
from pathlib import Path
from typing import Any

import yaml

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
PROCESS = Path("docs/security/security-process.yaml")
ALLOWLIST = Path("docs/security/vulnerability-allowlist.yaml")
SECURITY_MD = Path("SECURITY.md")
CHANGELOG = Path("CHANGELOG.md")
BEGIN, END = "<!-- supported-versions:begin -->", "<!-- supported-versions:end -->"
REQUIRED_ENTRY_FIELDS = ("id", "ecosystem", "package", "justification", "owner", "accepted_on", "expires_on")


def load_yaml(root: Path, rel: Path) -> Any:
    p = root / rel
    if not p.exists():
        raise SystemExit(f"security-gate: {rel} is missing")
    return yaml.safe_load(p.read_text(encoding="utf-8"))


def as_date(v: Any) -> date | None:
    if isinstance(v, date):
        return v
    if isinstance(v, datetime):
        return v.date()
    try:
        return date.fromisoformat(str(v))
    except Exception:
        return None


def validate_process(doc: Any) -> list[str]:
    errs = []
    if not isinstance(doc, dict) or doc.get("schema_version") != "nomos-security-process-v1":
        return ["security-process.yaml: schema_version must be nomos-security-process-v1"]
    for key in ("owner_role", "intake", "disclosure", "scanners", "dependency_updates", "allowlist", "sop", "claim_boundary"):
        if not doc.get(key):
            errs.append(f"security-process.yaml: {key} is required")
    intake = doc.get("intake") or {}
    for key in ("channel", "acknowledgement_target_days", "triage_target_days", "fix_or_mitigation_target_days_by_severity"):
        if key not in intake:
            errs.append(f"security-process.yaml: intake.{key} is required")
    sev = (intake.get("fix_or_mitigation_target_days_by_severity") or {})
    for s in ("critical", "high", "medium", "low"):
        if s not in sev:
            errs.append(f"security-process.yaml: fix target for severity {s} is required")
    scanners = doc.get("scanners") or []
    ids = {s.get("id") for s in scanners if isinstance(s, dict)}
    for must in ("govulncheck", "pip-audit"):
        if must not in ids:
            errs.append(f"security-process.yaml: scanner {must} must be declared")
    for s in scanners:
        if not isinstance(s, dict) or s.get("gate") is not True:
            errs.append(f"security-process.yaml: scanner {s.get('id') if isinstance(s, dict) else s!r} must be a gate (gate: true)")
    for i, m in enumerate(doc.get("manifests_not_scanned") or []):
        if not isinstance(m, dict) or not str(m.get("path", "")).strip() or not str(m.get("reason", "")).strip():
            errs.append(f"security-process.yaml: manifests_not_scanned #{i + 1} needs path and reason")
    return errs


MANIFEST_NAMES = ("package.json", "pyproject.toml", "go.mod", "Cargo.toml", "Gemfile", "pom.xml")


def repo_manifests(root: Path) -> list[str]:
    out = []
    for p in root.rglob("*"):
        if not p.is_file():
            continue
        rel = p.relative_to(root).as_posix()
        if any(seg in ("node_modules", ".git", ".venv", "dist") for seg in rel.split("/")):
            continue
        if p.name in MANIFEST_NAMES or (p.name.startswith("requirements") and p.suffix == ".txt"):
            out.append(rel)
    return sorted(out)


def manifest_coverage(root: Path, doc: dict) -> list[str]:
    """Every dependency manifest is scanned, watched by Dependabot, or excluded by name with a reason."""
    scanned: set[str] = set()
    for s in doc.get("scanners") or []:
        for scope in (s.get("scope") or []) if isinstance(s, dict) else []:
            scanned.add(str(scope).rstrip("/"))
    dirs: set[str] = set()
    dep = root / str(doc.get("dependency_updates") or "")
    if dep.is_file():
        for u in (yaml.safe_load(dep.read_text(encoding="utf-8")) or {}).get("updates") or []:
            dirs.add(str(u.get("directory", "")).strip("/"))
    excluded = {str(m.get("path")) for m in doc.get("manifests_not_scanned") or [] if isinstance(m, dict)}
    errs = []
    for rel in repo_manifests(root):
        parent = rel.rsplit("/", 1)[0] if "/" in rel else ""
        if rel in scanned or parent in scanned or parent in dirs or rel in excluded:
            continue
        errs.append(f"manifest {rel} is neither scanned, watched by Dependabot, nor listed in manifests_not_scanned with a reason")
    for rel in excluded:
        if not (root / rel).exists():
            errs.append(f"manifests_not_scanned lists {rel}, which does not exist — remove the stale exclusion")
    return errs


def live_allowlist(root: Path, today: date) -> tuple[dict[str, dict], list[str]]:
    doc = load_yaml(root, ALLOWLIST)
    errs: list[str] = []
    live: dict[str, dict] = {}
    if not isinstance(doc, dict) or doc.get("schema_version") != "nomos-vulnerability-allowlist-v1":
        return {}, ["vulnerability-allowlist.yaml: schema_version must be nomos-vulnerability-allowlist-v1"]
    for i, e in enumerate(doc.get("entries") or []):
        where = f"allowlist entry #{i + 1}"
        if not isinstance(e, dict):
            errs.append(f"{where}: not a mapping"); continue
        missing = [f for f in REQUIRED_ENTRY_FIELDS if not str(e.get(f, "")).strip()]
        if missing:
            errs.append(f"{where} ({e.get('id', '?')}): missing {', '.join(missing)}"); continue
        exp, acc = as_date(e["expires_on"]), as_date(e["accepted_on"])
        if exp is None or acc is None:
            errs.append(f"{where} ({e['id']}): accepted_on/expires_on must be ISO dates"); continue
        if exp <= acc:
            errs.append(f"{where} ({e['id']}): expires_on must be after accepted_on")
        if exp < today:
            errs.append(f"{where} ({e['id']}): expired on {exp} — an expired acceptance is a red finding, not a silence")
            continue
        live[str(e["id"])] = e
    return live, errs


def govulncheck_findings(path: Path) -> list[dict[str, str]]:
    out: dict[str, dict[str, str]] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            d = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(d, dict):
            continue
        f = d.get("finding")
        if not f or not f.get("osv"):
            continue
        trace = f.get("trace") or [{}]
        called = bool(trace and trace[0].get("function"))
        mod = trace[0].get("module", "?") if trace else "?"
        cur = out.setdefault(f["osv"], {"id": f["osv"], "package": mod, "called": "false"})
        if called:
            cur["called"] = "true"
    return sorted(out.values(), key=lambda x: x["id"])


def pip_audit_findings(path: Path) -> list[dict[str, str]]:
    d = json.loads(path.read_text(encoding="utf-8"))
    out = []
    for dep in d.get("dependencies", []):
        for v in dep.get("vulns", []) or []:
            out.append({"id": v.get("id", "?"), "package": f"{dep.get('name')}=={dep.get('version')}", "fix_versions": ",".join(v.get("fix_versions") or [])})
    return sorted(out, key=lambda x: x["id"])


def gate(findings: list[dict[str, str]], live: dict[str, dict], scanner: str) -> list[str]:
    red = []
    for f in findings:
        if f["id"] in live:
            continue
        red.append(f"{scanner}: {f['id']} in {f['package']} is not accepted (allowlist entry with expiry required, or fix it)")
    return red


def release_versions(root: Path) -> list[tuple[str, str]]:
    text = (root / CHANGELOG).read_text(encoding="utf-8")
    return re.findall(r"^## (v[0-9]+\.[0-9]+\.[0-9]+[A-Za-z0-9.-]*) - ([0-9]{4}-[0-9]{2}-[0-9]{2})", text, flags=re.M)


def render_supported(root: Path) -> str:
    versions = release_versions(root)
    lines = [BEGIN, "<!-- GENERATED from CHANGELOG.md by scripts/security_process_gate.py --emit-docs; do not edit by hand, CI fails on drift -->",
             "| Version | Security support |", "|---|---|"]
    for i, (v, d) in enumerate(versions):
        lines.append(f"| `{v}` ({d}) | {'Best-effort alpha triage' if i == 0 else 'Superseded; not supported'} |")
    lines.append(f"| earlier | Not supported |")
    lines.append(END)
    return "\n".join(lines)


def splice(text: str, block: str) -> str:
    if BEGIN in text and END in text:
        pre, rest = text.split(BEGIN, 1)
        _, post = rest.split(END, 1)
        return pre + block + post
    # First emission: replace everything under "## Supported Versions" up to the
    # next heading (the hand-written table) with the generated block.
    m = re.search(r"## Supported Versions\n\n", text)
    if not m:
        return text.rstrip("\n") + "\n\n## Supported Versions\n\n" + block + "\n"
    nxt = re.search(r"\n## ", text[m.end():])
    end = m.end() + nxt.start() + 1 if nxt else len(text)
    return text[: m.end()] + block + "\n\n" + text[end:].lstrip("\n")


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=str(ROOT_DEFAULT))
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--govulncheck", action="append", default=[], help="govulncheck -json output file (repeatable)")
    ap.add_argument("--pip-audit", action="append", default=[], help="pip-audit -f json output file (repeatable)")
    ap.add_argument("--emit-docs", action="store_true")
    ap.add_argument("--today", default="", help="ISO date for tests (default: today)")
    args = ap.parse_args()
    root = Path(args.root).resolve()
    today = date.fromisoformat(args.today) if args.today else date.today()
    process = load_yaml(root, PROCESS)
    errs = validate_process(process)
    if not errs:
        errs += manifest_coverage(root, process)
    live, aerrs = live_allowlist(root, today)
    errs += aerrs
    for f in args.govulncheck:
        errs += gate(govulncheck_findings(Path(f)), live, "govulncheck")
    for f in args.pip_audit:
        errs += gate(pip_audit_findings(Path(f)), live, "pip-audit")
    if errs:
        print("security-gate: RED", file=sys.stderr)
        for e in errs:
            print("  -", e, file=sys.stderr)
        return 1
    block = render_supported(root)
    sec = root / SECURITY_MD
    if args.emit_docs:
        sec.write_text(splice(sec.read_text(encoding="utf-8"), block), encoding="utf-8")
        print(f"security-gate: wrote supported-versions section into {SECURITY_MD}")
    if args.check:
        text = sec.read_text(encoding="utf-8")
        if BEGIN not in text or block not in text:
            print(f"security-gate: DRIFT — {SECURITY_MD} supported-versions section is not generated from {CHANGELOG}; run --emit-docs", file=sys.stderr)
            return 4
        print(f"security-gate: OK — process valid, {len(live)} live allowlist entr(ies), scanners gated: govulncheck, pip-audit; SECURITY.md fresh")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
