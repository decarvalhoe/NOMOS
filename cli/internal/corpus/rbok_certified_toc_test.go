package corpus

import (
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

func TestBuildCertifiedTOCFromFeedsUsesDynamicHeadingDepth(t *testing.T) {
	feed := LawbookFeed{Nodes: []LawbookNode{
		{NodeID: "DOC-1", NodeType: NodeDocument, Title: "Doc", Span: LawbookSourceSpan{StartLine: 1}},
		{NodeID: "CH-1", NodeType: NodeChapter, Title: "Chapter", Span: LawbookSourceSpan{StartLine: 2}},
		{NodeID: "SEC-1", NodeType: NodeSection, Title: "Section", Span: LawbookSourceSpan{StartLine: 3}},
		{NodeID: "SUB-1", NodeType: NodeSubsection, Title: "Subsection", Span: LawbookSourceSpan{StartLine: 4}},
		{NodeID: "ART-1", NodeType: NodeArticle, Title: "Article", Span: LawbookSourceSpan{StartLine: 5}},
	}}

	toc := BuildCertifiedTOCFromFeeds([]LawbookFeed{feed}, "DOC-RBOK")
	if toc.EntryCount != 5 {
		t.Fatalf("expected 5 heading entries, got %d", toc.EntryCount)
	}
	if toc.MaxDepth != 4 {
		t.Fatalf("expected max depth 4, got %d", toc.MaxDepth)
	}
	if toc.StructureHash == "" {
		t.Fatal("expected structure hash")
	}
	strictTOC := fidelity.TOCArtifactFromCertified(toc)
	if !strictTOC.Certified || !fidelity.VerifyTOCArtifact(strictTOC) {
		t.Fatal("expected certified TOC to convert into a valid strict gate TOC artifact")
	}
}
