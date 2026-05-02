package compliance

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EvidenceStatus captures the lifecycle status of an evidence entry.
type EvidenceStatus string

const (
	EvidencePresent          EvidenceStatus = "present_draft"
	EvidenceDraftNotEffective EvidenceStatus = "draft_not_effective"
	EvidenceRequiresEvidence EvidenceStatus = "requires_evidence"
	EvidenceGeneratedByCI    EvidenceStatus = "generated_by_workflow_when_run"
	EvidenceEffective        EvidenceStatus = "effective"
	EvidenceArchived         EvidenceStatus = "archived"
)

// IsValid returns true if the status is recognized.
func (s EvidenceStatus) IsValid() bool {
	switch s {
	case EvidencePresent, EvidenceDraftNotEffective, EvidenceRequiresEvidence,
		EvidenceGeneratedByCI, EvidenceEffective, EvidenceArchived:
		return true
	default:
		return false
	}
}

// IsActionable returns true if the evidence can support claims.
func (s EvidenceStatus) IsActionable() bool {
	switch s {
	case EvidencePresent, EvidenceGeneratedByCI, EvidenceEffective:
		return true
	default:
		return false
	}
}

// GapSeverity indicates the impact of a blocking gap.
type GapSeverity string

const (
	SeverityMinor    GapSeverity = "minor"
	SeverityMajor    GapSeverity = "major"
	SeverityCritical GapSeverity = "critical"
)

// GapStatus tracks the resolution state.
type GapStatus string

const (
	GapOpen     GapStatus = "open"
	GapAccepted GapStatus = "accepted"
	GapResolved GapStatus = "resolved"
)

// EvidenceCategory is a single evidence entry in the ledger.
type EvidenceCategory struct {
	ID               string         `yaml:"id" json:"id"`
	Category         string         `yaml:"category" json:"category"`
	ExpectedLocation string         `yaml:"expected_location" json:"expected_location"`
	CurrentStatus    EvidenceStatus `yaml:"current_status" json:"current_status"`
	ClaimAllowed     string         `yaml:"claim_allowed" json:"claim_allowed"`
}

// BlockingGap is a gap that prevents certain claims.
type BlockingGap struct {
	ID           string      `yaml:"id" json:"id"`
	Description  string      `yaml:"description" json:"description"`
	Severity     GapSeverity `yaml:"severity" json:"severity"`
	Status       GapStatus   `yaml:"status" json:"status"`
	BlocksClaims []string    `yaml:"blocks_claims" json:"blocks_claims"`
}

// EvidenceLedger is the on-disk representation of the evidence ledger.
type EvidenceLedger struct {
	SchemaVersion      string             `yaml:"schema_version" json:"schema_version"`
	GeneratedAt        string             `yaml:"generated_at" json:"generated_at"`
	Status             string             `yaml:"status" json:"status"`
	ClaimBoundary      string             `yaml:"claim_boundary" json:"claim_boundary"`
	EvidenceCategories []EvidenceCategory `yaml:"evidence_categories" json:"evidence_categories"`
	BlockingGaps       []BlockingGap      `yaml:"blocking_gaps" json:"blocking_gaps"`
}

// LedgerVerification is the result of verifying a ledger.
type LedgerVerification struct {
	Valid             bool              `json:"valid"`
	TotalEntries      int              `json:"total_entries"`
	ActionableEntries int              `json:"actionable_entries"`
	BlockedEntries    int              `json:"blocked_entries"`
	OpenGaps          int              `json:"open_gaps"`
	BlockedClaims     []string         `json:"blocked_claims"`
	Findings          []LedgerFinding  `json:"findings"`
	ComplianceRatio   float64          `json:"compliance_ratio"`
}

// LedgerFinding is a single issue found during verification.
type LedgerFinding struct {
	EntryID     string `json:"entry_id"`
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Remediation string `json:"remediation"`
}

// LoadLedger reads and parses an evidence ledger YAML file.
func LoadLedger(path string) (EvidenceLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EvidenceLedger{}, fmt.Errorf("reading ledger: %w", err)
	}
	return ParseLedger(data)
}

// ParseLedger parses evidence ledger YAML bytes.
func ParseLedger(data []byte) (EvidenceLedger, error) {
	var ledger EvidenceLedger
	if err := yaml.Unmarshal(data, &ledger); err != nil {
		return EvidenceLedger{}, fmt.Errorf("parsing ledger: %w", err)
	}
	return ledger, nil
}

// VerifyLedger checks an evidence ledger for completeness and consistency.
func VerifyLedger(ledger EvidenceLedger, repoRoot string) LedgerVerification {
	var findings []LedgerFinding
	actionable := 0
	blocked := 0

	for _, entry := range ledger.EvidenceCategories {
		entryFindings := verifyEntry(entry, repoRoot)
		findings = append(findings, entryFindings...)

		if entry.CurrentStatus.IsActionable() {
			actionable++
		} else {
			blocked++
		}
	}

	// Collect blocked claims from open gaps.
	blockedClaims := collectBlockedClaims(ledger.BlockingGaps)
	openGaps := countOpenGaps(ledger.BlockingGaps)

	// Validate gaps.
	for _, gap := range ledger.BlockingGaps {
		gapFindings := verifyGap(gap)
		findings = append(findings, gapFindings...)
	}

	total := len(ledger.EvidenceCategories)
	ratio := 0.0
	if total > 0 {
		ratio = float64(actionable) / float64(total)
	}

	valid := len(findings) == 0

	return LedgerVerification{
		Valid:             valid,
		TotalEntries:      total,
		ActionableEntries: actionable,
		BlockedEntries:    blocked,
		OpenGaps:          openGaps,
		BlockedClaims:     blockedClaims,
		Findings:          findings,
		ComplianceRatio:   ratio,
	}
}

// RegisterEntry adds or updates an evidence entry in the ledger.
func RegisterEvidenceEntry(ledger *EvidenceLedger, entry EvidenceCategory) {
	for i, existing := range ledger.EvidenceCategories {
		if existing.ID == entry.ID {
			ledger.EvidenceCategories[i] = entry
			return
		}
	}
	ledger.EvidenceCategories = append(ledger.EvidenceCategories, entry)
}

// UpdateEntryStatus updates the status of an evidence entry by ID.
func UpdateEntryStatus(ledger *EvidenceLedger, entryID string, status EvidenceStatus) error {
	for i, entry := range ledger.EvidenceCategories {
		if entry.ID == entryID {
			ledger.EvidenceCategories[i].CurrentStatus = status
			return nil
		}
	}
	return fmt.Errorf("entry %q not found in ledger", entryID)
}

// ResolveGap marks a gap as resolved.
func ResolveGap(ledger *EvidenceLedger, gapID string, _ time.Time) error {
	for i, gap := range ledger.BlockingGaps {
		if gap.ID == gapID {
			ledger.BlockingGaps[i].Status = GapResolved
			return nil
		}
	}
	return fmt.Errorf("gap %q not found in ledger", gapID)
}

// ValidateLedger checks the ledger for structural validity.
func ValidateLedger(ledger EvidenceLedger) []string {
	var errs []string

	if strings.TrimSpace(ledger.SchemaVersion) == "" {
		errs = append(errs, "schema_version is required")
	}
	if strings.TrimSpace(ledger.GeneratedAt) == "" {
		errs = append(errs, "generated_at is required")
	}
	if len(ledger.EvidenceCategories) == 0 {
		errs = append(errs, "at least one evidence_category is required")
	}

	seen := map[string]bool{}
	for i, entry := range ledger.EvidenceCategories {
		if strings.TrimSpace(entry.ID) == "" {
			errs = append(errs, fmt.Sprintf("evidence_categories[%d].id is required", i))
		} else if seen[entry.ID] {
			errs = append(errs, fmt.Sprintf("evidence_categories[%d].id %q is duplicated", i, entry.ID))
		} else {
			seen[entry.ID] = true
		}
		if strings.TrimSpace(entry.Category) == "" {
			errs = append(errs, fmt.Sprintf("evidence_categories[%d].category is required", i, ))
		}
		if strings.TrimSpace(entry.ExpectedLocation) == "" {
			errs = append(errs, fmt.Sprintf("evidence_categories[%d].expected_location is required", i))
		}
	}

	gapSeen := map[string]bool{}
	for i, gap := range ledger.BlockingGaps {
		if strings.TrimSpace(gap.ID) == "" {
			errs = append(errs, fmt.Sprintf("blocking_gaps[%d].id is required", i))
		} else if gapSeen[gap.ID] {
			errs = append(errs, fmt.Sprintf("blocking_gaps[%d].id %q is duplicated", i, gap.ID))
		} else {
			gapSeen[gap.ID] = true
		}
		if len(gap.BlocksClaims) == 0 {
			errs = append(errs, fmt.Sprintf("blocking_gaps[%d].blocks_claims must not be empty", i))
		}
	}

	return errs
}

func verifyEntry(entry EvidenceCategory, repoRoot string) []LedgerFinding {
	var findings []LedgerFinding

	if entry.CurrentStatus == EvidenceRequiresEvidence {
		findings = append(findings, LedgerFinding{
			EntryID:     entry.ID,
			Code:        "EVIDENCE_MISSING",
			Severity:    "high",
			Message:     fmt.Sprintf("Evidence %s (%s) requires evidence but none exists.", entry.ID, entry.Category),
			Remediation: fmt.Sprintf("Provide evidence at %s.", entry.ExpectedLocation),
		})
	}

	if repoRoot != "" && entry.CurrentStatus.IsActionable() {
		path := entry.ExpectedLocation
		if !strings.HasPrefix(path, "/") {
			path = repoRoot + "/" + path
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			findings = append(findings, LedgerFinding{
				EntryID:     entry.ID,
				Code:        "EVIDENCE_FILE_MISSING",
				Severity:    "medium",
				Message:     fmt.Sprintf("Evidence %s claims %s status but file %s not found.", entry.ID, entry.CurrentStatus, entry.ExpectedLocation),
				Remediation: fmt.Sprintf("Create or restore %s.", entry.ExpectedLocation),
			})
		}
	}

	return findings
}

func verifyGap(gap BlockingGap) []LedgerFinding {
	var findings []LedgerFinding

	if gap.Status == GapOpen && (gap.Severity == SeverityMajor || gap.Severity == SeverityCritical) {
		findings = append(findings, LedgerFinding{
			EntryID:     gap.ID,
			Code:        "GAP_BLOCKING",
			Severity:    "high",
			Message:     fmt.Sprintf("Gap %s is open (%s): %s", gap.ID, gap.Severity, gap.Description),
			Remediation: fmt.Sprintf("Resolve gap %s to unblock claims: %s.", gap.ID, strings.Join(gap.BlocksClaims, ", ")),
		})
	}

	return findings
}

func collectBlockedClaims(gaps []BlockingGap) []string {
	claimSet := map[string]bool{}
	for _, gap := range gaps {
		if gap.Status == GapOpen {
			for _, claim := range gap.BlocksClaims {
				claimSet[claim] = true
			}
		}
	}
	var claims []string
	for claim := range claimSet {
		claims = append(claims, claim)
	}
	sort.Strings(claims)
	return claims
}

func countOpenGaps(gaps []BlockingGap) int {
	count := 0
	for _, gap := range gaps {
		if gap.Status == GapOpen {
			count++
		}
	}
	return count
}
