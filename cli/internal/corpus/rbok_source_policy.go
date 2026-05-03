package corpus

import (
	"path"
	"strings"
)

// SourceRole describes the function of a source file in the RBOK lawbook.
type SourceRole string

const (
	RoleLawbook    SourceRole = "lawbook"
	RoleSchema     SourceRole = "schema"
	RoleReference  SourceRole = "reference"
	RoleDerived    SourceRole = "derived"
	RoleOutOfScope SourceRole = "out_of_scope"
)

// RBOKSourceClassification is the policy result for a single RBOK source file.
type RBOKSourceClassification struct {
	Path        string     `json:"path"`
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	Role        SourceRole `json:"role"`
	AllowedUses []string   `json:"allowed_uses"`
	Reason      string     `json:"reason"`
}

// ClassifyRBOKSource applies the RBOK lawbook source policy to a file path.
// The path should be relative to the corpus root (forward-slash separated).
func ClassifyRBOKSource(filePath string) RBOKSourceClassification {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.Split(normalized, "/")
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
			AllowedUses: nil,
			Reason:      "OS artifact, script, or tooling file",
		}
	}

	// Derived: generated files, German translations, testdata.
	if isDerived(lower, ext, parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "derived",
			Status:      "active",
			Role:        RoleDerived,
			AllowedUses: []string{"citation_internal"},
			Reason:      "generated, translated, or test fixture",
		}
	}

	// Reference: archived and initial source documents.
	if isReference(lower, parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "reference",
			Status:      "active",
			Role:        RoleReference,
			AllowedUses: []string{"human_review_only", "citation_internal"},
			Reason:      "archived, original, or initial source document",
		}
	}

	// Schema: CUE/JSON schemas in 98_schemas.
	if isSchema(parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "reference",
			Status:      "active",
			Role:        RoleSchema,
			AllowedUses: []string{"structured_contract", "citation_internal"},
			Reason:      "CUE/JSON schema from 98_schemas",
		}
	}

	// Primary: canonical lawbook content (00_meta, 01_referentiel, 02_domaines, 03_parcours).
	if isPrimary(parts) {
		return RBOKSourceClassification{
			Path:        filePath,
			Priority:    "primary",
			Status:      "active",
			Role:        RoleLawbook,
			AllowedUses: []string{"structured_contract", "vector_index", "citation_internal", "golden_case"},
			Reason:      "canonical lawbook source",
		}
	}

	// Default: secondary.
	return RBOKSourceClassification{
		Path:        filePath,
		Priority:    "secondary",
		Status:      "active",
		Role:        RoleLawbook,
		AllowedUses: []string{"structured_contract", "citation_internal"},
		Reason:      "lawbook source outside primary directories",
	}
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

func isSchema(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	top := strings.ToLower(parts[0])
	return strings.HasPrefix(top, "98_schemas") || strings.HasPrefix(top, "98_schema") ||
		strings.HasPrefix(top, "98_schémas") || strings.HasPrefix(top, "98_schéma")
}

func isReference(lower string, parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	for _, p := range parts {
		switch strings.ToLower(p) {
		case "archive", "archives", "archived":
			return true
		}
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
		if lp == "scripts" || lp == "script" || lp == ".git" ||
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
