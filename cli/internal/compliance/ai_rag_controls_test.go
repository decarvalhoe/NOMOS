package compliance

import (
	"errors"
	"testing"
)

func TestEvaluateAllPass(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.98,
		ConfidenceScore:    0.92,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc123",
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if !eval.Verdict.Pass {
		t.Fatalf("expected pass, got: %s", eval.Verdict.Summary)
	}
	if eval.Verdict.Blocking {
		t.Fatal("expected non-blocking")
	}
	for _, r := range eval.Controls {
		if r.Status == "failed" {
			t.Fatalf("unexpected failure: %s - %s", r.ControlID, r.Message)
		}
	}
}

func TestEvaluateHallucinationFails(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.50, // below 0.95 threshold
		ConfidenceScore:    0.92,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected fail due to low citation rate")
	}

	found := findResult(eval, "ai.hallucination-guard")
	if found.Status != "failed" {
		t.Fatalf("expected hallucination guard to fail, got %s", found.Status)
	}
}

func TestEvaluateCitationMissing(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       false,
		CitationRate:       0.0,
		ConfidenceScore:    0.99,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected fail due to missing citations")
	}

	found := findResult(eval, "ai.citation-requirement")
	if found.Status != "failed" {
		t.Fatalf("expected citation control to fail, got %s", found.Status)
	}
}

func TestEvaluateConfidenceBelowThreshold(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.99,
		ConfidenceScore:    0.55, // below 0.8
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected fail due to low confidence")
	}

	found := findResult(eval, "ai.confidence-threshold")
	if found.Status != "failed" {
		t.Fatalf("expected confidence control to fail, got %s", found.Status)
	}
}

func TestEvaluateHumanReviewPending(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.99,
		ConfidenceScore:    0.95,
		HumanReviewStatus:  "pending",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected fail due to pending review")
	}

	found := findResult(eval, "ai.human-review-gate")
	if found.Status != "failed" {
		t.Fatalf("expected human review to fail, got %s", found.Status)
	}
}

func TestEvaluateHumanReviewRejected(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:      true,
		CitationRate:      0.99,
		ConfidenceScore:   0.95,
		HumanReviewStatus: "rejected",
		ProvenanceHash:    "sha256:abc",
	}

	eval := Evaluate(controls, input)
	found := findResult(eval, "ai.human-review-gate")
	if found.Status != "failed" {
		t.Fatalf("expected rejected review to fail, got %s", found.Status)
	}
}

func TestEvaluateProvenanceMissing(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.99,
		ConfidenceScore:    0.95,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "", // missing
		InjectionTestsPass: true,
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected fail due to missing provenance")
	}

	found := findResult(eval, "ai.data-provenance")
	if found.Status != "failed" {
		t.Fatalf("expected provenance control to fail, got %s", found.Status)
	}
}

func TestEvaluateInjectionWarningNotBlocking(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.99,
		ConfidenceScore:    0.95,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: false, // fails but gate_mode=warning
		RefusalTestsPass:   true,
	}

	eval := Evaluate(controls, input)
	if !eval.Verdict.Pass {
		t.Fatal("expected pass — injection tests are warning-only, not blocking")
	}

	found := findResult(eval, "ai.injection-tests")
	if found.Status != "warning" {
		t.Fatalf("expected warning status, got %s", found.Status)
	}
}

func TestEvaluateRefusalWarningNotBlocking(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:       true,
		CitationRate:       0.99,
		ConfidenceScore:    0.95,
		HumanReviewStatus:  "approved",
		ProvenanceHash:     "sha256:abc",
		InjectionTestsPass: true,
		RefusalTestsPass:   false, // warning-only
	}

	eval := Evaluate(controls, input)
	if !eval.Verdict.Pass {
		t.Fatal("expected pass — refusal tests are warning-only")
	}

	found := findResult(eval, "ai.refusal-behavior")
	if found.Status != "warning" {
		t.Fatalf("expected warning, got %s", found.Status)
	}
}

func TestGateCheckPassReturnsNil(t *testing.T) {
	eval := AIRAGEvaluation{Verdict: EvalVerdict{Pass: true}}
	if err := GateCheck(eval); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestGateCheckFailReturnsError(t *testing.T) {
	eval := AIRAGEvaluation{Verdict: EvalVerdict{Pass: false, Blocking: true, Summary: "2 blocking control(s) failed."}}
	err := GateCheck(eval)
	if !errors.Is(err, ErrAIRAGGateFailed) {
		t.Fatalf("expected ErrAIRAGGateFailed, got: %v", err)
	}
}

func TestDefaultControlsHasExpectedCount(t *testing.T) {
	controls := DefaultControls()
	if len(controls) != 7 {
		t.Fatalf("expected 7 default controls, got %d", len(controls))
	}
}

func TestEvaluateMultipleBlockingFailures(t *testing.T) {
	controls := DefaultControls()
	input := EvalInput{
		HasCitations:      false,
		CitationRate:      0.0,
		ConfidenceScore:   0.1,
		HumanReviewStatus: "",
		ProvenanceHash:    "",
	}

	eval := Evaluate(controls, input)
	if eval.Verdict.Pass {
		t.Fatal("expected total failure")
	}

	failCount := 0
	for _, r := range eval.Controls {
		if r.Status == "failed" {
			failCount++
		}
	}
	if failCount < 5 {
		t.Fatalf("expected at least 5 failures, got %d", failCount)
	}
}

func findResult(eval AIRAGEvaluation, controlID string) ControlResult {
	for _, r := range eval.Controls {
		if r.ControlID == controlID {
			return r
		}
	}
	return ControlResult{ControlID: controlID, Status: "not_found"}
}
