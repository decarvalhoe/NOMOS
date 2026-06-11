#!/usr/bin/env bash
# CKM-00 non-regression harness.
#
# This is the pivot guardrail: CKM changes must keep the existing CLI, CUE
# contracts, Python workflow tests, e2e harness, and RBOK integrations green.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${CKM_OUT_DIR:-$(mktemp -d)}"
CLI_BIN="$OUT_DIR/nomos"

[ -f "$ROOT_DIR/scripts/nomos-env.sh" ] && source "$ROOT_DIR/scripts/nomos-env.sh"

step() {
  echo ""
  echo "=== $1 ==="
}

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "FATAL: $1 not found in PATH" >&2
    exit 1
  fi
}

write_runtime_fixture() {
  local corpus="$1"
  mkdir -p "$corpus/00_meta" "$corpus/01_rbok/referentiel" "$corpus/02_parcours/habitation" "$corpus/03_workbooks" "$corpus/98_schemas"
  cat > "$corpus/00_meta/glossaire.yaml" <<'YAML'
glossaire:
  - terme: Garantie habitation
    definition: Une garantie habitation formalise une promesse contractuelle vérifiable.
YAML
  cat > "$corpus/01_rbok/referentiel/garanties.md" <<'MD'
# Garanties habitation

Une garantie habitation décrit les conditions métier applicables à une couverture client.
MD
  cat > "$corpus/02_parcours/habitation/souscription.yaml" <<'YAML'
parcours:
  id: souscription
  description: Le parcours de souscription collecte les informations nécessaires à la qualification métier.
YAML
  printf '{"generated": true}\n' > "$corpus/03_workbooks/report.json"
  printf '{"type": "object"}\n' > "$corpus/98_schemas/schema.json"
}

write_lawbook_fixture() {
  local corpus="$1"
  mkdir -p "$corpus/00_meta" "$corpus/01_referentiel" "$corpus/02_domaines/habitation" "$corpus/99_RBOK_initial_pdf"
  cat > "$corpus/00_meta/glossary.yaml" <<'YAML'
terms:
  - id: T1
    label: Garantie habitation
YAML
  cat > "$corpus/01_referentiel/garanties.md" <<'MD'
# Garanties habitation

## Définition

Une garantie habitation formalise une promesse contractuelle vérifiable.

## Règle

Toute réponse doit citer la source applicable ou s'abstenir.
MD
  cat > "$corpus/02_domaines/habitation/garanties.yaml" <<'YAML'
garanties:
  - water_damage
YAML
  printf '%%PDF-1.4 original\n' > "$corpus/99_RBOK_initial_pdf/contract.pdf"
}

step "1/11 - Toolchain"
require_tool go
require_tool cue
require_tool python3
echo "go: $(go version)"
echo "cue: $(cue version | head -1)"
echo "python3: $(python3 --version)"

step "2/11 - CLI build"
cd "$ROOT_DIR/cli"
go build -o ../nomos .
go build -o "$CLI_BIN" .

step "3/11 - Go tests"
go vet ./...
go test ./...

for mod in "$ROOT_DIR"/control-plane/*/go.mod; do
  [ -f "$mod" ] || continue
  dir="$(dirname "$mod")"
  echo "control-plane/$(basename "$dir")"
  (cd "$dir" && go vet ./... && go test ./...)
done

step "4/11 - Python workflow tests"
cd "$ROOT_DIR"
python3 -m unittest discover -s tests -v

step "5/11 - CUE schemas and existing domain profiles"
cue vet specs/*.cue
cue vet specs/atomization-spine.cue specs/point-in-time.cue specs/examples/point-in-time-atoms.valid.yaml -d '#TemporalAtomSet'
python3 scripts/ckm_point_in_time_resolve.py \
  --atoms specs/examples/point-in-time-atoms.valid.yaml \
  --work-id eli:example:regulation:demo \
  --as-of 2024-03-01 >/dev/null
cue vet specs/atomization-spine.cue specs/facets.cue specs/examples/facets.atom.valid.yaml -d '#FacetedAtom'
cue vet specs/atomization-spine.cue specs/facets.cue specs/examples/facets.chunk.valid.yaml -d '#FacetedChunk'
cue vet specs/atomization-spine.cue specs/facets.cue specs/examples/facets.business-metier.valid.yaml -d '#FacetedAtom'
cue vet specs/atomization-spine.cue specs/facets.cue specs/knowledge-lens.cue specs/examples/knowledge-lens.valid.yaml -d '#KnowledgeLensBundle'
python3 scripts/ckm_knowledge_lens_filter.py \
  --candidates specs/examples/knowledge-lens-candidates.valid.yaml \
  --lens specs/examples/knowledge-lens.valid.yaml \
  --preset architect-permit-review >/dev/null
cue vet specs/atomization-spine.cue specs/facets.cue specs/canon-promotion.cue specs/examples/canon-promotion.valid.yaml -d '#CanonPromotionBundle'
python3 scripts/ckm_canon_promotion_validate.py --bundle specs/examples/canon-promotion.valid.yaml >/dev/null
if python3 scripts/ckm_canon_promotion_validate.py --bundle specs/examples/canon-promotion.invalid-shared.yaml >/dev/null 2>&1; then
  echo "FAIL: invalid CKM canon promotion fixture passed" >&2
  exit 1
fi
if cue vet specs/atomization-spine.cue specs/facets.cue specs/examples/facets.invalid-trust-tier.yaml -d '#FacetedAtom'; then
  echo "FAIL: invalid CKM facet trust tier passed" >&2
  exit 1
fi
cue vet specs/facet-ontology.cue specs/examples/facet-ontology.valid.yaml -d '#FacetOntology'
python3 scripts/ckm_facet_ontology_validate.py --ontology specs/examples/facet-ontology.valid.yaml >/dev/null
if python3 scripts/ckm_facet_ontology_validate.py --ontology specs/examples/facet-ontology.invalid-overlap.yaml >/dev/null 2>&1; then
  echo "FAIL: invalid CKM facet ontology disjoint overlap passed" >&2
  exit 1
fi
cue vet specs/atomization-spine.cue specs/facets.cue specs/nomos-trace-manifest.cue attestations/nomos-attestation.cue specs/canonical-knowledge-bundle.cue specs/examples/canonical-knowledge-bundle.valid.json -d '#CanonicalKnowledgeBundle'
python3 scripts/ckm_bundle_validate.py --bundle specs/examples/canonical-knowledge-bundle.valid.json

# SEAM-2 (#535): the facet-vocab gate must REJECT an out-of-vocabulary value.
if python3 - <<'PY'
import json, subprocess, sys, tempfile
b = json.load(open("specs/examples/canonical-knowledge-bundle.emitted.json"))
b["feeds"][0]["nodes"][0]["facets"]["trust_tier"] = "trust-me-bro"
fp = tempfile.NamedTemporaryFile("w", suffix=".json", delete=False)
json.dump(b, fp); fp.close()
sys.exit(subprocess.run([sys.executable, "scripts/ckm_bundle_validate.py", "--bundle", fp.name]).returncode)
PY
then
  echo "FAIL: out-of-vocabulary facet value passed the bundle validator" >&2
  exit 1
fi

# SEAM-3 (#536): the real emitter must produce a contract+vocab-valid bundle, and
# the committed canonical fixture must validate. Corpus is copied OUTSIDE any git
# checkout (the emitter refuses push-capable source roots).
seam_corpus="$(mktemp -d)/golden"
cp -r cli/internal/corpus/testdata/portable-golden-corpus "$seam_corpus"
seam_bundle="$(mktemp -d)/seam-emitted.json"
"$CLI_BIN" bundle --root "$seam_corpus" --bundle-id nomos-canonical-golden \
  --domain built-environment --country CH --canton VD --commune Lausanne \
  --repo RBOKproject/Nomos --commit "$(git rev-parse HEAD)" --branch ci --out "$seam_bundle"
cue vet specs/atomization-spine.cue specs/facets.cue specs/nomos-trace-manifest.cue attestations/nomos-attestation.cue specs/canonical-knowledge-bundle.cue "$seam_bundle" -d '#CanonicalKnowledgeBundle'
python3 scripts/ckm_bundle_validate.py --bundle "$seam_bundle"
python3 scripts/ckm_bundle_validate.py --bundle specs/examples/canonical-knowledge-bundle.emitted.json
domain_profiles=(
  specs/examples/nomos-domain-profile.gxp.valid.yaml
  specs/examples/nomos-domain-profile.ai.valid.yaml
  specs/examples/nomos-domain-profile.legal.valid.yaml
  specs/examples/nomos-domain-profile.finance.valid.yaml
  specs/examples/nomos-domain-profile.high-assurance.valid.yaml
  specs/examples/nomos-domain-profile.medical-samd.valid.yaml
  specs/examples/nomos-domain-profile.six-sigma-capa.valid.yaml
  specs/examples/nomos-domain-profile.cyber-supplier-assurance.valid.yaml
  specs/examples/nomos-domain-profile.verifiable-evidence.valid.yaml
  specs/examples/nomos-domain-profile.business-operations.valid.yaml
)
for profile in "${domain_profiles[@]}"; do
  cue vet specs/nomos-domain-profile.cue "$profile" -d '#DomainProfile'
done
if cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.unsupported-claim.invalid.yaml -d '#DomainProfile'; then
  echo "FAIL: unsupported domain compliance claim passed" >&2
  exit 1
fi

step "6/11 - CKM signed claim-boundary predicate gate"
cue vet attestations/nomos-attestation.cue docs/regulated/claim-boundary/ckm-refused-claims.json -d '#InTotoStatement'

step "6b/11 - Attestation claim-boundary guard (no 'signed'/'Sigstore'/'certified' without proof)"
python3 scripts/claim_boundary_guard.py --root .

step "7/11 - CKM additive metadata guard"
# metadata remains open for CKM additive fields until an explicit schema_version bump + migration.
python3 - <<'PY'
from pathlib import Path

text = Path("specs/atomization-spine.cue").read_text()
for block in ("#Atom:", "#Chunk:"):
    start = text.index(block)
    next_block = text.find("\n// #", start + 1)
    segment = text[start: next_block if next_block != -1 else len(text)]
    if "metadata?:    {...}" not in segment and "metadata?:        {...}" not in segment:
        raise SystemExit(f"{block} no longer keeps open metadata for additive CKM fields")
print("metadata remains open for CKM additive fields")
PY

step "8/11 - CKM cite-or-abstain metrics gate"
python3 scripts/regulated_rag_answer_evidence.py \
  --output "$OUT_DIR/rag-answer-evidence.json"

step "9/11 - Baseline e2e"
bash scripts/e2e.sh

step "10/11 - RBOK runtime E2E fixture"
runtime_corpus="$OUT_DIR/rbok-runtime-corpus"
write_runtime_fixture "$runtime_corpus"
bash scripts/rbok-runtime-e2e.sh \
  --corpus "$runtime_corpus" \
  --out "$OUT_DIR/rbok-runtime-output" \
  --cli "$CLI_BIN" \
  --corpus-id realisons-business \
  --project-id rbok

step "11/11 - RBOK lawbook E2E fixture"
lawbook_corpus="${CKM_RBOK_LAWBOOK_CORPUS:-}"
if [ -z "$lawbook_corpus" ]; then
  lawbook_corpus="$OUT_DIR/rbok-lawbook-corpus"
  write_lawbook_fixture "$lawbook_corpus"
fi
bash scripts/rbok-lawbook-e2e.sh \
  --corpus "$lawbook_corpus" \
  --out "$OUT_DIR/rbok-lawbook-output" \
  --cli "$CLI_BIN" \
  --profile rbok-lawbook \
  --corpus-id rbok-lawbook \
  --project-id rbok

echo ""
echo "CKM non-regression harness passed."
