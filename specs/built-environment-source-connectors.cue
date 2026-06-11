package nomos

// CKM-10 built-environment source connector contract.
//
// This contract captures read-only connector snapshots for Swiss machine
// sources. It is a pack contract, not a live scraper implementation.

#SHA256:      =~"^sha256:[a-f0-9]{64}$"
#UTCDateTime: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"

// CKM-H5 follow-up (#539): a hash that is OBVIOUSLY not a digest, used to mark a
// source family that has not yet been fetched live. It can never be mistaken for
// a real sha256 because it carries no `sha256:` prefix and no hex body, so a
// validator or consumer cannot treat it as real evidence.
#PlaceholderHash: "placeholder:not-fetched"

#BuiltEnvironmentSourceConnectors: {
	schema_version: "ckm-built-environment-connectors-v1"
	domain_profile: "built-environment"
	claim_boundary: string & !=""
	// W23-2 (#591): the four historical families stay mandatory; the list is
	// open-ended upward (geoportail_cantonal joined as the fifth).
	connectors: [#CHMachineConnector, #CHMachineConnector, #CHMachineConnector, #CHMachineConnector, ...#CHMachineConnector]
	sia_sidecar_ref: string & !=""
}

// #CHMachineConnector is the shared shape; the hashing/evidence contract is then
// refined by #FetchedConnector | #DeclaredPlaceholderConnector so that the
// fetch status and the hash form cannot disagree.
#CHMachineConnector: {
	id:              =~"^[a-z0-9][a-z0-9-]*$"
	source_family:   "fedlex_eli" | "swisstopo_stac" | "rdppf_oereb" | "ofs" | "geoportail_cantonal"
	authority_level: "confederation" | "canton" | "commune" | "mixed"
	machine_source:  true

	// fetched            ⇒ a real read-only fetch produced a real sha256 and the
	//                      connector points at committed point-in-time evidence.
	// declared_placeholder ⇒ no live fetch yet; the hash MUST be the non-digest
	//                      placeholder so it can never be mistaken for real.
	status: "fetched" | "declared_placeholder"

	retrieval: {
		mode:             "read_only_http" | "read_only_api"
		write_policy:     "read_only"
		endpoint_pattern: string & !=""
		retrieved_at_utc: #UTCDateTime
	}
	dating: {
		as_of_date_policy: "required"
		date_fields: [string & !="", ...string]
		timezone: string & !=""
	}
	hashing: {
		content_hash:     #SHA256 | #PlaceholderHash
		hash_scope:       "raw_response" | "canonical_snapshot" | "metadata_and_assets"
		canonicalization: string & !=""
	}
	outputs: {
		source_manifest_type: "api_export" | "html" | "database_export"
		stores_full_text:     bool | *false
		evidence_role:        "source_authority_register" | "geodata_snapshot" | "statistical_reference"
	}

	// Discriminated refinement: bind status to the hash form and evidence.
	{#FetchedConnector | #DeclaredPlaceholderConnector}
}

// A fetched connector must carry a real digest and a reference to the committed
// evidence file that backs it (the receipt of the real fetch).
#FetchedConnector: {
	status: "fetched"
	hashing: content_hash: #SHA256
	evidence_ref: string & !=""
}

// A declared placeholder must carry the non-digest placeholder hash and must NOT
// claim an evidence_ref (there is no evidence; nothing was fetched).
#DeclaredPlaceholderConnector: {
	status: "declared_placeholder"
	hashing: content_hash: #PlaceholderHash
	evidence_ref?: _|_
}
