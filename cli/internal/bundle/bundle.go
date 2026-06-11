// Package bundle emits the Canonical Knowledge Bundle (CKM-13 / CKM-H4): the
// versioned, inspectable artifact a downstream consumer (Aedifica, RBOK, …)
// imports instead of depending on NOMOS code.
//
// The audit (#518) found that the bundle contract + Python validator were real
// but there was NO Go emitter, so Aedifica consumed a hand-crafted fixture
// (`bundle_id: "aedifica-fixture"`). This package makes NOMOS *produce* the
// bundle from a real corpus run: real source files → the genuine atomization
// engine → faceted nodes with real content hashes.
//
// The emitted shape conforms to specs/canonical-knowledge-bundle.cue and passes
// scripts/ckm_bundle_validate.py (proven end-to-end in bundle_test.go).
package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/attestation"
)

const (
	// SchemaVersion is the bundle contract version (specs/canonical-knowledge-bundle.cue).
	SchemaVersion = "ckm-bundle-v1"
	// FeedFormat identifies the canonical knowledge feed payload format.
	FeedFormat = "nomos.canonical-knowledge-feed.v1"
	// DefaultClaimBoundary is the honest, minimal handoff claim (doctrine §2.2):
	// the bundle asserts provenance, not downstream success or correctness.
	DefaultClaimBoundary = "Canonical bundle handoff only; no downstream import success or regulatory correctness claim."
	// AttesterAttestLevel is deliberately "basic", not "signed": the bundle is
	// tamper-evident by content hash but is NOT cryptographically signed. Emitting
	// "signed" here without a signature is the exact claim-boundary violation the
	// audit (#519) flagged in the hand-crafted fixture.
	AttesterAttestLevel = "basic"
)

// Bundle is the top-level #CanonicalKnowledgeBundle artifact.
type Bundle struct {
	SchemaVersion string                      `json:"schema_version"`
	BundleID      string                      `json:"bundle_id"`
	GeneratedAt   string                      `json:"generated_at"`
	Producer      string                      `json:"producer"`
	ClaimBoundary string                      `json:"claim_boundary"`
	Feeds         []Feed                      `json:"feeds"`
	RAGMetadata   []RAGMetadata               `json:"rag_metadata"`
	TraceManifest TraceManifest               `json:"trace_manifest"`
	Attestation   attestation.InTotoStatement `json:"attestation"`
}

// Feed is a #BundleFeed: a named set of faceted nodes.
type Feed struct {
	FeedID string `json:"feed_id"`
	Format string `json:"format"`
	Nodes  []Node `json:"nodes"`
}

// Node is a #BundleNode: one traceable knowledge atom with real provenance.
type Node struct {
	NodeID      string             `json:"node_id"`
	Text        string             `json:"text"`
	SourcePath  string             `json:"source_path"`
	SourceHash  string             `json:"source_hash"`
	Span        Span               `json:"span"`
	ParentChain []string           `json:"parent_chain"`
	Facets      atomization.Facets `json:"facets"`
}

// Span is a #BundleSpan. All fields are optional; line spans are populated from
// the atomization source span.
type Span struct {
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	StartByte int    `json:"start_byte,omitempty"`
	EndByte   int    `json:"end_byte,omitempty"`
	Locator   string `json:"locator,omitempty"`
}

// RAGMetadata is a #BundleRAGMetadata entry linking a node to its retrieval chunk.
type RAGMetadata struct {
	NodeID         string              `json:"node_id"`
	ChunkID        string              `json:"chunk_id"`
	SourcePath     string              `json:"source_path"`
	SourceHash     string              `json:"source_hash"`
	ParentChain    []string            `json:"parent_chain"`
	Facets         *atomization.Facets `json:"facets,omitempty"`
	EmbeddingModel string              `json:"embedding_model,omitempty"`
	EmbeddingDim   int                 `json:"embedding_dim,omitempty"`
}

// SourceFile is one real source document to atomize into the bundle.
type SourceFile struct {
	// RelPath is the corpus-relative path recorded as source_path.
	RelPath string
	// Content is the raw bytes; its sha256 becomes the node source_hash.
	Content []byte
}

// BuildInput is the full set of inputs for a bundle emission.
type BuildInput struct {
	BundleID      string
	Producer      string
	ClaimBoundary string
	Domain        string
	FeedID        string
	GeneratedAt   time.Time
	Sources       []SourceFile
	Trace         TraceManifest
}

// Build runs the real atomization engine over the source files and assembles a
// contract-conforming bundle. It returns an error if the result would not
// satisfy the bundle invariants (validated in-engine before returning).
func Build(in BuildInput) (Bundle, error) {
	if in.BundleID == "" {
		return Bundle{}, fmt.Errorf("bundle_id is required")
	}
	if in.Producer == "" {
		return Bundle{}, fmt.Errorf("producer is required")
	}
	if len(in.Sources) == 0 {
		return Bundle{}, fmt.Errorf("at least one source file is required")
	}

	claim := in.ClaimBoundary
	if claim == "" {
		claim = DefaultClaimBoundary
	}
	feedID := in.FeedID
	if feedID == "" {
		feedID = in.BundleID + "-feed"
	}

	var nodes []Node
	var ragMeta []RAGMetadata
	seenNodeIDs := map[string]bool{}

	// Deterministic order: sort sources by path so two runs over identical input
	// produce byte-identical bundles (important for evidence archival).
	sources := append([]SourceFile(nil), in.Sources...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].RelPath < sources[j].RelPath })

	for _, src := range sources {
		sourceHash := "sha256:" + hexDigest(src.Content)

		ast := atomization.ParseMarkdown(string(src.Content))
		// DocumentRef = the source path keeps atom IDs unique across files.
		set := atomization.Atomize(ast, atomization.AtomizeOptions{
			DocumentRef:  src.RelPath,
			SourceFile:   src.RelPath,
			Domain:       in.Domain,
			DefaultState: atomization.ReviewDraft,
			EmitFacets:   true,
		})

		atomByID := map[string]atomization.Atom{}
		for _, a := range set.Atoms {
			atomByID[a.ID] = a
		}

		for _, a := range set.Atoms {
			text := strings.TrimSpace(a.Text)
			if text == "" {
				continue // #BundleNode.text requires MinRunes(1)
			}
			nodeID := normalizeNodeID(a.ID)
			if seenNodeIDs[nodeID] {
				// Defensive: guarantee the uniqueness invariant the validator checks.
				nodeID = nodeID + "." + shortHash(a.CanonicalRef)
			}
			seenNodeIDs[nodeID] = true

			facets := atomization.Facets{}
			if a.Facets != nil {
				facets = *a.Facets
			}
			parentChain := parentChainIDs(a, atomByID)

			nodes = append(nodes, Node{
				NodeID:     nodeID,
				Text:       text,
				SourcePath: src.RelPath,
				SourceHash: sourceHash,
				Span: Span{
					StartLine: a.SourceSpan.StartLine,
					EndLine:   a.SourceSpan.EndLine,
				},
				ParentChain: parentChain,
				Facets:      facets,
			})

			facetCopy := facets
			ragMeta = append(ragMeta, RAGMetadata{
				NodeID:      nodeID,
				ChunkID:     "chunk:" + nodeID,
				SourcePath:  src.RelPath,
				SourceHash:  sourceHash,
				ParentChain: parentChain,
				Facets:      &facetCopy,
			})
		}
	}

	if len(nodes) == 0 {
		return Bundle{}, fmt.Errorf("no content-bearing atoms produced from %d source file(s)", len(sources))
	}

	feeds := []Feed{{FeedID: feedID, Format: FeedFormat, Nodes: nodes}}

	att, err := buildAttestation(in, feeds)
	if err != nil {
		return Bundle{}, err
	}

	b := Bundle{
		SchemaVersion: SchemaVersion,
		BundleID:      in.BundleID,
		GeneratedAt:   in.GeneratedAt.UTC().Format(time.RFC3339),
		Producer:      in.Producer,
		ClaimBoundary: claim,
		Feeds:         feeds,
		RAGMetadata:   ragMeta,
		TraceManifest: in.Trace,
		Attestation:   att,
	}

	if err := b.Validate(); err != nil {
		return Bundle{}, fmt.Errorf("emitted bundle failed in-engine validation: %w", err)
	}
	return b, nil
}

// buildAttestation produces an in-toto statement over the feed payload. The
// subject digest is the real sha256 of the canonical feed JSON, so a consumer
// can re-serialize feeds and verify the bundle was not tampered with. attestLevel
// is "basic" — hash-bound, not signed (see AttesterAttestLevel).
func buildAttestation(in BuildInput, feeds []Feed) (attestation.InTotoStatement, error) {
	feedJSON, err := json.Marshal(feeds)
	if err != nil {
		return attestation.InTotoStatement{}, fmt.Errorf("marshal feeds for attestation: %w", err)
	}
	subject := attestation.SubjectFromBytes(in.BundleID+"/feeds", feedJSON)

	att := attestation.NomosAttestation{
		ProjectID:   "nomos-ckm",
		Verdict:     "admitted",
		Confidence:  "medium",
		Timestamp:   in.GeneratedAt.UTC(),
		Evidence:    []string{fmt.Sprintf("%d feed node(s) over %d source file(s)", countNodes(feeds), len(in.Sources))},
		AttesterID:  in.Producer,
		AttestLevel: AttesterAttestLevel,
	}
	return attestation.GenerateStatement(att, []attestation.Subject{subject})
}

// Validate enforces the bundle invariants in-engine — the same invariants
// scripts/ckm_bundle_validate.py checks (≥1 feed, ≥1 node per feed, unique
// node_ids, no orphan rag_metadata) plus per-node facet contract validation.
func (b Bundle) Validate() error {
	if b.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version %q != %q", b.SchemaVersion, SchemaVersion)
	}
	if len(b.Feeds) == 0 {
		return fmt.Errorf("bundle must contain at least one feed")
	}
	nodeIDs := map[string]bool{}
	for fi, feed := range b.Feeds {
		if len(feed.Nodes) == 0 {
			return fmt.Errorf("feeds[%d] %q has no nodes", fi, feed.FeedID)
		}
		for ni, node := range feed.Nodes {
			if node.NodeID == "" {
				return fmt.Errorf("feeds[%d].nodes[%d] missing node_id", fi, ni)
			}
			if nodeIDs[node.NodeID] {
				return fmt.Errorf("duplicate node_id %q", node.NodeID)
			}
			nodeIDs[node.NodeID] = true
			if strings.TrimSpace(node.Text) == "" {
				return fmt.Errorf("node %q has empty text", node.NodeID)
			}
			if !sourceHashRe(node.SourceHash) {
				return fmt.Errorf("node %q has invalid source_hash %q", node.NodeID, node.SourceHash)
			}
			if err := node.Facets.Validate(); err != nil {
				return fmt.Errorf("node %q: %w", node.NodeID, err)
			}
		}
	}
	for i, m := range b.RAGMetadata {
		if m.NodeID == "" {
			return fmt.Errorf("rag_metadata[%d] missing node_id", i)
		}
		if !nodeIDs[m.NodeID] {
			return fmt.Errorf("rag_metadata[%d] references unknown node_id %q", i, m.NodeID)
		}
	}
	return nil
}

// Marshal renders the bundle as indented JSON.
func (b Bundle) Marshal() ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

func countNodes(feeds []Feed) int {
	n := 0
	for _, f := range feeds {
		n += len(f.Nodes)
	}
	return n
}

func parentChainIDs(a atomization.Atom, byID map[string]atomization.Atom) []string {
	chain := []string{}
	visited := map[string]bool{}
	cur := a.ParentID
	for cur != "" && !visited[cur] {
		visited[cur] = true
		parent, ok := byID[cur]
		if !ok {
			break
		}
		chain = append([]string{normalizeNodeID(parent.ID)}, chain...)
		cur = parent.ParentID
	}
	return chain
}

// normalizeNodeID coerces an atom ID into the #BundleNode node_id charset
// (^[A-Z0-9][A-Z0-9._-]*$).
func normalizeNodeID(id string) string {
	up := strings.ToUpper(id)
	var b strings.Builder
	for _, r := range up {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	out = strings.TrimLeft(out, "._-")
	if out == "" {
		out = "N-" + shortHash(id)
	}
	return out
}

func sourceHashRe(s string) bool {
	for _, prefix := range []string{"sha256:", "sha384:", "sha512:"} {
		if strings.HasPrefix(s, prefix) {
			hexPart := s[len(prefix):]
			if hexPart == "" {
				return false
			}
			for _, r := range hexPart {
				if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func hexDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func shortHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return strings.ToUpper(hex.EncodeToString(h[:4]))
}
