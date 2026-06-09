from __future__ import annotations

import subprocess
import shutil
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


class CKMFacetContractTests(unittest.TestCase):
    def test_existing_spine_without_facets_remains_valid(self) -> None:
        result = run_cue(
            "vet",
            "specs/atomization-spine.cue",
            "specs/examples/atomization-spine.valid.yaml",
            "-d",
            "#AtomizationSpine",
        )

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_faceted_atom_and_chunk_metadata_validate_optionally(self) -> None:
        for definition, fixture in [
            ("#FacetedAtom", "specs/examples/facets.atom.valid.yaml"),
            ("#FacetedChunk", "specs/examples/facets.chunk.valid.yaml"),
        ]:
            with self.subTest(definition=definition):
                result = run_cue(
                    "vet",
                    "specs/atomization-spine.cue",
                    "specs/facets.cue",
                    fixture,
                    "-d",
                    definition,
                )

                self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_facets_reject_unknown_core_axis_value(self) -> None:
        result = run_cue(
            "vet",
            "specs/atomization-spine.cue",
            "specs/facets.cue",
            "specs/examples/facets.invalid-trust-tier.yaml",
            "-d",
            "#FacetedAtom",
        )

        self.assertNotEqual(result.returncode, 0, "invalid trust tier unexpectedly passed")
        self.assertIn("trust_tier", result.stderr + result.stdout)


if __name__ == "__main__":
    unittest.main()
