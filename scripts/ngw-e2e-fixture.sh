#!/usr/bin/env bash
# NGW-10 (#395) end-to-end fixture runner.
#
# Drives the full NGW chain on the synthetic corpus + output pair under
# tests/fixtures/ngw-e2e/ without touching any real corpus. All three
# publication modes (artifact_only, pull_request, direct_push) are
# exercised in dry-run / local-safe mode.
#
# Read-only invariant: the fixture corpus is treated read-only; a
# pre-/post-run snapshot of every file's path+size is captured and any
# divergence is a hard failure (exit 4). Outputs are written to a
# per-timestamp $RUN_DIR under ./reports/ngw-e2e/, never inside the
# fixture directory.
#
# Exit codes:
#   0 — chain ran successfully end-to-end on the fixture
#   1 — a stage failed
#   4 — fixture corpus mutated during the run
#   5 — preflight failure (missing fixture, missing tool, etc.)

set -euo pipefail

FIXTURE_DIR="${FIXTURE_DIR:-tests/fixtures/ngw-e2e}"
RUN_DIR="${RUN_DIR:-./reports/ngw-e2e/$(date -u +%Y%m%dT%H%M%SZ)}"
RUN_TIMESTAMP="${RUN_TIMESTAMP:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NOMOS_REPO="$(cd "$SCRIPT_DIR/.." && pwd)"

CORPUS_DIR="$NOMOS_REPO/$FIXTURE_DIR/corpus"
CONFIG_PATH="$CORPUS_DIR/.nomos/corpus-workflows.yaml"
NOMOS_CLI_PKG_DIR="$NOMOS_REPO/cli"

log() { printf '[ngw-e2e] %s\n' "$*"; }
die() { local code="$1"; shift; printf '[ngw-e2e] FATAL: %s\n' "$*" >&2; exit "$code"; }
require_tool() { command -v "$1" >/dev/null 2>&1 || die 5 "required tool not on PATH: $1"; }

require_tool go
require_tool python3
require_tool jq

if [[ ! -d "$CORPUS_DIR" ]]; then
  die 5 "fixture corpus not found at: $CORPUS_DIR"
fi
if [[ ! -f "$CONFIG_PATH" ]]; then
  die 5 "fixture config not found at: $CONFIG_PATH"
fi

log "NGW-10 (#395) E2E fixture runner"
log "fixture : $CORPUS_DIR"
log "run dir : $RUN_DIR"
log "nomos   : $NOMOS_REPO"

mkdir -p "$RUN_DIR"
RUN_DIR="$(cd "$RUN_DIR" && pwd)"

# Snapshot the fixture corpus so we can detect mutation.
snapshot_before="$RUN_DIR/fixture-snapshot-before.txt"
( cd "$CORPUS_DIR" && find . -type f -printf '%P %s\n' | LC_ALL=C sort ) > "$snapshot_before"

# ---- build NOMOS CLI -------------------------------------------------------

log "step 1: build NOMOS CLI"
nomos_bin="$RUN_DIR/nomos"
( cd "$NOMOS_CLI_PKG_DIR" && go build -o "$nomos_bin" . )

# ---- run nomos github plan -------------------------------------------------

log "step 2: nomos github plan (impacts-both — both scopes feed the per-mode runs)"
plan_path="$RUN_DIR/nomos-diff.json"
"$nomos_bin" github plan \
  --config       "$CONFIG_PATH" \
  --changed-paths "$NOMOS_REPO/$FIXTURE_DIR/changed-paths-impacts-both.txt" \
  --out          "$plan_path" \
  --frozen-time  "$RUN_TIMESTAMP" \
  --format       json

impacted_count=$(jq '.impacted | length' "$plan_path")
log "  plan -> $plan_path (impacted=$impacted_count)"
if [[ "$impacted_count" != "2" ]]; then
  die 1 "expected impacted=2 (scope-a + scope-b), got $impacted_count"
fi

# ---- per-mode trace + publisher dry-run ------------------------------------

# Synthesise a small outputs directory the publisher will copy/plan from.
outputs_dir="$RUN_DIR/outputs"
mkdir -p "$outputs_dir"
echo '{"format":"nomos.corpus-feed.v1"}' > "$outputs_dir/feed.json"

source_sha="f1e2d3c4b5a6978869504132241302d4e5f6a7b8"
output_sha="abcdef1234567890abcdef1234567890abcdef12"

declare -A summary

run_mode() {
  local mode="$1" scope="$2" target_path="$3" branch="$4"
  local trace_yaml="$RUN_DIR/trace-${mode}.yaml"
  local trace_json="$RUN_DIR/trace-${mode}.json"
  local diff_for_scope="$RUN_DIR/nomos-diff-${scope}.json"

  log "step 3.$mode: scope=$scope mode=$mode"

  # Build a per-scope diff plan (the trace generator validates that the
  # workflow_id maps to an impacted entry).
  jq --arg sid "$scope" \
    '.impacted = (.impacted | map(select(.workflow_id == $sid)))
     | .skipped = []' \
    "$plan_path" > "$diff_for_scope"

  # Generate trace manifest.
  python3 "$NOMOS_REPO/scripts/nomos_trace_manifest.py" \
    --diff-plan          "$diff_for_scope" \
    --workflow-id        "$scope" \
    --event              pull_request \
    --workflow-run-id    "ngw-e2e-${mode}" \
    --corpus-repo        "ngw-e2e/synthetic" \
    --corpus-base-ref    "main" \
    --corpus-base-sha    "1f9d2c8b07e3a4f5d6c7b8a9e0d1c2b3a4f5d6c7" \
    --corpus-head-ref    "feature/synth" \
    --corpus-head-sha    "9b2a3c4d5e6f7081923a4b5c6d7e8f90a1b2c3d4" \
    --pull-request       7 \
    --output-repo        "ngw-e2e/output" \
    --output-branch      "$branch" \
    --output-path        "$target_path" \
    --output-commit-sha  "$output_sha" \
    --publish-mode       "$mode" \
    --risk-class         "low" \
    --generated-path-guard   "pass" \
    --source-read-only-guard "pass" \
    --artifact-feed      "${target_path}feed.json" \
    --out-yaml           "$trace_yaml" \
    --out-json           "$trace_json" \
    --frozen-time        "$RUN_TIMESTAMP" \
    --no-cue-vet
  trace_rc=$?
  if [[ "$trace_rc" != "0" ]]; then
    die 1 "trace generator failed for $mode (rc=$trace_rc)"
  fi

  # Optional cue-vet (best-effort).
  if command -v cue >/dev/null 2>&1; then
    if cue vet \
        "$NOMOS_REPO/specs/nomos-trace-manifest.cue" \
        "$trace_yaml" \
        -d '#NomosTraceManifest' >/dev/null 2>"$RUN_DIR/cue-vet-${mode}.err"; then
      log "  cue vet $mode: PASS"
    else
      log "  cue vet $mode: FAIL (see $RUN_DIR/cue-vet-${mode}.err)"
      cat "$RUN_DIR/cue-vet-${mode}.err" >&2
      die 1 "cue vet failed for $mode"
    fi
  else
    log "  cue not on PATH; skipping schema validation for $mode"
  fi

  # Publisher dry-run via Python (the publisher exposes its mode entry
  # points as importable functions; we shell out to its CLI to mirror the
  # path tests would take).
  local publish_log="$RUN_DIR/publish-${mode}.json"
  python3 "$NOMOS_REPO/scripts/nomos_github_publish.py" \
    --config         "$CONFIG_PATH" \
    --workflow-id    "$scope" \
    --diff-plan      "$diff_for_scope" \
    --outputs-dir    "$outputs_dir" \
    --trace-manifest "$trace_yaml" \
    --mode           "$mode" \
    --target-repo    "ngw-e2e/output" \
    --target-branch  "$branch" \
    --target-path    "$target_path" \
    --source-sha     "$source_sha" \
    --branch-strategy per_pr \
    --source-pr-number 7 \
    --commit-subject "nomos: refresh $scope" \
    --dry-run \
    > "$publish_log"
  publish_rc=$?
  log "  publisher $mode: rc=$publish_rc -> $publish_log"
  summary[$mode]="$publish_rc"

  # Commenter dry-run (only meaningful when the scope enables it).
  local comment_log="$RUN_DIR/comment-${mode}.txt"
  python3 "$NOMOS_REPO/scripts/nomos_github_comment.py" \
    --config      "$CONFIG_PATH" \
    --workflow-id "$scope" \
    --diff-plan   "$diff_for_scope" \
    --gate-status skipped \
    --dry-run \
    > "$comment_log"
  log "  commenter $scope: rc=$? -> $comment_log"
}

run_mode artifact_only scope-a output/scope-a/ main
run_mode pull_request  scope-a output/scope-a/ nomos/refresh-scope-a-pr-7
run_mode direct_push   scope-b output/scope-b/ main

# ---- read-only invariant ---------------------------------------------------

snapshot_after="$RUN_DIR/fixture-snapshot-after.txt"
( cd "$CORPUS_DIR" && find . -type f -printf '%P %s\n' | LC_ALL=C sort ) > "$snapshot_after"
if ! diff -q "$snapshot_before" "$snapshot_after" > /dev/null; then
  diff -u "$snapshot_before" "$snapshot_after" > "$RUN_DIR/fixture-mutation.diff" || true
  die 4 "fixture corpus was mutated during the run; see $RUN_DIR/fixture-mutation.diff"
fi
log "fixture corpus untouched (read-only invariant verified)"

# ---- summary ---------------------------------------------------------------

log ""
log "summary:"
log "  fixture           : $CORPUS_DIR (2KB synthetic)"
log "  plan artifact     : $plan_path"
for mode in artifact_only pull_request direct_push; do
  printf '[ngw-e2e]   publisher %-13s rc=%s\n' "$mode" "${summary[$mode]:-?}"
done
log "  evidence          : $RUN_DIR/"
exit 0
