package corpus

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// FSQ-05 (#368): the corpus body ledger is a separate artifact from the
// curated feed. The feed (feed.json) carries canonical_atom-derived units
// only — it is the "doctrinal" view. The body ledger covers EVERY byte of
// EVERY admitted source: semantic atoms, structure-only spans, coverage-only
// spans, unsupported-blocking spans, excluded-by-policy spans, AND
// binary/reference sources marked as such.
//
// Splitting the two artifacts lets attestation distinguish three claims:
//   - "I have full source-body fidelity" (body ledger has zero uncovered
//     bytes for every admitted text source);
//   - "I have a curated feed" (feed.json with canonical_atom units);
//   - both.
//
// The model is generic — any corpus, not just RBOK lawbook.

// BodyLedgerFormat is the wire-format identifier for CorpusBodyLedger.
const BodyLedgerFormat = "nomos.corpus-body-ledger.v1"

// CorpusBodyLedger is the top-level full-body coverage artifact for a
// corpus. It is distinct from Feed: every admitted source contributes a
// per-source byte-coverage report, and binary/reference sources surface
// without segments alongside text sources that carry their full segment
// ledger.
type CorpusBodyLedger struct {
	Format          string             `json:"format"`
	GeneratedAt     string             `json:"generated_at"`
	CorpusRoot      string             `json:"corpus_root,omitempty"`
	SourceCount     int                `json:"source_count"`
	AdmittedCount   int                `json:"admitted_count"`
	Sources         []BodyLedgerSource `json:"sources"`
	CoverageSummary CoverageSummary    `json:"coverage_summary"`
}

// BodyLedgerSource is one source's row in the body ledger. For text
// sources Segments is the typed scanner output; for binary or reference
// sources Segments is empty and the bytes are recorded under
// ByteCoverage.BinaryBytes (or UnsupportedBytes if the operator declared
// AdmissionStatus=admitted + AtomizationStatus=unsupported).
type BodyLedgerSource struct {
	SourceID          string             `json:"source_id"`
	Path              string             `json:"path"`
	SizeBytes         int64              `json:"size_bytes"`
	Hash              string             `json:"hash,omitempty"`
	AdmissionStatus   string             `json:"admission_status"`
	AtomizationStatus string             `json:"atomization_status,omitempty"`
	SourceRole        string             `json:"source_role"`
	FormatSupport     string             `json:"format_support"`
	ExclusionReason   string             `json:"exclusion_reason,omitempty"`
	Segments          []SourceSegment    `json:"segments,omitempty"`
	ByteCoverage      ByteCoverageReport `json:"byte_coverage"`
}

// ByteCoverageReport partitions a single source's bytes by the disposition
// of the segments that own them. For text sources every byte should be
// accounted for in exactly one bucket; UncoveredBytes > 0 means the typed
// scanner left source bytes outside any segment, which the strict gate
// flags as a fidelity defect (BODY_LEDGER_UNCOVERED_TEXT_SOURCE).
//
// For non-text sources (Segments is empty), exactly one of BinaryBytes /
// UnsupportedBytes carries the SizeBytes, depending on whether the
// operator declared the source admitted-but-unsupported or as a
// binary/reference role.
type ByteCoverageReport struct {
	TotalBytes        int64 `json:"total_bytes"`
	SemanticBytes     int64 `json:"semantic_bytes"`
	StructureBytes    int64 `json:"structure_bytes"`
	CoverageOnlyBytes int64 `json:"coverage_only_bytes"`
	MetadataBytes     int64 `json:"metadata_bytes"`
	UnsupportedBytes  int64 `json:"unsupported_bytes"`
	BinaryBytes       int64 `json:"binary_bytes"`
	UncoveredBytes    int64 `json:"uncovered_bytes"`
}

// CoverageSummary aggregates the per-source ByteCoverageReports into a
// corpus-wide view, plus two breakdowns by FSQ-02 admission classification:
// BySourceRole (canonical / reference / derivative / metadata / binary),
// BySourceStatus (admitted / excluded / blocked).
type CoverageSummary struct {
	TotalBytes        int64            `json:"total_bytes"`
	SemanticBytes     int64            `json:"semantic_bytes"`
	StructureBytes    int64            `json:"structure_bytes"`
	CoverageOnlyBytes int64            `json:"coverage_only_bytes"`
	MetadataBytes     int64            `json:"metadata_bytes"`
	UnsupportedBytes  int64            `json:"unsupported_bytes"`
	BinaryBytes       int64            `json:"binary_bytes"`
	UncoveredBytes    int64            `json:"uncovered_bytes"`
	BySourceRole      map[string]int64 `json:"by_source_role"`
	BySourceStatus    map[string]int64 `json:"by_source_status"`
}

// BodyLedgerInput is the input to BuildCorpusBodyLedger. The caller pairs
// each ManifestSource (with its FSQ-02 admission classification already
// resolved) with the source's bytes and, if applicable, its typed scanner
// segments. Content + Segments are nil for binary or reference sources.
type BodyLedgerInput struct {
	CorpusRoot  string
	GeneratedAt string
	Sources     []BodyLedgerSourceInput
}

// BodyLedgerSourceInput is one source's entry in the BodyLedgerInput.
// Content carries the source bytes (used to derive SizeBytes when
// Source.Hash carries no length information); Segments is the typed
// scanner output (nil for non-text sources). The caller is responsible
// for backfilling FSQ-02 admission defaults on Source before calling.
type BodyLedgerSourceInput struct {
	Source   ManifestSource
	Content  []byte
	Segments []SourceSegment
	// SizeBytes lets the caller override the derived length (useful when
	// the source bytes are not loaded into memory; the caller passed nil
	// Content but knows the on-disk size). When zero, SizeBytes is
	// derived from len(Content).
	SizeBytes int64
}

// BuildCorpusBodyLedger constructs the body ledger from the FSQ-02-classified
// source manifest plus per-source segment slices. It is a pure function:
// no I/O, no mutation of inputs.
//
// Per-source byte accounting:
//
//   - Text source (non-empty Segments): byte sums are computed from
//     ROOT-LEVEL segments only (those with empty ParentSegmentID). The
//     markdown scanner guarantees root-level segments cover the source
//     contiguously without overlap, so summing them avoids double-counting
//     when the segment tree includes children (table cells inside rows,
//     inline refs inside paragraphs). UncoveredBytes is the residual
//     between SizeBytes and the sum of root-level segment lengths.
//
//   - Non-text source (empty Segments) admitted as
//     AtomizationUnsupported: bytes go to ByteCoverageReport.UnsupportedBytes.
//
//   - Non-text source (empty Segments) for any other admission shape:
//     bytes go to ByteCoverageReport.BinaryBytes. Excluded sources also
//     fall here; their bytes are then accounted for under
//     CoverageSummary.BySourceStatus["excluded"] but not under any
//     per-disposition bucket beyond BinaryBytes.
//
// CoverageSummary.UncoveredBytes is the sum of per-source UncoveredBytes;
// it is the single number callers (attestation, strict gate) check to
// decide CoversFullSourceBody.
func BuildCorpusBodyLedger(input BodyLedgerInput) (CorpusBodyLedger, error) {
	ledger := CorpusBodyLedger{
		Format:      BodyLedgerFormat,
		GeneratedAt: input.GeneratedAt,
		CorpusRoot:  input.CorpusRoot,
		SourceCount: len(input.Sources),
		Sources:     make([]BodyLedgerSource, 0, len(input.Sources)),
		CoverageSummary: CoverageSummary{
			BySourceRole:   map[string]int64{},
			BySourceStatus: map[string]int64{},
		},
	}
	if ledger.GeneratedAt == "" {
		ledger.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}

	for _, in := range input.Sources {
		row, err := buildBodyLedgerSource(in)
		if err != nil {
			return CorpusBodyLedger{}, fmt.Errorf("source %q: %w", in.Source.ID, err)
		}
		if row.AdmissionStatus == AdmissionAdmitted {
			ledger.AdmittedCount++
		}
		ledger.Sources = append(ledger.Sources, row)

		c := row.ByteCoverage
		s := &ledger.CoverageSummary
		s.TotalBytes += c.TotalBytes
		s.SemanticBytes += c.SemanticBytes
		s.StructureBytes += c.StructureBytes
		s.CoverageOnlyBytes += c.CoverageOnlyBytes
		s.MetadataBytes += c.MetadataBytes
		s.UnsupportedBytes += c.UnsupportedBytes
		s.BinaryBytes += c.BinaryBytes
		s.UncoveredBytes += c.UncoveredBytes
		if role := row.SourceRole; role != "" {
			s.BySourceRole[role] += c.TotalBytes
		}
		if st := row.AdmissionStatus; st != "" {
			s.BySourceStatus[st] += c.TotalBytes
		}
	}

	return ledger, nil
}

func buildBodyLedgerSource(in BodyLedgerSourceInput) (BodyLedgerSource, error) {
	src := in.Source
	size := in.SizeBytes
	if size == 0 {
		size = int64(len(in.Content))
	}
	row := BodyLedgerSource{
		SourceID:          src.ID,
		Path:              src.Path,
		SizeBytes:         size,
		Hash:              src.Hash,
		AdmissionStatus:   src.AdmissionStatus,
		AtomizationStatus: src.AtomizationStatus,
		SourceRole:        src.SourceRole,
		FormatSupport:     src.FormatSupport,
		ExclusionReason:   src.ExclusionReason,
	}

	if len(in.Segments) > 0 {
		row.Segments = append([]SourceSegment(nil), in.Segments...)
		row.ByteCoverage = computeTextCoverage(in.Segments, size)
	} else {
		row.ByteCoverage = computeNonTextCoverage(src, size)
	}

	if row.ByteCoverage.TotalBytes == 0 {
		row.ByteCoverage.TotalBytes = size
	}
	return row, nil
}

// computeTextCoverage sums root-level segment lengths into the per-disposition
// buckets. Root-level segments (ParentSegmentID == "") are guaranteed by the
// typed Markdown scanner to cover the source bytes contiguously without
// overlap; using them avoids double-counting when child segments (cells,
// inline refs) live inside their parents.
func computeTextCoverage(segments []SourceSegment, totalBytes int64) ByteCoverageReport {
	var r ByteCoverageReport
	r.TotalBytes = totalBytes
	var covered int64
	for _, seg := range segments {
		if seg.ParentSegmentID != "" {
			continue
		}
		span := int64(seg.EndByte - seg.StartByte)
		if span <= 0 {
			continue
		}
		covered += span
		switch seg.Disposition {
		case DispositionCanonicalAtom:
			r.SemanticBytes += span
		case DispositionStructureOnly:
			r.StructureBytes += span
		case DispositionCoverageOnly:
			r.CoverageOnlyBytes += span
		case DispositionExcludedByPolicy:
			r.MetadataBytes += span
		case DispositionUnsupportedBlocking:
			r.UnsupportedBytes += span
		}
	}
	if covered < totalBytes {
		r.UncoveredBytes = totalBytes - covered
	}
	return r
}

// computeNonTextCoverage records the byte total for a source that the
// caller did not segment (binary, reference, or unsupported). Per the
// FSQ-05 dispatch:
//   - admitted + unsupported atomization → UnsupportedBytes
//   - everything else (excluded, blocked, admitted-binary/reference)  → BinaryBytes
func computeNonTextCoverage(src ManifestSource, size int64) ByteCoverageReport {
	r := ByteCoverageReport{TotalBytes: size}
	if src.AdmissionStatus == AdmissionAdmitted && src.AtomizationStatus == AtomizationUnsupported {
		r.UnsupportedBytes = size
		return r
	}
	r.BinaryBytes = size
	return r
}

// MarshalCorpusBodyLedger serialises the ledger to indented JSON.
// Determinism: maps in CoverageSummary are stringly-keyed, which Go's
// encoding/json sorts alphabetically; segment ordering inside each
// BodyLedgerSource is preserved from the scanner output.
func MarshalCorpusBodyLedger(ledger CorpusBodyLedger) ([]byte, error) {
	// Stable per-source ordering: preserve input order. We only sort
	// the BySourceRole / BySourceStatus map keys for output, which
	// encoding/json handles by default for string-keyed maps.
	return json.MarshalIndent(ledger, "", "  ")
}

// SortedSourceRoles returns the BySourceRole map keys in deterministic
// order. Useful for tests and for textual reporting that wants the same
// ordering as the JSON encoder produces.
func (s CoverageSummary) SortedSourceRoles() []string {
	keys := make([]string, 0, len(s.BySourceRole))
	for k := range s.BySourceRole {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
