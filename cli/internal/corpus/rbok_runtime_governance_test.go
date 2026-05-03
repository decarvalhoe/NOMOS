package corpus

import (
	"testing"
)

func completeLayer() RuntimeLayer {
	return RuntimeLayer{
		ID:       "layer-law",
		Name:     "Code des assurances",
		Type:     "law",
		Owner:    "legal@corp.com",
		Status:   "active",
		Version:  "2026-Q1",
		Domain:   "assurance",
		Priority: 1,
		Nodes: []RuntimeNode{
			{ID: "N-001", CanonicalRef: "L.113-2", NodeType: "article", SourceHash: "sha256:aaa", Status: "active"},
			{ID: "N-002", CanonicalRef: "L.113-3", NodeType: "article", SourceHash: "sha256:bbb", Status: "active", ParentID: "N-001"},
		},
	}
}

func TestRuntimeGovernanceAdmissible(t *testing.T) {
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{completeLayer()}}
	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictAdmissible {
		t.Fatalf("expected admissible, got %s (findings: %+v)", result.Verdict, result.Findings)
	}
	if result.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", result.TotalFindings)
	}
	if len(result.LayerResults) != 1 {
		t.Fatalf("expected 1 layer result, got %d", len(result.LayerResults))
	}
	if !result.LayerResults[0].Complete {
		t.Fatal("expected complete metadata")
	}
}

func TestRuntimeGovernanceMissingOwnerBlocks(t *testing.T) {
	layer := completeLayer()
	layer.Owner = ""
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked for missing owner, got %s", result.Verdict)
	}
	if result.Blocking == 0 {
		t.Fatal("expected blocking findings")
	}
}

func TestRuntimeGovernanceMissingIDBlocks(t *testing.T) {
	layer := completeLayer()
	layer.ID = ""
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked for missing ID, got %s", result.Verdict)
	}
}

func TestRuntimeGovernanceMissingVersionPartial(t *testing.T) {
	layer := completeLayer()
	layer.Version = ""
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial for missing version, got %s", result.Verdict)
	}
	if result.Blocking != 0 {
		t.Fatal("version missing should not be blocking")
	}
}

func TestRuntimeGovernanceDuplicateNodeIDBlocks(t *testing.T) {
	layer := completeLayer()
	layer.Nodes = append(layer.Nodes, RuntimeNode{
		ID: "N-001", CanonicalRef: "L.113-2-dup", NodeType: "article", SourceHash: "sha256:ccc", Status: "active",
	})
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked for duplicate node, got %s", result.Verdict)
	}
	found := false
	for _, f := range result.Findings {
		if f.Field == "node.id" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected finding for duplicate node.id")
	}
}

func TestRuntimeGovernanceMissingSourceHashBlocks(t *testing.T) {
	layer := completeLayer()
	layer.Nodes[0].SourceHash = ""
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked for missing source_hash, got %s", result.Verdict)
	}
}

func TestRuntimeGovernanceMissingCanonicalRefPartial(t *testing.T) {
	layer := completeLayer()
	layer.Nodes[0].CanonicalRef = ""
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}

	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial for missing canonical_ref, got %s", result.Verdict)
	}
}

func TestRuntimeGovernanceAuthorityConflictBlocks(t *testing.T) {
	layer1 := completeLayer()
	layer1.ID = "layer-1"
	layer1.Nodes[0].AuthorityOf = []string{"target-X"}

	layer2 := RuntimeLayer{
		ID: "layer-2", Name: "Layer 2", Owner: "team@corp.com",
		Status: "active", Version: "1.0", Domain: "assurance",
		Nodes: []RuntimeNode{
			{ID: "N-100", CanonicalRef: "R.1", NodeType: "regulation", SourceHash: "sha256:ddd", Status: "active", AuthorityOf: []string{"target-X"}},
		},
	}

	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer1, layer2}}
	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected blocked for authority conflict, got %s", result.Verdict)
	}
	found := false
	for _, f := range result.Findings {
		if f.Field == "authority_of" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected authority_of conflict finding")
	}
}

func TestRuntimeGovernanceCrossLayerParentWarning(t *testing.T) {
	layer1 := RuntimeLayer{
		ID: "law", Name: "Law", Owner: "owner", Status: "active", Version: "1", Domain: "d",
		Nodes: []RuntimeNode{
			{ID: "LAW-1", CanonicalRef: "L.1", NodeType: "article", SourceHash: "sha256:111", Status: "active"},
		},
	}
	layer2 := RuntimeLayer{
		ID: "ops", Name: "Ops", Owner: "owner", Status: "active", Version: "1", Domain: "d",
		Nodes: []RuntimeNode{
			{ID: "OPS-1", CanonicalRef: "O.1", NodeType: "procedure", SourceHash: "sha256:222", Status: "active", ParentID: "LAW-1"},
		},
	}

	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer1, layer2}}
	result := EvaluateRuntimeGovernance(corpus)

	// Cross-layer parent is a warning, not blocking.
	if result.Verdict == VerdictBlocked {
		t.Fatal("cross-layer parent should not block")
	}
	found := false
	for _, f := range result.Findings {
		if f.Field == "node.parent_id" {
			found = true
			if f.Blocking {
				t.Fatal("cross-layer parent finding should not be blocking")
			}
		}
	}
	if !found {
		t.Fatal("expected cross-layer parent finding")
	}
}

func TestRuntimeGovernanceMultipleLayersAllClean(t *testing.T) {
	layer1 := completeLayer()
	layer1.ID = "law"
	layer2 := RuntimeLayer{
		ID: "ops", Name: "Operations", Owner: "ops@corp.com",
		Status: "active", Version: "1.0", Domain: "assurance",
		Nodes: []RuntimeNode{
			{ID: "OP-1", CanonicalRef: "SOP-1", NodeType: "procedure", SourceHash: "sha256:eee", Status: "active"},
		},
	}

	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer1, layer2}}
	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictAdmissible {
		t.Fatalf("expected admissible, got %s", result.Verdict)
	}
	if len(result.LayerResults) != 2 {
		t.Fatalf("expected 2 layer results, got %d", len(result.LayerResults))
	}
}

func TestRuntimeGovernanceEmptyCorpus(t *testing.T) {
	corpus := RuntimeCorpus{}
	result := EvaluateRuntimeGovernance(corpus)

	if result.Verdict != VerdictAdmissible {
		t.Fatalf("expected admissible for empty corpus, got %s", result.Verdict)
	}
	if result.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", result.TotalFindings)
	}
}

func TestRuntimeGovernanceLayerNodeCount(t *testing.T) {
	layer := completeLayer()
	corpus := RuntimeCorpus{Layers: []RuntimeLayer{layer}}
	result := EvaluateRuntimeGovernance(corpus)

	if result.LayerResults[0].NodeCount != 2 {
		t.Fatalf("expected 2 nodes, got %d", result.LayerResults[0].NodeCount)
	}
}
