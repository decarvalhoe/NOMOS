package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONMatchesCompatibilitySnapshot(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, sampleReport()); err != nil {
		t.Fatalf("write json: %v", err)
	}
	assertSnapshot(t, "standard.json", out.String())

	var decoded Report
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json output must be machine-readable: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %q, got %q", SchemaVersion, decoded.SchemaVersion)
	}
	if decoded.ReportType != ReportType {
		t.Fatalf("expected report type %q, got %q", ReportType, decoded.ReportType)
	}
}

func TestWriteMarkdownMatchesCompatibilitySnapshot(t *testing.T) {
	var out bytes.Buffer
	if err := WriteMarkdown(&out, sampleReport()); err != nil {
		t.Fatalf("write markdown: %v", err)
	}
	assertSnapshot(t, "standard.md", out.String())
}

func TestNormalizeDoesNotMutateInput(t *testing.T) {
	report := sampleReport()
	_ = Normalize(report)

	if report.Checks[0].ID != "product.ui" {
		t.Fatalf("normalize mutated check order")
	}
	if report.Findings[0].ID != "FINDING-002" {
		t.Fatalf("normalize mutated finding order")
	}
	if report.Evidence[0].ID != "EVIDENCE-002" {
		t.Fatalf("normalize mutated evidence order")
	}
}

func sampleReport() Report {
	return Report{
		GeneratedAt: "2026-04-30T00:00:00Z",
		Run: Run{
			ID:   "run-20260430-000000",
			Mode: "validate",
			Tool: Tool{
				Name:    "nomos",
				Version: "0.1.0",
			},
			Command:    []string{"nomos", "validate", "--report", "nomos-report.json"},
			DurationMS: 1280,
		},
		Project: Project{
			ID:           "insurance-example",
			Name:         "Insurance Example",
			Domain:       "insurance",
			RiskLevel:    "medium",
			ManifestPath: "nomos.project.yaml",
		},
		Summary: Summary{
			CheckCount:           2,
			FindingCount:         1,
			BlockingFindingCount: 0,
			EvidenceCount:        2,
			Coverage: Coverage{
				UnitTotal:         10,
				UnitCovered:       8,
				UnitPartial:       1,
				UnitMissing:       1,
				UnitNotApplicable: 0,
				CoverageRatio:     0.8,
			},
		},
		Verdict: Verdict{
			Status:   "warn",
			Severity: "medium",
			Blocking: false,
			Summary:  "Nomos can validate the repository, but one product surface needs evidence.",
			NextActions: []string{
				"Attach canonical matrix evidence for UI catalogue rendering.",
				"Keep sample data out of product surfaces.",
			},
		},
		Checks: []Check{
			{
				ID:          "product.ui",
				Name:        "Product UI traceability",
				Category:    "product",
				Status:      "warning",
				Severity:    "medium",
				FindingIDs:  []string{"FINDING-002"},
				EvidenceIDs: []string{"EVIDENCE-002"},
			},
			{
				ID:          "sources.manifest",
				Name:        "Source manifest",
				Category:    "sources",
				Status:      "passed",
				Severity:    "info",
				EvidenceIDs: []string{"EVIDENCE-001"},
			},
		},
		Findings: []Finding{
			{
				ID:          "FINDING-002",
				Code:        "NOMOS_PRODUCT_SAMPLE_LEAK",
				Severity:    "medium",
				Status:      "open",
				Blocking:    false,
				Message:     "UI catalogue still renders sample data.",
				Remediation: "Replace sample catalogue with read-model data backed by canonical sources.",
				Target: Target{
					Type: "ui",
					Path: "web/app/page.tsx",
				},
				EvidenceIDs: []string{"EVIDENCE-002"},
			},
		},
		Evidence: []Evidence{
			{
				ID:          "EVIDENCE-002",
				Type:        "code_reference",
				Description: "UI renders hardcoded sample catalogue.",
				Target: &Target{
					Type:    "ui",
					Path:    "web/app/page.tsx",
					Locator: "line 42",
				},
				Producer: "nomos",
			},
			{
				ID:          "EVIDENCE-001",
				Type:        "source_manifest",
				Description: "Source manifest exists.",
				URI:         "docs/canonical/source-manifest.yaml",
				Producer:    "nomos",
			},
		},
	}
}

func assertSnapshot(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join("testdata", "snapshots", name)
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot %s: %v", name, err)
	}
	if got != string(wantBytes) {
		t.Fatalf("snapshot %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, string(wantBytes))
	}
}
