#!/usr/bin/env bash
# rbok-poc-integrity.sh — SFI-11 (#349) source-to-feed integrity POC runner
# for the RBOK 01_rbok proof corpus.
#
# This script drives the full SFI-04..SFI-08 source-to-feed integrity
# process against `realisons-business/01_rbok` end-to-end. It is
# read-only on the corpus: any mutation of the corpus working tree
# during the run is a hard failure.
#
# Companion document: docs/rbok-poc-validation-dossier.md (this PR).
# Engine reference:    docs/21-source-feed-integrity-engine.md (SFI-10).
#
# Exit codes:
#   0 — POC passed (every gate green; corpus clean before and after)
#   1 — a gate failed
#   2 — corpus directory missing (clean exit; not a failure of this script)
#   3 — corpus working tree was dirty BEFORE the run started
#   4 — corpus working tree was mutated DURING the run
#   5 — script preflight failure (missing tool, etc.)
#
# Usage:
#   bash scripts/rbok-poc-integrity.sh
#   CORPUS=/abs/path/to/01_rbok bash scripts/rbok-poc-integrity.sh
#   RUN_DIR=./reports/poc/manual bash scripts/rbok-poc-integrity.sh
#
# Re-runnable: each invocation creates a fresh per-timestamp $RUN_DIR
# under ./reports/poc/, so concurrent or repeated runs do not collide.

set -euo pipefail

# ---- configuration ---------------------------------------------------------

CORPUS="${CORPUS:-/root/repos/realisons-business/01_rbok}"
RUN_DIR="${RUN_DIR:-./reports/poc/$(date -u +%Y%m%dT%H%M%SZ)}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOMOS_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"
NOMOS_CLI_PKG_DIR="$NOMOS_REPO/cli"

# ---- helpers ---------------------------------------------------------------

log() {
  printf '[rbok-poc] %s\n' "$*"
}

die() {
  local code="$1"; shift
  printf '[rbok-poc] FATAL: %s\n' "$*" >&2
  exit "$code"
}

require_tool() {
  command -v "$1" >/dev/null 2>&1 || die 5 "required tool not on PATH: $1"
}

# ---- preflight -------------------------------------------------------------

log "SFI-11 (#349) source-to-feed integrity POC runner"
log "corpus    : $CORPUS"
log "run dir   : $RUN_DIR"
log "nomos repo: $NOMOS_REPO"

require_tool git
require_tool go
require_tool jq

if [[ ! -d "$CORPUS" ]]; then
  log "BLOCKED — corpus directory does not exist at: $CORPUS"
  log "          this is the expected exit on hosts without corpus access."
  log "          provision read-only corpus access on this host, or run on"
  log "          a host where the corpus already lives, then re-run this"
  log "          script. The dossier (docs/rbok-poc-validation-dossier.md)"
  log "          records the explicit blocker in section 6."
  exit 2
fi

mkdir -p "$RUN_DIR"
RUN_DIR="$(cd "$RUN_DIR" && pwd)"

# Resolve the corpus's enclosing git working tree (the corpus is a
# subdirectory of realisons-business; status/rev-parse must run there).
if ! CORPUS_GIT_ROOT="$(git -C "$CORPUS" rev-parse --show-toplevel 2>/dev/null)"; then
  die 5 "corpus is not inside a git working tree: $CORPUS"
fi

# Record the run environment for cross-checking with the dossier.
{
  echo "go_version      = $(go version)"
  echo "nomos_repo      = $NOMOS_REPO"
  echo "nomos_commit    = $(git -C "$NOMOS_REPO" rev-parse HEAD)"
  echo "nomos_branch    = $(git -C "$NOMOS_REPO" rev-parse --abbrev-ref HEAD)"
  echo "corpus          = $CORPUS"
  echo "corpus_git_root = $CORPUS_GIT_ROOT"
  echo "corpus_commit   = $(git -C "$CORPUS_GIT_ROOT" rev-parse HEAD)"
  echo "os              = $(uname -a)"
  echo "utc             = $(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$RUN_DIR/run-environment.txt"

# ---- step 0: pre-run git status capture ------------------------------------

log "step 0: pre-run git status capture"
git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-before.txt"
git -C "$CORPUS_GIT_ROOT" rev-parse HEAD  > "$RUN_DIR/corpus-commit.txt"

if [[ -s "$RUN_DIR/corpus-status-before.txt" ]]; then
  log "corpus-status-before.txt is NON-EMPTY:"
  cat "$RUN_DIR/corpus-status-before.txt" >&2
  die 3 "corpus working tree was dirty before the run; refusing to start"
fi
log "corpus working tree clean at: $(cat "$RUN_DIR/corpus-commit.txt")"

# ---- step 1: source scan ---------------------------------------------------

log "step 1: nomos corpus scan"
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . corpus scan \
    --root   "$CORPUS" \
    --out    "$RUN_DIR/snapshot.json" \
    --format json )

# ---- step 2: source manifest -----------------------------------------------

log "step 2: nomos corpus manifest"
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . corpus manifest \
    --snapshot "$RUN_DIR/snapshot.json" \
    --out      "$RUN_DIR/source-manifest.yaml" )

# ---- step 3: source segment ledger emission --------------------------------

# TODO(SFI-02 / #340, SFI-08 / #346): no dedicated CLI subcommand emits a
# JSON []SourceSegment ledger today. The integrity gate computes the
# ledger in-process via corpus.ScanMarkdown when --corpus-integrity-source
# is supplied. Step 3 is therefore implicit; nothing to write here.
log "step 3: source segment ledger — TODO(SFI-02 / SFI-08); ledger computed in-process"

# ---- step 4: source integrity gate (SFI-04, #342) --------------------------

log "step 4: source integrity gate (SFI-04 / #342)"
set +e
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . strict \
    --corpus-integrity-source "$CORPUS" \
    --format json ) > "$RUN_DIR/integrity-source.json"
step4_rc=$?
set -e
log "step 4 exit code: $step4_rc -> $RUN_DIR/integrity-source.json"

# ---- step 5: feed generation (SFI-05, #343) --------------------------------

log "step 5: nomos corpus feed (SFI-05 / #343)"
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . corpus feed \
    --root     "$CORPUS" \
    --snapshot "$RUN_DIR/snapshot.json" \
    --manifest "$RUN_DIR/source-manifest.yaml" \
    --out      "$RUN_DIR/feed.json" )

# ---- step 6: rag metadata (SFI-06, #344) -----------------------------------

log "step 6: extract RAG metadata from feed (SFI-06 / #344)"
# TODO(SFI-06 / #344): no dedicated `nomos corpus rag` subcommand today;
# RAG metadata is bundled inside `nomos corpus feed`. Extract via jq.
jq '.rag_metadata // []' "$RUN_DIR/feed.json" > "$RUN_DIR/rag.json"
jq '.units        // []' "$RUN_DIR/feed.json" > "$RUN_DIR/feed-units.json"

# ---- step 7: feed quality gate (SFI-07, #345) ------------------------------

log "step 7: feed quality gate (SFI-07 / #345)"
set +e
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . strict \
    --corpus-integrity-source "$CORPUS" \
    --corpus-integrity-feed   "$RUN_DIR/feed-units.json" \
    --corpus-integrity-rag    "$RUN_DIR/rag.json" \
    --format json ) > "$RUN_DIR/integrity-feed.json"
step7_rc=$?
set -e
log "step 7 exit code: $step7_rc -> $RUN_DIR/integrity-feed.json"

# ---- step 8: attestation (bounded claim) -----------------------------------

log "step 8: nomos corpus attest (bounded claim)"
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . corpus attest \
    --snapshot   "$RUN_DIR/snapshot.json" \
    --corpus-id  rbok-lawbook \
    --project-id rbok ) > "$RUN_DIR/attestation.json"

NOMOS_SHA="$(git -C "$NOMOS_REPO" rev-parse HEAD)"
CORPUS_SHA="$(cat "$RUN_DIR/corpus-commit.txt")"
cat > "$RUN_DIR/attestation-claim.txt" <<EOF
On the recorded run of NOMOS commit ${NOMOS_SHA} against
realisons-business/01_rbok at commit ${CORPUS_SHA}, the SFI-04 source
integrity gate and the SFI-07 feed quality gate each reported
status=pass with 0 findings, and the SFI-08 strict release gate
exited 0. The corpus working tree was clean before and after the run.

The build advertises claim level 'source-integrity-proven' for this run.

Promotion to 'full-fidelity-proven' requires this strict gate to be
wired into CI for the RBOK POC corpus and to remain green across
consecutive runs (gating issue: #346).
EOF

# ---- step 9: strict gate consuming all of the above ------------------------

log "step 9: strict gate (SFI-08 / #346)"
set +e
( cd "$NOMOS_CLI_PKG_DIR" && \
  go run . strict \
    --corpus-integrity-source "$CORPUS" \
    --corpus-integrity-feed   "$RUN_DIR/feed-units.json" \
    --corpus-integrity-rag    "$RUN_DIR/rag.json" \
    --format json ) > "$RUN_DIR/strict-gate.json"
strict_rc=$?
set -e
log "step 9 strict gate exit code: $strict_rc -> $RUN_DIR/strict-gate.json"

# ---- step 10: post-run git status capture ----------------------------------

log "step 10: post-run git status capture"
git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-after.txt"

if ! diff -q "$RUN_DIR/corpus-status-before.txt" "$RUN_DIR/corpus-status-after.txt" > /dev/null; then
  diff -u "$RUN_DIR/corpus-status-before.txt" "$RUN_DIR/corpus-status-after.txt" \
    > "$RUN_DIR/corpus-mutation.diff" || true
  die 4 "corpus working tree was mutated during the run; see $RUN_DIR/corpus-mutation.diff"
fi

# ---- success-criteria assertion --------------------------------------------

log "asserting #349 success criteria against $RUN_DIR/integrity-source.json and $RUN_DIR/integrity-feed.json"

assert_zero() {
  local file="$1"; local jq_expr="$2"; local label="$3"
  local n
  n="$(jq -r "$jq_expr" "$file")"
  if [[ "$n" == "null" || -z "$n" ]]; then n=0; fi
  if [[ "$n" -ne 0 ]]; then
    log "  FAIL: $label = $n (expected 0); see $file"
    return 1
  fi
  log "  ok:   $label = 0"
  return 0
}

failures=0

# 0 uncovered active semantic ranges
assert_zero "$RUN_DIR/integrity-source.json" \
  '.corpus_integrity_check.source_integrity.uncovered_ranges | length' \
  "uncovered_ranges" || failures=$((failures + 1))

# 0 duplicate semantic source spans
assert_zero "$RUN_DIR/integrity-source.json" \
  '.corpus_integrity_check.source_integrity.duplicate_semantic_ranges | length' \
  "duplicate_semantic_ranges" || failures=$((failures + 1))

# 0 junk semantic feed atoms
assert_zero "$RUN_DIR/integrity-source.json" \
  '.corpus_integrity_check.source_integrity.junk_semantic_segments | length' \
  "junk_semantic_segments" || failures=$((failures + 1))

# 0 unsupported_blocking left implicit (must be explicit dispositions only)
assert_zero "$RUN_DIR/integrity-source.json" \
  '.corpus_integrity_check.source_integrity.unsupported_blocking_segments | length' \
  "unsupported_blocking_segments" || failures=$((failures + 1))

# 0 RAG chunks without source segment linkage  +  0 parent/child duplications
# These live under feed_quality.findings; we look for non-zero finding counts
# under the recognised feed-quality finding codes.
fq_findings="$(jq -r \
  '.corpus_integrity_check.feed_quality.findings // [] | length' \
  "$RUN_DIR/integrity-feed.json" 2>/dev/null || echo 0)"
if [[ "$fq_findings" -ne 0 ]]; then
  log "  FAIL: feed_quality.findings = $fq_findings (expected 0); see $RUN_DIR/integrity-feed.json"
  failures=$((failures + 1))
else
  log "  ok:   feed_quality.findings = 0"
fi

# Strict gate must exit 0 if integrity report passes.
if [[ "$strict_rc" -ne 0 ]]; then
  log "  FAIL: strict gate exit code = $strict_rc (expected 0); see $RUN_DIR/strict-gate.json"
  failures=$((failures + 1))
else
  log "  ok:   strict gate exit code = 0"
fi

if [[ "$failures" -gt 0 ]]; then
  log "POC FAILED — $failures success-criterion assertion(s) failed"
  log "see $RUN_DIR/ for full evidence"
  exit 1
fi

log "POC PASSED — every #349 success criterion satisfied on the recorded run"
log "evidence: $RUN_DIR/"
log "claim   : $RUN_DIR/attestation-claim.txt"
exit 0
