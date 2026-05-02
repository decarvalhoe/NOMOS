# Regulated Templates

These templates are starter records for regulated-by-design Nomos and Praxis work.

They do not certify compliance. They make claims, evidence, risks, and release gates explicit enough to be reviewed and automated later.

## Template Use

1. Copy the relevant template into `docs/regulated/`.
2. Fill every field or mark it `not_applicable` with rationale.
3. Link controls to issues, implementation, tests, and evidence.
4. Keep claim status honest: `not_qualified`, `planned`, `implemented`, `verified`, `approved`, `waived`, or `blocked`.
5. Do not close a regulated issue without updated evidence.

## Available Templates

- `controlled-document.yaml` - metadata skeleton for controlled documents.
- `licensed-reference-intake.yaml` - intake record for GAMP 5, ISO and other licensed bible sources processed outside Git.
- `regulated-product-profile.yaml` - product intended use, claim boundary and evidence ownership.
- `intended-use.yaml` - intended-use and risk framing record.
- `control-matrix.yaml` - external reference to control to evidence mapping.
- `traceability-matrix.yaml` - requirement, implementation, verification and evidence traceability.
- `validation-plan.md` - validation planning document.
- `validation-protocol.yaml` - executable validation protocol record.
- `validation-summary-report.yaml` - validation close-out report record.
- `deviation-capa-record.yaml` - deviation and CAPA record.
- `training-record.yaml` - training and competence evidence record.
- `release-evidence-bundle.yaml` - release evidence inventory.
- `alcoa-evidence-envelope.yaml` - ALCOA+ evidence metadata envelope.
- `atomization-certification-report.yaml` - atomization coverage and certification record.
- `supplier-assurance-pack.md` - supplier assurance structure.
- `customer-integration-checklist.md` - customer shared-responsibility checklist.
- `periodic-review.md` - periodic review record.
- `ai-rag-governance.md` - AI/RAG governance checklist.
