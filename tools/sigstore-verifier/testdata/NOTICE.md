# Fixture provenance

`bundle-provenance.sigstore.json` and `trusted-root-public-good.json` are copied
verbatim from the `examples/` directory of sigstore-go v1.3.0
(https://github.com/sigstore/sigstore-go, Apache License 2.0). The bundle is a
REAL Sigstore bundle: a SLSA provenance statement for an npm release of
sigstore-js, signed keylessly by a GitHub Actions identity, with a Fulcio
certificate, a Rekor transparency-log entry (inclusion proof + checkpoint) and
a signed certificate timestamp. The trusted root is the Sigstore public-good
instance's root material at the time of the copy.

They are test fixtures: verifying them proves that this verifier can verify a
supplied bundle offline against supplied trust material. It says nothing about
NOMOS artifacts, which are not signed by Sigstore.

`othername.sigstore.json` and `trusted-root-scaffolding.json` come from
`pkg/testing/data/` of the same sigstore-go release (Apache License 2.0). The
bundle is a v0.3 bundle with a message signature, a Fulcio-style certificate
whose SAN is an OtherName (`foo!oidc.local`, issuer `http://oidc.local:8080`),
and a Rekor entry carrying BOTH an inclusion promise and an inclusion proof with
a signed checkpoint, verifiable against the scaffolding (test instance) trusted
root. It exercises the proof/checkpoint path the public-good v0.1 bundle does
not have.
