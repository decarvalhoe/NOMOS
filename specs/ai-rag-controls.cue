package nomos

import "strings"

// #AIRAGControlLevel defines risk severity for AI/RAG controls.
#AIRAGControlLevel:
	"critical" |
	"high" |
	"medium" |
	"low"

// #AIRAGControlStatus tracks whether a control is enforced.
#AIRAGControlStatus:
	"enforced" |
	"monitored" |
	"planned" |
	"not_applicable"

// #HumanReviewDecision captures the outcome of a human review gate.
#HumanReviewDecision:
	"approved" |
	"rejected" |
	"needs_revision" |
	"deferred" |
	"pending"

// #AIRAGControl defines a single risk control for AI/RAG usage.
#AIRAGControl: {
	id:          =~"^[a-z][a-z0-9._-]*$"
	name:        string & strings.MinRunes(1)
	category:    "hallucination" | "citation" | "confidence" | "human_review" | "provenance" | "injection" | "refusal"
	level:       #AIRAGControlLevel
	status:      #AIRAGControlStatus
	description: string & strings.MinRunes(1)
	gate_mode:   "blocking" | "warning" | "informational"
	threshold?:  number & >=0 & <=1
	evidence_required: bool | *true
	remediation?: string
}

// #AIRAGEvaluation records the result of running AI/RAG controls.
#AIRAGEvaluation: {
	schema_version: string | *"0.1.0"
	evaluated_at:   string
	evaluator:      string & strings.MinRunes(1)
	model?: {
		provider: string
		name:     string
		version:  string
	}
	controls: [...#AIRAGControlResult]
	verdict: #AIRAGEvalVerdict
}

// #AIRAGControlResult is one control's evaluation outcome.
#AIRAGControlResult: {
	control_id: =~"^[a-z][a-z0-9._-]*$"
	status:     "passed" | "failed" | "warning" | "skipped"
	score?:     number & >=0 & <=1
	evidence_id?: string
	message?:   string
}

// #AIRAGEvalVerdict is the overall evaluation decision.
#AIRAGEvalVerdict: {
	pass:     bool
	blocking: bool
	summary:  string & strings.MinRunes(1)
}

// #AIRAGControlSet is the complete set of controls for a project.
#AIRAGControlSet: {
	schema_version: string | *"0.1.0"
	project_id:     =~"^[a-z0-9][a-z0-9-]*$"
	domain:         string & strings.MinRunes(1)
	controls: [...#AIRAGControl]
	// At least one control must be defined.
	controls: [_, ...]
}
