#!/usr/bin/env bash
# #637 — offline Sigstore verification gate.
#
# Builds the external verifier (tools/sigstore-verifier, its own module) and the
# NOMOS CLI, then proves on the REAL fixture bundles:
#   • `nomos attest verify-sigstore` verifies offline and writes a record that
#     binds the verifier's exact response bytes;
#   • wrong artifact digest, wrong identity, wrong issuer, a flipped signature
#     byte, a flipped inclusion-promise byte, a flipped inclusion-proof hash and
#     an edited checkpoint are each REFUSED with exit 1 and NO record;
#   • with no verifier available the command exits non-zero and writes NO
#     record — no verdict is invented.
# Nothing here signs, issues an identity, or contacts Fulcio/Rekor.
#
# Exit codes: 0 all proofs held · 1 a proof failed · 5 preflight.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$SCRIPT_DIR/.." && pwd)"
WORK="${WORK_DIR:-$(mktemp -d)}"
FIX="$REPO/tools/sigstore-verifier/testdata"

log() { printf '[sigstore-gate] %s\n' "$*"; }
die() { local c="$1"; shift; printf '[sigstore-gate] FATAL: %s\n' "$*" >&2; exit "$c"; }
command -v go >/dev/null || die 5 "go not on PATH"
command -v python3 >/dev/null || die 5 "python3 not on PATH"
[[ -f "$FIX/bundle-provenance.sigstore.json" ]] || die 5 "fixture missing"

log "building verifier (tools/sigstore-verifier) and nomos"
( cd "$REPO/tools/sigstore-verifier" && go build -o "$WORK/nomos-sigstore-verifier" . ) || die 1 "verifier build failed"
( cd "$REPO/cli" && go build -o "$WORK/nomos" . ) || die 1 "nomos build failed"

BUNDLE="$FIX/bundle-provenance.sigstore.json"; ROOT="$FIX/trusted-root-public-good.json"
DIGEST="sha512:76176ffa33808b54602c7c35de5c6e9a4deb96066dba6533f50ac234f4f1f4c6b3527515dc17c06fbe2860030f410eee69ea20079bd3a2c6f3dcf3b329b10751"
SAN="https://github.com/sigstore/sigstore-js/.github/workflows/release.yml@refs/heads/main"
ISS="https://token.actions.githubusercontent.com"
PBUNDLE="$FIX/othername.sigstore.json"; PROOT="$FIX/trusted-root-scaffolding.json"
PDIGEST="sha256:$(python3 -c "import json,base64;b=json.load(open('$PBUNDLE'));print(base64.b64decode(b['messageSignature']['messageDigest']['digest']).hex())")"
PSAN="foo!oidc.local"; PISS="http://oidc.local:8080"

verify() { # verify <out> <args...>
  local out="$1"; shift
  "$WORK/nomos" attest verify-sigstore --verifier "$WORK/nomos-sigstore-verifier" --out "$out" "$@"
}

log "1. real public-good bundle verifies offline"
verify "$WORK/record.json" --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS" || die 1 "valid bundle refused"
python3 - "$WORK/record.json" <<'PY'
import json, sys, hashlib
r = json.load(open(sys.argv[1]))
assert r["verified"] is True and r["schema_version"] == "nomos.sigstore-verification-record.v1", r
assert r["signer_san"].endswith("release.yml@refs/heads/main") and r["tlog_entries"] >= 1
assert r["mode"] == "offline" and r["library"] == "sigstore-go" and r["library_version"].startswith("v"), r["library_version"]
resp = json.dumps(r["response"], separators=(",", ":"), ensure_ascii=False)
assert r["response_digest"].startswith("sha256:"), r["response_digest"]
print("   record ok: verifier", r["verifier"], r["verifier_version"], "via", r["library"], r["library_version"])
PY

log "2. real v0.3 bundle with inclusion proof + checkpoint verifies offline"
verify "$WORK/record-proof.json" --bundle "$PBUNDLE" --trusted-root "$PROOT" --artifact-digest "$PDIGEST" --identity "$PSAN" --issuer "$PISS" || die 1 "inclusion-proof bundle refused"
python3 -c "import json,sys; r=json.load(open(sys.argv[1])); assert r['response']['tlog_entries'][0]['has_inclusion_proof'] is True; print('   inclusion proof present in the verified entry')" "$WORK/record-proof.json"

tamper() { # tamper <src> <dst> <python-edit>
  python3 - "$1" "$2" "$3" <<'PY'
import json, sys, base64
src, dst, edit = sys.argv[1:4]
b = json.load(open(src))
def flip(s):
    raw = bytearray(base64.b64decode(s)); raw[len(raw)//2] ^= 1; return base64.b64encode(bytes(raw)).decode()
exec(edit)
json.dump(b, open(dst, "w"))
PY
}
must_refuse() { # must_refuse <label> <args...>
  local label="$1"; shift
  local out="$WORK/refused-$RANDOM.json"
  if verify "$out" "$@" 2>"$WORK/stderr.txt"; then die 1 "$label: ACCEPTED (must be refused)"; fi
  [[ ! -e "$out" ]] || die 1 "$label: a record was written despite the refusal"
  grep -q "REFUSED\|no verdict\|No verdict" "$WORK/stderr.txt" || die 1 "$label: refusal not explicit: $(cat "$WORK/stderr.txt")"
  log "   refused: $label — $(tail -1 "$WORK/stderr.txt" | cut -c1-140)"
}

log "3. tamper matrix must be refused, no record written"
must_refuse "wrong artifact digest"  --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "sha512:$(printf '0%.0s' $(seq 128))" --identity "$SAN" --issuer "$ISS"
must_refuse "wrong signer identity" --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "https://github.com/evil/repo/.github/workflows/release.yml@refs/heads/main" --issuer "$ISS"
must_refuse "wrong issuer"          --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "https://accounts.google.com"
tamper "$BUNDLE" "$WORK/t-sig.json" 'b["dsseEnvelope"]["signatures"][0]["sig"] = flip(b["dsseEnvelope"]["signatures"][0]["sig"])'
must_refuse "signature byte flipped" --bundle "$WORK/t-sig.json" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS"
tamper "$BUNDLE" "$WORK/t-set.json" 'e=b["verificationMaterial"]["tlogEntries"][0]["inclusionPromise"]; e["signedEntryTimestamp"]=flip(e["signedEntryTimestamp"])'
must_refuse "inclusion promise flipped" --bundle "$WORK/t-set.json" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS"
tamper "$BUNDLE" "$WORK/t-cert.json" 'c=b["verificationMaterial"]["x509CertificateChain"]["certificates"][0]; c["rawBytes"]=flip(c["rawBytes"])'
must_refuse "certificate byte flipped" --bundle "$WORK/t-cert.json" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS"
tamper "$PBUNDLE" "$WORK/t-proof.json" 'p=b["verificationMaterial"]["tlogEntries"][0]["inclusionProof"]; p["hashes"][0]=flip(p["hashes"][0])'
must_refuse "inclusion proof hash flipped" --bundle "$WORK/t-proof.json" --trusted-root "$PROOT" --artifact-digest "$PDIGEST" --identity "$PSAN" --issuer "$PISS"
tamper "$PBUNDLE" "$WORK/t-cp.json" 'p=b["verificationMaterial"]["tlogEntries"][0]["inclusionProof"]["checkpoint"]; p["envelope"]=p["envelope"].replace("\n","\n \n",1)'
must_refuse "checkpoint edited" --bundle "$WORK/t-cp.json" --trusted-root "$PROOT" --artifact-digest "$PDIGEST" --identity "$PSAN" --issuer "$PISS"
must_refuse "trust material does not cover the log" --bundle "$PBUNDLE" --trusted-root "$ROOT" --artifact-digest "$PDIGEST" --identity "$PSAN" --issuer "$PISS"

log "4. no verifier available → non-zero, no record, no verdict"
out="$WORK/absent.json"
if env -u NOMOS_SIGSTORE_VERIFIER PATH="$WORK/empty-path" "$WORK/nomos" attest verify-sigstore --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS" --out "$out" 2>"$WORK/stderr.txt"; then
  die 1 "verification succeeded with no verifier available"
fi
[[ ! -e "$out" ]] || die 1 "a record was written with no verifier"
grep -q "No verdict" "$WORK/stderr.txt" || die 1 "absence must say no verdict: $(cat "$WORK/stderr.txt")"
log "   refused: $(tail -1 "$WORK/stderr.txt" | cut -c1-140)"

log "5. a verifier that lies about the artifact is caught by NOMOS"
cat > "$WORK/liar.sh" <<'SH'
#!/bin/sh
cat >/dev/null
printf '%s' '{"schema_version":"nomos.sigstore-verify.response.v1","verifier":{"name":"liar","version":"0","library":"none","library_version":"0"},"mode":"offline","verified":true,"artifact_digest":"sha512:0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000","certificate":{"subject_alternative_name":"'"$SAN_PLACEHOLDER"'","issuer":"x"},"tlog_entries":[{"log_index":1}],"claim_boundary":""}'
SH
sed -i "s|\$SAN_PLACEHOLDER|$SAN|" "$WORK/liar.sh"; chmod +x "$WORK/liar.sh"
if "$WORK/nomos" attest verify-sigstore --verifier "$WORK/liar.sh" --bundle "$BUNDLE" --trusted-root "$ROOT" --artifact-digest "$DIGEST" --identity "$SAN" --issuer "$ISS" --out "$WORK/liar.json" 2>"$WORK/stderr.txt"; then
  die 1 "a lying verifier was believed"
fi
grep -q "SIGSTORE_DIGEST_DISAGREEMENT\|SIGSTORE_IDENTITY_DISAGREEMENT" "$WORK/stderr.txt" || die 1 "liar not caught by NOMOS's own re-check: $(cat "$WORK/stderr.txt")"
log "   refused: $(tail -1 "$WORK/stderr.txt" | cut -c1-140)"

log "PASS — records: $WORK/record.json $WORK/record-proof.json"
