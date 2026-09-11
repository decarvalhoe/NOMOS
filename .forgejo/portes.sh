#!/usr/bin/env bash
# portes.sh — les portes opposables de NOMOS, jouées par .forgejo/workflows/portes.yml
# sur la forge souveraine, et rejouables telles quelles sur un poste :
#
#     bash .forgejo/portes.sh                # toutes les portes, arrêt au premier rouge
#     bash .forgejo/portes.sh --liste        # les noms, sans rien jouer
#
# Pourquoi un script et pas des étapes YAML : du code enfermé dans un YAML n'a
# pas de banc possible (leçon du 2026-08-21, websites) ; ici la même commande
# tourne sur le runner et dans un conteneur local. Le journal est écrit dans
# $PORTES_LOG (défaut : $RUNNER_TEMP/portes.log) pour que le workflow puisse le
# publier en commentaire de PR — l'API de la forge ne rend pas les logs de job.
#
# Ce que ce script ne fait pas : publier, signer, parler à une forge. Il n'a
# besoin d'aucun secret.

set -u

GO_VERSION="${GO_VERSION:-1.26.6}"
GO_SHA256="${GO_SHA256:-708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89}"
# CUE : version de docs/support-model.yaml (toolchain.cue.version) ; cue-lang ne
# publie pas de fichier de sommes, celle-ci a été calculée le 2026-09-11 sur
# l'archive téléchargée depuis la release GitHub et le binaire annonce v0.16.1.
CUE_VERSION="${CUE_VERSION:-v0.16.1}"
CUE_SHA256="${CUE_SHA256:-5d644c1305a2b86504c8dcd2ec829cf5b4999efc2cf51ee375624e0455f774ae}"
GOVULNCHECK_VERSION="${GOVULNCHECK_VERSION:-v1.7.0}"
export CGO_ENABLED="${CGO_ENABLED:-1}"
RUNNER_TEMP="${RUNNER_TEMP:-/tmp}"
PORTES_LOG="${PORTES_LOG:-$RUNNER_TEMP/portes.log}"
mkdir -p "$(dirname "$PORTES_LOG")"
: > "$PORTES_LOG"
# Le toolchain Go s'installe sous $RUNNER_TEMP/go ; le PATH est posé ici, dans
# le processus principal, parce que chaque porte tourne dans un sous-shell.
export PATH="$RUNNER_TEMP/go/bin:$RUNNER_TEMP/cue:$RUNNER_TEMP/gobin:$PATH"
export GOBIN="$RUNNER_TEMP/gobin"

journal() { printf '%s\n' "$*" | tee -a "$PORTES_LOG"; }

# etape <nom> <commande...> : joue la commande, journalise, s'arrête au rouge.
etape() {
  local nom="$1"; shift
  journal "=== $nom"
  local debut; debut=$(date +%s)
  if "$@" >>"$PORTES_LOG" 2>&1; then
    journal "ok     $nom ($(( $(date +%s) - debut )) s)"
  else
    local code=$?
    journal "ROUGE  $nom (code $code, $(( $(date +%s) - debut )) s)"
    journal "--- dernières lignes :"
    tail -n 40 "$PORTES_LOG" | grep -v '^\(===\|ok  \|ROUGE\|---\)' | tail -n 25 | tee -a "$PORTES_LOG" >/dev/null
    journal "*** PORTE ROUGE : $nom"
    exit "$code"
  fi
}

toolchain_go() {
  local archive="$RUNNER_TEMP/go.tgz"
  curl -fsSL --retry 3 -o "$archive" "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
  echo "${GO_SHA256}  ${archive}" | sha256sum -c -
  rm -rf "$RUNNER_TEMP/go"
  tar -C "$RUNNER_TEMP" -xzf "$archive"
  go version
  local declare; declare="$(sed -n 's/^toolchain go//p' cli/go.mod)"
  if [ -n "$declare" ] && [ "$declare" != "$GO_VERSION" ]; then
    echo "cli/go.mod déclare toolchain go${declare}, ce script épingle ${GO_VERSION} : mettre GO_VERSION et GO_SHA256 à jour." >&2
    return 1
  fi
}

deps_python() {
  if python3 -m pip --version >/dev/null 2>&1; then
    python3 -m pip install --quiet --disable-pip-version-check -r scripts/requirements-sidecar.txt \
      || python3 -m pip install --quiet --disable-pip-version-check --break-system-packages -r scripts/requirements-sidecar.txt
  elif command -v apt-get >/dev/null 2>&1; then
    echo "pip absent : paquets système python3-yaml et python3-jsonschema (versions non épinglées, signalé)."
    apt-get update -qq && apt-get install -y -qq python3-yaml python3-jsonschema >/dev/null
  else
    echo "Ni pip ni apt-get : impossible d'installer PyYAML et jsonschema." >&2
    return 1
  fi
  python3 -c 'import sys, yaml, jsonschema; print(sys.version.split()[0], "PyYAML", yaml.__version__, "jsonschema", jsonschema.__version__)'
}

toolchain_cue() {
  local archive="$RUNNER_TEMP/cue.tgz"
  curl -fsSL --retry 3 -o "$archive" "https://github.com/cue-lang/cue/releases/download/${CUE_VERSION}/cue_${CUE_VERSION}_linux_amd64.tar.gz"
  echo "${CUE_SHA256}  ${archive}" | sha256sum -c -
  rm -rf "$RUNNER_TEMP/cue"; mkdir -p "$RUNNER_TEMP/cue"
  tar -C "$RUNNER_TEMP/cue" -xzf "$archive" cue
  cue version | head -1
  local declare; declare="$(sed -n 's/^    version: \(v[0-9.]*\)$/\1/p' docs/support-model.yaml | head -1)"
  if [ -n "$declare" ] && [ "$declare" != "$CUE_VERSION" ]; then
    echo "docs/support-model.yaml déclare CUE ${declare}, ce script épingle ${CUE_VERSION} : mettre CUE_VERSION et CUE_SHA256 à jour." >&2
    return 1
  fi
}

scanners() {
  go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
  if ! command -v pip-audit >/dev/null 2>&1; then
    python3 -m pip install --quiet --disable-pip-version-check pip-audit \
      || python3 -m pip install --quiet --disable-pip-version-check --break-system-packages pip-audit
  fi
  govulncheck -version | head -1; pip-audit --version
}

# Les mêmes commandes que le job cue-vet de ci.yml : contrats valides, exemples
# invalides refusés (un contrat qui accepte son contre-exemple est rouge).
refuse() { if cue vet "$@" >/dev/null 2>&1; then echo "ATTENDU ROUGE mais accepté : cue vet $*" >&2; return 1; fi; }
cue_vet() {
  cue vet specs/*.cue
  for ok in gxp ai legal; do cue vet specs/nomos-domain-profile.cue "specs/examples/nomos-domain-profile.${ok}.valid.yaml" -d '#DomainProfile'; done
  refuse specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.unsupported-claim.invalid.yaml -d '#DomainProfile'
  cue vet specs/nomos-praxis-evidence.cue specs/examples/nomos-praxis-evidence.valid.yaml -d '#PraxisEvidenceExchange'
  for bad in specs/examples/nomos-praxis-evidence.invalid-*.yaml; do refuse specs/nomos-praxis-evidence.cue "$bad" -d '#PraxisEvidenceExchange'; done
  cue vet specs/nomos-praxis-evidence.cue specs/examples/nomos-praxis-mapping.valid.json -d '#PraxisAtomMapping'
  for bad in specs/examples/nomos-praxis-mapping.invalid-*.json; do refuse specs/nomos-praxis-evidence.cue "$bad" -d '#PraxisAtomMapping'; done
  cue vet specs/portfolio-status.cue specs/examples/portfolio-status.valid.json -d '#PortfolioStatus'
  for bad in specs/examples/portfolio-status.invalid-*.json; do refuse specs/portfolio-status.cue "$bad" -d '#PortfolioStatus'; done
  cue vet specs/domain-cartography.cue specs/examples/domain-cartography.valid.yaml -d '#DomainCartography'
  cue vet specs/domain-cartography.cue docs/cross-consumption-kit/cartography.yaml -d '#DomainCartography'
  for bad in specs/examples/domain-cartography.invalid-*.yaml; do refuse specs/domain-cartography.cue "$bad" -d '#DomainCartography'; done
  cue vet specs/domain-pack.cue docs/regulated/domain-packs/built-environment/pack.yaml -d '#DomainPack'
  cue vet specs/domain-pack.cue docs/regulated/domain-packs/built-environment/aec-vocabulary.yaml -d '#PackVocabulary'
  refuse specs/domain-pack.cue specs/examples/domain-pack.code-artifact.invalid.yaml -d '#DomainPack'
  refuse specs/domain-pack.cue specs/examples/domain-pack.mechanics-field.invalid.yaml -d '#DomainPack'
}

securite() {
  python3 scripts/security_process_gate.py --root . --check --scan govulncheck,pip-audit --report "$RUNNER_TEMP/security-gate.json"
  python3 -m unittest tests.test_security_process_gate
}

go_cli()        { (cd cli && go vet ./... && go test ./...); }
go_sigstore()   { (cd tools/sigstore-verifier && go test ./...); }
matrice()       { python3 scripts/vrc_wiring_matrix.py --root . && git diff --exit-code .vrc-wiring-matrix; }
roadmap()       {
  python3 scripts/roadmap_lane_guard.py --root . \
  && python3 scripts/roadmap_lane_guard.py --root . --emit-docs \
  && git diff --exit-code docs/15-product-backlog.md docs/29-post-alpha-release-issue-list.md docs/47-roadmap-lanes-and-risk-based-validation.md
}

PORTES=(
  "Toolchain Go ${GO_VERSION} (go.dev, somme vérifiée)|toolchain_go"
  "Dépendances Python des sidecars (épinglées)|deps_python"
  "Toolchain CUE ${CUE_VERSION} (release cue-lang, somme épinglée)|toolchain_cue"
  "Scanners de sécurité — govulncheck ${GOVULNCHECK_VERSION}, pip-audit|scanners"
  "Contrats CUE — valides acceptés, contre-exemples refusés|cue_vet"
  "Porte de sécurité — processus, allowlist, versions, govulncheck, pip-audit (NRT-025) et ses preuves|securite"
  "Moteur Go — vet et tests (cli)|go_cli"
  "Vérificateur Sigstore — tests du module externe|go_sigstore"
  "Matrice de câblage — calculée, committée sans dérive|matrice"
  "Compteurs de capacités des README — recopiés depuis la matrice|python3 scripts/readme_capability_counts_guard.py --root . --check"
  "Claim boundary — aucune capacité d'attestation surdéclarée|python3 scripts/claim_boundary_guard.py --root ."
  "Modèle de support — déclaré et recoupé|python3 scripts/support_model_guard.py --root . --check"
  "Ledger d'evidence — index en vigueur, sans dérive ni péremption|python3 scripts/evidence_ledger_guard.py --root . --check"
  "Registre de roadmap — cohérent, tables de files sans dérive|roadmap"
  "Sidecars Python — suite unittest (Go présent)|python3 -m unittest discover -s tests"
)

if [ "${1:-}" = "--liste" ]; then
  for p in "${PORTES[@]}"; do printf '%s\n' "${p%%|*}"; done
  exit 0
fi

journal "portes.sh — $(date -u +%Y-%m-%dT%H:%M:%SZ) — révision $(git rev-parse --short HEAD 2>/dev/null || echo '?')"
# Chaque porte tourne dans un sous-shell du script lui-même (les fonctions
# ci-dessus y sont visibles), avec `set -e` pour que la première commande
# rouge arrête la porte. Un `bash -c` séparé ne les verrait pas : c'est
# exactement le rouge « command not found » du premier run sur la forge.
porte() { ( set -e; eval "$1" ); }
for p in "${PORTES[@]}"; do
  nom="${p%%|*}"; cmd="${p#*|}"
  etape "$nom" porte "$cmd"
done
journal "*** TOUTES LES PORTES SONT VERTES"
