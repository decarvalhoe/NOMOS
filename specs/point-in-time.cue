package nomos

import "strings"

// CKM-11 point-in-time regulatory atom model.
//
// Temporal data remains under metadata.temporal so existing atoms remain valid.
// The model follows a lightweight FRBR/ELI split: work_id identifies the stable
// normative work; expression_id identifies the version in force for a period.

#ISODate: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"

#TemporalAtomSet: {
	schema_version: string | *"0.1.0"
	atoms: [#TemporalAtom, ...#TemporalAtom]
	events?: [...#TemporalEvent]
}

#TemporalAtom: #Atom & {
	metadata: {
		temporal: #TemporalMetadata
		...
	}
}

#TemporalMetadata: {
	work_id:       string & strings.MinRunes(1)
	expression_id: string & strings.MinRunes(1)
	version_label?: string
	effective_from: #ISODate
	effective_to?:  #ISODate
	event_ids?: [...string]
}

#TemporalEvent: {
	event_id: string & strings.MinRunes(1)
	event_type: "enactment" | "amendment" | "abrogation" | "consolidation"
	work_id: string & strings.MinRunes(1)
	event_date: #ISODate
	source_ref?: string
}
