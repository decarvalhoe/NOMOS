package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// AdapterFunc is the signature any adapter must implement to be tested.
// It takes source content and returns atoms.
type AdapterFunc func(source string) ([]PortableAtom, error)

// RoundtripFunc reconstructs source from atoms (for roundtrip testing).
type RoundtripFunc func(atoms []PortableAtom) (string, error)

// ConformanceCheck identifies one conformance criterion.
type ConformanceCheck string

const (
	CheckLosslessness   ConformanceCheck = "losslessness"
	CheckSpanAccuracy   ConformanceCheck = "span_accuracy"
	CheckRoundtrip      ConformanceCheck = "roundtrip"
	CheckHashIntegrity  ConformanceCheck = "hash_integrity"
	CheckIDUniqueness   ConformanceCheck = "id_uniqueness"
	CheckParentValidity ConformanceCheck = "parent_validity"
	CheckDepthBounds    ConformanceCheck = "depth_bounds"
	CheckRefPresence    ConformanceCheck = "ref_presence"
)

// ConformanceResult records the outcome of one check.
type ConformanceResult struct {
	Check   ConformanceCheck `json:"check"`
	Pass    bool             `json:"pass"`
	Message string           `json:"message,omitempty"`
	Detail  string           `json:"detail,omitempty"`
}

// HarnessResult is the full conformance test output.
type HarnessResult struct {
	AdapterID    string              `json:"adapter_id"`
	TotalChecks  int                 `json:"total_checks"`
	Passed       int                 `json:"passed"`
	Failed       int                 `json:"failed"`
	Pass         bool                `json:"pass"`
	Results      []ConformanceResult `json:"results"`
}

// HarnessConfig configures the conformance harness.
type HarnessConfig struct {
	AdapterID       string
	Source          string
	SourceLines     []string
	Adapter         AdapterFunc
	Roundtrip       RoundtripFunc // nil to skip roundtrip check
	MaxDepth        int           // 0 = no limit
	LossThreshold   float64       // 0.0-1.0, fraction of source covered
}

// RunConformance executes the full conformance harness against an adapter.
func RunConformance(config HarnessConfig) (HarnessResult, error) {
	if config.Adapter == nil {
		return HarnessResult{}, fmt.Errorf("adapter function is required")
	}
	if config.Source == "" {
		return HarnessResult{}, fmt.Errorf("source content is required")
	}
	if config.LossThreshold == 0 {
		config.LossThreshold = 0.8
	}
	if config.SourceLines == nil {
		config.SourceLines = strings.Split(config.Source, "\n")
	}

	atoms, err := config.Adapter(config.Source)
	if err != nil {
		return HarnessResult{
			AdapterID:   config.AdapterID,
			TotalChecks: 1,
			Failed:      1,
			Results: []ConformanceResult{{
				Check: CheckLosslessness, Pass: false, Message: fmt.Sprintf("adapter error: %v", err),
			}},
		}, nil
	}

	var results []ConformanceResult

	results = append(results, checkLosslessness(config.Source, atoms, config.LossThreshold))
	results = append(results, checkSpanAccuracy(config.SourceLines, atoms))
	results = append(results, checkHashIntegrity(atoms))
	results = append(results, checkIDUniqueness(atoms))
	results = append(results, checkParentValidity(atoms))
	results = append(results, checkDepthBounds(atoms, config.MaxDepth))
	results = append(results, checkRefPresence(atoms))

	if config.Roundtrip != nil {
		results = append(results, checkRoundtrip(config.Source, atoms, config.Roundtrip))
	}

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		} else {
			failed++
		}
	}

	return HarnessResult{
		AdapterID:   config.AdapterID,
		TotalChecks: len(results),
		Passed:      passed,
		Failed:      failed,
		Pass:        failed == 0,
		Results:     results,
	}, nil
}

func checkLosslessness(source string, atoms []PortableAtom, threshold float64) ConformanceResult {
	sourceLen := len(strings.TrimSpace(source))
	if sourceLen == 0 {
		return ConformanceResult{Check: CheckLosslessness, Pass: true, Message: "empty source"}
	}

	coveredChars := 0
	for _, atom := range atoms {
		coveredChars += len(strings.TrimSpace(atom.Text))
	}

	ratio := float64(coveredChars) / float64(sourceLen)
	if ratio >= threshold {
		return ConformanceResult{
			Check:   CheckLosslessness,
			Pass:    true,
			Message: fmt.Sprintf("coverage %.1f%% (threshold %.0f%%)", ratio*100, threshold*100),
		}
	}
	return ConformanceResult{
		Check:   CheckLosslessness,
		Pass:    false,
		Message: fmt.Sprintf("coverage %.1f%% below threshold %.0f%%", ratio*100, threshold*100),
		Detail:  fmt.Sprintf("covered %d of %d chars", coveredChars, sourceLen),
	}
}

func checkSpanAccuracy(sourceLines []string, atoms []PortableAtom) ConformanceResult {
	totalLines := len(sourceLines)
	violations := 0

	for _, atom := range atoms {
		if atom.SourceLine <= 0 {
			violations++
			continue
		}
		if atom.SourceLine > totalLines {
			violations++
			continue
		}
		// Verify line content contains atom text (first 20 chars).
		expected := strings.TrimSpace(atom.Text)
		if len(expected) > 40 {
			expected = expected[:40]
		}
		actualLine := sourceLines[atom.SourceLine-1]
		if !strings.Contains(strings.ToLower(actualLine), strings.ToLower(expected[:min(len(expected), 15)])) {
			// Allow tolerance — some atoms span multiple lines.
			if atom.SourceLine < totalLines {
				nextLine := sourceLines[atom.SourceLine]
				if !strings.Contains(strings.ToLower(nextLine), strings.ToLower(expected[:min(len(expected), 15)])) {
					violations++
				}
			}
		}
	}

	if violations == 0 {
		return ConformanceResult{Check: CheckSpanAccuracy, Pass: true, Message: "all spans accurate"}
	}
	return ConformanceResult{
		Check:   CheckSpanAccuracy,
		Pass:    false,
		Message: fmt.Sprintf("%d atoms have inaccurate source spans", violations),
	}
}

func checkHashIntegrity(atoms []PortableAtom) ConformanceResult {
	mismatches := 0
	for _, atom := range atoms {
		if atom.ContentHash == "" {
			mismatches++
			continue
		}
		expected := computeSHA256(strings.TrimSpace(atom.Text))
		if atom.ContentHash != expected {
			mismatches++
		}
	}
	if mismatches == 0 {
		return ConformanceResult{Check: CheckHashIntegrity, Pass: true, Message: "all hashes valid"}
	}
	return ConformanceResult{
		Check:   CheckHashIntegrity,
		Pass:    false,
		Message: fmt.Sprintf("%d atoms have invalid content hashes", mismatches),
	}
}

func checkIDUniqueness(atoms []PortableAtom) ConformanceResult {
	seen := map[string]bool{}
	duplicates := 0
	for _, atom := range atoms {
		if seen[atom.ID] {
			duplicates++
		}
		seen[atom.ID] = true
	}
	if duplicates == 0 {
		return ConformanceResult{Check: CheckIDUniqueness, Pass: true, Message: "all IDs unique"}
	}
	return ConformanceResult{
		Check:   CheckIDUniqueness,
		Pass:    false,
		Message: fmt.Sprintf("%d duplicate atom IDs", duplicates),
	}
}

func checkParentValidity(atoms []PortableAtom) ConformanceResult {
	ids := map[string]bool{}
	for _, atom := range atoms {
		ids[atom.ID] = true
	}
	orphans := 0
	for _, atom := range atoms {
		if atom.ParentID != "" && !ids[atom.ParentID] {
			orphans++
		}
	}
	if orphans == 0 {
		return ConformanceResult{Check: CheckParentValidity, Pass: true, Message: "all parents valid"}
	}
	return ConformanceResult{
		Check:   CheckParentValidity,
		Pass:    false,
		Message: fmt.Sprintf("%d atoms reference non-existent parents", orphans),
	}
}

func checkDepthBounds(atoms []PortableAtom, maxDepth int) ConformanceResult {
	if maxDepth <= 0 {
		return ConformanceResult{Check: CheckDepthBounds, Pass: true, Message: "no depth limit configured"}
	}
	violations := 0
	for _, atom := range atoms {
		if atom.Depth > maxDepth {
			violations++
		}
	}
	if violations == 0 {
		return ConformanceResult{Check: CheckDepthBounds, Pass: true, Message: fmt.Sprintf("all atoms within depth %d", maxDepth)}
	}
	return ConformanceResult{
		Check:   CheckDepthBounds,
		Pass:    false,
		Message: fmt.Sprintf("%d atoms exceed max depth %d", violations, maxDepth),
	}
}

func checkRefPresence(atoms []PortableAtom) ConformanceResult {
	missing := 0
	for _, atom := range atoms {
		if atom.CanonicalRef == "" {
			missing++
		}
	}
	if missing == 0 {
		return ConformanceResult{Check: CheckRefPresence, Pass: true, Message: "all atoms have canonical refs"}
	}
	return ConformanceResult{
		Check:   CheckRefPresence,
		Pass:    false,
		Message: fmt.Sprintf("%d atoms missing canonical_ref", missing),
	}
}

func checkRoundtrip(source string, atoms []PortableAtom, roundtrip RoundtripFunc) ConformanceResult {
	reconstructed, err := roundtrip(atoms)
	if err != nil {
		return ConformanceResult{
			Check:   CheckRoundtrip,
			Pass:    false,
			Message: fmt.Sprintf("roundtrip error: %v", err),
		}
	}

	sourceNorm := normalizeWhitespace(source)
	reconNorm := normalizeWhitespace(reconstructed)

	if sourceNorm == reconNorm {
		return ConformanceResult{Check: CheckRoundtrip, Pass: true, Message: "perfect roundtrip"}
	}

	// Compute similarity.
	similarity := computeSimilarity(sourceNorm, reconNorm)
	if similarity >= 0.9 {
		return ConformanceResult{
			Check:   CheckRoundtrip,
			Pass:    true,
			Message: fmt.Sprintf("roundtrip similarity %.1f%%", similarity*100),
		}
	}
	return ConformanceResult{
		Check:   CheckRoundtrip,
		Pass:    false,
		Message: fmt.Sprintf("roundtrip similarity %.1f%% (need >= 90%%)", similarity*100),
	}
}

func computeSHA256(content string) string {
	h := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(h[:])
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func computeSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	longer := a
	shorter := b
	if len(a) < len(b) {
		longer = b
		shorter = a
	}
	if len(longer) == 0 {
		return 1.0
	}
	// Simple character-level overlap.
	matches := 0
	used := make([]bool, len(longer))
	for _, ch := range shorter {
		for i, lch := range longer {
			if !used[i] && lch == ch {
				matches++
				used[i] = true
				break
			}
		}
	}
	return float64(matches) / float64(len(longer))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
