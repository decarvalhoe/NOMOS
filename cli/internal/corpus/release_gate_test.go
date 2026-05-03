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

func writeMultiFeedArtifact(t *testing.T, dir string, name string, nodes []LawbookNode) {
	t.Helper()
	assembly := MultiFeedAssembly{
		Format:        FeedFormatVersion,
		DocumentCount: 1,
		TotalNodes:    len(nodes),
		Feeds: []LawbookFeed{
			{
				SchemaVersion: "0.1.0",
				FeedID:        "rbok-feed",
				DocumentID:    "RBOK-DOC",
				NodeCount:     len(nodes),
				Nodes:         nodes,
			},
		},
	}
	data, err := json.MarshalIndent(assembly, "", "  ")
	if err != nil {
		t.Fatalf("marshal multi feed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write multi feed: %v", err)
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

func writeDiagnoseGovernanceArtifact(t *testing.T, dir string, name string, verdict DiagnoseVerdict) {
	t.Helper()
	data, err := json.Marshal(verdict)
	if err != nil {
		t.Fatalf("marshal diagnose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("write diagnose: %v", err)
	}
}

func sampleNodes() []LawbookNode {
	return []LawbookNode{
		{NodeID: "DOC-001", NodeType: NodeDocument, Text: "doc"},
		{NodeID: "ART-001", NodeType: NodeArticle, Text: "art"},
		{NodeID: "PAR-001", NodeType: NodeParagraph, Text: "para"},
		{NodeID: "ALI-001", NodeType: NodeAlinea, Text: "alinea"},
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

func TestEvaluateReleaseGate_AcceptsMultiFeedAssembly(t *testing.T) {
	dir := t.TempDir()
	writeMultiFeedArtifact(t, dir, "rbok-lawbook-feed.json", sampleNodes())
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
		t.Fatalf("expected pass for multi-feed assembly, got %s: %+v", result.Verdict, result.Checks)
	}
}

func TestEvaluateReleaseGate_AcceptsProfileDiagnoseGovernance(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeDiagnoseGovernanceArtifact(t, dir, "rbok-governance.json", DiagnoseVerdict{
		Profile:    ProfileRBOKLawbook,
		Verdict:    VerdictAdmissible,
		Confidence: "high",
		Summary:    "ready",
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GatePass {
		t.Fatalf("expected pass for profile diagnose governance, got %s: %+v", result.Verdict, result.Checks)
	}
	for _, check := range result.Checks {
		if check.Name == "governance" && !strings.Contains(check.Detail, VerdictAdmissible) {
			t.Fatalf("expected governance detail to include diagnose verdict, got %q", check.Detail)
		}
	}
}

func TestEvaluateReleaseGate_NodeTypesIgnoreNonFeedArtifacts(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeGovernanceArtifact(t, dir, "rbok-governance.json", GovernanceResult{
		Verdict:       VerdictAdmissible,
		TotalFindings: 0,
		Blocking:      0,
	})
	traceabilityRows := []TraceabilityEntry{
		{NodeID: "DOC-001", NodeType: string(NodeDocument)},
		{NodeID: "ART-001", NodeType: string(NodeArticle)},
	}
	data, err := json.Marshal(traceabilityRows)
	if err != nil {
		t.Fatalf("marshal traceability: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rbok-traceability-matrix.json"), data, 0o644); err != nil {
		t.Fatalf("write traceability: %v", err)
	}
	engineImport := struct {
		Nodes []LawbookNode `json:"nodes"`
	}{Nodes: sampleNodes()}
	data, err = json.Marshal(engineImport)
	if err != nil {
		t.Fatalf("marshal engine import: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rbok-engine-import.json"), data, 0o644); err != nil {
		t.Fatalf("write engine import: %v", err)
	}
	profileOutput := struct {
		Sections map[string]any `json:"sections"`
	}{
		Sections: map[string]any{
			string(OutputFeed): struct {
				Feeds []struct {
					Nodes []LawbookNode `json:"nodes"`
				} `json:"feeds"`
			}{
				Feeds: []struct {
					Nodes []LawbookNode `json:"nodes"`
				}{{Nodes: sampleNodes()}},
			},
		},
	}
	data, err = json.Marshal(profileOutput)
	if err != nil {
		t.Fatalf("marshal profile output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile-output.json"), data, 0o644); err != nil {
		t.Fatalf("write profile output: %v", err)
	}

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, check := range result.Checks {
		if check.Name == "node_types" && !strings.Contains(check.Detail, "document=1") {
			t.Fatalf("expected node type counts from feed only, got %q", check.Detail)
		}
	}
}

func TestEvaluateReleaseGate_ProfileDiagnoseBlockersFail(t *testing.T) {
	dir := t.TempDir()
	writeFeedArtifact(t, dir, "rbok-feed.json", sampleNodes())
	writeAttestationArtifact(t, dir, "rbok-attestation.json")
	writeDiagnoseGovernanceArtifact(t, dir, "rbok-governance.json", DiagnoseVerdict{
		Profile:  ProfileRBOKLawbook,
		Verdict:  VerdictBlocked,
		Blockers: []string{"blocked binary", "missing primary"},
		Summary:  "blocked",
	})

	config := DefaultRBOKLawbookGateConfig(dir)
	result, err := EvaluateReleaseGate(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != GateFail {
		t.Fatalf("expected fail for profile diagnose blockers, got %s: %+v", result.Verdict, result.Checks)
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
	if len(config.RequiredNodeTypes) != 4 {
		t.Fatalf("expected 4 required node types, got %d", len(config.RequiredNodeTypes))
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
