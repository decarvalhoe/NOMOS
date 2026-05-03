package nomos

import "strings"

// #TOCArtifact is the certified Table of Contents for a document.
#TOCArtifact: {
	schema_version: string | *"0.1.0"
	artifact_type:  "nomos.toc.v1"
	generated_at:   string
	document_id:    =~"^[A-Za-z0-9][A-Za-z0-9._-]*$"
	source_hash:    string & strings.MinRunes(1)
	tree_hash:      string & strings.MinRunes(1)
	total_entries:  int & >=0
	max_depth:      int & >=0
	certified:      bool
	entries: [...#TOCEntry]
	artifact_hash:  =~"^sha256:[a-f0-9]{64}$"

	// Invariant: total_entries matches entries length.
	total_entries: len(entries)
}

// #TOCEntry is a single line in the certified TOC.
#TOCEntry: {
	id:           =~"^[A-Za-z0-9][A-Za-z0-9._-]*$"
	number:       string & strings.MinRunes(1)
	title:        string & strings.MinRunes(1)
	depth:        int & >=1
	hash?:        string
	parent_id?:   string
	has_children: bool
	page_ref?:    string
}
