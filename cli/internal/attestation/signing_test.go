package attestation

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testStatement(t *testing.T) InTotoStatement {
	t.Helper()
	att := NomosAttestation{
		ProjectID:   "nomos-ckm",
		Verdict:     "admitted",
		Confidence:  "high",
		Timestamp:   time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		Evidence:    []string{"corpus.json"},
		AttesterID:  "nomos",
		AttestLevel: "signed",
	}
	stmt, err := GenerateStatement(att, []Subject{SubjectFromBytes("corpus.json", []byte("real bytes"))})
	if err != nil {
		t.Fatalf("GenerateStatement: %v", err)
	}
	return stmt
}

func TestSignVerify_Roundtrip(t *testing.T) {
	signer, err := GenerateSigner()
	if err != nil {
		t.Fatalf("GenerateSigner: %v", err)
	}
	env, err := signer.SignStatement(testStatement(t))
	if err != nil {
		t.Fatalf("SignStatement: %v", err)
	}
	if env.Signatures[0].Sig == "" {
		t.Fatal("signature is empty — nothing was actually signed")
	}
	if env.Signatures[0].KeyID != signer.KeyID() {
		t.Fatal("signature keyid does not match signer")
	}
	pub, err := ParsePublicKeyPEM(mustPEM(t, signer.PublicKeyPEM))
	if err != nil {
		t.Fatalf("ParsePublicKeyPEM: %v", err)
	}
	if err := VerifyEnvelope(env, pub); err != nil {
		t.Fatalf("verification of a freshly signed envelope failed: %v", err)
	}
}

// The crypto sieve (doctrine §2.3): altering one byte of the signed payload —
// here, an artifact digest recorded in the statement — must make Verify fail.
// Without real signing this passes; the failure is the proof.
func TestVerify_FailsWhenArtifactDigestTampered(t *testing.T) {
	signer, _ := GenerateSigner()
	env, err := signer.SignStatement(testStatement(t))
	if err != nil {
		t.Fatalf("SignStatement: %v", err)
	}
	pub, _ := ParsePublicKeyPEM(mustPEM(t, signer.PublicKeyPEM))

	// Sanity: it verifies before tampering.
	if err := VerifyEnvelope(env, pub); err != nil {
		t.Fatalf("pre-tamper verification failed: %v", err)
	}

	// Decode the signed payload, flip an artifact digest, re-encode in place.
	raw, _ := base64.StdEncoding.DecodeString(env.Payload)
	var stmt InTotoStatement
	if err := json.Unmarshal(raw, &stmt); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	orig := stmt.Subject[0].Digest["sha256"]
	stmt.Subject[0].Digest["sha256"] = flipHexChar(orig)
	tamperedPayload, _ := json.Marshal(stmt)
	env.Payload = base64.StdEncoding.EncodeToString(tamperedPayload)

	if err := VerifyEnvelope(env, pub); err == nil {
		t.Fatal("verification PASSED after an artifact digest was tampered — signing is not real")
	}
}

func TestVerify_FailsWithWrongKey(t *testing.T) {
	signer, _ := GenerateSigner()
	env, _ := signer.SignStatement(testStatement(t))

	other, _ := GenerateSigner()
	otherPub, _ := ParsePublicKeyPEM(mustPEM(t, other.PublicKeyPEM))
	if err := VerifyEnvelope(env, otherPub); err == nil {
		t.Fatal("verification PASSED with an unrelated public key")
	}
}

func TestVerify_FailsWhenSignatureTampered(t *testing.T) {
	signer, _ := GenerateSigner()
	env, _ := signer.SignStatement(testStatement(t))
	pub, _ := ParsePublicKeyPEM(mustPEM(t, signer.PublicKeyPEM))

	sig, _ := base64.StdEncoding.DecodeString(env.Signatures[0].Sig)
	sig[len(sig)-1] ^= 0x01
	env.Signatures[0].Sig = base64.StdEncoding.EncodeToString(sig)
	if err := VerifyEnvelope(env, pub); err == nil {
		t.Fatal("verification PASSED with a tampered signature")
	}
}

func TestSignerPEM_Roundtrip(t *testing.T) {
	signer, _ := GenerateSigner()
	priv := mustPEM(t, signer.PrivateKeyPEM)
	loaded, err := SignerFromPEM(priv)
	if err != nil {
		t.Fatalf("SignerFromPEM: %v", err)
	}
	if loaded.KeyID() != signer.KeyID() {
		t.Fatal("reloaded signer has a different key id")
	}
	// A signature from the reloaded key verifies against the original public key.
	env, _ := loaded.SignStatement(testStatement(t))
	pub, _ := ParsePublicKeyPEM(mustPEM(t, signer.PublicKeyPEM))
	if err := VerifyEnvelope(env, pub); err != nil {
		t.Fatalf("reloaded-key signature did not verify: %v", err)
	}
}

func TestPAE_MatchesDSSESpec(t *testing.T) {
	// DSSEv1 SP len(type) SP type SP len(body) SP body
	got := string(pae("application/vnd.in-toto+json", []byte("hello")))
	want := "DSSEv1 28 application/vnd.in-toto+json 5 hello"
	if got != want {
		t.Fatalf("PAE = %q, want %q", got, want)
	}
}

func flipHexChar(s string) string {
	if s == "" {
		return "a"
	}
	b := []byte(s)
	if b[0] == 'a' {
		b[0] = 'b'
	} else {
		b[0] = 'a'
	}
	return string(b)
}

func mustPEM(t *testing.T, fn func() ([]byte, error)) []byte {
	t.Helper()
	p, err := fn()
	if err != nil {
		t.Fatalf("PEM: %v", err)
	}
	if !strings.Contains(string(p), "-----BEGIN") {
		t.Fatalf("not a PEM block: %q", p)
	}
	return p
}
