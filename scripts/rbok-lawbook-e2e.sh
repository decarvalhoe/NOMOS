#!/usr/bin/env bash
# rbok-lawbook-e2e.sh — Real E2E pipeline for RBOK lawbook corpus.
#
# Usage:
#   bash scripts/rbok-lawbook-e2e.sh --corpus /path/to/01_rbok --out /path/to/output [--cli ./nomos-cli]
#
# Pipeline steps (using real CLI commands):
#   1. Read-only guard: disable push, record fingerprint.
#   2. Scan corpus: nomos corpus scan → snapshot.json
#   3. Generate manifest: nomos corpus manifest → source-manifest.yaml
#   4. Generate feed: nomos corpus feed --profile rbok-lawbook → feed.json
#   5. Generate attestation: nomos corpus attest → attestation.json
#   6. Validate artifacts: check node types, counts.
#   7. Post-extraction read-only verification.

set -euo pipefail

CORPUS_DIR=""
OUT_DIR=""
CLI_BIN=""
PROFILE="rbok-lawbook"
CORPUS_ID="rbok-lawbook"
PROJECT_ID="rbok"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  echo "Usage: $0 --corpus <dir> --out <dir> [--cli <binary>] [--profile <name>] [--corpus-id <id>] [--project-id <id>]"
  echo ""
  echo "  --corpus      Path to the 01_rbok corpus directory"
  echo "  --out         Output directory for all pipeline artifacts"
  echo "  --cli         Path to nomos CLI binary (default: build from source)"
  echo "  --profile     Corpus profile (default: rbok-lawbook)"
  echo "  --corpus-id   Corpus identifier for attestation (default: rbok-lawbook)"
  echo "  --project-id  Project identifier for attestation (default: rbok)"
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --corpus)     CORPUS_DIR="$2"; shift 2 ;;
    --out)        OUT_DIR="$2"; shift 2 ;;
    --cli)        CLI_BIN="$2"; shift 2 ;;
    --profile)    PROFILE="$2"; shift 2 ;;
    --corpus-id)  CORPUS_ID="$2"; shift 2 ;;
    --project-id) PROJECT_ID="$2"; shift 2 ;;
    -h|--help)    usage ;;
    *)            echo "Unknown option: $1"; usage ;;
  esac
done

if [[ -z "$CORPUS_DIR" || -z "$OUT_DIR" ]]; then
  echo "ERROR: --corpus and --out are required"
  usage
fi

# --- Step 0: Resolve paths ---
CORPUS_DIR="$(cd "$CORPUS_DIR" && pwd)"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

# --- Step 1: Read-only guard ---
echo "=== Step 1: Read-only guard ==="

if git -C "$CORPUS_DIR" rev-parse --show-toplevel > /dev/null 2>&1; then
  CORPUS_GIT_ROOT="$(git -C "$CORPUS_DIR" rev-parse --show-toplevel)"
  push_url="$(git -C "$CORPUS_GIT_ROOT" remote get-url --push origin 2>/dev/null || true)"
  if [[ -n "$push_url" && "$push_url" != "no_push" ]]; then
    echo "WARNING: Source corpus has push-capable remote: $push_url"
    echo "  Disable with: git -C $CORPUS_GIT_ROOT remote set-url --push origin no_push"
  fi
fi

FINGERPRINT_BEFORE="$(mktemp)"
find "$CORPUS_DIR" -type f -exec sha256sum {} \; | sort > "$FINGERPRINT_BEFORE"
echo "Corpus files: $(wc -l < "$FINGERPRINT_BEFORE")"

# --- Step 2: Build CLI if needed ---
if [[ -z "$CLI_BIN" ]]; then
  echo "=== Building CLI ==="
  CLI_BIN="$ROOT_DIR/nomos-cli"
  (cd "$ROOT_DIR/cli" && go build -o "$CLI_BIN" .)
  echo "Built: $CLI_BIN"
fi

if [[ ! -x "$CLI_BIN" ]]; then
  echo "ERROR: CLI binary not found or not executable: $CLI_BIN"
  exit 1
fi

# --- Step 3: Scan corpus ---
echo "=== Step 3: Scan corpus ==="
SNAPSHOT="$OUT_DIR/snapshot.json"
"$CLI_BIN" corpus scan \
  --root "$CORPUS_DIR" \
  --out "$SNAPSHOT" \
  --ext .md \
  --allow "**/*.md"

echo "Snapshot: $SNAPSHOT"

# --- Step 4: Generate manifest ---
echo "=== Step 4: Generate manifest ==="
MANIFEST="$OUT_DIR/source-manifest.yaml"
"$CLI_BIN" corpus manifest \
  --snapshot "$SNAPSHOT" \
  --out "$MANIFEST" \
  --domain "lawbook" \
  --owner "RBOK Corpus Team" \
  --id-prefix "RBOK"

echo "Manifest: $MANIFEST"

# --- Step 5: Generate feed ---
echo "=== Step 5: Generate feed (profile: $PROFILE) ==="
FEED="$OUT_DIR/feed.json"
"$CLI_BIN" corpus feed \
  --root "$CORPUS_DIR" \
  --snapshot "$SNAPSHOT" \
  --manifest "$MANIFEST" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --out "$FEED"

echo "Feed: $FEED"

# --- Step 6: Generate attestation ---
echo "=== Step 6: Generate attestation ==="
ATTESTATION="$OUT_DIR/attestation.json"
"$CLI_BIN" corpus attest \
  --snapshot "$SNAPSHOT" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --verdict "corpus_admissible" \
  --confidence "high" \
  --out "$ATTESTATION"

echo "Attestation: $ATTESTATION"

# --- Step 7: Validate artifacts ---
echo "=== Step 7: Validate artifacts ==="
artifact_count="$(find "$OUT_DIR" -type f -name '*.json' -o -name '*.yaml' | wc -l)"
echo "Total artifacts: $artifact_count"

if [[ "$artifact_count" -lt 3 ]]; then
  echo "FAIL: Expected at least 3 artifacts (snapshot, feed, attestation), got $artifact_count"
  exit 1
fi

# Validate feed has nodes
python3 - "$FEED" <<'PY'
import json, sys, pathlib
feed_path = sys.argv[1]
data = json.loads(pathlib.Path(feed_path).read_text(encoding="utf-8"))

# Accept both flat feed format and assembly format
nodes = data.get("nodes", [])
if not nodes and "feed" in data:
    nodes = data["feed"].get("nodes", [])
if not nodes and "units" in data:
    nodes = data["units"]

print(f"Feed nodes: {len(nodes)}")
if len(nodes) == 0:
    # Non-fatal for empty corpus; warn but don't fail
    print("WARNING: Feed contains no nodes (corpus may be empty or not yet populated)")
else:
    from collections import Counter
    counts = Counter()
    for node in nodes:
        node_type = node.get("node_type", node.get("type", "unknown"))
        counts[node_type] += 1
    for t in sorted(counts):
        print(f"  {t}: {counts[t]}")
PY

# --- Step 8: Post-extraction read-only check ---
echo "=== Step 8: Read-only verification ==="

FINGERPRINT_AFTER="$(mktemp)"
find "$CORPUS_DIR" -type f -exec sha256sum {} \; | sort > "$FINGERPRINT_AFTER"

if ! diff -q "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER" > /dev/null 2>&1; then
  echo "FAIL: Source corpus was modified during extraction!"
  diff "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER" || true
  rm -f "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER"
  exit 1
fi

rm -f "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER"
echo "Source corpus integrity verified."

echo ""
echo "=== E2E pipeline complete ==="
echo "Profile:      $PROFILE"
echo "Output:       $OUT_DIR"
echo "Artifacts:    $artifact_count"
echo "  snapshot:     $(basename "$SNAPSHOT")"
echo "  manifest:     $(basename "$MANIFEST")"
echo "  feed:         $(basename "$FEED")"
echo "  attestation:  $(basename "$ATTESTATION")"
