# ADR-0005: Niveau d'attestation en v0.1

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

Nomos produit des attestations qui certifient le resultat d'une admission ou d'un check. Le schema `nomos.project.yaml` definit trois niveaux d'attestation : `none`, `basic`, `signed`. La question est de savoir quel niveau est supporte et recommande en v0.1.

Le package `cli/internal/attestation` implemente la generation d'enveloppes in-toto, la provenance SLSA v1, et la signature DSSE.

> **Mise a jour (CKM-H1 / CKM-H1-FU, #529 / #537).** La verification cryptographique
> EST desormais implementee. `cli/internal/attestation/signing.go` realise une
> signature ECDSA P-256 reelle sur l'encodage DSSE v1 PAE (`pae`,
> `Signer.SignStatement`, `VerifyEnvelope`, entierement dans la stdlib Go, sans
> binaire cosign ni reseau). Alterer un seul octet du payload signe — par exemple
> un digest d'artefact enregistre dans le statement — fait echouer la verification
> (preuve adversariale : `TestVerify_FailsWhenArtifactDigestTampered`). L'ancien
> chemin factice (`WrapCosignEnvelope`, qui codait en dur `sig: ""`, et
> `VerifyCosignEnvelope`, qui ne validait que la presence des champs) a ete
> SUPPRIME : il ne produisait pas de signature et n'en verifiait aucune. Une
> enveloppe avec `sig: ""` ou alteree est maintenant rejetee
> (`TestVerifyEnvelope_RejectsEmptySignature`). Le mode keyless Sigstore
> (Fulcio/Rekor) reste un suivi documente ; le chemin offline ci-dessus est son
> equivalent disponible aujourd'hui. Un garde de frontiere de revendication
> (`scripts/claim_boundary_guard.py`, cable dans la CI et
> `scripts/ckm-non-regression.sh`) echoue si la documentation revendique
> « signed »/« Sigstore »/« certified » au sens d'une capacite sans preuve.

## Options considerees

### Option A: basic par defaut, signed optionnel

- `basic` : attestation generee avec digest SHA-256, sans signature cryptographique.
- `signed` : enveloppe cosign avec champ signature a remplir par un outil externe (cosign, gpg).
- Le CLI genere l'enveloppe ; la signature est deleguee a l'outillage existant.

### Option B: signed obligatoire

- Toute attestation doit etre signee.
- Bloque l'adoption par les projets qui n'ont pas de setup de signature.
- Premature pour une v0.1.

### Option C: none par defaut

- Pas d'attestation generee par defaut.
- Reduit la valeur ajoutee de Nomos sur la chaine de confiance.

## Decision

En v0.1, le niveau par defaut est `basic`. Le mode `regulated` genere des enveloppes cosign pre-remplies (`signed`) dont la signature est deleguee a un outil externe. Le CLI ne gere pas les cles privees.

## Consequences

- Le mode `minimal` genere des attestations `basic` (in-toto statement + digest, pas de signature).
- Le niveau `signed` est desormais REEL : `nomos attest sign` et `AttestCommand`
  produisent une enveloppe DSSE signee ECDSA P-256, verifiable via `nomos attest
  verify` / `VerifyEnvelope`. La signature n'est plus deleguee a un outil externe
  pour ce chemin, et le CLI ne stocke pas de cle privee persistante (cle ephemere
  par defaut, ou `--key`).
- Le CLI verifie la signature cryptographique (pas seulement la structure) :
  une enveloppe `sig: ""` ou alteree est rejetee.
- Les schemas CUE (`attestations/nomos-attestation.cue`) definissent les formats
  in-toto, SLSA et cosign ; le type `#CosignEnvelope` reste pour la compatibilite
  de schema, mais les helpers Go factices correspondants ont ete supprimes.
- Suivi v0.2+ : mode keyless Sigstore (Fulcio/Rekor) et stockage des cles
  publiques par le control-plane pour la verification distribuee.
