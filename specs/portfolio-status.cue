package nomos

import "strings"

// NRT-019 (#667) — portfolio status v1: a COMPUTED view of committed machine
// sources. Every section names its source (path, sha256, as_of) or is
// explicitly unavailable with a reason. Structs are closed: a free-text field
// that is not a claim boundary has no place here — narrative is what this
// contract exists to replace.

#PortfolioStatusSchema: "nomos-portfolio-status-v1"
#Sha256: =~"^sha256:[a-f0-9]{64}$"
#Date: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}(T[0-9]{2}:[0-9]{2}:[0-9]{2}Z)?$"

#Source: {
	path:      string & strings.MinRunes(1)
	sha256:    #Sha256
	as_of?:    #Date
	freshness: "fresh" | "stale" | "undated"
}

#Unavailable: {
	available: false
	reason:    string & strings.MinRunes(1)
}

#Capabilities: {
	available: true
	registry:  #Source
	matrix:    #Source
	total:     int & >=0
	computed: {
		real:    int & >=0
		partial: int & >=0
		sidecar: int & >=0
		stub:    int & >=0
		absent:  int & >=0
	}
	mismatches:             int & >=0
	generic_check_failures: int & >=0
	expected_vs_computed_agree: bool
}

#LaneCounts: {
	autonomous_open:   int & >=0
	autonomous_closed: int & >=0
	passive:           int & >=0
	human:             int & >=0
	external:          int & >=0
	queue:             [...int]
}

#RegulatedItem: {
	issue:          int & >0
	dispatch:       "passive" | "human" | "external"
	delivery_state: string & strings.MinRunes(1)
	claim_state:    "bounded" | "locked" | "unlocked" | "prohibited"
}

#Roadmap: {
	available: true
	source:    #Source
	lanes: {
		product:   #LaneCounts
		devops:    #LaneCounts
		regulated: #LaneCounts
	}
	regulated_items: [...#RegulatedItem]
}

#Gap: {
	id:            =~"^GAP-[A-Z0-9][A-Z0-9._-]*$"
	severity:      "minor" | "major" | "critical"
	status:        "open" | "mitigated" | "resolved" | "closed"
	blocks_claims: [...string]
}

#Gaps: {
	available: true
	source:    #Source
	total:     int & >=0
	open:      int & >=0
	items:     [...#Gap]
}

#Capa: {
	record_id:              =~"^CAPA-[0-9]{4}-[0-9]{3}$"
	status:                 "open" | "pending_effectiveness_check" | "closed"
	severity:               "minor" | "major" | "critical"
	opened:                 #Date
	closed?:                #Date
	effectiveness_verified: bool
	retro_documented:       bool
	source:                 #Source
}

#CapaSection: {
	available: true
	directory: string & strings.MinRunes(1)
	total:     int & >=0
	open:      int & >=0
	records:   [...#Capa]
}

#Review: {
	record_id:   string & strings.MinRunes(1)
	record_type: "management_review" | "internal_audit" | "role_assignment"
	date:        #Date
	decisions:   int & >=0
	actions:     int & >=0
	findings:    int & >=0
	source:      #Source
}

#Reviews: {
	available: true
	directory: string & strings.MinRunes(1)
	total:     int & >=0
	records:   [...#Review]
}

#RepeatedCI: {
	available:                     true
	source:                        #Source
	consecutive_green_runs:        int & >=0
	target_consecutive_green_runs: int & >0
	claim_unlocked:                bool
}

#PraxisGate: {
	available:   true
	record:      #Source
	status:      "blocked" | "activatable"
	unmet_count: int & >=0
	checks:      int & >0
}

#Competence: {
	available:         true
	attestation_files: int & >=0
	waiver:            #Source
	waived_records:    int & >=0
	// The per-role status is computed by the Python gate; this view counts files only.
	role_status_computed_by: "scripts/training_competence_gate.py"
}

#DomainPack: {
	pack_id:            =~"^[a-z0-9][a-z0-9-]*$"
	source:             #Source
	has_claim_boundary: bool
}

#DomainPacks: {
	available: true
	directory: string & strings.MinRunes(1)
	total:     int & >=0
	packs:     [...#DomainPack]
}

#PublicSources: {
	available: true
	source:    #Source
	total:     int & >=0
	by_status: {[string]: int & >=0}
}

#ReleaseCandidate: {
	available:       true
	source:          #Source
	version:         string & strings.MinRunes(1)
	approval_status: "pending"
	verdict:         string & strings.MinRunes(1)
	open_gaps:       int & >=0
}

#PortfolioStatus: {
	schema_version: #PortfolioStatusSchema
	generated_at:   #Date
	repo_root:      string & strings.MinRunes(1)
	freshness_policy: {
		stale_after_days: int & >0
	}
	capabilities:      #Capabilities | #Unavailable
	roadmap:           #Roadmap | #Unavailable
	gaps:              #Gaps | #Unavailable
	capa:              #CapaSection | #Unavailable
	reviews:           #Reviews | #Unavailable
	repeated_ci:       #RepeatedCI | #Unavailable
	praxis_gate:       #PraxisGate | #Unavailable
	competence:        #Competence | #Unavailable
	domain_packs:      #DomainPacks | #Unavailable
	public_sources:    #PublicSources | #Unavailable
	release_candidate: #ReleaseCandidate | #Unavailable
	sections_unavailable: int & >=0
	sections_stale:       int & >=0
	status_digest:        #Sha256
	claim_boundary:       string & strings.MinRunes(40)
}
