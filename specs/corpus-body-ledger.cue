package nomos

// FSQ-05 (#368): the corpus body ledger is a separate artifact from the
// curated feed (specs/rbok-lawbook-feed.cue and the JSON `Feed` shape).
// The feed carries canonical_atom-derived units only — the doctrinal
// retrievable view. The body ledger covers EVERY byte of EVERY admitted
// source: semantic atoms, structure-only spans, coverage-only spans,
// unsupported-blocking spans, excluded-by-policy spans, AND
// binary/reference sources marked as such. Splitting the two artifacts
// lets attestation distinguish three claims: full source body fidelity,
// curated feed coverage, or both (see attestation.ClaimCoverage).

// #CorpusBodyLedger mirrors corpus.CorpusBodyLedger. format is the
// stable wire-format identifier; sources is one row per source from the
// FSQ-02 admission classification.
#CorpusBodyLedger: {
	format:           "nomos.corpus-body-ledger.v1"
	generated_at:     string & !=""
	corpus_root?:     string
	source_count:     int & >=0
	admitted_count:   int & >=0 & <=source_count
	sources:          [...#BodyLedgerSource]
	coverage_summary: #CoverageSummary
	merkle?:          #MerkleSummary
}

// #BodyLedgerSource is the per-source entry. For text sources the
// segments slice carries the typed scanner output and byte_coverage
// partitions the bytes by Disposition. For binary or reference sources
// segments is empty and byte_coverage carries the SizeBytes under
// binary_bytes (or unsupported_bytes when admission_status=admitted +
// atomization_status=unsupported). FSQ-02 admission fields are
// reproduced in-place so a single ledger fully describes corpus state.
#BodyLedgerSource: {
	source_id:         string & !=""
	path:              string & !=""
	size_bytes:        int & >=0
	hash?:             string
	admission_status:  "admitted" | "excluded" | "blocked"
	atomization_status?: "atomized" | "coverage_only" | "not_atomized" |
		"unsupported" | "derivative" | "excluded"
	source_role:       "canonical" | "reference" | "derivative" | "metadata" | "binary"
	format_support:    "supported" | "partial" | "unsupported"
	exclusion_reason?: string
	// segments mirrors corpus.SourceSegment; left as an open shape so
	// FSQ-03 (#366) table-row enrichment fields (row_canonical_text,
	// column_headers, ...) and future per-format extensions do not
	// require schema bumps. Strict per-segment validation lives in
	// specs/source-segment-ledger.cue.
	segments?: [...{
		segment_id:  string & !=""
		source_id:   string & !=""
		source_path: string & !=""
		kind:        string & !=""
		disposition: "canonical_atom" | "structure_only" | "coverage_only" |
			"excluded_by_policy" | "unsupported_blocking"
		start_byte:  int & >=0
		end_byte:    int & >=start_byte
		start_line:  int & >=1
		end_line:    int & >=start_line
		...
	}]
	byte_coverage: #ByteCoverageReport
	merkle_proof?: #MerkleProof
}

#MerkleSummary: {
	algorithm: "sha256-pair-v1"
	root:      =~"^[A-Fa-f0-9]{64}$"
	leaf_count: int & >=1
}

#MerkleProof: {
	leaf_hash:  =~"^[A-Fa-f0-9]{64}$"
	leaf_index: int & >=0
	path?: [...{
		position: "left" | "right"
		hash:     =~"^[A-Fa-f0-9]{64}$"
	}]
}

// #ByteCoverageReport partitions one source's bytes by disposition.
// total_bytes equals the on-disk SizeBytes; the per-disposition fields
// must sum to total_bytes (any leftover surfaces as uncovered_bytes,
// which the strict gate flags as BODY_LEDGER_UNCOVERED_TEXT_SOURCE for
// admitted+atomized sources).
#ByteCoverageReport: {
	total_bytes:         int & >=0
	semantic_bytes:      int & >=0
	structure_bytes:     int & >=0
	coverage_only_bytes: int & >=0
	metadata_bytes:      int & >=0
	unsupported_bytes:   int & >=0
	binary_bytes:        int & >=0
	uncovered_bytes:     int & >=0
}

// #CoverageSummary is the corpus-wide aggregate. by_source_role and
// by_source_status are deterministic alphabetised maps of (key -> bytes)
// projecting the ledger by FSQ-02 source_role and admission_status.
#CoverageSummary: {
	total_bytes:         int & >=0
	semantic_bytes:      int & >=0
	structure_bytes:     int & >=0
	coverage_only_bytes: int & >=0
	metadata_bytes:      int & >=0
	unsupported_bytes:   int & >=0
	binary_bytes:        int & >=0
	uncovered_bytes:     int & >=0
	by_source_role: {
		[string]: int & >=0
	}
	by_source_status: {
		[string]: int & >=0
	}
}
