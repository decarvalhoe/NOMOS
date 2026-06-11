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

// --- CKM Claim Boundary ---

func testClaimBoundaryPredicate() ClaimBoundaryPredicate {
	return ClaimBoundaryPredicate{
		ProjectID:   "ckm-test",
		GeneratedAt: testTime,
		RefusedClaims: []RefusedClaim{
			{
				ClaimID:          "claim.no-trace-for-y",
				Statement:        "Cannot prove traceability for Y, so Nomos refuses to assert it.",
				Reason:           "No source-backed atom or body-ledger segment supports Y.",
				RequiredEvidence: []string{"source_span", "atom_id", "body_ledger_merkle_proof"},
				Decision:         "refused",
			},
		},
		Verifier:      "nomos",
		SignatureMode: "dsse-cosign",
		Signature: ClaimBoundarySignature{
			KeyID:     "nomos-test-key",
			Signature: "MEUCIQDfixture-signature",
			SignedAt:  testTime,
			LogURI:    "rekor://fixture-entry",
		},
		ClaimBoundary: "Refusal predicate only; no correctness or regulatory compliance claim.",
	}
}

func TestGenerateClaimBoundaryStatementRecordsRefusedClaims(t *testing.T) {
	predicate := testClaimBoundaryPredicate()
	stmt, err := GenerateClaimBoundaryStatement(predicate, testSubjects())
	if err != nil {
		t.Fatalf("GenerateClaimBoundaryStatement failed: %v", err)
	}
	if stmt.PredicateType != ClaimBoundaryPredicateType {
		t.Fatalf("expected predicate type %s, got %s", ClaimBoundaryPredicateType, stmt.PredicateType)
	}

	var decoded ClaimBoundaryPredicate
	if err := json.Unmarshal(stmt.Predicate, &decoded); err != nil {
		t.Fatalf("decode predicate: %v", err)
	}
	if len(decoded.RefusedClaims) != 1 {
		t.Fatalf("expected one refused claim, got %d", len(decoded.RefusedClaims))
	}
	if decoded.RefusedClaims[0].Reason == "" {
		t.Fatal("expected refusal reason")
	}
	if decoded.Signature.Signature == "" {
		t.Fatal("expected signature metadata")
	}
}

func TestVerifyClaimBoundaryStatementRejectsMissingRefusalReason(t *testing.T) {
	predicate := testClaimBoundaryPredicate()
	predicate.RefusedClaims[0].Reason = ""
	stmt, err := GenerateClaimBoundaryStatement(predicate, testSubjects())
	if err != nil {
		t.Fatalf("GenerateClaimBoundaryStatement setup failed: %v", err)
	}
	if err := VerifyClaimBoundaryStatement(stmt); err == nil {
		t.Fatal("expected missing refusal reason to fail verification")
	}
}

func TestVerifyClaimBoundaryStatementRejectsSignedModeWithoutSignature(t *testing.T) {
	predicate := testClaimBoundaryPredicate()
	predicate.Signature.Signature = ""
	stmt, err := GenerateClaimBoundaryStatement(predicate, testSubjects())
	if err != nil {
		t.Fatalf("GenerateClaimBoundaryStatement setup failed: %v", err)
	}
	if err := VerifyClaimBoundaryStatement(stmt); err == nil {
		t.Fatal("expected signed mode without signature to fail verification")
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

// --- Integration: full pipeline (real ECDSA P-256 DSSE signing) ---

// CKM-H1-FU (#537): the legacy WrapCosignEnvelope/VerifyCosignEnvelope path was
// removed. The full pipeline now signs the statement for real and verifies it.
func TestFullPipeline_StatementToSignedEnvelope(t *testing.T) {
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

	signer, err := GenerateSigner()
	if err != nil {
		t.Fatalf("GenerateSigner: %v", err)
	}
	env, err := signer.SignStatement(stmt)
	if err != nil {
		t.Fatalf("SignStatement: %v", err)
	}
	if env.Signatures[0].Sig == "" {
		t.Fatal("envelope carries an empty signature — nothing was signed")
	}

	pub := &signer.priv.PublicKey
	payload, err := VerifyEnvelopePayload(env, pub)
	if err != nil {
		t.Fatalf("verification of a freshly signed envelope failed: %v", err)
	}

	var roundTripped InTotoStatement
	if err := json.Unmarshal(payload, &roundTripped); err != nil {
		t.Fatalf("failed to round-trip statement from verified payload: %v", err)
	}
	if roundTripped.Type != InTotoStatementType {
		t.Fatalf("round-trip: wrong type %s", roundTripped.Type)
	}
	if roundTripped.PredicateType != NomosPredicateType {
		t.Fatalf("round-trip: wrong predicate type %s", roundTripped.PredicateType)
	}
}

func TestFullPipeline_ProvenanceToSignedEnvelope(t *testing.T) {
	prov := testProvenance()
	subjects := testSubjects()

	stmt, err := GenerateProvenance(prov, subjects)
	if err != nil {
		t.Fatalf("GenerateProvenance: %v", err)
	}
	if err := VerifyProvenance(stmt); err != nil {
		t.Fatalf("VerifyProvenance: %v", err)
	}

	signer, _ := GenerateSigner()
	env, err := signer.SignStatement(stmt)
	if err != nil {
		t.Fatalf("SignStatement: %v", err)
	}
	if err := VerifyEnvelope(env, &signer.priv.PublicKey); err != nil {
		t.Fatalf("verification of a freshly signed provenance envelope failed: %v", err)
	}
}

// Adversarial proof (doctrine §2.3): an envelope with an EMPTY signature (the
// exact Sig:"" the deleted WrapCosignEnvelope produced) must be REJECTED. The old
// VerifyCosignEnvelope only checked field-presence and would have accepted this.
func TestVerifyEnvelope_RejectsEmptySignature(t *testing.T) {
	signer, _ := GenerateSigner()
	env, err := signer.SignStatement(testStatement(t))
	if err != nil {
		t.Fatalf("SignStatement: %v", err)
	}
	pub := &signer.priv.PublicKey

	// Sanity: the real signature verifies.
	if err := VerifyEnvelope(env, pub); err != nil {
		t.Fatalf("pre-tamper verification failed: %v", err)
	}

	// Now blank the signature, exactly like the legacy fake path did.
	env.Signatures[0].Sig = ""
	if err := VerifyEnvelope(env, pub); err == nil {
		t.Fatal("verification PASSED on an empty (Sig:\"\") signature — the fake path is back")
	}

	// And an envelope whose only signature is empty must also be rejected.
	emptyOnly := DSSEEnvelope{
		PayloadType: env.PayloadType,
		Payload:     env.Payload,
		Signatures:  []DSSESignature{{KeyID: signer.KeyID(), Sig: ""}},
	}
	if err := VerifyEnvelope(emptyOnly, pub); err == nil {
		t.Fatal("verification PASSED on an envelope whose only signature is empty")
	}
}

// --- CKM supply-chain predicate ---

func TestGenerateSupplyChainStatementRecordsPipelineStages(t *testing.T) {
	pred := SupplyChainPredicate{
		ProjectID: "ckm-project",
		CorpusID:  "ckm-corpus",
		Signature: SupplyChainSignature{
			Mode:   SignatureModeUnsigned,
			Status: SignatureStatusUnsigned,
		},
		Steps: []SupplyChainStep{
			{Name: StepIngestion, Materials: []Subject{SubjectFromBytes("source.md", []byte("source"))}, Products: []Subject{SubjectFromBytes("snapshot.json", []byte("snapshot"))}},
			{Name: StepCanon, Materials: []Subject{SubjectFromBytes("snapshot.json", []byte("snapshot"))}, Products: []Subject{SubjectFromBytes("feed.json", []byte("feed"))}},
			{Name: StepEmbedding, Materials: []Subject{SubjectFromBytes("feed.json", []byte("feed"))}, Products: []Subject{SubjectFromBytes("rag.json", []byte("rag"))}},
		},
	}

	stmt, err := GenerateSupplyChainStatement(pred)
	if err != nil {
		t.Fatalf("GenerateSupplyChainStatement: %v", err)
	}
	if stmt.Type != InTotoStatementType {
		t.Fatalf("expected in-toto type, got %s", stmt.Type)
	}
	if stmt.PredicateType != SupplyChainPredicateType {
		t.Fatalf("expected supply-chain predicate, got %s", stmt.PredicateType)
	}
	if len(stmt.Subject) != 4 {
		t.Fatalf("expected deduplicated subjects for all products/materials, got %d", len(stmt.Subject))
	}

	var decoded SupplyChainPredicate
	if err := json.Unmarshal(stmt.Predicate, &decoded); err != nil {
		t.Fatalf("decode supply-chain predicate: %v", err)
	}
	if decoded.Signature.Status != SignatureStatusUnsigned {
		t.Fatalf("unsigned mode must be explicit, got %q", decoded.Signature.Status)
	}
	if decoded.Signature.TrustTier != "unverified" {
		t.Fatalf("unsigned mode must be lower trust, got %q", decoded.Signature.TrustTier)
	}
	for _, name := range []SupplyChainStepName{StepIngestion, StepCanon, StepEmbedding} {
		if !decoded.HasStep(name) {
			t.Fatalf("missing step %s", name)
		}
	}
}

func TestVerifySupplyChainStatementFailsWhenArtifactHashChanges(t *testing.T) {
	pred := SupplyChainPredicate{
		ProjectID: "ckm-project",
		CorpusID:  "ckm-corpus",
		Signature: SupplyChainSignature{
			Mode:      SignatureModeSigstoreKeyless,
			Status:    SignatureStatusSigned,
			RekorUUID: "rekor-entry-1",
		},
		Steps: []SupplyChainStep{
			{Name: StepCanon, Products: []Subject{SubjectFromBytes("feed.json", []byte("feed-v1"))}},
		},
	}
	stmt, err := GenerateSupplyChainStatement(pred)
	if err != nil {
		t.Fatalf("GenerateSupplyChainStatement: %v", err)
	}

	if err := VerifySupplyChainStatement(stmt, map[string][]byte{"feed.json": []byte("feed-v1")}); err != nil {
		t.Fatalf("expected matching artifact to verify: %v", err)
	}
	if err := VerifySupplyChainStatement(stmt, map[string][]byte{"feed.json": []byte("feed-v2")}); err == nil {
		t.Fatal("expected changed artifact hash to fail verification")
	}
}

func TestVerifySupplyChainStatementRejectsSignedClaimWithoutRekorEntry(t *testing.T) {
	pred := SupplyChainPredicate{
		ProjectID: "ckm-project",
		CorpusID:  "ckm-corpus",
		Signature: SupplyChainSignature{
			Mode:   SignatureModeSigstoreKeyless,
			Status: SignatureStatusSigned,
		},
		Steps: []SupplyChainStep{
			{Name: StepCanon, Products: []Subject{SubjectFromBytes("feed.json", []byte("feed-v1"))}},
		},
	}
	stmt, err := GenerateSupplyChainStatement(pred)
	if err != nil {
		t.Fatalf("GenerateSupplyChainStatement: %v", err)
	}

	if err := VerifySupplyChainStatement(stmt, map[string][]byte{"feed.json": []byte("feed-v1")}); err == nil {
		t.Fatal("expected signed claim without Rekor UUID to fail verification")
	}
}
