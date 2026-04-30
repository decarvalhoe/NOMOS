package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/report"
)

// GenerateSPDX produces an SPDX 2.3 document from a NomosReport.
// The document describes the project as the primary package and
// references the Nomos report as an external artefact.
func GenerateSPDX(nr report.NomosReport) SPDXDocument {
	projectPkgID := "SPDXRef-Package-" + sanitizeID(nr.Project.ID)
	reportPkgID := "SPDXRef-Package-nomos-report"
	namespace := fmt.Sprintf("https://nomos.dev/spdx/%s/%s", sanitizeID(nr.Project.ID), nr.Run.ID)

	downloadLocation := "NOASSERTION"
	if nr.Project.Repository != "" {
		downloadLocation = nr.Project.Repository
	}

	projectPkg := SPDXPackage{
		SPDXID:                projectPkgID,
		Name:                  nr.Project.ID,
		VersionInfo:           nr.Run.Tool.Version,
		DownloadLocation:      downloadLocation,
		FilesAnalyzed:         false,
		PrimaryPackagePurpose: "APPLICATION",
	}
	if nr.Project.Name != "" {
		projectPkg.Name = nr.Project.Name
	}

	reportPkg := SPDXPackage{
		SPDXID:                reportPkgID,
		Name:                  "nomos-report",
		VersionInfo:           nr.SchemaVersion,
		DownloadLocation:      "NOASSERTION",
		FilesAnalyzed:         false,
		PrimaryPackagePurpose: "OTHER",
		Comment:               fmt.Sprintf("Nomos execution report (verdict: %s)", nr.Verdict.Status),
	}
	if nr.Project.ManifestHash != "" {
		reportPkg.ExternalRefs = append(reportPkg.ExternalRefs, SPDXExternalRef{
			ReferenceCategory: "OTHER",
			ReferenceType:     "nomos-manifest-hash",
			ReferenceLocator:  nr.Project.ManifestHash,
		})
	}

	// Add evidence items as external refs on the report package.
	for _, e := range nr.Evidence {
		if e.URI != "" {
			reportPkg.ExternalRefs = append(reportPkg.ExternalRefs, SPDXExternalRef{
				ReferenceCategory: "OTHER",
				ReferenceType:     "nomos-evidence",
				ReferenceLocator:  e.URI,
			})
		}
	}

	return SPDXDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              fmt.Sprintf("nomos-sbom-%s", sanitizeID(nr.Project.ID)),
		DocumentNamespace: namespace,
		CreationInfo: SPDXCreationInfo{
			Created:  nr.GeneratedAt,
			Creators: []string{fmt.Sprintf("Tool: nomos-%s", nr.Run.Tool.Version)},
		},
		Packages: []SPDXPackage{projectPkg, reportPkg},
		Relationships: []SPDXRelationship{
			{
				SPDXElementID:      "SPDXRef-DOCUMENT",
				RelationshipType:   "DESCRIBES",
				RelatedSPDXElement: projectPkgID,
			},
			{
				SPDXElementID:      projectPkgID,
				RelationshipType:   "GENERATED_FROM",
				RelatedSPDXElement: reportPkgID,
			},
		},
	}
}

// WriteSPDX writes the SPDX document as indented JSON.
func WriteSPDX(w io.Writer, doc SPDXDocument) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func sanitizeID(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			return r
		}
		return '-'
	}, s)
}
