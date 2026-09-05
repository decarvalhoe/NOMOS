package compliance

// NRT-018 (#662) — the gate is blocked on the real repository for named
// reasons, opens only on a complete synthetic proof, refuses a record that
// claims more than the proof supports, and never records an activation.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const gateRecord = "../../../docs/regulated/qualification/praxis-activation-gate.yaml"

func TestPraxisGateIsBlockedOnTheRealRepositoryForNamedReasons(t *testing.T) {
	v, err := EvaluatePraxisActivation(gateRecord, "../../..", time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != PraxisGateStatusBlocked || v.UnmetCount == 0 || len(v.Reasons) != v.UnmetCount {
		t.Fatalf("verdict: %+v", v)
	}
	joined := strings.Join(v.Reasons, "\n")
	for _, must := range []string{"review:independent_nomosside_review", "artifact:production_journey_evidence", "artifact:validation_inventory", "aq_status", "cannot be generated"} {
		if !strings.Contains(joined, must) {
			t.Errorf("reasons must name %q:\n%s", must, joined)
		}
	}
	if !strings.Contains(v.ClaimBoundary, "is not an activation") {
		t.Fatalf("claim boundary: %q", v.ClaimBoundary)
	}
}

// completeProof writes a synthetic repository where every requirement is met.
func completeProof(t *testing.T) (root, record string) {
	t.Helper()
	root = t.TempDir()
	w := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	accepted := "schema_version: \"0.1.0\"\nacceptance:\n  status: accepted\n  accepted_by: quality-owner (fixture)\n  accepted_at: 2026-09-05\n"
	w("docs/regulated/qualification/iq.yaml", accepted)
	w("docs/regulated/qualification/oq.yaml", accepted)
	w("docs/regulated/qualification/pq.yaml", "overall_result: PASSED\n")
	w("docs/regulated/validation-pack/validation-inventory.yaml", "validations:\n  - id: VAL-1\n    risk_level: high\n    last_verified: 2026-09-05\n  - id: VAL-2\n    risk_level: critical\n    waiver: WAIV-1\n  - id: VAL-3\n    risk_level: low\n    last_verified: \"\"\n")
	w("docs/regulated/customer-integration/praxis-atom-mapping.md", "# Mapping\n\nStatus: verified\n")
	for _, r := range []string{"independent_nomosside_review", "praxis_boundary_review", "release_approver_signoff"} {
		w("docs/regulated/qualification/reviews/"+r+".yaml", "status: completed\nreviewed_by: someone (fixture)\nreviewed_at: 2026-09-05\n")
	}
	for name, rt := range map[string]string{"aq": "acceptance_qualification_decision", "recon": "reconstruction_verdict", "strict": "qualified_corpus_strict_gate_verdict"} {
		status := "accepted"
		if rt != "acceptance_qualification_decision" {
			status = "pass"
		}
		w("docs/regulated/qualification/decisions/"+name+".yaml", "record_type: "+rt+"\nstatus: "+status+"\ndecided_by: board (fixture)\ndecided_at: 2026-09-05\n")
	}
	record = filepath.Join(root, "gate.yaml")
	os.WriteFile(record, []byte(`schema_version: "0.2.0"
record_type: praxis_activation_gate
activation_id: FIXTURE-ACTIVATION
current_status: activatable_pending_human_decision
claim_boundary: fixture
nomos_required_proof:
  required_aq_status: accepted
  required_reconstruction_verdict: pass
  required_strict_gate_status: pass
  required_artifacts:
    - {path: docs/regulated/qualification/iq.yaml, role: installation_baseline, required_state: accepted}
    - {path: docs/regulated/qualification/oq.yaml, role: operational_gate_protocol, required_state: accepted}
    - {path: docs/regulated/qualification/pq.yaml, role: production_journey_evidence, required_state: passed}
    - {path: docs/regulated/validation-pack/validation-inventory.yaml, role: validation_inventory, required_state: no_open_high_or_critical_unwaived_gap}
    - {path: docs/regulated/customer-integration/praxis-atom-mapping.md, role: evidence_contract, required_state: verified}
  required_reviews: [independent_nomosside_review, praxis_boundary_review, release_approver_signoff]
consumer_guard:
  praxis_may_consume_unverified_nomos_atoms_as_regulated_evidence: false
`), 0o644)
	return root, record
}

func TestPraxisGateOpensOnlyOnCompleteProof(t *testing.T) {
	root, record := completeProof(t)
	v, err := EvaluatePraxisActivation(record, root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != PraxisGateStatusActivatable || v.UnmetCount != 0 || len(v.Checks) != 11 {
		t.Fatalf("complete proof must be activatable: %+v", v)
	}
	// Drop one human review → blocked.
	os.Remove(filepath.Join(root, "docs/regulated/qualification/reviews/release_approver_signoff.yaml"))
	os.WriteFile(record, []byte(strings.Replace(readFile(t, record), "current_status: activatable_pending_human_decision", "current_status: blocked_missing_review", 1)), 0o644)
	v, err = EvaluatePraxisActivation(record, root, time.Now())
	if err != nil || v.Status != PraxisGateStatusBlocked || v.UnmetCount != 1 {
		t.Fatalf("missing review must block: %v %+v", err, v)
	}
	// Unsigned review → still blocked, named as unsigned.
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/reviews/release_approver_signoff.yaml"), []byte("status: completed\n"), 0o644)
	v, _ = EvaluatePraxisActivation(record, root, time.Now())
	if v.Status != PraxisGateStatusBlocked || !strings.Contains(strings.Join(v.Reasons, " "), "completed_unsigned") {
		t.Fatalf("unsigned review must block: %+v", v.Reasons)
	}
	// Acceptance block without accepted_by/accepted_at → blocked, named unsigned.
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/iq.yaml"), []byte("acceptance:\n  status: accepted\n"), 0o644)
	v, _ = EvaluatePraxisActivation(record, root, time.Now())
	if v.Status != PraxisGateStatusBlocked || !strings.Contains(strings.Join(v.Reasons, " "), "accepted_unsigned") {
		t.Fatalf("unsigned acceptance must block: %+v", v.Reasons)
	}
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/iq.yaml"), []byte("acceptance:\n  status: accepted\n  accepted_by: q\n  accepted_at: 2026-09-05\n"), 0o644)
	// Unsigned decision record → blocked.
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/reviews/release_approver_signoff.yaml"), []byte("status: completed\nreviewed_by: x\nreviewed_at: 2026-09-05\n"), 0o644)
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/decisions/aq.yaml"), []byte("record_type: acceptance_qualification_decision\nstatus: accepted\n"), 0o644)
	v, _ = EvaluatePraxisActivation(record, root, time.Now())
	if v.Status != PraxisGateStatusBlocked || !strings.Contains(strings.Join(v.Reasons, " "), "accepted_unsigned") {
		t.Fatalf("unsigned decision must block: %+v", v.Reasons)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPraxisGateRefusesForgedRecords(t *testing.T) {
	root, record := completeProof(t)
	// Remove a review, but keep a record that claims to be ready → forged.
	os.Remove(filepath.Join(root, "docs/regulated/qualification/reviews/praxis_boundary_review.yaml"))
	_, err := EvaluatePraxisActivation(record, root, time.Now())
	wantPraxis(t, err, CodePraxisGateForged, "claims more than the proof supports")
	// A record stating "activated" is never a gate state.
	os.WriteFile(record, []byte(strings.Replace(readFile(t, record), "current_status: activatable_pending_human_decision", "current_status: activated", 1)), 0o644)
	os.WriteFile(filepath.Join(root, "docs/regulated/qualification/reviews/praxis_boundary_review.yaml"), []byte("status: completed\nreviewed_by: x\nreviewed_at: 2026-09-05\n"), 0o644)
	_, err = EvaluatePraxisActivation(record, root, time.Now())
	wantPraxis(t, err, CodePraxisGateForged, "human decision")
	// consumer_guard flipped → refused at load.
	os.WriteFile(record, []byte(strings.Replace(strings.Replace(readFile(t, record), "current_status: activated", "current_status: activatable_pending_human_decision", 1), "regulated_evidence: false", "regulated_evidence: true", 1)), 0o644)
	_, err = EvaluatePraxisActivation(record, root, time.Now())
	wantPraxis(t, err, CodePraxisGateForged, "consumer_guard")
	// Real record with a wrong type.
	p := filepath.Join(root, "bad.yaml")
	os.WriteFile(p, []byte("record_type: something_else\nactivation_id: x\ncurrent_status: blocked\n"), 0o644)
	_, err = EvaluatePraxisActivation(p, root, time.Now())
	wantPraxis(t, err, CodePraxisGateRecord, "record_type")
}

func TestPraxisVerdictVerification(t *testing.T) {
	root, record := completeProof(t)
	v, err := EvaluatePraxisActivation(record, root, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	v.RecordPath = "gate.yaml"
	write := func(m map[string]any) string {
		raw, _ := json.Marshal(m)
		p := filepath.Join(root, "verdict.json")
		os.WriteFile(p, raw, 0o644)
		return p
	}
	asMap := func() map[string]any {
		raw, _ := json.Marshal(v)
		var m map[string]any
		json.Unmarshal(raw, &m)
		return m
	}
	if _, err := VerifyPraxisActivationVerdict(write(asMap()), root); err != nil {
		t.Fatalf("honest verdict must verify: %v", err)
	}
	m := asMap()
	m["status"] = "activated"
	_, err = VerifyPraxisActivationVerdict(write(m), root)
	wantPraxis(t, err, CodePraxisGateVerdict, "never records an activation")
	m = asMap()
	m["status"] = "activatable"
	m["unmet_count"] = float64(2)
	_, err = VerifyPraxisActivationVerdict(write(m), root)
	wantPraxis(t, err, CodePraxisGateVerdict, "contradiction")
	m = asMap()
	m["checks"].([]any)[0].(map[string]any)["met"] = false
	_, err = VerifyPraxisActivationVerdict(write(m), root)
	wantPraxis(t, err, CodePraxisGateVerdict, "checks say")
	// Record edited after the verdict → the verdict no longer binds it.
	os.WriteFile(record, []byte(readFile(t, record)+"\n# edited\n"), 0o644)
	_, err = VerifyPraxisActivationVerdict(write(asMap()), root)
	wantPraxis(t, err, CodePraxisGateVerdict, "record_sha256")
}

func TestExchangeRegulatedRelianceNeedsAnActivatableVerdict(t *testing.T) {
	root, record := completeProof(t)
	v, _ := EvaluatePraxisActivation(record, root, time.Now())
	v.RecordPath = "gate.yaml"
	verdictRaw, _ := json.Marshal(v)
	os.WriteFile(filepath.Join(root, "verdict.json"), verdictRaw, 0o644)
	// One verified artifact with a matching record.
	os.WriteFile(filepath.Join(root, "artifact.yaml"), []byte("a: 1\n"), 0o644)
	os.WriteFile(filepath.Join(root, "record.json"), []byte("{}"), 0o644)
	ex := PraxisEvidenceExchange{
		SchemaVersion: PraxisExchangeSchema, ExchangeID: "X", GeneratedAt: "2026-09-05T00:00:00Z",
		Producer: PraxisProduct{"nomos", "1"}, Consumer: PraxisProduct{"praxis", "1"},
		NomosArtifacts: []NomosArtifactRef{{ArtifactID: "A", Kind: "control_matrix", Path: "artifact.yaml", Sha256: "sha256:" + sha256Hex([]byte("a: 1\n"))[7:],
			Verification: PraxisVerification{State: "verified", RecordPath: "record.json", RecordSha256: "sha256:" + sha256Hex([]byte("{}"))[7:]}}},
		Reliance: "regulated_evidence", ActivationVerdictPath: "verdict.json", ActivationVerdictSha256: "sha256:" + sha256Hex(verdictRaw)[7:],
		ClaimBoundary: "fixture exchange bound to an activatable verdict on a synthetic complete proof",
	}
	rep, err := VerifyPraxisExchange(ex, root)
	if err != nil {
		t.Fatalf("regulated reliance on an activatable verdict must verify: %v", err)
	}
	if !hasCheck(rep.Checks, "activation_verdict") {
		t.Fatalf("checks: %v", rep.Checks)
	}
	// Same exchange bound to a BLOCKED verdict (hash updated honestly) → refused.
	v2 := v
	v2.Status = PraxisGateStatusBlocked
	v2.UnmetCount = 1
	v2.Checks = append([]PraxisGateCheck{}, v.Checks...)
	v2.Checks[0].Met = false
	blockedRaw, _ := json.Marshal(v2)
	os.WriteFile(filepath.Join(root, "verdict.json"), blockedRaw, 0o644)
	ex.ActivationVerdictSha256 = "sha256:" + sha256Hex(blockedRaw)[7:]
	_, err = VerifyPraxisExchange(ex, root)
	wantPraxis(t, err, CodePraxisRelianceUnsupported, "needs activatable")
}

func hasCheck(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
