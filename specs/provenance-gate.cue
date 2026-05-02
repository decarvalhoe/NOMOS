package nomos

import "strings"

// #SLSALevel indicates the SLSA build level achieved.
#SLSALevel: "slsa1" | "slsa2" | "slsa3" | "slsa4"

// #ProvenanceGateStatus is the result of a provenance verification.
#ProvenanceGateStatus: "passed" | "failed" | "skipped"

// #BuilderID identifies the build system that produced the artifact.
#BuilderID: string & strings.MinRunes(1)

// #BuildType classifies how the artifact was built.
#BuildType: string & strings.MinRunes(1)

// #ProvenanceInvocation captures the build invocation details.
#ProvenanceInvocation: {
	config_source: {
		uri:         string & strings.MinRunes(1)
		digest:      =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
		entry_point: string
	}
	parameters?: {...}
	environment?: {...}
}

// #ProvenanceSubject is an artifact subject with digest.
#ProvenanceSubject: {
	name:   string & strings.MinRunes(1)
	digest: [string]: string
}

// #ProvenanceRecord is a single provenance attestation record.
#ProvenanceRecord: {
	record_id:    =~"^[A-Z0-9][A-Z0-9._-]*$"
	builder_id:   #BuilderID
	build_type:   #BuildType
	slsa_level:   #SLSALevel
	invocation:   #ProvenanceInvocation
	subjects: [...#ProvenanceSubject] & [_, ...]
	metadata: {
		build_started_on?:  string
		build_finished_on?: string
		reproducible:       bool | *false
		completeness: {
			parameters:  bool | *false
			environment: bool | *false
			materials:   bool | *false
		}
	}
	verified:     bool
	verified_at?: string
	verifier?:    string
}

// #ProvenanceGateResult is the output of running the provenance gate.
#ProvenanceGateResult: {
	schema_version: string | *"0.1.0"
	gate_id:        =~"^[a-z0-9][a-z0-9._-]*$"
	status:         #ProvenanceGateStatus
	slsa_level:     #SLSALevel
	min_level:      #SLSALevel
	checked_at:     string
	records: [...#ProvenanceRecord]
	findings: [...{
		code:        string
		severity:    "info" | "low" | "medium" | "high" | "critical"
		message:     string
		remediation: string
	}]
	summary: {
		total_records:    int & >=0
		verified_count:   int & >=0
		unverified_count: int & >=0
		min_level_met:    bool
	}
}
