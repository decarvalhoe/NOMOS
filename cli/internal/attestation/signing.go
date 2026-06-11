package attestation

// CKM-H1 — real cryptographic signing for NOMOS attestations.
//
// The audit (#518/#519) found the "signed / supply-chain certified" claim was
// false: WrapCosignEnvelope hard-codes Sig:"" , the committed predicates carried
// fixture signature *strings*, and "verification" only checked that a field was
// non-empty. Nothing was cryptographically signed.
//
// This file makes the claim true. It implements the DSSE (Dead Simple Signing
// Envelope) v1 PAE encoding and signs it with ECDSA P-256 — entirely in the Go
// standard library, no external cosign binary and no network. Keyless Sigstore
// (Fulcio/Rekor) needs an OIDC round-trip and is a documented follow-up; what is
// here is real, offline, and tamper-evident:
//
//   - Sign produces a real ECDSA signature over PAE(payloadType, payload).
//   - Verify recomputes the PAE and checks the signature; flipping a single byte
//     of the payload (e.g. an artifact digest recorded in the statement) makes
//     Verify fail. That tamper-fail is the proof (doctrine §2.3, crypto sieve).
//
// Unsigned remains the honest default elsewhere; this is the path that earns the
// word "signed".

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
)

// DSSEPayloadTypeInToto is the payload type for an in-toto statement carried in
// a DSSE envelope (matches the cosign/in-toto convention).
const DSSEPayloadTypeInToto = "application/vnd.in-toto+json"

// SignatureModeDSSEECDSAP256 names the real signing scheme implemented here. It
// is deliberately distinct from "sigstore-keyless": this is key-based DSSE, not
// a Fulcio/Rekor keyless flow.
const SignatureModeDSSEECDSAP256 = "dsse-ecdsa-p256"

// DSSEEnvelope is a DSSE v1 envelope. Payload and each Sig are base64 (std).
type DSSEEnvelope struct {
	PayloadType string          `json:"payloadType"`
	Payload     string          `json:"payload"`
	Signatures  []DSSESignature `json:"signatures"`
}

// DSSESignature is one signature over the envelope's PAE.
type DSSESignature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// Signer holds an ECDSA P-256 private key and signs DSSE payloads.
type Signer struct {
	priv *ecdsa.PrivateKey
}

// GenerateSigner creates a fresh ECDSA P-256 signer.
func GenerateSigner() (*Signer, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ecdsa key: %w", err)
	}
	return &Signer{priv: priv}, nil
}

// SignerFromPEM loads a signer from a PKCS#8 or SEC1 PEM private key.
func SignerFromPEM(pemBytes []byte) (*Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		ecKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not ECDSA")
		}
		return &Signer{priv: ecKey}, nil
	}
	ecKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse EC private key: %w", err)
	}
	return &Signer{priv: ecKey}, nil
}

// PrivateKeyPEM returns the signer's private key as a PKCS#8 PEM. Handle with
// care — this is secret material.
func (s *Signer) PrivateKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(s.priv)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// PublicKeyPEM returns the signer's public key as a PKIX PEM. Safe to publish.
func (s *Signer) PublicKeyPEM() ([]byte, error) {
	return PublicKeyPEM(&s.priv.PublicKey)
}

// KeyID returns a stable fingerprint of the public key (hex sha256 of DER).
func (s *Signer) KeyID() string {
	return publicKeyID(&s.priv.PublicKey)
}

// Sign produces a DSSE envelope: the payload is base64-encoded and the signature
// is a real ECDSA signature over PAE(payloadType, payload).
func (s *Signer) Sign(payloadType string, payload []byte) (DSSEEnvelope, error) {
	if payloadType == "" {
		return DSSEEnvelope{}, fmt.Errorf("payloadType is required")
	}
	digest := sha256.Sum256(pae(payloadType, payload))
	sig, err := ecdsa.SignASN1(rand.Reader, s.priv, digest[:])
	if err != nil {
		return DSSEEnvelope{}, fmt.Errorf("ecdsa sign: %w", err)
	}
	return DSSEEnvelope{
		PayloadType: payloadType,
		Payload:     base64.StdEncoding.EncodeToString(payload),
		Signatures: []DSSESignature{{
			KeyID: s.KeyID(),
			Sig:   base64.StdEncoding.EncodeToString(sig),
		}},
	}, nil
}

// SignStatement marshals an in-toto statement and signs it as a DSSE envelope.
func (s *Signer) SignStatement(stmt InTotoStatement) (DSSEEnvelope, error) {
	payload, err := json.Marshal(stmt)
	if err != nil {
		return DSSEEnvelope{}, fmt.Errorf("marshal statement: %w", err)
	}
	return s.Sign(DSSEPayloadTypeInToto, payload)
}

// VerifyEnvelope checks that at least one signature on the envelope is a valid
// ECDSA signature over the envelope's PAE for the given public key. It returns a
// non-nil error if the payload was tampered with, the signature is forged, or
// the key does not match.
func VerifyEnvelope(env DSSEEnvelope, pub *ecdsa.PublicKey) error {
	if pub == nil {
		return fmt.Errorf("public key is required")
	}
	if env.PayloadType == "" {
		return fmt.Errorf("envelope has empty payloadType")
	}
	payload, err := base64.StdEncoding.DecodeString(env.Payload)
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if len(env.Signatures) == 0 {
		return fmt.Errorf("envelope has no signatures")
	}
	digest := sha256.Sum256(pae(env.PayloadType, payload))
	wantKeyID := publicKeyID(pub)
	for _, s := range env.Signatures {
		sig, err := base64.StdEncoding.DecodeString(s.Sig)
		if err != nil {
			continue
		}
		if ecdsa.VerifyASN1(pub, digest[:], sig) {
			return nil
		}
		_ = wantKeyID // key id is advisory; the cryptographic check is authoritative
	}
	return fmt.Errorf("no valid signature for the provided public key (tampered, forged, or wrong key)")
}

// VerifyEnvelopePayload verifies the envelope and returns the decoded payload so
// callers can re-parse the trusted statement only after the signature checks out.
func VerifyEnvelopePayload(env DSSEEnvelope, pub *ecdsa.PublicKey) ([]byte, error) {
	if err := VerifyEnvelope(env, pub); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(env.Payload)
}

// PublicKeyPEM encodes an ECDSA public key as PKIX PEM.
func PublicKeyPEM(pub *ecdsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// ParsePublicKeyPEM loads an ECDSA public key from a PKIX PEM.
func ParsePublicKeyPEM(pemBytes []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("PEM key is not ECDSA")
	}
	return pub, nil
}

// pae implements DSSE v1 Pre-Authentication Encoding:
//
//	"DSSEv1" SP LEN(type) SP type SP LEN(body) SP body
func pae(payloadType string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString("DSSEv1")
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(payloadType)))
	b.WriteByte(' ')
	b.WriteString(payloadType)
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(payload)))
	b.WriteByte(' ')
	b.Write(payload)
	return b.Bytes()
}

func publicKeyID(pub *ecdsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}
