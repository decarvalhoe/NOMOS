package corpus

import (
	"strings"
	"testing"
)

func validNode() LawbookNode {
	return LawbookNode{
		NodeID:       "DOC-CIVIL-CODE",
		DocumentID:   "DOC-CIVIL-CODE",
		NodeType:     NodeDocument,
		CanonicalRef: "civil-code",
		DisplayRef:   "Code civil",
		Depth:        0,
		OrdinalPath:  "1",
		SourcePath:   "sources/civil-code.pdf",
		SourceHash:   "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Status:       StatusActive,
		Priority:     PriorityHigh,
		Domain:       "civil-law",
	}
}

func validFeed() LawbookFeed {
	node := validNode()
	article := LawbookNode{
		NodeID:       "ART-1234",
		DocumentID:   "DOC-CIVIL-CODE",
		NodeType:     NodeArticle,
		CanonicalRef: "civil-code/art-1234",
		DisplayRef:   "Art. 1234",
		Depth:        4,
		OrdinalPath:  "1.1.1.1.1",
		SourcePath:   "sources/civil-code.pdf",
		SourceHash:   "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		Status:       StatusActive,
		Priority:     PriorityMedium,
		Domain:       "civil-law",
		ParentID:     "DOC-CIVIL-CODE",
	}
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "civil-code-feed",
		DocumentID:    "DOC-CIVIL-CODE",
		Domain:        "civil-law",
		GeneratedAt:   "2026-05-02T10:00:00Z",
		SourcePath:    "sources/civil-code.pdf",
		SourceHash:    "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		NodeCount:     2,
		Nodes:         []LawbookNode{node, article},
	}
}

func TestValidNodePasses(t *testing.T) {
	errs := ValidateNode(validNode())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidFeedPasses(t *testing.T) {
	errs := ValidateFeed(validFeed())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestNodeInvalidNodeID(t *testing.T) {
	n := validNode()
	n.NodeID = "invalid-lowercase"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "node_id")
}

func TestNodeInvalidDocumentID(t *testing.T) {
	n := validNode()
	n.DocumentID = ""
	errs := ValidateNode(n)
	assertContainsError(t, errs, "document_id")
}

func TestNodeInvalidNodeType(t *testing.T) {
	n := validNode()
	n.NodeType = "invalid"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "node_type")
}

func TestNodeMissingCanonicalRef(t *testing.T) {
	n := validNode()
	n.CanonicalRef = ""
	errs := ValidateNode(n)
	assertContainsError(t, errs, "canonical_ref")
}

func TestNodeMissingDisplayRef(t *testing.T) {
	n := validNode()
	n.DisplayRef = "  "
	errs := ValidateNode(n)
	assertContainsError(t, errs, "display_ref")
}

func TestNodeDepthMismatch(t *testing.T) {
	n := validNode()
	n.Depth = 3 // document should be 0
	errs := ValidateNode(n)
	assertContainsError(t, errs, "depth")
}

func TestNodeInvalidOrdinalPath(t *testing.T) {
	n := validNode()
	n.OrdinalPath = "abc"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "ordinal_path")
}

func TestNodeInvalidSourceHash(t *testing.T) {
	n := validNode()
	n.SourceHash = "md5:abc123"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "source_hash")
}

func TestNodeInvalidStatus(t *testing.T) {
	n := validNode()
	n.Status = "unknown"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "status")
}

func TestNodeInvalidPriority(t *testing.T) {
	n := validNode()
	n.Priority = "urgent"
	errs := ValidateNode(n)
	assertContainsError(t, errs, "priority")
}

func TestNodeMissingDomain(t *testing.T) {
	n := validNode()
	n.Domain = ""
	errs := ValidateNode(n)
	assertContainsError(t, errs, "domain")
}

func TestFeedInvalidFeedID(t *testing.T) {
	f := validFeed()
	f.FeedID = "UPPERCASE"
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "feed_id")
}

func TestFeedNodeCountMismatch(t *testing.T) {
	f := validFeed()
	f.NodeCount = 99
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "node_count")
}

func TestFeedInvalidDocumentID(t *testing.T) {
	f := validFeed()
	f.DocumentID = "bad id"
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "document_id")
}

func TestFeedMissingDomain(t *testing.T) {
	f := validFeed()
	f.Domain = ""
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "domain")
}

func TestFeedMissingGeneratedAt(t *testing.T) {
	f := validFeed()
	f.GeneratedAt = ""
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "generated_at")
}

func TestFeedInvalidSourceHash(t *testing.T) {
	f := validFeed()
	f.SourceHash = "nope"
	errs := ValidateFeed(f)
	assertContainsError(t, errs, "source_hash")
}

func TestFeedPropagatesNodeErrors(t *testing.T) {
	f := validFeed()
	f.Nodes[0].NodeID = "bad"
	f.Nodes[0].Status = "bogus"
	errs := ValidateFeed(f)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
	assertContainsError(t, errs, "nodes[0]")
}

func TestAllNodeTypes(t *testing.T) {
	types := AllNodeTypes()
	if len(types) != 16 {
		t.Fatalf("expected 16 node types, got %d", len(types))
	}
	for _, nt := range types {
		if !nt.IsValid() {
			t.Fatalf("expected %s to be valid", nt)
		}
	}
}

func TestNodeTypeDepths(t *testing.T) {
	cases := []struct {
		nt    LawbookNodeType
		depth int
	}{
		{NodeDocument, 0},
		{NodeChapter, 1},
		{NodeSection, 2},
		{NodeSubsection, 3},
		{NodeArticle, 4},
		{NodeClause, 5},
		{NodeSubclause, 6},
		{NodeParagraph, 5},
		{NodeAlinea, 6},
	}
	for _, tc := range cases {
		if tc.nt.Depth() != tc.depth {
			t.Errorf("%s.Depth() = %d, want %d", tc.nt, tc.nt.Depth(), tc.depth)
		}
	}
}

func TestNodeArticleValidation(t *testing.T) {
	n := LawbookNode{
		NodeID:       "ART-1382",
		DocumentID:   "DOC-CIVIL-CODE",
		NodeType:     NodeArticle,
		CanonicalRef: "civil-code/livre-3/titre-4/art-1382",
		DisplayRef:   "Art. 1382",
		Depth:        4,
		OrdinalPath:  "1.3.4.1.1",
		SourcePath:   "sources/civil-code.pdf",
		SourceHash:   "sha256:2222222222222222222222222222222222222222222222222222222222222222",
		Status:       StatusAmended,
		Priority:     PriorityCritical,
		Domain:       "civil-liability",
		Title:        "Responsabilité du fait personnel",
		ParentID:     "SECTION-RESP",
	}
	errs := ValidateNode(n)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid article, got %v", errs)
	}
}

func TestNodeAlineaValidation(t *testing.T) {
	n := LawbookNode{
		NodeID:       "AL-1382-1",
		DocumentID:   "DOC-CIVIL-CODE",
		NodeType:     NodeAlinea,
		CanonicalRef: "civil-code/art-1382/al-1",
		DisplayRef:   "Art. 1382, al. 1",
		Depth:        6,
		OrdinalPath:  "1.3.4.1.1.1.1",
		SourcePath:   "sources/civil-code.pdf",
		SourceHash:   "sha256:3333333333333333333333333333333333333333333333333333333333333333",
		Status:       StatusActive,
		Priority:     PriorityLow,
		Domain:       "civil-liability",
		Text:         "Tout fait quelconque de l'homme...",
	}
	errs := ValidateNode(n)
	if len(errs) != 0 {
		t.Fatalf("expected no errors for valid alinea, got %v", errs)
	}
}

func assertContainsError(t *testing.T, errs []string, keyword string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e, keyword) {
			return
		}
	}
	t.Fatalf("expected error containing %q in %v", keyword, errs)
}
