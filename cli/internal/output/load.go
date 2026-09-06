package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/docload"
)

// ReportSchema is the nomos-report contract version this engine writes.
const ReportSchema = "0.1.0"

// validVerdictStatuses are the report gate statuses this engine writes
// (cli/internal/report/generate.go): pass, warn, fail.
var validVerdictStatuses = map[string]bool{"pass": true, "warn": true, "fail": true}

// LoadReport is the engine's loader for a nomos-report.json: schema version
// and verdict status are checked against what this engine writes.
func LoadReport(path string) (Report, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("read report %s: %w", path, err)
	}
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return Report{}, fmt.Errorf("decode report %s: %w", path, err)
	}
	if r.SchemaVersion != ReportSchema {
		return Report{}, fmt.Errorf("report %s: schema_version %q, this engine reads %s", path, r.SchemaVersion, ReportSchema)
	}
	if !validVerdictStatuses[r.Verdict.Status] {
		return Report{}, fmt.Errorf("report %s: verdict status %q is not one this engine writes (pass|warn|fail)", path, r.Verdict.Status)
	}
	return r, nil
}

// Verdict vocabularies mirror specs/verdicts.cue (the stable `verdicts`
// contract): the product scope verdicts, the corpus admission verdicts, and the
// confidence and escalation levels a verdict case carries.
var (
	VerdictNames       = map[string]bool{"in_scope": true, "partial": true, "blocked": true, "out_of_scope": true}
	CorpusVerdictNames = map[string]bool{"corpus_admissible": true, "corpus_partial": true, "corpus_blocked": true}
	ConfidenceLevels   = map[string]bool{"low": true, "medium": true, "high": true}
	EscalationLevels   = map[string]bool{"none": true, "domain_owner": true, "product_owner": true, "compliance_owner": true}
)

// VerdictCase is one labelled case of the verdicts contract.
type VerdictCase struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Verdict        string `json:"verdict"`
	Confidence     string `json:"confidence"`
	Escalation     string `json:"escalation"`
	ExpectedAction string `json:"expected_action"`
}

// VerdictCases is a verdict case list (specs/examples/verdict-cases.yaml).
type VerdictCases struct {
	SchemaVersion string        `json:"schema_version"`
	Cases         []VerdictCase `json:"cases"`
}

// LoadVerdictCases is the engine's reader for a verdict case list: every case
// must carry a verdict, confidence and escalation from the contract's
// vocabularies. Before NRT-031 no Go code read this document; the vocabulary
// above is what `nomos diagnose`/`nomos corpus` emit.
func LoadVerdictCases(path string) (VerdictCases, error) {
	var doc VerdictCases
	if err := docload.Load(path, &doc); err != nil {
		return VerdictCases{}, err
	}
	if doc.SchemaVersion != ReportSchema {
		return VerdictCases{}, fmt.Errorf("%s: schema_version %q, this engine reads %s", path, doc.SchemaVersion, ReportSchema)
	}
	if len(doc.Cases) == 0 {
		return VerdictCases{}, fmt.Errorf("%s: no cases", path)
	}
	for _, c := range doc.Cases {
		if !VerdictNames[c.Verdict] && !CorpusVerdictNames[c.Verdict] {
			return VerdictCases{}, fmt.Errorf("%s: case %s: verdict %q is not in the verdicts contract", path, c.ID, c.Verdict)
		}
		if !ConfidenceLevels[c.Confidence] || !EscalationLevels[c.Escalation] {
			return VerdictCases{}, fmt.Errorf("%s: case %s: confidence %q / escalation %q outside the contract", path, c.ID, c.Confidence, c.Escalation)
		}
	}
	return doc, nil
}
