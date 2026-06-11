package nomos

// VRC-20 (D1, #563) — the domain-pack contract: a pack is 100 % DECLARATIVE.
//
// doc 40 §7 + §13, doc 45 §5 D1: values live in the pack, mechanics live in
// core. A pack is EXACTLY: vocabularies for the open facet axes + an authority
// register of machine sources + lens presets + a golden corpus + a
// claim-boundary instance + an applicability scorecard. Nothing else — the
// definition is closed, an extra field fails `cue vet`. And never code: every
// path must match the positive declarative allowlist, so a pack that tries to
// ship mechanics (.go/.py/.sh/…) cannot even name the file.

// A declarative artifact path: yaml/md/json only. Source code, scripts and
// binaries cannot match (there is no negative list to bypass — the allowlist
// is the whole rule).
#DeclarativePath: string & =~"^[A-Za-z0-9._/-]+\\.(yaml|yml|md|json)$" & !~"\\.\\."

// Pack-owned artifacts live under the pack tree, never sprinkled in core.
#PackLocalPath: #DeclarativePath & =~"^docs/regulated/domain-packs/"

// Golden-corpus material is data the pipeline ingests (md/markup/pdf are
// DATA here, not mechanics). It lives either in the pack tree or in the
// corpus testdata tree the gate executes.
#CorpusDocName: string & =~"^[A-Za-z0-9._-]+\\.(md|xml|html|pdf|yaml|json|txt)$" & !~"\\.\\."

// The open-term facet axes a pack may provide vocabulary for. The AXES are
// core mechanics (facets.cue); the pack owns only the TERMS.
#OpenTermAxis: "activity" | "discipline_role"

#DomainPack: {
	schema_version: "nomos-domain-pack-v1"
	pack_id:        =~"^[a-z0-9][a-z0-9-]*$"
	domain_profile: =~"^[a-z0-9][a-z0-9-]*$"

	// The domain-profile instance this pack rides — claim ladder, authorized
	// and blocked claims (DOR-001 contract, vetted separately).
	profile_ref: #DeclarativePath

	// The pack's own claim-boundary instance: one honest statement of what
	// this pack's content may and may not be used to claim.
	claim_boundary: string & =~".*\\S.*"

	// 1. Vocabularies — SKOS-style: one term list per open axis, in one
	// pack-owned file. At least one axis, no axis outside the open set.
	vocabularies: {
		file: #PackLocalPath
		axes: [#OpenTermAxis, ...#OpenTermAxis]
	}

	// 2. Authority register of machine sources (the connectors manifest,
	// vetted by its own contract — referenced here, never duplicated).
	source_register: {
		file:     #PackLocalPath
		contract: =~"^#[A-Za-z][A-Za-z0-9]*$"
	}

	// 3. Lens presets — the activable specialist views (knowledge-lens.cue
	// shapes; the preset id is repeated here so the manifest alone names the
	// pack's surface area).
	lens_presets: [#LensPresetRef, ...#LensPresetRef]

	// 4. Golden corpus — the pack's executable proof material (data only).
	golden_corpus: {
		root: string & =~"^(docs/regulated/domain-packs|cli/internal/corpus/testdata)/[A-Za-z0-9._/-]+$" & !~"\\.\\."
		documents: [#CorpusDocName, ...#CorpusDocName]
	}

	// 5. Applicability scorecard — where the pack applies, where it does
	// not, and what stays blocked (the honest perimeter, row by row).
	scorecard: [#ScorecardRow, ...#ScorecardRow]
}

#LensPresetRef: {
	id:   =~"^LENS-[A-Z0-9-]+$"
	file: #PackLocalPath & =~"\\.lens\\.(yaml|yml)$"
}

#ScorecardRow: {
	area:   string & =~".*\\S.*"
	status: "applicable" | "partial" | "out_of_scope" | "blocked"
	note:   string & =~".*\\S.*"
}

// The generic shape of a pack vocabulary file (the aec-vocabulary.yaml form):
// one list of {id, label_fr} terms per open axis, plus hash-only references.
// Terms are namespaced under the pack (e.g. `aec.conception`) so two packs
// can never collide.
#PackVocabulary: {
	record_type:    "aec_pack_vocabulary" | "pack_vocabulary"
	schema_version: =~"^[0-9]+\\.[0-9]+\\.[0-9]+$"
	domain_profile: =~"^[a-z0-9][a-z0-9-]*$"
	activity?: [#VocabularyTerm, ...#VocabularyTerm]
	discipline_role?: [#VocabularyTerm, ...#VocabularyTerm]
	references?: {...}
}

#VocabularyTerm: {
	id:       =~"^[a-z][a-z0-9_]*\\.[a-z][a-z0-9_]*$"
	label_fr: string & =~".*\\S.*"
}
