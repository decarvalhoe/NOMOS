package nomos

// #Project defines the canonical project admission manifest for Nomos.
// It validates `nomos.project.yaml`, the first contract Nomos reads before
// detecting adapters, validating canonical sources, or producing reports.
//
// Two modes are supported:
//   - "product" (default): a code repository with surfaces, toolchain, and
//     build/test commands. This is the standard admission path.
//   - "canonical_corpus": an authoritative source collection with no code
//     surfaces. Requires source inventory, hashability, owner/confidentiality
//     metadata, and read-only execution. No build/test toolchain.

#Project: #ProductProject | #CorpusProject

// #ProductProject is the default mode for code repositories.
#ProductProject: {
	schema_version: string | *"0.1.0"
	mode:           *"product" | "product"

	project: #ProjectIdentity
	scope:   #Scope

	// surfaces is required because adapters and gates operate on concrete
	// product surfaces rather than a repository as an undifferentiated blob.
	surfaces: [#SurfaceDecl, ...#SurfaceDecl]

	// toolchain declares the executable project commands Nomos can ask humans or
	// CI to run. Build and test are required; lint/typecheck are optional because
	// not every project has separate commands for them.
	toolchain: {
		build: [#Command, ...#Command]
		test: [#Command, ...#Command]
		lint?: [...#Command]
		typecheck?: [...#Command]
		package_managers?: [...#NonEmptyString]
		ci_systems?: [...#NonEmptyString]
	}

	compliance?: #Compliance
	evidence?:   #Evidence
	notes?:      string
}

// #CorpusProject admits a repository as an authoritative source collection.
// A corpus has no code surfaces, no build/test toolchain, and is read-only
// from an execution standpoint. It is verified through source inventory
// completeness, hash integrity, and ownership metadata.
#CorpusProject: {
	schema_version: string | *"0.1.0"
	mode:           "canonical_corpus"

	project: #ProjectIdentity
	scope:   #Scope

	// corpus_surfaces: optional docs/data surfaces for organizational purposes.
	// Unlike product mode, corpus surfaces are never inspected by adapters.
	corpus_surfaces?: [...#CorpusSurfaceDecl]

	// source_inventory is required for corpus mode. It points to the manifest
	// that registers all authoritative sources with hashes and metadata.
	source_inventory: {
		manifest_path: #NonEmptyString
		hash_required: bool | *true
		owner_required: bool | *true
		confidentiality_required: bool | *true
	}

	// corpus_policy controls how the corpus is consumed.
	corpus_policy: {
		execution: *"read_only" | "read_only"
		allowed_consumers?: [...#NonEmptyString]
		retention?: #NonEmptyString
	}

	compliance?: #Compliance
	evidence?:   #Evidence
	notes?:      string
}

// #ProjectIdentity carries stable metadata used to identify ownership,
// lifecycle, domain and risk before Nomos performs any repository inspection.
#ProjectIdentity: {
	id:           =~"^[a-z0-9][a-z0-9-]*$"
	name:         #NonEmptyString
	description?: string
	repository?:  string
	domain:       #NonEmptyString
	lifecycle:    #LifecycleMode
	risk_level:   #RiskLevel
	owners: [#Owner, ...#Owner]
}

// #Scope states where Nomos should apply.
#Scope: {
	verdict?:    #ScopeVerdict
	confidence?: #ConfidenceLevel
	in_scope: [#NonEmptyString, ...#NonEmptyString]
	out_of_scope?: [...#NonEmptyString]
	assumptions?: [...#NonEmptyString]
	bounded_contexts?: [...#NonEmptyString]
	blockers?: [...#Blocker]
}

#Compliance: {
	regulated?: bool | *false
	standards?: [...#NonEmptyString]
	data_sensitivity?:   "public" | "internal" | "restricted" | "secret"
	exceptions_allowed?: bool | *false
}

#Evidence: {
	required_reports?: [...#RequiredReport]
	attestation_level?: "none" | "basic" | "signed"
}

#Owner: {
	name:   #NonEmptyString
	role?:  #NonEmptyString
	email?: =~"^[^@\\s]+@[^@\\s]+[.][^@\\s]+$"
}

#Blocker: {
	id:           =~"^[A-Z0-9][A-Z0-9-]*$"
	severity:     "low" | "medium" | "high" | "critical"
	description:  #NonEmptyString
	remediation?: #NonEmptyString
}

#SurfaceDecl: {
	name:      =~"^[a-z0-9][a-z0-9-]*$"
	type:      #SurfaceType
	path?:     #NonEmptyString
	stack?:    #NonEmptyString
	critical?: bool | *false
	entrypoints?: [...#NonEmptyString]
}

// #CorpusSurfaceDecl is a lightweight surface for corpus mode.
// Only docs and data types are allowed — no code surfaces.
#CorpusSurfaceDecl: {
	name: =~"^[a-z0-9][a-z0-9-]*$"
	type: #CorpusSurfaceType
	path?: #NonEmptyString
}

#CorpusSurfaceType: "docs" | "data"

#LifecycleMode: "greenfield" | "brownfield"

#RiskLevel: "low" | "medium" | "high" | "critical"

#SurfaceType: "api" | "ui" | "worker" | "data" | "infra" | "docs" | "event" | "cli" | "batch"

#ScopeVerdict: "in_scope" | "partial" | "blocked" | "out_of_scope"

#ConfidenceLevel: "low" | "medium" | "high"

#RequiredReport:
	"nomos-report.json" |
	"coverage-report.md" |
	"attestation" |
	"sbom" |
	"provenance"

#Command: #NonEmptyString

#NonEmptyString: string & =~".*\\S.*"
