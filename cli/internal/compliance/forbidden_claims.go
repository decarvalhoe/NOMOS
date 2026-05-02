package compliance

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ForbiddenClaim defines a claim that must not appear in public docs.
type ForbiddenClaim struct {
	ID                    string   `yaml:"id"                      json:"id"`
	Pattern               string   `yaml:"pattern"                 json:"pattern"`
	Aliases               []string `yaml:"aliases"                 json:"aliases"`
	Reason                string   `yaml:"reason"                  json:"reason"`
	RequiredEvidenceLevel string   `yaml:"required_evidence_level" json:"required_evidence_level"`
	CurrentStatus         string   `yaml:"current_status"          json:"current_status"`
}

// AllowedAlternative suggests safe wording.
type AllowedAlternative struct {
	InsteadOf string `yaml:"instead_of" json:"instead_of"`
	Use       string `yaml:"use"        json:"use"`
}

// ForbiddenClaimsRegistry is the top-level YAML structure.
type ForbiddenClaimsRegistry struct {
	SchemaVersion     string               `yaml:"schema_version"`
	DocumentID        string               `yaml:"document_id"`
	Status            string               `yaml:"status"`
	Owner             string               `yaml:"owner"`
	Product           string               `yaml:"product"`
	ForbiddenClaims   []ForbiddenClaim     `yaml:"forbidden_claims"`
	AllowedAlternatives []AllowedAlternative `yaml:"allowed_alternatives"`
}

// ForbiddenClaimViolation is a single occurrence of a forbidden claim.
type ForbiddenClaimViolation struct {
	ClaimID     string `json:"claim_id"`
	Pattern     string `json:"pattern"`
	SourcePath  string `json:"source_path"`
	Line        int    `json:"line"`
	LineText    string `json:"line_text"`
	Reason      string `json:"reason"`
	Alternative string `json:"alternative,omitempty"`
}

// ForbiddenClaimsResult holds the gate evaluation output.
type ForbiddenClaimsResult struct {
	TotalFiles  int                       `json:"total_files"`
	Violations  []ForbiddenClaimViolation  `json:"violations"`
	GateVerdict string                     `json:"gate_verdict"`
}

// LoadForbiddenClaims reads the forbidden claims YAML registry.
func LoadForbiddenClaims(path string) (ForbiddenClaimsRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ForbiddenClaimsRegistry{}, fmt.Errorf("read forbidden claims: %w", err)
	}
	var reg ForbiddenClaimsRegistry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return ForbiddenClaimsRegistry{}, fmt.Errorf("parse forbidden claims: %w", err)
	}
	return reg, nil
}

// compiledForbiddenClaim holds a pre-compiled regex for a forbidden claim.
type compiledForbiddenClaim struct {
	claim   ForbiddenClaim
	re      *regexp.Regexp
	altText string
}

// ScanForbiddenClaims scans Markdown files for forbidden claims.
func ScanForbiddenClaims(paths []string, registry ForbiddenClaimsRegistry) (ForbiddenClaimsResult, error) {
	compiled := compileClaims(registry)

	var violations []ForbiddenClaimViolation
	totalFiles := 0

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return ForbiddenClaimsResult{}, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			v, n, err := scanDirForbidden(p, compiled)
			if err != nil {
				return ForbiddenClaimsResult{}, err
			}
			violations = append(violations, v...)
			totalFiles += n
		} else {
			v, err := scanFileForbidden(p, compiled)
			if err != nil {
				return ForbiddenClaimsResult{}, err
			}
			violations = append(violations, v...)
			totalFiles++
		}
	}

	verdict := "pass"
	if len(violations) > 0 {
		verdict = "fail"
	}

	return ForbiddenClaimsResult{
		TotalFiles:  totalFiles,
		Violations:  violations,
		GateVerdict: verdict,
	}, nil
}

// forbiddenExclusionPatterns match lines that should be ignored
// (methodology descriptions, negations, headings, code blocks).
var forbiddenExclusionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(does not claim|not compliant|not yet|not certified|no guarantee)\b`),
	regexp.MustCompile(`(?i)\b(must have|must exist|should|will be|planned|roadmap)\b`),
	regexp.MustCompile(`(?i)\b(forbidden|prohibited|not allowed|must not)\b`),
	regexp.MustCompile(`(?i)\b(instead of|use:|alternative)\b`),
	regexp.MustCompile(`(?i)^\s*#`),
	regexp.MustCompile("(?i)^\\s*`"),
	regexp.MustCompile(`(?i)^\s*\|`), // table rows
}

func compileClaims(registry ForbiddenClaimsRegistry) []compiledForbiddenClaim {
	altMap := map[string]string{}
	for _, a := range registry.AllowedAlternatives {
		altMap[strings.ToLower(a.InsteadOf)] = a.Use
	}

	var result []compiledForbiddenClaim
	for _, fc := range registry.ForbiddenClaims {
		allPatterns := []string{regexp.QuoteMeta(fc.Pattern)}
		for _, alias := range fc.Aliases {
			allPatterns = append(allPatterns, regexp.QuoteMeta(alias))
		}
		combined := `(?i)\b(?:` + strings.Join(allPatterns, "|") + `)\b`
		// Handle hyphenated patterns: also match without hyphens
		combined = strings.ReplaceAll(combined, `\-`, `[-\\s]?`)

		re, err := regexp.Compile(combined)
		if err != nil {
			continue
		}
		result = append(result, compiledForbiddenClaim{
			claim:   fc,
			re:      re,
			altText: altMap[strings.ToLower(fc.Pattern)],
		})
	}
	return result
}

func scanDirForbidden(dir string, compiled []compiledForbiddenClaim) ([]ForbiddenClaimViolation, int, error) {
	var violations []ForbiddenClaimViolation
	count := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".mdx" {
			return nil
		}
		v, err := scanFileForbidden(path, compiled)
		if err != nil {
			return err
		}
		violations = append(violations, v...)
		count++
		return nil
	})
	return violations, count, err
}

func scanFileForbidden(path string, compiled []compiledForbiddenClaim) ([]ForbiddenClaimViolation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var violations []ForbiddenClaimViolation
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if isForbiddenExcluded(line) {
			continue
		}

		for _, c := range compiled {
			if c.re.MatchString(line) {
				violations = append(violations, ForbiddenClaimViolation{
					ClaimID:     c.claim.ID,
					Pattern:     c.claim.Pattern,
					SourcePath:  path,
					Line:        lineNum,
					LineText:    strings.TrimSpace(line),
					Reason:      c.claim.Reason,
					Alternative: c.altText,
				})
			}
		}
	}
	return violations, scanner.Err()
}

func isForbiddenExcluded(line string) bool {
	for _, p := range forbiddenExclusionPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}
