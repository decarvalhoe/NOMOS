package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ChunkMetadata carries the metadata required by the RAG ingestion pipeline.
// Every chunk stored in a vector database must carry this envelope so that
// retrieval results are traceable, filterable, and auditable.
//
// SFI-06 (#344): source-backed chunks (built via BuildRAGMetadata) additionally
// carry SourceSegmentID, CanonicalUnitID, SegmentKind, NormalizedTextHash, and
// structured byte/line spans so retrieval consumers can re-prove provenance
// against a canonical_atom SourceSegment. These fields are optional on the
// struct (omitempty) for backward compatibility with the legacy Enrich path.
type ChunkMetadata struct {
	ChunkID          string   `json:"chunk_id"`
	SourceID         string   `json:"source_id"`
	SourcePath       string   `json:"source_path"`
	SourceHash       string   `json:"source_hash"`
	Domain           string   `json:"domain"`
	UnitIDs          []string `json:"unit_ids,omitempty"`
	Locator          string   `json:"locator"`
	Priority         string   `json:"priority"`
	Status           string   `json:"status"`
	Confidence       string   `json:"confidence"`
	SemanticTags     []string `json:"semantic_tags,omitempty"`
	TokenCount       int      `json:"token_count"`
	CharCount        int      `json:"char_count"`
	IngestedAt       string   `json:"ingested_at"`
	IngestionVersion string   `json:"ingestion_version"`

	// SFI-06 (#344) source-segment linkage. Optional; populated by
	// BuildRAGMetadata for canonical_atom-backed chunks.
	SourceSegmentID    string `json:"source_segment_id,omitempty"`
	CanonicalUnitID    string `json:"canonical_unit_id,omitempty"`
	SegmentKind        string `json:"segment_kind,omitempty"`
	NormalizedTextHash string `json:"normalized_text_hash,omitempty"`
	StartByte          int    `json:"start_byte,omitempty"`
	EndByte            int    `json:"end_byte,omitempty"`
	StartLine          int    `json:"start_line,omitempty"`
	EndLine            int    `json:"end_line,omitempty"`
}

// ChunkInput holds the raw data needed to build chunk metadata.
type ChunkInput struct {
	Content    string
	SourceID   string
	SourcePath string
	SourceHash string
	Domain     string
	UnitIDs    []string
	Locator    string
	Priority   string
	Status     string
	Confidence string
	Tags       []string
}

// EnrichConfig controls metadata enrichment parameters.
type EnrichConfig struct {
	IngestionVersion string
	Now              time.Time
	// TokenEstimateRatio is the approximate chars-per-token ratio.
	// Default (0) uses 4, which is a reasonable estimate for English/French text.
	TokenEstimateRatio float64
}

func (c EnrichConfig) tokenRatio() float64 {
	if c.TokenEstimateRatio > 0 {
		return c.TokenEstimateRatio
	}
	return 4.0
}

// Enrich builds ChunkMetadata from a ChunkInput.
func Enrich(input ChunkInput, config EnrichConfig) (ChunkMetadata, error) {
	if input.Content == "" {
		return ChunkMetadata{}, fmt.Errorf("chunk content must not be empty")
	}
	if input.SourceID == "" {
		return ChunkMetadata{}, fmt.Errorf("source_id is required")
	}
	if input.Domain == "" {
		return ChunkMetadata{}, fmt.Errorf("domain is required")
	}

	if err := validateConfidence(input.Confidence); err != nil {
		return ChunkMetadata{}, err
	}
	if err := validatePriority(input.Priority); err != nil {
		return ChunkMetadata{}, err
	}
	if err := validateStatus(input.Status); err != nil {
		return ChunkMetadata{}, err
	}

	charCount := utf8.RuneCountInString(input.Content)
	tokenCount := estimateTokens(charCount, config.tokenRatio())
	chunkID := computeChunkID(input.SourceID, input.Locator, input.Content)

	now := config.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return ChunkMetadata{
		ChunkID:          chunkID,
		SourceID:         input.SourceID,
		SourcePath:       input.SourcePath,
		SourceHash:       input.SourceHash,
		Domain:           input.Domain,
		UnitIDs:          input.UnitIDs,
		Locator:          input.Locator,
		Priority:         input.Priority,
		Status:           input.Status,
		Confidence:       input.Confidence,
		SemanticTags:     input.Tags,
		TokenCount:       tokenCount,
		CharCount:        charCount,
		IngestedAt:       now.Format(time.RFC3339),
		IngestionVersion: config.IngestionVersion,
	}, nil
}

// estimateTokens provides a rough token count from character count.
func estimateTokens(charCount int, charsPerToken float64) int {
	if charCount == 0 || charsPerToken <= 0 {
		return 0
	}
	return int(float64(charCount)/charsPerToken + 0.5)
}

// computeChunkID produces a deterministic chunk ID from source + locator + content.
func computeChunkID(sourceID, locator, content string) string {
	h := sha256.New()
	h.Write([]byte(sourceID))
	h.Write([]byte{0})
	h.Write([]byte(locator))
	h.Write([]byte{0})
	h.Write([]byte(content))
	digest := hex.EncodeToString(h.Sum(nil))
	return "chunk-" + digest[:16]
}

func validateConfidence(c string) error {
	switch c {
	case "high", "medium", "low":
		return nil
	case "":
		return fmt.Errorf("confidence is required (high, medium, or low)")
	default:
		return fmt.Errorf("invalid confidence %q; expected high, medium, or low", c)
	}
}

func validatePriority(p string) error {
	switch p {
	case "primary", "secondary", "legacy", "derived", "reference":
		return nil
	case "":
		return fmt.Errorf("priority is required")
	default:
		return fmt.Errorf("invalid priority %q", p)
	}
}

func validateStatus(s string) error {
	switch s {
	case "active", "superseded", "duplicate", "out_of_scope", "needs_review", "blocked":
		return nil
	case "":
		return fmt.Errorf("status is required")
	default:
		return fmt.Errorf("invalid status %q", s)
	}
}

// EnrichBatch processes multiple chunks and returns metadata for each.
// It stops on the first error.
func EnrichBatch(inputs []ChunkInput, config EnrichConfig) ([]ChunkMetadata, error) {
	results := make([]ChunkMetadata, 0, len(inputs))
	for i, input := range inputs {
		meta, err := Enrich(input, config)
		if err != nil {
			return nil, fmt.Errorf("chunk[%d]: %w", i, err)
		}
		results = append(results, meta)
	}
	return results, nil
}

// FilterByConfidence returns only chunks matching the given confidence level.
func FilterByConfidence(chunks []ChunkMetadata, confidence string) []ChunkMetadata {
	var out []ChunkMetadata
	for _, c := range chunks {
		if c.Confidence == confidence {
			out = append(out, c)
		}
	}
	return out
}

// FilterByTag returns chunks that have at least one of the given tags.
func FilterByTag(chunks []ChunkMetadata, tags ...string) []ChunkMetadata {
	tagSet := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		tagSet[strings.ToLower(t)] = struct{}{}
	}
	var out []ChunkMetadata
	for _, c := range chunks {
		for _, st := range c.SemanticTags {
			if _, ok := tagSet[strings.ToLower(st)]; ok {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// RAGIngestionVersion identifies the source-backed RAG metadata schema
// produced by BuildRAGMetadata. Bump for future SFI tickets that change
// chunk shape or semantics.
const RAGIngestionVersion = "sfi-06"

// Stable rejection codes emitted by BuildRAGMetadata. Downstream consumers
// (SFI-07 feed quality gate, dashboards) key off these strings, so they
// MUST NOT change without coordination.
const (
	RAGChunkNoSegment           = "RAG_CHUNK_NO_SEGMENT"
	RAGChunkNoUnit              = "RAG_CHUNK_NO_UNIT"
	RAGChunkNonSemanticSource   = "RAG_CHUNK_NON_SEMANTIC_SOURCE"
	RAGChunkUnsupportedBlocking = "RAG_CHUNK_UNSUPPORTED_BLOCKING"
	RAGChunkEmptyText           = "RAG_CHUNK_EMPTY_TEXT"
	RAGChunkMissingSpan         = "RAG_CHUNK_MISSING_SPAN"
)

// RAGRejection is the error type returned by BuildRAGMetadata when a chunk
// is refused. Code is one of the stable RAG_CHUNK_* constants.
type RAGRejection struct {
	Code      string
	UnitID    string
	SegmentID string
	Message   string
}

func (e *RAGRejection) Error() string {
	parts := []string{e.Code + ": " + e.Message}
	if e.UnitID != "" {
		parts = append(parts, "unit_id="+e.UnitID)
	}
	if e.SegmentID != "" {
		parts = append(parts, "segment_id="+e.SegmentID)
	}
	return strings.Join(parts, " ")
}

// RAGBuildInput pairs a source-derived feed unit with the per-chunk fields
// (content, manifest-side classification, optional locator) needed to build
// hardened RAG chunk metadata. The Unit must carry a SourceSegmentID that
// resolves to a canonical_atom SourceSegment in the supplied lookup map.
type RAGBuildInput struct {
	Unit       FeedUnit
	Content    string
	SourceHash string
	Domain     string
	Priority   string
	Status     string
	Confidence string
	Tags       []string
	Locator    string
}

// BuildRAGMetadata builds source-backed RAG chunk metadata. Each input unit
// MUST link back to a canonical_atom SourceSegment via SourceSegmentID. Chunks
// pointing at non-semantic source (layout, separators, structure-only,
// metadata, blank lines, decorative separators, table separators) are rejected
// with stable RAG_CHUNK_* codes. Empty-text chunks, chunks missing source
// linkage, and chunks missing canonical-unit identifiers are likewise
// rejected. The first rejection short-circuits.
//
// Matrix-derived feed units (no source-segment linkage) are NOT supported
// here; use Enrich/EnrichBatch for those.
func BuildRAGMetadata(inputs []RAGBuildInput, segments map[string]SourceSegment, config EnrichConfig) ([]ChunkMetadata, error) {
	out := make([]ChunkMetadata, 0, len(inputs))
	for i, in := range inputs {
		meta, err := buildOneRAGChunk(in, segments, config)
		if err != nil {
			return nil, fmt.Errorf("rag chunk[%d]: %w", i, err)
		}
		out = append(out, meta)
	}
	return out, nil
}

func buildOneRAGChunk(in RAGBuildInput, segments map[string]SourceSegment, config EnrichConfig) (ChunkMetadata, error) {
	u := in.Unit
	segID := strings.TrimSpace(u.SourceSegmentID)
	if segID == "" {
		return ChunkMetadata{}, &RAGRejection{
			Code:    RAGChunkNoSegment,
			UnitID:  u.UnitID,
			Message: "feed unit has no source_segment_id",
		}
	}
	seg, ok := segments[segID]
	if !ok {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkNoSegment,
			UnitID:    u.UnitID,
			SegmentID: segID,
			Message:   "source segment not present in lookup",
		}
	}

	if seg.Disposition == DispositionUnsupportedBlocking {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkUnsupportedBlocking,
			UnitID:    u.UnitID,
			SegmentID: seg.SegmentID,
			Message:   "segment disposition is unsupported_blocking",
		}
	}
	if isNonSemanticRAGSource(seg) {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkNonSemanticSource,
			UnitID:    u.UnitID,
			SegmentID: seg.SegmentID,
			Message:   fmt.Sprintf("segment is non-semantic (kind=%q disposition=%q)", seg.Kind, string(seg.Disposition)),
		}
	}

	content := strings.TrimSpace(in.Content)
	if content == "" {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkEmptyText,
			UnitID:    u.UnitID,
			SegmentID: seg.SegmentID,
			Message:   "chunk content is empty after trim",
		}
	}

	canonicalUnitID := strings.TrimSpace(seg.CanonicalUnitID)
	if canonicalUnitID == "" {
		canonicalUnitID = strings.TrimSpace(seg.SegmentID)
	}
	if canonicalUnitID == "" {
		return ChunkMetadata{}, &RAGRejection{
			Code:    RAGChunkNoUnit,
			UnitID:  u.UnitID,
			Message: "segment has neither canonical_unit_id nor segment_id",
		}
	}

	if seg.EndByte <= seg.StartByte {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkMissingSpan,
			UnitID:    u.UnitID,
			SegmentID: seg.SegmentID,
			Message:   fmt.Sprintf("segment has degenerate byte span [%d, %d)", seg.StartByte, seg.EndByte),
		}
	}

	if err := validateConfidence(in.Confidence); err != nil {
		return ChunkMetadata{}, err
	}
	if err := validatePriority(in.Priority); err != nil {
		return ChunkMetadata{}, err
	}
	if err := validateStatus(in.Status); err != nil {
		return ChunkMetadata{}, err
	}

	domain := strings.TrimSpace(in.Domain)
	if domain == "" {
		domain = strings.TrimSpace(u.Domain)
	}
	if domain == "" {
		return ChunkMetadata{}, fmt.Errorf("domain is required")
	}

	sourceID := firstNonEmptyTrim(u.SourceID, in.Unit.SourceID)
	sourcePath := strings.TrimSpace(u.SourcePath)
	if sourceID == "" {
		return ChunkMetadata{}, fmt.Errorf("source_id is required")
	}

	locator := strings.TrimSpace(in.Locator)
	if locator == "" {
		locator = fmt.Sprintf("%s:L%d-L%d", sourcePath, seg.StartLine, seg.EndLine)
	}
	chunkID := fmt.Sprintf("chunk:%s:%d-%d", sourceID, seg.StartByte, seg.EndByte)

	charCount := utf8.RuneCountInString(content)
	tokenCount := estimateTokens(charCount, config.tokenRatio())

	now := config.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	version := strings.TrimSpace(config.IngestionVersion)
	if version == "" {
		version = RAGIngestionVersion
	}

	unitIDs := []string{}
	if uid := strings.TrimSpace(u.UnitID); uid != "" {
		unitIDs = append(unitIDs, uid)
	}
	if canonicalUnitID != "" && (len(unitIDs) == 0 || unitIDs[0] != canonicalUnitID) {
		unitIDs = append(unitIDs, canonicalUnitID)
	}

	tags := append([]string{}, in.Tags...)
	for _, h := range u.HeadingPath {
		if h := strings.TrimSpace(h); h != "" {
			tags = append(tags, h)
		}
	}
	if len(tags) == 0 {
		tags = nil
	}

	return ChunkMetadata{
		ChunkID:            chunkID,
		SourceID:           sourceID,
		SourcePath:         sourcePath,
		SourceHash:         in.SourceHash,
		Domain:             domain,
		UnitIDs:            unitIDs,
		Locator:            locator,
		Priority:           in.Priority,
		Status:             in.Status,
		Confidence:         in.Confidence,
		SemanticTags:       tags,
		TokenCount:         tokenCount,
		CharCount:          charCount,
		IngestedAt:         now.Format(time.RFC3339),
		IngestionVersion:   version,
		SourceSegmentID:    seg.SegmentID,
		CanonicalUnitID:    canonicalUnitID,
		SegmentKind:        seg.Kind,
		NormalizedTextHash: seg.NormalizedTextHash,
		StartByte:          seg.StartByte,
		EndByte:            seg.EndByte,
		StartLine:          seg.StartLine,
		EndLine:            seg.EndLine,
	}, nil
}

// isNonSemanticRAGSource is true when the segment must not back a RAG chunk
// because its disposition or kind makes it layout/structure/metadata, not
// retrievable text.
func isNonSemanticRAGSource(seg SourceSegment) bool {
	switch string(seg.Disposition) {
	case string(DispositionCoverageOnly),
		string(DispositionStructureOnly),
		string(DispositionExcludedByPolicy),
		"metadata":
		return true
	}
	switch seg.Kind {
	case KindBlank, KindDecorativeSeparator, KindTableSeparator, KindMetadata:
		return true
	}
	return false
}

func firstNonEmptyTrim(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
