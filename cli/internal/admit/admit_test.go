package admit

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
)

func TestAdmit_OutOfScope_NoFiles(t *testing.T) {
	report := detect.Report{FilesScanned: 0}
	result := Admit(report)

	if result.Verdict != VerdictOutOfScope {
		t.Fatalf("expected out_of_scope, got %s", result.Verdict)
	}
	if result.Confidence != ConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", result.Confidence)
	}
}

func TestAdmit_OutOfScope_NoLanguages(t *testing.T) {
	report := detect.Report{FilesScanned: 10}
	result := Admit(report)

	if result.Verdict != VerdictOutOfScope {
		t.Fatalf("expected out_of_scope, got %s", result.Verdict)
	}
}

func TestAdmit_OutOfScope_NoSurfaces(t *testing.T) {
	report := detect.Report{
		FilesScanned: 10,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 5}},
	}
	result := Admit(report)

	if result.Verdict != VerdictOutOfScope {
		t.Fatalf("expected out_of_scope, got %s", result.Verdict)
	}
	if result.Confidence != ConfidenceMedium {
		t.Fatalf("expected medium confidence, got %s", result.Confidence)
	}
}

func TestAdmit_Admitted_FullRepo(t *testing.T) {
	report := detect.Report{
		FilesScanned: 50,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 30}},
		Tools:        []detect.ToolFinding{{Name: "Go modules", Kind: "language-manifest"}},
		CI:           []detect.CIFinding{{Provider: "GitHub Actions"}},
		Surfaces: []detect.SurfaceFinding{
			{Name: "api", Confidence: "high"},
			{Name: "infra", Confidence: "high"},
		},
	}
	result := Admit(report)

	if result.Verdict != VerdictAdmitted {
		t.Fatalf("expected admitted, got %s", result.Verdict)
	}
	if result.Confidence != ConfidenceHigh {
		t.Fatalf("expected high confidence, got %s", result.Confidence)
	}
	if len(result.Remediations) != 0 {
		t.Fatalf("expected no remediations, got %d", len(result.Remediations))
	}
}

func TestAdmit_Admitted_SingleSurface_MediumConfidence(t *testing.T) {
	report := detect.Report{
		FilesScanned: 20,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 15}},
		Tools:        []detect.ToolFinding{{Name: "Go modules", Kind: "language-manifest"}},
		CI:           []detect.CIFinding{{Provider: "GitHub Actions"}},
		Surfaces:     []detect.SurfaceFinding{{Name: "api", Confidence: "high"}},
	}
	result := Admit(report)

	if result.Verdict != VerdictAdmitted {
		t.Fatalf("expected admitted, got %s", result.Verdict)
	}
	if result.Confidence != ConfidenceMedium {
		t.Fatalf("expected medium confidence for single surface, got %s", result.Confidence)
	}
}

func TestAdmit_Partial_MissingCI(t *testing.T) {
	report := detect.Report{
		FilesScanned: 30,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 20}},
		Tools:        []detect.ToolFinding{{Name: "Go modules", Kind: "language-manifest"}},
		Surfaces:     []detect.SurfaceFinding{{Name: "api", Confidence: "high"}},
	}
	result := Admit(report)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial, got %s", result.Verdict)
	}
	if len(result.Remediations) == 0 {
		t.Fatal("expected remediations for missing CI")
	}
	found := false
	for _, r := range result.Remediations {
		if r.Gap == "missing_ci" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing_ci gap in remediations")
	}
}

func TestAdmit_Partial_MissingTools(t *testing.T) {
	report := detect.Report{
		FilesScanned: 30,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 20}},
		CI:           []detect.CIFinding{{Provider: "GitHub Actions"}},
		Surfaces:     []detect.SurfaceFinding{{Name: "api", Confidence: "high"}},
	}
	result := Admit(report)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial, got %s", result.Verdict)
	}
	found := false
	for _, r := range result.Remediations {
		if r.Gap == "missing_tools" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing_tools gap")
	}
}

func TestAdmit_Partial_LowSurfaceConfidence(t *testing.T) {
	report := detect.Report{
		FilesScanned: 30,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 20}},
		Tools:        []detect.ToolFinding{{Name: "Go modules", Kind: "language-manifest"}},
		CI:           []detect.CIFinding{{Provider: "GitHub Actions"}},
		Surfaces:     []detect.SurfaceFinding{{Name: "api", Confidence: "medium"}},
	}
	result := Admit(report)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial, got %s", result.Verdict)
	}
	found := false
	for _, r := range result.Remediations {
		if r.Gap == "low_surface_confidence" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected low_surface_confidence gap")
	}
}

func TestAdmit_Partial_MultipleGaps_LowConfidence(t *testing.T) {
	report := detect.Report{
		FilesScanned: 30,
		Languages:    []detect.LanguageFinding{{Name: "Go", Files: 20}},
		Surfaces:     []detect.SurfaceFinding{{Name: "api", Confidence: "medium"}},
	}
	result := Admit(report)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial, got %s", result.Verdict)
	}
	if result.Confidence != ConfidenceLow {
		t.Fatalf("expected low confidence with 3+ gaps, got %s", result.Confidence)
	}
	if len(result.Remediations) < 3 {
		t.Fatalf("expected at least 3 remediations, got %d", len(result.Remediations))
	}
}

func TestAdmit_Format(t *testing.T) {
	report := detect.Report{FilesScanned: 0}
	result := Admit(report)

	if result.Format != ResultFormat {
		t.Fatalf("expected format %s, got %s", ResultFormat, result.Format)
	}
}

func TestWriteJSON(t *testing.T) {
	result := AdmissionResult{
		Format:     ResultFormat,
		Verdict:    VerdictAdmitted,
		Confidence: ConfidenceHigh,
		Reason:     "test",
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, result); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var decoded AdmissionResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON output: %v", err)
	}
	if decoded.Verdict != VerdictAdmitted {
		t.Fatalf("decoded verdict mismatch: got %s", decoded.Verdict)
	}
	if decoded.Remediations != nil {
		t.Fatal("expected remediations to be omitted from JSON")
	}
}

func TestWriteJSON_WithRemediations(t *testing.T) {
	result := AdmissionResult{
		Format:     ResultFormat,
		Verdict:    VerdictPartial,
		Confidence: ConfidenceMedium,
		Reason:     "gaps found",
		Remediations: []Remediation{
			{Gap: "missing_ci", Description: "no CI"},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, result); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	var decoded AdmissionResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode JSON output: %v", err)
	}
	if len(decoded.Remediations) != 1 {
		t.Fatalf("expected 1 remediation, got %d", len(decoded.Remediations))
	}
	if decoded.Remediations[0].Gap != "missing_ci" {
		t.Fatalf("expected missing_ci gap, got %s", decoded.Remediations[0].Gap)
	}
}
