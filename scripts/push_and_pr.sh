#!/usr/bin/env bash
# Push all agent branches to GitHub and create PRs
# Requires: push access on RBOKproject/Nomos
set -euo pipefail

REPO="RBOKproject/Nomos"
BASE="codex/nomos-product-bootstrap"
BARE="/tmp/Nomos"

cd "$BARE"

# Ensure remote is set correctly
git remote set-url origin "github-claude:${REPO}.git" 2>/dev/null || true

declare -A AGENT_BRANCHES=(
  [claude]="agents/claude"
  [codex]="agents/codex"
  [copilot]="agents/copilot"
  [cursor]="agents/cursor"
  [gemini]="agents/gemini"
)

declare -A AGENT_TITLES=(
  [claude]="feat: CLI checks, strict mode, exceptions, product-check, dashboard (NOM-203,501,504,601,602,703,802,903)"
  [codex]="feat: adapters Node/Python, report, remediation, SBOM exports (NOM-402,403,503,702,804)"
  [copilot]="feat: matrix check, partial mode, CI integration, storage (NOM-502,701,801,902)"
  [cursor]="feat: init, admit, attestations (NOM-202,303,803)"
  [gemini]="feat: diagnose, output renderers, JVM adapter, registry (NOM-302,204,404,901)"
)

declare -A AGENT_BODIES=(
  [claude]="## Summary
- NOM-203: Manifest validation (\`nomos validate\`)
- NOM-501: Sources check (\`nomos sources check\`)
- NOM-504: Contracts check (\`nomos contracts check\`)
- NOM-601: Product check (\`nomos product-check\`)
- NOM-602: Strict mode (\`nomos strict\`)
- NOM-703: Characterization test & strangler templates
- NOM-802: Expiring exceptions
- NOM-903: Portfolio dashboard

## Test plan
- [x] \`go vet ./...\` passes
- [x] \`go test ./...\` — 8 packages, all green
- [x] \`cue vet specs/*.cue\` passes
- [x] Zero uncommitted files"

  [codex]="## Summary
- NOM-402: Node/TypeScript adapter with detection
- NOM-403: Python adapter (FastAPI/Django/Flask)
- NOM-503: Report generation (\`nomos report\`)
- NOM-702: Remediation backlog generator
- NOM-804: SPDX 2.3 & CycloneDX 1.5 exports

## Test plan
- [x] \`go vet ./...\` passes
- [x] \`go test ./...\` — 5 packages, all green
- [x] \`cue vet specs/*.cue\` passes
- [x] Zero uncommitted files"

  [copilot]="## Summary
- NOM-502: Matrix check (\`nomos matrix check\`)
- NOM-701: Partial mode evaluation
- NOM-801: GitHub Action & GitLab CI reusable jobs
- NOM-902: Report & attestation storage

## Test plan
- [x] \`go vet ./...\` passes
- [x] \`go test ./...\` — 4 packages, all green
- [x] \`cue vet specs/*.cue\` passes
- [x] Zero uncommitted files"

  [cursor]="## Summary
- NOM-202: \`nomos init\` implementation
- NOM-303: \`nomos admit\` with confidence levels
- NOM-803: Attestation generation & verification (in-toto/SLSA/cosign)

## Test plan
- [x] \`go vet ./...\` passes
- [x] \`go test ./...\` — 4 packages, all green
- [x] \`cue vet specs/*.cue\` passes
- [x] Zero uncommitted files"

  [gemini]="## Summary
- NOM-302: \`nomos diagnose\` implementation
- NOM-204: Standard output renderers (JSON/Markdown)
- NOM-404: JVM/Spring adapter with fixtures
- NOM-901: Project registry with execution tracking

## Test plan
- [x] \`go vet ./...\` passes
- [x] \`go test ./...\` — 4 packages, all green
- [x] \`cue vet specs/*.cue\` passes
- [x] Zero uncommitted files"
)

echo "=== Pushing branches ==="
for agent in claude codex copilot cursor gemini; do
  branch="${AGENT_BRANCHES[$agent]}"
  echo "Pushing $branch..."
  git push origin "$branch:$branch" 2>&1 || echo "FAILED to push $branch"
done

echo ""
echo "=== Creating PRs ==="
for agent in claude codex copilot cursor gemini; do
  branch="${AGENT_BRANCHES[$agent]}"
  title="${AGENT_TITLES[$agent]}"
  body="${AGENT_BODIES[$agent]}"

  echo "Creating PR for $agent ($branch)..."
  GH_CONFIG_DIR="/root/.config/gh-$agent" gh pr create \
    --repo "$REPO" \
    --base "$BASE" \
    --head "$branch" \
    --title "$title" \
    --body "$(cat <<EOF
$body

---
Generated with [Claude Code](https://claude.com/claude-code)
EOF
)" 2>&1 || echo "FAILED to create PR for $agent"
  echo ""
done

echo "=== Done ==="
