package nomos

// #611 — external, immutable corpus snapshots as NOMOS input.
//
// The operational store (PostgreSQL, a crawler database) stays external and
// mutable; NOMOS never reads it. It consumes a SNAPSHOT: an envelope plus one
// JSON record per line, whose Merkle root the envelope declares and the verifier
// recomputes. One changed byte, one record added or dropped, and the root no
// longer matches. Go mirror: cli/internal/corpus/external_snapshot.go.

#ExternalSnapshot: {
	format:      "nomos.external-snapshot.v1"
	snapshot_id: string & !=""
	generated_at: #RFC3339
	// What exported it — a snapshot names its producer, always.
	producer:           string & !=""
	db_schema_version?: string
	// A snapshot that is not immutable is a view. Refused.
	immutable: true
	// Counts are checked against the records, never trusted.
	source_count:  int & >=1
	version_count: int & >=1
	// Merkle root over every record's leaf hash, records sorted by
	// (source_id, version_id). Recomputed on verify.
	content_hash_root: #StableHash
	records_file?:     string
	claim_boundary?:   string
}

// #SnapshotRecord is one exported source version. locator is a path for a file
// export and a canonical URL for a web export; a web record also carries its
// #610 provenance, which the import hands to the manifest untouched.
#SnapshotRecord: {
	source_id:    string & !=""
	version_id:   string & !=""
	locator:      string & !=""
	content_hash: #StableHash
	size_bytes?:  int & >=0
	captured_at:  #RFC3339
	source_type?: #SourceType
	web_source?:  #WebSource
	// Where the producer wrote the normalised export (#612): a web record
	// has an identity (locator, a URL) and an export (a file to atomise).
	export_path?: string & !=""
}

#RFC3339: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"
