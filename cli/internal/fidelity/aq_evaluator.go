package fidelity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// AQLevel represents Nomos Quality levels.
type AQLevel string

const (
	AQNQ0 AQLevel = "NQ-0" // no quality claim
	AQNQ1 AQLevel = "NQ-1" // method draft, documentation baseline
	AQNQ2 AQLevel = "NQ-2" // executable checks, CI gates, evidence present
	AQNQ3 AQLevel = "NQ-3" // validated, reviewed, auditable
)

// AQClaimKind classifies a detected claim.
type AQClaimKind string

const (
	ClaimQuality    AQClaimKind = "quality"
	ClaimCompliance AQClaimKind = "compliance"
	ClaimValidation AQClaimKind = "validation"
	ClaimCertified  AQClaimKind = "certified"
	ClaimRegulated  AQClaimKind = "regulated"
)

// DetectedClaim is a claim found in a document.
type DetectedClaim struct {
	Text       string      `json:"text"`
	Kind       AQClaimKind `json:"kind"`
	SourcePath string      `json:"source_path"`
	Line       int         `json:"line"`
}

// AQEvidenceLevel describes what evidence backs a claim.
type AQEvidenceLevel struct {
	HasDocumentation bool `json:"has_documentation"`
	HasCIGate        bool `json:"has_ci_gate"`
	HasTests         bool `json:"has_tests"`
	HasReview        bool `json:"has_review"`
	HasAuditTrail    bool `json:"has_audit_trail"`
	HasApproval      bool `json:"has_approval"`
}

// MaxSupportedLevel returns the highest NQ level the evidence supports.
func (e AQEvidenceLevel) MaxSupportedLevel() AQLevel {
	if e.HasApproval && e.HasAuditTrail && e.HasReview &&
		e.HasTests && e.HasCIGate && e.HasDocumentation {
		return AQNQ3
	}
	if e.HasTests && e.HasCIGate && e.HasDocumentation {
		return AQNQ2
	}
	if e.HasDocumentation {
		return AQNQ1
	}
	return AQNQ0
}

// EvaluatedClaim is a claim with its evaluation result.
type EvaluatedClaim struct {
	Claim          DetectedClaim `json:"claim"`
	RequiredLevel  AQLevel       `json:"required_level"`
	SupportedLevel AQLevel       `json:"supported_level"`
	Supported      bool          `json:"supported"`
	Blocking       bool          `json:"blocking"`
	Reason         string        `json:"reason"`
}

// AQEvaluationResult is the output of the AQ evaluator.
type AQEvaluationResult struct {
	CurrentLevel AQLevel          `json:"current_level"`
	TotalClaims  int              `json:"total_claims"`
	Supported    int              `json:"supported"`
	Unsupported  int              `json:"unsupported"`
	Blocking     int              `json:"blocking"`
	Claims       []EvaluatedClaim `json:"claims"`
}

type claimRule struct {
	pattern *regexp.Regexp
	kind    AQClaimKind
	level   AQLevel
}

var claimRules = []claimRule{
	{regexp.MustCompile(`(?i)\bcertified\b`), ClaimCertified, AQNQ3},
	{regexp.MustCompile(`(?i)\bregulated[- ]grade\b`), ClaimRegulated, AQNQ3},
	{regexp.MustCompile(`(?i)\bpart\s*11\s*compli`), ClaimRegulated, AQNQ3},
	{regexp.MustCompile(`(?i)\bgxp\s*compli`), ClaimRegulated, AQNQ3},
	{regexp.MustCompile(`(?i)\biso\s*\d+\s*compli`), ClaimCompliance, AQNQ3},
	{regexp.MustCompile(`(?i)\bfully\s+validated\b`), ClaimValidation, AQNQ3},
	{regexp.MustCompile(`(?i)\baudited\b.*\bcompli`), ClaimCompliance, AQNQ3},
	{regexp.MustCompile(`(?i)\bvalidat(ed|ion)\b`), ClaimValidation, AQNQ2},
	{regexp.MustCompile(`(?i)\bcompli(ant|ance)\b`), ClaimCompliance, AQNQ2},
	{regexp.MustCompile(`(?i)\bquality\s*(system|management|assured)\b`), ClaimQuality, AQNQ2},
	{regexp.MustCompile(`(?i)\bverified\b`), ClaimValidation, AQNQ1},
	{regexp.MustCompile(`(?i)\btested\b`), ClaimQuality, AQNQ1},
}

// ScanClaimsFromReader scans lines for AQ claims.
func ScanClaimsFromReader(r io.Reader, sourcePath string) ([]DetectedClaim, error) {
	var claims []DetectedClaim
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		for _, rule := range claimRules {
			if rule.pattern.MatchString(line) {
				claims = append(claims, DetectedClaim{
					Text:       strings.TrimSpace(line),
					Kind:       rule.kind,
					SourcePath: sourcePath,
					Line:       lineNum,
				})
				break // one claim per line
			}
		}
	}
	return claims, scanner.Err()
}

// ScanClaims scans a single file for AQ claims.
func ScanClaims(filePath string) ([]DetectedClaim, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ScanClaimsFromReader(f, filePath)
}

// ScanClaimsInDir scans all .md and .yaml files under root.
func ScanClaimsInDir(root string) ([]DetectedClaim, error) {
	var all []DetectedClaim
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			b := strings.ToLower(d.Name())
			if b == ".git" || b == "node_modules" || b == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		claims, scanErr := ScanClaims(path)
		if scanErr != nil {
			return nil
		}
		for i := range claims {
			claims[i].SourcePath = filepath.ToSlash(rel)
		}
		all = append(all, claims...)
		return nil
	})
	return all, err
}

// EvaluateClaims evaluates detected claims against the evidence level.
func EvaluateClaims(claims []DetectedClaim, evidence AQEvidenceLevel) AQEvaluationResult {
	current := evidence.MaxSupportedLevel()
	result := AQEvaluationResult{
		CurrentLevel: current,
		TotalClaims:  len(claims),
	}
	for _, c := range claims {
		req := matchRequiredLevel(c)
		ok := aqOrd(current) >= aqOrd(req)
		ec := EvaluatedClaim{
			Claim:          c,
			RequiredLevel:  req,
			SupportedLevel: current,
			Supported:      ok,
			Blocking:       !ok,
		}
		if ok {
			ec.Reason = fmt.Sprintf("claim requires %s, evidence supports %s", req, current)
			result.Supported++
		} else {
			ec.Reason = fmt.Sprintf("claim requires %s but evidence only supports %s — overclaim", req, current)
			result.Unsupported++
			result.Blocking++
		}
		result.Claims = append(result.Claims, ec)
	}
	return result
}

// WriteAQResultJSON serializes the result.
func WriteAQResultJSON(w io.Writer, result AQEvaluationResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func matchRequiredLevel(c DetectedClaim) AQLevel {
	for _, rule := range claimRules {
		if rule.pattern.MatchString(c.Text) {
			return rule.level
		}
	}
	return AQNQ1
}

func aqOrd(l AQLevel) int {
	switch l {
	case AQNQ0:
		return 0
	case AQNQ1:
		return 1
	case AQNQ2:
		return 2
	case AQNQ3:
		return 3
	}
	return 0
}
