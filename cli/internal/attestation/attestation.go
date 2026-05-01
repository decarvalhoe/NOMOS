package attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

const (
	InTotoStatementType = "https://in-toto.io/Statement/v1"
	SLSAPredicateType   = "https://slsa.dev/provenance/v1"
	CosignPayloadType   = "application/vnd.dev.cosign.simplesigning.v1+json"
	NomosPredicateType  = "https://nomos.dev/attestation/v1"
)

// Subject identifies an artifact by its digest.
type Subject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// InTotoStatement is a DSSE in-toto statement envelope.
type InTotoStatement struct {
	Type          string          `json:"_type"`
	Subject       []Subject       `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

// NomosAttestation is the Nomos-specific predicate embedded in an in-toto statement.
type NomosAttestation struct {
	ProjectID   string    `json:"projectId"`
	Verdict     string    `json:"verdict"`
	Confidence  string    `json:"confidence"`
	Timestamp   time.Time `json:"timestamp"`
	Evidence    []string  `json:"evidence"`
	AttesterID  string    `json:"attesterId"`
	AttestLevel string    `json:"attestLevel"`
}

// GenerateStatement creates an in-toto statement wrapping a NomosAttestation predicate.
func GenerateStatement(att NomosAttestation, subjects []Subject) (InTotoStatement, error) {
	if len(subjects) == 0 {
		return InTotoStatement{}, fmt.Errorf("at least one subject is required")
	}
	if att.ProjectID == "" {
		return InTotoStatement{}, fmt.Errorf("projectId is required")
	}
	if att.Verdict == "" {
		return InTotoStatement{}, fmt.Errorf("verdict is required")
	}

	predBytes, err := json.Marshal(att)
	if err != nil {
		return InTotoStatement{}, fmt.Errorf("marshal predicate: %w", err)
	}

	return InTotoStatement{
		Type:          InTotoStatementType,
		Subject:       subjects,
		PredicateType: NomosPredicateType,
		Predicate:     json.RawMessage(predBytes),
	}, nil
}

// SLSAProvenance holds SLSA v1 provenance metadata.
type SLSAProvenance struct {
	BuildDefinition SLSABuildDefinition `json:"buildDefinition"`
	RunDetails      SLSARunDetails      `json:"runDetails"`
}

// SLSABuildDefinition describes what was built and how.
type SLSABuildDefinition struct {
	BuildType            string            `json:"buildType"`
	ExternalParameters   map[string]string `json:"externalParameters"`
	InternalParameters   map[string]string `json:"internalParameters,omitempty"`
	ResolvedDependencies []SLSADependency  `json:"resolvedDependencies,omitempty"`
}

// SLSADependency identifies a resolved build input.
type SLSADependency struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

// SLSARunDetails describes where and when the build ran.
type SLSARunDetails struct {
	Builder   SLSABuilder   `json:"builder"`
	Metadata  SLSAMetadata  `json:"metadata"`
}

// SLSABuilder identifies the build system.
type SLSABuilder struct {
	ID string `json:"id"`
}

// SLSAMetadata holds build timing and reproducibility info.
type SLSAMetadata struct {
	InvocationID string    `json:"invocationId"`
	StartedOn    time.Time `json:"startedOn"`
	FinishedOn   time.Time `json:"finishedOn"`
}

// GenerateProvenance creates an in-toto statement with SLSA provenance predicate.
func GenerateProvenance(prov SLSAProvenance, subjects []Subject) (InTotoStatement, error) {
	if len(subjects) == 0 {
		return InTotoStatement{}, fmt.Errorf("at least one subject is required")
	}
	if prov.RunDetails.Builder.ID == "" {
		return InTotoStatement{}, fmt.Errorf("builder ID is required")
	}

	predBytes, err := json.Marshal(prov)
	if err != nil {
		return InTotoStatement{}, fmt.Errorf("marshal provenance: %w", err)
	}

	return InTotoStatement{
		Type:          InTotoStatementType,
		Subject:       subjects,
		PredicateType: SLSAPredicateType,
		Predicate:     json.RawMessage(predBytes),
	}, nil
}

// VerifyProvenance checks that a provenance statement has required fields
// and that subjects have valid digests.
func VerifyProvenance(stmt InTotoStatement) error {
	if stmt.Type != InTotoStatementType {
		return fmt.Errorf("unexpected statement type %q", stmt.Type)
	}
	if stmt.PredicateType != SLSAPredicateType {
		return fmt.Errorf("unexpected predicate type %q, expected SLSA provenance", stmt.PredicateType)
	}
	if len(stmt.Subject) == 0 {
		return fmt.Errorf("statement has no subjects")
	}
	for i, subj := range stmt.Subject {
		if subj.Name == "" {
			return fmt.Errorf("subject[%d] has empty name", i)
		}
		if len(subj.Digest) == 0 {
			return fmt.Errorf("subject[%d] %q has no digests", i, subj.Name)
		}
	}

	var prov SLSAProvenance
	if err := json.Unmarshal(stmt.Predicate, &prov); err != nil {
		return fmt.Errorf("invalid provenance predicate: %w", err)
	}
	if prov.RunDetails.Builder.ID == "" {
		return fmt.Errorf("provenance missing builder ID")
	}
	if prov.BuildDefinition.BuildType == "" {
		return fmt.Errorf("provenance missing build type")
	}
	return nil
}

// CosignEnvelope is a simplified cosign signing envelope for Nomos attestations.
type CosignEnvelope struct {
	PayloadType string           `json:"payloadType"`
	Payload     string           `json:"payload"`
	Signatures  []CosignSig      `json:"signatures"`
}

// CosignSig holds a single signature entry.
type CosignSig struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// WrapCosignEnvelope wraps a JSON-serializable payload in a cosign-compatible
// DSSE envelope. The signature field is left empty for an external signer to fill.
func WrapCosignEnvelope(payload any, keyID string) (CosignEnvelope, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return CosignEnvelope{}, fmt.Errorf("marshal payload: %w", err)
	}

	return CosignEnvelope{
		PayloadType: CosignPayloadType,
		Payload:     string(payloadBytes),
		Signatures: []CosignSig{
			{KeyID: keyID, Sig: ""},
		},
	}, nil
}

// VerifyCosignEnvelope checks envelope structure integrity (not cryptographic
// signature verification, which requires the actual key).
func VerifyCosignEnvelope(env CosignEnvelope) error {
	if env.PayloadType == "" {
		return fmt.Errorf("envelope has empty payload type")
	}
	if env.Payload == "" {
		return fmt.Errorf("envelope has empty payload")
	}
	if len(env.Signatures) == 0 {
		return fmt.Errorf("envelope has no signatures")
	}
	for i, sig := range env.Signatures {
		if sig.KeyID == "" {
			return fmt.Errorf("signature[%d] has empty key ID", i)
		}
	}
	return nil
}

// DigestSHA256 computes a hex-encoded SHA-256 digest of the given data.
func DigestSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SubjectFromBytes creates a Subject with a SHA-256 digest of the given data.
func SubjectFromBytes(name string, data []byte) Subject {
	return Subject{
		Name:   name,
		Digest: map[string]string{"sha256": DigestSHA256(data)},
	}
}

// WriteJSON writes a JSON-encoded value to w with indentation.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
