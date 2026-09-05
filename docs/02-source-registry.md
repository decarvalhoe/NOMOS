# 02 - Registre Des Sources

## Pourquoi

On ne peut pas prouver la conformité d'un produit si l'on ne sait pas quelles sources existent. Le registre des sources est donc le premier artefact non négociable.

Il ne sert pas seulement à lister les bons documents. Il sert aussi à rendre visibles les doublons, les sources obsolètes, les scraps incomplets, les fichiers legacy suspects, les exports propriétaires, les décisions humaines, les notes contradictoires et les sources volontairement hors scope.

Une source ignorée sans entrée explicite devient une bombe métier : personne ne peut savoir si elle a été oubliée, rejetée ou remplacée.

## Artefact

Chemin recommandé :

```text
docs/canonical/source-manifest.yaml
```

Le fichier peut être écrit à la main au début, puis généré ou contrôlé par un outil. À maturité, il doit être vérifié par CI.

## Champs Obligatoires

| Champ | Rôle |
|---|---|
| `id` | Identifiant stable, lisible, non réutilisé. |
| `path` | Chemin ou URL source. |
| `type` | `markdown`, `pdf`, `html`, `php`, `csv`, `database_export`, `image`, `audio`, `decision`, etc. |
| `domain` | Domaine métier principal. |
| `priority` | Priorité d'autorité : `primary`, `secondary`, `legacy`, `derived`, `reference`. |
| `status` | `active`, `superseded`, `duplicate`, `out_of_scope`, `needs_review`, `blocked`. |
| `hash` | Hash du contenu normalisé. |
| `version` | Version métier, date d'effet, tag légal ou commit si disponible. |
| `owner` | Responsable de validation. |
| `license` | Statut de droit d'usage. |
| `confidentiality` | `public`, `internal`, `restricted`, `secret`. |
| `notes` | Raison d'inclusion, exclusion ou doute. |

## Priorité D'autorité

La priorité évite les arbitrages implicites.

| Priorité | Sens |
|---|---|
| `primary` | Source d'autorité directe : loi publiée, règle canonique, contrat signé, norme officielle. |
| `secondary` | Explication officielle ou guide d'application. |
| `legacy` | Système existant qui montre une interprétation historique ou un comportement réel. |
| `derived` | Export, compilation, table consolidée, documentation générée. |
| `reference` | Lexique, exemple, forum interne, note utile mais non normative. |

La priorité ne décide pas seule. Elle indique le poids initial dans une résolution de conflit. Un legacy peut l'emporter temporairement si le produit doit préserver une compatibilité, mais cela doit être un decision record.

## Statuts

| Statut | Sens |
|---|---|
| `active` | Doit être prise en compte et indexée. |
| `superseded` | Remplacée par une autre source, gardée pour historique. |
| `duplicate` | Même contenu ou contenu équivalent à une source active. |
| `out_of_scope` | Connue, exclue explicitement. |
| `needs_review` | Source incertaine, non validée. |
| `blocked` | Source inaccessible, corrompue ou juridiquement bloquée. |

## Hash

Le hash doit porter sur un contenu normalisé :

- PDF : extraction textuelle stable + metadata minimale ;
- Markdown : contenu avec fins de ligne normalisées ;
- HTML : contenu utile après retrait navigation/publicité si le scraping est autorisé ;
- code legacy : fichier brut ou AST si l'équipe a un parseur stable ;
- image/scans : hash du fichier + hash OCR si OCR utilisé.

Le manifest doit permettre de répondre :

- quelle source a changé ?
- quelle unité est impactée ?
- quels tests doivent être rejoués ?
- quels chunks vectoriels sont obsolètes ?

## Sources Web (#610)

Une page crawlée n'est pas un fichier sur disque. Elle a été récupérée quelque
part, à un instant, par un crawler, sous une décision robots/licence — et ce que
NOMOS conserve ensuite est une **capture**, jamais la vérité courante du site.

Le bloc optionnel `web_source` porte cette provenance (`#WebSource` dans
`specs/source-manifest.cue`, `corpus.WebSource` côté moteur). Il est validé
fail-closed par le manifeste, le feed et `nomos check` : une source web sans
`canonical_url`, sans `content_hash` stable (`algo:hex`), sans `fetched_at`
RFC 3339 ni `crawler_version` est **refusée**, avec un code d'erreur stable
(`WEB_SOURCE_NO_CONTENT_HASH`, `WEB_SOURCE_UNSTABLE_HASH`, …).

Les cinq natures demandées — contenu canonique, référence externe, binaire/média,
non supporté, exclu par politique — ne sont pas réinventées : ce sont les champs
FSQ-02 déjà portés par toute source (`source_role`, `atomization_status`,
`admission_status`). Une source web les porte comme les autres.

Deux règles coûtent quelque chose et sont voulues :

- `robots_decision` / `licence_decision: undecided` **peut être enregistré** — un
  crawler peut honnêtement ne pas savoir — mais **ne peut jamais être admis**
  dans un feed. L'admission exige `allowed` des deux côtés et `scope_policy`
  hors `out_scope`.
- `content_hash` est le hash des **octets bruts tels que récupérés** ;
  `normalized_content_hash` celui du texte normalisé dont dérivent l'export
  Markdown et l'unité de feed. Deux captures qui ne diffèrent que par le
  chrome de page partagent le second, jamais le premier.

Correspondance avec la preuve de connecteur (`nomos-connector-evidence-v1`) :
`fetched_url ← FetchResult.URL`, `http_status ← StatusCode`,
`content_hash ← SHA256`, `etag/last_modified ← ETag/LastModified`,
`fetched_at ← FetchedAt`. `canonical_url` est l'adresse par laquelle la source
est *connue* (après redirections), qui diffère légitimement de l'URL demandée.

Limite de revendication : « source web capturée à l'instant T par le crawler
nommé ». Les décisions robots/licence sont **enregistrées telles que prises**,
jamais adjugées par NOMOS ; celles d'un site réel restent hors fixture.

## Procédure D'inventaire

1. Scanner les dossiers de documentation, code legacy, exports, scraps, assets et dumps.
2. Lister les fichiers et URLs dans le manifest avec statut `needs_review`.
3. Calculer les hashes.
4. Classer par domaine et priorité.
5. Marquer les doublons et hors scope avec justification.
6. Identifier les sources bloquées et leur plan de récupération.
7. Faire valider les sources `primary` par un propriétaire métier.
8. Geler un baseline de manifest avant atomisation.

## Tests Associés

- Le manifest est valide contre son schéma.
- Tous les chemins locaux existent.
- Tous les hashes correspondent au contenu courant.
- Aucune source active n'est sans propriétaire.
- Aucune source active n'est sans licence/confidentialité.
- Aucune source active n'est absente de l'index vectoriel.
- Aucune source `primary` n'est en `needs_review` dans une release stricte.

