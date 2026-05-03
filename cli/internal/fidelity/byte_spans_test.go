package fidelity

import (
	"testing"
)

const simpleSource = `# Title

First paragraph.

## Section

Second paragraph with content.

- Item one
- Item two

` + "```go\nfunc main() {}\n```\n"

func TestAnnotateByteSpans_PopulatesAllNodes(t *testing.T) {
	cast := ParseMarkdown(simpleSource)
	annotated := AnnotateByteSpans(cast, simpleSource)

	if len(annotated) != len(cast.Nodes) {
		t.Fatalf("expected %d annotated nodes, got %d", len(cast.Nodes), len(annotated))
	}
	for _, a := range annotated {
		if a.Kind == KindDocument {
			continue
		}
		if a.ByteSpan.StartByte < 0 {
			t.Fatalf("node %s has negative start_byte", a.ID)
		}
		if a.ByteSpan.EndByte < a.ByteSpan.StartByte {
			t.Fatalf("node %s has end_byte < start_byte", a.ID)
		}
	}
}

func TestAnnotateByteSpans_DocumentSpansEntireSource(t *testing.T) {
	cast := ParseMarkdown(simpleSource)
	annotated := AnnotateByteSpans(cast, simpleSource)

	doc := annotated[0]
	if doc.ByteSpan.StartByte != 0 {
		t.Fatalf("document start_byte should be 0, got %d", doc.ByteSpan.StartByte)
	}
	if doc.ByteSpan.EndByte != len(simpleSource) {
		t.Fatalf("document end_byte should be %d, got %d", len(simpleSource), doc.ByteSpan.EndByte)
	}
}

func TestCheckSpanConformance_SimpleDoc(t *testing.T) {
	cast := ParseMarkdown(simpleSource)
	result := CheckSpanConformance(cast, simpleSource)

	if result.Checked == 0 {
		t.Fatal("expected checked nodes > 0")
	}
	if result.CoverageRatio <= 0 {
		t.Fatalf("expected positive coverage ratio, got %f", result.CoverageRatio)
	}
	if result.TotalBytes != len(simpleSource) {
		t.Fatalf("expected total_bytes %d, got %d", len(simpleSource), result.TotalBytes)
	}
	// Log results for visibility.
	t.Logf("verdict=%s checked=%d conformant=%d non_conformant=%d skipped=%d coverage=%.2f",
		result.Verdict, result.Checked, result.Conformant, result.NonConformant, result.Skipped, result.CoverageRatio)
}

func TestCheckSpanConformance_Fixture(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	result := CheckSpanConformance(cast, src)

	if result.Checked == 0 {
		t.Fatal("expected checked nodes > 0")
	}
	t.Logf("fixture: verdict=%s checked=%d conformant=%d non_conformant=%d coverage=%.2f violations=%d",
		result.Verdict, result.Checked, result.Conformant, result.NonConformant, result.CoverageRatio, len(result.Violations))
}

func TestCheckSpanConformance_HeadingRoundtrip(t *testing.T) {
	src := "# Hello World\n\nBody text here.\n"
	cast := ParseMarkdown(src)
	result := CheckSpanConformance(cast, src)

	// Heading raw_text should match source bytes.
	if result.NonConformant > 0 {
		for _, v := range result.Violations {
			t.Logf("violation: %s kind=%s reason=%s expected=%q actual=%q",
				v.NodeID, v.Kind, v.Reason, v.Expected, v.Actual)
		}
		t.Fatalf("expected 0 non-conformant for simple heading doc, got %d", result.NonConformant)
	}
}

func TestCheckSpanConformance_CodeBlockRoundtrip(t *testing.T) {
	src := "# Doc\n\n```python\ndef f():\n    pass\n```\n"
	cast := ParseMarkdown(src)
	result := CheckSpanConformance(cast, src)

	if result.NonConformant > 0 {
		for _, v := range result.Violations {
			t.Logf("violation: %s kind=%s reason=%s", v.NodeID, v.Kind, v.Reason)
		}
	}
	// Code block content should be found in source.
	if result.Conformant == 0 {
		t.Fatal("expected at least 1 conformant node")
	}
}

func TestCheckSpanConformance_Empty(t *testing.T) {
	cast := ParseMarkdown("")
	result := CheckSpanConformance(cast, "")

	if result.Verdict != "conformant" {
		t.Fatalf("expected conformant for empty, got %s", result.Verdict)
	}
	if result.TotalBytes != 0 {
		t.Fatalf("expected 0 total_bytes, got %d", result.TotalBytes)
	}
}

func TestCheckSpanConformance_SkipsDocumentNode(t *testing.T) {
	cast := ParseMarkdown("# Title\n")
	result := CheckSpanConformance(cast, "# Title\n")

	if result.Skipped < 1 {
		t.Fatal("expected document node to be skipped")
	}
}

func TestCheckSpanConformance_ViolationDetails(t *testing.T) {
	// Construct a CAST with a deliberately wrong RawText.
	cast := CAST{
		Root: "doc-root",
		Nodes: []CNode{
			{ID: "doc-root", Kind: KindDocument, Span: Span{StartLine: 1, EndLine: 1}},
			{ID: "bad-node", Kind: KindParagraph, Text: "WRONG TEXT",
				RawText: "WRONG TEXT", ParentID: "doc-root",
				Span: Span{StartLine: 1, EndLine: 1}},
		},
	}
	result := CheckSpanConformance(cast, "Actual source text.\n")

	if result.NonConformant != 1 {
		t.Fatalf("expected 1 non-conformant, got %d", result.NonConformant)
	}
	if len(result.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].NodeID != "bad-node" {
		t.Fatalf("expected violation on bad-node, got %s", result.Violations[0].NodeID)
	}
	if result.Verdict != "non_conformant" {
		t.Fatalf("expected non_conformant verdict, got %s", result.Verdict)
	}
}

func TestCheckSpanConformance_CoverageBytes(t *testing.T) {
	src := "# Title\n\nParagraph.\n"
	cast := ParseMarkdown(src)
	result := CheckSpanConformance(cast, src)

	if result.CoveredBytes == 0 {
		t.Fatal("expected covered bytes > 0")
	}
	if result.CoveredBytes > result.TotalBytes {
		t.Fatalf("covered %d > total %d", result.CoveredBytes, result.TotalBytes)
	}
}

func TestComputeLineOffsets(t *testing.T) {
	src := "line1\nline2\nline3\n"
	offsets := computeLineOffsets(src)

	if len(offsets) != 4 {
		t.Fatalf("expected 4 offsets (3 lines + trailing), got %d", len(offsets))
	}
	if offsets[0] != 0 {
		t.Fatalf("first offset should be 0, got %d", offsets[0])
	}
	if offsets[1] != 6 {
		t.Fatalf("second offset should be 6, got %d", offsets[1])
	}
	if offsets[2] != 12 {
		t.Fatalf("third offset should be 12, got %d", offsets[2])
	}
}

func TestComputeLineOffsets_SingleLine(t *testing.T) {
	offsets := computeLineOffsets("hello")
	if len(offsets) != 1 {
		t.Fatalf("expected 1 offset for no newlines, got %d", len(offsets))
	}
	if offsets[0] != 0 {
		t.Fatalf("expected offset 0, got %d", offsets[0])
	}
}

func TestComputeLineOffsets_Empty(t *testing.T) {
	offsets := computeLineOffsets("")
	if len(offsets) != 1 {
		t.Fatalf("expected 1 offset for empty, got %d", len(offsets))
	}
}

func TestMatchesSource_Exact(t *testing.T) {
	if !matchesSource("hello", "hello") {
		t.Fatal("exact match should pass")
	}
}

func TestMatchesSource_TrimmedMatch(t *testing.T) {
	if !matchesSource("hello", "  hello  ") {
		t.Fatal("trimmed match should pass")
	}
}

func TestMatchesSource_ContainedMatch(t *testing.T) {
	if !matchesSource("hello", "prefix hello suffix") {
		t.Fatal("contained match should pass")
	}
}

func TestMatchesSource_Mismatch(t *testing.T) {
	if matchesSource("hello", "world") {
		t.Fatal("mismatch should fail")
	}
}

func TestResolveByteSpan_LineToBytes(t *testing.T) {
	src := "line1\nline2\nline3\n"
	offsets := computeLineOffsets(src)

	node := CNode{Span: Span{StartLine: 2, EndLine: 2}}
	bs := resolveByteSpan(node, offsets, len(src))

	if bs.StartByte != 6 {
		t.Fatalf("expected start_byte 6, got %d", bs.StartByte)
	}
	// Line 2 ends at offset 12 (start of line 3).
	if bs.EndByte != 12 {
		t.Fatalf("expected end_byte 12, got %d", bs.EndByte)
	}
}

func TestResolveByteSpan_MultiLine(t *testing.T) {
	src := "aaa\nbbb\nccc\n"
	offsets := computeLineOffsets(src)

	node := CNode{Span: Span{StartLine: 1, EndLine: 3}}
	bs := resolveByteSpan(node, offsets, len(src))

	if bs.StartByte != 0 {
		t.Fatalf("expected start_byte 0, got %d", bs.StartByte)
	}
	if bs.EndByte != len(src) {
		t.Fatalf("expected end_byte %d, got %d", len(src), bs.EndByte)
	}
}
