package report

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
)

func TestGenerateFromFullStackCorpus(t *testing.T) {
	dr, err := detect.Detect(filepath.Join("..", "detect", "testdata", "corpus", "fullstack"))
	if err != nil {
		t.Fatalf("detect fullstack corpus: %v", err)
	}

	report := Generate(dr, Options{
		ProjectID:   "fullstack-test",
		ProjectName: "Fullstack Test",
		Domain:      "testing",
		RiskLevel:   "low",
		Mode:        "report",
		Command:     []string{"nomos", "report"},
	})

	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", SchemaVersion, report.SchemaVersion)
	}
	if report.ReportType != ReportType {
		t.Fatalf("expected report_type %q, got %q", ReportType, report.ReportType)
	}
	if report.Run.ID == "" {
		t.Fatal("expected run.id to be populated")
	}
	if report.Run.Tool.Name != "nomos" {
		t.Fatalf("expected run.tool.name %q, got %q", "nomos", report.Run.Tool.Name)
	}
	if report.Project.ID != "fullstack-test" {
		t.Fatalf("expected project.id %q, got %q", "fullstack-test", report.Project.ID)
	}
	if report.Project.Domain != "testing" {
		t.Fatalf("expected project.domain %q, got %q", "testing", report.Project.Domain)
	}
	if report.Verdict.Status == "" {
		t.Fatal("expected verdict.status to be populated")
	}
	if report.Summary.CheckCount != len(report.Checks) {
		t.Fatalf("expected summary.check_count %d to match checks length %d",
			report.Summary.CheckCount, len(report.Checks))
	}
	if report.Summary.FindingCount != len(report.Findings) {
		t.Fatalf("expected summary.finding_count %d to match findings length %d",
			report.Summary.FindingCount, len(report.Findings))
	}
	if report.Summary.EvidenceCount != len(report.Evidence) {
		t.Fatalf("expected summary.evidence_count %d to match evidence length %d",
			report.Summary.EvidenceCount, len(report.Evidence))
	}

	assertHasCheck(t, report, "sources.languages", "passed")
	assertHasCheck(t, report, "sources.surfaces", "passed")
}

func TestGenerateFromNodeTypeScriptFixture(t *testing.T) {
	dr, err := detect.Detect(filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"))
	if err != nil {
		t.Fatalf("detect node-typescript fixture: %v", err)
	}

	report := Generate(dr, Options{
		ProjectID: "nextjs-fixture",
		Domain:    "insurance",
		RiskLevel: "medium",
	})

	assertHasCheck(t, report, "sources.languages", "passed")
	assertHasCheck(t, report, "sources.surfaces", "passed")
	assertHasCheck(t, report, "product.hardcoded-catalogs", "warning")

	if len(report.Findings) == 0 {
		t.Fatal("expected at least one finding for hardcoded catalogue")
	}
	foundCatalog := false
	for _, f := range report.Findings {
		if f.Code == "NOMOS_PRODUCT_HARDCODED_CATALOG" {
			foundCatalog = true
			if f.Status != "open" {
				t.Fatalf("expected finding status %q, got %q", "open", f.Status)
			}
			if len(f.EvidenceIDs) == 0 {
				t.Fatal("expected finding to have evidence IDs")
			}
		}
	}
	if !foundCatalog {
		t.Fatalf("expected NOMOS_PRODUCT_HARDCODED_CATALOG finding in %#v", report.Findings)
	}

	if len(report.Evidence) == 0 {
		t.Fatal("expected at least one evidence item")
	}
	for _, e := range report.Evidence {
		if e.Type != "code_reference" {
			t.Fatalf("expected evidence type %q, got %q", "code_reference", e.Type)
		}
	}

	if report.Verdict.Status != "warn" {
		t.Fatalf("expected verdict status %q, got %q", "warn", report.Verdict.Status)
	}
}

func TestGenerateEmptyRepo(t *testing.T) {
	root := t.TempDir()

	dr, err := detect.Detect(root)
	if err != nil {
		t.Fatalf("detect empty repo: %v", err)
	}

	report := Generate(dr, Options{
		ProjectID: "empty-project",
		Domain:    "testing",
		RiskLevel: "low",
	})

	assertHasCheck(t, report, "sources.languages", "warning")
	assertHasCheck(t, report, "sources.surfaces", "warning")
	if report.Verdict.Status != "warn" {
		t.Fatalf("expected verdict %q for empty repo, got %q", "warn", report.Verdict.Status)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for empty repo, got %d", len(report.Findings))
	}
}

func TestWriteJSONProducesValidReport(t *testing.T) {
	dr, err := detect.Detect(filepath.Join("..", "detect", "testdata", "corpus", "fullstack"))
	if err != nil {
		t.Fatalf("detect fullstack corpus: %v", err)
	}

	report := Generate(dr, Options{
		ProjectID: "json-test",
		Domain:    "testing",
		RiskLevel: "low",
	})

	var buf bytes.Buffer
	if err := WriteJSON(&buf, report); err != nil {
		t.Fatalf("write json: %v", err)
	}

	var decoded NomosReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json report: %v\n%s", err, buf.String())
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", SchemaVersion, decoded.SchemaVersion)
	}
	if decoded.ReportType != ReportType {
		t.Fatalf("expected report_type %q, got %q", ReportType, decoded.ReportType)
	}
	if decoded.Run.Tool.Name != "nomos" {
		t.Fatalf("expected tool name %q, got %q", "nomos", decoded.Run.Tool.Name)
	}
}

func TestSummaryCountsMatchSlices(t *testing.T) {
	dr, err := detect.Detect(filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"))
	if err != nil {
		t.Fatalf("detect node-typescript fixture: %v", err)
	}

	report := Generate(dr, Options{
		ProjectID: "counts-test",
		Domain:    "testing",
		RiskLevel: "low",
	})

	if report.Summary.CheckCount != len(report.Checks) {
		t.Fatalf("check_count %d != len(checks) %d", report.Summary.CheckCount, len(report.Checks))
	}
	if report.Summary.FindingCount != len(report.Findings) {
		t.Fatalf("finding_count %d != len(findings) %d", report.Summary.FindingCount, len(report.Findings))
	}
	if report.Summary.EvidenceCount != len(report.Evidence) {
		t.Fatalf("evidence_count %d != len(evidence) %d", report.Summary.EvidenceCount, len(report.Evidence))
	}

	blockingCount := 0
	for _, f := range report.Findings {
		if f.Blocking {
			blockingCount++
		}
	}
	if report.Summary.BlockingFindingCount != blockingCount {
		t.Fatalf("blocking_finding_count %d != actual %d",
			report.Summary.BlockingFindingCount, blockingCount)
	}
}

func assertHasCheck(t *testing.T, report NomosReport, id string, expectedStatus string) {
	t.Helper()
	for _, c := range report.Checks {
		if c.ID == id {
			if c.Status != expectedStatus {
				t.Fatalf("check %q: expected status %q, got %q", id, expectedStatus, c.Status)
			}
			return
		}
	}
	t.Fatalf("expected check %q in %#v", id, report.Checks)
}
