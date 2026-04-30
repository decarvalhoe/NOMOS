package nomos

// #Project defines the canonical project admission manifest for Nomos.
// It validates `nomos.project.yaml`, the first contract Nomos reads before
// detecting adapters, validating canonical sources, or producing reports.

#Project: {
	schema_version: string | *"0.1.0"

	// project carries stable metadata used to identify ownership, lifecycle,
	// domain and risk before Nomos performs any repository inspection.
	project: {
		id:          =~"^[a-z0-9][a-z0-9-]*$"
		name:        #NonEmptyString
		description?: string
		repository?:  string
		domain:      #NonEmptyString
		lifecycle:   #LifecycleMode
		risk_level:  #RiskLevel
		owners:      [#Owner, ...#Owner]
	}

	// scope states where Nomos should apply. `in_scope` is required so an
	// admission verdict cannot be detached from explicit product boundaries.
	scope: {
		verdict?:        #ScopeVerdict
		confidence?:     "low" | "medium" | "high"
		in_scope:        [#NonEmptyString, ...#NonEmptyString]
		out_of_scope?:   [...#NonEmptyString]
		assumptions?:    [...#NonEmptyString]
		bounded_contexts?: [...#NonEmptyString]
		blockers?:       [...#Blocker]
	}

	// surfaces is required because adapters and gates operate on concrete
	// product surfaces rather than a repository as an undifferentiated blob.
	surfaces: [#SurfaceDecl, ...#SurfaceDecl]

	// toolchain declares the executable project commands Nomos can ask humans or
	// CI to run. Build and test are required; lint/typecheck are optional because
	// not every project has separate commands for them.
	toolchain: {
		build:            [#Command, ...#Command]
		test:             [#Command, ...#Command]
		lint?:            [...#Command]
		typecheck?:       [...#Command]
		package_managers?: [...#NonEmptyString]
		ci_systems?:      [...#NonEmptyString]
	}

	compliance?: {
		regulated?:       bool | *false
		standards?:       [...#NonEmptyString]
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
	name:  #NonEmptyString
	role?: #NonEmptyString
	email?: =~"^[^@\\s]+@[^@\\s]+[.][^@\\s]+$"
}

#Blocker: {
	id:          =~"^[A-Z0-9][A-Z0-9-]*$"
	severity:    "low" | "medium" | "high" | "critical"
	description: #NonEmptyString
	remediation?: #NonEmptyString
}

#SurfaceDecl: {
	name:        =~"^[a-z0-9][a-z0-9-]*$"
	type:        #SurfaceType
	path?:       #NonEmptyString
	stack?:      #NonEmptyString
	critical?:   bool | *false
	entrypoints?: [...#NonEmptyString]
}

#LifecycleMode: "greenfield" | "brownfield"

#RiskLevel: "low" | "medium" | "high" | "critical"

#SurfaceType: "api" | "ui" | "worker" | "data" | "infra" | "docs" | "event" | "cli" | "batch"

#ScopeVerdict: "in_scope" | "partial" | "blocked" | "out_of_scope"

#RequiredReport:
	"nomos-report.json" |
	"coverage-report.md" |
	"attestation" |
	"sbom" |
	"provenance"

#Command: #NonEmptyString

#NonEmptyString: string & =~".*\\S.*"
