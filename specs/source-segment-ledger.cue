package nomos

// #SourceSegmentDisposition mirrors corpus.Disposition (see
// cli/internal/corpus/source_segment.go). It classifies how a segment
// participates in downstream artifacts (feed, RAG, coverage,
// attestation).
#SourceSegmentDisposition:
	"canonical_atom" |
	"structure_only" |
	"coverage_only" |
	"excluded_by_policy" |
	"unsupported_blocking"

// #SourceSegment is the CUE projection of corpus.SourceSegment. Every
// segment carries exact byte and line/column spans plus deterministic
// content hashes; canonical_atom segments additionally carry both
// raw_text_hash and normalized_text_hash, and unsupported_blocking
// segments carry a non-empty unsupported_reason.
#SourceSegment: {
	segment_id:           string & !=""
	source_id:            string & !=""
	source_path:          string & !=""
	kind:                 string & !=""
	disposition:          #SourceSegmentDisposition
	start_byte:           int & >=0
	end_byte:             int & >=start_byte
	start_line:           int & >=1
	start_column:         int & >=1
	end_line:             int & >=start_line
	end_column:           int & >=1
	raw_text_hash?:       string
	normalized_text_hash?: string
	parent_segment_id?:   string
	canonical_unit_id?:   string
	include_in_feed:      bool
	include_in_rag:       bool
	unsupported_reason?:  string

	// Conditional invariants mirror the validation in
	// SourceSegment.Validate (cli/internal/corpus/source_segment.go).
	if disposition == "canonical_atom" {
		raw_text_hash:        string & !=""
		normalized_text_hash: string & !=""
	}
	if disposition == "unsupported_blocking" {
		unsupported_reason: string & !=""
	}
}

// #SourceSegmentLedger is the per-source ledger artifact. The Go side
// emits a flat []SourceSegment slice; the on-disk evidence pack groups
// segments by source, pairs them with the source identity, and records
// the ingestion version so a future re-run can be replayed.
#SourceSegmentLedger: {
	source_id:         string & !=""
	source_path:       string & !=""
	source_hash:       =~"^sha256:[a-f0-9]+$"
	ingestion_version: string & !=""
	segments: [...#SourceSegment]
}
