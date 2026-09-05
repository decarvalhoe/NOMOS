package nomos

import "strings"

// NRT-016 (#660) — the Nomos ⇄ Praxis evidence exchange contract, v1.
//
// Nomos is the canonical producer; Praxis is a downstream consumer that emits
// runtime evidence, findings and CAPA status back. This contract fixes what
// crosses the boundary in each direction and how much may be relied upon:
//
//   reliance == "regulated_evidence" is only legal when EVERY referenced Nomos
//   artifact is verified and carries a verification record. Otherwise the
//   exchange is "not_qualified_external_input" — usable for schema drift
//   checks, dry runs and fixture design, never as regulated evidence
//   (docs/regulated/qualification/praxis-activation-gate.yaml, consumer_guard).
//
// The contract does not activate Praxis. Activation is a human decision
// under docs/28; NRT-018 (#662) only computes whether it could be taken.

#PraxisExchangeSchema: "nomos-praxis-evidence-exchange-v1"

#Sha256: =~"^sha256:[a-f0-9]{64}$"
#Timestamp: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"
#Ident: =~"^[A-Za-z0-9][A-Za-z0-9._:-]*$"

// What Nomos hands over. Every artifact is named by kind, located, hashed and
// carries its own verification state — the reader recomputes the hash.
#NomosArtifactKind:
	"corpus_attestation" |
	"release_candidate_manifest" |
	"control_matrix" |
	"lawbook_feed" |
	"atom_set" |
	"body_ledger" |
	"canonical_matrix" |
	"evidence_ledger"

#NomosArtifactRef: {
	artifact_id: #Ident
	kind:        #NomosArtifactKind
	path:        string & strings.MinRunes(1)
	sha256:      #Sha256
	// Verification state as NOMOS records it — never as Praxis reports it.
	verification: {
		state: "verified" | "unverified" | "failed"
		// A verified artifact names the record that verified it.
		if state == "verified" {
			record_path: string & strings.MinRunes(1)
			record_sha256: #Sha256
		}
		if state != "verified" {
			record_path?:   string
			record_sha256?: #Sha256
		}
	}
	claim_ids?: [...#Ident]
}

// What Praxis sends back. Scenario evidence is downstream: it may cite Nomos
// claims and atoms, it may never redefine them.
#PraxisScenarioResult: "pass" | "fail" | "blocked" | "not_run"

#PraxisScenario: {
	scenario_id:    #Ident
	test_id:        #Ident
	nomos_claim_ids: [...#Ident]
	nomos_atom_ids:  [...#Ident]
	result:          #PraxisScenarioResult
	evidence_sha256: #Sha256
	evidence_ref:    string & strings.MinRunes(1)
	executed_at:     #Timestamp
	praxis_version:  string & strings.MinRunes(1)
}

#RuntimeFinding: {
	finding_id:  #Ident
	scenario_id: #Ident
	severity:    "critical" | "major" | "minor" | "observation"
	status:      "open" | "mitigated" | "closed"
	summary:     string & strings.MinRunes(1)
	capa_id?:    #Ident
}

#CapaStatus: {
	capa_id:     #Ident
	finding_ids: [...#Ident] & [_, ...]
	control_ids: [...#Ident]
	status:      "open" | "in_progress" | "verified_effective" | "closed"
	owner:       string & strings.MinRunes(1)
	opened_at:   #Timestamp
	closed_at?:  #Timestamp
}

#Reliance: "not_qualified_external_input" | "regulated_evidence"

#PraxisEvidenceExchange: {
	schema_version: #PraxisExchangeSchema
	exchange_id:    #Ident
	generated_at:   #Timestamp
	producer: {
		product: "nomos"
		version: string & strings.MinRunes(1)
	}
	consumer: {
		product: "praxis"
		version: string & strings.MinRunes(1)
	}
	nomos_artifacts:  [...#NomosArtifactRef] & [_, ...]
	praxis_scenarios: [...#PraxisScenario]
	runtime_findings: [...#RuntimeFinding]
	capa:             [...#CapaStatus]
	reliance:         #Reliance
	// Regulated reliance requires every artifact verified (cross-field rule).
	if reliance == "regulated_evidence" {
		nomos_artifacts: [...{verification: state: "verified"}]
		activation_verdict_path:   string & strings.MinRunes(1)
		activation_verdict_sha256: #Sha256
	}
	claim_boundary: string & strings.MinRunes(40)
}

// NRT-017 (#661) — the atom → Praxis check mapping. Only the exposed fields of
// docs/regulated/customer-integration/praxis-atom-mapping.md cross the boundary.
#PraxisAtomMappingSchema: "nomos-praxis-atom-mapping-v1"

#MappedAtom: {
	nomos_atom_id:       #Ident
	canonical_ref:       string & strings.MinRunes(1)
	atom_type:           "rule" | "clause" | "definition" | "list_item" | "table" | "code_block" | "meta"
	content_hash:        #Sha256
	certification_state: "approved" // only approved atoms cross the boundary
	domain:              string & strings.MinRunes(1)
	source_file:         string & strings.MinRunes(1)
	source_line:         int & >=1
	praxis_checks: [...{
		scenario_id: #Ident
		test_id:     #Ident
		runtime_evidence_ids: [...#Ident]
	}] & [_, ...]
	// Internal Nomos fields never cross (block_id, parent_id, depth): closed struct.
}

#PraxisAtomMapping: {
	schema_version: #PraxisAtomMappingSchema
	mapping_id:     #Ident
	generated_at:   #Timestamp
	feed_ref: {
		path:   string & strings.MinRunes(1)
		sha256: #Sha256
	}
	authority:  "nomos" // Praxis never becomes the authority for an atom
	atoms:      [...#MappedAtom] & [_, ...]
	claim_boundary: string & strings.MinRunes(40)
}
