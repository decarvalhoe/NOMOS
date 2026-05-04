package nomos

// #IntegrityReport mirrors corpus.IntegrityReport produced by SFI-04
// (cli/internal/corpus/source_integrity_gate.go). Status is "pass" iff
// findings is empty; SFI-09 (#347) extended the schema with
// cross-field constraints and the optional fields used by downstream
// gates.
#IntegrityReport: {
	status:                        "pass" | "fail"
	source_count:                  int & >=0
	segment_count:                 int & >=0
	semantic_segment_count:        int & >=0
	uncovered_ranges:              [...#ByteRange]
	duplicate_semantic_ranges:     [...#ByteRange]
	junk_semantic_segments:        [...string]
	unsupported_blocking_segments: [...string]
	findings:                      [...#Finding]
}

// #ByteRange is the half-open [start_byte, end_byte) interval used by
// the integrity gate to locate uncovered or duplicated source spans.
#ByteRange: {
	source_id:  string & !=""
	start_byte: int & >=0
	end_byte:   int & >=start_byte
}

// #Finding is a single rule violation from either the source-integrity
// gate (SFI-04) or the feed-quality gate (SFI-07). The `code` regex is
// the public, stable contract: downstream consumers (CLI, dashboards,
// the SFI-08 strict release gate) key off these strings.
#Finding: {
	code:        =~"^(SOURCE|FEED|RAG)_[A-Z_]+$"
	segment_id?: string
	source_id?:  string
	unit_id?:    string
	chunk_id?:   string
	start_byte?: int & >=0
	end_byte?:   int & >=0
	message:     string & !=""
}

// #FeedQualityReport mirrors corpus.FeedQualityReport produced by
// SFI-07 (cli/internal/corpus/feed_quality_gate.go). Status is "pass"
// iff findings is empty. Matrix-derived feed units are skipped by the
// gate, so source_derived_unit_count <= feed_unit_count.
#FeedQualityReport: {
	status:                    "pass" | "fail"
	feed_unit_count:           int & >=0
	source_derived_unit_count: int & >=0 & <=feed_unit_count
	chunk_count:               int & >=0
	duplicate_span_count:      int & >=0
	findings:                  [...#Finding]
}

// #CorpusIntegrityCheck is the aggregate strict-gate evidence shape
// produced by SFI-08 wiring. It bundles the SFI-04 source integrity
// report and the SFI-07 feed quality report behind a single status:
//   - "pass": both subordinate reports are present and pass;
//   - "fail": at least one subordinate report is present and fails;
//   - "not_provided": no subordinate report was produced (the strict
//     gate downgrades fidelity claims accordingly).
#CorpusIntegrityCheck: {
	status:            "pass" | "fail" | "not_provided"
	source_integrity?: #IntegrityReport
	feed_quality?:     #FeedQualityReport
	summary?:          string
}
