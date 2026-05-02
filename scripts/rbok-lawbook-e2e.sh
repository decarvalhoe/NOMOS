#!/usr/bin/env bash
# rbok-lawbook-e2e.sh — Local E2E extraction of RBOK lawbook Markdown.
#
# Usage:
#   bash scripts/rbok-lawbook-e2e.sh --corpus /path/to/01_rbok --out /path/to/output [--cli ./nomos-cli]
#
# This script:
#   1. Validates the corpus directory exists and is read-only (no push remote).
#   2. Finds all Markdown files in the corpus.
#   3. Extracts lawbook nodes from each file.
#   4. Validates required node types are present.
#   5. Verifies the source corpus was not modified.

set -euo pipefail

CORPUS_DIR=""
OUT_DIR=""
CLI_BIN=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
  echo "Usage: $0 --corpus <dir> --out <dir> [--cli <binary>]"
  echo ""
  echo "  --corpus   Path to the 01_rbok corpus directory"
  echo "  --out      Output directory for extracted JSON nodes"
  echo "  --cli      Path to nomos CLI binary (default: build from source)"
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --corpus) CORPUS_DIR="$2"; shift 2 ;;
    --out)    OUT_DIR="$2"; shift 2 ;;
    --cli)    CLI_BIN="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "Unknown option: $1"; usage ;;
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
echo "=== Read-only guard ==="

# Check if corpus is inside a git repo
CORPUS_GIT_ROOT=""
if git -C "$CORPUS_DIR" rev-parse --show-toplevel > /dev/null 2>&1; then
  CORPUS_GIT_ROOT="$(git -C "$CORPUS_DIR" rev-parse --show-toplevel)"
  push_url="$(git -C "$CORPUS_GIT_ROOT" remote get-url --push origin 2>/dev/null || true)"
  if [[ -n "$push_url" && "$push_url" != "no_push" ]]; then
    echo "WARNING: Source corpus has push-capable remote: $push_url"
    echo "  Disable with: git -C $CORPUS_GIT_ROOT remote set-url --push origin no_push"
  fi
fi

# Record fingerprint before extraction
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

# --- Step 3: Find Markdown files ---
echo "=== Scanning corpus ==="
md_files=()
while IFS= read -r -d '' f; do
  md_files+=("$f")
done < <(find "$CORPUS_DIR" -type f -name '*.md' -print0 | sort -z)

echo "Markdown files found: ${#md_files[@]}"

if [[ ${#md_files[@]} -eq 0 ]]; then
  echo "FAIL: No Markdown files found in $CORPUS_DIR"
  exit 1
fi

# --- Step 4: Extract nodes ---
echo "=== Extracting lawbook nodes ==="
extracted=0
for md_file in "${md_files[@]}"; do
  rel_path="${md_file#"$CORPUS_DIR"/}"
  slug="$(echo "$rel_path" | sed 's/\.md$//' | tr '/' '-' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')"
  out_file="$OUT_DIR/${slug}.json"

  echo "  $rel_path -> $(basename "$out_file")"

  (cd "$ROOT_DIR/cli" && go run ./internal/corpus/cmd/extract \
    --input "$md_file" \
    --slug "$slug" \
    --output "$out_file")

  extracted=$((extracted + 1))
done

echo "Extracted: $extracted file(s)"

# --- Step 5: Validate artifacts ---
echo "=== Validating artifacts ==="
artifact_count="$(find "$OUT_DIR" -type f -name '*.json' | wc -l)"
echo "JSON artifacts: $artifact_count"

if [[ "$artifact_count" -eq 0 ]]; then
  echo "FAIL: No JSON artifacts produced"
  exit 1
fi

# Check required node types
python3 - "$OUT_DIR" <<'PY'
import json
import pathlib
import sys
from collections import Counter

out_dir = pathlib.Path(sys.argv[1])
required = {"document", "article", "paragraph", "alinea"}
counts = Counter()
for path in out_dir.glob("*.json"):
    data = json.loads(path.read_text(encoding="utf-8"))
    for node in data.get("nodes", []):
        node_type = node.get("node_type")
        if node_type:
            counts[node_type] += 1

for node_type in sorted(counts):
    print(f"  {node_type}: {counts[node_type]} node(s)")

missing = sorted(required - set(counts))
if missing:
    print(f"WARNING: missing expected node types: {', '.join(missing)}")
PY

# --- Step 6: Read-only verification ---
echo "=== Post-extraction read-only check ==="

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
echo "=== E2E complete ==="
echo "Output: $OUT_DIR"
echo "Artifacts: $artifact_count"
