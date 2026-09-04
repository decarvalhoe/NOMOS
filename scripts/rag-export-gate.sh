#!/usr/bin/env bash
# RAG interop export gate (#614).
#
# Replays the `nomos rag` contract on the REAL in-repo public reference corpus
# (the same documents scripts/process_public_bibles.py processes) and turns red
# when any of its load-bearing properties stops holding:
#
#   1. byte-determinism — `rag export` (every format) and `rag manifest` emit
#      identical bytes across two runs on the same feed; the index digest is
#      identical across a full re-scan of the same corpus;
#   2. fail-closed — `--strict` must pass with ZERO rejected chunks, the export
#      must not be empty, and the export line count must equal the manifest's
#      chunk_count (nothing silently dropped or duplicated);
#   3. citation safety — the structural context prefix is present in
#      embedding_text and NEVER leaks into body_text; every record carries a
#      chunk_id, source_id and source_hash;
#   4. provable staleness — a 1-byte mutation of ONE source moves the index
#      digest and that source's per-source digest, while the untouched source
#      keeps its digest (invalidation is per source, not whole-index).
#
# The corpus is snapshotted into a push-free git checkout; the live repo is
# never scanned (read-only discipline, same as process_public_bibles.py).
#
# Claim boundary: this proves the export contract (determinism, provenance
# binding, staleness detection). It measures nothing about retrieval quality;
# NOMOS does not embed, retrieve, or rerank.
#
# Usage:
#   bash scripts/rag-export-gate.sh [--nomos-bin <path>] [--out-dir <dir>]
#
# RAG_GATE_MIN_CHUNKS (default 1) is the adversarial knob: set it above the
# corpus size to prove the gate can go red.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NOMOS_BIN=""
OUT_DIR=""
MIN_CHUNKS="${RAG_GATE_MIN_CHUNKS:-1}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --nomos-bin) NOMOS_BIN="$2"; shift 2 ;;
    --out-dir) OUT_DIR="$2"; shift 2 ;;
    -h|--help) sed -n '2,32p' "${BASH_SOURCE[0]}"; exit 0 ;;
    *) echo "rag export gate: unknown argument: $1" >&2; exit 2 ;;
  esac
done

OUT_DIR="${OUT_DIR:-$(mktemp -d)}"
mkdir -p "$OUT_DIR"

step() { echo ""; echo "=== $1 ==="; }
die()  { echo "rag export gate: FAIL — $*" >&2; exit 1; }

# The corpus: the in-repo public reference-basis documents (RCP-010).
CORPUS_DOCS=(
  "$ROOT_DIR/docs/regulated/reference-basis/README.md"
  "$ROOT_DIR/docs/regulated/reference-basis/nomos-bible-corpus-policy.md"
)

if [[ -z "$NOMOS_BIN" ]]; then
  step "build nomos"
  NOMOS_BIN="$OUT_DIR/nomos"
  (cd "$ROOT_DIR/cli" && go build -o "$NOMOS_BIN" .)
fi
[[ -x "$NOMOS_BIN" ]] || die "nomos binary not executable: $NOMOS_BIN"

# snapshot_corpus <dir>: copy the docs into a fresh git checkout with NO remote,
# so the CLI's push-capable-remote refusal can never target the live repo.
snapshot_corpus() {
  local corpus="$1"
  mkdir -p "$corpus"
  local doc
  for doc in "${CORPUS_DOCS[@]}"; do
    [[ -f "$doc" ]] || die "corpus document missing: $doc"
    cp "$doc" "$corpus/"
  done
  git -c init.defaultBranch=main -C "$corpus" init -q
  git -C "$corpus" add -A
  git -C "$corpus" -c user.email=nomos@local -c user.name=nomos commit -qm "rag gate snapshot"
}

# build_feed <corpus> <workdir>: scan -> manifest -> feed, writes <workdir>/feed.json
build_feed() {
  local corpus="$1" work="$2"
  mkdir -p "$work"
  "$NOMOS_BIN" corpus scan --root "$corpus" --out "$work/snapshot.json" >/dev/null
  "$NOMOS_BIN" corpus manifest --snapshot "$work/snapshot.json" \
    --out "$work/source-manifest.yaml" --domain public-reference-basis >/dev/null
  "$NOMOS_BIN" corpus feed --root "$corpus" --snapshot "$work/snapshot.json" \
    --manifest "$work/source-manifest.yaml" --out "$work/feed.json" >/dev/null
}

# json_get <file> <key>: top-level scalar field
json_get() {
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$1" "$2"
}

# ---- run A: the corpus as committed --------------------------------------

step "snapshot corpus (push-free checkout)"
CORPUS="$OUT_DIR/corpus"
snapshot_corpus "$CORPUS"

step "feed A"
build_feed "$CORPUS" "$OUT_DIR/a"

step "rag export --strict, every format, twice: byte-identical"
for fmt in jsonl langchain llamaindex; do
  "$NOMOS_BIN" rag export --feed "$OUT_DIR/a/feed.json" --format "$fmt" --strict \
    --output "$OUT_DIR/a/export-$fmt-1.jsonl"
  "$NOMOS_BIN" rag export --feed "$OUT_DIR/a/feed.json" --format "$fmt" --strict \
    --output "$OUT_DIR/a/export-$fmt-2.jsonl" 2>/dev/null
  cmp -s "$OUT_DIR/a/export-$fmt-1.jsonl" "$OUT_DIR/a/export-$fmt-2.jsonl" \
    || die "rag export --format $fmt is not byte-deterministic across two runs"
done
EXPORT="$OUT_DIR/a/export-jsonl-1.jsonl"

step "rag manifest --strict, twice: byte-identical"
"$NOMOS_BIN" rag manifest --feed "$OUT_DIR/a/feed.json" --strict --output "$OUT_DIR/a/manifest-1.json"
"$NOMOS_BIN" rag manifest --feed "$OUT_DIR/a/feed.json" --strict --output "$OUT_DIR/a/manifest-2.json" 2>/dev/null
cmp -s "$OUT_DIR/a/manifest-1.json" "$OUT_DIR/a/manifest-2.json" \
  || die "rag manifest is not byte-deterministic across two runs"
MANIFEST="$OUT_DIR/a/manifest-1.json"

step "fail-closed counts"
CHUNKS="$(json_get "$MANIFEST" chunk_count)"
REJECTED="$(json_get "$MANIFEST" rejected_count)"
DIGEST="$(json_get "$MANIFEST" chunk_digest)"
LINES="$(wc -l < "$EXPORT" | tr -d ' ')"
echo "chunks=$CHUNKS rejected=$REJECTED lines=$LINES digest=$DIGEST"
[[ "$REJECTED" == "0" ]] || die "$REJECTED chunk(s) rejected on the reference corpus (see stderr above)"
[[ "$CHUNKS" -ge "$MIN_CHUNKS" ]] || die "exported $CHUNKS chunk(s), expected at least $MIN_CHUNKS"
[[ "$LINES" == "$CHUNKS" ]] || die "export has $LINES line(s) but the manifest counts $CHUNKS chunk(s)"

step "citation safety on every exported record"
python3 - "$EXPORT" <<'PY'
import json, sys
path = sys.argv[1]
seen = set()
prefixed = 0
with open(path, encoding="utf-8") as fp:
    for lineno, line in enumerate(fp, start=1):
        rec = json.loads(line)
        cid, body, emb = rec["chunk_id"], rec["body_text"], rec["embedding_text"]
        prefix = rec.get("context_prefix", "")
        prov = rec["provenance"]
        if not cid or not prov.get("source_id") or not prov.get("source_hash"):
            sys.exit(f"line {lineno}: record without chunk_id/source_id/source_hash exported: {cid!r}")
        if cid in seen:
            sys.exit(f"line {lineno}: duplicate chunk_id exported: {cid}")
        seen.add(cid)
        if not body.strip():
            sys.exit(f"line {lineno}: empty body_text exported for {cid}")
        if prefix:
            prefixed += 1
            if emb != prefix + "\n\n" + body:
                sys.exit(f"line {lineno}: embedding_text is not <prefix>\\n\\n<body> for {cid}")
            if body.startswith(prefix):
                sys.exit(f"line {lineno}: context prefix leaked into the citable body of {cid}")
        elif emb != body:
            sys.exit(f"line {lineno}: no prefix but embedding_text != body_text for {cid}")
if prefixed == 0:
    sys.exit("no record carries a structural context prefix: contextualisation is not happening")
print(f"records={len(seen)} with_context_prefix={prefixed}: no prefix leak, no missing provenance")
PY

# ---- run B: full re-scan of the same corpus -------------------------------

step "feed B (full re-scan): index digest identical"
build_feed "$CORPUS" "$OUT_DIR/b"
"$NOMOS_BIN" rag manifest --feed "$OUT_DIR/b/feed.json" --strict --output "$OUT_DIR/b/manifest.json" 2>/dev/null
DIGEST_B="$(json_get "$OUT_DIR/b/manifest.json" chunk_digest)"
[[ "$DIGEST_B" == "$DIGEST" ]] || die "index digest moved across a re-scan of an unchanged corpus: $DIGEST -> $DIGEST_B"

# ---- run M: one source mutated by one line --------------------------------

step "staleness: 1-byte source mutation moves that source's digest only"
MUT="$OUT_DIR/corpus-mutated"
snapshot_corpus "$MUT"
printf '\nLigne ajoutee par le gate pour prouver la detection de staleness.\n' >> "$MUT/README.md"
git -C "$MUT" -c user.email=nomos@local -c user.name=nomos commit -qam "rag gate mutation"
build_feed "$MUT" "$OUT_DIR/m"
"$NOMOS_BIN" rag manifest --feed "$OUT_DIR/m/feed.json" --strict --output "$OUT_DIR/m/manifest.json" 2>/dev/null
python3 - "$MANIFEST" "$OUT_DIR/m/manifest.json" <<'PY'
import json, sys
before = json.load(open(sys.argv[1], encoding="utf-8"))
after = json.load(open(sys.argv[2], encoding="utf-8"))
if before["chunk_digest"] == after["chunk_digest"]:
    sys.exit("index digest did NOT move after a source mutation: staleness is not detectable")
b = {s["source_id"]: s for s in before["sources"]}
a = {s["source_id"]: s for s in after["sources"]}
if set(b) != set(a):
    sys.exit(f"source set changed under a content-only mutation: {sorted(b)} vs {sorted(a)}")
moved, kept = [], []
for sid in sorted(b):
    hash_moved = b[sid]["source_hash"] != a[sid]["source_hash"]
    digest_moved = b[sid]["chunk_digest"] != a[sid]["chunk_digest"]
    if hash_moved and not digest_moved:
        sys.exit(f"{sid}: source_hash moved but its chunk digest did not: stale chunks would pass as fresh")
    if digest_moved and not hash_moved:
        sys.exit(f"{sid}: chunk digest moved although the source did not: invalidation is not per source")
    (moved if hash_moved else kept).append(sid)
if len(moved) != 1:
    sys.exit(f"expected exactly one mutated source, got {moved}")
if not kept:
    sys.exit("no untouched source left to prove per-source isolation")
print(f"mutated={moved[0]} (digest moved) untouched={kept} (digest kept)")
PY

echo ""
echo "rag export gate: OK — $CHUNKS chunk(s), $(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["sources"]))' "$MANIFEST") source(s), digest $DIGEST; deterministic, fail-closed, staleness provable per source"
