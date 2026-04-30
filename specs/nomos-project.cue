package nomos

// #Project defines the canonical project admission manifest for Nomos.
// It is intended to validate files such as `nomos.project.yaml`.

#Project: {
	schema_version: string | *"0.1.0"

	project: {
		id:          =~"^[a-z0-9][a-z0-9-]*$"
		name:        string
		description?: string
		repository?:  string
		domain:      string
		lifecycle:   "greenfield" | "brownfield"
		risk_level:  "low" | "medium" | "high" | "critical"
		owners: [...#Owner]
	}

	scope: {
		verdict?:        #ScopeVerdict
		confidence?:     #ConfidenceLevel
		in_scope:        [...string]
		out_of_scope?:   [...string]
		assumptions?:    [...string]
		bounded_contexts?: [...string]
		blockers?:       [...#Blocker]
	}

	surfaces: [...#SurfaceDecl]

	toolchain?: {
		build?:            [...string]
		test?:             [...string]
		lint?:             [...string]
		typecheck?:        [...string]
		package_managers?: [...string]
		ci_systems?:       [...string]
	}

	compliance?: {
		regulated?:       bool | *false
		standards?:       [...string]
		data_sensitivity?: "public" | "internal" | "restricted" | "secret"
		exceptions_allowed?: bool | *false
	}

	evidence?: {
		required_reports?: [...#RequiredReport]
		attestation_level?: "none" | "basic" | "signed"
	}

	notes?: string
}

#Owner: {
	name:  string
	role?: string
	email?: string
}

#Blocker: {
	id:          string
	severity:    "low" | "medium" | "high" | "critical"
	description: string
	remediation?: string
}

#SurfaceDecl: {
	name:        string
	type:        "api" | "ui" | "worker" | "data" | "infra" | "docs" | "event" | "cli" | "batch"
	path?:       string
	stack?:      string
	critical?:   bool | *false
	entrypoints?: [...string]
}

#ScopeVerdict: "in_scope" | "partial" | "blocked" | "out_of_scope"

#RequiredReport:
	"nomos-report.json" |
	"coverage-report.md" |
	"attestation" |
	"sbom" |
	"provenance"
