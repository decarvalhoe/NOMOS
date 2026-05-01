package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func validManifest() SidecarManifest {
	return SidecarManifest{
		SchemaVersion: "0.1.0",
		Sources: []SidecarSource{
			{
				ID:              "SRC-001",
				Path:            "docs/contract.pdf",
				Type:            "pdf",
				Domain:          "insurance",
				Priority:        "primary",
				Status:          "active",
				Hash:            "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Owner:           "Alice Example",
				License:         "proprietary",
				Confidentiality: "restricted",
				AllowedUses:     []string{"structured_contract"},
			},
		},
	}
}

func TestValidateValidManifest(t *testing.T) {
	result := ValidateSidecar(validManifest(), "")
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.SourceCount != 1 {
		t.Fatalf("expected 1 source, got %d", result.SourceCount)
	}
}

func TestValidateNoSources(t *testing.T) {
	m := SidecarManifest{SchemaVersion: "0.1.0"}
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeNoSources)
}

func TestValidateMissingOwner(t *testing.T) {
	m := validManifest()
	m.Sources[0].Owner = ""
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeOwnerMissing)
}

func TestValidateMissingConfidentiality(t *testing.T) {
	m := validManifest()
	m.Sources[0].Confidentiality = ""
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeConfidentialityEmpty)
}

func TestValidateInvalidConfidentiality(t *testing.T) {
	m := validManifest()
	m.Sources[0].Confidentiality = "top-secret"
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeConfidentialityBad)
}

func TestValidateMissingHash(t *testing.T) {
	m := validManifest()
	m.Sources[0].Hash = ""
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeHashMissing)
}

func TestValidateMalformedHash(t *testing.T) {
	m := validManifest()
	m.Sources[0].Hash = "md5:abcdef"
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeHashMalformed)
}

func TestValidateHashMismatch(t *testing.T) {
	dir := t.TempDir()
	content := []byte("real file content")
	writeCorpusFile(t, dir, "docs/contract.pdf", content)

	actualHash := sha256sum(content)

	m := validManifest()
	m.Sources[0].Hash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	result := ValidateSidecar(m, dir)
	assertInvalid(t, result)
	assertHasCode(t, result, CodeHashMismatch)

	// Now fix the hash — should pass.
	m.Sources[0].Hash = "sha256:" + actualHash
	result = ValidateSidecar(m, dir)
	if !result.Valid {
		t.Fatalf("expected valid after hash fix, got: %v", result.Errors)
	}
}

func TestValidateFileMissing(t *testing.T) {
	dir := t.TempDir()
	// Do not create the file.
	m := validManifest()
	result := ValidateSidecar(m, dir)
	assertInvalid(t, result)
	assertHasCode(t, result, CodeFileMissing)
}

func TestValidateFileUndeclared(t *testing.T) {
	dir := t.TempDir()
	writeCorpusFile(t, dir, "docs/contract.pdf", []byte("content"))
	writeCorpusFile(t, dir, "docs/extra-file.txt", []byte("surprise"))

	m := validManifest()
	m.Sources[0].Hash = "sha256:" + sha256sum([]byte("content"))

	result := ValidateSidecar(m, dir)
	assertInvalid(t, result)
	assertHasCode(t, result, CodeFileUndeclared)
}

func TestValidateMissingID(t *testing.T) {
	m := validManifest()
	m.Sources[0].ID = ""
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeIDMissing)
}

func TestValidateInvalidID(t *testing.T) {
	m := validManifest()
	m.Sources[0].ID = "lowercase-bad"
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeIDInvalid)
}

func TestValidateDuplicateID(t *testing.T) {
	m := validManifest()
	dup := m.Sources[0]
	m.Sources = append(m.Sources, dup)
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	assertHasCode(t, result, CodeIDDuplicate)
}

func TestValidateMultipleErrors(t *testing.T) {
	m := validManifest()
	m.Sources[0].Owner = ""
	m.Sources[0].Hash = ""
	m.Sources[0].Confidentiality = ""
	result := ValidateSidecar(m, "")
	assertInvalid(t, result)
	if len(result.Errors) < 3 {
		t.Fatalf("expected at least 3 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestParseSidecarManifestBytes(t *testing.T) {
	yaml := `
schema_version: "0.1.0"
sources:
  - id: SRC-001
    path: docs/file.pdf
    type: pdf
    domain: test
    priority: primary
    status: active
    hash: "sha256:aabb"
    owner: Bob
    license: MIT
    confidentiality: public
    allowed_uses:
      - citation_internal
`
	m, err := ParseSidecarManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(m.Sources))
	}
	if m.Sources[0].ID != "SRC-001" {
		t.Fatalf("expected SRC-001, got %q", m.Sources[0].ID)
	}
}

func TestSidecarErrorString(t *testing.T) {
	e := SidecarError{SourceID: "SRC-1", Code: CodeHashMissing, Field: "hash", Message: "hash is required"}
	s := e.Error()
	if s == "" {
		t.Fatal("expected non-empty error string")
	}
}

func TestValidAllConfidentialityLevels(t *testing.T) {
	for _, level := range []string{"public", "internal", "restricted", "secret"} {
		m := validManifest()
		m.Sources[0].Confidentiality = level
		result := ValidateSidecar(m, "")
		if !result.Valid {
			t.Fatalf("confidentiality %q should be valid, got: %v", level, result.Errors)
		}
	}
}

// --- helpers ---

func writeCorpusFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func assertInvalid(t *testing.T, result SidecarResult) {
	t.Helper()
	if result.Valid {
		t.Fatal("expected invalid result")
	}
}

func assertHasCode(t *testing.T, result SidecarResult, code string) {
	t.Helper()
	for _, e := range result.Errors {
		if e.Code == code {
			return
		}
	}
	t.Fatalf("expected error code %q in %v", code, result.Errors)
}
