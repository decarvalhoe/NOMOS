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


# Structural fixtures for the precision test. Each is comfortably above
# MIN_CLAIM_TOKENS so that, if the structural skip were removed, the repeated
# block would be compared and reported — which is what makes the skip
# load-bearing rather than decorative.
_LONG_BULLET = (
    "- the corpus integrity report for this build is present and passes on "
    "coverage, duplicate spans, junk content, feed linkage and RAG linkage, "
    "and the strict release gate consumes it, which is recorded per dossier "
    "and never generalised into a platform wide statement about arbitrary "
    "customer corpora or arbitrary document formats\n"
    "- second entry of the same enumeration, kept short"
)

_LONG_TABLE = (
    "| level | meaning | gating |\n"
    "| --- | --- | --- |\n"
    "| artifact-generated | NOMOS produced the artifact without crashing and "
    "with the documented schema | existing validate and canonical check gates, "
    "active today on the recorded profile feeds and nowhere else |\n"
    "| source-traced | generated nodes carry source spans that resolve to a "
    "recorded source manifest entry | source span emission and manifest hash "
    "check, active today on the recorded profile feeds |"
)

_LONG_FENCE = (
    "```\n"
    "python3 scripts/claim_boundary_guard.py --root . --quiet\n"
    "python3 scripts/cite_or_abstain_bench.py --root . --verify-references\n"
    "python3 scripts/regulated_evidence_pack.py --output evidence-pack.json\n"
    "nomos answer bench --corpus corpus.yaml --thresholds thresholds.yaml\n"
    "nomos corpus attest --corpus-body-ledger --profile rbok-lawbook\n"
    "```"
)

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

    def test_bounded_sigstore_verification_marker(self) -> None:
        # #637: "verifies supplied bundles" is backed by the real verification
        # capability; any issuance wording in the same sentence still fails, and
        # without the capability markers the bounded phrasing is not backed.
        verify_ok = "NOMOS verifies a supplied Sigstore bundle offline, including its transparency-log inclusion proof."
        self.assertIsNone(guard.classify_line(verify_ok, signing_present=True, verify_present=True))
        self.assertIsNotNone(
            guard.classify_line(verify_ok, signing_present=True, verify_present=False),
            "without the verification markers the bounded phrasing is an unbacked Sigstore claim",
        )
        for issuance in (
            "NOMOS verifies supplied Sigstore bundles and signs its attestations keyless with Fulcio.",
            "NOMOS attestations are Sigstore-signed; it also verifies supplied bundles.",
            "NOMOS issues Sigstore bundles for its predicates and verifies the supplied ones.",
            "NOMOS publishes its attestations to Rekor and verifies supplied bundles offline.",
        ):
            self.assertIsNotNone(
                guard.classify_line(issuance, signing_present=True, verify_present=True),
                f"issuance wording must keep failing: {issuance}",
            )
        # The real tree carries both markers.
        self.assertTrue(guard.sigstore_verification_present(ROOT))

    def test_forged_maturity_claim_turns_guard_red(self) -> None:
        # VRC-01 (#547) adversarial proof: the doc-40 class of overclaim —
        # asserting multi-environment / customer-production integration without
        # recorded evidence — must turn the guard red.
        for forged in (
            "NOMOS est éprouvé et intégré dans plusieurs environnements.",
            "NOMOS is proven and deployed across multiple projects.",
            "Le moteur est en production chez plusieurs clients.",
        ):
            bogus = f"\n## Maturité\n\n{forged}\n"
            with tempfile.TemporaryDirectory() as tmp:
                root = _make_tree(Path(tmp), with_signing=True)
                docs = root / "docs" / "maturity.md"
                docs.write_text("# Maturité\n" + bogus, encoding="utf-8")
                violations = guard.scan(root)
                self.assertTrue(violations, f"forged maturity claim must be flagged: {forged}")
                self.assertEqual(guard.main(["--root", str(root)]), 1)

    def test_quoted_or_negated_maturity_language_is_not_flagged(self) -> None:
        # Claim-boundary work that QUOTES or NEGATES the overclaim is clean.
        for line in (
            "doc 40 affirmait « éprouvé et intégré dans plusieurs environnements », non soutenu par un record.",
            "Il n'y a pas d'intégration multi-environnements prouvée par un record.",
            "La phrase forgée « NOMOS est en production chez N clients » doit rendre le guard rouge.",
        ):
            self.assertIsNone(
                guard.classify_line(line, signing_present=True),
                f"false positive on: {line}",
            )

    def test_real_repository_tree_is_clean(self) -> None:
        # The committed tree must pass the guard as shipped.
        self.assertEqual(guard.scan(ROOT), [], "shipped tree has an unbacked claim")

    def test_restated_claim_turns_guard_red(self) -> None:
        # ADVERSARIAL (#582 regression): the same claim landed three times in
        # docs/public-claim-boundary.md as three paraphrases. Every individual
        # line was clean, so the line-based checks stayed green. Re-stating a
        # claim in different words must be caught: a claim has one normative
        # wording, and a reader cannot tell which of three variants binds.
        claim = (
            "The cite-or-abstain gate is measured by a public bench over a labelled "
            "corpus built on the in-repo public reference basis documents, and it "
            "reports the two error directions separately, false cite rate and must "
            "cite recall, so that over abstention is visible as its own defect "
            "rather than hidden inside a single aggregate accuracy number."
        )
        paraphrase = (
            "The cite-or-abstain gate is measured by a public bench over a labelled "
            "corpus built on the in-repo public reference-basis documents; it "
            "reports the two error directions separately — false cite rate and must "
            "cite recall — so that over-abstention is visible as its own defect, "
            "rather than hidden inside a single aggregate accuracy number."
        )
        with tempfile.TemporaryDirectory() as tmpdir:
            root = _make_tree(Path(tmpdir), with_signing=True)
            doc = root / "docs" / "public-claim-boundary.md"
            doc.write_text(
                f"# Claim Boundary\n\n{claim}\n\n{paraphrase}\n",
                encoding="utf-8",
            )

            duplicates = guard.find_duplicate_claims(root)
            self.assertEqual(len(duplicates), 1, duplicates)
            self.assertIn("restates the claim already made", duplicates[0][3])

            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--root", str(root)],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 1, result.stdout)
            self.assertIn("restated claim", result.stderr)

            # Dropping the paraphrase makes the guard green again.
            doc.write_text(f"# Claim Boundary\n\n{claim}\n", encoding="utf-8")
            self.assertEqual(guard.find_duplicate_claims(root), [])

    def test_distinct_claims_and_structure_are_not_flagged(self) -> None:
        # Precision: two genuinely different claims of similar length, a repeated
        # table row, a repeated list item, and a repeated fenced code block are
        # all legitimate and must not read as a restated claim.
        with tempfile.TemporaryDirectory() as tmpdir:
            root = _make_tree(Path(tmpdir), with_signing=True)
            (root / "docs" / "two-claims.md").write_text(
                "# Two Claims\n\n"
                "The export gate proves that every emitted chunk carries a chunk "
                "identifier, a source identifier, a source hash and a body, and it "
                "refuses any record missing one of them, which is a contract claim "
                "about the export shape and not about retrieval quality at all.\n\n"
                "The fidelity gate proves that a recorded strict run reported full "
                "fidelity for one specific corpus and configuration, which is a run "
                "scoped statement about that dossier and never a platform wide "
                "source to feed proof across arbitrary customer corpora.\n\n"
                # Two separate list blocks that enumerate the same long entry:
                # legitimate in a checklist repeated per section, and long enough
                # to clear the token floor, so only the structural skip keeps it
                # out of the comparison.
                f"{_LONG_BULLET}\n\n"
                "Some prose between the two enumerations keeps them apart.\n\n"
                f"{_LONG_BULLET}\n\n"
                # Same for two identical table blocks.
                f"{_LONG_TABLE}\n\n"
                "More prose between the two tables keeps them apart.\n\n"
                f"{_LONG_TABLE}\n\n"
                # And for two identical fenced code blocks, which the fence
                # handling must drop before any comparison happens.
                f"{_LONG_FENCE}\n\n"
                f"{_LONG_FENCE}\n",
                encoding="utf-8",
            )
            self.assertEqual(guard.find_duplicate_claims(root), [])

    def test_real_repository_tree_has_no_restated_claims(self) -> None:
        # The shipped tree states each claim exactly once.
        self.assertEqual(
            guard.find_duplicate_claims(ROOT),
            [],
            "shipped tree restates a claim in different words",
        )

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
