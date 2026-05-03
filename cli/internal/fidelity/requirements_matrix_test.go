package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func fullInput() RTMInput {
	return RTMInput{
		Requirements: []Requirement{
			{ID: "REQ-001", Title: "Water damage coverage", Source: "contract.md", Priority: "high"},
			{ID: "REQ-002", Title: "Deductible calculation", Source: "contract.md", Priority: "medium"},
			{ID: "REQ-003", Title: "Roof exclusion", Source: "contract.md", Priority: "low"},
		},
		Implementations: []ImplementationRef{
			{RequirementID: "REQ-001", Path: "core/water.go", Symbol: "EvalWater"},
			{RequirementID: "REQ-002", Path: "core/deductible.go", Symbol: "CalcDeductible"},
			{RequirementID: "REQ-003", Path: "core/exclusions.go"},
		},
		Tests: []TestRef{
			{RequirementID: "REQ-001", Path: "tests/water_test.go", Passing: true},
			{RequirementID: "REQ-002", Path: "tests/deductible_test.go", Passing: true},
			{RequirementID: "REQ-003", Path: "tests/exclusion_test.go", Passing: true},
		},
		Evidence: []EvidenceRef{
			{RequirementID: "REQ-001", ArtifactID: "nomos-report", EvidenceType: "coverage_report"},
			{RequirementID: "REQ-002", ArtifactID: "nomos-report", EvidenceType: "coverage_report"},
			{RequirementID: "REQ-003", ArtifactID: "nomos-report", EvidenceType: "coverage_report"},
		},
	}
}

func TestBuildRTMFullCoverage(t *testing.T) {
	rtm := BuildRTM(fullInput())
	if rtm.Format != RTMFormat {
		t.Fatalf("format: %q", rtm.Format)
	}
	if rtm.TotalRequirements != 3 {
		t.Fatalf("total: %d", rtm.TotalRequirements)
	}
	if rtm.CoveredCount != 3 {
		t.Fatalf("covered: %d", rtm.CoveredCount)
	}
	if rtm.CoverageRatio < 0.99 {
		t.Fatalf("ratio: %f", rtm.CoverageRatio)
	}
	if len(rtm.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(rtm.Findings), rtm.Findings)
	}
	if rtm.HasBlockingFindings() {
		t.Fatal("should have no blocking findings")
	}
}

func TestBuildRTMNoImpl(t *testing.T) {
	input := fullInput()
	input.Implementations = nil
	rtm := BuildRTM(input)
	if rtm.CoveredCount != 0 {
		t.Fatalf("covered: %d", rtm.CoveredCount)
	}
	assertRTMFindingCode(t, rtm, CodeNoImpl)
	if !rtm.HasBlockingFindings() {
		t.Fatal("high-priority missing impl should block")
	}
}

func TestBuildRTMNoTests(t *testing.T) {
	input := fullInput()
	input.Tests = nil
	rtm := BuildRTM(input)
	assertRTMFindingCode(t, rtm, CodeNoTest)
}

func TestBuildRTMNoEvidence(t *testing.T) {
	input := fullInput()
	input.Evidence = nil
	rtm := BuildRTM(input)
	assertRTMFindingCode(t, rtm, CodeNoEvidence)
}

func TestBuildRTMFailingTest(t *testing.T) {
	input := fullInput()
	input.Tests[0].Passing = false
	rtm := BuildRTM(input)
	assertRTMFindingCode(t, rtm, CodeTestFailing)
	// REQ-001 should be partial.
	for _, row := range rtm.Rows {
		if row.Requirement.ID == "REQ-001" {
			if row.TestStatus != TracePartial {
				t.Fatalf("REQ-001 test status: %q", row.TestStatus)
			}
			if row.OverallStatus != TracePartial {
				t.Fatalf("REQ-001 overall: %q", row.OverallStatus)
			}
		}
	}
}

func TestBuildRTMPartialCoverage(t *testing.T) {
	input := RTMInput{
		Requirements: []Requirement{
			{ID: "REQ-001", Title: "Covered", Priority: "high"},
			{ID: "REQ-002", Title: "Partial", Priority: "medium"},
			{ID: "REQ-003", Title: "Missing", Priority: "low"},
		},
		Implementations: []ImplementationRef{
			{RequirementID: "REQ-001", Path: "impl.go"},
			{RequirementID: "REQ-002", Path: "impl.go"},
		},
		Tests: []TestRef{
			{RequirementID: "REQ-001", Path: "test.go", Passing: true},
		},
		Evidence: []EvidenceRef{
			{RequirementID: "REQ-001", ArtifactID: "report", EvidenceType: "coverage"},
		},
	}
	rtm := BuildRTM(input)
	if rtm.CoveredCount != 1 {
		t.Fatalf("covered: %d", rtm.CoveredCount)
	}
	if rtm.PartialCount != 1 {
		t.Fatalf("partial: %d", rtm.PartialCount)
	}
	if rtm.MissingCount != 1 {
		t.Fatalf("missing: %d", rtm.MissingCount)
	}
}

func TestBuildRTMEmpty(t *testing.T) {
	rtm := BuildRTM(RTMInput{})
	if rtm.TotalRequirements != 0 {
		t.Fatalf("total: %d", rtm.TotalRequirements)
	}
	if rtm.CoverageRatio != 0 {
		t.Fatalf("ratio: %f", rtm.CoverageRatio)
	}
}

func TestBuildRTMRowDetails(t *testing.T) {
	rtm := BuildRTM(fullInput())
	row := rtm.Rows[0]
	if row.Requirement.ID != "REQ-001" {
		t.Fatalf("first row: %q", row.Requirement.ID)
	}
	if len(row.Implementations) != 1 {
		t.Fatalf("impls: %d", len(row.Implementations))
	}
	if row.Implementations[0].Symbol != "EvalWater" {
		t.Fatalf("symbol: %q", row.Implementations[0].Symbol)
	}
	if len(row.Tests) != 1 {
		t.Fatalf("tests: %d", len(row.Tests))
	}
	if len(row.Evidence) != 1 {
		t.Fatalf("evidence: %d", len(row.Evidence))
	}
}

func TestBuildRTMMultipleImplsPerReq(t *testing.T) {
	input := RTMInput{
		Requirements: []Requirement{{ID: "REQ-001", Title: "Multi-impl", Priority: "high"}},
		Implementations: []ImplementationRef{
			{RequirementID: "REQ-001", Path: "a.go"},
			{RequirementID: "REQ-001", Path: "b.go"},
		},
		Tests:    []TestRef{{RequirementID: "REQ-001", Path: "t.go", Passing: true}},
		Evidence: []EvidenceRef{{RequirementID: "REQ-001", ArtifactID: "r", EvidenceType: "report"}},
	}
	rtm := BuildRTM(input)
	if len(rtm.Rows[0].Implementations) != 2 {
		t.Fatalf("expected 2 impls, got %d", len(rtm.Rows[0].Implementations))
	}
	if rtm.Rows[0].OverallStatus != TraceCovered {
		t.Fatalf("status: %q", rtm.Rows[0].OverallStatus)
	}
}

func TestBuildRTMHighPriorityBlocking(t *testing.T) {
	input := RTMInput{
		Requirements: []Requirement{{ID: "REQ-001", Title: "Critical", Priority: "critical"}},
	}
	rtm := BuildRTM(input)
	blocking := 0
	for _, f := range rtm.Findings {
		if f.Blocking {
			blocking++
		}
	}
	if blocking == 0 {
		t.Fatal("critical priority missing impl/test should be blocking")
	}
}

func TestBuildRTMLowPriorityNonBlocking(t *testing.T) {
	input := RTMInput{
		Requirements: []Requirement{{ID: "REQ-001", Title: "Low", Priority: "low"}},
	}
	rtm := BuildRTM(input)
	for _, f := range rtm.Findings {
		if f.Code == CodeNoImpl && f.Blocking {
			t.Fatal("low priority missing impl should not be blocking")
		}
	}
}

func TestBuildRTMFindingsSorted(t *testing.T) {
	input := RTMInput{
		Requirements: []Requirement{
			{ID: "REQ-Z", Title: "Z", Priority: "high"},
			{ID: "REQ-A", Title: "A", Priority: "high"},
		},
	}
	rtm := BuildRTM(input)
	for i := 1; i < len(rtm.Findings); i++ {
		if rtm.Findings[i].RequirementID < rtm.Findings[i-1].RequirementID {
			t.Fatalf("findings not sorted: %q before %q",
				rtm.Findings[i-1].RequirementID, rtm.Findings[i].RequirementID)
		}
	}
}

func TestGapSummary(t *testing.T) {
	rtm := BuildRTM(fullInput())
	s := rtm.GapSummary()
	if !strings.Contains(s, "3/3") {
		t.Fatalf("summary: %q", s)
	}
	if !strings.Contains(s, "100%") {
		t.Fatalf("summary: %q", s)
	}
}

func TestGapSummaryEmpty(t *testing.T) {
	rtm := BuildRTM(RTMInput{})
	if rtm.GapSummary() != "no requirements" {
		t.Fatalf("summary: %q", rtm.GapSummary())
	}
}

func TestWriteRTMJSON(t *testing.T) {
	rtm := BuildRTM(fullInput())
	var buf bytes.Buffer
	if err := WriteRTM(&buf, rtm); err != nil {
		t.Fatalf("write: %v", err)
	}
	var decoded RTM
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != RTMFormat {
		t.Fatalf("format: %q", decoded.Format)
	}
	if decoded.CoveredCount != rtm.CoveredCount {
		t.Fatal("round-trip mismatch")
	}
}

func TestOverallStatusLogic(t *testing.T) {
	cases := []struct {
		impl, test, evid TraceStatus
		want             TraceStatus
	}{
		{TraceCovered, TraceCovered, TraceCovered, TraceCovered},
		{TraceMissing, TraceMissing, TraceMissing, TraceMissing},
		{TraceCovered, TraceMissing, TraceCovered, TracePartial},
		{TraceCovered, TracePartial, TraceCovered, TracePartial},
		{TraceMissing, TraceCovered, TraceCovered, TracePartial},
	}
	for _, tc := range cases {
		got := computeOverall(tc.impl, tc.test, tc.evid)
		if got != tc.want {
			t.Fatalf("overall(%s,%s,%s) = %q, want %q", tc.impl, tc.test, tc.evid, got, tc.want)
		}
	}
}

// --- helper ---

func assertRTMFindingCode(t *testing.T, rtm RTM, code string) {
	t.Helper()
	for _, f := range rtm.Findings {
		if f.Code == code {
			return
		}
	}
	t.Fatalf("expected finding code %q in %v", code, rtm.Findings)
}
