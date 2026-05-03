package fidelity

import (
	"fmt"
	"strings"
)

// PostureAction defines what the LLM must do with a chunk.
type PostureAction string

const (
	ActionCite       PostureAction = "cite"        // Quote verbatim with source reference
	ActionParaphrase PostureAction = "paraphrase"  // Restate in own words with attribution
	ActionSummarize  PostureAction = "summarize"   // Condense with source link
	ActionRefuse     PostureAction = "refuse"      // Decline to answer (out of scope / no source)
	ActionDisclaim   PostureAction = "disclaim"    // Answer with explicit uncertainty disclaimer
	ActionDefer      PostureAction = "defer"       // Redirect to authoritative human
)

// AllActions returns all posture actions.
func AllActions() []PostureAction {
	return []PostureAction{ActionCite, ActionParaphrase, ActionSummarize, ActionRefuse, ActionDisclaim, ActionDefer}
}

// IsValid returns true if the action is recognized.
func (a PostureAction) IsValid() bool {
	for _, valid := range AllActions() {
		if a == valid {
			return true
		}
	}
	return false
}

// PostureRule defines when a particular action applies.
type PostureRule struct {
	ID          string        `json:"id" yaml:"id"`
	Condition   string        `json:"condition" yaml:"condition"`
	Action      PostureAction `json:"action" yaml:"action"`
	Priority    int           `json:"priority" yaml:"priority"` // lower = higher priority
	Rationale   string        `json:"rationale" yaml:"rationale"`
	RequiresCitation bool     `json:"requires_citation" yaml:"requires_citation"`
}

// PostureContract defines the complete conversation posture for a domain.
type PostureContract struct {
	SchemaVersion string        `json:"schema_version" yaml:"schema_version"`
	Domain        string        `json:"domain" yaml:"domain"`
	Rules         []PostureRule `json:"rules" yaml:"rules"`
}

// ChunkContext describes the context of a chunk being used in conversation.
type ChunkContext struct {
	ChunkID          string `json:"chunk_id"`
	GovernanceStatus string `json:"governance_status"` // active, amended, stale, archived
	Confidence       string `json:"confidence"`        // high, medium, low
	Domain           string `json:"domain"`
	InScope          bool   `json:"in_scope"`
	HasSource        bool   `json:"has_source"`
	IsStale          bool   `json:"is_stale"`
	IsRepealed       bool   `json:"is_repealed"`
}

// PostureDecision is the output of evaluating posture for a chunk.
type PostureDecision struct {
	ChunkID    string        `json:"chunk_id"`
	Action     PostureAction `json:"action"`
	RuleID     string        `json:"rule_id"`
	Rationale  string        `json:"rationale"`
	MustCite   bool          `json:"must_cite"`
	Caveats    []string      `json:"caveats,omitempty"`
}

// DefaultPostureContract returns the standard Nomos conversation posture.
func DefaultPostureContract() PostureContract {
	return PostureContract{
		SchemaVersion: "0.1.0",
		Domain:        "*",
		Rules: []PostureRule{
			{
				ID:               "POSTURE-REFUSE-OUT-OF-SCOPE",
				Condition:        "chunk is out of scope",
				Action:           ActionRefuse,
				Priority:         0,
				Rationale:        "LLM must not answer questions outside the canonical scope.",
				RequiresCitation: false,
			},
			{
				ID:               "POSTURE-REFUSE-REPEALED",
				Condition:        "chunk source is repealed",
				Action:           ActionRefuse,
				Priority:         1,
				Rationale:        "Repealed sources must not be presented as current.",
				RequiresCitation: false,
			},
			{
				ID:               "POSTURE-DISCLAIM-STALE",
				Condition:        "chunk source is stale or amended",
				Action:           ActionDisclaim,
				Priority:         2,
				Rationale:        "Stale sources may be outdated; user must be warned.",
				RequiresCitation: true,
			},
			{
				ID:               "POSTURE-DISCLAIM-LOW-CONFIDENCE",
				Condition:        "chunk confidence is low",
				Action:           ActionDisclaim,
				Priority:         3,
				Rationale:        "Low-confidence chunks require explicit uncertainty statement.",
				RequiresCitation: true,
			},
			{
				ID:               "POSTURE-DEFER-NO-SOURCE",
				Condition:        "chunk has no traceable source",
				Action:           ActionDefer,
				Priority:         4,
				Rationale:        "Without source provenance, defer to human authority.",
				RequiresCitation: false,
			},
			{
				ID:               "POSTURE-CITE-HIGH-CONFIDENCE",
				Condition:        "chunk is active, in scope, high confidence",
				Action:           ActionCite,
				Priority:         10,
				Rationale:        "High-confidence active chunks should be quoted with citation.",
				RequiresCitation: true,
			},
			{
				ID:               "POSTURE-PARAPHRASE-MEDIUM",
				Condition:        "chunk is active, in scope, medium confidence",
				Action:           ActionParaphrase,
				Priority:         11,
				Rationale:        "Medium-confidence chunks can be paraphrased with attribution.",
				RequiresCitation: true,
			},
		},
	}
}

// EvaluatePosture determines the appropriate action for a chunk context.
func EvaluatePosture(ctx ChunkContext, contract PostureContract) PostureDecision {
	// Rules are evaluated in priority order (lower = checked first).
	sorted := sortedRules(contract.Rules)

	for _, rule := range sorted {
		if matchesCondition(ctx, rule) {
			return PostureDecision{
				ChunkID:   ctx.ChunkID,
				Action:    rule.Action,
				RuleID:    rule.ID,
				Rationale: rule.Rationale,
				MustCite:  rule.RequiresCitation,
				Caveats:   buildCaveats(ctx, rule),
			}
		}
	}

	// Fallback: if no rule matches, disclaim.
	return PostureDecision{
		ChunkID:   ctx.ChunkID,
		Action:    ActionDisclaim,
		RuleID:    "POSTURE-FALLBACK",
		Rationale: "No explicit rule matched; defaulting to disclaim.",
		MustCite:  true,
		Caveats:   []string{"No matching posture rule found."},
	}
}

// ValidateContract checks posture contract for structural validity.
func ValidateContract(c PostureContract) []string {
	var errs []string
	if c.Domain == "" {
		errs = append(errs, "domain is required")
	}
	if len(c.Rules) == 0 {
		errs = append(errs, "at least one rule is required")
	}

	ids := map[string]bool{}
	for i, r := range c.Rules {
		if r.ID == "" {
			errs = append(errs, fmt.Sprintf("rules[%d].id is required", i))
		} else if ids[r.ID] {
			errs = append(errs, fmt.Sprintf("rules[%d].id %q is duplicated", i, r.ID))
		} else {
			ids[r.ID] = true
		}
		if !r.Action.IsValid() {
			errs = append(errs, fmt.Sprintf("rules[%d].action %q is not valid", i, r.Action))
		}
		if r.Condition == "" {
			errs = append(errs, fmt.Sprintf("rules[%d].condition is required", i))
		}
	}
	return errs
}

func matchesCondition(ctx ChunkContext, rule PostureRule) bool {
	switch {
	case strings.Contains(rule.Condition, "out of scope") && !ctx.InScope:
		return true
	case strings.Contains(rule.Condition, "repealed") && ctx.IsRepealed:
		return true
	case strings.Contains(rule.Condition, "stale") && ctx.IsStale:
		return true
	case strings.Contains(rule.Condition, "low confidence") && ctx.Confidence == "low":
		return true
	case strings.Contains(rule.Condition, "no traceable source") && !ctx.HasSource:
		return true
	case strings.Contains(rule.Condition, "high confidence") && ctx.Confidence == "high" && ctx.InScope && !ctx.IsStale && !ctx.IsRepealed:
		return true
	case strings.Contains(rule.Condition, "medium confidence") && ctx.Confidence == "medium" && ctx.InScope && !ctx.IsStale && !ctx.IsRepealed:
		return true
	default:
		return false
	}
}

func sortedRules(rules []PostureRule) []PostureRule {
	sorted := make([]PostureRule, len(rules))
	copy(sorted, rules)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority < sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func buildCaveats(ctx ChunkContext, rule PostureRule) []string {
	var caveats []string
	if ctx.IsStale {
		caveats = append(caveats, "Source may be outdated.")
	}
	if ctx.Confidence == "low" {
		caveats = append(caveats, "Confidence is low; verify with authoritative source.")
	}
	if !ctx.HasSource {
		caveats = append(caveats, "No traceable source provenance.")
	}
	return caveats
}
