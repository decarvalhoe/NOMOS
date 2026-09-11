"""NRT-027 (#680) — the integration guide runs, or CI is red and says which block.

Synthetic guides prove the refusals with a stub binary; the real guide is
replayed end to end when Go is available (it builds cli/)."""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "integration_guide_replay.py"
TRUE = shutil.which("true") or "/bin/true"

GOOD = """# guide

Prose may mention `<!-- replay -->` without starting a block.

<!-- contracts -->

| Contrat | Stabilite | Version | Role |
|---|---|---|---|
| `nomos-project` | stable | 0.1.0 | projet |
| `adapter-manifest` | experimental | 0.1.0 | adapter |

<!-- replay expects: out/a.txt -->

```bash
echo a > out/a.txt
```

<!-- replay -->

```bash
test -f out/a.txt
```

<!-- replay expects: out/b.txt -->

```bash
"$NOMOS" && echo b > out/b.txt
```
"""


def run(root: Path, guide: Path, *extra: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), "--root", str(root), "--guide", str(guide), "--nomos", TRUE, "--min-blocks", "3", *extra], capture_output=True, text=True)


class ReplayTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name) / "repo"
        (self.root / "specs").mkdir(parents=True)
        shutil.copy2(ROOT / "specs" / "contract-registry.yaml", self.root / "specs" / "contract-registry.yaml")
        self.guide = self.root / "guide.md"

    def write(self, text: str) -> None:
        self.guide.write_text(text, encoding="utf-8")

    def test_good_synthetic_guide_is_green(self) -> None:
        self.write(GOOD)
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 0, r.stderr + r.stdout)
        self.assertIn("3 block(s) ran", r.stdout)
        self.assertIn("2 contract row(s) agree", r.stdout)

    def test_broken_command_is_red_and_named(self) -> None:
        self.write(GOOD.replace("test -f out/a.txt", "test -f out/a.txt\nfalse  # regression", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("block 2 (guide:", r.stderr)
        self.assertIn("exited 1", r.stderr)

    def test_missing_expected_artifact_is_red(self) -> None:
        self.write(GOOD.replace("echo a > out/a.txt", "echo a > out/other.txt", 1).replace("test -f out/a.txt", "true", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("ran but did not produce: out/a.txt", r.stderr)

    def test_stability_not_confirmed_by_registry_is_red(self) -> None:
        self.write(GOOD.replace("| `adapter-manifest` | experimental |", "| `adapter-manifest` | stable |", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("`adapter-manifest` is declared stable in the guide but experimental in the registry", r.stderr)
        self.write(GOOD.replace("| `nomos-project` | stable | 0.1.0 |", "| `nomos-project` | stable | 9.9.9 |", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("version 9.9.9 in the guide, 0.1.0 in the registry", r.stderr)

    def test_unknown_contract_is_red(self) -> None:
        self.write(GOOD.replace("`adapter-manifest`", "`imaginary-contract`", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("contract `imaginary-contract` is not in specs/contract-registry.yaml", r.stderr)

    def test_guide_without_enough_blocks_or_contracts_is_red(self) -> None:
        self.write(GOOD.split("<!-- replay -->\n")[0])
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("has 1 replay block(s); at least 3 are required", r.stderr)
        self.write(GOOD.replace("<!-- contracts -->", "<!-- table -->", 1))
        r = run(self.root, self.guide)
        self.assertEqual(r.returncode, 1)
        self.assertIn("names no contract", r.stderr)

    def test_marker_without_fence_is_refused(self) -> None:
        self.write(GOOD.replace("```bash\necho a > out/a.txt\n```", "echo a > out/a.txt", 1))
        r = run(self.root, self.guide)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("replay marker is not followed by a fenced block", r.stderr)


class RealGuideTest(unittest.TestCase):
    def test_the_real_guide_runs_against_the_fixtures(self) -> None:
        if not shutil.which("go") or not shutil.which("jq"):
            self.skipTest("go and jq are required to replay the real guide")
        r = subprocess.run([sys.executable, str(SCRIPT), "--root", str(ROOT)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr + r.stdout)
        self.assertIn("block(s) ran against the fixtures", r.stdout)


if __name__ == "__main__":
    unittest.main()
