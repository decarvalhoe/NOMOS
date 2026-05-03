package corpus

import (
	"path"
	"strings"
)

// SourceRole describes the function of a source file in the RBOK lawbook.
type SourceRole string

const (
	RoleLawbook     SourceRole = "lawbook"
	RoleSupporting  SourceRole = "supporting"
	RoleEvidence    SourceRole = "evidence"
	RoleOperational SourceRole = "operational"
	RoleSchema      SourceRole = "schema"
	RoleReference   SourceRole = "reference"
	RoleDerived     SourceRole = "derived"
	RoleArchive     SourceRole = "archive"
	RoleOutOfScope  SourceRole = "out_of_scope"
)

// RBOKSourceClassification is the policy result for a single RBOK source file.
type RBOKSourceClassification struct {
	Path        string     `json:"path"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	Role        SourceRole `json:"role"`
	SourceClass string     `json:"source_class"`
	CorpusLayer string     `json:"corpus_layer"`
	Authority   string     `json:"authority"`
	AllowedUses []string   `json:"allowed_uses"`
	Reason      string     `json:"reason"`
}

// ClassifyRBOKSource applies the RBOK lawbook source policy to a file path.
// The path should be relative to the corpus root (forward-slash separated).
func ClassifyRBOKSource(filePath string) RBOKSourceClassification {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.Split(normalized, "/")
	policyParts := rbokPolicyParts(parts)
	base := path.Base(normalized)
	ext := strings.ToLower(path.Ext(normalized))
	lower := strings.ToLower(normalized)

	// Out-of-scope: OS artifacts, scripts, tooling.
	if isOutOfScope(base, lower, parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "out_of_scope",
			Status:      "out_of_scope",
			Role:        RoleOutOfScope,
			SourceClass: "out_of_scope",
			CorpusLayer: "out_of_scope",
			Authority:   "none",
			AllowedUses: nil,
			Reason:      "OS artifact, script, or tooling file",
		}
	}

	if isArchive(policyParts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "archive",
			Status:      "out_of_scope",
			Role:        RoleArchive,
			SourceClass: "archive",
			CorpusLayer: "archive",
			Authority:   "none",
			AllowedUses: nil,
			Reason:      "archive excluded by default",
		}
	}

	// Derived: generated files, German translations, testdata.
	if isDerived(lower, ext, parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "derived",
			Status:      "active",
			Role:        RoleDerived,
			SourceClass: "derived",
			CorpusLayer: corpusLayerForParts(policyParts),
			Authority:   "derived",
			AllowedUses: []string{"citation_internal"},
			Reason:      "generated, translated, or test fixture",
		}
	}

	// Schema: CUE/JSON schemas in 98_schemas.
	if isSchema(policyParts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "reference",
			Status:      "active",
			Role:        RoleSchema,
			SourceClass: "schema",
			CorpusLayer: "schema",
			Authority:   "reference",
			AllowedUses: []string{"structured_contract", "citation_internal"},
			Reason:      "CUE/JSON schema from 98_schemas",
		}
	}

	// Reference: original PDFs and initial source documents.
	if isReference(lower, policyParts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "reference",
			Status:      "active",
			Role:        RoleReference,
			SourceClass: "reference",
			CorpusLayer: "reference_original",
			Authority:   "reference",
			AllowedUses: []string{"human_review_only", "citation_internal"},
			Reason:      "original PDF or initial source document",
		}
	}

	if isRuntimeBinding(policyParts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "primary",
			Status:      "active",
			Role:        RoleLawbook,
			SourceClass: "runtime_binding",
			CorpusLayer: "runtime_binding",
			Authority:   "primary",
			AllowedUses: []string{"structured_contract", "vector_index", "runtime_binding", "citation_internal", "golden_case"},
			Reason:      "canonical runtime binding source",
		}
	}

	// Primary: canonical lawbook content (00_meta, 01_referentiel, 02_domaines, 03_parcours).
	if isPrimary(policyParts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "primary",
			Status:      "active",
			Role:        RoleLawbook,
			SourceClass: "canonical_corpus",
			CorpusLayer: "canonical_core",
			Authority:   "primary",
			AllowedUses: []string{"structured_contract", "vector_index", "citation_internal", "golden_case"},
			Reason:      "canonical lawbook source",
		}
	}

	if classification, ok := classifySupportingRoot(parts, filePath); ok {
		return classification
	}

	// Default: secondary.
	return RBOKSourceClassification{
		Path:        filePath,
		Priority:    "secondary",
		Status:      "active",
		Role:        RoleLawbook,
		SourceClass: "supporting_context",
		CorpusLayer: corpusLayerForParts(policyParts),
		Authority:   "supporting",
		AllowedUses: []string{"structured_contract", "citation_internal"},
		Reason:      "lawbook source outside primary directories",
	}
}

func rbokPolicyParts(parts []string) []string {
	if len(parts) > 0 && strings.EqualFold(parts[0], "01_rbok") {
		return parts[1:]
	}
	return parts
}

func isPrimary(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	top := strings.ToLower(parts[0])
	switch {
	case strings.HasPrefix(top, "00_meta"):
		return true
	case strings.HasPrefix(top, "01_referentiel"):
		return true
	case strings.HasPrefix(top, "02_domaines"):
		return true
	case strings.HasPrefix(top, "03_parcours"):
		return true
	}
	return false
}

func isRuntimeBinding(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	return strings.HasPrefix(strings.ToLower(parts[0]), "03_parcours")
}

func isSchema(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	top := strings.ToLower(parts[0])
	return strings.HasPrefix(top, "98_schemas") || strings.HasPrefix(top, "98_schema") ||
		strings.HasPrefix(top, "98_sch")
}

func isReference(lower string, parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	top := strings.ToLower(parts[0])
	if strings.HasPrefix(top, "99_rbok") || strings.HasPrefix(top, "99_initial") {
		return true
	}
	if strings.HasSuffix(lower, ".pdf") && strings.Contains(lower, "initial") {
		return true
	}
	return false
}

func isDerived(lower, ext string, parts []string) bool {
	// German translations.
	if strings.HasSuffix(lower, ".de.yaml") || strings.HasSuffix(lower, ".de.yml") ||
		strings.HasSuffix(lower, ".de.md") || strings.HasSuffix(lower, ".de.json") {
		return true
	}
	if strings.Contains(lower, ".testdata.") || strings.Contains(lower, ".fixture.") {
		return true
	}
	// Generated files.
	if strings.Contains(lower, "generated") || strings.Contains(lower, "_gen.") ||
		strings.Contains(lower, ".gen.") {
		return true
	}
	// Testdata directories.
	for _, p := range parts {
		if strings.ToLower(p) == "testdata" || strings.ToLower(p) == "test_data" ||
			strings.ToLower(p) == "fixtures" {
			return true
		}
	}
	// Build output.
	if ext == ".min.js" || ext == ".min.css" {
		return true
	}
	return false
}

func isArchive(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	top := strings.ToLower(parts[0])
	return strings.HasPrefix(top, "99_archive")
}

func isOutOfScope(base, lower string, parts []string) bool {
	// OS artifacts.
	lowerBase := strings.ToLower(base)
	switch lowerBase {
	case ".ds_store", "thumbs.db", "desktop.ini", ".gitkeep":
		return true
	}
	// Script and tooling directories.
	for _, p := range parts {
		lp := strings.ToLower(p)
		if lp == "scripts" || lp == "script" || lp == ".git" || lp == ".github" ||
			lp == "tools" || lp == "uploads" ||
			lp == "node_modules" || lp == "__pycache__" || lp == ".venv" {
			return true
		}
	}
	// Script files at any level.
	switch strings.ToLower(path.Ext(base)) {
	case ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd":
		return true
	}
	// Lock files.
	if lowerBase == "package-lock.json" || lowerBase == "yarn.lock" ||
		lowerBase == "pnpm-lock.yaml" || lowerBase == "poetry.lock" {
		return true
	}
	return false
}

func classifySupportingRoot(parts []string, filePath string) (RBOKSourceClassification, bool) {
	if len(parts) == 0 {
		return RBOKSourceClassification{}, false
	}
	top := strings.ToLower(parts[0])
	switch top {
	case "02_organisation":
		return supportingClassification(filePath, RoleSupporting, "supporting_context", "organisation", "supporting", "organisation supporting context"), true
	case "03_catalogue_services":
		return supportingClassification(filePath, RoleSupporting, "supporting_context", "service_catalog", "supporting", "service catalogue supporting context"), true
	case "04_marketing":
		return supportingClassification(filePath, RoleEvidence, "experience_evidence", "market_context", "evidence", "marketing and experience evidence"), true
	case "05_pilotage":
		return supportingClassification(filePath, RoleOperational, "operational_context", "pilotage", "operational", "pilotage operational context"), true
	default:
		return RBOKSourceClassification{}, false
	}
}

func supportingClassification(filePath string, role SourceRole, sourceClass, layer, authority, reason string) RBOKSourceClassification {
	return RBOKSourceClassification{
		Path:        filePath,
		Priority:    "secondary",
		Status:      "active",
		Role:        role,
		SourceClass: sourceClass,
		CorpusLayer: layer,
		Authority:   authority,
		AllowedUses: []string{"citation_internal", "vector_index"},
		Reason:      reason,
	}
}

func corpusLayerForParts(parts []string) string {
	if len(parts) == 0 {
		return "unspecified"
	}
	top := strings.ToLower(parts[0])
	switch {
	case strings.HasPrefix(top, "00_meta"):
		return "metadata"
	case strings.HasPrefix(top, "01_referentiel"):
		return "reference_model"
	case strings.HasPrefix(top, "02_domaines"):
		return "domain_doctrine"
	case strings.HasPrefix(top, "03_parcours"):
		return "runtime_binding"
	case strings.HasPrefix(top, "98_sch"):
		return "schema"
	case strings.HasPrefix(top, "99_rbok") || strings.HasPrefix(top, "99_initial"):
		return "reference_original"
	default:
		return "supporting_context"
	}
}
