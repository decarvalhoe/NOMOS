# ADR-0005: Niveau d'attestation en v0.1

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

Nomos produit des attestations qui certifient le resultat d'une admission ou d'un check. Le schema `nomos.project.yaml` definit trois niveaux d'attestation : `none`, `basic`, `signed`. La question est de savoir quel niveau est supporte et recommande en v0.1.

Le package `cli/internal/attestation` implemente la generation d'enveloppes in-toto, la provenance SLSA v1, et le wrapping cosign DSSE. La verification structurelle est en place mais la verification cryptographique (signature/verification avec cle reelle) n'est pas encore implementee.

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
- Le mode `regulated` genere des enveloppes cosign avec `sig: ""` a remplir par `cosign sign` ou equivalent.
- Le CLI valide la structure de l'enveloppe (`VerifyCosignEnvelope`) mais pas la signature cryptographique.
- La verification cryptographique est reportee a la v0.2 quand le control-plane pourra stocker les cles publiques.
- Les schemas CUE (`attestations/nomos-attestation.cue`) definissent deja les formats in-toto, SLSA et cosign pour assurer la compatibilite future.
- La CI peut integrer `cosign sign` comme etape post-admission pour les projets `regulated`.
