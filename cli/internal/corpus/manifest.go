package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var extToSourceType = map[string]string{
	".md":   "markdown",
	".mdx":  "markdown",
	".txt":  "markdown",
	".pdf":  "pdf",
	".html": "html",
	".htm":  "html",
	".php":  "php",
	".go":   "source_code",
	".py":   "source_code",
	".js":   "source_code",
	".ts":   "source_code",
	".java": "source_code",
	".kt":   "source_code",
	".cs":   "source_code",
	".rb":   "source_code",
	".rs":   "source_code",
	".c":    "source_code",
	".cpp":  "source_code",
	".h":    "source_code",
	".yaml": "source_code",
	".yml":  "source_code",
	".json": "source_code",
	".toml": "source_code",
	".csv":  "csv",
	".tsv":  "csv",
	".xls":  "spreadsheet",
	".xlsx": "spreadsheet",
	".ods":  "spreadsheet",
	".png":  "image",
	".jpg":  "image",
	".jpeg": "image",
	".gif":  "image",
	".svg":  "image",
	".mp3":  "audio",
	".wav":  "audio",
	".ogg":  "audio",
	".sql":  "database_export",
	".dump": "database_export",
}

// ManifestSource is a single entry in the sidecar source manifest.
//
// FSQ-02 (#365): the AdmissionStatus / AtomizationStatus / ExclusionReason
// / SourceRole / FormatSupport / DerivativeOf fields make the admission
// and atomization policy of every source explicit and machine-checkable.
// All six are omitempty so legacy manifest YAML (which omits them) keeps
// loading; the feed-generation pipeline backfills defaults from the
// extension heuristic before validation.
type ManifestSource struct {
	ID              string   `yaml:"id"`
	Path            string   `yaml:"path"`
	Type            string   `yaml:"type"`
	Domain          string   `yaml:"domain"`
	Priority        string   `yaml:"priority"`
	Status          string   `yaml:"status"`
	Hash            string   `yaml:"hash"`
	Owner           string   `yaml:"owner"`
	License         string   `yaml:"license"`
	Confidentiality string   `yaml:"confidentiality"`
	AllowedUses     []string `yaml:"allowed_uses"`

	// FSQ-02 (#365) admission + atomization policy.
	AdmissionStatus   string `yaml:"admission_status,omitempty"`
	AtomizationStatus string `yaml:"atomization_status,omitempty"`
	ExclusionReason   string `yaml:"exclusion_reason,omitempty"`
	SourceRole        string `yaml:"source_role,omitempty"`
	FormatSupport     string `yaml:"format_support,omitempty"`
	DerivativeOf      string `yaml:"derivative_of,omitempty"`
}

// Admission returns a SourceAdmission projection of the manifest entry.
func (m ManifestSource) Admission() SourceAdmission {
	return SourceAdmission{
		AdmissionStatus:   m.AdmissionStatus,
		AtomizationStatus: m.AtomizationStatus,
		ExclusionReason:   m.ExclusionReason,
		SourceRole:        m.SourceRole,
		FormatSupport:     m.FormatSupport,
		DerivativeOf:      m.DerivativeOf,
	}
}

// Validate enforces the FSQ-02 admission rules on the manifest entry.
func (m ManifestSource) Validate() error {
	return m.Admission().Validate()
}

// SidecarManifest is the YAML structure matching source-manifest.cue.
type SidecarManifest struct {
	SchemaVersion string           `yaml:"schema_version"`
	Sources       []ManifestSource `yaml:"sources"`
}

// ManifestOptions configures sidecar manifest generation.
type ManifestOptions struct {
	Domain          string
	Owner           string
	License         string
	Confidentiality string
	Priority        string
	AllowedUses     []string
	IDPrefix        string
}

// GenerateManifest converts a Snapshot into a SidecarManifest.
// Defaults are applied from ManifestOptions for fields that cannot
// be inferred from the scan alone.
func GenerateManifest(snap Snapshot, opts ManifestOptions) SidecarManifest {
	priority := opts.Priority
	if priority == "" {
		priority = "primary"
	}
	confidentiality := opts.Confidentiality
	if confidentiality == "" {
		confidentiality = "internal"
	}
	license := opts.License
	if license == "" {
		license = "internal"
	}
	owner := opts.Owner
	if owner == "" {
		owner = "unknown"
	}
	allowedUses := opts.AllowedUses
	if len(allowedUses) == 0 {
		allowedUses = []string{"structured_contract", "citation_internal"}
	}
	prefix := opts.IDPrefix
	if prefix == "" {
		prefix = "CORPUS"
	}

	sources := make([]ManifestSource, 0, len(snap.Sources))
	seenIDs := map[string]bool{}
	for i, entry := range snap.Sources {
		id := generateID(prefix, i+1, entry.Path)
		if seenIDs[id] {
			id = id + "-" + pathHashSuffix(entry.Path)
		}
		seenIDs[id] = true
		def := DefaultAdmissionForPath(entry.Path)
		sources = append(sources, ManifestSource{
			ID:                id,
			Path:              entry.Path,
			Type:              inferSourceType(entry.Extension),
			Domain:            opts.Domain,
			Priority:          priority,
			Status:            "active",
			Hash:              entry.Hash,
			Owner:             owner,
			License:           license,
			Confidentiality:   confidentiality,
			AllowedUses:       allowedUses,
			AdmissionStatus:   def.AdmissionStatus,
			AtomizationStatus: def.AtomizationStatus,
			ExclusionReason:   def.ExclusionReason,
			SourceRole:        def.SourceRole,
			FormatSupport:     def.FormatSupport,
			DerivativeOf:      def.DerivativeOf,
		})
	}

	return SidecarManifest{
		SchemaVersion: "0.1.0",
		Sources:       sources,
	}
}

// WriteManifestYAML writes the sidecar manifest as YAML.
func WriteManifestYAML(w io.Writer, manifest SidecarManifest) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(manifest)
}

func inferSourceType(ext string) string {
	ext = strings.ToLower(ext)
	if t, ok := extToSourceType[ext]; ok {
		return t
	}
	return "source_code"
}

func generateID(prefix string, index int, path string) string {
	// Use filename stem as slug, uppercased
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	slug := strings.ToUpper(stem)
	slug = sanitizeIDChars(slug)
	if slug == "" {
		slug = fmt.Sprintf("%04d", index)
	}
	return fmt.Sprintf("%s-%s", prefix, slug)
}

func pathHashSuffix(path string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(path)))
	return strings.ToUpper(hex.EncodeToString(sum[:3]))
}

func sanitizeIDChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == ' ', r == '.':
			b.WriteByte('-')
		}
	}
	result := b.String()
	// Collapse multiple dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}
