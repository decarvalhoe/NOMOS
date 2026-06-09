from __future__ import annotations

import shutil
import subprocess
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def run_cue(*args: str) -> subprocess.CompletedProcess[str]:
    if shutil.which("cue") is None:
        raise unittest.SkipTest("cue is not installed")

    return subprocess.run(
        ["cue", *args],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


class CKMFacetOntologyTests(unittest.TestCase):
    def test_bfo_iof_pack_ontology_and_odp_validate(self) -> None:
        result = run_cue(
            "vet",
            "specs/facet-ontology.cue",
            "specs/examples/facet-ontology.valid.yaml",
            "-d",
            "#FacetOntology",
        )

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_axis_disjoint_union_rejects_overlapping_terms(self) -> None:
        result = subprocess.run(
            [
                sys.executable,
                "scripts/ckm_facet_ontology_validate.py",
                "--ontology",
                "specs/examples/facet-ontology.invalid-overlap.yaml",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertNotEqual(result.returncode, 0, "overlapping axis terms unexpectedly passed")
        self.assertIn("disjoint", result.stderr + result.stdout)


if __name__ == "__main__":
    unittest.main()
