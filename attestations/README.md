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
- artifact signing;
- cosign integration;
- release-bundle signatures;
- customer evidence export.

## Claim Boundary

An attestation proves a recorded pipeline event. It does not prove that the source was correct, licensed, complete, or legally applicable. It also does not replace customer validation in regulated use.
