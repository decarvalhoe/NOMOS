package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReleaseGateVerdict is the outcome of the release gate evaluation.
type ReleaseGateVerdict string

const (
	GatePass   ReleaseGateVerdict = "pass"
	GateFail   ReleaseGateVerdict = "fail"
	GateWarn   ReleaseGateVerdict = "warn"
)

// ReleaseGateCheck is a single pass/fail check within the gate.
type ReleaseGateCheck struct {
	Name    string             `json:"name"`
	Verdict ReleaseGateVerdict `json:"verdict"`
	Detail  string             `json:"detail"`
}

// ReleaseGateResult holds the full release gate evaluation.
type ReleaseGateResult struct {
	Profile  string              `json:"profile"`
	Verdict  ReleaseGateVerdict  `json:"verdict"`
	Checks   []ReleaseGateCheck  `json:"checks"`
	Blocking int                 `json:"blocking"`
	Warnings int                 `json:"warnings"`
}

// ReleaseGateConfig specifies what the gate expects to find.
type ReleaseGateConfig struct {
	Profile            string
	ArtifactsDir       string
	RequiredNodeTypes  []LawbookNodeType
	RequireFeed        bool
	RequireAttestation bool
	RequireGovernance  bool
	MaxBlockingFindings int
}

// DefaultRBOKLawbookGateConfig returns the default gate config for the
// rbok-lawbook profile.
func DefaultRBOKLawbookGateConfig(artifactsDir string) ReleaseGateConfig {
	return ReleaseGateConfig{
		Profile:      "rbok-lawbook",
		ArtifactsDir: artifactsDir,
		RequiredNodeTypes: []LawbookNodeType{
			NodeDocument,
			NodeArticle,
			NodeParagraph,
			NodeAlinea,
		},
		RequireFeed:         true,
		RequireAttestation:  true,
		RequireGovernance:   true,
		MaxBlockingFindings: 0,
	}
}

// EvaluateReleaseGate runs all release gate checks and returns a verdict.
func EvaluateReleaseGate(config ReleaseGateConfig) (ReleaseGateResult, error) {
	absDir, err := filepath.Abs(config.ArtifactsDir)
	if err != nil {
		return ReleaseGateResult{}, fmt.Errorf("resolve artifacts dir: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil {
		return ReleaseGateResult{}, fmt.Errorf("stat artifacts dir: %w", err)
	}
	if !info.IsDir() {
		return ReleaseGateResult{}, fmt.Errorf("artifacts path %q is not a directory", absDir)
	}

	var checks []ReleaseGateCheck

	// Check 1: Feed artifacts present and valid.
	if config.RequireFeed {
		checks = append(checks, checkFeedArtifacts(absDir))
	}

	// Check 2: Required node types present in feed files.
	if len(config.RequiredNodeTypes) > 0 {
		checks = append(checks, checkNodeTypes(absDir, config.RequiredNodeTypes))
	}

	// Check 3: Attestation present.
	if config.RequireAttestation {
		checks = append(checks, checkAttestation(absDir))
	}

	// Check 4: Governance report present and not blocked.
	if config.RequireGovernance {
		checks = append(checks, checkGovernance(absDir, config.MaxBlockingFindings))
	}

	blocking := 0
	warnings := 0
	for _, c := range checks {
		if c.Verdict == GateFail {
			blocking++
		} else if c.Verdict == GateWarn {
			warnings++
		}
	}

	verdict := GatePass
	if blocking > 0 {
		verdict = GateFail
	} else if warnings > 0 {
		verdict = GateWarn
	}

	return ReleaseGateResult{
		Profile:  config.Profile,
		Verdict:  verdict,
		Checks:   checks,
		Blocking: blocking,
		Warnings: warnings,
	}, nil
}

func checkFeedArtifacts(dir string) ReleaseGateCheck {
	feedFiles, _ := filepath.Glob(filepath.Join(dir, "*-feed.json"))
	if len(feedFiles) == 0 {
		feedFiles, _ = filepath.Glob(filepath.Join(dir, "*.feed.json"))
	}
	if len(feedFiles) == 0 {
		// Also try plain JSON files that contain feed data.
		feedFiles = findJSONFilesWithKey(dir, "nodes")
	}

	if len(feedFiles) == 0 {
		return ReleaseGateCheck{
			Name:    "feed_present",
			Verdict: GateFail,
			Detail:  "no feed artifacts found in " + dir,
		}
	}

	// Validate at least one feed has nodes.
	totalNodes := 0
	for _, f := range feedFiles {
		n := countNodesInFile(f)
		totalNodes += n
	}

	if totalNodes == 0 {
		return ReleaseGateCheck{
			Name:    "feed_present",
			Verdict: GateFail,
			Detail:  fmt.Sprintf("found %d feed file(s) but 0 nodes", len(feedFiles)),
		}
	}

	return ReleaseGateCheck{
		Name:    "feed_present",
		Verdict: GatePass,
		Detail:  fmt.Sprintf("%d feed file(s), %d nodes", len(feedFiles), totalNodes),
	}
}

func checkNodeTypes(dir string, required []LawbookNodeType) ReleaseGateCheck {
	found := map[LawbookNodeType]int{}
	jsonFiles, _ := filepath.Glob(filepath.Join(dir, "*.json"))

	for _, f := range jsonFiles {
		types := extractNodeTypes(f)
		for t, n := range types {
			found[t] += n
		}
	}

	var missing []string
	for _, req := range required {
		if found[req] == 0 {
			missing = append(missing, string(req))
		}
	}

	if len(missing) > 0 {
		return ReleaseGateCheck{
			Name:    "node_types",
			Verdict: GateFail,
			Detail:  "missing required node types: " + strings.Join(missing, ", "),
		}
	}

	var parts []string
	for _, req := range required {
		parts = append(parts, fmt.Sprintf("%s=%d", req, found[req]))
	}
	return ReleaseGateCheck{
		Name:    "node_types",
		Verdict: GatePass,
		Detail:  strings.Join(parts, ", "),
	}
}

func checkAttestation(dir string) ReleaseGateCheck {
	patterns := []string{
		filepath.Join(dir, "*attestation*.json"),
		filepath.Join(dir, "*-attest.json"),
		filepath.Join(dir, "*.att.json"),
	}

	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		if len(matches) > 0 {
			// Validate it has the in-toto type.
			for _, m := range matches {
				if isValidAttestation(m) {
					return ReleaseGateCheck{
						Name:    "attestation",
						Verdict: GatePass,
						Detail:  fmt.Sprintf("valid attestation: %s", filepath.Base(m)),
					}
				}
			}
		}
	}

	return ReleaseGateCheck{
		Name:    "attestation",
		Verdict: GateFail,
		Detail:  "no valid attestation artifact found",
	}
}

func checkGovernance(dir string, maxBlocking int) ReleaseGateCheck {
	patterns := []string{
		filepath.Join(dir, "*governance*.json"),
		filepath.Join(dir, "*governance*.yaml"),
		filepath.Join(dir, "*governance*.yml"),
	}

	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		for _, m := range matches {
			result, err := parseGovernanceFile(m)
			if err != nil {
				continue
			}
			if result.Blocking > maxBlocking {
				return ReleaseGateCheck{
					Name:    "governance",
					Verdict: GateFail,
					Detail:  fmt.Sprintf("governance has %d blocking findings (max %d)", result.Blocking, maxBlocking),
				}
			}
			verdict := GatePass
			detail := fmt.Sprintf("verdict=%s, blocking=%d, total=%d", result.Verdict, result.Blocking, result.TotalFindings)
			if result.Verdict == VerdictPartial {
				verdict = GateWarn
			}
			return ReleaseGateCheck{
				Name:    "governance",
				Verdict: verdict,
				Detail:  detail,
			}
		}
	}

	return ReleaseGateCheck{
		Name:    "governance",
		Verdict: GateFail,
		Detail:  "no governance report found",
	}
}

// --- helpers ---

func findJSONFilesWithKey(dir string, key string) []string {
	allJSON, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	var result []string
	for _, f := range allJSON {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		if _, ok := raw[key]; ok {
			result = append(result, f)
		}
	}
	return result
}

func countNodesInFile(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var doc struct {
		Nodes []json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0
	}
	return len(doc.Nodes)
}

func extractNodeTypes(path string) map[LawbookNodeType]int {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Nodes []struct {
			NodeType string `json:"node_type"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	counts := map[LawbookNodeType]int{}
	for _, n := range doc.Nodes {
		if n.NodeType != "" {
			counts[LawbookNodeType(n.NodeType)]++
		}
	}
	return counts
}

func isValidAttestation(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var stmt struct {
		Type          string `json:"_type"`
		PredicateType string `json:"predicateType"`
	}
	if err := json.Unmarshal(data, &stmt); err != nil {
		return false
	}
	return stmt.Type == InTotoStatementType && stmt.PredicateType != ""
}

func parseGovernanceFile(path string) (GovernanceResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GovernanceResult{}, err
	}
	var result GovernanceResult
	if err := json.Unmarshal(data, &result); err != nil {
		return GovernanceResult{}, err
	}
	return result, nil
}
