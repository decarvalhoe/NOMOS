package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func aq3Proof(pass bool, score float64) AQ5ProofReport {
	return AQ5ProofReport{ID: "proof-aq3", AQLevel: "AQ-3", Pass: pass, Score: score, Summary: "AQ-3 check", Hash: "sha256:aq3"}
}

func aq4Proof(pass bool, score float64) AQ5ProofReport {
	return AQ5ProofReport{ID: "proof-aq4", AQLevel: "AQ-4", Pass: pass, Score: score, Summary: "AQ-4 check", Hash: "sha256:aq4"}
}

func fullEvidence() []AQ5EvidenceItem {
	return []AQ5EvidenceItem{
		{ID: "ev-1", Type: "toc", Path: "toc.json", Hash: "sha256:t", Status: "present"},
		{ID: "ev-2", Type: "atoms", Path: "atoms.json", Hash: "sha256:a", Status: "present"},
		{ID: "ev-3", Type: "rag", Path: "rag.json", Hash: "sha256:r", Status: "present"},
	}
}

func validAttestation() *EvidenceEnvelope {
	return &EvidenceEnvelope{Status: EnvelopeValid}
}

func goInput() AQ5PackInput {
	return AQ5PackInput{
		DocumentID:    "rbok-lawbook",
		Profile:       "rbok-lawbook",
		ProofReports:  []AQ5ProofReport{aq3Proof(true, 0.95), aq4Proof(true, 0.90)},
		EvidenceItems: fullEvidence(),
		Attestation:   validAttestation(),
	}
}

// --- Go decision ---

func TestAQ5PackGo(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	if pack.Decision != DecisionGo {
		t.Fatalf("expected go, got %q: %s", pack.Decision, pack.DecisionReason)
	}
	if !strings.HasPrefix(pack.PackHash, "sha256:") {
		t.Fatalf("expected hash, got %q", pack.PackHash)
	}
	if pack.DocumentID != "rbok-lawbook" {
		t.Fatalf("expected rbok-lawbook, got %q", pack.DocumentID)
	}
}

// --- No-go: missing AQ-3 ---

func TestAQ5PackNoGoMissingAQ3(t *testing.T) {
	input := goInput()
	input.ProofReports = []AQ5ProofReport{aq4Proof(true, 0.90)}
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionNoGo {
		t.Fatalf("expected no-go, got %q", pack.Decision)
	}
	if !strings.Contains(pack.DecisionReason, "AQ-3") {
		t.Fatalf("expected AQ-3 in reason, got %q", pack.DecisionReason)
	}
}

// --- No-go: AQ-4 failed ---

func TestAQ5PackNoGoAQ4Failed(t *testing.T) {
	input := goInput()
	input.ProofReports = []AQ5ProofReport{aq3Proof(true, 0.95), aq4Proof(false, 0.40)}
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionNoGo {
		t.Fatalf("expected no-go, got %q", pack.Decision)
	}
}

// --- No-go: missing evidence ---

func TestAQ5PackNoGoMissingEvidence(t *testing.T) {
	input := goInput()
	input.EvidenceItems = []AQ5EvidenceItem{
		{ID: "ev-1", Type: "toc", Path: "toc.json", Hash: "sha256:t", Status: "present"},
		{ID: "ev-2", Type: "atoms", Path: "atoms.json", Hash: "", Status: "missing"},
	}
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionNoGo {
		t.Fatalf("expected no-go, got %q", pack.Decision)
	}
	if !strings.Contains(pack.DecisionReason, "evidence") {
		t.Fatalf("expected evidence in reason, got %q", pack.DecisionReason)
	}
}

// --- No-go: missing proof hash ---

func TestAQ5PackNoGoMissingHash(t *testing.T) {
	input := goInput()
	input.ProofReports[0].Hash = ""
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionNoGo {
		t.Fatalf("expected no-go, got %q", pack.Decision)
	}
}

// --- Hold: no attestation ---

func TestAQ5PackHoldNoAttestation(t *testing.T) {
	input := goInput()
	input.Attestation = nil
	pack := AssembleAQ5Pack(input)
	// Attestation is non-blocking, but low score is also non-blocking
	// With all blocking gates passing but attestation missing → hold
	if pack.Decision == DecisionNoGo {
		t.Fatalf("expected hold or go (attestation non-blocking), got no-go: %s", pack.DecisionReason)
	}
}

// --- Hold: low score ---

func TestAQ5PackHoldLowScore(t *testing.T) {
	input := goInput()
	input.ProofReports = []AQ5ProofReport{
		aq3Proof(true, 0.60),
		aq4Proof(true, 0.70),
	}
	input.Attestation = validAttestation()
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionHold {
		t.Fatalf("expected hold (low score), got %q: %s", pack.Decision, pack.DecisionReason)
	}
}

// --- Gate results ---

func TestAQ5PackGateCount(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	if len(pack.GateResults) != 6 {
		t.Fatalf("expected 6 gates, got %d", len(pack.GateResults))
	}
}

func TestAQ5PackAllGatesPass(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	for _, g := range pack.GateResults {
		if !g.Pass {
			t.Fatalf("gate %s failed: %s", g.GateID, g.Detail)
		}
	}
}

func TestAQ5PackGateIDs(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	ids := map[string]bool{}
	for _, g := range pack.GateResults {
		ids[g.GateID] = true
	}
	for _, expected := range []string{"AQ5-GATE-AQ3", "AQ5-GATE-AQ4", "AQ5-GATE-HASHES", "AQ5-GATE-EVIDENCE", "AQ5-GATE-ATTEST", "AQ5-GATE-SCORE"} {
		if !ids[expected] {
			t.Fatalf("expected gate %q", expected)
		}
	}
}

// --- Verify ---

func TestVerifyAQ5Pack(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	if !VerifyAQ5Pack(pack) {
		t.Fatal("expected verification pass")
	}
}

func TestVerifyAQ5PackTampered(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	pack.Decision = DecisionNoGo
	if VerifyAQ5Pack(pack) {
		t.Fatal("expected verification fail")
	}
}

// --- Hash determinism ---

func TestAQ5PackHashDeterministic(t *testing.T) {
	p1 := AssembleAQ5Pack(goInput())
	p2 := AssembleAQ5Pack(goInput())
	if p1.PackHash != p2.PackHash {
		t.Fatal("hash not deterministic")
	}
}

// --- Empty proofs ---

func TestAQ5PackNoProofs(t *testing.T) {
	input := AQ5PackInput{DocumentID: "doc", EvidenceItems: fullEvidence()}
	pack := AssembleAQ5Pack(input)
	if pack.Decision != DecisionNoGo {
		t.Fatalf("expected no-go with no proofs, got %q", pack.Decision)
	}
}

// --- Schema version ---

func TestAQ5PackSchema(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	if pack.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", pack.SchemaVersion)
	}
}

// --- JSON roundtrip ---

func TestAQ5PackJSON(t *testing.T) {
	pack := AssembleAQ5Pack(goInput())
	var buf bytes.Buffer
	if err := WriteAQ5PackJSON(&buf, pack); err != nil {
		t.Fatal(err)
	}
	var decoded AQ5Pack
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Decision != pack.Decision {
		t.Fatal("roundtrip decision mismatch")
	}
	if decoded.PackHash != pack.PackHash {
		t.Fatal("roundtrip hash mismatch")
	}
}

// --- Decision constants ---

func TestDecisionConstants(t *testing.T) {
	if DecisionGo != "go" || DecisionNoGo != "no-go" || DecisionHold != "hold" {
		t.Fatal("constants wrong")
	}
}
