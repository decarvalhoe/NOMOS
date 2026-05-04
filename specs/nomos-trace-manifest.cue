package nomos

// NGW-02 (#387) — Trace manifest contract for `nomos-trace.yaml` /
// `nomos-trace.json`.
//
// Every NOMOS GitHub workflow run MUST emit a trace manifest regardless
// of publication mode. The manifest is the machine-checkable record that
// links a source repo + ref pair to the artifacts a workflow produced
// (or attempted to produce) and to the policy gates it traversed.
//
// Cross-schema alignment: the `policy.publish_mode` and
// `policy.risk_class` enums mirror `#PublishSpec.mode` and
// `#PublishSpec.risk_class` from `specs/nomos-github-workflow.cue`
// (NGW-01 #386). Drift between these schemas is a contract break.
//
// The schema is generic. RBOK is one example tenant; the contract does
// not encode any RBOK-specific assumption.

// #NomosTraceManifest is the top-level shape of `nomos-trace.yaml` /
// `nomos-trace.json`. The top-level keys (run / corpus / scope / diff /
// output / policy) are mandatory; `artifacts` is optional unless the
// publish mode is `artifact_only`, in which case the conditional
// invariant below promotes it to mandatory.
#NomosTraceManifest: {
	schema_version: =~"^0\\.[0-9]+\\.[0-9]+$"
	run:        #TraceRun
	corpus:     #TraceCorpus
	scope:      #TraceScope
	diff:       #TraceDiff
	output:     #TraceOutput
	artifacts?: #TraceArtifacts
	policy:     #TracePolicy

	// Invariant 1: when the workflow publishes to a branch (either by
	// committing a pull request or pushing directly), the output entry
	// MUST carry a non-empty branch name and a 7-40 hex commit_sha.
	// The invalid fixture
	// (specs/examples/nomos-trace-manifest.invalid.yaml) violates this
	// rule by setting `policy.publish_mode: pull_request` while leaving
	// `output.commit_sha` unset; `cue vet` must fail with a message
	// naming the missing field.
	if policy.publish_mode == "pull_request" {
		output: {
			branch:     string & !=""
			commit_sha: =~"^[0-9a-f]{7,40}$"
		}
	}
	if policy.publish_mode == "direct_push" {
		output: {
			branch:     string & !=""
			commit_sha: =~"^[0-9a-f]{7,40}$"
		}
	}

	// Invariant 2: when the workflow only uploads an Actions artifact
	// (no commit), the manifest MUST list the produced artifact files
	// so reviewers can locate them outside the destination repository.
	if policy.publish_mode == "artifact_only" {
		artifacts: #TraceArtifacts
	}
}

// #TraceRun records the GitHub Actions / repository_dispatch event the
// run was triggered from plus its workflow run identifier. The
// generated_at field is a UTC RFC3339 timestamp; the regex matches the
// Go `time.RFC3339Nano` format with a literal `Z` suffix.
#TraceRun: {
	event:           "pull_request" | "push" | "repository_dispatch" | "workflow_dispatch" | "schedule"
	workflow_run_id: string & !=""
	generated_at:    =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?Z$"
}

// #TraceCorpus identifies the source repository and the exact pair of
// refs / SHAs the workflow inspected. Both `base_sha` and `head_sha`
// are mandatory: a manifest without them cannot prove what bytes were
// processed.
#TraceCorpus: {
	repo:          =~"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$"
	base_ref:      string & !=""
	base_sha:      =~"^[0-9a-f]{7,40}$"
	head_ref:      string & !=""
	head_sha:      =~"^[0-9a-f]{7,40}$"
	pull_request?: int & >0
}

// #TraceScope names the workflow scope this run executed and the glob
// pattern set that defined its file footprint. `id` MUST match a
// `workflows[].id` value from `nomos-github-workflow.yaml` (NGW-01).
#TraceScope: {
	id:    string & !=""
	paths: [...string] & [_, ...]
}

// #TraceDiff records what changed between base_sha and head_sha and
// whether the configured scope was impacted. An empty `changed_paths`
// list with `impacted: false` is the legal "nothing-to-do" shape.
#TraceDiff: {
	changed_paths: [...string]
	impacted:      bool
}

// #TraceOutput identifies the durable destination of the workflow's
// generated artifacts. The two sentinel values `corpus` and `output`
// match the NGW-01 `#OutputSpec.repo` enum so a runner can resolve
// them at run-time. `branch` and `commit_sha` are optional at the
// type level so an `artifact_only` manifest can omit them; the
// conditional invariant on `#NomosTraceManifest` promotes them to
// mandatory whenever the publish mode actually commits a change.
#TraceOutput: {
	repo:        =~"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$" | "corpus" | "output"
	branch?:     string
	path:        string & !=""
	commit_sha?: =~"^[0-9a-f]{7,40}$"
}

// #TraceArtifacts is the open map of artifact filenames the run
// produced. The five named keys (feed / body_ledger / rag_metadata /
// attestation / diff_report) are well-known; additional NOMOS or
// tenant-specific filenames can be added as free-form string entries.
#TraceArtifacts: {
	feed?:         string
	body_ledger?:  string
	rag_metadata?: string
	attestation?:  string
	diff_report?:  string
	[string]:      string
}

// #TracePolicy records the publish-mode + risk-class pair that drove
// the run, plus the pass/fail/skipped status of the two operator-
// facing safety guards. The publish_mode and risk_class enums match
// `#PublishSpec.mode` and `#PublishSpec.risk_class` exactly; any drift
// must be coordinated across both schemas in a schema_version bump.
#TracePolicy: {
	publish_mode:           "artifact_only" | "pull_request" | "direct_push"
	risk_class:             "low" | "medium" | "high" | "regulated"
	generated_path_guard:   "pass" | "fail" | "skipped"
	source_read_only_guard: "pass" | "fail" | "skipped"
}
