package githubworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	specSourceOwned = "../../../specs/examples/nomos-github-workflow.source-owned.valid.yaml"
	specOutputOwned = "../../../specs/examples/nomos-github-workflow.output-owned.valid.yaml"
)

func TestLoadConfig_SourceOwned(t *testing.T) {
	t.Parallel()
	cfg, findings, err := LoadConfig(specSourceOwned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings on the canonical source-owned fixture; got %+v", findings)
	}
	if cfg.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema_version=0.1.0, got %q", cfg.SchemaVersion)
	}
	if len(cfg.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(cfg.Workflows))
	}
	w := cfg.Workflows[0]
	if w.ID != "rbok-lawbook" {
		t.Fatalf("expected id=rbok-lawbook, got %q", w.ID)
	}
	if w.Source.Repo != "RBOKproject/realisons-business" {
		t.Fatalf("expected source.repo, got %q", w.Source.Repo)
	}
	if got := strings.Join(w.Source.Paths, ","); got != "01_rbok/**" {
		t.Fatalf("expected paths=[01_rbok/**], got %q", got)
	}
	if w.Output.Path != "rbok-lawbook/" {
		t.Fatalf("expected output.path=rbok-lawbook/, got %q", w.Output.Path)
	}
	if w.Publish.Mode != "pull_request" {
		t.Fatalf("expected publish.mode=pull_request, got %q", w.Publish.Mode)
	}
	if !w.Notify.SourcePRComment.Enabled {
		t.Fatalf("expected notify.source_pr_comment.enabled=true")
	}
	if w.Notify.SourcePRComment.Mode != "summary" {
		t.Fatalf("expected notify mode=summary, got %q", w.Notify.SourcePRComment.Mode)
	}
	if len(w.Nomos.Commands) == 0 {
		t.Fatalf("expected at least one nomos command")
	}
}

func TestLoadConfig_OutputOwned(t *testing.T) {
	t.Parallel()
	cfg, findings, err := LoadConfig(specOutputOwned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected zero findings on the canonical output-owned fixture; got %+v", findings)
	}
	if len(cfg.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(cfg.Workflows))
	}
	w := cfg.Workflows[0]
	if w.Output.Repo != "corpus" {
		t.Fatalf("expected output.repo=corpus, got %q", w.Output.Repo)
	}
	if w.Publish.Mode != "direct_push" {
		t.Fatalf("expected publish.mode=direct_push, got %q", w.Publish.Mode)
	}
	if w.Notify.SourcePRComment.Enabled {
		t.Fatalf("expected notify.source_pr_comment.enabled=false")
	}
}

func TestLoadConfig_DuplicateID(t *testing.T) {
	t.Parallel()
	body := `schema_version: "0.1.0"
workflows:
  - id: dup
    source:
      repo: a/b
      base_branch: main
      paths: [docs/**]
    output:
      repo: corpus
      branch: main
      path: out/
    nomos:
      corpus_id: x
      project_id: y
      commands: [scan]
    publish:
      mode: artifact_only
      target_repo: output
      target_branch: main
      target_path: out/
      branch_strategy: fixed
      risk_class: low
    notify:
      source_pr_comment: { enabled: false }
  - id: dup
    source:
      repo: a/b
      base_branch: main
      paths: [other/**]
    output:
      repo: corpus
      branch: main
      path: out2/
    nomos:
      corpus_id: x
      project_id: y
      commands: [scan]
    publish:
      mode: artifact_only
      target_repo: output
      target_branch: main
      target_path: out2/
      branch_strategy: fixed
      risk_class: low
    notify:
      source_pr_comment: { enabled: false }
`
	path := writeTempConfig(t, body)
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error on duplicate workflow id, got nil")
	}
	if !strings.Contains(err.Error(), CodeDuplicateWorkflowID) {
		t.Fatalf("expected error to embed %s, got %v", CodeDuplicateWorkflowID, err)
	}
}

func TestLoadConfig_DirectPushNoTargetPath(t *testing.T) {
	t.Parallel()
	body := `schema_version: "0.1.0"
workflows:
  - id: w1
    source:
      repo: a/b
      base_branch: main
      paths: [docs/**]
    output:
      repo: corpus
      branch: main
      path: out/
    nomos:
      corpus_id: x
      project_id: y
      commands: [scan]
    publish:
      mode: direct_push
      target_repo: output
      target_branch: main
      target_path: ""
      branch_strategy: fixed
      risk_class: low
    notify:
      source_pr_comment: { enabled: false }
`
	path := writeTempConfig(t, body)
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error on direct_push without target_path, got nil")
	}
	if !strings.Contains(err.Error(), CodeDirectPushNoTargetPath) {
		t.Fatalf("expected error to embed %s, got %v", CodeDirectPushNoTargetPath, err)
	}
}

func TestLoadConfig_UnknownPublishMode(t *testing.T) {
	t.Parallel()
	body := `schema_version: "0.1.0"
workflows:
  - id: w1
    source:
      repo: a/b
      base_branch: main
      paths: [docs/**]
    output:
      repo: corpus
      branch: main
      path: out/
    nomos:
      corpus_id: x
      project_id: y
      commands: [scan]
    publish:
      mode: telepathy
      target_repo: output
      target_branch: main
      target_path: out/
      branch_strategy: fixed
      risk_class: low
    notify:
      source_pr_comment: { enabled: false }
`
	path := writeTempConfig(t, body)
	_, _, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error on unknown publish.mode, got nil")
	}
	if !strings.Contains(err.Error(), CodeUnknownPublishMode) {
		t.Fatalf("expected error to embed %s, got %v", CodeUnknownPublishMode, err)
	}
}

func TestLoadConfig_NonFatalFindings(t *testing.T) {
	t.Parallel()
	body := `schema_version: "0.1.0"
workflows:
  - id: w1
    source:
      repo: a/b
      base_branch: main
      paths: [docs/**]
    output:
      repo: corpus
      branch: main
      path: out/
    nomos:
      corpus_id: x
      project_id: y
      commands: [scan, telepath-2026]
    publish:
      mode: pull_request
      target_repo: output
      target_branch: main
      target_path: out/
      branch_strategy: galactic
      risk_class: cosmic
    notify:
      source_pr_comment:
        enabled: true
        mode: nostromo
        include: [changed_scopes]
`
	path := writeTempConfig(t, body)
	_, findings, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("expected non-fatal findings (no error), got %v", err)
	}
	wantCodes := map[string]bool{
		CodeUnknownBranchStrategy: false,
		CodeUnknownRiskClass:      false,
		CodeUnknownNotifyMode:     false,
		CodeUnknownNomosCommand:   false,
	}
	for _, f := range findings {
		if _, ok := wantCodes[f.Code]; ok {
			wantCodes[f.Code] = true
		}
	}
	for code, seen := range wantCodes {
		if !seen {
			t.Fatalf("expected non-fatal finding %s; got %+v", code, findings)
		}
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus-workflows.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}
