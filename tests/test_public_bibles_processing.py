"""#644 — real public references: count only what completed the chain, retain
the evidence, and name every blocked source.

Doctrine §2.3: the proof is the failure. The offline test drives a synthetic
capture through all six stages and shows that one changed byte turns it into a
named mismatch with a stale receipt — and that a fixture document is never
counted as an external source.

Skips when the Go toolchain is unavailable.
"""

from __future__ import annotations

import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
_SPEC = importlib.util.spec_from_file_location("process_public_bibles", ROOT / "scripts" / "process_public_bibles.py")
proc = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(proc)

FIXTURE_CAPTURE = ROOT / "tests" / "fixtures" / "public-source-capture" / "NOMOS-SYNTH-PUBLIC-001.md"


def build_nomos(dest_dir: Path) -> str | None:
    if shutil.which("go") is None:
        return None
    binary = dest_dir / ("nomos.exe" if os.name == "nt" else "nomos")
    result = subprocess.run(["go", "build", "-o", str(binary), "."], cwd=ROOT / "cli", capture_output=True, text=True)
    return str(binary) if result.returncode == 0 else None


def sha256_of(path: Path) -> str:
    return "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()


def synthetic_repo(tmp: Path, capture_sha: str | None, *, blocked: bool = False) -> Path:
    """A minimal repo whose register holds ONE public reference, the synthetic one."""
    repo = tmp / "repo"
    (repo / "scripts").mkdir(parents=True)
    shutil.copy(ROOT / "scripts" / "regulated_reference_canon.py", repo / "scripts" / "regulated_reference_canon.py")
    base = repo / "docs" / "regulated" / "reference-basis"
    base.mkdir(parents=True)
    (base / "README.md").write_text("# Reference basis\n\nFixture policy document one, long enough to atomise into a unit.\n", encoding="utf-8")
    (base / "nomos-bible-corpus-policy.md").write_text("# Policy\n\nFixture policy document two, also long enough to atomise.\n", encoding="utf-8")
    (base / "external-reference-register.yaml").write_text(
        """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: NOMOS-SYNTH-PUBLIC-001
    title: Synthetic public guidance
    publisher: Example Authority
    url: https://example.invalid/guidance/section-4
    version_or_date: "edition 2026-09"
    checked_on: "2026-09-05"
    evidence_status: requires_evidence
""".lstrip(), encoding="utf-8")
    if capture_sha is not None or blocked:
        entry = {"reference_id": "NOMOS-SYNTH-PUBLIC-001", "official_url": "https://example.invalid/guidance/section-4",
                 "version_identity": "edition 2026-09", "licence_or_policy": "public; hash only", "storage_policy": "hash_only_no_full_text"}
        if blocked:
            entry.update({"status": "blocked", "blocked_reason": "fetch_failed: HTTP Error 404: Not Found", "checked_on": "2026-09-05"})
        else:
            entry.update({"status": "captured_hash_only", "captured_on": "2026-09-05", "sha256": capture_sha, "size_bytes": FIXTURE_CAPTURE.stat().st_size})
        (base / "public-source-snapshots.yaml").write_text(
            yaml.safe_dump({"schema_version": proc.SNAPSHOTS_SCHEMA, "sources": [entry]}, sort_keys=False), encoding="utf-8")
    return repo


@unittest.skipIf(shutil.which("go") is None, "Go toolchain unavailable; cannot build nomos")
class PublicSourceProcessingTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls._tmp = tempfile.TemporaryDirectory()
        cls.nomos = build_nomos(Path(cls._tmp.name))
        if cls.nomos is None:
            raise unittest.SkipTest("nomos build failed")

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    # --- the real tree: honest counts, six-stage fixture ---------------------

    def test_real_tree_counts_nothing_it_did_not_process(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            summary = proc.process(ROOT, self.nomos, None, Path(tmp) / "retain")
        split = summary["bible_split"]
        self.assertGreaterEqual(split["public_classified_count"], 20)
        # No local capture exists in CI: nothing may be counted as processed.
        self.assertEqual(split["external_public_sources_processed"], 0)
        self.assertEqual(split["external_public_sources_processed"] + split["external_public_sources_blocked"], split["public_classified_count"])
        fx = summary["fixture_documents"]
        self.assertFalse(fx["counted_as_external"])
        self.assertEqual(sorted(fx["files"]), ["README.md", "nomos-bible-corpus-policy.md"])
        for stage in proc.STAGES:
            self.assertEqual(fx["pipeline_steps"][stage], 0, (stage, fx))
        self.assertEqual(fx["source_mutation"], "none")
        self.assertEqual(summary["licensed_leak"], [])
        for r in summary["public_sources"]:
            if r["status"] == "blocked":
                self.assertTrue(r["blocked_reason"], r)
        self.assertIn("never counted as external", summary["claim_boundary"])
        self.assertTrue(proc.acceptance_ok(summary))

    def test_real_snapshots_file_records_dated_hash_only_captures(self) -> None:
        snap = proc.load_snapshots(ROOT)
        self.assertEqual(snap["schema_version"], proc.SNAPSHOTS_SCHEMA)
        captured = [e for e in snap["sources"] if e["status"] == "captured_hash_only"]
        blocked = [e for e in snap["sources"] if e["status"] == "blocked"]
        self.assertGreaterEqual(len(captured), 1)
        for e in captured:
            self.assertRegex(e["sha256"], r"^sha256:[0-9a-f]{64}$")
            self.assertTrue(e["captured_on"])
            self.assertEqual(e["storage_policy"], "hash_only_no_full_text")
        for e in blocked:
            self.assertTrue(e["blocked_reason"].startswith("fetch_failed"), e)

    # --- offline: a capture goes through all six stages and is retained ------

    def test_matching_capture_completes_the_chain_and_is_retained(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            repo = synthetic_repo(tmp, sha256_of(FIXTURE_CAPTURE))
            captures = tmp / "captures"; captures.mkdir()
            shutil.copy(FIXTURE_CAPTURE, captures / FIXTURE_CAPTURE.name)
            retain = tmp / "retain"
            summary = proc.process(repo, self.nomos, captures, retain)

            self.assertEqual(summary["bible_split"]["external_public_sources_processed"], 1, summary["public_sources"])
            src = summary["public_sources"][0]
            self.assertEqual(src["status"], "processed")
            for stage in proc.STAGES:
                self.assertEqual(src["pipeline_steps"][stage], 0, (stage, src))
            # Retained and content-addressed, outside the repo.
            self.assertTrue(retain.is_dir())
            for name in ("snapshot.json", "source-manifest.yaml", "feed.json", "body-ledger.json", "attestation.json", "strict-gate.json"):
                self.assertIn(name, src["artifacts"], src["artifacts"])
                self.assertRegex(src["artifacts"][name], r"^sha256:[0-9a-f]{64}$")
            # The committed receipt addresses those artifacts and the capture.
            receipt_path = repo / src["receipt"]
            receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
            self.assertEqual(receipt["schema_version"], proc.RECEIPT_SCHEMA)
            self.assertEqual(receipt["capture_sha256"], sha256_of(FIXTURE_CAPTURE))
            self.assertEqual(receipt["artifacts"], src["artifacts"])
            self.assertTrue(proc.acceptance_ok(summary))

    def test_one_changed_byte_is_a_named_mismatch_and_the_old_receipt_is_stale(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            repo = synthetic_repo(tmp, sha256_of(FIXTURE_CAPTURE))
            captures = tmp / "captures"; captures.mkdir()
            shutil.copy(FIXTURE_CAPTURE, captures / FIXTURE_CAPTURE.name)
            retain = tmp / "retain"
            first = proc.process(repo, self.nomos, captures, retain)
            self.assertEqual(first["public_sources"][0]["status"], "processed")

            # One byte.
            target = captures / FIXTURE_CAPTURE.name
            target.write_bytes(target.read_bytes().replace(b"Section 4", b"Section 5", 1))
            second = proc.process(repo, self.nomos, captures, retain)
            src = second["public_sources"][0]
            self.assertEqual(src["status"], "blocked")
            self.assertTrue(src["blocked_reason"].startswith("capture_hash_mismatch"), src)
            self.assertEqual(second["bible_split"]["external_public_sources_processed"], 0)
            # The receipt from the first run is now stale — reported, not reused.
            self.assertEqual(len(src["stale_receipts"]), 1, src)
            self.assertTrue(proc.acceptance_ok(second), "a blocked source is an honest state, not a failed run")

    def test_missing_entry_and_blocked_entry_are_explicit(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            repo = synthetic_repo(tmp, None)
            summary = proc.process(repo, self.nomos, None, tmp / "retain")
            src = summary["public_sources"][0]
            self.assertEqual(src["status"], "blocked")
            self.assertTrue(src["blocked_reason"].startswith("no_snapshot_entry"), src)
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            repo = synthetic_repo(tmp, None, blocked=True)
            summary = proc.process(repo, self.nomos, None, tmp / "retain")
            src = summary["public_sources"][0]
            self.assertEqual(src["status"], "blocked")
            self.assertIn("404", src["blocked_reason"])

    def test_hash_only_capture_without_content_is_not_processed(self) -> None:
        # The state the real tree is in: a dated hash exists, the content does not.
        with tempfile.TemporaryDirectory() as tmp:
            tmp = Path(tmp)
            repo = synthetic_repo(tmp, sha256_of(FIXTURE_CAPTURE))
            summary = proc.process(repo, self.nomos, None, tmp / "retain")
            src = summary["public_sources"][0]
            self.assertEqual(src["status"], "blocked")
            self.assertTrue(src.get("captured_hash_only"))
            self.assertEqual(summary["bible_split"]["external_public_sources_captured_hash_only"], 1)
            self.assertEqual(summary["bible_split"]["external_public_sources_processed"], 0)


class ClassificationTests(unittest.TestCase):
    def test_classification_excludes_iso_and_gamp_from_public(self) -> None:
        split = proc.classify_bibles(ROOT)
        licensed_ids = {b["id"] for b in split["licensed"]}
        for blocked in ("ISO-13485-2016", "ISO-IEC-IEEE-12207-2026", "ISPE-GAMP5-2E-2022"):
            self.assertIn(blocked, licensed_ids)
            self.assertNotIn(blocked, split["public"])

    def test_acceptance_refuses_a_processed_source_without_a_receipt(self) -> None:
        # ADVERSARIAL: "processed" must be backed by a receipt, or it is a claim.
        summary = {
            "fixture_documents": {"source_mutation": "none", "pipeline_steps": {s: 0 for s in proc.STAGES}},
            "licensed_leak": [],
            "public_sources": [{"reference_id": "X", "status": "processed"}],
        }
        self.assertFalse(proc.acceptance_ok(summary))
        summary["public_sources"][0]["receipt"] = "docs/…/x.receipt.json"
        self.assertTrue(proc.acceptance_ok(summary))

    def test_processed_requires_every_stage_including_the_strict_gate(self) -> None:
        # ADVERSARIAL: a run where everything but the strict gate passed is NOT
        # processed. Tested on the rule itself, because a fixture where strict
        # happens to pass cannot tell whether strict was required at all.
        green = {s: 0 for s in proc.STAGES}
        self.assertTrue(proc.chain_complete(green, "none"))
        for stage in proc.STAGES:
            with self.subTest(stage=stage):
                steps = dict(green)
                steps[stage] = 1
                self.assertFalse(proc.chain_complete(steps, "none"), f"{stage} failing still counted as processed")
        skipped = dict(green); skipped["strict"] = -1
        self.assertFalse(proc.chain_complete(skipped, "none"), "a stage that never ran counted as passed")
        self.assertFalse(proc.chain_complete(green, "DETECTED"), "a mutated source counted as processed")

    def test_acceptance_refuses_a_blocked_source_without_a_reason(self) -> None:
        summary = {
            "fixture_documents": {"source_mutation": "none", "pipeline_steps": {s: 0 for s in proc.STAGES}},
            "licensed_leak": [],
            "public_sources": [{"reference_id": "X", "status": "blocked"}],
        }
        self.assertFalse(proc.acceptance_ok(summary))


if __name__ == "__main__":
    unittest.main()
