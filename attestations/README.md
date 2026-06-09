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

## Intended Expansion

Future attestation work may include:

- SLSA-aligned provenance;
- CKM supply-chain predicates for source -> canon -> embedding stages;
- artifact signing via Sigstore/Rekor keyless workflows;
- cosign integration and verification;
- release-bundle signatures;
- customer evidence export.

## Signed Claim Boundary

Nomos also records negative attestations for claims it cannot prove. A
claim-boundary predicate lists each refused claim, the reason, the evidence that
would be required, and signing metadata. This inverts the normal provenance
model: the signed artifact is not "Y is true", but "Nomos cannot produce the
required trace for Y, therefore Nomos refuses to assert Y."

## CKM Supply-Chain Predicate

The CKM predicate type is `https://nomos.dev/ckm/supply-chain/v1`. It records
each transformation stage (`ingestion`, `canon`, `embedding`) with material and
product digests. Unsigned mode is still allowed for local or offline runs, but it
is explicitly marked `trust_tier: unverified`. A signed claim must use
`sigstore-keyless` mode and carry a Rekor UUID; local verification checks recorded
artifact hashes and refuses changed artifacts.

## Claim Boundary

An attestation proves a recorded pipeline event. It does not prove that the source was correct, licensed, complete, or legally applicable. It also does not replace customer validation in regulated use.
