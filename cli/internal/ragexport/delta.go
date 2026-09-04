package ragexport

import (
	"fmt"
	"sort"
)

// DeltaSchemaVersion identifies the reindexing-plan contract.
const DeltaSchemaVersion = "nomos-rag-index-delta-v1"

// Actions a consumer must take on a chunk. They are the whole vocabulary a
// re-indexer needs; Nomos never performs them.
const (
	// ActionEmbed: (re-)embed and (re-)index embedding_text.
	ActionEmbed = "embed"
	// ActionUpdateMetadata: the indexed text is unchanged; only the stored
	// provenance (source_hash) must be refreshed so citations re-prove.
	ActionUpdateMetadata = "update_metadata"
	// ActionDelete: the chunk no longer exists in the corpus.
	ActionDelete = "delete"
)

// Reasons behind an action. Stable strings: consumers may branch on them.
const (
	ReasonAdded                 = "added"
	ReasonRemoved               = "removed"
	ReasonBodyChanged           = "body_changed"
	ReasonContextChanged        = "context_changed"
	ReasonSourceHashChanged     = "source_hash_changed"
	ReasonContextGrammarChanged = "context_grammar_changed"
	ReasonSchemaChanged         = "schema_changed"
	ReasonLensChanged           = "lens_changed"
	ReasonNoChunkFingerprints   = "old_manifest_has_no_chunk_fingerprints"
	ReasonOldDigestMismatch     = "old_manifest_digest_mismatch"
)

// Source statuses in a delta.
const (
	SourceUnchanged = "unchanged"
	SourceChanged   = "changed"
	SourceAdded     = "added"
	SourceRemoved   = "removed"
)

// ChunkDelta is one actionable chunk.
type ChunkDelta struct {
	ChunkID  string `json:"chunk_id"`
	SourceID string `json:"source_id"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// SourceDelta is the per-source view: which sources moved, and by how much.
type SourceDelta struct {
	SourceID      string `json:"source_id"`
	Status        string `json:"status"`
	OldSourceHash string `json:"old_source_hash,omitempty"`
	NewSourceHash string `json:"new_source_hash,omitempty"`
	OldChunkCount int    `json:"old_chunk_count"`
	NewChunkCount int    `json:"new_chunk_count"`
}

// DeltaSummary is the calculated headcount of the plan.
type DeltaSummary struct {
	Unchanged      int `json:"unchanged"`
	Embed          int `json:"embed"`
	UpdateMetadata int `json:"update_metadata"`
	Delete         int `json:"delete"`
	SourcesChanged int `json:"sources_changed"`
}

// Delta is the reindexing plan between the manifest an index was built from
// and the manifest of the corpus as it is now. It answers, with evidence:
// is the index stale, and if so, exactly which chunks must be embedded again,
// which only need their provenance refreshed, and which must be deleted.
//
// Fail-closed on the inputs themselves: an old manifest whose digest does not
// match its own chunk list (hand-edited), or that carries no chunk
// fingerprints at all, cannot vouch for freshness and forces a full reindex.
type Delta struct {
	SchemaVersion      string   `json:"schema_version"`
	Stale              bool     `json:"stale"`
	FullReindex        bool     `json:"full_reindex"`
	FullReindexReasons []string `json:"full_reindex_reasons"`
	// FullReindexReason is the machine-readable code behind FullReindex
	// (schema_changed, context_grammar_changed, lens_changed, ...).
	FullReindexReason string        `json:"full_reindex_reason,omitempty"`
	OldChunkDigest    string        `json:"old_chunk_digest"`
	NewChunkDigest    string        `json:"new_chunk_digest"`
	Sources           []SourceDelta `json:"sources"`
	Chunks            []ChunkDelta  `json:"chunks"`
	Summary           DeltaSummary  `json:"summary"`
}

func sameLens(a, b *LensBinding) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.ID == b.ID && a.Digest == b.Digest
	}
}

func describeLens(l *LensBinding) string {
	if l == nil {
		return "<unscoped>"
	}
	return fmt.Sprintf("%s (%s)", l.ID, l.Digest)
}

// Diff computes the plan that takes an index built from old to the corpus
// described by current. Unchanged chunks are counted, not listed; the listed
// chunks are exactly the work to do, sorted by chunk_id.
func Diff(old, current Manifest) Delta {
	d := Delta{
		SchemaVersion:      DeltaSchemaVersion,
		FullReindexReasons: []string{},
		OldChunkDigest:     old.ChunkDigest,
		NewChunkDigest:     current.ChunkDigest,
		Sources:            []SourceDelta{},
		Chunks:             []ChunkDelta{},
	}

	// Contract-level changes invalidate every embedding at once.
	fullReason := ""
	if old.RecordSchemaVersion != current.RecordSchemaVersion {
		d.FullReindexReasons = append(d.FullReindexReasons,
			fmt.Sprintf("record schema changed: %q -> %q", old.RecordSchemaVersion, current.RecordSchemaVersion))
		fullReason = ReasonSchemaChanged
	}
	if old.ContextPrefixVersion != current.ContextPrefixVersion {
		d.FullReindexReasons = append(d.FullReindexReasons,
			fmt.Sprintf("context prefix grammar changed: %q -> %q (every embedding_text moved)", old.ContextPrefixVersion, current.ContextPrefixVersion))
		if fullReason == "" {
			fullReason = ReasonContextGrammarChanged
		}
	}
	// A different scope is a different index: chunks that were out of scope
	// are now in (or the reverse), and the consumer's WHERE clause was written
	// for the old lens.
	if !sameLens(old.Lens, current.Lens) {
		d.FullReindexReasons = append(d.FullReindexReasons,
			fmt.Sprintf("retrieval scope changed: lens %s -> %s (the index was built for a different scope)",
				describeLens(old.Lens), describeLens(current.Lens)))
		if fullReason == "" {
			fullReason = ReasonLensChanged
		}
	}
	// An old manifest that cannot vouch for its chunks cannot vouch for
	// freshness either.
	switch {
	case len(old.Chunks) == 0 && old.ChunkCount > 0:
		d.FullReindexReasons = append(d.FullReindexReasons,
			"old manifest carries no chunk fingerprints: chunk-level freshness cannot be proved")
		if fullReason == "" {
			fullReason = ReasonNoChunkFingerprints
		}
	case len(old.Chunks) > 0 && digestOfFingerprints(old.Chunks) != old.ChunkDigest:
		d.FullReindexReasons = append(d.FullReindexReasons,
			"old manifest digest does not match its own chunk list: the manifest was edited, it cannot vouch for the index")
		if fullReason == "" {
			fullReason = ReasonOldDigestMismatch
		}
	}
	d.FullReindex = len(d.FullReindexReasons) > 0
	d.FullReindexReason = fullReason

	oldByID := make(map[string]ManifestChunk, len(old.Chunks))
	for _, c := range old.Chunks {
		oldByID[c.ChunkID] = c
	}
	newByID := make(map[string]ManifestChunk, len(current.Chunks))
	for _, c := range current.Chunks {
		newByID[c.ChunkID] = c
	}
	ids := make([]string, 0, len(oldByID)+len(newByID))
	for id := range oldByID {
		ids = append(ids, id)
	}
	for id := range newByID {
		if _, seen := oldByID[id]; !seen {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	for _, id := range ids {
		o, inOld := oldByID[id]
		n, inNew := newByID[id]
		switch {
		case inNew && !inOld:
			reason := ReasonAdded
			// Under a full reindex every embed carries the one cause: the
			// consumer rebuilds, it does not triage.
			if d.FullReindex && fullReason != "" {
				reason = fullReason
			}
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: n.SourceID, Action: ActionEmbed, Reason: reason})
		case inOld && !inNew:
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: o.SourceID, Action: ActionDelete, Reason: ReasonRemoved})
		case d.FullReindex:
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: n.SourceID, Action: ActionEmbed, Reason: fullReason})
		case o.BodyHash != n.BodyHash:
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: n.SourceID, Action: ActionEmbed, Reason: ReasonBodyChanged})
		case o.EmbeddingHash != n.EmbeddingHash:
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: n.SourceID, Action: ActionEmbed, Reason: ReasonContextChanged})
		case o.SourceHash != n.SourceHash:
			d.Chunks = append(d.Chunks, ChunkDelta{ChunkID: id, SourceID: n.SourceID, Action: ActionUpdateMetadata, Reason: ReasonSourceHashChanged})
		default:
			d.Summary.Unchanged++
		}
	}
	for _, c := range d.Chunks {
		switch c.Action {
		case ActionEmbed:
			d.Summary.Embed++
		case ActionUpdateMetadata:
			d.Summary.UpdateMetadata++
		case ActionDelete:
			d.Summary.Delete++
		}
	}

	// Per-source view.
	oldSrc := make(map[string]ManifestSource, len(old.Sources))
	for _, s := range old.Sources {
		oldSrc[s.SourceID] = s
	}
	newSrc := make(map[string]ManifestSource, len(current.Sources))
	for _, s := range current.Sources {
		newSrc[s.SourceID] = s
	}
	srcIDs := make([]string, 0, len(oldSrc)+len(newSrc))
	for id := range oldSrc {
		srcIDs = append(srcIDs, id)
	}
	for id := range newSrc {
		if _, seen := oldSrc[id]; !seen {
			srcIDs = append(srcIDs, id)
		}
	}
	sort.Strings(srcIDs)
	for _, id := range srcIDs {
		o, inOld := oldSrc[id]
		n, inNew := newSrc[id]
		sd := SourceDelta{SourceID: id, OldSourceHash: o.SourceHash, NewSourceHash: n.SourceHash,
			OldChunkCount: o.ChunkCount, NewChunkCount: n.ChunkCount}
		switch {
		case inNew && !inOld:
			sd.Status = SourceAdded
		case inOld && !inNew:
			sd.Status = SourceRemoved
		case o.SourceHash != n.SourceHash || o.ChunkDigest != n.ChunkDigest:
			sd.Status = SourceChanged
		default:
			sd.Status = SourceUnchanged
		}
		if sd.Status != SourceUnchanged {
			d.Summary.SourcesChanged++
		}
		d.Sources = append(d.Sources, sd)
	}

	d.Stale = d.FullReindex || len(d.Chunks) > 0
	// Safety net: the digests disagree but nothing above explains it. Never
	// report "fresh" on a contradiction.
	if !d.Stale && old.ChunkDigest != current.ChunkDigest {
		d.FullReindexReasons = append(d.FullReindexReasons,
			"chunk digests differ although no chunk-level change was found: refusing to call the index fresh")
		d.FullReindex = true
		d.Stale = true
	}
	return d
}
