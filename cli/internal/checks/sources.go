package checks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	upperIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
	digestPattern  = regexp.MustCompile(`^(sha256|sha384|sha512):.+$`)
)

var (
	sourceTypes      = []string{"markdown", "pdf", "html", "php", "source_code", "csv", "database_export", "spreadsheet", "image", "audio", "decision", "api_export"}
	sourcePriorities = []string{"primary", "secondary", "legacy", "derived", "reference"}
	sourceStatuses   = []string{"active", "superseded", "duplicate", "out_of_scope", "needs_review", "blocked"}
	confidentiality  = []string{"public", "internal", "restricted", "secret"}
	allowedUses      = []string{"structured_contract", "vector_index", "citation_internal", "citation_external", "golden_case", "human_review_only"}
)

type sourceManifest struct {
	SchemaVersion string   `yaml:"schema_version"`
	Sources       []source `yaml:"sources"`
}

type source struct {
	ID              string   `yaml:"id"`
	Path            string   `yaml:"path"`
	Type            string   `yaml:"type"`
	Domain          string   `yaml:"domain"`
	Priority        string   `yaml:"priority"`
	Status          string   `yaml:"status"`
	Hash            string   `yaml:"hash"`
	Version         string   `yaml:"version"`
	Owner           string   `yaml:"owner"`
	License         string   `yaml:"license"`
	Confidentiality string   `yaml:"confidentiality"`
	AllowedUses     []string `yaml:"allowed_uses"`
	RedactionPolicy string   `yaml:"redaction_policy"`
	Notes           string   `yaml:"notes"`
}

type CheckResult struct {
	Valid   bool          `json:"valid"`
	Sources []SourceCheck `json:"sources"`
}

type SourceCheck struct {
	ID     string       `json:"id"`
	Path   string       `json:"path"`
	Valid  bool         `json:"valid"`
	Errors []CheckError `json:"errors,omitempty"`
}

type CheckError struct {
	SourceID string `json:"source_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

func CheckSources(manifestPath string, baseDir string) (CheckResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return CheckResult{}, fmt.Errorf("reading manifest: %w", err)
	}
	return CheckSourcesFromBytes(data, baseDir)
}

func CheckSourcesFromBytes(data []byte, baseDir string) (CheckResult, error) {
	var manifest sourceManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return CheckResult{}, fmt.Errorf("parsing manifest: %w", err)
	}

	result := CheckResult{Valid: true}
	for _, src := range manifest.Sources {
		sc := checkSource(src, baseDir)
		if !sc.Valid {
			result.Valid = false
		}
		result.Sources = append(result.Sources, sc)
	}
	return result, nil
}

func checkSource(src source, baseDir string) SourceCheck {
	sc := SourceCheck{
		ID:    src.ID,
		Path:  src.Path,
		Valid: true,
	}

	checkID(&sc, src)
	checkPath(&sc, src, baseDir)
	checkHash(&sc, src)
	checkOwner(&sc, src)
	checkStatus(&sc, src)
	checkType(&sc, src)
	checkPriority(&sc, src)
	checkConfidentiality(&sc, src)
	checkAllowedUses(&sc, src)

	sc.Valid = len(sc.Errors) == 0
	return sc
}

func checkID(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.ID) == "" {
		addCheckError(sc, src.ID, "MISSING_ID", "source id is required")
		return
	}
	if !upperIDPattern.MatchString(src.ID) {
		addCheckError(sc, src.ID, "INVALID_ID",
			fmt.Sprintf("id %q must match %s", src.ID, upperIDPattern.String()))
	}
}

func checkPath(sc *SourceCheck, src source, baseDir string) {
	if strings.TrimSpace(src.Path) == "" {
		addCheckError(sc, src.ID, "MISSING_SOURCE", "source path is required")
		return
	}
	if baseDir == "" {
		return
	}
	resolved := filepath.Join(baseDir, src.Path)
	if _, err := os.Stat(resolved); err != nil {
		addCheckError(sc, src.ID, "MISSING_SOURCE",
			fmt.Sprintf("file not found: %s", resolved))
	}
}

func checkHash(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Hash) == "" {
		addCheckError(sc, src.ID, "INVALID_HASH", "hash is required")
		return
	}
	if !digestPattern.MatchString(src.Hash) {
		addCheckError(sc, src.ID, "INVALID_HASH",
			fmt.Sprintf("hash %q must match algo:hex (sha256|sha384|sha512)", src.Hash))
	}
}

func checkOwner(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Owner) == "" {
		addCheckError(sc, src.ID, "NO_OWNER", "owner is required")
	}
}

func checkStatus(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Status) == "" {
		addCheckError(sc, src.ID, "INVALID_STATUS", "status is required")
		return
	}
	if !slices.Contains(sourceStatuses, src.Status) {
		addCheckError(sc, src.ID, "INVALID_STATUS",
			fmt.Sprintf("status %q must be one of: %s", src.Status, strings.Join(sourceStatuses, ", ")))
	}
}

func checkType(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Type) == "" {
		addCheckError(sc, src.ID, "INVALID_TYPE", "type is required")
		return
	}
	if !slices.Contains(sourceTypes, src.Type) {
		addCheckError(sc, src.ID, "INVALID_TYPE",
			fmt.Sprintf("type %q must be one of: %s", src.Type, strings.Join(sourceTypes, ", ")))
	}
}

func checkPriority(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Priority) == "" {
		addCheckError(sc, src.ID, "INVALID_PRIORITY", "priority is required")
		return
	}
	if !slices.Contains(sourcePriorities, src.Priority) {
		addCheckError(sc, src.ID, "INVALID_PRIORITY",
			fmt.Sprintf("priority %q must be one of: %s", src.Priority, strings.Join(sourcePriorities, ", ")))
	}
}

func checkConfidentiality(sc *SourceCheck, src source) {
	if strings.TrimSpace(src.Confidentiality) == "" {
		addCheckError(sc, src.ID, "INVALID_CONFIDENTIALITY", "confidentiality is required")
		return
	}
	if !slices.Contains(confidentiality, src.Confidentiality) {
		addCheckError(sc, src.ID, "INVALID_CONFIDENTIALITY",
			fmt.Sprintf("confidentiality %q must be one of: %s", src.Confidentiality, strings.Join(confidentiality, ", ")))
	}
}

func checkAllowedUses(sc *SourceCheck, src source) {
	if len(src.AllowedUses) == 0 {
		addCheckError(sc, src.ID, "NO_ALLOWED_USES", "at least one allowed_use is required")
		return
	}
	for _, use := range src.AllowedUses {
		if !slices.Contains(allowedUses, use) {
			addCheckError(sc, src.ID, "INVALID_ALLOWED_USE",
				fmt.Sprintf("allowed_use %q must be one of: %s", use, strings.Join(allowedUses, ", ")))
		}
	}
}

func addCheckError(sc *SourceCheck, sourceID string, code string, message string) {
	sc.Errors = append(sc.Errors, CheckError{
		SourceID: sourceID,
		Code:     code,
		Message:  message,
	})
}
