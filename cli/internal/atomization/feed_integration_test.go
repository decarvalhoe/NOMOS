package atomization

import (
	"testing"
)

func approvedAtom(id, title string, atomType AtomType) Atom {
	return Atom{
		ID:           id,
		CanonicalRef: "doc/" + id,
		Type:         atomType,
		Title:        title,
		Text:         "Content of " + title,
		ContentHash:  "sha256:abc123",
		SourceSpan:   SourceSpan{File: "test.md", StartLine: 1, EndLine: 5},
		BlockID:      "block-1",
		Depth:        2,
		ReviewState:  ReviewApproved,
		Domain:       "insurance",
	}
}

func draftAtom(id, title string) Atom {
	a := approvedAtom(id, title, AtomRule)
	a.ReviewState = ReviewDraft
	return a
}

func testConfig() FeedIntegrationConfig {
	return FeedIntegrationConfig{
		Domain:     "insurance",
		Owner:      "team",
		SourcePath: "corpus/rules.md",
		SourceHash: "sha256:feedhash",
		Version:    "1.0",
	}
}

// --- IsCertified ---

func TestIsCertified_Approved(t *testing.T) {
	atom := approvedAtom("A1", "Rule 1", AtomRule)
	if !IsCertified(atom, ReviewApproved) {
		t.Fatal("approved atom should be certified")
	}
}

func TestIsCertified_Draft(t *testing.T) {
	atom := draftAtom("A2", "Rule 2")
	if IsCertified(atom, ReviewApproved) {
		t.Fatal("draft atom should not be certified when min=approved")
	}
}

func TestIsCertified_PendingWithMinPending(t *testing.T) {
	atom := approvedAtom("A3", "Rule 3", AtomRule)
	atom.ReviewState = ReviewPending
	if !IsCertified(atom, ReviewPending) {
		t.Fatal("pending atom should be certified when min=pending")
	}
}

func TestIsCertified_Rejected(t *testing.T) {
	atom := approvedAtom("A4", "Rule 4", AtomRule)
	atom.ReviewState = ReviewRejected
	if IsCertified(atom, ReviewDraft) {
		t.Fatal("rejected atom should never be certified")
	}
}

// --- ProjectAtomsToLawbookFeed ---

func TestProjectLawbookFeed_OnlyApproved(t *testing.T) {
	atoms := []Atom{
		approvedAtom("A1", "Approved rule", AtomRule),
		draftAtom("A2", "Draft rule"),
		approvedAtom("A3", "Another approved", AtomClause),
	}

	result := ProjectAtomsToLawbookFeed(atoms, testConfig())

	if result.TotalAtoms != 3 {
		t.Fatalf("expected 3 total, got %d", result.TotalAtoms)
	}
	if result.CertifiedAtoms != 2 {
		t.Fatalf("expected 2 certified, got %d", result.CertifiedAtoms)
	}
	if result.RejectedAtoms != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.RejectedAtoms)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
}

func TestProjectLawbookFeed_EntryFields(t *testing.T) {
	atoms := []Atom{approvedAtom("A1", "Test rule", AtomRule)}
	result := ProjectAtomsToLawbookFeed(atoms, testConfig())

	entry := result.Entries[0]
	if entry.AtomID != "A1" {
		t.Fatalf("expected atom_id A1, got %q", entry.AtomID)
	}
	if entry.NodeType != "article" {
		t.Fatalf("expected node_type article, got %q", entry.NodeType)
	}
	if entry.Status != "active" {
		t.Fatalf("expected status active, got %q", entry.Status)
	}
	if entry.Priority != "high" {
		t.Fatalf("expected priority high for rule, got %q", entry.Priority)
	}
	if entry.Domain != "insurance" {
		t.Fatalf("expected domain insurance, got %q", entry.Domain)
	}
	if entry.ReviewState != "approved" {
		t.Fatalf("expected review_state approved, got %q", entry.ReviewState)
	}
	if entry.Text == "" {
		t.Fatal("expected non-empty text")
	}
	if entry.NodeID == "" {
		t.Fatal("expected non-empty node_id")
	}
}

func TestProjectLawbookFeed_FeedMetadata(t *testing.T) {
	result := ProjectAtomsToLawbookFeed(nil, testConfig())

	if result.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema 0.1.0, got %q", result.SchemaVersion)
	}
	if result.Domain != "insurance" {
		t.Fatalf("expected domain insurance, got %q", result.Domain)
	}
	if result.FeedID == "" {
		t.Fatal("expected non-empty feed_id")
	}
	if result.GeneratedAt == "" {
		t.Fatal("expected non-empty generated_at")
	}
}

func TestProjectLawbookFeed_EmptyAtoms(t *testing.T) {
	result := ProjectAtomsToLawbookFeed(nil, testConfig())
	if result.TotalAtoms != 0 {
		t.Fatalf("expected 0 total, got %d", result.TotalAtoms)
	}
	if len(result.Entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result.Entries))
	}
}

func TestProjectLawbookFeed_AllRejected(t *testing.T) {
	atoms := []Atom{draftAtom("A1", "Draft"), draftAtom("A2", "Draft2")}
	result := ProjectAtomsToLawbookFeed(atoms, testConfig())

	if result.CertifiedAtoms != 0 {
		t.Fatalf("expected 0 certified, got %d", result.CertifiedAtoms)
	}
	if result.RejectedAtoms != 2 {
		t.Fatalf("expected 2 rejected, got %d", result.RejectedAtoms)
	}
}

func TestProjectLawbookFeed_CustomMinState(t *testing.T) {
	atoms := []Atom{
		draftAtom("A1", "Draft rule"),
		approvedAtom("A2", "Approved rule", AtomRule),
	}
	config := testConfig()
	config.MinReviewState = ReviewDraft

	result := ProjectAtomsToLawbookFeed(atoms, config)
	if result.CertifiedAtoms != 2 {
		t.Fatalf("expected 2 certified with min=draft, got %d", result.CertifiedAtoms)
	}
}

func TestProjectLawbookFeed_DomainFallback(t *testing.T) {
	atom := approvedAtom("A1", "Rule", AtomRule)
	atom.Domain = ""
	result := ProjectAtomsToLawbookFeed([]Atom{atom}, testConfig())

	if result.Entries[0].Domain != "insurance" {
		t.Fatalf("expected config domain fallback, got %q", result.Entries[0].Domain)
	}
}

// --- ProjectAtomsToEngineImport ---

func TestProjectEngineImport_OnlyApproved(t *testing.T) {
	atoms := []Atom{
		approvedAtom("A1", "Rule", AtomRule),
		draftAtom("A2", "Draft"),
	}

	result := ProjectAtomsToEngineImport(atoms, testConfig())
	if result.CertifiedAtoms != 1 {
		t.Fatalf("expected 1 certified, got %d", result.CertifiedAtoms)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result.Nodes))
	}
}

func TestProjectEngineImport_NodeFields(t *testing.T) {
	atoms := []Atom{approvedAtom("A1", "Test rule", AtomDefinition)}
	result := ProjectAtomsToEngineImport(atoms, testConfig())

	node := result.Nodes[0]
	if node.ExternalID != "A1" {
		t.Fatalf("expected external_id A1, got %q", node.ExternalID)
	}
	if node.NodeType != "definition" {
		t.Fatalf("expected node_type definition, got %q", node.NodeType)
	}
	if node.Content == "" {
		t.Fatal("expected non-empty content")
	}
	if node.SourcePath != "corpus/rules.md" {
		t.Fatalf("expected source_path from config, got %q", node.SourcePath)
	}
}

func TestProjectEngineImport_ContractVersion(t *testing.T) {
	result := ProjectAtomsToEngineImport(nil, testConfig())
	if result.ContractVersion != EngineProfileVersion {
		t.Fatalf("expected %s, got %s", EngineProfileVersion, result.ContractVersion)
	}
}

func TestProjectEngineImport_EmptyAtoms(t *testing.T) {
	result := ProjectAtomsToEngineImport(nil, testConfig())
	if len(result.Nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(result.Nodes))
	}
}

// --- mapAtomTypeToNodeType ---

func TestMapAtomTypeToNodeType(t *testing.T) {
	cases := []struct {
		atomType AtomType
		expected string
	}{
		{AtomRule, "article"},
		{AtomClause, "article"},
		{AtomDefinition, "definition"},
		{AtomListItem, "article"},
		{AtomTable, "annex"},
		{AtomCodeBlock, "annex"},
		{AtomMeta, "definition"},
	}
	for _, tc := range cases {
		got := mapAtomTypeToNodeType(tc.atomType)
		if got != tc.expected {
			t.Errorf("mapAtomTypeToNodeType(%s) = %q, want %q", tc.atomType, got, tc.expected)
		}
	}
}

// --- reviewRank ---

func TestReviewRank_Order(t *testing.T) {
	if reviewRank(ReviewRejected) >= reviewRank(ReviewDraft) {
		t.Fatal("rejected should rank below draft")
	}
	if reviewRank(ReviewDraft) >= reviewRank(ReviewPending) {
		t.Fatal("draft should rank below pending")
	}
	if reviewRank(ReviewPending) >= reviewRank(ReviewAmended) {
		t.Fatal("pending should rank below amended")
	}
	if reviewRank(ReviewAmended) >= reviewRank(ReviewApproved) {
		t.Fatal("amended should rank below approved")
	}
}
