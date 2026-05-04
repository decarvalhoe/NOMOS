package githubworkflow

import (
	"path/filepath"
	"sort"
	"strings"
)

// DiffPlan is the wire format produced by `nomos github plan`. Consumers
// (the reusable workflow #389, the publisher #390, the source PR commenter
// #391) read this JSON to decide which workflows to execute and which
// changed paths to surface in PR comments.
type DiffPlan struct {
	SchemaVersion          string             `json:"schema_version"`
	GeneratedAt            string             `json:"generated_at"`
	ConfigPath             string             `json:"config_path"`
	ChangedPathCount       int                `json:"changed_path_count"`
	Impacted               []ImpactedWorkflow `json:"impacted"`
	Skipped                []SkippedWorkflow  `json:"skipped"`
	IgnoredGeneratedPaths  []string           `json:"ignored_generated_paths"`
}

// DiffPlanSchemaVersion identifies the JSON shape of DiffPlan. Bump on
// breaking shape changes.
const DiffPlanSchemaVersion = "ngw-diff-v1"

// ImpactedWorkflow names a workflow whose source.paths intersect the
// non-generated changed paths.
type ImpactedWorkflow struct {
	WorkflowID   string   `json:"workflow_id"`
	ScopeID      string   `json:"scope_id"`
	ScopePaths   []string `json:"scope_paths"`
	MatchedPaths []string `json:"matched_paths"`
}

// SkippedWorkflow names a workflow that was considered but not impacted,
// with a stable Reason code so consumers can route differently.
type SkippedWorkflow struct {
	WorkflowID string `json:"workflow_id"`
	Reason     string `json:"reason"`
	ReasonText string `json:"reason_text"`
}

// Stable skip reason codes.
const (
	ReasonNoPathsMatch       = "NGW_DIFF_NO_PATHS_MATCH"
	ReasonAllPathsGenerated  = "NGW_DIFF_ALL_PATHS_GENERATED"
)

// PlanScopedDiff is the pure planner. It takes a parsed config and a slice
// of changed paths (typically from `git diff --name-only`) and returns the
// deterministic DiffPlan.
//
// Algorithm:
//  1. Build the union of every workflow's output.path (the "generated set").
//  2. Partition changed paths into ignored (under any generated path) and
//     scrutinised (the rest).
//  3. For each workflow, match scrutinised paths against its source.paths
//     glob list. If any match, emit an ImpactedWorkflow with the matched
//     paths sorted ASC. Otherwise emit a SkippedWorkflow with the most
//     specific stable reason.
//
// The function is pure: no I/O, no mutation of inputs. GeneratedAt is set
// to the empty string here; the cmd layer fills it from --frozen-time or
// time.Now().UTC().
func PlanScopedDiff(cfg WorkflowConfig, changedPaths []string) DiffPlan {
	plan := DiffPlan{
		SchemaVersion:         DiffPlanSchemaVersion,
		ChangedPathCount:      len(changedPaths),
		Impacted:              []ImpactedWorkflow{},
		Skipped:               []SkippedWorkflow{},
		IgnoredGeneratedPaths: []string{},
	}

	outputs := make([]string, 0, len(cfg.Workflows))
	for _, w := range cfg.Workflows {
		if op := strings.TrimSpace(w.Output.Path); op != "" {
			outputs = append(outputs, op)
		}
	}

	scrutinised := make([]string, 0, len(changedPaths))
	ignoredSet := map[string]struct{}{}
	for _, raw := range changedPaths {
		p := normalisePath(raw)
		if p == "" {
			continue
		}
		if anyOutputContains(p, outputs) {
			ignoredSet[p] = struct{}{}
			continue
		}
		scrutinised = append(scrutinised, p)
	}
	for p := range ignoredSet {
		plan.IgnoredGeneratedPaths = append(plan.IgnoredGeneratedPaths, p)
	}
	sort.Strings(plan.IgnoredGeneratedPaths)

	for _, w := range cfg.Workflows {
		matched := matchSourcePaths(w.Source.Paths, scrutinised)
		ownOutput := strings.TrimSpace(w.Output.Path)
		if len(matched) == 0 {
			plan.Skipped = append(plan.Skipped, SkippedWorkflow{
				WorkflowID: w.ID,
				Reason:     ReasonNoPathsMatch,
				ReasonText: "no changed path falls inside this workflow's source.paths",
			})
			continue
		}
		if ownOutput != "" && allPathsUnderOutput(matched, ownOutput) {
			plan.Skipped = append(plan.Skipped, SkippedWorkflow{
				WorkflowID: w.ID,
				Reason:     ReasonAllPathsGenerated,
				ReasonText: "all matched paths fall inside this workflow's own output.path (loop guard)",
			})
			continue
		}
		sort.Strings(matched)
		scope := append([]string{}, w.Source.Paths...)
		sort.Strings(scope)
		plan.Impacted = append(plan.Impacted, ImpactedWorkflow{
			WorkflowID:   w.ID,
			ScopeID:      w.ID,
			ScopePaths:   scope,
			MatchedPaths: matched,
		})
	}

	sort.SliceStable(plan.Impacted, func(i, j int) bool {
		return plan.Impacted[i].WorkflowID < plan.Impacted[j].WorkflowID
	})
	sort.SliceStable(plan.Skipped, func(i, j int) bool {
		return plan.Skipped[i].WorkflowID < plan.Skipped[j].WorkflowID
	})
	return plan
}

// normalisePath trims whitespace, normalises separators to forward slashes,
// and strips a leading "./" so matching is consistent across platforms.
func normalisePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

// anyOutputContains returns true when path falls under at least one of the
// supplied output prefix paths.
func anyOutputContains(path string, outputs []string) bool {
	for _, op := range outputs {
		if pathUnderPrefix(path, op) {
			return true
		}
	}
	return false
}

// allPathsUnderOutput returns true when every path in paths falls under
// ownOutput. Used as the secondary loop-guard skip reason.
func allPathsUnderOutput(paths []string, ownOutput string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, p := range paths {
		if !pathUnderPrefix(p, ownOutput) {
			return false
		}
	}
	return true
}

// pathUnderPrefix returns true when path is the prefix itself or lives under
// prefix (treating prefix as a directory). Trailing slashes on prefix are
// tolerated.
func pathUnderPrefix(path, prefix string) bool {
	prefix = strings.TrimSuffix(filepath.ToSlash(prefix), "/")
	if prefix == "" {
		return false
	}
	path = strings.TrimSuffix(filepath.ToSlash(path), "/")
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// matchSourcePaths returns the subset of paths that match at least one of
// the supplied glob patterns. Order is preserved from paths; the caller
// re-sorts.
func matchSourcePaths(patterns, paths []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	var out []string
	seen := map[string]struct{}{}
	for _, p := range paths {
		for _, pat := range patterns {
			if matchGlob(pat, p) {
				if _, dup := seen[p]; !dup {
					seen[p] = struct{}{}
					out = append(out, p)
				}
				break
			}
		}
	}
	return out
}

// matchGlob is a small, dependency-free glob matcher supporting `*`, `?`,
// and `**`. Semantics:
//
//   - `**` matches any path (zero or more segments, including across `/`).
//   - `prefix/**` matches `prefix` and any path under it.
//   - `**/suffix` matches any path that ends with `suffix` at a `/`
//     boundary (suffix itself may contain `*` and `?`).
//   - `prefix/**/suffix` matches paths under `prefix` whose tail matches
//     `suffix`.
//   - Patterns without `**` are matched with `path/filepath.Match`
//     semantics (`*` does not cross `/`; `?` matches any single non-`/`
//     rune).
//
// Pattern and path are normalised to forward slashes before matching.
// At most one `**` segment is supported in a single pattern; nested `**`
// is accepted at the top level (we recursively walk the tail) but not
// inside a sub-pattern.
func matchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(pattern), "./"))
	path = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(path), "./"))
	if pattern == "" {
		return path == ""
	}
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}
	if pattern == "**" {
		return true
	}
	idx := strings.Index(pattern, "**")
	prefix := strings.TrimSuffix(pattern[:idx], "/")
	suffix := strings.TrimPrefix(pattern[idx+2:], "/")
	if prefix != "" {
		if path != prefix && !strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	if suffix == "" {
		return true
	}
	rest := path
	if prefix != "" {
		if path == prefix {
			rest = ""
		} else {
			rest = strings.TrimPrefix(path, prefix+"/")
		}
	}
	for i := 0; i <= len(rest); i++ {
		if i > 0 && rest[i-1] != '/' {
			continue
		}
		tail := rest[i:]
		if ok, _ := filepath.Match(suffix, tail); ok {
			return true
		}
		if strings.Contains(suffix, "**") {
			if matchGlob(suffix, tail) {
				return true
			}
		}
	}
	return false
}
