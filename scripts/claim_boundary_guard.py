#!/usr/bin/env python3
"""CKM-H1-FU — claim-boundary guard for attestation-capability language.

The audit (#518/#519/#537) established the doctrine: NOMOS may use the words
"signed", "Sigstore", "cryptographically signed", or "certified" *as a present
capability claim* only when that capability actually exists in the tree. PR #529
added REAL key-based signing (ECDSA P-256 DSSE) in
``cli/internal/attestation/signing.go`` — so claims of cryptographic/DSSE signing
are now backed by proof. Keyless **Sigstore** (Fulcio/Rekor transparency log) is
still a documented follow-up and is NOT present; asserting it as a live capability
is a false claim.

This guard fails CI when prose in ``README*.md``, ``attestations/**``, or
``docs/**`` makes an affirmative attestation-capability claim that is not backed
by proof:

* claims of **cryptographic / ECDSA / DSSE signing** require the real signing
  capability marker (``signing.go``) to be present;
* claims of **keyless Sigstore / Fulcio / Rekor** signing always fail, because
  that capability is not implemented (only the offline key-based path is);
* "certified" used as a trust claim about NOMOS output requires the marker.

Precision (avoid false positives). The guard deliberately does NOT flag:

* quoted tokens / enum values (e.g. ``"signed"``, ``sigstore-keyless`` as a mode
  literal, ``status: signed``) — code/schema vocabulary, not a prose claim;
* negated or "downgraded" statements ("must not be described as signed", "does
  not describe a hash-only artifact as signed", "unsigned");
* future / aspirational phrasing ("intended", "planned", "follow-up",
  "expansion", "roadmap", "will", "future") on the same line.

It also fails when a claim is **restated** in the same file in different words.
A claim has exactly one normative wording; three paraphrases of one claim leave a
reader unable to tell which one binds. #582 landed the cite-or-abstain bench
paragraph three times in ``docs/public-claim-boundary.md`` and its whole section
three times in ``docs/05-knowledge-base-and-rag.md``: every individual line was
clean, so the line-based checks above stayed green. Prose paragraphs at or above
``MIN_CLAIM_TOKENS`` whose normalised token sets overlap at or above
``DUPLICATE_CLAIM_RATIO`` are reported. Headings, tables, list blocks and fenced
code are excluded — enumerations and command blocks legitimately repeat wording.

Run standalone (exit 0 = clean, 1 = a bare claim was found):

    python3 scripts/claim_boundary_guard.py --root .

It is wired into ``scripts/ckm-non-regression.sh`` and exercised by
``tests/test_claim_boundary_guard.py`` (which includes the adversarial proof that
a bogus "attestations are Sigstore-signed" line turns the guard red).
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

# Proof markers: presence of the real signing capability in the tree. The guard
# treats cryptographic/DSSE signing claims as backed iff a marker file exists and
# actually implements signing primitives.
SIGNING_MARKER = Path("cli/internal/attestation/signing.go")
SIGNING_MARKER_TOKENS = ("ecdsa", "dsse", "SignASN1", "VerifyASN1")

# #637: offline VERIFICATION of a supplied Sigstore bundle is a real, bounded
# capability — the engine side of the process boundary plus the external
# verifier module. A sentence that says NOMOS *verifies supplied bundles* is
# backed iff both markers are present; any sentence that says NOMOS signs,
# issues or publishes with Sigstore keeps failing regardless.
# #645: keyless ISSUANCE is real only against injected, non-production
# endpoints. A sentence that says so is backed iff both issuance markers are
# present AND the sentence names the injected/non-production bound; a sentence
# that claims production, public-good or a Sigstore public instance keeps failing.
SIGSTORE_ISSUE_MARKERS = (
    (Path("cli/internal/attestation/sigstore_issue.go"), ("IssueSigstoreBundle", "SIGSTORE_PRODUCTION_FORBIDDEN", "CheckEndpointPolicy")),
    (Path("tools/sigstore-verifier/issue.go"), ("sign.Bundle(", "PRODUCTION_FORBIDDEN", "nomos.sigstore-issue.response.v1")),
)
_SIGSTORE_ISSUE_BOUNDED = re.compile(
    r"\b(injected|non[- ]production|fixture|localhost|controlled|staging|injecté\w*|hors[- ]production)\b",
    re.IGNORECASE,
)
_SIGSTORE_PRODUCTION_WORDS = re.compile(
    r"\b(production|public[- ]good|sigstore\.dev|sigstage\.dev|public\s+instance|instance\s+publique)\b",
    re.IGNORECASE,
)

SIGSTORE_VERIFY_MARKERS = (
    (Path("cli/internal/attestation/sigstore_external.go"), ("VerifySigstoreBundle", "SIGSTORE_DIGEST_DISAGREEMENT", "no verdict")),
    (Path("tools/sigstore-verifier/main.go"), ("verify.NewVerifier", "WithCertificateIdentity", "nomos.sigstore-verify.response.v1")),
)

# Files in scope: documentation/prose surfaces only. Code and CUE schemas carry
# the field vocabulary ("signed", "sigstore-keyless") legitimately and are out of
# scope by extension.
DOC_SUFFIXES = {".md"}
SCOPE_DIRS = ("attestations", "docs")
ROOT_GLOBS = ("README*.md",)

# The subject the guard cares about: NOMOS's *own* attestations / predicates /
# envelopes / output. Generic mentions of a signing standard (in a comparison
# table or "tools to adopt" list) are landscape, not a self-capability claim.
_NOMOS_SUBJECT = re.compile(
    r"\b(nomos|attestation|attestations|predicate|predicates|envelope|envelopes|"
    r"artifact|artifacts|statement|statements|output|outputs|bundle|bundles|they|"
    r"these|records?)\b",
    re.IGNORECASE,
)

# Capability-claim verbs/adjectives that, applied to attestations/predicates,
# assert a *present* signing or certification capability.
_SIGNED_CLAIM = re.compile(
    r"\b(are|is|now|produces?|emits?|generates?|provides?|ships?|comes?)\b[^.]{0,80}?"
    r"\b(signed|cryptographically\s+signed|digitally\s+signed|certified)\b",
    re.IGNORECASE,
)
# Direct "X-signed / cryptographically signed / Sigstore-signed" adjective forms,
# and "signs ... with a real cryptographic signature" capability sentences.
_SIGNS_CLAIM = re.compile(
    r"\b(sign|signs|signing)\b[^.]{0,80}?\b(cryptographic|signature|ecdsa|dsse|sigstore)\b",
    re.IGNORECASE,
)
# An affirmative "<subject> are/is Sigstore-signed" adjective claim — the precise
# adversarial target. This fires even in a table row, because it asserts the
# subject *is* keyless-signed rather than merely listing Sigstore as a standard.
_SIGSTORE_ADJ_CLAIM = re.compile(
    r"\b(are|is|now)\b[^.|]{0,60}?\bsigstore[- ]?signed\b"
    r"|\bsigstore[- ]?signed\b[^.|]{0,30}?\b(and\s+certified|attestations?|predicates?)\b",
    re.IGNORECASE,
)
# A looser Sigstore-as-live-signing phrasing (prose, not table landscape).
_SIGSTORE_PROSE_CLAIM = re.compile(
    r"\b(sigstore|fulcio|rekor)\b[^.|]{0,40}?\b(sign|signed|signing|keyless|transparency|issue[sd]?|issuing|publish\w*|emit\w*)\b"
    r"|\b(sign|signed|signing|keyless|issue[sd]?|issuing|publish\w*|emit\w*)\b[^.|]{0,40}?\b(sigstore|fulcio|rekor)\b",
    re.IGNORECASE,
)
_CERTIFIED_CLAIM = re.compile(
    r"\b(supply[- ]chain\s+certified|cryptographically\s+certified|are\s+certified|is\s+certified)\b",
    re.IGNORECASE,
)

# VRC-01 (#547): maturity/integration overclaims. The audited case is doc 40
# asserting "éprouvé et intégré dans plusieurs environnements" while the
# recorded evidence is POC-scoped (one private corpus, recorded runs — see
# public-claim-boundary.md). Asserting multi-environment / customer-production
# maturity requires integration records; without them the claim must be
# downgraded, quoted, or negated.
_MATURITY_CLAIM = re.compile(
    r"\b(éprouvé\w*|eprouv\w*|intégré\w*|integr(?:e|é)s?\b|proven|deployed|"
    r"en\s+production|in\s+production)\b"
    r"[^.|]{0,80}?"
    r"\b(multi[- ]environ\w*|plusieurs\s+(?:environnements|projets|clients)|"
    r"several\s+environments|multiple\s+(?:projects|customers|environments)|"
    r"chez\s+\S+\s+clients?)\b",
    re.IGNORECASE,
)

# Headings whose section is forward-looking; claims under them are deferred, not
# present-tense. Matched against the nearest preceding markdown heading.
_DEFERRED_HEADING = re.compile(
    r"\b(intended|planned|future|roadmap|follow[- ]up|expansion|next\s+steps|"
    r"aspiration\w*|to\s+adopt|state[- ]of[- ]the[- ]art|positioning|"
    r"capitalization|improvement\s+plan|landscape|comparison|prior\s+art)\b",
    re.IGNORECASE,
)

# Context that DOWNGRADES / NEGATES / DEFERS a line so it is not an affirmative
# present-tense capability claim.
_SAFE_CONTEXT = re.compile(
    r"\b(not|never|cannot|can't|must\s+not|does\s+not|do\s+not|without|unsigned|"
    r"un[- ]signed|no\s+signature|placeholder|fake|bogus|downgrad\w*|"
    r"intended|intend\w*|plan\w*|planned|future|follow[- ]up|roadmap|expansion|"
    r"aspiration\w*|plus tard|will\s+\w+|would\s+\w+|todo|tbd|"
    r"hash[- ]only|field[- ]presence|"
    # French negations / claim-boundary downgrades (VRC-01 #547)
    r"pas|aucune?|jamais|borné\w*|non\s+soutenu\w*|poc[- ]scoped|forgée?)\b",
    re.IGNORECASE,
)

# A line where the only occurrences of the trigger word are inside quotes or
# inline code is schema/vocabulary, not prose. We strip those spans before
# re-checking; if nothing claim-like remains, the line is clean. French
# guillemets are stripped too (VRC-01 #547): quoting an overclaim in order to
# refute or track it is claim-boundary work, not a claim.
_QUOTED_SPAN = re.compile(r"`[^`]*`|\"[^\"]*\"|'[^']*'|«[^»]*»")


def sigstore_verification_present(root: Path) -> bool:
    """True iff the offline Sigstore VERIFICATION capability (#637) is in the tree."""
    for marker, tokens in SIGSTORE_VERIFY_MARKERS:
        path = root / marker
        if not path.is_file():
            return False
        try:
            text = path.read_text(encoding="utf-8")
        except OSError:
            return False
        if not all(token in text for token in tokens):
            return False
    return True


# Bounded verification phrasing: "verif… supplied/provided … bundle(s)". It is
# honest only when nothing in the same sentence claims issuance.
_SIGSTORE_VERIFY_BOUNDED = re.compile(
    r"\bverif\w*\b[^.|]{0,80}?\b(supplied|provided|given|fourni\w*)\b[^.|]{0,60}?\bbundles?\b"
    r"|\bbundles?\b[^.|]{0,40}?\b(supplied|provided|given|fourni\w*)\b[^.|]{0,60}?\bverif\w*\b",
    re.IGNORECASE,
)
_SIGSTORE_ISSUANCE_WORDS = re.compile(
    r"\b(signs?|signed|signing|signe\w*|issue[sd]?|issuing|émet\w*|emit\w*|publish\w*|publie\w*|keyless|writes?\s+to\s+rekor)\b",
    re.IGNORECASE,
)


def sigstore_issuance_present(root: Path) -> bool:
    """True iff the injected-environment keyless issuance (#645) is in the tree."""
    for marker, tokens in SIGSTORE_ISSUE_MARKERS:
        path = root / marker
        if not path.is_file():
            return False
        try:
            text = path.read_text(encoding="utf-8")
        except OSError:
            return False
        if not all(token in text for token in tokens):
            return False
    return True


_NEGATED_PRODUCTION = re.compile(
    r"\b(no|never|not|non|without|refus\w*|forbid\w*|interdit\w*|jamais|aucune?|pas\s+(?:de|en|d'))\W{0,3}(?:\w+\W{0,3}){0,3}?"
    r"(production|public[- ]good|sigstore\.dev|sigstage\.dev|public\s+instance|instance\s+publique)\b",
    re.IGNORECASE,
)


def _strip_negated_production(text: str) -> str:
    """Remove 'no production' / 'production is refused' spans so a negation is not read as a claim."""
    text = _NEGATED_PRODUCTION.sub(" ", text)
    return re.sub(r"\b(production|public[- ]good|sigstore\.dev|sigstage\.dev)\b[^.|]{0,40}?\b(refused|forbidden|never|not|interdit\w*|refusé\w*)\b", " ", text, flags=re.IGNORECASE)


def signing_capability_present(root: Path) -> bool:
    """True iff the real key-based signing implementation is present in the tree."""
    marker = root / SIGNING_MARKER
    if not marker.is_file():
        return False
    try:
        text = marker.read_text(encoding="utf-8")
    except OSError:
        return False
    return all(token in text for token in SIGNING_MARKER_TOKENS)


def iter_scoped_files(root: Path) -> list[Path]:
    files: list[Path] = []
    for pattern in ROOT_GLOBS:
        files.extend(sorted(root.glob(pattern)))
    for scope in SCOPE_DIRS:
        base = root / scope
        if not base.is_dir():
            continue
        for path in sorted(base.rglob("*")):
            if path.is_file() and path.suffix.lower() in DOC_SUFFIXES:
                files.append(path)
    # de-dup while preserving order
    seen: set[Path] = set()
    unique: list[Path] = []
    for path in files:
        resolved = path.resolve()
        if resolved in seen:
            continue
        seen.add(resolved)
        unique.append(path)
    return unique


def _strip_quoted(line: str) -> str:
    return _QUOTED_SPAN.sub(" ", line)


def classify_line(
    line: str,
    signing_present: bool,
    section_deferred: bool = False,
    verify_present: bool = False,
    issue_present: bool = False,
) -> str | None:
    """Return a violation reason for a line, or None if the line is clean.

    A line violates when it makes an affirmative present-tense capability claim
    about NOMOS's own attestations that is not backed by proof.

    ``section_deferred`` is True when the nearest preceding heading marks the
    section as forward-looking / landscape (Intended Expansion, roadmap,
    state-of-the-art positioning, comparison) — claims there are not present-tense
    self-claims.
    """
    # Vocabulary/enum and quoted-token usage is not a prose claim.
    bare = _strip_quoted(line)
    if not bare.strip():
        return None
    # Downgraded / negated / deferred lines are honest claim-boundary language.
    if _SAFE_CONTEXT.search(line):
        return None

    is_table_row = bare.lstrip().startswith("|")

    # #637 bounded marker: "verifies supplied bundles" is backed by the real
    # verification capability — but only when the same sentence claims no
    # issuance. "verifies supplied bundles and signs them keyless" still fails.
    if verify_present and _SIGSTORE_VERIFY_BOUNDED.search(bare) and not _SIGSTORE_ISSUANCE_WORDS.search(bare):
        return None

    # #645 bounded marker: issuance wording is honest only when the sentence
    # itself names the injected/non-production bound and says nothing about
    # production or a public instance. "signs keyless with Sigstore" alone,
    # or "...in production", keeps failing.
    if (
        issue_present
        and _SIGSTORE_ISSUANCE_WORDS.search(bare)
        and _SIGSTORE_ISSUE_BOUNDED.search(bare)
        and not _SIGSTORE_PRODUCTION_WORDS.search(_strip_negated_production(bare))
    ):
        return None

    # Affirmative "<subject> are Sigstore-signed" adjective claim — the precise
    # adversarial target. Fires anywhere (incl. a forged table row), because it
    # asserts the subject *is* keyless-signed, not merely lists Sigstore.
    if _SIGSTORE_ADJ_CLAIM.search(bare):
        return (
            "asserts attestations are Sigstore-signed as a present capability; "
            "only offline key-based ECDSA P-256 DSSE is implemented (Sigstore is a documented follow-up)"
        )

    # Looser Sigstore-as-live-signing prose. Skip when the section is deferred or
    # the line is a comparison-table row enumerating standards (landscape).
    if _SIGSTORE_PROSE_CLAIM.search(bare) and not section_deferred and not is_table_row:
        if _NOMOS_SUBJECT.search(bare):
            return (
                "asserts keyless Sigstore/Fulcio/Rekor signing of NOMOS attestations as a "
                "present capability; only offline key-based ECDSA P-256 DSSE is implemented"
            )

    crypto_claim = _SIGNED_CLAIM.search(bare) or _SIGNS_CLAIM.search(bare)
    certified_claim = _CERTIFIED_CLAIM.search(bare)
    if (crypto_claim or certified_claim) and _NOMOS_SUBJECT.search(bare) and not signing_present:
        return (
            "claims NOMOS attestations are signed/certified but the signing capability "
            f"marker {SIGNING_MARKER.as_posix()} is absent"
        )

    # Maturity overclaims (VRC-01 #547): multi-environment / customer-production
    # integration claims require recorded integration evidence; today the
    # recorded evidence is POC-scoped, so an affirmative claim is unbacked.
    if _MATURITY_CLAIM.search(bare) and not section_deferred and not is_table_row:
        return (
            "asserts multi-environment/production integration maturity without recorded "
            "integration evidence (public-claim-boundary: recorded evidence is POC-scoped)"
        )
    return None


# A claim stated twice in slightly different words has no normative form: a
# reader cannot tell which wording binds. Paragraphs at or above this many
# tokens whose normalised token sets overlap at or above the ratio below are
# reported as the same claim restated. #582 landed the bench paragraph three
# times (three paraphrases of one claim) and the line-based checks above could
# not see it, because every individual line was clean.
MIN_CLAIM_TOKENS = 40
DUPLICATE_CLAIM_RATIO = 0.90
_CLAIM_TOKEN = re.compile(r"[a-z0-9]+")


def _claim_tokens(paragraph: str) -> list[str]:
    """Normalise a paragraph to comparable claim tokens.

    Backticks, punctuation and dash style are dropped, so two paraphrases that
    differ only in typography normalise to the same tokens.
    """
    return _CLAIM_TOKEN.findall(paragraph.lower())


def _claim_paragraphs(lines: list[str]) -> list[tuple[int, str]]:
    """Yield ``(first_lineno, text)`` for prose paragraphs, skipping code fences.

    Headings, tables and list blocks are excluded: enumerations legitimately
    repeat wording, and only free prose carries a claim.
    """
    paragraphs: list[tuple[int, str]] = []
    buffer: list[str] = []
    start = 0
    in_fence = False

    def flush() -> None:
        if not buffer:
            return
        text = " ".join(buffer)
        stripped = buffer[0].lstrip()
        structural = stripped.startswith(("#", "|", "-", "*", ">", "+")) or (
            stripped[:2].isdigit() and stripped[1:2] == "."
        )
        if not structural:
            paragraphs.append((start, text))

    for lineno, line in enumerate(lines, start=1):
        if line.lstrip().startswith("```"):
            flush()
            buffer = []
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        if line.strip():
            if not buffer:
                start = lineno
            buffer.append(line.strip())
            continue
        flush()
        buffer = []
    flush()
    return paragraphs


def find_duplicate_claims(root: Path) -> list[tuple[Path, int, str, str]]:
    """Report paragraphs that restate a claim already made in the same file."""
    violations: list[tuple[Path, int, str, str]] = []
    for path in iter_scoped_files(root):
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError as exc:
            # An in-scope file the guard cannot read is a finding, not a skip
            # (docs/43 principle 8): an unscanned file would be a false green.
            violations.append((path, 0, f"file could not be read and was not scanned ({exc})", ""))
            continue
        seen: list[tuple[int, set[str]]] = []
        for lineno, text in _claim_paragraphs(lines):
            tokens = _claim_tokens(text)
            if len(tokens) < MIN_CLAIM_TOKENS:
                continue
            current = set(tokens)
            for earlier_lineno, earlier in seen:
                union = current | earlier
                if not union:
                    continue
                overlap = len(current & earlier) / len(union)
                if overlap >= DUPLICATE_CLAIM_RATIO:
                    violations.append(
                        (
                            path,
                            lineno,
                            text[:160],
                            (
                                f"restates the claim already made at line {earlier_lineno} "
                                f"({overlap:.0%} token overlap); a claim has exactly one "
                                "normative wording"
                            ),
                        )
                    )
                    break
            else:
                seen.append((lineno, current))
    return violations


def scan(root: Path) -> list[tuple[Path, int, str, str]]:
    signing_present = signing_capability_present(root)
    verify_present = sigstore_verification_present(root)
    issue_present = sigstore_issuance_present(root)
    violations: list[tuple[Path, int, str, str]] = []
    for path in iter_scoped_files(root):
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError as exc:
            # An in-scope file the guard cannot read is a finding, not a skip
            # (docs/43 principle 8): an unscanned file would be a false green.
            violations.append((path, 0, f"file could not be read and was not scanned ({exc})", ""))
            continue
        section_deferred = False
        for lineno, line in enumerate(lines, start=1):
            if line.lstrip().startswith("#"):
                section_deferred = bool(_DEFERRED_HEADING.search(line))
            reason = classify_line(line, signing_present, section_deferred, verify_present, issue_present)
            if reason is not None:
                violations.append((path, lineno, line.strip(), reason))
    return violations


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root to scan.")
    parser.add_argument(
        "--quiet",
        action="store_true",
        help="Only print the summary line, not each violation.",
    )
    args = parser.parse_args(argv)

    root = Path(args.root).resolve()
    signing_present = signing_capability_present(root)
    violations = scan(root)
    duplicates = find_duplicate_claims(root)
    violations.extend(duplicates)
    violations.sort(key=lambda item: (item[0].as_posix(), item[1]))

    if violations:
        if not args.quiet:
            for path, lineno, snippet, reason in violations:
                rel = path.resolve().relative_to(root).as_posix()
                print(f"{rel}:{lineno}: claim-boundary violation: {reason}", file=sys.stderr)
                print(f"    > {snippet}", file=sys.stderr)
        print(
            f"claim-boundary guard: FAIL — {len(violations) - len(duplicates)} unbacked "
            f"attestation claim(s), {len(duplicates)} restated claim(s) "
            f"(signing capability present={signing_present})",
            file=sys.stderr,
        )
        return 1

    print(
        f"claim-boundary guard: OK — no unbacked attestation-capability claims, "
        f"no restated claims (signing capability present={signing_present})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
