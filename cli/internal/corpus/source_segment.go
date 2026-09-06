package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Disposition classifies how a SourceSegment participates in downstream
// artifacts (feed, RAG, coverage, attestation). It is a typed string enum.
type Disposition string

const (
	// DispositionCanonicalAtom marks a segment as a canonical semantic atom
	// eligible to back a feed unit.
	DispositionCanonicalAtom Disposition = "canonical_atom"
	// DispositionStructureOnly marks a segment that contributes structural
	// context (e.g. headings, containers) but no semantic feed text.
	DispositionStructureOnly Disposition = "structure_only"
	// DispositionCoverageOnly marks a segment that exists for source-coverage
	// accounting only (e.g. blank lines, layout-only fragments).
	DispositionCoverageOnly Disposition = "coverage_only"
	// DispositionExcludedByPolicy marks a segment intentionally excluded
	// from downstream artifacts by an explicit policy decision.
	DispositionExcludedByPolicy Disposition = "excluded_by_policy"
	// DispositionUnsupportedBlocking marks a segment whose kind/content is
	// not yet supported and which must block downstream integrity gates.
	DispositionUnsupportedBlocking Disposition = "unsupported_blocking"
)

// SourceSegment is the low-level ledger entry proving exactly what was seen
// in a single source file before any feed, RAG chunk, matrix row, or
// attestation is produced. Every segment carries exact byte and line/column
// spans plus deterministic content hashes.
type SourceSegment struct {
	SegmentID          string      `json:"segment_id"`
	SourceID           string      `json:"source_id"`
	SourcePath         string      `json:"source_path"`
	Kind               string      `json:"kind"`
	Disposition        Disposition `json:"disposition"`
	StartByte          int         `json:"start_byte"`
	EndByte            int         `json:"end_byte"`
	StartLine          int         `json:"start_line"`
	StartColumn        int         `json:"start_column"`
	EndLine            int         `json:"end_line"`
	EndColumn          int         `json:"end_column"`
	RawTextHash        string      `json:"raw_text_hash"`
	NormalizedTextHash string      `json:"normalized_text_hash"`
	ParentSegmentID    string      `json:"parent_segment_id"`
	CanonicalUnitID    string      `json:"canonical_unit_id"`
	IncludeInFeed      bool        `json:"include_in_feed"`
	IncludeInRAG       bool        `json:"include_in_rag"`
	UnsupportedReason  string      `json:"unsupported_reason"`

	// FSQ-03 (#366): optional table-context fields. Set on table_row data
	// segments (and partially on table containers / table_header) to let
	// downstream consumers reconstruct row-level canonical text without
	// re-parsing the source. None of these alter validation invariants;
	// they are descriptive metadata only.
	RowCanonicalText string   `json:"row_canonical_text,omitempty"`
	ColumnHeaders    []string `json:"column_headers,omitempty"`
	RowIndex         int      `json:"row_index,omitempty"`
	TableID          string   `json:"table_id,omitempty"`
	TableRole        string   `json:"table_role,omitempty"`

	// CKM-06: optional inclusion proof binding this segment into the
	// corpus body-ledger Merkle root. Omitted by older ledgers.
	MerkleProof *MerkleProof `json:"merkle_proof,omitempty"`
}

// validDispositions enumerates the allowed Disposition values for fast
// membership checks during validation.
var validDispositions = map[Disposition]struct{}{
	DispositionCanonicalAtom:       {},
	DispositionStructureOnly:       {},
	DispositionCoverageOnly:        {},
	DispositionExcludedByPolicy:    {},
	DispositionUnsupportedBlocking: {},
}

// Validate enforces the structural invariants of a SourceSegment.
func (s SourceSegment) Validate() error {
	if strings.TrimSpace(s.SegmentID) == "" {
		return errors.New("source segment: segment_id must be non-empty")
	}
	if strings.TrimSpace(s.SourceID) == "" {
		return errors.New("source segment: source_id must be non-empty")
	}
	if strings.TrimSpace(s.SourcePath) == "" {
		return errors.New("source segment: source_path must be non-empty")
	}
	if strings.TrimSpace(s.Kind) == "" {
		return errors.New("source segment: kind must be non-empty")
	}
	if _, ok := validDispositions[s.Disposition]; !ok {
		return fmt.Errorf("source segment: disposition %q is not a recognised value", string(s.Disposition))
	}
	if s.StartByte > s.EndByte {
		return fmt.Errorf("source segment: start_byte %d must be <= end_byte %d", s.StartByte, s.EndByte)
	}
	if s.StartLine > s.EndLine {
		return fmt.Errorf("source segment: start_line %d must be <= end_line %d", s.StartLine, s.EndLine)
	}
	if s.StartLine == s.EndLine && s.StartColumn > s.EndColumn {
		return fmt.Errorf("source segment: on single line %d, start_column %d must be <= end_column %d",
			s.StartLine, s.StartColumn, s.EndColumn)
	}
	if s.Disposition == DispositionUnsupportedBlocking && strings.TrimSpace(s.UnsupportedReason) == "" {
		return errors.New("source segment: unsupported_reason must be non-empty when disposition is unsupported_blocking")
	}
	if s.Disposition == DispositionCanonicalAtom {
		if strings.TrimSpace(s.RawTextHash) == "" {
			return errors.New("source segment: raw_text_hash must be non-empty for canonical_atom disposition")
		}
		if strings.TrimSpace(s.NormalizedTextHash) == "" {
			return errors.New("source segment: normalized_text_hash must be non-empty for canonical_atom disposition")
		}
	}
	return nil
}

// ComputeRawTextHash returns the deterministic sha256 hex digest of the
// raw byte slice. It is byte-exact: any change in input bytes (including
// whitespace) produces a different digest.
func ComputeRawTextHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ComputeNormalizedTextHash returns a deterministic sha256 hex digest
// computed over a normalized form of the input text. The normalization is:
//  1. trim trailing whitespace from each line,
//  2. collapse runs of whitespace inside a line to a single space,
//  3. strip leading and trailing blank lines.
//
// Lines are joined with a single LF. The normalization is intentionally
// conservative so that semantically equivalent text produces the same hash
// regardless of incidental whitespace.
func ComputeNormalizedTextHash(text string) string {
	normalized := normalizeText(text)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// normalizeText applies the normalization documented on
// ComputeNormalizedTextHash and returns the resulting string.
func normalizeText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = collapseInlineWhitespace(strings.TrimRightFunc(line, unicode.IsSpace))
	}
	start := 0
	for start < len(lines) && lines[start] == "" {
		start++
	}
	end := len(lines)
	for end > start && lines[end-1] == "" {
		end--
	}
	return strings.Join(lines[start:end], "\n")
}

// collapseInlineWhitespace collapses runs of whitespace inside a single
// line into one space. The input is expected to already have trailing
// whitespace trimmed.
func collapseInlineWhitespace(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inSpace := false
	for _, r := range line {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
			continue
		}
		b.WriteRune(r)
		inSpace = false
	}
	return b.String()
}
