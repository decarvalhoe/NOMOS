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

# ---- rag verify: the gate a consumer runs before trusting its index --------

step "rag verify: an index built from A is fresh against re-scan B"
"$NOMOS_BIN" rag verify --manifest "$MANIFEST" --feed "$OUT_DIR/b/feed.json" --strict \
  --output "$OUT_DIR/b/verify.json" \
  || die "rag verify called the index stale on an unchanged corpus"

step "rag verify: the same index is stale against mutated M, exit 1, README only"
set +e
"$NOMOS_BIN" rag verify --manifest "$MANIFEST" --feed "$OUT_DIR/m/feed.json" --strict \
  --output "$OUT_DIR/m/verify.json"
verify_rc=$?
set -e
[[ $verify_rc -eq 1 ]] || die "rag verify must exit 1 on a mutated source, got $verify_rc"
python3 - "$OUT_DIR/m/verify.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1], encoding="utf-8"))
if not d["stale"]:
    sys.exit("verify plan says fresh after a source mutation")
if d["full_reindex"]:
    sys.exit(f"a one-source edit must not force a full reindex: {d['full_reindex_reasons']}")
changed = [s["source_id"] for s in d["sources"] if s["status"] != "unchanged"]
if changed != ["CORPUS-README"]:
    sys.exit(f"expected only CORPUS-README to change, got {changed}")
if not d["chunks"]:
    sys.exit("stale index but an empty plan: nothing to reindex would be reported")
foreign = sorted({c["source_id"] for c in d["chunks"] if c["source_id"] != "CORPUS-README"})
if foreign:
    sys.exit(f"plan touches untouched source(s) {foreign}: invalidation is not per source")
actions = sorted({c["action"] for c in d["chunks"]})
print(f"plan: {len(d['chunks'])} chunk action(s) {actions}, all on CORPUS-README; summary={d['summary']}")
PY

step "rag verify: a hand-edited manifest cannot vouch for an index"
python3 - "$MANIFEST" "$OUT_DIR/a/manifest-tampered.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1], encoding="utf-8"))
# Forge freshness: keep the digest, swap one chunk fingerprint.
m["chunks"][0]["embedding_hash"] = "sha256:" + "0" * 64
json.dump(m, open(sys.argv[2], "w", encoding="utf-8"), indent=2)
PY
set +e
"$NOMOS_BIN" rag verify --manifest "$OUT_DIR/a/manifest-tampered.json" --feed "$OUT_DIR/b/feed.json" \
  --output "$OUT_DIR/a/verify-tampered.json" 2>/dev/null
tamper_rc=$?
set -e
[[ $tamper_rc -eq 1 ]] || die "rag verify accepted a manifest whose digest does not match its chunk list (exit $tamper_rc)"
grep -q '"old_manifest_digest_mismatch"' "$OUT_DIR/a/verify-tampered.json" \
  || die "tampered manifest was not reported as a digest mismatch"

# ---- lens-scoped export on the REAL AEC golden bundle -----------------------
#
# The retrieval-scope promise: a Knowledge Lens is enforced on the corpus
# handed to the index, so a chunk the lens excludes can never be retrieved.
# The verdicts are re-derived here with an INDEPENDENT re-implementation of
# the consumer kit's lens semantics (scripts/nomos_reference_retrieval.py): the
# gate does not let the engine judge the engine.

step "bundle: emit a REAL CKM bundle from the AEC golden corpus (push-free copy)"
GOLDEN_SRC="$ROOT_DIR/cli/internal/corpus/testdata/aec-golden-corpus/vd-lausanne"
PRESETS="$ROOT_DIR/docs/regulated/domain-packs/built-environment/aec-lens-presets"
HARNESS="$ROOT_DIR/docs/regulated/domain-packs/built-environment/retrieval-harness.yaml"
G="$OUT_DIR/golden"
mkdir -p "$G"
cp -R "$GOLDEN_SRC" "$G/corpus"
git -c init.defaultBranch=main -C "$G/corpus" init -q
git -C "$G/corpus" add -A
git -C "$G/corpus" -c user.email=nomos@local -c user.name=nomos commit -qm "rag gate golden snapshot"
"$NOMOS_BIN" bundle --root "$G/corpus" --bundle-id aec-golden-rag-gate --repo example/aec-golden \
  --commit 0123456789abcdef0123456789abcdef01234567 --out "$G/bundle.json" >/dev/null

step "lens-scoped export: LENS-AEC-PERMIS with the pack's document facets, twice, byte-identical"
"$NOMOS_BIN" rag export --bundle "$G/bundle.json" --lens "$PRESETS/permis.lens.yaml" \
  --document-facets "$HARNESS" --strict --output "$G/export-permis-1.jsonl"
"$NOMOS_BIN" rag export --bundle "$G/bundle.json" --lens "$PRESETS/permis.lens.yaml" \
  --document-facets "$HARNESS" --strict --output "$G/export-permis-2.jsonl" 2>/dev/null
cmp -s "$G/export-permis-1.jsonl" "$G/export-permis-2.jsonl" || die "lens-scoped export is not byte-deterministic"
"$NOMOS_BIN" rag manifest --bundle "$G/bundle.json" --lens "$PRESETS/permis.lens.yaml" \
  --document-facets "$HARNESS" --strict --output "$G/manifest-permis.json" 2>/dev/null

step "independent lens check: exported set == consumer-kit verdicts, no confidential leak, contract computed"
python3 -c 'import yaml' 2>/dev/null || die "PyYAML is required for the independent lens check (python3 -m pip install pyyaml)"
python3 - "$G/export-permis-1.jsonl" "$G/manifest-permis.json" "$PRESETS/permis.lens.yaml" "$HARNESS" "$G/bundle.json" <<'PY'
import json, sys, yaml
export, manifest, lens_path, harness_path, bundle_path = sys.argv[1:6]
lens = yaml.safe_load(open(lens_path, encoding="utf-8"))
doc_facets = yaml.safe_load(open(harness_path, encoding="utf-8"))["document_facets"]
bundle = json.load(open(bundle_path, encoding="utf-8"))
nodes = [n for f in bundle["feeds"] for n in f["nodes"]]

def as_list(v):
    return [] if v is None else (v if isinstance(v, list) else [v])
def sel_matches(facets, sel):
    return all(set(as_list(facets.get(a))) & set(as_list(e)) for a, e in sel.items())
def any_matches(facets, sels):
    return any(sel_matches(facets, s) for s in sels)
def lens_includes(facets, lens):
    ex = lens.get("exclude") or {}
    if any_matches(facets, ex.get("any_of") or []):
        return False
    inc = lens.get("include") or {}
    if inc.get("all_of") and not all(sel_matches(facets, s) for s in inc["all_of"]):
        return False
    if inc.get("any_of") and not any_matches(facets, inc["any_of"]):
        return False
    if inc.get("none_of") and any_matches(facets, inc["none_of"]):
        return False
    return True
def enriched(node):
    f = dict(node.get("facets") or {})
    f.update(doc_facets.get(node["source_path"], {}))
    return f

expected_in = {"chunk:" + n["node_id"] for n in nodes if lens_includes(enriched(n), lens)}
expected_out = {"chunk:" + n["node_id"] for n in nodes} - expected_in
exported = {}
with open(export, encoding="utf-8") as fp:
    for line in fp:
        rec = json.loads(line)
        exported[rec["chunk_id"]] = rec
if set(exported) != expected_in:
    sys.exit(f"lens verdicts disagree with the consumer-kit semantics: exported={sorted(exported)} expected={sorted(expected_in)}")
if not expected_out:
    sys.exit("the lens excluded nothing on the golden corpus: this check would prove nothing")
for cid, rec in exported.items():
    if rec["provenance"]["source_path"] == "journal-interne.md":
        sys.exit(f"confidential document leaked through LENS-AEC-PERMIS: {cid}")
    if "aec.permis" not in (rec["metadata"].get("facets") or {}).get("activity", []):
        sys.exit(f"exported record without the permis activity facet: {cid}")
m = json.load(open(manifest, encoding="utf-8"))
if (m.get("lens") or {}).get("id") != lens["id"] or not m["lens"]["digest"].startswith("sha256:"):
    sys.exit(f"manifest does not bind the lens: {m.get('lens')}")
if m["excluded_by_lens_count"] != len(expected_out) or m["chunk_count"] != len(expected_in):
    sys.exit(f"manifest counts drifted: {m['chunk_count']}/{m['excluded_by_lens_count']} vs {len(expected_in)}/{len(expected_out)}")
c = m["retrieval_contract"]
fields = {f["field"]: f["values"] for f in c["filter_fields"]}
if c["scope"] != "lens" or fields.get("facets.activity") != ["aec.permis"] or "journal-interne.md" in fields.get("source_id", []):
    sys.exit(f"retrieval contract drifted: scope={c['scope']} fields={fields}")
if not any(u["capability"] == "temporal_scoping" for u in c["unsupported"]):
    sys.exit("retrieval contract must declare temporal scoping unsupported")
print(f"lens {lens['id']}: {len(expected_in)} in scope, {len(expected_out)} excluded; consumer-kit semantics agree; no confidential leak; contract computed")
PY

step "rag verify: another lens, or no lens, over the same bundle is stale (lens_changed); the same lens is fresh"
set +e
"$NOMOS_BIN" rag verify --manifest "$G/manifest-permis.json" --bundle "$G/bundle.json" \
  --lens "$PRESETS/dt-chantier.lens.yaml" --document-facets "$HARNESS" --output "$G/verify-other-lens.json" 2>/dev/null
other_rc=$?
"$NOMOS_BIN" rag verify --manifest "$G/manifest-permis.json" --bundle "$G/bundle.json" \
  --document-facets "$HARNESS" --output "$G/verify-no-lens.json" 2>/dev/null
none_rc=$?
"$NOMOS_BIN" rag verify --manifest "$G/manifest-permis.json" --bundle "$G/bundle.json" \
  --lens "$PRESETS/permis.lens.yaml" --document-facets "$HARNESS" --output "$G/verify-same-lens.json" 2>/dev/null
same_rc=$?
set -e
[[ $other_rc -eq 1 ]] || die "verify under another lens must exit 1, got $other_rc"
[[ $none_rc -eq 1 ]] || die "verify without the lens must exit 1, got $none_rc"
[[ $same_rc -eq 0 ]] || die "verify under the same lens must exit 0, got $same_rc"
grep -q '"full_reindex_reason": "lens_changed"' "$G/verify-other-lens.json" || die "lens change not reported as lens_changed"
grep -q '"full_reindex_reason": "lens_changed"' "$G/verify-no-lens.json" || die "dropped lens not reported as lens_changed"

echo ""
echo "rag export gate: OK — $CHUNKS chunk(s), $(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["sources"]))' "$MANIFEST") source(s), digest $DIGEST; deterministic, fail-closed, staleness provable per source, verify gates fresh/stale/tampered, lens enforced at the base level on the AEC golden bundle"
