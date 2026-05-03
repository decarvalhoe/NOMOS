#!/usr/bin/env bash
# rbok-lawbook-e2e.sh — Real E2E pipeline for RBOK lawbook corpus.
#
# Usage:
#   bash scripts/rbok-lawbook-e2e.sh --corpus /path/to/01_rbok --out /path/to/output [--cli ./nomos-cli]
#
# Pipeline steps (using real CLI commands):
#   1. Read-only guard: disable push, record fingerprint.
#   2. Diagnose full rbok-lawbook profile.
#   3. Generate gate-compatible lawbook artifact pack.
#   4. Run RBOK lawbook release gate.
#   5. Validate artifacts: check node types, counts, attestation.
#   6. Post-extraction read-only verification.

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

# --- Step 3: Diagnose full profile ---
echo "=== Step 3: Diagnose corpus profile ==="
DIAGNOSIS="$OUT_DIR/diagnosis.json"
"$CLI_BIN" corpus diagnose \
  --profile "$PROFILE" \
  --root "$CORPUS_DIR" \
  --format json > "$DIAGNOSIS"

echo "Diagnosis: $DIAGNOSIS"

# --- Step 4: Generate lawbook artifact pack ---
echo "=== Step 4: Generate lawbook artifact pack (profile: $PROFILE) ==="
PACK="$OUT_DIR/artifact-pack.json"
"$CLI_BIN" corpus feed \
  --profile "$PROFILE" \
  --root "$CORPUS_DIR" \
  --artifacts-dir "$OUT_DIR" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --out "$PACK"

echo "Artifact pack: $PACK"

# --- Step 5: Run release gate ---
echo "=== Step 5: Run release gate ==="
(cd "$ROOT_DIR/cli" && go run ./internal/corpus/cmd/release-gate --artifacts "$OUT_DIR" --profile "$PROFILE")

# --- Step 6: Validate artifacts ---
echo "=== Step 6: Validate artifacts ==="
artifact_count="$(find "$OUT_DIR" -type f \( -name '*.json' -o -name '*.yaml' \) | wc -l)"
echo "Total artifacts: $artifact_count"

if [[ "$artifact_count" -lt 6 ]]; then
  echo "FAIL: Expected at least 6 artifacts, got $artifact_count"
  exit 1
fi

for f in rbok-lawbook-feed.json rbok-lawbook-index.json rbok-rag-metadata.json rbok-engine-import.json rbok-governance.json rbok-attestation.json; do
  if [[ ! -f "$OUT_DIR/$f" ]]; then
    echo "FAIL: missing expected artifact: $f"
    exit 1
  fi
  echo "  ok: $f ($(wc -c < "$OUT_DIR/$f") bytes)"
done

# Validate lawbook feed has nodes.
python3 - "$OUT_DIR/rbok-lawbook-feed.json" <<'PY'
import json, sys, pathlib
feed_path = sys.argv[1]
data = json.loads(pathlib.Path(feed_path).read_text(encoding="utf-8"))

nodes = list(data.get("nodes", []))
for feed in data.get("feeds", []):
    nodes.extend(feed.get("nodes", []))

print(f"Feed nodes: {len(nodes)}")
if len(nodes) == 0:
    print("FAIL: Feed contains no nodes")
    sys.exit(1)
else:
    from collections import Counter
    counts = Counter()
    for node in nodes:
        node_type = node.get("node_type", node.get("type", "unknown"))
        counts[node_type] += 1
    for t in sorted(counts):
        print(f"  {t}: {counts[t]}")
PY

# --- Step 7: Post-extraction read-only check ---
echo "=== Step 7: Read-only verification ==="

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
echo "  diagnosis:    $(basename "$DIAGNOSIS")"
echo "  pack:         $(basename "$PACK")"
echo "  feed:         rbok-lawbook-feed.json"
echo "  attestation:  rbok-attestation.json"
