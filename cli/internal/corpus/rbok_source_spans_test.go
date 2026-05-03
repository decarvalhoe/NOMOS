package corpus

import (
	"strings"
	"testing"
)

const rbokFixtureDoc = `# Code des assurances
| Champ | Valeur |
|---|---|
| Reference | CA-2026 |
| Statut | Actif |

## Chapitre 1 - Garanties

La garantie habitation couvre les risques suivants.

### Section 1.1 - Dégât des eaux

L'assuré est couvert pour les dégâts des eaux sous réserve des exclusions.

- Fuite de canalisation
- Infiltration par toiture
- Débordement d'appareil ménager

### Section 1.2 - Incendie

La garantie incendie couvre les dommages causés par le feu.

## Chapitre 2 - Exclusions

Les exclusions suivantes s'appliquent à toutes les garanties.
`

// --- Spans populated ---

func TestExtractWithSpansPopulated(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "01_rbok/code-assurances.md")
	for _, n := range result.Nodes {
		if !n.Span.IsValid() {
			t.Fatalf("node %s (%s) has invalid span", n.NodeID, n.NodeType)
		}
		if n.Span.File != "01_rbok/code-assurances.md" {
			t.Fatalf("node %s: expected file path, got %q", n.NodeID, n.Span.File)
		}
		if n.Span.StartLine < 1 {
			t.Fatalf("node %s: start_line %d < 1", n.NodeID, n.Span.StartLine)
		}
		if n.Span.StartCol < 1 {
			t.Fatalf("node %s: start_col %d < 1", n.NodeID, n.Span.StartCol)
		}
	}
}

// --- Byte offsets ---

func TestExtractWithSpansByteOffsets(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	for _, n := range result.Nodes {
		if n.Span.ByteOffset < 0 {
			t.Fatalf("node %s: negative byte offset %d", n.NodeID, n.Span.ByteOffset)
		}
		if n.Span.ByteLength <= 0 && n.Text != "" {
			t.Fatalf("node %s: zero byte length for non-empty text", n.NodeID)
		}
	}
}

// --- Heading nodes have correct lines ---

func TestExtractWithSpansHeadingLines(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	headings := map[string]int{} // title → line
	for _, n := range result.Nodes {
		if n.NodeType == NodeDocument || n.NodeType == NodeChapter || n.NodeType == NodeSection {
			headings[n.Title] = n.Span.StartLine
		}
	}
	// H1 is line 1
	if headings["Code des assurances"] != 1 {
		t.Fatalf("expected H1 at line 1, got %d", headings["Code des assurances"])
	}
	// "Chapitre 1 - Garanties" comes after metadata table
	if line, ok := headings["Chapitre 1 - Garanties"]; ok && line < 6 {
		t.Fatalf("expected chapter after metadata, got line %d", line)
	}
}

// --- Span ordering ---

func TestExtractWithSpansOrdering(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	// Heading nodes should have increasing start lines
	var headingLines []int
	for _, n := range result.Nodes {
		if n.NodeType == NodeDocument || n.NodeType == NodeChapter || n.NodeType == NodeSection {
			headingLines = append(headingLines, n.Span.StartLine)
		}
	}
	for i := 1; i < len(headingLines); i++ {
		if headingLines[i] < headingLines[i-1] {
			t.Fatalf("heading lines not ordered: %d before %d", headingLines[i-1], headingLines[i])
		}
	}
}

// --- Span coverage ---

func TestSpanCoverageFullDoc(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	cov := ComputeSpanCoverage(result.Nodes, len(rbokFixtureDoc))
	if cov.TotalNodes == 0 {
		t.Fatal("expected nodes")
	}
	if cov.WithSpan == 0 {
		t.Fatal("expected nodes with spans")
	}
	if cov.CoverageRatio <= 0 {
		t.Fatalf("expected positive coverage ratio, got %.2f", cov.CoverageRatio)
	}
	if cov.CoverageRatio > 1.0 {
		t.Fatalf("coverage ratio > 1.0: %.2f", cov.CoverageRatio)
	}
}

func TestSpanCoverageAllHaveSpans(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	cov := ComputeSpanCoverage(result.Nodes, len(rbokFixtureDoc))
	if cov.WithoutSpan != 0 {
		t.Fatalf("expected 0 without span, got %d", cov.WithoutSpan)
	}
	if cov.WithSpan != cov.TotalNodes {
		t.Fatalf("expected all with span: %d/%d", cov.WithSpan, cov.TotalNodes)
	}
}

func TestSpanCoverageEmpty(t *testing.T) {
	cov := ComputeSpanCoverage(nil, 0)
	if cov.TotalNodes != 0 || cov.CoverageRatio != 0 {
		t.Fatal("expected zero for empty")
	}
}

// --- Source span string ---

func TestSourceSpanStringSingleLine(t *testing.T) {
	s := LawbookSourceSpan{File: "doc.md", StartLine: 5, EndLine: 5}
	if s.String() != "doc.md:5" {
		t.Fatalf("expected doc.md:5, got %q", s.String())
	}
}

func TestSourceSpanStringRange(t *testing.T) {
	s := LawbookSourceSpan{File: "doc.md", StartLine: 5, EndLine: 10}
	if s.String() != "doc.md:5-10" {
		t.Fatalf("expected doc.md:5-10, got %q", s.String())
	}
}

func TestSourceSpanIsValid(t *testing.T) {
	valid := LawbookSourceSpan{StartLine: 1, EndLine: 3}
	if !valid.IsValid() {
		t.Fatal("expected valid")
	}
	invalid := LawbookSourceSpan{StartLine: 0, EndLine: 0}
	if invalid.IsValid() {
		t.Fatal("expected invalid")
	}
}

// --- Backward compat ---

func TestExtractMarkdownLegacyStillWorks(t *testing.T) {
	result := ExtractMarkdown(rbokFixtureDoc, "ca-2026")
	if len(result.Nodes) == 0 {
		t.Fatal("expected nodes from legacy API")
	}
	// Spans should still be populated (with empty file)
	for _, n := range result.Nodes {
		if n.Span.StartLine < 1 {
			t.Fatalf("legacy node %s missing start_line", n.NodeID)
		}
	}
}

// --- Node types from fixture ---

func TestExtractFixtureNodeTypes(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	types := map[LawbookNodeType]int{}
	for _, n := range result.Nodes {
		types[n.NodeType]++
	}
	if types[NodeDocument] != 1 {
		t.Fatalf("expected 1 document, got %d", types[NodeDocument])
	}
	if types[NodeChapter] < 2 {
		t.Fatalf("expected >= 2 chapters, got %d", types[NodeChapter])
	}
	if types[NodeSection] < 2 {
		t.Fatalf("expected >= 2 sections, got %d", types[NodeSection])
	}
}

// --- List items become alineas with spans ---

func TestExtractListItemAlineasHaveSpans(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	alineaCount := 0
	for _, n := range result.Nodes {
		if n.NodeType == NodeAlinea {
			alineaCount++
			if !n.Span.IsValid() {
				t.Fatalf("alinea %s has invalid span", n.NodeID)
			}
			if n.Span.ByteLength <= 0 {
				t.Fatalf("alinea %s has zero byte length", n.NodeID)
			}
		}
	}
	if alineaCount < 3 {
		t.Fatalf("expected >= 3 alineas from list items, got %d", alineaCount)
	}
}

// --- Metadata extraction still works ---

func TestExtractMetadataWithSpans(t *testing.T) {
	result := ExtractMarkdownWithSpans(rbokFixtureDoc, "ca-2026", "test.md")
	var docNode *LawbookNode
	for i := range result.Nodes {
		if result.Nodes[i].NodeType == NodeDocument {
			docNode = &result.Nodes[i]
			break
		}
	}
	if docNode == nil {
		t.Fatal("expected document node")
	}
	if docNode.Metadata == nil {
		t.Fatal("expected metadata from table")
	}
	ref, _ := docNode.Metadata["reference"].(string)
	if !strings.Contains(ref, "CA-2026") {
		t.Fatalf("expected CA-2026 in metadata, got %q", ref)
	}
}

// --- computeLineOffsets ---

func TestComputeLineOffsets(t *testing.T) {
	offsets := computeLineOffsets("abc\ndef\nghi")
	if len(offsets) != 3 {
		t.Fatalf("expected 3 offsets, got %d", len(offsets))
	}
	if offsets[0] != 0 || offsets[1] != 4 || offsets[2] != 8 {
		t.Fatalf("expected [0,4,8], got %v", offsets)
	}
}
