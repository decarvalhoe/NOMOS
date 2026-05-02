package atomization

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sampleAST() AST {
	return AST{
		Root:       "doc-root",
		SourceHash: "sha256:abc123def456",
		SourceLen:  500,
		Blocks: []Block{
			{ID: "doc-root", Type: BlockDocument, Text: ""},
			{ID: "h1", Type: BlockHeading, Level: 1, Text: "Code des assurances", ParentID: "doc-root",
				Span: Span{StartLine: 1, EndLine: 1}, Hash: "sha256:h1hash", Children: []string{"h2a"}},
			{ID: "h2a", Type: BlockHeading, Level: 2, Text: "Garanties", ParentID: "h1",
				Span: Span{StartLine: 3, EndLine: 3}, Hash: "sha256:h2hash", Children: []string{"p1"}},
			{ID: "p1", Type: BlockParagraph, Text: "La garantie couvre les dégâts des eaux.", ParentID: "h2a",
				Span: Span{StartLine: 5, EndLine: 5}, Hash: "sha256:p1hash"},
			{ID: "list1", Type: BlockList, Text: "- item a\n- item b", ParentID: "h2a",
				Span: Span{StartLine: 7, EndLine: 8}, Hash: "sha256:listhash"},
			{ID: "blank", Type: BlockBlankLine, Span: Span{StartLine: 9, EndLine: 9}},
		},
		LossReport: LossReport{IsLossless: true},
	}
}

func defaultInput() EngineProfileInput {
	return EngineProfileInput{
		SourcePath:   "02_domaines/assurance-habitation/garanties.md",
		Domain:       "insurance-home",
		Owner:        "actuarial@example.com",
		Version:      "2026",
		Jurisdiction: "FR",
		DocumentType: "regulation",
		Status:       "active",
	}
}

func TestProjectASTProducesDocument(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	if out.Document.ExternalID == "" {
		t.Fatal("expected non-empty document external_id")
	}
	if out.Document.Title != "Code des assurances" {
		t.Fatalf("expected title from H1, got %q", out.Document.Title)
	}
	if out.Document.DocumentType != "regulation" {
		t.Fatalf("expected regulation, got %q", out.Document.DocumentType)
	}
	if out.Document.Jurisdiction != "FR" {
		t.Fatalf("expected FR, got %q", out.Document.Jurisdiction)
	}
	if out.Document.Domain != "insurance-home" {
		t.Fatalf("expected insurance-home, got %q", out.Document.Domain)
	}
	if out.Document.Owner != "actuarial@example.com" {
		t.Fatalf("expected owner, got %q", out.Document.Owner)
	}
	if out.Document.SourceHash != "sha256:abc123def456" {
		t.Fatalf("expected source hash, got %q", out.Document.SourceHash)
	}
}

func TestProjectASTProducesNodes(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	// Should skip document root and blank line → 4 nodes
	if len(out.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(out.Nodes))
	}
	for _, n := range out.Nodes {
		if n.ExternalID == "" {
			t.Fatal("node missing external_id")
		}
		if n.DocumentExternalID == "" {
			t.Fatal("node missing document_external_id")
		}
		if n.NodeType == "" {
			t.Fatalf("node %s missing node_type", n.ExternalID)
		}
		if n.CanonicalRef == "" {
			t.Fatalf("node %s missing canonical_ref", n.ExternalID)
		}
		if n.SourceHash == "" {
			t.Fatalf("node %s missing source_hash", n.ExternalID)
		}
	}
}

func TestProjectASTNodeTypes(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	types := map[string]bool{}
	for _, n := range out.Nodes {
		types[n.NodeType] = true
	}
	if !types["section"] {
		t.Fatal("expected section node type from headings")
	}
	if !types["article"] {
		t.Fatal("expected article node type from paragraphs/lists")
	}
}

func TestProjectASTStructureOnly(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	for _, n := range out.Nodes {
		if n.ExternalID == "h1" {
			if !n.StructureOnly {
				t.Fatal("H1 with children should be structure_only")
			}
			if n.Content != "" {
				t.Fatal("structure_only node should have empty content")
			}
		}
		if n.ExternalID == "p1" {
			if n.StructureOnly {
				t.Fatal("paragraph should not be structure_only")
			}
			if n.Content == "" {
				t.Fatal("paragraph should have content")
			}
		}
	}
}

func TestProjectASTDepths(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	depths := map[string]int{}
	for _, n := range out.Nodes {
		depths[n.ExternalID] = n.Depth
	}
	if depths["h1"] != 0 {
		t.Fatalf("expected h1 depth 0, got %d", depths["h1"])
	}
	if depths["h2a"] != 1 {
		t.Fatalf("expected h2a depth 1, got %d", depths["h2a"])
	}
	if depths["p1"] != 2 {
		t.Fatalf("expected p1 depth 2, got %d", depths["p1"])
	}
}

func TestProjectASTRevision(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	if out.Revision.RevisionNumber != 1 {
		t.Fatalf("expected revision 1, got %d", out.Revision.RevisionNumber)
	}
	if out.Revision.DocumentExternalID != out.Document.ExternalID {
		t.Fatal("revision document_external_id mismatch")
	}
	if out.Revision.NodeCount != len(out.Nodes) {
		t.Fatalf("expected node_count %d, got %d", len(out.Nodes), out.Revision.NodeCount)
	}
	if out.Revision.CreatedBy != "nomos-cli" {
		t.Fatalf("expected nomos-cli, got %q", out.Revision.CreatedBy)
	}
	if out.Revision.SourceHash != out.Document.SourceHash {
		t.Fatal("revision source_hash should match document")
	}
}

func TestProjectASTGovernanceGatePass(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	if !out.GovernanceGate.PublishAllowed {
		t.Fatalf("expected publish allowed, got findings: %v", out.GovernanceGate.Findings)
	}
}

func TestProjectASTGovernanceGateBlocksMissingOwner(t *testing.T) {
	input := defaultInput()
	input.Owner = ""
	out := ProjectAST(sampleAST(), input)
	if out.GovernanceGate.PublishAllowed {
		t.Fatal("expected publish blocked for missing owner")
	}
	found := false
	for _, f := range out.GovernanceGate.Findings {
		if f.Code == "corpus_partial" && strings.Contains(f.Message, "owner") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected corpus_partial finding for owner")
	}
}

func TestProjectASTGovernanceGateBlocksMissingDomain(t *testing.T) {
	input := defaultInput()
	input.Domain = ""
	out := ProjectAST(sampleAST(), input)
	if out.GovernanceGate.PublishAllowed {
		t.Fatal("expected publish blocked for missing domain")
	}
}

func TestProjectASTContractVersion(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	if out.ContractVersion != EngineProfileVersion {
		t.Fatalf("expected %s, got %s", EngineProfileVersion, out.ContractVersion)
	}
}

func TestProjectASTCanonicalRefsUnique(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	seen := map[string]bool{}
	for _, n := range out.Nodes {
		if seen[n.CanonicalRef] {
			t.Fatalf("duplicate canonical_ref: %s", n.CanonicalRef)
		}
		seen[n.CanonicalRef] = true
	}
}

func TestProjectASTDisplayRef(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	for _, n := range out.Nodes {
		if n.DisplayRef == "" {
			t.Fatalf("node %s missing display_ref", n.ExternalID)
		}
	}
}

func TestProjectASTPriority(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	for _, n := range out.Nodes {
		if n.Priority <= 0 {
			t.Fatalf("node %s has non-positive priority %d", n.ExternalID, n.Priority)
		}
	}
}

func TestWriteEngineJSON(t *testing.T) {
	out := ProjectAST(sampleAST(), defaultInput())
	var buf bytes.Buffer
	if err := WriteEngineJSON(&buf, out); err != nil {
		t.Fatal(err)
	}
	var decoded EngineProfileOutput
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Document.ExternalID != out.Document.ExternalID {
		t.Fatal("roundtrip mismatch")
	}
	if len(decoded.Nodes) != len(out.Nodes) {
		t.Fatal("node count mismatch")
	}
}

func TestProjectASTDefaultsApplied(t *testing.T) {
	out := ProjectAST(sampleAST(), EngineProfileInput{SourcePath: "test.md"})
	if out.Document.DocumentType != "internal" {
		t.Fatalf("expected default internal, got %q", out.Document.DocumentType)
	}
	if out.Document.Jurisdiction != "FR" {
		t.Fatalf("expected default FR, got %q", out.Document.Jurisdiction)
	}
}
