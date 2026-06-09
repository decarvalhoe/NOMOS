from __future__ import annotations

import json
import subprocess
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class CKMPointInTimeTests(unittest.TestCase):
    def test_temporal_atoms_validate_optionally(self) -> None:
        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/atomization-spine.cue",
                "specs/point-in-time.cue",
                "specs/examples/point-in-time-atoms.valid.yaml",
                "-d",
                "#TemporalAtomSet",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_as_of_resolver_selects_version_in_force(self) -> None:
        result = subprocess.run(
            [
                sys.executable,
                str(ROOT / "scripts" / "ckm_point_in_time_resolve.py"),
                "--atoms",
                "specs/examples/point-in-time-atoms.valid.yaml",
                "--work-id",
                "eli:example:regulation:demo",
                "--as-of",
                "2024-03-01",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        resolved = json.loads(result.stdout)
        self.assertEqual(resolved["selected_atom_id"], "ATOM-PIT-2024")
        self.assertEqual(resolved["effective_from"], "2024-01-01")
        self.assertEqual(resolved["effective_to"], "2024-12-31")

    def test_as_of_resolver_refuses_when_no_version_is_in_force(self) -> None:
        result = subprocess.run(
            [
                sys.executable,
                str(ROOT / "scripts" / "ckm_point_in_time_resolve.py"),
                "--atoms",
                "specs/examples/point-in-time-atoms.valid.yaml",
                "--work-id",
                "eli:example:regulation:demo",
                "--as-of",
                "2023-03-01",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
        report = json.loads(result.stdout)
        self.assertEqual(report["status"], "not_in_force")


if __name__ == "__main__":
    unittest.main()
