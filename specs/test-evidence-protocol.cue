package nomos

import "strings"

// #EvidenceRequirement defines one type of required test evidence.
#EvidenceRequirement: {
	id:             =~"^EVD-[0-9]+$"
	name:           string & strings.MinRunes(1)
	description:    string & strings.MinRunes(1)
	format:         "json" | "yaml" | "xml" | "html" | "text"
	retention_days: int & >=1
	required:       bool
	fields:         [string, ...string]
}

// #CollectionStep defines one step in the evidence collection protocol.
#CollectionStep: {
	id:         =~"^STEP-[0-9]+$"
	action:     string & strings.MinRunes(1)
	output:     string & strings.MinRunes(1)
	automation: "ci_workflow" | "manual" | "ci_artifact_upload"
}

// #TestEvidenceProtocol is the full protocol schema.
#TestEvidenceProtocol: {
	schema_version:      string | *"0.1.0"
	document_id:         =~"^[A-Z0-9][A-Z0-9._-]*$"
	title:               string & strings.MinRunes(1)
	status:              "draft" | "active" | "deprecated"
	owner:               string
	effective_date:      string
	purpose:             string & strings.MinRunes(1)
	evidence_requirements: [...#EvidenceRequirement]
	collection_protocol: {
		steps: [...#CollectionStep]
	}
	integrity_controls: {
		hash_algorithm:    "sha256" | "sha384" | "sha512"
		chain_required:    bool
		tamper_detection:  string
		retention_export:  string
	}
}

// #ReleaseChecklistItem defines one item in the release checklist.
#ReleaseChecklistItem: {
	id:           =~"^RC-[0-9]+$"
	category:     "evidence" | "fidelity" | "integrity" | "governance" | "approval" | "deployment" | "documentation"
	item:         string & strings.MinRunes(1)
	gate:         string & strings.MinRunes(1)
	evidence_ref: string & strings.MinRunes(1)
	blocking:     bool
	status:       "not_verified" | "verified" | "waived" | "failed"
	role?:        string
	threshold?:   string
}

// #ReleaseChecklist is the full release checklist schema.
#ReleaseChecklist: {
	schema_version:  string | *"0.1.0"
	document_id:     =~"^[A-Z0-9][A-Z0-9._-]*$"
	title:           string & strings.MinRunes(1)
	status:          "draft" | "active" | "deprecated"
	owner:           string
	effective_date:  string
	purpose:         string & strings.MinRunes(1)
	checklist:       [...#ReleaseChecklistItem]
	completion_criteria: {
		all_blocking_verified:    bool
		approval_chain_valid:     bool
		evidence_pack_uploaded:   bool
		no_open_blocking_findings: bool
	}
}
