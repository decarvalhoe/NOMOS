"""Le garde des compteurs de capacités des README.

Preuve adversariale (docs/43 §2.3) : chaque test fait échouer le garde sur une
dérive réelle avant de montrer qu'il l'accepte une fois corrigée. Le cas
fondateur est celui du 2026-09-11 : README FR à 57/44/11/2, README EN à
40/32/7/1, matrice à 66/47/17/2.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import readme_capability_counts_guard as guard  # noqa: E402

FR_LINE = (
    "| Registre de capacités | {t} capacités déclarées dans `scripts/vrc_wiring_matrix_registry.json` ; "
    "leur statut est CALCULÉ depuis l'arbre à chaque CI ({r} réelles, {s} sidecar, {a} absentes par conception, 0 écart) — x |"
)
EN_LINE = (
    "| Capability registry | {t} capabilities declared in `scripts/vrc_wiring_matrix_registry.json`; "
    "their status is COMPUTED from the tree on every CI run ({r} real, {s} sidecar, {a} absent, 0 mismatch) — x |"
)
TABLE_FR = "| Matrice de câblage (VRC-00) | {t} capacités, 0 écart entre registre et arbre | y |"
TABLE_EN = "| Wiring matrix (VRC-00) | {t} capabilities, 0 mismatch between registry and tree | y |"
TABLE_DE = "| Wiring-Matrix (VRC-00) | {t} Faehigkeiten, 0 Abweichung zwischen Register und Baum | y |"


def _matrix(total: int, real: int, sidecar: int, absent: int) -> dict:
    return {
        "summary": {
            "capabilities": total,
            "computed": {"real": real, "partial": 0, "sidecar": sidecar, "stub": 0, "absent": absent},
            "mismatches": 0,
        }
    }


class TmpRepo:
    def __init__(self, matrix: dict, readmes: dict[str, str]):
        self.dir = tempfile.TemporaryDirectory()
        self.root = Path(self.dir.name)
        (self.root / ".vrc-wiring-matrix").mkdir()
        (self.root / guard.MATRIX_PATH).write_text(json.dumps(matrix), encoding="utf-8")
        for name, text in readmes.items():
            (self.root / name).write_text(text, encoding="utf-8")

    def read(self, name: str) -> str:
        return (self.root / name).read_text(encoding="utf-8")


class ReadmeCapabilityCountsGuardTest(unittest.TestCase):
    def test_drift_of_2026_09_11_is_refused_then_rewritten(self):
        repo = TmpRepo(
            _matrix(66, 47, 17, 2),
            {
                "README.md": "\n".join([FR_LINE.format(t=57, r=44, s=11, a=2), TABLE_FR.format(t=40)]),
                "README.en.md": "\n".join([EN_LINE.format(t=40, r=32, s=7, a=1), TABLE_EN.format(t=40)]),
                "README.de.md": TABLE_DE.format(t=40),
            },
        )
        self.assertEqual(guard.main(["--root", str(repo.root), "--check"]), 1)
        self.assertEqual(guard.main(["--root", str(repo.root), "--write"]), 0)
        self.assertIn("66 capacités déclarées", repo.read("README.md"))
        self.assertIn("(47 réelles, 17 sidecar, 2 absentes", repo.read("README.md"))
        self.assertIn("| 66 capacités, 0 écart", repo.read("README.md"))
        self.assertIn("(47 real, 17 sidecar, 2 absent", repo.read("README.en.md"))
        self.assertIn("| 66 Faehigkeiten, 0 Abweichung", repo.read("README.de.md"))
        self.assertEqual(guard.main(["--root", str(repo.root), "--check"]), 0)

    def test_aligned_readmes_pass_and_are_left_untouched(self):
        fr = "\n".join([FR_LINE.format(t=66, r=47, s=17, a=2), TABLE_FR.format(t=66)])
        repo = TmpRepo(_matrix(66, 47, 17, 2), {"README.md": fr})
        self.assertEqual(guard.main(["--root", str(repo.root), "--check"]), 0)
        self.assertEqual(guard.main(["--root", str(repo.root), "--write"]), 0)
        self.assertEqual(repo.read("README.md"), fr)

    def test_one_wrong_component_is_enough(self):
        fr = FR_LINE.format(t=66, r=47, s=16, a=3)
        repo = TmpRepo(_matrix(66, 47, 17, 2), {"README.md": fr})
        _, drifts = guard.scan(fr, guard.load_expected(repo.root))
        self.assertEqual(len(drifts), 2)
        self.assertTrue(any("sidecar déclaré 16, matrice 17" in d for d in drifts))
        self.assertTrue(any("absent déclaré 3, matrice 2" in d for d in drifts))

    def test_readme_without_the_sentence_is_not_a_drift(self):
        repo = TmpRepo(_matrix(66, 47, 17, 2), {"README.de.md": "# Nomos\n\nKeine Zahlen hier.\n"})
        self.assertEqual(guard.main(["--root", str(repo.root), "--check"]), 0)

    def test_real_repository_is_aligned(self):
        # Le dépôt lui-même doit passer : c'est la garde vivante, pas un décor.
        self.assertEqual(guard.main(["--root", str(ROOT), "--check"]), 0)


if __name__ == "__main__":
    unittest.main()
