# Attestations

Attestations record what Nomos processed and which evidence was produced.

## Current Alpha Output

Nomos can generate in-toto style corpus attestation records. These records help prove:

- corpus identity;
- project identity;
- source scan summary;
- units extracted;
- diagnosis verdict;
- generation timestamp;
- relevant metadata.

## Cryptographic Signing (ECDSA P-256 DSSE)

NOMOS signs attestation predicates with a **real** cryptographic signature —
ECDSA P-256 over the DSSE v1 Pre-Authentication Encoding — implemented in the Go
standard library (no external cosign binary, no network):

```
nomos attest keygen --out priv.pem --pub-out pub.pem
nomos attest sign   --statement statement.json --key priv.pem --out envelope.json
nomos attest verify --envelope envelope.json --pub pub.pem
```

Verification fails if any byte of the signed payload changes after signing
(including an artifact digest recorded in the statement) — this is genuine
tamper-evidence, not a field-presence check. Until a predicate is signed this
way it is **unsigned and tamper-evident by hash only**; NOMOS does not describe a
hash-only artifact as "signed".

### Intended Expansion

- **Keyless** Sigstore/Fulcio + Rekor transparency-log workflows (needs an OIDC
  round-trip; the key-based DSSE path above is the offline equivalent today);
- SLSA-aligned provenance build integration;
- release-bundle signatures and customer evidence export.

## Signed Claim Boundary

Nomos also records negative attestations for claims it cannot prove. A
claim-boundary predicate lists each refused claim, the reason, the evidence that
would be required, and signing metadata. This inverts the normal provenance
model: the artifact does not assert "Y is true", but "Nomos cannot produce the
required trace for Y, therefore Nomos refuses to assert Y."

The predicate's embedded `signature` field records `signatureMode` (`none`,
`dsse-cosign`, or `sigstore-keyless`); when it is `none` the statement is
unsigned and must not be described as signed. A real signature is produced by
wrapping the statement in a DSSE envelope with `nomos attest sign` (see above) —
not by populating the embedded field with a placeholder string.

## CKM Supply-Chain Predicate

The CKM predicate type is `https://nomos.dev/ckm/supply-chain/v1`. It records
each transformation stage (`ingestion`, `canon`, `embedding`) with material and
product digests. Unsigned mode is still allowed for local or offline runs, but it
is explicitly marked `trust_tier: unverified`. A signed claim must use
`sigstore-keyless` mode and carry a Rekor UUID; local verification checks recorded
artifact hashes and refuses changed artifacts.

## Claim Boundary

An attestation proves a recorded pipeline event. It does not prove that the source was correct, licensed, complete, or legally applicable. It also does not replace customer validation in regulated use.
