package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChainLink identifies one step in the evidence chain.
type ChainLink struct {
	Name   string `json:"name"`
	Status string `json:"status"` // present, missing, invalid
	Detail string `json:"detail"`
}

// ReconstructionResult is the outcome for a single validation entry.
type ReconstructionResult struct {
	ValidationID  string      `json:"validation_id"`
	Title         string      `json:"title"`
	RiskLevel     string      `json:"risk_level"`
	Reconstructed bool        `json:"reconstructed"`
	Chain         []ChainLink `json:"chain"`
	MissingLinks  int         `json:"missing_links"`
}

// ReconstructionReviewResult is the full review outcome.
type ReconstructionReviewResult struct {
	TotalValidations int                    `json:"total_validations"`
	Reconstructed    int                    `json:"reconstructed"`
	Failed           int                    `json:"failed"`
	TotalMissing     int                    `json:"total_missing"`
	Verdict          string                 `json:"verdict"`
	Results          []ReconstructionResult `json:"results"`
}

// ReconstructionConfig holds paths needed for the review.
type ReconstructionConfig struct {
	RepoRoot      string
	InventoryPath string
	LedgerPath    string
	IntendedUsePath string
}

// DefaultReconstructionConfig returns the config for the Nomos repo root.
func DefaultReconstructionConfig(repoRoot string) ReconstructionConfig {
	return ReconstructionConfig{
		RepoRoot:        repoRoot,
		InventoryPath:   filepath.Join(repoRoot, "docs", "regulated", "validation-pack", "validation-inventory.yaml"),
		LedgerPath:      filepath.Join(repoRoot, "docs", "regulated", "evidence-index", "evidence-ledger.yaml"),
		IntendedUsePath: filepath.Join(repoRoot, "docs", "regulated", "validation-pack", "intended-use-model.yaml"),
	}
}

// RunReconstructionReview checks that every validation entry has a
// complete evidence chain that an independent reviewer can follow.
func RunReconstructionReview(config ReconstructionConfig) (ReconstructionReviewResult, error) {
	inv, err := LoadInventory(config.InventoryPath)
	if err != nil {
		return ReconstructionReviewResult{}, fmt.Errorf("load inventory: %w", err)
	}

	var results []ReconstructionResult
	reconstructed := 0
	failed := 0
	totalMissing := 0

	for _, v := range inv.Validations {
		r := reconstructEntry(v, config)
		results = append(results, r)
		totalMissing += r.MissingLinks
		if r.Reconstructed {
			reconstructed++
		} else {
			failed++
		}
	}

	verdict := "passed"
	if failed > 0 {
		verdict = "failed"
	}

	return ReconstructionReviewResult{
		TotalValidations: len(inv.Validations),
		Reconstructed:    reconstructed,
		Failed:           failed,
		TotalMissing:     totalMissing,
		Verdict:          verdict,
		Results:          results,
	}, nil
}

func reconstructEntry(v ValidationEntry, config ReconstructionConfig) ReconstructionResult {
	var chain []ChainLink
	missingCount := 0

	// Link 1: Validation entry has an ID and title.
	chain = append(chain, checkLink("validation_entry",
		v.ID != "" && v.Title != "",
		fmt.Sprintf("id=%s title=%q", v.ID, v.Title)))

	// Link 2: Risk level is valid.
	chain = append(chain, checkLink("risk_level",
		validRiskLevels[v.RiskLevel],
		v.RiskLevel))

	// Link 3: Method is documented.
	chain = append(chain, checkLink("method",
		strings.TrimSpace(v.Method) != "",
		truncateDetail(v.Method, 60)))

	// Link 4: Evidence artifact is declared.
	chain = append(chain, checkLink("evidence_artifact",
		strings.TrimSpace(v.EvidenceArtifact) != "",
		truncateDetail(v.EvidenceArtifact, 60)))

	// Link 5: Acceptance gate is assigned.
	chain = append(chain, checkLink("acceptance_gate",
		strings.TrimSpace(v.AcceptanceGate) != "",
		v.AcceptanceGate))

	// Link 6: Owner is assigned.
	chain = append(chain, checkLink("owner",
		strings.TrimSpace(v.Owner) != "",
		v.Owner))

	// Link 7: Verification command is reproducible (if provided).
	if v.VerificationCommand != "" {
		chain = append(chain, checkLink("verification_command",
			isReproducibleCommand(v.VerificationCommand),
			truncateDetail(v.VerificationCommand, 60)))
	}

	// Link 8: Status is beyond planned.
	chain = append(chain, checkLink("implementation_status",
		v.Status != "" && v.Status != "not_qualified" && v.Status != "planned",
		v.Status))

	// Link 9: For high/critical risk, check test protocol exists.
	if v.RiskLevel == "high" || v.RiskLevel == "critical" {
		protocolExists := hasTestProtocol(v.ID, config.RepoRoot)
		chain = append(chain, checkLink("test_protocol",
			protocolExists,
			fmt.Sprintf("risk=%s requires executed protocol", v.RiskLevel)))
	}

	// Link 10: Intended-use traceability (if ref provided).
	if v.IntendedUseRef != "" {
		iuExists := intendedUseFunctionExists(v.IntendedUseRef, config.IntendedUsePath)
		chain = append(chain, checkLink("intended_use_trace",
			iuExists,
			v.IntendedUseRef))
	}

	for _, link := range chain {
		if link.Status == "missing" {
			missingCount++
		}
	}

	return ReconstructionResult{
		ValidationID:  v.ID,
		Title:         v.Title,
		RiskLevel:     v.RiskLevel,
		Reconstructed: missingCount == 0,
		Chain:         chain,
		MissingLinks:  missingCount,
	}
}

func checkLink(name string, ok bool, detail string) ChainLink {
	status := "present"
	if !ok {
		status = "missing"
	}
	return ChainLink{Name: name, Status: status, Detail: detail}
}

func isReproducibleCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false
	}
	// Must start with a known tool.
	for _, prefix := range []string{"go ", "cd ", "cue ", "bash ", "python"} {
		if strings.HasPrefix(cmd, prefix) {
			return true
		}
	}
	return false
}

func hasTestProtocol(validationID string, repoRoot string) bool {
	packDir := filepath.Join(repoRoot, "docs", "regulated", "validation-pack")
	entries, err := os.ReadDir(packDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasPrefix(name, "tp-") && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			// Read the file and check if it references this validation.
			data, err := os.ReadFile(filepath.Join(packDir, e.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(string(data), validationID) {
				return true
			}
		}
	}
	return false
}

func intendedUseFunctionExists(funcRef string, iuPath string) bool {
	data, err := os.ReadFile(iuPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), funcRef)
}

func truncateDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
