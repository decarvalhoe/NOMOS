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
	GatePass ReleaseGateVerdict = "pass"
	GateFail ReleaseGateVerdict = "fail"
	GateWarn ReleaseGateVerdict = "warn"
)

// ReleaseGateCheck is a single pass/fail check within the gate.
type ReleaseGateCheck struct {
	Name    string             `json:"name"`
	Verdict ReleaseGateVerdict `json:"verdict"`
	Detail  string             `json:"detail"`
}

// ReleaseGateResult holds the full release gate evaluation.
type ReleaseGateResult struct {
	Profile  string             `json:"profile"`
	Verdict  ReleaseGateVerdict `json:"verdict"`
	Checks   []ReleaseGateCheck `json:"checks"`
	Blocking int                `json:"blocking"`
	Warnings int                `json:"warnings"`
}

// ReleaseGateConfig specifies what the gate expects to find.
type ReleaseGateConfig struct {
	Profile                   string
	ArtifactsDir              string
	RequiredNodeTypes         []LawbookNodeType
	RequireStructuralDepth    bool
	RequireFeed               bool
	RequireAttestation        bool
	RequireGovernance         bool
	RequireCertifiedTOC       bool
	RequireStrictFidelityGate bool
	MaxBlockingFindings       int
}

// DefaultRBOKLawbookGateConfig returns the default gate config for the
// rbok-lawbook profile.
func DefaultRBOKLawbookGateConfig(artifactsDir string) ReleaseGateConfig {
	return ReleaseGateConfig{
		Profile:      "rbok-lawbook",
		ArtifactsDir: artifactsDir,
		RequiredNodeTypes: []LawbookNodeType{
			NodeDocument,
			NodeParagraph,
			NodeAlinea,
		},
		RequireStructuralDepth:    true,
		RequireFeed:               true,
		RequireAttestation:        true,
		RequireGovernance:         true,
		RequireCertifiedTOC:       true,
		RequireStrictFidelityGate: true,
		MaxBlockingFindings:       0,
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
	if len(config.RequiredNodeTypes) > 0 || config.RequireStructuralDepth {
		checks = append(checks, checkNodeTypes(absDir, config.RequiredNodeTypes, config.RequireStructuralDepth))
	}

	// Check 3: Attestation present.
	if config.RequireAttestation {
		checks = append(checks, checkAttestation(absDir))
	}

	// Check 4: Governance report present and not blocked.
	if config.RequireGovernance {
		checks = append(checks, checkGovernance(absDir, config.MaxBlockingFindings))
	}

	// Check 5: Certified TOC artifact present and valid.
	if config.RequireCertifiedTOC {
		checks = append(checks, checkCertifiedTOC(absDir))
	}

	// Check 6: Strict fidelity gate must not be silently red.
	if config.RequireStrictFidelityGate {
		checks = append(checks, checkStrictFidelityGate(absDir))
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
	feedFiles := findFeedArtifactFiles(dir)

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

func checkNodeTypes(dir string, required []LawbookNodeType, requireStructuralDepth bool) ReleaseGateCheck {
	found := map[LawbookNodeType]int{}
	feedFiles := findFeedArtifactFiles(dir)
	var nodes []releaseGateNode

	for _, f := range feedFiles {
		nodes = append(nodes, extractReleaseGateNodes(f)...)
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
	structuralDetail := ""
	if requireStructuralDepth {
		var structuralMissing []string
		structuralDetail, structuralMissing = validateStructuralDepth(nodes)
		missing = append(missing, structuralMissing...)
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
	if structuralDetail != "" {
		parts = append(parts, structuralDetail)
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

type releaseGateNode struct {
	NodeID     string          `json:"node_id"`
	DocumentID string          `json:"document_id"`
	NodeType   LawbookNodeType `json:"node_type"`
	ParentID   string          `json:"parent_id"`
	Depth      int             `json:"depth"`
}

func findFeedArtifactFiles(dir string) []string {
	feedFiles, _ := filepath.Glob(filepath.Join(dir, "*-feed.json"))
	if len(feedFiles) == 0 {
		feedFiles, _ = filepath.Glob(filepath.Join(dir, "*.feed.json"))
	}
	if len(feedFiles) == 0 {
		// Also try plain JSON files that contain feed data.
		feedFiles = findJSONFilesWithKey(dir, "nodes")
	}
	return feedFiles
}

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
	return len(extractReleaseGateNodes(path))
}

func extractNodeTypes(path string) map[LawbookNodeType]int {
	counts := map[LawbookNodeType]int{}
	for _, n := range extractReleaseGateNodes(path) {
		if n.NodeType != "" {
			counts[n.NodeType]++
		}
	}
	return counts
}

func extractReleaseGateNodes(path string) []releaseGateNode {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Nodes []releaseGateNode `json:"nodes"`
		Feeds []struct {
			Nodes []releaseGateNode `json:"nodes"`
		} `json:"feeds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	nodes := append([]releaseGateNode{}, doc.Nodes...)
	for _, feed := range doc.Feeds {
		nodes = append(nodes, feed.Nodes...)
	}
	return nodes
}

func validateStructuralDepth(nodes []releaseGateNode) (string, []string) {
	byID := map[string]releaseGateNode{}
	for _, n := range nodes {
		if n.NodeID != "" {
			byID[n.NodeID] = n
		}
	}

	depthCounts := map[int]int{}
	structuralCount := 0
	var failures []string
	for _, n := range nodes {
		if !isStructuralNodeType(n.NodeType) {
			continue
		}
		structuralCount++
		depth, err := parentChainDepth(n, byID)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		depthCounts[depth]++
	}

	if structuralCount == 0 {
		return "", []string{"no structural heading nodes found"}
	}

	maxDepth := 0
	for depth := range depthCounts {
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	if maxDepth == 0 {
		failures = append(failures, "structural heading depth could not be resolved")
	}
	for depth := 1; depth <= maxDepth; depth++ {
		if depthCounts[depth] == 0 {
			failures = append(failures, fmt.Sprintf("missing structural level depth %d before max depth %d", depth, maxDepth))
		}
	}

	var parts []string
	for depth := 1; depth <= maxDepth; depth++ {
		parts = append(parts, fmt.Sprintf("d%d=%d", depth, depthCounts[depth]))
	}
	return fmt.Sprintf("structural_depth=max:%d %s", maxDepth, strings.Join(parts, ", ")), failures
}

func parentChainDepth(node releaseGateNode, byID map[string]releaseGateNode) (int, error) {
	if node.ParentID == "" {
		return 0, fmt.Errorf("structural node %s has no parent", node.NodeID)
	}
	depth := 0
	seen := map[string]bool{node.NodeID: true}
	current := node
	for current.ParentID != "" {
		parent, ok := byID[current.ParentID]
		if !ok {
			return 0, fmt.Errorf("structural node %s references missing parent %s", node.NodeID, current.ParentID)
		}
		if seen[parent.NodeID] {
			return 0, fmt.Errorf("structural node %s has a parent cycle at %s", node.NodeID, parent.NodeID)
		}
		seen[parent.NodeID] = true
		depth++
		if parent.NodeType == NodeDocument {
			return depth, nil
		}
		current = parent
	}
	return 0, fmt.Errorf("structural node %s parent chain does not reach a document", node.NodeID)
}

func isStructuralNodeType(t LawbookNodeType) bool {
	switch t {
	case NodeChapter, NodeSection, NodeSubsection, NodeArticle:
		return true
	default:
		return false
	}
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

func checkCertifiedTOC(dir string) ReleaseGateCheck {
	tocPath := filepath.Join(dir, "rbok-certified-toc.json")
	data, err := os.ReadFile(tocPath)
	if err != nil {
		return ReleaseGateCheck{
			Name:    "certified_toc",
			Verdict: GateFail,
			Detail:  "CERTIFIED_TOC_ARTIFACT_MISSING: rbok-certified-toc.json not found",
		}
	}

	var toc struct {
		Format        string `json:"format"`
		StructureHash string `json:"structure_hash"`
		EntryCount    int    `json:"entry_count"`
	}
	if err := json.Unmarshal(data, &toc); err != nil {
		return ReleaseGateCheck{
			Name:    "certified_toc",
			Verdict: GateFail,
			Detail:  "CERTIFIED_TOC_INVALID: " + err.Error(),
		}
	}

	if toc.EntryCount == 0 {
		return ReleaseGateCheck{
			Name:    "certified_toc",
			Verdict: GateFail,
			Detail:  "CERTIFIED_TOC_EMPTY: certified TOC has 0 entries",
		}
	}

	if toc.StructureHash == "" {
		return ReleaseGateCheck{
			Name:    "certified_toc",
			Verdict: GateFail,
			Detail:  "CERTIFIED_TOC_NO_HASH: structure_hash is empty",
		}
	}

	return ReleaseGateCheck{
		Name:    "certified_toc",
		Verdict: GatePass,
		Detail:  fmt.Sprintf("certified TOC: %d entries, hash=%s", toc.EntryCount, toc.StructureHash[:20]+"..."),
	}
}

func checkStrictFidelityGate(dir string) ReleaseGateCheck {
	gatePath := filepath.Join(dir, "rbok-strict-fidelity-gate.json")
	data, err := os.ReadFile(gatePath)
	if err != nil {
		return ReleaseGateCheck{
			Name:    "strict_fidelity_gate",
			Verdict: GateFail,
			Detail:  "STRICT_FIDELITY_GATE_MISSING: rbok-strict-fidelity-gate.json not found",
		}
	}

	var gate struct {
		Pass          bool `json:"pass"`
		BlockingCount int  `json:"blocking_count"`
		Findings      []struct {
			Code     string `json:"code"`
			Blocking bool   `json:"blocking"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &gate); err != nil {
		return ReleaseGateCheck{
			Name:    "strict_fidelity_gate",
			Verdict: GateFail,
			Detail:  "STRICT_FIDELITY_GATE_INVALID: " + err.Error(),
		}
	}

	blocking := gate.BlockingCount
	if blocking == 0 {
		for _, finding := range gate.Findings {
			if finding.Blocking {
				blocking++
			}
		}
	}
	if !gate.Pass || blocking > 0 {
		return ReleaseGateCheck{
			Name:    "strict_fidelity_gate",
			Verdict: GateFail,
			Detail:  fmt.Sprintf("strict fidelity gate failed: pass=%v, blocking=%d, findings=%d", gate.Pass, blocking, len(gate.Findings)),
		}
	}

	return ReleaseGateCheck{
		Name:    "strict_fidelity_gate",
		Verdict: GatePass,
		Detail:  fmt.Sprintf("strict fidelity gate: pass=true, blocking=0, findings=%d", len(gate.Findings)),
	}
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
