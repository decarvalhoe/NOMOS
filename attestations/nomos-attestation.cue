package nomos

// #InTotoStatement defines the DSSE in-toto v1 statement envelope
// used by Nomos for both admission attestations and SLSA provenance.

#InTotoStatement: {
	_type:         "https://in-toto.io/Statement/v1"
	subject:       [#Subject, ...#Subject]
	predicateType: #PredicateType
	predicate:     #NomosAttestation | #SLSAProvenance | #CKMSupplyChain
}

#Subject: {
	name:   string & =~".*\\S.*"
	digest: [string]: =~"^[A-Fa-f0-9]+$"
}

#PredicateType:
	"https://nomos.dev/attestation/v1" |
	"https://slsa.dev/provenance/v1" |
	"https://nomos.dev/ckm/supply-chain/v1"

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

// #CKMSupplyChain records the Canonical Knowledge Mesh transformation chain.
// It is a custom in-toto predicate for source -> canon -> embedding stages.
#CKMSupplyChain: {
	version:   string | *"0.1.0"
	projectId: =~"^[a-z0-9][a-z0-9-]*$"
	corpusId:  =~"^[a-z0-9][a-z0-9-]*$"
	signature: {
		mode:       "unsigned" | "sigstore-keyless"
		status:     "unsigned" | "signed"
		trust_tier: "unverified" | "signed"
		rekor_uuid?: string & =~".*\\S.*"
	}
	steps: [#CKMSupplyChainStep, ...#CKMSupplyChainStep]
}

#CKMSupplyChainStep: {
	name:      "ingestion" | "canon" | "embedding"
	materials?: [...#Subject]
	products:  [#Subject, ...#Subject]
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
