package corpus

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var attestNow = time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)

func TestGenerateCorpusAttestationBasic(t *testing.T) {
	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "insurance-sources",
		ProjectID:      "regulated-benefits",
		ScannerVersion: "0.1.0",
		Verdict:        "corpus_admissible",
		Confidence:     "high",
		FilesScanned:   42,
		UnitsExtracted: 10,
		ScannedFiles:   []string{"doc1.pdf", "doc2.yaml", "doc3.md"},
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stmt.Type != InTotoStatementType {
		t.Fatalf("expected in-toto type, got %s", stmt.Type)
	}
	if stmt.PredicateType != CorpusPredicateType {
		t.Fatalf("expected corpus predicate type, got %s", stmt.PredicateType)
	}
	if len(stmt.Subject) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(stmt.Subject))
	}
	if stmt.Subject[0].Name != "corpus:insurance-sources" {
		t.Fatalf("expected subject name corpus:insurance-sources, got %s", stmt.Subject[0].Name)
	}
	if stmt.Subject[0].Digest["sha256"] == "" {
		t.Fatal("expected sha256 digest in subject")
	}
}

func TestGenerateCorpusAttestationPredicate(t *testing.T) {
	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "test-corpus",
		ProjectID:      "test-project",
		ScannerVersion: "0.2.0",
		Verdict:        "corpus_partial",
		Confidence:     "medium",
		FilesScanned:   5,
		UnitsExtracted: 2,
		ScannedFiles:   []string{"a.md", "b.yaml"},
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pred CorpusPredicate
	if err := json.Unmarshal(stmt.Predicate, &pred); err != nil {
		t.Fatalf("unmarshal predicate: %v", err)
	}

	if pred.Version != CorpusAttestationVersion {
		t.Fatalf("expected version %s, got %s", CorpusAttestationVersion, pred.Version)
	}
	if pred.CorpusID != "test-corpus" {
		t.Fatalf("expected corpusId test-corpus, got %s", pred.CorpusID)
	}
	if pred.ProjectID != "test-project" {
		t.Fatalf("expected projectId test-project, got %s", pred.ProjectID)
	}
	if pred.ScannerVersion != "0.2.0" {
		t.Fatalf("expected scanner version 0.2.0, got %s", pred.ScannerVersion)
	}
	if pred.Verdict != "corpus_partial" {
		t.Fatalf("expected verdict corpus_partial, got %s", pred.Verdict)
	}
	if pred.Confidence != "medium" {
		t.Fatalf("expected confidence medium, got %s", pred.Confidence)
	}
	if pred.FilesScanned != 5 {
		t.Fatalf("expected 5 files scanned, got %d", pred.FilesScanned)
	}
	if pred.UnitsExtracted != 2 {
		t.Fatalf("expected 2 units extracted, got %d", pred.UnitsExtracted)
	}
	if !pred.Timestamp.Equal(attestNow) {
		t.Fatalf("expected timestamp %v, got %v", attestNow, pred.Timestamp)
	}
}

func TestGenerateCorpusAttestationWithPolicy(t *testing.T) {
	policy := &Policy{
		Allow:  []string{"docs/**"},
		Ignore: []string{"**/*.tmp"},
	}

	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "policy-corpus",
		ProjectID:      "policy-project",
		ScannerVersion: "0.1.0",
		Verdict:        "corpus_admissible",
		FilesScanned:   3,
		ScannedFiles:   []string{"docs/a.md"},
		Policy:         policy,
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pred CorpusPredicate
	if err := json.Unmarshal(stmt.Predicate, &pred); err != nil {
		t.Fatalf("unmarshal predicate: %v", err)
	}

	if pred.Policy == nil {
		t.Fatal("expected policy in predicate")
	}
	if len(pred.Policy.AllowPatterns) != 1 || pred.Policy.AllowPatterns[0] != "docs/**" {
		t.Fatalf("expected allow docs/**, got %v", pred.Policy.AllowPatterns)
	}
	if len(pred.Policy.IgnorePatterns) != 1 || pred.Policy.IgnorePatterns[0] != "**/*.tmp" {
		t.Fatalf("expected ignore **/*.tmp, got %v", pred.Policy.IgnorePatterns)
	}
}

func TestGenerateCorpusAttestationRecordsScopeAndDiagnosis(t *testing.T) {
	diagnosis := &DiagnoseVerdict{
		Profile:    ProfileRBOKLawbook,
		Verdict:    "in_scope",
		Confidence: "high",
		Summary:    "admitted",
	}
	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "rbok",
		ProjectID:      "airbook",
		ScannerVersion: "0.1.0",
		Verdict:        VerdictAdmissible,
		Confidence:     "high",
		Scope:          "full_profile",
		Diagnosis:      diagnosis,
		FilesScanned:   3,
		ScannedFiles:   []string{"a.md"},
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pred CorpusPredicate
	if err := json.Unmarshal(stmt.Predicate, &pred); err != nil {
		t.Fatalf("unmarshal predicate: %v", err)
	}
	if pred.Scope != "full_profile" {
		t.Fatalf("expected full_profile scope, got %q", pred.Scope)
	}
	if pred.Diagnosis == nil || pred.Diagnosis.Verdict != "in_scope" {
		t.Fatalf("expected embedded diagnosis, got %+v", pred.Diagnosis)
	}
}

func TestGenerateCorpusAttestationRejectsAdmissibleWhenDiagnosisBlocked(t *testing.T) {
	_, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:     "rbok",
		ProjectID:    "airbook",
		Verdict:      VerdictAdmissible,
		Confidence:   "high",
		Scope:        "full_profile",
		FilesScanned: 3,
		ScannedFiles: []string{"a.md"},
		Diagnosis: &DiagnoseVerdict{
			Profile:    ProfileRBOKLawbook,
			Verdict:    "blocked",
			Confidence: "low",
			Blockers:   []string{"blocked binary: 00_meta/firmware.bin"},
		},
		Now: attestNow,
	})
	if err == nil {
		t.Fatal("expected admissible attestation to be rejected for blocked diagnosis")
	}
}

func TestGenerateCorpusAttestationDeterministicHash(t *testing.T) {
	opts := CorpusAttestationOptions{
		CorpusID:       "hash-test",
		ProjectID:      "hash-project",
		ScannerVersion: "0.1.0",
		Verdict:        "corpus_admissible",
		ScannedFiles:   []string{"b.yaml", "a.md", "c.json"},
		Now:            attestNow,
	}

	stmt1, _ := GenerateCorpusAttestation(opts)
	// Reorder files — hash should be the same (sorted internally).
	opts.ScannedFiles = []string{"c.json", "a.md", "b.yaml"}
	stmt2, _ := GenerateCorpusAttestation(opts)

	if stmt1.Subject[0].Digest["sha256"] != stmt2.Subject[0].Digest["sha256"] {
		t.Fatalf("expected deterministic hash regardless of file order: %s != %s",
			stmt1.Subject[0].Digest["sha256"], stmt2.Subject[0].Digest["sha256"])
	}
}

func TestGenerateCorpusAttestationMissingCorpusID(t *testing.T) {
	_, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		ProjectID: "proj",
		Verdict:   "corpus_admissible",
	})
	if err == nil {
		t.Fatal("expected error for missing corpusId")
	}
}

func TestGenerateCorpusAttestationMissingProjectID(t *testing.T) {
	_, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID: "corpus",
		Verdict:  "corpus_admissible",
	})
	if err == nil {
		t.Fatal("expected error for missing projectId")
	}
}

func TestGenerateCorpusAttestationMissingVerdict(t *testing.T) {
	_, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:  "corpus",
		ProjectID: "proj",
	})
	if err == nil {
		t.Fatal("expected error for missing verdict")
	}
}

func TestGenerateCorpusAttestationDefaultScannerVersion(t *testing.T) {
	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:  "corpus",
		ProjectID: "proj",
		Verdict:   "corpus_admissible",
		Now:       attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pred CorpusPredicate
	_ = json.Unmarshal(stmt.Predicate, &pred)
	if pred.ScannerVersion != "unknown" {
		t.Fatalf("expected default scanner version 'unknown', got %s", pred.ScannerVersion)
	}
}

func TestWriteAttestation(t *testing.T) {
	stmt, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "write-test",
		ProjectID:      "write-proj",
		ScannerVersion: "0.1.0",
		Verdict:        "corpus_admissible",
		ScannedFiles:   []string{"file.md"},
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteAttestation(&buf, stmt); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded CorpusAttestationStatement
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v\n%s", err, buf.String())
	}
	if decoded.Type != InTotoStatementType {
		t.Fatalf("expected in-toto type after round-trip, got %s", decoded.Type)
	}
	if decoded.PredicateType != CorpusPredicateType {
		t.Fatalf("expected corpus predicate type, got %s", decoded.PredicateType)
	}
}

func TestWriteAttestationContainsAllFields(t *testing.T) {
	stmt, _ := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "fields-test",
		ProjectID:      "fields-proj",
		ScannerVersion: "1.0.0",
		Verdict:        "corpus_blocked",
		Confidence:     "low",
		FilesScanned:   100,
		UnitsExtracted: 50,
		ScannedFiles:   []string{"x.md"},
		Metadata:       map[string]any{"env": "ci"},
		Now:            attestNow,
	})

	var buf bytes.Buffer
	_ = WriteAttestation(&buf, stmt)
	output := buf.String()

	for _, expected := range []string{
		`"_type"`, `"subject"`, `"predicateType"`, `"predicate"`,
		`"corpusId"`, `"projectId"`, `"snapshotHash"`, `"scannerVersion"`,
		`"verdict"`, `"confidence"`, `"filesScanned"`, `"unitsExtracted"`,
	} {
		if !bytes.Contains(buf.Bytes(), []byte(expected)) {
			t.Errorf("expected output to contain %s:\n%s", expected, output)
		}
	}
}

func TestSnapshotHashEmptyFiles(t *testing.T) {
	hash := computeSnapshotHash(nil)
	if hash == "" {
		t.Fatal("expected non-empty hash even for empty file list")
	}
	// Empty list should produce consistent hash.
	hash2 := computeSnapshotHash([]string{})
	if hash != hash2 {
		t.Fatalf("expected same hash for nil and empty slice: %s != %s", hash, hash2)
	}
}
