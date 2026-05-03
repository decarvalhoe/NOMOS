package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var igTime = time.Date(2026, 5, 3, 16, 0, 0, 0, time.UTC)

func allPerfectDims() []DimensionResult {
	return StandardDimensions(1.0, 1.0, 1.0, 1.0, 1.0, 1.0)
}

func TestIntegratedGateAllPass(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: allPerfectDims(), Now: igTime,
	})
	if !r.Pass {
		t.Fatalf("expected pass: %s", r.Summary)
	}
	if r.GlobalScore < 0.99 {
		t.Fatalf("global: %f", r.GlobalScore)
	}
	if r.StructureScore < 0.99 {
		t.Fatalf("structure: %f", r.StructureScore)
	}
	if r.SemanticScore < 0.99 {
		t.Fatalf("semantic: %f", r.SemanticScore)
	}
	if r.Format != IntegratedGateFormat {
		t.Fatalf("format: %q", r.Format)
	}
	if len(r.Blockers) != 0 {
		t.Fatalf("blockers: %v", r.Blockers)
	}
}

func TestIntegratedGateStructureFail(t *testing.T) {
	dims := StandardDimensions(0.50, 1.0, 1.0, 1.0, 1.0, 1.0)
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims, Now: igTime,
	})
	if r.Pass {
		t.Fatal("low AST coverage should fail")
	}
	if r.StructureScore > 0.85 {
		t.Fatalf("structure score should be low: %f", r.StructureScore)
	}
	found := false
	for _, b := range r.Blockers {
		if strings.Contains(b, "structure score") || strings.Contains(b, "ast-coverage") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected structure blocker: %v", r.Blockers)
	}
}

func TestIntegratedGateSemanticFail(t *testing.T) {
	dims := StandardDimensions(1.0, 1.0, 0.30, 1.0, 0.20, 1.0)
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims, Now: igTime,
	})
	if r.Pass {
		t.Fatal("low semantic scores should fail")
	}
	if r.SemanticScore > 0.75 {
		t.Fatalf("semantic: %f", r.SemanticScore)
	}
}

func TestIntegratedGateBlockingDimension(t *testing.T) {
	dims := []DimensionResult{
		StructureDimension("ast", "AST", 0.50, 1.0, true),
	}
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims, Now: igTime,
	})
	if r.Pass {
		t.Fatal("blocking failed dimension should fail gate")
	}
	hasBlocker := false
	for _, b := range r.Blockers {
		if strings.Contains(b, "ast") {
			hasBlocker = true
		}
	}
	if !hasBlocker {
		t.Fatalf("expected blocking dim in blockers: %v", r.Blockers)
	}
}

func TestIntegratedGateNonBlockingWarning(t *testing.T) {
	dims := []DimensionResult{
		StructureDimension("ast", "AST", 1.0, 1.0, true),
		StructureDimension("hierarchy", "Hierarchy", 0.50, 1.0, false), // non-blocking
		SemanticDimension("lexicon", "Lexicon", 1.0, 1.0, false),
	}
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims,
		Threshold:  Threshold{GlobalMin: 0.50, StructureMin: 0.50, SemanticMin: 0.50},
		Now:        igTime,
	})
	// Non-blocking failed dim should not fail the gate if thresholds are met.
	if !r.Pass {
		t.Fatalf("non-blocking dim should not fail: %v", r.Blockers)
	}
}

func TestIntegratedGateCustomThreshold(t *testing.T) {
	dims := StandardDimensions(0.90, 0.90, 0.90, 0.90, 0.90, 0.90)
	// Default threshold: global 0.80, struct 0.85, semantic 0.75 → should pass.
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: dims, Now: igTime})
	if !r.Pass {
		t.Fatalf("default threshold should pass at 0.90: %v", r.Blockers)
	}

	// Strict threshold: 0.95 → should fail.
	r = RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims, Threshold: StrictThreshold(), Now: igTime,
	})
	if r.Pass {
		t.Fatal("strict threshold should fail at 0.90")
	}
}

func TestIntegratedGateWeightedScoring(t *testing.T) {
	dims := []DimensionResult{
		{ID: "heavy", Dimension: DimStructure, Score: 1.0, Weight: 10.0, Status: DimPassed},
		{ID: "light", Dimension: DimStructure, Score: 0.0, Weight: 0.1, Status: DimFailed, Blocking: false},
	}
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims,
		Threshold:  Threshold{GlobalMin: 0.90, StructureMin: 0.90, SemanticMin: 0},
		Now:        igTime,
	})
	// Heavy weight (10) at 1.0 + light (0.1) at 0.0 → ~0.99.
	if r.StructureScore < 0.95 {
		t.Fatalf("weighted structure: %f", r.StructureScore)
	}
}

func TestIntegratedGateSkippedDims(t *testing.T) {
	dims := []DimensionResult{
		{ID: "active", Dimension: DimStructure, Score: 1.0, Weight: 1.0, Status: DimPassed},
		{ID: "skipped", Dimension: DimStructure, Score: 0.0, Weight: 1.0, Status: DimSkipped},
	}
	r := RunIntegratedGate(IntegratedGateInput{
		Dimensions: dims, Now: igTime,
	})
	// Skipped should not affect score.
	if r.StructureScore < 0.99 {
		t.Fatalf("skipped should not lower score: %f", r.StructureScore)
	}
}

func TestIntegratedGateEmpty(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{Now: igTime})
	if !r.Pass {
		t.Fatal("empty should pass")
	}
	if r.GlobalScore < 0.99 {
		t.Fatalf("empty global: %f", r.GlobalScore)
	}
}

func TestIntegratedGateTimestamp(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{Now: igTime})
	if r.GeneratedAt != "2026-05-03T16:00:00Z" {
		t.Fatalf("timestamp: %q", r.GeneratedAt)
	}
}

func TestIntegratedGateSummaryString(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: allPerfectDims(), Now: igTime})
	s := r.SummaryString()
	if !strings.Contains(s, "PASSED") {
		t.Fatalf("summary: %q", s)
	}
	if !strings.Contains(s, "100%") {
		t.Fatalf("summary: %q", s)
	}
}

func TestIntegratedGateSummaryFailed(t *testing.T) {
	dims := StandardDimensions(0.50, 0.50, 0.50, 0.50, 0.50, 0.50)
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: dims, Now: igTime})
	s := r.SummaryString()
	if !strings.Contains(s, "FAILED") {
		t.Fatalf("summary: %q", s)
	}
	if !strings.Contains(s, "blocker") {
		t.Fatalf("summary: %q", s)
	}
}

func TestIntegratedGateDimsSorted(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: allPerfectDims(), Now: igTime})
	for i := 1; i < len(r.Dimensions); i++ {
		a, b := r.Dimensions[i-1], r.Dimensions[i]
		if a.Dimension > b.Dimension || (a.Dimension == b.Dimension && a.ID > b.ID) {
			t.Fatalf("dims not sorted: %s/%s before %s/%s", a.Dimension, a.ID, b.Dimension, b.ID)
		}
	}
}

func TestWriteIntegratedGateResult(t *testing.T) {
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: allPerfectDims(), Now: igTime})
	var buf bytes.Buffer
	if err := WriteIntegratedGateResult(&buf, r); err != nil {
		t.Fatalf("write: %v", err)
	}
	var decoded IntegratedGateResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != IntegratedGateFormat {
		t.Fatalf("format: %q", decoded.Format)
	}
	if decoded.Pass != r.Pass {
		t.Fatal("round-trip mismatch")
	}
}

func TestDefaultThresholdValues(t *testing.T) {
	th := DefaultThreshold()
	if th.GlobalMin != 0.80 || th.StructureMin != 0.85 || th.SemanticMin != 0.75 {
		t.Fatalf("defaults: %+v", th)
	}
}

func TestStrictThresholdValues(t *testing.T) {
	th := StrictThreshold()
	if th.GlobalMin != 0.95 || th.StructureMin != 0.95 || th.SemanticMin != 0.95 {
		t.Fatalf("strict: %+v", th)
	}
}

func TestScoreToStatus(t *testing.T) {
	cases := []struct {
		score float64
		want  DimensionStatus
	}{
		{1.0, DimPassed},
		{0.95, DimWarning},
		{0.80, DimWarning},
		{0.79, DimFailed},
		{0.0, DimFailed},
	}
	for _, tc := range cases {
		got := scoreToStatus(tc.score)
		if got != tc.want {
			t.Fatalf("scoreToStatus(%f) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestStandardDimensions(t *testing.T) {
	dims := StandardDimensions(1, 1, 1, 1, 1, 1)
	if len(dims) != 6 {
		t.Fatalf("expected 6, got %d", len(dims))
	}
	structCount, semCount := 0, 0
	for _, d := range dims {
		if d.Dimension == DimStructure {
			structCount++
		} else {
			semCount++
		}
	}
	if structCount != 3 || semCount != 3 {
		t.Fatalf("struct=%d sem=%d", structCount, semCount)
	}
}

func TestMixedPassFail(t *testing.T) {
	dims := StandardDimensions(1.0, 1.0, 0.0, 1.0, 1.0, 1.0)
	r := RunIntegratedGate(IntegratedGateInput{Dimensions: dims, Now: igTime})
	if r.Pass {
		t.Fatal("zero ref-integrity should fail")
	}
	// ref-integrity is blocking with score 0.
	foundRef := false
	for _, b := range r.Blockers {
		if strings.Contains(b, "ref-integrity") {
			foundRef = true
		}
	}
	if !foundRef {
		t.Fatalf("expected ref-integrity blocker: %v", r.Blockers)
	}
}
