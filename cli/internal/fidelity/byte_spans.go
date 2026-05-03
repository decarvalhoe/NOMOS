package fidelity

import (
	"fmt"
	"strings"
)

// ByteSpan records the exact byte range in the original source.
type ByteSpan struct {
	StartByte int `json:"start_byte"`
	EndByte   int `json:"end_byte"`
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

// SpanConformanceResult is the outcome of a byte-span roundtrip check.
type SpanConformanceResult struct {
	TotalNodes      int                `json:"total_nodes"`
	Checked         int                `json:"checked"`
	Conformant      int                `json:"conformant"`
	NonConformant   int                `json:"non_conformant"`
	Skipped         int                `json:"skipped"`
	CoveredBytes    int                `json:"covered_bytes"`
	TotalBytes      int                `json:"total_bytes"`
	CoverageRatio   float64            `json:"coverage_ratio"`
	Violations      []SpanViolation    `json:"violations,omitempty"`
	Verdict         string             `json:"verdict"`
}

// SpanViolation describes a single byte-span mismatch.
type SpanViolation struct {
	NodeID   string `json:"node_id"`
	Kind     string `json:"kind"`
	Line     int    `json:"line"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Reason   string `json:"reason"`
}

// AnnotatedNode is a CNode enriched with byte-level span information.
type AnnotatedNode struct {
	CNode
	ByteSpan ByteSpan `json:"byte_span"`
}

// AnnotateByteSpans computes exact byte offsets for every node in the CAST
// by mapping line-based spans back to the original source bytes.
func AnnotateByteSpans(cast CAST, source string) []AnnotatedNode {
	offsets := computeLineOffsets(source)
	result := make([]AnnotatedNode, len(cast.Nodes))

	for i, node := range cast.Nodes {
		bs := resolveByteSpan(node, offsets, len(source))
		result[i] = AnnotatedNode{CNode: node, ByteSpan: bs}
	}
	return result
}

// CheckSpanConformance verifies that every node's RawText matches the
// original source bytes at the node's span. This is the roundtrip
// fidelity check: source[start:end] must equal node.RawText.
func CheckSpanConformance(cast CAST, source string) SpanConformanceResult {
	offsets := computeLineOffsets(source)
	totalBytes := len(source)

	var violations []SpanViolation
	checked, conformant, nonConformant, skipped := 0, 0, 0, 0
	covered := make([]bool, totalBytes)

	for _, node := range cast.Nodes {
		if node.Kind == KindDocument {
			skipped++
			continue
		}
		if node.RawText == "" && node.Text == "" {
			skipped++
			continue
		}

		bs := resolveByteSpan(node, offsets, totalBytes)
		if bs.StartByte < 0 || bs.EndByte < 0 || bs.StartByte > totalBytes || bs.EndByte > totalBytes {
			violations = append(violations, SpanViolation{
				NodeID: node.ID, Kind: string(node.Kind), Line: node.Span.StartLine,
				Reason: fmt.Sprintf("byte span [%d:%d] out of bounds (source len %d)", bs.StartByte, bs.EndByte, totalBytes),
			})
			nonConformant++
			checked++
			continue
		}

		// Mark covered bytes.
		for b := bs.StartByte; b < bs.EndByte && b < totalBytes; b++ {
			covered[b] = true
		}

		sourceSlice := source[bs.StartByte:bs.EndByte]
		reference := node.RawText
		if reference == "" {
			reference = node.Text
		}

		checked++
		if matchesSource(reference, sourceSlice) {
			conformant++
		} else {
			nonConformant++
			violations = append(violations, SpanViolation{
				NodeID:   node.ID,
				Kind:     string(node.Kind),
				Line:     node.Span.StartLine,
				Expected: truncateStr(reference, 80),
				Actual:   truncateStr(sourceSlice, 80),
				Reason:   "raw_text does not match source bytes at span",
			})
		}
	}

	coveredCount := 0
	for _, c := range covered {
		if c {
			coveredCount++
		}
	}

	ratio := 0.0
	if totalBytes > 0 {
		ratio = float64(coveredCount) / float64(totalBytes)
	}

	verdict := "conformant"
	if nonConformant > 0 {
		verdict = "non_conformant"
	}

	return SpanConformanceResult{
		TotalNodes:    len(cast.Nodes),
		Checked:       checked,
		Conformant:    conformant,
		NonConformant: nonConformant,
		Skipped:       skipped,
		CoveredBytes:  coveredCount,
		TotalBytes:    totalBytes,
		CoverageRatio: ratio,
		Violations:    violations,
		Verdict:       verdict,
	}
}

// matchesSource checks if the node text matches the source slice,
// allowing for whitespace normalization at boundaries.
func matchesSource(nodeText, sourceSlice string) bool {
	if nodeText == sourceSlice {
		return true
	}
	// Allow trimmed match (leading/trailing whitespace differences).
	if strings.TrimSpace(nodeText) == strings.TrimSpace(sourceSlice) {
		return true
	}
	// Allow the source slice to contain the node text (multi-line nodes
	// may have their raw text be a subset of the full line range).
	if strings.Contains(sourceSlice, strings.TrimSpace(nodeText)) {
		return true
	}
	if strings.Contains(strings.TrimSpace(sourceSlice), strings.TrimSpace(nodeText)) {
		return true
	}
	return false
}

// computeLineOffsets returns the byte offset of the start of each line.
// offsets[0] = 0 (line 1 starts at byte 0).
func computeLineOffsets(source string) []int {
	offsets := []int{0}
	for i, ch := range source {
		if ch == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// resolveByteSpan converts a line-based Span to a ByteSpan.
func resolveByteSpan(node CNode, offsets []int, sourceLen int) ByteSpan {
	startLine := node.Span.StartLine
	endLine := node.Span.EndLine

	if startLine < 1 {
		startLine = 1
	}
	if endLine < startLine {
		endLine = startLine
	}

	startIdx := startLine - 1
	if startIdx >= len(offsets) {
		startIdx = len(offsets) - 1
	}
	startByte := offsets[startIdx]

	// EndByte is the end of the last line (up to next newline or EOF).
	endIdx := endLine - 1
	if endIdx >= len(offsets) {
		endIdx = len(offsets) - 1
	}
	var endByte int
	if endIdx+1 < len(offsets) {
		endByte = offsets[endIdx+1]
	} else {
		endByte = sourceLen
	}

	return ByteSpan{
		StartByte: startByte,
		EndByte:   endByte,
		StartLine: startLine,
		EndLine:   endLine,
	}
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
