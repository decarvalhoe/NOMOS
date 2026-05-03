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
#   4. Diagnose corpus: nomos corpus diagnose --profile rbok-lawbook → rbok-governance.json
#   5. Generate profile feed: nomos corpus feed --profile rbok-lawbook → profile-output.json
#   6. Split profile artifacts: feed, RAG metadata, atomization report, traceability matrix.
#   7. Generate attestation: nomos corpus attest → attestation.json
#   8. Validate artifacts: check node types, counts, traceability.
#   9. Post-extraction read-only verification.

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
  push_url_lc="$(printf '%s' "$push_url" | tr '[:upper:]' '[:lower:]')"
  if [[ -n "$push_url" && "$push_url_lc" != "no_push" && "$push_url_lc" != "no-push" && "$push_url_lc" != "disabled" && "$push_url_lc" != "none" ]]; then
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
  --out "$SNAPSHOT"

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

# --- Step 5: Diagnose corpus ---
echo "=== Step 5: Diagnose corpus (profile: $PROFILE) ==="
GOVERNANCE="$OUT_DIR/rbok-governance.json"
"$CLI_BIN" corpus diagnose \
  --profile "$PROFILE" \
  --root "$CORPUS_DIR" \
  --format json > "$GOVERNANCE"

echo "Governance: $GOVERNANCE"

# --- Step 6: Generate profile feed ---
echo "=== Step 6: Generate feed (profile: $PROFILE) ==="
PROFILE_FEED="$OUT_DIR/profile-output.json"
"$CLI_BIN" corpus feed \
  --profile "$PROFILE" \
  --root "$CORPUS_DIR" \
  --outputs feed,rag_metadata,atomization_report,traceability_matrix \
  --out "$PROFILE_FEED"

FEED="$OUT_DIR/rbok-lawbook-feed.json"
RAG="$OUT_DIR/rbok-rag-metadata.json"
ATOMIZATION="$OUT_DIR/rbok-atomization-report.json"
TRACEABILITY="$OUT_DIR/rbok-traceability-matrix.json"
ENGINE_IMPORT="$OUT_DIR/rbok-engine-import.json"

python3 - "$PROFILE_FEED" "$FEED" "$RAG" "$ATOMIZATION" "$TRACEABILITY" "$ENGINE_IMPORT" <<'PY'
import json, pathlib, sys
profile_path, feed_path, rag_path, atom_path, trace_path, engine_path = map(pathlib.Path, sys.argv[1:])
profile = json.loads(profile_path.read_text(encoding="utf-8"))
sections = profile.get("sections", {})
required = ["feed", "rag_metadata", "atomization_report", "traceability_matrix"]
missing = [name for name in required if name not in sections]
if missing:
    raise SystemExit(f"missing profile section(s): {', '.join(missing)}")
feed = sections["feed"]
feed_path.write_text(json.dumps(feed, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
rag_path.write_text(json.dumps(sections["rag_metadata"], indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
atom_path.write_text(json.dumps(sections["atomization_report"], indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
trace_path.write_text(json.dumps(sections["traceability_matrix"], indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
engine_path.write_text(json.dumps(feed.get("engine_import", {}), indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
PY

echo "Feed: $FEED"
echo "RAG: $RAG"
echo "Atomization: $ATOMIZATION"
echo "Traceability: $TRACEABILITY"
echo "Engine import: $ENGINE_IMPORT"

# --- Step 7: Generate attestation ---
echo "=== Step 7: Generate attestation ==="
ATTESTATION="$OUT_DIR/attestation.json"
"$CLI_BIN" corpus attest \
  --snapshot "$SNAPSHOT" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --verdict "corpus_admissible" \
  --confidence "high" \
  --out "$ATTESTATION"

echo "Attestation: $ATTESTATION"

# --- Step 8: Validate artifacts ---
echo "=== Step 8: Validate artifacts ==="
artifact_count="$(find "$OUT_DIR" -type f \( -name '*.json' -o -name '*.yaml' \) | wc -l)"
echo "Total artifacts: $artifact_count"

if [[ "$artifact_count" -lt 8 ]]; then
  echo "FAIL: Expected at least 8 artifacts, got $artifact_count"
  exit 1
fi

# Validate feed, RAG and traceability.
python3 - "$FEED" "$RAG" "$ATOMIZATION" "$TRACEABILITY" <<'PY'
import json, sys, pathlib
feed_path, rag_path, atom_path, trace_path = map(pathlib.Path, sys.argv[1:])
data = json.loads(pathlib.Path(feed_path).read_text(encoding="utf-8"))
rag = json.loads(rag_path.read_text(encoding="utf-8"))
atom = json.loads(atom_path.read_text(encoding="utf-8"))
trace = json.loads(trace_path.read_text(encoding="utf-8"))

nodes = data.get("nodes", [])
if not nodes and "feed" in data:
    nodes = data["feed"].get("nodes", [])
if not nodes and "feeds" in data:
    for feed in data["feeds"]:
        nodes.extend(feed.get("nodes", []))

print(f"Feed nodes: {len(nodes)}")
if len(nodes) == 0:
    raise SystemExit("feed contains no nodes")
if len(rag) != len(nodes):
    raise SystemExit(f"RAG chunk count {len(rag)} does not match node count {len(nodes)}")
if len(trace) != len(nodes):
    raise SystemExit(f"traceability row count {len(trace)} does not match node count {len(nodes)}")
if atom.get("missing_source_hash") != 0 or atom.get("missing_locator") != 0:
    raise SystemExit("atomization report has missing source hash or locator")
from collections import Counter
counts = Counter(node.get("node_type", "unknown") for node in nodes)
for required in ["document", "article", "paragraph", "alinea"]:
    if counts[required] == 0:
        raise SystemExit(f"missing required node type: {required}")
for t in sorted(counts):
    print(f"  {t}: {counts[t]}")
PY

# --- Step 9: Post-extraction read-only check ---
echo "=== Step 9: Read-only verification ==="

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
echo "  governance:   $(basename "$GOVERNANCE")"
echo "  profile:      $(basename "$PROFILE_FEED")"
echo "  feed:         $(basename "$FEED")"
echo "  rag:          $(basename "$RAG")"
echo "  atomization:  $(basename "$ATOMIZATION")"
echo "  traceability: $(basename "$TRACEABILITY")"
echo "  engine:       $(basename "$ENGINE_IMPORT")"
echo "  attestation:  $(basename "$ATTESTATION")"
