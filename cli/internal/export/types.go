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
	Checksums            []SPDXChecksum    `json:"checksums,omitempty"`
	ExternalRefs         []SPDXExternalRef `json:"externalRefs,omitempty"`
	PrimaryPackagePurpose string           `json:"primaryPackagePurpose,omitempty"`
	Comment              string            `json:"comment,omitempty"`
}

// SPDXChecksum carries a content digest for a package (SPDX 2.3 §7.10).
type SPDXChecksum struct {
	Algorithm     string `json:"algorithm"`     // e.g. "SHA256"
	ChecksumValue string `json:"checksumValue"` // hex, no algorithm prefix
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
	Hashes     []CDXHash         `json:"hashes,omitempty"`
	Properties []CDXProperty     `json:"properties,omitempty"`
	ExternalReferences []CDXExternalReference `json:"externalReferences,omitempty"`
}

// CDXHash carries a content digest for a component (CycloneDX 1.5 hash object).
type CDXHash struct {
	Alg     string `json:"alg"`     // e.g. "SHA-256"
	Content string `json:"content"` // hex, no algorithm prefix
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
