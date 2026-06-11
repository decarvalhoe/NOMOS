"""CKM-H1-FU — the claim-boundary guard turns red on an unbacked attestation claim.

Audit (#537): PR #529 added REAL ECDSA P-256 DSSE signing but never delivered the
promised CI claim-boundary guard. This test pins the guard's contract with an
ADVERSARIAL proof (doctrine §2.3): injecting a bogus "attestations are
Sigstore-signed" line makes the guard EXIT NON-ZERO; the real tree passes. The
failure on the forged claim is the proof.
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "claim_boundary_guard.py"

_SPEC = importlib.util.spec_from_file_location("claim_boundary_guard", SCRIPT)
guard = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(guard)

SIGNING_GO = """package attestation
// ecdsa dsse SignASN1 VerifyASN1 — real signing capability marker.
func SignASN1() {}
func VerifyASN1() {}
"""


def _make_tree(tmp: Path, *, with_signing: bool, readme_extra: str = "") -> Path:
    """Build a minimal repo tree the guard understands."""
    root = tmp / "repo"
    (root / "attestations").mkdir(parents=True)
    (root / "docs").mkdir(parents=True)
    if with_signing:
        marker = root / "cli" / "internal" / "attestation"
        marker.mkdir(parents=True)
        (marker / "signing.go").write_text(SIGNING_GO, encoding="utf-8")

    # The affirmative signing claim is on its own line (no negation), so the
    # no-marker case exposes a genuinely bare claim. The downgrade is a separate
    # line carrying its own honest "unsigned / not signed" context.
    readme = (
        "# Attestations\n\n"
        "## Cryptographic Signing (ECDSA P-256 DSSE)\n\n"
        "NOMOS attestations are cryptographically signed with ECDSA P-256 DSSE.\n\n"
        "Until a predicate is signed this way it is unsigned and tamper-evident by hash only.\n\n"
        "### Intended Expansion\n\n"
        "- Keyless Sigstore/Fulcio + Rekor transparency-log workflows (follow-up).\n"
    )
    (root / "attestations" / "README.md").write_text(readme + readme_extra, encoding="utf-8")
    (root / "README.md").write_text("# NOMOS\n\nA canonical knowledge engine.\n", encoding="utf-8")
    return root


class ClaimBoundaryGuardTests(unittest.TestCase):
    def test_clean_tree_with_signing_capability_passes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _make_tree(Path(tmp), with_signing=True)
            self.assertTrue(guard.signing_capability_present(root))
            self.assertEqual(guard.scan(root), [], "honest tree must be clean")
            self.assertEqual(guard.main(["--root", str(root)]), 0)

    def test_bogus_sigstore_signed_claim_turns_guard_red(self) -> None:
        # The headline adversarial proof: a forged "attestations are
        # Sigstore-signed" line must make the guard exit non-zero even though the
        # real ECDSA signing capability is present (Sigstore keyless is NOT).
        bogus = "\n## Trust\n\nAll NOMOS attestations are Sigstore-signed and cryptographically certified.\n"
        with tempfile.TemporaryDirectory() as tmp:
            root = _make_tree(Path(tmp), with_signing=True, readme_extra=bogus)
            violations = guard.scan(root)
            self.assertTrue(violations, "bogus Sigstore-signed claim must be flagged")
            self.assertTrue(
                any("Sigstore" in reason for *_, reason in violations),
                violations,
            )
            self.assertEqual(guard.main(["--root", str(root)]), 1)

    def test_revert_makes_guard_green_again(self) -> None:
        # Same tree without the bogus line passes — proving the guard reacts to
        # the claim, not to incidental content.
        with tempfile.TemporaryDirectory() as tmp:
            root = _make_tree(Path(tmp), with_signing=True)
            self.assertEqual(guard.main(["--root", str(root)]), 0)

    def test_crypto_signing_claim_without_marker_fails(self) -> None:
        # "proof = the real signing capability is present": strip the marker and
        # the same honest-looking signing claim is no longer backed → red.
        with tempfile.TemporaryDirectory() as tmp:
            root = _make_tree(Path(tmp), with_signing=False)
            self.assertFalse(guard.signing_capability_present(root))
            violations = guard.scan(root)
            self.assertTrue(
                any("marker" in reason for *_, reason in violations),
                f"unbacked signing claim must fail without the marker; got {violations}",
            )
            self.assertEqual(guard.main(["--root", str(root)]), 1)

    def test_negated_and_quoted_usage_is_not_flagged(self) -> None:
        # Precision: downgraded / quoted vocabulary must never trip the guard.
        for line in (
            'NOMOS does not describe a hash-only artifact as "signed".',
            "The predicate is unsigned and must not be described as signed.",
            'signatureMode is "none" | "dsse-cosign" | "sigstore-keyless".',
            "An unsigned envelope carries no signature.",
        ):
            self.assertIsNone(
                guard.classify_line(line, signing_present=True),
                f"false positive on: {line}",
            )

    def test_real_repository_tree_is_clean(self) -> None:
        # The committed tree must pass the guard as shipped.
        self.assertEqual(guard.scan(ROOT), [], "shipped tree has an unbacked claim")

    def test_script_runs_as_subprocess(self) -> None:
        # End-to-end: the script is invokable and exits 0 on the real tree.
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)


if __name__ == "__main__":
    unittest.main()
