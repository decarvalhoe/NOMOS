package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ReleaseDecision is the go/no-go outcome.
type ReleaseDecision string

const (
	DecisionGo   ReleaseDecision = "go"
	DecisionNoGo ReleaseDecision = "no-go"
	DecisionHold ReleaseDecision = "hold"
)

// AQ5ProofReport is an input proof from a prior AQ level.
type AQ5ProofReport struct {
	ID       string `json:"id"`
	AQLevel  string `json:"aq_level"` // "AQ-3", "AQ-4", etc.
	Pass     bool   `json:"pass"`
	Score    float64 `json:"score"`
	Summary  string `json:"summary"`
	Hash     string `json:"hash"`
}

// AQ5GateResult is a single gate check in the AQ-5 pack.
type AQ5GateResult struct {
	GateID   string `json:"gate_id"`
	Name     string `json:"name"`
	Pass     bool   `json:"pass"`
	Blocking bool   `json:"blocking"`
	Detail   string `json:"detail"`
}

// AQ5EvidenceItem is a single evidence artifact in the manifest.
type AQ5EvidenceItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Hash        string `json:"hash"`
	Status      string `json:"status"` // present, missing, invalid
}

// AQ5Pack is the complete AQ-5 evidence pack.
type AQ5Pack struct {
	SchemaVersion string            `json:"schema_version"`
	PackID        string            `json:"pack_id"`
	GeneratedAt   string            `json:"generated_at"`
	DocumentID    string            `json:"document_id"`
	Profile       string            `json:"profile"`
	ProofReports  []AQ5ProofReport  `json:"proof_reports"`
	EvidenceItems []AQ5EvidenceItem `json:"evidence_items"`
	GateResults   []AQ5GateResult   `json:"gate_results"`
	Decision      ReleaseDecision   `json:"decision"`
	DecisionReason string           `json:"decision_reason"`
	PackHash      string            `json:"pack_hash"`
}

// AQ5PackInput configures pack assembly.
type AQ5PackInput struct {
	DocumentID    string
	Profile       string
	ProofReports  []AQ5ProofReport
	EvidenceItems []AQ5EvidenceItem
	Attestation   *EvidenceEnvelope
}

// AssembleAQ5Pack builds the complete AQ-5 evidence pack and makes
// the release decision based on proof reports, evidence, and gates.
func AssembleAQ5Pack(input AQ5PackInput) AQ5Pack {
	now := time.Now().UTC()

	gates := runAQ5Gates(input)

	decision, reason := makeDecision(input.ProofReports, input.EvidenceItems, gates)

	pack := AQ5Pack{
		SchemaVersion:  "0.1.0",
		PackID:         fmt.Sprintf("aq5-%s-%s", input.DocumentID, now.Format("20060102")),
		GeneratedAt:    now.Format(time.RFC3339),
		DocumentID:     input.DocumentID,
		Profile:        input.Profile,
		ProofReports:   input.ProofReports,
		EvidenceItems:  input.EvidenceItems,
		GateResults:    gates,
		Decision:       decision,
		DecisionReason: reason,
	}

	pack.PackHash = computePackHash(pack)
	return pack
}

// VerifyAQ5Pack checks that the pack hash matches its contents.
func VerifyAQ5Pack(pack AQ5Pack) bool {
	stored := pack.PackHash
	pack.PackHash = ""
	return stored == computePackHash(pack)
}

// WriteAQ5PackJSON serializes the pack.
func WriteAQ5PackJSON(w io.Writer, pack AQ5Pack) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pack)
}

func runAQ5Gates(input AQ5PackInput) []AQ5GateResult {
	var gates []AQ5GateResult

	// Gate 1: AQ-3 proof present and passing
	aq3 := findProof(input.ProofReports, "AQ-3")
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-AQ3", Name: "AQ-3 proof report",
		Pass: aq3 != nil && aq3.Pass, Blocking: true,
		Detail: proofDetail(aq3, "AQ-3"),
	})

	// Gate 2: AQ-4 proof present and passing
	aq4 := findProof(input.ProofReports, "AQ-4")
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-AQ4", Name: "AQ-4 proof report",
		Pass: aq4 != nil && aq4.Pass, Blocking: true,
		Detail: proofDetail(aq4, "AQ-4"),
	})

	// Gate 3: All proof reports have hashes
	allHashed := true
	for _, p := range input.ProofReports {
		if p.Hash == "" {
			allHashed = false
			break
		}
	}
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-HASHES", Name: "Proof report hashes",
		Pass: allHashed, Blocking: true,
		Detail: ternaryStr(allHashed, "all proof reports have hashes", "one or more proof reports missing hash"),
	})

	// Gate 4: Evidence manifest complete (no missing items)
	missingCount := 0
	for _, e := range input.EvidenceItems {
		if e.Status == "missing" {
			missingCount++
		}
	}
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-EVIDENCE", Name: "Evidence manifest complete",
		Pass: missingCount == 0, Blocking: true,
		Detail: ternaryStr(missingCount == 0,
			fmt.Sprintf("all %d evidence items present", len(input.EvidenceItems)),
			fmt.Sprintf("%d evidence items missing", missingCount)),
	})

	// Gate 5: Attestation present
	hasAttestation := input.Attestation != nil && input.Attestation.Status == EnvelopeValid
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-ATTEST", Name: "Attestation envelope",
		Pass: hasAttestation, Blocking: false,
		Detail: ternaryStr(hasAttestation, "valid attestation envelope present", "no valid attestation"),
	})

	// Gate 6: Minimum proof score
	minScore := minProofScore(input.ProofReports)
	gates = append(gates, AQ5GateResult{
		GateID: "AQ5-GATE-SCORE", Name: "Minimum proof score >= 0.8",
		Pass: len(input.ProofReports) > 0 && minScore >= 0.8, Blocking: false,
		Detail: fmt.Sprintf("minimum score: %.2f", minScore),
	})

	return gates
}

func makeDecision(proofs []AQ5ProofReport, evidence []AQ5EvidenceItem, gates []AQ5GateResult) (ReleaseDecision, string) {
	blockingFailed := 0
	nonBlockingFailed := 0
	var reasons []string

	for _, g := range gates {
		if !g.Pass {
			if g.Blocking {
				blockingFailed++
				reasons = append(reasons, fmt.Sprintf("[BLOCK] %s: %s", g.Name, g.Detail))
			} else {
				nonBlockingFailed++
				reasons = append(reasons, fmt.Sprintf("[WARN] %s: %s", g.Name, g.Detail))
			}
		}
	}

	if blockingFailed > 0 {
		return DecisionNoGo, fmt.Sprintf("no-go: %d blocking gate(s) failed — %s",
			blockingFailed, strings.Join(reasons, "; "))
	}

	if nonBlockingFailed > 0 {
		return DecisionHold, fmt.Sprintf("hold: all blocking gates pass but %d advisory gate(s) failed — %s",
			nonBlockingFailed, strings.Join(reasons, "; "))
	}

	return DecisionGo, "go: all gates pass, evidence complete, proofs verified"
}

func findProof(proofs []AQ5ProofReport, level string) *AQ5ProofReport {
	for i := range proofs {
		if proofs[i].AQLevel == level {
			return &proofs[i]
		}
	}
	return nil
}

func proofDetail(p *AQ5ProofReport, level string) string {
	if p == nil {
		return fmt.Sprintf("%s proof report not provided", level)
	}
	if p.Pass {
		return fmt.Sprintf("%s passed (score=%.2f): %s", level, p.Score, p.Summary)
	}
	return fmt.Sprintf("%s failed (score=%.2f): %s", level, p.Score, p.Summary)
}

func minProofScore(proofs []AQ5ProofReport) float64 {
	if len(proofs) == 0 {
		return 0
	}
	min := proofs[0].Score
	for _, p := range proofs[1:] {
		if p.Score < min {
			min = p.Score
		}
	}
	return min
}

func computePackHash(pack AQ5Pack) string {
	h := sha256.New()
	h.Write([]byte(pack.DocumentID))
	h.Write([]byte(pack.Profile))
	h.Write([]byte(string(pack.Decision)))
	for _, p := range pack.ProofReports {
		h.Write([]byte(p.ID + p.Hash))
	}
	for _, e := range pack.EvidenceItems {
		h.Write([]byte(e.ID + e.Hash + e.Status))
	}
	for _, g := range pack.GateResults {
		h.Write([]byte(g.GateID))
		if g.Pass {
			h.Write([]byte("1"))
		} else {
			h.Write([]byte("0"))
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func ternaryStr(cond bool, ifTrue, ifFalse string) string {
	if cond {
		return ifTrue
	}
	return ifFalse
}
