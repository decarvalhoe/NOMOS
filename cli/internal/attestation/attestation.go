package attestation

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
	InTotoStatementType        = "https://in-toto.io/Statement/v1"
	SLSAPredicateType          = "https://slsa.dev/provenance/v1"
	CosignPayloadType          = "application/vnd.dev.cosign.simplesigning.v1+json"
	NomosPredicateType         = "https://nomos.dev/attestation/v1"
	ClaimBoundaryPredicateType = "https://nomos.dev/claim-boundary/v1"
	SupplyChainPredicateType   = "https://nomos.dev/ckm/supply-chain/v1"
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

// SupplyChainStepName identifies a CKM transformation stage.
type SupplyChainStepName string

const (
	StepIngestion SupplyChainStepName = "ingestion"
	StepCanon     SupplyChainStepName = "canon"
	StepEmbedding SupplyChainStepName = "embedding"
)

const (
	SignatureModeUnsigned        = "unsigned"
	SignatureModeSigstoreKeyless = "sigstore-keyless"
	SignatureStatusUnsigned      = "unsigned"
	SignatureStatusSigned        = "signed"
)

// SupplyChainSignature records how the predicate was, or was not, signed.
type SupplyChainSignature struct {
	Mode      string `json:"mode"`
	Status    string `json:"status"`
	TrustTier string `json:"trust_tier"`
	RekorUUID string `json:"rekor_uuid,omitempty"`
}

// SupplyChainStep records one material-to-product transformation.
type SupplyChainStep struct {
	Name      SupplyChainStepName `json:"name"`
	Materials []Subject           `json:"materials,omitempty"`
	Products  []Subject           `json:"products"`
}

// SupplyChainPredicate attests the CKM source -> canon -> embedding chain.
type SupplyChainPredicate struct {
	Version   string               `json:"version"`
	ProjectID string               `json:"projectId"`
	CorpusID  string               `json:"corpusId"`
	Signature SupplyChainSignature `json:"signature"`
	Steps     []SupplyChainStep    `json:"steps"`
}

// HasStep reports whether the predicate contains a named transformation stage.
func (p SupplyChainPredicate) HasStep(name SupplyChainStepName) bool {
	for _, step := range p.Steps {
		if step.Name == name {
			return true
		}
	}
	return false
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

// RefusedClaim records a claim Nomos explicitly refuses because supporting
// evidence is absent or insufficient.
type RefusedClaim struct {
	ClaimID          string   `json:"claimId"`
	Statement        string   `json:"statement"`
	Reason           string   `json:"reason"`
	RequiredEvidence []string `json:"requiredEvidence"`
	Decision         string   `json:"decision"`
}

// ClaimBoundarySignature records external signing evidence for a refusal
// predicate. Cryptographic verification is delegated to the signing backend.
type ClaimBoundarySignature struct {
	KeyID     string    `json:"keyId"`
	Signature string    `json:"signature"`
	SignedAt  time.Time `json:"signedAt"`
	LogURI    string    `json:"logUri,omitempty"`
}

// ClaimBoundaryPredicate is the Nomos predicate for claims it cannot prove and
// therefore refuses to assert.
type ClaimBoundaryPredicate struct {
	ProjectID     string                 `json:"projectId"`
	GeneratedAt   time.Time              `json:"generatedAt"`
	RefusedClaims []RefusedClaim         `json:"refusedClaims"`
	Verifier      string                 `json:"verifier"`
	SignatureMode string                 `json:"signatureMode"`
	Signature     ClaimBoundarySignature `json:"signature"`
	ClaimBoundary string                 `json:"claimBoundary"`
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

// GenerateClaimBoundaryStatement creates an in-toto statement for refused
// claims. Use VerifyClaimBoundaryStatement before trusting the predicate.
func GenerateClaimBoundaryStatement(predicate ClaimBoundaryPredicate, subjects []Subject) (InTotoStatement, error) {
	if len(subjects) == 0 {
		return InTotoStatement{}, fmt.Errorf("at least one subject is required")
	}
	if predicate.ProjectID == "" {
		return InTotoStatement{}, fmt.Errorf("projectId is required")
	}

	predBytes, err := json.Marshal(predicate)
	if err != nil {
		return InTotoStatement{}, fmt.Errorf("marshal claim boundary predicate: %w", err)
	}

	return InTotoStatement{
		Type:          InTotoStatementType,
		Subject:       subjects,
		PredicateType: ClaimBoundaryPredicateType,
		Predicate:     json.RawMessage(predBytes),
	}, nil
}

// VerifyClaimBoundaryStatement checks the structure of a signed claim-boundary
// refusal predicate. It does not perform online Rekor or key verification.
func VerifyClaimBoundaryStatement(stmt InTotoStatement) error {
	if stmt.Type != InTotoStatementType {
		return fmt.Errorf("unexpected statement type %q", stmt.Type)
	}
	if stmt.PredicateType != ClaimBoundaryPredicateType {
		return fmt.Errorf("unexpected predicate type %q, expected claim boundary", stmt.PredicateType)
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

	var predicate ClaimBoundaryPredicate
	if err := json.Unmarshal(stmt.Predicate, &predicate); err != nil {
		return fmt.Errorf("invalid claim boundary predicate: %w", err)
	}
	if predicate.ProjectID == "" {
		return fmt.Errorf("claim boundary missing projectId")
	}
	if len(predicate.RefusedClaims) == 0 {
		return fmt.Errorf("claim boundary has no refused claims")
	}
	for i, claim := range predicate.RefusedClaims {
		if claim.ClaimID == "" {
			return fmt.Errorf("refusedClaims[%d] missing claimId", i)
		}
		if claim.Statement == "" {
			return fmt.Errorf("refusedClaims[%d] missing statement", i)
		}
		if claim.Reason == "" {
			return fmt.Errorf("refusedClaims[%d] missing reason", i)
		}
		if claim.Decision != "refused" {
			return fmt.Errorf("refusedClaims[%d] decision must be refused", i)
		}
		if len(claim.RequiredEvidence) == 0 {
			return fmt.Errorf("refusedClaims[%d] missing required evidence list", i)
		}
	}
	if predicate.Verifier == "" {
		return fmt.Errorf("claim boundary missing verifier")
	}
	if predicate.SignatureMode == "dsse-cosign" || predicate.SignatureMode == "sigstore-keyless" {
		if predicate.Signature.KeyID == "" {
			return fmt.Errorf("signed claim boundary missing key ID")
		}
		if predicate.Signature.Signature == "" {
			return fmt.Errorf("signed claim boundary missing signature")
		}
		if predicate.Signature.SignedAt.IsZero() {
			return fmt.Errorf("signed claim boundary missing signedAt")
		}
	}
	return nil
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
	Builder  SLSABuilder  `json:"builder"`
	Metadata SLSAMetadata `json:"metadata"`
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

// GenerateSupplyChainStatement creates an in-toto statement for CKM supply-chain
// transformations. The predicate is additive: unsigned mode is allowed, but it
// is explicitly marked as lower-trust instead of being confused with Sigstore.
func GenerateSupplyChainStatement(pred SupplyChainPredicate) (InTotoStatement, error) {
	if pred.ProjectID == "" {
		return InTotoStatement{}, fmt.Errorf("projectId is required")
	}
	if pred.CorpusID == "" {
		return InTotoStatement{}, fmt.Errorf("corpusId is required")
	}
	if len(pred.Steps) == 0 {
		return InTotoStatement{}, fmt.Errorf("at least one supply-chain step is required")
	}
	for i, step := range pred.Steps {
		if step.Name == "" {
			return InTotoStatement{}, fmt.Errorf("step[%d] has empty name", i)
		}
		if len(step.Products) == 0 {
			return InTotoStatement{}, fmt.Errorf("step[%d] %s has no products", i, step.Name)
		}
	}
	pred.Version = firstNonEmpty(pred.Version, "0.1.0")
	pred.Signature = normalizeSupplyChainSignature(pred.Signature)

	subjects := collectSupplyChainSubjects(pred.Steps)
	predBytes, err := json.Marshal(pred)
	if err != nil {
		return InTotoStatement{}, fmt.Errorf("marshal supply-chain predicate: %w", err)
	}
	return InTotoStatement{
		Type:          InTotoStatementType,
		Subject:       subjects,
		PredicateType: SupplyChainPredicateType,
		Predicate:     json.RawMessage(predBytes),
	}, nil
}

// VerifySupplyChainStatement checks statement structure and verifies supplied
// artifact bytes against the digests recorded in the statement subjects.
func VerifySupplyChainStatement(stmt InTotoStatement, artifacts map[string][]byte) error {
	if stmt.Type != InTotoStatementType {
		return fmt.Errorf("unexpected statement type %q", stmt.Type)
	}
	if stmt.PredicateType != SupplyChainPredicateType {
		return fmt.Errorf("unexpected predicate type %q, expected CKM supply-chain", stmt.PredicateType)
	}
	if len(stmt.Subject) == 0 {
		return fmt.Errorf("statement has no subjects")
	}
	var pred SupplyChainPredicate
	if err := json.Unmarshal(stmt.Predicate, &pred); err != nil {
		return fmt.Errorf("invalid supply-chain predicate: %w", err)
	}
	if pred.ProjectID == "" || pred.CorpusID == "" {
		return fmt.Errorf("supply-chain predicate missing projectId or corpusId")
	}
	if len(pred.Steps) == 0 {
		return fmt.Errorf("supply-chain predicate has no steps")
	}
	if pred.Signature.Status == SignatureStatusSigned {
		if pred.Signature.Mode != SignatureModeSigstoreKeyless {
			return fmt.Errorf("signed supply-chain predicate must use %s mode", SignatureModeSigstoreKeyless)
		}
		if pred.Signature.RekorUUID == "" {
			return fmt.Errorf("signed supply-chain predicate missing Rekor UUID")
		}
	}
	for _, subj := range stmt.Subject {
		expected := subj.Digest["sha256"]
		if expected == "" {
			return fmt.Errorf("subject %q has no sha256 digest", subj.Name)
		}
		actualBytes, ok := artifacts[subj.Name]
		if !ok {
			continue
		}
		actual := DigestSHA256(actualBytes)
		if actual != expected {
			return fmt.Errorf("artifact %q sha256 mismatch: got %s want %s", subj.Name, actual, expected)
		}
	}
	return nil
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
	PayloadType string      `json:"payloadType"`
	Payload     string      `json:"payload"`
	Signatures  []CosignSig `json:"signatures"`
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

func normalizeSupplyChainSignature(sig SupplyChainSignature) SupplyChainSignature {
	if sig.Mode == "" {
		sig.Mode = SignatureModeUnsigned
	}
	if sig.Status == "" {
		if sig.Mode == SignatureModeUnsigned {
			sig.Status = SignatureStatusUnsigned
		} else {
			sig.Status = SignatureStatusSigned
		}
	}
	if sig.TrustTier == "" {
		if sig.Status == SignatureStatusSigned && sig.Mode == SignatureModeSigstoreKeyless && sig.RekorUUID != "" {
			sig.TrustTier = "signed"
		} else {
			sig.TrustTier = "unverified"
		}
	}
	return sig
}

func collectSupplyChainSubjects(steps []SupplyChainStep) []Subject {
	byKey := map[string]Subject{}
	for _, step := range steps {
		for _, subj := range append(append([]Subject{}, step.Materials...), step.Products...) {
			if subj.Name == "" || len(subj.Digest) == 0 {
				continue
			}
			byKey[subj.Name+"\x00"+subj.Digest["sha256"]] = subj
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]Subject, 0, len(keys))
	for _, key := range keys {
		out = append(out, byKey[key])
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
