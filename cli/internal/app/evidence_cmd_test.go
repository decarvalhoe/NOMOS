package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvidenceSignAndVerifyDetectsTamperedArtifact(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "evidence.json")
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(artifactPath, []byte(`{"status":"original"}`), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"evidence", "sign",
		"--artifact", artifactPath,
		"--out", bundlePath,
		"--bundle-id", "bundle-1",
		"--issuer", "nomos-test",
		"--subject", "sample-evidence",
		"--signature-ref", "external-signature:sig-1",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected sign exit 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	rawBundle := readFileAbs(t, bundlePath)
	if !strings.Contains(rawBundle, `"verification_status": "prepared_for_external_signature"`) {
		t.Fatalf("expected prepared signed status, got %s", rawBundle)
	}
	if !strings.Contains(rawBundle, `"signature_value_or_external_ref": "external-signature:sig-1"`) {
		t.Fatalf("expected external signature reference, got %s", rawBundle)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"evidence", "verify", "--bundle", bundlePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected verify exit 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "verified"`) {
		t.Fatalf("expected verified status, got %s", stdout.String())
	}

	if err := os.WriteFile(artifactPath, []byte(`{"status":"tampered"}`), 0o600); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"evidence", "verify", "--bundle", bundlePath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected verify failure after tamper, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "artifact hash mismatch") {
		t.Fatalf("expected hash mismatch error, got %q", stderr.String())
	}
}

func TestEvidenceUnsignedModeIsAllowedButMarkedWeaker(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "evidence.json")
	bundlePath := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(artifactPath, []byte(`{"status":"unsigned"}`), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"evidence", "sign",
		"--artifact", artifactPath,
		"--out", bundlePath,
		"--bundle-id", "bundle-unsigned",
		"--issuer", "nomos-test",
		"--subject", "sample-evidence",
		"--unsigned",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected unsigned sign exit 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	rawBundle := readFileAbs(t, bundlePath)
	if !strings.Contains(rawBundle, `"signature_mode": "unsigned_weaker"`) {
		t.Fatalf("expected unsigned weaker mode, got %s", rawBundle)
	}
	if !strings.Contains(rawBundle, `"verification_status": "unsigned_weaker"`) {
		t.Fatalf("expected unsigned weaker status, got %s", rawBundle)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"evidence", "verify", "--bundle", bundlePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected unsigned verify exit 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "verified_unsigned_weaker"`) {
		t.Fatalf("expected verified unsigned weaker status, got %s", stdout.String())
	}
}

func TestEvidenceHashOutputsSHA256(t *testing.T) {
	dir := t.TempDir()
	artifactPath := filepath.Join(dir, "evidence.txt")
	if err := os.WriteFile(artifactPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"evidence", "hash", "--artifact", artifactPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected hash exit 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"sha256": "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"`) {
		t.Fatalf("expected SHA-256 digest for hello, got %s", stdout.String())
	}
}
