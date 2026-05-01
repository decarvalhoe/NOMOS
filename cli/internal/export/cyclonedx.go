package export

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/RBOKproject/Nomos/cli/internal/report"
)

// GenerateCycloneDX produces a CycloneDX 1.5 BOM from a NomosReport.
// The BOM describes the project as the primary component, attaches
// the Nomos report as a second component, and links findings as
// properties.
func GenerateCycloneDX(nr report.NomosReport) CDXBom {
	projectRef := "nomos-project-" + sanitizeID(nr.Project.ID)
	reportRef := "nomos-report-" + sanitizeID(nr.Run.ID)

	projectComponent := CDXComponent{
		Type:    "application",
		BomRef:  projectRef,
		Name:    nr.Project.ID,
		Version: nr.Run.Tool.Version,
	}
	if nr.Project.Name != "" {
		projectComponent.Name = nr.Project.Name
	}
	if nr.Project.Repository != "" {
		projectComponent.ExternalReferences = append(projectComponent.ExternalReferences, CDXExternalReference{
			Type: "vcs",
			URL:  nr.Project.Repository,
		})
	}

	reportComponent := CDXComponent{
		Type:    "data",
		BomRef:  reportRef,
		Name:    "nomos-report",
		Version: nr.SchemaVersion,
		Properties: []CDXProperty{
			{Name: "nomos:verdict:status", Value: nr.Verdict.Status},
			{Name: "nomos:verdict:severity", Value: nr.Verdict.Severity},
			{Name: "nomos:summary:check_count", Value: fmt.Sprintf("%d", nr.Summary.CheckCount)},
			{Name: "nomos:summary:finding_count", Value: fmt.Sprintf("%d", nr.Summary.FindingCount)},
			{Name: "nomos:summary:blocking_finding_count", Value: fmt.Sprintf("%d", nr.Summary.BlockingFindingCount)},
			{Name: "nomos:summary:coverage_ratio", Value: fmt.Sprintf("%.3f", nr.Summary.Coverage.CoverageRatio)},
		},
	}
	if nr.Project.ManifestHash != "" {
		reportComponent.Properties = append(reportComponent.Properties, CDXProperty{
			Name:  "nomos:manifest_hash",
			Value: nr.Project.ManifestHash,
		})
	}

	// Attach findings as properties on the report component.
	for _, f := range nr.Findings {
		reportComponent.Properties = append(reportComponent.Properties, CDXProperty{
			Name:  fmt.Sprintf("nomos:finding:%s", f.ID),
			Value: fmt.Sprintf("[%s] %s: %s", f.Severity, f.Code, f.Message),
		})
	}

	// Attach evidence URIs as external references.
	for _, e := range nr.Evidence {
		if e.URI != "" {
			reportComponent.ExternalReferences = append(reportComponent.ExternalReferences, CDXExternalReference{
				Type: "other",
				URL:  e.URI,
			})
		}
	}

	return CDXBom{
		BomFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: fmt.Sprintf("urn:uuid:nomos:%s:%s", sanitizeID(nr.Project.ID), nr.Run.ID),
		Version:      1,
		Metadata: CDXMetadata{
			Timestamp: nr.GeneratedAt,
			Tools: []CDXTool{
				{
					Vendor:  "Nomos",
					Name:    "nomos",
					Version: nr.Run.Tool.Version,
				},
			},
			Component: &projectComponent,
		},
		Components: []CDXComponent{projectComponent, reportComponent},
		Dependencies: []CDXDependency{
			{
				Ref:       projectRef,
				DependsOn: []string{reportRef},
			},
		},
	}
}

// WriteCycloneDX writes the CycloneDX BOM as indented JSON.
func WriteCycloneDX(w io.Writer, bom CDXBom) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(bom)
}
