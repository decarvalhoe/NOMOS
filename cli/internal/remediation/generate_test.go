package remediation

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
	"github.com/RBOKproject/Nomos/cli/internal/report"
)

func makeReport(t *testing.T, detectRoot string, projectID string) report.NomosReport {
	t.Helper()
	dr, err := detect.Detect(detectRoot)
	if err != nil {
		t.Fatalf("detect %s: %v", detectRoot, err)
	}
	return report.Generate(dr, report.Options{
		ProjectID: projectID,
		Domain:    "testing",
		RiskLevel: "low",
	})
}

func TestGenerateFromNodeTypeScriptFixture(t *testing.T) {
	nr := makeReport(t,
		filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"),
		"nextjs-fixture",
	)

	backlog := Generate(nr)

	if backlog.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", SchemaVersion, backlog.SchemaVersion)
	}
	if backlog.ProjectID != "nextjs-fixture" {
		t.Fatalf("expected project_id %q, got %q", "nextjs-fixture", backlog.ProjectID)
	}
	if backlog.TotalItems == 0 {
		t.Fatal("expected at least one remediation item from fixture with hardcoded catalogue")
	}
	if backlog.TotalItems != len(backlog.Items) {
		t.Fatalf("total_items %d != len(items) %d", backlog.TotalItems, len(backlog.Items))
	}

	foundCatalog := false
	for _, item := range backlog.Items {
		if item.Code == "NOMOS_PRODUCT_HARDCODED_CATALOG" {
			foundCatalog = true
			if item.Title != "Hardcoded catalogue in product code" {
				t.Fatalf("expected human title, got %q", item.Title)
			}
			if item.Target.Type != "code" {
				t.Fatalf("expected target type %q, got %q", "code", item.Target.Type)
			}
			if item.Target.Path == "" {
				t.Fatal("expected target path to be populated")
			}
			if item.Remediation == "" {
				t.Fatal("expected remediation text to be populated")
			}
		}
	}
	if !foundCatalog {
		t.Fatalf("expected NOMOS_PRODUCT_HARDCODED_CATALOG item in %#v", backlog.Items)
	}
}

func TestGenerateSortsByCriticality(t *testing.T) {
	nr := report.NomosReport{
		Project: report.Project{ID: "sort-test"},
		Findings: []report.Finding{
			{
				ID: "F-001", Code: "NOMOS_EVIDENCE_MISSING", Severity: "low",
				Blocking: false, Message: "low sev", Status: "open",
				Target: report.Target{Type: "code"}, EvidenceIDs: []string{},
			},
			{
				ID: "F-002", Code: "NOMOS_SOURCE_HASH_MISMATCH", Severity: "critical",
				Blocking: true, Message: "critical blocker", Status: "open",
				Target: report.Target{Type: "source"}, EvidenceIDs: []string{},
			},
			{
				ID: "F-003", Code: "NOMOS_PRODUCT_HARDCODED_CATALOG", Severity: "medium",
				Blocking: false, Message: "medium sev", Status: "open",
				Target: report.Target{Type: "code"}, EvidenceIDs: []string{},
			},
		},
	}

	backlog := Generate(nr)

	if len(backlog.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(backlog.Items))
	}
	if backlog.Items[0].Severity != "critical" {
		t.Fatalf("expected first item critical, got %q", backlog.Items[0].Severity)
	}
	if backlog.Items[1].Severity != "medium" {
		t.Fatalf("expected second item medium, got %q", backlog.Items[1].Severity)
	}
	if backlog.Items[2].Severity != "low" {
		t.Fatalf("expected third item low, got %q", backlog.Items[2].Severity)
	}
	if backlog.BlockingItems != 1 {
		t.Fatalf("expected 1 blocking item, got %d", backlog.BlockingItems)
	}
}

func TestGenerateEmptyFindings(t *testing.T) {
	nr := report.NomosReport{
		Project:  report.Project{ID: "empty-project"},
		Findings: []report.Finding{},
	}

	backlog := Generate(nr)

	if backlog.TotalItems != 0 {
		t.Fatalf("expected 0 items, got %d", backlog.TotalItems)
	}
	if backlog.BlockingItems != 0 {
		t.Fatalf("expected 0 blocking items, got %d", backlog.BlockingItems)
	}
	if len(backlog.Items) != 0 {
		t.Fatalf("expected empty items slice, got %d", len(backlog.Items))
	}
}

func TestWriteJSON(t *testing.T) {
	nr := makeReport(t,
		filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"),
		"json-export",
	)
	backlog := Generate(nr)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, backlog); err != nil {
		t.Fatalf("write json: %v", err)
	}

	var decoded Backlog
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json: %v\n%s", err, buf.String())
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", SchemaVersion, decoded.SchemaVersion)
	}
	if decoded.TotalItems != backlog.TotalItems {
		t.Fatalf("expected total_items %d, got %d", backlog.TotalItems, decoded.TotalItems)
	}
}

func TestWriteMarkdown(t *testing.T) {
	nr := makeReport(t,
		filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"),
		"md-export",
	)
	backlog := Generate(nr)

	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, backlog); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	md := buf.String()
	for _, expected := range []string{
		"# Remediation Backlog",
		"**Project:** md-export",
		"**Total items:**",
		"## Items",
		"## Details",
		"NOMOS_PRODUCT_HARDCODED_CATALOG",
		"**Remediation:**",
	} {
		if !strings.Contains(md, expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, md)
		}
	}
}

func TestWriteMarkdownEmpty(t *testing.T) {
	backlog := Generate(report.NomosReport{
		Project: report.Project{ID: "empty"},
	})

	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, backlog); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	md := buf.String()
	if !strings.Contains(md, "No remediation items") {
		t.Fatalf("expected empty backlog message, got:\n%s", md)
	}
}

func TestBlockingItemPriorityAboveSameSeverity(t *testing.T) {
	nr := report.NomosReport{
		Project: report.Project{ID: "blocking-test"},
		Findings: []report.Finding{
			{
				ID: "F-001", Code: "NOMOS_EVIDENCE_MISSING", Severity: "high",
				Blocking: false, Message: "non-blocking high", Status: "open",
				Target: report.Target{Type: "code"}, EvidenceIDs: []string{},
			},
			{
				ID: "F-002", Code: "NOMOS_SOURCE_HASH_MISMATCH", Severity: "high",
				Blocking: true, Message: "blocking high", Status: "open",
				Target: report.Target{Type: "source"}, EvidenceIDs: []string{},
			},
		},
	}

	backlog := Generate(nr)

	if len(backlog.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(backlog.Items))
	}
	if !backlog.Items[0].Blocking {
		t.Fatal("expected blocking item first")
	}
	if backlog.Items[1].Blocking {
		t.Fatal("expected non-blocking item second")
	}
}
