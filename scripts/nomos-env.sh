#!/usr/bin/env bash

# Source this file to enable the local Nomos toolchain installed in .tools/.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export NOMOS_TOOLS_ROOT="$ROOT_DIR/.tools"
mkdir -p \
  "$NOMOS_TOOLS_ROOT/cache/go-build" \
  "$NOMOS_TOOLS_ROOT/cache/gomod" \
  "$NOMOS_TOOLS_ROOT/cache/cue"
export GOCACHE="$NOMOS_TOOLS_ROOT/cache/go-build"
export GOMODCACHE="$NOMOS_TOOLS_ROOT/cache/gomod"
export GOPATH="$NOMOS_TOOLS_ROOT/cache/gopath"
export CUE_CACHE_DIR="$NOMOS_TOOLS_ROOT/cache/cue"
export PATH="$NOMOS_TOOLS_ROOT/go/bin:$NOMOS_TOOLS_ROOT/cue:$PATH"
