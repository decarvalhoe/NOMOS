# Intrants de valorisation — cadres neutres pour évaluation externe

> Ce document rassemble des **cadres et points de référence neutres** qu'un analyste externe peut appliquer lui-même. Il **ne propose aucune fourchette de valeur**, aucune auto-évaluation, et ne positionne pas NOMOS sur une échelle de valeur. Les comparables de marché cités sont des **repères de catégorie**, pas des comparables directs de NOMOS à son stade actuel (alpha — voir [evidence-and-maturity.md](evidence-and-maturity.md)).
>
> Pour l'état réel du produit, voir [evidence-and-maturity.md](evidence-and-maturity.md). Pour les limites de claims, voir [public-claim-boundary.md](../public-claim-boundary.md).

## Pourquoi ce document est séparé

La valorisation d'un projet à un stade précoce dépend d'hypothèses (maturité, revenus, pilotes, rétention, barrières de reproductibilité, valeur stratégique) que **seul un évaluateur indépendant doit poser**. Pour préserver l'impartialité de l'analyse, ce dépôt fournit les *intrants* mais ne formule pas de verdict de valeur.

## 1. Cadres de capitalisation comptable (intrant)

Les coûts de développement d'un actif incorporel développé en interne ne peuvent être capitalisés que lorsque les critères applicables sont remplis : faisabilité technique, intention d'achever, capacité à utiliser ou vendre, bénéfices économiques futurs probables, ressources disponibles, et mesure fiable des coûts. Référentiels :

- [IAS 38 — Immobilisations incorporelles](https://www.ifrs.org/issued-standards/list-of-standards/ias-38-intangible-assets/)
- [Swiss GAAP RPC 10 — Valeurs immatérielles](https://www.fer.ch/en/standards/swiss-gaap-fer-10-immaterielle-werte/)

Éléments potentiellement éligibles, à apprécier par l'analyste : temps de développement, architecture, tests, documentation, CI, records de validation, outillage et infrastructure directement attribuables. L'inventaire factuel correspondant est dans [evidence-and-maturity.md](evidence-and-maturity.md).

## 2. Contexte de catégorie de marché (intrant)

NOMOS recoupe plusieurs catégories logicielles établies. Ces catégories situent le *domaine*, pas la valeur de NOMOS :

| Catégorie | Description |
|---|---|
| Content / document control régulé | Contenus contrôlés, révisables et auditables en environnement régulé. |
| QMS et validation lifecycle management | Preuve que logiciels et processus restent fit-for-intended-use. |
| Gouvernance IA / RAG | Prouver ce qu'une IA peut utiliser, citer, conserver et restituer. |
| Vertical SaaS pour industries régulées | Logiciels spécialisés intégrés aux opérations. |

Repères de catégorie (références publiques, **pas des comparables directs de NOMOS à son stade alpha**) :

- [Veeva Vault QualityDocs](https://www.veeva.com/products/vault-qualitydocs/) — content management qualité régulé (catégorie GxP mature).
- [ValGenesis](https://www.valgenesis.com/) — validation lifecycle management GxP / life sciences.
- [FDA Computer Software Assurance](https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software-0) — approche risk-based.
- [21 CFR Part 11](https://www.law.cornell.edu/cfr/text/21/part-11) — electronic records / signatures (FDA).

> La capitalisation boursière d'éditeurs matures (par ex. Veeva) relève d'entreprises à revenu récurrent établi et large base installée. Elle n'est **pas transposable** à un projet en alpha sans revenu, et n'est citée que pour situer la catégorie.

## 3. Multiples de valorisation (intrant conditionnel)

Les multiples SaaS publics et privés (souvent exprimés en multiple d'ARR) ne deviennent pertinents qu'**une fois un revenu récurrent établi**, et varient fortement selon croissance, net revenue retention, marge brute, profitabilité, concentration client et valeur stratégique. Référence générale : [SaaS Valuation Multiples](https://saasvaluationmultiple.com/).

> NOMOS n'a, à ce stade, **pas de revenu récurrent** : les multiples d'ARR ne sont pas applicables tels quels. Donnée de contexte uniquement.

## 4. Facteurs qui feraient évoluer une évaluation

Sans poser de chiffre, les leviers usuellement structurants pour ce type d'actif :

- maturité technique et profondeur de preuve (mono-corpus → multi-corpus → formats multiples) ;
- pilotes clients payants ou lettres d'intention ;
- barrières de reproductibilité et différenciation défendable ;
- revenus récurrents et rétention ;
- clôture des écarts régulés (voir [evidence-and-maturity.md](evidence-and-maturity.md), section 5).

## 5. Notes de positionnement commercial (hypothèses, non engageantes)

Le pack de positionnement et pricing DOR-023 est suivi dans [`commercial-positioning-pack.yaml`](../regulated/domain-packs/commercial-positioning/commercial-positioning-pack.yaml). Ces hypothèses de packaging et de pricing sont des **notes de stratégie**, sans revendication de certification, de conformité, de validation régulée, ni de suffisance légale.

---

> Ce document ne contient volontairement **aucune fourchette de valeur de NOMOS**. L'évaluation relève de l'analyste externe, sur la base de [evidence-and-maturity.md](evidence-and-maturity.md) et des cadres ci-dessus.
