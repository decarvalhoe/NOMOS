package ragexport

import (
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

func manifestOf(t *testing.T, chunks []corpus.ChunkMetadata) Manifest {
	t.Helper()
	feed := &corpus.Feed{
		Format:      "nomos-corpus-feed",
		GeneratedAt: "2026-06-01T00:00:00Z",
		ContentHash: "sha256:feed",
	}
	return BuildManifest(feed, Build(chunks), FormatJSONL)
}

func TestManifest_IsDeterministicAndCarriesNoWallClock(t *testing.T) {
	chunks := []corpus.ChunkMetadata{sampleChunk()}
	first := manifestOf(t, chunks)
	second := manifestOf(t, chunks)

	if first.ChunkDigest != second.ChunkDigest {
		t.Fatal("manifest digest is not reproducible")
	}
	if first.FeedGeneratedAt != "2026-06-01T00:00:00Z" {
		t.Fatalf("manifest must inherit the feed timestamp, got %q", first.FeedGeneratedAt)
	}
	if first.ContextPrefixVersion != ContextPrefixVersion {
		t.Fatal("manifest must record the context grammar an index was built with")
	}
}

// The whole point of the manifest: a source edit must be detectable without
// re-reading the corpus.
func TestManifest_DigestMovesWhenBodyChanges(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})

	edited := sampleChunk()
	edited.ChunkText = "Titre I/Chapitre 3\n\nLe delai ne court pas."
	after := manifestOf(t, []corpus.ChunkMetadata{edited})

	if before.ChunkDigest == after.ChunkDigest {
		t.Fatal("digest survived a body edit: a stale index would be indistinguishable from a fresh one")
	}
}

func TestManifest_DigestMovesWhenSourceHashChanges(t *testing.T) {
	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})

	rehashed := sampleChunk()
	rehashed.SourceHash = "sha256:cccc"
	after := manifestOf(t, []corpus.ChunkMetadata{rehashed})

	if before.ChunkDigest == after.ChunkDigest {
		t.Fatal("digest survived a source rehash")
	}
}

// A changed source must invalidate its own chunks and nothing else, or every
// edit would force a full re-embed of the corpus.
func TestManifest_PerSourceDigestIsolatesTheChangedSource(t *testing.T) {
	other := sampleChunk()
	other.ChunkID = "chunk:SRC-2:000-050"
	other.SourceID = "SRC-2"
	other.SourceHash = "sha256:dddd"
	other.ChunkText = "Titre I/Chapitre 3\n\nTexte de la seconde source."

	before := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), other})

	editedOther := other
	editedOther.SourceHash = "sha256:eeee"
	after := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), editedOther})

	beforeBySource := map[string]string{}
	for _, s := range before.Sources {
		beforeBySource[s.SourceID] = s.ChunkDigest
	}
	afterBySource := map[string]string{}
	for _, s := range after.Sources {
		afterBySource[s.SourceID] = s.ChunkDigest
	}

	if beforeBySource["SRC-1"] != afterBySource["SRC-1"] {
		t.Fatal("an untouched source moved: every edit would force a full re-embed")
	}
	if beforeBySource["SRC-2"] == afterBySource["SRC-2"] {
		t.Fatal("the edited source did not move: its stale chunks would stay indexed")
	}
}

func TestManifest_CountsRejectionsRatherThanHidingThem(t *testing.T) {
	broken := sampleChunk()
	broken.ChunkID = "chunk:SRC-1:900-999"
	broken.SourceHash = ""

	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk(), broken})
	if m.ChunkCount != 1 {
		t.Fatalf("expected 1 indexed chunk, got %d", m.ChunkCount)
	}
	if m.RejectedCount != 1 {
		t.Fatalf("expected the rejection to be counted, got %d", m.RejectedCount)
	}
}

func TestManifest_SourceHashesProjection(t *testing.T) {
	m := manifestOf(t, []corpus.ChunkMetadata{sampleChunk()})
	hashes := m.SourceHashes()
	if hashes["SRC-1"] != "sha256:aaaa" {
		t.Fatalf("expected source hash lookup, got %+v", hashes)
	}
}
