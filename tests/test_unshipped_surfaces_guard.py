"""Le garde des surfaces non livrées (REP-1, #747).

Preuve adversariale (docs/43 §2.3) : chaque test fait d'abord rougir le garde
sur une revendication réelle — « the SDK is shipped » avec un `sdk/` réduit à
son README — avant de montrer qu'il l'accepte dès qu'un vrai fichier existe
sous la surface. Le cas fondateur est celui du 2026-09-11 : les trois README
omettaient la ligne `sdk/` de leur tableau d'arborescence.
"""

from __future__ import annotations

import json
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import unshipped_surfaces_guard as guard  # noqa: E402

TABLE = "| `cli/` | Go CLI. |\n| `sdk/` | No SDK shipped (README only). |\n| `policies/` | No executable policy (README only). |\n"


class TmpRepo:
    def __init__(self, files: dict[str, str]):
        self.dir = tempfile.TemporaryDirectory()
        self.root = Path(self.dir.name)
        for name, text in files.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")


class UnshippedSurfacesGuardTest(unittest.TestCase):
    def test_sdk_claimed_shipped_with_readme_only_sdk_is_refused(self):
        repo = TmpRepo(
            {
                "sdk/README.md": "# SDK\n",
                "README.md": "# Nomos\n\nThe SDK is shipped with this release.\n\n" + TABLE,
            }
        )
        report = repo.root / "report.json"
        self.assertEqual(guard.main(["--root", str(repo.root), "--check", "--report", str(report)]), 1)
        data = json.loads(report.read_text(encoding="utf-8"))
        self.assertFalse(data["ok"])
        self.assertTrue(data["surfaces"]["sdk/"]["readme_only"])
        self.assertEqual([(f["file"], f["line"], f["surface"]) for f in data["failures"]], [("README.md", 3, "sdk/")])

    def test_same_claim_with_a_real_file_under_sdk_passes(self):
        repo = TmpRepo(
            {
                "sdk/README.md": "# SDK\n",
                "sdk/nomos/__init__.py": "__version__ = '0.0.1'\n",
                "README.md": "# Nomos\n\nThe SDK is shipped with this release.\n\n" + TABLE,
            }
        )
        report = repo.root / "report.json"
        self.assertEqual(guard.main(["--root", str(repo.root), "--check", "--report", str(report)]), 0)
        data = json.loads(report.read_text(encoding="utf-8"))
        self.assertFalse(data["surfaces"]["sdk/"]["readme_only"])
        self.assertEqual(data["surfaces"]["sdk/"]["files"], ["nomos/__init__.py"])

    def test_missing_sdk_row_in_directory_table_is_refused(self):
        table_without_sdk = "| `cli/` | Go CLI. |\n| `policies/` | No executable policy (README only). |\n"
        repo = TmpRepo({"sdk/README.md": "# SDK\n", "README.md": "# Nomos\n\n" + table_without_sdk})
        report = repo.root / "report.json"
        self.assertEqual(guard.main(["--root", str(repo.root), "--check", "--report", str(report)]), 1)
        failures = json.loads(report.read_text(encoding="utf-8"))["failures"]
        self.assertEqual(len(failures), 1)
        self.assertEqual(failures[0]["surface"], "sdk/")
        self.assertIn("absente du tableau", failures[0]["reason"])
        self.assertEqual(failures[0]["line"], 3)

    def test_claims_in_the_three_languages_are_refused_and_negations_accepted(self):
        claims = {
            "README.md": "Le SDK est livré et les politiques sont exécutables.",
            "README.en.md": "NOMOS ships a stable SDK and executable policies.",
            "README.de.md": "Das SDK ist verfuegbar und die Policies sind operativ.",
        }
        for name, sentence in claims.items():
            with self.subTest(readme=name):
                repo = TmpRepo({"sdk/README.md": "#\n", "policies/README.md": "#\n", name: sentence + "\n\n" + TABLE})
                self.assertEqual(guard.main(["--root", str(repo.root)]), 1)
        negated = {
            "README.md": "Aucun SDK livré ; aucune politique exécutable.",
            "README.en.md": "No SDK is shipped; no executable policy lives here.",
            "README.de.md": "Kein SDK ausgeliefert; keine ausfuehrbare Policy.",
        }
        for name, sentence in negated.items():
            with self.subTest(readme=name):
                repo = TmpRepo({"sdk/README.md": "#\n", "policies/README.md": "#\n", name: sentence + "\n\n" + TABLE})
                self.assertEqual(guard.main(["--root", str(repo.root)]), 0)

    def test_readme_only_example_claimed_tooled_is_refused_until_a_file_exists(self):
        files = {
            "examples/clinical/README.md": "# Clinique\n",
            "README.md": "The `clinical/` example is a tooled, runnable example.\n",
        }
        repo = TmpRepo(files)
        self.assertEqual(guard.main(["--root", str(repo.root)]), 1)
        repo = TmpRepo({**files, "examples/clinical/source-manifest.example.yaml": "sources: []\n"})
        self.assertEqual(guard.main(["--root", str(repo.root)]), 0)

    def test_surfaces_are_computed_from_the_tree(self):
        surfaces = guard.compute_surfaces(ROOT)
        self.assertTrue(surfaces["sdk/"]["readme_only"])
        self.assertTrue(surfaces["policies/"]["readme_only"])
        self.assertTrue(surfaces["examples/clinical/"]["readme_only"])
        self.assertTrue(surfaces["examples/fiscal-tax/"]["readme_only"])
        self.assertFalse(surfaces["examples/insurance/"]["readme_only"])

    def test_real_repository_passes(self):
        # Le dépôt lui-même doit passer : c'est la garde vivante, pas un décor.
        self.assertEqual(guard.main(["--root", str(ROOT), "--check"]), 0)


if __name__ == "__main__":
    unittest.main()
