#!/usr/bin/env python3
"""Guard the independent roadmap lanes declared in docs/roadmap-lanes.yaml.

This is a dispatch guard, not a claim or compliance validator. It prevents the
repository from recreating the false blockers removed by ADR-VRC-0004:

* only autonomous items enter each lane's dispatch queue;
* hard dependencies target autonomous work in the same lane;
* passive, human and external facts are inputs/claim gates, never task blockers;
* tooling intended for regulated use declares intended use, impact, validation
  state and bounded reliance;
* every identifier is provider-qualified (ADR-0006 FN-4): an item is `#N` on
  the tracker named by its `tracker` field, or by `default_tracker` otherwise.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
import os
import shutil
from typing import Any, Callable

import yaml

# Le module frère vit dans scripts/ ; le script peut être chargé depuis
# ailleurs (tests, importlib), d'où l'ajout explicite au chemin.
_SCRIPTS_DIR = str(Path(__file__).resolve().parent)
if _SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, _SCRIPTS_DIR)

from forge_provider import (  # noqa: E402
    ENV_TOKEN,
    ENV_TOKEN_FILE,
    GITHUB_DEFAULT_URL,
    ForgeConfigError,
    ForgeError,
    Provider,
    make_provider,
    provider_from_env,
    read_token,
    repo_from_env,
    resolve_provider_name,
)


DEFAULT_REGISTRY = Path("docs/roadmap-lanes.yaml")
# Versions du registre que ce guard sait lire (docs/16 §3 : un champ optionnel
# ajouté = MINOR). 1.0.0 : identifiants GitHub implicites. 1.1.0 : `tracker` par
# item et `default_tracker` (ADR-0006 FN-4). Un registre 1.0.0 reste lu, tous
# ses items sur GitHub ; il ne peut pas porter les champs de 1.1.0 sans
# annoncer sa version, sinon la version ne dit plus rien.
SCHEMA_VERSIONS = ("1.0.0", "1.1.0")
TRACKER_SCHEMA_VERSION = "1.1.0"
TRACKERS = ("github", "forgejo", "gitlab")
DEFAULT_TRACKER = "github"
LANES = {"product", "devops", "regulated"}
DISPATCH = {"autonomous", "passive", "human", "external"}
UMBRELLA_ROLES = {"epic", "parent"}
ITEM_STATES = {"open", "closed"}
DELIVERY_STATES = {"planned", "partial", "implemented", "verified", "blocked", "split"}
EVIDENCE_STATES = {"none", "accumulating", "requires_human", "requires_external", "present"}
CLAIM_STATES = {"bounded", "locked", "unlocked", "prohibited"}
REGULATED_TOOL_FIELDS = {"intended_use", "impact", "validation_state", "reliance"}
TOOL_IMPACTS = {"support", "evidence", "decision", "critical_decision"}
TOOL_VALIDATION_STATES = {
    "planned",
    "development",
    "technically_verified",
    "validated_for_intended_use",
}
TOOL_RELIANCE = {
    "manual_review",
    "manual_verification_required",
    "supporting_use_until_validated",
    "sole_reliance_validated",
}


def item_tracker(registry: dict[str, Any], item: dict[str, Any]) -> str:
    """Tracker d'un item : son champ `tracker`, sinon `default_tracker`, sinon GitHub.

    Renvoie la valeur telle que déclarée (une valeur inconnue est refusée par
    `validate`, pas maquillée ici).
    """
    tracker = item.get("tracker")
    if tracker is None:
        tracker = registry.get("default_tracker", DEFAULT_TRACKER)
    return str(tracker)


def validate_trackers(registry: dict[str, Any]) -> list[str]:
    """Version du schéma et qualification des identifiants (1.1.0)."""
    failures: list[str] = []
    version = registry.get("schema_version")
    if version not in SCHEMA_VERSIONS:
        failures.append(
            f"schema_version {version!r} is not one of {', '.join(SCHEMA_VERSIONS)}"
        )
    default = registry.get("default_tracker")
    if default is not None and default not in TRACKERS:
        failures.append(
            f"default_tracker {default!r} is not one of {', '.join(TRACKERS)}"
        )
    qualified: list[str] = []
    if default is not None:
        qualified.append("default_tracker")
    entries = [
        (f"issue #{item.get('issue')}", item)
        for item in registry.get("items") or []
        if isinstance(item, dict)
    ] + [
        (f"umbrella issue #{umbrella.get('issue')}", umbrella)
        for umbrella in registry.get("umbrella_issues") or []
        if isinstance(umbrella, dict)
    ]
    for label, entry in entries:
        if "tracker" not in entry:
            continue
        qualified.append(f"{label}.tracker")
        if entry["tracker"] not in TRACKERS:
            failures.append(
                f"{label}: unknown tracker {entry['tracker']!r}; expected one of {', '.join(TRACKERS)}"
            )
    if qualified and version in SCHEMA_VERSIONS and version != TRACKER_SCHEMA_VERSION:
        failures.append(
            f"schema_version {version} cannot carry {qualified[0]}; "
            f"tracker qualification requires schema_version {TRACKER_SCHEMA_VERSION}"
        )
    return failures


def validate(registry: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    items = registry.get("items")
    if not isinstance(items, list) or not items:
        return ["items: at least one roadmap item is required"]
    failures.extend(validate_trackers(registry))

    by_issue: dict[int, dict[str, Any]] = {}
    for index, item in enumerate(items):
        if not isinstance(item, dict) or not isinstance(item.get("issue"), int):
            failures.append(f"items[{index}]: integer issue is required")
            continue
        issue = item["issue"]
        if issue in by_issue:
            failures.append(f"issue #{issue}: declared twice")
        by_issue[issue] = item
        if item.get("lane") not in LANES:
            failures.append(f"issue #{issue}: unknown lane {item.get('lane')!r}")
        if item.get("dispatch") not in DISPATCH:
            failures.append(f"issue #{issue}: unknown dispatch state {item.get('dispatch')!r}")
        for field, allowed in (
            ("state", ITEM_STATES),
            ("delivery_state", DELIVERY_STATES),
            ("evidence_state", EVIDENCE_STATES),
            ("claim_state", CLAIM_STATES),
        ):
            if item.get(field) not in allowed:
                failures.append(f"issue #{issue}: unknown {field} {item.get(field)!r}")
        if not isinstance(item.get("depends_on"), list):
            failures.append(f"issue #{issue}: depends_on must be an explicit list")
        if "inputs" in item and not isinstance(item.get("inputs"), list):
            failures.append(f"issue #{issue}: inputs must be a list")
        tool = item.get("regulated_tool")
        if tool is not None:
            if not isinstance(tool, dict):
                failures.append(f"issue #{issue}: regulated_tool must be a mapping")
            else:
                missing = sorted(REGULATED_TOOL_FIELDS - set(tool))
                if missing:
                    failures.append(
                        f"issue #{issue}: regulated_tool misses {', '.join(missing)}"
                    )
                for field in REGULATED_TOOL_FIELDS:
                    if field in tool and not str(tool[field]).strip():
                        failures.append(f"issue #{issue}: regulated_tool.{field} is empty")
                if tool.get("impact") not in TOOL_IMPACTS:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool impact {tool.get('impact')!r}"
                    )
                if tool.get("validation_state") not in TOOL_VALIDATION_STATES:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool validation_state "
                        f"{tool.get('validation_state')!r}"
                    )
                if tool.get("reliance") not in TOOL_RELIANCE:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool reliance {tool.get('reliance')!r}"
                    )
                if (
                    tool.get("reliance") == "sole_reliance_validated"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: sole reliance requires validated_for_intended_use"
                    )
                if (
                    tool.get("impact") == "critical_decision"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: critical_decision is prohibited before validated_for_intended_use"
                    )
                if (
                    item.get("claim_state") == "unlocked"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: an unvalidated regulated tool cannot unlock its claim"
                    )
        if item.get("claim_state") == "unlocked" and item.get("delivery_state") in {
            "planned",
            "blocked",
            "partial",
            "split",
        }:
            failures.append(
                f"issue #{issue}: delivery_state {item.get('delivery_state')} cannot carry an unlocked claim"
            )

    for issue, item in by_issue.items():
        for dependency in item.get("depends_on", []):
            target = by_issue.get(dependency)
            if target is None:
                failures.append(f"issue #{issue}: dependency #{dependency} is not declared")
                continue
            if target.get("dispatch") != "autonomous":
                failures.append(
                    f"issue #{issue}: hard dependency #{dependency} is {target.get('dispatch')}; "
                    "passive/human/external work is a nonblocking input or claim gate"
                )
            if target.get("lane") != item.get("lane"):
                failures.append(
                    f"issue #{issue}: hard dependency #{dependency} crosses "
                    f"{item.get('lane')} -> {target.get('lane')}; use inputs instead"
                )

    # Same-lane autonomous dependencies can still deadlock if they form a
    # cycle. Detect that explicitly instead of leaving every queue item waiting
    # forever while the registry says pass.
    visiting: list[int] = []
    visited: set[int] = set()

    def visit(issue: int) -> None:
        if issue in visited:
            return
        if issue in visiting:
            start = visiting.index(issue)
            cycle = visiting[start:] + [issue]
            failures.append(
                "hard dependency cycle: " + " -> ".join(f"#{node}" for node in cycle)
            )
            return
        visiting.append(issue)
        for dependency in by_issue[issue].get("depends_on", []):
            if dependency in by_issue:
                visit(dependency)
        visiting.pop()
        visited.add(issue)

    for issue in sorted(by_issue):
        visit(issue)

    selection = registry.get("selection_policy")
    selection = selection if isinstance(selection, dict) else {}
    if selection.get("eligible_dispatch") != "autonomous":
        failures.append("selection_policy.eligible_dispatch must be autonomous")
    hard = selection.get("hard_dependencies")
    if not isinstance(hard, dict) or hard.get("same_lane_only") is not True or hard.get("autonomous_only") is not True:
        failures.append(
            "selection_policy.hard_dependencies must require same_lane_only and autonomous_only"
        )
    if selection.get("cross_lane_relationship") != "inputs_are_nonblocking":
        failures.append(
            "selection_policy.cross_lane_relationship must be inputs_are_nonblocking"
        )
    queues = selection.get("dispatch_queues")
    if not isinstance(queues, dict):
        failures.append("selection_policy.dispatch_queues must be a mapping by lane")
        queues = {}
    extra_queues = sorted(set(queues) - LANES)
    if extra_queues:
        failures.append(
            "selection_policy.dispatch_queues has unknown lane(s): "
            + ", ".join(extra_queues)
        )
    ordered: list[int] = []
    for lane in sorted(LANES):
        order = queues.get(lane)
        if not isinstance(order, list):
            failures.append(f"selection_policy.dispatch_queues.{lane} must be a list")
            continue
        if len(order) != len(set(order)):
            failures.append(f"dispatch queue {lane} contains duplicates")
        ordered.extend(order)
        for issue in order:
            item = by_issue.get(issue)
            if item is None:
                failures.append(f"dispatch queue {lane}: issue #{issue} is not declared")
            elif item.get("state") != "open" or item.get("dispatch") != "autonomous":
                failures.append(
                    f"dispatch queue {lane}: issue #{issue} is not an open autonomous item"
                )
            elif item.get("lane") != lane:
                failures.append(
                    f"dispatch queue {lane}: issue #{issue} belongs to {item.get('lane')}"
                )
    if len(ordered) != len(set(ordered)):
        failures.append("an issue appears in more than one dispatch queue")
    for issue, item in by_issue.items():
        for input_issue in item.get("inputs", []):
            if input_issue not in by_issue:
                failures.append(f"issue #{issue}: nonblocking input #{input_issue} is not declared")
    # Umbrella issues: visible, declared, never dispatched, never double-declared.
    umbrellas = registry.get("umbrella_issues") or []
    if not isinstance(umbrellas, list):
        failures.append("umbrella_issues must be a list")
        umbrellas = []
    for index, umbrella in enumerate(umbrellas):
        if not isinstance(umbrella, dict) or not isinstance(umbrella.get("issue"), int):
            failures.append(f"umbrella_issues[{index}]: integer issue is required")
            continue
        issue = umbrella["issue"]
        if issue in by_issue:
            failures.append(f"umbrella issue #{issue} is also declared as a roadmap item")
        if umbrella.get("role") not in UMBRELLA_ROLES:
            failures.append(f"umbrella issue #{issue}: unknown role {umbrella.get('role')!r}")
        if not str(umbrella.get("note", "")).strip():
            failures.append(f"umbrella issue #{issue}: a note is required")
        if issue in ordered:
            failures.append(f"umbrella issue #{issue} appears in a dispatch queue")

    open_autonomous = {
        issue
        for issue, item in by_issue.items()
        if item.get("state") == "open" and item.get("dispatch") == "autonomous"
    }
    missing_from_order = sorted(open_autonomous - set(ordered))
    if missing_from_order:
        failures.append(
            "dispatch queues omit open autonomous issue(s): "
            + ", ".join(f"#{issue}" for issue in missing_from_order)
        )
    return failures


# --- generated queue tables -------------------------------------------------
#
# The queue composition used to be retyped by hand in several documents, each
# with its own wording, and nothing compared them to the registry. The first two
# closures after the registry landed left the delivered items at the head of the
# published queues. So the tables are now GENERATED here, between markers, and
# CI regenerates them and fails on any diff — the same pattern the wiring matrix
# already uses.
QUEUE_DOCS = (
    Path("docs/47-roadmap-lanes-and-risk-based-validation.md"),
    Path("docs/29-post-alpha-release-issue-list.md"),
    Path("docs/15-product-backlog.md"),
)
QUEUE_BEGIN = "<!-- roadmap-queues:begin -->"
QUEUE_END = "<!-- roadmap-queues:end -->"


def render_queue_table(registry: dict[str, Any]) -> str:
    """Render the Product/DevOps queues as one Markdown table, from the registry."""
    by_issue = {
        int(item["issue"]): item
        for item in registry.get("items") or []
        if isinstance(item, dict) and isinstance(item.get("issue"), int)
    }
    queues = (registry.get("selection_policy") or {}).get("dispatch_queues") or {}
    product = [int(i) for i in queues.get("product") or []]
    devops = [int(i) for i in queues.get("devops") or []]

    default = str(registry.get("default_tracker", DEFAULT_TRACKER))

    def cell(issue: int | None) -> str:
        if issue is None:
            return "—"
        item = by_issue.get(issue)
        title = str(item.get("title", "")).strip() if item else "(not declared)"
        # `#N` alone means the default tracker; an item hosted elsewhere says
        # so, otherwise a reader would look it up on the wrong forge.
        tracker = item_tracker(registry, item) if item else default
        qualifier = "" if tracker == default else f" ({tracker})"
        return f"#{issue}{qualifier} — {title}"

    rows = []
    for index in range(max(len(product), len(devops), 1)):
        left = product[index] if index < len(product) else None
        right = devops[index] if index < len(devops) else None
        rows.append(f"| {cell(left)} | {cell(right)} |")

    lines = [
        QUEUE_BEGIN,
        "<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs;"
        " do not edit by hand, CI fails on drift -->",
        "| Product queue | DevOps queue |",
        "|---|---|",
        *rows,
        QUEUE_END,
    ]
    return "\n".join(lines)


def emit_docs(root: Path, registry: dict[str, Any]) -> list[str]:
    """Rewrite the marked block of every queue doc. Returns problems, if any."""
    problems: list[str] = []
    table = render_queue_table(registry)
    for rel in QUEUE_DOCS:
        path = root / rel
        if not path.is_file():
            problems.append(f"{rel.as_posix()}: missing")
            continue
        text = path.read_text(encoding="utf-8")
        begin = text.find(QUEUE_BEGIN)
        end = text.find(QUEUE_END)
        if begin < 0 or end < 0 or end < begin:
            problems.append(f"{rel.as_posix()}: no {QUEUE_BEGIN} … {QUEUE_END} block")
            continue
        end += len(QUEUE_END)
        updated = text[:begin] + table + text[end:]
        if updated != text:
            path.write_text(updated, encoding="utf-8")
    return problems


# Liaison d'un tracker : le fournisseur qui le sert et le dépôt `owner/name`
# où vivent ses issues — ou l'erreur de configuration qui empêche de le lire.
Binding = tuple[Provider, str]


def registry_trackers(registry: dict[str, Any]) -> list[str]:
    """Trackers utilisés par au moins un item, dans l'ordre de première apparition."""
    seen: list[str] = []
    for item in registry.get("items") or []:
        if not isinstance(item, dict):
            continue
        tracker = item_tracker(registry, item)
        if tracker not in seen:
            seen.append(tracker)
    return seen


def _tracker_var(tracker: str, suffix: str) -> str:
    return f"NOMOS_FORGE_{tracker.upper()}_{suffix}"


def bind_tracker(
    tracker: str,
    environ: dict[str, str],
    *,
    primary: str,
    primary_repo: str,
    which: Callable[[str], str | None] | None = None,
    **kw: Any,
) -> Binding:
    """Construit le fournisseur d'un tracker depuis l'environnement.

    Le fournisseur principal (`NOMOS_FORGE_PROVIDER`, variables nues
    `NOMOS_FORGE_URL`/`_TOKEN_FILE`/`_TOKEN`, dépôt `--repo` ou
    `NOMOS_FORGE_REPO`) sert le tracker qui porte son nom. Tout autre tracker
    est configuré par ses variables préfixées : `NOMOS_FORGE_FORGEJO_URL`,
    `NOMOS_FORGE_FORGEJO_TOKEN_FILE` (ou `_TOKEN`) et `NOMOS_FORGE_FORGEJO_REPO`
    — idem `GITHUB`, `GITLAB`. GitHub garde ses replis (GITHUB_TOKEN, GH_TOKEN,
    binaire `gh`). Le principal `fake` sert tous les trackers en mémoire
    (tests). Une variable absente lève `ForgeConfigError` qui la nomme.
    """
    which = which or shutil.which
    if primary == "fake":
        repo = environ.get(_tracker_var(tracker, "REPO"), "").strip() or primary_repo
        return make_provider("fake"), repo
    if tracker == primary:
        return provider_from_env(environ, which=which, **kw), primary_repo

    repo_var = _tracker_var(tracker, "REPO")
    repo = environ.get(repo_var, "").strip()
    if not repo:
        raise ForgeConfigError(f"{repo_var} missing")
    url_var = _tracker_var(tracker, "URL")
    url = environ.get(url_var, "").strip()
    token_file_var = _tracker_var(tracker, "TOKEN_FILE")
    token_var = _tracker_var(tracker, "TOKEN")
    # Vue de l'environnement pour `read_token` : les variables préfixées prennent
    # la place des variables nues, rien d'autre ne fuit du principal.
    view = {
        ENV_TOKEN_FILE: environ.get(token_file_var, ""),
        ENV_TOKEN: environ.get(token_var, ""),
        "GITHUB_TOKEN": environ.get("GITHUB_TOKEN", ""),
        "GH_TOKEN": environ.get("GH_TOKEN", ""),
    }
    try:
        token = read_token(view, *(("GITHUB_TOKEN", "GH_TOKEN") if tracker == "github" else ()))
    except ForgeConfigError as exc:
        raise ForgeConfigError(str(exc).replace(ENV_TOKEN_FILE, token_file_var)) from exc
    if tracker == "github":
        gh_path = None if token else which("gh")
        if not token and not gh_path:
            raise ForgeConfigError(
                f"{token_file_var}, {token_var}, GITHUB_TOKEN, GH_TOKEN or the `gh` binary missing"
            )
        return make_provider("github", base_url=url or GITHUB_DEFAULT_URL, token=token, gh_path=gh_path, **kw), repo
    if not url:
        raise ForgeConfigError(f"{url_var} missing")
    if not token:
        raise ForgeConfigError(f"{token_file_var} or {token_var} missing")
    return make_provider(tracker, base_url=url, token=token, **kw), repo


def bind_trackers(
    registry: dict[str, Any],
    environ: dict[str, str] | None = None,
    *,
    explicit_repo: str = "",
    **kw: Any,
) -> dict[str, Binding | ForgeError]:
    """Une liaison par tracker du registre ; l'erreur nommée quand elle manque.

    Le fournisseur principal et son dépôt doivent être configurés (erreur
    levée, comme avant FN-4) ; un tracker secondaire mal configuré n'arrête pas
    la vérification des autres : ses items recevront chacun un échec nommé.
    """
    env = dict(os.environ if environ is None else environ)
    primary = resolve_provider_name(env)
    primary_repo = repo_from_env(env, explicit=explicit_repo)
    bindings: dict[str, Binding | ForgeError] = {}
    for tracker in registry_trackers(registry):
        try:
            bindings[tracker] = bind_tracker(
                tracker, env, primary=primary, primary_repo=primary_repo, **kw
            )
        except ForgeConfigError as exc:
            bindings[tracker] = exc
    return bindings


def verify_tracker(
    registry: dict[str, Any], bindings: dict[str, Binding | ForgeError]
) -> list[str]:
    """Compare each item's declared state with ITS tracker. Network; not for CI.

    The guard above validates the registry's internal consistency and nothing
    else, so it stayed green while two closed issues sat at the head of their
    queues. This is the check that would have noticed. Each item is read on
    the tracker its identifier names, through that tracker's provider and
    repository (`bindings[tracker] = (provider, "owner/name")`). A tracker
    without a binding, a binding that is an error, or an unreachable tracker
    is a failure naming the item and the tracker — never a pass: the absence
    of an answer is not agreement.
    """
    problems: list[str] = []
    for item in registry.get("items") or []:
        if not isinstance(item, dict) or not isinstance(item.get("issue"), int):
            continue
        issue = item["issue"]
        tracker = item_tracker(registry, item)
        binding = bindings.get(tracker)
        if binding is None:
            problems.append(f"issue #{issue} ({tracker}): tracker not configured")
            continue
        if isinstance(binding, ForgeError):
            problems.append(f"issue #{issue} ({tracker}): {binding}")
            continue
        provider, repo = binding
        try:
            live = provider.get_issue(repo, issue)["state"]
        except (ForgeError, ValueError) as exc:
            problems.append(f"issue #{issue} ({tracker}): tracker unreachable ({exc})")
            continue
        declared = str(item.get("state", "")).lower()
        if live != declared:
            problems.append(
                f"issue #{issue} ({tracker}): registry says {declared!r}, tracker says {live!r}"
            )
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root")
    parser.add_argument("--registry", default=str(DEFAULT_REGISTRY), help="Roadmap lane registry")
    parser.add_argument(
        "--emit-docs",
        action="store_true",
        help="Regenerate the queue tables in the roadmap docs from the registry.",
    )
    parser.add_argument(
        "--verify-tracker",
        action="store_true",
        help="Compare declared item states with their trackers through the forge provider "
        "(network; not for CI). The primary provider (NOMOS_FORGE_PROVIDER) reads its "
        "own tracker from --repo or NOMOS_FORGE_REPO; any other tracker named by the "
        "registry is configured by NOMOS_FORGE_<TRACKER>_URL, _TOKEN_FILE (or _TOKEN) "
        "and _REPO, and its items fail by name when that configuration is missing.",
    )
    parser.add_argument(
        "--verify-github",
        action="store_true",
        help="Deprecated alias of --verify-tracker.",
    )
    parser.add_argument(
        "--repo",
        default="",
        help="Tracker repository as owner/name (default: NOMOS_FORGE_REPO).",
    )
    args = parser.parse_args()
    if args.verify_github:
        print(
            "notice: --verify-github is deprecated, use --verify-tracker (ADR-0006)",
            file=sys.stderr,
        )
        args.verify_tracker = True
    root = Path(args.root).resolve()
    path = Path(args.registry)
    if not path.is_absolute():
        path = root / path
    try:
        registry = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    except (OSError, yaml.YAMLError) as exc:
        print(json.dumps({"status": "error", "registry": str(path), "failures": [str(exc)]}, indent=2))
        return 2
    failures = validate(registry)
    if args.emit_docs:
        failures.extend(emit_docs(root, registry))
    if args.verify_tracker:
        try:
            bindings = bind_trackers(registry, explicit_repo=args.repo)
        except ForgeError as exc:
            # Configuration principale absente : erreur nommée, pas de
            # vérification « sautée ». Un tracker secondaire manquant est
            # rapporté item par item par verify_tracker.
            print(json.dumps({"status": "error", "registry": str(path), "failures": [str(exc)]}, indent=2))
            return 2
        failures.extend(verify_tracker(registry, bindings))
    try:
        registry_path = path.resolve().relative_to(root).as_posix()
    except ValueError:
        registry_path = path.as_posix()
    verdict = {
        "status": "fail" if failures else "pass",
        "registry": registry_path,
        "items": len(registry.get("items") or []),
        "autonomous_queues": (registry.get("selection_policy") or {}).get("dispatch_queues", {}),
        "failures": failures,
    }
    print(json.dumps(verdict, indent=2, sort_keys=True))
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
