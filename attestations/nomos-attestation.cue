package nomos

// #InTotoStatement defines the DSSE in-toto v1 statement envelope
// used by Nomos for both admission attestations and SLSA provenance.

#InTotoStatement: {
	"_type":       "https://in-toto.io/Statement/v1"
	subject:       [#Subject, ...#Subject]
	predicateType: #PredicateType
	predicate:     #NomosAttestation | #SLSAProvenance | #ClaimBoundaryPredicate
}

#Subject: {
	name:   string & =~".*\\S.*"
	digest: [string]: =~"^[A-Fa-f0-9]+$"
}

#PredicateType:
	"https://nomos.dev/attestation/v1" |
	"https://slsa.dev/provenance/v1" |
	"https://nomos.dev/claim-boundary/v1"

// #NomosAttestation is the Nomos-specific predicate for admission attestations.
#NomosAttestation: {
	projectId:   =~"^[a-z0-9][a-z0-9-]*$"
	verdict:     #AttestVerdict
	confidence:  "low" | "medium" | "high"
	timestamp:   string
	evidence:    [...string]
	attesterId:  string & =~".*\\S.*"
	attestLevel: "none" | "basic" | "signed"
}

#AttestVerdict:
	"admitted" |
	"refused" |
	"partial" |
	"out_of_scope"

// #ClaimBoundaryPredicate records claims Nomos refuses because the required
// evidence chain is absent or insufficient.
#ClaimBoundaryPredicate: {
	projectId:   =~"^[a-z0-9][a-z0-9-]*$"
	generatedAt: string
	refusedClaims: [#RefusedClaim, ...#RefusedClaim]
	verifier: string & =~".*\\S.*"
	signatureMode: "none" | "dsse-cosign" | "sigstore-keyless"
	signature: #ClaimBoundarySignature
	claimBoundary: string & =~".*\\S.*"
}

#RefusedClaim: {
	claimId: string & =~".*\\S.*"
	statement: string & =~".*\\S.*"
	reason: string & =~".*\\S.*"
	requiredEvidence: [string & =~".*\\S.*", ...string]
	decision: "refused"
}

#ClaimBoundarySignature: {
	keyId: string
	signature: string
	signedAt: string
	logUri?: string
}

// #SLSAProvenance follows the SLSA v1 provenance predicate schema.
#SLSAProvenance: {
	buildDefinition: {
		buildType:            string & =~".*\\S.*"
		externalParameters:   {[string]: string}
		internalParameters?:  {[string]: string}
		resolvedDependencies?: [...#SLSADependency]
	}
	runDetails: {
		builder: {
			id: string & =~".*\\S.*"
		}
		metadata: {
			invocationId: string
			startedOn:    string
			finishedOn:   string
		}
	}
}

#SLSADependency: {
	uri:    string
	digest: [string]: =~"^[A-Fa-f0-9]+$"
}

// #CosignEnvelope is the DSSE envelope compatible with cosign simple signing.
#CosignEnvelope: {
	payloadType: string & =~".*\\S.*"
	payload:     string & =~".*\\S.*"
	signatures:  [#CosignSig, ...#CosignSig]
}

#CosignSig: {
	keyid: string & =~".*\\S.*"
	sig:   string
}
