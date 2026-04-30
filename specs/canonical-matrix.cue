package nomos

// #CanonicalMatrix defines the traceability matrix linking units to all downstream layers.

#CanonicalMatrix: {
	schema_version: string | *"0.1.0"
	units: [...#Unit]
}

#Unit: {
	unit_id:     =~"^[A-Z0-9][A-Z0-9-]*$"
	unit_type:   #UnitType
	name:        string
	domain:      string
	criticality: #Criticality
	source_refs: [...#SourceRef]

	business_rule: string

	canonical_contract?: {
		path:      string
		object_id: string
		status:    "planned" | "present" | "deprecated"
	}

	schema_refs?:   [...string]
	db_refs?:       [...#DBRef]
	vector_refs?:   [...#VectorRef]
	core_refs?:     [...#CodeRef]
	api_refs?:      [...#APIRef]
	ui_refs?:       [...#UIRef]
	test_refs?:     [...string]
	decision_refs?: [...string]
	gaps?:          [...string]

	status: "covered" | "partial" | "missing" | "not_applicable" | "deprecated"
}

#SourceRef: {
	source_id: string
	locator:   string
	hash?:     string
}

#DBRef: {
	table: string
	key?:  string
}

#VectorRef: {
	collection: string
	filter?:    string
}

#CodeRef: {
	package?: string
	module?:  string
	symbol?:  string
}

#APIRef: {
	method?: string
	path:    string
}

#UIRef: {
	app?: string
	path: string
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
