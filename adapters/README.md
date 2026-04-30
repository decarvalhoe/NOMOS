# Adapters

Ce dossier contient le contrat public des adapters Nomos et, a terme, les
implementations versionnees par stack et surface.

Contrat courant :

- `adapter-contract.md` : regles de manifeste, capabilities, versioning et
  compatibilite ;
- `../specs/adapter-manifest.cue` : schema CUE machine-readable du manifeste ;
- `../specs/examples/adapter-manifest.node-typescript.yaml` : exemple concret de
  manifeste adapter.

Chaque adapter doit declarer :

- stacks supportees ;
- surfaces detectables ;
- capabilities stables ou experimentales ;
- commandes exposees au CLI Nomos ;
- patterns interdits ;
- limites connues ;
- version compatible du coeur Nomos.

Nom de fichier attendu pour une implementation :

```text
adapter.nomos.yaml
```

Le manifeste est l'interface stable entre le coeur Nomos et les adapters. Une
implementation sans manifeste valide n'est pas chargeable par le coeur, meme si
son code existe.
