#!/usr/bin/env bash
# #645 — keyless issuance against INJECTED, NON-PRODUCTION endpoints, gated.
#
# Starts the localhost fixture Fulcio/Rekor pair (tools/sigstore-verifier/cmd/
# nomos-sigstore-fixture-services), then proves through the NOMOS CLI:
#   1. `nomos attest sign-sigstore` issues a bundle against the injected
#      endpoints with the fixture identity and verifies it independently
#      (record written, bundle kept);
#   2. `nomos attest verify-sigstore` re-verifies the issued bundle offline
#      against the fixture trust material (an independent second pass);
#   3. production endpoints are refused BEFORE any request (no flag exists);
#      an unlisted host is refused; a wrong expected identity is refused;
#      a tampered issued bundle is refused;
#   4. with the services stopped, issuance leaves NO partial bundle;
#   5. with no verifier binary, no verdict is invented.
# No production service is contacted at any point; nothing is published.
#
# Exit codes: 0 all proofs held · 1 a proof failed · 5 preflight.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK="${WORK_DIR:-$(mktemp -d)}"
log() { printf '[sigstore-issue-gate] %s\n' "$*"; }
die() { local c="$1"; shift; printf '[sigstore-issue-gate] FATAL: %s\n' "$*" >&2; exit "$c"; }
command -v go >/dev/null || die 5 "go not on PATH"
command -v jq >/dev/null || die 5 "jq not on PATH"

log "building verifier/issuer, fixture services and nomos"
( cd "$REPO/tools/sigstore-verifier" && go build -o "$WORK/nomos-sigstore-verifier" . && go build -o "$WORK/nomos-sigstore-fixture-services" ./cmd/nomos-sigstore-fixture-services ) || die 1 "tool build failed"
( cd "$REPO/cli" && go build -o "$WORK/nomos" . ) || die 1 "nomos build failed"

SVC="$WORK/svc"; mkdir -p "$SVC"
"$WORK/nomos-sigstore-fixture-services" --out-dir "$SVC" > "$SVC/log.txt" 2>&1 &
SVC_PID=$!
trap 'kill $SVC_PID 2>/dev/null || true' EXIT
for _ in $(seq 1 100); do [[ -f "$SVC/services.json" ]] && break; sleep 0.1; done
[[ -f "$SVC/services.json" ]] || die 1 "fixture services did not start: $(cat "$SVC/log.txt")"
FULCIO="$(jq -r .fulcio_url "$SVC/services.json")"; REKOR="$(jq -r .rekor_url "$SVC/services.json")"
SAN="$(jq -r .subject "$SVC/services.json")"; ISS="$(jq -r .oidc_issuer "$SVC/services.json")"
log "fixture services: fulcio=$FULCIO rekor=$REKOR (NON-PRODUCTION)"
printf 'artifact signed against injected fixture services\n' > "$WORK/artifact.txt"

sign() { "$WORK/nomos" attest sign-sigstore --verifier "$WORK/nomos-sigstore-verifier" --artifact "$WORK/artifact.txt" --trusted-root "$SVC/trusted_root.json" --id-token-file "$SVC/id_token" "$@"; }
must_refuse() { # must_refuse <label> <bundle> <args...>
  local label="$1" bundle="$2"; shift 2
  if sign --out-bundle "$bundle" "$@" 2>"$WORK/stderr.txt" >/dev/null; then die 1 "$label: ACCEPTED (must be refused)"; fi
  [[ ! -e "$bundle" ]] || die 1 "$label: a bundle was left behind"
  grep -q "REFUSED\|No verdict" "$WORK/stderr.txt" || die 1 "$label: refusal not explicit: $(cat "$WORK/stderr.txt")"
  log "   refused: $label — $(tail -1 "$WORK/stderr.txt" | cut -c1-150)"
}

log "1. issue against the injected endpoints, independently verified"
sign --fulcio-url "$FULCIO" --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS" --out-bundle "$WORK/issued.sigstore.json" --out "$WORK/issuance-record.json" || die 1 "issuance refused"
jq -e '.non_production == true and .independent_verification.verified == true and (.request.id_token|startswith("<redacted:"))' "$WORK/issuance-record.json" >/dev/null || die 1 "record invariants"
jq -e '.independent_verification.response.tlog_entries[0].has_inclusion_promise == true' "$WORK/issuance-record.json" >/dev/null || die 1 "no inclusion promise in the issued bundle"
log "   record ok: $(jq -r '"signer \(.signer_san) issuer \(.signer_issuer) bundle \(.bundle_digest[0:23])… endpoints \(.endpoints.fulcio)"' "$WORK/issuance-record.json")"

log "2. second, independent offline verification of the issued bundle"
"$WORK/nomos" attest verify-sigstore --verifier "$WORK/nomos-sigstore-verifier" --bundle "$WORK/issued.sigstore.json" --trusted-root "$SVC/trusted_root.json" --artifact "$WORK/artifact.txt" --identity "$SAN" --issuer "$ISS" --require-sct 0 --out "$WORK/verify-record.json" || die 1 "issued bundle failed offline verification"

log "3. refusals"
must_refuse "production fulcio" "$WORK/p1.json" --fulcio-url https://fulcio.sigstore.dev --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS"
must_refuse "production rekor" "$WORK/p2.json" --fulcio-url "$FULCIO" --rekor-url https://rekor.sigstore.dev --identity "$SAN" --issuer "$ISS"
must_refuse "staging instance without allow-list is still a public instance" "$WORK/p3.json" --fulcio-url https://fulcio.sigstage.dev --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS"
must_refuse "unlisted host" "$WORK/p4.json" --fulcio-url https://fulcio.corp.example.com --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS"
must_refuse "wrong expected identity" "$WORK/p5.json" --fulcio-url "$FULCIO" --rekor-url "$REKOR" --identity "someone-else@nomos.invalid" --issuer "$ISS"
must_refuse "wrong expected issuer" "$WORK/p6.json" --fulcio-url "$FULCIO" --rekor-url "$REKOR" --identity "$SAN" --issuer "https://accounts.google.com"
python3 - "$WORK/issued.sigstore.json" "$WORK/tampered.json" <<'PY'
import json, sys, base64
b = json.load(open(sys.argv[1])); sig = bytearray(base64.b64decode(b["messageSignature"]["signature"])); sig[len(sig)//2] ^= 1
b["messageSignature"]["signature"] = base64.b64encode(bytes(sig)).decode(); json.dump(b, open(sys.argv[2], "w"))
PY
if "$WORK/nomos" attest verify-sigstore --verifier "$WORK/nomos-sigstore-verifier" --bundle "$WORK/tampered.json" --trusted-root "$SVC/trusted_root.json" --artifact "$WORK/artifact.txt" --identity "$SAN" --issuer "$ISS" --require-sct 0 2>"$WORK/stderr.txt"; then die 1 "tampered issued bundle verified"; fi
log "   refused: tampered issued bundle — $(tail -1 "$WORK/stderr.txt" | cut -c1-120)"

log "4. services stopped → no partial bundle"
kill $SVC_PID; wait $SVC_PID 2>/dev/null || true
must_refuse "service unavailable" "$WORK/p7.json" --fulcio-url "$FULCIO" --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS"

log "5. no issuer/verifier binary → no verdict"
if env -u NOMOS_SIGSTORE_VERIFIER PATH="$WORK/empty" "$WORK/nomos" attest sign-sigstore --artifact "$WORK/artifact.txt" --trusted-root "$SVC/trusted_root.json" --id-token-file "$SVC/id_token" --fulcio-url "$FULCIO" --rekor-url "$REKOR" --identity "$SAN" --issuer "$ISS" --out-bundle "$WORK/p8.json" 2>"$WORK/stderr.txt"; then die 1 "issuance succeeded with no binary"; fi
grep -q "No verdict" "$WORK/stderr.txt" || die 1 "absence must say no verdict"
[[ ! -e "$WORK/p8.json" ]] || die 1 "bundle left behind with no binary"
log "   refused: $(tail -1 "$WORK/stderr.txt" | cut -c1-120)"
log "PASS — records: $WORK/issuance-record.json $WORK/verify-record.json"
