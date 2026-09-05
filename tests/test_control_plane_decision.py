"""NRT-022 (#670, ADR-0007) — the control-plane decision is executed, not narrated:
the archived directory is gone, the repository map and project manifest no longer
declare it, the ADRs record the chain, and the multi-project view is the engine's."""

from __future__ import annotations

import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]


class ControlPlaneDecisionTests(unittest.TestCase):
    def test_archived_code_is_gone(self) -> None:
        self.assertFalse((ROOT / "control-plane").exists(), "control-plane/ must not come back (ADR-0007)")

    def test_repository_map_and_manifest_no_longer_declare_it(self) -> None:
        for name in ("README.md", "README.en.md", "README.de.md"):
            self.assertNotIn("| `control-plane/` |", (ROOT / name).read_text(encoding="utf-8"), name)
        manifest = yaml.safe_load((ROOT / "nomos.project.yaml").read_text(encoding="utf-8"))
        self.assertNotIn("control-plane", [s.get("name") for s in manifest.get("surfaces", [])])

    def test_adr_chain_is_recorded(self) -> None:
        adr6 = (ROOT / "docs/decisions/0006-control-plane-archive.md").read_text(encoding="utf-8")
        adr7 = (ROOT / "docs/decisions/0007-control-plane-decision-portfolio-view.md").read_text(encoding="utf-8")
        self.assertIn("Status: superseded by [ADR-0007]", adr6)
        self.assertIn("**câblé**", adr7)
        self.assertIn("**retiré**", adr7)

    def test_view_lives_in_the_engine(self) -> None:
        self.assertTrue((ROOT / "cli/internal/portfolio/projects.go").exists())
        self.assertIn("case \"projects\":", (ROOT / "cli/internal/app/portfolio_cmd.go").read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
