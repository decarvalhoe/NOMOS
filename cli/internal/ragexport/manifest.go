package ragexport

import (
	"sort"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// Manifest is the fingerprint of what was handed to an index. It exists so a
// consumer can answer one question with evidence rather than trust: "is my
// index still the corpus Nomos exported?"
//
// It carries no wall clock of its own — FeedGeneratedAt comes from the feed —
// so re-running `rag manifest` on an unchanged feed produces identical bytes.
type Manifest struct {
	SchemaVersion        string           `json:"schema_version"`
	RecordSchemaVersion  string           `json:"record_schema_version"`
	ContextPrefixVersion string           `json:"context_prefix_version"`
	Format               string           `json:"format"`
	FeedFormat           string           `json:"feed_format,omitempty"`
	FeedContentHash      string           `json:"feed_content_hash,omitempty"`
	FeedGeneratedAt      string           `json:"feed_generated_at,omitempty"`
	ChunkCount           int              `json:"chunk_count"`
	RejectedCount        int              `json:"rejected_count"`
	ChunkDigest          string           `json:"chunk_digest"`
	Sources              []ManifestSource `json:"sources"`
	Chunks               []ManifestChunk  `json:"chunks"`
}

// ManifestSource binds a source to the chunks that entered the index from it,
// and to the hash those chunks were built against. A source whose hash moved
// invalidates exactly its own chunks — not the whole index.
type ManifestSource struct {
	SourceID    string `json:"source_id"`
	SourceHash  string `json:"source_hash"`
	ChunkCount  int    `json:"chunk_count"`
	ChunkDigest string `json:"chunk_digest"`
}

// ManifestChunk is the per-chunk fingerprint: what a consumer needs to decide,
// chunk by chunk and without the export at hand, what to re-embed (embedding
// text moved), what to re-cite (body moved), what merely needs its provenance
// refreshed (source hash moved, text identical) and what to delete.
type ManifestChunk struct {
	ChunkID       string `json:"chunk_id"`
	SourceID      string `json:"source_id"`
	SourceHash    string `json:"source_hash"`
	EmbeddingHash string `json:"embedding_hash"`
	BodyHash      string `json:"body_hash"`
}

// fingerprintOf reduces a record to its manifest fingerprint.
func fingerprintOf(rec Record) ManifestChunk {
	return ManifestChunk{
		ChunkID:       rec.ChunkID,
		SourceID:      rec.Provenance.SourceID,
		SourceHash:    rec.Provenance.SourceHash,
		EmbeddingHash: "sha256:" + hashText(rec.EmbeddingText),
		BodyHash:      "sha256:" + hashText(rec.BodyText),
	}
}

// BuildManifest fingerprints an export result against the feed it came from.
func BuildManifest(feed *corpus.Feed, result Result, format Format) Manifest {
	m := Manifest{
		SchemaVersion:        ManifestSchemaVersion,
		RecordSchemaVersion:  RecordSchemaVersion,
		ContextPrefixVersion: ContextPrefixVersion,
		Format:               string(format),
		ChunkCount:           len(result.Records),
		RejectedCount:        len(result.Rejections),
		ChunkDigest:          digestOf(result.Records),
		Sources:              []ManifestSource{},
		Chunks:               make([]ManifestChunk, 0, len(result.Records)),
	}
	if feed != nil {
		m.FeedFormat = feed.Format
		m.FeedContentHash = feed.ContentHash
		m.FeedGeneratedAt = feed.GeneratedAt
	}
	// Records are already sorted by chunk_id, so the fingerprint list is too.
	for _, rec := range result.Records {
		m.Chunks = append(m.Chunks, fingerprintOf(rec))
	}

	// Group by source so a per-source digest can be compared independently.
	bySource := map[string][]Record{}
	hashBySource := map[string]string{}
	for _, rec := range result.Records {
		id := rec.Provenance.SourceID
		bySource[id] = append(bySource[id], rec)
		// Records are already sorted by chunk_id; the first hash seen for a
		// source is the deterministic representative.
		if _, seen := hashBySource[id]; !seen {
			hashBySource[id] = rec.Provenance.SourceHash
		}
	}
	ids := make([]string, 0, len(bySource))
	for id := range bySource {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		m.Sources = append(m.Sources, ManifestSource{
			SourceID:    id,
			SourceHash:  hashBySource[id],
			ChunkCount:  len(bySource[id]),
			ChunkDigest: digestOf(bySource[id]),
		})
	}
	return m
}

// SourceHashes projects the manifest into a source_id → hash lookup, the shape
// a staleness check needs.
func (m Manifest) SourceHashes() map[string]string {
	out := make(map[string]string, len(m.Sources))
	for _, s := range m.Sources {
		if id := strings.TrimSpace(s.SourceID); id != "" {
			out[id] = s.SourceHash
		}
	}
	return out
}
