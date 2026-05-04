package nomos

// #FeedAuditReport mirrors the JSON shape emitted by the `feed-audit`
// CLI binary (cli/internal/corpus/cmd/feed-audit). The audit is a
// deterministic measurement of feed and RAG quality — distinct from the
// SFI-04 source integrity gate (which validates the segment ledger),
// the SFI-07 feed quality gate (which validates the artifact), and the
// future FSQ-06 semantic gate. This schema is the wire format consumed
// by FSQ-06 (#369) when it begins to score audit metrics.
#FeedAuditReport: {
	schema_version: =~"^fsq-audit-v[0-9]+$"
	generated_at:   =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"
	feed_path:      string & !=""
	rag_path:       null | string
	corpus_root:    null | string

	totals:                    #FeedAuditTotals
	unit_kind_distribution:    {[string]: int & >=0}
	chunk_kind_distribution:   {[string]: int & >=0}
	length_distribution:       #FeedAuditLengthDistribution
	duplicate_normalized_text: #FeedAuditDuplicates
	table_cell_ratio:          #FeedAuditTableCellRatio
	yaml_raw_decoded_mismatch: #FeedAuditYAMLMismatch
	source_coverage:           #FeedAuditSourceCoverage
	top_offenders:             #FeedAuditTopOffenders
}

#FeedAuditTotals: {
	feed_unit_count:           int & >=0
	chunk_count:               int & >=0
	source_backed_unit_count:  int & >=0 & <=feed_unit_count
	source_backed_chunk_count: int & >=0 & <=chunk_count
	sources_declared_active:   int & >=0
	sources_with_zero_units:   int & >=0 & <=sources_declared_active
}

#FeedAuditLengthDistribution: {
	tokens:     #FeedAuditLengthBuckets
	characters: #FeedAuditLengthBuckets
}

// #FeedAuditLengthBuckets uses the cumulative-style buckets emitted by
// the audit. Each le_N counter includes all units whose length is <= N;
// gt_N is the residual. The exact buckets present depend on the
// dimension (tokens vs characters); both are optional integers >= 0.
#FeedAuditLengthBuckets: {
	le_2?:    int & >=0
	le_10?:   int & >=0
	le_25?:   int & >=0
	le_50?:   int & >=0
	le_100?:  int & >=0
	le_200?:  int & >=0
	le_1000?: int & >=0
	gt_100?:  int & >=0
	gt_1000?: int & >=0
}

#FeedAuditDuplicates: {
	group_count:           int & >=0
	duplicated_unit_count: int & >=0
	top_groups:            [...#FeedAuditDuplicateGroup]
}

#FeedAuditDuplicateGroup: {
	normalized_hash: string & !=""
	occurrences:     int & >=2
	sample_text:     string
	examples:        [...#FeedAuditUnitRef]
}

#FeedAuditUnitRef: {
	unit_id:     string
	source_path: string
	line:        int & >=0
}

#FeedAuditTableCellRatio: {
	table_cell_unit_count: int & >=0
	total_unit_count:      int & >=0
	ratio:                 number & >=0 & <=1
}

#FeedAuditYAMLMismatch: {
	yaml_unit_count:      int & >=0
	raw_decoded_mismatch: int & >=0 & <=yaml_unit_count
	examples:             [...#FeedAuditYAMLMismatchSample]
}

#FeedAuditYAMLMismatchSample: {
	unit_id:         string
	source_path:     string
	line:            int & >=0
	raw_excerpt:     string
	decoded_excerpt: string
}

#FeedAuditSourceCoverage: {
	by_extension:            {[string]: #FeedAuditExtensionCoverage}
	sources_with_zero_units: [...#FeedAuditZeroUnitSource]
}

#FeedAuditExtensionCoverage: {
	sources:           int & >=0
	with_units:        int & >=0 & <=sources
	byte_coverage_pct: null | (number & >=0 & <=100)
}

#FeedAuditZeroUnitSource: {
	path:       string & !=""
	size_bytes: int & >=0
}

#FeedAuditTopOffenders: {
	very_short_units: [...#FeedAuditShortUnitExample]
	duplicated_units: [...#FeedAuditDuplicatedUnitExample]
}

#FeedAuditShortUnitExample: {
	unit_id:     string
	source_path: string
	line:        int & >=0
	char_count:  int & >=0
	text:        string
}

#FeedAuditDuplicatedUnitExample: {
	unit_id:         string
	source_path:     string
	line:            int & >=0
	normalized_hash: string & !=""
}
