package specs

// #IntegrityReport mirrors corpus.IntegrityReport produced by SFI-04
// (cli/internal/corpus/source_integrity_gate.go). It is intentionally
// minimal here; SFI-09 (#347) extends this with additional invariants
// and cross-field constraints.
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

#ByteRange: {
	source_id:  string
	start_byte: int & >=0
	end_byte:   int & >=0
}

#Finding: {
	code:        =~"^SOURCE_[A-Z_]+$"
	segment_id?: string
	source_id?:  string
	start_byte?: int & >=0
	end_byte?:   int & >=0
	message:     string
}
