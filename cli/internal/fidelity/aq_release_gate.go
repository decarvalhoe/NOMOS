package fidelity

import (
	"fmt"
	"strings"
)

// AQLevel represents an Assurance Quality level.
type AQLevel int

const (
	AQ0 AQLevel = 0 // No assurance — unverified
	AQ1 AQLevel = 1 // Basic — automated tests pass
	AQ2 AQLevel = 2 // Structured — coverage + schema validation
	AQ3 AQLevel = 3 // Governed — evidence chain + lexicon + gates
	AQ4 AQLevel = 4 // Verified — independent review + reconstruction
	AQ5 AQLevel = 5 // Certified — full regulatory evidence pack
)

// String returns the level label.
func (l AQLevel) String() string {
	return fmt.Sprintf("AQ-%d", int(l))
}

// AQRequirement is a single evidence requirement for a given AQ level.
type AQRequirement struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	MinLevel    AQLevel `json:"min_level"`
	Critical    bool    `json:"critical"`
}

// AQCheckResult is the evaluation of a single requirement.
type AQCheckResult struct {
	RequirementID string `json:"requirement_id"`
	Description   string `json:"description"`
	MinLevel      string `json:"min_level"`
	Satisfied     bool   `json:"satisfied"`
	Detail        string `json:"detail"`
	Critical      bool   `json:"critical"`
}

// AQGateResult is the full release gate evaluation.
type AQGateResult struct {
	TargetLevel     string          `json:"target_level"`
	AchievedLevel   string          `json:"achieved_level"`
	Verdict         string          `json:"verdict"`
	TotalChecks     int             `json:"total_checks"`
	Passed          int             `json:"passed"`
	Failed          int             `json:"failed"`
	CriticalFailed  int             `json:"critical_failed"`
	Checks          []AQCheckResult `json:"checks"`
}

// AQEvidence holds the evidence inputs for gate evaluation.
type AQEvidence struct {
	TestsPassing        bool
	TestCount           int
	CoverageRatio       float64
	SchemaValidation    bool
	LexiconCompliance   bool
	EvidenceChainComplete bool
	FidelityGatePassed  bool
	SelfCompliancePassed bool
	IndependentReview   bool
	ReconstructionPassed bool
	RegulatoryPack      bool
	SBOMPresent         bool
	ProvenancePresent   bool
	ApprovalSigned      bool
}

// DefaultAQRequirements returns the standard requirements per AQ level.
func DefaultAQRequirements() []AQRequirement {
	return []AQRequirement{
		// AQ-1: Basic
		{ID: "AQ1-TESTS", Description: "All automated tests pass", MinLevel: AQ1, Critical: true},
		{ID: "AQ1-COUNT", Description: "Minimum test count > 0", MinLevel: AQ1, Critical: true},

		// AQ-2: Structured
		{ID: "AQ2-COVERAGE", Description: "Test coverage ratio >= 0.60", MinLevel: AQ2, Critical: true},
		{ID: "AQ2-SCHEMA", Description: "Schema validation passes (CUE vet)", MinLevel: AQ2, Critical: true},

		// AQ-3: Governed
		{ID: "AQ3-LEXICON", Description: "Lexicon compliance verified", MinLevel: AQ3, Critical: true},
		{ID: "AQ3-EVIDENCE", Description: "Evidence chain complete for all claims", MinLevel: AQ3, Critical: true},
		{ID: "AQ3-FIDELITY", Description: "Fidelity gate passes", MinLevel: AQ3, Critical: true},
		{ID: "AQ3-SELFCOMP", Description: "Self-compliance evaluation passes", MinLevel: AQ3, Critical: false},

		// AQ-4: Verified
		{ID: "AQ4-REVIEW", Description: "Independent reconstruction review completed", MinLevel: AQ4, Critical: true},
		{ID: "AQ4-RECONSTRUCT", Description: "Evidence chain fully reconstructable", MinLevel: AQ4, Critical: true},

		// AQ-5: Certified
		{ID: "AQ5-REGPACK", Description: "Full regulatory evidence pack present", MinLevel: AQ5, Critical: true},
		{ID: "AQ5-SBOM", Description: "Software bill of materials present", MinLevel: AQ5, Critical: false},
		{ID: "AQ5-PROVENANCE", Description: "SLSA provenance attestation present", MinLevel: AQ5, Critical: false},
		{ID: "AQ5-APPROVAL", Description: "Release approval signed", MinLevel: AQ5, Critical: true},
	}
}

// EvaluateAQGate checks evidence against the target AQ level.
func EvaluateAQGate(target AQLevel, evidence AQEvidence) AQGateResult {
	return EvaluateAQGateWith(target, evidence, DefaultAQRequirements())
}

// EvaluateAQGateWith checks evidence against custom requirements.
func EvaluateAQGateWith(target AQLevel, evidence AQEvidence, requirements []AQRequirement) AQGateResult {
	var checks []AQCheckResult
	passed, failed, critFailed := 0, 0, 0

	for _, req := range requirements {
		if req.MinLevel > target {
			continue
		}
		satisfied, detail := evaluateRequirement(req, evidence)
		check := AQCheckResult{
			RequirementID: req.ID,
			Description:   req.Description,
			MinLevel:      AQLevel(req.MinLevel).String(),
			Satisfied:     satisfied,
			Detail:        detail,
			Critical:      req.Critical,
		}
		checks = append(checks, check)
		if satisfied {
			passed++
		} else {
			failed++
			if req.Critical {
				critFailed++
			}
		}
	}

	achieved := computeAchievedLevel(checks, requirements)
	verdict := "pass"
	if critFailed > 0 {
		verdict = "fail"
	} else if failed > 0 {
		verdict = "pass_with_warnings"
	}

	return AQGateResult{
		TargetLevel:    target.String(),
		AchievedLevel:  achieved.String(),
		Verdict:        verdict,
		TotalChecks:    len(checks),
		Passed:         passed,
		Failed:         failed,
		CriticalFailed: critFailed,
		Checks:         checks,
	}
}

func evaluateRequirement(req AQRequirement, ev AQEvidence) (bool, string) {
	switch req.ID {
	case "AQ1-TESTS":
		return ev.TestsPassing, boolDetail("tests passing", ev.TestsPassing)
	case "AQ1-COUNT":
		ok := ev.TestCount > 0
		return ok, fmt.Sprintf("test count: %d", ev.TestCount)
	case "AQ2-COVERAGE":
		ok := ev.CoverageRatio >= 0.60
		return ok, fmt.Sprintf("coverage: %.1f%% (min 60%%)", ev.CoverageRatio*100)
	case "AQ2-SCHEMA":
		return ev.SchemaValidation, boolDetail("schema validation", ev.SchemaValidation)
	case "AQ3-LEXICON":
		return ev.LexiconCompliance, boolDetail("lexicon compliance", ev.LexiconCompliance)
	case "AQ3-EVIDENCE":
		return ev.EvidenceChainComplete, boolDetail("evidence chain", ev.EvidenceChainComplete)
	case "AQ3-FIDELITY":
		return ev.FidelityGatePassed, boolDetail("fidelity gate", ev.FidelityGatePassed)
	case "AQ3-SELFCOMP":
		return ev.SelfCompliancePassed, boolDetail("self-compliance", ev.SelfCompliancePassed)
	case "AQ4-REVIEW":
		return ev.IndependentReview, boolDetail("independent review", ev.IndependentReview)
	case "AQ4-RECONSTRUCT":
		return ev.ReconstructionPassed, boolDetail("reconstruction", ev.ReconstructionPassed)
	case "AQ5-REGPACK":
		return ev.RegulatoryPack, boolDetail("regulatory pack", ev.RegulatoryPack)
	case "AQ5-SBOM":
		return ev.SBOMPresent, boolDetail("SBOM", ev.SBOMPresent)
	case "AQ5-PROVENANCE":
		return ev.ProvenancePresent, boolDetail("provenance", ev.ProvenancePresent)
	case "AQ5-APPROVAL":
		return ev.ApprovalSigned, boolDetail("approval", ev.ApprovalSigned)
	default:
		return false, "unknown requirement"
	}
}

func computeAchievedLevel(checks []AQCheckResult, requirements []AQRequirement) AQLevel {
	// Find the highest level where all critical requirements pass.
	reqsByLevel := map[AQLevel][]string{}
	for _, req := range requirements {
		if req.Critical {
			reqsByLevel[req.MinLevel] = append(reqsByLevel[req.MinLevel], req.ID)
		}
	}

	passedSet := map[string]bool{}
	for _, c := range checks {
		if c.Satisfied {
			passedSet[c.RequirementID] = true
		}
	}

	achieved := AQ0
	for level := AQ1; level <= AQ5; level++ {
		criticalIDs := reqsByLevel[level]
		allPass := true
		for _, id := range criticalIDs {
			if !passedSet[id] {
				allPass = false
				break
			}
		}
		if !allPass {
			break
		}
		achieved = level
	}
	return achieved
}

func boolDetail(name string, ok bool) string {
	if ok {
		return name + ": passed"
	}
	return name + ": failed"
}

// FormatGateReport returns a human-readable summary of the gate result.
func FormatGateReport(result AQGateResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "AQ Release Gate: %s\n", strings.ToUpper(result.Verdict))
	fmt.Fprintf(&b, "Target: %s | Achieved: %s\n", result.TargetLevel, result.AchievedLevel)
	fmt.Fprintf(&b, "Checks: %d passed, %d failed (%d critical)\n\n", result.Passed, result.Failed, result.CriticalFailed)

	for _, c := range result.Checks {
		status := "PASS"
		if !c.Satisfied {
			status = "FAIL"
			if c.Critical {
				status = "FAIL [CRITICAL]"
			}
		}
		fmt.Fprintf(&b, "  [%s] %s — %s\n", status, c.RequirementID, c.Detail)
	}
	return b.String()
}
