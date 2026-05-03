package atomization

import (
	"strings"
	"testing"
)

const lawbookFixture = `# Code des Assurances

## Chapitre L.113 — Obligations

### Article L.113-2

L'assure est oblige de repondre exactement aux questions posees par l'assureur.

### Article L.113-3

L'assure est oblige de payer la prime aux epoques convenues.

## Chapitre L.114 — Prescription

### Article L.114-1

Toutes actions derivant d'un contrat d'assurance sont prescrites par deux ans.
`

func TestAtomizeSourceBasic(t *testing.T) {
	config := LawbookEngineConfig{
		DocumentID: "FR-CODE-ASSURANCES",
		SourceFile: "corpus/fr/code-assurances.md",
		Domain:     "assurance",
	}
	result := AtomizeSource(lawbookFixture, config)

	if result.DocumentID != "FR-CODE-ASSURANCES" {
		t.Fatalf("expected doc ID, got %s", result.DocumentID)
	}
	if result.SourceFile != "corpus/fr/code-assurances.md" {
		t.Fatalf("expected source file, got %s", result.SourceFile)
	}
	if result.TotalAtoms == 0 {
		t.Fatal("expected atoms")
	}
}

func TestAtomizeSourceProducesAtomIDs(t *testing.T) {
	config := LawbookEngineConfig{DocumentID: "DOC", Domain: "test"}
	result := AtomizeSource(lawbookFixture, config)

	for _, atom := range result.Atoms {
		if !strings.HasPrefix(atom.ID, "DOC.") {
			t.Fatalf("expected DOC. prefix, got %s", atom.ID)
		}
	}
}

func TestAtomizeSourceContentHashes(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource(lawbookFixture, config)

	for _, atom := range result.Atoms {
		if atom.ContentHash == "" {
			t.Fatalf("atom %s has empty content hash", atom.ID)
		}
		if !strings.HasPrefix(atom.ContentHash, "sha256:") {
			t.Fatalf("expected sha256: prefix, got %s", atom.ContentHash)
		}
	}
}

func TestAtomizeSourceSourceSpans(t *testing.T) {
	config := LawbookEngineConfig{
		DocumentID: "DOC",
		SourceFile: "test.md",
		Domain:     "test",
	}
	result := AtomizeSource(lawbookFixture, config)

	for _, atom := range result.Atoms {
		if atom.SourceSpan.File != "test.md" {
			t.Fatalf("expected source file test.md, got %s", atom.SourceSpan.File)
		}
	}
}

func TestAtomizeSourceParentChains(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource(lawbookFixture, config)

	hasParent := false
	for _, atom := range result.Atoms {
		if atom.Type == AtomClause && atom.Depth > 1 && atom.ParentID != "" {
			hasParent = true
			break
		}
	}
	if !hasParent {
		t.Fatal("expected at least one heading atom with parent chain")
	}
}

func TestAtomizeSourceParentMap(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource(lawbookFixture, config)

	if len(result.ParentMap) == 0 {
		t.Fatal("expected non-empty parent map")
	}

	hasChildren := false
	for _, children := range result.ParentMap {
		if len(children) > 0 {
			hasChildren = true
			break
		}
	}
	if !hasChildren {
		t.Fatal("expected parent with children in parent map")
	}
}

func TestAtomizeSourceCanonicalRefs(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource(lawbookFixture, config)

	clauseAtoms := 0
	for _, atom := range result.Atoms {
		if atom.Type == AtomClause {
			clauseAtoms++
			if atom.CanonicalRef == "" {
				t.Fatalf("clause atom %s has empty canonical_ref", atom.ID)
			}
			if strings.Contains(atom.CanonicalRef, " ") {
				t.Fatalf("canonical_ref should not contain spaces: %s", atom.CanonicalRef)
			}
		}
	}
	if clauseAtoms == 0 {
		t.Fatal("expected clause atoms from headings")
	}
}

func TestAtomizeSourceDomain(t *testing.T) {
	config := LawbookEngineConfig{DocumentID: "D", Domain: "insurance"}
	result := AtomizeSource("# Title\n\nBody.\n", config)

	for _, atom := range result.Atoms {
		if atom.Domain != "insurance" {
			t.Fatalf("expected domain insurance, got %s", atom.Domain)
		}
	}
}

func TestAtomizeSourceDepthFilter(t *testing.T) {
	config := LawbookEngineConfig{
		DocumentID: "DOC",
		Domain:     "test",
		MaxDepth:   2,
	}
	result := AtomizeSource(lawbookFixture, config)

	for _, atom := range result.Atoms {
		if atom.Type == AtomClause && atom.Depth > 2 {
			t.Fatalf("atom %s at depth %d should be filtered (max 2)", atom.ID, atom.Depth)
		}
	}
}

func TestAtomizeSourceDeterministic(t *testing.T) {
	config := DefaultEngineConfig()
	r1 := AtomizeSource(lawbookFixture, config)
	r2 := AtomizeSource(lawbookFixture, config)

	if r1.TotalAtoms != r2.TotalAtoms {
		t.Fatal("non-deterministic atom count")
	}
	for i := range r1.Atoms {
		if r1.Atoms[i].ID != r2.Atoms[i].ID {
			t.Fatalf("non-deterministic ID at index %d", i)
		}
		if r1.Atoms[i].ContentHash != r2.Atoms[i].ContentHash {
			t.Fatalf("non-deterministic hash at index %d", i)
		}
	}
}

func TestAtomizeSourceEmptyDocument(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource("", config)

	if result.TotalAtoms == 0 {
		t.Fatal("expected at least root atom for empty doc")
	}
}

func TestAtomizeSourceDisplayRef(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource("# Important Title\n\nBody.\n", config)

	found := false
	for _, atom := range result.Atoms {
		if atom.Title == "Important Title" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Title field for heading atom")
	}
}

func TestAtomizeLawbookFromAST(t *testing.T) {
	ast := ParseMarkdown(lawbookFixture)
	config := LawbookEngineConfig{DocumentID: "TEST", SourceFile: "f.md", Domain: "d"}
	result := AtomizeLawbook(ast, config)

	if result.SourceHash != ast.SourceHash {
		t.Fatalf("expected source hash propagated, got %s", result.SourceHash)
	}
	if result.TotalAtoms == 0 {
		t.Fatal("expected atoms from AST")
	}
}

func TestAtomizeSourceReviewState(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource("# Title\n\nContent.\n", config)

	for _, atom := range result.Atoms {
		if atom.ReviewState != ReviewDraft {
			t.Fatalf("expected draft review state, got %s", atom.ReviewState)
		}
	}
}

func TestAtomizeSourceBlockIDLinked(t *testing.T) {
	config := DefaultEngineConfig()
	result := AtomizeSource("# Title\n\nParagraph.\n", config)

	for _, atom := range result.Atoms {
		if atom.BlockID == "" {
			t.Fatalf("atom %s has empty block_id", atom.ID)
		}
	}
}
