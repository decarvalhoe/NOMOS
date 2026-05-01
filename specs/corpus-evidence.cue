package nomos

// #CorpusEvidenceType enumerates evidence types specific to source corpus
// verification. These types prove that the canonical source registry is
// intact, classified, owned, and tamper-evident.

#CorpusEvidenceType:
	"source_hash" |
	"file_inventory" |
	"confidentiality_tag" |
	"owner_assignment" |
	"read_only_proof"

// #CorpusEvidence is a single evidence entry collected during corpus checks.
#CorpusEvidence: {
	id:          =~"^[A-Z0-9][A-Z0-9._-]*$"
	type:        #CorpusEvidenceType
	source_id:   =~"^[A-Z0-9][A-Z0-9-]*$"
	description: string & =~"\\S"
	collected_at: string
	collector:    #CorpusCollector
	value:        #CorpusEvidenceValue
}

// #CorpusCollector identifies who or what produced the evidence.
#CorpusCollector:
	"nomos-cli" |
	"ci-pipeline" |
	"manual-review" |
	"external-tool"

// #CorpusEvidenceValue carries the type-specific payload.
#CorpusEvidenceValue: #SourceHashValue | #FileInventoryValue | #ConfidentialityTagValue | #OwnerAssignmentValue | #ReadOnlyProofValue

// #SourceHashValue proves file integrity via cryptographic hash.
// Collected by: computing sha256 of each source file at check time
// and comparing against the hash declared in source-manifest.yaml.
#SourceHashValue: {
	kind:          "source_hash"
	algorithm:     "sha256" | "sha384" | "sha512"
	expected_hash: =~"^[a-f0-9]+$"
	actual_hash:   =~"^[a-f0-9]+$"
	match:         bool
	path:          string & =~"\\S"
}

// #FileInventoryValue proves that every file declared in the source
// manifest physically exists and that no undeclared files appear in
// the corpus directory.
// Collected by: listing all files under the corpus root and diffing
// against the paths declared in source-manifest.yaml.
#FileInventoryValue: {
	kind:              "file_inventory"
	declared_count:    int & >=0
	found_count:       int & >=0
	missing:           [...string]
	undeclared:        [...string]
	complete:          bool
}

// #ConfidentialityTagValue proves that every source file has a
// confidentiality classification assigned in the manifest and that
// the classification is within the allowed vocabulary.
// Collected by: reading the confidentiality field from each source
// entry in source-manifest.yaml and verifying it is non-empty and
// belongs to #Confidentiality.
#ConfidentialityTagValue: {
	kind:            "confidentiality_tag"
	source_level:    #Confidentiality
	tag_present:     bool
	tag_valid:       bool
}

// #OwnerAssignmentValue proves that every source file has an
// accountable owner declared.
// Collected by: reading the owner field from each source entry in
// source-manifest.yaml and verifying it is non-empty.
#OwnerAssignmentValue: {
	kind:           "owner_assignment"
	owner:          string & =~"\\S"
	owner_present:  bool
}

// #ReadOnlyProofValue proves that the source file has not been
// modified since it was registered in the manifest.
// Collected by: checking filesystem permissions, git history, or
// a content-addressable store to confirm no writes occurred after
// the recorded hash was computed.
#ReadOnlyProofValue: {
	kind:           "read_only_proof"
	method:         "fs_permissions" | "git_log" | "cas_lookup"
	path:           string & =~"\\S"
	last_modified:  string
	immutable:      bool
}

// #CorpusEvidenceSet is a collection of corpus evidence entries,
// typically produced by `nomos sources check --evidence`.
#CorpusEvidenceSet: {
	schema_version: string | *"0.1.0"
	source_manifest_hash: =~"^sha256:[a-f0-9]+$"
	entries: [#CorpusEvidence, ...#CorpusEvidence]
}
