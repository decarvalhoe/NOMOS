package parse

import (
	"strings"
	"testing"
)

func TestSourceSpanRequiresPreciseAnchor(t *testing.T) {
	span := SourceSpan{
		SourceID: "SRC-RBOK-001",
		Path:     "01_rbok/example.md",
		Hash:     testHash("a"),
	}

	errs := span.Validate()
	if !hasError(errs, "at least one source selector") {
		t.Fatalf("expected missing selector error, got %v", errs)
	}
}

func TestSourceSpanAcceptsLineColumnByteAndQuoteSelectors(t *testing.T) {
	span := SourceSpan{
		SourceID:    "SRC-RBOK-001",
		Path:        "01_rbok/example.md",
		Hash:        testHash("b"),
		StartLine:   intPtr(10),
		EndLine:     intPtr(12),
		StartColumn: intPtr(1),
		EndColumn:   intPtr(42),
		StartByte:   intPtr(120),
		EndByte:     intPtr(240),
		TextQuote: &TextQuoteSelector{
			Exact:  "A governed statement.",
			Prefix: "Before ",
			Suffix: " after.",
		},
	}

	if errs := span.Validate(); len(errs) != 0 {
		t.Fatalf("expected valid span, got %v", errs)
	}
}

func TestDocumentASTRejectsUnsupportedNodeWithoutFinding(t *testing.T) {
	ast := DocumentAST{
		SourceID: "SRC-RBOK-001",
		Parser: ParserInfo{
			Name:    "commonmark",
			Version: "0.31.2",
		},
		Root: ASTNode{
			NodeID:   "AST-ROOT",
			NodeType: NodeRoot,
			Span:     validSpan(),
			Children: []ASTNode{
				{
					NodeID:   "AST-HTML",
					NodeType: NodeUnsupportedBlock,
					Span:     validSpan(),
					Raw:      "<custom/>",
				},
			},
		},
	}

	errs := ast.Validate()
	if !hasError(errs, "unsupported node AST-HTML must include a finding") {
		t.Fatalf("expected unsupported finding error, got %v", errs)
	}
}

func TestDocumentASTAcceptsUnsupportedNodeWithFinding(t *testing.T) {
	ast := DocumentAST{
		SourceID: "SRC-RBOK-001",
		Parser: ParserInfo{
			Name:    "commonmark",
			Version: "0.31.2",
		},
		Root: ASTNode{
			NodeID:   "AST-ROOT",
			NodeType: NodeRoot,
			Span:     validSpan(),
			Children: []ASTNode{
				{
					NodeID:   "AST-HTML",
					NodeType: NodeUnsupportedBlock,
					Span:     validSpan(),
					Raw:      "<custom/>",
					Findings: []Finding{
						{
							Code:     "unsupported_block.raw_html",
							Severity: SeverityWarning,
							Message:  "Raw HTML is preserved but not semantically interpreted.",
						},
					},
				},
			},
		},
	}

	if errs := ast.Validate(); len(errs) != 0 {
		t.Fatalf("expected valid AST, got %v", errs)
	}
}

func TestDocumentASTRejectsDuplicateNodeIDs(t *testing.T) {
	ast := DocumentAST{
		SourceID: "SRC-RBOK-001",
		Parser: ParserInfo{
			Name:    "commonmark",
			Version: "0.31.2",
		},
		Root: ASTNode{
			NodeID:   "AST-ROOT",
			NodeType: NodeRoot,
			Span:     validSpan(),
			Children: []ASTNode{
				{NodeID: "AST-DUP", NodeType: NodeParagraph, Span: validSpan(), Text: "one"},
				{NodeID: "AST-DUP", NodeType: NodeParagraph, Span: validSpan(), Text: "two"},
			},
		},
	}

	errs := ast.Validate()
	if !hasError(errs, "duplicate node_id AST-DUP") {
		t.Fatalf("expected duplicate id error, got %v", errs)
	}
}

func validSpan() SourceSpan {
	return SourceSpan{
		SourceID:  "SRC-RBOK-001",
		Path:      "01_rbok/example.md",
		Hash:      testHash("c"),
		StartLine: intPtr(1),
		EndLine:   intPtr(1),
	}
}

func intPtr(v int) *int {
	return &v
}

func testHash(char string) string {
	return "sha256:" + strings.Repeat(char, 64)
}

func hasError(errs []string, needle string) bool {
	for _, err := range errs {
		if strings.Contains(err, needle) {
			return true
		}
	}
	return false
}
