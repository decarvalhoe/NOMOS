package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Diagnostic codes for sidecar validation.
const (
	CodeHashMismatch          = "HASH_MISMATCH"
	CodeHashMissing           = "HASH_MISSING"
	CodeHashMalformed         = "HASH_MALFORMED"
	CodeOwnerMissing          = "OWNER_MISSING"
	CodeConfidentialityEmpty  = "CONFIDENTIALITY_EMPTY"
	CodeConfidentialityBad    = "CONFIDENTIALITY_INVALID"
	CodeFileMissing           = "FILE_MISSING"
	CodeFileUndeclared        = "FILE_UNDECLARED"
	CodeIDMissing             = "ID_MISSING"
	CodeIDInvalid             = "ID_INVALID"
	CodeIDDuplicate           = "ID_DUPLICATE"
	CodeNoSources             = "NO_SOURCES"
)

var (
	sidecarIDPattern    = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
	hashDigestPattern   = regexp.MustCompile(`^(sha256|sha384|sha512):[a-fA-F0-9]+$`)
	validConfidentiality = []string{"public", "internal", "restricted", "secret"}
)

// SidecarManifest is the YAML structure of a source-manifest sidecar file.
type SidecarManifest struct {
	SchemaVersion string          `yaml:"schema_version"`
	Sources       []SidecarSource `yaml:"sources"`
}

// SidecarSource is one entry in the sidecar manifest.
type SidecarSource struct {
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
}

// SidecarError is a single structured validation error.
type SidecarError struct {
	SourceID string `json:"source_id"`
	Code     string `json:"code"`
	Field    string `json:"field"`
	Message  string `json:"message"`
}

func (e SidecarError) Error() string {
	return fmt.Sprintf("[%s] %s: %s (%s)", e.Code, e.SourceID, e.Message, e.Field)
}

// SidecarResult holds the outcome of sidecar validation.
type SidecarResult struct {
	Valid       bool           `json:"valid"`
	SourceCount int            `json:"source_count"`
	Errors      []SidecarError `json:"errors"`
}

// ParseSidecarManifest reads and parses a sidecar manifest YAML file.
func ParseSidecarManifest(path string) (SidecarManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SidecarManifest{}, fmt.Errorf("read sidecar manifest: %w", err)
	}
	return ParseSidecarManifestBytes(data)
}

// ParseSidecarManifestBytes parses sidecar manifest YAML from bytes.
func ParseSidecarManifestBytes(data []byte) (SidecarManifest, error) {
	var m SidecarManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return SidecarManifest{}, fmt.Errorf("parse sidecar yaml: %w", err)
	}
	return m, nil
}

// ValidateSidecar validates a sidecar manifest against the corpus on disk.
// corpusRoot is the directory containing the actual source files referenced
// by the manifest paths. Pass empty string to skip file-system checks.
func ValidateSidecar(manifest SidecarManifest, corpusRoot string) SidecarResult {
	result := SidecarResult{
		Valid:       true,
		SourceCount: len(manifest.Sources),
	}

	if len(manifest.Sources) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, SidecarError{
			Code:    CodeNoSources,
			Field:   "sources",
			Message: "manifest declares no sources",
		})
		return result
	}

	seenIDs := map[string]bool{}
	declaredPaths := map[string]bool{}

	for _, src := range manifest.Sources {
		validateSourceID(&result, src, seenIDs)
		validateOwner(&result, src)
		validateConfidentiality(&result, src)
		validateHash(&result, src, corpusRoot)

		if src.Path != "" {
			declaredPaths[src.Path] = true
		}

		if corpusRoot != "" && src.Path != "" {
			validateFileExists(&result, src, corpusRoot)
		}
	}

	// Check for undeclared files in corpus root.
	if corpusRoot != "" {
		checkUndeclaredFiles(&result, declaredPaths, corpusRoot)
	}

	result.Valid = len(result.Errors) == 0
	return result
}

func validateSourceID(result *SidecarResult, src SidecarSource, seen map[string]bool) {
	if strings.TrimSpace(src.ID) == "" {
		addSidecarErr(result, src.ID, CodeIDMissing, "id", "source id is required")
		return
	}
	if !sidecarIDPattern.MatchString(src.ID) {
		addSidecarErr(result, src.ID, CodeIDInvalid, "id",
			fmt.Sprintf("id %q does not match ^[A-Z0-9][A-Z0-9-]*$", src.ID))
	}
	if seen[src.ID] {
		addSidecarErr(result, src.ID, CodeIDDuplicate, "id",
			fmt.Sprintf("duplicate source id %q", src.ID))
	}
	seen[src.ID] = true
}

func validateOwner(result *SidecarResult, src SidecarSource) {
	if strings.TrimSpace(src.Owner) == "" {
		addSidecarErr(result, src.ID, CodeOwnerMissing, "owner",
			"owner is required for every source")
	}
}

func validateConfidentiality(result *SidecarResult, src SidecarSource) {
	if strings.TrimSpace(src.Confidentiality) == "" {
		addSidecarErr(result, src.ID, CodeConfidentialityEmpty, "confidentiality",
			"confidentiality classification is required")
		return
	}
	if !slices.Contains(validConfidentiality, src.Confidentiality) {
		addSidecarErr(result, src.ID, CodeConfidentialityBad, "confidentiality",
			fmt.Sprintf("confidentiality %q must be one of: %s",
				src.Confidentiality, strings.Join(validConfidentiality, ", ")))
	}
}

func validateHash(result *SidecarResult, src SidecarSource, corpusRoot string) {
	if strings.TrimSpace(src.Hash) == "" {
		addSidecarErr(result, src.ID, CodeHashMissing, "hash", "hash is required")
		return
	}
	if !hashDigestPattern.MatchString(src.Hash) {
		addSidecarErr(result, src.ID, CodeHashMalformed, "hash",
			fmt.Sprintf("hash %q must match algo:hex (sha256|sha384|sha512)", src.Hash))
		return
	}

	// Verify sha256 hash against actual file if corpus root is provided.
	if corpusRoot == "" || src.Path == "" {
		return
	}
	parts := strings.SplitN(src.Hash, ":", 2)
	if parts[0] != "sha256" {
		return // Only verify sha256 for now.
	}

	filePath := filepath.Join(corpusRoot, src.Path)
	actual, err := hashFileSHA256(filePath)
	if err != nil {
		return // FILE_MISSING is reported by validateFileExists.
	}
	if actual != parts[1] {
		addSidecarErr(result, src.ID, CodeHashMismatch, "hash",
			fmt.Sprintf("declared sha256:%s but file hashes to sha256:%s", parts[1], actual))
	}
}

func validateFileExists(result *SidecarResult, src SidecarSource, corpusRoot string) {
	filePath := filepath.Join(corpusRoot, src.Path)
	if _, err := os.Stat(filePath); err != nil {
		addSidecarErr(result, src.ID, CodeFileMissing, "path",
			fmt.Sprintf("file not found: %s", src.Path))
	}
}

func checkUndeclaredFiles(result *SidecarResult, declared map[string]bool, corpusRoot string) {
	_ = filepath.WalkDir(corpusRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(corpusRoot, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !declared[rel] {
			addSidecarErr(result, "", CodeFileUndeclared, "path",
				fmt.Sprintf("file %q exists in corpus but is not declared in manifest", rel))
		}
		return nil
	})
}

func hashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func addSidecarErr(result *SidecarResult, sourceID, code, field, message string) {
	result.Errors = append(result.Errors, SidecarError{
		SourceID: sourceID,
		Code:     code,
		Field:    field,
		Message:  message,
	})
}
