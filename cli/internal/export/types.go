package export

// --- SPDX 2.3 JSON types ---

// SPDXDocument represents an SPDX 2.3 document in JSON format.
// See: https://spdx.github.io/spdx-spec/v2.3/
type SPDXDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      SPDXCreationInfo   `json:"creationInfo"`
	Packages          []SPDXPackage      `json:"packages"`
	Relationships     []SPDXRelationship `json:"relationships"`
}

type SPDXCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
}

type SPDXPackage struct {
	SPDXID               string            `json:"SPDXID"`
	Name                 string            `json:"name"`
	VersionInfo          string            `json:"versionInfo"`
	DownloadLocation     string            `json:"downloadLocation"`
	FilesAnalyzed        bool              `json:"filesAnalyzed"`
	Supplier             string            `json:"supplier,omitempty"`
	ExternalRefs         []SPDXExternalRef `json:"externalRefs,omitempty"`
	PrimaryPackagePurpose string           `json:"primaryPackagePurpose,omitempty"`
	Comment              string            `json:"comment,omitempty"`
}

type SPDXExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type SPDXRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

// --- CycloneDX 1.5 JSON types ---

// CDXBom represents a CycloneDX 1.5 BOM in JSON format.
// See: https://cyclonedx.org/docs/1.5/json/
type CDXBom struct {
	BomFormat    string          `json:"bomFormat"`
	SpecVersion  string          `json:"specVersion"`
	SerialNumber string          `json:"serialNumber"`
	Version      int             `json:"version"`
	Metadata     CDXMetadata     `json:"metadata"`
	Components   []CDXComponent  `json:"components"`
	Dependencies []CDXDependency `json:"dependencies,omitempty"`
}

type CDXMetadata struct {
	Timestamp string     `json:"timestamp"`
	Tools     []CDXTool  `json:"tools"`
	Component *CDXComponent `json:"component,omitempty"`
}

type CDXTool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CDXComponent struct {
	Type       string            `json:"type"`
	BomRef     string            `json:"bom-ref,omitempty"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	Purl       string            `json:"purl,omitempty"`
	Properties []CDXProperty     `json:"properties,omitempty"`
	ExternalReferences []CDXExternalReference `json:"externalReferences,omitempty"`
}

type CDXProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CDXExternalReference struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type CDXDependency struct {
	Ref       string   `json:"ref"`
	DependsOn []string `json:"dependsOn,omitempty"`
}
