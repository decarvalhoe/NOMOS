package diagnose

import (
	"path/filepath"
	"testing"
)

func TestDiagnoseNomosRepoPassesAdmission(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	report := diagnoseReference(t, repoRoot)
	classification := reportClassification(t, report)

	if classification.PreliminaryVerdict != verdictInScope {
		t.Fatalf("expected verdict %q, got %q (blockers=%v missing=%v)",
			verdictInScope, classification.PreliminaryVerdict,
			classification.Blockers, classification.MissingEvidence)
	}
	if report.Verdict.Status != "pass" {
		t.Fatalf("expected pass verdict, got %q: %s", report.Verdict.Status, report.Verdict.Summary)
	}
	if report.Verdict.Blocking {
		t.Fatal("expected non-blocking verdict")
	}
	if len(classification.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", classification.Blockers)
	}
	if len(classification.MissingEvidence) != 0 {
		t.Fatalf("expected no missing evidence, got %v", classification.MissingEvidence)
	}
}
