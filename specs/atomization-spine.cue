package nomos

import "strings"

// =============================================================================
// Atomization Spine — Semantic schemas for structures, atoms, refs, matrix,
// chunks, and certificates.
// =============================================================================

// #AtomID is a stable, globally unique identifier for an atom.
#AtomID: =~"^ATOM-[A-Z0-9][A-Z0-9._-]*$"

// #StructureID identifies a document structure.
#StructureID: =~"^STRUCT-[A-Z0-9][A-Z0-9._-]*$"

// #RefID identifies a reference link.
#RefID: =~"^REF-[A-Z0-9][A-Z0-9._-]*$"

// #ChunkID identifies a vector-store chunk.
#ChunkID: =~"^CHUNK-[A-Z0-9][A-Z0-9._-]*$"

// #CertID identifies a certificate.
#CertID: =~"^CERT-[A-Z0-9][A-Z0-9._-]*$"

// #ReviewState tracks the review lifecycle of an atom.
#ReviewState: "draft" | "pending_review" | "approved" | "rejected" | "archived"

// #AtomKind classifies the semantic nature of an atom.
#AtomKind:
	"rule" |
	"clause" |
	"definition" |
	"exception" |
	"condition" |
	"obligation" |
	"permission" |
	"prohibition" |
	"formula" |
	"table" |
	"enumeration" |
	"reference" |
	"annotation"

// #SourceSpan locates the atom in its source document.
#SourceSpan: {
	source_id:   string & strings.MinRunes(1)
	path:        string & strings.MinRunes(1)
	start_line?: int & >=0
	end_line?:   int & >=0
	locator?:    string
	hash:        =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
}

// #Structure is a parsed document structure (tree of headings/sections).
#Structure: {
	structure_id: #StructureID
	document_id:  string & strings.MinRunes(1)
	title:        string & strings.MinRunes(1)
	domain:       string & strings.MinRunes(1)
	source_path:  string & strings.MinRunes(1)
	source_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	depth:        int & >=0 & <=10
	ordinal_path: =~"^[0-9]+(\\.[0-9]+)*$"
	parent_id?:   #StructureID | ""
	children?:    [...#StructureID]
	atom_ids?:    [...#AtomID]
	metadata?:    {...}
}

// #Atom is the fundamental unit of knowledge in the canonical chain.
#Atom: {
	atom_id:      #AtomID
	kind:         #AtomKind
	structure_id: #StructureID
	document_id:  string & strings.MinRunes(1)
	domain:       string & strings.MinRunes(1)

	// Content
	title:        string & strings.MinRunes(1)
	content:      string & strings.MinRunes(1)
	normalized?:  string

	// Provenance
	source_span:  #SourceSpan
	hash:         =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"

	// Lifecycle
	review_state: #ReviewState
	reviewer?:    string
	reviewed_at?: string
	created_at:   string
	updated_at:   string

	// Relations
	depends_on?:  [...#AtomID]
	supersedes?:  [...#AtomID]
	ref_ids?:     [...#RefID]

	// Classification
	priority:     "critical" | "high" | "medium" | "low"
	criticality?: "blocking" | "important" | "informational"
	tags?:        [...string]
	metadata?:    {...}
}

// #Reference links an atom to an external or internal target.
#Reference: {
	ref_id:       #RefID
	atom_id:      #AtomID
	ref_type:     "cites" | "implements" | "contradicts" | "supersedes" | "amends" | "depends_on" | "cross_reference"
	target_type:  "atom" | "document" | "external" | "standard" | "contract"
	target_id:    string & strings.MinRunes(1)
	target_locator?: string
	confidence:   "high" | "medium" | "low"
	verified:     bool | *false
	metadata?:    {...}
}

// #MatrixRow maps an atom through the canonical chain.
#MatrixRow: {
	atom_id:       #AtomID
	structure_id:  #StructureID
	domain:        string & strings.MinRunes(1)
	source_refs:   [...string] & [_, ...]
	contract_path?: string
	schema_refs?:  [...string]
	db_refs?:      [...string]
	api_refs?:     [...string]
	ui_refs?:      [...string]
	test_refs?:    [...string]
	chunk_ids?:    [...#ChunkID]
	coverage:      "covered" | "partial" | "missing" | "not_applicable"
	gaps?:         [...string]
}

// #Chunk is a vector-store chunk derived from one or more atoms.
#Chunk: {
	chunk_id:     #ChunkID
	atom_ids:     [...#AtomID] & [_, ...]
	structure_id: #StructureID
	document_id:  string & strings.MinRunes(1)
	domain:       string & strings.MinRunes(1)

	// Content
	content:      string & strings.MinRunes(1)
	token_count:  int & >=1

	// Provenance
	source_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	parent_chain: [...string]

	// Governance
	governance_status: "active" | "amended" | "stale" | "archived"
	review_state:      #ReviewState
	priority:          "critical" | "high" | "medium" | "low"

	// Embedding metadata
	embedding_model?: string
	embedding_dim?:   int & >=1
	metadata?:        {...}
}

// #Certificate attests that an atom or set of atoms has been verified.
#Certificate: {
	cert_id:      #CertID
	atom_ids:     [...#AtomID] & [_, ...]
	issued_at:    string
	issuer:       string & strings.MinRunes(1)
	cert_type:    "review_approval" | "accuracy_check" | "compliance_attestation" | "provenance_verification"
	scope:        string & strings.MinRunes(1)
	valid_until?: string
	hash:         =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	evidence_refs: [...string]
	revoked:      bool | *false
	revoked_at?:  string
	revoke_reason?: string
	metadata?:    {...}
}

// #AtomizationSpine is the top-level container for a full spine export.
#AtomizationSpine: {
	schema_version: string | *"0.1.0"
	generated_at:   string
	domain:         string & strings.MinRunes(1)
	structures:     [...#Structure]
	atoms:          [...#Atom]
	references:     [...#Reference]
	matrix:         [...#MatrixRow]
	chunks:         [...#Chunk]
	certificates:   [...#Certificate]

	summary: {
		structure_count:  int & >=0
		atom_count:       int & >=0
		reference_count:  int & >=0
		matrix_row_count: int & >=0
		chunk_count:      int & >=0
		certificate_count: int & >=0
	}
}
