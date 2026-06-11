package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/attestation"
)

func TestAttestCLI_KeygenSignVerifyRoundtrip(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "priv.pem")
	pub := filepath.Join(dir, "pub.pem")
	subject := filepath.Join(dir, "subject.txt")
	envelope := filepath.Join(dir, "env.json")
	if err := os.WriteFile(subject, []byte("real artifact bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) (int, string) {
		var out, errb bytes.Buffer
		code := attestCommand(args, &out, &errb)
		return code, out.String() + errb.String()
	}

	if code, out := run("keygen", "--out", priv, "--pub-out", pub); code != 0 {
		t.Fatalf("keygen failed (%d): %s", code, out)
	}
	if code, out := run("sign", "--project-id", "nomos-ckm", "--verdict", "admitted", "--subject", subject, "--key", priv, "--out", envelope); code != 0 {
		t.Fatalf("sign failed (%d): %s", code, out)
	}
	if code, out := run("verify", "--envelope", envelope, "--pub", pub); code != 0 {
		t.Fatalf("verify of a freshly signed envelope failed (%d): %s", code, out)
	}
}

// Adversarial at the CLI boundary: tampering the signed payload on disk must make
// `nomos attest verify` exit non-zero.
func TestAttestCLI_VerifyFailsOnTamper(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "priv.pem")
	pub := filepath.Join(dir, "pub.pem")
	subject := filepath.Join(dir, "subject.txt")
	envelope := filepath.Join(dir, "env.json")
	_ = os.WriteFile(subject, []byte("real artifact bytes"), 0o644)

	run := func(args ...string) int {
		var out, errb bytes.Buffer
		return attestCommand(args, &out, &errb)
	}
	if c := run("keygen", "--out", priv, "--pub-out", pub); c != 0 {
		t.Fatalf("keygen failed: %d", c)
	}
	if c := run("sign", "--project-id", "nomos-ckm", "--verdict", "admitted", "--subject", subject, "--key", priv, "--out", envelope); c != 0 {
		t.Fatalf("sign failed: %d", c)
	}

	// Tamper the base64 payload (flip an artifact digest) and rewrite the file.
	raw, err := os.ReadFile(envelope)
	if err != nil {
		t.Fatal(err)
	}
	var env attestation.DSSEEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	payload, _ := base64.StdEncoding.DecodeString(env.Payload)
	var stmt attestation.InTotoStatement
	if err := json.Unmarshal(payload, &stmt); err != nil {
		t.Fatal(err)
	}
	stmt.Subject[0].Digest["sha256"] = "deadbeef" + stmt.Subject[0].Digest["sha256"][8:]
	tampered, _ := json.Marshal(stmt)
	env.Payload = base64.StdEncoding.EncodeToString(tampered)
	out, _ := json.MarshalIndent(env, "", "  ")
	if err := os.WriteFile(envelope, out, 0o644); err != nil {
		t.Fatal(err)
	}

	if c := run("verify", "--envelope", envelope, "--pub", pub); c == 0 {
		t.Fatal("verify exited 0 on a tampered envelope")
	}
}

func TestAttestCLI_SignEphemeralRequiresPubOut(t *testing.T) {
	dir := t.TempDir()
	subject := filepath.Join(dir, "s.txt")
	_ = os.WriteFile(subject, []byte("x"), 0o644)
	var out, errb bytes.Buffer
	// No --key (ephemeral) and no --pub-out → must refuse (unverifiable signature).
	code := attestCommand([]string{"sign", "--project-id", "nomos-ckm", "--verdict", "admitted", "--subject", subject}, &out, &errb)
	if code == 0 {
		t.Fatalf("sign with ephemeral key and no --pub-out should fail; output: %s", out.String()+errb.String())
	}
}
