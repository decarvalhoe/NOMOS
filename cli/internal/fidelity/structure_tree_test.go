package fidelity

import (
	"strings"
	"testing"
)

func sampleHeadings() []HeadingInput {
	return []HeadingInput{
		{ID: "H1", Title: "Introduction", Level: 1, Hash: "sha256:h1"},
		{ID: "H2", Title: "Architecture", Level: 2, Hash: "sha256:h2"},
		{ID: "H3", Title: "Data Layer", Level: 3, Hash: "sha256:h3"},
		{ID: "H4", Title: "API Layer", Level: 3, Hash: "sha256:h4", CrossRefs: []string{"H3"}},
		{ID: "H5", Title: "Deployment", Level: 2, Hash: "sha256:h5"},
		{ID: "H6", Title: "Appendix", Level: 1, Hash: "sha256:h6"},
	}
}

func TestBuildStructureTreeBasic(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC", SourceHash: "sha256:src"})

	if tree.DocumentID != "DOC" {
		t.Fatalf("expected DOC, got %s", tree.DocumentID)
	}
	if tree.SourceHash != "sha256:src" {
		t.Fatalf("expected source hash, got %s", tree.SourceHash)
	}
	if tree.TotalNodes != 7 { // root + 6 headings
		t.Fatalf("expected 7 total nodes, got %d", tree.TotalNodes)
	}
	if tree.MaxDepth != 3 {
		t.Fatalf("expected max depth 3, got %d", tree.MaxDepth)
	}
}

func TestBuildStructureTreeRootChildren(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	// Root should have 2 children: Introduction (H1) and Appendix (H1).
	if len(tree.Root.Children) != 2 {
		t.Fatalf("expected 2 root children, got %d", len(tree.Root.Children))
	}
	if tree.Root.Children[0].Title != "Introduction" {
		t.Fatalf("expected Introduction first, got %s", tree.Root.Children[0].Title)
	}
	if tree.Root.Children[1].Title != "Appendix" {
		t.Fatalf("expected Appendix second, got %s", tree.Root.Children[1].Title)
	}
}

func TestBuildStructureTreeNesting(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	intro := tree.Root.Children[0]
	// Introduction should have 2 children: Architecture, Deployment.
	if len(intro.Children) != 2 {
		t.Fatalf("expected 2 children under Introduction, got %d", len(intro.Children))
	}
	arch := intro.Children[0]
	if arch.Title != "Architecture" {
		t.Fatalf("expected Architecture, got %s", arch.Title)
	}
	// Architecture should have 2 children: Data Layer, API Layer.
	if len(arch.Children) != 2 {
		t.Fatalf("expected 2 children under Architecture, got %d", len(arch.Children))
	}
}

func TestBuildStructureTreeNumbering(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	flat := tree.FlatTOC()
	expected := map[string]string{
		"H1": "1",
		"H2": "1.1",
		"H3": "1.1.1",
		"H4": "1.1.2",
		"H5": "1.2",
		"H6": "2",
	}
	for _, node := range flat {
		if exp, ok := expected[node.ID]; ok {
			if node.Number != exp {
				t.Fatalf("node %s expected number %s, got %s", node.ID, exp, node.Number)
			}
		}
	}
}

func TestBuildStructureTreeHash(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	if tree.TreeHash == "" {
		t.Fatal("expected non-empty tree hash")
	}
	if !strings.HasPrefix(tree.TreeHash, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %s", tree.TreeHash)
	}
}

func TestBuildStructureTreeVerify(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	if !tree.Verify() {
		t.Fatal("tree should verify against its own hash")
	}
}

func TestBuildStructureTreeVerifyDetectsTamper(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})
	tree.TreeHash = "sha256:tampered"

	if tree.Verify() {
		t.Fatal("tampered tree should not verify")
	}
}

func TestBuildStructureTreeDeterministic(t *testing.T) {
	t1 := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})
	t2 := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	if t1.TreeHash != t2.TreeHash {
		t.Fatal("tree hash should be deterministic")
	}
}

func TestBuildStructureTreeFlatTOC(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})
	flat := tree.FlatTOC()

	if len(flat) != 7 { // root + 6
		t.Fatalf("expected 7 flat entries, got %d", len(flat))
	}
	// First should be root.
	if flat[0].Title != "Document Root" {
		t.Fatalf("expected root first, got %s", flat[0].Title)
	}
}

func TestBuildStructureTreeFindByID(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	node := tree.FindByID("H4")
	if node == nil {
		t.Fatal("expected to find H4")
	}
	if node.Title != "API Layer" {
		t.Fatalf("expected API Layer, got %s", node.Title)
	}
}

func TestBuildStructureTreeFindByIDNotFound(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	node := tree.FindByID("NONEXISTENT")
	if node != nil {
		t.Fatal("expected nil for nonexistent ID")
	}
}

func TestBuildStructureTreeCrossRefIndex(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})
	index := tree.CrossRefIndex()

	// H4 references H3.
	sources, ok := index["H3"]
	if !ok {
		t.Fatal("expected H3 in cross-ref index")
	}
	if len(sources) != 1 || sources[0] != "H4" {
		t.Fatalf("expected H4 referencing H3, got %v", sources)
	}
}

func TestBuildStructureTreeEmptyHeadings(t *testing.T) {
	tree := BuildStructureTree(nil, TreeConfig{DocumentID: "DOC"})

	if tree.TotalNodes != 1 {
		t.Fatalf("expected 1 (root only), got %d", tree.TotalNodes)
	}
	if tree.MaxDepth != 0 {
		t.Fatalf("expected max depth 0, got %d", tree.MaxDepth)
	}
	if tree.TreeHash == "" {
		t.Fatal("expected tree hash even for empty tree")
	}
}

func TestBuildStructureTreeParentIDs(t *testing.T) {
	tree := BuildStructureTree(sampleHeadings(), TreeConfig{DocumentID: "DOC"})

	node := tree.FindByID("H3")
	if node == nil {
		t.Fatal("expected H3")
	}
	if node.ParentID != "H2" {
		t.Fatalf("expected H3 parent to be H2, got %s", node.ParentID)
	}
}

func TestBuildStructureTreeSkippedLevels(t *testing.T) {
	// Jump from H1 directly to H3 (skipping H2).
	headings := []HeadingInput{
		{ID: "A", Title: "Top", Level: 1, Hash: "sha256:a"},
		{ID: "B", Title: "Deep", Level: 3, Hash: "sha256:b"},
	}
	tree := BuildStructureTree(headings, TreeConfig{DocumentID: "DOC"})

	// B should still be nested under A (nearest parent).
	if tree.TotalNodes != 3 {
		t.Fatalf("expected 3 nodes, got %d", tree.TotalNodes)
	}
	node := tree.FindByID("B")
	if node == nil {
		t.Fatal("expected to find B")
	}
	if node.ParentID != "A" {
		t.Fatalf("expected B parent A (nearest), got %s", node.ParentID)
	}
}
