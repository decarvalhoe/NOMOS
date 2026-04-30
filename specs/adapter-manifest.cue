package nomos

// #AdapterManifest defines the public contract every Nomos adapter must
// publish. Adapter authors should store it as `adapter.nomos.yaml` at the
// adapter root.

#AdapterManifest: {
	schema_version: string | *"0.1.0"

	adapter:      #AdapterIdentity
	compatibility: #AdapterCompatibility
	stack_support: [#AdapterStackSupport, ...#AdapterStackSupport]
	capabilities:  #AdapterCapabilities

	commands?: [...#AdapterCommand]
	limitations?: [...#AdapterLimitation]
	test_contract: #AdapterTestContract

	metadata?: {
		homepage?:     string
		repository?:   string
		documentation?: string
		notes?:        string
	}
}

#AdapterIdentity: {
	id:      #AdapterID
	name:    string
	version: #SemVer
	status:  #AdapterLifecycleStatus
	owners: [#AdapterOwner, ...#AdapterOwner]

	license?: string
	package?: {
		type:     "oci" | "npm" | "pypi" | "maven" | "nuget" | "binary" | "source"
		name:     string
		version?:  #SemVer
		url?:      string
		checksum?: #Digest
	}
}

#AdapterCompatibility: {
	nomos_core: {
		min_version:     #SemVer
		max_version?:    #SemVer
		tested_versions?: [...#SemVer]
	}

	manifest_contract: {
		version: #SemVer | *"0.1.0"
	}

	schemas?: {
		nomos_project?:     #SchemaVersionSupport
		source_manifest?:   #SchemaVersionSupport
		canonical_matrix?:  #SchemaVersionSupport
		adapter_manifest?:  #SchemaVersionSupport
	}

	deprecates?: [...#AdapterDeprecatedContract]
}

#SchemaVersionSupport: {
	min_version:  #SemVer
	max_version?: #SemVer
	mode?:        "read" | "write" | "read_write" | *"read_write"
}

#AdapterDeprecatedContract: {
	field:       string
	since:       #SemVer
	remove_after: #SemVer
	replacement?: string
	reason:      string
}

#AdapterStackSupport: {
	language:       #AdapterLanguage
	runtime?:       string
	frameworks?:    [...string]
	package_managers?: [...string]
	file_globs:     [string, ...string]
	exclude_globs?: [...string]
	surfaces:       [#AdapterSurfaceType, ...#AdapterSurfaceType]
}

#AdapterCapabilities: {
	contract_version: #SemVer | *"0.1.0"
	provides:         [#AdapterCapability, ...#AdapterCapability]
	requires?:        [...#AdapterRequirement]
}

#AdapterCapability: {
	id:       #AdapterCapabilityID
	category: #AdapterCapabilityCategory
	status:   #AdapterCapabilityStatus

	surfaces: [#AdapterSurfaceType, ...#AdapterSurfaceType]
	languages?: [...#AdapterLanguage]
	frameworks?: [...string]

	inputs:   [#AdapterInputKind, ...#AdapterInputKind]
	outputs:  [#AdapterOutputKind, ...#AdapterOutputKind]
	evidence: [#AdapterEvidenceKind, ...#AdapterEvidenceKind]

	confidence?: "low" | "medium" | "high"
	notes?:      string
}

#AdapterRequirement: {
	id:      string
	type:    "tool" | "runtime" | "grammar" | "schema" | "policy" | "environment"
	version?: #SemVer
	optional?: bool | *false
	reason:   string
}

#AdapterCommand: {
	id:   #AdapterCommandID
	argv: [string, ...string]

	cwd?: "repo_root" | "adapter_root" | *"repo_root"
	input: {
		transport: "argv" | "stdin-json" | "file-json"
		repo_arg?:   string
		schema_ref?: string
	}
	output: {
		transport: "stdout-json" | "file-json" | "exit-code"
		schema_ref?: string
	}

	timeout_seconds?: int & >=1 & <=3600
	required?:        bool | *false
}

#AdapterLimitation: {
	id:          =~"^[a-z][a-z0-9-]*$"
	severity:    "low" | "medium" | "high" | "critical"
	description: string
	mitigation?: string
}

#AdapterTestContract: {
	fixtures?: [...#AdapterFixture]
	required_checks: [#AdapterRequiredCheck, ...#AdapterRequiredCheck]
	compatibility_matrix?: [...#AdapterCompatibilityCase]
}

#AdapterFixture: {
	id:      =~"^[a-z][a-z0-9-]*$"
	path:    string
	purpose: string
	expected_capabilities?: [...#AdapterCapabilityID]
}

#AdapterCompatibilityCase: {
	nomos_core: #SemVer
	adapter:    #SemVer
	status:     "supported" | "deprecated" | "blocked"
	reason?:    string
}

#AdapterOwner: {
	name:  string
	role?: string
	email?: string
}

#AdapterID: =~"^[a-z][a-z0-9-]*$"

#SemVer: =~"^[0-9]+[.][0-9]+[.][0-9]+(-[0-9A-Za-z.-]+)?([+][0-9A-Za-z.-]+)?$"

#Digest: =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"

#AdapterLifecycleStatus:
	"experimental" |
	"supported" |
	"deprecated" |
	"retired"

#AdapterCapabilityStatus:
	"experimental" |
	"stable" |
	"deprecated"

#AdapterCapabilityCategory:
	"detection" |
	"extraction" |
	"validation" |
	"evidence" |
	"integration"

#AdapterCapabilityID:
	"repo_metadata_detection" |
	"language_detection" |
	"surface_detection" |
	"route_detection" |
	"service_detection" |
	"data_model_detection" |
	"config_detection" |
	"dependency_detection" |
	"fixture_detection" |
	"mock_detection" |
	"hardcoded_catalog_detection" |
	"forbidden_pattern_detection" |
	"provenance_extraction" |
	"test_surface_detection" |
	"ci_detection" |
	"adapter_self_check"

#AdapterSurfaceType:
	"api" |
	"ui" |
	"worker" |
	"data" |
	"infra" |
	"docs" |
	"event" |
	"cli" |
	"batch"

#AdapterLanguage:
	"javascript" |
	"typescript" |
	"python" |
	"java" |
	"kotlin" |
	"scala" |
	"go" |
	"csharp" |
	"sql" |
	"terraform" |
	"yaml" |
	"unknown"

#AdapterInputKind:
	"filesystem" |
	"package_manifest" |
	"lockfile" |
	"source_file" |
	"ast" |
	"config_file" |
	"ci_file" |
	"schema_file" |
	"nomos_project"

#AdapterOutputKind:
	"surface_inventory" |
	"capability_result" |
	"forbidden_pattern_finding" |
	"provenance_link" |
	"adapter_diagnostic" |
	"command_result"

#AdapterEvidenceKind:
	"file_path" |
	"line_range" |
	"symbol" |
	"route" |
	"package_metadata" |
	"ast_node" |
	"config_key" |
	"command_exit_code"

#AdapterCommandID:
	"detect" |
	"analyze" |
	"verify" |
	"self-check"

#AdapterRequiredCheck:
	"manifest-validates" |
	"capabilities-declared" |
	"version-compatibility-declared" |
	"commands-smoke-tested" |
	"fixtures-pass" |
	"limitations-declared"
