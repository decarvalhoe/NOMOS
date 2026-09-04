package ragexport

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// secondChunk is a well-formed chunk from a different source, so tests can
// prove that invalidation is per source.
func secondChunk() corpus.ChunkMetadata {
	other := sampleChunk()
	other.ChunkID = "chunk:SRC-2:000-050"
	other.SourceID = "SRC-2"
	other.SourceHash = "sha256:dddd"
	other.ChunkText = "Titre I/Chapitre 3\n\nTexte de la seconde source."
	return other
}

func chunkPlan(d Delta, id string) (ChunkDelta, bool) {
	for _, c := range d.Chunks {
		if c.ChunkID == id {
			return c, true
		}
	}
	return ChunkDelta{}, false
}

func sourceStatus(d Delta, id string) string {
	for _, s := range d.Sources {
		if s.SourceID == id {
			return s.Status
		}
	}
	return "<missing>"
}

func TestDiff_IdenticalManifestsAreFresh(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})
	d := Diff(m, m)
	if d.Stale || d.FullReindex {
		t.Fatalf("identical manifests reported stale: %+v", d)
	}
	if len(d.Chunks) != 0 || d.Summary.Unchanged != 2 {
		t.Fatalf("expected 2 unchanged chunks and an empty plan, got %+v", d.Summary)
	}
	for _, s := range d.Sources {
		if s.Status != SourceUnchanged {
			t.Fatalf("source %s reported %s on identical manifests", s.SourceID, s.Status)
		}
	}
}

// The whole point of the plan: an edited body must be scheduled for
// re-embedding, and ONLY that chunk.
func TestDiff_BodyEditIsReindexedNotIgnored(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})

	edited := sampleChunk()
	edited.ChunkText = "Titre I/Chapitre 3\n\nLe delai ne court pas."
	edited.SourceHash = "sha256:a2a2" // the file moved with its body
	after := manifestOf(t, []corpus.ChunkMetadata{edited, secondChunk()})

	d := Diff(before, after)
	if !d.Stale {
		t.Fatal("a body edit left the index reported fresh")
	}
	if d.FullReindex {
		t.Fatalf("a one-chunk edit must not force a full reindex: %v", d.FullReindexReasons)
	}
	c, ok := chunkPlan(d, "chunk:SRC-1:100-220")
	if !ok || c.Action != ActionEmbed || c.Reason != ReasonBodyChanged {
		t.Fatalf("edited chunk must be re-embedded for body_changed, got %+v (found=%v)", c, ok)
	}
	if _, touched := chunkPlan(d, "chunk:SRC-2:000-050"); touched {
		t.Fatal("a chunk of the untouched source was scheduled: invalidation is not per source")
	}
	if d.Summary.Unchanged != 1 || d.Summary.Embed != 1 {
		t.Fatalf("summary drifted: %+v", d.Summary)
	}
	if sourceStatus(d, "SRC-1") != SourceChanged || sourceStatus(d, "SRC-2") != SourceUnchanged {
		t.Fatalf("per-source statuses drifted: %+v", d.Sources)
	}
	if d.Summary.SourcesChanged != 1 {
		t.Fatalf("expected 1 changed source, got %d", d.Summary.SourcesChanged)
	}
}

// A heading move changes embedding_text but not the citable body: the chunk
// must be re-embedded (retrieval text moved) and the reason must say why.
func TestDiff_ContextOnlyChangeIsReembedded(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})

	moved := sampleChunk()
	moved.ContextHeadingPath = []string{"Titre II", "Chapitre 3"}
	moved.ChunkText = "Titre II/Chapitre 3\n\nLe delai court des la notification."
	after := manifestOf(t, []corpus.ChunkMetadata{moved})

	c, ok := chunkPlan(Diff(before, after), "chunk:SRC-1:100-220")
	if !ok || c.Action != ActionEmbed || c.Reason != ReasonContextChanged {
		t.Fatalf("context-only change must re-embed for context_changed, got %+v (found=%v)", c, ok)
	}
}

// Same text, new source hash (an unrelated edit elsewhere in the file): the
// embedding is still valid, but the stored provenance would no longer
// re-prove a citation — so it is a metadata refresh, not a re-embed.
func TestDiff_SourceRehashWithSameTextOnlyRefreshesMetadata(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})

	rehashed := sampleChunk()
	rehashed.SourceHash = "sha256:cccc"
	after := manifestOf(t, []corpus.ChunkMetadata{rehashed})

	d := Diff(before, after)
	if !d.Stale {
		t.Fatal("a source rehash left the index reported fresh: citations would carry a dead hash")
	}
	c, ok := chunkPlan(d, "chunk:SRC-1:100-220")
	if !ok || c.Action != ActionUpdateMetadata || c.Reason != ReasonSourceHashChanged {
		t.Fatalf("expected update_metadata/source_hash_changed, got %+v (found=%v)", c, ok)
	}
	if d.Summary.Embed != 0 || d.Summary.UpdateMetadata != 1 {
		t.Fatalf("summary drifted: %+v", d.Summary)
	}
}

func TestDiff_AddedAndRemovedChunks(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})
	after := manifestOf(t, []corpus.ChunkMetadata{secondChunk()})

	d := Diff(before, after)
	gone, ok := chunkPlan(d, "chunk:SRC-1:100-220")
	if !ok || gone.Action != ActionDelete || gone.Reason != ReasonRemoved {
		t.Fatalf("a vanished chunk must be deleted, got %+v (found=%v)", gone, ok)
	}
	added, ok := chunkPlan(d, "chunk:SRC-2:000-050")
	if !ok || added.Action != ActionEmbed || added.Reason != ReasonAdded {
		t.Fatalf("a new chunk must be embedded, got %+v (found=%v)", added, ok)
	}
	if sourceStatus(d, "SRC-1") != SourceRemoved || sourceStatus(d, "SRC-2") != SourceAdded {
		t.Fatalf("per-source statuses drifted: %+v", d.Sources)
	}
	if d.Summary.Delete != 1 || d.Summary.Embed != 1 || d.Summary.Unchanged != 0 {
		t.Fatalf("summary drifted: %+v", d.Summary)
	}
}

// A new context grammar moves every embedding_text at once: nothing in the
// index can be trusted, and the plan must say so for every chunk.
func TestDiff_ContextGrammarBumpForcesFullReindex(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})
	old := m
	old.ContextPrefixVersion = "structural-context-v0"

	d := Diff(old, m)
	if !d.FullReindex || !d.Stale {
		t.Fatalf("a grammar bump must force a full reindex: %+v", d)
	}
	if len(d.Chunks) != 2 {
		t.Fatalf("every chunk must be scheduled, got %d", len(d.Chunks))
	}
	for _, c := range d.Chunks {
		if c.Action != ActionEmbed || c.Reason != ReasonContextGrammarChanged {
			t.Fatalf("expected embed/context_grammar_changed for %s, got %+v", c.ChunkID, c)
		}
	}
}

func TestDiff_SchemaBumpForcesFullReindex(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})
	old := m
	old.RecordSchemaVersion = "nomos-rag-chunk-v0"

	d := Diff(old, m)
	c, ok := chunkPlan(d, "chunk:SRC-1:100-220")
	if !d.FullReindex || !ok || c.Reason != ReasonSchemaChanged {
		t.Fatalf("a record schema bump must force a full reindex, got %+v", d)
	}
}

// A manifest whose digest is not the one its own chunk list produces was
// edited by hand. It must not be able to vouch for an index — even when the
// edit would make the index look fresh.
func TestDiff_TamperedOldManifestCannotVouchForFreshness(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})
	tampered := m
	tampered.Chunks = append([]ManifestChunk(nil), m.Chunks...)
	tampered.Chunks[0].EmbeddingHash = "sha256:" + strings.Repeat("0", 64)

	d := Diff(tampered, m)
	if !d.FullReindex || !d.Stale {
		t.Fatalf("a tampered manifest was accepted as a baseline: %+v", d)
	}
	found := false
	for _, r := range d.FullReindexReasons {
		if strings.Contains(r, "does not match its own chunk list") {
			found = true
		}
	}
	if !found {
		t.Fatalf("digest mismatch not reported: %v", d.FullReindexReasons)
	}
	for _, c := range d.Chunks {
		if c.Reason != ReasonOldDigestMismatch {
			t.Fatalf("expected every chunk to carry %s, got %+v", ReasonOldDigestMismatch, c)
		}
	}
}

// A manifest without chunk fingerprints cannot prove chunk-level freshness:
// fail closed with a full reindex rather than guess.
func TestDiff_OldManifestWithoutFingerprintsForcesFullReindex(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})
	legacy := m
	legacy.Chunks = nil

	d := Diff(legacy, m)
	if !d.FullReindex || len(d.Chunks) != 2 {
		t.Fatalf("expected a full reindex over 2 chunks, got %+v", d)
	}
	for _, c := range d.Chunks {
		if c.Action != ActionEmbed || c.Reason != ReasonNoChunkFingerprints {
			t.Fatalf("expected embed/%s, got %+v", ReasonNoChunkFingerprints, c)
		}
	}
}

func TestDiff_PlanIsDeterministicAndSorted(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})
	edited := sampleChunk()
	edited.ChunkText = "Titre I/Chapitre 3\n\nLe delai ne court pas."
	after := manifestOf(t, []corpus.ChunkMetadata{secondChunk(), edited})

	first, err := json.Marshal(Diff(before, after))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(Diff(before, after))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("two diffs of the same manifests differ: a CI gate could not diff plans")
	}
	d := Diff(before, after)
	ids := make([]string, 0, len(d.Chunks))
	for _, c := range d.Chunks {
		ids = append(ids, c.ChunkID)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("plan is not sorted by chunk_id: %v", ids)
	}
}

// The digest must be recomputable from the manifest alone, or the tamper
// check above would be impossible for a consumer to replay.
func TestManifest_DigestIsRecomputableFromItsOwnFingerprints(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), secondChunk()})
	if got := digestOfFingerprints(m.Chunks); got != m.ChunkDigest {
		t.Fatalf("digest recomputed from fingerprints %s != manifest digest %s", got, m.ChunkDigest)
	}
	if len(m.Chunks) != 2 {
		t.Fatalf("expected 2 fingerprints, got %d", len(m.Chunks))
	}
	if m.Chunks[0].EmbeddingHash == m.Chunks[0].BodyHash {
		t.Fatal("embedding and body hashes coincide: the context prefix is not part of embedding_text")
	}
	if !strings.HasPrefix(m.Chunks[0].EmbeddingHash, "sha256:") || !strings.HasPrefix(m.Chunks[0].BodyHash, "sha256:") {
		t.Fatalf("hashes must be prefixed with their algorithm: %+v", m.Chunks[0])
	}
}
