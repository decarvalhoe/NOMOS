package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Known profile names.
const ProfileRBOKLawbook = "rbok-lawbook"

// OutputFlag controls which feed sections are emitted.
type OutputFlag string

const (
	OutputIndex      OutputFlag = "index"
	OutputGovernance OutputFlag = "governance"
	OutputCitation   OutputFlag = "citation"
	OutputImport     OutputFlag = "import"
)

var allOutputFlags = []OutputFlag{OutputIndex, OutputGovernance, OutputCitation, OutputImport}

// Profile describes a corpus processing profile.
type Profile struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Outputs     []OutputFlag `json:"outputs"`
}

var profileRegistry = map[string]Profile{
	ProfileRBOKLawbook: {
		Name:        ProfileRBOKLawbook,
		Description: "RBOK Lawbook corpus profile: classifies sources, generates governance-aware feed with index, governance, citation, and import outputs.",
		Outputs:     allOutputFlags,
	},
}

// LookupProfile returns the profile for a given name, or an error if unknown.
func LookupProfile(name string) (Profile, error) {
	p, ok := profileRegistry[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		known := KnownProfiles()
		return Profile{}, fmt.Errorf("unknown profile %q; known profiles: %s", name, strings.Join(known, ", "))
	}
	return p, nil
}

// KnownProfiles returns sorted profile names.
func KnownProfiles() []string {
	names := make([]string, 0, len(profileRegistry))
	for k := range profileRegistry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ProfileFeedInput configures a profiled feed run.
type ProfileFeedInput struct {
	Profile     string     `json:"profile"`
	CorpusRoot  string     `json:"corpus_root"`
	MatrixPath  string     `json:"matrix_path"`
	ManifestPath string    `json:"manifest_path"`
	Outputs     []OutputFlag `json:"outputs"`
}

// ProfileFeedResult holds the profiled feed output sections.
type ProfileFeedResult struct {
	Profile     string                    `json:"profile"`
	Sections    map[OutputFlag]json.RawMessage `json:"sections"`
	SourceCount int                        `json:"source_count"`
	UnitCount   int                        `json:"unit_count"`
	Errors      []string                   `json:"errors,omitempty"`
}

// RunProfileFeed executes a profiled corpus feed generation.
func RunProfileFeed(input ProfileFeedInput) (ProfileFeedResult, error) {
	profile, err := LookupProfile(input.Profile)
	if err != nil {
		return ProfileFeedResult{}, err
	}

	outputs := input.Outputs
	if len(outputs) == 0 {
		outputs = profile.Outputs
	}

	result := ProfileFeedResult{
		Profile:  profile.Name,
		Sections: make(map[OutputFlag]json.RawMessage),
	}

	// Classify all sources using the profile policy.
	classifications, classifyErrors := classifyCorpusSources(input.CorpusRoot)
	result.SourceCount = len(classifications)
	result.Errors = append(result.Errors, classifyErrors...)

	// Build sections.
	for _, out := range outputs {
		section, err := buildSection(out, classifications)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("section %s: %v", out, err))
			continue
		}
		result.Sections[out] = section
	}

	return result, nil
}

func classifyCorpusSources(corpusRoot string) ([]RBOKSourceClassification, []string) {
	if corpusRoot == "" {
		return nil, []string{"corpus_root is empty"}
	}

	var classifications []RBOKSourceClassification
	var errors []string

	err := filepath.WalkDir(corpusRoot, func(filePath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(corpusRoot, filePath)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		c := ClassifyRBOKSource(rel)
		classifications = append(classifications, c)

		// Detect binary files by checking for null bytes in the first 512 bytes.
		if isBinaryFile(filePath) {
			errors = append(errors, fmt.Sprintf("blocked binary: %s", rel))
		}

		return nil
	})
	if err != nil {
		return nil, []string{fmt.Sprintf("scan corpus: %v", err)}
	}

	return classifications, errors
}

func isBinaryFile(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

// IndexEntry is a single entry in the index section.
type IndexEntry struct {
	Path     string     `json:"path"`
	Priority string     `json:"priority"`
	Role     SourceRole `json:"role"`
}

// GovernanceEntry summarises governance posture per source.
type GovernanceEntry struct {
	Path        string   `json:"path"`
	Priority    string   `json:"priority"`
	Status      string   `json:"status"`
	AllowedUses []string `json:"allowed_uses"`
	Blocked     bool     `json:"blocked"`
}

func buildSection(flag OutputFlag, classifications []RBOKSourceClassification) (json.RawMessage, error) {
	switch flag {
	case OutputIndex:
		entries := make([]IndexEntry, 0, len(classifications))
		for _, c := range classifications {
			if c.Role == RoleOutOfScope {
				continue
			}
			entries = append(entries, IndexEntry{
				Path:     c.Path,
				Priority: c.Priority,
				Role:     c.Role,
			})
		}
		return json.Marshal(entries)

	case OutputGovernance:
		entries := make([]GovernanceEntry, 0, len(classifications))
		for _, c := range classifications {
			entries = append(entries, GovernanceEntry{
				Path:        c.Path,
				Priority:    c.Priority,
				Status:      c.Status,
				AllowedUses: c.AllowedUses,
				Blocked:     c.Role == RoleOutOfScope,
			})
		}
		return json.Marshal(entries)

	case OutputCitation:
		var citeable []IndexEntry
		for _, c := range classifications {
			for _, use := range c.AllowedUses {
				if use == "citation_internal" || use == "citation_external" {
					citeable = append(citeable, IndexEntry{
						Path:     c.Path,
						Priority: c.Priority,
						Role:     c.Role,
					})
					break
				}
			}
		}
		return json.Marshal(citeable)

	case OutputImport:
		var importable []IndexEntry
		for _, c := range classifications {
			for _, use := range c.AllowedUses {
				if use == "structured_contract" || use == "vector_index" {
					importable = append(importable, IndexEntry{
						Path:     c.Path,
						Priority: c.Priority,
						Role:     c.Role,
					})
					break
				}
			}
		}
		return json.Marshal(importable)

	default:
		return nil, fmt.Errorf("unknown output flag %q", flag)
	}
}

// WriteProfileFeedJSON serialises the result to a writer.
func WriteProfileFeedJSON(w io.Writer, result ProfileFeedResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
