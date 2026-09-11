# 52 — Plan de reprise de développement (2026-09-11)

> Date : 2026-09-11. Statut : plan actif. Décision associée :
> [ADR-0006](adr/0006-forge-neutrality.md). Il prolonge `docs/51` (beta
> atteinte le 2026-09-07) et suit la doctrine `docs/43`. Claim boundary : ce
> plan ne crée aucune release, aucune claim régulée, aucun SLA ; la `v1.0.0`
> finale reste l'acte humain de #720.

---

## 0. D'où l'on repart — mesuré le 2026-09-11

Inventaire complet du dépôt (clone GitHub `decarvalhoe/NOMOS`, `main` =
`a4d0517`, 482 commits, arbre propre) et ligne de base rejouée sur une
machine sans Go préinstallé :

| Mesure | Résultat |
|---|---|
| `go vet ./...` (Go 1.26.6, `toolchain` du `go.mod`) | vert |
| `go test ./...` | 34 paquets `ok` |
| `python3 -m unittest discover -s tests` | 583 tests, OK, 102 ignorés |
| Matrice de câblage | 66 capacités : 47 `real`, 17 `sidecar`, 2 `absent`, 0 écart |
| Files de dispatch (`docs/roadmap-lanes.yaml`) | vides : produit 0, devops 0 ; régulé = 1 passif, 3 humains, 5 externes |

Ce que l'inventaire a trouvé et qui n'était écrit nulle part :

1. **Les README recopiaient une matrice périmée.** FR : 57 capacités
   (44/11/2) ; EN : 40 (32/7/1) ; matrice : 66 (47/17/2). La matrice était
   juste, la prose se taisait — docs/43 §2.8.
2. **La forge souveraine portait une autre lignée.** `RBOKproject/nomos`
   s'était arrêté le 12.08.2026 sur `12906aea` + 16 commits de kit agentique
   (`.forgejo/`, `AGENTS.md`) inconnus de GitHub ; GitHub avait trois mois
   d'avance. Ancêtre commun : `12906aea`, fusion sans conflit.
3. **Identité du dépôt incohérente.** Remote `decarvalhoe/NOMOS` ; module Go,
   manifeste et 273 références disent `RBOKproject/Nomos`.
4. **Adhérence GitHub dans les sidecars.** 10 fichiers appellent `gh`
   directement ; 6 workflows sur 13 dépendent de `GITHUB_TOKEN` ou d'étapes
   marketplace que les runners de la forge n'ont pas. Le moteur Go, lui, ne
   dépend d'aucune forge.
5. **Un filtre de workflow mort** : `ckm-non-regression.yml` surveillait
   `control-plane/**`, retiré par ADR-0007 (decisions).
6. **Trois dossiers coquilles** : `sdk/`, `policies/`, `references/` (README
   seuls) ; `examples/` n'a de contenu que pour l'assurance.
7. **Six PR dependabot vertes** (#730–#735) en attente de fusion.
8. **Sigstore keyless** : seul le chemin hors ligne à clé existe ; l'émission
   production est `absent` par construction (#638, lane régulée, externe).

## 1. Directions retenues

| Direction | Source | Conséquence pour ce plan |
|---|---|---|
| La forge souveraine est le dépôt de référence ; GitLab doit marcher ; GitHub n'est pas privilégié | exigence d'organisation du 11.09.2026, ADR-0006 | la neutralité de forge est le chantier produit/devops de la reprise |
| Roadmaps indépendantes, files autonomes | ADR-VRC-0004, `docs/47` | les tranches entrent dans la lane `devops` comme items `dispatch:autonomous` |
| Pas de *done* sans preuve adversariale ; ce qui se tait ment | `docs/43` §2.3 et §2.8 | chaque tranche livre un test qui échoue sans le correctif, et un garde plutôt qu'une constante |
| La release est un acte humain | `docs/51` B5, #720 | rien ici ne tague ni ne publie |

## 2. Vagues

### Vague 0 — Outillage et ligne de base (faite le 2026-09-11)

- Go 1.26.6 installé sous `~/.local/go` (somme SHA-256 vérifiée contre
  `go.dev/dl`), `PATH` dans `~/.bashrc` ; ligne de base ci-dessus.
- Compteurs de capacités des trois README réalignés sur la matrice, et
  **garde** `scripts/readme_capability_counts_guard.py` (`--check` en CI,
  `--write` en remède) avec son test adversarial
  `tests/test_readme_capability_counts_guard.py`.
- `ckm-non-regression.yml` ne filtre plus `control-plane/**`.

### Vague 1 — Réconciliation de la forge (PR ouverte le 2026-09-11)

- Branche `sync/github-main-2026-09-11` = GitHub `main` + fusion `--no-ff`
  de la `main` de la forge (le kit survit, aucun contenu perdu, aucune
  réécriture d'historique). PR #2 sur `RBOKproject/nomos`, étiquettes
  `type:task` + `status:review`, sections « Vérifié / NON vérifié ».
- Après fusion : la forge `main` est au niveau de GitHub `main` ; chaque
  fusion GitHub est ensuite reflétée sur la forge le jour même (ADR-0006 §1)
  jusqu'au portage des portes (vague 2, tranche 3).

### Vague 2 — Neutralité de forge (lane `devops`, quatre tranches)

| Tranche | Issue | Contenu | Dépend de |
|---|---|---|---|
| FN-1 | #737 | `scripts/forge_provider.py` (`github`/`forgejo`/`gitlab`/`fake`) ; migration du commentaire collant de PR et du collecteur d'evidence CI ; tests sans binaire ni réseau | — |
| FN-2 | #736 | migration des scripts regulated, du publisher, du lane guard ; suppression de `push_and_pr.sh` (script de bootstrap mort) | FN-1 |
| FN-3 | #738 | portes opposables sous `.forgejo/workflows/portes.yml` + `.forgejo/portes.sh` sans `uses:` marketplace (livré le 2026-09-11, voir `docs/07` « Où tournent les portes ») ; contextes exigés sur `main` de la forge : acte de l'administrateur | — |
| FN-4 | #739 | champ `tracker` par item du registre de roadmap ; `--verify-tracker` via le provider ; bump `schema_version` du contrat | FN-1 |
| FN-5 | #746 | `cue vet` et la porte de sécurité (`govulncheck`, `pip-audit`) ajoutés à `.forgejo/portes.sh` ; CUE épinglé par somme ; ne restent hors forge que la matrice corpus macOS/Windows et `bundle-release.yml` | FN-3 |

Ordre de la file `devops` : FN-1, FN-3, FN-2, FN-4. Une tranche est *done*
quand `unittest` est vert sans `gh` dans le `PATH`, que la matrice de
câblage, le support model et le claim boundary sont inchangés, et que la PR
montre la commande qui l'a prouvé.

### Vague 3 — Hygiène (sans issue, à la main)

- Fusionner les six PR dependabot vertes (#730–#735) : patchs et mineures
  sur des fixtures, `upload-artifact` 4→7 et `vitest` 3→4 avec CI verte.
  **Acte de l'opérateur** : la fusion de PR GitHub est hors des droits de
  l'agent dans cette session.
- Trancher l'identité du dépôt : soit transférer `decarvalhoe/NOMOS` vers
  l'organisation `RBOKproject` sur GitHub, soit corriger les 273 références.
  Décision d'Eric ; le plan ne présume rien.
- Coquilles `sdk/`, `policies/`, `references/`, `examples/` : soit les
  remplir, soit les déclarer explicitement « surface non livrée » dans le
  README (le claim boundary y gagne). Proposition : déclarer, ne pas remplir.

### Vague 4 — Ce qui reste humain ou externe (inchangé)

#720 (release beta via SOP), #561 (release alpha via SOP), #562 (records de
compétence), #638 (Sigstore production), #701 (preuve croisée avec le RAG
juridique voisin), #192–#196 (acquisition et licences des référentiels).
Ils bloquent des claims, jamais le dispatcher (`docs/47`).

## 3. Conduite

- **Où l'on développe** : PR sur GitHub tant que les portes opposables y
  tournent (`ci.yml` et ses gardes), fusion quand la CI est verte et la revue
  faite ; miroir sur la forge après fusion. Quand FN-3 est livrée, la forge
  devient le lieu des PR et GitHub un miroir.
- **Identité** : sur la forge, l'agent écrit sous `agent-claude` (auteur ≠
  valideur) ; sur GitHub, sous le compte de l'opérateur.
- **Preuve** : chaque PR cite les commandes exécutées et ce qui n'a pas été
  vérifié (banc ≠ production : dire par quel transport le code arrive là où
  il tourne — runner GitHub, runner de la forge, poste).

## 4. Risques

| Risque | Parade |
|---|---|
| Les workflows GitHub, une fois sur la forge, saturent les runners en échouant (`uses:` interdit) | ils échouaient déjà sur la forge depuis `12906aea` ; FN-3 les remplace au lieu de les réparer |
| Un provider qui échoue en silence masque une non-publication | erreur nommée obligatoire (ADR-0006 §3), testée avec `fake` |
| Deux trackers (GitHub, forge) pendant la transition | FN-4 qualifie chaque identifiant (`default_tracker`, `tracker` par item, schéma 1.1.0) et `--verify-tracker` lit chaque item sur sa propre forge ; un tracker non configuré échoue par nom d'item |
| Le registre de roadmap et les docs générées divergent | `roadmap_lane_guard.py` regénère les tables et échoue sur toute dérive |
