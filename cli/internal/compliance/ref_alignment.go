package compliance

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrGateFailed       = errors.New("external reference alignment gate failed")
	ErrRegisterNotFound = errors.New("reference register file not found")
)

// Known reference prefixes to detect in documentation.
var refPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bGAMP\s*5\b`),
	regexp.MustCompile(`(?i)\bISPE[- ]GAMP`),
	regexp.MustCompile(`(?i)\bICH[- ]Q[0-9]+`),
	regexp.MustCompile(`(?i)\bISO[- /]*(?:IEC)?[- /]*(?:IEEE)?[- /]*[0-9]+`),
	regexp.MustCompile(`(?i)\bSLSA\b`),
	regexp.MustCompile(`(?i)\bNIST[- ](?:SP|AI)[- ][0-9]+`),
	regexp.MustCompile(`(?i)\b21\s*CFR\s*(?:Part\s*)?[0-9]+`),
	regexp.MustCompile(`(?i)\bEudraLex[- ](?:Volume|Vol)?\s*[0-9]+`),
	regexp.MustCompile(`(?i)\bNASA[- ]NPR[- ][0-9]+`),
	regexp.MustCompile(`(?i)\bFDA[- ](?:CSA|GPSV|DATA)`),
}

// RegisterEntry represents one entry in the external reference register.
type RegisterEntry struct {
	ID             string `yaml:"id" json:"id"`
	Title          string `yaml:"title" json:"title"`
	EvidenceStatus string `yaml:"evidence_status" json:"evidence_status"`
	CheckedOn      string `yaml:"checked_on" json:"checked_on"`
}

// Register holds parsed external reference register data.
type Register struct {
	SchemaVersion string          `yaml:"schema_version"`
	References    []RegisterEntry `yaml:"references"`
}

// CitedRef represents an external reference found in documentation.
type CitedRef struct {
	RawMatch string `json:"raw_match"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// AlignmentFinding reports a reference governance issue.
type AlignmentFinding struct {
	RefMatch       string `json:"ref_match"`
	File           string `json:"file"`
	Line           int    `json:"line"`
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	RegisterID     string `json:"register_id,omitempty"`
	EvidenceStatus string `json:"evidence_status,omitempty"`
}

// AlignmentResult is the output of the reference alignment gate.
type AlignmentResult struct {
	Pass      bool               `json:"pass"`
	Cited     []CitedRef         `json:"cited"`
	Governed  []string           `json:"governed"`
	Findings  []AlignmentFinding `json:"findings"`
}

// CheckAlignment scans documentation files for external reference citations
// and verifies each has a governed entry in the register.
func CheckAlignment(docFiles []string, registerPath string) (AlignmentResult, error) {
	register, err := LoadRegister(registerPath)
	if err != nil {
		return AlignmentResult{}, err
	}

	cited := scanFiles(docFiles)

	result := AlignmentResult{
		Cited:    cited,
		Governed: governedIDs(register),
	}

	// Build lookup index: normalized ref fragments → register entry.
	index := buildIndex(register)

	seen := map[string]bool{}
	for _, ref := range cited {
		normalized := normalizeMatch(ref.RawMatch)
		entry, found := matchRegister(normalized, index)

		key := normalized + "|" + ref.File
		if seen[key] {
			continue
		}
		seen[key] = true

		if !found {
			result.Findings = append(result.Findings, AlignmentFinding{
				RefMatch: ref.RawMatch,
				File:     ref.File,
				Line:     ref.Line,
				Code:     "ref_not_governed",
				Severity: "error",
				Message:  fmt.Sprintf("external reference %q is cited but has no entry in the reference register", ref.RawMatch),
			})
		} else {
			// Check evidence_status and review date.
			if entry.EvidenceStatus == "" {
				result.Findings = append(result.Findings, AlignmentFinding{
					RefMatch:   ref.RawMatch,
					File:       ref.File,
					Line:       ref.Line,
					Code:       "ref_missing_evidence_status",
					Severity:   "error",
					RegisterID: entry.ID,
					Message:    fmt.Sprintf("reference %q (register: %s) has no evidence_status", ref.RawMatch, entry.ID),
				})
			}
			if entry.CheckedOn == "" {
				result.Findings = append(result.Findings, AlignmentFinding{
					RefMatch:   ref.RawMatch,
					File:       ref.File,
					Line:       ref.Line,
					Code:       "ref_missing_review_date",
					Severity:   "warning",
					RegisterID: entry.ID,
					Message:    fmt.Sprintf("reference %q (register: %s) has no checked_on date", ref.RawMatch, entry.ID),
				})
			} else if isStale(entry.CheckedOn, 365) {
				result.Findings = append(result.Findings, AlignmentFinding{
					RefMatch:       ref.RawMatch,
					File:           ref.File,
					Line:           ref.Line,
					Code:           "ref_review_stale",
					Severity:       "warning",
					RegisterID:     entry.ID,
					EvidenceStatus: entry.EvidenceStatus,
					Message:        fmt.Sprintf("reference %q (register: %s) review date %s is older than 1 year", ref.RawMatch, entry.ID, entry.CheckedOn),
				})
			}
		}
	}

	// Deduplicate findings by code+register_id+file.
	result.Findings = dedup(result.Findings)
	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].File == result.Findings[j].File {
			return result.Findings[i].Line < result.Findings[j].Line
		}
		return result.Findings[i].File < result.Findings[j].File
	})

	// Gate passes only if no error-severity findings.
	result.Pass = true
	for _, f := range result.Findings {
		if f.Severity == "error" {
			result.Pass = false
			break
		}
	}

	return result, nil
}

// GateError returns an error if the gate failed, nil otherwise.
func GateError(result AlignmentResult) error {
	if result.Pass {
		return nil
	}
	errorCount := 0
	for _, f := range result.Findings {
		if f.Severity == "error" {
			errorCount++
		}
	}
	return fmt.Errorf("%w: %d ungoverned or incomplete references", ErrGateFailed, errorCount)
}

// LoadRegister parses the external reference register YAML.
func LoadRegister(path string) (Register, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Register{}, fmt.Errorf("%w: %s", ErrRegisterNotFound, path)
		}
		return Register{}, err
	}
	var reg Register
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return Register{}, fmt.Errorf("parse register %s: %w", path, err)
	}
	return reg, nil
}

func scanFiles(files []string) []CitedRef {
	var cited []CitedRef
	for _, file := range files {
		refs := scanFileForRefs(file)
		cited = append(cited, refs...)
	}
	return cited
}

func scanFileForRefs(path string) []CitedRef {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var refs []CitedRef
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, pattern := range refPatterns {
			matches := pattern.FindAllString(line, -1)
			for _, m := range matches {
				refs = append(refs, CitedRef{
					RawMatch: strings.TrimSpace(m),
					File:     path,
					Line:     lineNum,
				})
			}
		}
	}
	return refs
}

func governedIDs(reg Register) []string {
	ids := make([]string, 0, len(reg.References))
	for _, entry := range reg.References {
		ids = append(ids, entry.ID)
	}
	sort.Strings(ids)
	return ids
}

type indexEntry struct {
	fragments []string
	entry     RegisterEntry
}

func buildIndex(reg Register) []indexEntry {
	entries := make([]indexEntry, 0, len(reg.References))
	for _, ref := range reg.References {
		frags := extractFragments(ref.ID, ref.Title)
		entries = append(entries, indexEntry{fragments: frags, entry: ref})
	}
	return entries
}

func extractFragments(id, title string) []string {
	var frags []string
	// ID fragments: split by dash and take meaningful parts.
	frags = append(frags, strings.ToLower(id))
	parts := strings.Split(id, "-")
	for _, p := range parts {
		if len(p) > 2 {
			frags = append(frags, strings.ToLower(p))
		}
	}
	// Title fragments.
	titleLower := strings.ToLower(title)
	frags = append(frags, titleLower)
	return frags
}

func matchRegister(normalized string, index []indexEntry) (RegisterEntry, bool) {
	norm := strings.ToLower(normalized)
	// Direct ID match.
	for _, ie := range index {
		if strings.ToLower(ie.entry.ID) == norm {
			return ie.entry, true
		}
	}
	// Fragment match.
	for _, ie := range index {
		for _, frag := range ie.fragments {
			if strings.Contains(frag, norm) || strings.Contains(norm, frag) {
				return ie.entry, true
			}
		}
	}
	// Fuzzy: match significant tokens.
	tokens := significantTokens(norm)
	for _, ie := range index {
		idLower := strings.ToLower(ie.entry.ID)
		matched := 0
		for _, tok := range tokens {
			if strings.Contains(idLower, tok) {
				matched++
			}
		}
		if matched > 0 && matched >= len(tokens)/2+1 {
			return ie.entry, true
		}
	}
	return RegisterEntry{}, false
}

func significantTokens(s string) []string {
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return ' '
	}, s)
	var tokens []string
	for _, tok := range strings.Fields(s) {
		if len(tok) >= 2 {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

func normalizeMatch(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, "  ", " ")
	return s
}

func isStale(dateStr string, days int) bool {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	return time.Since(t) > time.Duration(days)*24*time.Hour
}

func dedup(findings []AlignmentFinding) []AlignmentFinding {
	seen := map[string]bool{}
	var result []AlignmentFinding
	for _, f := range findings {
		key := fmt.Sprintf("%s|%s|%s|%s", f.Code, f.RegisterID, f.RefMatch, f.File)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, f)
	}
	return result
}
