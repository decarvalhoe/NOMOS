#!/usr/bin/env bash
# #612 — Recursio → NOMOS offline end-to-end fixture runner.
#
# Drives the whole web-source chain on the synthetic site + export under
# tests/fixtures/recursio-e2e/ with no network and no crawler:
#
#   site/*.html ──(scripts/recursio_export_fixture.py, deterministic)──▶ export/
#   export/{sources.jsonl, snapshot.json}  ─▶ snapshot verify ─▶ snapshot import
#   ─▶ scan ─▶ feed ─▶ body-ledger ─▶ attest --external-snapshot ─▶ strict gate
#
# What this proves, and only this:
#   • the committed export IS a fresh normalisation of the committed site
#     (byte for byte) — so HTML → normalised text → Markdown is reproducible;
#   • the sealed snapshot verifies, imports, and every pipeline stage exits 0
#     on the imported manifest, offline;
#   • the attestation carries the web-source type and the snapshot coverage
#     (snapshot id, root, counts, web_sources) — asserted, not assumed;
#   • the fixture is never mutated (before/after path+size snapshot).
# It does NOT prove anything about a real site: robots and licence decisions
# in the fixture are `allowed` by construction, and Recursio itself is absent.
#
# The corpus handed to `nomos` is a throwaway git checkout under $RUN_DIR
# holding a COPY of export/captures — never the live repository (corpus
# commands refuse a dirty working tree, and the corpus must never be the repo).
#
# Exit codes (same convention as scripts/ngw-e2e-fixture.sh):
#   0 — chain ran end-to-end and every assertion held
#   1 — a stage failed
#   4 — fixture drift or mutation (export stale vs site, or fixture changed during run)
#   5 — preflight failure (missing fixture, missing tool)

set -euo pipefail

FIXTURE_DIR="${FIXTURE_DIR:-tests/fixtures/recursio-e2e}"
# FIXTURE_ROOT lets a test point the runner at a COPY of the repository's
# fixture tree (to mutate it) while the CLI is still built from this checkout.
FIXTURE_ROOT="${FIXTURE_ROOT:-}"
RUN_DIR="${RUN_DIR:-./reports/recursio-e2e/$(date -u +%Y%m%dT%H%M%SZ)}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOMOS_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"
FIXTURE_ROOT="${FIXTURE_ROOT:-$NOMOS_REPO}"
FIXTURE="$FIXTURE_ROOT/$FIXTURE_DIR"
EXPORT="$FIXTURE/export"

log() { printf '[recursio-e2e] %s\n' "$*"; }
die() { local code="$1"; shift; printf '[recursio-e2e] FATAL: %s\n' "$*" >&2; exit "$code"; }
require_tool() { command -v "$1" >/dev/null 2>&1 || die 5 "required tool not on PATH: $1"; }

require_tool go
require_tool python3
require_tool jq
require_tool git

[[ -d "$FIXTURE/site" ]] || die 5 "fixture site not found at: $FIXTURE/site"
[[ -f "$EXPORT/sources.jsonl" ]] || die 5 "fixture export records not found at: $EXPORT/sources.jsonl"
[[ -f "$EXPORT/snapshot.json" ]] || die 5 "fixture snapshot envelope not found at: $EXPORT/snapshot.json"

log "#612 Recursio → NOMOS offline E2E"
log "fixture : $FIXTURE"
log "run dir : $RUN_DIR"

mkdir -p "$RUN_DIR"
RUN_DIR="$(cd "$RUN_DIR" && pwd)"
OUT="$RUN_DIR/out"; mkdir -p "$OUT"

# Read-only invariant: path+size of every fixture file, before and after.
before="$RUN_DIR/fixture-snapshot-before.txt"
after="$RUN_DIR/fixture-snapshot-after.txt"
( cd "$FIXTURE" && find . -type f -printf '%P %s\n' | LC_ALL=C sort ) > "$before"

# Stage 0 — the export is a fresh normalisation of the site (exit 4 otherwise).
log "stage 0: export ≡ normalise(site)"
if ! python3 "$SCRIPT_DIR/recursio_export_fixture.py" --check --root "$FIXTURE_ROOT"; then
  die 4 "committed export drifts from a fresh normalisation of the site — regenerate deliberately with scripts/recursio_export_fixture.py"
fi

# Build the CLI once.
NOMOS_BIN="$RUN_DIR/nomos"
log "building nomos"
( cd "$NOMOS_REPO/cli" && go build -o "$NOMOS_BIN" . ) || die 1 "go build failed"

# The corpus nomos reads: a throwaway git checkout holding a copy of the captures.
CORPUS="$RUN_DIR/corpus"; mkdir -p "$CORPUS"
cp -R "$EXPORT/captures" "$CORPUS/captures"
( cd "$CORPUS" && git init -q && git add -A && git -c user.email=nomos@local -c user.name=nomos commit -qm "recursio export captures" )
ENVELOPE="$RUN_DIR/snapshot.json"; RECORDS="$RUN_DIR/sources.jsonl"
cp "$EXPORT/snapshot.json" "$ENVELOPE"; cp "$EXPORT/sources.jsonl" "$RECORDS"

stage() { local name="$1"; shift; log "stage: $name"; "$@" || die 1 "stage failed: $name"; }

stage "snapshot verify"  "$NOMOS_BIN" corpus snapshot verify --envelope "$ENVELOPE" --records "$RECORDS" --out "$OUT/snapshot-verify.json"
stage "snapshot import"  "$NOMOS_BIN" corpus snapshot import --envelope "$ENVELOPE" --records "$RECORDS" --out "$OUT/source-manifest.yaml"
stage "scan"             "$NOMOS_BIN" corpus scan --root "$CORPUS" --out "$OUT/scan.json"
stage "feed"             "$NOMOS_BIN" corpus feed --root "$CORPUS" --snapshot "$OUT/scan.json" --manifest "$OUT/source-manifest.yaml" --out "$OUT/feed.json"
stage "body-ledger"      "$NOMOS_BIN" corpus body-ledger --root "$CORPUS" --manifest "$OUT/source-manifest.yaml" --out "$OUT/body-ledger.json"
stage "attest"           "$NOMOS_BIN" corpus attest --snapshot "$OUT/scan.json" --corpus-id recursio-fixture --project-id nomos \
                           --feed "$OUT/feed.json" --corpus-body-ledger "$OUT/body-ledger.json" \
                           --external-snapshot "$ENVELOPE" --external-snapshot-records "$RECORDS" --out "$OUT/attestation.json"
log "stage: strict gate"
"$NOMOS_BIN" strict --external-snapshot "$ENVELOPE" --external-snapshot-records "$RECORDS" \
  --corpus-integrity-source "$CORPUS" --corpus-integrity-feed "$OUT/feed.json" \
  --corpus-body-ledger "$OUT/body-ledger.json" --format json > "$OUT/strict.json" \
  || die 1 "stage failed: strict gate — see $OUT/strict.json"

# Assertions — measured on the artifacts, never assumed.
log "assertions"
expected_root="$(jq -r .content_hash_root "$ENVELOPE")"
expected_records="$(wc -l < "$RECORDS" | tr -d ' ')"
web_records="$(jq -c 'select(.web_source != null)' "$RECORDS" | wc -l | tr -d ' ')"
meta="$(jq -c '.predicate.metadata.external_snapshot // .metadata.external_snapshot // empty' "$OUT/attestation.json")"
[[ -n "$meta" ]] || die 1 "attestation carries no external_snapshot metadata"
assert_eq() { local what="$1" got="$2" want="$3"; [[ "$got" == "$want" ]] || die 1 "attestation $what: got '$got', want '$want'"; log "  ok $what = $got"; }
assert_eq "external_snapshot.content_hash_root" "$(jq -r .content_hash_root <<<"$meta")" "$expected_root"
assert_eq "external_snapshot.snapshot_id"       "$(jq -r .snapshot_id <<<"$meta")" "$(jq -r .snapshot_id "$ENVELOPE")"
assert_eq "external_snapshot.records"           "$(jq -r .records <<<"$meta")" "$expected_records"
assert_eq "external_snapshot.web_sources"       "$(jq -r .web_sources <<<"$meta")" "$web_records"
assert_eq "external_snapshot.source_types.html" "$(jq -r '.source_types.html' <<<"$meta")" "$web_records"
manifest_sources="$(grep -c '^  - id:' "$OUT/source-manifest.yaml" || true)"
assert_eq "manifest sources" "$manifest_sources" "$expected_records"
scanned="$(jq -r .total_files "$OUT/scan.json")"
assert_eq "scanned files" "$scanned" "$expected_records"
jq -e '.valid == true' "$OUT/strict.json" > /dev/null || die 1 "strict gate reported valid=false — see $OUT/strict.json"
jq -e '[.sections[] | select(.name == "external-snapshot")] | length == 1 and all(.valid)' "$OUT/strict.json" > /dev/null \
  || die 1 "strict gate did not run (or failed) the external-snapshot section — see $OUT/strict.json"
log "  ok strict.valid = true, external-snapshot section = valid"

# Read-only invariant after the run.
( cd "$FIXTURE" && find . -type f -printf '%P %s\n' | LC_ALL=C sort ) > "$after"
if ! diff -u "$before" "$after" > "$RUN_DIR/fixture-mutation.diff"; then
  die 4 "fixture mutated during the run — see $RUN_DIR/fixture-mutation.diff"
fi

jq -n --arg root "$expected_root" --argjson records "$expected_records" --argjson web "$web_records" \
  --arg run_dir "$RUN_DIR" '{schema_version:"nomos-recursio-e2e-v1", status:"pass", content_hash_root:$root, records:$records, web_sources:$web, run_dir:$run_dir,
  claim_boundary:"Offline fixture: proves the chain and the attestation binding on a synthetic site; says nothing about any real site, its robots or its licence."}' \
  > "$RUN_DIR/summary.json"
log "PASS — summary: $RUN_DIR/summary.json"
