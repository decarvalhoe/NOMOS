package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFeedArtifact(t *testing.T, dir string, name string, nodes []LawbookNode) {
	t.Helper()
	feed := struct {
		Nodes []LawbookNode `json:"nodes"`
	}{Nodes: nodes}
	data, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		t.Fatalf("marshal feed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write feed: %v", err)
	}
}

func writeAttestationArtifact(t *testing.T, dir string, name string) {
	t.Helper()
	stmt := CorpusAttestationStatement{
		Type:          InTotoStatementType,
		PredicateType: CorpusPredicateType,
		Subject:       []CorpusSubject{{Name: "test", Digest: map[string]string{"sha256": "abc"}}},
		Predicate:     json.RawMessage(`{"version":"0.1.0"}`),
	}
	data, err := json.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal attestation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write attestation: %v", err)
	}
}

func writeGovernanceArtifact(t *testing.T, dir string, name string, result GovernanceResult) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal governance: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write governance: %v", err)
	}
}

func sampleNodes() []LawbookNode {
	return []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
		{NodeID: "ART-001", NodeType: NodeArticle, ParentID: "DOC-001", Text: "art"},
		{NodeID: "PAR-001", NodeType: NodeParagraph, ParentID: "ART-001", Text: "para"},
		{NodeID: "ALI-001", NodeType: NodeAlinea, ParentID: "PAR-001", Text: "alinea"},
	}
}

func TestEvaluateReleaseGate_AllPass(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GatePass {
		t.Fatalf("expected pass, got %s", result.Verdict)
	}
	if result.Blocking != 0 {
		t.Fatalf("expected 0 blocking, got %d", result.Blocking)
	}
	if len(result.Checks) != 4 {
		t.Fatalf("expected 4 checks, got %d", len(result.Checks))
	}
}

func TestEvaluateReleaseGateReadsNestedMultiFeedNodes(t *testing.T) {
	dir := t.TempDir()
	feed := LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "rbok-lawbook-feed",
		DocumentID:    "DOC-RBOK",
		Domain:        "rbok",
		SourcePath:    "01_referentiel/ref.md",
		SourceHash:    "sha256:aaaa",
		NodeCount:     len(sampleNodes()),
		Nodes:         sampleNodes(),
	}
	assembly := AssembleMultiFeed([]LawbookFeed{feed}, MultiAssembleOptions{})
	if err := WriteMultiFeedArtifacts(assembly, dir); err != nil {
		t.Fatalf("write multi feed: %v", err)
	}
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})

	result, err := EvaluateReleaseGate(DefaultRBOKLawbookGateConfig(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GatePass {
		t.Fatalf("expected nested multi-feed nodes to pass release gate, got %s: %+v", result.Verdict, result.Checks)
	}
}

func TestEvaluateReleaseGateAcceptsSectionAsNormativeHeading(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
		{NodeID: "SEC-001", NodeType: NodeSection, ParentID: "DOC-001", Text: "section"},
		{NodeID: "PAR-001", NodeType: NodeParagraph, ParentID: "SEC-001", Text: "para"},
		{NodeID: "ALI-001", NodeType: NodeAlinea, ParentID: "PAR-001", Text: "alinea"},
	})
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})

	result, err := EvaluateReleaseGate(DefaultRBOKLawbookGateConfig(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GatePass {
		t.Fatalf("expected section-level normative heading to pass, got %s: %+v", result.Verdict, result.Checks)
	}
}

func TestEvaluateReleaseGateValidatesDynamicStructuralDepth(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
		{NodeID: "CH-001", NodeType: NodeChapter, ParentID: "DOC-001", Text: "chapter"},
		{NodeID: "SEC-001", NodeType: NodeSection, ParentID: "CH-001", Text: "section"},
		{NodeID: "ART-001", NodeType: NodeArticle, ParentID: "SEC-001", Text: "article"},
		{NodeID: "PAR-001", NodeType: NodeParagraph, ParentID: "ART-001", Text: "para"},
		{NodeID: "ALI-001", NodeType: NodeAlinea, ParentID: "PAR-001", Text: "alinea"},
	})
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})

	result, err := EvaluateReleaseGate(DefaultRBOKLawbookGateConfig(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GatePass {
		t.Fatalf("expected dynamic depth pass, got %s: %+v", result.Verdict, result.Checks)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "node_types" && strings.Contains(check.Detail, "structural_depth=max:3") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected structural max depth in gate detail: %+v", result.Checks)
	}
}

func TestEvaluateReleaseGateFailsBrokenStructuralParentChain(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
		{NodeID: "SEC-001", NodeType: NodeSection, ParentID: "MISSING", Text: "section"},
		{NodeID: "PAR-001", NodeType: NodeParagraph, ParentID: "SEC-001", Text: "para"},
		{NodeID: "ALI-001", NodeType: NodeAlinea, ParentID: "PAR-001", Text: "alinea"},
	})
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})

	result, err := EvaluateReleaseGate(DefaultRBOKLawbookGateConfig(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected broken structural parent chain to fail, got %s", result.Verdict)
	}
}

func TestEvaluateReleaseGate_MissingFeed(t *testing.T) {
	dir := t.TempDir()
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict: VerdictAdmissible,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail, got %s", result.Verdict)
	}
	if result.Blocking < 1 {
		t.Fatal("expected at least 1 blocking check")
	}
}

func TestEvaluateReleaseGate_MissingNodeTypes(t *testing.T) {
	dir := t.TempDir()
	// Feed with only document nodes, missing article/paragraph/alinea.
	writeFeedArtifact(t, dir, "rbok-feed.json", []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
	})
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict: VerdictAdmissible,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for missing node types, got %s", result.Verdict)
	}

	found := false
	for _, c := range result.Checks {
		if c.Name == "node_types" && c.Verdict == GateFail {
			found = true
		}
	}
	if !found {
		t.Fatal("expected node_types check to fail")
	}
}

func TestEvaluateReleaseGate_MissingAttestation(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict: VerdictAdmissible,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for missing attestation, got %s", result.Verdict)
	}
}

func TestEvaluateReleaseGate_MissingGovernance(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for missing governance, got %s", result.Verdict)
	}
}

func TestEvaluateReleaseGate_GovernanceBlocked(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictBlocked,
		TotalFindings: 3,
		Blocking:      2,
		Findings: []Finding{
			{ID: "F-001", Severity: "critical", Blocking: true, Message: "test"},
			{ID: "F-002", Severity: "critical", Blocking: true, Message: "test2"},
		},
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for blocked governance, got %s", result.Verdict)
	}
}

func TestEvaluateReleaseGate_GovernancePartialWarns(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictPartial,
		TotalFindings: 1,
		Blocking:      0,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateWarn {
		t.Fatalf("expected warn for partial governance, got %s", result.Verdict)
	}
	if result.Warnings != 1 {
		t.Fatalf("expected 1 warning, got %d", result.Warnings)
	}
}

func TestEvaluateReleaseGate_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for empty dir, got %s", result.Verdict)
	}
	if result.Blocking != 4 {
		t.Fatalf("expected 4 blocking checks for empty dir, got %d", result.Blocking)
	}
}

func TestEvaluateReleaseGate_InvalidDir(t *testing.T) {
	_, err := EvaluateReleaseGate(ReleaseGateConfig{
		ArtifactsDir: "/nonexistent/path",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
}

func TestEvaluateReleaseGate_EmptyFeed(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", []LawbookNode{})
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict: VerdictAdmissible,
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for empty feed, got %s", result.Verdict)
	}
}

func TestEvaluateReleaseGate_Profile(t *testing.T) {
	dir := t.TempDir()
	config := DefaultRBOKLawbookGateConfig(dir)
	if config.Profile != "rbok-lawbook" {
		t.Fatalf("expected profile rbok-lawbook, got %s", config.Profile)
	}
}

func TestDefaultRBOKLawbookGateConfig(t *testing.T) {
	config := DefaultRBOKLawbookGateConfig("/tmp/test")
	if len(config.RequiredNodeTypes) != 3 {
		t.Fatalf("expected 3 required node types, got %d", len(config.RequiredNodeTypes))
	}
	if !config.RequireStructuralDepth {
		t.Fatal("expected RequireStructuralDepth to be true")
	}
	if !config.RequireFeed {
		t.Fatal("expected RequireFeed to be true")
	}
	if !config.RequireAttestation {
		t.Fatal("expected RequireAttestation to be true")
	}
	if !config.RequireGovernance {
		t.Fatal("expected RequireGovernance to be true")
	}
	if config.MaxBlockingFindings != 0 {
		t.Fatalf("expected MaxBlockingFindings 0, got %d", config.MaxBlockingFindings)
	}
}
