package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var mfNow = time.Date(2026, 5, 3, 17, 0, 0, 0, time.UTC)

func collectedArtifacts() []EvidenceArtifact {
	return []EvidenceArtifact{
		{ID: "nomos-report", Type: "report", Path: "nomos-report.json", Hash: "sha256:1111", Status: "present", Producer: "nomos"},
		{ID: "coverage-report", Type: "report", Path: "coverage-report.md", Hash: "sha256:2222", Status: "present"},
		{ID: "attestation", Type: "attestation", Path: "attestation.json", Hash: "sha256:3333", Status: "present"},
		{ID: "gate-results", Type: "gate", Path: "gate-results.json", Hash: "sha256:4444", Status: "present"},
	}
}

func mfOpts() ManifestOptions {
	return ManifestOptions{
		ManifestID: "manifest-001",
		Domain:     "insurance",
		Now:        mfNow,
	}
}

func TestBuildManifestComplete(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	if !m.Complete {
		t.Fatal("expected complete manifest (all required present)")
	}
	if m.ManifestID != "manifest-001" {
		t.Fatalf("expected manifest-001, got %s", m.ManifestID)
	}
	if m.Domain != "insurance" {
		t.Fatalf("expected insurance, got %s", m.Domain)
	}
	if m.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", m.SchemaVersion)
	}
}

func TestBuildManifestIncomplete(t *testing.T) {
	// Only provide 2 of 4 required.
	collected := []EvidenceArtifact{
		{ID: "nomos-report", Type: "report", Path: "nomos-report.json", Hash: "sha256:1111", Status: "present"},
		{ID: "gate-results", Type: "gate", Path: "gate-results.json", Hash: "sha256:4444", Status: "present"},
	}
	m := BuildManifest(collected, DefaultRequiredArtifacts(), mfOpts())

	if m.Complete {
		t.Fatal("expected incomplete (missing attestation, coverage-report)")
	}
	if m.GateSummary.MissingCount < 2 {
		t.Fatalf("expected at least 2 missing, got %d", m.GateSummary.MissingCount)
	}
}

func TestBuildManifestCounts(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	if m.GateSummary.PresentCount != 4 {
		t.Fatalf("expected 4 present, got %d", m.GateSummary.PresentCount)
	}
	// 5 optional artifacts are missing.
	if m.GateSummary.MissingCount != 5 {
		t.Fatalf("expected 5 missing (optional), got %d", m.GateSummary.MissingCount)
	}
	if m.GateSummary.TotalArtifacts != 9 {
		t.Fatalf("expected 9 total, got %d", m.GateSummary.TotalArtifacts)
	}
}

func TestBuildManifestHash(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	if m.ManifestHash == "" {
		t.Fatal("expected manifest hash")
	}
	if !strings.HasPrefix(m.ManifestHash, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %s", m.ManifestHash)
	}
}

func TestBuildManifestHashDeterministic(t *testing.T) {
	m1 := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	m2 := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	if m1.ManifestHash != m2.ManifestHash {
		t.Fatal("expected deterministic hash")
	}
}

func TestVerifyManifestHashValid(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	if !VerifyManifestHash(m) {
		t.Fatal("expected hash to verify")
	}
}

func TestVerifyManifestHashTampered(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	m.Artifacts[0].Hash = "sha256:tampered"
	if VerifyManifestHash(m) {
		t.Fatal("expected tampered manifest to fail hash check")
	}
}

func TestVerifyManifestValid(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	errs := VerifyManifest(m)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestVerifyManifestMissingFields(t *testing.T) {
	m := EvidenceManifest{}
	errs := VerifyManifest(m)
	if len(errs) < 4 {
		t.Fatalf("expected multiple errors, got %d: %v", len(errs), errs)
	}
}

func TestVerifyManifestDuplicateID(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	m.Artifacts = append(m.Artifacts, EvidenceArtifact{ID: "nomos-report", Status: "present"})
	errs := VerifyManifest(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicated") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate error, got %v", errs)
	}
}

func TestMissingRequired(t *testing.T) {
	collected := []EvidenceArtifact{
		{ID: "nomos-report", Type: "report", Hash: "sha256:aa", Status: "present"},
	}
	m := BuildManifest(collected, DefaultRequiredArtifacts(), mfOpts())
	missing := MissingRequired(m, DefaultRequiredArtifacts())

	if len(missing) != 3 {
		t.Fatalf("expected 3 missing required, got %d: %v", len(missing), missing)
	}
}

func TestMissingRequiredAllPresent(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	missing := MissingRequired(m, DefaultRequiredArtifacts())
	if len(missing) != 0 {
		t.Fatalf("expected 0 missing, got %v", missing)
	}
}

func TestBuildManifestExtraArtifacts(t *testing.T) {
	collected := append(collectedArtifacts(), EvidenceArtifact{
		ID: "extra-custom", Type: "custom", Path: "custom.json", Hash: "sha256:extra", Status: "present",
	})
	m := BuildManifest(collected, DefaultRequiredArtifacts(), mfOpts())

	found := false
	for _, a := range m.Artifacts {
		if a.ID == "extra-custom" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected extra artifact in manifest")
	}
}

func TestBuildManifestStaleArtifact(t *testing.T) {
	collected := []EvidenceArtifact{
		{ID: "nomos-report", Type: "report", Hash: "sha256:old", Status: "stale"},
		{ID: "coverage-report", Type: "report", Hash: "sha256:2222", Status: "present"},
		{ID: "attestation", Type: "attestation", Hash: "sha256:3333", Status: "present"},
		{ID: "gate-results", Type: "gate", Hash: "sha256:4444", Status: "present"},
	}
	m := BuildManifest(collected, DefaultRequiredArtifacts(), mfOpts())

	if m.Complete {
		t.Fatal("expected incomplete when required artifact is stale")
	}
	if m.GateSummary.StaleCount != 1 {
		t.Fatalf("expected 1 stale, got %d", m.GateSummary.StaleCount)
	}
}

func TestWriteManifestJSON(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	var buf bytes.Buffer
	if err := WriteManifestJSON(&buf, m); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded EvidenceManifest
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.ManifestID != "manifest-001" {
		t.Fatalf("expected manifest-001, got %s", decoded.ManifestID)
	}
}

func TestWriteManifestYAML(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())

	var buf bytes.Buffer
	if err := WriteManifestYAMLFidelity(&buf, m); err != nil {
		t.Fatalf("write error: %v", err)
	}

	content := buf.String()
	if !strings.Contains(content, "manifest_id:") {
		t.Fatalf("expected YAML with manifest_id, got:\n%s", content)
	}
}

func TestFormatSummary(t *testing.T) {
	m := BuildManifest(collectedArtifacts(), DefaultRequiredArtifacts(), mfOpts())
	s := m.FormatSummary()

	if !strings.Contains(s, "manifest-001") {
		t.Fatalf("expected manifest ID in summary, got:\n%s", s)
	}
	if !strings.Contains(s, "insurance") {
		t.Fatalf("expected domain in summary, got:\n%s", s)
	}
}

func TestDefaultRequiredArtifacts(t *testing.T) {
	req := DefaultRequiredArtifacts()
	if len(req) < 4 {
		t.Fatalf("expected at least 4 required artifacts, got %d", len(req))
	}
	requiredCount := 0
	for _, r := range req {
		if r.Required {
			requiredCount++
		}
	}
	if requiredCount != 4 {
		t.Fatalf("expected 4 required=true, got %d", requiredCount)
	}
}
