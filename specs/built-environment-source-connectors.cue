package nomos

// CKM-10 built-environment source connector contract.
//
// This contract captures read-only connector snapshots for Swiss machine
// sources. It is a pack contract, not a live scraper implementation.

#SHA256: =~"^sha256:[a-f0-9]{64}$"
#UTCDateTime: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"

#BuiltEnvironmentSourceConnectors: {
	schema_version: "ckm-built-environment-connectors-v1"
	domain_profile: "built-environment"
	claim_boundary: string & !=""
	connectors: [#CHMachineConnector, #CHMachineConnector, #CHMachineConnector, #CHMachineConnector]
	sia_sidecar_ref: string & !=""
}

#CHMachineConnector: {
	id: =~"^[a-z0-9][a-z0-9-]*$"
	source_family: "fedlex_eli" | "swisstopo_stac" | "rdppf_oereb" | "ofs"
	authority_level: "confederation" | "canton" | "commune" | "mixed"
	machine_source: true
	retrieval: {
		mode: "read_only_http" | "read_only_api"
		write_policy: "read_only"
		endpoint_pattern: string & !=""
		retrieved_at_utc: #UTCDateTime
	}
	dating: {
		as_of_date_policy: "required"
		date_fields: [string & !="", ...string]
		timezone: string & !=""
	}
	hashing: {
		content_hash: #SHA256
		hash_scope: "raw_response" | "canonical_snapshot" | "metadata_and_assets"
		canonicalization: string & !=""
	}
	outputs: {
		source_manifest_type: "api_export" | "html" | "database_export"
		stores_full_text: bool | *false
		evidence_role: "source_authority_register" | "geodata_snapshot" | "statistical_reference"
	}
}
