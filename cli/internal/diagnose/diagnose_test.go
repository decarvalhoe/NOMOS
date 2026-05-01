package diagnose

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/output"
)

func TestDiagnoseClassifiesFullStackReferenceAsBlocked(t *testing.T) {
	report := diagnoseReference(t, filepath.Join("..", "detect", "testdata", "corpus", "fullstack"))
	classification := reportClassification(t, report)

	if classification.PreliminaryVerdict != verdictBlocked {
		t.Fatalf("expected verdict %q, got %q", verdictBlocked, classification.PreliminaryVerdict)
	}
	if classification.Confidence != "low" {
		t.Fatalf("expected low confidence, got %q", classification.Confidence)
	}
	assertGapIDs(t, classification.Blockers, []string{"project_manifest", "source_manifest"})
	assertGapIDs(t, classification.MissingEvidence, []string{"canonical_matrix", "owner_or_decision_record", "test_evidence"})
	assertSurfaceNames(t, classification.Surfaces, []string{"api", "data", "docs", "infra", "ui", "worker"})
	if report.Verdict.Status != "blocked" || !report.Verdict.Blocking {
		t.Fatalf("expected blocked report verdict, got %#v", report.Verdict)
	}
}

func TestDiagnoseClassifiesReadyReferenceAsInScope(t *testing.T) {
	report := diagnoseReference(t, filepath.Join("testdata", "corpus", "nomos-ready"))
	classification := reportClassification(t, report)

	if classification.PreliminaryVerdict != verdictInScope {
		t.Fatalf("expected verdict %q, got %q", verdictInScope, classification.PreliminaryVerdict)
	}
	if classification.Confidence != "high" {
		t.Fatalf("expected high confidence, got %q", classification.Confidence)
	}
	if len(classification.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %#v", classification.Blockers)
	}
	if len(classification.MissingEvidence) != 0 {
		t.Fatalf("expected no missing evidence, got %#v", classification.MissingEvidence)
	}
	if report.Verdict.Status != "pass" || report.Verdict.Blocking {
		t.Fatalf("expected pass report verdict, got %#v", report.Verdict)
	}
}

func TestDiagnoseClassifiesDocsOnlyReferenceAsOutOfScope(t *testing.T) {
	report := diagnoseReference(t, filepath.Join("testdata", "corpus", "docs-only"))
	classification := reportClassification(t, report)

	if classification.PreliminaryVerdict != verdictOutOfScope {
		t.Fatalf("expected verdict %q, got %q", verdictOutOfScope, classification.PreliminaryVerdict)
	}
	if classification.Confidence != "medium" {
		t.Fatalf("expected medium confidence, got %q", classification.Confidence)
	}
	if len(classification.Blockers) != 0 || len(classification.MissingEvidence) != 0 {
		t.Fatalf("expected no gaps, got blockers=%#v missing=%#v", classification.Blockers, classification.MissingEvidence)
	}
	if report.Summary.Coverage.UnitNotApplicable != 1 {
		t.Fatalf("expected not applicable coverage, got %#v", report.Summary.Coverage)
	}
}

func TestDiagnoseJSONIsMachineReadable(t *testing.T) {
	report := diagnoseReference(t, filepath.Join("testdata", "corpus", "nomos-ready"))

	var out bytes.Buffer
	if err := output.WriteJSON(&out, report); err != nil {
		t.Fatalf("write json: %v", err)
	}

	var decoded output.Report
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json report: %v\n%s", err, out.String())
	}
	if decoded.SchemaVersion != output.SchemaVersion {
		t.Fatalf("expected schema version %q, got %q", output.SchemaVersion, decoded.SchemaVersion)
	}
	if decoded.Run.Mode != "admission" {
		t.Fatalf("expected admission mode, got %q", decoded.Run.Mode)
	}
}

func diagnoseReference(t *testing.T, root string) output.Report {
	t.Helper()
	report, err := Diagnose(root, Options{
		Now:         time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		ToolVersion: "test",
		Command:     []string{"nomos", "diagnose", "--root", root},
	})
	if err != nil {
		t.Fatalf("diagnose %s: %v", root, err)
	}
	return report
}

func reportClassification(t *testing.T, report output.Report) Classification {
	t.Helper()
	raw, ok := report.Metadata[metadataKey]
	if !ok {
		t.Fatalf("missing %q metadata in %#v", metadataKey, report.Metadata)
	}
	classification, ok := raw.(Classification)
	if !ok {
		t.Fatalf("expected Classification metadata, got %T", raw)
	}
	return classification
}

func assertGapIDs(t *testing.T, gaps []Gap, want []string) {
	t.Helper()
	if len(gaps) != len(want) {
		t.Fatalf("expected gap IDs %#v, got %#v", want, gaps)
	}
	for index, gap := range gaps {
		if gap.ID != want[index] {
			t.Fatalf("expected gap IDs %#v, got %#v", want, gaps)
		}
	}
}

func assertSurfaceNames(t *testing.T, surfaces []SurfaceClassification, want []string) {
	t.Helper()
	if len(surfaces) != len(want) {
		t.Fatalf("expected surfaces %#v, got %#v", want, surfaces)
	}
	for index, surface := range surfaces {
		if surface.Name != want[index] {
			t.Fatalf("expected surfaces %#v, got %#v", want, surfaces)
		}
	}
}
