"""NRT-025 (#678) — the security gate turns red on what it names, and only an
unexpired, complete allowlist entry can accept a finding."""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "security_process_gate.py"

GOVULN = "\n".join(json.dumps(x) for x in [
    {"config": {"protocol_version": "v1.0.0"}},
    {"osv": {"id": "GO-2099-0001", "summary": "fixture"}},
    {"finding": {"osv": "GO-2099-0001", "trace": [{"module": "example.com/vuln", "function": "Do"}]}},
]) + "\n"
PIPAUDIT = json.dumps({"dependencies": [{"name": "somepkg", "version": "1.0.0", "vulns": [{"id": "PYSEC-2099-1", "fix_versions": ["1.0.1"]}]}]})


def run(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), "--root", str(root), "--today", "2026-09-06", *args], capture_output=True, text=True)


def copy_root(tmp: Path) -> Path:
    root = tmp / "repo"
    for rel in ("docs/security", "SECURITY.md", "CHANGELOG.md", "docs/regulated/security-privacy/vulnerability-and-incident-management-sop.md", ".github/dependabot.yml",
                "cli/go.mod", "tools/sigstore-verifier/go.mod", "scripts/requirements-sidecar.txt",
                "adapters/node-typescript/fixtures/nextjs-api-ui/package.json", "adapters/python/fixtures/django-app/pyproject.toml", "adapters/jvm/fixtures/spring-rest-service/pom.xml",
                "adapters/python/fixtures/fastapi-service/pyproject.toml", "adapters/python/fixtures/flask-api/pyproject.toml",
                "cli/internal/detect/testdata/corpus/fullstack/go.mod", "cli/internal/detect/testdata/corpus/fullstack/web/package.json"):
        src, dst = ROOT / rel, root / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        (shutil.copytree if src.is_dir() else shutil.copy2)(src, dst)
    return root


def allowlist_with(root: Path, entry: dict) -> None:
    import yaml
    p = root / "docs/security/vulnerability-allowlist.yaml"
    doc = yaml.safe_load(p.read_text(encoding="utf-8"))
    doc["entries"] = [entry]
    p.write_text(yaml.safe_dump(doc, sort_keys=False), encoding="utf-8")


class SecurityGateTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = copy_root(Path(self._tmp.name))
        self.gv = Path(self._tmp.name) / "gv.json"; self.gv.write_text(GOVULN)
        self.pa = Path(self._tmp.name) / "pa.json"; self.pa.write_text(PIPAUDIT)

    def test_repository_is_green_and_docs_fresh(self) -> None:
        r = run(ROOT, "--check")
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_finding_without_acceptance_is_red(self) -> None:
        r = run(self.root, "--govulncheck", str(self.gv))
        self.assertEqual(r.returncode, 1)
        self.assertIn("GO-2099-0001", r.stderr)
        r = run(self.root, "--pip-audit", str(self.pa))
        self.assertEqual(r.returncode, 1)
        self.assertIn("PYSEC-2099-1", r.stderr)

    def test_live_complete_acceptance_is_green_but_expired_is_red(self) -> None:
        base = {"id": "GO-2099-0001", "ecosystem": "go", "package": "example.com/vuln", "justification": "not reachable in our use", "owner": "security_owner", "accepted_on": "2026-09-01", "expires_on": "2026-12-01"}
        allowlist_with(self.root, base)
        r = run(self.root, "--govulncheck", str(self.gv))
        self.assertEqual(r.returncode, 0, r.stderr)
        allowlist_with(self.root, {**base, "expires_on": "2026-09-05"})
        r = run(self.root, "--govulncheck", str(self.gv))
        self.assertEqual(r.returncode, 1)
        self.assertIn("expired on 2026-09-05", r.stderr)
        self.assertIn("GO-2099-0001 in example.com/vuln is not accepted", r.stderr)

    def test_incomplete_acceptance_is_red(self) -> None:
        allowlist_with(self.root, {"id": "GO-2099-0001", "ecosystem": "go", "package": "x", "justification": "j", "owner": "o", "accepted_on": "2026-09-01"})
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("missing expires_on", r.stderr)

    def test_process_without_gated_scanner_is_red(self) -> None:
        p = self.root / "docs/security/security-process.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("    gate: true\n  - id: pip-audit", "    gate: false\n  - id: pip-audit", 1), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("must be a gate", r.stderr)

    def test_unscanned_manifest_is_red(self) -> None:
        m = self.root / "tools" / "forgotten" / "package.json"
        m.parent.mkdir(parents=True)
        m.write_text("{}")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("manifest tools/forgotten/package.json is neither scanned, watched by Dependabot, nor listed", r.stderr)

    def test_stale_manifest_exclusion_is_red(self) -> None:
        p = self.root / "docs/security/security-process.yaml"
        p.write_text(p.read_text(encoding="utf-8").replace("  - path: cli/internal/detect/testdata/corpus/fullstack/go.mod\n", "  - path: cli/internal/detect/testdata/corpus/gone/go.mod\n", 1), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("manifests_not_scanned lists cli/internal/detect/testdata/corpus/gone/go.mod, which does not exist", r.stderr)

    def test_stale_supported_versions_is_drift(self) -> None:
        p = self.root / "SECURITY.md"
        p.write_text(p.read_text(encoding="utf-8").replace("Best-effort alpha triage", "Full support"), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 4)
        self.assertIn("DRIFT", r.stderr)


if __name__ == "__main__":
    unittest.main()
