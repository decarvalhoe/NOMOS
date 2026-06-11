package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return string(data)
}

func TestExtractMarkdown_FullFixture(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	if len(result.Nodes) == 0 {
		t.Fatal("expected nodes, got none")
	}

	// Find all heading-level nodes (not paragraph/alinea).
	var headingNodes []LawbookNode
	for _, n := range result.Nodes {
		if n.NodeType == NodeDocument || n.NodeType == NodeChapter || n.NodeType == NodeSection || n.NodeType == NodeArticle {
			headingNodes = append(headingNodes, n)
		}
	}

	// Expected: 1 document + 2 chapters + 2 sections + 3 articles = 8
	if len(headingNodes) != 8 {
		t.Fatalf("expected 8 heading nodes, got %d", len(headingNodes))
	}
}

func TestExtractMarkdown_DocumentNode(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	doc := result.Nodes[0]
	if doc.NodeType != NodeDocument {
		t.Fatalf("expected document node, got %s", doc.NodeType)
	}
	if doc.Depth != NodeDocument.Depth() {
		t.Fatalf("expected depth %d, got %d", NodeDocument.Depth(), doc.Depth)
	}
	if doc.Title != "Reglement General Assurance Auto" {
		t.Fatalf("unexpected title: %q", doc.Title)
	}
	if doc.ParentID != "" {
		t.Fatalf("document should have no parent, got %q", doc.ParentID)
	}
	if len(doc.ParentID) != 0 {
		t.Fatalf("document should have empty parent chain, got %v", doc.ParentID)
	}
}

func TestExtractMarkdown_Metadata(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	doc := result.Nodes[0]
	if doc.Metadata == nil {
		t.Fatal("expected metadata on document node")
	}
	if doc.Metadata["reference"] != "RGA-2026-001" {
		t.Fatalf("expected reference RGA-2026-001, got %q", doc.Metadata["reference"])
	}
	if doc.Metadata["statut"] != "En vigueur" {
		t.Fatalf("expected statut 'En vigueur', got %q", doc.Metadata["statut"])
	}
	if doc.Metadata["emetteur"] != "Direction Juridique" {
		t.Fatalf("expected emetteur 'Direction Juridique', got %q", doc.Metadata["emetteur"])
	}
}

func TestExtractMarkdown_ChapterParent(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	var chapter LawbookNode
	for _, n := range result.Nodes {
		if n.NodeType == NodeChapter && strings.Contains(n.Title, "Chapitre 1") {
			chapter = n
			break
		}
	}
	if chapter.NodeID == "" {
		t.Fatal("chapter 1 not found")
	}
	if chapter.Depth != NodeChapter.Depth() {
		t.Fatalf("expected depth %d, got %d", NodeChapter.Depth(), chapter.Depth)
	}
	if chapter.ParentID != result.Nodes[0].NodeID {
		t.Fatalf("chapter parent should be document, got %q", chapter.ParentID)
	}
	if chapter.ParentID == "" {
		t.Fatalf("expected non-empty parent ID, got %d", len(chapter.ParentID))
	}
}

func TestExtractMarkdown_SectionParent(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	var section LawbookNode
	var chapter1ID string
	for _, n := range result.Nodes {
		if n.NodeType == NodeChapter && strings.Contains(n.Title, "Chapitre 1") {
			chapter1ID = n.NodeID
		}
		if n.NodeType == NodeSection && strings.Contains(n.Title, "Section 1.1") {
			section = n
		}
	}
	if section.NodeID == "" {
		t.Fatal("section 1.1 not found")
	}
	if section.ParentID != chapter1ID {
		t.Fatalf("section parent should be chapter 1, got %q", section.ParentID)
	}
	if section.Depth != NodeSection.Depth() {
		t.Fatalf("expected depth %d, got %d", NodeSection.Depth(), section.Depth)
	}
}

func TestExtractMarkdown_ArticleParent(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	var article LawbookNode
	for _, n := range result.Nodes {
		if n.NodeType == NodeArticle && strings.Contains(n.Title, "Article 1") && !strings.Contains(n.Title, "Article 10") {
			article = n
			break
		}
	}
	if article.NodeID == "" {
		t.Fatal("article 1 not found")
	}
	if article.Depth != NodeArticle.Depth() {
		t.Fatalf("expected depth %d, got %d", NodeArticle.Depth(), article.Depth)
	}
	// Parent chain should be: document -> chapter -> section
	if article.ParentID == "" {
		t.Fatalf("expected non-empty parent ID, got %d: %v", len(article.ParentID), article.ParentID)
	}
}

func TestExtractMarkdown_AlineaNodes(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	var alineas []LawbookNode
	for _, n := range result.Nodes {
		if n.NodeType == NodeAlinea {
			alineas = append(alineas, n)
		}
	}
	// The fixture has 3 list items plus prose blocks. Prose blocks must also
	// become atomic alineas.
	if len(alineas) <= 3 {
		t.Fatalf("expected prose and list alineas, got %d", len(alineas))
	}
	var listAlinea LawbookNode
	for _, alinea := range alineas {
		if strings.Contains(alinea.Text, "Personne physique") {
			listAlinea = alinea
			break
		}
	}
	if listAlinea.NodeID == "" {
		t.Fatal("expected list item to become an alinea")
	}
	if listAlinea.Depth != NodeAlinea.Depth() {
		t.Fatalf("expected alinea depth %d, got %d", NodeAlinea.Depth(), listAlinea.Depth)
	}
}

func TestExtractMarkdown_ParagraphNodes(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	var paragraphs []LawbookNode
	for _, n := range result.Nodes {
		if n.NodeType == NodeParagraph {
			paragraphs = append(paragraphs, n)
		}
	}
	if len(paragraphs) == 0 {
		t.Fatal("expected paragraph nodes")
	}
	if paragraphs[0].Depth != NodeParagraph.Depth() {
		t.Fatalf("expected paragraph depth %d, got %d", NodeParagraph.Depth(), paragraphs[0].Depth)
	}
}

func TestExtractMarkdown_ProseParagraphBecomesAtomicAlinea(t *testing.T) {
	src := "# Doc\n\nFirst governed statement.\n\nSecond governed statement."
	result := ExtractMarkdown(src, "test")

	var paragraphs []LawbookNode
	var alineas []LawbookNode
	for _, n := range result.Nodes {
		switch n.NodeType {
		case NodeParagraph:
			paragraphs = append(paragraphs, n)
		case NodeAlinea:
			alineas = append(alineas, n)
		}
	}
	if len(paragraphs) != 2 {
		t.Fatalf("expected 2 paragraph containers, got %d", len(paragraphs))
	}
	if len(alineas) != 2 {
		t.Fatalf("expected 2 atomic alineas, got %d", len(alineas))
	}
	for i := range paragraphs {
		if alineas[i].ParentID != paragraphs[i].NodeID {
			t.Fatalf("alinea %d parent = %q, want paragraph %q", i, alineas[i].ParentID, paragraphs[i].NodeID)
		}
		if alineas[i].Text != paragraphs[i].Text {
			t.Fatalf("alinea %d text = %q, want %q", i, alineas[i].Text, paragraphs[i].Text)
		}
	}
}

func TestExtractMarkdown_ListBlockHasParagraphContainerAndAlineaItems(t *testing.T) {
	src := "# Doc\n\n- First item\n- Second item\n1. Numbered item"
	result := ExtractMarkdown(src, "test")

	var paragraphs []LawbookNode
	var alineas []LawbookNode
	for _, n := range result.Nodes {
		switch n.NodeType {
		case NodeParagraph:
			paragraphs = append(paragraphs, n)
		case NodeAlinea:
			alineas = append(alineas, n)
		}
	}
	if len(paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph container, got %d", len(paragraphs))
	}
	if len(alineas) != 3 {
		t.Fatalf("expected 3 list alineas, got %d", len(alineas))
	}
	for i, alinea := range alineas {
		if alinea.ParentID != paragraphs[0].NodeID {
			t.Fatalf("alinea %d parent = %q, want %q", i, alinea.ParentID, paragraphs[0].NodeID)
		}
	}
	if alineas[2].Text != "Numbered item" {
		t.Fatalf("expected numbered list text to be stripped, got %q", alineas[2].Text)
	}
}

func TestExtractMarkdown_StableIDs(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	r1 := ExtractMarkdown(src, "rga-2026")
	r2 := ExtractMarkdown(src, "rga-2026")

	if len(r1.Nodes) != len(r2.Nodes) {
		t.Fatal("different node counts across runs")
	}
	for i := range r1.Nodes {
		if r1.Nodes[i].NodeID != r2.Nodes[i].NodeID {
			t.Fatalf("node[%d] ID changed: %q vs %q", i, r1.Nodes[i].NodeID, r2.Nodes[i].NodeID)
		}
	}
}

func TestExtractMarkdown_UniqueIDs(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	seen := map[string]bool{}
	for _, n := range result.Nodes {
		if seen[n.NodeID] {
			t.Fatalf("duplicate NodeID: %s (type=%s, title=%q)", n.NodeID, n.NodeType, n.Title)
		}
		seen[n.NodeID] = true
	}
}

func TestExtractMarkdown_CanonicalRef(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	doc := result.Nodes[0]
	if !strings.HasPrefix(doc.CanonicalRef, "rga-2026/document/") {
		t.Fatalf("expected canonical ref prefix 'rga-2026/document/', got %q", doc.CanonicalRef)
	}
}

func TestExtractMarkdown_DisplayRef(t *testing.T) {
	src := loadFixture(t, "lawbook-sample.md")
	result := ExtractMarkdown(src, "rga-2026")

	doc := result.Nodes[0]
	if !strings.HasPrefix(doc.DisplayRef, "document: ") {
		t.Fatalf("expected display ref prefix 'document: ', got %q", doc.DisplayRef)
	}
}

func TestExtractMarkdown_Empty(t *testing.T) {
	result := ExtractMarkdown("", "test")
	if len(result.Nodes) != 0 {
		t.Fatalf("expected 0 nodes for empty input, got %d", len(result.Nodes))
	}
}

func TestExtractMarkdown_NoHeaders(t *testing.T) {
	result := ExtractMarkdown("Just some text\nwithout any headers.", "test")
	if len(result.Nodes) != 0 {
		t.Fatalf("expected 0 nodes for text without headers, got %d", len(result.Nodes))
	}
}

func TestExtractMarkdown_H1Only(t *testing.T) {
	result := ExtractMarkdown("# Title\n\nSome body text.", "test")
	if len(result.Nodes) < 1 {
		t.Fatal("expected at least 1 node")
	}
	if result.Nodes[0].NodeType != NodeDocument {
		t.Fatalf("expected document node, got %s", result.Nodes[0].NodeType)
	}
}

func TestExtractMarkdown_MetadataKeyNormalization(t *testing.T) {
	src := "# Doc\n\n| Champ | Valeur |\n|---|---|\n| Référence | REF-001 |\n| Status | Draft |\n| Issuer | Team A |\n"
	result := ExtractMarkdown(src, "test")
	doc := result.Nodes[0]
	if doc.Metadata["reference"] != "REF-001" {
		t.Fatalf("expected reference REF-001, got %q", doc.Metadata["reference"])
	}
	if doc.Metadata["statut"] != "Draft" {
		t.Fatalf("expected statut Draft, got %q", doc.Metadata["statut"])
	}
	if doc.Metadata["emetteur"] != "Team A" {
		t.Fatalf("expected emetteur Team A, got %q", doc.Metadata["emetteur"])
	}
}

func TestExtractMarkdown_NoMetadataTable(t *testing.T) {
	src := "# Doc\n\nJust text, no table.\n"
	result := ExtractMarkdown(src, "test")
	doc := result.Nodes[0]
	if doc.Metadata != nil {
		t.Fatalf("expected nil metadata, got %v", doc.Metadata)
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"Article 1 - Assuré", "article-1-assur-"},
		{"Chapitre 2", "chapitre-2"},
		{"Simple", "simple"},
		{"UPPER CASE", "upper-case"},
		{"a--b", "a-b"},
		{"", ""},
	}
	for _, tc := range cases {
		got := slugify(tc.input)
		// Trim trailing dash for comparison since slugify trims.
		got = strings.TrimRight(got, "-")
		want := strings.TrimRight(tc.want, "-")
		if got != want {
			t.Errorf("slugify(%q) = %q, want %q", tc.input, got, want)
		}
	}
}

func TestComputeNodeID_Deterministic(t *testing.T) {
	id1 := computeNodeID("test/doc/title")
	id2 := computeNodeID("test/doc/title")
	if id1 != id2 {
		t.Fatalf("expected deterministic ID, got %q and %q", id1, id2)
	}
	if !strings.HasPrefix(id1, "N-") {
		t.Fatalf("expected N- prefix, got %q", id1)
	}
}

func TestComputeNodeID_Unique(t *testing.T) {
	id1 := computeNodeID("test/doc/a")
	id2 := computeNodeID("test/doc/b")
	if id1 == id2 {
		t.Fatal("expected different IDs for different refs")
	}
}

// --- RBOK realistic fixture tests ---

func TestRBOK_FullHierarchy(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	counts := map[LawbookNodeType]int{}
	for _, n := range result.Nodes {
		counts[n.NodeType]++
	}

	// 1 document, 3 chapters, 5 sections, 10 articles
	if counts[NodeDocument] != 1 {
		t.Fatalf("expected 1 document, got %d", counts[NodeDocument])
	}
	if counts[NodeChapter] != 3 {
		t.Fatalf("expected 3 chapters, got %d", counts[NodeChapter])
	}
	if counts[NodeSection] != 6 {
		t.Fatalf("expected 6 sections, got %d", counts[NodeSection])
	}
	if counts[NodeArticle] != 10 {
		t.Fatalf("expected 10 articles, got %d", counts[NodeArticle])
	}
	if counts[NodeParagraph] == 0 {
		t.Fatal("expected paragraph nodes")
	}
	if counts[NodeAlinea] == 0 {
		t.Fatal("expected alinea nodes")
	}
}

func TestRBOK_MetadataComplete(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	doc := result.Nodes[0]
	if doc.Metadata == nil {
		t.Fatal("expected metadata on document node")
	}
	if doc.Metadata["reference"] != "RBOK-PS-2026-003" {
		t.Fatalf("expected reference RBOK-PS-2026-003, got %v", doc.Metadata["reference"])
	}
	if doc.Metadata["statut"] != "En vigueur" {
		t.Fatalf("expected statut 'En vigueur', got %v", doc.Metadata["statut"])
	}
	if doc.Metadata["emetteur"] != "Direction Metier Sante" {
		t.Fatalf("expected emetteur 'Direction Metier Sante', got %v", doc.Metadata["emetteur"])
	}
	if doc.Metadata["derniere_revision"] != "2026-03-15" {
		t.Fatalf("expected derniere_revision 2026-03-15, got %v", doc.Metadata["derniere_revision"])
	}
	if doc.Metadata["version"] != "3.2" {
		t.Fatalf("expected version 3.2, got %v", doc.Metadata["version"])
	}
	if doc.Metadata["domaine"] != "prestations-sante" {
		t.Fatalf("expected domaine prestations-sante, got %v", doc.Metadata["domaine"])
	}
}

func TestRBOK_ParentChainIntegrity(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	nodeByID := map[string]LawbookNode{}
	for _, n := range result.Nodes {
		nodeByID[n.NodeID] = n
	}

	// Every non-document heading node must have a parent.
	for _, n := range result.Nodes {
		if n.NodeType == NodeDocument || n.NodeType == NodeParagraph || n.NodeType == NodeAlinea {
			continue
		}
		if n.ParentID == "" {
			t.Fatalf("heading node %q (%s) has no parent", n.Title, n.NodeType)
		}
		parent, ok := nodeByID[n.ParentID]
		if !ok {
			t.Fatalf("node %q parent %q not found", n.Title, n.ParentID)
		}
		if parent.Depth >= n.Depth {
			t.Fatalf("node %q (depth %d) has parent %q (depth %d) — parent should be shallower",
				n.Title, n.Depth, parent.Title, parent.Depth)
		}
	}
}

func TestRBOK_CanonicalRefsAreConsistent(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	for _, n := range result.Nodes {
		if n.CanonicalRef == "" {
			t.Fatalf("node %q has empty canonical_ref", n.Title)
		}
		if !strings.HasPrefix(n.CanonicalRef, "rbok-ps-2026/") {
			t.Fatalf("canonical_ref %q should start with doc slug", n.CanonicalRef)
		}
		if !strings.Contains(n.CanonicalRef, string(n.NodeType)) {
			t.Fatalf("canonical_ref %q should contain node type %s", n.CanonicalRef, n.NodeType)
		}
	}
}

func TestRBOK_NumberedListBecomesAlineas(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	// Article 6 has a numbered list with 3 items.
	var art6Alineas []LawbookNode
	var inArt6 bool
	for _, n := range result.Nodes {
		if n.NodeType == NodeArticle && strings.Contains(n.Title, "Article 6") {
			inArt6 = true
			continue
		}
		if inArt6 && n.NodeType == NodeAlinea {
			art6Alineas = append(art6Alineas, n)
		}
		if inArt6 && (n.NodeType == NodeArticle || n.NodeType == NodeSection || n.NodeType == NodeChapter) {
			break
		}
	}
	if len(art6Alineas) < 3 {
		t.Fatalf("expected at least 3 alineas under Article 6, got %d", len(art6Alineas))
	}
}

func TestRBOK_DepthsMatchNodeTypes(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	for _, n := range result.Nodes {
		expected := n.NodeType.Depth()
		if n.Depth != expected {
			t.Fatalf("node %q type=%s depth=%d expected=%d", n.Title, n.NodeType, n.Depth, expected)
		}
	}
}

func TestRBOK_AllIDsStableAcrossRuns(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	r1 := ExtractMarkdown(src, "rbok-ps-2026")
	r2 := ExtractMarkdown(src, "rbok-ps-2026")

	if len(r1.Nodes) != len(r2.Nodes) {
		t.Fatalf("node count changed: %d vs %d", len(r1.Nodes), len(r2.Nodes))
	}
	for i := range r1.Nodes {
		if r1.Nodes[i].NodeID != r2.Nodes[i].NodeID {
			t.Fatalf("node[%d] ID unstable: %q vs %q", i, r1.Nodes[i].NodeID, r2.Nodes[i].NodeID)
		}
	}
}

func TestRBOK_AllIDsUnique(t *testing.T) {
	src := loadFixture(t, "rbok-reglement-complet.md")
	result := ExtractMarkdown(src, "rbok-ps-2026")

	seen := map[string]string{}
	for _, n := range result.Nodes {
		if prev, dup := seen[n.NodeID]; dup {
			t.Fatalf("duplicate ID %s: %q and %q", n.NodeID, prev, n.Title)
		}
		seen[n.NodeID] = n.Title
	}
}
