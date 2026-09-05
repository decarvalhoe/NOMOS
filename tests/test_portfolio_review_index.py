"""NRT-021 (#669) — the review-record index is a fresh build of the committed
records, and its guard turns red on the defects it names."""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "portfolio_review_index.py"
RECORDS = Path("docs/regulated/operations/records")


def run(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), "--root", str(root), *args], capture_output=True, text=True)


def copy_records(tmp: Path) -> Path:
    root = tmp / "repo"
    shutil.copytree(ROOT / RECORDS, root / RECORDS)
    # cited artifacts the real records point at
    for rel in ("docs/45-vision-reality-closure-plan.md", "docs/46-vrc-epic-issue-list.md", "docs/decisions/0006-control-plane-archive.md",
                ".vrc-wiring-matrix/wiring-matrix.md", "docs/regulated/lifecycle/validation-master-plan.md", "docs/public-claim-boundary.md",
                "cli/internal/app/corpus_cmd.go", "cli/internal/corpus/body_ledger_verify.go", "cli/internal/app/attest_supply_chain.go"):
        src = ROOT / rel
        if src.exists():
            (root / rel).parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, root / rel)
    index = json.loads((ROOT / RECORDS / "index.json").read_text(encoding="utf-8"))
    for r in index["records"]:
        for c in r["cited_artifacts"]:
            src = ROOT / c["path"]
            if src.is_file() and not (root / c["path"]).exists():
                (root / c["path"]).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(src, root / c["path"])
    return root


class ReviewIndexTests(unittest.TestCase):
    def test_committed_index_is_fresh_and_guard_passes(self) -> None:
        r = run(ROOT, "--check")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        index = json.loads((ROOT / RECORDS / "index.json").read_text(encoding="utf-8"))
        self.assertEqual(index["schema_version"], "nomos-review-record-index-v1")
        self.assertEqual(index["cited_artifacts_missing"], 0)
        self.assertGreaterEqual(index["total"], 6)
        self.assertEqual(index["by_type"]["deviation_capa"], 3)
        for r in index["records"]:
            self.assertTrue(r["record_id"] and r["date"] and r["sha256"].startswith("sha256:"), r)

    def test_document_numbers_are_not_paths(self) -> None:
        sys.path.insert(0, str(ROOT / "scripts"))
        import portfolio_review_index as m  # noqa: E402
        self.assertEqual(m.cited_paths("see docs/45 §1 and docs/45-vision-reality-closure-plan.md, plus scripts/x.py."),
                         ["docs/45-vision-reality-closure-plan.md", "scripts/x.py"])

    def _mutated(self, edit) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as tmp:
            root = copy_records(Path(tmp))
            edit(root)
            return run(root)

    def test_removed_cited_artifact_is_red(self) -> None:
        r = self._mutated(lambda root: (root / "docs/decisions/0006-control-plane-archive.md").unlink())
        self.assertEqual(r.returncode, 1)
        self.assertIn("does not exist in the tree", r.stderr)

    def test_action_without_owner_is_red(self) -> None:
        def edit(root: Path) -> None:
            p = root / RECORDS / "2026-06-11-management-review-record.yaml"
            p.write_text(p.read_text(encoding="utf-8").replace("    owner: release_manager\n", "", 1), encoding="utf-8")
        r = self._mutated(edit)
        self.assertEqual(r.returncode, 1)
        self.assertIn("has no owner", r.stderr)

    def test_decision_without_id_is_red(self) -> None:
        def edit(root: Path) -> None:
            p = root / RECORDS / "2026-06-11-management-review-record.yaml"
            p.write_text(p.read_text(encoding="utf-8").replace("  - id: MR-2026-001-D1\n", "  - \n", 1), encoding="utf-8")
        r = self._mutated(edit)
        self.assertEqual(r.returncode, 1)
        self.assertIn("has no id", r.stderr)

    def test_undated_record_and_closed_capa_without_date_are_red(self) -> None:
        def edit(root: Path) -> None:
            p = root / RECORDS / "2026-06-11-internal-audit-record.yaml"
            p.write_text(p.read_text(encoding="utf-8").replace("date: 2026-06-11\n", "", 1), encoding="utf-8")
            c = root / RECORDS / "capa/CAPA-2026-002-orphan-commands.yaml"
            c.write_text(c.read_text(encoding="utf-8").replace("closed: 2026-06-11\n", "", 1), encoding="utf-8")
        r = self._mutated(edit)
        self.assertEqual(r.returncode, 1)
        self.assertIn("date missing", r.stderr)
        self.assertIn("closed CAPA without a closed date", r.stderr)

    def test_duplicate_record_id_is_red(self) -> None:
        def edit(root: Path) -> None:
            src = root / RECORDS / "2026-06-11-internal-audit-record.yaml"
            (root / RECORDS / "2026-06-12-copy.yaml").write_text(src.read_text(encoding="utf-8"), encoding="utf-8")
        r = self._mutated(edit)
        self.assertEqual(r.returncode, 1)
        self.assertIn("appears more than once", r.stderr)

    def test_stale_index_is_drift(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = copy_records(Path(tmp))
            p = root / RECORDS / "index.json"
            p.write_text(p.read_text(encoding="utf-8").replace('"total": ', '"total": 1'), encoding="utf-8")
            r = run(root, "--check")
            self.assertEqual(r.returncode, 4)
            self.assertIn("DRIFT", r.stderr)


if __name__ == "__main__":
    unittest.main()
