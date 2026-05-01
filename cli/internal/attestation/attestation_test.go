package attestation

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var testTime = time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

func testSubjects() []Subject {
	return []Subject{
		SubjectFromBytes("nomos-report.json", []byte(`{"verdict":"admitted"}`)),
	}
}

// --- In-toto Statement ---

func TestGenerateStatement(t *testing.T) {
	att := NomosAttestation{
		ProjectID:   "my-project",
		Verdict:     "admitted",
		Confidence:  "high",
		Timestamp:   testTime,
		Evidence:    []string{"nomos-report.json"},
		AttesterID:  "ci-pipeline",
		AttestLevel: "signed",
	}

	stmt, err := GenerateStatement(att, testSubjects())
	if err != nil {
		t.Fatalf("GenerateStatement failed: %v", err)
	}
	if stmt.Type != InTotoStatementType {
		t.Fatalf("expected type %s, got %s", InTotoStatementType, stmt.Type)
	}
	if stmt.PredicateType != NomosPredicateType {
		t.Fatalf("expected predicate type %s, got %s", NomosPredicateType, stmt.PredicateType)
	}
	if len(stmt.Subject) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(stmt.Subject))
	}

	var decoded NomosAttestation
	if err := json.Unmarshal(stmt.Predicate, &decoded); err != nil {
		t.Fatalf("failed to decode predicate: %v", err)
	}
	if decoded.ProjectID != "my-project" {
		t.Fatalf("expected projectId my-project, got %s", decoded.ProjectID)
	}
	if decoded.Verdict != "admitted" {
		t.Fatalf("expected verdict admitted, got %s", decoded.Verdict)
	}
}

func TestGenerateStatement_NoSubjects(t *testing.T) {
	att := NomosAttestation{ProjectID: "p", Verdict: "admitted"}
	_, err := GenerateStatement(att, nil)
	if err == nil {
		t.Fatal("expected error for empty subjects")
	}
}

func TestGenerateStatement_NoProjectID(t *testing.T) {
	att := NomosAttestation{Verdict: "admitted"}
	_, err := GenerateStatement(att, testSubjects())
	if err == nil {
		t.Fatal("expected error for empty projectId")
	}
}

func TestGenerateStatement_NoVerdict(t *testing.T) {
	att := NomosAttestation{ProjectID: "p"}
	_, err := GenerateStatement(att, testSubjects())
	if err == nil {
		t.Fatal("expected error for empty verdict")
	}
}

// --- SLSA Provenance ---

func testProvenance() SLSAProvenance {
	return SLSAProvenance{
		BuildDefinition: SLSABuildDefinition{
			BuildType:          "https://nomos.dev/build/v1",
			ExternalParameters: map[string]string{"repository": "https://github.com/example/repo"},
		},
		RunDetails: SLSARunDetails{
			Builder:  SLSABuilder{ID: "https://github.com/actions/runner"},
			Metadata: SLSAMetadata{InvocationID: "run-123", StartedOn: testTime, FinishedOn: testTime.Add(5 * time.Minute)},
		},
	}
}

func TestGenerateProvenance(t *testing.T) {
	stmt, err := GenerateProvenance(testProvenance(), testSubjects())
	if err != nil {
		t.Fatalf("GenerateProvenance failed: %v", err)
	}
	if stmt.PredicateType != SLSAPredicateType {
		t.Fatalf("expected predicate type %s, got %s", SLSAPredicateType, stmt.PredicateType)
	}

	var prov SLSAProvenance
	if err := json.Unmarshal(stmt.Predicate, &prov); err != nil {
		t.Fatalf("failed to decode provenance: %v", err)
	}
	if prov.RunDetails.Builder.ID != "https://github.com/actions/runner" {
		t.Fatalf("unexpected builder ID: %s", prov.RunDetails.Builder.ID)
	}
}

func TestGenerateProvenance_NoSubjects(t *testing.T) {
	_, err := GenerateProvenance(testProvenance(), nil)
	if err == nil {
		t.Fatal("expected error for empty subjects")
	}
}

func TestGenerateProvenance_NoBuilder(t *testing.T) {
	prov := testProvenance()
	prov.RunDetails.Builder.ID = ""
	_, err := GenerateProvenance(prov, testSubjects())
	if err == nil {
		t.Fatal("expected error for empty builder ID")
	}
}

func TestVerifyProvenance_Valid(t *testing.T) {
	stmt, err := GenerateProvenance(testProvenance(), testSubjects())
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := VerifyProvenance(stmt); err != nil {
		t.Fatalf("VerifyProvenance failed: %v", err)
	}
}

func TestVerifyProvenance_WrongType(t *testing.T) {
	stmt := InTotoStatement{Type: "wrong", PredicateType: SLSAPredicateType}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for wrong type")
	}
}

func TestVerifyProvenance_WrongPredicateType(t *testing.T) {
	stmt := InTotoStatement{Type: InTotoStatementType, PredicateType: "wrong"}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for wrong predicate type")
	}
}

func TestVerifyProvenance_NoSubjects(t *testing.T) {
	stmt := InTotoStatement{Type: InTotoStatementType, PredicateType: SLSAPredicateType}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for no subjects")
	}
}

func TestVerifyProvenance_EmptySubjectName(t *testing.T) {
	stmt := InTotoStatement{
		Type:          InTotoStatementType,
		PredicateType: SLSAPredicateType,
		Subject:       []Subject{{Name: "", Digest: map[string]string{"sha256": "abc"}}},
	}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for empty subject name")
	}
}

func TestVerifyProvenance_NoDigest(t *testing.T) {
	stmt := InTotoStatement{
		Type:          InTotoStatementType,
		PredicateType: SLSAPredicateType,
		Subject:       []Subject{{Name: "file.json"}},
	}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for no digest")
	}
}

func TestVerifyProvenance_MissingBuilderID(t *testing.T) {
	prov := testProvenance()
	prov.RunDetails.Builder.ID = ""
	predBytes, _ := json.Marshal(prov)
	stmt := InTotoStatement{
		Type:          InTotoStatementType,
		PredicateType: SLSAPredicateType,
		Subject:       testSubjects(),
		Predicate:     json.RawMessage(predBytes),
	}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for missing builder ID")
	}
}

func TestVerifyProvenance_MissingBuildType(t *testing.T) {
	prov := testProvenance()
	prov.BuildDefinition.BuildType = ""
	predBytes, _ := json.Marshal(prov)
	stmt := InTotoStatement{
		Type:          InTotoStatementType,
		PredicateType: SLSAPredicateType,
		Subject:       testSubjects(),
		Predicate:     json.RawMessage(predBytes),
	}
	if err := VerifyProvenance(stmt); err == nil {
		t.Fatal("expected error for missing build type")
	}
}

// --- Cosign Envelope ---

func TestWrapCosignEnvelope(t *testing.T) {
	payload := map[string]string{"verdict": "admitted"}
	env, err := WrapCosignEnvelope(payload, "test-key-001")
	if err != nil {
		t.Fatalf("WrapCosignEnvelope failed: %v", err)
	}
	if env.PayloadType != CosignPayloadType {
		t.Fatalf("expected payload type %s, got %s", CosignPayloadType, env.PayloadType)
	}
	if len(env.Signatures) != 1 {
		t.Fatalf("expected 1 signature slot, got %d", len(env.Signatures))
	}
	if env.Signatures[0].KeyID != "test-key-001" {
		t.Fatalf("expected keyid test-key-001, got %s", env.Signatures[0].KeyID)
	}
	if env.Signatures[0].Sig != "" {
		t.Fatal("expected empty sig for unsigned envelope")
	}
}

func TestVerifyCosignEnvelope_Valid(t *testing.T) {
	env, _ := WrapCosignEnvelope(map[string]string{"a": "b"}, "key-1")
	if err := VerifyCosignEnvelope(env); err != nil {
		t.Fatalf("VerifyCosignEnvelope failed: %v", err)
	}
}

func TestVerifyCosignEnvelope_EmptyPayloadType(t *testing.T) {
	env := CosignEnvelope{Payload: "x", Signatures: []CosignSig{{KeyID: "k"}}}
	if err := VerifyCosignEnvelope(env); err == nil {
		t.Fatal("expected error for empty payload type")
	}
}

func TestVerifyCosignEnvelope_EmptyPayload(t *testing.T) {
	env := CosignEnvelope{PayloadType: "x", Signatures: []CosignSig{{KeyID: "k"}}}
	if err := VerifyCosignEnvelope(env); err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestVerifyCosignEnvelope_NoSignatures(t *testing.T) {
	env := CosignEnvelope{PayloadType: "x", Payload: "y"}
	if err := VerifyCosignEnvelope(env); err == nil {
		t.Fatal("expected error for no signatures")
	}
}

func TestVerifyCosignEnvelope_EmptyKeyID(t *testing.T) {
	env := CosignEnvelope{PayloadType: "x", Payload: "y", Signatures: []CosignSig{{KeyID: ""}}}
	if err := VerifyCosignEnvelope(env); err == nil {
		t.Fatal("expected error for empty key ID")
	}
}

// --- Helpers ---

func TestDigestSHA256(t *testing.T) {
	digest := DigestSHA256([]byte("hello"))
	if len(digest) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(digest))
	}
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if digest != expected {
		t.Fatalf("expected %s, got %s", expected, digest)
	}
}

func TestSubjectFromBytes(t *testing.T) {
	subj := SubjectFromBytes("report.json", []byte("data"))
	if subj.Name != "report.json" {
		t.Fatalf("expected name report.json, got %s", subj.Name)
	}
	if _, ok := subj.Digest["sha256"]; !ok {
		t.Fatal("expected sha256 digest")
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	att := NomosAttestation{ProjectID: "test", Verdict: "admitted", Confidence: "high"}
	if err := WriteJSON(&buf, att); err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}
	var decoded NomosAttestation
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode output: %v", err)
	}
	if decoded.ProjectID != "test" {
		t.Fatalf("expected projectId test, got %s", decoded.ProjectID)
	}
}

// --- Integration: full pipeline ---

func TestFullPipeline_StatementToCosign(t *testing.T) {
	att := NomosAttestation{
		ProjectID:   "integration-test",
		Verdict:     "admitted",
		Confidence:  "high",
		Timestamp:   testTime,
		Evidence:    []string{"report.json"},
		AttesterID:  "ci",
		AttestLevel: "signed",
	}
	subjects := []Subject{SubjectFromBytes("report.json", []byte("content"))}

	stmt, err := GenerateStatement(att, subjects)
	if err != nil {
		t.Fatalf("GenerateStatement: %v", err)
	}

	env, err := WrapCosignEnvelope(stmt, "nomos-signing-key")
	if err != nil {
		t.Fatalf("WrapCosignEnvelope: %v", err)
	}
	if err := VerifyCosignEnvelope(env); err != nil {
		t.Fatalf("VerifyCosignEnvelope: %v", err)
	}

	var roundTripped InTotoStatement
	if err := json.Unmarshal([]byte(env.Payload), &roundTripped); err != nil {
		t.Fatalf("failed to round-trip statement from envelope: %v", err)
	}
	if roundTripped.Type != InTotoStatementType {
		t.Fatalf("round-trip: wrong type %s", roundTripped.Type)
	}
	if roundTripped.PredicateType != NomosPredicateType {
		t.Fatalf("round-trip: wrong predicate type %s", roundTripped.PredicateType)
	}
}

func TestFullPipeline_ProvenanceToCosign(t *testing.T) {
	prov := testProvenance()
	subjects := testSubjects()

	stmt, err := GenerateProvenance(prov, subjects)
	if err != nil {
		t.Fatalf("GenerateProvenance: %v", err)
	}
	if err := VerifyProvenance(stmt); err != nil {
		t.Fatalf("VerifyProvenance: %v", err)
	}

	env, err := WrapCosignEnvelope(stmt, "slsa-key")
	if err != nil {
		t.Fatalf("WrapCosignEnvelope: %v", err)
	}
	if err := VerifyCosignEnvelope(env); err != nil {
		t.Fatalf("VerifyCosignEnvelope: %v", err)
	}
}
