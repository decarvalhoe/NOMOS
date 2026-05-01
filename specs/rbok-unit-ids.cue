package nomos

// #RBOKUnitID defines the stable identifier pattern for all RBOK domain objects.
// Pattern: RBOK-{type}-{slug}-{version}
//
// The type prefix encodes the category of the domain concept so that IDs are
// self-describing, sortable, and greppable across the canonical matrix, source
// manifest, contracts, and downstream systems.

#RBOKUnitID: =~"^RBOK-(CNP|PRC|DIM|PAR|BOJ|SVC|CLM|RUL|EXC|FRM|TRM|WFL|SCN|LGB|AMB|DEC)-[a-z0-9][a-z0-9-]*-v[0-9]+$"

// Type prefixes:
//
//   CNP  concept         Core RBOK concept (e.g. policy, benefit, coverage)
//   PRC  principle       RBOK architectural or design principle
//   DIM  dimension       CLAIRE dimension (Contexte, Limites, Ambiguites, etc.)
//   PAR  parcours        User journey or business process flow
//   BOJ  business-object Canonical business entity (policy, claim, member, etc.)
//   SVC  service         Business service or capability
//   CLM  claim           Marketing or product claim requiring evidence
//   RUL  rule            Business rule with testable predicate
//   EXC  exception       Documented exception to a rule
//   FRM  formula         Calculation or derivation formula
//   TRM  term            Glossary term with canonical definition
//   WFL  workflow        Multi-step orchestration or approval flow
//   SCN  scenario        Test scenario or acceptance case
//   LGB  legacy          Legacy behavior preserved for compatibility
//   AMB  ambiguity       Open ambiguity requiring human decision
//   DEC  decision        Resolved decision with trace to ADR

#RBOKTypePrefix:
	"CNP" |
	"PRC" |
	"DIM" |
	"PAR" |
	"BOJ" |
	"SVC" |
	"CLM" |
	"RUL" |
	"EXC" |
	"FRM" |
	"TRM" |
	"WFL" |
	"SCN" |
	"LGB" |
	"AMB" |
	"DEC"

// #RBOKUnitRef is used in cross-references (source_refs, decision_refs, etc.)
// where the full RBOK ID appears as a string.
#RBOKUnitRef: #RBOKUnitID

// #RBOKSlug is the human-readable portion of the ID.
#RBOKSlug: =~"^[a-z0-9][a-z0-9-]*$"

// #RBOKVersion is the version suffix.
#RBOKVersion: =~"^v[0-9]+$"
