package fidelity

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const RTMFormat = "nomos.requirements-traceability-matrix.v1"

// TraceStatus tracks coverage of each traceability link.
type TraceStatus string

const (
	TraceCovered    TraceStatus = "covered"
	TracePartial    TraceStatus = "partial"
	TraceMissing    TraceStatus = "missing"
	TraceNotApplicable TraceStatus = "not_applicable"
)

// Requirement is a single requirement in the matrix.
type Requirement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Source      string `json:"source"`
	Priority    string `json:"priority"`
	Domain      string `json:"domain,omitempty"`
}

// ImplementationRef links a requirement to its implementation.
type ImplementationRef struct {
	RequirementID string `json:"requirement_id"`
	Path          string `json:"path"`
	Symbol        string `json:"symbol,omitempty"`
	Description   string `json:"description,omitempty"`
}

// TestRef links a requirement to its test evidence.
type TestRef struct {
	RequirementID string `json:"requirement_id"`
	Path          string `json:"path"`
	TestName      string `json:"test_name,omitempty"`
	Passing       bool   `json:"passing"`
}

// EvidenceRef links a requirement to its generated evidence.
type EvidenceRef struct {
	RequirementID string `json:"requirement_id"`
	ArtifactID    string `json:"artifact_id"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
	EvidenceType  string `json:"evidence_type"`
}

// TraceRow is one row in the traceability matrix.
type TraceRow struct {
	Requirement     Requirement       `json:"requirement"`
	Implementations []ImplementationRef `json:"implementations"`
	Tests           []TestRef          `json:"tests"`
	Evidence        []EvidenceRef      `json:"evidence"`
	ImplStatus      TraceStatus        `json:"impl_status"`
	TestStatus      TraceStatus        `json:"test_status"`
	EvidenceStatus  TraceStatus        `json:"evidence_status"`
	OverallStatus   TraceStatus        `json:"overall_status"`
}

// RTM is the full requirements traceability matrix.
type RTM struct {
	Format           string     `json:"format"`
	TotalRequirements int       `json:"total_requirements"`
	CoveredCount     int        `json:"covered_count"`
	PartialCount     int        `json:"partial_count"`
	MissingCount     int        `json:"missing_count"`
	CoverageRatio    float64    `json:"coverage_ratio"`
	Rows             []TraceRow `json:"rows"`
	Findings         []RTMFinding `json:"findings,omitempty"`
}

// RTMFinding describes a traceability gap.
type RTMFinding struct {
	RequirementID string `json:"requirement_id"`
	Code          string `json:"code"`
	Severity      string `json:"severity"`
	Message       string `json:"message"`
	Blocking      bool   `json:"blocking"`
}

const (
	CodeNoImpl     = "NO_IMPLEMENTATION"
	CodeNoTest     = "NO_TEST"
	CodeNoEvidence = "NO_EVIDENCE"
	CodeTestFailing = "TEST_FAILING"
)

// RTMInput provides the raw data for matrix construction.
type RTMInput struct {
	Requirements    []Requirement
	Implementations []ImplementationRef
	Tests           []TestRef
	Evidence        []EvidenceRef
}

// BuildRTM constructs the traceability matrix from input data.
func BuildRTM(input RTMInput) RTM {
	// Index by requirement ID.
	implByReq := groupBy(input.Implementations, func(r ImplementationRef) string { return r.RequirementID })
	testByReq := groupBy(input.Tests, func(r TestRef) string { return r.RequirementID })
	evidByReq := groupBy(input.Evidence, func(r EvidenceRef) string { return r.RequirementID })

	var rows []TraceRow
	var findings []RTMFinding
	covered, partial, missing := 0, 0, 0

	for _, req := range input.Requirements {
		impls := implByReq[req.ID]
		tests := testByReq[req.ID]
		evids := evidByReq[req.ID]

		implStatus := computeImplStatus(impls)
		testStatus := computeTestStatus(tests)
		evidStatus := computeEvidenceStatus(evids)
		overall := computeOverall(implStatus, testStatus, evidStatus)

		row := TraceRow{
			Requirement:     req,
			Implementations: toImplSlice(impls),
			Tests:           toTestSlice(tests),
			Evidence:        toEvidSlice(evids),
			ImplStatus:      implStatus,
			TestStatus:      testStatus,
			EvidenceStatus:  evidStatus,
			OverallStatus:   overall,
		}
		rows = append(rows, row)

		// Generate findings for gaps.
		if implStatus == TraceMissing {
			findings = append(findings, RTMFinding{
				RequirementID: req.ID, Code: CodeNoImpl, Severity: severityFor(req.Priority),
				Message: fmt.Sprintf("requirement %q has no implementation", req.ID), Blocking: isHighPriority(req.Priority),
			})
		}
		if testStatus == TraceMissing {
			findings = append(findings, RTMFinding{
				RequirementID: req.ID, Code: CodeNoTest, Severity: severityFor(req.Priority),
				Message: fmt.Sprintf("requirement %q has no test", req.ID), Blocking: isHighPriority(req.Priority),
			})
		}
		if evidStatus == TraceMissing {
			findings = append(findings, RTMFinding{
				RequirementID: req.ID, Code: CodeNoEvidence, Severity: "medium",
				Message: fmt.Sprintf("requirement %q has no evidence artifact", req.ID),
			})
		}
		if testStatus == TracePartial {
			findings = append(findings, RTMFinding{
				RequirementID: req.ID, Code: CodeTestFailing, Severity: "high",
				Message: fmt.Sprintf("requirement %q has failing test(s)", req.ID), Blocking: true,
			})
		}

		switch overall {
		case TraceCovered:
			covered++
		case TracePartial:
			partial++
		case TraceMissing:
			missing++
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].RequirementID < findings[j].RequirementID
	})

	ratio := 0.0
	total := len(input.Requirements)
	if total > 0 {
		ratio = float64(covered) / float64(total)
	}

	return RTM{
		Format:            RTMFormat,
		TotalRequirements: total,
		CoveredCount:      covered,
		PartialCount:      partial,
		MissingCount:      missing,
		CoverageRatio:     ratio,
		Rows:              rows,
		Findings:          findings,
	}
}

// WriteRTM writes the matrix as indented JSON.
func WriteRTM(w io.Writer, rtm RTM) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rtm)
}

// HasBlockingFindings returns true if any finding is blocking.
func (r RTM) HasBlockingFindings() bool {
	for _, f := range r.Findings {
		if f.Blocking {
			return true
		}
	}
	return false
}

// GapSummary returns a human-readable gap summary.
func (r RTM) GapSummary() string {
	if r.TotalRequirements == 0 {
		return "no requirements"
	}
	return fmt.Sprintf("%d/%d covered (%.0f%%), %d partial, %d missing, %d findings",
		r.CoveredCount, r.TotalRequirements, r.CoverageRatio*100,
		r.PartialCount, r.MissingCount, len(r.Findings))
}

func computeImplStatus(impls []interface{}) TraceStatus {
	if len(impls) == 0 {
		return TraceMissing
	}
	return TraceCovered
}

func computeTestStatus(tests []interface{}) TraceStatus {
	if len(tests) == 0 {
		return TraceMissing
	}
	allPassing := true
	for _, ti := range tests {
		t := ti.(TestRef)
		if !t.Passing {
			allPassing = false
		}
	}
	if allPassing {
		return TraceCovered
	}
	return TracePartial
}

func computeEvidenceStatus(evids []interface{}) TraceStatus {
	if len(evids) == 0 {
		return TraceMissing
	}
	return TraceCovered
}

func computeOverall(impl, test, evid TraceStatus) TraceStatus {
	if impl == TraceCovered && test == TraceCovered && evid == TraceCovered {
		return TraceCovered
	}
	if impl == TraceMissing && test == TraceMissing && evid == TraceMissing {
		return TraceMissing
	}
	return TracePartial
}

func severityFor(priority string) string {
	switch strings.ToLower(priority) {
	case "critical", "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func isHighPriority(priority string) bool {
	p := strings.ToLower(priority)
	return p == "critical" || p == "high"
}

// groupBy groups a slice of items by a key function.
func groupBy[T any](items []T, keyFn func(T) string) map[string][]interface{} {
	m := map[string][]interface{}{}
	for _, item := range items {
		key := keyFn(item)
		m[key] = append(m[key], item)
	}
	return m
}

func toImplSlice(items []interface{}) []ImplementationRef {
	result := make([]ImplementationRef, 0, len(items))
	for _, i := range items {
		result = append(result, i.(ImplementationRef))
	}
	return result
}

func toTestSlice(items []interface{}) []TestRef {
	result := make([]TestRef, 0, len(items))
	for _, i := range items {
		result = append(result, i.(TestRef))
	}
	return result
}

func toEvidSlice(items []interface{}) []EvidenceRef {
	result := make([]EvidenceRef, 0, len(items))
	for _, i := range items {
		result = append(result, i.(EvidenceRef))
	}
	return result
}
