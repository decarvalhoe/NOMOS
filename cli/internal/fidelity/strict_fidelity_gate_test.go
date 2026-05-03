package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func cleanCAST() *CAST {
	return &CAST{
		Root: "doc", SourceHash: "sha256:src",
		Nodes: []CNode{
			{ID: "doc", Kind: KindDocument, Span: Span{1, 10}},
			{ID: "h1", Kind: KindHeading, Text: "Title", Level: 1, Span: Span{1, 1}, Hash: "h1h"},
			{ID: "p1", Kind: KindParagraph, Text: "Content.", Span: Span{3, 5}, Hash: "p1h"},
			{ID: "tbl", Kind: KindTable, Text: "2x3 table", Span: Span{7, 9}, Hash: "th",
				Props: map[string]string{"col_count": "3", "row_count": "2"}},
		},
	}
}

func cleanTOC() *TOCArtifact {
	toc := GenerateTOCFromHeadings(
		[]HeadingInput{{ID: "h1", Title: "Title", Level: 1, Hash: "h1h"}},
		"doc", "sha256:src", DefaultTOCConfig(),
	)
	return &toc
}

func cleanInput() StrictGateInput {
	return StrictGateInput{
		CAST:       cleanCAST(),
		TOC:        cleanTOC(),
		DocumentID: "doc",
		SourceLen:  100,
	}
}

// --- Pass ---

func TestStrictGatePass(t *testing.T) {
	result := RunStrictFidelityGate(cleanInput())
	if !result.Pass {
		t.Fatalf("expected pass, got findings: %v", result.Findings)
	}
	if result.BlockingCount != 0 {
		t.Fatalf("expected 0 blocking, got %d", result.BlockingCount)
	}
	if !strings.HasPrefix(result.GateHash, "sha256:") {
		t.Fatalf("expected gate hash, got %q", result.GateHash)
	}
}

// --- Untyped table ---

func TestStrictGateUntypedTable(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes[3].Props = nil // remove table metadata
	result := RunStrictFidelityGate(input)
	if result.Pass {
		t.Fatal("expected fail for untyped table")
	}
	assertStrictFinding(t, result, "UNTYPED_TABLE")
}

// --- Missing span ---

func TestStrictGateMissingSpan(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes[2].Span = Span{0, 0} // zero span
	result := RunStrictFidelityGate(input)
	if result.Pass {
		t.Fatal("expected fail for missing span")
	}
	assertStrictFinding(t, result, "MISSING_SPAN")
}

// --- Inverted span ---

func TestStrictGateInvertedSpan(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes[2].Span = Span{10, 5}
	result := RunStrictFidelityGate(input)
	assertStrictFinding(t, result, "INVERTED_SPAN")
}

// --- TOC missing ---

func TestStrictGateTOCMissing(t *testing.T) {
	input := cleanInput()
	input.TOC = nil
	result := RunStrictFidelityGate(input)
	if result.Pass {
		t.Fatal("expected fail for missing TOC")
	}
	assertStrictFinding(t, result, "TOC_MISSING")
}

// --- TOC not certified ---

func TestStrictGateTOCNotCertified(t *testing.T) {
	input := cleanInput()
	toc := *input.TOC
	toc.Certified = false
	input.TOC = &toc
	result := RunStrictFidelityGate(input)
	assertStrictFinding(t, result, "TOC_NOT_CERTIFIED")
}

// --- TOC tampered ---

func TestStrictGateTOCTampered(t *testing.T) {
	input := cleanInput()
	toc := *input.TOC
	toc.ArtifactHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	input.TOC = &toc
	result := RunStrictFidelityGate(input)
	assertStrictFinding(t, result, "TOC_HASH_INVALID")
}

// --- Lexicon ungoverned ---

func TestStrictGateLexiconCanonicalPasses(t *testing.T) {
	input := cleanInput()
	lex := NewLexicon()
	lex.Add(Term{Canonical: "franchise"})
	input.Lexicon = lex
	result := RunStrictFidelityGate(input)
	// All terms via Add() are TermCanonical → no ungoverned finding
	if !result.Pass {
		t.Fatal("lexicon with canonical terms should pass")
	}
	for _, f := range result.Findings {
		if f.Code == "LEXICON_UNGOVERNED" {
			t.Fatal("unexpected ungoverned finding for canonical term")
		}
	}
}

// --- Lexicon governed (no finding) ---

func TestStrictGateLexiconGoverned(t *testing.T) {
	input := cleanInput()
	lex := NewLexicon()
	lex.Add(Term{Canonical: "franchise", Status: TermCanonical})
	input.Lexicon = lex
	result := RunStrictFidelityGate(input)
	for _, f := range result.Findings {
		if f.Code == "LEXICON_UNGOVERNED" {
			t.Fatal("unexpected ungoverned finding for approved term")
		}
	}
}

// --- Code block no language (non-blocking) ---

func TestStrictGateCodeBlockNoLang(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes = append(input.CAST.Nodes, CNode{
		ID: "cb1", Kind: KindCodeBlock, Text: "code", Span: Span{11, 13}, Hash: "cbh",
	})
	result := RunStrictFidelityGate(input)
	// Non-blocking
	if !result.Pass {
		t.Fatal("code block without lang should not block")
	}
	assertStrictFinding(t, result, "CODE_BLOCK_NO_LANG")
}

// --- Image no alt text (non-blocking) ---

func TestStrictGateImageNoAlt(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes = append(input.CAST.Nodes, CNode{
		ID: "img1", Kind: KindImage, Text: "", Span: Span{14, 14}, Hash: "imh",
		Props: map[string]string{"url": "x.png", "alt_text": ""},
	})
	result := RunStrictFidelityGate(input)
	if !result.Pass {
		t.Fatal("image without alt should not block")
	}
	assertStrictFinding(t, result, "IMAGE_NO_ALT")
}

// --- Excessive empty nodes ---

func TestStrictGateExcessiveEmpty(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes = []CNode{
		{ID: "doc", Kind: KindDocument},
		{ID: "h1", Kind: KindHeading, Text: "", Span: Span{1, 1}},
		{ID: "p1", Kind: KindParagraph, Text: "", Span: Span{2, 2}},
		{ID: "p2", Kind: KindParagraph, Text: "", Span: Span{3, 3}},
	}
	result := RunStrictFidelityGate(input)
	assertStrictFinding(t, result, "EXCESSIVE_EMPTY_NODES")
}

// --- No CAST ---

func TestStrictGateNoCAST(t *testing.T) {
	input := StrictGateInput{TOC: cleanTOC(), DocumentID: "doc"}
	result := RunStrictFidelityGate(input)
	// No CAST checks fire → depends on TOC only
	if !result.Pass {
		t.Fatalf("expected pass without CAST, got findings: %v", result.Findings)
	}
}

// --- Multiple blockers ---

func TestStrictGateMultipleBlockers(t *testing.T) {
	input := cleanInput()
	input.CAST.Nodes[2].Span = Span{0, 0}
	input.CAST.Nodes[3].Props = nil
	input.TOC = nil
	result := RunStrictFidelityGate(input)
	if result.Pass {
		t.Fatal("expected fail with multiple blockers")
	}
	if result.BlockingCount < 3 {
		t.Fatalf("expected >= 3 blocking, got %d", result.BlockingCount)
	}
}

// --- Verify gate hash ---

func TestVerifyStrictGate(t *testing.T) {
	result := RunStrictFidelityGate(cleanInput())
	if !VerifyStrictGate(result) {
		t.Fatal("expected verification pass")
	}
}

func TestVerifyStrictGateTampered(t *testing.T) {
	result := RunStrictFidelityGate(cleanInput())
	result.Pass = !result.Pass
	if VerifyStrictGate(result) {
		t.Fatal("expected verification fail")
	}
}

// --- Hash determinism ---

func TestStrictGateHashDeterministic(t *testing.T) {
	r1 := RunStrictFidelityGate(cleanInput())
	r2 := RunStrictFidelityGate(cleanInput())
	if r1.GateHash != r2.GateHash {
		t.Fatal("gate hash not deterministic")
	}
}

// --- JSON roundtrip ---

func TestStrictGateJSON(t *testing.T) {
	result := RunStrictFidelityGate(cleanInput())
	var buf bytes.Buffer
	if err := WriteStrictGateJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	var decoded StrictGateResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Pass != result.Pass {
		t.Fatal("roundtrip mismatch")
	}
	if decoded.GateHash != result.GateHash {
		t.Fatal("hash mismatch")
	}
}

// --- helper ---

func assertStrictFinding(t *testing.T, result StrictGateResult, code string) {
	t.Helper()
	for _, f := range result.Findings {
		if f.Code == code {
			return
		}
	}
	var codes []string
	for _, f := range result.Findings {
		codes = append(codes, f.Code)
	}
	t.Fatalf("expected finding %q in %v", code, codes)
}
