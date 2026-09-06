"""docs/43 principle 8 — the silent handlers fixed under the silence guard
(docs/49 §3) now say something. The proofs are the refusals:

* an audit export manifest that exists but cannot be read is never
  reinitialised silently: the export stops;
* a scoped file the claim guard cannot read is a violation, not a skip;
* a publish candidate the path guard cannot inspect is refused, never assumed
  safe.
"""

from __future__ import annotations

import importlib.util
import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))


def load_script(name: str):
    spec = importlib.util.spec_from_file_location(name, SCRIPTS / f"{name}.py")
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


audit_export = load_script("regulated_audit_log_export")
claim_guard = load_script("claim_boundary_guard")
publish = load_script("nomos_github_publish")


class AuditManifestTests(unittest.TestCase):
    def test_missing_manifest_starts_a_fresh_chain(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            manifest = audit_export.load_manifest(Path(tmp) / "manifest.json")
        self.assertEqual(manifest["exports"], [])
        self.assertEqual(manifest["policy_ref"], "RCP-004")

    def test_corrupt_manifest_is_refused_not_reinitialised(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "manifest.json"
            path.write_text('{"schema_version": "0.1.0", "exports": [ {truncated', encoding="utf-8")
            with self.assertRaises(ValueError) as caught:
                audit_export.load_manifest(path)
        self.assertIn("refusing to reinitialise", str(caught.exception))

    def test_readable_manifest_is_returned_as_is(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "manifest.json"
            path.write_text(json.dumps({"schema_version": "0.1.0", "policy_ref": "RCP-004", "exports": [{"filename": "a.jsonl"}]}), encoding="utf-8")
            manifest = audit_export.load_manifest(path)
        self.assertEqual([e["filename"] for e in manifest["exports"]], ["a.jsonl"])


@unittest.skipIf(hasattr(os, "geteuid") and os.geteuid() == 0, "root reads unreadable files; the permission probe needs an unprivileged user")
class ClaimGuardUnreadableFileTests(unittest.TestCase):
    def test_unreadable_scoped_file_is_a_violation_not_a_skip(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            docs = root / "docs"
            docs.mkdir()
            readable = docs / "fine.md"
            readable.write_text("# Fine\n\nNothing claimed here.\n", encoding="utf-8")
            unreadable = docs / "secret.md"
            unreadable.write_text("# Hidden\n\nAttestations are Sigstore-signed.\n", encoding="utf-8")
            unreadable.chmod(0)
            try:
                violations = claim_guard.scan(root)
                duplicates = claim_guard.find_duplicate_claims(root)
            finally:
                unreadable.chmod(0o644)
        unread = [v for v in violations if v[0] == unreadable]
        self.assertTrue(unread, f"the unreadable file must be reported, got {violations}")
        self.assertEqual(unread[0][1], 0)
        self.assertIn("could not be read", unread[0][2])
        self.assertTrue(any(v[0] == unreadable and "could not be read" in v[2] for v in duplicates))


class PublishPathGuardTests(unittest.TestCase):
    def test_uninspectable_candidate_is_refused_not_assumed_safe(self) -> None:
        with mock.patch.object(publish.os.path, "islink", side_effect=OSError("stat failed")):
            violating, reason = publish._is_violating("out/artifact.json", "out")
        self.assertTrue(violating)
        self.assertIn("could not be inspected", reason)

    def test_plain_candidate_inside_target_is_accepted(self) -> None:
        violating, reason = publish._is_violating("out/artifact.json", "out")
        self.assertFalse(violating, reason)


if __name__ == "__main__":
    unittest.main()
