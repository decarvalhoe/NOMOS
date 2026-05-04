// Package githubworkflow loads and reasons about
// .nomos/corpus-workflows.yaml, the configuration contract introduced
// in NGW-01 (#386). It is consumed by the scoped diff planner
// (NGW-03 / #388 — this package's diff.go) and by future NGW tickets
// (publisher #390, source PR commenter #391, trace generator #392, the
// reusable workflow #389, and the end-to-end integration #395).
//
// The package is a pure-Go library: no I/O, no GitHub network calls,
// no shelling out. The only filesystem operation is the LoadConfig
// read of the YAML file.
package githubworkflow

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowConfig is the top-level shape of `.nomos/corpus-workflows.yaml`.
// It mirrors `#NomosGitHubWorkflowConfig` in `specs/nomos-github-workflow.cue`.
type WorkflowConfig struct {
	SchemaVersion string     `yaml:"schema_version" json:"schema_version"`
	Workflows     []Workflow `yaml:"workflows"      json:"workflows"`
}

// Workflow is one scope (one corpus / one output target). It mirrors
// `#NomosWorkflow` in the CUE schema.
type Workflow struct {
	ID          string           `yaml:"id"                       json:"id"`
	Description string           `yaml:"description,omitempty"    json:"description,omitempty"`
	Source      SourceSpec       `yaml:"source"                   json:"source"`
	Output      OutputSpec       `yaml:"output"                   json:"output"`
	Nomos       NomosCommandSpec `yaml:"nomos"                    json:"nomos"`
	Publish     PublishSpec      `yaml:"publish"                  json:"publish"`
	Notify      NotifySpec       `yaml:"notify"                   json:"notify"`
}

// SourceSpec mirrors `#SourceSpec` in the CUE schema.
type SourceSpec struct {
	Repo       string   `yaml:"repo"                  json:"repo"`
	BaseBranch string   `yaml:"base_branch"           json:"base_branch"`
	Paths      []string `yaml:"paths"                 json:"paths"`
	Extensions []string `yaml:"extensions,omitempty"  json:"extensions,omitempty"`
	Profile    string   `yaml:"profile,omitempty"     json:"profile,omitempty"`
}

// OutputSpec mirrors `#OutputSpec` in the CUE schema.
type OutputSpec struct {
	Repo   string `yaml:"repo"   json:"repo"`
	Branch string `yaml:"branch" json:"branch"`
	Path   string `yaml:"path"   json:"path"`
}

// NomosCommandSpec mirrors `#NomosCommandSpec` in the CUE schema.
type NomosCommandSpec struct {
	CorpusID  string   `yaml:"corpus_id"  json:"corpus_id"`
	ProjectID string   `yaml:"project_id" json:"project_id"`
	Commands  []string `yaml:"commands"   json:"commands"`
}

// PublishSpec mirrors `#PublishSpec` in the CUE schema.
type PublishSpec struct {
	Mode               string `yaml:"mode"                            json:"mode"`
	TargetRepo         string `yaml:"target_repo"                     json:"target_repo"`
	TargetBranch       string `yaml:"target_branch"                   json:"target_branch"`
	TargetPath         string `yaml:"target_path"                     json:"target_path"`
	BranchStrategy     string `yaml:"branch_strategy"                 json:"branch_strategy"`
	RiskClass          string `yaml:"risk_class"                      json:"risk_class"`
	ControlledDecision string `yaml:"controlled_decision,omitempty"   json:"controlled_decision,omitempty"`
}

// NotifySpec mirrors `#NotifySpec` in the CUE schema.
type NotifySpec struct {
	SourcePRComment NotifyComment `yaml:"source_pr_comment" json:"source_pr_comment"`
}

// NotifyComment mirrors the inner `source_pr_comment` block of `#NotifySpec`.
type NotifyComment struct {
	Enabled bool     `yaml:"enabled"           json:"enabled"`
	Mode    string   `yaml:"mode,omitempty"    json:"mode,omitempty"`
	Include []string `yaml:"include,omitempty" json:"include,omitempty"`
}

// ConfigFinding is a non-fatal validation hit. Fatal failures are returned
// via the error return of LoadConfig.
type ConfigFinding struct {
	Code       string `json:"code"`
	WorkflowID string `json:"workflow_id,omitempty"`
	Message    string `json:"message"`
}

// Stable finding/error codes emitted by LoadConfig. Downstream consumers
// (diff planner, publisher, dashboards) key off these strings; they MUST
// NOT change without coordination.
const (
	CodeDuplicateWorkflowID    = "NGW_CONFIG_DUPLICATE_WORKFLOW_ID"
	CodeUnknownPublishMode     = "NGW_CONFIG_UNKNOWN_PUBLISH_MODE"
	CodeDirectPushNoTargetPath = "NGW_CONFIG_DIRECT_PUSH_NO_TARGET_PATH"
	CodeUnknownRiskClass       = "NGW_CONFIG_UNKNOWN_RISK_CLASS"
	CodeUnknownBranchStrategy  = "NGW_CONFIG_UNKNOWN_BRANCH_STRATEGY"
	CodeUnknownNotifyMode      = "NGW_CONFIG_UNKNOWN_NOTIFY_MODE"
	CodeUnknownNomosCommand    = "NGW_CONFIG_UNKNOWN_NOMOS_COMMAND"
)

// validPublishModes is the closed enum of acceptable publish.mode values.
// Any other value is a fatal CodeUnknownPublishMode error.
var validPublishModes = map[string]struct{}{
	"artifact_only": {},
	"pull_request":  {},
	"direct_push":   {},
}

// validBranchStrategies / validRiskClasses / validNotifyModes / validNomosCommands
// are the closed enums whose deviations are surfaced as non-fatal findings
// (the YAML may legitimately carry profile-specific extensions; we warn
// rather than block).
var (
	validBranchStrategies = map[string]struct{}{
		"fixed":           {},
		"per_pr":          {},
		"per_source_ref":  {},
		"dated":           {},
	}
	validRiskClasses = map[string]struct{}{
		"low":        {},
		"medium":     {},
		"high":       {},
		"regulated":  {},
	}
	validNotifyModes = map[string]struct{}{
		"summary":       {},
		"detailed":      {},
		"failures_only": {},
	}
	validNomosCommands = map[string]struct{}{
		"scan":             {},
		"manifest":         {},
		"validate-sidecar": {},
		"feed":             {},
		"body-ledger":      {},
		"attest":           {},
		"strict":           {},
	}
)

// LoadConfig reads and validates a workflow config file. It returns the
// typed config, a slice of non-fatal findings, and an error for fatal
// problems (invalid YAML, missing required field, unknown enum, duplicate
// workflow id, or direct_push without a target_path).
//
// Pure I/O is limited to a single os.ReadFile call; the function does not
// touch the corpus repository.
func LoadConfig(path string) (WorkflowConfig, []ConfigFinding, error) {
	if strings.TrimSpace(path) == "" {
		return WorkflowConfig{}, nil, errors.New("githubworkflow: config path required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkflowConfig{}, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg WorkflowConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return WorkflowConfig{}, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	findings, err := validateConfig(&cfg)
	if err != nil {
		return WorkflowConfig{}, findings, err
	}
	return cfg, findings, nil
}

// validateConfig enforces the fatal invariants and accumulates non-fatal
// findings. It mutates nothing on cfg beyond reading.
func validateConfig(cfg *WorkflowConfig) ([]ConfigFinding, error) {
	var findings []ConfigFinding
	seen := map[string]struct{}{}
	for i, w := range cfg.Workflows {
		if strings.TrimSpace(w.ID) == "" {
			return findings, fmt.Errorf("workflow[%d]: id is required", i)
		}
		if _, dup := seen[w.ID]; dup {
			return findings, fmt.Errorf("%s: workflow id %q appears more than once",
				CodeDuplicateWorkflowID, w.ID)
		}
		seen[w.ID] = struct{}{}

		if _, ok := validPublishModes[w.Publish.Mode]; !ok {
			return findings, fmt.Errorf("%s: workflow %q publish.mode %q is not in {artifact_only, pull_request, direct_push}",
				CodeUnknownPublishMode, w.ID, w.Publish.Mode)
		}
		if w.Publish.Mode == "direct_push" && strings.TrimSpace(w.Publish.TargetPath) == "" {
			return findings, fmt.Errorf("%s: workflow %q publish.mode=direct_push requires a non-empty publish.target_path",
				CodeDirectPushNoTargetPath, w.ID)
		}

		if _, ok := validBranchStrategies[w.Publish.BranchStrategy]; !ok {
			findings = append(findings, ConfigFinding{
				Code:       CodeUnknownBranchStrategy,
				WorkflowID: w.ID,
				Message: fmt.Sprintf("publish.branch_strategy %q is not in {fixed, per_pr, per_source_ref, dated}",
					w.Publish.BranchStrategy),
			})
		}
		if _, ok := validRiskClasses[w.Publish.RiskClass]; !ok {
			findings = append(findings, ConfigFinding{
				Code:       CodeUnknownRiskClass,
				WorkflowID: w.ID,
				Message: fmt.Sprintf("publish.risk_class %q is not in {low, medium, high, regulated}",
					w.Publish.RiskClass),
			})
		}
		if w.Notify.SourcePRComment.Enabled {
			if _, ok := validNotifyModes[w.Notify.SourcePRComment.Mode]; !ok {
				findings = append(findings, ConfigFinding{
					Code:       CodeUnknownNotifyMode,
					WorkflowID: w.ID,
					Message: fmt.Sprintf("notify.source_pr_comment.mode %q is not in {summary, detailed, failures_only}",
						w.Notify.SourcePRComment.Mode),
				})
			}
		}
		for _, cmd := range w.Nomos.Commands {
			if _, ok := validNomosCommands[cmd]; !ok {
				findings = append(findings, ConfigFinding{
					Code:       CodeUnknownNomosCommand,
					WorkflowID: w.ID,
					Message: fmt.Sprintf("nomos.commands entry %q is not in the recognised CLI subcommand set", cmd),
				})
			}
		}
	}
	return findings, nil
}
