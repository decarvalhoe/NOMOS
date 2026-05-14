package corpus

import (
	"fmt"
	"sort"
	"strings"
)

// Stable finding codes emitted by the SFI-07 feed quality gate. Downstream
// consumers (CLI, dashboards, the SFI-08 strict release gate) key off these
// strings; they MUST NOT change without coordination.
//
// The RAG_CHUNK_* codes intentionally reuse the SFI-06 constants from
// rag_metadata.go (RAGChunkNoUnit / RAGChunkNoSegment / RAGChunkNonSemanticSource)
// so that the build-time rejection codes and the after-the-fact gate findings
// share one source of truth. They already use the exact literal strings
// required by SFI-07, so no aliasing is needed.
const (
	FindingFeedUnitNoSegment = "FEED_UNIT_NO_SEGMENT"
	FindingFeedUnitNoSource  = "FEED_UNIT_NO_SOURCE"
	FindingFeedUnitNoSpan    = "FEED_UNIT_NO_SPAN"
	FindingFeedEmptyText     = "FEED_EMPTY_TEXT"
	FindingFeedJunkText      = "FEED_JUNK_TEXT"
	FindingFeedDuplicateSpan = "FEED_DUPLICATE_SPAN"
)

// FeedQualityInput is the read-only view of the artifacts to be validated.
// All slices may be nil; an empty corpus yields a passing report.
type FeedQualityInput struct {
	FeedUnits []FeedUnit
	Chunks    []ChunkMetadata
	Segments  []SourceSegment
}

// FeedQualityFinding is a single rule violation. The JSON shape is the wire
// format consumed by the SFI-08 release gate and the SFI-09 CUE schema.
type FeedQualityFinding struct {
	Code      string `json:"code"`
	UnitID    string `json:"unit_id,omitempty"`
	ChunkID   string `json:"chunk_id,omitempty"`
	SegmentID string `json:"segment_id,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
	Message   string `json:"message"`
}

// FeedQualityReport summarises a CheckFeedQuality run. Status is "pass" iff
// Findings is empty.
type FeedQualityReport struct {
	Status                 string               `json:"status"`
	FeedUnitCount          int                  `json:"feed_unit_count"`
	SourceDerivedUnitCount int                  `json:"source_derived_unit_count"`
	ChunkCount             int                  `json:"chunk_count"`
	DuplicateSpanCount     int                  `json:"duplicate_span_count"`
	Findings               []FeedQualityFinding `json:"findings"`
}

// CheckFeedQuality validates final feed and RAG artifacts against the
// SourceSegment ledger they claim to be derived from. It is stateless,
// side-effect-free, and complementary to:
//
//   - the SFI-04 source integrity gate (which validates the ledger itself
//     and is a precondition, not a synonym);
//   - the SFI-06 build-time RAG rejection rules in BuildRAGMetadata
//     (which prevent junk chunks at construction time).
//
// SFI-07 is the consumer-facing check on artifacts that have already been
// produced. Matrix-derived feed units (those without source-segment evidence)
// are skipped — only source-derived units are scrutinised.
func CheckFeedQuality(input FeedQualityInput) FeedQualityReport {
	report := FeedQualityReport{
		FeedUnitCount: len(input.FeedUnits),
		ChunkCount:    len(input.Chunks),
		Findings:      []FeedQualityFinding{},
	}

	sourceDerived := make([]FeedUnit, 0, len(input.FeedUnits))
	for _, u := range input.FeedUnits {
		if isSourceDerivedFeedUnit(u) {
			sourceDerived = append(sourceDerived, u)
		}
	}
	report.SourceDerivedUnitCount = len(sourceDerived)

	for _, u := range sourceDerived {
		report.Findings = append(report.Findings, checkFeedUnitShape(u)...)
		report.Findings = append(report.Findings, checkFeedUnitText(u)...)
	}

	report.Findings = append(report.Findings, findDuplicateFeedSpans(sourceDerived, &report)...)

	segByID := make(map[string]SourceSegment, len(input.Segments))
	for _, s := range input.Segments {
		segByID[s.SegmentID] = s
	}
	for _, c := range input.Chunks {
		report.Findings = append(report.Findings, checkOneChunk(c, segByID)...)
	}

	if len(report.Findings) == 0 {
		report.Status = "pass"
	} else {
		report.Status = "fail"
	}
	return report
}

// isSourceDerivedFeedUnit returns true when a FeedUnit carries any of the
// SFI-05 source-segment markers introduced in #343. The discriminator is the
// union of those markers so that *any* missing single field still routes the
// unit through the gate (and triggers the matching FEED_UNIT_NO_* finding).
//
// Matrix-derived units (built from canonical-matrix YAML) carry only
// SourceIDs and never set SourceSegmentID/SourcePath/spans/NormalizedTextHash,
// so they fall on the not-source-derived side and are correctly skipped from
// these checks.
func isSourceDerivedFeedUnit(u FeedUnit) bool {
	return u.SourceSegmentID != "" ||
		u.SourcePath != "" ||
		u.NormalizedTextHash != "" ||
		u.StartByte != 0 ||
		u.EndByte != 0 ||
		len(u.HeadingPath) > 0
}

func checkFeedUnitShape(u FeedUnit) []FeedQualityFinding {
	var out []FeedQualityFinding
	if strings.TrimSpace(u.SourceSegmentID) == "" {
		out = append(out, FeedQualityFinding{
			Code:     FindingFeedUnitNoSegment,
			UnitID:   u.UnitID,
			SourceID: u.SourceID,
			Message:  "source-derived feed unit has empty source_segment_id",
		})
	}
	if strings.TrimSpace(u.SourceSegmentID) != "" && strings.TrimSpace(u.SourceID) == "" {
		out = append(out, FeedQualityFinding{
			Code:      FindingFeedUnitNoSource,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			Message:   "feed unit has source_segment_id but source_id is empty",
		})
	}
	if strings.TrimSpace(u.SourceSegmentID) != "" && u.StartByte == 0 && u.EndByte == 0 {
		out = append(out, FeedQualityFinding{
			Code:      FindingFeedUnitNoSpan,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit has source_segment_id but byte span is missing (start_byte=end_byte=0)",
		})
	}
	return out
}

func checkFeedUnitText(u FeedUnit) []FeedQualityFinding {
	text := u.BusinessRule
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []FeedQualityFinding{{
			Code:      FindingFeedEmptyText,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit business_rule is empty after trim",
		}}
	}
	if isJunkSemantic([]byte(text)) {
		return []FeedQualityFinding{{
			Code:      FindingFeedJunkText,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit business_rule is punctuation/layout-only or matches a markdown table separator",
		}}
	}
	return nil
}

// findDuplicateFeedSpans detects two or more source-derived feed units that
// share the same (SourceID, StartByte, EndByte). Implemented on the feed
// side independently of the SFI-04 ledger-side check (#342) so that even a
// passing ledger cannot smuggle a duplicated artifact past this gate.
func findDuplicateFeedSpans(units []FeedUnit, report *FeedQualityReport) []FeedQualityFinding {
	type spanKey struct {
		src   string
		start int
		end   int
	}
	groups := map[spanKey][]FeedUnit{}
	order := []spanKey{}
	for _, u := range units {
		if strings.TrimSpace(u.SourceSegmentID) == "" {
			continue
		}
		if u.StartByte == 0 && u.EndByte == 0 {
			continue
		}
		k := spanKey{u.SourceID, u.StartByte, u.EndByte}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], u)
	}
	var out []FeedQualityFinding
	for _, k := range order {
		group := groups[k]
		if len(group) < 2 {
			continue
		}
		report.DuplicateSpanCount++
		for i := 1; i < len(group); i++ {
			out = append(out, FeedQualityFinding{
				Code:      FindingFeedDuplicateSpan,
				UnitID:    group[i].UnitID,
				SegmentID: group[i].SourceSegmentID,
				SourceID:  group[i].SourceID,
				Message: fmt.Sprintf(
					"duplicate (source_id=%s, span=[%d,%d)) already claimed by feed unit %q",
					k.src, k.start, k.end, group[0].UnitID,
				),
			})
		}
	}
	return out
}

func checkOneChunk(c ChunkMetadata, segByID map[string]SourceSegment) []FeedQualityFinding {
	var out []FeedQualityFinding

	if strings.TrimSpace(c.CanonicalUnitID) == "" && len(c.UnitIDs) == 0 {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNoUnit,
			ChunkID:   c.ChunkID,
			SegmentID: c.SourceSegmentID,
			SourceID:  c.SourceID,
			Message:   "chunk has no canonical_unit_id and no feed-unit linkage (unit_ids is empty)",
		})
	}

	segID := strings.TrimSpace(c.SourceSegmentID)
	if segID == "" {
		out = append(out, FeedQualityFinding{
			Code:     RAGChunkNoSegment,
			ChunkID:  c.ChunkID,
			SourceID: c.SourceID,
			Message:  "chunk has no source_segment_id",
		})
		return out
	}

	seg, ok := segByID[segID]
	if !ok {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNoSegment,
			ChunkID:   c.ChunkID,
			SegmentID: segID,
			SourceID:  c.SourceID,
			Message:   "chunk source_segment_id does not resolve to any segment in the supplied ledger",
		})
		return out
	}

	if seg.Disposition != DispositionCanonicalAtom || isNonSemanticSegmentKindForRAG(seg.Kind) {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNonSemanticSource,
			ChunkID:   c.ChunkID,
			SegmentID: seg.SegmentID,
			SourceID:  c.SourceID,
			Message: fmt.Sprintf(
				"chunk source segment is non-semantic (kind=%q disposition=%q)",
				seg.Kind, string(seg.Disposition),
			),
		})
	}
	return out
}

// isNonSemanticSegmentKindForRAG matches the explicit kind list in the
// SFI-07 dispatch: blank, decorative_separator, table_separator, metadata,
// and the literal "structure_only" (kept for parity with the dispatch even
// though current scanners use it only as a Disposition).
func isNonSemanticSegmentKindForRAG(kind string) bool {
	switch kind {
	case KindBlank,
		KindDecorativeSeparator,
		KindTableSeparator,
		KindMetadata,
		"structure_only":
		return true
	}
	return false
}

// =============================================================================
// FSQ-06 (#369) — blocking semantic feed quality gate.
//
// CheckSemanticQuality complements (does NOT replace) CheckFeedQuality. The
// SFI-07 gate fails on broken plumbing (missing source linkage, empty text,
// duplicate spans). The FSQ-06 gate fails on technically valid but
// semantically low-value feed: one-token chunks, stop-label cells, duplicate
// labels, table-cell units missing row context, metadata-table leaks, and
// admitted sources that produced zero units without a recorded reason.
// Findings are severity-grouped (blocking / warning / informational) and the
// profile is recorded in the report for evidence reproducibility.
// =============================================================================

// SemanticFindingSeverity classifies the impact of a SemanticQualityFinding.
// Severity tiers are part of the public contract: the strict release gate
// only fails on `blocking`; warnings surface for review without blocking.
type SemanticFindingSeverity string

const (
	SemanticSeverityBlocking      SemanticFindingSeverity = "blocking"
	SemanticSeverityWarning       SemanticFindingSeverity = "warning"
	SemanticSeverityInformational SemanticFindingSeverity = "informational"
)

// Stable finding codes emitted by CheckSemanticQuality. Downstream consumers
// (strict gate, dashboards, FSQ-09 schema) key off these strings; they MUST
// NOT change without coordination.
const (
	FindingFeedUnitBelowTokenMin       = "FEED_UNIT_BELOW_TOKEN_MIN"
	FindingFeedUnitBelowCharMin        = "FEED_UNIT_BELOW_CHAR_MIN"
	FindingFeedStopLabel               = "FEED_STOP_LABEL"
	FindingFeedDuplicateNormalizedText = "FEED_DUPLICATE_NORMALIZED_TEXT"
	FindingFeedTableCellNotRowContext  = "FEED_TABLE_CELL_NOT_ROW_CONTEXT"
	FindingFeedMetadataTableLeaked     = "FEED_METADATA_TABLE_LEAKED"
	FindingFeedRawDecodedMismatch      = "FEED_RAW_DECODED_MISMATCH"
	FindingSourceZeroUnitNoReason      = "SOURCE_ZERO_UNIT_NO_REASON"
	FindingShortCriticalRequiresReview = "SHORT_CRITICAL_REQUIRES_REVIEW"
)

// profileKindDefault is the lookup key the profile uses to express a
// catch-all minimum for kinds not enumerated in MinTokensByKind /
// MinCharsByKind.
const profileKindDefault = "default"

// SemanticQualityProfile is the threshold model for CheckSemanticQuality.
// The struct is JSON-serialisable and used both as the default for RBOK
// (DefaultRBOKProfile) and as the `--corpus-semantic-quality-profile` flag
// payload on the strict gate.
//
// Defaults (RBOK):
//   - MinTokensByKind: paragraph=3, list_item=2, callout=3, table_row=2, default=1
//   - MinCharsByKind:  paragraph=20, list_item=12, callout=20, table_row=12, default=4
//   - StopLabelDenylist (case-folded): champ, valeur, statut, status, version,
//     field, value, key, name, date, id
//   - DuplicateThreshold: 3 (≥ N occurrences → blocking)
//   - DuplicateWarningThreshold: 2 (between this and DuplicateThreshold-1 → warning)
//   - AllowTableCellWithoutRow: false
//   - AllowMetadataTableUnits: false
//   - RawDecodedMismatchSeverity: informational
type SemanticQualityProfile struct {
	MinTokensByKind            map[string]int          `json:"min_tokens_by_kind"`
	MinCharsByKind             map[string]int          `json:"min_chars_by_kind"`
	StopLabelDenylist          []string                `json:"stop_label_denylist"`
	DuplicateThreshold         int                     `json:"duplicate_threshold"`
	DuplicateWarningThreshold  int                     `json:"duplicate_warning_threshold"`
	AllowTableCellWithoutRow   bool                    `json:"allow_table_cell_without_row"`
	AllowMetadataTableUnits    bool                    `json:"allow_metadata_table_units"`
	RawDecodedMismatchSeverity SemanticFindingSeverity `json:"raw_decoded_mismatch_severity"`
}

// DefaultRBOKProfile returns the canonical RBOK profile. The thresholds are
// generic — RBOK does not embed RBOK-specific semantics in this profile;
// callers can swap the profile for any other corpus.
func DefaultRBOKProfile() SemanticQualityProfile {
	return SemanticQualityProfile{
		MinTokensByKind: map[string]int{
			KindParagraph:      3,
			KindListItem:       2,
			KindCallout:        3,
			"table_row":        2,
			profileKindDefault: 1,
		},
		MinCharsByKind: map[string]int{
			KindParagraph:      20,
			KindListItem:       12,
			KindCallout:        20,
			"table_row":        12,
			profileKindDefault: 4,
		},
		StopLabelDenylist: []string{
			"champ", "valeur", "statut", "status", "version",
			"field", "value", "key", "name", "date", "id",
		},
		DuplicateThreshold:         3,
		DuplicateWarningThreshold:  2,
		AllowTableCellWithoutRow:   false,
		AllowMetadataTableUnits:    false,
		RawDecodedMismatchSeverity: SemanticSeverityInformational,
	}
}

// SemanticQualityInput is the full set of artifacts CheckSemanticQuality
// inspects. Every slice / pointer is optional: an empty input yields a
// passing report.
type SemanticQualityInput struct {
	Feed               []FeedUnit
	Chunks             []ChunkMetadata
	Segments           []SourceSegment
	Sources            []FeedSource
	BodyLedger         *CorpusBodyLedger
	ShortCriticalAtoms *ShortCriticalAtomsReport
	Profile            SemanticQualityProfile
}

// SemanticQualityFinding is one rule violation. JSON shape is the wire format
// surfaced by the strict release gate and (eventually) the FSQ-09 CUE schema.
type SemanticQualityFinding struct {
	Code            string                  `json:"code"`
	Severity        SemanticFindingSeverity `json:"severity"`
	UnitID          string                  `json:"unit_id,omitempty"`
	ChunkID         string                  `json:"chunk_id,omitempty"`
	SourceID        string                  `json:"source_id,omitempty"`
	SourcePath      string                  `json:"source_path,omitempty"`
	StartLine       int                     `json:"start_line,omitempty"`
	Message         string                  `json:"message"`
	RemediationHint string                  `json:"remediation_hint,omitempty"`
}

// SemanticQualityReport is the result of CheckSemanticQuality. Status is
// derived deterministically from the findings: "fail" if any blocking,
// "warn" if zero blocking but at least one warning, otherwise "pass".
// Informational findings never change the status. The Profile is recorded
// in the report so reviewers know exactly which thresholds applied.
type SemanticQualityReport struct {
	Status                    string                   `json:"status"`
	BlockingFindingCount      int                      `json:"blocking_finding_count"`
	WarningFindingCount       int                      `json:"warning_finding_count"`
	InformationalFindingCount int                      `json:"informational_finding_count"`
	Profile                   SemanticQualityProfile   `json:"profile"`
	Findings                  []SemanticQualityFinding `json:"findings"`
}

// CheckSemanticQuality applies the FSQ-06 semantic gate. It is stateless
// and side-effect-free. Callers wishing to use the canonical RBOK defaults
// can leave Profile zero; CheckSemanticQuality detects the zero value and
// substitutes DefaultRBOKProfile().
func CheckSemanticQuality(input SemanticQualityInput) SemanticQualityReport {
	profile := input.Profile
	if isZeroSemanticProfile(profile) {
		profile = DefaultRBOKProfile()
	}

	report := SemanticQualityReport{
		Profile:  profile,
		Findings: []SemanticQualityFinding{},
	}

	segByID := make(map[string]SourceSegment, len(input.Segments))
	for _, s := range input.Segments {
		segByID[s.SegmentID] = s
	}

	denySet := buildStopLabelSet(profile.StopLabelDenylist)

	for _, u := range input.Feed {
		if !isSourceDerivedFeedUnit(u) {
			continue
		}
		kind := unitSemanticKind(u, segByID)
		report.Findings = append(report.Findings, checkUnitTokenAndCharMin(u, kind, profile)...)
		report.Findings = append(report.Findings, checkUnitStopLabel(u, denySet)...)
		report.Findings = append(report.Findings, checkUnitTableCellContext(u, kind, profile)...)
		report.Findings = append(report.Findings, checkUnitMetadataTableLeak(u, profile)...)
		report.Findings = append(report.Findings, checkUnitRawDecodedMismatch(u, profile)...)
	}

	report.Findings = append(report.Findings, findDuplicateNormalizedText(input.Feed, profile)...)
	report.Findings = append(report.Findings, findZeroUnitSources(input.Sources, input.Feed, input.ShortCriticalAtoms)...)
	report.Findings = append(report.Findings, checkShortCriticalAtomDispositions(input.ShortCriticalAtoms)...)

	finalizeSemanticReport(&report)
	return report
}

func finalizeSemanticReport(r *SemanticQualityReport) {
	for _, f := range r.Findings {
		switch f.Severity {
		case SemanticSeverityBlocking:
			r.BlockingFindingCount++
		case SemanticSeverityWarning:
			r.WarningFindingCount++
		case SemanticSeverityInformational:
			r.InformationalFindingCount++
		}
	}
	switch {
	case r.BlockingFindingCount > 0:
		r.Status = "fail"
	case r.WarningFindingCount > 0:
		r.Status = "warn"
	default:
		r.Status = "pass"
	}
	sort.SliceStable(r.Findings, func(i, j int) bool {
		return semanticFindingLess(r.Findings[i], r.Findings[j])
	})
}

// semanticFindingLess implements the deterministic order required by the
// dispatch: severity DESC (blocking > warning > informational), then code
// ASC, then unit_id ASC, then chunk_id ASC.
func semanticFindingLess(a, b SemanticQualityFinding) bool {
	sa, sb := severityRank(a.Severity), severityRank(b.Severity)
	if sa != sb {
		return sa < sb // lower rank = higher severity, so blocking comes first
	}
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.UnitID != b.UnitID {
		return a.UnitID < b.UnitID
	}
	return a.ChunkID < b.ChunkID
}

func severityRank(s SemanticFindingSeverity) int {
	switch s {
	case SemanticSeverityBlocking:
		return 0
	case SemanticSeverityWarning:
		return 1
	case SemanticSeverityInformational:
		return 2
	default:
		return 3
	}
}

// isZeroSemanticProfile returns true when the caller passed an unset
// SemanticQualityProfile (all fields default-valued). We treat that as
// "use the RBOK defaults" rather than rejecting the input.
func isZeroSemanticProfile(p SemanticQualityProfile) bool {
	return len(p.MinTokensByKind) == 0 &&
		len(p.MinCharsByKind) == 0 &&
		len(p.StopLabelDenylist) == 0 &&
		p.DuplicateThreshold == 0 &&
		p.DuplicateWarningThreshold == 0 &&
		!p.AllowTableCellWithoutRow &&
		!p.AllowMetadataTableUnits &&
		p.RawDecodedMismatchSeverity == ""
}

// buildStopLabelSet case-folds the profile denylist into a lookup map keyed
// by the lowercase form. The dispatch mandates case-folded comparison.
func buildStopLabelSet(list []string) map[string]struct{} {
	out := make(map[string]struct{}, len(list))
	for _, label := range list {
		clean := strings.TrimSpace(strings.ToLower(label))
		if clean == "" {
			continue
		}
		out[clean] = struct{}{}
	}
	return out
}

// unitSemanticKind picks the most informative kind label for a feed unit:
// authoritative seg.Kind via the segment lookup, then the trailing kind on
// the SourceSegmentID (markdown_scanner.segmentID encodes it as ":<kind>"),
// then UnitType as fallback.
func unitSemanticKind(u FeedUnit, segByID map[string]SourceSegment) string {
	if seg, ok := segByID[u.SourceSegmentID]; ok && strings.TrimSpace(seg.Kind) != "" {
		return seg.Kind
	}
	if k := segmentKindFromSegmentID(u.SourceSegmentID); k != "" {
		return k
	}
	return strings.TrimSpace(u.UnitType)
}

// segmentKindFromSegmentID parses the trailing "<kind>" on a segment id of
// the form "seg:<src>:<a>-<b>:<kind>". Returns "" when the id does not
// match the expected shape.
func segmentKindFromSegmentID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "seg:") {
		return ""
	}
	idx := strings.LastIndex(id, ":")
	if idx == -1 || idx == len(id)-1 {
		return ""
	}
	return id[idx+1:]
}

// minByKind returns the threshold for the given kind, falling back to the
// "default" key when no kind-specific value is present. Returns 0 (no
// threshold) when neither is configured.
func minByKind(table map[string]int, kind string) int {
	if v, ok := table[kind]; ok {
		return v
	}
	if v, ok := table[profileKindDefault]; ok {
		return v
	}
	return 0
}

// tokenCountSemantic returns the whitespace-separated token count for a
// feed unit's text. Matches the FSQ-01 audit definition (no tokenizer dep).
func tokenCountSemantic(text string) int {
	return len(strings.Fields(text))
}

// runeCount returns utf8 rune count without importing unicode/utf8 here —
// the corpus package already pulls strings, and len() over runes via
// strings.Count is awkward. Use strings.IndexByte-free walk: use a
// for-range loop.
func runeCount(text string) int {
	n := 0
	for range text {
		n++
	}
	return n
}

func checkUnitTokenAndCharMin(u FeedUnit, kind string, profile SemanticQualityProfile) []SemanticQualityFinding {
	var out []SemanticQualityFinding
	text := u.BusinessRule
	tokens := tokenCountSemantic(text)
	chars := runeCount(strings.TrimSpace(text))

	if min := minByKind(profile.MinTokensByKind, kind); min > 0 && tokens < min {
		out = append(out, SemanticQualityFinding{
			Code:       FindingFeedUnitBelowTokenMin,
			Severity:   SemanticSeverityBlocking,
			UnitID:     u.UnitID,
			SourceID:   u.SourceID,
			SourcePath: u.SourcePath,
			StartLine:  u.StartLine,
			Message: fmt.Sprintf(
				"feed unit has %d token(s); profile requires ≥ %d for kind %q",
				tokens, min, kind,
			),
			RemediationHint: "compose with surrounding context (heading, row context) before adding to the feed",
		})
	}
	if min := minByKind(profile.MinCharsByKind, kind); min > 0 && chars < min {
		out = append(out, SemanticQualityFinding{
			Code:       FindingFeedUnitBelowCharMin,
			Severity:   SemanticSeverityBlocking,
			UnitID:     u.UnitID,
			SourceID:   u.SourceID,
			SourcePath: u.SourcePath,
			StartLine:  u.StartLine,
			Message: fmt.Sprintf(
				"feed unit has %d character(s); profile requires ≥ %d for kind %q",
				chars, min, kind,
			),
			RemediationHint: "raise the minimum-char threshold OR drop the unit from the feed",
		})
	}
	return out
}

func checkUnitStopLabel(u FeedUnit, denySet map[string]struct{}) []SemanticQualityFinding {
	if len(denySet) == 0 {
		return nil
	}
	folded := strings.ToLower(strings.TrimSpace(u.BusinessRule))
	folded = strings.TrimRight(folded, ".:;,!?")
	if _, hit := denySet[folded]; !hit {
		return nil
	}
	return []SemanticQualityFinding{{
		Code:       FindingFeedStopLabel,
		Severity:   SemanticSeverityBlocking,
		UnitID:     u.UnitID,
		SourceID:   u.SourceID,
		SourcePath: u.SourcePath,
		StartLine:  u.StartLine,
		Message: fmt.Sprintf(
			"feed unit text %q is a stop-label from the profile denylist",
			strings.TrimSpace(u.BusinessRule),
		),
		RemediationHint: "drop the unit, or compose the cell with its row/heading context",
	}}
}

func checkUnitTableCellContext(u FeedUnit, kind string, profile SemanticQualityProfile) []SemanticQualityFinding {
	if profile.AllowTableCellWithoutRow {
		return nil
	}
	if kind != KindTableCell {
		return nil
	}
	if strings.TrimSpace(u.TableID) != "" && len(u.ColumnHeaders) > 0 {
		return nil
	}
	return []SemanticQualityFinding{{
		Code:       FindingFeedTableCellNotRowContext,
		Severity:   SemanticSeverityBlocking,
		UnitID:     u.UnitID,
		SourceID:   u.SourceID,
		SourcePath: u.SourcePath,
		StartLine:  u.StartLine,
		Message: "table_cell unit lacks row context (table_id and/or column_headers unset); " +
			"cells must carry row scope before reaching the feed",
		RemediationHint: "compose the cell into its parent table_row unit, or drop it from the feed",
	}}
}

func checkUnitMetadataTableLeak(u FeedUnit, profile SemanticQualityProfile) []SemanticQualityFinding {
	if profile.AllowMetadataTableUnits {
		return nil
	}
	if u.TableRole != "metadata_table" {
		return nil
	}
	return []SemanticQualityFinding{{
		Code:       FindingFeedMetadataTableLeaked,
		Severity:   SemanticSeverityBlocking,
		UnitID:     u.UnitID,
		SourceID:   u.SourceID,
		SourcePath: u.SourcePath,
		StartLine:  u.StartLine,
		Message: "feed unit derived from a metadata_table row leaked into the curated feed; " +
			"metadata tables should remain coverage-only",
		RemediationHint: "exclude metadata_table rows from feed extraction",
	}}
}

func checkUnitRawDecodedMismatch(u FeedUnit, profile SemanticQualityProfile) []SemanticQualityFinding {
	// Only YAML-derived units (post-FSQ-04) carry RawText / DecodedValue.
	if strings.TrimSpace(u.RawText) == "" && strings.TrimSpace(u.DecodedValue) == "" {
		return nil
	}
	if u.BusinessRuleMode == "raw" {
		return nil
	}
	if strings.TrimSpace(u.RawText) == strings.TrimSpace(u.DecodedValue) {
		return nil
	}
	severity := profile.RawDecodedMismatchSeverity
	if severity == "" {
		severity = SemanticSeverityInformational
	}
	return []SemanticQualityFinding{{
		Code:       FindingFeedRawDecodedMismatch,
		Severity:   severity,
		UnitID:     u.UnitID,
		SourceID:   u.SourceID,
		SourcePath: u.SourcePath,
		StartLine:  u.StartLine,
		Message: fmt.Sprintf(
			"YAML feed unit raw_text %q differs from decoded_value %q while business_rule_mode=%q",
			truncForFinding(u.RawText, 80), truncForFinding(u.DecodedValue, 80), u.BusinessRuleMode,
		),
		RemediationHint: "set business_rule_mode=raw to keep both sides explicit, " +
			"or document the YAML scalar normalisation policy (FSQ-04)",
	}}
}

func truncForFinding(s string, max int) string {
	if runeCount(s) <= max {
		return s
	}
	out := make([]rune, 0, max)
	for _, r := range s {
		if len(out) >= max {
			break
		}
		out = append(out, r)
	}
	return string(out) + "..."
}

func findDuplicateNormalizedText(units []FeedUnit, profile SemanticQualityProfile) []SemanticQualityFinding {
	if profile.DuplicateThreshold <= 0 && profile.DuplicateWarningThreshold <= 0 {
		return nil
	}
	groups := map[string][]FeedUnit{}
	order := []string{}
	for _, u := range units {
		if !isSourceDerivedFeedUnit(u) {
			continue
		}
		hash := strings.TrimSpace(u.NormalizedTextHash)
		if hash == "" {
			continue
		}
		if _, seen := groups[hash]; !seen {
			order = append(order, hash)
		}
		groups[hash] = append(groups[hash], u)
	}
	var out []SemanticQualityFinding
	for _, h := range order {
		group := groups[h]
		count := len(group)
		if count < 2 {
			continue
		}
		var severity SemanticFindingSeverity
		switch {
		case profile.DuplicateThreshold > 0 && count >= profile.DuplicateThreshold:
			severity = SemanticSeverityBlocking
		case profile.DuplicateWarningThreshold > 0 && count >= profile.DuplicateWarningThreshold:
			severity = SemanticSeverityWarning
		default:
			continue
		}
		for i := 1; i < len(group); i++ {
			out = append(out, SemanticQualityFinding{
				Code:       FindingFeedDuplicateNormalizedText,
				Severity:   severity,
				UnitID:     group[i].UnitID,
				SourceID:   group[i].SourceID,
				SourcePath: group[i].SourcePath,
				StartLine:  group[i].StartLine,
				Message: fmt.Sprintf(
					"normalized text appears %d times in the feed (first claim: unit %q)",
					count, group[0].UnitID,
				),
				RemediationHint: "collapse duplicate atoms, or move the repeated label out of the curated feed",
			})
		}
	}
	return out
}

// findZeroUnitSources verifies the FSQ-02 invariant post-hoc: a source
// declared admitted+atomized must yield ≥1 feed unit, and if not it must
// carry an explicit ExclusionReason. The build-time check
// (ValidateAtomizedAgainstUnitCount) is the primary defence; this gate
// re-asserts it on the final artifact so a regression cannot slip past.
func findZeroUnitSources(sources []FeedSource, units []FeedUnit, shortCriticalAtoms *ShortCriticalAtomsReport) []SemanticQualityFinding {
	if len(sources) == 0 {
		return nil
	}
	used := map[string]struct{}{}
	for _, u := range units {
		for _, sid := range u.SourceIDs {
			used[sid] = struct{}{}
		}
		if u.SourceID != "" {
			used[u.SourceID] = struct{}{}
		}
	}
	for sourceID, count := range governedShortCriticalAtomCountsBySource(shortCriticalAtoms) {
		if count > 0 {
			used[sourceID] = struct{}{}
		}
	}
	var out []SemanticQualityFinding
	for _, s := range sources {
		if s.AdmissionStatus != AdmissionAdmitted {
			continue
		}
		if s.AtomizationStatus != AtomizationAtomized {
			continue
		}
		if _, has := used[s.ID]; has {
			continue
		}
		if strings.TrimSpace(s.ExclusionReason) != "" {
			continue
		}
		out = append(out, SemanticQualityFinding{
			Code:       FindingSourceZeroUnitNoReason,
			Severity:   SemanticSeverityBlocking,
			SourceID:   s.ID,
			SourcePath: s.Path,
			Message: fmt.Sprintf(
				"source %q is admitted+atomized, produced 0 feed units, and carries no exclusion_reason",
				s.ID,
			),
			RemediationHint: "either record an exclusion_reason or downgrade atomization_status to coverage_only/not_atomized",
		})
	}
	return out
}

func checkShortCriticalAtomDispositions(report *ShortCriticalAtomsReport) []SemanticQualityFinding {
	if report == nil {
		return nil
	}
	var out []SemanticQualityFinding
	for _, atom := range report.Atoms {
		if strings.TrimSpace(atom.Disposition) != "" && atom.Disposition != ShortCriticalRequiresReview {
			continue
		}
		message := fmt.Sprintf(
			"short critical fragment %q has unresolved disposition %q",
			atom.Fragment,
			atom.Disposition,
		)
		out = append(out, SemanticQualityFinding{
			Code:            FindingShortCriticalRequiresReview,
			Severity:        SemanticSeverityBlocking,
			SourceID:        atom.SourceID,
			SourcePath:      atom.SourcePath,
			StartLine:       atom.StartLine,
			Message:         message,
			RemediationHint: "classify the fragment as non_semantic, contextualized_in_parent, lexicon_atom, identifier_atom, or normative_value_atom",
		})
	}
	return out
}
