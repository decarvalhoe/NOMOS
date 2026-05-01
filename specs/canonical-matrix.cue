package nomos

// #CanonicalMatrix defines the traceability matrix linking units to all downstream layers.

#CanonicalMatrix: {
	schema_version: string | *"0.1.0"
	units: [#Unit, ...#Unit]
}

#Unit: #UnitBase & (#CoveredUnit | #PartialUnit | #MissingUnit | #NotApplicableUnit | #DeprecatedUnit)

#UnitBase: {
	let uid = unit_id

	unit_id:     =~"^[A-Z0-9][A-Z0-9-]*$"
	unit_type:   #UnitType
	name:        string
	domain:      =~"^[a-z0-9][a-z0-9-]*$"
	criticality: #Criticality
	source_refs: [#SourceRef, ...#SourceRef]

	business_rule: #NonEmptyString

	canonical_contract?: {
		path:      #RelativePath
		object_id: uid
		status:    "planned" | "present" | "deprecated"
	}

	schema_refs?:   [...#RelativePath]
	db_refs?:       [...#DBRef]
	vector_refs?:   [...#VectorRef]
	core_refs?:     [...#CodeRef]
	api_refs?:      [...#APIRef]
	ui_refs?:       [...#UIRef]
	test_refs?:     [...#RelativePath]
	decision_refs?: [...#DecisionRef]
	gaps?:          [...#NonEmptyString]

	status: #CoverageStatus
}

#CoveredUnit: {
	...
	status: "covered"
	test_refs: [#RelativePath, ...#RelativePath]
	gaps?:     [] | *[]
}

#PartialUnit: {
	...
	status: "partial"
	gaps: [#NonEmptyString, ...#NonEmptyString]
}

#MissingUnit: {
	...
	status: "missing"
	gaps: [#NonEmptyString, ...#NonEmptyString]
}

#NotApplicableUnit: {
	...
	status: "not_applicable"
	gaps: [#NonEmptyString, ...#NonEmptyString]
}

#DeprecatedUnit: {
	...
	status:        "deprecated"
	decision_refs: [#DecisionRef, ...#DecisionRef]
}

#SourceRef: {
	source_id: =~"^[A-Z0-9][A-Z0-9-]*$"
	locator:   #NonEmptyString
	hash?:     =~"^sha256:[A-Za-z0-9._:-]+$"
}

#DBRef: {
	table: =~"^[A-Za-z_][A-Za-z0-9_]*(\\.[A-Za-z_][A-Za-z0-9_]*)?$"
	key?:  #NonEmptyString
}

#VectorRef: {
	collection: =~"^[A-Za-z0-9][A-Za-z0-9._-]*$"
	filter?:     #NonEmptyString
}

#CodeRef: {
	package?: #NonEmptyString
	module:   #RelativePath
	symbol?:  #NonEmptyString
}

#APIRef: {
	method?: #HTTPMethod
	path:    =~"^/.*$"
}

#UIRef: {
	app?:  =~"^[a-z0-9][a-z0-9-]*$"
	path: =~"^/.*$"
}

#UnitType:
	"rule" |
	"catalog_entry" |
	"exception" |
	"formula" |
	"term" |
	"workflow" |
	"scenario" |
	"legacy_behavior" |
	"ambiguity" |
	"decision"

#Criticality: "low" | "medium" | "high" | "critical"

#CoverageStatus: "covered" | "partial" | "missing" | "not_applicable" | "deprecated"

#HTTPMethod: "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD" | "OPTIONS"

#DecisionRef: =~"^(ADR|DEC)-[A-Z0-9][A-Z0-9._-]*$"

#RelativePath: =~"^[A-Za-z0-9][A-Za-z0-9._/@:+-]*$"

#NonEmptyString: string & =~"\\S"
