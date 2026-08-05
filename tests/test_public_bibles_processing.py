"""RCP-010 (#196), public half — process the public bibles read-only.

Adversarial intent (doctrine §2.3): the pipeline must produce atomization
reports + manifests over the public reference corpus WITHOUT mutating the source,
and a licensed bible must never leak into the processed set. The processor
returns non-zero on any of those failures, so a passing run is the proof.

Skips when the Go toolchain is unavailable (offline `unittest discover`).
"""

from __future__ import annotations

import importlib.util
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
_SPEC = importlib.util.spec_from_file_location(
    "process_public_bibles", ROOT / "scripts" / "process_public_bibles.py"
)
proc = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(proc)


def build_nomos(dest_dir: Path) -> str | None:
    if shutil.which("go") is None:
        return None
    binary = dest_dir / ("nomos.exe" if __import__("os").name == "nt" else "nomos")
    result = subprocess.run(
        ["go", "build", "-o", str(binary), "."],
        cwd=ROOT / "cli",
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        return None
    return str(binary)


class PublicBiblesProcessingTests(unittest.TestCase):
    def test_public_half_processes_read_only_without_licensed_leak(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            nomos_bin = build_nomos(Path(tmp))
            if nomos_bin is None:
                self.skipTest("Go toolchain unavailable; cannot build nomos")

            summary = proc.process(ROOT, nomos_bin)

            # Read-only guarantee: zero source mutation.
            self.assertEqual(summary["read_only_guard"], "pass", summary)
            self.assertEqual(summary["source_mutation"], "none", summary)

            # Deliverables exist.
            self.assertTrue(summary["artifacts_present"]["snapshot"], summary)
            self.assertTrue(summary["artifacts_present"]["manifest"], summary)
            self.assertEqual(summary["pipeline_steps"].get("scan"), 0, summary)
            self.assertEqual(summary["pipeline_steps"].get("manifest"), 0, summary)

            # The licensed bibles are blocked and never leak into processing.
            self.assertEqual(summary["licensed_leak"], [], summary)
            licensed_ids = {b["id"] for b in summary["bible_split"]["licensed_blocked"]}
            self.assertIn("ISPE-GAMP5-2E-2022", licensed_ids, summary)
            self.assertGreaterEqual(summary["bible_split"]["public_count"], 1, summary)

            # The acceptance predicate the CLI exit code keys off of.
            self.assertTrue(proc.acceptance_ok(summary), summary)

    def test_classification_excludes_iso_and_gamp_from_public(self) -> None:
        split = proc.classify_bibles(ROOT)
        licensed_ids = {b["id"] for b in split["licensed"]}
        for blocked in ("ISO-13485-2016", "ISO-IEC-IEEE-12207-2026", "ISPE-GAMP5-2E-2022"):
            self.assertIn(blocked, licensed_ids)
            self.assertNotIn(blocked, split["public"])


if __name__ == "__main__":
    unittest.main()
