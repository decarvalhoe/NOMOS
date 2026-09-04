// Package ragexport projects governed corpus chunks into the record shape a
// RAG stack actually consumes, without Nomos becoming a RAG framework: Nomos
// does not embed, retrieve, or rerank. It emits a provable corpus and verifies
// what comes back through the cite-or-abstain gate (cli/internal/answer).
//
// Two properties the export contract is built around:
//
//   - Deterministic structural context. Published retrieval work (Anthropic,
//     "Contextual Retrieval") situates each chunk in its document before
//     embedding and before lexical indexing, via a per-chunk LLM call. Nomos
//     already parsed that structure — heading path, table identity, column
//     labels, YAML key path, source role — so the same situating text is
//     derived from the parse, with no model in the loop. That makes it
//     reproducible and attestable. Whether it moves retrieval numbers on a
//     given corpus is an open measurement, not a claim: the eval harness is
//     what would establish it.
//
//   - Byte-determinism. Exporting the same feed twice yields the same bytes.
//     No wall clock is read here; every timestamp in a record comes from the
//     feed that produced it. A CI gate can therefore diff an export.
package ragexport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

const (
	// RecordSchemaVersion identifies the neutral chunk record contract.
	RecordSchemaVersion = "nomos-rag-chunk-v1"

	// ManifestSchemaVersion identifies the index manifest contract.
	ManifestSchemaVersion = "nomos-rag-index-manifest-v1"

	// ContextPrefixVersion identifies the context-prefix grammar. Any change to
	// the grammar MUST bump this: consumers key their re-embedding decision on
	// it, and the index manifest records which grammar an index was built with.
	ContextPrefixVersion = "structural-context-v1"

	// contextFieldSep separates context groups inside the prefix.
	contextFieldSep = " · "
	// contextHeadingSep separates heading levels inside the prefix.
	contextHeadingSep = " › "
	// contextBodySep separates the context prefix from the chunk body.
	contextBodySep = "\n\n"
)

// Format is a supported output projection.
type Format string

const (
	// FormatJSONL is the neutral Nomos record, one JSON object per line.
	FormatJSONL Format = "jsonl"
	// FormatLangChain projects onto {page_content, metadata}.
	FormatLangChain Format = "langchain"
	// FormatLlamaIndex projects onto {id_, text, metadata}.
	FormatLlamaIndex Format = "llamaindex"
)

// ParseFormat resolves a CLI format string, listing the alternatives on error.
func ParseFormat(s string) (Format, error) {
	switch Format(strings.TrimSpace(strings.ToLower(s))) {
	case FormatJSONL, "":
		return FormatJSONL, nil
	case FormatLangChain:
		return FormatLangChain, nil
	case FormatLlamaIndex:
		return FormatLlamaIndex, nil
	default:
		return "", fmt.Errorf("unknown format %q (try: jsonl, langchain, llamaindex)", s)
	}
}

// Provenance is the subset a consumer needs to re-prove a retrieved chunk
// against the source, without carrying the whole metadata envelope.
type Provenance struct {
	SourceID           string   `json:"source_id"`
	SourcePath         string   `json:"source_path,omitempty"`
	SourceHash         string   `json:"source_hash"`
	NormalizedTextHash string   `json:"normalized_text_hash,omitempty"`
	SourceSegmentIDs   []string `json:"source_segment_ids,omitempty"`
	CanonicalUnitID    string   `json:"canonical_unit_id,omitempty"`
	StartByte          int      `json:"start_byte,omitempty"`
	EndByte            int      `json:"end_byte,omitempty"`
	StartLine          int      `json:"start_line,omitempty"`
	EndLine            int      `json:"end_line,omitempty"`
}

// Record is one neutral, framework-agnostic RAG chunk record.
//
// EmbeddingText is what a consumer embeds AND what it feeds a lexical (BM25)
// index — contextual embeddings and contextual BM25 are the same text. BodyText
// is what a consumer displays and cites: the context prefix is a retrieval aid,
// never part of the source, and must not leak into a citation.
type Record struct {
	SchemaVersion        string     `json:"schema_version"`
	ChunkID              string     `json:"chunk_id"`
	EmbeddingText        string     `json:"embedding_text"`
	BodyText             string     `json:"body_text"`
	ContextPrefix        string     `json:"context_prefix,omitempty"`
	ContextPrefixVersion string     `json:"context_prefix_version"`
	Provenance           Provenance `json:"provenance"`
	Metadata             Metadata   `json:"metadata"`
}

// Metadata is the filterable envelope. Field names mirror ChunkMetadata so a
// vector store's metadata filters read the same as the corpus vocabulary.
type Metadata struct {
	Domain              string   `json:"domain,omitempty"`
	DomainTags          []string `json:"domain_tags,omitempty"`
	SemanticTags        []string `json:"semantic_tags,omitempty"`
	UnitIDs             []string `json:"unit_ids,omitempty"`
	Locator             string   `json:"locator,omitempty"`
	Priority            string   `json:"priority,omitempty"`
	Status              string   `json:"status,omitempty"`
	Confidence          string   `json:"confidence,omitempty"`
	SegmentKind         string   `json:"segment_kind,omitempty"`
	SourceRole          string   `json:"source_role,omitempty"`
	HeadingPath         []string `json:"heading_path,omitempty"`
	TableID             string   `json:"table_id,omitempty"`
	RowIndex            int      `json:"row_index,omitempty"`
	ColumnHeaders       []string `json:"column_headers,omitempty"`
	YAMLPath            string   `json:"yaml_path,omitempty"`
	CompositionStrategy string   `json:"chunk_composition_strategy,omitempty"`
	TokenCount          int      `json:"token_count,omitempty"`
	CharCount           int      `json:"char_count,omitempty"`
	IngestedAt          string   `json:"ingested_at,omitempty"`
	IngestionVersion    string   `json:"ingestion_version,omitempty"`
}

// Rejection is one chunk refused by the export contract, with the reason. The
// export is fail-closed: a chunk that cannot be re-proved against its source is
// never handed to an index, because a retrieval hit on it could not be cited.
type Rejection struct {
	ChunkID string `json:"chunk_id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Rejection codes.
const (
	RejectMissingChunkID    = "missing_chunk_id"
	RejectMissingSourceID   = "missing_source_id"
	RejectMissingSourceHash = "missing_source_hash"
	RejectEmptyBody         = "empty_body"
)

// Result carries the exported records plus everything refused, so the caller
// reports a calculated count rather than assuming the export was total.
type Result struct {
	Records    []Record    `json:"records"`
	Rejections []Rejection `json:"rejections"`
}

// BuildContextPrefix derives the deterministic situating text for a chunk from
// structure Nomos already parsed. Grammar (ContextPrefixVersion), groups joined
// by " · ", each emitted only when non-empty, always in this order:
//
//	<source_id> · <source_role> · <domain> · <h1> › <h2> › … · table <id> row <n> · columns <a>, <b> · yaml <path>
//
// The order is fixed so the output is stable across runs and comparable across
// corpora; a consumer that re-derives it gets the same string.
func BuildContextPrefix(m corpus.ChunkMetadata) string {
	var groups []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			groups = append(groups, s)
		}
	}

	add(m.SourceID)
	add(m.ContextSourceRole)
	add(m.Domain)

	if headings := filterEmpty(m.ContextHeadingPath); len(headings) > 0 {
		add(strings.Join(headings, contextHeadingSep))
	}

	if table := strings.TrimSpace(m.ContextTableID); table != "" {
		part := "table " + table
		// Row 0 is the composer's zero value, indistinguishable from "no row",
		// so it is not rendered: a wrong row number is worse than none.
		if m.ContextRowIndex > 0 {
			part += " row " + strconv.Itoa(m.ContextRowIndex)
		}
		add(part)
	}
	if cols := filterEmpty(m.ContextColumnHeaders); len(cols) > 0 {
		add("columns " + strings.Join(cols, ", "))
	}
	if yamlPath := strings.TrimSpace(m.ContextYAMLPath); yamlPath != "" {
		add("yaml " + yamlPath)
	}

	return strings.Join(groups, contextFieldSep)
}

// bodyOf recovers the chunk body. ComposeRAGChunks stores heading + separator +
// body in ChunkText; the heading is re-derived here rather than trusted, so a
// body that itself contains the separator is not truncated.
func bodyOf(m corpus.ChunkMetadata) string {
	text := m.ChunkText
	headings := filterEmpty(m.ContextHeadingPath)
	if len(headings) == 0 {
		return text
	}
	prefix := strings.Join(headings, "/") + contextBodySep
	return strings.TrimPrefix(text, prefix)
}

// Build projects composed chunk metadata into neutral records, refusing any
// chunk that could not be cited if it were retrieved.
func Build(chunks []corpus.ChunkMetadata) Result {
	result := Result{Records: []Record{}, Rejections: []Rejection{}}
	for _, m := range chunks {
		body := strings.TrimSpace(bodyOf(m))
		switch {
		case strings.TrimSpace(m.ChunkID) == "":
			result.Rejections = append(result.Rejections, Rejection{
				ChunkID: m.ChunkID, Code: RejectMissingChunkID,
				Message: "chunk has no chunk_id: it could not be addressed in an index",
			})
			continue
		case strings.TrimSpace(m.SourceID) == "":
			result.Rejections = append(result.Rejections, Rejection{
				ChunkID: m.ChunkID, Code: RejectMissingSourceID,
				Message: "chunk has no source_id: a retrieval hit could not be attributed",
			})
			continue
		case strings.TrimSpace(m.SourceHash) == "":
			result.Rejections = append(result.Rejections, Rejection{
				ChunkID: m.ChunkID, Code: RejectMissingSourceHash,
				Message: "chunk has no source_hash: staleness against the source could not be proved",
			})
			continue
		case body == "":
			result.Rejections = append(result.Rejections, Rejection{
				ChunkID: m.ChunkID, Code: RejectEmptyBody,
				Message: "chunk body is empty: nothing to embed or cite",
			})
			continue
		}

		prefix := BuildContextPrefix(m)
		embedding := body
		if prefix != "" {
			embedding = prefix + contextBodySep + body
		}

		result.Records = append(result.Records, Record{
			SchemaVersion:        RecordSchemaVersion,
			ChunkID:              m.ChunkID,
			EmbeddingText:        embedding,
			BodyText:             body,
			ContextPrefix:        prefix,
			ContextPrefixVersion: ContextPrefixVersion,
			Provenance: Provenance{
				SourceID:           m.SourceID,
				SourcePath:         m.SourcePath,
				SourceHash:         m.SourceHash,
				NormalizedTextHash: m.NormalizedTextHash,
				SourceSegmentIDs:   dedupeSegmentIDs(m),
				CanonicalUnitID:    m.CanonicalUnitID,
				StartByte:          m.StartByte,
				EndByte:            m.EndByte,
				StartLine:          m.StartLine,
				EndLine:            m.EndLine,
			},
			Metadata: Metadata{
				Domain:              m.Domain,
				DomainTags:          filterEmpty(m.DomainTags),
				SemanticTags:        filterEmpty(m.SemanticTags),
				UnitIDs:             filterEmpty(m.UnitIDs),
				Locator:             m.Locator,
				Priority:            m.Priority,
				Status:              m.Status,
				Confidence:          m.Confidence,
				SegmentKind:         m.SegmentKind,
				SourceRole:          m.ContextSourceRole,
				HeadingPath:         filterEmpty(m.ContextHeadingPath),
				TableID:             m.ContextTableID,
				RowIndex:            m.ContextRowIndex,
				ColumnHeaders:       filterEmpty(m.ContextColumnHeaders),
				YAMLPath:            m.ContextYAMLPath,
				CompositionStrategy: m.ChunkCompositionStrategy,
				TokenCount:          m.TokenCount,
				CharCount:           m.CharCount,
				IngestedAt:          m.IngestedAt,
				IngestionVersion:    m.IngestionVersion,
			},
		})
	}

	// Stable order by chunk_id: the export must not inherit feed ordering, or
	// re-running the composer on reordered units would churn the whole index.
	sort.SliceStable(result.Records, func(i, j int) bool {
		return result.Records[i].ChunkID < result.Records[j].ChunkID
	})
	sort.SliceStable(result.Rejections, func(i, j int) bool {
		return result.Rejections[i].ChunkID < result.Rejections[j].ChunkID
	})
	return result
}

// Encode writes records in the requested projection, one JSON object per line.
func Encode(records []Record, format Format) ([]byte, error) {
	var buf strings.Builder
	for _, rec := range records {
		var payload any
		switch format {
		case FormatJSONL:
			payload = rec
		case FormatLangChain:
			payload = map[string]any{
				"page_content": rec.EmbeddingText,
				"metadata":     langchainMetadata(rec),
			}
		case FormatLlamaIndex:
			payload = map[string]any{
				"id_":      rec.ChunkID,
				"text":     rec.EmbeddingText,
				"metadata": langchainMetadata(rec),
			}
		default:
			return nil, fmt.Errorf("unknown format %q", format)
		}
		line, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode chunk %s: %w", rec.ChunkID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return []byte(buf.String()), nil
}

// langchainMetadata flattens the record for stores that accept a single
// metadata map. Provenance is kept flat and prefix-free so metadata filters
// stay writable by hand (`source_id = "..."`).
func langchainMetadata(rec Record) map[string]any {
	out := map[string]any{
		"schema_version":         rec.SchemaVersion,
		"chunk_id":               rec.ChunkID,
		"body_text":              rec.BodyText,
		"context_prefix_version": rec.ContextPrefixVersion,
		"source_id":              rec.Provenance.SourceID,
		"source_hash":            rec.Provenance.SourceHash,
	}
	putString := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	putStrings := func(k string, v []string) {
		if len(v) > 0 {
			out[k] = v
		}
	}
	putInt := func(k string, v int) {
		if v != 0 {
			out[k] = v
		}
	}

	putString("context_prefix", rec.ContextPrefix)
	putString("source_path", rec.Provenance.SourcePath)
	putString("normalized_text_hash", rec.Provenance.NormalizedTextHash)
	putString("canonical_unit_id", rec.Provenance.CanonicalUnitID)
	putStrings("source_segment_ids", rec.Provenance.SourceSegmentIDs)
	putInt("start_byte", rec.Provenance.StartByte)
	putInt("end_byte", rec.Provenance.EndByte)
	putInt("start_line", rec.Provenance.StartLine)
	putInt("end_line", rec.Provenance.EndLine)

	m := rec.Metadata
	putString("domain", m.Domain)
	putStrings("domain_tags", m.DomainTags)
	putStrings("semantic_tags", m.SemanticTags)
	putStrings("unit_ids", m.UnitIDs)
	putString("locator", m.Locator)
	putString("priority", m.Priority)
	putString("status", m.Status)
	putString("confidence", m.Confidence)
	putString("segment_kind", m.SegmentKind)
	putString("source_role", m.SourceRole)
	putStrings("heading_path", m.HeadingPath)
	putString("table_id", m.TableID)
	putInt("row_index", m.RowIndex)
	putStrings("column_headers", m.ColumnHeaders)
	putString("yaml_path", m.YAMLPath)
	putString("chunk_composition_strategy", m.CompositionStrategy)
	putInt("token_count", m.TokenCount)
	putInt("char_count", m.CharCount)
	putString("ingested_at", m.IngestedAt)
	putString("ingestion_version", m.IngestionVersion)
	return out
}

// dedupeSegmentIDs returns the contributing segments, falling back to the
// single-segment field when the multi-segment list is absent.
func dedupeSegmentIDs(m corpus.ChunkMetadata) []string {
	ids := filterEmpty(m.SourceSegmentIDs)
	if len(ids) == 0 {
		if id := strings.TrimSpace(m.SourceSegmentID); id != "" {
			return []string{id}
		}
		return nil
	}
	return ids
}

func filterEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// digestOf is the stable fingerprint of a record set: chunk identity bound to
// the exact text a consumer indexed. It changes when a body changes, when the
// context grammar changes, and when chunks appear or disappear.
func digestOf(records []Record) string {
	h := sha256.New()
	for _, rec := range records {
		h.Write([]byte(rec.ChunkID))
		h.Write([]byte{0})
		h.Write([]byte(rec.Provenance.SourceHash))
		h.Write([]byte{0})
		h.Write([]byte(hashText(rec.EmbeddingText)))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
