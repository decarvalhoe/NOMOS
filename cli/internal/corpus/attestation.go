package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

const (
	InTotoStatementType      = "https://in-toto.io/Statement/v1"
	CorpusPredicateType      = "https://nomos.dev/corpus-attestation/v1"
	CorpusAttestationVersion = "0.1.0"
)

// CorpusSubject identifies the corpus snapshot artifact.
type CorpusSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// CorpusAttestationStatement is a DSSE in-toto statement for corpus scan.
type CorpusAttestationStatement struct {
	Type          string          `json:"_type"`
	Subject       []CorpusSubject `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

// CorpusPredicate is the Nomos corpus-specific predicate.
type CorpusPredicate struct {
	Version        string           `json:"version"`
	CorpusID       string           `json:"corpusId"`
	ProjectID      string           `json:"projectId"`
	SnapshotHash   string           `json:"snapshotHash"`
	Timestamp      time.Time        `json:"timestamp"`
	ScannerVersion string           `json:"scannerVersion"`
	Scope          string           `json:"scope,omitempty"`
	Verdict        string           `json:"verdict"`
	Confidence     string           `json:"confidence,omitempty"`
	FilesScanned   int              `json:"filesScanned"`
	UnitsExtracted int              `json:"unitsExtracted,omitempty"`
	Diagnosis      *DiagnoseVerdict `json:"diagnosis,omitempty"`
	Policy         *AttestPolicy    `json:"policy,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
}

// AttestPolicy captures which glob policy was active during the scan.
type AttestPolicy struct {
	AllowPatterns  []string `json:"allowPatterns,omitempty"`
	IgnorePatterns []string `json:"ignorePatterns,omitempty"`
}

// CorpusAttestationOptions configures attestation generation.
type CorpusAttestationOptions struct {
	CorpusID       string
	ProjectID      string
	ScannerVersion string
	Scope          string
	Verdict        string
	Confidence     string
	FilesScanned   int
	UnitsExtracted int
	ScannedFiles   []string
	Diagnosis      *DiagnoseVerdict
	Policy         *Policy
	Metadata       map[string]any
	Now            time.Time
}

// GenerateCorpusAttestation creates an in-toto statement attesting a corpus scan.
func GenerateCorpusAttestation(opts CorpusAttestationOptions) (CorpusAttestationStatement, error) {
	if opts.CorpusID == "" {
		return CorpusAttestationStatement{}, fmt.Errorf("corpusId is required")
	}
	if opts.ProjectID == "" {
		return CorpusAttestationStatement{}, fmt.Errorf("projectId is required")
	}
	if opts.Verdict == "" {
		return CorpusAttestationStatement{}, fmt.Errorf("verdict is required")
	}
	if opts.ScannerVersion == "" {
		opts.ScannerVersion = "unknown"
	}
	if err := validateAttestationClaim(opts.Verdict, opts.Diagnosis); err != nil {
		return CorpusAttestationStatement{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	snapshotHash := computeSnapshotHash(opts.ScannedFiles)

	var attestPolicy *AttestPolicy
	if opts.Policy != nil {
		attestPolicy = &AttestPolicy{
			AllowPatterns:  opts.Policy.Allow,
			IgnorePatterns: opts.Policy.Ignore,
		}
	}

	predicate := CorpusPredicate{
		Version:        CorpusAttestationVersion,
		CorpusID:       opts.CorpusID,
		ProjectID:      opts.ProjectID,
		SnapshotHash:   snapshotHash,
		Timestamp:      now,
		ScannerVersion: opts.ScannerVersion,
		Scope:          opts.Scope,
		Verdict:        opts.Verdict,
		Confidence:     opts.Confidence,
		FilesScanned:   opts.FilesScanned,
		UnitsExtracted: opts.UnitsExtracted,
		Diagnosis:      opts.Diagnosis,
		Policy:         attestPolicy,
		Metadata:       opts.Metadata,
	}

	predicateJSON, err := json.Marshal(predicate)
	if err != nil {
		return CorpusAttestationStatement{}, fmt.Errorf("marshal predicate: %w", err)
	}

	subject := CorpusSubject{
		Name: fmt.Sprintf("corpus:%s", opts.CorpusID),
		Digest: map[string]string{
			"sha256": snapshotHash,
		},
	}

	return CorpusAttestationStatement{
		Type:          InTotoStatementType,
		Subject:       []CorpusSubject{subject},
		PredicateType: CorpusPredicateType,
		Predicate:     predicateJSON,
	}, nil
}

func validateAttestationClaim(verdict string, diagnosis *DiagnoseVerdict) error {
	if diagnosis == nil {
		return nil
	}
	diagnosisVerdict := diagnosisVerdictToCorpusVerdict(diagnosis.Verdict)
	switch verdict {
	case VerdictAdmissible:
		if diagnosisVerdict != VerdictAdmissible {
			return fmt.Errorf("attestation verdict %s overclaims blocked or partial diagnosis %s", verdict, diagnosis.Verdict)
		}
	case VerdictPartial:
		if diagnosisVerdict == VerdictBlocked {
			return fmt.Errorf("attestation verdict %s overclaims blocked diagnosis %s", verdict, diagnosis.Verdict)
		}
	}
	return nil
}

func diagnosisVerdictToCorpusVerdict(verdict string) string {
	switch verdict {
	case "in_scope", VerdictAdmissible:
		return VerdictAdmissible
	case "partial", VerdictPartial:
		return VerdictPartial
	case "blocked", VerdictBlocked:
		return VerdictBlocked
	default:
		return verdict
	}
}

// WriteAttestation writes the attestation statement as indented JSON.
func WriteAttestation(w io.Writer, stmt CorpusAttestationStatement) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(stmt)
}

// computeSnapshotHash produces a deterministic hash of the scanned file list.
func computeSnapshotHash(files []string) string {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	h := sha256.New()
	for _, f := range sorted {
		h.Write([]byte(f))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}
