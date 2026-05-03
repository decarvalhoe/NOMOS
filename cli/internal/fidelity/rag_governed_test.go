package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testAtom(id, text, hash, reviewState string) PortableAtom {
	return PortableAtom{
		ID: id, CanonicalRef: "doc#" + id, Type: "rule",
		Text: text, ContentHash: hash, Depth: 1,
		Domain: "insurance", Profile: "rbok-lawbook", SourceLine: 10,
	}
}

func testGov(review, owner, conf string) GovernanceTag {
	return GovernanceTag{
		ReviewState: review, Owner: owner,
		Confidentiality: conf, Status: "active", Version: "1.0",
	}
}

func govMap(atoms []PortableAtom, gov GovernanceTag) map[string]GovernanceTag {
	m := make(map[string]GovernanceTag)
	for _, a := range atoms {
		m[a.ID] = gov
	}
	return m
}

// --- Happy path ---

func TestProjectGovernedCertified(t *testing.T) {
	atoms := []PortableAtom{
		testAtom("A1", "La garantie couvre les dégâts des eaux selon les conditions générales.", "sha256:abc", "approved"),
	}
	gov := govMap(atoms, testGov("approved", "actuarial@example.com", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())

	if result.TotalAtoms != 1 {
		t.Fatalf("expected 1 atom, got %d", result.TotalAtoms)
	}
	if result.Embeddable != 1 {
		t.Fatalf("expected 1 embeddable, got %d (rejected=%d)", result.Embeddable, result.Rejected)
	}
	chunk := result.Chunks[0]
	if chunk.Authority != AuthorityCertified {
		t.Fatalf("expected certified, got %q", chunk.Authority)
	}
	if !chunk.Embeddable {
		t.Fatalf("expected embeddable, reject=%q", chunk.RejectReason)
	}
	if chunk.Governance.Owner != "actuarial@example.com" {
		t.Fatalf("expected owner, got %q", chunk.Governance.Owner)
	}
	if chunk.Provenance.AtomID != "A1" {
		t.Fatalf("expected provenance atom A1, got %q", chunk.Provenance.AtomID)
	}
}

// --- Authority levels ---

func TestAuthorityProvisional(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Draft content here for review.", "sha256:x", "")}
	gov := govMap(atoms, testGov("draft", "owner", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	if result.Chunks[0].Authority != AuthorityProvisional {
		t.Fatalf("expected provisional, got %q", result.Chunks[0].Authority)
	}
}

func TestAuthorityDerived(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Rejected content that was reviewed.", "sha256:x", "")}
	gov := govMap(atoms, testGov("rejected", "owner", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	if result.Chunks[0].Authority != AuthorityDerived {
		t.Fatalf("expected derived, got %q", result.Chunks[0].Authority)
	}
}

func TestAuthorityUncertified(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "No hash content for testing.", "", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	config := DefaultGovernedConfig()
	config.RequireHash = false // don't reject, just classify
	result := ProjectGoverned(atoms, gov, config)
	if result.Chunks[0].Authority != AuthorityUncertified {
		t.Fatalf("expected uncertified, got %q", result.Chunks[0].Authority)
	}
}

// --- Safety gates ---

func TestGateRejectsEmptyContent(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
	if !strings.Contains(result.Chunks[0].RejectReason, "empty content") {
		t.Fatalf("expected empty content reason, got %q", result.Chunks[0].RejectReason)
	}
}

func TestGateRejectsMissingHash(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Some content for the test here.", "", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	config := DefaultGovernedConfig()
	config.RequireHash = true
	result := ProjectGoverned(atoms, gov, config)
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
	assertGateFailed(t, result.Chunks[0], "hash_present")
}

func TestGateRejectsConfidential(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Secret content that should not be embedded in RAG.", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "secret"))
	config := DefaultGovernedConfig()
	config.BlockConfidential = true
	result := ProjectGoverned(atoms, gov, config)
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
	assertGateFailed(t, result.Chunks[0], "confidentiality_clear")
}

func TestGateRejectsRestricted(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Restricted content that should not be in RAG index.", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "restricted"))
	config := DefaultGovernedConfig()
	config.BlockConfidential = true
	result := ProjectGoverned(atoms, gov, config)
	assertGateFailed(t, result.Chunks[0], "confidentiality_clear")
}

func TestGateRejectsUnapprovedWhenRequired(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Draft content pending review and approval.", "sha256:x", "")}
	gov := govMap(atoms, testGov("draft", "owner", "public"))
	config := DefaultGovernedConfig()
	config.RequireApproved = true
	result := ProjectGoverned(atoms, gov, config)
	assertGateFailed(t, result.Chunks[0], "approved_only")
}

func TestGateAllowsPublicApproved(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Approved public content ready for embedding into the vector index with full governance metadata and provenance chain verified.", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	config := DefaultGovernedConfig()
	config.RequireApproved = true
	config.BlockConfidential = true
	result := ProjectGoverned(atoms, gov, config)
	if result.Embeddable != 1 {
		t.Fatalf("expected embeddable, got rejected=%d reason=%q", result.Rejected, result.Chunks[0].RejectReason)
	}
}

func TestGateTokenBoundsReject(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "x", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	config := DefaultGovernedConfig()
	config.MinTokens = 50 // "x" is 1 token
	result := ProjectGoverned(atoms, gov, config)
	assertGateFailed(t, result.Chunks[0], "token_bounds")
}

// --- Provenance chain ---

func TestProvenanceFields(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Content with full provenance chain tracking.", "sha256:abc", "")}
	atoms[0].ParentID = "P1"
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	prov := result.Chunks[0].Provenance
	if prov.AtomID != "A1" {
		t.Fatalf("expected A1, got %q", prov.AtomID)
	}
	if prov.ParentID != "P1" {
		t.Fatalf("expected P1, got %q", prov.ParentID)
	}
	if prov.SourceHash != "sha256:abc" {
		t.Fatalf("expected hash, got %q", prov.SourceHash)
	}
	if prov.Profile != "rbok-lawbook" {
		t.Fatalf("expected rbok-lawbook, got %q", prov.Profile)
	}
}

// --- Chain hash ---

func TestChainHashDeterministic(t *testing.T) {
	atoms := []PortableAtom{
		testAtom("A1", "First content for hash chain test.", "sha256:a", ""),
		testAtom("A2", "Second content for hash chain test.", "sha256:b", ""),
	}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	r1 := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	r2 := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	if r1.ChainHash != r2.ChainHash {
		t.Fatalf("chain hash not deterministic: %q vs %q", r1.ChainHash, r2.ChainHash)
	}
	if !strings.HasPrefix(r1.ChainHash, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %q", r1.ChainHash)
	}
}

// --- Multiple atoms mixed ---

func TestProjectGovernedMixed(t *testing.T) {
	atoms := []PortableAtom{
		testAtom("OK", "Valid approved public content for embedding into the vector retrieval index with full governance metadata and provenance chain.", "sha256:a", ""),
		testAtom("EMPTY", "", "sha256:b", ""),
		testAtom("SECRET", "Secret restricted content should be blocked.", "sha256:c", ""),
	}
	govM := map[string]GovernanceTag{
		"OK":     testGov("approved", "owner", "public"),
		"EMPTY":  testGov("approved", "owner", "public"),
		"SECRET": testGov("approved", "owner", "secret"),
	}
	config := DefaultGovernedConfig()
	config.BlockConfidential = true
	result := ProjectGoverned(atoms, govM, config)
	if result.Embeddable != 1 {
		t.Fatalf("expected 1 embeddable, got %d", result.Embeddable)
	}
	if result.Rejected != 2 {
		t.Fatalf("expected 2 rejected, got %d", result.Rejected)
	}
}

// --- Empty input ---

func TestProjectGovernedEmpty(t *testing.T) {
	result := ProjectGoverned(nil, nil, DefaultGovernedConfig())
	if result.TotalAtoms != 0 {
		t.Fatalf("expected 0, got %d", result.TotalAtoms)
	}
	if !strings.HasPrefix(result.ChainHash, "sha256:") {
		t.Fatalf("expected sha256 chain hash even for empty, got %q", result.ChainHash)
	}
}

// --- JSON roundtrip ---

func TestGovernedJSON(t *testing.T) {
	atoms := []PortableAtom{testAtom("A1", "Content for JSON roundtrip serialization test.", "sha256:x", "")}
	gov := govMap(atoms, testGov("approved", "owner", "public"))
	result := ProjectGoverned(atoms, gov, DefaultGovernedConfig())
	var buf bytes.Buffer
	if err := WriteGovernedJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	var decoded GovernedProjectionResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Embeddable != result.Embeddable {
		t.Fatal("roundtrip mismatch")
	}
}

// --- Constants ---

func TestAuthorityConstants(t *testing.T) {
	if AuthorityCertified != "certified" || AuthorityProvisional != "provisional" ||
		AuthorityDerived != "derived" || AuthorityUncertified != "uncertified" {
		t.Fatal("authority constants wrong")
	}
}

// --- helpers ---

func assertGateFailed(t *testing.T, chunk GovernedChunk, gateName string) {
	t.Helper()
	for _, g := range chunk.SafetyGates {
		if g.Name == gateName && !g.Passed {
			return
		}
	}
	var names []string
	for _, g := range chunk.SafetyGates {
		status := "pass"
		if !g.Passed {
			status = "fail"
		}
		names = append(names, g.Name+"="+status)
	}
	t.Fatalf("expected gate %q to fail, gates: %v", gateName, names)
}
