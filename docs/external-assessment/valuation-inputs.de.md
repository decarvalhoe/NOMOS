# Bewertungs-Inputs — neutrale Rahmenwerke fuer externe Bewertung

> Sprachen: [FR](valuation-inputs.md) · [EN](valuation-inputs.en.md) · **DE**

> Dieses Dokument sammelt **neutrale Rahmenwerke und Referenzpunkte**, die eine externe Analyse selbst anwenden kann. Es schlaegt **keine Wertspanne** vor, keine Selbstbewertung, und platziert NOMOS auf keiner Wertskala. Die zitierten Marktvergleiche sind **Kategorie-Orientierungspunkte**, keine direkten Vergleichswerte fuer NOMOS in seinem aktuellen Stadium (Alpha — siehe [evidence-and-maturity.de.md](evidence-and-maturity.de.md)).
>
> Fuer den tatsaechlichen Produktstand siehe [evidence-and-maturity.de.md](evidence-and-maturity.de.md). Fuer Claim-Grenzen siehe [public-claim-boundary.md](../public-claim-boundary.md).

## Warum dieses Dokument getrennt ist

Die Bewertung eines Projekts im fruehen Stadium haengt von Annahmen ab (Reife, Umsatz, Piloten, Retention, Reproduktionsbarrieren, strategischer Wert), die **nur ein unabhaengiger Bewerter treffen sollte**. Um die Unparteilichkeit der Analyse zu wahren, liefert dieses Repository die *Inputs*, faellt aber kein Werturteil.

## 1. Buchhalterische Aktivierungsrahmen (Input)

Entwicklungskosten eines intern entwickelten immateriellen Vermoegenswerts duerfen nur aktiviert werden, wenn die anwendbaren Kriterien erfuellt sind: technische Machbarkeit, Absicht zur Fertigstellung, Nutzungs- oder Verkaufsfaehigkeit, wahrscheinlicher kuenftiger wirtschaftlicher Nutzen, verfuegbare Ressourcen und verlaessliche Kostenmessung. Standards:

- [IAS 38 — Intangible Assets](https://www.ifrs.org/issued-standards/list-of-standards/ias-38-intangible-assets/)
- [Swiss GAAP FER 10 — Immaterielle Werte](https://www.fer.ch/en/standards/swiss-gaap-fer-10-immaterielle-werte/)

Potenziell aktivierbare Elemente, durch die Analyse zu beurteilen: Entwicklungszeit, Architektur, Tests, Dokumentation, CI, Validation Records, direkt zurechenbares Tooling und Infrastruktur. Die entsprechende faktische Inventur ist in [evidence-and-maturity.de.md](evidence-and-maturity.de.md).

## 2. Marktkategorie-Kontext (Input)

NOMOS ueberschneidet mehrere etablierte Softwarekategorien. Diese Kategorien verorten die *Domaene*, nicht den Wert von NOMOS:

| Kategorie | Beschreibung |
|---|---|
| Reguliertes Content-/Document-Control | Kontrollierte, reviewbare, auditierbare Inhalte in regulierten Umgebungen. |
| QMS und Validation Lifecycle Management | Evidenz, dass Software und Prozesse fit-for-intended-use bleiben. |
| AI-/RAG-Governance | Nachweis, was eine KI verwenden, zitieren, speichern und beantworten darf. |
| Vertical SaaS fuer regulierte Industrien | In operative Prozesse eingebettete spezialisierte Software. |

Kategorie-Orientierungspunkte (oeffentliche Referenzen, **keine direkten Vergleichswerte fuer NOMOS im Alpha-Stadium**):

- [Veeva Vault QualityDocs](https://www.veeva.com/products/vault-qualitydocs/) — reguliertes Quality Content Management (reife GxP-Kategorie).
- [ValGenesis](https://www.valgenesis.com/) — Validation Lifecycle Management fuer GxP / Life Sciences.
- [FDA Computer Software Assurance](https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software-0) — risikobasierter Ansatz.
- [21 CFR Part 11](https://www.law.cornell.edu/cfr/text/21/part-11) — elektronische Records / Signaturen (FDA).

> Die Marktkapitalisierung reifer Anbieter (z. B. Veeva) spiegelt Unternehmen mit etabliertem wiederkehrendem Umsatz und grosser installierter Basis wider. Sie ist **nicht uebertragbar** auf ein Alpha-Projekt ohne Umsatz und wird nur zur Verortung der Kategorie zitiert.

## 3. Bewertungsmultiples (bedingter Input)

Oeffentliche und private SaaS-Multiples (oft als Vielfaches des ARR ausgedrueckt) werden erst relevant, **sobald wiederkehrender Umsatz besteht**, und variieren stark nach Wachstum, Net Revenue Retention, Bruttomarge, Profitabilitaet, Kundenkonzentration und strategischem Wert. Allgemeine Referenz: [SaaS Valuation Multiples](https://saasvaluationmultiple.com/).

> NOMOS hat in diesem Stadium **keinen wiederkehrenden Umsatz**: ARR-Multiples sind nicht unmittelbar anwendbar. Nur Kontext.

## 4. Faktoren, die eine Bewertung bewegen wuerden

Ohne eine Zahl zu nennen, die fuer diese Art von Vermoegenswert ueblicherweise strukturierenden Hebel:

- technische Reife und Beweistiefe (Single-Corpus → Multi-Corpus → mehrere Formate);
- bezahlte Kundenpiloten oder Letters of Intent;
- Reproduktionsbarrieren und verteidigbare Differenzierung;
- wiederkehrender Umsatz und Retention;
- Schliessung der regulierten Luecken (siehe [evidence-and-maturity.de.md](evidence-and-maturity.de.md), Abschnitt 5).

## 5. Notizen zur kommerziellen Positionierung (Annahmen, unverbindlich)

Das DOR-023-Positionierungs- und Pricing-Pack wird in [`commercial-positioning-pack.yaml`](../regulated/domain-packs/commercial-positioning/commercial-positioning-pack.yaml) verfolgt. Diese Packaging- und Pricing-Annahmen sind **Strategie-Notizen**, ohne Anspruch auf Zertifizierung, Konformitaet, regulierte Validierung oder rechtliche Hinlaenglichkeit.

---

> Dieses Dokument enthaelt bewusst **keine Wertspanne fuer NOMOS**. Die Bewertung liegt bei der externen Analyse, auf Basis von [evidence-and-maturity.de.md](evidence-and-maturity.de.md) und den obigen Rahmenwerken.
