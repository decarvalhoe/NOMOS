package nomos

// NGW-01 (#386) — Configuration contract for `.nomos/corpus-workflows.yaml`.
//
// This schema is the root contract of the NOMOS GitHub workflow
// integration epic (#385). Every downstream NGW ticket reads or
// references it: the trace schema (#387), the diff planner (#388), the
// reusable workflow (#389), the publisher (#390), the source PR
// commenter (#391), the trace generator (#392), the operator
// documentation (#393), the GitHub App boundary (#394), and the
// end-to-end integration (#395).
//
// The schema is generic. RBOK is one example tenant; the contract does
// not encode any RBOK-specific assumption.

// #NomosGitHubWorkflowConfig is the top-level shape of
// `.nomos/corpus-workflows.yaml`. The file always declares at least
// one workflow; every workflow self-describes its source repository,
// output target, NOMOS command sequence, publication policy, and
// notification policy.
#NomosGitHubWorkflowConfig: {
	schema_version: =~"^0\\.[0-9]+\\.[0-9]+$"
	workflows: [...#NomosWorkflow] & [_, ...]
}

// #NomosWorkflow describes one scope (e.g. one corpus / one output
// target). All sub-specs are required; the only optional fields live
// inside the sub-specs themselves.
#NomosWorkflow: {
	id:           string & !=""
	description?: string
	source:      #SourceSpec
	output:      #OutputSpec
	nomos:       #NomosCommandSpec
	publish:     #PublishSpec
	notify:      #NotifySpec
}

// #SourceSpec identifies the corpus repository and the in-repo paths
// that the workflow is responsible for.
#SourceSpec: {
	repo:        =~"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$"
	base_branch: string & !=""
	paths: [...string] & [_, ...]
	extensions?: [...string]
	profile?:    string
}

// #OutputSpec identifies the durable destination for generated
// artifacts. `corpus` and `output` are sentinel values resolved at
// run-time by the reusable workflow ("the same repo as source" /
// "the same repo as the workflow file"); a literal `owner/name`
// pair is also accepted for explicit cross-repo configuration.
#OutputSpec: {
	repo:   "corpus" | "output" | =~"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$"
	branch: string & !=""
	path:   string & !=""
}

// #NomosCommandSpec declares the NOMOS CLI invocation context.
// `corpus_id` and `project_id` flow into every artifact; the
// `commands` list is restricted to the existing CLI subcommand
// surface (cli/internal/app/corpus_cmd.go for the corpus/* subset
// plus the top-level `strict` command). Extending this list is a
// schema-versioned change.
#NomosCommandSpec: {
	corpus_id:  string & !=""
	project_id: string & !=""
	commands: [...#NomosCommand] & [_, ...]
}

// #NomosCommand is the closed enum of NOMOS CLI subcommands a
// workflow may invoke today. Adding a new value requires a new
// schema_version.
#NomosCommand:
	"scan" |
	"manifest" |
	"validate-sidecar" |
	"feed" |
	"body-ledger" |
	"attest" |
	"strict"

// #PublishSpec controls how generated artifacts reach the output
// destination. The conditional invariants below enforce two safety
// rules from the design (see
// `docs/superpowers/specs/2026-05-04-nomos-github-workflow-integration-design.md`):
//
//   1. `direct_push` against a `regulated` risk class is allowed only
//      when an explicit `controlled_decision` waiver reference is
//      present.
//   2. `target_path` is a relative path under the output root; absolute
//      paths and `..` traversal are rejected.
#PublishSpec: {
	mode:           "artifact_only" | "pull_request" | "direct_push"
	target_repo:    "corpus" | "output" | =~"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$"
	target_branch:  string & !=""
	target_path:    #RelativeOutputPath
	branch_strategy: "fixed" | "per_pr" | "per_source_ref" | "dated"
	risk_class:     "low" | "medium" | "high" | "regulated"

	// Optional waiver reference. Required by the conditional below
	// when mode == direct_push AND risk_class == regulated.
	controlled_decision?: string

	// Invariant 1: direct_push on a regulated risk class requires an
	// explicit controlled_decision string. The invalid fixture
	// (specs/examples/nomos-github-workflow.invalid.yaml) violates
	// this rule and must fail `cue vet`.
	if mode == "direct_push" && risk_class == "regulated" {
		controlled_decision: string & !=""
	}
}

// #RelativeOutputPath is a path relative to the output root. The
// schema rejects:
//   - the empty string;
//   - any path starting with "/" (absolute);
//   - any path containing a ".." segment (traversal).
//
// Allowed character set is conservative: ASCII letters, digits,
// dot, underscore, hyphen, and the path separator "/".
#RelativeOutputPath: string & !="" & =~"^[A-Za-z0-9_][A-Za-z0-9._/-]*$" & !~"(^|/)\\.\\.(/|$)"

// #NotifySpec controls source PR commenting. The toggle is the
// optional surface; when `enabled: true`, the rendering mode and
// include set are required.
#NotifySpec: {
	source_pr_comment: {
		enabled: bool
		if enabled == true {
			mode: "summary" | "detailed" | "failures_only"
			include: [...#NotifyInclude] & [_, ...]
		}
	}
}

// #NotifyInclude enumerates the comment payload sections the source
// PR commenter (#391) may render. Adding a value requires a schema
// version bump and a commenter-side change.
#NotifyInclude:
	"changed_scopes" |
	"diff_summary" |
	"output_location" |
	"trace_manifest" |
	"gate_status"
