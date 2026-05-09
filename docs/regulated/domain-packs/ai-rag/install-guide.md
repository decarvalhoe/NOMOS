# Install Guide — `ai-rag` Domain Pack

This is the first instantiated example of a Nomos domain pack install
guide. It is the AI/RAG governance pack: the structure, automation,
and documentation a customer installs on top of Nomos to assemble
AI/RAG governance evidence in their own environment.

The pack is the customer-install surface for the AI/RAG governance
folder at [`docs/regulated/ai-rag-governance/`](../../ai-rag-governance/)
and the AI/RAG governance template at
[`templates/regulated/ai-rag-governance.md`](../../../../templates/regulated/ai-rag-governance.md).

## 0. Pack Identity

- **Pack id:** `ai-rag`
- **Underlying DOR issue:** [DOR-010](https://github.com/RBOKproject/NOMOS/issues/422)
- **Pack version:** alpha-0.1
- **Status:** alpha — first packaged example for DOR-021. Acceptance
  is "structure-installable"; it is not regulator-approved and is not
  customer-validated.
- **Repository path:** `docs/regulated/domain-packs/ai-rag/`
- **Maintainer:** Nomos regulated-readiness owner (see
  `docs/regulated/product-profiles/nomos.yaml`).

## 1. Intended Use And Out-Of-Scope

- **Intended use.** Help a customer assemble AI/RAG governance
  evidence inside their own repository: deterministic-extraction
  precedence over LLM extraction, source-citation discipline,
  prompt-injection and unsafe-output testing, low-confidence routing
  to `needs_review`, model/provider/version recording, and
  human-or-gate-controlled review for risk-relevant units. The pack
  is a CLI / GitHub-workflow surface; it does not deploy a hosted
  service.
- **Out of scope.** The pack does not certify the customer's AI
  system, does not validate the customer's chosen LLM, does not
  redistribute model weights or training corpora, and does not record
  decisions about acceptable use in the customer's regulated context.
  The customer remains accountable for those.

## 2. Required Customer Inputs

### 2.1 Required References

| Reference id | Title | Acquisition path | Hash captured | Required by gate |
| --- | --- | --- | --- | --- |
| `nomos.public-claim-boundary` | NOMOS public claim boundary | bundled with the pack at [`docs/public-claim-boundary.md`](../../../public-claim-boundary.md) | not_applicable | regulated docs gate |
| `nomos.ai-rag-governance` | NOMOS AI/RAG governance template | bundled with the pack at [`templates/regulated/ai-rag-governance.md`](../../../../templates/regulated/ai-rag-governance.md) | not_applicable | pack-local checklist |
| `customer.intended-use-record` | Customer intended-use record naming the AI/RAG scope | customer document control | sha256 captured at intake | validation checklist 1.x |
| `customer.model-provider-policy` | Customer policy on AI model and provider use, including acceptable-use, retention, and PII handling | customer document control | sha256 captured at intake | validation checklist 1.x |
| `customer.prompt-injection-test-corpus` | Customer-owned corpus of prompt-injection / jailbreak test cases applicable to the deployment | customer document control | sha256 captured at intake | pack-local checklist |

The pack does not redistribute licensed AI standards or model
documentation. The customer acquires any licensed material (for
example, an industry-specific AI risk standard) through the publisher
and points the pack's reference loader at the local artefact through
`references/ai-rag.yaml` (see
[`templates/regulated/licensed-reference-intake.yaml`](../../../../templates/regulated/licensed-reference-intake.yaml)).

### 2.2 Required Workflow Configuration

| Item | Value owned by customer |
| --- | --- |
| Provider | GitHub (the pack's workflows assume GitHub; gitlab/other adopters extend the pack themselves) |
| Branch protection on `main` (or equivalent) | required reviews from the customer's quality and AI/RAG owners; required-status checks include the regulated docs gate |
| Approval reviewers | customer's AI/RAG owner team via CODEOWNERS for `docs/regulated/ai-rag-governance/` and the pack's evidence directory |
| Secrets / variables | none required by the pack itself; customer provides any model-provider keys their own jobs need |
| Self-hosted runners (if any) | only required if the customer's prompt-injection corpus or model-provider connector cannot run on hosted runners |

### 2.3 Required Customer Records

- Customer-integration checklist:
  [`templates/regulated/customer-integration-checklist.md`](../../../../templates/regulated/customer-integration-checklist.md)
  filled in with the AI/RAG scope row populated.
- Intended-use record naming the AI/RAG scope, the model(s) and
  provider(s) in scope, the data classification handled, and the
  customer-side accountable owner.
- Supplier qualification record covering Nomos as a supplier under the
  customer's QMS at the pack-supported version.

## 3. Install Steps

1. **Clone or update Nomos** at the pack-supported version:

   ```bash
   git -C <nomos-checkout> fetch origin
   git -C <nomos-checkout> checkout v0.1.0-alpha
   ```

2. **Configure references.** Copy the licensed-reference template
   and edit it to point at the customer's records named in
   section 2.1:

   ```bash
   cp templates/regulated/licensed-reference-intake.yaml \
      references/ai-rag.yaml
   $EDITOR references/ai-rag.yaml
   ```

3. **Adopt the AI/RAG governance template.** Copy the template into
   the customer repository under
   `docs/regulated/ai-rag-governance/<customer-record>.md` and complete
   the controlled-document markers (`document_id`, `version`,
   `status`, `effective_status`, `owner`, `approver`).

4. **Wire the regulated docs gate as a required check.** Adopt the
   gate's CI invocation as a required-status check on the protected
   branch:

   ```yaml
   # In the customer's GitHub workflow
   - name: regulated docs gate
     run: |
       python scripts/regulated_docs_gate.py \
         --report .regulated-doc-gate/regulated-doc-gate-report.json
   ```

5. **Run the gate locally** before opening the install PR:

   ```bash
   python scripts/regulated_docs_gate.py \
     --report .regulated-doc-gate/regulated-doc-gate-report.json
   ```

   Expect `"status": "passed"` and an empty `findings` array.

6. **Run the pack's smoke verification.** The smoke verification for
   the alpha pack is the docs gate plus a markdown structure check
   that confirms every required AI/RAG control bullet from the
   governance README appears in the customer's adopted record:

   ```bash
   for control in \
     "deterministic extraction has precedence" \
     "generated claims cite source IDs" \
     "prompt-injection and unsafe-output cases are tested" \
     "low-confidence output becomes \`needs_review\`" \
     "model/provider/version" \
     "RAG answers preserve citations" \
     "human review status is retained" ; do
     grep -F "$control" docs/regulated/ai-rag-governance/<customer-record>.md \
       || echo "MISSING: $control"
   done
   ```

   Every control must appear (or be explicitly marked as not
   applicable with rationale) before the validation checklist can be
   signed.

7. **Open a PR** that adds `references/ai-rag.yaml`, the customer's
   adopted AI/RAG governance record, and any pack-required workflow
   files. CODEOWNERS for `docs/regulated/ai-rag-governance/` must
   route to the customer's quality and AI/RAG owners. Approval is
   recorded under the existing approval workflow at
   `docs/regulated/validation-pack/approval-workflow.yaml`.

## 4. Expected Outputs

| Artefact | Path | Producer | Consumer |
| --- | --- | --- | --- |
| Regulated docs gate report | `.regulated-doc-gate/regulated-doc-gate-report.json` | `scripts/regulated_docs_gate.py` | release-evidence bundle |
| Customer AI/RAG governance record | `docs/regulated/ai-rag-governance/<customer-record>.md` | customer (from template) | pack-local checklist; release-evidence bundle |
| Reference loader output | derived from `references/ai-rag.yaml` | customer reference intake | evidence index |

## 5. Gates

| Gate | Command | Pass meaning | Fail meaning |
| --- | --- | --- | --- |
| Regulated docs gate | `python scripts/regulated_docs_gate.py` | YAML and controlled-doc structure pass; no overclaiming patterns. | Findings printed to stdout; exit 1; install incomplete. |
| AI/RAG control checklist | step 6 grep loop above | Every required control appears in the customer's adopted record. | At least one control is missing; the validation checklist cannot be signed. |

## 6. Claim Boundary

This pack helps a customer assemble AI/RAG governance evidence. It
does not certify the customer's AI system, does not validate the
customer's chosen LLM, and does not authorise a Part 11
e-signature-platform claim, an FDA-validated-AI claim, an EU AI Act
conformity claim, or any "bias-free AI" or "guaranteed-safe AI"
claim. See [`claim-boundary.md`](claim-boundary.md) and
[`docs/public-claim-boundary.md`](../../../public-claim-boundary.md).

## 7. Validation Checklist

Customers complete [`validation-checklist.md`](validation-checklist.md)
once per environment install. The checklist must be signed by the
customer-side accountable owner before the pack is treated as
operational.

## 8. Rollback And Recovery

- **Rollback path.** Revert the install PR. Drop `references/ai-rag.yaml`
  and the customer's adopted AI/RAG governance record. Remove the
  regulated docs gate from required-status checks if no other pack
  requires it.
- **Recovery on partial install.** The customer's intended-use record,
  signed customer-integration checklist, and AI/RAG control review
  decisions persist outside the repository; they are not invalidated
  by a partial rollback. The reference loader output and gate report
  must be regenerated before the pack is reinstalled.

## 9. Maintenance

- **Periodic review cadence.** At every Nomos minor release, or
  whenever the customer's AI/RAG scope materially changes (new model,
  new provider, new acceptable-use boundary, new regulated context).
- **Owner.** Customer-side AI/RAG accountable owner.
- **Change log.** Each material change to this guide is recorded in
  `docs/regulated/decisions/` with a decision-record entry.
