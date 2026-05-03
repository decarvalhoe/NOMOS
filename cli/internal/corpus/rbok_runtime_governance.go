package corpus

import (
	"fmt"
	"strings"
)

// RuntimeLayer represents one layer in a multi-layer RBOK corpus.
type RuntimeLayer struct {
	ID       string            `json:"id" yaml:"id"`
	Name     string            `json:"name" yaml:"name"`
	Type     string            `json:"type" yaml:"type"` // law, regulation, internal, operational
	Owner    string            `json:"owner" yaml:"owner"`
	Status   string            `json:"status" yaml:"status"` // active, draft, deprecated
	Version  string            `json:"version" yaml:"version"`
	Domain   string            `json:"domain" yaml:"domain"`
	Priority int               `json:"priority" yaml:"priority"`
	Nodes    []RuntimeNode     `json:"nodes" yaml:"nodes"`
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// RuntimeNode is a single node within a layer.
type RuntimeNode struct {
	ID           string   `json:"id" yaml:"id"`
	CanonicalRef string   `json:"canonical_ref" yaml:"canonical_ref"`
	ParentID     string   `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	NodeType     string   `json:"node_type" yaml:"node_type"`
	SourceHash   string   `json:"source_hash" yaml:"source_hash"`
	Status       string   `json:"status" yaml:"status"`
	AuthorityOf  []string `json:"authority_of,omitempty" yaml:"authority_of,omitempty"` // IDs this node is authoritative for
}

// RuntimeCorpus is the complete multi-layer corpus for governance evaluation.
type RuntimeCorpus struct {
	Layers []RuntimeLayer `json:"layers" yaml:"layers"`
}

// RuntimeGovernanceResult holds the evaluation outcome for a multi-layer corpus.
type RuntimeGovernanceResult struct {
	Verdict       string            `json:"verdict"`
	TotalFindings int               `json:"total_findings"`
	Blocking      int               `json:"blocking"`
	LayerResults  []LayerEvalResult `json:"layer_results"`
	Findings      []Finding         `json:"findings"`
}

// LayerEvalResult summarizes one layer's governance state.
type LayerEvalResult struct {
	LayerID  string `json:"layer_id"`
	Verdict  string `json:"verdict"`
	Complete bool   `json:"metadata_complete"`
	NodeCount int   `json:"node_count"`
}

// EvaluateRuntimeGovernance assesses a multi-layer corpus for metadata
// completeness, cross-layer consistency, and authority violations.
func EvaluateRuntimeGovernance(corpus RuntimeCorpus) RuntimeGovernanceResult {
	var findings []Finding
	var layerResults []LayerEvalResult
	findingIdx := 0

	for _, layer := range corpus.Layers {
		layerFindings, complete := evaluateLayerMetadata(layer, &findingIdx)
		findings = append(findings, layerFindings...)

		nodeFindings := evaluateLayerNodes(layer, &findingIdx)
		findings = append(findings, nodeFindings...)

		layerVerdict := VerdictAdmissible
		for _, f := range append(layerFindings, nodeFindings...) {
			if f.Blocking {
				layerVerdict = VerdictBlocked
				break
			}
			if layerVerdict != VerdictBlocked {
				layerVerdict = VerdictPartial
			}
		}
		if len(layerFindings) == 0 && len(nodeFindings) == 0 {
			layerVerdict = VerdictAdmissible
		}

		layerResults = append(layerResults, LayerEvalResult{
			LayerID:   layer.ID,
			Verdict:   layerVerdict,
			Complete:  complete,
			NodeCount: len(layer.Nodes),
		})
	}

	// Cross-layer checks.
	crossFindings := evaluateCrossLayer(corpus, &findingIdx)
	findings = append(findings, crossFindings...)

	// Compute overall verdict.
	blocking := 0
	for _, f := range findings {
		if f.Blocking {
			blocking++
		}
	}

	verdict := VerdictAdmissible
	if blocking > 0 {
		verdict = VerdictBlocked
	} else if len(findings) > 0 {
		verdict = VerdictPartial
	}

	return RuntimeGovernanceResult{
		Verdict:       verdict,
		TotalFindings: len(findings),
		Blocking:      blocking,
		LayerResults:  layerResults,
		Findings:      findings,
	}
}

func evaluateLayerMetadata(layer RuntimeLayer, idx *int) ([]Finding, bool) {
	var findings []Finding
	complete := true

	checks := []struct {
		field string
		value string
		block bool
	}{
		{"id", layer.ID, true},
		{"owner", layer.Owner, true},
		{"status", layer.Status, false},
		{"version", layer.Version, false},
		{"domain", layer.Domain, false},
	}

	for _, check := range checks {
		if strings.TrimSpace(check.value) == "" {
			*idx++
			complete = false
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("RTG-%03d", *idx),
				Severity:   severityFor(check.block),
				Blocking:   check.block,
				SourcePath: layer.ID,
				Field:      check.field,
				Message:    fmt.Sprintf("layer %q missing governance field: %s", layer.ID, check.field),
				Remediation: fmt.Sprintf("Add %s to layer %s metadata.", check.field, layer.ID),
			})
		}
	}

	return findings, complete
}

func evaluateLayerNodes(layer RuntimeLayer, idx *int) []Finding {
	var findings []Finding
	seen := map[string]bool{}

	for _, node := range layer.Nodes {
		// Duplicate node ID.
		if seen[node.ID] {
			*idx++
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("RTG-%03d", *idx),
				Severity:   "high",
				Blocking:   true,
				SourcePath: layer.ID,
				Field:      "node.id",
				Message:    fmt.Sprintf("duplicate node ID %q in layer %q", node.ID, layer.ID),
				Remediation: "Ensure all node IDs are unique within a layer.",
			})
		}
		seen[node.ID] = true

		// Missing source hash.
		if node.SourceHash == "" {
			*idx++
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("RTG-%03d", *idx),
				Severity:   "high",
				Blocking:   true,
				SourcePath: layer.ID,
				Field:      "node.source_hash",
				Message:    fmt.Sprintf("node %q in layer %q has no source_hash", node.ID, layer.ID),
				Remediation: "Compute and record SHA-256 hash of source content.",
			})
		}

		// Missing canonical ref.
		if node.CanonicalRef == "" {
			*idx++
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("RTG-%03d", *idx),
				Severity:   "medium",
				Blocking:   false,
				SourcePath: layer.ID,
				Field:      "node.canonical_ref",
				Message:    fmt.Sprintf("node %q in layer %q has no canonical_ref", node.ID, layer.ID),
				Remediation: "Assign a stable canonical reference for traceability.",
			})
		}
	}

	return findings
}

func evaluateCrossLayer(corpus RuntimeCorpus, idx *int) []Finding {
	var findings []Finding

	// Build authority map: node_id → layer that claims authority.
	authorityMap := map[string]string{}
	for _, layer := range corpus.Layers {
		for _, node := range layer.Nodes {
			for _, target := range node.AuthorityOf {
				if existingLayer, conflict := authorityMap[target]; conflict {
					*idx++
					findings = append(findings, Finding{
						ID:         fmt.Sprintf("RTG-%03d", *idx),
						Severity:   "critical",
						Blocking:   true,
						SourcePath: layer.ID,
						Field:      "authority_of",
						Message: fmt.Sprintf(
							"authority conflict: node %q in layer %q and layer %q both claim authority over %q",
							node.ID, layer.ID, existingLayer, target),
						Remediation: "Resolve authority conflict — only one layer may be authoritative per target.",
					})
				} else {
					authorityMap[target] = layer.ID
				}
			}
		}
	}

	// Check cross-layer reference consistency: nodes referencing parent_id in other layers.
	allNodes := map[string]string{} // node_id → layer_id
	for _, layer := range corpus.Layers {
		for _, node := range layer.Nodes {
			allNodes[node.ID] = layer.ID
		}
	}
	for _, layer := range corpus.Layers {
		for _, node := range layer.Nodes {
			if node.ParentID != "" {
				if parentLayer, exists := allNodes[node.ParentID]; exists {
					if parentLayer != layer.ID {
						*idx++
						findings = append(findings, Finding{
							ID:         fmt.Sprintf("RTG-%03d", *idx),
							Severity:   "medium",
							Blocking:   false,
							SourcePath: layer.ID,
							Field:      "node.parent_id",
							Message: fmt.Sprintf(
								"node %q in layer %q references parent %q from different layer %q",
								node.ID, layer.ID, node.ParentID, parentLayer),
							Remediation: "Verify cross-layer parent reference is intentional and documented.",
						})
					}
				}
			}
		}
	}

	return findings
}

func severityFor(blocking bool) string {
	if blocking {
		return "high"
	}
	return "medium"
}
