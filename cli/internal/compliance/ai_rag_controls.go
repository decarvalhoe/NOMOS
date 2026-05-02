package compliance

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrAIRAGGateFailed = errors.New("AI/RAG control gate failed")
)

// ControlCategory classifies the risk domain of a control.
type ControlCategory string

const (
	CategoryHallucination ControlCategory = "hallucination"
	CategoryCitation      ControlCategory = "citation"
	CategoryConfidence    ControlCategory = "confidence"
	CategoryHumanReview   ControlCategory = "human_review"
	CategoryProvenance    ControlCategory = "provenance"
	CategoryInjection     ControlCategory = "injection"
	CategoryRefusal       ControlCategory = "refusal"
)

// GateMode determines the enforcement behavior of a control.
type GateMode string

const (
	GateBlocking      GateMode = "blocking"
	GateWarning       GateMode = "warning"
	GateInformational GateMode = "informational"
)

// AIRAGControl defines a single risk control for AI/RAG usage.
type AIRAGControl struct {
	ID               string          `json:"id" yaml:"id"`
	Name             string          `json:"name" yaml:"name"`
	Category         ControlCategory `json:"category" yaml:"category"`
	Level            string          `json:"level" yaml:"level"`
	Status           string          `json:"status" yaml:"status"`
	Description      string          `json:"description" yaml:"description"`
	GateMode         GateMode        `json:"gate_mode" yaml:"gate_mode"`
	Threshold        *float64        `json:"threshold,omitempty" yaml:"threshold,omitempty"`
	EvidenceRequired bool            `json:"evidence_required" yaml:"evidence_required"`
	Remediation      string          `json:"remediation,omitempty" yaml:"remediation,omitempty"`
}

// ControlResult is the outcome of evaluating one control.
type ControlResult struct {
	ControlID  string  `json:"control_id"`
	Status     string  `json:"status"`
	Score      float64 `json:"score,omitempty"`
	EvidenceID string  `json:"evidence_id,omitempty"`
	Message    string  `json:"message,omitempty"`
}

// EvalVerdict is the overall pass/fail decision.
type EvalVerdict struct {
	Pass     bool   `json:"pass"`
	Blocking bool   `json:"blocking"`
	Summary  string `json:"summary"`
}

// AIRAGEvaluation records results of running all controls.
type AIRAGEvaluation struct {
	SchemaVersion string          `json:"schema_version"`
	EvaluatedAt   string          `json:"evaluated_at"`
	Evaluator     string          `json:"evaluator"`
	Controls      []ControlResult `json:"controls"`
	Verdict       EvalVerdict     `json:"verdict"`
}

// EvalInput provides the data needed to evaluate controls.
type EvalInput struct {
	HasCitations       bool
	CitationRate       float64 // 0.0-1.0: fraction of outputs with source citations
	ConfidenceScore    float64 // 0.0-1.0: model confidence
	HumanReviewStatus  string  // "approved", "pending", "rejected", ""
	ProvenanceHash     string  // non-empty if provenance tracked
	InjectionTestsPass bool
	RefusalTestsPass   bool
}

// DefaultControls returns the standard AI/RAG control set for Nomos.
func DefaultControls() []AIRAGControl {
	t80 := 0.8
	t95 := 0.95
	return []AIRAGControl{
		{
			ID:               "ai.hallucination-guard",
			Name:             "Hallucination guard",
			Category:         CategoryHallucination,
			Level:            "critical",
			Status:           "enforced",
			Description:      "AI-generated claims must cite a verifiable source unit. Unsourced claims are flagged as potential hallucinations.",
			GateMode:         GateBlocking,
			Threshold:        &t95,
			EvidenceRequired: true,
			Remediation:      "Add source_id and source_hash to every AI-generated unit before it enters canonical corpus.",
		},
		{
			ID:               "ai.citation-requirement",
			Name:             "Citation requirement",
			Category:         CategoryCitation,
			Level:            "critical",
			Status:           "enforced",
			Description:      "Every AI/RAG answer must include at least one canonical citation with display_ref.",
			GateMode:         GateBlocking,
			EvidenceRequired: true,
			Remediation:      "Configure RAG pipeline to append citations. Answers without citations must be refused.",
		},
		{
			ID:               "ai.confidence-threshold",
			Name:             "Confidence threshold",
			Category:         CategoryConfidence,
			Level:            "high",
			Status:           "enforced",
			Description:      "Outputs below confidence threshold become needs_review, not product law.",
			GateMode:         GateBlocking,
			Threshold:        &t80,
			EvidenceRequired: true,
			Remediation:      "Route low-confidence outputs to human review queue instead of automated acceptance.",
		},
		{
			ID:               "ai.human-review-gate",
			Name:             "Human review gate",
			Category:         CategoryHumanReview,
			Level:            "high",
			Status:           "enforced",
			Description:      "Critical or ambiguous AI outputs require explicit human approval before becoming authoritative.",
			GateMode:         GateBlocking,
			EvidenceRequired: true,
			Remediation:      "Ensure human_review_status is set to approved before publishing critical AI-extracted content.",
		},
		{
			ID:               "ai.data-provenance",
			Name:             "Data provenance",
			Category:         CategoryProvenance,
			Level:            "high",
			Status:           "enforced",
			Description:      "AI inputs must have traceable provenance: source_path, source_hash, extraction timestamp.",
			GateMode:         GateBlocking,
			EvidenceRequired: true,
			Remediation:      "Record source hash and path for every document fed to AI extraction pipeline.",
		},
		{
			ID:               "ai.injection-tests",
			Name:             "Prompt injection tests",
			Category:         CategoryInjection,
			Level:            "medium",
			Status:           "monitored",
			Description:      "Prompt injection test suite must pass before deploying RAG endpoints.",
			GateMode:         GateWarning,
			EvidenceRequired: true,
			Remediation:      "Run OWASP LLM Top 10 injection test suite and retain evidence.",
		},
		{
			ID:               "ai.refusal-behavior",
			Name:             "Refusal behavior tests",
			Category:         CategoryRefusal,
			Level:            "medium",
			Status:           "monitored",
			Description:      "RAG must refuse to answer when retrieved context is insufficient or out of scope.",
			GateMode:         GateWarning,
			EvidenceRequired: true,
			Remediation:      "Add refusal test cases and verify RAG declines rather than hallucinating.",
		},
	}
}

// Evaluate runs all controls against the provided input and returns an evaluation.
func Evaluate(controls []AIRAGControl, input EvalInput) AIRAGEvaluation {
	var results []ControlResult
	blockingFailures := 0

	for _, ctrl := range controls {
		result := evaluateControl(ctrl, input)
		results = append(results, result)
		if result.Status == "failed" && ctrl.GateMode == GateBlocking {
			blockingFailures++
		}
	}

	verdict := EvalVerdict{
		Pass:     blockingFailures == 0,
		Blocking: blockingFailures > 0,
	}
	if verdict.Pass {
		verdict.Summary = "All AI/RAG controls passed."
	} else {
		verdict.Summary = fmt.Sprintf("%d blocking control(s) failed.", blockingFailures)
	}

	return AIRAGEvaluation{
		SchemaVersion: "0.1.0",
		EvaluatedAt:   time.Now().UTC().Format(time.RFC3339),
		Evaluator:     "nomos",
		Controls:      results,
		Verdict:       verdict,
	}
}

// GateCheck returns an error if the evaluation has blocking failures.
func GateCheck(eval AIRAGEvaluation) error {
	if eval.Verdict.Pass {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrAIRAGGateFailed, eval.Verdict.Summary)
}

func evaluateControl(ctrl AIRAGControl, input EvalInput) ControlResult {
	switch ctrl.Category {
	case CategoryHallucination:
		return evalHallucination(ctrl, input)
	case CategoryCitation:
		return evalCitation(ctrl, input)
	case CategoryConfidence:
		return evalConfidence(ctrl, input)
	case CategoryHumanReview:
		return evalHumanReview(ctrl, input)
	case CategoryProvenance:
		return evalProvenance(ctrl, input)
	case CategoryInjection:
		return evalInjection(ctrl, input)
	case CategoryRefusal:
		return evalRefusal(ctrl, input)
	default:
		return ControlResult{ControlID: ctrl.ID, Status: "skipped", Message: "unknown category"}
	}
}

func evalHallucination(ctrl AIRAGControl, input EvalInput) ControlResult {
	threshold := 0.95
	if ctrl.Threshold != nil {
		threshold = *ctrl.Threshold
	}
	if input.CitationRate >= threshold {
		return ControlResult{ControlID: ctrl.ID, Status: "passed", Score: input.CitationRate}
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    "failed",
		Score:     input.CitationRate,
		Message:   fmt.Sprintf("citation rate %.2f below threshold %.2f", input.CitationRate, threshold),
	}
}

func evalCitation(ctrl AIRAGControl, input EvalInput) ControlResult {
	if input.HasCitations {
		return ControlResult{ControlID: ctrl.ID, Status: "passed"}
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    "failed",
		Message:   "no citations present in AI/RAG output",
	}
}

func evalConfidence(ctrl AIRAGControl, input EvalInput) ControlResult {
	threshold := 0.8
	if ctrl.Threshold != nil {
		threshold = *ctrl.Threshold
	}
	if input.ConfidenceScore >= threshold {
		return ControlResult{ControlID: ctrl.ID, Status: "passed", Score: input.ConfidenceScore}
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    "failed",
		Score:     input.ConfidenceScore,
		Message:   fmt.Sprintf("confidence %.2f below threshold %.2f — route to human review", input.ConfidenceScore, threshold),
	}
}

func evalHumanReview(ctrl AIRAGControl, input EvalInput) ControlResult {
	switch input.HumanReviewStatus {
	case "approved":
		return ControlResult{ControlID: ctrl.ID, Status: "passed"}
	case "rejected":
		return ControlResult{ControlID: ctrl.ID, Status: "failed", Message: "human review rejected this output"}
	case "pending", "":
		return ControlResult{ControlID: ctrl.ID, Status: "failed", Message: "human review not completed"}
	default:
		return ControlResult{ControlID: ctrl.ID, Status: "failed", Message: "unknown review status: " + input.HumanReviewStatus}
	}
}

func evalProvenance(ctrl AIRAGControl, input EvalInput) ControlResult {
	if input.ProvenanceHash != "" {
		return ControlResult{ControlID: ctrl.ID, Status: "passed"}
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    "failed",
		Message:   "no provenance hash recorded for AI input data",
	}
}

func evalInjection(ctrl AIRAGControl, input EvalInput) ControlResult {
	if input.InjectionTestsPass {
		return ControlResult{ControlID: ctrl.ID, Status: "passed"}
	}
	status := "failed"
	if ctrl.GateMode == GateWarning {
		status = "warning"
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    status,
		Message:   "prompt injection tests did not pass",
	}
}

func evalRefusal(ctrl AIRAGControl, input EvalInput) ControlResult {
	if input.RefusalTestsPass {
		return ControlResult{ControlID: ctrl.ID, Status: "passed"}
	}
	status := "failed"
	if ctrl.GateMode == GateWarning {
		status = "warning"
	}
	return ControlResult{
		ControlID: ctrl.ID,
		Status:    status,
		Message:   "refusal behavior tests did not pass",
	}
}
