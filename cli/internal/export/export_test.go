package export

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
		ProjectID:   projectID,
		ProjectName: "Test Project",
		Domain:      "testing",
		RiskLevel:   "low",
	})
}

func fixtureReport(t *testing.T) report.NomosReport {
	t.Helper()
	return makeReport(t,
		filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"),
		"nextjs-fixture",
	)
}

// --- SPDX tests ---

func TestGenerateSPDXStructure(t *testing.T) {
	nr := fixtureReport(t)
	doc := GenerateSPDX(nr)

	if doc.SPDXVersion != "SPDX-2.3" {
		t.Fatalf("expected SPDX-2.3, got %q", doc.SPDXVersion)
	}
	if doc.DataLicense != "CC0-1.0" {
		t.Fatalf("expected CC0-1.0 license, got %q", doc.DataLicense)
	}
	if doc.SPDXID != "SPDXRef-DOCUMENT" {
		t.Fatalf("expected SPDXRef-DOCUMENT, got %q", doc.SPDXID)
	}
	if !strings.Contains(doc.DocumentNamespace, "nextjs-fixture") {
		t.Fatalf("expected namespace to contain project ID, got %q", doc.DocumentNamespace)
	}
	if len(doc.CreationInfo.Creators) == 0 {
		t.Fatal("expected at least one creator")
	}
}

func TestSPDXContainsProjectAndReportPackages(t *testing.T) {
	nr := fixtureReport(t)
	doc := GenerateSPDX(nr)

	if len(doc.Packages) < 2 {
		t.Fatalf("expected at least 2 packages, got %d", len(doc.Packages))
	}

	var foundProject, foundReport bool
	for _, pkg := range doc.Packages {
		if strings.Contains(pkg.SPDXID, "nextjs-fixture") {
			foundProject = true
			if pkg.PrimaryPackagePurpose != "APPLICATION" {
				t.Fatalf("expected APPLICATION purpose, got %q", pkg.PrimaryPackagePurpose)
			}
		}
		if pkg.Name == "nomos-report" {
			foundReport = true
			if !strings.Contains(pkg.Comment, nr.Verdict.Status) {
				t.Fatalf("expected report comment to contain verdict, got %q", pkg.Comment)
			}
		}
	}
	if !foundProject {
		t.Fatal("expected project package in SPDX")
	}
	if !foundReport {
		t.Fatal("expected nomos-report package in SPDX")
	}
}

func TestSPDXRelationships(t *testing.T) {
	nr := fixtureReport(t)
	doc := GenerateSPDX(nr)

	if len(doc.Relationships) < 2 {
		t.Fatalf("expected at least 2 relationships, got %d", len(doc.Relationships))
	}

	var foundDescribes, foundGenerated bool
	for _, rel := range doc.Relationships {
		if rel.RelationshipType == "DESCRIBES" {
			foundDescribes = true
		}
		if rel.RelationshipType == "GENERATED_FROM" {
			foundGenerated = true
		}
	}
	if !foundDescribes {
		t.Fatal("expected DESCRIBES relationship")
	}
	if !foundGenerated {
		t.Fatal("expected GENERATED_FROM relationship linking project to report")
	}
}

func TestWriteSPDXJSON(t *testing.T) {
	nr := fixtureReport(t)
	doc := GenerateSPDX(nr)

	var buf bytes.Buffer
	if err := WriteSPDX(&buf, doc); err != nil {
		t.Fatalf("write spdx: %v", err)
	}

	var decoded SPDXDocument
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode spdx json: %v\n%s", err, buf.String())
	}
	if decoded.SPDXVersion != "SPDX-2.3" {
		t.Fatalf("expected SPDX-2.3 after round-trip, got %q", decoded.SPDXVersion)
	}
}

// --- CycloneDX tests ---

func TestGenerateCycloneDXStructure(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	if bom.BomFormat != "CycloneDX" {
		t.Fatalf("expected CycloneDX, got %q", bom.BomFormat)
	}
	if bom.SpecVersion != "1.5" {
		t.Fatalf("expected 1.5, got %q", bom.SpecVersion)
	}
	if bom.Version != 1 {
		t.Fatalf("expected version 1, got %d", bom.Version)
	}
	if !strings.Contains(bom.SerialNumber, "nextjs-fixture") {
		t.Fatalf("expected serial to contain project ID, got %q", bom.SerialNumber)
	}
}

func TestCycloneDXMetadata(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	if bom.Metadata.Timestamp != nr.GeneratedAt {
		t.Fatalf("expected timestamp %q, got %q", nr.GeneratedAt, bom.Metadata.Timestamp)
	}
	if len(bom.Metadata.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(bom.Metadata.Tools))
	}
	if bom.Metadata.Tools[0].Name != "nomos" {
		t.Fatalf("expected tool name %q, got %q", "nomos", bom.Metadata.Tools[0].Name)
	}
	if bom.Metadata.Component == nil {
		t.Fatal("expected metadata component")
	}
}

func TestCycloneDXContainsProjectAndReportComponents(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	if len(bom.Components) < 2 {
		t.Fatalf("expected at least 2 components, got %d", len(bom.Components))
	}

	var foundProject, foundReport bool
	for _, comp := range bom.Components {
		if comp.Type == "application" {
			foundProject = true
		}
		if comp.Name == "nomos-report" {
			foundReport = true
			// Verify verdict properties exist
			hasVerdict := false
			for _, p := range comp.Properties {
				if p.Name == "nomos:verdict:status" {
					hasVerdict = true
					if p.Value != nr.Verdict.Status {
						t.Fatalf("expected verdict %q, got %q", nr.Verdict.Status, p.Value)
					}
				}
			}
			if !hasVerdict {
				t.Fatal("expected nomos:verdict:status property on report component")
			}
		}
	}
	if !foundProject {
		t.Fatal("expected application component")
	}
	if !foundReport {
		t.Fatal("expected nomos-report component")
	}
}

func TestCycloneDXFindingsAsProperties(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	if len(nr.Findings) == 0 {
		t.Fatal("precondition: expected findings in fixture report")
	}

	var reportComp *CDXComponent
	for i, comp := range bom.Components {
		if comp.Name == "nomos-report" {
			reportComp = &bom.Components[i]
			break
		}
	}
	if reportComp == nil {
		t.Fatal("nomos-report component not found")
	}

	findingProps := 0
	for _, p := range reportComp.Properties {
		if strings.HasPrefix(p.Name, "nomos:finding:") {
			findingProps++
		}
	}
	if findingProps != len(nr.Findings) {
		t.Fatalf("expected %d finding properties, got %d", len(nr.Findings), findingProps)
	}
}

func TestCycloneDXDependencies(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	if len(bom.Dependencies) == 0 {
		t.Fatal("expected at least one dependency")
	}
	dep := bom.Dependencies[0]
	if !strings.Contains(dep.Ref, "nextjs-fixture") {
		t.Fatalf("expected dependency ref to contain project ID, got %q", dep.Ref)
	}
	if len(dep.DependsOn) == 0 {
		t.Fatal("expected project to depend on report")
	}
}

func TestWriteCycloneDXJSON(t *testing.T) {
	nr := fixtureReport(t)
	bom := GenerateCycloneDX(nr)

	var buf bytes.Buffer
	if err := WriteCycloneDX(&buf, bom); err != nil {
		t.Fatalf("write cyclonedx: %v", err)
	}

	var decoded CDXBom
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode cyclonedx json: %v\n%s", err, buf.String())
	}
	if decoded.BomFormat != "CycloneDX" {
		t.Fatalf("expected CycloneDX after round-trip, got %q", decoded.BomFormat)
	}
	if decoded.SpecVersion != "1.5" {
		t.Fatalf("expected 1.5 after round-trip, got %q", decoded.SpecVersion)
	}
}

// --- Cross-format tests ---

func TestSPDXAndCycloneDXFromSameReport(t *testing.T) {
	nr := fixtureReport(t)

	spdx := GenerateSPDX(nr)
	cdx := GenerateCycloneDX(nr)

	// Both should reference same timestamp
	if spdx.CreationInfo.Created != cdx.Metadata.Timestamp {
		t.Fatalf("timestamps differ: SPDX %q vs CDX %q",
			spdx.CreationInfo.Created, cdx.Metadata.Timestamp)
	}

	// Both should have 2 packages/components
	if len(spdx.Packages) != len(cdx.Components) {
		t.Fatalf("package count mismatch: SPDX %d vs CDX %d",
			len(spdx.Packages), len(cdx.Components))
	}
}

func TestEmptyReportExport(t *testing.T) {
	nr := makeReport(t, t.TempDir(), "empty-project")

	spdx := GenerateSPDX(nr)
	cdx := GenerateCycloneDX(nr)

	if len(spdx.Packages) != 2 {
		t.Fatalf("expected 2 SPDX packages for empty report, got %d", len(spdx.Packages))
	}
	if len(cdx.Components) != 2 {
		t.Fatalf("expected 2 CDX components for empty report, got %d", len(cdx.Components))
	}

	// No findings = no finding properties
	for _, comp := range cdx.Components {
		for _, p := range comp.Properties {
			if strings.HasPrefix(p.Name, "nomos:finding:") {
				t.Fatal("expected no finding properties for empty report")
			}
		}
	}
}
