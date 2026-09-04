package ragexport

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	"github.com/RBOKproject/Nomos/cli/internal/bundle"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// permisLens mirrors the pack preset LENS-AEC-PERMIS: permis activity AND
// public, never confidential — whatever the other axes say.
func permisLens() atomization.KnowledgeLens {
	return atomization.KnowledgeLens{
		ID: "LENS-TEST-PERMIS",
		Include: &atomization.LensPredicate{AllOf: []atomization.LensFacetSelection{{
			Activity:        []string{"aec.permis"},
			Confidentiality: "public",
		}}},
		Exclude: &atomization.LensPredicate{AnyOf: []atomization.LensFacetSelection{
			{Confidentiality: "confidential"},
		}},
	}
}

// engineFacets are the closed axes the atomizer derives on a bundle node; the
// open axes (activity, confidentiality, applicability) are pack data.
func engineFacets() *atomization.Facets {
	return &atomization.Facets{Nature: "rule", ScopeLevel: "atom", TrustTier: "unverified", Provenance: "source_backed"}
}

func facetedInput(chunkID, sourcePath string, f *atomization.Facets) Input {
	m := sampleChunk()
	m.ChunkID = chunkID
	m.SourceID = sourcePath
	m.SourcePath = sourcePath
	m.ContextHeadingPath = nil
	m.ChunkText = "Texte de " + sourcePath + " pour " + chunkID
	return Input{Chunk: m, Facets: f}
}

func withOpen(f *atomization.Facets, activity, confidentiality string) *atomization.Facets {
	cp := *f
	cp.Activity = []string{activity}
	cp.Confidentiality = atomization.FacetConfidentiality(confidentiality)
	return &cp
}

func exclusionOf(r Result, chunkID string) (Exclusion, bool) {
	for _, e := range r.Excluded {
		if e.ChunkID == chunkID {
			return e, true
		}
	}
	return Exclusion{}, false
}

// The base-level promise: a chunk the lens excludes is never handed out, so
// no consumer-side filter can leak it. The exclusions are named, not hidden.
func TestBuildInputs_LensExcludedChunkIsNeverExported(t *testing.T) {
	inputs := []Input{
		facetedInput("chunk:A-1", "conception.md", withOpen(engineFacets(), "aec.conception", "public")),
		facetedInput("chunk:A-2", "permis.md", withOpen(engineFacets(), "aec.permis", "public")),
		facetedInput("chunk:A-3", "journal-interne.md", withOpen(engineFacets(), "aec.permis", "confidential")),
	}
	lens := permisLens()
	r := BuildInputs(inputs, Options{Lens: &lens})

	if len(r.Records) != 1 || r.Records[0].ChunkID != "chunk:A-2" {
		t.Fatalf("only the permis chunk is in scope, got %+v", r.Records)
	}
	for _, rec := range r.Records {
		if _, excluded := exclusionOf(r, rec.ChunkID); excluded {
			t.Fatalf("%s is both exported and excluded", rec.ChunkID)
		}
	}
	if e, ok := exclusionOf(r, "chunk:A-1"); !ok || e.Code != ExcludeLens || e.Reason != "not_selected_by_lens" {
		t.Fatalf("conception chunk must be excluded as not selected, got %+v (found=%v)", e, ok)
	}
	if e, ok := exclusionOf(r, "chunk:A-3"); !ok || e.Code != ExcludeLens || e.Reason != "excluded_by_facets.confidentiality" {
		t.Fatalf("confidential chunk must be excluded on confidentiality, got %+v (found=%v)", e, ok)
	}
	if r.Lens == nil || r.Lens.ID != "LENS-TEST-PERMIS" || !strings.HasPrefix(r.Lens.Digest, "sha256:") {
		t.Fatalf("result must bind the lens, got %+v", r.Lens)
	}
	if len(r.Rejections) != 0 {
		t.Fatalf("exclusions are not rejections: %+v", r.Rejections)
	}
}

// No facets means membership cannot be proved: under a lens that is an
// exclusion, not a pass. Without a lens the same chunk exports normally.
func TestBuildInputs_LensWithoutFacetsFailsClosed(t *testing.T) {
	in := facetedInput("chunk:F-1", "rulebook.md", nil)
	lens := permisLens()

	scoped := BuildInputs([]Input{in}, Options{Lens: &lens})
	if len(scoped.Records) != 0 {
		t.Fatalf("a facet-less chunk passed a lens: %+v", scoped.Records)
	}
	if e, ok := exclusionOf(scoped, "chunk:F-1"); !ok || e.Code != ExcludeLensNoFacets {
		t.Fatalf("expected %s, got %+v (found=%v)", ExcludeLensNoFacets, e, ok)
	}

	unscoped := BuildInputs([]Input{in}, Options{})
	if len(unscoped.Records) != 1 || unscoped.Records[0].Metadata.Facets != nil || unscoped.Lens != nil {
		t.Fatalf("without a lens the chunk exports with no facets and no binding, got %+v", unscoped)
	}
}

// Pack document facets supply the open axes the engine cannot derive; they
// flip the verdict and the exported record carries the merged facets.
func TestBuildInputs_DocumentFacetsChangeTheVerdict(t *testing.T) {
	in := facetedInput("chunk:P-1", "permis.md", engineFacets())
	lens := permisLens()

	without := BuildInputs([]Input{in}, Options{Lens: &lens})
	if len(without.Records) != 0 {
		t.Fatal("engine facets alone carry no activity: the chunk must not be in scope")
	}

	docs := map[string]atomization.Facets{
		"permis.md": {Activity: []string{"aec.permis"}, Confidentiality: "public", Applicability: "applicable"},
	}
	with := BuildInputs([]Input{in}, Options{Lens: &lens, DocumentFacets: docs})
	if len(with.Records) != 1 {
		t.Fatalf("document facets must bring the chunk in scope, got %+v", with.Excluded)
	}
	f := with.Records[0].Metadata.Facets
	if f == nil || f.Nature != "rule" || len(f.Activity) != 1 || f.Activity[0] != "aec.permis" || f.Confidentiality != "public" {
		t.Fatalf("merged facets must keep engine axes and add pack axes, got %+v", f)
	}
}

// A pack keyed by source id (not path) still applies: the lookup falls back.
func TestBuildInputs_DocumentFacetsFallBackToSourceID(t *testing.T) {
	in := facetedInput("chunk:P-2", "permis.md", engineFacets())
	in.Chunk.SourcePath = ""
	lens := permisLens()
	docs := map[string]atomization.Facets{"permis.md": {Activity: []string{"aec.permis"}, Confidentiality: "public"}}
	r := BuildInputs([]Input{in}, Options{Lens: &lens, DocumentFacets: docs})
	if len(r.Records) != 1 {
		t.Fatalf("document facets keyed by source id were not applied: %+v", r.Excluded)
	}
}

func TestLensDigest_IsStableAndMovesWithTheLens(t *testing.T) {
	if LensDigest(permisLens()) != LensDigest(permisLens()) {
		t.Fatal("lens digest is not reproducible")
	}
	edited := permisLens()
	edited.Exclude.AnyOf = append(edited.Exclude.AnyOf, atomization.LensFacetSelection{Applicability: "blocked"})
	if LensDigest(edited) == LensDigest(permisLens()) {
		t.Fatal("an edited lens kept its digest: an index could claim a scope it was not built for")
	}
	renamed := permisLens()
	renamed.ID = "LENS-OTHER"
	if LensDigest(renamed) == LensDigest(permisLens()) {
		t.Fatal("the lens id is part of the binding")
	}
}

func TestManifest_BindsTheLensAndComputesTheContract(t *testing.T) {
	inputs := []Input{
		facetedInput("chunk:A-1", "conception.md", withOpen(engineFacets(), "aec.conception", "public")),
		facetedInput("chunk:A-2", "permis.md", withOpen(engineFacets(), "aec.permis", "public")),
		facetedInput("chunk:A-3", "journal-interne.md", withOpen(engineFacets(), "aec.permis", "confidential")),
	}
	lens := permisLens()
	feed := &corpus.Feed{Format: "ckm-bundle-v1", GeneratedAt: "2026-06-01T00:00:00Z", ContentHash: "sha256:bundle"}
	m := BuildManifest(feed, BuildInputs(inputs, Options{Lens: &lens}), FormatJSONL)

	if m.Lens == nil || m.Lens.ID != "LENS-TEST-PERMIS" {
		t.Fatalf("manifest must bind the lens, got %+v", m.Lens)
	}
	if m.ChunkCount != 1 || m.ExcludedByLensCount != 2 {
		t.Fatalf("counts drifted: chunks=%d excluded=%d", m.ChunkCount, m.ExcludedByLensCount)
	}
	c := m.RetrievalContract
	if c.SchemaVersion != RetrievalContractSchemaVersion || c.Scope != ScopeLens || c.Lens == nil || c.ExcludedByLens != 2 {
		t.Fatalf("contract header drifted: %+v", c)
	}
	var activity, confidentiality []string
	for _, f := range c.FilterFields {
		switch f.Field {
		case "facets.activity":
			activity = f.Values
		case "facets.confidentiality":
			confidentiality = f.Values
		}
	}
	if len(activity) != 1 || activity[0] != "aec.permis" || len(confidentiality) != 1 || confidentiality[0] != "public" {
		t.Fatalf("contract must list the values actually exported (never the excluded ones): activity=%v confidentiality=%v", activity, confidentiality)
	}
	if len(c.Unsupported) != 1 || c.Unsupported[0].Capability != "temporal_scoping" {
		t.Fatalf("temporal scoping must be declared unsupported: %+v", c.Unsupported)
	}
	if !strings.Contains(c.ClaimBoundary, "does not rank") {
		t.Fatalf("claim boundary must state that Nomos does not rank: %q", c.ClaimBoundary)
	}
}

func TestRetrievalContract_UnscopedExportSaysSo(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})
	c := m.RetrievalContract
	if c.Scope != ScopeUnscoped || c.Lens != nil || c.ExcludedByLens != 0 {
		t.Fatalf("an unscoped export must say so: %+v", c)
	}
	fields := map[string][]string{}
	for _, f := range c.FilterFields {
		fields[f.Field] = f.Values
	}
	if v := fields["priority"]; len(v) != 1 || v[0] != "primary" {
		t.Fatalf("observed priority vocabulary drifted: %v", v)
	}
	if v := fields["source_id"]; len(v) != 1 || v[0] != "SRC-1" {
		t.Fatalf("observed source ids drifted: %v", v)
	}
	for field := range fields {
		if strings.HasPrefix(field, "facets.") {
			t.Fatalf("a feed export carries no facets, yet the contract lists %s", field)
		}
	}
}

func TestRetrievalContract_IsDeterministic(t *testing.T) {
	inputs := []Input{
		facetedInput("chunk:A-2", "permis.md", withOpen(engineFacets(), "aec.permis", "public")),
		facetedInput("chunk:A-1", "conception.md", withOpen(engineFacets(), "aec.conception", "public")),
	}
	first, _ := json.Marshal(BuildRetrievalContract(BuildInputs(inputs, Options{})))
	second, _ := json.Marshal(BuildRetrievalContract(BuildInputs(inputs, Options{})))
	if string(first) != string(second) {
		t.Fatal("two contracts of the same export differ")
	}
}

func TestEncode_LangChainFlattensFacets(t *testing.T) {
	in := facetedInput("chunk:A-2", "permis.md", withOpen(engineFacets(), "aec.permis", "public"))
	r := BuildInputs([]Input{in}, Options{})
	out, err := Encode(r.Records, FormatLangChain)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Metadata map[string]any `json:"metadata"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Metadata["facet_nature"] != "rule" {
		t.Fatalf("facet_nature missing from flattened metadata: %+v", doc.Metadata)
	}
	activity, _ := doc.Metadata["facet_activity"].([]any)
	if len(activity) != 1 || activity[0] != "aec.permis" {
		t.Fatalf("facet_activity missing or wrong: %+v", doc.Metadata["facet_activity"])
	}
}

func TestInputsFromBundle_ProjectsNodesWithFacets(t *testing.T) {
	b := &bundle.Bundle{
		SchemaVersion: "ckm-bundle-v1",
		GeneratedAt:   "2026-06-01T00:00:00Z",
		Feeds: []bundle.Feed{{
			FeedID: "f",
			Nodes: []bundle.Node{
				{NodeID: "A-1", Text: "La mise a l'enquete dure trente jours.", SourcePath: "permis.md", SourceHash: "sha256:p1",
					Span: bundle.Span{StartLine: 5, EndLine: 5}, Facets: *engineFacets()},
				{NodeID: "A-2", Text: "Sans entree rag_metadata.", SourcePath: "permis.md", SourceHash: "sha256:p1",
					Span: bundle.Span{StartLine: 7, EndLine: 7}, Facets: *engineFacets()},
			},
		}},
		RAGMetadata: []bundle.RAGMetadata{{NodeID: "A-1", ChunkID: "chunk:A-1", SourcePath: "permis.md", SourceHash: "sha256:p1"}},
	}
	inputs := InputsFromBundle(b)
	if len(inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %d", len(inputs))
	}
	first := inputs[0]
	if first.Chunk.ChunkID != "chunk:A-1" || first.Chunk.SourceID != "permis.md" || first.Chunk.SourceHash != "sha256:p1" ||
		first.Chunk.CanonicalUnitID != "A-1" || first.Chunk.Locator != "permis.md:L5-L5" || first.Chunk.ChunkText != "La mise a l'enquete dure trente jours." {
		t.Fatalf("node projection drifted: %+v", first.Chunk)
	}
	if first.Facets == nil || first.Facets.Nature != "rule" {
		t.Fatalf("node facets must travel with the input: %+v", first.Facets)
	}
	if inputs[1].Chunk.ChunkID != "chunk:A-2" {
		t.Fatalf("a node without rag_metadata must still get a chunk id, got %q", inputs[1].Chunk.ChunkID)
	}
	r := BuildInputs(inputs, Options{})
	if len(r.Records) != 2 || len(r.Rejections) != 0 {
		t.Fatalf("bundle inputs must export cleanly: %+v", r.Rejections)
	}
	if r.Records[0].BodyText != "La mise a l'enquete dure trente jours." {
		t.Fatalf("node text must be the citable body, got %q", r.Records[0].BodyText)
	}
}
