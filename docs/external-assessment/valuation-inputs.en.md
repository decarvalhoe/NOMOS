# Valuation inputs — neutral frameworks for external assessment

> Languages: [FR](valuation-inputs.md) · **EN** · [DE](valuation-inputs.de.md)

> This document gathers **neutral frameworks and reference points** an external analyst can apply themselves. It proposes **no value range**, no self-assessment, and does not place NOMOS on any value scale. The market comparables cited are **category landmarks**, not direct comparables for NOMOS at its current stage (alpha — see [evidence-and-maturity.en.md](evidence-and-maturity.en.md)).
>
> For the actual product state, see [evidence-and-maturity.en.md](evidence-and-maturity.en.md). For claim limits, see [public-claim-boundary.md](../public-claim-boundary.md).

## Why this document is separate

Valuing an early-stage project depends on assumptions (maturity, revenue, pilots, retention, reproducibility barriers, strategic value) that **only an independent evaluator should make**. To preserve the impartiality of the analysis, this repository provides the *inputs* but states no value verdict.

## 1. Accounting capitalization frameworks (input)

Development costs of an internally generated intangible asset may be capitalized only when the applicable criteria are met: technical feasibility, intent to complete, ability to use or sell, probable future economic benefit, available resources, and reliable cost measurement. Standards:

- [IAS 38 — Intangible Assets](https://www.ifrs.org/issued-standards/list-of-standards/ias-38-intangible-assets/)
- [Swiss GAAP FER 10 — Intangible assets](https://www.fer.ch/en/standards/swiss-gaap-fer-10-immaterielle-werte/)

Potentially eligible items, for the analyst to assess: development time, architecture, tests, documentation, CI, validation records, directly attributable tooling and infrastructure. The corresponding factual inventory is in [evidence-and-maturity.en.md](evidence-and-maturity.en.md).

## 2. Market category context (input)

NOMOS intersects several established software categories. These categories situate the *domain*, not NOMOS's value:

| Category | Description |
|---|---|
| Regulated content / document control | Controlled, reviewable, auditable content in regulated environments. |
| QMS and validation lifecycle management | Evidence that software and processes remain fit-for-intended-use. |
| AI / RAG governance | Proving what an AI may use, cite, retain, and answer from. |
| Vertical SaaS for regulated industries | Specialized software embedded in operating processes. |

Category landmarks (public references, **not direct comparables for NOMOS at its alpha stage**):

- [Veeva Vault QualityDocs](https://www.veeva.com/products/vault-qualitydocs/) — regulated quality content management (mature GxP category).
- [ValGenesis](https://www.valgenesis.com/) — validation lifecycle management for GxP / life sciences.
- [FDA Computer Software Assurance](https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software-0) — risk-based approach.
- [21 CFR Part 11](https://www.law.cornell.edu/cfr/text/21/part-11) — electronic records / signatures (FDA).

> The market capitalization of mature vendors (e.g. Veeva) reflects companies with established recurring revenue and a large installed base. It is **not transposable** to an alpha-stage project with no revenue, and is cited only to situate the category.

## 3. Valuation multiples (conditional input)

Public and private SaaS multiples (often expressed as a multiple of ARR) become relevant only **once recurring revenue exists**, and vary widely with growth, net revenue retention, gross margin, profitability, customer concentration, and strategic value. General reference: [SaaS Valuation Multiples](https://saasvaluationmultiple.com/).

> NOMOS has, at this stage, **no recurring revenue**: ARR multiples are not applicable as-is. Context only.

## 4. Factors that would move a valuation

Without stating a number, the levers that usually structure this kind of asset:

- technical maturity and depth of proof (single-corpus → multi-corpus → multiple formats);
- paid customer pilots or letters of intent;
- reproducibility barriers and defensible differentiation;
- recurring revenue and retention;
- closure of regulated gaps (see [evidence-and-maturity.en.md](evidence-and-maturity.en.md), section 5).

## 5. Commercial positioning notes (assumptions, non-binding)

The DOR-023 positioning and pricing pack is tracked in [`commercial-positioning-pack.yaml`](../regulated/domain-packs/commercial-positioning/commercial-positioning-pack.yaml). These packaging and pricing assumptions are **strategy notes**, with no claim of certification, compliance, regulated validation, or legal sufficiency.

---

> This document deliberately contains **no value range for NOMOS**. Valuation is the external analyst's responsibility, based on [evidence-and-maturity.en.md](evidence-and-maturity.en.md) and the frameworks above.
