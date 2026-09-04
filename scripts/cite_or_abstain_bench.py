#!/usr/bin/env python3
"""VRC-46 (#582) — reproduction gate of the public cite-or-abstain bench.

The bench is MEASURED by the engine (`nomos answer bench`), never here: this
gate binds provenance around that measurement and refuses to let a published
result lie in silence. Checks, in order:

1. sources — every source document the corpus quotes is still the file it was
   quoted from (declared sha256 == actual sha256), every retrieved chunk
   points at a declared source with its real digest, and every quoted span
   text is VERBATIM in its source. A moved source or an invented quote is a
   stale bench: red.
2. references — every reference the methodology cites (references.yaml)
   carries a dated verification record (doc 41: sources are re-verified
   before any external use). ``--verify-references`` resolves them live
   again (network; skipped by default).
3. determinism — the engine run twice on the same corpus emits identical
   bytes: the published measurement can be replayed, not trusted.
4. bounds — the versioned thresholds hold (the engine exits 0: no violation,
   no corpus defect).
5. replay — the measurement replayed now equals the published one
   (results-<date>.json): measurement block, corpus digest, declared sources,
   threshold values. Any drift — engine, corpus, thresholds — is named.

``--publish`` writes a new dated envelope (after checks 1-4) instead of
comparing: that is how a change that legitimately moves the numbers is
re-published, in the same change, with its date. A stale or non-reproducible
tree is never published.

Engine resolution: ``--nomos-bin``, else ``$NOMOS_BIN``, else ``go run .`` in
``<root>/cli``, else ``nomos`` on PATH (the evidence sidecar's order, #624).

Exit 0 = every check passed; 1 = a check failed (named in the JSON verdict on
stdout and on stderr); 2 = usage error or engine unavailable (nothing was
measured, no verdict is written).

Claim boundary: the bench measures the cite-or-abstain GATE over a labelled
public corpus. It measures no retrieval, no embedding, no LLM, and it claims
nothing legal, clinical or regulatory.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Any, NamedTuple

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for the cite-or-abstain bench gate.", file=sys.stderr)
    raise SystemExit(2) from exc


SCRIPT_ROOT = Path(__file__).resolve().parents[1]
BENCH_DIR = Path("docs/regulated/ai-rag-governance/cite-or-abstain-bench")
DEFAULT_CORPUS = BENCH_DIR / "corpus.yaml"
DEFAULT_THRESHOLDS = BENCH_DIR / "bench-thresholds.yaml"
DEFAULT_REFERENCES = BENCH_DIR / "references.yaml"
RESULTS_GLOB = "results-*.json"
RESULTS_SCHEMA_VERSION = "nomos-cite-or-abstain-bench-results-v1"
VERDICT_SCHEMA_VERSION = "nomos-cite-or-abstain-bench-gate-v1"
ENGINE_UNAVAILABLE_CODE = "NOMOS_ENGINE_UNAVAILABLE"
CLAIM_BOUNDARY = (
    "Measurement of the cite-or-abstain gate over a labelled public corpus; "
    "no retrieval-quality, LLM-accuracy, NLI-accuracy, legal, clinical or regulatory claim."
)
ENGINE_CONFIGURATION = "lexical_entailment_v1 proxy only; no external scorer"
# 0 = measured and every bound holds, 1 = a violation or a corpus defect.
# Anything else is not a measurement (usage error, unreadable corpus, crash).
ENGINE_BENCH_EXIT_CODES = (0, 1)
DEFAULT_ENGINE_TIMEOUT_SECONDS = 900.0
DEFAULT_LIVE_TIMEOUT_SECONDS = 20.0
REQUIRED_REFERENCE_FIELDS = ("reference_id", "title", "official_url", "role")
TIMESTAMP_FORMAT = "%Y-%m-%dT%H:%M:%SZ"
DIFF_LIMIT = 25


# --- helpers ---------------------------------------------------------------------


class EngineError(RuntimeError):
    """The engine produced no measurement. Nothing is scored here instead."""


class Engine(NamedTuple):
    command: list[str]
    cwd: Path | None
    origin: str


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_yaml(path: Path) -> dict[str, Any]:
    loaded = yaml.safe_load(path.read_text(encoding="utf-8"))
    return loaded if isinstance(loaded, dict) else {}


def resolve(root: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def rel(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root.resolve()).as_posix()
    except ValueError:
        return path.as_posix()


def check(name: str, problems: list[str], detail: dict[str, Any] | None = None) -> dict[str, Any]:
    return {
        "name": name,
        "status": "fail" if problems else "pass",
        "problems": problems,
        "detail": detail or {},
    }


def _normalize(text: str) -> str:
    """Whitespace-folded, backtick-free text: a quote is verbatim when it is a
    substring of the source under this normalisation (Markdown code marks
    around an identifier are not part of the quoted sentence)."""
    return " ".join(text.replace("`", "").split())


# --- 1. sources ----------------------------------------------------------------------


def check_sources(root: Path, corpus: dict[str, Any]) -> dict[str, Any]:
    """The bench quotes real, unmoved, in-repo text — or it is stale."""
    problems: list[str] = []
    declared: dict[str, dict[str, Any]] = {}
    normalized_text: dict[str, str] = {}

    sources = corpus.get("sources")
    if not isinstance(sources, list) or not sources:
        problems.append("the corpus declares no sources: nothing to quote from")
        sources = []
    for source in sources:
        if not isinstance(source, dict) or not all(source.get(key) for key in ("source_id", "path", "sha256")):
            problems.append(f"source entry without source_id/path/sha256: {source!r}")
            continue
        source_id = str(source["source_id"])
        if source_id in declared:
            problems.append(f"{source_id}: declared twice")
            continue
        declared[source_id] = source
        path = root / str(source["path"])
        if not path.is_file():
            problems.append(f"{source['path']}: missing — the bench quotes a document that no longer exists")
            continue
        actual = sha256_file(path)
        expected = str(source["sha256"]).lower()
        if actual != expected:
            problems.append(
                f"{source['path']}: sha256 {actual[:12]}… differs from the declared {expected[:12]}… "
                "— the quoted text moved; re-publish the bench"
            )
        normalized_text[source_id] = _normalize(path.read_text(encoding="utf-8"))

    items = [item for item in (corpus.get("items") or []) if isinstance(item, dict)]
    for item in items:
        answer_id = str(item.get("answer_id") or "<unnamed item>")
        for collection in ("source_spans", "retrieved_chunks"):
            for entry in item.get(collection) or []:
                if not isinstance(entry, dict):
                    continue
                source_id = entry.get("source_id")
                if collection == "retrieved_chunks":
                    # What the bench says was RETRIEVED must be real, declared
                    # text carrying the real digest of its document.
                    if source_id not in declared:
                        problems.append(f"{answer_id}: retrieved chunk cites undeclared source {source_id!r}")
                    elif str(entry.get("source_hash", "")).lower() != "sha256:" + str(declared[source_id]["sha256"]).lower():
                        problems.append(
                            f"{answer_id}: retrieved chunk carries a source_hash that is not the declared digest of {source_id}"
                        )
                # A CITED span may deliberately bind to nothing (the
                # forged_citation category); its text, when present, must
                # still be a verbatim quote of a declared source.
                text = entry.get("text")
                if isinstance(text, str) and text.strip():
                    if source_id not in declared:
                        problems.append(f"{answer_id}: quoted text names undeclared source {source_id!r}")
                    elif source_id in normalized_text and _normalize(text) not in normalized_text[source_id]:
                        problems.append(f"{answer_id}: span text is not verbatim in {source_id}: {text[:60]!r}…")

    detail = {
        "sources": [
            {"source_id": s["source_id"], "path": s["path"], "sha256": s["sha256"]} for s in declared.values()
        ],
        "items": len(items),
    }
    return check("sources", problems, detail)


# --- 2. references ---------------------------------------------------------------


def check_references(path: Path, live: bool, timeout: float) -> list[dict[str, Any]]:
    """Every cited reference carries a dated verification record; live mode
    resolves the official URLs again (HTTP 200 or red)."""
    if not path.is_file():
        return [check("references", [f"{path}: missing — the methodology cites references that are not registered"])]
    doc = load_yaml(path)
    references = doc.get("references")
    problems: list[str] = []
    if not isinstance(references, list) or not references:
        problems.append("no reference declared")
        references = []
    now = datetime.now(timezone.utc)
    entries: list[dict[str, Any]] = []
    for reference in references:
        if not isinstance(reference, dict):
            problems.append(f"reference entry is not a mapping: {reference!r}")
            continue
        reference_id = str(reference.get("reference_id") or "<unnamed reference>")
        for field in REQUIRED_REFERENCE_FIELDS:
            if not reference.get(field):
                problems.append(f"{reference_id}: missing {field}")
        url = str(reference.get("official_url") or "")
        if url and not url.startswith("https://"):
            problems.append(f"{reference_id}: official_url must be https, got {url!r}")
        verification = reference.get("verification")
        verification = verification if isinstance(verification, dict) else {}
        stamp = verification.get("verified_at_utc")
        if not stamp:
            problems.append(
                f"{reference_id}: no verification.verified_at_utc — a source is re-verified before publication (doc 41)"
            )
        else:
            try:
                parsed = datetime.strptime(str(stamp), TIMESTAMP_FORMAT).replace(tzinfo=timezone.utc)
            except ValueError:
                problems.append(f"{reference_id}: verified_at_utc {stamp!r} is not an ISO UTC timestamp (YYYY-MM-DDTHH:MM:SSZ)")
            else:
                if parsed > now:
                    problems.append(f"{reference_id}: verified_at_utc {stamp} is in the future")
        if not verification.get("method"):
            problems.append(f"{reference_id}: no verification.method")
        entries.append({"reference_id": reference_id, "url": url, "verified_at_utc": stamp})
    checks = [check("references", problems, {"references": entries})]

    if live:
        live_problems: list[str] = []
        resolved: list[dict[str, Any]] = []
        for entry in entries:
            if not entry["url"]:
                continue
            request = urllib.request.Request(
                entry["url"],
                headers={"User-Agent": "nomos-cite-or-abstain-bench/1.0 (reference verification)"},
            )
            try:
                with urllib.request.urlopen(request, timeout=timeout) as response:  # noqa: S310 - https only, checked above
                    status = int(response.status)
            except urllib.error.HTTPError as exc:
                status = int(exc.code)
            except (urllib.error.URLError, TimeoutError, OSError) as exc:
                live_problems.append(f"{entry['reference_id']}: {entry['url']} unreachable ({exc})")
                continue
            resolved.append({"reference_id": entry["reference_id"], "url": entry["url"], "http_status": status})
            if status != 200:
                live_problems.append(f"{entry['reference_id']}: {entry['url']} answered HTTP {status}")
        checks.append(check("references_live", live_problems, {"resolved": resolved}))
    return checks


# --- engine ------------------------------------------------------------------------


def _binary_path(root: Path, value: str) -> str:
    path = Path(value).expanduser()
    if path.is_absolute() or os.sep not in value:
        return str(path)
    for base in (Path.cwd(), root):
        candidate = base / path
        if candidate.exists():
            return str(candidate.resolve())
    return str(path)


def resolve_engine(root: Path, nomos_bin: str | None) -> Engine:
    """Locate the engine: --nomos-bin, $NOMOS_BIN, `go run .`, then PATH."""
    if nomos_bin:
        return Engine([_binary_path(root, nomos_bin)], None, "--nomos-bin")
    env_bin = os.environ.get("NOMOS_BIN", "").strip()
    if env_bin:
        return Engine([_binary_path(root, env_bin)], None, "NOMOS_BIN")
    if shutil.which("go"):
        for cli_dir in (root / "cli", SCRIPT_ROOT / "cli"):
            if (cli_dir / "go.mod").is_file():
                return Engine(["go", "run", "."], cli_dir, f"go run ({cli_dir})")
    on_path = shutil.which("nomos")
    if on_path:
        return Engine([on_path], None, "PATH")
    raise EngineError(
        "no engine found: pass --nomos-bin, set NOMOS_BIN, run from a checkout with `go` on PATH, or put `nomos` on PATH"
    )


def _tail(text: str, limit: int = 400) -> str:
    text = text.strip()
    return text if len(text) <= limit else "…" + text[-limit:]


def _run(engine: Engine, args: list[str], timeout: float) -> subprocess.CompletedProcess[str]:
    argv = [*engine.command, *args]
    try:
        return subprocess.run(argv, cwd=engine.cwd, text=True, capture_output=True, timeout=timeout, check=False)
    except FileNotFoundError as exc:
        raise EngineError(f"engine command not found: {argv[0]} ({exc})") from exc
    except PermissionError as exc:
        raise EngineError(f"engine command not executable: {argv[0]} ({exc})") from exc
    except subprocess.TimeoutExpired as exc:
        raise EngineError(f"engine timed out after {timeout:.0f}s: {' '.join(argv)}") from exc


def engine_version(engine: Engine, timeout: float) -> str | None:
    try:
        proc = _run(engine, ["version"], timeout)
    except EngineError:
        return None
    if proc.returncode != 0:
        return None
    lines = [line.strip() for line in proc.stdout.splitlines() if line.strip()]
    return lines[-1] if lines else None


def run_bench(engine: Engine, corpus_path: Path, thresholds_path: Path, timeout: float) -> tuple[str, int, dict[str, Any]]:
    """Run `nomos answer bench` and return (raw stdout, exit code, measurement)."""
    args = ["answer", "bench", "--corpus", str(corpus_path), "--thresholds", str(thresholds_path)]
    proc = _run(engine, args, timeout)
    if proc.returncode not in ENGINE_BENCH_EXIT_CODES:
        raise EngineError(
            f"engine exited {proc.returncode} instead of a measurement: {_tail(proc.stderr) or '<no stderr>'}"
        )
    try:
        result = json.loads(proc.stdout)
    except ValueError as exc:
        detail = _tail(proc.stderr) or _tail(proc.stdout) or "<empty output>"
        raise EngineError(f"engine produced no bench measurement JSON (exit {proc.returncode}): {detail}") from exc
    if not isinstance(result, dict) or result.get("status") not in ("measured", "fail") or "items_detail" not in result:
        raise EngineError("engine output is not a bench measurement (status/items_detail missing)")
    return proc.stdout, proc.returncode, result


# --- 3-5. determinism, bounds, replay ------------------------------------------


def check_determinism(first: str, second: str) -> dict[str, Any]:
    problems = [] if first == second else ["two engine runs on the same corpus emitted different bytes: the measurement cannot be replayed"]
    return check("determinism", problems, {"runs": 2, "bytes": len(first.encode("utf-8"))})


def check_bounds(measurement: dict[str, Any], exit_code: int) -> dict[str, Any]:
    problems: list[str] = []
    for defect in measurement.get("defects") or []:
        problems.append(f"corpus defect: {defect}")
    for violation in measurement.get("violations") or []:
        problems.append(f"versioned bound broken: {violation}")
    if exit_code != 0 and not problems:
        problems.append(f"engine exited {exit_code} without naming a violation or a defect")
    if measurement.get("status") != "measured" and not problems:
        problems.append(f"engine status is {measurement.get('status')!r}, not a measurement")
    return check("bounds", problems, {"engine_exit_code": exit_code, "thresholds": measurement.get("thresholds")})


def measurement_summary(measurement: dict[str, Any]) -> dict[str, Any]:
    categories = [c for c in (measurement.get("categories") or []) if isinstance(c, dict)]
    return {
        "status": measurement.get("status"),
        "items": measurement.get("items"),
        "agreement": measurement.get("agreement"),
        "must_abstain_recall": measurement.get("must_abstain_recall"),
        "false_cites": measurement.get("false_cites"),
        "false_cite_rate": measurement.get("false_cite_rate"),
        "false_cites_by_category": {
            str(c.get("category")): c.get("false_cites") for c in categories if c.get("false_cites")
        },
        "must_cite_recall": measurement.get("must_cite_recall"),
        "missed_cites": measurement.get("missed_cites"),
        "violations": list(measurement.get("violations") or []),
        "defects": list(measurement.get("defects") or []),
    }


def build_envelope(
    root: Path,
    corpus_path: Path,
    corpus: dict[str, Any],
    thresholds_path: Path,
    thresholds: dict[str, Any],
    version: str | None,
    measurement: dict[str, Any],
    published_on: str,
) -> dict[str, Any]:
    gates = measurement.get("gates") if isinstance(measurement.get("gates"), dict) else {}
    return {
        "schema_version": RESULTS_SCHEMA_VERSION,
        "bench_id": corpus.get("bench_id"),
        "published_on": published_on,
        "claim_boundary": CLAIM_BOUNDARY,
        "corpus": {
            "path": rel(corpus_path, root),
            "sha256": sha256_file(corpus_path),
            "schema_version": corpus.get("schema_version"),
        },
        "thresholds": {
            "path": rel(thresholds_path, root),
            "sha256": sha256_file(thresholds_path),
            "values": thresholds,
        },
        "sources": [
            {"source_id": s.get("source_id"), "path": s.get("path"), "sha256": s.get("sha256")}
            for s in (corpus.get("sources") or [])
            if isinstance(s, dict)
        ],
        "engine": {
            "nomos_version": version,
            "command": ["nomos", "answer", "bench", "--corpus", rel(corpus_path, root), "--thresholds", rel(thresholds_path, root)],
            "configuration": ENGINE_CONFIGURATION,
            "scorer_configured": bool(gates.get("scorer_configured")),
        },
        "measurement": measurement,
    }


def deep_diff(published: Any, replayed: Any, prefix: str = "") -> list[str]:
    """Dotted paths where the published and replayed values differ."""
    out: list[str] = []
    if isinstance(published, dict) and isinstance(replayed, dict):
        for key in sorted(set(published) | set(replayed)):
            path = f"{prefix}.{key}" if prefix else str(key)
            if key not in published:
                out.append(f"{path}: absent from the published result, replayed {json.dumps(replayed[key])[:80]}")
            elif key not in replayed:
                out.append(f"{path}: published {json.dumps(published[key])[:80]}, absent from the replay")
            else:
                out.extend(deep_diff(published[key], replayed[key], path))
    elif isinstance(published, list) and isinstance(replayed, list):
        if len(published) != len(replayed):
            out.append(f"{prefix}: published {len(published)} entries, replayed {len(replayed)}")
        for index, (a, b) in enumerate(zip(published, replayed)):
            out.extend(deep_diff(a, b, f"{prefix}[{index}]"))
    elif published != replayed:
        out.append(f"{prefix}: published {json.dumps(published)[:80]}, replayed {json.dumps(replayed)[:80]}")
    return out


def check_replay(published_path: Path | None, now: dict[str, Any]) -> dict[str, Any]:
    """The published result is exactly what the tree measures today."""
    if published_path is None or not published_path.is_file():
        return check("replay", ["no published result to replay (results-<date>.json): publish one with --publish"])
    try:
        published = json.loads(published_path.read_text(encoding="utf-8"))
    except ValueError as exc:
        return check("replay", [f"{published_path}: not JSON ({exc})"])
    if not isinstance(published, dict):
        return check("replay", [f"{published_path}: not a result envelope"])
    problems: list[str] = []
    if published.get("schema_version") != RESULTS_SCHEMA_VERSION:
        problems.append(
            f"schema_version: published {published.get('schema_version')!r}, this gate replays {RESULTS_SCHEMA_VERSION!r}"
        )
    problems.extend(deep_diff(published.get("corpus"), now.get("corpus"), "corpus"))
    problems.extend(deep_diff(published.get("sources"), now.get("sources"), "sources"))
    published_thresholds = published.get("thresholds") if isinstance(published.get("thresholds"), dict) else {}
    problems.extend(deep_diff(published_thresholds.get("values"), now["thresholds"].get("values"), "thresholds.values"))
    problems.extend(deep_diff(published.get("measurement"), now.get("measurement"), "measurement"))
    # The engine version is recorded, not gated: a rebuilt engine that measures
    # the same numbers is the same bench; one that does not is caught above.
    published_engine = published.get("engine") if isinstance(published.get("engine"), dict) else {}
    detail = {
        "published": str(published_path),
        "published_on": published.get("published_on"),
        "published_engine": published_engine.get("nomos_version"),
        "replayed_engine": now["engine"].get("nomos_version"),
        "differences": len(problems),
    }
    return check("replay", problems[:DIFF_LIMIT], detail)


def newest_published(corpus_dir: Path) -> Path | None:
    candidates = sorted(corpus_dir.glob(RESULTS_GLOB))
    return candidates[-1] if candidates else None


# --- main -------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(description="Replay (or publish) the public cite-or-abstain bench result.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--nomos-bin", default=None, help="nomos binary (default: $NOMOS_BIN, then `go run .` in <root>/cli, then PATH).")
    parser.add_argument("--corpus", default=str(DEFAULT_CORPUS), help="Labelled bench corpus YAML.")
    parser.add_argument("--thresholds", default=str(DEFAULT_THRESHOLDS), help="Versioned bounds YAML.")
    parser.add_argument("--references", default=str(DEFAULT_REFERENCES), help="References cited by the methodology.")
    parser.add_argument(
        "--published",
        default=None,
        help="Published result to replay (default: the newest results-<date>.json next to the corpus); with --publish, the path to write.",
    )
    parser.add_argument("--publish", action="store_true", help="Write a new dated result envelope instead of replaying one.")
    parser.add_argument("--published-on", default=None, help="Publication date (YYYY-MM-DD) for --publish; default: today (UTC).")
    parser.add_argument("--verify-references", action="store_true", help="Resolve every cited reference live (network).")
    parser.add_argument("--engine-timeout", type=float, default=DEFAULT_ENGINE_TIMEOUT_SECONDS, help="Seconds allowed per engine invocation.")
    parser.add_argument("--live-timeout", type=float, default=DEFAULT_LIVE_TIMEOUT_SECONDS, help="Seconds allowed per reference fetch.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    corpus_path = resolve(root, args.corpus)
    thresholds_path = resolve(root, args.thresholds)
    references_path = resolve(root, args.references)
    for path, label in ((corpus_path, "corpus"), (thresholds_path, "thresholds")):
        if not path.is_file():
            print(f"cite-or-abstain bench: {label} not found: {path}", file=sys.stderr)
            return 2
    published_on = args.published_on or date.today().isoformat()
    try:
        date.fromisoformat(published_on)
    except ValueError:
        print(f"cite-or-abstain bench: --published-on must be YYYY-MM-DD, got {published_on!r}", file=sys.stderr)
        return 2

    corpus = load_yaml(corpus_path)
    thresholds = load_yaml(thresholds_path)
    checks: list[dict[str, Any]] = [check_sources(root, corpus)]
    checks.extend(check_references(references_path, args.verify_references, args.live_timeout))

    try:
        engine = resolve_engine(root, args.nomos_bin)
        version = engine_version(engine, args.engine_timeout)
        first, exit_code, measurement = run_bench(engine, corpus_path, thresholds_path, args.engine_timeout)
        second, _, _ = run_bench(engine, corpus_path, thresholds_path, args.engine_timeout)
    except EngineError as exc:
        print(f"{ENGINE_UNAVAILABLE_CODE}: {exc}", file=sys.stderr)
        print("nothing measured: the bench is measured by the engine, never by this gate.", file=sys.stderr)
        return 2

    checks.append(check_determinism(first, second))
    checks.append(check_bounds(measurement, exit_code))
    envelope = build_envelope(root, corpus_path, corpus, thresholds_path, thresholds, version, measurement, published_on)

    mode = "publish" if args.publish else "verify"
    if args.publish:
        published_path = resolve(root, args.published) if args.published else corpus_path.parent / f"results-{published_on}.json"
        failed = [c["name"] for c in checks if c["status"] == "fail"]
        if failed:
            checks.append(check("publish", ["not published: " + ", ".join(failed) + " failed — a stale or non-reproducible tree is never published"]))
        else:
            published_path.parent.mkdir(parents=True, exist_ok=True)
            published_path.write_text(json.dumps(envelope, indent=2, sort_keys=True) + "\n", encoding="utf-8")
            checks.append(check("publish", [], {"written": str(published_path), "published_on": published_on}))
    else:
        published_path = resolve(root, args.published) if args.published else newest_published(corpus_path.parent)
        checks.append(check_replay(published_path, envelope))

    status = "pass" if all(c["status"] == "pass" for c in checks) else "fail"
    verdict = {
        "schema_version": VERDICT_SCHEMA_VERSION,
        "mode": mode,
        "status": status,
        "claim_boundary": CLAIM_BOUNDARY,
        "engine": {"origin": engine.origin, "nomos_version": version, "command": [*engine.command, "answer", "bench"]},
        "corpus": envelope["corpus"],
        "published": str(published_path) if published_path else None,
        "checks": checks,
        "measurement_summary": measurement_summary(measurement),
    }
    print(json.dumps(verdict, indent=2, sort_keys=True))
    if status == "fail":
        for item in checks:
            for problem in item["problems"]:
                print(f"cite-or-abstain bench: {item['name']}: {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
