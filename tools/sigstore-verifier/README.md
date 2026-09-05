# nomos-sigstore-verifier

The external process behind `nomos attest verify-sigstore` (#637, ADR-0005).
It verifies a **supplied** Sigstore bundle **offline** against **supplied**
trust material using `sigstore-go`, and speaks a versioned JSON protocol on
stdin/stdout. It never signs, never issues an identity, never contacts Fulcio
or Rekor, never fetches a trusted root.

```
cd tools/sigstore-verifier && go build -o "$HOME/bin/nomos-sigstore-verifier" .
nomos attest verify-sigstore --bundle b.sigstore.json --trusted-root trusted_root.json \
  --artifact-digest sha512:<hex> --identity <SAN> --issuer <OIDC issuer> --out record.json
```

Exit codes: `0` verified · `1` refused (`verified: false`, `refusal.code`) ·
`2` protocol error. Refusal codes: `REQUEST_INCOMPLETE`, `TRUSTED_ROOT_UNREADABLE`,
`BUNDLE_UNREADABLE`, `IDENTITY_INVALID`, `ARTIFACT_UNREADABLE`,
`ARTIFACT_DIGEST_INVALID`, `ARTIFACT_MISMATCH`, `IDENTITY_MISMATCH`,
`INCLUSION_INVALID`, `SIGNATURE_INVALID`, `CERTIFICATE_INVALID`,
`VERIFICATION_FAILED`.

`testdata/` holds two REAL bundles (see `NOTICE.md`): a public-good v0.1 bundle
with an inclusion promise and SCT, and a v0.3 bundle with an inclusion proof and
signed checkpoint against the scaffolding root. The tests tamper each part and
require a refusal.
