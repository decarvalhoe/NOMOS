package corpus

import (
	"path"
	"strings"
)

// RuntimeLayer identifies a layer in the realisons-business corpus.
type RuntimeLayer string

const (
	LayerDoctrine   RuntimeLayer = "doctrine"     // 01_rbok: authoritative source of truth
	LayerRuntime    RuntimeLayer = "runtime"       // 02_parcours: runtime business paths
	LayerWorkbooks  RuntimeLayer = "workbooks"     // 03_workbooks: generated/derived artifacts
	LayerMeta       RuntimeLayer = "meta"          // 00_meta: index and governance metadata
	LayerSchemas    RuntimeLayer = "schemas"        // 98_schemas: structural schemas
	LayerReference  RuntimeLayer = "reference"     // 99_*: original reference PDFs
	LayerUnknown    RuntimeLayer = "unknown"        // unclassified
)

// RuntimeLayerClassification is the policy result for a realisons-business file.
type RuntimeLayerClassification struct {
	Path        string       `json:"path"`
	Layer       RuntimeLayer `json:"layer"`
	Priority    string       `json:"priority"`
	Status      string       `json:"status"`
	Role        SourceRole   `json:"role"`
	AllowedUses []string     `json:"allowed_uses"`
	Mutable     bool         `json:"mutable"`
	Reason      string       `json:"reason"`
}

// layerDef holds the classification template for a layer.
type layerDef struct {
	Layer       RuntimeLayer
	Priority    string
	Status      string
	Role        SourceRole
	AllowedUses []string
	Mutable     bool
	Reason      string
}

var layerDefs = map[RuntimeLayer]layerDef{
	LayerMeta: {
		Layer: LayerMeta, Priority: "primary", Status: "active",
		Role: RoleLawbook, Mutable: false,
		AllowedUses: []string{"structured_contract", "vector_index", "citation_internal"},
		Reason:      "governance metadata and corpus index",
	},
	LayerDoctrine: {
		Layer: LayerDoctrine, Priority: "primary", Status: "active",
		Role: RoleLawbook, Mutable: false,
		AllowedUses: []string{"structured_contract", "vector_index", "citation_internal", "citation_external", "golden_case"},
		Reason:      "authoritative doctrine — single source of truth",
	},
	LayerRuntime: {
		Layer: LayerRuntime, Priority: "primary", Status: "active",
		Role: RoleLawbook, Mutable: false,
		AllowedUses: []string{"structured_contract", "vector_index", "citation_internal", "golden_case"},
		Reason:      "runtime business paths and parcours",
	},
	LayerWorkbooks: {
		Layer: LayerWorkbooks, Priority: "derived", Status: "active",
		Role: RoleDerived, Mutable: true,
		AllowedUses: []string{"citation_internal"},
		Reason:      "generated workbooks and derived artifacts",
	},
	LayerSchemas: {
		Layer: LayerSchemas, Priority: "reference", Status: "active",
		Role: RoleSchema, Mutable: false,
		AllowedUses: []string{"structured_contract", "citation_internal"},
		Reason:      "structural schemas for validation",
	},
	LayerReference: {
		Layer: LayerReference, Priority: "reference", Status: "active",
		Role: RoleReference, Mutable: false,
		AllowedUses: []string{"human_review_only", "citation_internal"},
		Reason:      "original reference documents",
	},
}

// ClassifyRuntimeLayer applies the realisons-business layer policy.
// The path is relative to the corpus root, forward-slash separated.
func ClassifyRuntimeLayer(filePath string) RuntimeLayerClassification {
	normalized := path.Clean(strings.ReplaceAll(filePath, "\\", "/"))
	parts := strings.Split(normalized, "/")
	base := path.Base(normalized)
	lower := strings.ToLower(normalized)

	// Out-of-scope first (scripts, OS artifacts, tooling).
	if isRuntimeOutOfScope(base, lower, parts) {
		return RuntimeLayerClassification{
			Path: filePath, Layer: LayerUnknown,
			Priority: "out_of_scope", Status: "out_of_scope",
			Role: RoleOutOfScope, Mutable: false,
			Reason: "OS artifact, script, or tooling file",
		}
	}

	// Derived markers override layer (generated files anywhere).
	if isRuntimeDerived(lower, parts) {
		return RuntimeLayerClassification{
			Path: filePath, Layer: LayerWorkbooks,
			Priority: "derived", Status: "active",
			Role: RoleDerived, Mutable: true,
			AllowedUses: []string{"citation_internal"},
			Reason:      "generated or derived content",
		}
	}

	// Match layer by top-level directory.
	layer := matchLayer(parts)
	if def, ok := layerDefs[layer]; ok {
		return RuntimeLayerClassification{
			Path:        filePath,
			Layer:       def.Layer,
			Priority:    def.Priority,
			Status:      def.Status,
			Role:        def.Role,
			AllowedUses: def.AllowedUses,
			Mutable:     def.Mutable,
			Reason:      def.Reason,
		}
	}

	// Default: secondary, unknown layer.
	return RuntimeLayerClassification{
		Path: filePath, Layer: LayerUnknown,
		Priority: "secondary", Status: "active",
		Role: RoleLawbook, Mutable: false,
		AllowedUses: []string{"citation_internal"},
		Reason:      "file outside recognized layer directories",
	}
}

func matchLayer(parts []string) RuntimeLayer {
	if len(parts) == 0 {
		return LayerUnknown
	}
	top := strings.ToLower(parts[0])

	switch {
	case strings.HasPrefix(top, "00_meta"):
		return LayerMeta
	case strings.HasPrefix(top, "01_rbok"):
		return LayerDoctrine
	case strings.HasPrefix(top, "01_referentiel"):
		return LayerDoctrine
	case strings.HasPrefix(top, "02_parcours"):
		return LayerRuntime
	case strings.HasPrefix(top, "02_domaines"):
		return LayerRuntime
	case strings.HasPrefix(top, "03_workbook"):
		return LayerWorkbooks
	case strings.HasPrefix(top, "03_generated"):
		return LayerWorkbooks
	case strings.HasPrefix(top, "98_schema"):
		return LayerSchemas
	case strings.HasPrefix(top, "99_rbok"), strings.HasPrefix(top, "99_initial"):
		return LayerReference
	}
	return LayerUnknown
}

func isRuntimeOutOfScope(base, lower string, parts []string) bool {
	lowerBase := strings.ToLower(base)
	switch lowerBase {
	case ".ds_store", "thumbs.db", "desktop.ini", ".gitkeep":
		return true
	}
	for _, p := range parts {
		lp := strings.ToLower(p)
		if lp == "scripts" || lp == "script" || lp == ".git" ||
			lp == "node_modules" || lp == "__pycache__" || lp == ".venv" {
			return true
		}
	}
	switch strings.ToLower(path.Ext(base)) {
	case ".sh", ".bash", ".zsh", ".ps1", ".bat", ".cmd":
		return true
	}
	if lowerBase == "package-lock.json" || lowerBase == "yarn.lock" ||
		lowerBase == "pnpm-lock.yaml" || lowerBase == "poetry.lock" {
		return true
	}
	return false
}

func isRuntimeDerived(lower string, parts []string) bool {
	if strings.Contains(lower, "generated") || strings.Contains(lower, "_gen.") ||
		strings.Contains(lower, ".gen.") {
		return true
	}
	if strings.HasSuffix(lower, ".de.yaml") || strings.HasSuffix(lower, ".de.yml") ||
		strings.HasSuffix(lower, ".de.md") {
		return true
	}
	for _, p := range parts {
		if strings.ToLower(p) == "testdata" || strings.ToLower(p) == "fixtures" {
			return true
		}
	}
	return false
}
