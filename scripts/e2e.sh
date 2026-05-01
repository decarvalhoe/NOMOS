#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Source local toolchain if available.
[ -f "$ROOT_DIR/scripts/nomos-env.sh" ] && source "$ROOT_DIR/scripts/nomos-env.sh"

# 1. Verify toolchain
echo "=== 1/3 — Verify toolchain ==="
for cmd in go cue; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "FATAL: $cmd not found in PATH" >&2
    exit 1
  fi
  echo "  $cmd: $("$cmd" version 2>&1 | head -1)"
done

# 2. Go vet + test
echo ""
echo "=== 2/3 — Go vet & test (cli/) ==="
cd "$ROOT_DIR/cli"
go vet ./...
go test ./...

# Also test control-plane modules if present.
for mod in "$ROOT_DIR"/control-plane/*/go.mod; do
  [ -f "$mod" ] || continue
  dir="$(dirname "$mod")"
  echo "  $(basename "$dir")/"
  (cd "$dir" && go vet ./... && go test ./...)
done

# 3. CUE validation
echo ""
echo "=== 3/3 — CUE schema validation ==="
cd "$ROOT_DIR"
cue vet specs/*.cue

echo ""
echo "All checks passed."
