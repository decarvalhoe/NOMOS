"""VRC-46 (#582) — the public cite-or-abstain bench is reproducible, or it is
not published.

The engine measures (`nomos answer bench`, proved in Go); this file proves the
reproduction gate around it (doctrine §2.3, adversarial):

* on the real tree the published result replays byte for byte, and the
  measurement says what the methodology says (one false cite: the negation);
* a published number edited by hand is named and turns the gate red;
* a source document that moved makes the bench stale (red), so the bench can
  never quote a text that no longer exists;
* an invented quote (span text absent from its source) is red;
* a reference without a dated verification record is red (doc 41);
* publishing writes a dated envelope that the gate then replays.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "cli"
SCRIPT = ROOT / "scripts" / "cite_or_abstain_bench.py"
BENCH_DIR = ROOT / "docs/regulated/ai-rag-governance/cite-or-abstain-bench"
CORPUS = BENCH_DIR / "corpus.yaml"
SOURCE_DOCS = (
    ROOT / "docs/regulated/reference-basis/README.md",
    ROOT / "docs/regulated/reference-basis/nomos-bible-corpus-policy.md",
)


def published_result() -> Path:
    candidates = sorted(BENCH_DIR.glob("results-*.json"))
    if not candidates:
        raise AssertionError("no published bench result (results-<date>.json) in the tree")
    return candidates[-1]


def run_gate(root: Path, nomos: Path, *extra: str) -> tuple[int, dict, str]:
    env = dict(os.environ)
    env.pop("NOMOS_BIN", None)
    proc = subprocess.run(
        [sys.executable, str(SCRIPT), "--root", str(root), "--nomos-bin", str(nomos), *extra],
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
        env=env,
    )
    verdict = json.loads(proc.stdout) if proc.stdout.strip().startswith("{") else {}
    return proc.returncode, verdict, proc.stderr


def checks_by_name(verdict: dict) -> dict[str, dict]:
    return {c["name"]: c for c in verdict.get("checks", [])}


def copy_bench_tree(target: Path) -> None:
    """A minimal checkout: the bench folder plus the public sources it quotes."""
    for doc in SOURCE_DOCS:
        dest = target / doc.relative_to(ROOT)
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(doc, dest)
    shutil.copytree(BENCH_DIR, target / BENCH_DIR.relative_to(ROOT))


@unittest.skipIf(shutil.which("go") is None, "go not on PATH — the bench is measured by the real engine")
class CiteOrAbstainBenchGateTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls._tmp = tempfile.TemporaryDirectory()
        cls.nomos = Path(cls._tmp.name) / "nomos"
        build = subprocess.run(["go", "build", "-o", str(cls.nomos), "."], cwd=CLI, text=True, capture_output=True, check=False)
        if build.returncode != 0:
            raise AssertionError(f"engine build failed: {build.stderr}{build.stdout}")

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    def test_published_result_replays_on_the_real_tree(self) -> None:
        code, verdict, stderr = run_gate(ROOT, self.nomos)
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["status"], "pass")
        checks = checks_by_name(verdict)
        for name in ("sources", "references", "determinism", "bounds", "replay"):
            self.assertEqual(checks[name]["status"], "pass", checks[name])
        self.assertEqual(Path(verdict["published"]).resolve(), published_result().resolve())
        summary = verdict["measurement_summary"]
        # What the methodology says: every category is blocked except the
        # negation, the documented blind spot of the lexical proxy; no
        # legitimate answer is blocked.
        self.assertEqual(summary["status"], "measured")
        self.assertEqual(summary["false_cites"], 1)
        self.assertEqual(summary["false_cites_by_category"], {"negation": 1})
        self.assertEqual(summary["missed_cites"], 0)
        self.assertEqual(summary["must_cite_recall"], 1)
        self.assertEqual(summary["violations"], [])
        self.assertEqual(summary["defects"], [])
        published = json.loads(published_result().read_text(encoding="utf-8"))
        self.assertEqual(published["schema_version"], "nomos-cite-or-abstain-bench-results-v1")
        self.assertFalse(published["engine"]["scorer_configured"], "the published run is lexical only")
        self.assertEqual(published["measurement"]["false_cites"], 1)

    def test_a_hand_edited_published_number_is_named(self) -> None:
        published = json.loads(published_result().read_text(encoding="utf-8"))
        published["measurement"]["false_cites"] = 0
        published["measurement"]["categories"] = [
            {**c, "false_cites": 0} for c in published["measurement"]["categories"]
        ]
        with tempfile.TemporaryDirectory() as tmp:
            forged = Path(tmp) / "results-2099-01-01.json"
            forged.write_text(json.dumps(published), encoding="utf-8")
            code, verdict, stderr = run_gate(ROOT, self.nomos, "--published", str(forged))
        self.assertEqual(code, 1, stderr)
        replay = checks_by_name(verdict)["replay"]
        self.assertEqual(replay["status"], "fail")
        self.assertTrue(any("measurement.false_cites" in p for p in replay["problems"]), replay["problems"])
        # The other checks still pass: the tree is honest, the published file is not.
        self.assertEqual(checks_by_name(verdict)["sources"]["status"], "pass")

    def test_a_moved_source_makes_the_bench_stale(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_bench_tree(root)
            readme = root / "docs/regulated/reference-basis/README.md"
            readme.write_text(readme.read_text(encoding="utf-8") + "\n9. A rule added after publication.\n", encoding="utf-8")
            code, verdict, stderr = run_gate(root, self.nomos)
        self.assertEqual(code, 1, stderr)
        sources = checks_by_name(verdict)["sources"]
        self.assertEqual(sources["status"], "fail")
        self.assertTrue(any("README.md" in p and "differs" in p for p in sources["problems"]), sources["problems"])

    def test_an_invented_quote_is_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_bench_tree(root)
            corpus_path = root / CORPUS.relative_to(ROOT)
            corpus = yaml.safe_load(corpus_path.read_text(encoding="utf-8"))
            grounded = next(i for i in corpus["items"] if i["answer_id"] == "BENCH-PUB-SNAPSHOT")
            grounded["source_spans"][0]["text"] = "Public bibles may be processed from any convenient copy."
            corpus_path.write_text(yaml.safe_dump(corpus, sort_keys=False, allow_unicode=True), encoding="utf-8")
            code, verdict, stderr = run_gate(root, self.nomos)
        self.assertEqual(code, 1, stderr)
        sources = checks_by_name(verdict)["sources"]
        self.assertEqual(sources["status"], "fail")
        self.assertTrue(any("BENCH-PUB-SNAPSHOT" in p and "not verbatim" in p for p in sources["problems"]), sources["problems"])

    def test_an_unverified_reference_is_refused(self) -> None:
        references = yaml.safe_load((BENCH_DIR / "references.yaml").read_text(encoding="utf-8"))
        del references["references"][0]["verification"]["verified_at_utc"]
        with tempfile.TemporaryDirectory() as tmp:
            unverified = Path(tmp) / "references.yaml"
            unverified.write_text(yaml.safe_dump(references, sort_keys=False, allow_unicode=True), encoding="utf-8")
            code, verdict, stderr = run_gate(ROOT, self.nomos, "--references", str(unverified))
        self.assertEqual(code, 1, stderr)
        refs = checks_by_name(verdict)["references"]
        self.assertEqual(refs["status"], "fail")
        self.assertTrue(any("verified_at_utc" in p for p in refs["problems"]), refs["problems"])

    def test_publish_writes_a_dated_envelope_the_gate_replays(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp) / "results-2026-01-01.json"
            code, verdict, stderr = run_gate(ROOT, self.nomos, "--publish", "--published", str(target), "--published-on", "2026-01-01")
            self.assertEqual(code, 0, stderr)
            self.assertEqual(verdict["mode"], "publish")
            self.assertEqual(checks_by_name(verdict)["publish"]["status"], "pass")
            envelope = json.loads(target.read_text(encoding="utf-8"))
            self.assertEqual(envelope["published_on"], "2026-01-01")
            self.assertEqual(envelope["corpus"]["path"], "docs/regulated/ai-rag-governance/cite-or-abstain-bench/corpus.yaml")
            self.assertEqual(len(envelope["sources"]), 2)
            self.assertEqual(envelope["thresholds"]["values"]["min_must_cite_recall"], 1.0)
            # The freshly published envelope is exactly what the gate replays.
            code, verdict, stderr = run_gate(ROOT, self.nomos, "--published", str(target))
            self.assertEqual(code, 0, stderr)
            self.assertEqual(checks_by_name(verdict)["replay"]["status"], "pass")

    def test_publish_refuses_a_stale_tree(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_bench_tree(root)
            policy = root / "docs/regulated/reference-basis/nomos-bible-corpus-policy.md"
            policy.write_text(policy.read_text(encoding="utf-8").replace("GAMP 5 Second Edition", "GAMP 5 Third Edition"), encoding="utf-8")
            target = root / "results-2026-01-01.json"
            code, verdict, stderr = run_gate(root, self.nomos, "--publish", "--published", str(target))
        self.assertEqual(code, 1, stderr)
        self.assertFalse(target.exists(), "a stale bench must not be published")
        self.assertEqual(checks_by_name(verdict)["publish"]["status"], "fail")

    def test_required_engine_missing_measures_nothing(self) -> None:
        code, verdict, stderr = run_gate(ROOT, Path(self._tmp.name) / "no-such-nomos")
        self.assertEqual(code, 2, stderr)
        self.assertEqual(verdict, {})
        self.assertIn("NOMOS_ENGINE_UNAVAILABLE", stderr)

    @unittest.skipUnless(os.environ.get("NOMOS_LIVE_REFERENCES") == "1", "set NOMOS_LIVE_REFERENCES=1 to resolve the cited references over the network")
    def test_cited_references_resolve_live(self) -> None:
        code, verdict, stderr = run_gate(ROOT, self.nomos, "--verify-references")
        self.assertEqual(code, 0, stderr)
        live = checks_by_name(verdict)["references_live"]
        self.assertEqual(live["status"], "pass", live)
        self.assertEqual({r["http_status"] for r in live["detail"]["resolved"]}, {200})


if __name__ == "__main__":
    unittest.main()
