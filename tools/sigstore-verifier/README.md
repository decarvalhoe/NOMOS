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

## Keyless issuance (#645) — injected, non-production only

The same binary accepts `nomos.sigstore-issue.request.v1`: it signs an artifact
with an ephemeral key, obtains a certificate from the INJECTED Fulcio with the
SUPPLIED id token, records the signature in the INJECTED Rekor v1, writes the
bundle, then verifies it with its own verify path against the supplied trust
material before answering `issued: true`. Sigstore public instances
(`*.sigstore.dev`, `*.sigstage.dev`) are refused with no override; anything
that is not loopback, a reserved test domain or an allow-listed host is refused.

`cmd/nomos-sigstore-fixture-services` runs a localhost Fulcio + Rekor v1 pair
for tests and CI: generated CA, an UNSIGNED fixture OIDC token whose claims are
read and nothing else, a one-leaf virtual log (SET, inclusion proof, signed
checkpoint via sigstore-go's virtual CA), and a `trusted_root.json` written for
them. It runs no CT log, so verifying what it issues requires the explicit
`no_ct_log: true` decision — recorded in the record, never implied.

```bash
go build -o nomos-sigstore-fixture-services ./cmd/nomos-sigstore-fixture-services
./nomos-sigstore-fixture-services --out-dir /tmp/svc &      # prints the URLs; NON-PRODUCTION
nomos attest sign-sigstore --artifact a.txt --fulcio-url "$(jq -r .fulcio_url /tmp/svc/services.json)" \
  --rekor-url "$(jq -r .rekor_url /tmp/svc/services.json)" --id-token-file /tmp/svc/id_token \
  --trusted-root /tmp/svc/trusted_root.json --identity fixture-signer@nomos.invalid \
  --issuer https://oidc.fixture.invalid --out-bundle a.sigstore.json --out issuance-record.json
```
