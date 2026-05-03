from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def run_script(script: str, *args: str, cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(ROOT / "scripts" / script), *args],
        cwd=cwd,
        text=True,
        capture_output=True,
        check=False,
    )


def write_minimal_lawbook_feed(artifacts: Path) -> None:
    write(
        artifacts / "rbok-lawbook-feed.json",
        json.dumps(
            {
                "schema_version": "0.1.0",
                "feeds": [
                    {
                        "feed_id": "sample-feed",
                        "nodes": [
                            {
                                "node_id": "DOC1",
                                "node_type": "document",
                                "source_path": "sample.md",
                                "ordinal_path": "1",
                            },
                            {
                                "node_id": "P1",
                                "node_type": "paragraph",
                                "parent_id": "DOC1",
                                "source_path": "sample.md",
                                "ordinal_path": "1.1",
                                "text": "Intro with a link.",
                            },
                        ],
                    }
                ],
            },
            indent=2,
        ),
    )


class CorpusFidelityProofTests(unittest.TestCase):
    def test_proof_reports_partial_when_semantic_blocks_are_not_represented(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            corpus = repo / "corpus"
            artifacts = repo / "artifacts"
            write(
                corpus / "sample.md",
                """
# Sample

Intro with a [link](https://example.com).

| Term | Definition |
|---|---|
| Alpha | First |

> Important callout.

```yaml
alpha: true
```
""".lstrip(),
            )
            write_minimal_lawbook_feed(artifacts)
            output = repo / "out/fidelity-proof.json"

            result = run_script(
                "corpus_fidelity_proof.py",
                "--corpus",
                str(corpus),
                "--artifacts-dir",
                str(artifacts),
                "--profile",
                "rbok-lawbook",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "partial")
            self.assertFalse(report["full_fidelity_claim_allowed"])
            self.assertEqual(report["source_scan"]["tables"], 1)
            blocker_codes = {finding["code"] for finding in report["findings"] if finding["blocking"]}
            self.assertIn("TABLE_BLOCKS_NOT_TYPED", blocker_codes)
            self.assertIn("CODE_BLOCKS_NOT_TYPED", blocker_codes)
            self.assertIn("CALLOUT_BLOCKS_NOT_TYPED", blocker_codes)
            self.assertIn("BYTE_SPANS_MISSING", blocker_codes)

    def test_strict_mode_fails_when_full_fidelity_claim_is_blocked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            corpus = repo / "corpus"
            artifacts = repo / "artifacts"
            write(corpus / "sample.md", "# Sample\n\nParagraph.\n")
            write_minimal_lawbook_feed(artifacts)
            output = repo / "out/fidelity-proof.json"

            result = run_script(
                "corpus_fidelity_proof.py",
                "--corpus",
                str(corpus),
                "--artifacts-dir",
                str(artifacts),
                "--profile",
                "rbok-lawbook",
                "--report",
                str(output),
                "--strict",
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "partial")
            self.assertFalse(report["full_fidelity_claim_allowed"])


if __name__ == "__main__":
    unittest.main()
