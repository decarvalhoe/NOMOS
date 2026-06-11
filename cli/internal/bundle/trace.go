package bundle

import (
	"fmt"
	"regexp"
	"strings"
)

// TraceManifest mirrors specs/nomos-trace-manifest.cue (#NomosTraceManifest).
// It is the machine-checkable record linking the source repo+ref to the bundle.
type TraceManifest struct {
	SchemaVersion string         `json:"schema_version"`
	Run           TraceRun       `json:"run"`
	Corpus        TraceCorpus    `json:"corpus"`
	Scope         TraceScope     `json:"scope"`
	Diff          TraceDiff      `json:"diff"`
	Output        TraceOutput    `json:"output"`
	Artifacts     TraceArtifacts `json:"artifacts"`
	Policy        TracePolicy    `json:"policy"`
}

type TraceRun struct {
	Event         string `json:"event"`
	WorkflowRunID string `json:"workflow_run_id"`
	GeneratedAt   string `json:"generated_at"`
}

type TraceCorpus struct {
	Repo    string `json:"repo"`
	BaseRef string `json:"base_ref"`
	BaseSHA string `json:"base_sha"`
	HeadRef string `json:"head_ref"`
	HeadSHA string `json:"head_sha"`
}

type TraceScope struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
}

type TraceDiff struct {
	ChangedPaths []string `json:"changed_paths"`
	Impacted     bool     `json:"impacted"`
}

type TraceOutput struct {
	Repo string `json:"repo"`
	Path string `json:"path"`
}

type TraceArtifacts struct {
	Feed        string `json:"feed,omitempty"`
	RAGMetadata string `json:"rag_metadata,omitempty"`
	Attestation string `json:"attestation,omitempty"`
}

type TracePolicy struct {
	PublishMode         string `json:"publish_mode"`
	RiskClass           string `json:"risk_class"`
	GeneratedPathGuard  string `json:"generated_path_guard"`
	SourceReadOnlyGuard string `json:"source_read_only_guard"`
}

// TraceGitContext is the real provenance the command resolves from `git`.
type TraceGitContext struct {
	Repo   string // owner/name
	Branch string
	Commit string // 7-40 hex
}

var (
	repoRe   = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)
	commitRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
)

// NewTraceManifest builds a trace manifest from real git provenance. The bundle
// is a point-in-time snapshot, so base==head and the diff is the legal
// nothing-to-do shape (changed_paths empty, impacted false).
func NewTraceManifest(git TraceGitContext, generatedAtRFC3339, bundleID, workflowRunID, event string, scopePaths []string) (TraceManifest, error) {
	if !repoRe.MatchString(git.Repo) {
		return TraceManifest{}, fmt.Errorf("trace corpus repo %q is not owner/name; pass an explicit --repo or run inside a git checkout with an origin remote", git.Repo)
	}
	if !commitRe.MatchString(git.Commit) {
		return TraceManifest{}, fmt.Errorf("trace corpus commit %q is not a 7-40 hex sha; refusing to emit a synthetic provenance", git.Commit)
	}
	if git.Branch == "" {
		git.Branch = "HEAD"
	}
	if len(scopePaths) == 0 {
		scopePaths = []string{"**/*.md"}
	}
	if workflowRunID == "" {
		workflowRunID = "local-" + shortCommit(git.Commit)
	}
	if event == "" {
		event = "workflow_dispatch"
	}
	return TraceManifest{
		SchemaVersion: "0.1.0",
		Run: TraceRun{
			Event:         event,
			WorkflowRunID: workflowRunID,
			GeneratedAt:   generatedAtRFC3339,
		},
		Corpus: TraceCorpus{
			Repo:    git.Repo,
			BaseRef: git.Branch,
			BaseSHA: git.Commit,
			HeadRef: git.Branch,
			HeadSHA: git.Commit,
		},
		Scope: TraceScope{
			ID:    "ckm-bundle",
			Paths: scopePaths,
		},
		Diff: TraceDiff{
			ChangedPaths: []string{},
			Impacted:     false,
		},
		Output: TraceOutput{
			Repo: "output",
			Path: "bundles/" + bundleID,
		},
		Artifacts: TraceArtifacts{
			Feed:        bundleID + ".json",
			RAGMetadata: bundleID + ".json",
			Attestation: bundleID + ".json",
		},
		Policy: TracePolicy{
			PublishMode:         "artifact_only",
			RiskClass:           "regulated",
			GeneratedPathGuard:  "pass",
			SourceReadOnlyGuard: "pass",
		},
	}, nil
}

// ParseRepoFromRemote extracts owner/name from a git remote URL
// (https://github.com/Owner/Repo.git or git@github.com:Owner/Repo.git).
func ParseRepoFromRemote(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	if i := strings.Index(remote, "github.com"); i >= 0 {
		rest := remote[i+len("github.com"):]
		rest = strings.TrimLeft(rest, ":/")
		if repoRe.MatchString(rest) {
			return rest
		}
	}
	// Fall back to the last two path segments.
	parts := strings.FieldsFunc(remote, func(r rune) bool { return r == '/' || r == ':' })
	if len(parts) >= 2 {
		candidate := parts[len(parts)-2] + "/" + parts[len(parts)-1]
		if repoRe.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func shortCommit(commit string) string {
	if len(commit) >= 7 {
		return commit[:7]
	}
	return commit
}
