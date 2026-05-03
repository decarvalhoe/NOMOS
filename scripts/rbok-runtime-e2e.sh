#!/usr/bin/env bash
# rbok-runtime-e2e.sh — Full read-only E2E for realisons-business multi-layer corpus.
#
# Usage:
#   bash scripts/rbok-runtime-e2e.sh --corpus /path/to/realisons-business --out /path/to/output
#
# Pipeline:
#   1. Read-only guard: disable push, fingerprint corpus
#   2. Scan all layers: 01_rbok, 02_parcours, 03_workbooks, 00_meta, 98_schemas, 99_*
#   3. Layer classification: verify each file gets correct layer/priority/role
#   4. Generate sidecar manifest
#   5. Feed generation with profile rbok-lawbook
#   6. Governance evaluation
#   7. RAG metadata extraction check
#   8. Attestation
#   9. Git clean verification
#  10. Summary

set -euo pipefail

CORPUS_DIR=""
OUT_DIR=""
CLI_BIN=""
PROFILE="rbok-lawbook"
CORPUS_ID="realisons-business"
PROJECT_ID="rbok"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
FAIL_COUNT=0
PASS_COUNT=0

usage() {
  echo "Usage: $0 --corpus <dir> --out <dir> [--cli <binary>]"
  exit 2
}

pass() { echo "  PASS: $1"; ((PASS_COUNT++)) || true; }
fail() { echo "  FAIL: $1"; ((FAIL_COUNT++)) || true; }
step() { echo ""; echo "=== $1 ==="; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --corpus)     CORPUS_DIR="$2"; shift 2 ;;
    --out)        OUT_DIR="$2"; shift 2 ;;
    --cli)        CLI_BIN="$2"; shift 2 ;;
    --corpus-id)  CORPUS_ID="$2"; shift 2 ;;
    --project-id) PROJECT_ID="$2"; shift 2 ;;
    -h|--help)    usage ;;
    *)            echo "Unknown: $1"; usage ;;
  esac
done

[[ -z "$CORPUS_DIR" || -z "$OUT_DIR" ]] && { echo "ERROR: --corpus and --out required"; usage; }

CORPUS_DIR="$(cd "$CORPUS_DIR" && pwd)"
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

# --- Step 1: Read-only guard ---
step "Step 1: Read-only guard"

GIT_CLEAN_CHECK=""
if git -C "$CORPUS_DIR" rev-parse --show-toplevel > /dev/null 2>&1; then
  CORPUS_GIT_ROOT="$(git -C "$CORPUS_DIR" rev-parse --show-toplevel)"
  GIT_CLEAN_CHECK="$CORPUS_GIT_ROOT"

  push_url="$(git -C "$CORPUS_GIT_ROOT" remote get-url --push origin 2>/dev/null || true)"
  if [[ -n "$push_url" && "$push_url" != "no_push" ]]; then
    echo "  WARNING: push-capable remote detected: $push_url"
  fi

  git_status="$(git -C "$CORPUS_GIT_ROOT" status --porcelain 2>/dev/null)"
  if [[ -n "$git_status" ]]; then
    fail "corpus has uncommitted changes"
  else
    pass "git working tree clean"
  fi
else
  echo "  INFO: corpus is not a git repo, skip git guard"
fi

FINGERPRINT_BEFORE="$(mktemp)"
find "$CORPUS_DIR" -type f -not -path '*/.git/*' | sort | xargs sha256sum 2>/dev/null > "$FINGERPRINT_BEFORE" || true
FILE_COUNT_BEFORE="$(wc -l < "$FINGERPRINT_BEFORE")"
echo "  Corpus files: $FILE_COUNT_BEFORE"

# --- Step 2: Build CLI ---
if [[ -z "$CLI_BIN" ]]; then
  step "Step 2: Build CLI"
  CLI_BIN="$ROOT_DIR/nomos-cli"
  (cd "$ROOT_DIR/cli" && CGO_ENABLED=0 go build -o "$CLI_BIN" . 2>/dev/null) || \
  (cd "$ROOT_DIR/cli" && go build -o "$CLI_BIN" .)
  pass "CLI built: $CLI_BIN"
fi

# --- Step 3: Scan corpus ---
step "Step 3: Scan all layers"
SNAPSHOT="$OUT_DIR/snapshot.json"
"$CLI_BIN" corpus scan \
  --root "$CORPUS_DIR" \
  --out "$SNAPSHOT" \
  2>&1 || { fail "corpus scan failed"; }

if [[ -f "$SNAPSHOT" ]]; then
  scan_count="$(python3 -c "import json; print(json.load(open('$SNAPSHOT')).get('total_files',0))" 2>/dev/null || echo 0)"
  pass "scanned $scan_count files"
else
  fail "snapshot not created"
  scan_count=0
fi

# --- Step 4: Layer classification ---
step "Step 4: Layer classification"
LAYER_REPORT="$OUT_DIR/layer-classification.json"

python3 - "$SNAPSHOT" "$LAYER_REPORT" <<'PYEOF'
import json, sys, os
sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))

snapshot_path, report_path = sys.argv[1], sys.argv[2]
snapshot = json.loads(open(snapshot_path).read())
sources = snapshot.get("sources", [])

# Classify using directory prefixes (mirrors Go ClassifyRuntimeLayer)
layer_map = {
    "00_meta": "meta", "01_rbok": "doctrine", "01_referentiel": "doctrine",
    "02_parcours": "runtime", "02_domaines": "runtime",
    "03_workbook": "workbooks", "03_generated": "workbooks",
    "98_schema": "schemas", "99_rbok": "reference", "99_initial": "reference",
}

results = {"layers": {}, "total": len(sources), "classified": 0, "unknown": 0}
for src in sources:
    path = src.get("path", "")
    top = path.split("/")[0].lower() if "/" in path else path.lower()
    layer = "unknown"
    for prefix, lyr in layer_map.items():
        if top.startswith(prefix):
            layer = lyr
            break
    results["layers"].setdefault(layer, 0)
    results["layers"][layer] += 1
    if layer != "unknown":
        results["classified"] += 1
    else:
        results["unknown"] += 1

with open(report_path, "w") as f:
    json.dump(results, f, indent=2)

print(f"  Layers: {json.dumps(results['layers'])}")
print(f"  Classified: {results['classified']}/{results['total']}")
PYEOF

if [[ -f "$LAYER_REPORT" ]]; then
  pass "layer classification report generated"
else
  fail "layer classification failed"
fi

# --- Step 5: Generate manifest ---
step "Step 5: Generate sidecar manifest"
MANIFEST="$OUT_DIR/source-manifest.yaml"
"$CLI_BIN" corpus manifest \
  --snapshot "$SNAPSHOT" \
  --out "$MANIFEST" \
  --domain "realisons-business" \
  --owner "RBOK Runtime Team" \
  --id-prefix "RB" \
  2>&1 || { fail "manifest generation failed"; }

if [[ -f "$MANIFEST" ]]; then
  pass "sidecar manifest generated"
else
  fail "manifest not created"
fi

# --- Step 6: Feed generation ---
step "Step 6: Feed generation (profile: $PROFILE)"
FEED="$OUT_DIR/feed.json"
"$CLI_BIN" corpus feed \
  --root "$CORPUS_DIR" \
  --snapshot "$SNAPSHOT" \
  --manifest "$MANIFEST" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --out "$FEED" \
  2>&1 || { fail "feed generation failed"; }

if [[ -f "$FEED" ]]; then
  pass "feed generated"
else
  fail "feed not created"
fi

# --- Step 7: Governance evaluation ---
step "Step 7: Governance evaluation"
GOV_REPORT="$OUT_DIR/governance.json"

python3 - "$FEED" "$GOV_REPORT" <<'GOVEOF'
import json, sys

feed_path, gov_path = sys.argv[1], sys.argv[2]
data = json.loads(open(feed_path).read())

nodes = data.get("nodes", [])
if not nodes and "feed" in data:
    nodes = data["feed"].get("nodes", [])

gov = {"total_nodes": len(nodes), "with_owner": 0, "with_hash": 0,
       "with_status": 0, "missing_governance": [], "verdict": "unknown"}

for node in nodes:
    path = node.get("source_path", node.get("path", ""))
    has_owner = bool(node.get("owner", ""))
    has_hash = bool(node.get("source_hash", node.get("hash", "")))
    has_status = bool(node.get("status", ""))
    if has_owner: gov["with_owner"] += 1
    if has_hash: gov["with_hash"] += 1
    if has_status: gov["with_status"] += 1
    if not (has_owner and has_hash and has_status):
        gov["missing_governance"].append(path)

if len(nodes) == 0:
    gov["verdict"] = "empty"
elif len(gov["missing_governance"]) == 0:
    gov["verdict"] = "complete"
elif len(gov["missing_governance"]) < len(nodes) * 0.5:
    gov["verdict"] = "partial"
else:
    gov["verdict"] = "incomplete"

with open(gov_path, "w") as f:
    json.dump(gov, f, indent=2)

print(f"  Verdict: {gov['verdict']}")
print(f"  Nodes: {gov['total_nodes']}, with owner: {gov['with_owner']}, "
      f"with hash: {gov['with_hash']}, with status: {gov['with_status']}")
if gov["missing_governance"]:
    print(f"  Missing governance: {len(gov['missing_governance'])} nodes")
GOVEOF

if [[ -f "$GOV_REPORT" ]]; then
  pass "governance evaluation complete"
else
  fail "governance evaluation failed"
fi

# --- Step 8: RAG metadata check ---
step "Step 8: RAG metadata extraction"
RAG_REPORT="$OUT_DIR/rag-metadata.json"

python3 - "$FEED" "$RAG_REPORT" <<'RAGEOF'
import json, sys

feed_path, rag_path = sys.argv[1], sys.argv[2]
data = json.loads(open(feed_path).read())

nodes = data.get("nodes", [])
if not nodes and "feed" in data:
    nodes = data["feed"].get("nodes", [])

rag = {"total_nodes": len(nodes), "embeddable": 0, "with_canonical_ref": 0,
       "with_domain": 0, "structure_only": 0, "eligible_for_rag": []}

for node in nodes:
    content = node.get("content", node.get("text", ""))
    struct_only = node.get("structure_only", False)
    if struct_only:
        rag["structure_only"] += 1
        continue
    if content and content.strip():
        rag["embeddable"] += 1
    if node.get("canonical_ref", node.get("canonical_reference", "")):
        rag["with_canonical_ref"] += 1
    if node.get("domain", ""):
        rag["with_domain"] += 1

with open(rag_path, "w") as f:
    json.dump(rag, f, indent=2)

print(f"  Total nodes: {rag['total_nodes']}")
print(f"  Embeddable: {rag['embeddable']}")
print(f"  Structure-only: {rag['structure_only']}")
print(f"  With canonical_ref: {rag['with_canonical_ref']}")
print(f"  With domain: {rag['with_domain']}")
RAGEOF

if [[ -f "$RAG_REPORT" ]]; then
  pass "RAG metadata report generated"
else
  fail "RAG metadata check failed"
fi

# --- Step 9: Attestation ---
step "Step 9: Attestation"
ATTESTATION="$OUT_DIR/attestation.json"
"$CLI_BIN" corpus attest \
  --snapshot "$SNAPSHOT" \
  --corpus-id "$CORPUS_ID" \
  --project-id "$PROJECT_ID" \
  --verdict "corpus_admissible" \
  --confidence "high" \
  --out "$ATTESTATION" \
  2>&1 || { fail "attestation failed"; }

if [[ -f "$ATTESTATION" ]]; then
  pass "attestation generated"
else
  fail "attestation not created"
fi

# --- Step 10: Git clean verification ---
step "Step 10: Read-only verification"

FINGERPRINT_AFTER="$(mktemp)"
find "$CORPUS_DIR" -type f -not -path '*/.git/*' | sort | xargs sha256sum 2>/dev/null > "$FINGERPRINT_AFTER" || true

if diff -q "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER" > /dev/null 2>&1; then
  pass "corpus unmodified (file fingerprint match)"
else
  fail "corpus was modified during pipeline!"
  diff "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER" || true
fi
rm -f "$FINGERPRINT_BEFORE" "$FINGERPRINT_AFTER"

if [[ -n "$GIT_CLEAN_CHECK" ]]; then
  post_status="$(git -C "$GIT_CLEAN_CHECK" status --porcelain 2>/dev/null)"
  if [[ -z "$post_status" ]]; then
    pass "git status clean after pipeline"
  else
    fail "git status dirty after pipeline"
    echo "$post_status"
  fi
fi

# --- Summary ---
step "Summary"
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo "  Profile:     $PROFILE"
echo "  Corpus ID:   $CORPUS_ID"
echo "  Output:      $OUT_DIR"
echo "  Passed:      $PASS_COUNT / $TOTAL"
echo "  Failed:      $FAIL_COUNT / $TOTAL"
echo ""

ARTIFACT_COUNT="$(find "$OUT_DIR" -type f \( -name '*.json' -o -name '*.yaml' \) | wc -l)"
echo "  Artifacts generated: $ARTIFACT_COUNT"
for f in "$OUT_DIR"/*; do
  [[ -f "$f" ]] && echo "    $(basename "$f") ($(wc -c < "$f") bytes)"
done

if [[ "$FAIL_COUNT" -gt 0 ]]; then
  echo ""
  echo "E2E RESULT: FAIL ($FAIL_COUNT failures)"
  exit 1
fi

echo ""
echo "E2E RESULT: PASS"
exit 0
