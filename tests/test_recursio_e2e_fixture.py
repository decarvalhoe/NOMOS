"""#612 — Recursio → NOMOS offline E2E: the proofs are the failures.

1. the committed export is a fresh normalisation of the committed site;
2. one changed byte in one HTML page changes THAT page's raw hash, normalised
   hash and Markdown — and nothing else — and the drift check turns red (4);
3. the runner passes end-to-end offline and the attestation it produces binds
   the web source type and the snapshot coverage;
4. the runner refuses a fixture whose export is stale (exit 4), before any
   pipeline stage runs.
Runner tests are skipped when go or jq are absent (the Go tests cover the
attest binding on their own).
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FIXTURE_REL = Path("tests") / "fixtures" / "recursio-e2e"
NORMALISER = ROOT / "scripts" / "recursio_export_fixture.py"
RUNNER = ROOT / "scripts" / "recursio-e2e-fixture.sh"
HAVE_TOOLS = all(shutil.which(t) for t in ("go", "jq", "git"))


def run_normaliser(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(NORMALISER), *args], capture_output=True, text=True)


def records(path: Path) -> dict[str, dict]:
    return {json.loads(l)["source_id"]: json.loads(l) for l in path.read_text(encoding="utf-8").splitlines() if l.strip()}


def mutated_fixture_copy(tmp: Path) -> Path:
    """Copy the fixture tree under tmp/<root>/tests/fixtures/recursio-e2e and
    flip one byte in one page. Returns the copy's repo-root."""
    root = tmp / "root"
    dst = root / FIXTURE_REL
    shutil.copytree(ROOT / FIXTURE_REL, dst)
    page = dst / "site" / "reglement" / "art-7.html"
    html = page.read_text(encoding="utf-8")
    assert "une base et un supplément" in html
    page.write_text(html.replace("une base et un supplément", "une base ou un supplément", 1), encoding="utf-8")
    return root


class NormaliserReproducibility(unittest.TestCase):
    def test_committed_export_is_a_fresh_normalisation(self) -> None:
        r = run_normaliser("--check")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("byte-for-byte", r.stdout)

    def test_one_byte_in_one_page_moves_only_that_page(self) -> None:
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = mutated_fixture_copy(Path(tmp))
            check = run_normaliser("--check", "--root", str(root))
            self.assertEqual(check.returncode, 4, "a stale export must be named, not silently regenerated")
            self.assertIn("reglement__art-7.md", check.stderr)
            self.assertIn("sources.jsonl", check.stderr)

            fresh = Path(tmp) / "fresh"
            self.assertEqual(run_normaliser("--root", str(root), "--out", str(fresh)).returncode, 0)
            before = records(ROOT / FIXTURE_REL / "export" / "sources.jsonl")
            after = records(fresh / "sources.jsonl")
            self.assertEqual(set(before), set(after))
            for sid in before:
                b, a = before[sid], after[sid]
                moved = sid == "reglement-art-7"
                self.assertEqual(a["content_hash"] != b["content_hash"], moved, sid)
                self.assertEqual(a["web_source"]["normalized_content_hash"] != b["web_source"]["normalized_content_hash"], moved, sid)
                md_before = (ROOT / FIXTURE_REL / "export" / b["export_path"]).read_bytes()
                md_after = (fresh / a["export_path"]).read_bytes()
                self.assertEqual(md_after != md_before, moved, sid)
                # provenance that does NOT depend on content stays put
                self.assertEqual(a["locator"], b["locator"])
                self.assertEqual(a["web_source"]["fetched_at"], b["web_source"]["fetched_at"])

    def test_chrome_does_not_reach_the_normalised_hash(self) -> None:
        """Same content, different boilerplate → same normalised hash, different raw hash."""
        sys.path.insert(0, str(ROOT / "scripts"))
        import recursio_export_fixture as rx  # noqa: E402
        page = (ROOT / FIXTURE_REL / "site" / "index.html").read_bytes()
        text_a, md_a = rx.normalise(page)
        swapped = page.replace("© exemple".encode("utf-8"), "© autre exemple".encode("utf-8"))
        self.assertNotEqual(page, swapped)
        text_b, md_b = rx.normalise(swapped)
        self.assertEqual(text_a, text_b)
        self.assertEqual(md_a, md_b)
        self.assertNotEqual(rx.sha256_bytes(page), rx.sha256_bytes(swapped))


@unittest.skipUnless(HAVE_TOOLS, "go, jq and git are required for the E2E runner")
class RunnerEndToEnd(unittest.TestCase):
    def test_runner_passes_offline_and_attestation_binds_web_coverage(self) -> None:
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            env = {**os.environ, "RUN_DIR": str(Path(tmp) / "run")}
            r = subprocess.run(["bash", str(RUNNER)], capture_output=True, text=True, env=env, cwd=ROOT)
            self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
            summary = json.loads((Path(tmp) / "run" / "summary.json").read_text(encoding="utf-8"))
            self.assertEqual(summary["status"], "pass")
            self.assertEqual(summary["web_sources"], 3)
            att = json.loads((Path(tmp) / "run" / "out" / "attestation.json").read_text(encoding="utf-8"))
            meta = att["predicate"]["metadata"]["external_snapshot"]
            self.assertEqual(meta["source_types"], {"html": 3})
            self.assertEqual(meta["content_hash_root"], summary["content_hash_root"])
            self.assertIn("never reads the operational store", meta["claim_boundary"])
            # the fixture is untouched
            self.assertEqual(r.returncode, 0)
            self.assertFalse((Path(tmp) / "run" / "fixture-mutation.diff").read_text().strip())

    def test_runner_refuses_a_stale_export_before_any_stage(self) -> None:
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            root = mutated_fixture_copy(Path(tmp))
            env = {**os.environ, "RUN_DIR": str(Path(tmp) / "run"), "FIXTURE_ROOT": str(root)}
            r = subprocess.run(["bash", str(RUNNER)], capture_output=True, text=True, env=env, cwd=ROOT)
            self.assertEqual(r.returncode, 4, r.stdout + r.stderr)
            self.assertIn("drifts", r.stderr)
            self.assertFalse((Path(tmp) / "run" / "out" / "attestation.json").exists(), "no stage may run on a stale export")


if __name__ == "__main__":
    unittest.main()
