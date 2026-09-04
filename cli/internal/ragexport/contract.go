package ragexport

import "sort"

// RetrievalContractSchemaVersion identifies the retrieval-contract block.
const RetrievalContractSchemaVersion = "nomos-rag-retrieval-contract-v1"

// Scopes.
const (
	ScopeLens     = "lens"
	ScopeUnscoped = "unscoped"
)

// FieldVocabulary is one filterable field as it actually appears in the
// exported records: how many records carry it and which values occur. It is
// computed from the export, never declared, so a consumer's WHERE clause can
// be written against values that exist.
type FieldVocabulary struct {
	Field   string   `json:"field"`
	Records int      `json:"records"`
	Values  []string `json:"values"`
}

// UnsupportedScoping names a scoping a consumer might expect and must NOT
// derive from these records, with the reason.
type UnsupportedScoping struct {
	Capability string `json:"capability"`
	Reason     string `json:"reason"`
}

// RetrievalContract is what a consumer needs to scope retrieval correctly on
// top of an export: which lens (if any) already constrained the corpus, which
// fields it can filter on and with which values, and what it must not infer.
type RetrievalContract struct {
	SchemaVersion  string               `json:"schema_version"`
	Scope          string               `json:"scope"`
	Lens           *LensBinding         `json:"lens,omitempty"`
	ExcludedByLens int                  `json:"excluded_by_lens"`
	FilterFields   []FieldVocabulary    `json:"filter_fields"`
	Unsupported    []UnsupportedScoping `json:"unsupported"`
	ClaimBoundary  string               `json:"claim_boundary"`
}

type fieldCollector struct {
	field string
	get   func(Record) []string
}

func one(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

var contractCollectors = []fieldCollector{
	{"source_id", func(r Record) []string { return one(r.Provenance.SourceID) }},
	{"domain", func(r Record) []string { return one(r.Metadata.Domain) }},
	{"domain_tags", func(r Record) []string { return r.Metadata.DomainTags }},
	{"priority", func(r Record) []string { return one(r.Metadata.Priority) }},
	{"status", func(r Record) []string { return one(r.Metadata.Status) }},
	{"confidence", func(r Record) []string { return one(r.Metadata.Confidence) }},
	{"segment_kind", func(r Record) []string { return one(r.Metadata.SegmentKind) }},
	{"source_role", func(r Record) []string { return one(r.Metadata.SourceRole) }},
	{"facets.nature", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.Nature))
	}},
	{"facets.scope_level", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.ScopeLevel))
	}},
	{"facets.trust_tier", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.TrustTier))
	}},
	{"facets.provenance", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.Provenance))
	}},
	{"facets.confidentiality", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.Confidentiality))
	}},
	{"facets.applicability", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return one(string(r.Metadata.Facets.Applicability))
	}},
	{"facets.discipline_role", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return r.Metadata.Facets.DisciplineRole
	}},
	{"facets.activity", func(r Record) []string {
		if r.Metadata.Facets == nil {
			return nil
		}
		return r.Metadata.Facets.Activity
	}},
}

// BuildRetrievalContract computes the contract from an export result.
func BuildRetrievalContract(result Result) RetrievalContract {
	c := RetrievalContract{
		SchemaVersion:  RetrievalContractSchemaVersion,
		Scope:          ScopeUnscoped,
		Lens:           result.Lens,
		ExcludedByLens: len(result.Excluded),
		FilterFields:   []FieldVocabulary{},
		Unsupported: []UnsupportedScoping{{
			Capability: "temporal_scoping",
			Reason: "records carry no effective dates (effective_from / effective_to): " +
				"point-in-time resolution stays on atoms via `nomos pointintime`; " +
				"do not derive an as-of filter from these records",
		}},
		ClaimBoundary: "Scope is enforced on the corpus handed to the index (lens applied before any " +
			"retrieval); Nomos does not rank, and no retrieval-quality claim is made.",
	}
	if result.Lens != nil {
		c.Scope = ScopeLens
	}
	for _, col := range contractCollectors {
		values := map[string]struct{}{}
		records := 0
		for _, rec := range result.Records {
			vals := filterEmpty(col.get(rec))
			if len(vals) == 0 {
				continue
			}
			records++
			for _, v := range vals {
				values[v] = struct{}{}
			}
		}
		if records == 0 {
			continue
		}
		sorted := make([]string, 0, len(values))
		for v := range values {
			sorted = append(sorted, v)
		}
		sort.Strings(sorted)
		c.FilterFields = append(c.FilterFields, FieldVocabulary{Field: col.field, Records: records, Values: sorted})
	}
	return c
}
