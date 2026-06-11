package bundle

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
)

func testTrace(t *testing.T) TraceManifest {
	t.Helper()
	tr, err := NewTraceManifest(
		TraceGitContext{Repo: "RBOKproject/NOMOS", Branch: "main", Commit: "0123456789abcdef0123456789abcdef01234567"},
		"2026-06-10T00:00:00Z", "nomos-test-bundle", "", "", []string{"**/*.md"},
	)
	if err != nil {
		t.Fatalf("trace manifest: %v", err)
	}
	return tr
}

func sampleSources() []SourceFile {
	return []SourceFile{
		{RelPath: "rules.md", Content: []byte("# Garanties\n\nToute réponse gouvernée doit citer la source applicable ou s'abstenir.\n")},
		{RelPath: "sub/defs.md", Content: []byte("# Définitions\n\nUne garantie est une obligation contractuelle opposable.\n")},
	}
}

func buildSample(t *testing.T) Bundle {
	t.Helper()
	b, err := Build(BuildInput{
		BundleID:    "nomos-test-bundle",
		Producer:    "nomos",
		Domain:      "built-environment",
		GeneratedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		Sources:     sampleSources(),
		Trace:       testTrace(t),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return b
}

func TestBuild_ProducesValidBundleFromRealAtomization(t *testing.T) {
	b := buildSample(t)
	if b.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", b.SchemaVersion)
	}
	if len(b.Feeds) != 1 || len(b.Feeds[0].Nodes) == 0 {
		t.Fatalf("expected one feed with nodes, got %d feeds", len(b.Feeds))
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("emitted bundle invalid: %v", err)
	}
	// Every node carries a real content hash and contract-valid facets.
	for _, node := range b.Feeds[0].Nodes {
		if !strings.HasPrefix(node.SourceHash, "sha256:") {
			t.Errorf("node %s source_hash not a real sha256: %q", node.NodeID, node.SourceHash)
		}
		if node.Facets.IsZero() {
			t.Errorf("node %s has no facets", node.NodeID)
		}
		if err := node.Facets.Validate(); err != nil {
			t.Errorf("node %s facets invalid: %v", node.NodeID, err)
		}
	}
}

func TestBuild_RAGMetadataReferencesRealNodes(t *testing.T) {
	b := buildSample(t)
	nodeIDs := map[string]bool{}
	for _, n := range b.Feeds[0].Nodes {
		nodeIDs[n.NodeID] = true
	}
	if len(b.RAGMetadata) == 0 {
		t.Fatal("no rag_metadata emitted")
	}
	for _, m := range b.RAGMetadata {
		if !nodeIDs[m.NodeID] {
			t.Errorf("rag_metadata references orphan node_id %q", m.NodeID)
		}
	}
}

// Claim-boundary honesty (doctrine §2.2): the bundle is hash-bound, not signed.
// Emitting attestLevel "signed" without a signature is the exact violation the
// audit (#519) flagged. This test fails if anyone flips it to "signed" here.
func TestBuild_AttestationIsHonestlyUnsigned(t *testing.T) {
	b := buildSample(t)
	var pred map[string]any
	if err := json.Unmarshal(b.Attestation.Predicate, &pred); err != nil {
		t.Fatalf("decode predicate: %v", err)
	}
	level, _ := pred["attestLevel"].(string)
	if level == "signed" {
		t.Fatal("bundle claims attestLevel=signed but nothing is cryptographically signed (claim-boundary violation)")
	}
	if level != AttesterAttestLevel {
		t.Fatalf("attestLevel = %q, want %q", level, AttesterAttestLevel)
	}
	// The subject digest must be the real sha256 of the feed payload.
	if len(b.Attestation.Subject) == 0 {
		t.Fatal("attestation has no subject")
	}
	feedJSON, _ := json.Marshal(b.Feeds)
	want := hexDigest(feedJSON)
	if got := b.Attestation.Subject[0].Digest["sha256"]; got != want {
		t.Fatalf("attestation subject digest %q != real feed digest %q", got, want)
	}
}

// Adversarial: tampering a node's source_hash must make in-engine validation
// fail. Without the source_hash check this passes — so the failure is the proof.
func TestValidate_RejectsTamperedSourceHash(t *testing.T) {
	b := buildSample(t)
	b.Feeds[0].Nodes[0].SourceHash = "not-a-hash"
	if err := b.Validate(); err == nil {
		t.Fatal("validation accepted a tampered source_hash")
	}
}

func TestValidate_RejectsDuplicateNodeIDAndOrphanRAG(t *testing.T) {
	dup := buildSample(t)
	dup.Feeds[0].Nodes = append(dup.Feeds[0].Nodes, dup.Feeds[0].Nodes[0])
	if err := dup.Validate(); err == nil {
		t.Fatal("validation accepted a duplicate node_id")
	}

	orphan := buildSample(t)
	orphan.RAGMetadata = append(orphan.RAGMetadata, RAGMetadata{NodeID: "NODE-DOES-NOT-EXIST", ChunkID: "x"})
	if err := orphan.Validate(); err == nil {
		t.Fatal("validation accepted orphan rag_metadata")
	}
}

func TestBuild_IsDeterministic(t *testing.T) {
	a := buildSample(t)
	b := buildSample(t)
	aj, _ := a.Marshal()
	bj, _ := b.Marshal()
	if string(aj) != string(bj) {
		t.Fatal("two builds over identical input produced different bundles")
	}
}

func TestNodeIDsAreContractValid(t *testing.T) {
	b := buildSample(t)
	for _, n := range b.Feeds[0].Nodes {
		first := rune(n.NodeID[0])
		if !((first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9')) {
			t.Errorf("node_id %q does not start with [A-Z0-9]", n.NodeID)
		}
		for _, r := range n.NodeID {
			ok := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
			if !ok {
				t.Errorf("node_id %q contains illegal rune %q", n.NodeID, r)
			}
		}
	}
}

func TestParseRepoFromRemote(t *testing.T) {
	cases := map[string]string{
		"https://github.com/RBOKproject/NOMOS.git": "RBOKproject/NOMOS",
		"git@github.com:RBOKproject/NOMOS.git":     "RBOKproject/NOMOS",
		"https://github.com/owner/repo":            "owner/repo",
		"":                                         "",
	}
	for remote, want := range cases {
		if got := ParseRepoFromRemote(remote); got != want {
			t.Errorf("ParseRepoFromRemote(%q) = %q, want %q", remote, got, want)
		}
	}
}

// Sanity: a faceted node really came through the H3 engine path.
func TestBuild_NodesCarryDerivedFacets(t *testing.T) {
	b := buildSample(t)
	var sawRule bool
	for _, n := range b.Feeds[0].Nodes {
		if n.Facets.Nature == "rule" || n.Facets.Nature == "definition" {
			sawRule = true
		}
		if n.Facets.TrustTier == "certified" {
			t.Errorf("node %s claims certified from derivation (dishonest)", n.NodeID)
		}
	}
	if !sawRule {
		_ = atomization.FacetNature("") // keep import referenced if mapping changes
		t.Fatal("expected at least one rule/definition node from the sample corpus")
	}
}
