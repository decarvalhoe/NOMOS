package compliance

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ClaimStatus follows docs/25-regulated-by-design-structure.md Status Model.
type ClaimStatus string

const (
	StatusNotQualified ClaimStatus = "not_qualified"
	StatusPlanned      ClaimStatus = "planned"
	StatusImplemented  ClaimStatus = "implemented"
	StatusVerified     ClaimStatus = "verified"
	StatusApproved     ClaimStatus = "approved"
	StatusWaived       ClaimStatus = "waived"
	StatusBlocked      ClaimStatus = "blocked"
)

// EvidenceLevel describes the minimum evidence backing a claim.
type EvidenceLevel string

const (
	EvidenceNone           EvidenceLevel = "none"
	EvidenceIntendedUse    EvidenceLevel = "intended_use"
	EvidenceRiskClass      EvidenceLevel = "risk_classification"
	EvidenceControl        EvidenceLevel = "control_requirement"
	EvidenceImplementation EvidenceLevel = "implementation_ref"
	EvidenceVerification   EvidenceLevel = "verification_ref"
	EvidenceArtifact       EvidenceLevel = "evidence_artifact"
	EvidenceGate           EvidenceLevel = "release_gate"
)

// PublicClaim represents a claim found in public-facing documentation.
type PublicClaim struct {
	Text       string `json:"text"`
	SourcePath string `json:"source_path"`
	Line       int    `json:"line"`
}

// ClaimEvidence maps the Operating Rule fields that must back a claim.
type ClaimEvidence struct {
	IntendedUse     string `json:"intended_use,omitempty"`
	Owner           string `json:"owner,omitempty"`
	RiskClass       string `json:"risk_classification,omitempty"`
	ExternalRef     string `json:"external_ref,omitempty"`
	ControlReq      string `json:"control_requirement,omitempty"`
	ImplementRef    string `json:"implementation_ref,omitempty"`
	VerificationRef string `json:"verification_ref,omitempty"`
	EvidenceArtifact string `json:"evidence_artifact,omitempty"`
	ReleaseGate     string `json:"release_gate,omitempty"`
}

// GovernedClaim is the result of evaluating a single public claim.
type GovernedClaim struct {
	Claim         PublicClaim   `json:"claim"`
	Status        ClaimStatus   `json:"status"`
	Evidence      ClaimEvidence `json:"evidence"`
	MissingFields []string      `json:"missing_fields,omitempty"`
	Qualified     bool          `json:"qualified"`
}

// ClaimsGovernanceResult holds the full evaluation result.
type ClaimsGovernanceResult struct {
	TotalClaims    int             `json:"total_claims"`
	Qualified      int             `json:"qualified"`
	NotQualified   int             `json:"not_qualified"`
	GateVerdict    string          `json:"gate_verdict"`
	GovernedClaims []GovernedClaim `json:"governed_claims"`
}

// Claim detection patterns — matches strong affirmative language in docs.
var claimPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(comply|complies|compliant|compliance)\b`),
	regexp.MustCompile(`(?i)\b(certif(?:y|ied|ication|icate))\b`),
	regexp.MustCompile(`(?i)\b(guarantee[sd]?|warrant(?:y|ies|ed)?)\b`),
	regexp.MustCompile(`(?i)\b(ensure[sd]?|assure[sd]?)\b`),
	regexp.MustCompile(`(?i)\b(GDPR|HIPAA|SOC\s*2|ISO\s*\d+|FDA|MDR|IVDR|SOX)\b`),
	regexp.MustCompile(`(?i)\b(validated|verified|audited|attested)\b`),
	regexp.MustCompile(`(?i)\bproduction[- ]ready\b`),
	regexp.MustCompile(`(?i)\b(regulated[- ]grade|enterprise[- ]grade)\b`),
	regexp.MustCompile(`(?i)\b(zero[- ]downtime|99[.,]\d+%\s*(?:uptime|SLA|availability))\b`),
}

// Exclusion patterns — lines that describe methodology or future state,
// not active product claims.
var exclusionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(does not claim|not compliant|not yet|planned|roadmap)\b`),
	regexp.MustCompile(`(?i)\b(must have|must exist|should|will be)\b`),
	regexp.MustCompile(`(?i)^\s*#`),          // Markdown headings are not claims.
	regexp.MustCompile(`(?i)^\s*[-*]\s*\x60`), // Code-style list items.
}

// ScanPublicClaims scans Markdown files in the given paths for public claims.
func ScanPublicClaims(paths []string) ([]PublicClaim, error) {
	var claims []PublicClaim

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			dirClaims, err := scanDir(p)
			if err != nil {
				return nil, err
			}
			claims = append(claims, dirClaims...)
		} else {
			fileClaims, err := scanFile(p)
			if err != nil {
				return nil, err
			}
			claims = append(claims, fileClaims...)
		}
	}
	return claims, nil
}

// GovernClaims evaluates each claim against the provided evidence registry.
// Claims without full evidence backing are marked not_qualified.
func GovernClaims(claims []PublicClaim, registry map[string]ClaimEvidence) ClaimsGovernanceResult {
	var governed []GovernedClaim

	for _, claim := range claims {
		key := claimKey(claim)
		evidence, hasEvidence := registry[key]

		gc := GovernedClaim{
			Claim:    claim,
			Evidence: evidence,
		}

		if !hasEvidence {
			gc.Status = StatusNotQualified
			gc.MissingFields = allRequiredFields()
			gc.Qualified = false
		} else {
			missing := checkMissingFields(evidence)
			if len(missing) == 0 {
				gc.Status = StatusVerified
				gc.Qualified = true
			} else {
				gc.Status = StatusNotQualified
				gc.MissingFields = missing
				gc.Qualified = false
			}
		}

		governed = append(governed, gc)
	}

	qualified := 0
	notQualified := 0
	for _, gc := range governed {
		if gc.Qualified {
			qualified++
		} else {
			notQualified++
		}
	}

	verdict := "pass"
	if notQualified > 0 {
		verdict = "fail"
	}

	return ClaimsGovernanceResult{
		TotalClaims:    len(governed),
		Qualified:      qualified,
		NotQualified:   notQualified,
		GateVerdict:    verdict,
		GovernedClaims: governed,
	}
}

func claimKey(claim PublicClaim) string {
	return fmt.Sprintf("%s:%d", claim.SourcePath, claim.Line)
}

func allRequiredFields() []string {
	return []string{
		"intended_use", "owner", "risk_classification",
		"external_ref", "control_requirement", "implementation_ref",
		"verification_ref", "evidence_artifact", "release_gate",
	}
}

func checkMissingFields(e ClaimEvidence) []string {
	var missing []string
	if e.IntendedUse == "" {
		missing = append(missing, "intended_use")
	}
	if e.Owner == "" {
		missing = append(missing, "owner")
	}
	if e.RiskClass == "" {
		missing = append(missing, "risk_classification")
	}
	if e.ExternalRef == "" {
		missing = append(missing, "external_ref")
	}
	if e.ControlReq == "" {
		missing = append(missing, "control_requirement")
	}
	if e.ImplementRef == "" {
		missing = append(missing, "implementation_ref")
	}
	if e.VerificationRef == "" {
		missing = append(missing, "verification_ref")
	}
	if e.EvidenceArtifact == "" {
		missing = append(missing, "evidence_artifact")
	}
	if e.ReleaseGate == "" {
		missing = append(missing, "release_gate")
	}
	return missing
}

func scanDir(dir string) ([]PublicClaim, error) {
	var claims []PublicClaim
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
		fileClaims, err := scanFile(path)
		if err != nil {
			return err
		}
		claims = append(claims, fileClaims...)
		return nil
	})
	return claims, err
}

func scanFile(path string) ([]PublicClaim, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var claims []PublicClaim
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if isExcluded(line) {
			continue
		}
		if isClaim(line) {
			claims = append(claims, PublicClaim{
				Text:       strings.TrimSpace(line),
				SourcePath: path,
				Line:       lineNum,
			})
		}
	}
	return claims, scanner.Err()
}

func isClaim(line string) bool {
	for _, p := range claimPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}

func isExcluded(line string) bool {
	for _, p := range exclusionPatterns {
		if p.MatchString(line) {
			return true
		}
	}
	return false
}
