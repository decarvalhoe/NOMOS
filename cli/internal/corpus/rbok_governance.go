package corpus

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Verdict levels for governance evaluation.
const (
	VerdictAdmissible = "corpus_admissible"
	VerdictPartial    = "corpus_partial"
	VerdictBlocked    = "corpus_blocked"
)

// Finding describes a single governance issue.
type Finding struct {
	ID          string `json:"id"          yaml:"id"`
	Severity    string `json:"severity"    yaml:"severity"`
	Blocking    bool   `json:"blocking"    yaml:"blocking"`
	SourcePath  string `json:"source_path" yaml:"source_path"`
	Line        int    `json:"line"        yaml:"line"`
	Field       string `json:"field"       yaml:"field"`
	Message     string `json:"message"     yaml:"message"`
	Remediation string `json:"remediation" yaml:"remediation"`
}

// GovernanceResult holds the full evaluation outcome.
type GovernanceResult struct {
	Verdict       string    `json:"verdict"        yaml:"verdict"`
	TotalFindings int       `json:"total_findings" yaml:"total_findings"`
	Blocking      int       `json:"blocking"       yaml:"blocking"`
	Findings      []Finding `json:"findings"       yaml:"findings"`
}

// EvaluateGovernance scans a corpus root for governance metadata issues.
func EvaluateGovernance(root string) (GovernanceResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return GovernanceResult{}, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return GovernanceResult{}, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return GovernanceResult{}, fmt.Errorf("root must be a directory")
	}

	var findings []Finding
	var idx int
	canonicalRefs := map[string][]string{} // ref -> list of source paths

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := strings.ToLower(filepath.Base(path))
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		rel, _ := filepath.Rel(absRoot, path)
		rel = filepath.ToSlash(rel)
		lower := strings.ToLower(rel)

		switch {
		case strings.HasSuffix(lower, ".md"):
			fs := checkMarkdownTable(rel, path, &idx)
			findings = append(findings, fs...)
			collectCanonicalRefs(path, canonicalRefs, rel)
		case (strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")) && strings.Contains(lower, "parcours"):
			fs := checkParcoursYAML(rel, path, &idx)
			findings = append(findings, fs...)
		case strings.Contains(lower, "gov") && (strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".yaml")):
			// already covered by .md/.yaml branches
		}
		return nil
	})
	if err != nil {
		return GovernanceResult{}, fmt.Errorf("walk: %w", err)
	}

	// Check duplicate canonical refs
	for ref, paths := range canonicalRefs {
		if len(paths) > 1 {
			idx++
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("GOV-%04d", idx),
				Severity:    "medium",
				Blocking:    false,
				SourcePath:  paths[0],
				Field:       "canonical_ref",
				Message:     fmt.Sprintf("duplicate canonical_ref %q found in %d files: %s", ref, len(paths), strings.Join(paths, ", ")),
				Remediation: "Consolidate duplicate references to a single authoritative location.",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})

	blocking := 0
	for _, f := range findings {
		if f.Blocking {
			blocking++
		}
	}

	verdict := VerdictAdmissible
	if blocking > 0 {
		verdict = VerdictBlocked
	} else if len(findings) > 0 {
		verdict = VerdictPartial
	}

	return GovernanceResult{
		Verdict:       verdict,
		TotalFindings: len(findings),
		Blocking:      blocking,
		Findings:      findings,
	}, nil
}

// --- Markdown table metadata checks ---

var mdTableHeaderPattern = regexp.MustCompile(`^\|(.+)\|$`)
var mdTableRequiredHeaders = []string{"reference", "version", "status", "author"}

func checkMarkdownTable(rel string, path string, idx *int) []Finding {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var findings []Finding
	scanner := bufio.NewScanner(f)
	lineNum := 0
	inTable := false
	var tableHeaders []string
	tableStart := 0
	hasContent := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if mdTableHeaderPattern.MatchString(line) && !inTable {
			inTable = true
			tableStart = lineNum
			tableHeaders = parseTableHeaders(line)
			hasContent = false
			continue
		}

		if inTable {
			if strings.HasPrefix(line, "|") && strings.Contains(line, "---") {
				continue // separator row
			}
			if strings.HasPrefix(line, "|") {
				hasContent = true
				// Check for empty cells in data rows
				cells := parseTableCells(line)
				for i, cell := range cells {
					if strings.TrimSpace(cell) == "" && i < len(tableHeaders) {
						header := tableHeaders[i]
						if isGovernanceField(header) {
							*idx++
							findings = append(findings, Finding{
								ID:          fmt.Sprintf("GOV-%04d", *idx),
								Severity:    "medium",
								Blocking:    false,
								SourcePath:  rel,
								Line:        lineNum,
								Field:       header,
								Message:     fmt.Sprintf("empty governance field %q in table row", header),
								Remediation: fmt.Sprintf("Fill in the %q field for this table entry.", header),
							})
						}
					}
				}
				continue
			}
			// End of table
			if !hasContent && len(tableHeaders) > 0 {
				for _, required := range mdTableRequiredHeaders {
					if containsHeaderCI(tableHeaders, required) {
						*idx++
						findings = append(findings, Finding{
							ID:          fmt.Sprintf("GOV-%04d", *idx),
							Severity:    "high",
							Blocking:    true,
							SourcePath:  rel,
							Line:        tableStart,
							Field:       required,
							Message:     fmt.Sprintf("governance table with %q header has no data rows", required),
							Remediation: "Add at least one data row to the governance table.",
						})
					}
				}
			}
			inTable = false
			tableHeaders = nil
		}
	}

	// Handle table at end of file
	if inTable && !hasContent && len(tableHeaders) > 0 {
		for _, required := range mdTableRequiredHeaders {
			if containsHeaderCI(tableHeaders, required) {
				*idx++
				findings = append(findings, Finding{
					ID:          fmt.Sprintf("GOV-%04d", *idx),
					Severity:    "high",
					Blocking:    true,
					SourcePath:  rel,
					Line:        tableStart,
					Field:       required,
					Message:     fmt.Sprintf("governance table with %q header has no data rows", required),
					Remediation: "Add at least one data row to the governance table.",
				})
			}
		}
	}

	return findings
}

func parseTableHeaders(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	headers := make([]string, 0, len(parts))
	for _, p := range parts {
		headers = append(headers, strings.ToLower(strings.TrimSpace(p)))
	}
	return headers
}

func parseTableCells(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func containsHeaderCI(headers []string, target string) bool {
	target = strings.ToLower(target)
	for _, h := range headers {
		if strings.Contains(h, target) {
			return true
		}
	}
	return false
}

func isGovernanceField(header string) bool {
	for _, f := range []string{"reference", "version", "status", "author", "issuer", "owner", "date", "domain"} {
		if strings.Contains(header, f) {
			return true
		}
	}
	return false
}

// --- Parcours YAML checks ---

type parcoursCheckFile struct {
	Parcours parcoursCheckData `yaml:"parcours"`
}

type parcoursCheckData struct {
	ID      string             `yaml:"id"`
	Name    string             `yaml:"name"`
	Version string             `yaml:"version"`
	Owner   string             `yaml:"owner"`
	Status  string             `yaml:"status"`
	Domain  string             `yaml:"domain"`
	Etapes  []parcoursCheckEtape `yaml:"etapes"`
}

type parcoursCheckEtape struct {
	ID        string                  `yaml:"id"`
	Objectifs []parcoursCheckObjectif `yaml:"objectifs"`
}

type parcoursCheckObjectif struct {
	Criteres []parcoursCheckCritere `yaml:"criteres"`
}

type parcoursCheckCritere struct {
	ID string `yaml:"id"`
}

func checkParcoursYAML(rel string, path string, idx *int) []Finding {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var file parcoursCheckFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		*idx++
		return []Finding{{
			ID:          fmt.Sprintf("GOV-%04d", *idx),
			Severity:    "high",
			Blocking:    true,
			SourcePath:  rel,
			Field:       "yaml",
			Message:     fmt.Sprintf("invalid YAML: %v", err),
			Remediation: "Fix YAML syntax errors.",
		}}
	}

	p := file.Parcours
	var findings []Finding

	requiredFields := map[string]string{
		"id":      p.ID,
		"name":    p.Name,
		"version": p.Version,
		"owner":   p.Owner,
		"status":  p.Status,
		"domain":  p.Domain,
	}

	for field, value := range requiredFields {
		if strings.TrimSpace(value) == "" {
			*idx++
			severity := "high"
			blocking := true
			if field == "version" || field == "domain" {
				severity = "medium"
				blocking = false
			}
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("GOV-%04d", *idx),
				Severity:    severity,
				Blocking:    blocking,
				SourcePath:  rel,
				Field:       "parcours." + field,
				Message:     fmt.Sprintf("missing required parcours field %q", field),
				Remediation: fmt.Sprintf("Add parcours.%s to the YAML file.", field),
			})
		}
	}

	// Check for empty extractions (étapes with no critères)
	for _, etape := range p.Etapes {
		totalCriteres := 0
		for _, obj := range etape.Objectifs {
			totalCriteres += len(obj.Criteres)
		}
		if totalCriteres == 0 {
			*idx++
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("GOV-%04d", *idx),
				Severity:    "medium",
				Blocking:    false,
				SourcePath:  rel,
				Field:       fmt.Sprintf("parcours.etapes.%s", etape.ID),
				Message:     fmt.Sprintf("étape %q has no critères (empty extraction)", etape.ID),
				Remediation: "Add at least one critère to each étape objectif, or remove the empty étape.",
			})
		}
	}

	return findings
}

// --- Canonical ref dedup ---

var canonicalRefPattern = regexp.MustCompile(`(?i)canonical[_-]?ref[s]?\s*[:=]\s*(\S+)`)

func collectCanonicalRefs(path string, refs map[string][]string, rel string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		matches := canonicalRefPattern.FindStringSubmatch(scanner.Text())
		if len(matches) > 1 {
			ref := strings.TrimSpace(matches[1])
			if ref != "" {
				refs[ref] = append(refs[ref], rel)
			}
		}
	}
}
