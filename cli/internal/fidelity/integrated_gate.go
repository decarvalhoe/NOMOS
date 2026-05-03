package fidelity

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const IntegratedGateFormat = "nomos.fidelity-gate.v1"

// DimensionKind tags whether a check is structural or semantic.
type DimensionKind string

const (
	DimStructure DimensionKind = "structure"
	DimSemantic  DimensionKind = "semantic"
)

// DimensionStatus is the outcome of a single dimension check.
type DimensionStatus string

const (
	DimPassed  DimensionStatus = "passed"
	DimWarning DimensionStatus = "warning"
	DimFailed  DimensionStatus = "failed"
	DimSkipped DimensionStatus = "skipped"
)

// DimensionResult is a single scored check within a dimension.
type DimensionResult struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Dimension DimensionKind   `json:"dimension"`
	Status    DimensionStatus `json:"status"`
	Score     float64         `json:"score"`
	Weight    float64         `json:"weight"`
	Blocking  bool            `json:"blocking"`
	Details   []string        `json:"details,omitempty"`
}

// Threshold configures pass/fail cutoffs.
type Threshold struct {
	GlobalMin    float64 `json:"global_min"`    // minimum combined score to pass
	StructureMin float64 `json:"structure_min"` // minimum structure score
	SemanticMin  float64 `json:"semantic_min"`  // minimum semantic score
}

// DefaultThreshold returns the standard threshold (0.80 global, 0.85 structure, 0.75 semantic).
func DefaultThreshold() Threshold {
	return Threshold{
		GlobalMin:    0.80,
		StructureMin: 0.85,
		SemanticMin:  0.75,
	}
}

// StrictThreshold returns a strict threshold (0.95 across the board).
func StrictThreshold() Threshold {
	return Threshold{
		GlobalMin:    0.95,
		StructureMin: 0.95,
		SemanticMin:  0.95,
	}
}

// IntegratedGateInput configures the combined gate.
type IntegratedGateInput struct {
	Dimensions []DimensionResult
	Threshold  Threshold
	Now        time.Time
}

// IntegratedGateResult is the combined fidelity gate output.
type IntegratedGateResult struct {
	Format         string            `json:"format"`
	GeneratedAt    string            `json:"generated_at"`
	Pass           bool              `json:"pass"`
	GlobalScore    float64           `json:"global_score"`
	StructureScore float64           `json:"structure_score"`
	SemanticScore  float64           `json:"semantic_score"`
	Threshold      Threshold         `json:"threshold"`
	Dimensions     []DimensionResult `json:"dimensions"`
	Blockers       []string          `json:"blockers,omitempty"`
	Summary        string            `json:"summary"`
}

// RunIntegratedGate combines structure and semantic checks into a single scored gate.
func RunIntegratedGate(input IntegratedGateInput) IntegratedGateResult {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	threshold := input.Threshold
	if threshold.GlobalMin == 0 && threshold.StructureMin == 0 && threshold.SemanticMin == 0 {
		threshold = DefaultThreshold()
	}

	dims := make([]DimensionResult, len(input.Dimensions))
	copy(dims, input.Dimensions)
	sort.Slice(dims, func(i, j int) bool {
		if dims[i].Dimension == dims[j].Dimension {
			return dims[i].ID < dims[j].ID
		}
		return dims[i].Dimension < dims[j].Dimension
	})

	structScore := weightedScore(dims, DimStructure)
	semanticScore := weightedScore(dims, DimSemantic)
	globalScore := combinedScore(dims)

	var blockers []string

	// Check blocking failures.
	for _, d := range dims {
		if d.Blocking && d.Status == DimFailed {
			blockers = append(blockers, fmt.Sprintf("%s: %s", d.ID, d.Name))
		}
	}

	// Check threshold violations.
	if globalScore < threshold.GlobalMin {
		blockers = append(blockers, fmt.Sprintf("global score %.2f < threshold %.2f", globalScore, threshold.GlobalMin))
	}
	if structScore < threshold.StructureMin {
		blockers = append(blockers, fmt.Sprintf("structure score %.2f < threshold %.2f", structScore, threshold.StructureMin))
	}
	if semanticScore < threshold.SemanticMin {
		blockers = append(blockers, fmt.Sprintf("semantic score %.2f < threshold %.2f", semanticScore, threshold.SemanticMin))
	}

	pass := len(blockers) == 0

	summary := fmt.Sprintf("Fidelity gate %s: global=%.2f structure=%.2f semantic=%.2f",
		passLabel(pass), globalScore, structScore, semanticScore)
	if !pass {
		summary += fmt.Sprintf(" (%d blocker(s))", len(blockers))
	}

	return IntegratedGateResult{
		Format:         IntegratedGateFormat,
		GeneratedAt:    now.Format(time.RFC3339),
		Pass:           pass,
		GlobalScore:    globalScore,
		StructureScore: structScore,
		SemanticScore:  semanticScore,
		Threshold:      threshold,
		Dimensions:     dims,
		Blockers:       blockers,
		Summary:        summary,
	}
}

// WriteIntegratedGateResult writes the result as indented JSON.
func WriteIntegratedGateResult(w io.Writer, result IntegratedGateResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func weightedScore(dims []DimensionResult, kind DimensionKind) float64 {
	totalWeight := 0.0
	weightedSum := 0.0
	for _, d := range dims {
		if d.Dimension != kind {
			continue
		}
		if d.Status == DimSkipped {
			continue
		}
		w := d.Weight
		if w <= 0 {
			w = 1.0
		}
		totalWeight += w
		weightedSum += d.Score * w
	}
	if totalWeight == 0 {
		return 1.0 // no checks = perfect
	}
	return weightedSum / totalWeight
}

func combinedScore(dims []DimensionResult) float64 {
	totalWeight := 0.0
	weightedSum := 0.0
	for _, d := range dims {
		if d.Status == DimSkipped {
			continue
		}
		w := d.Weight
		if w <= 0 {
			w = 1.0
		}
		totalWeight += w
		weightedSum += d.Score * w
	}
	if totalWeight == 0 {
		return 1.0
	}
	return weightedSum / totalWeight
}

func passLabel(pass bool) string {
	if pass {
		return "PASSED"
	}
	return "FAILED"
}

// --- Convenience constructors for common dimensions ---

// StructureDimension creates a structure dimension result.
func StructureDimension(id, name string, score float64, weight float64, blocking bool, details ...string) DimensionResult {
	return DimensionResult{
		ID: id, Name: name, Dimension: DimStructure,
		Status: scoreToStatus(score), Score: score,
		Weight: weight, Blocking: blocking, Details: details,
	}
}

// SemanticDimension creates a semantic dimension result.
func SemanticDimension(id, name string, score float64, weight float64, blocking bool, details ...string) DimensionResult {
	return DimensionResult{
		ID: id, Name: name, Dimension: DimSemantic,
		Status: scoreToStatus(score), Score: score,
		Weight: weight, Blocking: blocking, Details: details,
	}
}

func scoreToStatus(score float64) DimensionStatus {
	if score >= 1.0 {
		return DimPassed
	}
	if score >= 0.80 {
		return DimWarning
	}
	if score > 0 {
		return DimFailed
	}
	return DimFailed
}

// StandardDimensions returns a typical set of structure + semantic checks.
func StandardDimensions(
	astCoverage, atomComplete, refIntegrity float64,
	lexiconScore, termConflict, hierarchyScore float64,
) []DimensionResult {
	return []DimensionResult{
		StructureDimension("ast-coverage", "AST source coverage", astCoverage, 2.0, true),
		StructureDimension("atom-completeness", "Atom field completeness", atomComplete, 1.5, true),
		StructureDimension("hierarchy", "Profile hierarchy conformance", hierarchyScore, 1.0, false),
		SemanticDimension("ref-integrity", "Canonical reference integrity", refIntegrity, 2.0, true),
		SemanticDimension("lexicon", "Lexicon compliance", lexiconScore, 1.0, false),
		SemanticDimension("term-definitions", "Term definition consistency", termConflict, 1.5, true),
	}
}

// SummaryString returns a compact summary for logging.
func (r IntegratedGateResult) SummaryString() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (global=%.0f%% struct=%.0f%% sem=%.0f%%)",
		passLabel(r.Pass), r.GlobalScore*100, r.StructureScore*100, r.SemanticScore*100)
	if len(r.Blockers) > 0 {
		fmt.Fprintf(&b, " [%d blocker(s)]", len(r.Blockers))
	}
	return b.String()
}
