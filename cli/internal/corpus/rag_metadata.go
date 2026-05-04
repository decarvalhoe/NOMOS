package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
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

	// FSQ-07 (#370) context-rich composition. Set by ComposeRAGChunks for
	// retrieval-ready chunks. The legacy SFI-06 BuildRAGMetadata path leaves
	// these empty (omitempty) for backward compatibility. ChunkText is the
	// composed retrieval text (heading prefix + body); SourceSegmentIDs
	// lists every segment that contributed (parent first, children sorted
	// by start_byte) so consumers can re-prove provenance for multi-segment
	// chunks. The Context* fields mirror the FeedUnit-level provenance.
	ChunkCompositionStrategy string   `json:"chunk_composition_strategy,omitempty"`
	SourceSegmentIDs         []string `json:"source_segment_ids,omitempty"`
	ContextHeadingPath       []string `json:"context_heading_path,omitempty"`
	ContextTableID           string   `json:"context_table_id,omitempty"`
	ContextRowIndex          int      `json:"context_row_index,omitempty"`
	ContextColumnHeaders     []string `json:"context_column_headers,omitempty"`
	ContextYAMLPath          string   `json:"context_yaml_path,omitempty"`
	ContextSourceRole        string   `json:"context_source_role,omitempty"`
	DomainTags               []string `json:"domain_tags,omitempty"`
	ChunkText                string   `json:"chunk_text,omitempty"`
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

// ----------------------------------------------------------------------------
// FSQ-07 (#370) — context-rich RAG chunk composer.
//
// ComposeRAGChunks extends the SFI-06 BuildRAGMetadata surface by composing
// retrieval-ready ChunkText that fuses heading scope, table-row column
// labels, YAML key paths, and source-role provenance into a single chunk.
// It reuses the SFI-06 RAGRejection error type for build-time rejections
// and adds three new rejection codes for FSQ-06 threshold/denylist
// violations applied after composition.
//
// The composer is a parallel public API: it does not replace the legacy
// BuildRAGMetadata path. FSQ-08 (#371) wires the strict-gate / build
// pipeline to ComposeRAGChunks.
// ----------------------------------------------------------------------------

// ChunkCompositionStrategy names the composer's per-unit branch.
type ChunkCompositionStrategy string

const (
	ChunkStrategySingleAtom   ChunkCompositionStrategy = "single_atom"
	ChunkStrategyTableRow     ChunkCompositionStrategy = "table_row"
	ChunkStrategyYAMLScalar   ChunkCompositionStrategy = "yaml_scalar"
	ChunkStrategyHeadingGroup ChunkCompositionStrategy = "heading_group"
)

// FSQ-07 (#370) rejection codes. Extends the SFI-06 RAG_CHUNK_* family;
// downstream consumers (FSQ-08 strict gate, dashboards) key off these
// strings, so they MUST NOT change without coordination.
const (
	RAGChunkBelowTokenMin = "RAG_CHUNK_BELOW_TOKEN_MIN"
	RAGChunkBelowCharMin  = "RAG_CHUNK_BELOW_CHAR_MIN"
	RAGChunkStopLabel     = "RAG_CHUNK_STOP_LABEL"
)

// composedChunkSeparator is the visual divider between heading prefix and
// chunk body in composed ChunkText. Middle dot (U+00B7) keeps the chunk
// readable while staying easy to split on for downstream tooling.
const composedChunkSeparator = " · "

// RAGComposeInput is the full set of artifacts ComposeRAGChunks consumes.
// FeedUnits is the only required field. Sources, Segments, BodyLedger, and
// Profile are optional context that improves the chunk metadata quality.
type RAGComposeInput struct {
	FeedUnits  []FeedUnit
	Sources    []FeedSource
	Segments   []SourceSegment
	BodyLedger *CorpusBodyLedger
	Profile    SemanticQualityProfile
	BaseConfig RAGBuildInput
}

// ComposeRAGChunks composes retrieval-ready RAG chunks from feed units. For
// each source-derived unit (i.e. SourceSegmentID set), the composer picks a
// strategy (table_row / yaml_scalar / single_atom), assembles a ChunkText
// that fuses heading path, column labels, and YAML key path, and emits a
// ChunkMetadata with full multi-segment provenance.
//
// Matrix-derived units (no SourceSegmentID) are skipped — they keep going
// through Enrich/EnrichBatch in the legacy path. The first rejection
// short-circuits with a *RAGRejection error.
//
// Output is sorted by FeedUnit.UnitID ASC for determinism (byte-identical
// output across two runs on identical input — important for evidence
// archival).
func ComposeRAGChunks(input RAGComposeInput) ([]ChunkMetadata, error) {
	if len(input.FeedUnits) == 0 {
		return nil, nil
	}

	units := make([]FeedUnit, 0, len(input.FeedUnits))
	for _, u := range input.FeedUnits {
		if strings.TrimSpace(u.SourceSegmentID) == "" {
			continue
		}
		units = append(units, u)
	}
	sort.SliceStable(units, func(i, j int) bool {
		return units[i].UnitID < units[j].UnitID
	})

	segByID := make(map[string]SourceSegment, len(input.Segments))
	for _, seg := range input.Segments {
		segByID[seg.SegmentID] = seg
	}
	srcByID := make(map[string]FeedSource, len(input.Sources))
	for _, src := range input.Sources {
		srcByID[src.ID] = src
	}

	profile := input.Profile
	if isZeroSemanticProfile(profile) {
		profile = DefaultRBOKProfile()
	}
	stopSet := buildStopLabelSet(profile.StopLabelDenylist)

	out := make([]ChunkMetadata, 0, len(units))
	for i, u := range units {
		chunk, err := composeOneRAGChunk(u, segByID, srcByID, profile, stopSet, input.BaseConfig)
		if err != nil {
			return nil, fmt.Errorf("rag compose[%d]: %w", i, err)
		}
		out = append(out, chunk)
	}
	return out, nil
}

func composeOneRAGChunk(
	u FeedUnit,
	segByID map[string]SourceSegment,
	srcByID map[string]FeedSource,
	profile SemanticQualityProfile,
	stopSet map[string]struct{},
	base RAGBuildInput,
) (ChunkMetadata, error) {
	seg, segOK := segByID[u.SourceSegmentID]

	strategy := pickCompositionStrategy(u)
	body := composeBody(u, strategy)
	heading := strings.TrimSpace(strings.Join(filterEmpty(u.HeadingPath), "/"))
	chunkText := body
	if heading != "" {
		chunkText = heading + composedChunkSeparator + body
	}

	// FSQ-06 stop-label denylist. The composer rejects pre-prefix bodies
	// that match a denylist label exactly so heading decoration cannot
	// disguise a junk single-word chunk.
	if isStopLabel(body, stopSet) {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkStopLabel,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			Message:   fmt.Sprintf("composed chunk body %q is a stop-label from the profile denylist", body),
		}
	}

	tokens := tokenCountSemantic(body)
	chars := runeCount(strings.TrimSpace(body))

	kindKey := strategyKindKey(strategy, seg)
	if min := minByKind(profile.MinTokensByKind, kindKey); min > 0 && tokens < min {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkBelowTokenMin,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			Message: fmt.Sprintf(
				"composed chunk has %d token(s); profile requires ≥ %d for kind %q",
				tokens, min, kindKey,
			),
		}
	}
	if min := minByKind(profile.MinCharsByKind, kindKey); min > 0 && chars < min {
		return ChunkMetadata{}, &RAGRejection{
			Code:      RAGChunkBelowCharMin,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			Message: fmt.Sprintf(
				"composed chunk has %d character(s); profile requires ≥ %d for kind %q",
				chars, min, kindKey,
			),
		}
	}

	// Resolve canonical-unit identifier the same way SFI-06 does (segment
	// canonical_unit_id falls back to the segment id itself).
	canonicalUnitID := ""
	if segOK {
		canonicalUnitID = strings.TrimSpace(seg.CanonicalUnitID)
		if canonicalUnitID == "" {
			canonicalUnitID = strings.TrimSpace(seg.SegmentID)
		}
	}
	if canonicalUnitID == "" {
		canonicalUnitID = strings.TrimSpace(u.SourceSegmentID)
	}

	// Provenance and span.
	startByte := u.StartByte
	endByte := u.EndByte
	startLine := u.StartLine
	endLine := u.EndLine
	if segOK {
		if startByte == 0 && endByte == 0 {
			startByte = seg.StartByte
			endByte = seg.EndByte
		}
		if startLine == 0 && endLine == 0 {
			startLine = seg.StartLine
			endLine = seg.EndLine
		}
	}

	sourceID := firstNonEmptyTrim(u.SourceID, base.Unit.SourceID)
	sourcePath := firstNonEmptyTrim(u.SourcePath, base.Unit.SourcePath)
	sourceHash := strings.TrimSpace(base.SourceHash)
	domain := firstNonEmptyTrim(base.Domain, u.Domain)
	role := ""
	if src, ok := srcByID[sourceID]; ok {
		role = strings.TrimSpace(src.SourceRole)
		if sourceHash == "" {
			sourceHash = strings.TrimSpace(src.Hash)
		}
		if domain == "" {
			domain = strings.TrimSpace(src.Domain)
		}
	}

	priority := firstNonEmptyTrim(base.Priority, "primary")
	status := firstNonEmptyTrim(base.Status, "active")
	confidence := firstNonEmptyTrim(base.Confidence, "medium")

	now := time.Now().UTC()
	version := strings.TrimSpace(base.Unit.UnitID) // unused; placeholder
	_ = version
	ingestionVersion := RAGIngestionVersion

	chunkID := fmt.Sprintf("chunk:%s:%d-%d", sourceID, startByte, endByte)
	locator := fmt.Sprintf("%s:L%d-L%d", sourcePath, startLine, endLine)

	unitIDs := []string{}
	if uid := strings.TrimSpace(u.UnitID); uid != "" {
		unitIDs = append(unitIDs, uid)
	}
	if canonicalUnitID != "" && (len(unitIDs) == 0 || unitIDs[0] != canonicalUnitID) {
		unitIDs = append(unitIDs, canonicalUnitID)
	}

	domainTags := composeDomainTags(domain, u.HeadingPath)

	segmentIDs := composeSegmentIDList(u, segByID)
	contextHeading := append([]string(nil), filterEmpty(u.HeadingPath)...)
	if len(contextHeading) == 0 {
		contextHeading = nil
	}
	var contextHeaders []string
	if len(u.ColumnHeaders) > 0 {
		contextHeaders = append([]string(nil), u.ColumnHeaders...)
	}

	return ChunkMetadata{
		ChunkID:                  chunkID,
		SourceID:                 sourceID,
		SourcePath:               sourcePath,
		SourceHash:               sourceHash,
		Domain:                   domain,
		UnitIDs:                  unitIDs,
		Locator:                  locator,
		Priority:                 priority,
		Status:                   status,
		Confidence:               confidence,
		SemanticTags:             domainTags,
		TokenCount:               tokens,
		CharCount:                chars,
		IngestedAt:               now.Format(time.RFC3339),
		IngestionVersion:         ingestionVersion,
		SourceSegmentID:          u.SourceSegmentID,
		CanonicalUnitID:          canonicalUnitID,
		SegmentKind:              seg.Kind,
		NormalizedTextHash:       u.NormalizedTextHash,
		StartByte:                startByte,
		EndByte:                  endByte,
		StartLine:                startLine,
		EndLine:                  endLine,
		ChunkCompositionStrategy: string(strategy),
		SourceSegmentIDs:         segmentIDs,
		ContextHeadingPath:       contextHeading,
		ContextTableID:           u.TableID,
		ContextRowIndex:          u.RowIndex,
		ContextColumnHeaders:     contextHeaders,
		ContextYAMLPath:          u.YAMLPath,
		ContextSourceRole:        role,
		DomainTags:               domainTags,
		ChunkText:                chunkText,
	}, nil
}

// pickCompositionStrategy chooses the composer branch from the FeedUnit's
// FSQ-03/FSQ-04 markers. heading_group is intentionally not picked here —
// see ComposeRAGChunks doc for why.
func pickCompositionStrategy(u FeedUnit) ChunkCompositionStrategy {
	if u.UnitType == string(ChunkStrategyTableRow) || strings.TrimSpace(u.TableID) != "" {
		return ChunkStrategyTableRow
	}
	if strings.TrimSpace(u.YAMLPath) != "" {
		return ChunkStrategyYAMLScalar
	}
	return ChunkStrategySingleAtom
}

// composeBody returns the body text (without heading prefix) for the chosen
// strategy. For table_row, FSQ-03 already populated BusinessRule with
// "Col=Val; ..."; we trust it. For yaml_scalar, we format
// "<YAMLPath> = <DecodedValue>" so the key path is co-located with the
// retrievable text.
func composeBody(u FeedUnit, strategy ChunkCompositionStrategy) string {
	switch strategy {
	case ChunkStrategyTableRow:
		return strings.TrimSpace(u.BusinessRule)
	case ChunkStrategyYAMLScalar:
		path := strings.TrimSpace(u.YAMLPath)
		value := strings.TrimSpace(u.DecodedValue)
		if value == "" {
			value = strings.TrimSpace(u.BusinessRule)
		}
		if path == "" {
			return value
		}
		return path + " = " + value
	default:
		return strings.TrimSpace(u.BusinessRule)
	}
}

// composeSegmentIDList returns the row segment id (when applicable) followed
// by every contributing child segment id sorted by StartByte. For non-table
// strategies this is just [SourceSegmentID]. For table_row strategy we
// prefer FeedUnit.BodyLedgerSegmentIDs (FSQ-05 already computed parent +
// children in stable order); when unavailable we synthesise the list from
// the segments map by looking up children whose ParentSegmentID equals the
// row id.
func composeSegmentIDList(u FeedUnit, segByID map[string]SourceSegment) []string {
	if len(u.BodyLedgerSegmentIDs) > 0 {
		return append([]string(nil), u.BodyLedgerSegmentIDs...)
	}
	if u.UnitType != string(ChunkStrategyTableRow) && strings.TrimSpace(u.TableID) == "" {
		return []string{u.SourceSegmentID}
	}
	rowID := u.SourceSegmentID
	out := []string{rowID}
	type child struct {
		id    string
		start int
	}
	var kids []child
	for _, seg := range segByID {
		if seg.ParentSegmentID != rowID {
			continue
		}
		kids = append(kids, child{id: seg.SegmentID, start: seg.StartByte})
	}
	sort.SliceStable(kids, func(i, j int) bool {
		return kids[i].start < kids[j].start
	})
	for _, k := range kids {
		out = append(out, k.id)
	}
	return out
}

// strategyKindKey returns the SemanticQualityProfile threshold key for a
// composed chunk. table_row uses the literal "table_row" key (matches FSQ-06
// defaults). yaml_scalar uses the unit's NodeKind when set, otherwise
// "default". single_atom prefers the underlying segment Kind so paragraphs
// honour the paragraph minimum, list items the list_item minimum, etc.
func strategyKindKey(strategy ChunkCompositionStrategy, seg SourceSegment) string {
	switch strategy {
	case ChunkStrategyTableRow:
		return "table_row"
	case ChunkStrategyYAMLScalar:
		return profileKindDefault
	default:
		if k := strings.TrimSpace(seg.Kind); k != "" {
			return k
		}
		return profileKindDefault
	}
}

// composeDomainTags assembles the SemanticTags / DomainTags surface. We
// preserve the source domain as the lead tag so retrieval filters can key
// off it directly, then append every non-empty heading path entry.
func composeDomainTags(domain string, headingPath []string) []string {
	var out []string
	if d := strings.TrimSpace(domain); d != "" {
		out = append(out, d)
	}
	for _, h := range headingPath {
		if h := strings.TrimSpace(h); h != "" {
			out = append(out, h)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isStopLabel reports whether body matches a denylist label exactly under
// the same case-folding rules as FSQ-06 (lowercase, trim trailing
// punctuation).
func isStopLabel(body string, stopSet map[string]struct{}) bool {
	if len(stopSet) == 0 {
		return false
	}
	folded := strings.ToLower(strings.TrimSpace(body))
	folded = strings.TrimRight(folded, ".:;,!?")
	_, hit := stopSet[folded]
	return hit
}

// filterEmpty returns the input with whitespace-only entries removed.
func filterEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if strings.TrimSpace(s) == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
